package reconcile

import (
	"errors"
	iofs "io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"daisy/internal/ops"
)

// memFS is an ops.FS held in memory for the lifecycle explorer, which
// clones a world at every state it reaches: a copy of two maps where a
// directory would be copied file by file. Its paths are absolute and
// slash-separated; it holds files and directories, no links, and Lock is
// a file written, the explorer running one pass at a time.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

var _ ops.FS = (*memFS)(nil)

func newMemFS() *memFS { return &memFS{files: map[string][]byte{}, dirs: map[string]bool{"/": true}} }

// clone is a copy that shares nothing with m.
func (m *memFS) clone() *memFS {
	c := &memFS{files: make(map[string][]byte, len(m.files)), dirs: maps.Clone(m.dirs)}
	for p, b := range m.files {
		c.files[p] = slices.Clone(b)
	}
	return c
}

func notExist(op, p string) error { return &iofs.PathError{Op: op, Path: p, Err: iofs.ErrNotExist} }

func (m *memFS) ReadFile(name string) ([]byte, error) {
	b, ok := m.files[path.Clean(name)]
	if !ok {
		return nil, notExist("open", name)
	}
	return slices.Clone(b), nil
}

func (m *memFS) WriteFile(name string, data []byte, _ os.FileMode) error {
	p := path.Clean(name)
	if !m.dirs[path.Dir(p)] || m.dirs[p] {
		return notExist("open", name)
	}
	m.files[p] = slices.Clone(data)
	return nil
}

func (m *memFS) MkdirAll(p string, _ os.FileMode) error {
	for p = path.Clean(p); !m.dirs[p]; p = path.Dir(p) {
		if _, file := m.files[p]; file {
			return &iofs.PathError{Op: "mkdir", Path: p, Err: errors.New("not a directory")}
		}
		m.dirs[p] = true
	}
	return nil
}

func (m *memFS) Mkdir(p string, _ os.FileMode) error {
	p = path.Clean(p)
	if _, file := m.files[p]; file || m.dirs[p] {
		return &iofs.PathError{Op: "mkdir", Path: p, Err: iofs.ErrExist}
	}
	if !m.dirs[path.Dir(p)] {
		return notExist("mkdir", p)
	}
	m.dirs[p] = true
	return nil
}

func (m *memFS) Chmod(name string, _ os.FileMode) error {
	if _, err := m.Lstat(name); err != nil {
		return err
	}
	return nil
}

// children is every path directly under dir.
func (m *memFS) children(dir string) []string {
	var out []string
	for _, set := range []map[string]bool{m.dirs, boolKeys(m.files)} {
		for p := range set {
			if p != "/" && path.Dir(p) == dir {
				out = append(out, p)
			}
		}
	}
	slices.Sort(out)
	return out
}

func boolKeys(files map[string][]byte) map[string]bool {
	out := make(map[string]bool, len(files))
	for p := range files {
		out[p] = true
	}
	return out
}

func (m *memFS) ReadDir(name string) ([]os.DirEntry, error) {
	dir := path.Clean(name)
	if !m.dirs[dir] {
		return nil, notExist("open", name)
	}
	var out []os.DirEntry
	for _, p := range m.children(dir) {
		fi, _ := m.Lstat(p)
		out = append(out, iofs.FileInfoToDirEntry(fi))
	}
	return out, nil
}

// memInfo is a file's or a directory's FileInfo.
type memInfo struct {
	name string
	size int64
	dir  bool
}

func (i memInfo) Name() string { return i.name }
func (i memInfo) Size() int64  { return i.size }
func (i memInfo) Mode() os.FileMode {
	if i.dir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return i.dir }
func (i memInfo) Sys() any           { return nil }

func (m *memFS) Lstat(name string) (os.FileInfo, error) {
	p := path.Clean(name)
	if b, ok := m.files[p]; ok {
		return memInfo{path.Base(p), int64(len(b)), false}, nil
	}
	if m.dirs[p] {
		return memInfo{path.Base(p), 0, true}, nil
	}
	return nil, notExist("lstat", name)
}

func (m *memFS) Stat(name string) (os.FileInfo, error) { return m.Lstat(name) }

var errNoLinks = errors.New("the explorer's file system holds no links")

func (m *memFS) Symlink(_, newname string) error {
	return &iofs.PathError{Op: "symlink", Path: newname, Err: errNoLinks}
}

func (m *memFS) Readlink(name string) (string, error) {
	return "", &iofs.PathError{Op: "readlink", Path: name, Err: errNoLinks}
}

// EvalSymlinks is the path itself, which no link can redirect.
func (m *memFS) EvalSymlinks(p string) (string, error) {
	if _, err := m.Lstat(p); err != nil {
		return "", err
	}
	return path.Clean(p), nil
}

func (m *memFS) Remove(name string) error {
	p := path.Clean(name)
	if _, ok := m.files[p]; ok {
		delete(m.files, p)
		return nil
	}
	if !m.dirs[p] {
		return notExist("remove", name)
	}
	if len(m.children(p)) > 0 {
		return &iofs.PathError{Op: "remove", Path: name, Err: errors.New("directory not empty")}
	}
	delete(m.dirs, p)
	return nil
}

// under: p is root or inside it.
func under(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }

func (m *memFS) RemoveAll(name string) error {
	root := path.Clean(name)
	maps.DeleteFunc(m.files, func(p string, _ []byte) bool { return under(p, root) })
	maps.DeleteFunc(m.dirs, func(p string, _ bool) bool { return p != "/" && under(p, root) })
	return nil
}

func (m *memFS) Rename(oldpath, newpath string) error {
	from, to := path.Clean(oldpath), path.Clean(newpath)
	if _, err := m.Lstat(from); err != nil {
		return err
	}
	if !m.dirs[path.Dir(to)] {
		return notExist("rename", newpath)
	}
	m.RemoveAll(to)
	for p, b := range maps.Clone(m.files) {
		if under(p, from) {
			delete(m.files, p)
			m.files[to+strings.TrimPrefix(p, from)] = b
		}
	}
	for p := range maps.Clone(m.dirs) {
		if under(p, from) {
			delete(m.dirs, p)
			m.dirs[to+strings.TrimPrefix(p, from)] = true
		}
	}
	return nil
}

func (m *memFS) Lock(name, holder string) (func(), error) {
	if err := m.WriteFile(name, []byte(holder+" pid 1\n"), 0o644); err != nil {
		return nil, err
	}
	return func() {}, nil
}

// memOps is an Ops whose file system is a memFS.
type memOps struct {
	*ops.Fake
	fs *memFS
}

func (o memOps) FS() ops.FS { return o.fs }
