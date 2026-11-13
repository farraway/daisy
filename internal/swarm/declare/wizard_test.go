package declare

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// scripted is an Ops whose Prompt writes each question to its writer and
// answers from answers in order, EOF once they run out; its environment
// is empty.
func scripted(answers ...string) (*ops.Fake, *[]string) {
	var asked []string
	return &ops.Fake{GetenvFn: func(string) string { return "" }, PromptFn: func(_ io.Reader, w io.Writer, q string) (string, error) {
		io.WriteString(w, q)
		asked = append(asked, q)
		if len(answers) == 0 {
			return "", io.EOF
		}
		a := answers[0]
		answers = answers[1:]
		return a, nil
	}}, &asked
}

func TestWizardDefaults(t *testing.T) {
	dir, _ := cloneFolder(t)
	o, asked := scripted()
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, Place: Folder(dir), Defaults: true}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	want := valid()
	want.Repos[0].Check, want.Repos[1].Protected = []string{}, defaultProtected
	want.Trusted, want.Resources = []string{"swarm-bot"}, []string{"briefs/", "docs/"}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("--defaults:\n%+v\nwant\n%+v", c, want)
	}
	if len(*asked) != 0 || errb.Len() != 0 {
		t.Errorf("--defaults asked %q, stderr %q", *asked, errb.String())
	}
	// A new entry protects the forges' workflows and what a member's CLI
	// loads from the repository (section 3).
	if p := []string{".github/", ".gitea/", ".forgejo/", ".claude/", ".grok/", "AGENTS.md", "CLAUDE.md"}; !reflect.DeepEqual(c.Repos[1].Protected, p) {
		t.Errorf("the default protected %q, want %q", c.Repos[1].Protected, p)
	}
}

// The questions in order, each default in brackets (section 11's wizard).
func TestWizardAsksInOrder(t *testing.T) {
	dir, _ := cloneFolder(t)
	o, asked := scripted(make([]string, 40)...)
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, Place: Folder(dir)}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Product repositories (comma-separated <owner>/<name>, the product first) [acme/app, acme/lib]: ",
		"Forge kind (gitea, github) [gitea]: ",
		"Forge API [https://forge.example.net/api/v1]: ",
		"Swarm repository [acme/swarm-app]: ",
		"Token variable [GITEA_TOKEN]: ",
		"Base branch [main]: ",
	}
	for _, r := range Roles {
		p := examplePersonas[r]
		want = append(want, r+" provider (anthropic, openai, xai) ["+p.Provider+"]: ", r+" model ["+p.Model+"]: ", r+" effort ["+p.Effort+"]: ",
			r+" co-author (- for none) ["+defaultCoAuthor(p.Provider, p.Model)+"]: ")
	}
	want = append(want, "app image [daisy-member:1]: ", "lib image [daisy-member:1]: ", "Max members [3]: ", "Max rounds [5]: ",
		"Timeout [45m]: ", "Interval [5m]: ", "Resources (comma-separated: paths in the swarm repository, or <name>:<path> in a declared one; - for none) [briefs/, docs/]: ",
		"Maintainer logins (comma-separated, not the swarm's; - for none) [-]: ", "Commit email [swarm@example.net]: ")
	if !reflect.DeepEqual(*asked, want) {
		t.Errorf("asked:\n%s\nwant:\n%s", strings.Join(*asked, "\n"), strings.Join(want, "\n"))
	}
	if errb.String() != strings.Join(want, "") {
		t.Errorf("the questions are not all on stderr: %q", errb.String())
	}
}

func TestWizardHonoursEachOverride(t *testing.T) {
	dir, _ := cloneFolder(t)
	answers := []string{"acme/lib, acme/app", "gitea", "https://forge.example.net:3000/api/v1", "acme/swarm-x", "SWARM_TOKEN", "trunk"}
	for _, r := range Roles {
		answers = append(answers, "openai", r+"-model", "low", r+" Bot <"+r+"@example.net>")
	}
	answers = append(answers, "lib-image:2", "app-image:3", "4", "6", "1h", "2m", "-", "jane, joe", "commits@example.net")
	o, _ := scripted(answers...)
	w := &Wizard{Ops: o, Err: io.Discard, Place: Folder(dir)}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	if c.Repos[0].Name != "lib" || c.Repos[0].Image != "lib-image:2" || c.Repos[1].Name != "app" || c.Repos[1].Image != "app-image:3" {
		t.Errorf("repos %+v", c.Repos)
	}
	if c.Forge != (Forge{Kind: "gitea", API: "https://forge.example.net:3000/api/v1", Repo: "acme/swarm-x", TokenEnv: "SWARM_TOKEN", CommitEmail: "commits@example.net"}) {
		t.Errorf("forge %+v", c.Forge)
	}
	if c.Base != "trunk" || c.MaxMembers != 4 || c.MaxRounds != 6 || c.Timeout != "1h" || c.Interval != "2m" || len(c.Resources) != 0 {
		t.Errorf("scalars %+v", c)
	}
	if !reflect.DeepEqual(c.Trusted, []string{"jane", "joe", "swarm-bot"}) {
		t.Errorf("trusted %q", c.Trusted)
	}
	for _, r := range Roles {
		if c.Personas[r] != (Persona{Provider: "openai", Model: r + "-model", Effort: "low", CoAuthor: r + " Bot <" + r + "@example.net>"}) {
			t.Errorf("%s: %+v", r, c.Personas[r])
		}
	}
}

// A role moved to another provider takes that provider's model as the
// default, and the co-author follows it; a provider no key names gets its
// default credential, the file's keys left as they were (section 11).
func TestWizardMovesARoleToXAI(t *testing.T) {
	_, clone := cloneFolder(t)
	answers := make([]string, 40)
	answers[6+4*4] = "xai" // the sympathetic's provider
	o, asked := scripted(answers...)
	file := valid()
	w := &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := (*asked)[6+4*4 : 6+5*4]; !reflect.DeepEqual(got, []string{"sympathetic provider (anthropic, openai, xai) [anthropic]: ",
		"sympathetic model [grok-4.7]: ", "sympathetic effort [high]: ", "sympathetic co-author (- for none) [grok-4.7 <noreply@x.ai>]: "}) {
		t.Errorf("asked %q", got)
	}
	if c.Personas["sympathetic"] != (Persona{Provider: "xai", Model: "grok-4.7", Effort: "high", CoAuthor: "grok-4.7 <noreply@x.ai>"}) {
		t.Errorf("sympathetic %+v", c.Personas["sympathetic"])
	}
	if k := c.Keys["xai"]; k.Env == nil || *k.Env != "XAI_API_KEY" || k.Slots != nil || len(c.Keys) != 3 {
		t.Errorf("keys %+v", c.Keys)
	}
	if _, ok := file.Keys["xai"]; ok {
		t.Error("the file's keys were changed")
	}
	if err := Validate(ops.Real(), Clone(clone), c, SkipPrompts); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}
}

// A file's xai login slot is kept by the wizard, as an openai key is:
// keys are not asked, either form stays as the file declares it (section
// 11).
func TestWizardKeepsTheFilesXAIForm(t *testing.T) {
	_, clone := cloneFolder(t)
	answers := make([]string, 40)
	answers[6+4*4] = "xai"
	o, _ := scripted(answers...)
	file := valid()
	slots, env := 2, "OPENAI_API_KEY"
	file.Keys = map[string]Key{"anthropic": file.Keys["anthropic"], "openai": {Env: &env}, "xai": {Slots: &slots}}
	w := &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	c, err := w.Ask()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Keys, file.Keys) {
		t.Errorf("keys %+v", c.Keys)
	}
}

// Every provider with an adapter has a default model and a default
// credential the validation accepts.
func TestProviderDefaults(t *testing.T) {
	for name := range providers {
		d, ok := providerDefaults[name]
		if !ok || d.model == "" {
			t.Errorf("%s: no default model", name)
		}
		personas := map[string]Persona{}
		for _, r := range Roles {
			personas[r] = Persona{Provider: name}
		}
		if err := checkKeys(map[string]Key{name: defaultKey(name)}, personas); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestWizardAsksARefusedAnswerAgain(t *testing.T) {
	dir, _ := cloneFolder(t)
	answers := []string{"notthere", "", "", "", "", "", ""}
	answers = append(answers, "", "-", "", "", "")   // owner: provider, model "-" refused, model, effort, co-author
	answers = append(answers, make([]string, 16)...) // the other four roles
	answers = append(answers, "", "", "x")           // the two images, then max members "x" refused
	o, asked := scripted(answers...)
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, Place: Folder(dir)}
	_, err := w.Ask()
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("want cancelled at EOF, got %v", err)
	}
	for _, want := range []string{
		`swarm.json: repos[0].repo: "notthere" is not <owner>/<name>` + "\nProduct repositories",
		`swarm.json: personas.owner.model: "-" is not a model; every role names its model` + "\nowner model [claude-opus-5-5]: ",
		`swarm.json: max_members: "x" is not an integer` + "\nMax members [3]: ",
	} {
		if !strings.Contains(errb.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, errb.String())
		}
	}
	if (*asked)[0] != (*asked)[1] {
		t.Errorf("question 1 not asked again: %q", (*asked)[:2])
	}
}

func TestWizardAccountQuestions(t *testing.T) {
	_, clone := cloneFolder(t)
	file := valid()
	file.Trusted = []string{"jane", "Swarm-Bot"}
	file.Forge.CommitEmail = "commits@example.net"
	worker := file.Personas["worker"]
	worker.CoAuthor = "Pair <pair@example.net>"
	file.Personas["worker"] = worker

	o, asked := scripted("swarm-bot, joe", "", "")
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, Place: Clone(clone), File: &file}
	c := file
	if err := w.AskAccount(&c, "swarm-bot", "swarm@example.net"); err != nil {
		t.Fatal(err)
	}
	if len(*asked) != 3 || (*asked)[0] != "Maintainer logins (comma-separated, not the swarm's; - for none) [jane]: " || (*asked)[2] != "Commit email [commits@example.net]: " {
		t.Errorf("asked %q", *asked)
	}
	if !strings.Contains(errb.String(), `swarm.json: trusted: "swarm-bot" is the token's own login; the swarm needs its own forge account (section 8)`) {
		t.Errorf("stderr %q", errb.String())
	}
	if !reflect.DeepEqual(c.Trusted, []string{"jane", "swarm-bot"}) || c.Forge.CommitEmail != "commits@example.net" {
		t.Errorf("trusted %q email %q", c.Trusted, c.Forge.CommitEmail)
	}

	// The co-author's default: the file's while the model is the file's,
	// the new model's once it changes.
	answers := make([]string, 6)
	answers = append(answers, "", "", "", "", "", "", "", "") // owner, architect
	answers = append(answers, "", "", "")                     // worker: provider, model, effort
	o, asked = scripted(answers...)
	w = &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [Pair <pair@example.net>]: " {
		t.Errorf("unchanged model: %q", got)
	}
	answers[len(answers)-2] = "claude-opus-6"
	o, asked = scripted(answers...)
	w = &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [claude-opus-6 <noreply@anthropic.com>]: " {
		t.Errorf("changed model: %q", got)
	}

	// `-` is no co-author, ""; the file's "" is offered as `-` while the
	// model is the file's, and --defaults keeps it.
	answers[len(answers)-2] = ""
	o, asked = scripted(append(answers, "-")...)
	w = &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	got, _ := w.Ask()
	if got.Personas["worker"].CoAuthor != "" {
		t.Errorf("- is not none: %q", got.Personas["worker"].CoAuthor)
	}
	worker.CoAuthor = ""
	file.Personas["worker"] = worker
	o, asked = scripted(answers...)
	w = &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [-]: " {
		t.Errorf("the file's none: %q", got)
	}
	w = &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file, Defaults: true}
	if got, err := w.Ask(); err != nil || got.Personas["worker"].CoAuthor != "" {
		t.Errorf("--defaults with the file's none: %q, %v", got.Personas["worker"].CoAuthor, err)
	}
}

// Question 35, the notes in the swarm repository's notes/: the file's
// answer the default, else no; anything but yes or no asked again;
// --defaults keeps the file's.
func TestWizardNotes(t *testing.T) {
	o, asked := scripted("", "maybe", "yes")
	var errb bytes.Buffer
	c := valid()
	w := &Wizard{Ops: o, Err: &errb}
	if err := w.AskNotes(&c); err != nil || c.Notes {
		t.Fatalf("default: %v %v", err, c.Notes)
	}
	if err := w.AskNotes(&c); err != nil || !c.Notes {
		t.Fatalf("yes: %v %v", err, c.Notes)
	}
	q := "Research notes, in acme/swarm-app's notes/ (yes, no) [no]: "
	if (*asked)[0] != q || len(*asked) != 3 || !strings.Contains(errb.String(), "[no]: swarm.json: notes: \"maybe\" is not yes or no\n"+q) {
		t.Errorf("asked %q, stderr %q", *asked, errb.String())
	}
	w = &Wizard{Ops: o, Err: io.Discard, Defaults: true}
	if err := w.AskNotes(&c); err != nil || !c.Notes {
		t.Errorf("--defaults with the file's yes: %v %v", err, c.Notes)
	}
}

func TestWizardDefaultsRefused(t *testing.T) {
	dir, _ := cloneFolder(t)
	o, _ := scripted()
	w := &Wizard{Ops: o, Err: io.Discard, Place: Folder(dir), Defaults: true}
	c, err := w.Ask()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AskAccount(&c, "swarm-bot", ""); errText(err) != "swarm.json: forge.commit_email: missing" {
		t.Errorf("no email to default to: %v", err)
	}
}

// Question 3's default (section 11; revision 6's plan, Step 6b): `init
// --api <url>`, else $GITEA_API_URL or $GITHUB_API_URL by the kind
// answered, else the guess from an SSH origin, which carries neither
// scheme nor port; a file's forge.api over the environment, the flag
// over the file; interactively the chosen value is the default shown.
func TestWizardForgeAPI(t *testing.T) {
	dir, _ := cloneFolder(t)
	const port, other = "http://forge.example.net:3000/api/v1", "https://elsewhere.example.net/api/v1"
	for _, tc := range []struct {
		name, flag string
		vars       map[string]string
		answers    []string // questions 1 and 2, when not --defaults
		file       bool
		want       string
	}{
		{name: "the guess", want: "https://forge.example.net/api/v1"},
		{name: "the flag", flag: port, want: port},
		{name: "the variable", vars: map[string]string{"GITEA_API_URL": port}, want: port},
		{name: "the flag over the variable", flag: port, vars: map[string]string{"GITEA_API_URL": other}, want: port},
		{name: "an empty variable", vars: map[string]string{"GITEA_API_URL": ""}, want: "https://forge.example.net/api/v1"},
		{name: "the other kind's variable", vars: map[string]string{"GITHUB_API_URL": other}, want: "https://forge.example.net/api/v1"},
		{name: "kind github's variable", vars: map[string]string{"GITEA_API_URL": port, "GITHUB_API_URL": other}, answers: []string{"", "github"}, want: other},
		{name: "the file over the variable", vars: map[string]string{"GITEA_API_URL": other}, file: true, want: "https://forge.example.net/api/v1"},
		{name: "the flag over the file", flag: port, vars: map[string]string{"GITEA_API_URL": other}, file: true, want: port},
		{name: "another kind than the file's: detected", answers: []string{"", "github"}, file: true, want: "https://api.github.com"},
		{name: "another kind than the file's: its variable", vars: map[string]string{"GITHUB_API_URL": other}, answers: []string{"", "github"}, file: true, want: other},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, interactive := range []bool{false, true} {
				answers := tc.answers
				if interactive && answers == nil {
					answers = []string{"", ""}
				}
				o, asked := scripted(append(answers, "")...)
				o.GetenvFn = func(k string) string { return tc.vars[k] }
				w := &Wizard{Ops: o, Err: io.Discard, Place: Folder(dir), Defaults: !interactive && tc.answers == nil, API: tc.flag}
				if tc.file {
					f := valid()
					w.File = &f
				}
				if !w.Defaults {
					w.Ask() // EOF after question 3
					if len(*asked) < 3 || (*asked)[2] != "Forge API ["+tc.want+"]: " {
						t.Errorf("interactive: asked %q", *asked)
					}
					continue
				}
				c, err := w.Ask()
				if err != nil || c.Forge.API != tc.want {
					t.Errorf("--defaults: %v %q, want %q", err, c.Forge.API, tc.want)
				}
			}
		})
	}

	// The flag's value validated as question 3's answer is.
	o, _ := scripted()
	w := &Wizard{Ops: o, Err: io.Discard, Place: Folder(dir), Defaults: true, API: "forge.example.net:3000"}
	if _, err := w.Ask(); errText(err) != `swarm.json: forge.api: "forge.example.net:3000" is not an absolute http(s) URL without credentials` {
		t.Errorf("an invalid --api: %v", err)
	}
}

// Question 2 answered with another kind than the file's: questions 3 and
// 5 default from detection for that kind, not from the file's forge; and
// another kind than the detected one keeps the origin's scheme and port
// (section 10, Detection; section 11, the wizard).
func TestWizardKindChangedFromTheFile(t *testing.T) {
	dir, _ := cloneFolder(t)
	other := t.TempDir()
	gitRepo(t, filepath.Join(other, "app"), "http://github.com:8080/acme/app.git")
	gitea := valid()
	github := valid()
	github.Forge.Kind, github.Forge.API, github.Forge.TokenEnv = "github", "https://api.github.com", "GITHUB_TOKEN"
	for _, tc := range []struct {
		name, dir  string
		file       *Config
		kind       string
		api, token string
	}{
		{"the file's kind", dir, &gitea, "gitea", "https://forge.example.net/api/v1", "GITEA_TOKEN"},
		{"gitea to github", dir, &gitea, "github", "https://api.github.com", "GITHUB_TOKEN"},
		{"github to gitea", dir, &github, "gitea", "https://forge.example.net/api/v1", "GITEA_TOKEN"},
		{"no file: gitea on an http origin on github.com", other, nil, "gitea", "http://github.com:8080/api/v1", "GITEA_TOKEN"},
	} {
		o, asked := scripted("", tc.kind, "", "")
		w := &Wizard{Ops: o, Err: io.Discard, Place: Folder(tc.dir), File: tc.file}
		w.Ask() // EOF after question 4
		if len(*asked) < 5 || (*asked)[2] != "Forge API ["+tc.api+"]: " || (*asked)[4] != "Token variable ["+tc.token+"]: " {
			t.Errorf("%s: asked %q", tc.name, *asked)
		}
	}
}
