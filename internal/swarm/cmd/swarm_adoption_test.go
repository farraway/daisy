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
