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

// valid is a declaration the clone of cloneFolder validates.
func valid() Config {
	envVar, slots, ssh := "CLAUDE_CODE_OAUTH_TOKEN", 1, "ssh -i /keys/app"
	personas := map[string]Persona{}
	for r, p := range examplePersonas {
		p.CoAuthor = defaultCoAuthor(p.Provider, p.Model)
		personas[r] = p
	}
	return Config{
		Forge: Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/swarm-app", TokenEnv: "GITEA_TOKEN", CommitEmail: "swarm@example.net"},
		Repos: []Repo{
			{Name: "app", Repo: "acme/app", SSHCommand: &ssh, Check: []string{"make", "check"}, Image: defaultImage, Protected: defaultProtected},
			{Name: "lib", Repo: "acme/lib", Check: []string{}, Image: defaultImage, Protected: []string{}},
		},
		Base: "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: Paths("docs/"),
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
	_, clone := cloneFolder(t)
	p := Clone(clone)

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
		{`swarm.json: repos[1].name: "app" is listed twice`, func(c *Config) { c.Repos[1].Name, c.Repos[1].Repo = "app", "acme/app" }},
		{`swarm.json: repos[0].check[1]: empty argument`, func(c *Config) { c.Repos[0].Check = []string{"make", ""} }},
		{`swarm.json: repos[0].deps.run: no argument (at least one is required)`, func(c *Config) { c.Repos[0].Deps = &Deps{Run: []string{}} }},
		{`swarm.json: repos[0].deps.run[1]: empty argument`, func(c *Config) { c.Repos[0].Deps = &Deps{Run: []string{"go", ""}} }},
		{`swarm.json: repos[0].deps.env.GO-FLAGS: "GO-FLAGS" is not a variable name`, func(c *Config) {
			c.Repos[0].Deps = &Deps{Run: []string{"go", "mod", "download"}, Env: map[string]string{"GOMODCACHE": "/deps/mod", "GO-FLAGS": "-modcacherw"}}
		}},
		{`swarm.json: repos[1].deps: no check to run it before; declare check, or remove deps`, func(c *Config) { c.Repos[1].Deps = &Deps{Run: []string{"go", "mod", "download"}} }},
		{`swarm.json: repos[0].services: "" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr("") }},
		{`swarm.json: repos[0].services: "/compose.yaml" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr("/compose.yaml") }},
		{`swarm.json: repos[0].services: "../compose.yaml" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr("../compose.yaml") }},
		{`swarm.json: repos[0].services: "test/./compose.yaml" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr("test/./compose.yaml") }},
		{`swarm.json: repos[0].services: "test/" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr("test/") }},
		{`swarm.json: repos[0].services: "." is not a file's path in the repository`, func(c *Config) { c.Repos[0].Services = ptr(".") }},
		{`swarm.json: repos[0].env.GO-FLAGS: "GO-FLAGS" is not a variable name`, func(c *Config) { c.Repos[0].Env = map[string]string{"CGO_ENABLED": "0", "GO-FLAGS": "-mod=mod"} }},
		{`swarm.json: repos[0].env.GOFLAGS: also in deps.env; declare it once`, func(c *Config) {
			c.Repos[0].Deps = &Deps{Run: []string{"go", "mod", "download"}, Env: map[string]string{"GOFLAGS": "-modcacherw"}}
			c.Repos[0].Env = map[string]string{"GOFLAGS": "-mod=mod"}
		}},
		{`swarm.json: repos[0].env.GITEA_TOKEN: a secret's variable is never given this way`, func(c *Config) { c.Repos[0].Env = map[string]string{"GITEA_TOKEN": "x"} }},
		{`swarm.json: repos[1].env.CLAUDE_CODE_OAUTH_TOKEN: a secret's variable is never given this way`, func(c *Config) { c.Repos[1].Env = map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": "x"} }},
		{`swarm.json: repos[0].env.DAISY_MAINTAINER_TOKEN: a secret's variable is never given this way`, func(c *Config) { c.Repos[0].Env = map[string]string{"DAISY_MAINTAINER_TOKEN": "x"} }},
		{`swarm.json: repos[0].env.TZ: a value holds a newline`, func(c *Config) { c.Repos[0].Env = map[string]string{"TZ": "UTC\nX=1"} }},
		{`swarm.json: repos[0].env.TZ: a value holds a newline`, func(c *Config) { c.Repos[0].Env = map[string]string{"TZ": "UTC\r"} }},
		{`swarm.json: repos[0].image: empty`, func(c *Config) { c.Repos[0].Image = "" }},
		{`swarm.json: repos[0].dockerfile: "" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("") }},
		{`swarm.json: repos[0].dockerfile: "/Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("/Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "../Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("../Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "images/../Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("images/../Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "images/./Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("images/./Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "images//Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("images//Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "./Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("./Dockerfile") }},
		{`swarm.json: repos[0].dockerfile: "images/" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("images/") }},
		{`swarm.json: repos[0].dockerfile: "." is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr(".") }},
		{`swarm.json: repos[0].dockerfile: "Dockerfile\n" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("Dockerfile\n") }},
		{`swarm.json: repos[0].dockerfile: "Dockerfile\r" is not a file's path in the repository`, func(c *Config) { c.Repos[0].Dockerfile = ptr("Dockerfile\r") }},
		{`swarm.json: repos[1].dockerfile: "../Dockerfile" is not a file's path in the repository`, func(c *Config) { c.Repos[1].Dockerfile = ptr("../Dockerfile") }},
		{`swarm.json: repos[0].protected[2]: "../ci/" is not a relative pattern without ..`, func(c *Config) { c.Repos[0].Protected = []string{"a/", "b", "../ci/"} }},
		{`swarm.json: repos[0].protected[0]: "" is not a relative pattern without ..`, func(c *Config) { c.Repos[0].Protected = []string{""} }},
		{`swarm.json: repos[1].rulings[1]: "/adr/" is not a relative pattern without ..`, func(c *Config) { c.Repos[1].Rulings = &[]string{"docs/adr/", "/adr/"} }},
		{`swarm.json: repos[0].rulings[0]: "" is not a relative pattern without ..`, func(c *Config) { c.Repos[0].Rulings = &[]string{""} }},
		{`swarm.json: base: "main.lock" is not a branch name`, func(c *Config) { c.Base = "main.lock" }},
		{`swarm.json: base: "a..b" is not a branch name`, func(c *Config) { c.Base = "a..b" }},
		{`swarm.json: max_members: 0 is not between 1 and 16`, func(c *Config) { c.MaxMembers = 0 }},
		{`swarm.json: max_rounds: 11 is not between 1 and 10`, func(c *Config) { c.MaxRounds = 11 }},
		{`swarm.json: timeout: "30s" is not between 1m and 24h`, func(c *Config) { c.Timeout = "30s" }},
		{`swarm.json: interval: "5 minutes" is not a Go duration`, func(c *Config) { c.Interval = "5 minutes" }},
		{`swarm.json: owner.max_open_proposals: 21 is not between 1 and 20`, func(c *Config) { c.Owner.MaxOpenProposals = 21 }},
		{`swarm.json: trusted: no login (at least one is required)`, func(c *Config) { c.Trusted = nil }},
		{`swarm.json: trusted[1]: "jane doe" is not a forge login`, func(c *Config) { c.Trusted = []string{"swarm-bot", "jane doe"} }},
		{`swarm.json: notify[1]: empty argument`, func(c *Config) { c.Notify = []string{"ntfy", ""} }},
		{`swarm.json: budget: missing (required when a gate is false)`, func(c *Config) { c.Gates.Merge = false }},
		{`swarm.json: budget.per_day: 0 is not greater than 0`, func(c *Config) { c.Budget = &Budget{PerDay: 0, Total: 10} }},
		{`swarm.json: budget.total: -1.5 is not greater than 0`, func(c *Config) { c.Budget = &Budget{PerDay: 1, Total: -1.5} }},
		{`swarm.json: personas.tester: not a role (owner, architect, worker, adversary, sympathetic, qa)`, func(c *Config) { c.Personas["tester"] = c.Personas["worker"] }},
		{`swarm.json: personas.owner: missing`, func(c *Config) { delete(c.Personas, "owner") }},
		{`swarm.json: personas.adversary.provider: "gemini" has no adapter`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Provider = "gemini" }) }},
		{`swarm.json: personas.adversary.model: empty; every role names its model`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Model = "" }) }},
		{`swarm.json: personas.adversary.model: "-" is not a model; every role names its model`, func(c *Config) { setPersona(c, "adversary", func(p *Persona) { p.Model = "-" }) }},
		{`swarm.json: personas.worker.model: "a b" contains whitespace`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a b" }) }},
		{`swarm.json: personas.worker.model: "a\u2003b" contains whitespace`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a\u2003b" }) }},
		{`swarm.json: personas.worker.model: "a\x00b" contains a control or format character`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a\x00b" }) }},
		{`swarm.json: personas.worker.model: "a\x7fb" contains a control or format character`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a\x7fb" }) }},
		{`swarm.json: personas.worker.model: "a\u200bb" contains a control or format character`, func(c *Config) { setPersona(c, "worker", func(p *Persona) { p.Model = "a\u200bb" }) }},
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
		// section 3's order: keys before personas, whose problems wait
		{`swarm.json: keys.openai: needs exactly one of env, slots`, func(c *Config) {
			c.Keys["openai"] = Key{}
			setPersona(c, "worker", func(p *Persona) { p.Effort = "Very High" })
		}},
	} {
		c := valid()
		c.Keys = map[string]Key{"anthropic": c.Keys["anthropic"], "openai": c.Keys["openai"]}
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}

	if err := Validate(ops.Real(), p, valid(), 0); err != nil {
		t.Errorf("the valid declaration: %v", err)
	}
	for _, k := range []Key{{Slots: ptr(2)}, {Env: ptr("XAI_API_KEY")}} { // xai takes either form (section 8)
		x := valid()
		setPersona(&x, "sympathetic", func(p *Persona) { p.Provider, p.Model, p.CoAuthor = "xai", "grok-4.5", "grok-4.5 <noreply@x.ai>" })
		x.Keys["xai"] = k
		if err := Validate(ops.Real(), p, x, 0); err != nil {
			t.Errorf("xai %+v: %v", k, err)
		}
	}
	none := valid()
	setPersona(&none, "worker", func(p *Persona) { p.CoAuthor = "" })
	if err := Validate(ops.Real(), p, none, 0); err != nil {
		t.Errorf(`co_author "", no trailer: %v`, err)
	}
	os.Remove(PromptFile(clone, "architect"))
	write(t, PromptFile(clone, "worker"), "")
	if got := errText(Validate(ops.Real(), p, valid(), 0)); got != "swarm.json: personas.architect: prompt prompts/architect.md is missing or empty" {
		t.Errorf("missing prompt: %s", got)
	}
	c := valid()
	c.Trusted, c.Forge.CommitEmail = nil, ""
	if err := Validate(ops.Real(), p, c, SkipPrompts|SkipAccount); err != nil {
		t.Errorf("init's first validation: %v", err)
	}
}

// The QA is declared by its trigger and its persona together
// (section 3, qa): each row of its errors byte for byte, its
// prompt required once declared, and a valid declaration of each form.
func TestValidateTheQA(t *testing.T) {
	_, clone := cloneFolder(t)
	p := Clone(clone)
	declareQA := func(c *Config, e QA) {
		c.QA = &e
		c.Personas[QARole] = c.Personas["owner"]
	}
	if got := errText(Validate(ops.Real(), p, func() Config { c := valid(); declareQA(&c, QA{Merges: ptr(10)}); return c }(), 0)); got != "swarm.json: personas.qa: prompt prompts/qa.md is missing or empty" {
		t.Errorf("no QA prompt: %s", got)
	}
	write(t, PromptFile(clone, QARole), DefaultPrompts[QARole])
	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: qa: no trigger (merges or tag is required)`, func(c *Config) { declareQA(c, QA{}) }},
		{`swarm.json: qa.merges: 0 is not greater than 0`, func(c *Config) { declareQA(c, QA{Merges: ptr(0)}) }},
		{`swarm.json: qa.merges: -3 is not greater than 0`, func(c *Config) { declareQA(c, QA{Merges: ptr(-3), Tag: ptr("v*")}) }},
		{`swarm.json: qa.tag: "" is not a tag pattern`, func(c *Config) { declareQA(c, QA{Tag: ptr("")}) }},
		{`swarm.json: qa.tag: "--list" is not a tag pattern`, func(c *Config) { declareQA(c, QA{Tag: ptr("--list")}) }},
		{`swarm.json: qa.tag: "v *" is not a tag pattern`, func(c *Config) { declareQA(c, QA{Tag: ptr("v *")}) }},
		{`swarm.json: qa.tag: "v\n" is not a tag pattern`, func(c *Config) { declareQA(c, QA{Tag: ptr("v\n")}) }},
		{`swarm.json: personas.qa: missing (required when qa is declared)`, func(c *Config) { c.QA = &QA{Merges: ptr(10)} }},
		{`swarm.json: personas.qa: no trigger runs it; declare qa, or remove personas.qa`, func(c *Config) { c.Personas[QARole] = c.Personas["owner"] }},
		{`swarm.json: personas.qa.effort: "Very High" is not one lowercase word`, func(c *Config) {
			declareQA(c, QA{Merges: ptr(10)})
			setPersona(c, QARole, func(p *Persona) { p.Effort = "Very High" })
		}},
		{`swarm.json: keys: no credential for provider "xai"`, func(c *Config) {
			declareQA(c, QA{Merges: ptr(10)})
			setPersona(c, QARole, func(p *Persona) { p.Provider, p.Model, p.CoAuthor = "xai", "grok-4.5", "" })
		}},
	} {
		c := valid()
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}
	for _, e := range []QA{{Merges: ptr(1)}, {Tag: ptr("v*")}, {Merges: ptr(10), Tag: ptr("release-*")}} {
		c := valid()
		declareQA(&c, e)
		if err := Validate(ops.Real(), p, c, 0); err != nil {
			t.Errorf("qa %s: %v", inline(e), err)
		}
		if got := c.DeclaredRoles(); !reflect.DeepEqual(got, AllRoles) {
			t.Errorf("declared roles: %v", got)
		}
	}
	if got := valid().DeclaredRoles(); !reflect.DeepEqual(got, Roles) {
		t.Errorf("declared roles without the QA: %v", got)
	}
}

// The QA trigger is written after owner, one line, and its
// persona last among the personas; both are read back; without them the
// file is as before (section 3).
func TestFormatQA(t *testing.T) {
	c := valid()
	if out := string(Format(c)); strings.Contains(out, "qa") {
		t.Errorf("Format wrote a QA run undeclared:\n%s", out)
	}
	c.QA = &QA{Merges: ptr(10), Tag: ptr("v*")}
	c.Personas[QARole] = c.Personas["owner"]
	out := string(Format(c))
	for _, want := range []string{
		`  "owner": {"max_open_proposals": 5},` + "\n" + `  "qa": {"merges": 10, "tag": "v*"},` + "\n" + `  "resources": `,
		`    "qa":          {"provider": "anthropic", "model": "claude-opus-5-5",   "effort": "high",   "co_author": "claude-opus-5-5 <noreply@anthropic.com>"}` + "\n  }\n}\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format lacks %q:\n%s", want, out)
		}
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
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
		{`"check": ["make", "check"],`, `"check": ["make", "check"], "deps": {"env": {}},`, "swarm.json: repos[0].deps.run: missing"},
		{`"check": ["make", "check"],`, `"check": ["make", "check"], "deps": {"run": ["go"], "cache": "/c"},`, "swarm.json: repos[0].deps.cache: unknown field"},
		{`"check": ["make", "check"],`, `"check": ["make", "check"], "deps": ["go"],`, "swarm.json: repos[0].deps: not an object"},
		{`"check": ["make", "check"],`, `"check": ["make", "check"], "deps": {"run": ["go"], "env": {"A": 1}},`, "swarm.json: repos[0].deps.env.A: cannot unmarshal number into Go value of type string"},
		// A resource is its path, or {"path", "note"} (section 3).
		{`"resources": ["docs/"],`, `"resources": [3],`, "swarm.json: resources[0]: not a string or an object"},
		{`"resources": ["docs/"],`, `"resources": [null],`, "swarm.json: resources[0]: not a string or an object"},
		{`"resources": ["docs/"],`, `"resources": [{"note": "the docs"}],`, "swarm.json: resources[0].path: missing"},
		{`"resources": ["docs/"],`, `"resources": [{"path": "docs/", "notes": "the docs"}],`, "swarm.json: resources[0].notes: unknown field"},
		{`"resources": ["docs/"],`, `"resources": ["docs/", {"path": "x", "note": 1}],`, "swarm.json: resources[1].note: cannot unmarshal number into Go value of type string"},
		// The old layout's fields, retired (revision 6's plan, Step 7).
		{`"repo": "acme/lib",`, `"repo": "acme/lib", "path": "lib",`, "swarm.json: repos[1].path: unknown field"},
		{`"resources": ["docs/"],`, `"resources": ["docs/"], "wiki": true,`, "swarm.json: wiki: unknown field"},
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
	dir := t.TempDir()
	if _, found, err := Load(ops.Real().FS(), dir); found || errText(err) != "swarm.json: not found at "+dir+"/swarm.json; run 'daisy swarm init'" {
		t.Errorf("absent: found %v, %v", found, err)
	}
}

// Format writes section 3's example byte for byte.
func TestFormatIsSectionThreesExample(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	envVar, slots := "CLAUDE_CODE_OAUTH_TOKEN", 1
	personas := map[string]Persona{}
	for r, p := range examplePersonas {
		p.CoAuthor = defaultCoAuthor(p.Provider, p.Model)
		personas[r] = p
	}
	c := Config{
		Forge: Forge{Kind: "gitea", API: "https://<forge-host>/api/v1", Repo: "<owner>/<repo>", TokenEnv: "GITEA_TOKEN", CommitEmail: "<swarm-email>"},
		Repos: []Repo{{Name: "<name>", Repo: "<owner>/<name>", Image: defaultImage, Protected: defaultProtected}},
		Base:  "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: Paths("briefs/", "docs/"),
		Trusted:   []string{"<maintainer-login>", "<swarm-login>"},
		Gates:     Gates{Triage: true, Plan: true, Merge: true},
		Keys:      map[string]Key{"anthropic": {Env: &envVar}, "openai": {Slots: &slots}},
		Personas:  personas,
	}
	m := regexp.MustCompile("(?s)\n## 3[.] `swarm.json`.*?```json\n(.*?)```").FindSubmatch(doc)
	if m == nil {
		t.Fatal("no example in section 3")
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

// notify is written after trusted, one line, and read back; with none
// there is no line (section 3).
func TestFormatNotify(t *testing.T) {
	c := valid()
	if out := string(Format(c)); strings.Contains(out, `"notify"`) {
		t.Errorf("Format wrote notify unset:\n%s", out)
	}
	c.Notify = []string{"ntfy", "publish", "swarm-app"}
	out := string(Format(c))
	if want := `"],` + "\n" + `  "notify": ["ntfy", "publish", "swarm-app"],` + "\n" + `  "gates": `; !strings.Contains(out, want) {
		t.Errorf("Format lacks %q:\n%s", want, out)
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
}

// A repository's deps is written on its line after check, its env's
// names in order, its services after deps, its env after services,
// names in order, and its dockerfile after image, and read back and
// written again byte for byte; with none the line is as before (section
// 3).
func TestFormatDeps(t *testing.T) {
	c := valid()
	c.Repos[0].Deps = &Deps{Run: []string{"sh", "-c", "go mod download && go list -deps -test ./... >/dev/null"}, Env: map[string]string{"GOMODCACHE": "/deps/go/mod", "GOFLAGS": "-modcacherw"}}
	c.Repos[1].Check = []string{"make"}
	c.Repos[1].Deps = &Deps{Run: []string{"npm", "ci"}}
	c.Repos[1].Services = ptr("test/compose.yaml")
	c.Repos[1].Env = map[string]string{"TZ": "UTC", "CGO_ENABLED": "0"}
	c.Repos[1].Dockerfile = ptr("images/ci/Dockerfile")
	out := string(Format(c))
	for _, want := range []string{
		`"check": ["make", "check"], "deps": {"run": ["sh", "-c", "go mod download && go list -deps -test ./... >/dev/null"], "env": {"GOFLAGS": "-modcacherw", "GOMODCACHE": "/deps/go/mod"}}, "image": `,
		`"check": ["make"], "deps": {"run": ["npm", "ci"]}, "services": "test/compose.yaml", "env": {"CGO_ENABLED": "0", "TZ": "UTC"}, "image": "daisy-member:1", "dockerfile": "images/ci/Dockerfile", "protected": []}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format lacks %q:\n%s", want, out)
		}
	}
	back, err := Parse([]byte(out))
	if err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
	if again := string(Format(back)); again != out {
		t.Errorf("not byte for byte:\n%s\nthen\n%s", out, again)
	}
	_, clone := cloneFolder(t)
	if err := Validate(ops.Real(), Clone(clone), c, 0); err != nil {
		t.Errorf("valid deps: %v", err)
	}
	if f := string(Format(valid())); strings.Contains(f, "deps") || strings.Contains(f, `"env": {`) || strings.Contains(f, "dockerfile") {
		t.Errorf("deps, env or dockerfile written with none:\n%s", f)
	}
}

// A repository's rulings is written on its line after protected, an
// explicit [] kept as written, and read back; absent, the line is as
// before (section 3).
func TestFormatRulings(t *testing.T) {
	c := valid()
	c.Repos[0].Rulings = &[]string{"docs/adr/", "RULES.md"}
	c.Repos[1].Rulings = &[]string{}
	out := string(Format(c))
	for _, want := range []string{
		`"protected": [".github/", ".gitea/", ".forgejo/", ".claude/", ".grok/", "AGENTS.md", "CLAUDE.md"], "rulings": ["docs/adr/", "RULES.md"]}`,
		`"protected": [], "rulings": []}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format lacks %q:\n%s", want, out)
		}
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v", err)
	}
	if strings.Contains(string(Format(valid())), "rulings") {
		t.Errorf("rulings written with none:\n%s", Format(valid()))
	}
}

// A repository's rulings are its rulings field when present, [] included;
// absent, docs/adr/ when protected names it, else none (section 3).
func TestRulingPaths(t *testing.T) {
	for _, c := range []struct {
		protected []string
		rulings   *[]string
		want      []string
	}{
		{[]string{".github/", "docs/adr/"}, nil, []string{"docs/adr/"}},
		{[]string{".github/", "docs/adr"}, nil, nil},
		{[]string{".github/"}, nil, nil},
		{[]string{"docs/adr/"}, &[]string{}, []string{}},
		{[]string{".github/"}, &[]string{"RULES.md"}, []string{"RULES.md"}},
	} {
		if got := (Repo{Protected: c.protected, Rulings: c.rulings}).RulingPaths(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("protected %q, rulings %v: %q, want %q", c.protected, c.rulings, got, c.want)
		}
	}
}

// `notes` is written after resources (and references) only when true,
// absent read as false, and read back (section 3).
func TestFormatNotes(t *testing.T) {
	c := valid()
	c.Notes = true
	out := string(Format(c))
	if !strings.Contains(out, `  "resources": ["docs/"],`+"\n"+`  "notes": true,`+"\n"+`  "trusted": `) {
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

// A resource with a note is written {"path", "note"}, one without its
// path alone, in resources and in a reference's paths, and each is read
// back; {"path"} without a note reads as the path alone (section 3).
func TestFormatResourceNotes(t *testing.T) {
	c := valid()
	c.Resources = []Resource{{Path: "docs/"}, {Path: "lib:guide/", Note: "the guide: rules & <limits>"}}
	c.References = []Reference{{Name: "spec", URL: "https://forge.example.org/std/spec.git", Pin: "v1", Paths: []Resource{{Path: "SPEC.md", Note: "the standard"}, {Path: "lib/"}}}}
	out := string(Format(c))
	for _, want := range []string{
		`  "resources": ["docs/", {"path": "lib:guide/", "note": "the guide: rules & <limits>"}],` + "\n",
		`"pin": "v1", "paths": [{"path": "SPEC.md", "note": "the standard"}, "lib/"]}` + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format lacks %q:\n%s", want, out)
		}
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v\n%+v", err, back)
	}
	_, clone := cloneFolder(t)
	if err := Validate(ops.Real(), Clone(clone), c, 0); err != nil {
		t.Errorf("valid notes: %v", err)
	}
	object := strings.Replace(out, `["docs/", `, `[{"path": "docs/"}, `, 1)
	if back, err := Parse([]byte(object)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("{\"path\"} alone: %v\n%+v", err, back)
	}
}

// The default prompts are section 4's, verbatim.
func TestDefaultPromptsAreSectionFours(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range AllRoles {
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
	// A kind other than the detected one (question 2's answer) keeps an
	// http(s) origin's scheme and port, as detection does; an IPv6 host
	// is bracketed.
	for _, tc := range []struct {
		raw, kind, api string
	}{
		{"http://github.com:8080/acme/app.git", "gitea", "http://github.com:8080/api/v1"},
		{"git@github.com:acme/app.git", "gitea", "https://github.com/api/v1"},
		{"https://forge.example.net:3000/acme/app.git", "github", "https://api.github.com"},
		{"http://[fd00::1]:3000/acme/app.git", "gitea", "http://[fd00::1]:3000/api/v1"},
		{"ssh://git@[fd00::1]:2222/acme/app.git", "gitea", "https://[fd00::1]/api/v1"},
	} {
		o, _ := parseOrigin(tc.raw)
		if got := forgeAs(o, tc.kind); got.API != tc.api || got.Kind != tc.kind || got.Repo != "acme/app" {
			t.Errorf("forgeAs(%q, %s) = %+v, want API %s", tc.raw, tc.kind, got, tc.api)
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
