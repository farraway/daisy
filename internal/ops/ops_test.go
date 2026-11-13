package ops_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"daisy/internal/ops"
)

func TestReadOnlyRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	ro := ops.ReadOnly(ops.Real())
	target := filepath.Join(dir, "file")

	err := ro.FS().WriteFile(target, []byte("x"), 0o644)
	if err == nil {
		t.Fatal("WriteFile under ReadOnly returned no error")
	}
	if !strings.Contains(err.Error(), "WriteFile") || !strings.Contains(err.Error(), target) {
		t.Errorf("error does not name the call and path: %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("file was written despite the refusal: %v", statErr)
	}
	for name, call := range map[string]func() error{
		"MkdirAll":  func() error { return ro.FS().MkdirAll(filepath.Join(dir, "d"), 0o755) },
		"Symlink":   func() error { return ro.FS().Symlink(target, filepath.Join(dir, "l")) },
		"Remove":    func() error { return ro.FS().Remove(target) },
		"RemoveAll": func() error { return ro.FS().RemoveAll(dir) },
		"Rename":    func() error { return ro.FS().Rename(target, target+".2") },
		"Mkdir":     func() error { return ro.FS().Mkdir(filepath.Join(dir, "m"), 0o755) },
		"Chmod":     func() error { return ro.FS().Chmod(dir, 0o700) },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s under ReadOnly: got %v, want an error naming the call", name, err)
		}
	}
}

func TestReadOnlyExecAllowsOnlyGitReads(t *testing.T) {
	ro := ops.ReadOnly(ops.Real())
	dir := t.TempDir()
	if _, _, err := ro.Exec(dir, "git", "commit", "-m", "x"); err == nil || !strings.Contains(err.Error(), "git commit") {
		t.Errorf("git commit under ReadOnly: got %v, want a refusal naming it", err)
	}
	if _, _, err := ro.Exec(dir, "git", "config", "--local", "user.name", "x"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("git config set under ReadOnly: got %v, want a refusal", err)
	}
	if _, _, err := ro.Exec(dir, "touch", "f"); err == nil || !strings.Contains(err.Error(), "touch") {
		t.Errorf("touch under ReadOnly: got %v, want a refusal", err)
	}
	if _, _, err := ro.Exec(dir, "git", "init", "-q"); err == nil {
		t.Error("git init under ReadOnly was allowed")
	}
	// Reads go through to the inner Ops and reach git.
	if _, _, err := ops.Real().Exec(dir, "git", "init", "-q"); err != nil {
		t.Fatal(err)
	}
	out, _, err := ro.Exec(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Errorf("git rev-parse under ReadOnly: out %q err %v", out, err)
	}
	// A swarm's mirrors are read for its briefs and its drift
	// (docs/swarm.md section 11, status): ls-tree, cat-file, and
	// hash-object without -w, which writes no object.
	for _, args := range [][]string{{"ls-tree", "HEAD"}, {"cat-file", "-t", "HEAD"}, {"hash-object", "--stdin"}} {
		if _, _, err := ro.Exec(dir, "git", args...); err != nil && strings.Contains(err.Error(), "read-only") {
			t.Errorf("git %v was refused: %v", args, err)
		}
	}
	if _, _, err := ro.Exec(dir, "git", "hash-object", "-w", "--stdin"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("git hash-object -w under ReadOnly: got %v, want a refusal", err)
	}
	if _, _, err := ro.Exec(dir, "git", "config", "--local", "--get-all", "user.name"); err == nil {
		// unset key: git exits 1, which is the inner git's answer, not a refusal
		t.Error("expected git's own non-zero exit for an unset key")
	} else if strings.Contains(err.Error(), "read-only") {
		t.Errorf("git config --get-all was refused: %v", err)
	}
}

func TestReadOnlyRefusesStartAndHTTPWrites(t *testing.T) {
	ro := ops.ReadOnly(ops.Real())
	if _, err := ro.Start(t.TempDir(), "true", nil, nil, nil); err == nil {
		t.Error("Start under ReadOnly was allowed")
	}
	if _, err := ro.HTTP().Do(nil); err == nil {
		t.Error("HTTP under ReadOnly was allowed")
	}
	// A GET or HEAD reaches the inner Doer; every other method is refused
	// before it does (docs/swarm.md section 14, item 31).
	var sent []string
	inner := &ops.Fake{HTTPFn: func() ops.Doer { return doerFunc(func(r *http.Request) { sent = append(sent, r.Method) }) }}
	ro = ops.ReadOnly(inner)
	for _, m := range []string{"GET", "HEAD", "POST", "PATCH", "PUT", "DELETE"} {
		req, _ := http.NewRequest(m, "https://forge.example.net/api/v1/user", nil)
		_, err := ro.HTTP().Do(req)
		if read := m == "GET" || m == "HEAD"; read != (err == nil) {
			t.Errorf("%s under ReadOnly: %v", m, err)
		} else if !read && !strings.Contains(err.Error(), "HTTP "+m+" https://forge.example.net/api/v1/user refused") {
			t.Errorf("%s refusal does not name the call: %v", m, err)
		}
	}
	if strings.Join(sent, " ") != "GET HEAD" {
		t.Errorf("reached the forge: %v", sent)
	}
}

type doerFunc func(*http.Request)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) {
	f(r)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestReadOnlyExecAllowsDockerPsOnly(t *testing.T) {
	var ran []string
	inner := &ops.Fake{ExecFn: func(_, name string, args ...string) ([]byte, []byte, error) {
		ran = append(ran, name+" "+strings.Join(args, " "))
		return nil, nil, nil
	}}
	ro := ops.ReadOnly(inner)
	if _, _, err := ro.Exec("", "docker", "ps", "--filter", "label=daisy.swarm"); err != nil {
		t.Errorf("docker ps under ReadOnly: %v", err)
	}
	for _, args := range [][]string{{"run", "img"}, {"stop", "c"}, {"rm", "c"}, {}} {
		if _, _, err := ro.Exec("", "docker", args...); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Errorf("docker %v under ReadOnly: %v", args, err)
		}
	}
	if strings.Join(ran, ";") != "docker ps --filter label=daisy.swarm" {
		t.Errorf("ran %q", ran)
	}
}

func TestFakeDefaultsToReal(t *testing.T) {
	f := &ops.Fake{GetenvFn: func(k string) string { return "v:" + k }}
	if got := f.Getenv("X"); got != "v:X" {
		t.Errorf("GetenvFn not used: %q", got)
	}
	if f.Now().IsZero() {
		t.Error("Now default returned the zero time")
	}
	dir := t.TempDir()
	if err := f.FS().WriteFile(filepath.Join(dir, "f"), []byte("a"), 0o644); err != nil {
		t.Errorf("FS default is not the real file system: %v", err)
	}
	if b, err := f.FS().ReadFile(filepath.Join(dir, "f")); err != nil || string(b) != "a" {
		t.Errorf("read back %q %v", b, err)
	}
}

func TestRealWriteFileIsAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "f")
	fs := ops.Real().FS()
	if err := fs.WriteFile(target, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(target)
	if err := fs.WriteFile(target, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := fs.ReadFile(target)
	if string(b) != "two" {
		t.Errorf("got %q", b)
	}
	// Atomic: a new file renamed over the old, never the old truncated and
	// rewritten in place, where a reader or a crash could see it half done.
	if after, _ := os.Stat(target); os.SameFile(before, after) {
		t.Error("the file was rewritten in place, not replaced")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %d entries", len(entries))
	}
}

// A child never sees the variables a git hook sets: every git call means
// the directory's repository and its own author (case 76 pins it through
// `log`).
func TestChildrenLoseGitHookVariables(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "Hook Author")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_DIR", "/nowhere/.git")
	t.Setenv("DAISY_KEEP_ME", "yes")
	out, _, err := ops.Real().Exec(t.TempDir(), "sh", "-c", `echo "[$GIT_AUTHOR_NAME][$GIT_CONFIG_COUNT][$GIT_CONFIG_KEY_0][$GIT_DIR][$DAISY_KEEP_ME]"`)
	if err != nil || strings.TrimSpace(string(out)) != "[][][][][yes]" {
		t.Errorf("child environment: %q %v", out, err)
	}
	var buf strings.Builder
	p, err := ops.Real().Start(t.TempDir(), "sh", nil, &buf, nil, "-c", `echo "[$GIT_AUTHOR_NAME]"`)
	if err != nil {
		t.Fatal(err)
	}
	p.Wait()
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("Start child environment: %q", buf.String())
	}
}

// CopyTree is `cp -a` over FS: files, modes and symlinks as they are, a
// directory's mode set after its contents (so a read-only one still fills).
func TestCopyTreeKeepsLinksAndModes(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	os.MkdirAll(filepath.Join(src, "sub", "locked"), 0o755)
	os.WriteFile(filepath.Join(src, "run.sh"), []byte("#!/bin/sh\n"), 0o750)
	os.WriteFile(filepath.Join(src, "sub", "locked", "f"), []byte("f"), 0o444)
	os.Chmod(filepath.Join(src, "sub", "locked"), 0o555)
	defer os.Chmod(filepath.Join(src, "sub", "locked"), 0o755)
	os.Symlink("../run.sh", filepath.Join(src, "sub", "link"))
	if err := ops.CopyTree(ops.Real().FS(), src, dst); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dst, "sub", "locked"), 0o755)
	if fi, _ := os.Stat(filepath.Join(dst, "run.sh")); fi.Mode().Perm() != 0o750 {
		t.Errorf("file mode %v", fi.Mode())
	}
	if fi, _ := os.Stat(filepath.Join(dst, "sub", "locked")); fi.Mode().Perm() != 0o555 {
		t.Errorf("dir mode %v", fi.Mode())
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "locked", "f")); string(b) != "f" {
		t.Errorf("content %q", b)
	}
	if l, err := os.Readlink(filepath.Join(dst, "sub", "link")); err != nil || l != "../run.sh" {
		t.Errorf("link %q %v", l, err)
	}
	if err := ops.CopyTree(ops.Real().FS(), src, dst); err == nil {
		t.Error("copying onto an existing destination succeeded")
	}
}

// CopyTreeUnderUmask is `cp -r`: each file and directory takes the
// source's mode under the umask (a new home, a skill's copy, L517), a
// read-only directory still filled; links stay links.
func TestCopyTreeUnderUmask(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	os.MkdirAll(filepath.Join(src, "sub", "locked"), 0o755)
	os.WriteFile(filepath.Join(src, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(filepath.Join(src, "sub", "notes"), []byte("n"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "locked", "f"), []byte("f"), 0o444)
	for p, m := range map[string]os.FileMode{"run.sh": 0o755, "sub": 0o755, "sub/notes": 0o644, "sub/locked/f": 0o444, "sub/locked": 0o555} {
		os.Chmod(filepath.Join(src, p), m) // the fixture's modes whatever the umask
	}
	defer os.Chmod(filepath.Join(src, "sub", "locked"), 0o755)
	os.Symlink("../run.sh", filepath.Join(src, "sub", "link"))
	old := syscall.Umask(0o027)
	defer syscall.Umask(old)
	if err := ops.CopyTreeUnderUmask(ops.Real().FS(), src, dst); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dst, "sub", "locked"), 0o755)
	for p, want := range map[string]os.FileMode{".": 0o750, "run.sh": 0o750, "sub": 0o750, "sub/notes": 0o640, "sub/locked": 0o550, "sub/locked/f": 0o440} {
		if fi, err := os.Stat(filepath.Join(dst, p)); err != nil {
			t.Error(err)
		} else if fi.Mode().Perm() != want {
			t.Errorf("%s: mode %v, want %v", p, fi.Mode().Perm(), want)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "locked", "f")); string(b) != "f" {
		t.Errorf("content %q", b)
	}
	if l, err := os.Readlink(filepath.Join(dst, "sub", "link")); err != nil || l != "../run.sh" {
		t.Errorf("link %q %v", l, err)
	}
	if err := ops.CopyTreeUnderUmask(ops.Real().FS(), src, dst); err == nil {
		t.Error("copying onto an existing destination succeeded")
	}
}

func TestMkdirIsExclusive(t *testing.T) {
	d := filepath.Join(t.TempDir(), "d")
	fs := ops.Real().FS()
	if err := fs.Mkdir(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fs.Mkdir(d, 0o700); err == nil {
		t.Error("second Mkdir succeeded")
	}
	if err := fs.Chmod(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(d); fi.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", fi.Mode())
	}
}

// One mode rule for every write: a new file gets perm under the umask, as
// `>` and `cp` make one; an existing file keeps its mode exactly, as `>`
// and `>>` keep it (WriteFile and Rewrite alike); CopyTree copies modes
// exactly, as `cp -a` (Step 22, D37).
func TestRealWriteFileHonoursTheUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := t.TempDir()
	fs := ops.Real().FS()
	if err := fs.WriteFile(filepath.Join(dir, "new"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wide := filepath.Join(dir, "wide")
	os.WriteFile(wide, []byte("x"), 0o666)
	os.Chmod(wide, 0o666)
	if err := ops.Rewrite(fs, wide, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ops.CopyTree(fs, wide, filepath.Join(dir, "copy")); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(dir, "kept")
	os.WriteFile(kept, []byte("x"), 0o644)
	os.Chmod(kept, 0o644) // the umask applied to the fixture too
	if err := fs.WriteFile(kept, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{"new": 0o600, "wide": 0o666, "copy": 0o666, "kept": 0o644} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if fi.Mode().Perm() != want {
			t.Errorf("%s: mode %v, want %v", name, fi.Mode().Perm(), want)
		}
	}
}

// IsTerminal is `[ -t 0 ]`: /dev/null is a character device but no
// terminal (review pass 1, F5); a pipe and a reader that is no file are none.
func TestIsTerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	for name, in := range map[string]io.Reader{"/dev/null": null, "a pipe": r, "a string": strings.NewReader("")} {
		if ops.IsTerminal(in) {
			t.Errorf("%s is a terminal", name)
		}
	}
	if tty, err := os.Open("/dev/tty"); err == nil {
		defer tty.Close()
		if !ops.IsTerminal(tty) {
			t.Error("/dev/tty is no terminal")
		}
	}
}

// Rewrite writes a link's target and keeps the link, the target created
// when it does not exist yet (a registry kept in dotfiles), as `>` does.
func TestRewriteWritesThroughLinks(t *testing.T) {
	dir := t.TempDir()
	fs := ops.Real().FS()
	os.WriteFile(filepath.Join(dir, "live"), []byte("old"), 0o644)
	os.Symlink(filepath.Join(dir, "live"), filepath.Join(dir, "a"))
	os.Symlink(filepath.Join(dir, "absent"), filepath.Join(dir, "b"))
	os.Symlink("rel-absent", filepath.Join(dir, "c"))
	// A relative link inside a linked directory reads from the real one.
	os.MkdirAll(filepath.Join(dir, "dotfiles", "daisy"), 0o755)
	os.WriteFile(filepath.Join(dir, "dotfiles", "homes"), []byte("old"), 0o644)
	os.Symlink("../homes", filepath.Join(dir, "dotfiles", "daisy", "homes"))
	os.Symlink("../gone", filepath.Join(dir, "dotfiles", "daisy", "gone"))
	os.MkdirAll(filepath.Join(dir, "config"), 0o755)
	os.Symlink(filepath.Join(dir, "dotfiles", "daisy"), filepath.Join(dir, "config", "daisy"))
	for link, target := range map[string]string{"a": "live", "b": "absent", "c": "rel-absent", "config/daisy/homes": "dotfiles/homes", "config/daisy/gone": "dotfiles/gone"} {
		if err := ops.Rewrite(fs, filepath.Join(dir, link), []byte("new"), 0o644); err != nil {
			t.Fatalf("%s: %v", link, err)
		}
		fi, _ := os.Lstat(filepath.Join(dir, link))
		b, _ := os.ReadFile(filepath.Join(dir, target))
		if fi.Mode()&os.ModeSymlink == 0 || string(b) != "new" {
			t.Errorf("%s: a link %v, %s holds %q", link, fi.Mode()&os.ModeSymlink != 0, target, b)
		}
	}
	for _, stray := range []string{"config/homes", "config/gone"} {
		if _, err := os.Lstat(filepath.Join(dir, stray)); err == nil {
			t.Errorf("a stray file at %s", stray)
		}
	}
}
