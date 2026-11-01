package ops_test

import (
	"os"
	"path/filepath"
	"strings"
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
