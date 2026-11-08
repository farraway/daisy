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
		// `cp -a` keeps the mode exactly; WriteFile applies the umask.
		if err := fsys.WriteFile(dst, b, fi.Mode().Perm()); err != nil {
			return err
		}
		return fsys.Chmod(dst, fi.Mode().Perm())
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
// is: a symlink's target is written, created when it does not exist yet,
// as the shell's `>` does (a file kept in dotfiles stays a link); the mode
// is WriteFile's rule (an existing file keeps its own, a new file gets
// perm under the umask).
func Rewrite(fsys FS, path string, data []byte, perm fs.FileMode) error {
	if real, err := fsys.EvalSymlinks(path); err == nil {
		return fsys.WriteFile(real, data, perm)
	}
	// A dangling link: follow it by hand, a relative target from the
	// link's real directory, as the kernel reads it.
	for range 40 { // the kernel's own limit on a chain of links
		fi, err := fsys.Lstat(path)
		if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
			break
		}
		target, err := fsys.Readlink(path)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(target) {
			dir := filepath.Dir(path)
			if real, err := fsys.EvalSymlinks(dir); err == nil {
				dir = real
			}
			target = filepath.Join(dir, target)
		}
		path = target
	}
	return fsys.WriteFile(path, data, perm)
}

// CheckTree reads, without writing, everything CopyTree would copy from
// src, so that a copy decided before a command's first write cannot fail
// on a file it cannot read halfway through.
func CheckTree(fsys FS, src string) error {
	fi, err := fsys.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		_, err := fsys.Readlink(src)
		return err
	case fi.Mode().IsRegular():
		_, err := fsys.ReadFile(src)
		return err
	case fi.IsDir():
		entries, err := fsys.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := CheckTree(fsys, filepath.Join(src, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("copying %s: not a file, directory or symlink", src)
}
