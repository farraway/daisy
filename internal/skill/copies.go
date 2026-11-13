package skill

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	iofs "io/fs"
	"path/filepath"
	"sort"
	"strings"

	"daisy/internal/ops"
)

// RootCopies sorts the skill directories in dir (the user's
// ~/.claude/skills, where `daisy install` once copied the root's skills) by
// whether an install left them. A directory named as a skill the root has
// or has had is a copy when it holds a SKILL.md and every file in it is,
// byte for byte, a version of that file in the root: its working tree, or
// any commit of its history under skills/. So a copy any root installed is
// one, however old, and so is a skill the root has since removed; one
// holding anything else (a user's edit, a file of their own) is kept, its
// owner's to remove. A skill of another name, ClaudeCodeSynced, a hidden
// name and a link are neither. A root that is not a repository has its
// working tree alone.
func RootCopies(o ops.Ops, root, dir string) (copies, kept []string) {
	fs := o.FS()
	v := rootVersions(o, root)
	entries, _ := fs.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".") || n == ClaudeCodeSynced || !v.names[n] {
			continue
		}
		if fi, err := fs.Lstat(filepath.Join(dir, n)); err != nil || !fi.IsDir() {
			continue
		}
		if v.isCopy(fs, filepath.Join(dir, n), n) {
			copies = append(copies, n)
		} else {
			kept = append(kept, n)
		}
	}
	sort.Strings(copies)
	sort.Strings(kept)
	return copies, kept
}

// versions are the root's skills as they are and have been: the names of
// the directories that have held a SKILL.md, and for each skill name and
// path under its directory, the object ids that file has had.
type versions struct {
	names  map[string]bool
	ids    map[string]map[string]bool // name "\x00" path → object ids
	sha256 bool
}

func rootVersions(o ops.Ops, root string) *versions {
	v := &versions{names: map[string]bool{}, ids: map[string]map[string]bool{}}
	// --relative: a root kept inside another repository reads its own
	// paths, relative to itself, as the root's.
	out, _, err := o.Exec(root, "git", "log", "--relative", "--format=", "--raw", "--no-abbrev", "--no-renames", "--", "skills")
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			meta, path, ok := strings.Cut(line, "\t")
			f := strings.Fields(meta)
			if !ok || !strings.HasPrefix(line, ":") || len(f) < 4 {
				continue
			}
			for _, id := range f[2:4] {
				if strings.Trim(id, "0") != "" {
					v.sha256 = len(id) == 64
					v.add(path, id)
				}
			}
		}
	}
	skills := filepath.Join(root, "skills")
	walk(o.FS(), skills, func(rel string, content []byte) {
		v.add("skills/"+rel, v.id(content))
	})
	return v
}

// add records id as a version of path (under skills/) for every directory
// on the way that could be the skill's: path skills/a/b/c.md is a's b/c.md
// and b's c.md.
func (v *versions) add(path, id string) {
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "skills" {
		return
	}
	if parts[len(parts)-1] == "SKILL.md" {
		v.names[parts[len(parts)-2]] = true
	}
	for i := 1; i < len(parts)-1; i++ {
		k := parts[i] + "\x00" + strings.Join(parts[i+1:], "/")
		if v.ids[k] == nil {
			v.ids[k] = map[string]bool{}
		}
		v.ids[k][id] = true
	}
}

// id is content's git object id as a blob, in the root's object format.
func (v *versions) id(content []byte) string {
	var h hash.Hash = sha1.New()
	if v.sha256 {
		h = sha256.New()
	}
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func (v *versions) isCopy(fs ops.FS, dir, name string) bool {
	if !isRegular(fs, filepath.Join(dir, "SKILL.md")) {
		return false
	}
	copy := true
	ok := walk(fs, dir, func(rel string, content []byte) {
		copy = copy && v.ids[name+"\x00"+rel][v.id(content)]
	})
	return ok && copy
}

// walk calls f with each file under dir, by its path there, and its
// content (a link's, its target, as git stores one); false when anything
// under dir cannot be read or is neither a file, a link nor a directory.
func walk(fs ops.FS, dir string, f func(rel string, content []byte)) bool {
	var visit func(rel string) bool
	visit = func(rel string) bool {
		entries, err := fs.ReadDir(filepath.Join(dir, rel))
		if err != nil {
			return false
		}
		for _, e := range entries {
			r := filepath.ToSlash(filepath.Join(rel, e.Name()))
			p := filepath.Join(dir, r)
			fi, err := fs.Lstat(p)
			switch {
			case err != nil:
				return false
			case fi.IsDir():
				if !visit(r) {
					return false
				}
			case fi.Mode()&iofs.ModeSymlink != 0:
				target, err := fs.Readlink(p)
				if err != nil {
					return false
				}
				f(r, []byte(target))
			case fi.Mode().IsRegular():
				b, err := fs.ReadFile(p)
				if err != nil {
					return false
				}
				f(r, b)
			default:
				return false
			}
		}
		return true
	}
	return visit("")
}
