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

	"daisy/internal/ops"
)

// ErrCancelled is the wizard's end at EOF: nothing is written.
var ErrCancelled = errors.New("Cancelled — no answer on stdin (EOF); nothing written.")

// defaultImage and defaultProtected are a new repository entry's image and
// protected paths (section 3).
const defaultImage = "daisy-member:1"

var defaultProtected = []string{".github/", ".gitea/", ".forgejo/", "AGENTS.md", "CLAUDE.md"}

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
	"xai":       {model: "grok-4.5", env: "XAI_API_KEY"},
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
	WS       string
	Defaults bool
	File     *Config
	own      bool // question 4's follow-up: the issues repository is the swarm's own
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
			return w.ask("Product repositories (comma-separated paths in the workspace, the product first)", w.defaultPaths(), func(a string) *Error {
				var e *Error
				c.Repos, origins, e = w.repos(a)
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
			return w.ask("Forge API", w.forgeDefault(origins[0], c.Forge.Kind).API, func(a string) *Error {
				c.Forge.API = a
				if !apiOK(a) {
					return fieldErr("forge.api", "%q is not an absolute http(s) URL without credentials", a)
				}
				return nil
			})
		},
		func() error {
			return w.ask("Issues repository", w.forgeDefault(origins[0], c.Forge.Kind).Repo, func(a string) *Error {
				c.Forge.Repo = a
				if !repoShape.MatchString(a) {
					return fieldErr("forge.repo", "%q is not <owner>/<repo>", a)
				}
				return nil
			})
		},
		func() error { return w.askOwn(c) },
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
			return w.ask("Resources (comma-separated workspace paths, - for none)", listDefault(w.resources(c)), func(a string) *Error {
				c.Resources = list(a)
				return checkResources(w.Ops, w.WS, c.Resources)
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

// askOwn is question 4's follow-up, asked when the issues repository is
// none of the declared ones: whether it is the swarm's own, holding no
// product code; the default yes when the file keeps its notes there or a
// clone of it lies directly in the workspace, else no.
func (w *Wizard) askOwn(c Config) error {
	w.own = false
	if _, ok := c.Declares(c.Forge.Repo); ok {
		return nil
	}
	def := "no"
	if (w.File != nil && w.File.Notes) || w.issuesClone(c) != "" {
		def = "yes"
	}
	return w.ask("Issues repository "+c.Forge.Repo+": the swarm's own, holding no product code (yes, no)", def, func(a string) *Error {
		switch a {
		case "yes", "no":
			w.own = a == "yes"
			return nil
		}
		return fieldErr("forge.repo", "%q is not yes or no", a)
	})
}

// issuesClone is the directory directly under the workspace whose origin
// is the issues repository on the forge's host, "" for none.
func (w *Wizard) issuesClone(c Config) string {
	entries, _ := w.Ops.FS().ReadDir(w.WS)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		o, _, err := checkRepoPath(w.Ops, w.WS, "", e.Name())
		if err == nil && strings.EqualFold(o.Host, c.Forge.Host()) && strings.EqualFold(o.Owner+"/"+o.Repo, c.Forge.Repo) {
			return e.Name()
		}
	}
	return ""
}

// resources is question 32's default: the file's resources, else
// section 3's; and, for the swarm's own issues repository, its clone's
// briefs/ when the clone lies in the workspace, holds that directory and
// no resource names it yet.
func (w *Wizard) resources(c Config) []string {
	out := slices.Clone(c.Resources)
	clone := w.issuesClone(c)
	if !w.own || clone == "" {
		return out
	}
	briefs := clone + "/briefs/"
	if fi, err := w.Ops.FS().Stat(filepath.Join(w.WS, briefs)); err != nil || !fi.IsDir() {
		return out
	}
	if slices.ContainsFunc(out, func(r string) bool { return filepath.Clean(r) == filepath.Clean(briefs) }) {
		return out
	}
	return append(out, briefs)
}

// AskNotes is question 35: where the swarm keeps its research notes. For
// the swarm's own issues repository (question 4's follow-up), whether in
// its notes/ (section 3, notes); for any other, whether in the issues
// repository's wiki (section 3, wiki). The file's answer is the default,
// else no; the other field is cleared.
func (w *Wizard) AskNotes(c *Config) error {
	question, field, keep := "Research wiki, the issues repository's wiki (yes, no)", "wiki", &c.Wiki
	if w.own {
		question, field, keep = "Research notes, in "+c.Forge.Repo+"'s notes/ (yes, no)", "notes", &c.Notes
		c.Wiki = false
	} else {
		c.Notes = false
	}
	def := "no"
	if *keep {
		def = "yes"
	}
	return w.ask(question, def, func(a string) *Error {
		switch a {
		case "yes", "no":
			*keep = a == "yes"
			return nil
		}
		return fieldErr(field, "%q is not yes or no", a)
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
	resources := []string{}
	if fi, err := w.Ops.FS().Stat(filepath.Join(w.WS, "docs")); err == nil && fi.IsDir() {
		resources = []string{"docs/"}
	}
	return Config{
		Base: "main", MaxMembers: 3, MaxRounds: 5, Timeout: "45m", Interval: "5m",
		Owner:     Owner{MaxOpenProposals: 5},
		Resources: resources,
		Gates:     Gates{Triage: true, Plan: true, Merge: true},
		Keys:      map[string]Key{"anthropic": defaultKey("anthropic"), "openai": defaultKey("openai")},
		Personas:  map[string]Persona{},
	}
}

// defaultPaths is question 1's default: the file's repositories; else
// `.` when the workspace is a repository with an origin, else every
// directory directly under it that is, in name order, else `.`.
func (w *Wizard) defaultPaths() string {
	if w.File != nil {
		var paths []string
		for _, r := range w.File.Repos {
			paths = append(paths, r.Path)
		}
		return strings.Join(paths, ", ")
	}
	if isRepoWithOrigin(w.Ops, w.WS) {
		return "."
	}
	entries, _ := w.Ops.FS().ReadDir(w.WS)
	var paths []string
	for _, e := range entries {
		if e.IsDir() && isRepoWithOrigin(w.Ops, filepath.Join(w.WS, e.Name())) {
			paths = append(paths, e.Name())
		}
	}
	if len(paths) == 0 {
		return "."
	}
	return strings.Join(paths, ", ")
}

func isRepoWithOrigin(o ops.Ops, dir string) bool {
	_, _, err := checkRepoPath(o, dir, "", ".")
	return err == nil
}

// repos is question 1's answer as entries, each named by its origin's
// repository and keeping the file's check, image and protected for a path
// the file declares.
func (w *Wizard) repos(answer string) ([]Repo, []Origin, *Error) {
	paths := list(answer)
	if len(paths) == 0 {
		return nil, nil, fieldErr("repos", "no repository (at least one is required)")
	}
	var repos []Repo
	var origins []Origin
	for i, p := range paths {
		origin, _, e := checkRepoPath(w.Ops, w.WS, fmt.Sprintf("repos[%d].path", i), p)
		if e != nil {
			return nil, nil, e
		}
		r := Repo{Name: origin.Repo, Path: p, Check: []string{}, Image: defaultImage, Protected: append([]string(nil), defaultProtected...)}
		if w.File != nil {
			for _, f := range w.File.Repos {
				if filepath.Clean(f.Path) == filepath.Clean(p) {
					r.Check, r.Image, r.Protected = f.Check, f.Image, f.Protected
				}
			}
		}
		if !nameShape.MatchString(r.Name) {
			return nil, nil, fieldErr(fmt.Sprintf("repos[%d].name", i), "%q is not a repository name", r.Name)
		}
		for _, prev := range repos {
			if prev.Name == r.Name {
				return nil, nil, fieldErr(fmt.Sprintf("repos[%d].name", i), "%q is listed twice", r.Name)
			}
		}
		repos, origins = append(repos, r), append(origins, origin)
	}
	return repos, origins, nil
}

// forgeDefault is the forge questions' defaults: the file's forge, else
// what the first repository's origin names, for kind when one is given.
func (w *Wizard) forgeDefault(first Origin, kind string) Forge {
	if w.File != nil {
		return w.File.Forge
	}
	d := detected(first)
	if kind != "" && kind != d.Kind {
		if kind == "github" {
			d.API, d.TokenEnv = "https://api.github.com", "GITHUB_TOKEN"
		} else {
			d.API, d.TokenEnv = "https://"+first.Host+"/api/v1", "GITEA_TOKEN"
		}
		d.Kind = kind
	}
	return d
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
