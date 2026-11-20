package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// The claim of L425 (the plan implementing the 2026-11-19 swarm roadmap,
// Step 7; the maintainer's answer of 2026-11-18): a revert that a later revert
// takes back (a reapply) no longer counts, and a chain is followed: a
// commit reverted once is reverted; one reverted then reapplied is not;
// one reverted, reapplied and reverted again is; a reapply in effect
// reverts the revert it names, and one reverted itself does not.
func TestRevertedReapplyCancelsTheRevert(t *testing.T) {
	o := ops.Real()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.net"}, args...)...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	commit := func(name string) string {
		os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644)
		git("add", "-A")
		git("commit", "-q", "-m", name)
		return git("rev-parse", "HEAD")
	}
	revert := func(sha string) string {
		git("revert", "--no-edit", sha)
		return git("rev-parse", "HEAD")
	}
	a, b, c := commit("a"), commit("b"), commit("c")
	revert(a)
	rb := revert(b)
	revert(rb) // b reapplied
	rc := revert(c)
	reapplied := revert(rc)
	revert(reapplied) // c reverted again

	named, err := Reverted(o, dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for sha, want := range map[string]bool{a: true, b: false, c: true, rb: true, rc: false, reapplied: true} {
		if got := named.Has(sha); got != want {
			t.Errorf("Has(%s) = %v, want %v (named %v)", git("log", "-1", "--format=%s", sha), got, want, named)
		}
	}
}
