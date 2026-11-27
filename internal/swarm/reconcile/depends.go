package reconcile

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/fence"
	"daisy/internal/swarm/forge"
)

// The declarations of section 12.1 (rules 1 and 2): `Depends:`, alone on
// a line, whitespace at either end aside; `Touches:` and `Safe with:`
// alone on a line of a slice's item, leading spaces aside.
var (
	dependsRe  = regexp.MustCompile(`^Depends: (none|#[1-9][0-9]*(?:, #[1-9][0-9]*)*)$`)
	touchesRe  = regexp.MustCompile(`^Touches: (.*)$`)
	safeWithRe = regexp.MustCompile(`^Safe with: (#[1-9][0-9]*(?:, #[1-9][0-9]*)*)$`)
	refRe      = regexp.MustCompile(`#([1-9][0-9]*)`)
)

// isDeclaration: text is a `Depends:` line alone (section 12.1, rule 1),
// which, as a class statement, is no counted entry.
func isDeclaration(text string) bool { return dependsRe.MatchString(strings.TrimSpace(text)) }

// refs is the issue numbers `#<m>` of text, in order, each once.
func refs(text string) []int {
	var out []int
	for _, m := range refRe.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(m[1])
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// declaredIn is the issues of text's first `Depends:` line outside a
// fenced block; ok is false when it holds none. `Depends: none` names
// none and is a declaration.
func declaredIn(text string) (deps []int, ok bool) {
	var fences fence.Scan
	for _, line := range strings.Split(text, "\n") {
		if fences.Line(line) {
			continue
		}
		if m := dependsRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return refs(m[1]), true
		}
	}
	return nil, false
}

// Deps is an issue's dependencies (section 12.1, rule 1): the issues its
// newest `Depends:` declaration names, and those the maintainer's newest
// `Blocked by #<m>` comments name, the issue itself left out of both.
type Deps struct {
	Declared  []int
	BlockedBy []int
}

// All is every dependency, the declaration's first, each once.
func (d Deps) All() []int {
	out := slices.Clone(d.Declared)
	for _, m := range d.BlockedBy {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// Dependencies is issue is's dependencies (section 12.1, rule 1): the
// issues of its newest `Depends:` declaration, read in three places, the
// newest by its entry's time standing (ties to the later place): the first
// such line outside a fence in the body of a proposal the swarm created
// (marked role=owner or role=qa), dated by the issue; the first such line
// outside a fence in plan, the accepted plan, dated planAt (planned false
// for no plan); and a comment by a login in trusted other than the
// swarm's whose text is the line alone; then the `Blocked by #<m>`
// comments that are the maintainer's newest (BlockedBy). A dependency on
// the issue itself is ignored.
func Dependencies(is forge.Issue, plan string, planned bool, planAt time.Time, swarmLogin string, trusted []string) Deps {
	var declared []int
	var at time.Time
	found := false
	take := func(deps []int, when time.Time) {
		if !found || !when.Before(at) {
			declared, at, found = deps, when, true
		}
	}
	body := bodyEntry(is)
	if m, ok := Swarms(body, swarmLogin); ok && (m.Role == "owner" || m.Role == declare.QARole) {
		if deps, ok := declaredIn(swarmText(body.Body)); ok {
			take(deps, is.Created)
		}
	}
	if planned {
		if deps, ok := declaredIn(plan); ok {
			take(deps, planAt)
		}
	}
	for _, c := range is.Comments {
		if !trustedOther(c.Author, swarmLogin, trusted) {
			continue
		}
		if m := dependsRe.FindStringSubmatch(strings.TrimSpace(c.Body)); m != nil {
			take(refs(m[1]), c.Created)
		}
	}
	self := func(m int) bool { return m == is.Number }
	return Deps{
		Declared:  slices.DeleteFunc(declared, self),
		BlockedBy: slices.DeleteFunc(BlockedBy(is, swarmLogin, trusted), self),
	}
}

// trustedOther: login is in trusted and is not the swarm's.
func trustedOther(login, swarmLogin string, trusted []string) bool {
	return !strings.EqualFold(login, swarmLogin) && slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, login) })
}

// dependencies is j's dependencies (Dependencies), its plan as readPlan
// read it, dated by its post.
func (j *Job) dependencies(trusted []string) Deps {
	plan, planned := j.PlanText()
	var at time.Time
	if e, _, ok := j.Plan(); ok {
		at = e.Created
	}
	return Dependencies(j.Issue, plan, planned, at, j.SwarmLogin, trusted)
}

// openIssues is the issues of ns the forge reads open, in order, and those
// it does not hold (forge.ErrNotFound): the one test of whether a
// dependency, or an issue the owner's hold waits on, still holds
// (section 12.1, rules 1 and 5).
func openIssues(f forge.Adapter, ns []int) (open, missing []int, err error) {
	for _, n := range ns {
		is, err := f.Issue(n)
		if errors.Is(err, forge.ErrNotFound) {
			missing = append(missing, n)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if is.Open {
			open = append(open, n)
		}
	}
	return open, missing, nil
}

// OpenDependencies is the dependencies of issue n that hold it (section
// 12.1, rule 1): those the forge reads open, whatever their stage. One the
// forge does not hold holds nothing, with a warning on warn naming the
// line that named it, so one mistyped number does not stop the pass.
func OpenDependencies(f forge.Adapter, n int, d Deps, warn io.Writer) ([]int, error) {
	open, missing, err := openIssues(f, d.All())
	if err != nil {
		return nil, err
	}
	for _, m := range missing {
		line := fmt.Sprintf("Blocked by #%d", m)
		if slices.Contains(d.Declared, m) {
			line = fmt.Sprintf("Depends: #%d", m)
		}
		fmt.Fprintf(warn, "⚠ #%d: %s names no issue; not held\n", n, line)
	}
	return open, nil
}

// Touch is what a slice declares and changes for section 12.1's rule 2:
// its `Touches:` patterns, the issues its `Safe with:` line names, and its
// branch files.
type Touch struct {
	Patterns []string
	Safe     []int
	Files    []string
}

// patternsOf is a `Touches:` line's patterns, comma-separated; `none`
// names none.
func patternsOf(list string) []string {
	var out []string
	for _, p := range strings.Split(list, ",") {
		if p = strings.TrimSpace(p); p != "" && p != "none" {
			out = append(out, p)
		}
	}
	return out
}

// touchesIn is the patterns of text's first `Touches:` line outside a
// fenced block, leading spaces aside; ok is false when it has none.
func touchesIn(text string) (patterns []string, ok bool) {
	var fences fence.Scan
	for _, line := range strings.Split(text, "\n") {
		if fences.Line(line) {
			continue
		}
		if m := touchesRe.FindStringSubmatch(strings.TrimRight(strings.TrimLeft(line, " \t"), " \t\r")); m != nil {
			return patternsOf(m[1]), true
		}
	}
	return nil, false
}

// ProposalTouches is the patterns of the `Touches:` line of the body of a
// proposal the swarm created (marked role=owner or role=qa), which stands
// for every slice of its job; none for another body.
func ProposalTouches(is forge.Issue, swarmLogin string) []string {
	body := bodyEntry(is)
	if m, ok := Swarms(body, swarmLogin); ok && (m.Role == "owner" || m.Role == declare.QARole) {
		p, _ := touchesIn(swarmText(body.Body))
		return p
	}
	return nil
}

// SliceTouch is slice s's declarations (section 12.1, rule 2): the
// patterns of the `Touches:` line in its item of plan, else the
// proposal's (ProposalTouches), and the issues of its item's `Safe with:`
// line; planned false for no plan.
func SliceTouch(is forge.Issue, plan string, planned bool, s int, swarmLogin string) Touch {
	var t Touch
	item := ""
	if planned {
		item = sliceText(plan, "", s)
	}
	patterns, ok := touchesIn(item)
	if !ok {
		patterns = ProposalTouches(is, swarmLogin)
	}
	t.Patterns = patterns
	var fences fence.Scan
	for _, line := range strings.Split(item, "\n") {
		if fences.Line(line) {
			continue
		}
		if m := safeWithRe.FindStringSubmatch(strings.TrimRight(strings.TrimLeft(line, " \t"), " \t\r")); m != nil {
			t.Safe = refs(m[1])
			break
		}
	}
	return t
}

// Overlap: slices a and b overlap (section 12.1, rule 2), a pattern of one
// matching a branch file of the other (as a protected pattern matches,
// Protected), or a pattern of each possibly matching one path
// (patternsMeet). A slice with no patterns overlaps another only by its
// branch files.
func Overlap(a, b Touch) bool {
	if len(Protected(b.Files, a.Patterns)) > 0 || len(Protected(a.Files, b.Patterns)) > 0 {
		return true
	}
	for _, p := range a.Patterns {
		for _, q := range b.Patterns {
			if patternsMeet(p, q) {
				return true
			}
		}
	}
	return false
}

// patternsMeet: patterns p and q may match one path, erring towards yes:
// either is a pattern without `/`, which matches at any depth, or their
// literal prefixes (each up to its first `*`, `?` or `[`) are one a
// prefix of the other.
func patternsMeet(p, q string) bool {
	if !strings.Contains(p, "/") || !strings.Contains(q, "/") {
		return true
	}
	lp, lq := literal(p), literal(q)
	return strings.HasPrefix(lp, lq) || strings.HasPrefix(lq, lp)
}

// literal is a pattern up to its first `*`, `?` or `[`, cleaned of a
// leading `./`.
func literal(p string) string {
	if i := strings.IndexAny(p, "*?["); i >= 0 {
		p = p[:i]
	}
	return strings.TrimPrefix(p, "./")
}

// touchRead is what rule 2 of section 12.1 reads beside the job: the
// forge, the mirrors as the pass fetched them, the jobs' workspaces and
// the runs.
type touchRead struct {
	f     forge.Adapter
	git   Git
	out   Outputs
	cfg   declare.Config
	login string
}

// openSlice is another issue's open slice in a repository: slice s of
// issue n, whose branch changes files.
type openSlice struct {
	n, s  int
	files []string
}

// overlapping is the issue whose open slice holds the first build of j's
// next slice (section 12.1, rule 2), the first by issue then slice; 0 for
// none. An open slice is one of another issue in the slice's repository
// whose branch swarm/<m>-<s> has an open pull request, or whose worker
// run is not collected (running, or ended and waiting for its
// collection); its branch files are those `base...branch` changes in the
// mirror (OpenBranches). Two overlap unless either's item names the
// other's issue on a `Safe with:` line. The caller holds only a worker's
// first build of a slice, at plan-1-accepted with no open pull request.
func (t touchRead) overlapping(j *Job) (int, error) {
	s, repo, bad := j.WorkerSlice()
	if len(bad) > 0 || repo == "" {
		return 0, nil // the launch's to refuse
	}
	plan, planned := j.PlanText()
	mine := SliceTouch(j.Issue, plan, planned, s, t.login)
	branches, err := t.git.OpenBranches(repo, t.cfg.Base, "")
	if err != nil {
		return 0, err
	}
	var open []openSlice
	add := func(o openSlice) {
		if !slices.ContainsFunc(open, func(p openSlice) bool { return p.n == o.n && p.s == o.s }) {
			open = append(open, o)
		}
	}
	filesOf := func(branch string) []string {
		if i := slices.IndexFunc(branches, func(b OpenBranch) bool { return b.Branch == branch }); i >= 0 {
			return branches[i].Files
		}
		return nil
	}
	for _, b := range branches {
		if b.N == j.N {
			if b.S == s {
				mine.Files = b.Files
			}
			continue
		}
		pulls, err := t.f.Pulls(b.N, Openers(t.login, t.cfg.Trusted))
		if err != nil {
			return 0, err
		}
		if slices.ContainsFunc(pulls, func(p forge.Pull) bool { return p.Open && p.Repo == repo && p.Head == b.Branch }) {
			add(openSlice{b.N, b.S, b.Files})
		}
	}
	runs, err := Runs(t.out.FS, t.out.State)
	if err != nil {
		return 0, err
	}
	for _, r := range runs {
		if r.Role != "worker" || r.N == j.N {
			continue
		}
		lines, _ := r.Result(t.out.FS)
		if _, done := Has(lines, "collected"); done || branchRepo(lines) != repo {
			continue
		}
		branch := runBranch(lines)
		add(openSlice{r.N, sliceOf(forge.Pull{Head: branch}, r.N), filesOf(branch)})
	}
	slices.SortFunc(open, func(a, b openSlice) int {
		if a.n != b.n {
			return a.n - b.n
		}
		return a.s - b.s
	})
	for _, o := range open {
		is, err := t.f.Issue(o.n)
		if errors.Is(err, forge.ErrNotFound) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if !is.Open {
			continue
		}
		// Its plan as its workspace holds it; one that cannot be read
		// leaves its proposal's patterns.
		p, err := t.out.jobPlan(is, t.login)
		if err != nil {
			p = jobPlan{}
		}
		theirs := SliceTouch(is, p.text, p.ok, o.s, t.login)
		theirs.Files = o.files
		if slices.Contains(mine.Safe, o.n) || slices.Contains(theirs.Safe, j.N) {
			continue
		}
		if Overlap(mine, theirs) {
			return o.n, nil
		}
	}
	return 0, nil
}

// touchRead is the pass's rule 2 reader.
func (s *Swarm) touchRead() touchRead {
	return touchRead{f: s.Forge, git: s.git(), out: s.outputs(), cfg: s.Cfg, login: s.SwarmLogin}
}
