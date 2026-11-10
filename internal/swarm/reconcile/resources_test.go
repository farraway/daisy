package reconcile

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// A Brief: or Met: line's path (section 4): in a swarm repository's
// clone the old layout's workspace path of a brief of the swarm
// repository, <repo>/<path>, names the brief at <path>; any other path,
// and every path in the old layout, is read as it is.
func TestBriefName(t *testing.T) {
	c := declare.Config{Forge: declare.Forge{Repo: "acme/swarm-app"}}
	clone, old := declare.Clone("/c"), declare.Workspace("/ws")
	for _, tc := range []struct {
		p          declare.Place
		name, want string
	}{
		{clone, "swarm-app/briefs/a.md", "briefs/a.md"},
		{clone, "briefs/a.md", "briefs/a.md"},
		{clone, "lib:briefs/a.md", "lib:briefs/a.md"},
		{clone, "swarm-other/briefs/a.md", "swarm-other/briefs/a.md"},
		{clone, "swarm-app/", "swarm-app/"},
		{old, "swarm-app/briefs/a.md", "swarm-app/briefs/a.md"},
	} {
		if got := BriefName(tc.p, c, tc.name); got != tc.want {
			t.Errorf("%v %q: %q, want %q", tc.p.Old(), tc.name, got, tc.want)
		}
	}
}

// The checkout writes nothing outside its directory: an archive entry
// leaving it is refused, a link is written as a link.
func TestExtractStaysInside(t *testing.T) {
	archive := func(hs ...*tar.Header) []byte {
		var b bytes.Buffer
		w := tar.NewWriter(&b)
		for _, h := range hs {
			w.WriteHeader(h)
			if h.Size > 0 {
				w.Write(bytes.Repeat([]byte("x"), int(h.Size)))
			}
		}
		w.Close()
		return b.Bytes()
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := extract(ops.Real().FS(), archive(&tar.Header{Name: "../escape", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644}), out); err == nil {
		t.Error("an entry outside the checkout was written")
	}
	if _, err := os.Lstat(filepath.Join(dir, "escape")); err == nil {
		t.Error("escape written")
	}
	if err := extract(ops.Real().FS(), archive(&tar.Header{Name: "d/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "d/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		&tar.Header{Name: "d/run", Typeflag: tar.TypeReg, Size: 2, Mode: 0o755}), out); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(out, "d/link")); err != nil || target != "/etc/passwd" {
		t.Errorf("link: %q %v", target, err)
	}
	if fi, err := os.Stat(filepath.Join(out, "d/run")); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("run: %v %v", fi, err)
	}
}
