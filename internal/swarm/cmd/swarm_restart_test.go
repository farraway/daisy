package cmd

import (
	"errors"
	"os"
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

// The claim of Step R4-B (docs/swarm.md section 7.10, rule 2, The
// restart): the installed binary swapped under a running apply while
// #1's architect runs is re-executed at the end of the pass after the one
// that saw it, the member still running, with the lock still held, the
// same arguments, a stdout line and a journal line naming the one
// running; the run is held by its record, and the next apply collects
// it once its member has ended.
func TestSwarmApplyRestartsWithMembersRunning(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Long", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	const long = "daisy-swarm.acme.swarm-app.1.architect.1"
	gate := make(chan struct{})
	var execs []exec
	a.recordExec(&execs, nil)
	a.signalAt(5, func(pass int) {
		if pass == 1 {
			if err := os.WriteFile(a.binary, []byte("build 2, a longer one\n"), 0o755); err != nil {
				t.Fatal(err)
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
	if e.path != a.binary || strings.Join(e.args, " ") != "swarm apply" || e.passes != 3 || len(e.running) != 1 || e.running[0].Name != long || !e.locked ||
		strings.Contains(e.result, "collected") {
		t.Errorf("re-exec %+v\n%s", e, a.stdout.String())
	}
	if out := a.stdout.String(); strings.Contains(out, "Restart pending") ||
		!strings.HasSuffix(out, "Pass 15:02:00: launched 0, collected 0, running 1\nRestart (a new build at "+a.binary+"): 1 running; re-executing "+a.binary+" swarm apply\n") {
		t.Errorf("stdout:\n%s", out)
	}
	if q, _ := os.ReadFile(a.state("journal")); string(q) != "2026-11-06 - 1502 0-restart-20261106T150200Z: apply re-executed (a new build), 1 running\n" {
		t.Errorf("journal queue %q", q)
	}
	// The process the exec started: the member ends, and it is collected
	// from its record and its files.
	close(gate)
	for running, _ := a.runner.Running(); len(running) > 0; running, _ = a.runner.Running() {
		time.Sleep(time.Millisecond)
	}
	if err := a.apply(a.git(), "--once"); err != nil {
		t.Fatal(err)
	}
	if r := a.result("1-architect-1"); !strings.Contains(r, "\nexited 0 ") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
}

// A new build copied over the running one with its size and modification
// time kept (`cp -p` of a build the same length): its change time tells
// it (triage item L345), and the loop re-executes onto it.
func TestSwarmApplyRestartSeesAPreservedTimesCopy(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	fi, err := os.Stat(a.binary)
	if err != nil {
		t.Fatal(err)
	}
	var execs []exec
	a.recordExec(&execs, nil)
	a.signalAt(5, func(pass int) {
		if pass == 1 {
			if err := os.WriteFile(a.binary, []byte("build 2\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(a.binary, fi.ModTime(), fi.ModTime()); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 1 || execs[0].passes != 3 {
		t.Errorf("re-execs %+v\n%s", execs, a.stdout.String())
	}
}

// daisy swarm restart (section 11): the request written, and shown by
// status; --once leaves it; the loop's first pass removes it and re-executes the build at the same path, unchanged, and
// the process it starts does not take the request again.
func TestSwarmRestartRequested(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	if err := a.verb(SwarmRestart); err != nil || a.stdout.String() != "Restart requested ("+a.state("restart")+"): the running apply re-executes itself at the end of its next pass\n" {
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
	if !strings.HasSuffix(a.stdout.String(), "\nRestart (daisy swarm restart at 2026-11-06T15:00:00Z): 0 running; re-executing "+a.binary+" swarm apply\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
	if q, _ := os.ReadFile(a.state("journal")); string(q) != "2026-11-06 - 1500 0-restart-20261106T150000Z: apply re-executed (daisy swarm restart at 2026-11-06T15:00:00Z), 0 running\n" {
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
	t.Parallel()
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
// tried again at the next pass.
func TestSwarmApplyRestartExecFails(t *testing.T) {
	t.Parallel()
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
	if err != nil || strings.Join(got, "\n") != "2026-11-06 - 1502 0-restart-20261106T150200Z: apply re-executed (a new build), 0 running\n"+
		"2026-11-06 - 1502 0-restart-20261106T150200Z-failed: exec failed: exec format error; apply goes on from the build it started with" {
		t.Errorf("journal %v %q", err, got)
	}
	if _, err := os.Stat(a.state("journal")); err == nil {
		t.Error("the restart's lines were not pushed")
	}
}

// A run ended and not collected (its comment refused by the forge) no
// longer holds the restart (section 7.10, rule 2): the loop re-executes
// at the end of the pass after the one that saw the new build, the run's
// record kept with its end line, and the process it starts collects it
// once the forge takes the comment.
func TestSwarmApplyRestartWaitsForTheCollection(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	var execs []exec
	a.recordExec(&execs, nil)
	a.signalAt(5, func(pass int) {
		if pass == 1 {
			os.WriteFile(a.binary, []byte("build 2, a longer one\n"), 0o755)
		}
	})
	if err := a.apply(&failing{Fake: a.forge, comments: 100}); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 1 || execs[0].passes != 3 || !strings.Contains(execs[0].result, "\nexited 0 ") || strings.Contains(execs[0].result, "collected") ||
		strings.Contains(a.stdout.String(), "Restart pending") {
		t.Fatalf("re-execs %+v\nstdout:\n%s", execs, a.stdout.String())
	}
	a.reexec = nil
	if err := a.apply(a.git(), "--once"); err != nil {
		t.Fatal(err)
	}
	if r := a.result("1-architect-1"); !strings.HasSuffix(r, "outcome posted\ncollected\n") || strings.Count(r, "\nexited ") != 1 {
		t.Errorf("result:\n%s", r)
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
	t.Parallel()
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
