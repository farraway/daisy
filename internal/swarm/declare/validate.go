package declare

import (
	"fmt"
	"maps"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"daisy/internal/gitx"
	"daisy/internal/ops"
	"daisy/internal/swarm/provider"
)

// providers is the adapter table (section 9): the variables an env
// credential may name, whether a slots credential is accepted, and the
// address of the default co-author.
var providers = provider.Providers

// defaultCoAuthor is a persona's default trailer value: its model and its
// provider's no-reply address.
func defaultCoAuthor(provider, model string) string {
	return model + " <" + providers[provider].Email + ">"
}

// Skip names checks Validate leaves out, for `swarm init`, which writes
// what they check.
type Skip int

const (
	// SkipPrompts: the prompt files, which init writes.
	SkipPrompts Skip = 1 << iota
	// SkipAccount: trusted and forge.commit_email, which init fills after
	// reading the token's account.
	SkipAccount
)

var (
	repoShape   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	varShape    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	nameShape   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
	branchShape = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)
	loginShape  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	effortShape = regexp.MustCompile(`^[a-z]+$`)
)

// Validate checks c, a parse of the swarm.json at p, in section 3's
// order and returns the first problem as an *Error. In a swarm
// repository's clone its origin is forge.repo's, each repository is
// named by repos[].repo and resources and references are read as names,
// their content checked at the launch (section 7.1, step 3); in the old
// layout repos[].path and resources are read under p.Root. The prompts
// are read in p.Dir.
func Validate(o ops.Ops, p Place, c Config, skip Skip) error {
	checks := []func() *Error{
		func() *Error { return checkForge(c.Forge, skip) },
		func() *Error { return checkClone(o, p, c.Forge) },
		func() *Error { return checkRepos(o, p, c.Forge, c.Repos) },
		func() *Error { return checkBase(c.Base) },
		func() *Error { return checkRange("max_members", c.MaxMembers, 1, 16) },
		func() *Error { return checkRange("max_rounds", c.MaxRounds, 1, 10) },
		func() *Error { return checkDuration("timeout", c.Timeout) },
		func() *Error { return checkDuration("interval", c.Interval) },
		func() *Error { return checkRange("owner.max_open_proposals", c.Owner.MaxOpenProposals, 1, 20) },
		func() *Error { return checkResources(o, p, c) },
		func() *Error { return checkReferences(p, c) },
		func() *Error { return checkNotes(c) },
		func() *Error {
			if skip&SkipAccount != 0 {
				return nil
			}
			return checkTrusted(c.Trusted)
		},
		func() *Error { return checkBudget(c.Gates, c.Budget) },
		func() *Error { return checkPersonas(o, p, c.Personas, skip) },
		func() *Error { return checkKeys(c.Keys, c.Personas) },
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateForge checks the forge fields alone, as `pause` and `resume`
// read them to find <state> (section 3).
func ValidateForge(f Forge) error {
	if err := checkForge(f, SkipAccount); err != nil {
		return err
	}
	return nil
}

func checkForge(f Forge, skip Skip) *Error {
	if f.Kind != "gitea" && f.Kind != "github" {
		return fieldErr("forge.kind", "%q has no adapter", f.Kind)
	}
	if !apiOK(f.API) {
		return fieldErr("forge.api", "%q is not an absolute http(s) URL without credentials", f.API)
	}
	if !repoShape.MatchString(f.Repo) {
		return fieldErr("forge.repo", "%q is not <owner>/<repo>", f.Repo)
	}
	if err := checkTokenEnv(f.TokenEnv); err != nil {
		return err
	}
	if skip&SkipAccount == 0 {
		if err := checkCommitEmail(f.CommitEmail); err != nil {
			return err
		}
	}
	if f.Board != nil {
		if f.Kind == "github" {
			return fieldErr("forge.board", "github has no board in version 1")
		}
		if *f.Board == "" {
			return fieldErr("forge.board", "empty")
		}
	}
	return nil
}

// checkTokenEnv: forge.token_env is a variable name, never the
// maintainer's (section 8, rule 5: no swarm process reads it) and never a
// provider's credential variable (a member is given that one, and the
// forge token never enters a container, rule 2).
func checkTokenEnv(v string) *Error {
	if !varShape.MatchString(v) {
		return fieldErr("forge.token_env", "%q is not a variable name", v)
	}
	if v == MaintainerTokenEnv {
		return fieldErr("forge.token_env", "%q is the maintainer's token; the swarm's is its own account's (section 8)", v)
	}
	for _, name := range slices.Sorted(maps.Keys(providers)) {
		if slices.Contains(providers[name].Env, v) {
			return fieldErr("forge.token_env", "%q is %s's credential variable, which a member is given; the forge token never enters a container (section 8)", v, name)
		}
	}
	return nil
}

func apiOK(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil &&
		!strings.ContainsAny(s, "?#") && !strings.HasSuffix(s, "/")
}

func checkCommitEmail(v string) *Error {
	if v == "" {
		return fieldErr("forge.commit_email", "missing")
	}
	if !isAddress(v) {
		return fieldErr("forge.commit_email", "%q is not an email address", v)
	}
	return nil
}

// checkClone: a swarm repository's clone has an origin, without
// credentials, that is forge.repo on the forge's host (section 3,
// forge.repo); the old layout has no such clone.
func checkClone(o ops.Ops, p Place, f Forge) *Error {
	if p.Old() {
		return nil
	}
	raw := readOrigin(o, p.Root)
	if raw == "" {
		return fieldErr("forge.repo", "the clone %s has no origin remote", p.Root)
	}
	origin, ok := parseOrigin(raw)
	if origin.Credentials {
		return fieldErr("forge.repo", "the clone's origin URL carries user information; use SSH or a credential helper")
	}
	if !ok {
		return fieldErr("forge.repo", "the clone's origin is %q, not <owner>/<repo> on a forge", raw)
	}
	if !strings.EqualFold(origin.Host, f.Host()) || !strings.EqualFold(origin.Owner+"/"+origin.Repo, f.Repo) {
		return fieldErr("forge.repo", "the clone's origin is %q, not %s on %s", raw, f.Repo, f.Host())
	}
	return nil
}

// checkRepos checks each declared repository: in a swarm repository's
// clone its repo, `<owner>/<name>` beside the swarm repository, and its
// ssh_command; in the old layout that its path's origin is <owner>/<name>
// on the forge's host. Each layout refuses the other's fields.
func checkRepos(o ops.Ops, p Place, f Forge, repos []Repo) *Error {
	if len(repos) == 0 {
		return fieldErr("repos", "no repository (at least one is required)")
	}
	owner, own, _ := strings.Cut(f.Repo, "/")
	for i, r := range repos {
		at := fmt.Sprintf("repos[%d]", i)
		if !nameShape.MatchString(r.Name) {
			return fieldErr(at+".name", "%q is not a repository name", r.Name)
		}
		if slices.ContainsFunc(repos[:i], func(p Repo) bool { return p.Name == r.Name }) {
			return fieldErr(at+".name", "%q is listed twice", r.Name)
		}
		if p.Old() {
			if err := checkOldRepo(o, p.Root, f, owner, at, r); err != nil {
				return err
			}
		} else if err := checkNamedRepo(owner, own, at, r); err != nil {
			return err
		}
		for j, a := range r.Check {
			if a == "" {
				return fieldErr(fmt.Sprintf("%s.check[%d]", at, j), "empty argument")
			}
		}
		if r.Image == "" {
			return fieldErr(at+".image", "empty")
		}
		for j, p := range r.Protected {
			if p == "" || filepath.IsAbs(p) || slices.Contains(strings.Split(p, "/"), "..") {
				return fieldErr(fmt.Sprintf("%s.protected[%d]", at, j), "%q is not a relative pattern without ..", p)
			}
		}
	}
	return nil
}

// checkNamedRepo is a swarm repository's clone's entry: not the swarm
// repository, no path, repo `<owner>/<name>` (owner forge.repo's, name
// the entry's), an ssh_command, when present, one non-empty line.
func checkNamedRepo(owner, own, at string, r Repo) *Error {
	if strings.EqualFold(r.Name, own) {
		return fieldErr(at+".name", "%q is the swarm repository", r.Name)
	}
	if r.Path != "" {
		return fieldErr(at+".path", "unknown field")
	}
	if r.Repo == "" {
		return fieldErr(at+".repo", "missing")
	}
	if !strings.EqualFold(r.Repo, owner+"/"+r.Name) {
		return fieldErr(at+".repo", "%q is not %s/%s", r.Repo, owner, r.Name)
	}
	if s := r.SSHCommand; s != nil {
		if *s == "" {
			return fieldErr(at+".ssh_command", "empty")
		}
		if strings.ContainsAny(*s, "\n\r") {
			return fieldErr(at+".ssh_command", "contains a newline")
		}
	}
	return nil
}

// checkOldRepo is the old layout's entry: no repo or ssh_command, and
// its path a clone in the workspace whose origin is <owner>/<name> on the
// forge's host.
func checkOldRepo(o ops.Ops, ws string, f Forge, owner, at string, r Repo) *Error {
	if r.Repo != "" {
		return fieldErr(at+".repo", "unknown field")
	}
	if r.SSHCommand != nil {
		return fieldErr(at+".ssh_command", "unknown field")
	}
	if r.Path == "" {
		return fieldErr(at+".path", "missing")
	}
	origin, raw, err := checkRepoPath(o, ws, at+".path", r.Path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(origin.Host, f.Host()) || !strings.EqualFold(origin.Owner, owner) || !strings.EqualFold(origin.Repo, r.Name) {
		return fieldErr(at+".path", "origin is %q, not %s/%s on %s", raw, owner, r.Name, f.Host())
	}
	return nil
}

// checkRepoPath: path, relative inside ws, is the top of a git repository
// whose origin URL (raw) carries no credentials and reads as an Origin.
func checkRepoPath(o ops.Ops, ws, field, path string) (origin Origin, raw string, err *Error) {
	clean := filepath.Clean(path)
	if path == "" || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, "../") {
		return Origin{}, "", fieldErr(field, "%q is not a relative path inside the workspace", path)
	}
	dir := filepath.Join(ws, clean)
	if !gitx.IsRepoRoot(o, dir) {
		return Origin{}, "", fieldErr(field, "%q is not a git repository", path)
	}
	raw = readOrigin(o, dir)
	if raw == "" {
		return Origin{}, "", fieldErr(field, "%q has no origin remote", path)
	}
	origin, ok := parseOrigin(raw)
	if origin.Credentials {
		return Origin{}, "", fieldErr(field, "origin URL carries user information; use SSH or a credential helper")
	}
	if !ok {
		return Origin{}, "", fieldErr(field, "origin is %q, not <owner>/<repo> on a forge", raw)
	}
	return origin, raw, nil
}

func checkBase(b string) *Error {
	if !branchShape.MatchString(b) || strings.Contains(b, "..") || strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".lock") {
		return fieldErr("base", "%q is not a branch name", b)
	}
	return nil
}

func checkRange(field string, n, lo, hi int) *Error {
	if n < lo || n > hi {
		return fieldErr(field, "%d is not between %d and %d", n, lo, hi)
	}
	return nil
}

func checkDuration(field, v string) *Error {
	d, err := time.ParseDuration(v)
	if err != nil {
		return fieldErr(field, "%q is not a Go duration", v)
	}
	if d < time.Minute || d > 24*time.Hour {
		return fieldErr(field, "%q is not between 1m and 24h", v)
	}
	return nil
}

// checkResources: in a swarm repository's clone, each entry is a path in
// the swarm repository or `<name>:<path>` in a declared repository,
// relative, without `..`, not the whole repository and listed once; what
// it holds is read at the pass's commit, where the launch checks it
// (section 7.1, step 3). In the old layout each resolves, through every
// link, to a path inside the workspace that is not the workspace, not
// under swarm/, neither is nor contains .env.sh, .daisy or .git, and is
// listed once.
func checkResources(o ops.Ops, p Place, c Config) *Error {
	if !p.Old() {
		return checkNamedResources(c)
	}
	_, err := resolveResources(o, p, c.Resources)
	return err
}

// SplitResource reads a resources entry of a swarm repository's clone
// (section 3): `<name>:<path>`, a path in the declared repository
// <name>, when the text before the first `:` is a repository name (no
// `/`), else a path in the swarm repository, name "".
func SplitResource(entry string) (name, path string) {
	if n, p, ok := strings.Cut(entry, ":"); ok && nameShape.MatchString(n) {
		return n, p
	}
	return "", entry
}

// relPath is why p is no path a resource may name in a repository, ""
// when it is one: relative, without `..`, and not the whole repository.
func relPath(p string) string {
	if p == "" || strings.HasPrefix(p, "/") || slices.Contains(strings.Split(p, "/"), "..") {
		return "is not a relative path"
	}
	if path.Clean(p) == "." {
		return "is the whole repository"
	}
	return ""
}

func checkNamedResources(c Config) *Error {
	type key struct{ repo, path string }
	var seen []key
	for i, r := range c.Resources {
		at := fmt.Sprintf("resources[%d]", i)
		name, p := SplitResource(r)
		if name != "" && !slices.ContainsFunc(c.Repos, func(d Repo) bool { return d.Name == name }) {
			return fieldErr(at, "%q names no declared repository", r)
		}
		if why := relPath(p); why != "" {
			return fieldErr(at, "%q %s", r, why)
		}
		k := key{name, path.Clean(p)}
		if slices.Contains(seen, k) {
			return fieldErr(at, "%q is listed twice", r)
		}
		seen = append(seen, k)
	}
	return nil
}

var commitShape = regexp.MustCompile(`^[0-9a-f]{40}$`)

// PinIsCommit: a reference's pin is a full commit, not a tag (section 3).
func PinIsCommit(pin string) bool { return commitShape.MatchString(pin) }

// checkReferences: a swarm repository's clone's references (section 3),
// each named as a repository but none declared and not the swarm
// repository, its URL a git URL without credentials, its pin a full commit
// or a tag, its paths as a resource's; the old layout has none.
func checkReferences(p Place, c Config) *Error {
	if c.References == nil {
		return nil
	}
	if p.Old() {
		return fieldErr("references", "unknown field")
	}
	_, own, _ := strings.Cut(c.Forge.Repo, "/")
	for i, r := range c.References {
		at := fmt.Sprintf("references[%d]", i)
		switch {
		case !nameShape.MatchString(r.Name):
			return fieldErr(at+".name", "%q is not a repository name", r.Name)
		case slices.ContainsFunc(c.References[:i], func(o Reference) bool { return o.Name == r.Name }):
			return fieldErr(at+".name", "%q is listed twice", r.Name)
		case slices.ContainsFunc(c.Repos, func(d Repo) bool { return d.Name == r.Name }):
			return fieldErr(at+".name", "%q is a declared repository", r.Name)
		case strings.EqualFold(r.Name, own):
			return fieldErr(at+".name", "%q is the swarm repository", r.Name)
		}
		if credentials, ok := gitURL(r.URL); credentials {
			return fieldErr(at+".url", "%q carries user information", r.URL)
		} else if !ok {
			return fieldErr(at+".url", "%q is not a git URL", r.URL)
		}
		if !PinIsCommit(r.Pin) && checkBase(r.Pin) != nil {
			return fieldErr(at+".pin", "%q is not a commit or a tag", r.Pin)
		}
		if len(r.Paths) == 0 {
			return fieldErr(at+".paths", "no path (at least one is required)")
		}
		for j, q := range r.Paths {
			pat := fmt.Sprintf("%s.paths[%d]", at, j)
			if why := relPath(q); why != "" {
				return fieldErr(pat, "%q %s", q, why)
			}
			if slices.ContainsFunc(r.Paths[:j], func(o string) bool { return path.Clean(o) == path.Clean(q) }) {
				return fieldErr(pat, "%q is listed twice", q)
			}
		}
	}
	return nil
}

// gitURL reads a reference's URL (section 3): https or http, or SSH in
// section 10's two SSH forms, naming a host and a path; credentials
// when an http(s) URL carries user information or an SSH one a password.
func gitURL(raw string) (credentials, ok bool) {
	if !strings.Contains(raw, "://") {
		m := scpLike.FindStringSubmatch(raw)
		return false, m != nil && strings.Trim(m[2], "/") != ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.Contains(raw, "@"), false
	}
	if u.User != nil {
		_, hasPassword := u.User.Password()
		credentials = u.Scheme != "ssh" || hasPassword
	}
	ok = u.Host != "" && strings.Trim(u.Path, "/") != "" && u.RawQuery == "" && u.Fragment == "" &&
		(u.Scheme == "ssh" || u.Scheme == "http" || u.Scheme == "https")
	return credentials, ok
}

// ResolveResources is each resource's path through every link, the
// source its mount is given, once section 3's checks hold of it now (a
// launch makes them again, so a link changed since the pass's validation
// mounts nothing outside them, section 7.2): the old layout's live
// mounts.
func ResolveResources(o ops.Ops, p Place, resources []string) ([]string, error) {
	real, err := resolveResources(o, p, resources)
	if err != nil {
		return nil, err
	}
	return real, nil
}

func resolveResources(o ops.Ops, p Place, resources []string) ([]string, *Error) {
	fs, ws := o.FS(), p.Root
	root, err := fs.EvalSymlinks(ws)
	if err != nil {
		return nil, fieldErr("resources", "the workspace %s cannot be resolved: %v", ws, err)
	}
	var seen []string
	for i, r := range resources {
		at := fmt.Sprintf("resources[%d]", i)
		if r == "" || filepath.IsAbs(r) {
			return nil, fieldErr(at, "%q is not a relative path", r)
		}
		real, err := fs.EvalSymlinks(filepath.Join(ws, r))
		if err != nil {
			return nil, fieldErr(at, "%q does not exist", r)
		}
		rel, err := filepath.Rel(root, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fieldErr(at, "%q is outside the workspace", r)
		}
		if rel == "." {
			return nil, fieldErr(at, "%q is the workspace", r)
		}
		if p.Old() && (rel == "swarm" || strings.HasPrefix(rel, "swarm/")) {
			return nil, fieldErr(at, "%q is under swarm/", r)
		}
		if name := forbiddenIn(fs, rel, real); name != "" {
			return nil, fieldErr(at, "%q contains %s", r, name)
		}
		if slices.Contains(seen, real) {
			return nil, fieldErr(at, "%q is listed twice", r)
		}
		seen = append(seen, real)
	}
	return seen, nil
}

// checkNotes: `notes` keeps the notes in the issues repository's notes/,
// so that repository is the swarm's own, none of repos (Daisy writes no
// declared repository's tree, section 2), and its wiki is not declared
// beside it; `wiki` alone keeps them in a declared issues repository's
// wiki, never the swarm's own repository's.
func checkNotes(c Config) *Error {
	if !c.Notes {
		if _, ok := c.Declares(c.Forge.Repo); c.Wiki && !ok {
			return fieldErr("wiki", "%s is none of repos; wiki keeps a declared issues repository's notes, and the swarm's own keeps them in notes/ (notes)", c.Forge.Repo)
		}
		return nil
	}
	if c.Wiki {
		return fieldErr("wiki", "true beside notes; the swarm's own repository keeps its notes in notes/, and its wiki is no better than its tree")
	}
	if _, ok := c.Declares(c.Forge.Repo); ok {
		return fieldErr("notes", "%s is a declared repository, which holds product code; notes/ is for the swarm's own issues repository (wiki keeps a product's)", c.Forge.Repo)
	}
	return nil
}

// Declares is the declared repository whose forge repository is repo
// (`<owner>/<name>`): repos are their owner's (section 14, item 7), so
// its name decides.
func (c Config) Declares(repo string) (Repo, bool) {
	_, name, _ := strings.Cut(repo, "/")
	for _, r := range c.Repos {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Repo{}, false
}

var forbiddenNames = []string{".env.sh", ".daisy", ".git"}

// forbiddenIn is the first of .env.sh, .daisy and .git that the resource
// at real (rel inside the workspace) is, lies in or holds anywhere in its
// tree, links not followed; "" when none.
func forbiddenIn(fs ops.FS, rel, real string) string {
	for _, part := range strings.Split(rel, "/") {
		if slices.Contains(forbiddenNames, part) {
			return part
		}
	}
	var walk func(dir string) string
	walk = func(dir string) string {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if slices.Contains(forbiddenNames, e.Name()) {
				return e.Name()
			}
		}
		for _, e := range entries {
			if e.IsDir() {
				if name := walk(filepath.Join(dir, e.Name())); name != "" {
					return name
				}
			}
		}
		return ""
	}
	return walk(real)
}

func checkTrusted(trusted []string) *Error {
	if len(trusted) == 0 {
		return fieldErr("trusted", "no login (at least one is required)")
	}
	for i, l := range trusted {
		if !loginShape.MatchString(l) {
			return fieldErr(fmt.Sprintf("trusted[%d]", i), "%q is not a forge login", l)
		}
	}
	return nil
}

func checkBudget(g Gates, b *Budget) *Error {
	if b == nil {
		if !g.Triage || !g.Plan || !g.Merge {
			return fieldErr("budget", "missing (required when a gate is false)")
		}
		return nil
	}
	for _, l := range []struct {
		field string
		v     float64
	}{{"budget.per_day", b.PerDay}, {"budget.total", b.Total}} {
		if l.v <= 0 {
			return fieldErr(l.field, "%s is not greater than 0", strconv.FormatFloat(l.v, 'f', -1, 64))
		}
	}
	return nil
}

func checkPersonas(o ops.Ops, at Place, personas map[string]Persona, skip Skip) *Error {
	var extra []string
	for r := range personas {
		if !slices.Contains(Roles, r) {
			extra = append(extra, r)
		}
	}
	if len(extra) > 0 {
		slices.Sort(extra)
		return fieldErr("personas."+extra[0], "not a role (%s)", strings.Join(Roles, ", "))
	}
	for _, r := range Roles {
		p, ok := personas[r]
		if !ok {
			return fieldErr("personas."+r, "missing")
		}
		if err := checkPersona(r, p); err != nil {
			return err
		}
		if skip&SkipPrompts == 0 {
			b, err := o.FS().ReadFile(PromptFile(at.Dir, r))
			if err != nil || len(b) == 0 {
				return fieldErr("personas."+r, "prompt %s is missing or empty", at.Rel(filepath.Join("prompts", r+".md")))
			}
		}
	}
	return nil
}

func checkPersona(role string, p Persona) *Error {
	at := "personas." + role
	if err := checkProvider(at+".provider", p.Provider); err != nil {
		return err
	}
	if err := checkModel(at+".model", p.Model); err != nil {
		return err
	}
	if !effortShape.MatchString(p.Effort) {
		return fieldErr(at+".effort", "%q is not one lowercase word", p.Effort)
	}
	return checkCoAuthor(at+".co_author", p.CoAuthor)
}

func checkProvider(field, v string) *Error {
	if _, ok := providers[v]; !ok {
		return fieldErr(field, "%q has no adapter", v)
	}
	return nil
}

// checkModel: every role names the model it runs, as one argv word.
func checkModel(field, v string) *Error {
	switch {
	case v == "":
		return fieldErr(field, "empty; every role names its model")
	case v == "-":
		return fieldErr(field, "%q is not a model; every role names its model", v)
	case strings.ContainsFunc(v, unicode.IsSpace):
		return fieldErr(field, "%q contains whitespace", v)
	}
	return nil
}

// isAddress: `<local>@<domain>`, both parts non-empty, without
// whitespace, control characters or angle brackets.
func isAddress(v string) bool {
	local, domain, ok := strings.Cut(v, "@")
	return ok && local != "" && domain != "" && !strings.Contains(domain, "@") &&
		!strings.ContainsFunc(v, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '<' || r == '>' })
}

// checkCoAuthor: `<name> <<email>>`, the name without control characters,
// angle brackets or whitespace at either end, the email an address; or ""
// for none, the role's commits carrying no Co-Authored-By trailer.
func checkCoAuthor(field, v string) *Error {
	if v == "" {
		return nil
	}
	name, rest, ok := strings.Cut(v, " <")
	email, closed := strings.CutSuffix(rest, ">")
	if !ok || !closed || name == "" || strings.TrimFunc(name, unicode.IsSpace) != name || !isAddress(email) ||
		strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || r == '<' || r == '>' }) {
		return fieldErr(field, "%q is not <name> <email>", v)
	}
	return nil
}

func checkKeys(keys map[string]Key, personas map[string]Persona) *Error {
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, name := range names {
		k, at := keys[name], "keys."+name
		p, ok := providers[name]
		if !ok {
			return fieldErr(at, "%q has no adapter", name)
		}
		if (k.Env == nil) == (k.Slots == nil) {
			return fieldErr(at, "needs exactly one of env, slots")
		}
		if k.Env != nil && !slices.Contains(p.Env, *k.Env) {
			return fieldErr(at+".env", "%q is not one of %s's variables (%s)", *k.Env, name, strings.Join(p.Env, ", "))
		}
		if k.Slots != nil {
			if !p.Slot() {
				return fieldErr(at+".slots", "%s takes no slot", name)
			}
			if err := checkRange(at+".slots", *k.Slots, 1, 16); err != nil {
				return err
			}
		}
	}
	for _, r := range Roles {
		if _, ok := keys[personas[r].Provider]; !ok {
			return fieldErr("keys", "no credential for provider %q", personas[r].Provider)
		}
	}
	return nil
}

// PromptFile is the prompt of role in dir, the configuration folder
// (Place.Dir).
func PromptFile(dir, role string) string { return filepath.Join(dir, "prompts", role+".md") }
