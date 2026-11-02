package ops

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// CopyTree copies the directory src to dst, which must not exist, as
// `cp -a` does: regular files with their modes, symlinks as links,
// directories with their modes set once their contents are in (so a
// read-only directory is still filled). Anything else is an error.
func CopyTree(fsys FS, src, dst string) error {
	fi, err := fsys.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := fsys.Readlink(src)
		if err != nil {
			return err
		}
		return fsys.Symlink(target, dst)
	case fi.Mode().IsRegular():
		if _, err := fsys.Lstat(dst); err == nil {
			return fmt.Errorf("copying %s: %s already exists", src, dst)
		}
		b, err := fsys.ReadFile(src)
		if err != nil {
			return err
		}
		return fsys.WriteFile(dst, b, fi.Mode().Perm())
	case fi.IsDir():
		if err := fsys.Mkdir(dst, 0o700); err != nil {
			return err
		}
		entries, err := fsys.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := CopyTree(fsys, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return fsys.Chmod(dst, fi.Mode().Perm())
	}
	return fmt.Errorf("copying %s: not a file, directory or symlink", src)
}

// Rewrite replaces the content of path atomically, keeping what the file
// is: a symlink's target is written (a file kept in dotfiles stays a
// link) and an existing file keeps its mode; a new file gets perm.
func Rewrite(fsys FS, path string, data []byte, perm fs.FileMode) error {
	if real, err := fsys.EvalSymlinks(path); err == nil {
		path = real
	}
	if fi, err := fsys.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	return fsys.WriteFile(path, data, perm)
}
