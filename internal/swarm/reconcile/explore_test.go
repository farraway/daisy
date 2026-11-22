package reconcile

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// exploreDepth is the explorer's bound, in events: `go test
// ./internal/swarm/reconcile -run TestExploreLifecycle -args
// -explore.depth=7` explores deeper than the suite does.
var exploreDepth = flag.Int("explore.depth", 5, "the lifecycle explorer's bound, in events")

// The swarm's lifecycle explored (section 5): every sequence of events up
// to the bound, over two issues, from #1 approved at review-2-approved
// and #2 at review-0-ready, each with its pull request open in app, the
// merge gate off. An event is a pass, a member's end, a maintainer's
// verb, a CI status change or a forge refusal (event). After each, the
// explorer checks that each open issue carries exactly one stage, that no
// pull request merged with a check failed or pending on its head, that
// no member launched on an issue blocked, paused or held by a `Blocked
// by #<m>`, and that every label Daisy wrote is a row of section 5's
// table (sectionFive). The first violation fails the test with the
// shortest sequence that reaches it.
func TestExploreLifecycle(t *testing.T) {
	start := time.Now()
	states, taken, bad := explore(t, *exploreDepth)
	if bad != nil {
		t.Fatalf("%s\nafter:\n%s", bad.why, numbered(bad.path))
	}
	var moves []string
	for _, m := range slices.Sorted(maps.Keys(taken)) {
		moves = append(moves, m+" ("+taken[m]+")")
	}
	t.Logf("explored %d states to depth %d in %s; Daisy's moves taken:\n  %s", states, *exploreDepth, time.Since(start).Round(time.Millisecond), strings.Join(moves, "\n  "))
	// The default bound reaches each of these, so an explorer that stops
	// launching, collecting or merging fails here instead of passing on
	// an empty search.
	for _, m := range []string{
		forge.ReviewReady + " → " + forge.ReviewInProgress,      // T21
		forge.ReviewInProgress + " → " + forge.ReviewApproved,   // T9
		forge.ReviewInProgress + " → " + forge.ChangesRequested, // T10
		forge.ReviewInProgress + " → " + forge.ReviewReady,      // T22
		forge.ReviewReady + " → +" + forge.Blocked,              // T17
		forge.ReviewApproved + " → " + forge.PlanAccepted,       // T14
		forge.ReviewApproved + " → closed",                      // T15
		forge.ReviewApproved + " → " + forge.Done,               // T15
		forge.ReviewApproved + " → " + forge.ChangesRequested,   // T26
		forge.ReviewApproved + " → +" + forge.Blocked,           // T16
		forge.ChangesRequested + " → " + forge.PlanInProgress,   // T20
		forge.PlanInProgress + " → " + forge.ReviewReady,        // T12
	} {
		if _, ok := taken[m]; !ok && *exploreDepth >= 5 {
			t.Errorf("the explorer took no %s", m)
		}
	}
}

// numbered is a counterexample's events, one per line.
func numbered(path []event) string {
	var b strings.Builder
	for i, e := range path {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, e)
	}
	return b.String()
}

// violation is the first broken invariant and the events that reached it.
type violation struct {
	why  string
	path []event
}

// explore is a breadth-first search of the event sequences up to depth,
// each state reached first by its shortest sequence and expanded once (a
// state is its fingerprint): a state of the frontier is rebuilt by
// replaying its sequence, and each child is a clone of it (world.clone)
// with one event more. It returns the states reached, each move Daisy
// made with the rows of section 5's table that make it, and the first
// violation.
func explore(t *testing.T, depth int) (states int, taken map[string]string, bad *violation) {
	t.Helper()
	seen := map[string]bool{newWorld(t).fingerprint(): true}
	taken = map[string]string{}
	frontier := [][]event{nil}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next [][]event
		for _, path := range frontier {
			parent := replay(t, path)
			parent.taken = taken
			for _, e := range parent.enabled() {
				w := parent.clone()
				w.apply(e)
				child := append(slices.Clone(path), e)
				if w.bad != "" {
					return len(seen), taken, &violation{w.bad, child}
				}
				if fp := w.fingerprint(); !seen[fp] {
					seen[fp] = true
					next = append(next, child)
				}
			}
		}
		frontier = next
	}
	return len(seen), taken, nil
}

// replay is the initial world with path applied.
func replay(t *testing.T, path []event) *world {
	t.Helper()
	w := newWorld(t)
	for _, e := range path {
		w.apply(e)
	}
	return w
}

// event is one step of the explorer: kind, the issue it is on (0 for
// none) and its argument (a role and an outcome, a CI state, an issue).
type event struct {
	kind string
	n    int
	arg  string
}

func (e event) String() string {
	switch e.kind {
	case "pass":
		return "a pass of apply"
	case "end":
		return fmt.Sprintf("#%d's %s ends: %s", e.n, strings.Fields(e.arg)[0], strings.Fields(e.arg)[1])
	case "ci":
		return fmt.Sprintf("CI reports %s on #%d's head", e.arg, e.n)
	case "block":
		return fmt.Sprintf("the maintainer blocks #%d by #%s", e.n, e.arg)
	case "refuse-merge":
		return fmt.Sprintf("the forge refuses the merge of #%d's pull request (a protection rule)", e.n)
	case "unsettled":
		return fmt.Sprintf("the forge recomputes the mergeability of #%d's pull request", e.n)
	case "settled":
		return fmt.Sprintf("the forge reports #%d's pull request mergeable", e.n)
	case "require":
		return "app protects main with required checks"
	case "refuse-relabel":
		return "the forge refuses Daisy's next relabel of an issue"
	}
	return fmt.Sprintf("the maintainer %ss #%d", e.kind, e.n)
}

// world is one explored state: a swarm over forge.Fake, runner.Fake and
// a memFS holding its clone's prompts and its <state>, its clock a
// minute on for each event, and the model's own record of what the
// maintainer did (which `Blocked by` comment holds which issue) and which
// refusals were armed.
type world struct {
	t       *testing.T
	fs      *memFS
	now     time.Time
	f       *forge.Fake // the swarm's token
	jane    *forge.Fake // the maintainer's
	run     *runner.Fake
	s       *Swarm
	out     bytes.Buffer
	heldBy  map[int]int        // issue → the issue its newest maintainer comment names, `Blocked by #<m>`
	refused map[forge.Ref]bool // the pull requests the forge refuses to merge
	require bool               // app's main requires checks
	armed   bool               // the forge refuses Daisy's next issue relabel
	used    bool               // that refusal has been armed once on this path
	bad     string             // the first violation
	taken   map[string]string  // each move Daisy made in the exploration, with its rows; nil while replaying
}

// explorerGates: the triage and plan gates on, the merge gate off, so
// that Daisy's merge step runs.
var explorerGates = declare.Gates{Triage: true, Plan: true, Merge: false}

// explorerRoles are the roles whose prompts the swarm holds.
var explorerRoles = []string{"owner", "architect", "worker", "adversary", "sympathetic"}

// newWorld is the explorer's first state: #1 approved by both reviewers
// at its head, its plan one slice; #2 ready for review, its plan two;
// each with its pull request open in app.
func newWorld(t *testing.T) *world {
	w := &world{t: t, fs: newMemFS(), now: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC), heldBy: map[int]int{}, refused: map[forge.Ref]bool{}}
	for _, role := range explorerRoles {
		p := declare.PromptFile(explorerClone, role)
		w.must(w.fs.MkdirAll(filepath.Dir(p), 0o755))
		w.must(w.fs.WriteFile(p, []byte("# "+role+"\n"), 0o644))
	}
	w.f = forge.NewFake(w.ops(), "acme/swarm-app", []string{"app"}, forge.Account{Login: "swarm-bot"})
	w.f.AddLabels(forge.Labels...)
	w.run = &runner.Fake{}
	for n, is := range []struct{ title, stage, plan string }{
		{"One", forge.ReviewApproved, "## Steps\n\n1. app: change main.go.\n"},
		{"Two", forge.ReviewReady, "## Steps\n\n1. app: change main.go.\n2. app: add notes.md.\n"},
	} {
		n++
		if _, err := w.f.CreateIssue(is.title, "Do "+is.title+".", []string{is.stage}); err != nil {
			t.Fatal(err)
		}
		w.f.Comment(forge.Ref{Repo: "swarm-app", N: n}, marker("architect", 1)+is.plan)
		m, err := w.f.CreatePull("app", fmt.Sprintf("swarm/%d-1", n), "main", fmt.Sprintf("acme/swarm-app#%d slice 1: %s", n, is.title), marker("worker", 1)+"v1\n\nRefs acme/swarm-app#"+strconv.Itoa(n))
		if err != nil {
			t.Fatal(err)
		}
		w.f.Push(forge.Ref{Repo: "app", N: m}, sha(n, 1))
		if is.stage == forge.ReviewApproved {
			for _, role := range []string{"adversary", "sympathetic"} {
				w.f.Review(forge.Ref{Repo: "app", N: m}, forge.Approve, marker(role, 1)+outputs["approve"])
			}
		}
	}
	w.attach()
	return w
}

// ops is the world's Ops: its clock, and no process: a path that reached
// git or docker fails the test.
func (w *world) ops() ops.Ops {
	return memOps{&ops.Fake{
		NowFn: func() time.Time { return w.now },
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			w.t.Errorf("the explorer ran %s %q in %s: a path it drives reached git or docker", name, args, dir)
			return nil, nil, errors.New("no process in the explorer")
		},
		StartFn: func(dir, name string, _ io.Reader, _, _ io.Writer, args ...string) (ops.Process, error) {
			w.t.Errorf("the explorer started %s %q in %s: a path it drives reached git, docker or notify", name, args, dir)
			return nil, errors.New("no process in the explorer")
		},
	}, w.fs}
}

// explorerClone is the swarm repository's clone in a world's memFS.
const explorerClone = "/clone"

// attach is the swarm over the world's forge and runner: one repository,
// app, the merge gate off, two review rounds, jane its maintainer.
func (w *world) attach() {
	w.jane = w.f.As(forge.Account{Login: "jane"})
	personas := map[string]declare.Persona{}
	for _, role := range explorerRoles {
		personas[role] = declare.Persona{Provider: "claude", Model: "m", Effort: "high"}
	}
	w.s = &Swarm{O: w.ops(), Place: declare.Clone(explorerClone), WS: explorerClone, Local: "/local", Forge: &watched{w.f, w},
		Runner: w.run, SwarmLogin: "swarm-bot", Out: &w.out, Err: &w.out, Secret: func(string) string { return "oauth-token-0123456789abcdef" },
		Cfg: declare.Config{
			Forge: declare.Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/swarm-app"},
			Repos: []declare.Repo{{Name: "app", Image: "app:dev"}}, Base: "main",
			MaxMembers: 8, MaxRounds: 2, Timeout: "10h", Trusted: []string{"jane", "swarm-bot"},
			Gates: explorerGates, Personas: personas,
		}}
}

// clone is a copy of w that shares nothing with it: its forge cloned,
// its file system copied, its model's record and running containers
// copied.
func (w *world) clone() *world {
	c := &world{t: w.t, fs: w.fs.clone(), now: w.now, heldBy: maps.Clone(w.heldBy), refused: maps.Clone(w.refused), require: w.require, armed: w.armed, used: w.used, taken: w.taken}
	c.f = w.f.Clone(c.ops())
	c.run = &runner.Fake{Others: slices.Clone(w.run.Others)}
	c.attach()
	return c
}

// sha is the head the worker's run k pushes on issue n.
func sha(n, k int) string { return fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("%d-%d", n, k)))) }

func (w *world) violate(format string, a ...any) {
	if w.bad == "" {
		w.bad = fmt.Sprintf(format, a...)
	}
}

// apply runs one event, the clock a minute on, then checks the
// invariants.
func (w *world) apply(e event) {
	w.now = w.now.Add(time.Minute)
	at := forge.Ref{Repo: "swarm-app", N: e.n}
	switch e.kind {
	case "pass":
		w.pass()
	case "end":
		f := strings.Fields(e.arg)
		w.end(e.n, f[0], f[1])
	case "pause":
		is := w.issue(e.n)
		w.must(Flag(w.jane, at, is.Labels, forge.Paused, &w.out))
	case "resume":
		is := w.issue(e.n)
		w.must(Unflag(w.jane, at, is.Labels, forge.Paused, &w.out))
	case "unblock":
		// swarm unblock (section 11): the comment where the stage's role
		// reads, then the flag cleared, then swarm:gated unless the
		// stage is gated.
		is := w.issue(e.n)
		on := at
		if stage, _ := StageOf(is); stage == forge.ChangesRequested {
			if cur, ok := w.current(e.n); ok && cur.Open {
				on = forge.Ref{Repo: cur.Repo, N: cur.Number}
			}
		}
		w.must(w.jane.Comment(on, "Go on."))
		if on == at {
			delete(w.heldBy, e.n) // a later comment of the maintainer's on the issue lifts its `Blocked by`
		}
		w.must(Unflag(w.jane, at, is.Labels, forge.Blocked, &w.out))
		w.regate(e.n)
	case "block":
		m, _ := strconv.Atoi(e.arg)
		w.must(Flag(w.jane, forge.Ref{Repo: "swarm-app", N: m}, w.issue(m).Labels, forge.Blocking, &w.out))
		w.must(w.jane.Comment(at, "Blocked by #"+e.arg))
		w.heldBy[e.n] = m
	case "accept":
		w.must(w.jane.Relabel(at, forge.PlanPosted, forge.PlanAccepted))
		w.regate(e.n)
	case "ci":
		cur, _ := w.current(e.n)
		w.f.SetStatus("app", cur.HeadSHA, "ci", e.arg)
	case "refuse-merge":
		cur, _ := w.current(e.n)
		w.f.RefuseMerge(forge.Ref{Repo: cur.Repo, N: cur.Number})
		w.refused[forge.Ref{Repo: cur.Repo, N: cur.Number}] = true
	case "unsettled", "settled":
		cur, _ := w.current(e.n)
		m := forge.MergeableUnknown
		if e.kind == "settled" {
			m = forge.MergeableYes
		}
		w.f.SetMergeable(forge.Ref{Repo: cur.Repo, N: cur.Number}, m)
	case "require":
		w.f.RequireChecks("app", "main")
		w.require = true
	case "refuse-relabel":
		w.armed, w.used = true, true
	default:
		w.t.Fatalf("no event %q", e.kind)
	}
	w.check()
}

func (w *world) must(err error) {
	if err != nil {
		w.t.Fatalf("explorer: %v\n%s", err, w.out.String())
	}
}

func (w *world) issue(n int) forge.Issue {
	is, err := w.f.Issue(n)
	if err != nil {
		w.t.Fatal(err)
	}
	return is
}

// current is issue n's current pull request, ok when it has one.
func (w *world) current(n int) (forge.Pull, bool) {
	ps, err := w.f.Pulls(n, Openers("swarm-bot", w.s.Cfg.Trusted))
	if err != nil {
		w.t.Fatal(err)
	}
	if len(ps) == 0 {
		return forge.Pull{}, false
	}
	return ps[len(ps)-1], true
}

// regate is the verbs' last write (section 11): swarm:gated cleared from
// issue n unless its labels are gated.
func (w *world) regate(n int) {
	is := w.issue(n)
	if !Gated(is.Labels, w.s.Cfg.Gates, ClassOf(is, "swarm-bot", w.s.Cfg.Trusted)) {
		w.must(Unflag(w.jane, forge.Ref{Repo: "swarm-app", N: n}, is.Labels, forge.Gated, &w.out))
	}
}

// pass is Pass's steps but the launch's preparation (section 11): (2)
// collectAll, (5) the stages with the role rule, each launch through
// world.launch, (6) the merge step, (8) the flag and the labels copied to
// the pull requests. The owner's batch, the milestones, notify, drift
// and the journal's push are left out: they move no stage of an open
// issue but by creating one, and they reach git.
func (w *world) pass() {
	s := w.s
	if s.live == nil {
		s.live, s.given = map[string]*live{}, map[string][]string{}
	}
	s.collected, s.started, s.moves, s.fetched = 0, 0, map[int]string{}, false
	running, err := s.Runner.Running()
	if err != nil {
		w.t.Fatal(err)
	}
	runs, err := Runs(w.fs, s.state())
	w.must(err)
	for _, r := range runs {
		if lines, _ := r.Result(w.fs); r.Role == "worker" && Ended(lines) != "" && !slices.Contains(lines, "collected") {
			w.t.Fatalf("%s ended uncollected: its collection is world.end's, the real one reaching git", r.Dir)
		}
	}
	errs := []error{s.collectAll(running)}
	if s.paused() != "" {
		errs = append(errs, s.stages(running, nil))
	} else if err := s.stages(running, w.launch); err != nil {
		errs = append(errs, err)
	} else if !s.Cfg.Gates.Merge {
		errs = append(errs, s.mergeStep())
	}
	s.flags()
	s.copyMoved()
	if err := errors.Join(errs...); err != nil && !errors.Is(err, errRelabelRefused) {
		w.t.Fatalf("pass: %v\n%s", err, w.out.String())
	}
}

// launch stands in for Swarm.launch past its preparation (section 7.1),
// which clones and so reaches git: the run's directory and the result's
// lines a launch writes, the container running on the runner (runner.Fake's
// Others, so that it runs until the explorer ends it), then the
// in-progress stage written by Swarm.inProgress, the launch's own. It
// first checks rule 1: nothing launches on an issue blocked, paused or
// held by a `Blocked by #<m>` naming an open issue.
func (w *world) launch(j *Job) error {
	s := w.s
	is := w.issue(j.N)
	for _, held := range []string{forge.Blocked, forge.Paused} {
		if slices.Contains(is.Labels, held) {
			w.violate("the %s launched on #%d, which carries %s", j.Role, j.N, held)
		}
	}
	if m := w.heldBy[j.N]; m != 0 && w.issue(m).Open {
		w.violate("the %s launched on #%d, held by the maintainer's Blocked by #%d", j.Role, j.N, m)
	}
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-%s-%d", j.N, j.Role, j.K)), N: j.N, K: j.K, Role: j.Role}
	if err := w.fs.MkdirAll(filepath.Join(r.Dir, "io"), 0o755); err != nil {
		return err
	}
	var lines, round []string
	switch j.Role {
	case "worker":
		sl, name, _ := j.WorkerSlice()
		lines = []string{"start " + sha(0, 0), fmt.Sprintf("branch swarm/%d-%d in %s", j.N, sl, name)}
	case "adversary", "sympathetic":
		cur, _ := j.current()
		rd, _ := j.round(cur)
		lines, round = []string{"start " + cur.HeadSHA}, []string{"round " + strconv.Itoa(rd)}
	default:
		lines = []string{"start app " + sha(0, 0)}
	}
	lines = append(lines, "label "+j.Label)
	if j.Question {
		lines = append(lines, "question")
	}
	lines = append(append(lines, "started "+stamp(s.O.Now()), "credential claude env CLAUDE_CODE_OAUTH_TOKEN", "persona claude m high"), round...)
	if err := w.fs.WriteFile(filepath.Join(r.Dir, "argv"), []byte("docker\nsh\n-c\n"+exitScript+"\n"), 0o644); err != nil {
		return err
	}
	if err := r.Append(s.O.FS(), lines...); err != nil {
		return err
	}
	w.run.Others = append(w.run.Others, runner.Running{Name: s.containerName(j.N, j.Role, j.K), Swarm: fmt.Sprintf("%s#%d/%s", s.Cfg.Forge.Repo, j.N, j.Role)})
	return s.inProgress(j, r)
}

// outputs is what a member's run writes to out.md, by its outcome.
var outputs = map[string]string{
	"approve": "VERDICT: approve\n\nFine.\n",
	"changes": "VERDICT: request-changes\n\n[blocking] main.go:1 says v1; the plan wants v2.\n",
	"blocked": "BLOCKED: the plan names no test.\n",
	"plan":    "## Steps\n\n1. app: change main.go again.\n",
}

// outcomes is each role's ends the explorer takes: the reviewers'
// verdicts and a block, the architect's revised plan, the worker's push.
var outcomes = map[string][]string{
	"adversary":   {"approve", "changes", "blocked"},
	"sympathetic": {"approve", "changes", "blocked"},
	"architect":   {"plan"},
	"worker":      {"push"},
}

// end is a member's container ending (section 7.7, ended): out.md and
// the exit status written as the container's sh writes them, the
// container gone, so the next pass collects the run. The worker's run is
// collected here, as far as its collection goes without git (section
// 7.6): its head pushed to its branch's pull request, opened when it has
// none, then T8 or T12 by the collection's own relabel, and the run
// recorded and collected. That stand-in is never refused (an armed
// refusal waits for the next relabel): a worker's collection retried at a
// pass would be the real one, which reaches git.
func (w *world) end(n int, role, outcome string) {
	s := w.s
	k := 0
	for i, c := range w.run.Others {
		if cn, crole, ck := container(c); cn == n && crole == role {
			k = ck
			w.run.Others = slices.Delete(w.run.Others, i, i+1)
			break
		}
	}
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-%s-%d", n, role, k)), N: n, K: k, Role: role}
	if role != "worker" {
		w.must(w.fs.WriteFile(filepath.Join(r.Dir, "io", "out.md"), []byte(outputs[outcome]), 0o644))
		w.must(w.fs.WriteFile(filepath.Join(r.Dir, "io", "exit"), []byte("0\n"), 0o644))
		return
	}
	fs := s.O.FS()
	w.must(r.Append(fs, "ended "+endStamp(s.O.Now())))
	lines, err := r.Result(fs)
	w.must(err)
	branch, head := runBranch(lines), sha(n, k)
	ps, err := s.pulls(n)
	w.must(err)
	pull := slices.IndexFunc(ps, func(p forge.Pull) bool { return p.Open && p.Head == branch })
	if pull >= 0 {
		ref := forge.Ref{Repo: ps[pull].Repo, N: ps[pull].Number}
		w.f.Push(ref, head)
		w.must(w.f.Comment(ref, marker("worker", k)+"Fixed."))
	} else {
		m, err := w.f.CreatePull("app", branch, "main", fmt.Sprintf("acme/swarm-app#%d slice %s: %s", n, strings.TrimPrefix(branch, fmt.Sprintf("swarm/%d-", n)), w.issue(n).Title), marker("worker", k)+"Built.\n\nRefs acme/swarm-app#"+strconv.Itoa(n))
		w.must(err)
		w.f.Push(forge.Ref{Repo: "app", N: m}, head)
	}
	label, _ := Has(lines, "label")
	c := &collection{s: s, r: r, lines: lines, head: runHead(r), label: strings.TrimPrefix(label, "label "), at: forge.Ref{Repo: "swarm-app", N: n}}
	if c.is, err = s.Forge.Issue(n); err != nil {
		w.t.Fatal(err)
	}
	armed := w.armed
	w.armed = false
	w.must(c.relabel(forge.ReviewReady))
	w.armed = armed
	w.must(c.end("posted"))
}

// container is the issue, role and run of a member's container, by its
// name (Swarm.containerName): `daisy-swarm.<owner>.<repo>.<n>.<role>.<k>`.
func container(c runner.Running) (n int, role string, k int) {
	f := strings.Split(c.Name, ".")
	n, _ = strconv.Atoi(f[3])
	k, _ = strconv.Atoi(f[5])
	return n, f[4], k
}

// enabled is every event that can happen next in w.
func (w *world) enabled() []event {
	es := []event{{kind: "pass"}}
	for _, c := range w.run.Others {
		n, role, _ := container(c)
		for _, o := range outcomes[role] {
			es = append(es, event{"end", n, role + " " + o})
		}
	}
	open := map[int]forge.Issue{}
	for n := 1; n <= 2; n++ {
		if is := w.issue(n); is.Open {
			open[n] = is
		}
	}
	for n := 1; n <= 2; n++ {
		is, ok := open[n]
		if !ok {
			continue
		}
		stage, _ := StageOf(is)
		switch {
		case slices.Contains(is.Labels, forge.Paused):
			es = append(es, event{kind: "resume", n: n})
		default:
			es = append(es, event{kind: "pause", n: n})
		}
		if slices.Contains(is.Labels, forge.Blocked) && !((stage == forge.ReviewInProgress || stage == forge.ReviewReady) && AtTheRoundLimit(is, "swarm-bot")) {
			es = append(es, event{kind: "unblock", n: n})
		}
		if _, other := open[3-n]; other && w.heldBy[n] == 0 {
			es = append(es, event{"block", n, strconv.Itoa(3 - n)})
		}
		if stage == forge.PlanPosted {
			es = append(es, event{kind: "accept", n: n})
		}
		cur, ok := w.current(n)
		if !ok || !cur.Open {
			continue
		}
		checks, _ := w.f.Status("app", cur.HeadSHA)
		switch {
		case len(checks) == 0:
			es = append(es, event{"ci", n, "pending"})
		case checks[0].State == "pending":
			es = append(es, event{"ci", n, "success"}, event{"ci", n, "failure"})
		}
		if !w.refused[forge.Ref{Repo: cur.Repo, N: cur.Number}] {
			es = append(es, event{kind: "refuse-merge", n: n})
		}
		if cur.Mergeable == forge.MergeableYes {
			es = append(es, event{kind: "unsettled", n: n})
		} else {
			es = append(es, event{kind: "settled", n: n})
		}
	}
	if !w.require {
		es = append(es, event{kind: "require"})
	}
	if !w.used {
		es = append(es, event{kind: "refuse-relabel"})
	}
	return es
}

// check is the invariant of the labels, after each event and each of
// Daisy's relabels: each open issue carries exactly one stage. A merge's
// checks and a launch's holds are checked as they are made
// (watched.Merge, world.launch), and each label Daisy writes as it
// writes it (watched.Relabel).
func (w *world) check() {
	for n := 1; n <= 2; n++ {
		is := w.issue(n)
		if !is.Open {
			continue
		}
		var stages []string
		for _, l := range is.Labels {
			if forge.IsStage(l) {
				stages = append(stages, l)
			}
		}
		if len(stages) != 1 {
			w.violate("#%d carries %d stages: %q", n, len(stages), is.Labels)
		}
	}
}

// fingerprint is the state as the code under exploration can read it:
// the issues and pull requests with their labels, comments, reviews and
// checks, every file of the world, the containers running and the
// model's own record; ids left out and every time replaced by its rank
// among the state's times, so that two states differing only in when
// things happened, in the same order, are one. It is a digest: the
// explorer holds one for every state it reached.
func (w *world) fingerprint() string {
	var b strings.Builder
	put := func(fields ...string) {
		for _, f := range fields {
			b.WriteString(f)
			b.WriteByte(0x1f)
		}
		b.WriteByte(0x1e)
	}
	t := func(x time.Time) string { return x.UTC().Format(time.RFC3339Nano) }
	for n := 1; n <= 2; n++ {
		is := w.issue(n)
		put("issue", strconv.Itoa(n), strconv.FormatBool(is.Open), t(is.Closed), is.Body, strings.Join(slices.Sorted(slices.Values(is.Labels)), ","))
		for _, c := range is.Comments {
			put("comment", c.Author, t(c.Created), c.Body)
		}
		ps, _ := w.f.Pulls(n, Openers("swarm-bot", w.s.Cfg.Trusted))
		for _, p := range ps {
			put("pull", p.Repo, strconv.Itoa(p.Number), p.Title, p.Body, p.Head, p.HeadSHA, strconv.FormatBool(p.Open), strconv.FormatBool(p.Merged), strconv.Itoa(int(p.Mergeable)), strings.Join(slices.Sorted(slices.Values(p.Labels)), ","))
			ref := forge.Ref{Repo: p.Repo, N: p.Number}
			cs, _ := w.f.PullComments(ref)
			for _, c := range cs {
				put("comment", c.Author, t(c.Created), c.Body)
			}
			rs, _ := w.f.Reviews(ref)
			for _, r := range rs {
				put("review", r.Author, t(r.Submitted), strconv.Itoa(int(r.State)), r.Commit, r.Body)
			}
			checks, _ := w.f.Status(p.Repo, p.HeadSHA)
			for _, c := range checks {
				put("check", c.Context, c.State)
			}
		}
	}
	for _, p := range slices.Sorted(maps.Keys(w.fs.files)) {
		put("file", p, string(w.fs.files[p]))
	}
	for _, c := range w.run.Others {
		put("running", c.Name)
	}
	put("model", fmt.Sprint(w.heldBy, w.refused, w.require, w.armed, w.used))
	sum := sha256.Sum256([]byte(ranked(b.String())))
	return string(sum[:])
}

// ranked is s with each time replaced by its rank among s's times: the
// clock's stamps (`2026-11-16T09:01:00Z`, RFC 3339) and the journal's
// (`2026-11-16 - 0901`), the clock never leaving the day it starts on.
func ranked(s string) string {
	const day = "2026-11-16"
	var found []string
	for _, head := range []string{day + "T", day + " - "} {
		for i := 0; ; {
			j := strings.Index(s[i:], head)
			if j < 0 {
				break
			}
			j += i
			end := j + len(head)
			for end < len(s) && strings.IndexByte("0123456789:.", s[end]) >= 0 {
				end++
			}
			if strings.HasSuffix(head, "T") && end < len(s) && s[end] == 'Z' {
				end++
			}
			found, i = append(found, s[j:end]), end
		}
	}
	all := slices.Compact(slices.Sorted(slices.Values(found)))
	pairs := make([]string, 0, 2*len(all))
	for i, x := range all {
		pairs = append(pairs, x, "t"+strconv.Itoa(i))
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// take is allowed, its moves recorded in the exploration's.
func (w *world) take(before, after forge.Issue) string {
	taken, why := w.allowed(before, after)
	if w.taken != nil {
		maps.Copy(w.taken, taken)
	}
	return why
}

// errRelabelRefused is the forge's refusal the explorer arms.
var errRelabelRefused = errors.New("forge: POST /repos/acme/swarm-app/issues/labels: 500 refused by the explorer")

// watched is the swarm's forge as the explorer watches Daisy's writes
// to it: each relabel of an issue checked against section 5's table as
// it is made, a refusal armed by the explorer answered, and each merge
// checked against the head's checks.
type watched struct {
	*forge.Fake
	w *world
}

func (f *watched) Relabel(at forge.Ref, remove, add string) error {
	w := f.w
	if at.Repo != "swarm-app" {
		return f.Fake.Relabel(at, remove, add) // the copy to a pull request, which no rule reads
	}
	if w.armed {
		w.armed = false
		return errRelabelRefused
	}
	before := w.issue(at.N)
	if err := f.Fake.Relabel(at, remove, add); err != nil {
		return err
	}
	w.check()
	after := w.issue(at.N)
	if why := w.take(before, after); why != "" {
		w.violate("Daisy relabelled #%d %q → %q: %s", at.N, before.Labels, after.Labels, why)
	}
	return nil
}

func (f *watched) CloseIssue(n int) error {
	w := f.w
	before := w.issue(n)
	if err := f.Fake.CloseIssue(n); err != nil {
		return err
	}
	if why := w.take(before, w.issue(n)); why != "" {
		w.violate("Daisy closed #%d at %q: %s", n, before.Labels, why)
	}
	return nil
}

// Merge checks the merged head's checks: none failed or pending (section
// 12, The checks).
func (f *watched) Merge(pr forge.Ref, head string) error {
	if err := f.Fake.Merge(pr, head); err != nil {
		return err
	}
	checks, _ := f.Fake.Status(pr.Repo, head)
	for _, c := range checks {
		if c.State != "success" {
			f.w.violate("Daisy merged %s#%d at %s with its check %s %s", pr.Repo, pr.N, head[:7], c.Context, c.State)
		}
	}
	return nil
}
