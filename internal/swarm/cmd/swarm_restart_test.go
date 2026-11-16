package cmd

import (
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/swarm/runner"
)

// exec is one re-exec the loop asked for, and what stood at that moment.
type exec struct {
	path    string
	args    []string
	running []runner.Running
	passes  int    // the passes stdout had reported
	locked  bool   // <state>/lock was still held
	flagged bool   // <state>/restart was still there
	result  string // run 1-architect-1's result
}

// recordExec makes a's re-exec record what stood at it, then answer err.
func (a *applyWS) recordExec(execs *[]exec, err error) {
	a.reexec = func(path string, args ...string) error {
		running, _ := a.runner.Running()
		_, lockErr := ops.Real().FS().Lock(a.state("lock"), "probe")
		var held *ops.LockedError
		_, flagErr := os.Lstat(a.state("restart"))
		*execs = append(*execs, exec{path, args, running, strings.Count(a.stdout.String(), "Pass "), errors.As(lockErr, &held), flagErr == nil, a.result("1-architect-1")})
		return err
	}
}

// signalAt is an interval wait that runs step after pass i and goes on,
// and from pass last on sends the loop a signal.
func (a *applyWS) signalAt(last int, step func(pass int)) {
	waits := 0
	var mu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		waits++
		a.tick()
		if waits >= last {
			a.signals <- os.Interrupt
			return make(chan time.Time)
		}
		if step != nil {
			step(waits)
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
}

// The claim of Step 19 (docs/swarm.md section 11, The restart): the
// installed binary swapped under a running apply while #1's architect
// runs is not re-executed at the pass that sees it, nor at the next,
// while the member runs; it is at the first pass after the member ended
// and was collected, with the lock still held, the same arguments, a
// stdout line and a journal line.
func TestSwarmApplyRestartsOntoANewBuild(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Long", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	const long = "daisy-swarm.acme.swarm-app.1.architect.1"
	gate := make(chan struct{})
	var execs []exec
	a.recordExec(&execs, nil)
	a.signalAt(5, func(pass int) {
		switch pass {
		case 1:
			if err := os.WriteFile(a.binary, []byte("build 2, a longer one\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		case 3:
			close(gate)
			for running, _ := a.runner.Running(); slices.ContainsFunc(running, func(c runner.Running) bool { return c.Name == long }); running, _ = a.runner.Running() {
				time.Sleep(time.Millisecond)
			}
		}
	})
	if err := a.applyTo(a.git(), held{a.runner, map[string]chan struct{}{long: gate}}, &a.stdout, &a.stderr); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 1 {
		t.Fatalf("%d re-execs:\n%s", len(execs), a.stdout.String())
	}
	e := execs[0]
	if e.path != a.binary || strings.Join(e.args, " ") != "swarm apply" || e.passes != 4 || len(e.running) != 0 || !e.locked ||
		!strings.HasSuffix(e.result, "outcome posted\ncollected\n") {
		t.Errorf("re-exec %+v\n%s", e, a.stdout.String())
	}
	pending := "Restart pending (a new build at " + a.binary + "): waiting for a pass with no member running\n"
	if out := a.stdout.String(); strings.Count(out, pending) != 1 || !strings.Contains(out, "running 1\n"+pending) ||
		!strings.HasSuffix(out, "Pass 15:03:00: launched 0, collected 1, running 0\nRestart (a new build at "+a.binary+"): no member running; re-executing "+a.binary+" swarm apply\n") {
		t.Errorf("stdout:\n%s", out)
	}
	if q, _ := os.ReadFile(a.state("journal")); string(q) != "2026-11-06 - 1503 0-restart-20261106T150300Z: apply re-executed (a new build)\n" {
		t.Errorf("journal queue %q", q)
	}
}

// daisy swarm restart (section 11): the request written, and shown by
// status; --once leaves it; the loop's first pass with no member running
// removes it and re-executes the build at the same path, unchanged, and
// the process it starts does not take the request again.
func TestSwarmRestartRequested(t *testing.T) {
	a := newApplyWS(t)
	if err := a.verb(SwarmRestart); err != nil || a.stdout.String() != "Restart requested ("+a.state("restart")+"): the running apply re-executes itself at its first pass with no member running\n" {
		t.Fatalf("restart: %v %q", err, a.stdout.String())
	}
	if err := a.verb(SwarmRestart); err != nil || a.stdout.String() != "Restart already requested at 2026-11-06T15:00:00Z\n" {
		t.Errorf("restart again: %v %q", err, a.stdout.String())
	}
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nState: "+a.state("")+" — restart requested at 2026-11-06T15:00:00Z — owner batch held: test") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
	if err := a.apply(a.git(), "--once"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.state("restart")); err != nil {
		t.Errorf("--once took the request: %v", err)
	}
	var execs []exec
	a.recordExec(&execs, nil)
	a.signals = make(chan os.Signal, 2)
	a.signalAt(1, nil)
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 1 || execs[0].path != a.binary || strings.Join(execs[0].args, " ") != "swarm apply" || execs[0].flagged || execs[0].passes != 1 {
		t.Fatalf("re-execs %+v\n%s", execs, a.stdout.String())
	}
	if !strings.HasSuffix(a.stdout.String(), "\nRestart (daisy swarm restart at 2026-11-06T15:00:00Z): no member running; re-executing "+a.binary+" swarm apply\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
	if q, _ := os.ReadFile(a.state("journal")); string(q) != "2026-11-06 - 1500 0-restart-20261106T150000Z: apply re-executed (daisy swarm restart at 2026-11-06T15:00:00Z)\n" {
		t.Errorf("journal queue %q", q)
	}
	// The process the exec started: its passes take no request.
	a.reexec = nil
	a.signalAt(3, nil)
	if err := a.apply(a.git()); err != nil || strings.Contains(a.stdout.String(), "Restart") {
		t.Errorf("%v\n%s", err, a.stdout.String())
	}
}

// The same build written again (a rebuild of the same source): its
// modification time changes, its content does not, and no pass restarts.
func TestSwarmApplyRestartNotOnTheSameBuild(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.signalAt(4, func(pass int) {
		if pass == 1 {
			later := a.Now().Add(time.Hour)
			if err := os.Chtimes(a.binary, later, later); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := a.apply(a.git()); err != nil || strings.Count(a.stdout.String(), "Pass ") != 4 || strings.Contains(a.stdout.String(), "Restart") {
		t.Errorf("%v\n%s", err, a.stdout.String())
	}
}

// A new build that cannot be executed: a warning and a journal line, the
// loop goes on from the build it started with, and that build is not
// tried again at the next idle pass.
func TestSwarmApplyRestartExecFails(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	var execs []exec
	a.recordExec(&execs, syscall.ENOEXEC)
	a.signalAt(5, func(pass int) {
		if pass == 1 {
			os.WriteFile(a.binary, []byte("build 2, truncated\n"), 0o755)
		}
	})
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 1 || execs[0].passes != 3 || strings.Count(a.stdout.String(), "Pass ") != 5 {
		t.Fatalf("re-execs %+v\n%s", execs, a.stdout.String())
	}
	if !strings.Contains(a.stderr.String(), "⚠ restart: exec "+a.binary+": exec format error; apply goes on from the build it started with\n") {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
	// Pushed by the next pass, as a run's line is.
	lines, err := reconcile.ReadJournal(ops.Real().FS(), a.state(""))
	var got []string
	for _, l := range lines {
		if strings.HasPrefix(l.Run, "0-restart-") {
			got = append(got, l.Day+" "+l.Line)
		}
	}
	if err != nil || strings.Join(got, "\n") != "2026-11-06 - 1502 0-restart-20261106T150200Z: apply re-executed (a new build)\n"+
		"2026-11-06 - 1502 0-restart-20261106T150200Z-failed: exec failed: exec format error; apply goes on from the build it started with" {
		t.Errorf("journal %v %q", err, got)
	}
	if _, err := os.Stat(a.state("journal")); err == nil {
		t.Error("the restart's lines were not pushed")
	}
}

// A run ended and not collected holds the restart as a running member
// does (rule 2's holders): while its comment cannot be posted, no pass
// re-executes.
func TestSwarmApplyRestartWaitsForTheCollection(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.signalAt(5, func(pass int) {
		if pass == 1 {
			os.WriteFile(a.binary, []byte("build 2, a longer one\n"), 0o755)
		}
	})
	if err := a.apply(&failing{Fake: a.forge, comments: 100}); err != nil {
		t.Fatal(err)
	}
	if r := a.result("1-architect-1"); strings.Contains(r, "collected") || !strings.Contains(a.stdout.String(), "Restart pending (a new build at "+a.binary+")") {
		t.Errorf("result:\n%s\nstdout:\n%s", r, a.stdout.String())
	}
}

// signalledAtIdle is the runner that sends the loop a signal at its
// third read of the running set, the restart's look for a member after
// the start's and the pass's.
type signalledAtIdle struct {
	*runner.Fake
	signals chan os.Signal
	reads   *int
}

func (r signalledAtIdle) Running() ([]runner.Running, error) {
	if *r.reads++; *r.reads == 3 {
		r.signals <- os.Interrupt
	}
	return r.Fake.Running()
}

// A signal during the restart's look for a member: no re-exec, the
// request kept for the next apply, and the drain at once, without
// waiting out the interval.
func TestSwarmApplyRestartSignalled(t *testing.T) {
	a := newApplyWS(t)
	if err := a.verb(SwarmRestart); err != nil {
		t.Fatal(err)
	}
	a.signals = make(chan os.Signal, 2)
	a.after = func(time.Duration) <-chan time.Time {
		t.Error("waited out the interval")
		a.signals <- os.Interrupt // the signal the loop missed, so that it ends
		return nil
	}
	reads := 0
	a.stdout.Reset()
	if err := a.applyTo(a.git(), signalledAtIdle{a.runner, a.signals, &reads}, &a.stdout, &a.stderr); err != nil || reads != 3 {
		t.Fatalf("%v, %d reads of the running set\n%s", err, reads, a.stdout.String())
	}
	if _, err := os.Stat(a.state("restart")); err != nil || strings.Contains(a.stdout.String(), "Restart") {
		t.Errorf("request %v\n%s", err, a.stdout.String())
	}
}
