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
func CopyTree(fsys FS, src, dst string) error { return copyTree(fsys, src, dst, true) }

// CopyTreeUnderUmask is CopyTree as `cp -r` copies: each file and
// directory takes the source's mode under the umask, as anything new
// Daisy writes does. The copies a user is given to keep (a new home, a
// skill installed in a workspace) are made so.
func CopyTreeUnderUmask(fsys FS, src, dst string) error { return copyTree(fsys, src, dst, false) }

func copyTree(fsys FS, src, dst string, exact bool) error {
	fi, err := fsys.Lstat(src)
	if err != nil {
		return err
	}
	perm := fi.Mode().Perm()
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
		// A new file: WriteFile gives it perm under the umask.
		if err := fsys.WriteFile(dst, b, perm); err != nil {
			return err
		}
		if exact {
			return fsys.Chmod(dst, perm)
		}
		return nil
	case fi.IsDir():
		// The owner's rwx while it fills, under the umask like perm.
		if err := fsys.Mkdir(dst, perm|0o700); err != nil {
			return err
		}
		entries, err := fsys.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(fsys, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), exact); err != nil {
				return err
			}
		}
		if exact {
			return fsys.Chmod(dst, perm)
		}
		if perm&0o700 == 0o700 {
			return nil
		}
		// perm under the umask is what Mkdir gave less the owner's bits
		// perm lacks: the umask is read from its effect, never set.
		made, err := fsys.Stat(dst)
		if err != nil {
			return err
		}
		return fsys.Chmod(dst, made.Mode().Perm()&^(0o700&^perm))
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
