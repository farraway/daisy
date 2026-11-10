package commands

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

func prRef(m int) forge.Ref { return forge.Ref{Repo: "app", N: m} }

// verb runs a swarm verb as the cli does, from the folder the swarm is
// found from, with apply's clock and $DAISY_ROOT and $DAISY_LOCAL the
// fixture's (no other variable).
func (a *applyWS) verb(fn func(*Context) error) error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{
		NowFn:   fixedClock().Now,
		GetwdFn: func() (string, error) { return a.cwd, nil },
		GetenvFn: func(k string) string {
			return map[string]string{"DAISY_ROOT": a.root, "DAISY_LOCAL": a.local}[k]
		},
	}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr}
	return fn(c)
}

func (a *applyWS) resume() {
	a.t.Helper()
	if err := a.verb(SwarmResume); err != nil {
		a.t.Fatal(err)
	}
}

// A job across two repositories (section 13, Step 29's third row): slice
// 1 in the second declared repository, slice 2 in the first; each built
// in its repository's mirror, clone, image, check and protected, its pull
// request opened there with the Refs form for its repository; Pulls(n)
// finds both; slice 2 after slice 1's merge, with its repository and merge
// commit under Earlier pull requests; the architect's clones at
// /work/<name> in the first repository's image.
func TestSwarmApplyTwoRepositories(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Repos[1].Image, c.Repos[1].Check, c.Repos[1].Protected = "daisy-member-lib:1", []string{"go", "test", "./..."}, []string{"go.sum"}
	})
	plan := "## Steps\n\n1. lib: add the check.\n2. app: move the pin to slice 1's merge.\n"
	a.planned(plan)
	arch := a.runner.Launches()[0]
	if arch.Workdir != "/work/app" || arch.Image != "daisy-member:1" || !strings.Contains(a.result("1-architect-1"), "start app ") || !strings.Contains(a.result("1-architect-1"), "\nstart lib ") {
		t.Errorf("architect: %+v\n%s", arch, a.result("1-architect-1"))
	}
	a.runner.Script(1, "worker", work("lib v1"))
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	ls := a.runner.Launches()
	w, check := ls[1], ls[2]
	if w.Image != "daisy-member-lib:1" || w.Workdir != "/work" || check.Image != "daisy-member-lib:1" || !reflect.DeepEqual(check.Argv, []string{"go", "test", "./..."}) {
		t.Errorf("slice 1's specs: %+v %+v", w, check)
	}
	if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, "in acme/lib, on the branch checked out in /work (swarm/1-1)") || !strings.Contains(sys, "These paths are protected: go.sum.") {
		t.Errorf("slice 1's Task:\n%s", sys)
	}
	ps := a.pulls(1)
	if len(ps) != 1 || ps[0].Repo != "lib" || ps[0].Number != 1 || ps[0].Title != "acme/app#1 slice 1: Drift check" || !strings.HasSuffix(ps[0].Body, "\n\nRefs acme/app#1") {
		t.Fatalf("slice 1's pull request: %+v", ps)
	}
	if _, _, err := ops.Real().Exec(a.upstream("lib"), "git", "rev-parse", "--verify", "refs/heads/swarm/1-1"); err != nil {
		t.Errorf("not pushed to lib: %v", err)
	}
	a.pass()
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
		t.Fatalf("slice 1 not approved: %v", got)
	}
	a.forge.Merge(forge.Ref{Repo: "lib", N: 1}, a.pulls(1)[0].HeadSHA)
	a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	a.runner.Script(1, "worker", work("app v2"))
	a.pass()
	ps = a.pulls(1)
	if len(ps) != 2 || ps[1].Repo != "app" || ps[1].Number != 2 || ps[1].Head != "swarm/1-2" || !strings.HasSuffix(ps[1].Body, "\n\nRefs #1") {
		t.Fatalf("Pulls(1): %+v", ps)
	}
	if p := a.file("1-worker-2", "io/prompt.md"); !strings.Contains(p, "# Earlier pull requests\n\n- lib#1 swarm/1-1 merged "+ps[0].MergeSHA+": acme/app#1 slice 1: Drift check\n") {
		t.Errorf("slice 2's prompt:\n%s", p)
	}
	if sys := a.file("1-worker-2", "io/system.md"); !strings.Contains(sys, "A pin to an earlier slice's change names its merged commit") {
		t.Errorf("slice 2's Task:\n%s", sys)
	}
}

// Bad slices (section 7.5) with two repositories, and what one repository
// allows.
func TestSwarmApplyBadSlices(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		one        bool   // only app declared
		removeLib  bool   // lib removed from repos after the plan
		bad        string // "" when the worker builds in app
	}{
		{name: "an undeclared repository", plan: "## Steps\n\n1. other: x.\n", bad: "1"},
		{name: "a slice naming none", plan: "## Steps\n\n1. app: x.\n2. y.\n", bad: "2"},
		{name: "no slice lines", plan: "## Research\n\nx\n", bad: "1"},
		{name: "a slice's repository removed", plan: "## Steps\n\n1. lib: x.\n", removeLib: true, bad: "1"},
		{name: "one repository: a slice naming none", plan: "## Steps\n\n1. x.\n", one: true},
		{name: "one repository: no slice lines", plan: "## Research\n\nx\n", one: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			if tc.one {
				a.edit(func(c *declare.Config) { c.Repos = c.Repos[:1] })
			}
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: tc.plan})
			a.pass()
			if tc.removeLib {
				a.edit(func(c *declare.Config) { c.Repos = c.Repos[:1] })
			}
			if tc.bad != "" && !tc.removeLib {
				if b := a.bodies(1); !strings.HasSuffix(b[len(b)-1], "architect run 1 failed: bad slices: "+tc.bad+" (at swarm:proposal-accepted). Remove swarm:blocked to resume.") {
					t.Errorf("architect: %q", b)
				}
				return
			}
			a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
			a.runner.Script(1, "worker", work("v1"))
			a.pass()
			if tc.bad != "" {
				b := a.bodies(1)
				if !strings.HasSuffix(b[len(b)-1], "worker run 1 failed: bad slices: "+tc.bad+" (at swarm:plan-1-accepted). Remove swarm:blocked to resume.") ||
					!strings.Contains(a.result("1-worker-1"), "launch-failed ") || len(a.runner.Launches()) != 1 {
					t.Errorf("worker: %q\n%s", b, a.result("1-worker-1"))
				}
				return
			}
			if ps := a.pulls(1); len(ps) != 1 || ps[0].Repo != "app" {
				t.Errorf("one repository: %+v\n%s", ps, a.result("1-worker-1"))
			}
		})
	}
}

// A worker continuing its open pull request whose repository was removed
// from repos since (section 7.1, step 2): the plan still names it for the
// slice, so only the launch's guard stops it: bad slices, nothing
// launched, T17 at the stage it was launched on.
func TestSwarmApplyContinuedPullRepositoryRemoved(t *testing.T) {
	a := newApplyWS(t)
	a.inReview("## Steps\n\n1. lib: change main.go.\n")
	if ps := a.pulls(1); len(ps) != 1 || ps[0].Repo != "lib" {
		t.Fatalf("pulls %+v", ps)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := labels(a.issue(1)); !slices.Contains(got, forge.ChangesRequested) {
		t.Fatalf("not sent back: %v\n%s", got, a.stdout.String())
	}
	a.edit(func(c *declare.Config) { c.Repos = c.Repos[:1] })
	launched := len(a.runner.Launches())
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	r := a.result("1-worker-2")
	b := a.bodies(1)
	if !strings.Contains(r, "\nlaunch-failed ") || !strings.Contains(r, "\nfailed: bad slices: 1\n") || len(a.runner.Launches()) != launched ||
		!strings.HasSuffix(b[len(b)-1], "worker run 2 failed: bad slices: 1 (at "+forge.ChangesRequested+"). Remove swarm:blocked to resume.") {
		t.Errorf("%s\n%q\nstdout:\n%s", r, b, a.stdout.String())
	}
}

// pause and resume (section 11): pause mid-lifecycle stops launches and
// merges while collection goes on; resume continues; both report what
// they found.
func TestSwarmPauseAndResume(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.release()
	if err := a.verb(SwarmPause); err != nil || a.stdout.String() != "Paused: no new member will launch ("+a.state("paused")+"); running members finish and are posted\n" {
		t.Fatalf("pause: %v %q", err, a.stdout.String())
	}
	if err := a.verb(SwarmPause); err != nil || a.stdout.String() != "Already paused since 2026-11-06T15:00:00Z\n" {
		t.Errorf("pause again: %v %q", err, a.stdout.String())
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if len(a.runner.Launches()) != 1 || !strings.Contains(a.stdout.String(), "Paused since 2026-11-06T15:00:00Z ("+a.state("paused")+"): launching nothing\n") {
		t.Fatalf("paused pass:\n%s", a.stdout.String())
	}
	a.resume()
	if a.stdout.String() != "Resumed: members launch at the next pass\n" {
		t.Errorf("resume: %q", a.stdout.String())
	}
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
		t.Errorf("after resume: %v", got)
	}
	a.release() // the batch, unscripted, held itself in that pass
	a.resume()
	if a.stdout.String() != "Not paused\n" {
		t.Errorf("resume again: %q", a.stdout.String())
	}
}

// The lock and the labels (section 11, apply's start): EnsureLabels runs
// after the lock, creating a deleted label again; after RefuseLabels and
// a deleted label apply exits 1 before any pass. On a forge holding the
// eleven labels of Step 14 under their old names, the first apply renames
// the seven old stage names in place and creates the five new labels
// (section 14, item 74): every issue, open or closed, reads its new
// stage, and the pass works on the new names.
func TestSwarmApplyEnsuresTheLabels(t *testing.T) {
	a := newApplyWS(t)
	f := forge.NewFake(fixedClock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	f.AddLabels(forge.Labels[1:]...)
	if err := a.apply(f, "--once"); err != nil || !reflect.DeepEqual(f.Labels()[len(forge.Labels)-1], "swarm:proposal-posted") ||
		!strings.HasPrefix(a.stdout.String(), "✓ Labels: 16 present (1 created, 0 renamed)\n") {
		t.Errorf("%v %v\n%s", err, f.Labels(), a.stdout.String())
	}
	if err := a.apply(f, "--once"); err != nil || strings.Contains(a.stdout.String(), "Labels:") {
		t.Errorf("nothing to make, nothing said: %v\n%s", err, a.stdout.String())
	}
	g := forge.NewFake(fixedClock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	g.AddLabels(forge.Labels[1:]...)
	g.RefuseLabels()
	a.release()
	err := a.apply(g, "--once")
	if err == nil || err.Error() != "Error: forge: EnsureLabels: acme's labels missing: swarm:proposal-posted; an owner of acme makes them once" || strings.Contains(a.stdout.String(), "Pass ") {
		t.Errorf("refused: %v\n%s", err, a.stdout.String())
	}

	old := forge.NewFake(fixedClock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	old.AddLabels("swarm:proposed", "swarm:ready", "swarm:planning", "swarm:planned", "swarm:in-review", "swarm:reviewed", "swarm:approved", "swarm:blocked", "swarm:gated", "swarm:done", "swarm:rejected")
	proposed, _ := old.CreateIssue("p", "", []string{"swarm:proposed", "swarm:gated"})
	approved, _ := old.CreateIssue("a", "", []string{"swarm:approved"})
	done, _ := old.CreateIssue("d", "", []string{"swarm:done"})
	old.CloseIssue(done)
	reviewed, _ := old.CreateIssue("r", "", []string{"swarm:reviewed"})
	old.CloseIssue(reviewed)
	a.release()
	if err := a.apply(old, "--once"); err != nil || !strings.HasPrefix(a.stdout.String(), "✓ Labels: 16 present (5 created, 7 renamed)\n") {
		t.Fatalf("the migration: %v\n%s", err, a.stdout.String())
	}
	for _, tc := range []struct {
		n    int
		want string
	}{{proposed, "swarm:proposal-posted"}, {approved, "swarm:review-2-approved"}, {done, "swarm:done"}, {reviewed, "swarm:review-3-changes-requested"}} {
		is, _ := old.Issue(tc.n)
		if !slices.Contains(is.Labels, tc.want) || slices.ContainsFunc(is.Labels, func(l string) bool { _, was := forge.Renamed[l]; return was }) {
			t.Errorf("#%d after the migration: %v, want %s", tc.n, is.Labels, tc.want)
		}
	}
	if strings.Contains(a.stdout.String(), "gated cleared") {
		t.Errorf("the flag moved on a renamed stage:\n%s", a.stdout.String())
	}
}

// gated is the runner whose containers end only once open is closed.
type gated struct {
	*runner.Fake
	open chan struct{}
}

func (g gated) Start(s runner.Spec, d time.Duration) (runner.Container, error) {
	c, err := g.Fake.Start(s, d)
	return gatedRun{c, g.open}, err
}

type gatedRun struct {
	runner.Container
	open chan struct{}
}

func (r gatedRun) Wait() (runner.Exit, error) { <-r.open; return r.Container.Wait() }

// Two `apply --once` at once (section 11, Lock): the role launches once,
// the second exits 1 naming the holder before reading the forge.
func TestSwarmApplyConcurrently(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan}, runner.Canned{Out: archPlan})
	open := make(chan struct{})
	first := make(chan error)
	var out1, err1b bytes.Buffer
	go func() { first <- a.applyTo(a.forge, gated{a.runner, open}, &out1, &err1b, "--once") }()
	for len(a.runner.Launches()) == 0 {
		time.Sleep(time.Millisecond)
	}
	second := &countingRuns{Fake: a.runner}
	err := a.applyTo(readsNothing{t: t}, second, &bytes.Buffer{}, &bytes.Buffer{}, "--once")
	close(open)
	if err1 := <-first; err1 != nil {
		t.Fatal(err1)
	}
	if err == nil || !strings.HasPrefix(err.Error(), "Error: swarm apply is already running for acme/app (apply --once since ") {
		t.Errorf("second: %v", err)
	}
	if n := len(a.runner.Launches()); n != 1 {
		t.Errorf("launched %d", n)
	}
	if second.ps != 0 {
		t.Errorf("the second read docker ps %d times before refusing", second.ps)
	}
}

// readsNothing is a forge any call to which fails the test (Login with an
// error, any other method with a panic): an apply that finds the lock held
// reads nothing from the forge.
type readsNothing struct {
	forge.Adapter
	t *testing.T
}

func (r readsNothing) Login() (forge.Account, error) {
	r.t.Error("Login() with the lock held")
	return forge.Account{}, errors.New("read with the lock held")
}

// countingRuns counts the reads of the running set.
type countingRuns struct {
	*runner.Fake
	ps int
}

func (c *countingRuns) Running() ([]runner.Running, error) { c.ps++; return c.Fake.Running() }

// Without --once (section 11): a pass every interval until a signal, then
// the running members collected and exit 0; a second signal during that
// exits at once with ErrInterrupted (130).
func TestSwarmApplyLoop(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	waits := 0
	var mu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		waits++
		c := make(chan time.Time, 1)
		if waits == 1 {
			c <- time.Time{}
		} else {
			a.signals <- os.Interrupt
		}
		return c
	}
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(a.stdout.String(), "Pass "); n != 2 {
		t.Errorf("%d passes:\n%s", n, a.stdout.String())
	}

	c := newApplyWS(t)
	c.signals = make(chan os.Signal, 2)
	c.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	c.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	open := make(chan struct{})
	c.after = func(time.Duration) <-chan time.Time {
		c.signals <- os.Interrupt
		c.signals <- os.Interrupt
		return nil
	}
	err := c.applyTo(c.git(), gated{c.runner, open}, &c.stdout, &c.stderr)
	close(open)
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("second signal: %v", err)
	}
	if r := c.result("1-architect-1"); strings.Contains(r, "collected") {
		t.Errorf("collected after the second signal:\n%s", r)
	}
}

// held is the runner whose containers named in hold run until their
// channel is closed; the others end at once.
type held struct {
	*runner.Fake
	hold map[string]chan struct{}
}

func (h held) Start(s runner.Spec, d time.Duration) (runner.Container, error) {
	c, err := h.Fake.Start(s, d)
	if ch := h.hold[s.Name]; ch != nil && err == nil {
		return gatedRun{c, ch}, nil
	}
	return c, err
}

// The loop runs a pass every interval whatever runs (section 11): #1's
// architect outlives two intervals; the pass after it starts launches
// #2's, proposed meanwhile, and the pass after that collects #2's; the
// first pass after #1's member ends collects it, its comment posted and
// its label moved once; the drain after the signal finds nothing left.
func TestSwarmApplyLoopCollectsALaterPass(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Long", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	const long = "daisy-swarm.acme.app.1.architect.1"
	gate := make(chan struct{})
	var once sync.Once
	var forced atomic.Bool
	end := func() { once.Do(func() { close(gate) }) }
	defer end()
	guard := time.AfterFunc(10*time.Second, func() { forced.Store(true); end() }) // a pass waiting for #1 would never reach the interval
	defer guard.Stop()
	waits := 0
	a.after = func(time.Duration) <-chan time.Time {
		waits++
		a.tick()
		switch waits {
		case 1:
			a.forge.CreateIssue("Short", "y", []string{"swarm:proposal-accepted"})
		case 3:
			end()
			for running, _ := a.runner.Running(); slices.ContainsFunc(running, func(c runner.Running) bool { return c.Name == long }); running, _ = a.runner.Running() {
				time.Sleep(time.Millisecond)
			}
		case 5:
			a.signals <- os.Interrupt
			return make(chan time.Time)
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	err := a.applyTo(a.git(), held{a.runner, map[string]chan struct{}{long: gate}}, &a.stdout, &a.stderr)
	lines := strings.Split(a.stdout.String(), "\n")
	var passes []int
	for i, l := range lines {
		if strings.HasPrefix(l, "Pass ") {
			passes = append(passes, i)
		}
	}
	at := func(line string) int { return slices.Index(lines, line) }
	if err != nil || forced.Load() || len(passes) != 5 {
		t.Fatalf("%v, forced %v, %d passes:\n%s", err, forced.Load(), len(passes), a.stdout.String())
	}
	if l := at("#2 architect run 1: launched"); l < passes[0] || l > passes[1] || !strings.HasSuffix(lines[passes[1]], "launched 1, collected 0, running 1") {
		t.Errorf("#2 not launched by pass 2 while #1 ran:\n%s", a.stdout.String())
	}
	if l := at("#2 architect run 1: posted comment"); l < passes[1] || l > passes[2] {
		t.Errorf("#2 not collected by pass 3:\n%s", a.stdout.String())
	}
	if l := at("#1 architect run 1: posted comment"); l < passes[2] || l > passes[4] {
		t.Errorf("#1 not collected by a pass after it ended:\n%s", a.stdout.String())
	}
	moved := 0
	for _, e := range a.forge.Events(issue1) {
		if e.What == "labeled swarm:plan-0-posted" {
			moved++
		}
	}
	if b := a.bodies(1); len(b) != 1 || b[0] != mark("architect", 1)+archPlan || moved != 1 || len(a.runner.Launches()) != 2 ||
		!strings.HasSuffix(a.result("1-architect-1"), "outcome posted\ncollected\n") {
		t.Errorf("comments %q, moved %d, launched %d\n%s", b, moved, len(a.runner.Launches()), a.result("1-architect-1"))
	}
}

// signalling is the runner that sends the loop a signal at its first launch.
type signalling struct {
	*runner.Fake
	signals chan os.Signal
}

func (r signalling) Start(s runner.Spec, d time.Duration) (runner.Container, error) {
	if len(r.Fake.Launches()) == 0 {
		r.signals <- os.Interrupt
	}
	return r.Fake.Start(s, d)
}

// A signal during a pass ends that pass's launching: the member launched
// before it is collected, nothing launches after it, the drain starts
// without waiting out the interval, and the exit is 0.
func TestSwarmApplyLoopSignalledDuringAPass(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	waited := false
	a.after = func(time.Duration) <-chan time.Time {
		waited = true
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	for _, n := range []int{1, 2} {
		a.forge.CreateIssue("Job", "x", []string{"swarm:proposal-accepted"})
		a.runner.Script(n, "architect", runner.Canned{Out: archPlan})
	}
	err := a.applyTo(a.git(), signalling{a.runner, a.signals}, &a.stdout, &a.stderr)
	if err != nil || waited || len(a.runner.Launches()) != 1 || !strings.HasSuffix(a.result("1-architect-1"), "outcome posted\ncollected\n") {
		t.Errorf("%v: waited %v, launched %d\n%s", err, waited, len(a.runner.Launches()), a.stdout.String())
	}
}

// signalledAtPs is the runner that sends the loop n signals at its first
// read of the running set, during the pass.
type signalledAtPs struct {
	*runner.Fake
	signals chan os.Signal
	n       int
	once    *sync.Once
}

func (r signalledAtPs) Running() ([]runner.Running, error) {
	r.once.Do(func() {
		for range r.n {
			r.signals <- os.Interrupt
		}
	})
	return r.Fake.Running()
}

// Two signals during a pass that left no member running exit at once
// with ErrInterrupted (130), as a second signal during the drain does.
func TestSwarmApplyLoopTwoSignalsInAPass(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	waited := false
	a.after = func(time.Duration) <-chan time.Time { waited = true; return nil }
	err := a.applyTo(a.git(), signalledAtPs{a.runner, a.signals, 2, &sync.Once{}}, &a.stdout, &a.stderr)
	if !errors.Is(err, ErrInterrupted) || waited {
		t.Errorf("%v, waited %v\n%s", err, waited, a.stdout.String())
	}
}

// heldAfterASignal is the runner that sends the loop a signal at its
// first launch, during the pass, and a second soon after, while the
// member it started is held running.
type heldAfterASignal struct {
	*runner.Fake
	signals chan os.Signal
	open    chan struct{}
}

func (r heldAfterASignal) Start(s runner.Spec, d time.Duration) (runner.Container, error) {
	r.signals <- os.Interrupt
	c, err := r.Fake.Start(s, d)
	go func() { time.Sleep(50 * time.Millisecond); r.signals <- os.Interrupt }()
	return gatedRun{c, r.open}, err
}

// A signal during a pass, then a second while its member still runs:
// ErrInterrupted (130), the member left running and its run uncollected.
func TestSwarmApplyLoopSecondSignalAfterAPass(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.after = func(time.Duration) <-chan time.Time { t.Error("waited out the interval"); return nil }
	a.forge.CreateIssue("Job", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	open := make(chan struct{})
	err := a.applyTo(a.git(), heldAfterASignal{a.runner, a.signals, open}, &a.stdout, &a.stderr)
	close(open)
	if !errors.Is(err, ErrInterrupted) || strings.Contains(a.result("1-architect-1"), "collected") {
		t.Errorf("%v\n%s", err, a.result("1-architect-1"))
	}
}

// The loop takes no signal once it has returned: the drain's watcher for
// a second signal ends with it.
func TestSwarmApplyLoopLeavesNoWatcher(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.after = func(time.Duration) <-chan time.Time { a.signals <- os.Interrupt; return nil }
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	a.signals <- os.Interrupt
	for range 20 {
		if len(a.signals) == 0 {
			t.Fatal("a signal sent after the loop returned was taken")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// signalledAtIssues is the forge that sends the loop a signal at its
// first list of issues, in the pass's step (5).
type signalledAtIssues struct {
	forge.Adapter
	signals chan os.Signal
	once    *sync.Once
}

func (f signalledAtIssues) Issues(s forge.State, label string) ([]forge.Issue, error) {
	f.once.Do(func() { f.signals <- os.Interrupt })
	return f.Adapter.Issues(s, label)
}

// A signal during a pass skips its merge step (section 11): an approved
// issue with the merge gate off is not merged by it.
func TestSwarmApplyLoopSignalSkipsTheMergeStep(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved"}) {
		t.Fatalf("not approved: %v", got)
	}
	a.signals = make(chan os.Signal, 2)
	a.after = func(time.Duration) <-chan time.Time { t.Error("waited out the interval"); return nil }
	err := a.applyTo(signalledAtIssues{a.git(), a.signals, &sync.Once{}}, a.runner, &a.stdout, &a.stderr)
	if err != nil || a.pulls(1)[0].Merged || strings.Contains(a.stdout.String(), "merged pull") {
		t.Errorf("%v\n%s", err, a.stdout.String())
	}
}

// The loop's lock, adapter and token are its start's: a pass after
// swarm.json's forge.repo changed is skipped with the reason, never run
// on the other repository's <state> without its lock (section 11).
func TestSwarmApplyLoopForgeChanged(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	waits := 0
	var mu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		waits++
		c := make(chan time.Time, 1)
		if waits == 1 {
			a.edit(func(c *declare.Config) { c.Forge.Repo = "acme/lib" })
			c <- time.Time{}
		} else {
			a.signals <- os.Interrupt
		}
		return c
	}
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(a.stdout.String(), "Pass "); n != 1 || !strings.Contains(a.stderr.String(), "swarm.json's forge (kind, api, repo or token_env) changed since apply started; pass skipped") {
		t.Errorf("%d passes\nstderr:\n%s", n, a.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(a.local, "swarm", "forge.example.net", "acme", "lib")); err == nil {
		t.Error("a pass ran on acme/lib's <state>")
	}
}
