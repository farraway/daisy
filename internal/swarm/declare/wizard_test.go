package declare

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// scripted is an Ops whose Prompt writes each question to its writer and
// answers from answers in order, EOF once they run out.
func scripted(answers ...string) (*ops.Fake, *[]string) {
	var asked []string
	return &ops.Fake{PromptFn: func(_ io.Reader, w io.Writer, q string) (string, error) {
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

// bareWorkspace is workspace without the prompts: what init finds.
func bareWorkspace(t *testing.T) string {
	ws := workspace(t)
	os.RemoveAll(filepath.Join(ws, "swarm"))
	return ws
}

func TestWizardDefaults(t *testing.T) {
	ws := bareWorkspace(t)
	o, asked := scripted()
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, WS: ws, Defaults: true}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	want := valid()
	want.Repos[0].Check, want.Repos[1].Protected = []string{}, defaultProtected
	want.Trusted = []string{"swarm-bot"}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("--defaults:\n%+v\nwant\n%+v", c, want)
	}
	if len(*asked) != 0 || errb.Len() != 0 {
		t.Errorf("--defaults asked %q, stderr %q", *asked, errb.String())
	}

	// A workspace that is itself the repository: `.`.
	hermetic(t)
	solo := t.TempDir()
	gitRepo(t, solo, "git@github.com:acme/solo.git")
	w = &Wizard{Ops: o, Err: &errb, WS: solo, Defaults: true}
	c, err = w.Ask()
	if err != nil || len(c.Repos) != 1 || c.Repos[0].Path != "." || c.Repos[0].Name != "solo" ||
		c.Forge != (Forge{Kind: "github", API: "https://api.github.com", Repo: "acme/solo", TokenEnv: "GITHUB_TOKEN"}) || len(c.Resources) != 0 {
		t.Errorf("solo: %v %+v", err, c)
	}
}

// The questions in order, each default in brackets (section 11's wizard).
func TestWizardAsksInOrder(t *testing.T) {
	ws := bareWorkspace(t)
	o, asked := scripted(make([]string, 40)...)
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, WS: ws}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Product repositories (comma-separated paths in the workspace, the product first) [app, lib]: ",
		"Forge kind (gitea, github) [gitea]: ",
		"Forge API [https://forge.example.net/api/v1]: ",
		"Issues repository [acme/app]: ",
		"Token variable [GITEA_TOKEN]: ",
		"Base branch [main]: ",
	}
	for _, r := range Roles {
		p := examplePersonas[r]
		want = append(want, r+" provider (anthropic, openai, xai) ["+p.Provider+"]: ", r+" model ["+p.Model+"]: ", r+" effort ["+p.Effort+"]: ",
			r+" co-author (- for none) ["+defaultCoAuthor(p.Provider, p.Model)+"]: ")
	}
	want = append(want, "app image [daisy-member:1]: ", "lib image [daisy-member:1]: ", "Max members [3]: ", "Max rounds [5]: ",
		"Timeout [45m]: ", "Interval [5m]: ", "Resources (comma-separated workspace paths, - for none) [docs/]: ",
		"Maintainer logins (comma-separated, not the swarm's; - for none) [-]: ", "Commit email [swarm@example.net]: ")
	if !reflect.DeepEqual(*asked, want) {
		t.Errorf("asked:\n%s\nwant:\n%s", strings.Join(*asked, "\n"), strings.Join(want, "\n"))
	}
	if errb.String() != strings.Join(want, "") {
		t.Errorf("the questions are not all on stderr: %q", errb.String())
	}
}

func TestWizardHonoursEachOverride(t *testing.T) {
	ws := bareWorkspace(t)
	answers := []string{"lib, app", "gitea", "https://forge.example.net:3000/api/v1", "acme/lib", "SWARM_TOKEN", "trunk"}
	for _, r := range Roles {
		answers = append(answers, "openai", r+"-model", "low", r+" Bot <"+r+"@example.net>")
	}
	answers = append(answers, "lib-image:2", "app-image:3", "4", "6", "1h", "2m", "-", "jane, joe", "commits@example.net")
	o, _ := scripted(answers...)
	w := &Wizard{Ops: o, Err: io.Discard, WS: ws}
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
	if c.Forge != (Forge{Kind: "gitea", API: "https://forge.example.net:3000/api/v1", Repo: "acme/lib", TokenEnv: "SWARM_TOKEN", CommitEmail: "commits@example.net"}) {
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
	ws := bareWorkspace(t)
	answers := make([]string, 40)
	answers[6+4*4] = "xai" // the sympathetic's provider
	o, asked := scripted(answers...)
	file := valid()
	w := &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
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
	if err := Validate(ops.Real(), ws, c, SkipPrompts); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}
}

// A file's xai login slot is kept by the wizard, as an openai key is:
// keys are not asked, either form stays as the file declares it (section
// 11).
func TestWizardKeepsTheFilesXAIForm(t *testing.T) {
	ws := bareWorkspace(t)
	answers := make([]string, 40)
	answers[6+4*4] = "xai"
	o, _ := scripted(answers...)
	file := valid()
	slots, env := 2, "OPENAI_API_KEY"
	file.Keys = map[string]Key{"anthropic": file.Keys["anthropic"], "openai": {Env: &env}, "xai": {Slots: &slots}}
	w := &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
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
	ws := bareWorkspace(t)
	answers := []string{"notthere", "", "", "", "", "", ""}
	answers = append(answers, "", "-", "", "", "")   // owner: provider, model "-" refused, model, effort, co-author
	answers = append(answers, make([]string, 16)...) // the other four roles
	answers = append(answers, "", "", "x")           // the two images, then max members "x" refused
	o, asked := scripted(answers...)
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, WS: ws}
	_, err := w.Ask()
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("want cancelled at EOF, got %v", err)
	}
	for _, want := range []string{
		`swarm.json: repos[0].path: "notthere" is not a git repository` + "\nProduct repositories",
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
	ws := bareWorkspace(t)
	file := valid()
	file.Trusted = []string{"jane", "Swarm-Bot"}
	file.Forge.CommitEmail = "commits@example.net"
	worker := file.Personas["worker"]
	worker.CoAuthor = "Pair <pair@example.net>"
	file.Personas["worker"] = worker

	o, asked := scripted("swarm-bot, joe", "", "")
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, WS: ws, File: &file}
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
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [Pair <pair@example.net>]: " {
		t.Errorf("unchanged model: %q", got)
	}
	answers[len(answers)-2] = "claude-opus-6"
	o, asked = scripted(answers...)
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [claude-opus-6 <noreply@anthropic.com>]: " {
		t.Errorf("changed model: %q", got)
	}

	// `-` is no co-author, ""; the file's "" is offered as `-` while the
	// model is the file's, and --defaults keeps it.
	answers[len(answers)-2] = ""
	o, asked = scripted(append(answers, "-")...)
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	got, _ := w.Ask()
	if got.Personas["worker"].CoAuthor != "" {
		t.Errorf("- is not none: %q", got.Personas["worker"].CoAuthor)
	}
	worker.CoAuthor = ""
	file.Personas["worker"] = worker
	o, asked = scripted(answers...)
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	w.Ask()
	if got := (*asked)[len(*asked)-1]; got != "worker co-author (- for none) [-]: " {
		t.Errorf("the file's none: %q", got)
	}
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file, Defaults: true}
	if got, err := w.Ask(); err != nil || got.Personas["worker"].CoAuthor != "" {
		t.Errorf("--defaults with the file's none: %q, %v", got.Personas["worker"].CoAuthor, err)
	}
}

// Question 35 for a product issues repository, the wiki: the file's
// answer the default, else no; anything but yes or no asked again;
// --defaults keeps the file's; notes cleared.
func TestWizardWiki(t *testing.T) {
	o, asked := scripted("", "maybe", "yes")
	var errb bytes.Buffer
	c := valid()
	c.Notes = true
	w := &Wizard{Ops: o, Err: &errb}
	if err := w.AskNotes(&c); err != nil || c.Wiki || c.Notes {
		t.Fatalf("default: %v %v %v", err, c.Wiki, c.Notes)
	}
	if err := w.AskNotes(&c); err != nil || !c.Wiki {
		t.Fatalf("yes: %v %v", err, c.Wiki)
	}
	if (*asked)[0] != "Research wiki, the issues repository's wiki (yes, no) [no]: " || len(*asked) != 3 || !strings.Contains(errb.String(), "[no]: swarm.json: wiki: \"maybe\" is not yes or no\nResearch wiki") {
		t.Errorf("asked %q, stderr %q", *asked, errb.String())
	}
	w = &Wizard{Ops: o, Err: io.Discard, Defaults: true}
	if err := w.AskNotes(&c); err != nil || !c.Wiki {
		t.Errorf("--defaults with the file's yes: %v %v", err, c.Wiki)
	}
}

// The swarm's own issues repository (section 11, question 4's follow-up
// and question 35), its clone left out of question 1's answer, as no
// product repository: an issues repository none of the declared ones is
// asked about, yes by default when its clone lies in the workspace; yes
// adds the clone's briefs/ to the resources' default and asks for notes
// in place of the wiki; no asks for the wiki, refusing yes (validation's
// reason, section 3), since the wiki keeps a declared issues repository's
// notes only; a declared one asks neither follow-up.
func TestWizardOwnIssuesRepository(t *testing.T) {
	ws := bareWorkspace(t)
	gitRepo(t, filepath.Join(ws, "swarm-x"), "git@forge.example.net:acme/swarm-x.git")
	os.MkdirAll(filepath.Join(ws, "swarm-x", "briefs"), 0o755)

	// Question 1's default leaves the clone of swarm-x out (section 3's
	// convention for the swarm's own repository): app and lib alone.
	o, _ := scripted()
	if c, err := (&Wizard{Ops: o, Err: io.Discard, WS: ws, Defaults: true}).Ask(); err != nil || len(c.Repos) != 2 || c.Repos[0].Name != "app" || c.Repos[1].Name != "lib" {
		t.Errorf("question 1's default with the swarm's own clone: %v %+v", err, c.Repos)
	}
	answers := func(issues, own string) []string {
		a := []string{"app, lib", "", "", issues}
		if own != "-" {
			a = append(a, own)
		}
		return append(a, make([]string, 40)...)
	}
	for _, tc := range []struct {
		name, issues, own string
		resources         []string
		last              string
		notes, wiki       bool
		refused           bool
	}{
		{name: "own", issues: "acme/swarm-x", own: "", resources: []string{"docs/", "swarm-x/briefs/"}, last: "Research notes, in acme/swarm-x's notes/ (yes, no) [no]: ", notes: true},
		{name: "not own", issues: "acme/swarm-x", own: "no", resources: []string{"docs/"}, last: "Research wiki, the issues repository's wiki (yes, no) [no]: ", refused: true},
		{name: "declared", issues: "acme/lib", own: "-", resources: []string{"docs/"}, last: "Research wiki, the issues repository's wiki (yes, no) [no]: ", wiki: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, asked := scripted(answers(tc.issues, tc.own)...)
			var errb bytes.Buffer
			w := &Wizard{Ops: o, Err: &errb, WS: ws}
			c, err := w.Ask()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.Resources, tc.resources) {
				t.Errorf("resources %q, want %q", c.Resources, tc.resources)
			}
			ownQ := "Issues repository acme/swarm-x: the swarm's own, holding no product code (yes, no) [yes]: "
			if got := (*asked)[4]; (tc.own != "-") != (got == ownQ) {
				t.Errorf("question 5 asked %q", got)
			}
			if got := (*asked)[len(*asked)-1]; !strings.HasPrefix(got, "Resources (comma-separated workspace paths, - for none) [") {
				t.Errorf("the last of Ask: %q", got)
			}
			yes := []string{"yes", "no"}
			o.PromptFn = func(_ io.Reader, _ io.Writer, q string) (string, error) {
				*asked = append(*asked, q)
				a := yes[0]
				yes = yes[1:]
				return a, nil
			}
			n := len(*asked)
			if err := w.AskNotes(&c); err != nil || c.Notes != tc.notes || c.Wiki != tc.wiki || (*asked)[len(*asked)-1] != tc.last {
				t.Errorf("question 35 %q: %v notes %v wiki %v", (*asked)[len(*asked)-1], err, c.Notes, c.Wiki)
			}
			refusal := "swarm.json: wiki: acme/swarm-x is none of repos; wiki keeps a declared issues repository's notes, and the swarm's own keeps them in notes/ (notes)\n"
			if again := len(*asked)-n == 2; again != tc.refused || strings.HasSuffix(errb.String(), refusal) != tc.refused {
				t.Errorf("question 35 asked %d times, stderr %q", len(*asked)-n, errb.String())
			}
		})
	}

	// Its default: yes for a file keeping its notes there, the clone gone.
	os.RemoveAll(filepath.Join(ws, "swarm-x"))
	file := valid()
	file.Forge.Repo, file.Notes = "acme/swarm-x", true
	o, asked := scripted(make([]string, 40)...)
	w := &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	if _, err := w.Ask(); err != nil || (*asked)[4] != "Issues repository acme/swarm-x: the swarm's own, holding no product code (yes, no) [yes]: " {
		t.Errorf("the file's own: %v %q", err, (*asked)[4])
	}
	// The file's notes are for its own forge.repo: another answered, no.
	o, asked = scripted("", "", "", "acme/elsewhere", "maybe", "")
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file}
	w.Ask()
	if (*asked)[4] != "Issues repository acme/elsewhere: the swarm's own, holding no product code (yes, no) [no]: " {
		t.Errorf("the file's notes, another issues repository: %q", *asked)
	}
	// --defaults keeping a file's wiki on an issues repository none of
	// repos fails with validation's reason.
	file.Notes, file.Wiki = false, true
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws, File: &file, Defaults: true}
	c := file
	if err := w.AskNotes(&c); errText(err) != "swarm.json: wiki: acme/swarm-x is none of repos; wiki keeps a declared issues repository's notes, and the swarm's own keeps them in notes/ (notes)" {
		t.Errorf("--defaults with the file's wiki on its own: %v", err)
	}
	o, asked = scripted("app, lib", "", "", "acme/elsewhere", "maybe", "")
	w = &Wizard{Ops: o, Err: io.Discard, WS: ws}
	w.Ask()
	if (*asked)[4] != "Issues repository acme/elsewhere: the swarm's own, holding no product code (yes, no) [no]: " || (*asked)[5] != (*asked)[4] {
		t.Errorf("no clone, no file: %q", *asked)
	}
}

func TestWizardDefaultsRefused(t *testing.T) {
	ws := bareWorkspace(t)
	o, _ := scripted()
	w := &Wizard{Ops: o, Err: io.Discard, WS: ws, Defaults: true}
	c, err := w.Ask()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AskAccount(&c, "swarm-bot", ""); errText(err) != "swarm.json: forge.commit_email: missing" {
		t.Errorf("no email to default to: %v", err)
	}
}
