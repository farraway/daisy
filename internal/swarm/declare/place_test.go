package declare

import (
	"os"
	"path/filepath"
	"testing"

	"daisy/internal/ops"
)

// Finding the swarm (docs/swarm.md section 11; section 13, Step 2's first
// row), on real temp folders: a swarm repository's clone with no .daisy/
// anywhere, found from its top and from a folder inside it, its .git a
// directory or a worktree's file; a clone inside a Daisy workspace; the
// old layout's swarm.json in <ws>/swarm/ no swarm (revision 6's plan, Step
// 7); init's folder, in a workspace or outside any; the not-found
// message.
func TestFind(t *testing.T) {
	mk := func(p string) string {
		t.Helper()
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	touch := func(p string) {
		t.Helper()
		mk(filepath.Dir(p))
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs := ops.Real().FS()
	find := func(cwd string, init bool) (Place, string) {
		p, err := Find(fs, cwd, init)
		if err != nil {
			return p, err.Error()
		}
		return p, ""
	}

	clone := mk(filepath.Join(t.TempDir(), "swarm-app"))
	mk(filepath.Join(clone, ".git"))
	touch(filepath.Join(clone, "swarm.json"))
	deep := mk(filepath.Join(clone, "briefs", "m0"))
	for _, cwd := range []string{clone, deep} {
		if p, err := find(cwd, false); err != "" || p != Clone(clone) || p.Root != clone || p.Outside {
			t.Errorf("clone from %s: %+v %q", cwd, p, err)
		}
	}

	worktree := mk(filepath.Join(t.TempDir(), "wt"))
	touch(filepath.Join(worktree, ".git")) // gitdir: …, as a linked worktree has
	touch(filepath.Join(worktree, "swarm.json"))
	if p, err := find(worktree, false); err != "" || p != Clone(worktree) {
		t.Errorf("worktree: %+v %q", p, err)
	}

	// The old layout, retired: a swarm.json in <ws>/swarm/ is no swarm, from the
	// workspace or from its swarm/ (no repository's top).
	ws := mk(t.TempDir())
	mk(filepath.Join(ws, ".daisy"))
	mk(filepath.Join(ws, ".git")) // a workspace may be a repository; its swarm/ is no clone's top
	touch(filepath.Join(ws, "swarm", "swarm.json"))
	for _, cwd := range []string{ws, filepath.Join(ws, "swarm"), mk(filepath.Join(ws, "notes"))} {
		want := "swarm.json: not found in " + cwd + " or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'"
		if p, err := find(cwd, false); err != want {
			t.Errorf("old layout from %s: %+v %q", cwd, p, err)
		}
	}

	// A clone inside a workspace: the clone.
	inner := mk(filepath.Join(ws, "swarm-app"))
	mk(filepath.Join(inner, ".git"))
	touch(filepath.Join(inner, "swarm.json"))
	if p, err := find(mk(filepath.Join(inner, "prompts")), false); err != "" || p != Clone(inner) {
		t.Errorf("clone in a workspace: %+v %q", p, err)
	}

	// A swarm.json without .git, outside any workspace: not found.
	loose := mk(t.TempDir())
	touch(filepath.Join(loose, "swarm.json"))
	want := "swarm.json: not found in " + loose + " or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'"
	if _, err := find(loose, false); err != want {
		t.Errorf("not found: %q\nwant %q", err, want)
	}
	// A workspace holding no clone, and a folder in no workspace:
	// init's folder, where it makes the clone (Outside); no other verb's.
	bare := mk(t.TempDir())
	mk(filepath.Join(bare, ".daisy"))
	sub := mk(filepath.Join(bare, "products"))
	for _, cwd := range []string{bare, sub, loose} {
		if p, err := find(cwd, true); err != "" || p != Folder(cwd) || !p.Outside || p.Root != cwd {
			t.Errorf("init in %s: %+v %q", cwd, p, err)
		}
	}
	if _, err := find(bare, false); err != "swarm.json: not found in "+bare+" or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'" {
		t.Errorf("status in a workspace without a swarm: %q", err)
	}
	// init in a clone, as every verb; in the old layout's workspace, its
	// folder.
	if p, err := find(deep, true); err != "" || p != Clone(clone) || p.Outside {
		t.Errorf("init in a clone: %+v %q", p, err)
	}
	if p, err := find(ws, true); err != "" || p != Folder(ws) {
		t.Errorf("init in the old layout's workspace: %+v %q", p, err)
	}
}
