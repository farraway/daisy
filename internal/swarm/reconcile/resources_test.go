package reconcile

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"daisy/internal/ops"
)

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
