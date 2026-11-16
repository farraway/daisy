package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// `daisy --version` is `daisy version`; a binary the toolchain did not
// stamp with a commit (a test binary is one) says so and how to build one
// that has it, rather than print a version it does not know. The stamped
// path is TestInstallDevChecksTheInstalledCommit's, through a real build.
func TestVersionWithoutAStamp(t *testing.T) {
	m := newMachine(t)
	for _, args := range [][]string{{"version"}, {"--version"}} {
		code, out, errs := m.run(t, m.ws, args...)
		if code != 1 || out != "" || !strings.Contains(errs, "carries no commit") || !strings.Contains(errs, "make build") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	for _, args := range [][]string{{"version", "x"}, {"--version", "x"}} {
		if code, _, errs := m.run(t, m.ws, args...); code != 2 || !strings.Contains(errs, "Usage: daisy version") {
			t.Errorf("%v: exit %d err %q", args, code, errs)
		}
	}
}

// `make -C <root> install-dev` pulls the root fast-forward, builds,
// installs and exits non-zero unless ~/bin/daisy's `daisy version` is the
// root's HEAD. The root is a scratch clone of a scratch upstream holding
// this tree's source; HOME is scratch, so the link install writes is too.
// A build that leaves the old binary in place (GO=true: it exits 0 and
// writes nothing, as a read-only Go cache hidden behind `| tail` once did)
// is refused naming both commits; a real build installs a binary whose
// `daisy version` prints HEAD's sha. A tree git status shows changes in
// (an untracked file is one) builds HEAD marked modified, which may hold
// code HEAD does not: refused too, naming the change; that binary left in
// place over a clean tree is a stale build, and said so.
func TestInstallDevChecksTheInstalledCommit(t *testing.T) {
	if _, err := os.Stat(".git"); err != nil {
		t.Skip("not a git checkout: no tracked set to copy into the scratch root")
	}
	scratch := t.TempDir()
	upstream, root, home := filepath.Join(scratch, "upstream"), filepath.Join(scratch, "root"), filepath.Join(scratch, "home")
	var environ []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case k == "HOME", k == "GOFLAGS", k == "MAKEFLAGS", k == "MFLAGS", k == "MAKELEVEL", strings.HasPrefix(k, "DAISY_"):
		default:
			environ = append(environ, kv)
		}
	}
	environ = append(environ, "HOME="+home, "GIT_AUTHOR_NAME=Fixture Author", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture Author", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	run := func(dir, name string, args ...string) (int, string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = dir, environ
		out, err := cmd.CombinedOutput()
		if _, exited := err.(*exec.ExitError); err != nil && !exited {
			t.Fatalf("%s %v: %v", name, args, err)
		}
		return cmd.ProcessState.ExitCode(), string(out)
	}
	must := func(dir, name string, args ...string) string {
		t.Helper()
		code, out := run(dir, name, args...)
		if code != 0 {
			t.Fatalf("%s %v: exit %d\n%s", name, args, code, out)
		}
		return strings.TrimSpace(out)
	}

	// The source as a commit of this tree would hold it, read by absolute
	// path: TestMain's chdir comes before the test log starts, so go test's
	// cache would resolve a relative name in cmd/daisy and keep a pass
	// after the Makefile changed.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, listed := run(wd, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	for _, f := range strings.Split(listed, "\x00") {
		info, err := os.Lstat(filepath.Join(wd, f))
		if f == "" || err != nil || !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(wd, f))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(upstream, f), string(b))
		os.Chmod(filepath.Join(upstream, f), info.Mode().Perm())
	}
	must(upstream, "git", "init", "-q")
	must(upstream, "git", "add", "-A")
	must(upstream, "git", "commit", "-q", "-m", "A")
	built := must(upstream, "git", "rev-parse", "HEAD")
	must(scratch, "git", "clone", "-q", upstream, root)
	must(root, "make", "--no-print-directory", "build")
	writeFile(t, filepath.Join(upstream, "CHANGES"), "B\n")
	must(upstream, "git", "add", "-A")
	must(upstream, "git", "commit", "-q", "-m", "B")
	head := must(upstream, "git", "rev-parse", "HEAD")

	code, out := run(scratch, "make", "-C", root, "install-dev", "GO=true")
	t.Logf("install-dev GO=true:\n%s", out)
	if code == 0 || !strings.Contains(out, built) || !strings.Contains(out, head) {
		t.Errorf("a build that left %s in place: exit %d, want non-zero naming it and HEAD %s", built, code, head)
	}
	if got := must(root, "git", "rev-parse", "HEAD"); got != head {
		t.Errorf("not pulled: root HEAD %s, upstream %s", got, head)
	}

	code, out = run(scratch, "make", "-C", root, "install-dev")
	t.Logf("install-dev:\n%s", out)
	if code != 0 {
		t.Errorf("install-dev: exit %d", code)
	}
	if link, err := os.Readlink(filepath.Join(home, "bin", "daisy")); err != nil || link != filepath.Join(root, "bin", "daisy") {
		t.Errorf("~/bin/daisy is %q (%v), want the root's bin/daisy", link, err)
	}
	if got := must(root, filepath.Join(home, "bin", "daisy"), "version"); got != head {
		t.Errorf("daisy version: %q, want HEAD %s", got, head)
	}

	writeFile(t, filepath.Join(root, "untracked.txt"), "x\n")
	code, out = run(scratch, "make", "-C", root, "install-dev")
	if code == 0 || !strings.Contains(out, "built from a tree with changes HEAD does not hold") || !strings.Contains(out, "?? untracked.txt") {
		t.Errorf("a modified tree: exit %d, want non-zero naming the change\n%s", code, out)
	}

	// The tree cleaned, a build that leaves that modified binary in place is
	// the stale build it is, not a tree with changes.
	if err := os.Remove(filepath.Join(root, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	code, out = run(scratch, "make", "-C", root, "install-dev", "GO=true")
	if code == 0 || !strings.Contains(out, "the build did not install HEAD") || strings.Contains(out, "built from a tree with changes") {
		t.Errorf("a stale modified binary over a clean tree: exit %d, want non-zero naming the stale build\n%s", code, out)
	}
	if got := must(root, filepath.Join(home, "bin", "daisy"), "version"); got != head+" (modified)" {
		t.Errorf("daisy version of a modified tree: %q, want %s (modified)", got, head)
	}
}
