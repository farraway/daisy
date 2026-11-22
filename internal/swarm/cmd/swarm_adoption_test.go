package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
