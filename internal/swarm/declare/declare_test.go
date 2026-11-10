package declare

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// hermetic keeps git from reading the developer's configuration.
func hermetic(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitRepo makes dir a repository whose origin is url ("" for none).
func gitRepo(t *testing.T, dir, url string) {
	t.Helper()
	o := ops.Real()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, errb, err := o.Exec(dir, "git", "init", "-q"); err != nil {
		t.Fatalf("git init: %v %s", err, errb)
	}
	if url != "" {
		if _, errb, err := o.Exec(dir, "git", "remote", "add", "origin", url); err != nil {
			t.Fatalf("git remote add: %v %s", err, errb)
		}
	}
}

// workspace is a temp workspace with the product at app and its library
// at lib, both of the organization acme on forge.example.net, a docs/
// directory and the five prompts.
func workspace(t *testing.T) string {
	t.Helper()
	hermetic(t)
	ws := t.TempDir()
	gitRepo(t, filepath.Join(ws, "app"), "git@forge.example.net:acme/app.git")
	gitRepo(t, filepath.Join(ws, "lib"), "https://forge.example.net/acme/lib.git")
	write(t, filepath.Join(ws, "docs", "roadmap.md"), "# Roadmap\n")
	for _, r := range Roles {
		write(t, PromptFile(Workspace(ws).Dir, r), DefaultPrompts[r])
	}
	return ws
}

func valid() Config {
	envVar, slots := "CLAUDE_CODE_OAUTH_TOKEN", 1
	personas := map[string]Persona{}
	for r, p := range examplePersonas {
		p.CoAuthor = defaultCoAuthor(p.Provider, p.Model)
		personas[r] = p
	}
	return Config{
		Forge: Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/app", TokenEnv: "GITEA_TOKEN", CommitEmail: "swarm@example.net"},
		Repos: []Repo{
			{Name: "app", Path: "app", Check: []string{"make", "check"}, Image: defaultImage, Protected: defaultProtected},
			{Name: "lib", Path: "lib", Check: []string{}, Image: defaultImage, Protected: []string{}},
		},
		Base: "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: []string{"docs/"},
		Trusted:   []string{"jane", "swarm-bot"},
		Gates:     Gates{Triage: true, Plan: true, Merge: true},
		Keys:      map[string]Key{"anthropic": {Env: &envVar}, "openai": {Slots: &slots}},
		Personas:  personas,
	}
}

func ptr[T any](v T) *T { return &v }

// Every row of section 3's error table, one case each, the message byte
// for byte (after `Error: `).
func TestValidateNamesTheField(t *testing.T) {
	ws := workspace(t)
	gitRepo(t, filepath.Join(ws, "plain"), "")
	gitRepo(t, filepath.Join(ws, "leaky"), "https://jane:tok@forge.example.net/acme/leaky.git")
	gitRepo(t, filepath.Join(ws, "elsewhere"), "git@other.example.net:acme/elsewhere.git")
	gitRepo(t, filepath.Join(ws, "theirs"), "git@forge.example.net:someone/theirs.git")
	gitRepo(t, filepath.Join(ws, "local"), "/srv/git/local.git")
	os.MkdirAll(filepath.Join(ws, "app", "sub"), 0o755)
	os.MkdirAll(filepath.Join(ws, "notrepo"), 0o755)
	write(t, filepath.Join(ws, "secrets", ".env.sh"), "")
	os.MkdirAll(filepath.Join(ws, "bound", "deep", ".daisy"), 0o755)
	os.MkdirAll(filepath.Join(ws, "swarm", "notes"), 0o755)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(ws, "escape"))
	os.Symlink("docs", filepath.Join(ws, "docs-link"))

	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: forge.kind: "gitlab" has no adapter`, func(c *Config) { c.Forge.Kind = "gitlab" }},
		{`swarm.json: forge.api: "https://jane@forge.example.net/api/v1" is not an absolute http(s) URL without credentials`, func(c *Config) { c.Forge.API = "https://jane@forge.example.net/api/v1" }},
		{`swarm.json: forge.api: "https://forge.example.net/api/v1/" is not an absolute http(s) URL without credentials`, func(c *Config) { c.Forge.API += "/" }},
		{`swarm.json: forge.api: "forge.example.net/api/v1" is not an absolute http(s) URL without credentials`, func(c *Config) { c.Forge.API = "forge.example.net/api/v1" }},
		{`swarm.json: forge.repo: "acme" is not <owner>/<repo>`, func(c *Config) { c.Forge.Repo = "acme" }},
		{`swarm.json: forge.token_env: "GITEA-TOKEN" is not a variable name`, func(c *Config) { c.Forge.TokenEnv = "GITEA-TOKEN" }},
		{`swarm.json: forge.token_env: "DAISY_MAINTAINER_TOKEN" is the maintainer's token; the swarm's is its own account's (section 8)`, func(c *Config) { c.Forge.TokenEnv = "DAISY_MAINTAINER_TOKEN" }},
		{`swarm.json: forge.token_env: "CLAUDE_CODE_OAUTH_TOKEN" is anthropic's credential variable, which a member is given; the forge token never enters a container (section 8)`, func(c *Config) { c.Forge.TokenEnv = "CLAUDE_CODE_OAUTH_TOKEN" }},
		{`swarm.json: forge.commit_email: missing`, func(c *Config) { c.Forge.CommitEmail = "" }},
		{`swarm.json: forge.commit_email: "swarm at example.net" is not an email address`, func(c *Config) { c.Forge.CommitEmail = "swarm at example.net" }},
		{`swarm.json: forge.commit_email: "swarm\v@example.net" is not an email address`, func(c *Config) { c.Forge.CommitEmail = "swarm\v@example.net" }},
		{`swarm.json: forge.commit_email: "swarm@exa\u2003mple.net" is not an email address`, func(c *Config) { c.Forge.CommitEmail = "swarm@exa\u2003mple.net" }},
		{`swarm.json: forge.board: github has no board in version 1`, func(c *Config) {
			c.Forge.Kind, c.Forge.API, c.Forge.Board = "github", "https://api.github.com", ptr("Swarm")
		}},
		{`swarm.json: forge.board: empty`, func(c *Config) { c.Forge.Board = ptr("") }},
		{`swarm.json: repos: no repository (at least one is required)`, func(c *Config) { c.Repos = nil }},
		{`swarm.json: repos[1].name: "-lib" is not a repository name`, func(c *Config) { c.Repos[1].Name = "-lib" }},
		{`swarm.json: repos[1].name: "app" is listed twice`, func(c *Config) { c.Repos[1].Name = "app" }},
		{`swarm.json: repos[0].path: "notrepo" is not a git repository`, func(c *Config) { c.Repos[0].Path = "notrepo" }},
		{`swarm.json: repos[0].path: "docs" is not a git repository`, func(c *Config) { c.Repos[0].Path = "docs" }},
		{`swarm.json: repos[0].path: "app/sub" is not a git repository`, func(c *Config) { c.Repos[0].Path = "app/sub" }},
		{`swarm.json: repos[0].path: origin is "/srv/git/local.git", not <owner>/<repo> on a forge`, func(c *Config) { c.Repos[0].Path, c.Repos[0].Name = "local", "local" }},
		{`swarm.json: repos[0].path: "../app" is not a relative path inside the workspace`, func(c *Config) { c.Repos[0].Path = "../app" }},
		{`swarm.json: repos[0].path: "plain" has no origin remote`, func(c *Config) { c.Repos[0].Path = "plain" }},
		{`swarm.json: repos[0].path: origin URL carries user information; use SSH or a credential helper`, func(c *Config) { c.Repos[0].Path, c.Repos[0].Name = "leaky", "leaky" }},
		{`swarm.json: repos[1].path: origin is "git@other.example.net:acme/elsewhere.git", not acme/elsewhere on forge.example.net`, func(c *Config) { c.Repos[1].Path, c.Repos[1].Name = "elsewhere", "elsewhere" }},
		{`swarm.json: repos[1].path: origin is "git@forge.example.net:someone/theirs.git", not acme/theirs on forge.example.net`, func(c *Config) { c.Repos[1].Path, c.Repos[1].Name = "theirs", "theirs" }},
		{`swarm.json: repos[1].path: origin is "https://forge.example.net/acme/lib.git", not acme/library on forge.example.net`, func(c *Config) { c.Repos[1].Name = "library" }},
		{`swarm.json: repos[0].check[1]: empty argument`, func(c *Config) { c.Repos[0].Check = []string{"make", ""} }},
		{`swarm.json: repos[0].image: empty`, func(c *Config) { c.Repos[0].Image = "" }},
		{`swarm.json: repos[0].protected[2]: "../ci/" is not a relative pattern without ..`, func(c *Config) { c.Repos[0].Protected = []string{"a/", "b", "../ci/"} }},
		{`swarm.json: repos[0].protected[0]: "" is not a relative pattern without ..`, func(c *Config) { c.Repos[0].Protected = []string{""} }},
		{`swarm.json: base: "main.lock" is not a branch name`, func(c *Config) { c.Base = "main.lock" }},
		{`swarm.json: base: "a..b" is not a branch name`, func(c *Config) { c.Base = "a..b" }},
		{`swarm.json: max_members: 0 is not between 1 and 16`, func(c *Config) { c.MaxMembers = 0 }},
		{`swarm.json: max_rounds: 11 is not between 1 and 10`, func(c *Config) { c.MaxRounds = 11 }},
		{`swarm.json: timeout: "30s" is not between 1m and 24h`, func(c *Config) { c.Timeout = "30s" }},
		{`swarm.json: interval: "5 minutes" is not a Go duration`, func(c *Config) { c.Interval = "5 minutes" }},
		{`swarm.json: owner.max_open_proposals: 21 is not between 1 and 20`, func(c *Config) { c.Owner.MaxOpenProposals = 21 }},
		{`swarm.json: resources[1]: "." is the workspace`, func(c *Config) { c.Resources = []string{"docs/", "."} }},
		{`swarm.json: resources[0]: "/etc" is not a relative path`, func(c *Config) { c.Resources = []string{"/etc"} }},
		{`swarm.json: resources[0]: "missing/" does not exist`, func(c *Config) { c.Resources = []string{"missing/"} }},
		{`swarm.json: resources[0]: "escape" is outside the workspace`, func(c *Config) { c.Resources = []string{"escape"} }},
		{`swarm.json: resources[0]: "secrets" contains .env.sh`, func(c *Config) { c.Resources = []string{"secrets"} }},
		{`swarm.json: resources[0]: "bound" contains .daisy`, func(c *Config) { c.Resources = []string{"bound"} }},
		{`swarm.json: resources[0]: "app" contains .git`, func(c *Config) { c.Resources = []string{"app"} }},
		{`swarm.json: resources[0]: "app/.git/hooks" contains .git`, func(c *Config) { c.Resources = []string{"app/.git/hooks"} }},
		{`swarm.json: resources[0]: "swarm/notes" is under swarm/`, func(c *Config) { c.Resources = []string{"swarm/notes"} }},
		{`swarm.json: resources[1]: "docs-link" is listed twice`, func(c *Config) { c.Resources = []string{"docs/", "docs-link"} }},
		{`swarm.json: wiki: true beside notes; the swarm's own repository keeps its notes in notes/, and its wiki is no better than its tree`, func(c *Config) {
			c.Forge.Repo, c.Wiki, c.Notes = "acme/swarm-x", true, true
		}},
		{`swarm.json: wiki: acme/swarm-x is none of repos; wiki keeps a declared issues repository's notes, and the swarm's own keeps them in notes/ (notes)`, func(c *Config) { c.Forge.Repo, c.Wiki = "acme/swarm-x", true }},
		{`swarm.json: notes: acme/lib is a declared repository, which holds product code; notes/ is for the swarm's own issues repository (wiki keeps a product's)`, func(c *Config) { c.Forge.Repo, c.Notes = "acme/lib", true }},
		{`swarm.json: trusted: no login (at least one is required)`, func(c *Config) { c.Trusted = nil }},
		{`swarm.json: trusted[1]: "jane doe" is not a forge login`, func(c *Config) { c.Trusted = []string{"swarm-bot", "jane doe"} }},
		{`swarm.json: budget: missing (required when a gate is false)`, func(c *Config) { c.Gates.Merge = false }},
		{`swarm.json: budget.per_day: 0 is not greater than 0`, func(c *Config) { c.Budget = &Budget{PerDay: 0, Total: 10} }},
		{`swarm.json: budget.total: -1.5 is not greater than 0`, func(c *Config) { c.Budget = &Budget{PerDay: 1, Total: -1.5} }},
		{`swarm.json: personas.tester: not a role (owner, architect, worker, adversary, sympathetic)`, func(c *Config) { c.Personas["tester"] = c.Personas["worker"] }},
		{`swarm.json: personas.owner: missing`, func(c *Config) { delete(c.Personas, "owner") }},
		{`swarm.json: personas.adversary.provider: "gemini" has no adapter`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Provider = "gemini" }) }},
		{`swarm.json: personas.adversary.model: empty; every role names its model`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Model = "" }) }},
		{`swarm.json: personas.adversary.model: "-" is not a model; every role names its model`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Model = "-" }) }},
		{`swarm.json: personas.worker.model: "a b" contains whitespace`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a b" }) }},
		{`swarm.json: personas.worker.model: "a\u2003b" contains whitespace`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a\u2003b" }) }},
		{`swarm.json: personas.worker.effort: "Very High" is not one lowercase word`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Effort = "Very High" }) }},
		{`swarm.json: personas.worker.co_author: "claude-sonnet-5-5" is not <name> <email>`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.CoAuthor = "claude-sonnet-5-5" }) }},
		{`swarm.json: personas.worker.co_author: " " is not <name> <email>`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.CoAuthor = " " }) }},
		{`swarm.json: personas.worker.co_author: "m <a b@x.y>" is not <name> <email>`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.CoAuthor = "m <a b@x.y>" }) }},
		{`swarm.json: personas.worker.co_author: "m  <a@b>" is not <name> <email>`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.CoAuthor = "m  <a@b>" }) }},
		{`swarm.json: personas.worker.co_author: "m\rx <a@b>" is not <name> <email>`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.CoAuthor = "m\rx <a@b>" }) }},
		{`swarm.json: keys: no credential for provider "openai"`, func(c *Config) { delete(c.Keys, "openai") }},
		{`swarm.json: keys.openai: needs exactly one of env, slots`, func(c *Config) { c.Keys["openai"] = Key{Env: ptr("OPENAI_API_KEY"), Slots: ptr(1)} }},
		{`swarm.json: keys.openai: needs exactly one of env, slots`, func(c *Config) { c.Keys["openai"] = Key{} }},
		{`swarm.json: keys.anthropic.env: "ANTHROPIC_KEY" is not one of anthropic's variables (ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN)`, func(c *Config) { c.Keys["anthropic"] = Key{Env: ptr("ANTHROPIC_KEY")} }},
		{`swarm.json: keys.anthropic.slots: anthropic takes no slot`, func(c *Config) { c.Keys["anthropic"] = Key{Slots: ptr(1)} }},
		{`swarm.json: keys.openai.slots: 0 is not between 1 and 16`, func(c *Config) { c.Keys["openai"] = Key{Slots: ptr(0)} }},
		{`swarm.json: keys.xai: needs exactly one of env, slots`, func(c *Config) { c.Keys["xai"] = Key{Env: ptr("XAI_API_KEY"), Slots: ptr(1)} }},
		{`swarm.json: keys.xai.slots: 17 is not between 1 and 16`, func(c *Config) { c.Keys["xai"] = Key{Slots: ptr(17)} }},
		{`swarm.json: keys.gemini: "gemini" has no adapter`, func(c *Config) { c.Keys["gemini"] = Key{Env: ptr("GEMINI_API_KEY")} }},
	} {
		c := valid()
		c.Keys = map[string]Key{"anthropic": c.Keys["anthropic"], "openai": c.Keys["openai"]}
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), Workspace(ws), c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}

	if err := Validate(ops.Real(), Workspace(ws), valid(), 0); err != nil {
		t.Errorf("the valid declaration: %v", err)
	}
	for _, k := range []Key{{Slots: ptr(2)}, {Env: ptr("XAI_API_KEY")}} { // xai takes either form (section 8)
		x := valid()
		setPersona(&x, "sympathetic", func(p *Persona) { p.Provider, p.Model, p.CoAuthor = "xai", "grok-4.5", "grok-4.5 <noreply@x.ai>" })
		x.Keys["xai"] = k
		if err := Validate(ops.Real(), Workspace(ws), x, 0); err != nil {
			t.Errorf("xai %+v: %v", k, err)
		}
	}
	none := valid()
	setPersona(&none, "worker", func(p *Persona) { p.CoAuthor = "" })
	if err := Validate(ops.Real(), Workspace(ws), none, 0); err != nil {
		t.Errorf(`co_author "", no trailer: %v`, err)
	}
	os.Remove(PromptFile(Workspace(ws).Dir, "architect"))
	write(t, PromptFile(Workspace(ws).Dir, "worker"), "")
	if got := errText(Validate(ops.Real(), Workspace(ws), valid(), 0)); got != "swarm.json: personas.architect: prompt swarm/prompts/architect.md is missing or empty" {
		t.Errorf("missing prompt: %s", got)
	}
	c := valid()
	c.Trusted, c.Forge.CommitEmail = nil, ""
	if err := Validate(ops.Real(), Workspace(ws), c, SkipPrompts|SkipAccount); err != nil {
		t.Errorf("init's first validation: %v", err)
	}
}

func setPersona(c *Config, role string, edit func(*Persona)) {
	p := c.Personas[role]
	edit(&p)
	c.Personas[role] = p
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestParseNamesTheField(t *testing.T) {
	good := string(Format(valid()))
	for _, tc := range []struct{ from, to, want string }{
		{`"max_members": 3`, `"max_members": "3"`, "swarm.json: max_members: cannot unmarshal string into Go value of type int"},
		{`"base": "main",`, `"base": "main", "colour": "red",`, "swarm.json: colour: unknown field"},
		{`"effort": "medium",`, `"effort": "medium", "temperature": 1,`, "swarm.json: personas.worker.temperature: unknown field"},
		{`"check": ["make", "check"]`, `"check": "make check"`, "swarm.json: repos[0].check: not an array"},
		{`"protected": []}`, `"protected": [1]}`, "swarm.json: repos[1].protected[0]: cannot unmarshal number into Go value of type string"},
		{`"owner": {"max_open_proposals": 5},`, `"owner": {},`, "swarm.json: owner.max_open_proposals: missing"},
		{`"owner": {"max_open_proposals": 5},`, `"owner": [],`, "swarm.json: owner: not an object"},
		{`"merge": true`, `"merge": null`, "swarm.json: gates.merge: null is not a boolean"}, // not a gate silently off
		{`"max_members": 3`, `"max_members": 1e400`, "swarm.json: max_members: cannot unmarshal number 1e400 into Go value of type int"},
		{`"check": ["make", "check"]`, `"check": null`, "swarm.json: repos[0].check: null is not an array"},
	} {
		src := strings.Replace(good, tc.from, tc.to, 1)
		if src == good {
			t.Fatalf("%q not in the formatted file", tc.from)
		}
		_, err := Parse([]byte(src))
		if got := errText(err); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}
	src := strings.Replace(good, `"token_env": "GITEA_TOKEN",`+"\n"+`    "commit_email": "swarm@example.net"`, `"token_env": "GITEA_TOKEN"`, 1)
	if _, err := Parse([]byte(src)); errText(err) != "swarm.json: forge.commit_email: missing" {
		t.Errorf("commit_email absent: %v", err)
	}
	src = strings.Replace(good, `"co_author": "claude-sonnet-5-5 <noreply@anthropic.com>"`, `"co": 1`, 1)
	if _, err := Parse([]byte(src)); errText(err) != "swarm.json: personas.worker.co: unknown field" {
		t.Errorf("co_author renamed: %v", err)
	}
	if _, err := Parse([]byte(`{"forge": }`)); errText(err) != "swarm.json: invalid character '}' looking for beginning of value at byte 11" {
		t.Errorf("syntax: %v", err)
	}
	c, err := Parse([]byte(good))
	if err != nil || !reflect.DeepEqual(c, valid()) {
		t.Errorf("round trip: %v\n%+v", err, c)
	}
	ws := t.TempDir()
	if _, found, err := Load(ops.Real().FS(), Workspace(ws).Dir); found || errText(err) != "swarm.json: not found at "+ws+"/swarm/swarm.json; run 'daisy swarm init'" {
		t.Errorf("absent: found %v, %v", found, err)
	}
}

// Format writes section 3's example byte for byte. Until Step 3 of
// revision 6's plan builds repos[].repo, it writes the old layout's form
// of it, which section 11 (Moving a swarm into its repository) holds;
// that step points this test back at section 3.
func TestFormatIsSectionThreesExample(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("(?s)### Moving a swarm into its repository.*?```json\n(.*?)```").FindSubmatch(doc)
	if m == nil {
		t.Fatal("no example of the old layout in section 11")
	}
	envVar, slots := "CLAUDE_CODE_OAUTH_TOKEN", 1
	personas := map[string]Persona{}
	for r, p := range examplePersonas {
		p.CoAuthor = defaultCoAuthor(p.Provider, p.Model)
		personas[r] = p
	}
	c := Config{
		Forge: Forge{Kind: "gitea", API: "https://<forge-host>/api/v1", Repo: "<owner>/<repo>", TokenEnv: "GITEA_TOKEN", CommitEmail: "<swarm-email>"},
		Repos: []Repo{{Name: "<repo>", Path: "app", Image: defaultImage, Protected: defaultProtected}},
		// max_rounds 5 is revision 5's default (section 3); the wizard
		// still offers 3 until the code step that carries it.
		Base: "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: []string{"docs/"},
		Trusted:   []string{"<maintainer-login>", "<swarm-login>"},
		Gates:     Gates{Triage: true, Plan: true, Merge: true},
		Keys:      map[string]Key{"anthropic": {Env: &envVar}, "openai": {Slots: &slots}},
		Personas:  personas,
	}
	if got := Format(c); !bytes.Equal(got, m[1]) {
		t.Errorf("Format:\n%s\nsection 3:\n%s", got, m[1])
	}
}

// A budget and a board are written in their places, and read back.
func TestFormatKeepsBudgetAndBoard(t *testing.T) {
	c := valid()
	c.Gates.Merge, c.Budget, c.Forge.Board = false, &Budget{PerDay: 5, Total: 50.5}, ptr("Swarm board")
	out := string(Format(c))
	for _, want := range []string{
		`    "commit_email": "swarm@example.net",` + "\n" + `    "board": "Swarm board"` + "\n  },",
		`  "gates": {"triage": true, "plan": true, "merge": false},` + "\n" + `  "budget": {"per_day": 5, "total": 50.5},` + "\n" + `  "keys": `,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format lacks %q:\n%s", want, out)
		}
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
}

// `wiki` is written after resources only when true, absent read as false
// (a file written before Step 19 is unchanged), and null refused.
func TestFormatWiki(t *testing.T) {
	c := valid()
	if strings.Contains(string(Format(c)), "wiki") {
		t.Errorf("wiki written while false:\n%s", Format(c))
	}
	c.Wiki = true
	out := string(Format(c))
	if !strings.Contains(out, `  "resources": ["docs/"],`+"\n"+`  "wiki": true,`+"\n"+`  "trusted": `) {
		t.Errorf("Format:\n%s", out)
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
	if _, err := Parse([]byte(strings.Replace(out, `"wiki": true`, `"wiki": null`, 1))); errText(err) != "swarm.json: wiki: null is not a boolean" {
		t.Errorf("null: %v", err)
	}
}

// The swarm's own issues repository (section 3, notes): its clone in the
// workspace, none of repos, holds the briefs under briefs/, a resource
// like any other, and is no product repository (Declares finds it not, so
// no mirror, worker, check or image is its); notes: true is valid for it,
// written after wiki's place and read back.
func TestOwnIssuesRepository(t *testing.T) {
	ws := workspace(t)
	gitRepo(t, filepath.Join(ws, "swarm-x"), "git@forge.example.net:acme/swarm-x.git")
	write(t, filepath.Join(ws, "swarm-x", "briefs", "m0.md"), "Status: active\n\n## M1\n")
	c := valid()
	c.Forge.Repo, c.Notes, c.Resources = "acme/swarm-x", true, []string{"docs/", "swarm-x/briefs/"}
	if err := Validate(ops.Real(), Workspace(ws), c, 0); err != nil {
		t.Fatalf("own issues repository: %v", err)
	}
	if _, ok := c.Declares(c.Forge.Repo); ok || len(c.Repos) != 2 || !c.KeepsNotes() {
		t.Errorf("the issues repository declared: %+v", c.Repos)
	}
	if r, ok := c.Declares("acme/lib"); !ok || r.Path != "lib" {
		t.Errorf("Declares(acme/lib) = %+v, %v", r, ok)
	}
	out := string(Format(c))
	if !strings.Contains(out, `  "resources": ["docs/", "swarm-x/briefs/"],`+"\n"+`  "notes": true,`+"\n"+`  "trusted": `) {
		t.Errorf("Format:\n%s", out)
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
	c.Notes = false
	if strings.Contains(string(Format(c)), "notes") {
		t.Errorf("notes written while false:\n%s", Format(c))
	}
}

// The default prompts are section 4's, verbatim.
func TestDefaultPromptsAreSectionFours(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Roles {
		m := regexp.MustCompile("(?s)\n### " + r + "\n\n```markdown\n(.*?)```").FindSubmatch(doc)
		if m == nil || string(m[1]) != DefaultPrompts[r] {
			t.Errorf("%s's prompt differs from section 4", r)
		}
	}
}

func TestParseOrigin(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want Origin
		ok   bool
	}{
		{"git@forge.example.net:acme/app.git", Origin{Scheme: "ssh", Host: "forge.example.net", Owner: "acme", Repo: "app"}, true},
		{"forge.example.net:acme/app", Origin{Scheme: "ssh", Host: "forge.example.net", Owner: "acme", Repo: "app"}, true},
		{"ssh://git@forge.example.net:2222/acme/app.git", Origin{Scheme: "ssh", Host: "forge.example.net", Port: "2222", Owner: "acme", Repo: "app"}, true},
		{"ssh://git:pw@forge.example.net/acme/app.git", Origin{Scheme: "ssh", Host: "forge.example.net", Owner: "acme", Repo: "app", Credentials: true}, true},
		{"https://forge.example.net:3000/acme/app.git/", Origin{Scheme: "https", Host: "forge.example.net", Port: "3000", Owner: "acme", Repo: "app"}, true},
		{"https://tok@forge.example.net/acme/app", Origin{Scheme: "https", Host: "forge.example.net", Owner: "acme", Repo: "app", Credentials: true}, true},
		{"https://jane:tok@forge.example.net/app", Origin{Credentials: true}, false},
		{"https://forge.example.net/a/b/c", Origin{}, false},
		{"/srv/git/app.git", Origin{}, false},
	} {
		got, ok := parseOrigin(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseOrigin(%q) = %+v, %v", tc.raw, got, ok)
		}
	}
	for _, tc := range []struct {
		raw  string
		want Forge
	}{
		{"git@github.com:acme/app.git", Forge{Kind: "github", API: "https://api.github.com", Repo: "acme/app", TokenEnv: "GITHUB_TOKEN"}},
		{"ssh://git@forge.example.net:2222/acme/app.git", Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/app", TokenEnv: "GITEA_TOKEN"}},
		{"https://forge.example.net:3000/acme/app.git", Forge{Kind: "gitea", API: "https://forge.example.net:3000/api/v1", Repo: "acme/app", TokenEnv: "GITEA_TOKEN"}},
	} {
		o, _ := parseOrigin(tc.raw)
		if got := detected(o); got != tc.want {
			t.Errorf("detected(%q) = %+v", tc.raw, got)
		}
	}
}

func TestForgeHostAndState(t *testing.T) {
	for _, tc := range []struct {
		f          Forge
		host, path string
	}{
		{Forge{Kind: "gitea", API: "http://forge.example.net:3000/api/v1", Repo: "acme/app"}, "forge.example.net:3000", "/l/swarm/forge.example.net:3000/acme/app"},
		{Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/app"}, "forge.example.net", "/l/swarm/forge.example.net/acme/app"},
		{Forge{Kind: "github", API: "https://api.github.com", Repo: "acme/app"}, "github.com", "/l/swarm/github.com/acme/app"},
	} {
		if h, p := tc.f.ForgeHost(), tc.f.State("/l"); h != tc.host || p != tc.path {
			t.Errorf("%+v: %q %q", tc.f, h, p)
		}
	}
}
