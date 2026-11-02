package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// installRoot as a table: the binary's own root (<exe>/../.., through
// links) is the root; $DAISY_ROOT, when set, must name it or install
// refuses, naming both.
func TestInstallRoot(t *testing.T) {
	real := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	own, other, links := real(t.TempDir()), real(t.TempDir()), real(t.TempDir())
	for _, r := range []string{own, other} {
		if err := os.MkdirAll(filepath.Join(r, "templates"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(own, "bin", "daisy")
	linkedRoot, linkedExe := filepath.Join(links, "root"), filepath.Join(links, "daisy")
	os.Symlink(own, linkedRoot)
	os.Symlink(exe, linkedExe)
	for _, c := range []struct {
		name, env, exe string
		exeErr         error
		root           string   // wanted; "" when install must refuse
		errHas         []string // in the refusal
	}{
		{name: "unset: the binary's root", exe: exe, root: own},
		{name: "unset: the binary through a link", exe: linkedExe, root: own},
		{name: "equal: that root", env: own, exe: exe, root: own},
		{name: "equal through a link: as set", env: linkedRoot, exe: linkedExe, root: linkedRoot},
		{name: "differs: refused naming both", env: other, exe: exe, errHas: []string{other, own, "DAISY_ROOT=" + own + " " + own + "/bin/daisy install", "unset DAISY_ROOT"}},
		{name: "differs, absent: refused naming both", env: filepath.Join(other, "gone"), exe: exe, errHas: []string{filepath.Join(other, "gone"), own}},
		{name: "set, binary unknown: refused", env: own, exeErr: errors.New("no exe"), errHas: []string{own, "no exe", "cannot check"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := &ops.Fake{
				GetenvFn:     func(k string) string { return map[string]string{"DAISY_ROOT": c.env}[k] },
				ExecutableFn: func() (string, error) { return c.exe, c.exeErr },
			}
			root, err := installRoot(o)
			if c.root != "" {
				if err != nil || root != c.root {
					t.Errorf("root %q err %v, want %q", root, err, c.root)
				}
				return
			}
			if err == nil {
				t.Fatalf("not refused: root %q", root)
			}
			for _, s := range c.errHas {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("refusal lacks %q:\n%s", s, err)
				}
			}
		})
	}
}
