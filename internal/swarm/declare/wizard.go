package declare

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/gitx"
	"daisy/internal/ops"
)

// ErrCancelled is the wizard's end at EOF: nothing is written.
var ErrCancelled = errors.New("Cancelled — no answer on stdin (EOF); nothing written.")

// defaultImage and defaultProtected are a new repository entry's image and
// protected paths (section 3): the forges' workflows, and the instructions,
// settings and hooks a member's CLI loads from the repository (.claude/,
// .grok/, AGENTS.md, CLAUDE.md).
const defaultImage = "daisy-member:1"

var defaultProtected = []string{".github/", ".gitea/", ".forgejo/", ".claude/", ".grok/", "AGENTS.md", "CLAUDE.md"}

// examplePersonas are section 3's example: the defaults without a file.
var examplePersonas = map[string]Persona{
	"owner":       {Provider: "anthropic", Model: "claude-opus-5-5", Effort: "high"},
	"architect":   {Provider: "anthropic", Model: "claude-fable-5-1", Effort: "high"},
	"worker":      {Provider: "anthropic", Model: "claude-sonnet-5-5", Effort: "medium"},
	"adversary":   {Provider: "openai", Model: "gpt-6.1-sol", Effort: "high"},
	"sympathetic": {Provider: "anthropic", Model: "claude-opus-5-5", Effort: "high"},
}

// providerDefaults are each provider's model, a role's default when the
// wizard moves it to that provider, and its credential, added to keys for
// a provider a persona names and no key does (section 11).
var providerDefaults = map[string]struct {
	model, env string
	slots      int
}{
	"anthropic": {model: "claude-opus-5-5", env: "CLAUDE_CODE_OAUTH_TOKEN"},
	"openai":    {model: "gpt-6.1-sol", slots: 1},
	"xai":       {model: "grok-4.7", env: "XAI_API_KEY"},
}

// defaultKey is provider's credential in providerDefaults.
func defaultKey(provider string) Key {
	d := providerDefaults[provider]
	if d.env != "" {
		return Key{Env: &d.env}
	}
	return Key{Slots: &d.slots}
}

// Wizard asks `swarm init`'s questions (docs/swarm.md section 11) through
// Ops.Prompt, the questions and each refusal on Err. A default comes from
// File, the existing swarm.json, else from detection; an empty answer
// takes it. With Defaults it asks nothing and takes every default, a
// refused one being the error.
type Wizard struct {
	Ops      ops.Ops
	In       io.Reader
	Err      io.Writer
	Place    Place // where the swarm is; detection reads the clones under Place.Root (Outside), or beside it in a swarm repository's clone
	Defaults bool
	File     *Config
	API      string // `init --api <url>`: question 3's default over the file's and every other
}

// ask puts one question until take accepts the answer (or the default).
func (w *Wizard) ask(question, def string, take func(answer string) *Error) error {
	if w.Defaults {
		if e := take(def); e != nil {
			return e
		}
		return nil
	}
	q := question + ": "
	if def != "" {
		q = question + " [" + def + "]: "
	}
	for {
		a, err := w.Ops.Prompt(w.In, w.Err, q)
		if errors.Is(err, io.EOF) {
			return ErrCancelled
		}
		if err != nil {
			return fmt.Errorf("Error: cannot read the answer: %v", err)
		}
		if a = strings.TrimSpace(a); a == "" {
			a = def
		}
		e := take(a)
		if e == nil {
			return nil
		}
		fmt.Fprintln(w.Err, e.Error())
	}
}

// Ask is questions 1–32: everything but trusted and forge.commit_email,
// which AskAccount fills once the token's account is known. The fields
// not asked keep the file's values, else section 3's defaults.
func (w *Wizard) Ask() (Config, error) {
	c := w.unasked()
	var origins []Origin
	steps := []func() error{
		func() error {
			return w.ask("Product repositories (comma-separated <owner>/<name>, the product first)", w.defaultNames(), func(a string) *Error {
				var e *Error
				c.Repos, origins, e = w.named(a)
				return e
			})
		},
		func() error {
			return w.ask("Forge kind (gitea, github)", w.forgeDefault(origins[0], "").Kind, func(a string) *Error {
				c.Forge.Kind = a
				if a != "gitea" && a != "github" {
					return fieldErr("forge.kind", "%q has no adapter", a)
				}
				return nil
			})
		},
		func() error {
			return w.ask("Forge API", w.apiDefault(origins[0], c.Forge.Kind), func(a string) *Error {
				c.Forge.API = a
				if !apiOK(a) {
					return fieldErr("forge.api", "%q is not an absolute http(s) URL without credentials", a)
				}
				return nil
			})
		},
		func() error {
			return w.ask("Swarm repository", w.swarmDefault(c, origins[0]), func(a string) *Error {
				c.Forge.Repo = a
				if !repoShape.MatchString(a) {
					return fieldErr("forge.repo", "%q is not <owner>/<repo>", a)
				}
				if _, ok := c.Declares(a); ok {
					return fieldErr("forge.repo", "%s is a declared repository, which holds product code; the swarm repository holds none", a)
				}
				return nil
			})
		},
		func() error {
			return w.ask("Token variable", w.forgeDefault(origins[0], c.Forge.Kind).TokenEnv, func(a string) *Error {
				c.Forge.TokenEnv = a
				return checkTokenEnv(a)
			})
		},
		func() error {
			return w.ask("Base branch", c.Base, func(a string) *Error { c.Base = a; return checkBase(a) })
		},
		func() error {
			if err := w.personas(&c); err != nil {
				return err
			}
			keys := map[string]Key{}
			maps.Copy(keys, c.Keys) // the file's map stays the file's
			for _, r := range Roles {
				p := c.Personas[r].Provider
				if _, ok := keys[p]; !ok {
					keys[p] = defaultKey(p)
				}
			}
			c.Keys = keys
			return nil
		},
		func() error {
			for i := range c.Repos {
				r := &c.Repos[i]
				err := w.ask(r.Name+" image", r.Image, func(a string) *Error {
					if r.Image = a; a == "" {
						return fieldErr(fmt.Sprintf("repos[%d].image", i), "empty")
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
		func() error { return w.number("Max members", "max_members", &c.MaxMembers, 1, 16) },
		func() error { return w.number("Max rounds", "max_rounds", &c.MaxRounds, 1, 10) },
		func() error {
			return w.ask("Timeout", c.Timeout, func(a string) *Error { c.Timeout = a; return checkDuration("timeout", a) })
		},
		func() error {
			return w.ask("Interval", c.Interval, func(a string) *Error { c.Interval = a; return checkDuration("interval", a) })
		},
		func() error {
			return w.ask("Resources (comma-separated: paths in the swarm repository, or <name>:<path> in a declared one; - for none)", listDefault(c.Resources), func(a string) *Error {
				c.Resources = list(a)
				return checkResources(c)
			})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return Config{}, err
		}
	}
	return c, nil
}

// AskAccount is questions 33 and 34, asked once the token's account is
// known: the maintainer's logins, which may not be the swarm's (trusted is
// written as them, then the swarm's login), and the commit address,
// defaulting to the account's email.
func (w *Wizard) AskAccount(c *Config, login, email string) error {
	var def []string
	if w.File != nil {
		for _, l := range w.File.Trusted {
			if !strings.EqualFold(l, login) {
				def = append(def, l)
			}
		}
	}
	err := w.ask("Maintainer logins (comma-separated, not the swarm's; - for none)", listDefault(def), func(a string) *Error {
		logins := list(a)
		for i, l := range logins {
			if strings.EqualFold(l, login) {
				return fieldErr("trusted", "%q is the token's own login; the swarm needs its own forge account (section 8)", l)
			}
			if !loginShape.MatchString(l) {
				return fieldErr(fmt.Sprintf("trusted[%d]", i), "%q is not a forge login", l)
			}
		}
		c.Trusted = append(logins, login)
		return nil
	})
	if err != nil {
		return err
	}
	if w.File != nil && w.File.Forge.CommitEmail != "" {
		email = w.File.Forge.CommitEmail
	}
	return w.ask("Commit email", email, func(a string) *Error { c.Forge.CommitEmail = a; return checkCommitEmail(a) })
}

// ownName: a repository named swarm-<name>, section 3's convention for the
// swarm repository.
func ownName(repo string) bool { return strings.HasPrefix(strings.ToLower(repo), "swarm-") }

// AskNotes is question 35: whether the swarm keeps its research notes in
// the swarm repository's notes/ (section 3, notes); the file's answer is
// the default, else no.
func (w *Wizard) AskNotes(c *Config) error {
	def := "no"
	if c.Notes {
		def = "yes"
	}
	return w.ask("Research notes, in "+c.Forge.Repo+"'s notes/ (yes, no)", def, func(a string) *Error {
		switch a {
		case "yes", "no":
			c.Notes = a == "yes"
			return nil
		}
		return fieldErr("notes", "%q is not yes or no", a)
	})
}

// unasked is the file's values, else section 3's defaults: the fields no
// question asks, and the defaults of those that are.
func (w *Wizard) unasked() Config {
	if w.File != nil {
		c := *w.File
		c.Personas = map[string]Persona{}
		for r, p := range w.File.Personas {
			c.Personas[r] = p
		}
		return c
	}
	return Config{
		Base: "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: []string{"briefs/", "docs/"}, // the directories init seeds (section 3)
		Gates:     Gates{Triage: true, Plan: true, Merge: true},
		Keys:      map[string]Key{"anthropic": defaultKey("anthropic"), "openai": defaultKey("openai")},
		Personas:  map[string]Persona{},
	}
}

// detectedClone is one clone directly under the folder detection reads: the
// forge repository its origin names, and its core.sshCommand ("" for
// none).
type detectedClone struct {
	origin Origin
	ssh    string
}

// clones is what detection reads (section 11, the wizard): every clone directly under the folder init runs in (Outside),
// or, in a swarm repository's clone, under the folder holding it, whose
// origin is on a forge, in name order, but the swarm repository's own
// clone.
func (w *Wizard) clones() []detectedClone {
	dir := filepath.Dir(w.Place.Root)
	if w.Place.Outside {
		dir = w.Place.Root
	}
	entries, _ := w.Ops.FS().ReadDir(dir)
	var out []detectedClone
	for _, e := range entries {
		if !e.IsDir() || filepath.Join(dir, e.Name()) == w.Place.Root {
			continue
		}
		clone := filepath.Join(dir, e.Name())
		if !gitx.IsRepoRoot(w.Ops, clone) {
			continue
		}
		o, ok := parseOrigin(readOrigin(w.Ops, clone))
		if !ok || o.Credentials {
			continue
		}
		ssh, _, _ := w.Ops.Exec(clone, "git", "config", "--get", "core.sshCommand")
		out = append(out, detectedClone{o, strings.TrimSpace(string(ssh))})
	}
	return out
}

// cloneOrigin is the swarm repository's clone's origin, ok false when it
// has none that reads as one (section 10, Detection), or no clone is made
// yet (Outside).
func (w *Wizard) cloneOrigin() (Origin, bool) {
	if w.Place.Outside {
		return Origin{}, false
	}
	o, ok := parseOrigin(readOrigin(w.Ops, w.Place.Root))
	return o, ok && !o.Credentials
}

// defaultNames is question 1's default: the file's repositories; else
// each clone detection finds but one whose repository begins swarm-
// (section 3's convention for the swarm repository), as its origin's
// <owner>/<repo>.
func (w *Wizard) defaultNames() string {
	var names []string
	if w.File != nil {
		for _, r := range w.File.Repos {
			names = append(names, r.Repo)
		}
		return strings.Join(names, ", ")
	}
	for _, d := range w.clones() {
		if !ownName(d.origin.Repo) {
			names = append(names, d.origin.Owner+"/"+d.origin.Repo)
		}
	}
	return strings.Join(names, ", ")
}

// named is question 1's answer as entries:
// each `<owner>/<name>`, named by its <name>, its ssh_command the file's
// for the same repository, else the core.sshCommand of the clone
// detection finds of it; the file's check, image and protected kept. The
// origins are what detection reads for the forge's questions: the swarm
// repository's clone's origin, else the first repository's clone's.
func (w *Wizard) named(answer string) ([]Repo, []Origin, *Error) {
	names := list(answer)
	if len(names) == 0 {
		return nil, nil, fieldErr("repos", "no repository (at least one is required)")
	}
	found := w.clones()
	var repos []Repo
	var first Origin
	for i, a := range names {
		at := fmt.Sprintf("repos[%d]", i)
		if !repoShape.MatchString(a) {
			return nil, nil, fieldErr(at+".repo", "%q is not <owner>/<name>", a)
		}
		_, name, _ := strings.Cut(a, "/")
		r := Repo{Name: name, Repo: a, Check: []string{}, Image: defaultImage, Protected: append([]string(nil), defaultProtected...)}
		for _, d := range found {
			if strings.EqualFold(d.origin.Owner+"/"+d.origin.Repo, a) {
				if i == 0 {
					first = d.origin
				}
				if d.ssh != "" {
					r.SSHCommand = &d.ssh
				}
			}
		}
		if w.File != nil {
			for _, f := range w.File.Repos {
				if strings.EqualFold(f.Repo, a) {
					r.Check, r.Image, r.Protected, r.SSHCommand = f.Check, f.Image, f.Protected, f.SSHCommand
				}
			}
		}
		if !nameShape.MatchString(r.Name) {
			return nil, nil, fieldErr(at+".name", "%q is not a repository name", r.Name)
		}
		if slices.ContainsFunc(repos, func(p Repo) bool { return p.Name == r.Name }) {
			return nil, nil, fieldErr(at+".name", "%q is listed twice", r.Name)
		}
		repos = append(repos, r)
	}
	if o, ok := w.cloneOrigin(); ok {
		first = o
	}
	return repos, []Origin{first}, nil
}

// swarmDefault is question 4's default: the file's forge.repo; else the clone's origin; else the first clone
// detection finds whose repository begins swarm- on the forge's host;
// else <owner>/swarm-<name> after the first product repository (section
// 14, item 103).
func (w *Wizard) swarmDefault(c Config, first Origin) string {
	if w.File != nil {
		return w.File.Forge.Repo
	}
	if o, ok := w.cloneOrigin(); ok {
		return o.Owner + "/" + o.Repo
	}
	for _, d := range w.clones() {
		if ownName(d.origin.Repo) && strings.EqualFold(d.origin.Host, c.Forge.Host()) {
			return d.origin.Owner + "/" + d.origin.Repo
		}
	}
	owner, name, _ := strings.Cut(c.Repos[0].Repo, "/")
	return owner + "/swarm-" + name
}

// forgeDefault is the forge questions' defaults for kind, question 2's
// answer ("" before it): the file's forge while kind is the file's; else
// what the first repository's origin names, for kind when one is given.
func (w *Wizard) forgeDefault(first Origin, kind string) Forge {
	if w.File != nil && (kind == "" || kind == w.File.Forge.Kind) {
		return w.File.Forge
	}
	if kind == "" {
		return detected(first)
	}
	return forgeAs(first, kind)
}

// apiEnv is the variable naming a self-hosted forge's API, by kind
// (section 10, Detection).
var apiEnv = map[string]string{"gitea": "GITEA_API_URL", "github": "GITHUB_API_URL"}

// apiDefault is question 3's default: `--api`'s value, else the file's
// forge.api while kind is the file's, else $GITEA_API_URL or
// $GITHUB_API_URL by kind when set and not empty, else what the first
// repository's origin names.
func (w *Wizard) apiDefault(first Origin, kind string) string {
	if w.API != "" {
		return w.API
	}
	if w.File != nil && kind == w.File.Forge.Kind {
		return w.File.Forge.API
	}
	if v := w.Ops.Getenv(apiEnv[kind]); v != "" {
		return v
	}
	return w.forgeDefault(first, kind).API
}

// personas is questions 7–26: per role, provider, model, effort and the
// co-author. The model's default is the provider's (providerDefaults)
// when the answer moves the role to another provider; the co-author's
// follows the model unless the file's persona has the same provider and
// model; `-` is none, an empty answer being the default, as the list
// questions take it.
func (w *Wizard) personas(c *Config) error {
	for _, role := range Roles {
		p, ok := c.Personas[role]
		if !ok {
			p = examplePersonas[role]
		}
		was := p
		at := "personas." + role
		err := w.ask(role+" provider ("+strings.Join(slices.Sorted(maps.Keys(providers)), ", ")+")", p.Provider, func(a string) *Error { p.Provider = a; return checkProvider(at+".provider", a) })
		if err == nil && p.Provider != was.Provider {
			p.Model = providerDefaults[p.Provider].model
		}
		if err == nil {
			err = w.ask(role+" model", p.Model, func(a string) *Error { p.Model = a; return checkModel(at+".model", a) })
		}
		if err == nil {
			err = w.ask(role+" effort", p.Effort, func(a string) *Error {
				if p.Effort = a; !effortShape.MatchString(a) {
					return fieldErr(at+".effort", "%q is not one lowercase word", a)
				}
				return nil
			})
		}
		if err == nil {
			def := defaultCoAuthor(p.Provider, p.Model)
			if ok && was.Provider == p.Provider && was.Model == p.Model {
				def = cmp.Or(was.CoAuthor, "-")
			}
			err = w.ask(role+" co-author (- for none)", def, func(a string) *Error {
				if a == "-" {
					a = ""
				}
				p.CoAuthor = a
				return checkCoAuthor(at+".co_author", a)
			})
		}
		if err != nil {
			return err
		}
		c.Personas[role] = p
	}
	return nil
}

func (w *Wizard) number(question, field string, n *int, lo, hi int) error {
	return w.ask(question, strconv.Itoa(*n), func(a string) *Error {
		v, err := strconv.Atoi(a)
		if err != nil {
			return fieldErr(field, "%q is not an integer", a)
		}
		*n = v
		return checkRange(field, v, lo, hi)
	})
}

// list is a comma-separated answer, `-` being none.
func list(a string) []string {
	out := []string{}
	if a == "-" {
		return out
	}
	for _, s := range strings.Split(a, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func listDefault(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ", ")
}
