package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"daisy/internal/swarm/runner"
)

// A member runs detached, its sh writing the CLI's standard output and
// error to /job/io/stdout and /job/io/stderr beside its exit status
// (section 7.10, rule 2; 7.2): the launch's argv is `docker run
// --detach`, and the collection reads the run's streams, its cost and
// its exit status from those files, which <run>/stdout and <run>/stderr
// copy.
func TestMemberArgvDetachedWithStdoutFile(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult, Stderr: "a warning\n", Code: 3})
	if err := a.apply(a.git(), "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	spec := a.runner.Launches()[0]
	const script = `"$@" >/job/io/stdout 2>/job/io/stderr; s=$?; echo "$s" >/job/io/exit; exit "$s"`
	if args := spec.Args(); !slices.Equal(args[:3], []string{"run", "--detach", "--rm"}) || !slices.Equal(spec.Argv[:4], []string{"sh", "-c", script, "sh"}) {
		t.Errorf("args %q", args)
	}
	run := a.state("runs/1-architect-1")
	for name, want := range map[string]string{"stdout": claudeResult, "stderr": "a warning\n", "io/stdout": claudeResult, "io/exit": "3\n"} {
		if b, err := os.ReadFile(filepath.Join(run, name)); err != nil || string(b) != want {
			t.Errorf("%s: %q %v", name, b, err)
		}
	}
	if r := a.result("1-architect-1"); !strings.Contains(r, "\nexited 3 2026-11-06T15:00:00Z\ncost 0.25 tokens 10/20 turns 4 cached 0\nfailed: exit 3\n") {
		t.Errorf("result:\n%s", r)
	}
}

// A worker run's check writes its output and its exit status into
// <run>/step/ (section 7.10, rule 2): one whose apply died, its container
// gone, is collected from those files, `check exited <code>` and
// check.out its output, not started again.
func TestSwarmCheckCollectedFromItsFiles(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker("exited 0 2026-11-06T15:10:00Z")
	a.gitRunIn(a.state("git/app"), "fetch", "-q", "--no-tags", a.state("runs/1-worker-1/work"), "+refs/heads/swarm/1-1:refs/swarm-runs/1-worker-1")
	run := a.state("runs/1-worker-1")
	os.WriteFile(filepath.Join(run, "io", "out.md"), []byte("v1\n\nNot established: none\n"), 0o644)
	os.MkdirAll(filepath.Join(run, "step"), 0o755)
	os.WriteFile(filepath.Join(run, "step", "out"), []byte("ok 1 test\n"), 0o644)
	os.WriteFile(filepath.Join(run, "step", "exit"), []byte("0\n"), 0o644)
	f, _ := os.OpenFile(filepath.Join(run, "result"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("fetched " + head + "\ncheck started 2026-11-06T15:11:00Z\n")
	f.Close()
	a.pass()
	r := a.result("1-worker-1")
	if out, _ := os.ReadFile(filepath.Join(run, "check.out")); strings.Count(r, "\ncheck started ") != 1 || !strings.Contains(r, "\ncheck exited 0 ") ||
		!strings.Contains(r, "\npushed "+head+"\n") || len(a.launched("check")) != 0 || string(out) != "ok 1 test\n" {
		t.Errorf("check.out %q, checks launched %d\n%s", out, len(a.launched("check")), r)
	}

	// A check this process starts: detached, its sh writing to /job/step.
	b := newApplyWS(t)
	b.planned(oneSlice)
	b.runner.Script(1, "worker", work("v1"))
	b.runner.Script(1, "check", runner.Canned{Stdout: "ok 2 tests\n", Stderr: "warn\n", Code: 0})
	b.pass()
	checks := b.launched("check")
	if len(checks) != 1 {
		t.Fatalf("checks %d", len(checks))
	}
	c := checks[0]
	const script = `"$@" >/job/step/out 2>&1; s=$?; echo "$s" >/job/step/exit; exit "$s"`
	stepDir := b.state("runs/1-worker-1/step")
	if args := c.Args(); args[1] != "--detach" || !slices.Equal(c.Argv, []string{"sh", "-c", script, "sh", "make", "check"}) ||
		!slices.Contains(c.Mounts, runner.Mount{Src: stepDir, Dst: "/job/step"}) {
		t.Errorf("check spec %+v", c)
	}
	if out, _ := os.ReadFile(b.state("runs/1-worker-1/check.out")); string(out) != "ok 2 tests\nwarn\n" || !strings.Contains(b.result("1-worker-1"), "\ncheck exited 0 ") {
		t.Errorf("check.out %q\n%s", out, b.result("1-worker-1"))
	}
}

// stuck is the runner whose members' Wait returns only once release is
// closed, their containers gone from the running set at once: a member
// that finished while this process's wait on it has not returned.
type stuck struct {
	*runner.Fake
	release chan struct{}
}

func (s stuck) Start(spec runner.Spec, d time.Duration) (runner.Container, error) {
	c, err := s.Fake.Start(spec, d)
	if err != nil {
		return nil, err
	}
	c.Wait() // the Fake's container gone, its files written
	return gatedRun{c, s.release}, nil
}

// Any apply collects a run of its own whose container is gone and whose
// io/exit is written, though its wait on it has not returned (section
// 7.10, rule 2; request F36): #1's architect, launched at pass 1, is
// collected at pass 2.
// Not parallel: its guard is the wall clock's 10s.
func TestSwarmApplyCollectsItsOwnFinishedRun(t *testing.T) {
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	release := make(chan struct{})
	var once sync.Once
	var forced atomic.Bool
	end := func() { once.Do(func() { close(release) }) }
	defer end()
	guard := time.AfterFunc(10*time.Second, func() { forced.Store(true); end() })
	defer guard.Stop()
	a.signalAt(2, nil)
	if err := a.applyTo(a.git(), stuck{a.runner, release}, &a.stdout, &a.stderr); err != nil {
		t.Fatal(err)
	}
	out := a.stdout.String()
	second := strings.Index(out, "Pass 15:01:00: launched 0, collected 1, running 0\n")
	if forced.Load() || second < 0 || strings.Index(out, "#1 architect run 1: posted comment\n") > second ||
		!strings.Contains(a.result("1-architect-1"), "\nexited 0 2026-11-06T15:01:00Z\ncost 0.25 tokens 10/20 turns 4 cached 0\n") {
		t.Errorf("forced %v\n%s\n%s", forced.Load(), out, a.result("1-architect-1"))
	}
}

// adopted writes run 1 of #1's architect as an apply that died left it:
// its argv the detached one, its result to its persona line and its
// deadline, its member's files in io/, no container.
func (a *applyWS) adopted(started, deadline string, io map[string]string) string {
	a.t.Helper()
	run := a.state("runs/1-architect-1")
	os.MkdirAll(filepath.Join(run, "io"), 0o755)
	os.MkdirAll(filepath.Join(run, "work"), 0o755)
	os.WriteFile(filepath.Join(run, "argv"), []byte(strings.Join(append([]string{"docker", "run", "--detach"}, exitPrefix...), "\n")+"\nclaude\n"), 0o644)
	for name, b := range io {
		os.WriteFile(filepath.Join(run, "io", name), []byte(b), 0o644)
	}
	head := "start app 0123\nlabel swarm:proposal-accepted\nstarted " + started + "\ndeadline " + deadline + "\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\npersona anthropic claude-fable-5-1 high\n"
	os.WriteFile(filepath.Join(run, "result"), []byte(head), 0o644)
	return head
}

// A run another apply launched detached, its container gone, is
// collected from its files with its cost and its provider's fault
// (section 7.10, rule 2): exited, not ended, its stdout copied.
func TestSwarmAdoptedRunKeepsItsCostAndFault(t *testing.T) {
	t.Parallel()
	const failed = `{"type":"result","is_error":true,"result":"Invalid API key · Please run /login","total_cost_usd":0.01,"num_turns":1,"usage":{"input_tokens":5,"output_tokens":1}}`
	for _, tc := range []struct {
		name, exit, stdout, want string
	}{
		{"posted", "0\n", claudeResult, "exited 0 2026-11-06T15:01:00Z\ncost 0.25 tokens 10/20 turns 4 cached 0\nposted comment\n"},
		{"a fault", "1\n", failed, "exited 1 2026-11-06T15:01:00Z\ncost 0.01 tokens 5/1 turns 1 cached 0\nfault: provider: Invalid API key · Please run /login\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			head := a.adopted("2026-11-06T14:30:00Z", "2026-11-06T15:30:00Z", map[string]string{"out.md": archPlan, "exit": tc.exit, "stdout": tc.stdout, "stderr": "a warning\n"})
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan}) // run 2, after the fault
			a.tick()
			if err := a.apply(a.git(), "--once"); err != nil {
				t.Fatalf("%v\n%s", err, a.stderr.String())
			}
			r := a.result("1-architect-1")
			if !strings.HasPrefix(r, head+tc.want) {
				t.Errorf("result:\n%s", r)
			}
			for name, want := range map[string]string{"stdout": tc.stdout, "stderr": "a warning\n"} {
				if b, _ := os.ReadFile(a.state("runs/1-architect-1/" + name)); string(b) != want {
					t.Errorf("%s: %q", name, b)
				}
			}
		})
	}
}
