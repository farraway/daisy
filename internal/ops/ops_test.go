package ops_test

import (
	"io"
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
	if _, _, err := ro.Exec(dir, "git", "config", "--local", "--get-all", "user.name"); err == nil {
		// unset key: git exits 1, which is the inner git's answer, not a refusal
		t.Error("expected git's own non-zero exit for an unset key")
	} else if strings.Contains(err.Error(), "read-only") {
		t.Errorf("git config --get-all was refused: %v", err)
	}
}

func TestReadOnlyRefusesStartAndHTTP(t *testing.T) {
	ro := ops.ReadOnly(ops.Real())
	if _, err := ro.Start(t.TempDir(), "true", nil, nil, nil); err == nil {
		t.Error("Start under ReadOnly was allowed")
	}
	if _, err := ro.HTTP().Do(nil); err == nil {
		t.Error("HTTP under ReadOnly was allowed")
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
	if err := fs.WriteFile(target, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := fs.ReadFile(target)
	if string(b) != "two" {
		t.Errorf("got %q", b)
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
