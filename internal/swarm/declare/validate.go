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
// order and returns the first problem as an *Error: the clone's origin
// is forge.repo's, each repository is named by repos[].repo, and
// resources and references are read as names, their content checked at
// the launch (section 7.1, step 3). The prompts are read in p.Root.
func Validate(o ops.Ops, p Place, c Config, skip Skip) error {
	checks := []func() *Error{
		func() *Error { return checkForge(c.Forge, skip) },
		func() *Error { return checkClone(o, p, c.Forge) },
		func() *Error { return checkRepos(c) },
		func() *Error { return checkBase(c.Base) },
		func() *Error { return checkRange("max_members", c.MaxMembers, 1, 16) },
		func() *Error { return checkRange("max_rounds", c.MaxRounds, 1, 10) },
		func() *Error { return checkDuration("timeout", c.Timeout) },
		func() *Error { return checkDuration("interval", c.Interval) },
		func() *Error { return checkRange("owner.max_open_proposals", c.Owner.MaxOpenProposals, 1, 20) },
		func() *Error { return checkQA(c.QA) },
		func() *Error { return checkResources(c) },
		func() *Error { return checkReferences(c) },
		func() *Error {
			if skip&SkipAccount != 0 {
				return nil
			}
			return checkTrusted(c.Trusted)
		},
		func() *Error { return checkNotify(c.Notify) },
		func() *Error { return checkBudget(c.Gates, c.Budget) },
		func() *Error { return checkKeys(c.Keys, c.Personas, c.DeclaredRoles()) },
		func() *Error { return checkPersonas(o, p, c, skip) },
		func() *Error { return checkPromptParts(o, p, c, skip) },
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
// forge.repo); init's folder has no clone yet (Outside).
func checkClone(o ops.Ops, p Place, f Forge) *Error {
	if p.Outside {
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

// IsCloneOf: dir is the top of a clone whose origin is forge.repo on the
// forge's host, the clone init adopts (section 11, init).
func IsCloneOf(o ops.Ops, dir string, f Forge) bool {
	return gitx.IsRepoRoot(o, dir) && checkClone(o, Clone(dir), f) == nil
}

// checkRepos checks each declared repository: its repo, `<owner>/<name>`
// beside the swarm repository, its ssh_command and its env, which names
// no secret of c's.
func checkRepos(c Config) *Error {
	f, repos := c.Forge, c.Repos
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
		if err := checkNamedRepo(owner, own, at, r); err != nil {
			return err
		}
		for j, a := range r.Check {
			if a == "" {
				return fieldErr(fmt.Sprintf("%s.check[%d]", at, j), "empty argument")
			}
		}
		if err := checkDeps(at, r); err != nil {
			return err
		}
		if p := r.Services; p != nil && !isFilePath(*p) {
			return fieldErr(at+".services", "%q is not a file's path in the repository", *p)
		}
		if err := checkEnv(at, r, c.SecretNames()); err != nil {
			return err
		}
		if r.Image == "" {
			return fieldErr(at+".image", "empty")
		}
		if p := r.Dockerfile; p != nil && !isFilePath(*p) {
			return fieldErr(at+".dockerfile", "%q is not a file's path in the repository", *p)
		}
		for _, list := range []struct {
			field    string
			patterns []string
		}{{"protected", r.Protected}, {"rulings", r.RulingPaths()}} {
			for j, p := range list.patterns {
				if p == "" || filepath.IsAbs(p) || slices.Contains(strings.Split(p, "/"), "..") {
					return fieldErr(fmt.Sprintf("%s.%s[%d]", at, list.field, j), "%q is not a relative pattern without ..", p)
				}
			}
		}
	}
	return nil
}

// Unchecked is each declared repository, by its index in repos, whose
// pull requests the merge step merges with no check of Daisy's run on
// them: with gates.merge false, each whose check is empty (section 12).
// Validate refuses neither: apply warns of each (UncheckedWarning), and
// status lists each under Waiting for you, which notify tells.
func (c Config) Unchecked() []int {
	if c.Gates.Merge {
		return nil
	}
	var out []int
	for i, r := range c.Repos {
		if len(r.Check) == 0 {
			out = append(out, i)
		}
	}
	return out
}

// UncheckedWarning is apply's warning of repos[i], an index Unchecked
// returns (section 3).
func (c Config) UncheckedWarning(i int) string {
	return fmt.Sprintf("swarm.json: repos[%d] (%s) has no check and gates.merge is false: no check of Daisy's runs on its pull requests before the merge step merges them", i, c.Repos[i].Name)
}

// checkDeps is a repository's dependency step, when declared: an argv of
// non-empty arguments, run only before a check, so the repository
// declares one, and each env name a variable name.
func checkDeps(at string, r Repo) *Error {
	d := r.Deps
	if d == nil {
		return nil
	}
	if len(d.Run) == 0 {
		return fieldErr(at+".deps.run", "no argument (at least one is required)")
	}
	for j, a := range d.Run {
		if a == "" {
			return fieldErr(fmt.Sprintf("%s.deps.run[%d]", at, j), "empty argument")
		}
	}
	for _, k := range slices.Sorted(maps.Keys(d.Env)) {
		if !varShape.MatchString(k) {
			return fieldErr(at+".deps.env."+k, "%q is not a variable name", k)
		}
	}
	if len(r.Check) == 0 {
		return fieldErr(at+".deps", "no check to run it before; declare check, or remove deps")
	}
	return nil
}

// isFilePath is a file's path in a repository, as services and
// dockerfile are declared (section 3): relative, clean (path.Clean's
// own), without .. or .git, not the repository's root (.) nor ending /,
// one line without a NUL, and no space at either end.
func isFilePath(p string) bool {
	parts := strings.Split(p, "/")
	return p != "" && p != "." && !filepath.IsAbs(p) && !strings.HasSuffix(p, "/") && !strings.ContainsAny(p, "\n\r\x00") &&
		strings.Trim(p, " ") == p && !slices.Contains(parts, "..") && !slices.Contains(parts, ".git") && path.Clean(p) == p
}

// checkEnv is a repository's env, in name order: each name a variable
// name, none a secret's (section 3, Secret values: the file is the swarm
// repository's) and none also in deps.env, so a variable has one
// declared value; each value one line.
func checkEnv(at string, r Repo, secrets []string) *Error {
	for _, k := range slices.Sorted(maps.Keys(r.Env)) {
		field := at + ".env." + k
		switch {
		case !varShape.MatchString(k):
			return fieldErr(field, "%q is not a variable name", k)
		case slices.Contains(secrets, k):
			return fieldErr(field, "a secret's variable is never given this way")
		case r.Deps != nil && hasKey(r.Deps.Env, k):
			return fieldErr(field, "also in deps.env; declare it once")
		case strings.ContainsAny(r.Env[k], "\n\r"):
			return fieldErr(field, "a value holds a newline")
		}
	}
	return nil
}

func hasKey(m map[string]string, k string) bool { _, ok := m[k]; return ok }

// checkNamedRepo is a declared repository's entry: not the swarm
// repository, repo `<owner>/<name>` (owner forge.repo's, name the
// entry's), an ssh_command, when present, one non-empty line.
func checkNamedRepo(owner, own, at string, r Repo) *Error {
	if strings.EqualFold(r.Name, own) {
		return fieldErr(at+".name", "%q is the swarm repository", r.Name)
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

// SplitResource reads a resources entry (section 3): `<name>:<path>`, a path in the declared repository
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

// checkResources: each entry is a path in the swarm repository or
// `<name>:<path>` in a declared repository, relative, without `..`, not
// the whole repository, not the swarm repository's journal/ (Daisy's
// record of the runs, which no member is given: section 7.6) and listed
// once; what it holds is read at the
// pass's commit, where the launch checks it (section 7.1, step 3).
func checkResources(c Config) *Error {
	type key struct{ repo, path string }
	var seen []key
	for i, res := range c.Resources {
		at, r := fmt.Sprintf("resources[%d]", i), res.Path
		if err := checkNote(at, res); err != nil {
			return err
		}
		name, p := SplitResource(r)
		if name != "" && !slices.ContainsFunc(c.Repos, func(d Repo) bool { return d.Name == name }) {
			return fieldErr(at, "%q names no declared repository", r)
		}
		if why := relPath(p); why != "" {
			return fieldErr(at, "%q %s", r, why)
		}
		if cl := path.Clean(p); name == "" && (cl == JournalDir || strings.HasPrefix(cl, JournalDir+"/")) {
			return fieldErr(at, "%q is the swarm's journal, which no member reads", r)
		}
		k := key{name, path.Clean(p)}
		if slices.Contains(seen, k) {
			return fieldErr(at, "%q is listed twice", r)
		}
		seen = append(seen, k)
	}
	return nil
}

// checkNote: a resource's note is one line, since the prompt lists it on
// its path's line (section 7.3).
func checkNote(at string, r Resource) *Error {
	if strings.ContainsAny(r.Note, "\n\r") {
		return fieldErr(at+".note", "contains a newline")
	}
	return nil
}

// JournalDir is the swarm's journal in the swarm repository (section 7.6,
// The run's record).
const JournalDir = "journal"

var commitShape = regexp.MustCompile(`^[0-9a-f]{40}$`)

// PinIsCommit: a reference's pin is a full commit, not a tag (section 3).
func PinIsCommit(pin string) bool { return commitShape.MatchString(pin) }

// checkReferences: the references (section 3), each named as a
// repository but none declared and not the swarm repository, its URL a
// git URL without credentials, its pin a full commit or a tag, its paths
// as a resource's.
func checkReferences(c Config) *Error {
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
		for j, res := range r.Paths {
			pat, q := fmt.Sprintf("%s.paths[%d]", at, j), res.Path
			if err := checkNote(pat, res); err != nil {
				return err
			}
			if why := relPath(q); why != "" {
				return fieldErr(pat, "%q %s", q, why)
			}
			if slices.ContainsFunc(r.Paths[:j], func(o Resource) bool { return path.Clean(o.Path) == path.Clean(q) }) {
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

// Declares is the declared repository whose forge repository is repo
// (`<owner>/<name>`): repos are their owner's (docs/swarm/decisions/0007-every-declared-repository-is-owners-at-its-origin.md), so
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

// checkNotify: notify, when present, is one argv, each element
// non-empty, as a check is (section 3).
func checkNotify(argv []string) *Error {
	for i, a := range argv {
		if a == "" {
			return fieldErr(fmt.Sprintf("notify[%d]", i), "empty argument")
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

// checkQA: the QA trigger, when declared, names merges, a
// tag pattern or both (section 3, qa); Daisy picks no number.
func checkQA(e *QA) *Error {
	switch {
	case e == nil:
		return nil
	case e.Merges == nil && e.Tag == nil:
		return fieldErr("qa", "no trigger (merges or tag is required)")
	case e.Merges != nil && *e.Merges < 1:
		return fieldErr("qa.merges", "%d is not greater than 0", *e.Merges)
	case e.Tag != nil && !tagPattern(*e.Tag):
		return fieldErr("qa.tag", "%q is not a tag pattern", *e.Tag)
	}
	return nil
}

// tagPattern: one git argument naming tags by a pattern, `v*`: not
// empty, not read as an option, without whitespace or a control
// character.
func tagPattern(v string) bool {
	return v != "" && !strings.HasPrefix(v, "-") && !strings.ContainsFunc(v, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

func checkPersonas(o ops.Ops, at Place, c Config, skip Skip) *Error {
	personas := c.Personas
	var extra []string
	for r := range personas {
		if !slices.Contains(AllRoles, r) {
			extra = append(extra, r)
		}
	}
	if len(extra) > 0 {
		slices.Sort(extra)
		return fieldErr("personas."+extra[0], "not a role (%s)", strings.Join(AllRoles, ", "))
	}
	if _, ok := personas[QARole]; ok && c.QA == nil {
		return fieldErr("personas."+QARole, "no trigger runs it; declare qa, or remove personas.qa")
	}
	for _, r := range c.DeclaredRoles() {
		p, ok := personas[r]
		if !ok && r == QARole {
			return fieldErr("personas."+r, "missing (required when qa is declared)")
		}
		if !ok {
			return fieldErr("personas."+r, "missing")
		}
		if err := checkPersona(r, p, c.Keys); err != nil {
			return err
		}
		if skip&SkipPrompts == 0 {
			b, err := o.FS().ReadFile(PromptFile(at.Root, r))
			if err != nil || len(b) == 0 {
				return fieldErr("personas."+r, "prompt %s is missing or empty", filepath.Join("prompts", r+".md"))
			}
		}
	}
	return nil
}

// checkPromptParts: each prompt_parts entry's path is relative and
// without `..`, so in the clone, and names a file there that is not
// empty (unless the prompts are skipped, as at init's first
// validation); its roles are one at least, each a role swarm.json may
// declare (section 3, prompt_parts).
func checkPromptParts(o ops.Ops, at Place, c Config, skip Skip) *Error {
	for i, part := range c.PromptParts {
		field := fmt.Sprintf("prompt_parts[%d]", i)
		if part.Path == "" || filepath.IsAbs(part.Path) || slices.Contains(strings.Split(filepath.ToSlash(part.Path), "/"), "..") {
			return fieldErr(field+".path", "%q is outside the clone", part.Path)
		}
		if len(part.Roles) == 0 {
			return fieldErr(field+".roles", "no role (at least one is required)")
		}
		for j, r := range part.Roles {
			if !slices.Contains(AllRoles, r) {
				return fieldErr(fmt.Sprintf("%s.roles[%d]", field, j), "%q is not a role (%s)", r, strings.Join(AllRoles, ", "))
			}
		}
		if skip&SkipPrompts == 0 {
			b, err := o.FS().ReadFile(filepath.Join(at.Root, part.Path))
			if err != nil || len(b) == 0 {
				return fieldErr(field+".path", "%s is missing or empty", part.Path)
			}
		}
	}
	return nil
}

// checkPersona: the persona's fields, then each fallback entry's as the
// persona's, no entry (the persona among them) naming the provider,
// model and effort of an earlier one (section 3,
// personas.<role>.fallback), and each entry's slots those of its
// provider's credential (personas.<role>.slots); then each by_label
// entry's label, not empty and no label twice, its fields and slots as
// a fallback entry's, and no entry naming the provider, model and effort
// of the persona or of an earlier by_label entry (personas.<role>.by_label;
// the maintainer's ruling of 2026-11-26); one the same as a fallback
// entry is valid, the chain holding it once.
func checkPersona(role string, p Persona, keys map[string]Key) *Error {
	at := "personas." + role
	entries := p.Entries()
	for i, e := range entries {
		field := at
		if i > 0 {
			field = fmt.Sprintf("%s.fallback[%d]", at, i-1)
		}
		if err := checkEntry(field, e); err != nil {
			return err
		}
		if err := checkSlots(field+".slots", e, keys); err != nil {
			return err
		}
		for j, was := range entries[:i] {
			if was.Is(e) {
				same := at
				if j > 0 {
					same = fmt.Sprintf("%s.fallback[%d]", at, j-1)
				}
				return fieldErr(field, "the same as %s", same)
			}
		}
	}
	for i, b := range p.ByLabel {
		field := fmt.Sprintf("%s.by_label[%d]", at, i)
		if b.Label == "" {
			return fieldErr(field+".label", "empty; every by_label entry names its label")
		}
		if slices.ContainsFunc(p.ByLabel[:i], func(o ByLabel) bool { return o.Label == b.Label }) {
			return fieldErr(field+".label", "%q is listed twice", b.Label)
		}
		if err := checkEntry(field, b.Entry()); err != nil {
			return err
		}
		if err := checkSlots(field+".slots", b.Entry(), keys); err != nil {
			return err
		}
		if entries[0].Is(b.Entry()) {
			return fieldErr(field, "the same as %s", at)
		}
		for j, was := range p.ByLabel[:i] {
			if was.Entry().Is(b.Entry()) {
				return fieldErr(field, "the same as %s.by_label[%d]", at, j)
			}
		}
	}
	return nil
}

// checkSlots: an entry's slots, when it lists any, name slots its
// provider's credential counts, one or more, none twice (section 3,
// personas.<role>.slots); a provider whose credential is env has none.
func checkSlots(field string, e Entry, keys map[string]Key) *Error {
	if e.Slots == nil {
		return nil
	}
	key := keys[e.Provider]
	switch {
	case key.Slots == nil:
		return fieldErr(field, "%s's credential is env, which has no slots", e.Provider)
	case len(e.Slots) == 0:
		return fieldErr(field, "no slot")
	}
	for i, j := range e.Slots {
		at := fmt.Sprintf("%s[%d]", field, i)
		if err := checkRange(at, j, 1, *key.Slots); err != nil {
			return err
		}
		if slices.Contains(e.Slots[:i], j) {
			return fieldErr(at, "%d is listed twice", j)
		}
	}
	return nil
}

func checkEntry(at string, e Entry) *Error {
	if err := checkProvider(at+".provider", e.Provider); err != nil {
		return err
	}
	if err := checkModel(at+".model", e.Model); err != nil {
		return err
	}
	if !effortShape.MatchString(e.Effort) {
		return fieldErr(at+".effort", "%q is not one lowercase word", e.Effort)
	}
	return checkCoAuthor(at+".co_author", e.CoAuthor)
}

func checkProvider(field, v string) *Error {
	if _, ok := providers[v]; !ok {
		return fieldErr(field, "%q has no adapter", v)
	}
	return nil
}

// checkModel: every role names the model it runs, as one argv word,
// without a control or format character (a zero-width space among them)
// that the word would carry unseen.
func checkModel(field, v string) *Error {
	switch {
	case v == "":
		return fieldErr(field, "empty; every role names its model")
	case v == "-":
		return fieldErr(field, "%q is not a model; every role names its model", v)
	case strings.ContainsFunc(v, unicode.IsSpace):
		return fieldErr(field, "%q contains whitespace", v)
	case strings.ContainsFunc(v, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }):
		return fieldErr(field, "%q contains a control or format character", v)
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

// checkKeys runs before checkPersonas (section 3's order), so a persona
// missing or naming a provider without an adapter is left to it.
func checkKeys(keys map[string]Key, personas map[string]Persona, roles []string) *Error {
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
		if t := k.Tokens; t != nil {
			if t.PerDay == nil && t.PerWeek == nil {
				return fieldErr(at+".tokens", "no budget (per_day or per_week)")
			}
			for _, b := range []struct {
				field string
				v     *int
			}{{"per_day", t.PerDay}, {"per_week", t.PerWeek}} {
				if b.v != nil && *b.v <= 0 {
					return fieldErr(at+".tokens."+b.field, "%d is not greater than 0", *b.v)
				}
			}
		}
	}
	for _, r := range roles {
		p, ok := personas[r]
		if !ok {
			continue
		}
		for _, e := range p.All() { // the persona's provider, then each fallback and by_label entry's (section 3)
			if _, adapter := providers[e.Provider]; !adapter {
				continue
			}
			if _, ok := keys[e.Provider]; !ok {
				return fieldErr("keys", "no credential for provider %q", e.Provider)
			}
		}
	}
	return nil
}

// PromptFile is the prompt of role in dir, the swarm repository's clone
// (Place.Root).
func PromptFile(dir, role string) string { return filepath.Join(dir, "prompts", role+".md") }
