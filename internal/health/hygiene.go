package health

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Placeholders are the home names the documentation and the fixtures use;
// they identify nobody, so the scan does not look for them. AGENTS.md
// (Publication Hygiene) lists the same four.
var Placeholders = []string{"work", "personal", "example", "testhome"}

// Terms are what the publishable surface must not contain, derived from
// this machine (a hardcoded list would itself leak): every registered
// home's name but the placeholders, the git identity (user.name,
// user.email) of every usable home's repository and of the root's, $USER,
// and $HOME followed by a slash. Each term once, compared without case.
func Terms(o ops.Ops, e env.Env) []string {
	fs := o.FS()
	var terms []string
	add := func(t string) {
		if t != "" && !slices.ContainsFunc(terms, func(u string) bool { return strings.EqualFold(u, t) }) {
			terms = append(terms, t)
		}
	}
	identity := func(dir string) {
		for _, key := range []string{"user.name", "user.email"} {
			out, _, _ := o.Exec(dir, "git", "config", key)
			add(strings.TrimRight(string(out), "\n"))
		}
	}
	for _, h := range env.Homes(fs, e.HomesFile()) {
		if !slices.Contains(Placeholders, h.Name) {
			add(h.Name)
		}
		// The line a name resolves to is its first.
		if path, _ := env.HomePath(fs, e.HomesFile(), h.Name); env.HomePathProblem(fs, e.Root, filepath.Clean(path)) == "" {
			identity(filepath.Clean(path))
		}
	}
	add(o.Getenv("USER"))
	identity(e.Root)
	if home := o.Getenv("HOME"); home != "" && home != "/" {
		add(home + "/")
	}
	return terms
}

// Publishable is the root's publishable surface, root-relative: every path
// the root tracks (`ls-files --cached`, a gitlink's path included), then
// every untracked, unignored one (`--others --exclude-standard`) but those
// under a registered home inside the root (env.RootExclusions, which a
// root commit leaves out too). A tracked path is published wherever it lies. This
// is what a push publishes, so git's ignore rules are the one declaration
// of what does not.
func Publishable(o ops.Ops, e env.Env) ([]string, error) {
	list := func(args ...string) ([]string, error) {
		out, errb, err := o.Exec(e.Root, "git", append([]string{"ls-files", "-z"}, args...)...)
		if err != nil {
			return nil, fmt.Errorf("git ls-files in %s: %v %s", e.Root, err, strings.TrimSpace(string(errb)))
		}
		var paths []string
		for _, f := range strings.Split(string(out), "\x00") {
			if f != "" && !slices.Contains(paths, f) { // an unmerged path is listed once per stage
				paths = append(paths, f)
			}
		}
		return paths, nil
	}
	files, err := list("--cached")
	if err != nil {
		return nil, err
	}
	others, err := list("--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	// What the root tracks is published wherever it lies; only untracked
	// files under a home are the home's.
	excluded := env.RootExclusions(o.FS(), e)
	for _, f := range others {
		if !slices.ContainsFunc(excluded, func(ex string) bool { return f == ex || strings.HasPrefix(f, ex+"/") }) {
			files = append(files, f)
		}
	}
	return files, nil
}

// Staged is one index entry git would commit as a file or a link: its
// path, mode and object id (any stage, so a conflicted path is every
// side of it). Gitlinks are another repository's and are not listed.
type Staged struct{ Path, Mode, Object string }

// Index is every file and link entry in the root's index (`ls-files -s`).
// The scan reads the object of each one the worktree no longer matches —
// by id, never by `:path`, so neither assume-unchanged, skip-worktree, a
// conflict nor a path shaped like stage syntax hides it: a commit
// publishes the index, not the worktree.
func Index(o ops.Ops, root string) ([]Staged, error) {
	out, errb, err := o.Exec(root, "git", "ls-files", "-s", "-z")
	if err != nil {
		return nil, fmt.Errorf("git ls-files -s in %s: %v %s", root, err, strings.TrimSpace(string(errb)))
	}
	var entries []Staged
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || f[0] == "160000" {
			continue
		}
		entries = append(entries, Staged{Path: path, Mode: f[0], Object: f[1]})
	}
	return entries, nil
}

// blobID is git's object id for content: SHA-1, or SHA-256 when the
// repository's ids are that long.
func blobID(content []byte, idLen int) string {
	header := fmt.Sprintf("blob %d\x00", len(content))
	if idLen == 64 {
		sum := sha256.Sum256(append([]byte(header), content...))
		return hex.EncodeToString(sum[:])
	}
	sum := sha1.Sum(append([]byte(header), content...))
	return hex.EncodeToString(sum[:])
}

// worktreeID is the id the worktree's version of an entry would have: a
// link's text for a link, a regular file's content otherwise; "" when
// there is none.
func worktreeID(fs ops.FS, path string, s Staged) string {
	fi, err := fs.Lstat(path)
	if err != nil {
		return ""
	}
	if s.Mode == "120000" {
		if fi.Mode()&iofs.ModeSymlink == 0 {
			return ""
		}
		t, err := fs.Readlink(path)
		if err != nil {
			return ""
		}
		return blobID([]byte(t), len(s.Object))
	}
	if !fi.Mode().IsRegular() {
		return ""
	}
	b, err := fs.ReadFile(path)
	if err != nil {
		return ""
	}
	return blobID(b, len(s.Object))
}

// Texts are what a publishable path puts in front of a reader: a regular
// file's content; a symlink's link text, which is what git publishes for
// it, and, when it points at a file, that file's content too (as the bash
// scan read it). A directory, a missing path or any other kind is nothing.
// err is set for a regular file that exists and cannot be read: the scan
// cannot vouch for it.
func Texts(fs ops.FS, path string) (texts [][]byte, err error) {
	fi, lerr := fs.Lstat(path)
	if lerr != nil {
		return nil, nil
	}
	if fi.Mode()&iofs.ModeSymlink != 0 {
		target, err := fs.Readlink(path)
		if err != nil {
			return nil, err
		}
		texts = append(texts, []byte(target))
	}
	if st, err := fs.Stat(path); err == nil && st.Mode().IsRegular() {
		b, err := fs.ReadFile(path)
		switch {
		case err == nil:
			texts = append(texts, b)
		case fi.Mode()&iofs.ModeSymlink == 0:
			return texts, err
		}
		// A link's target that cannot be read is not published (the link
		// text is), so it is nothing to vouch for.
	}
	return texts, nil
}

// Contains: term occurs in text without regard to case, as a whole word
// at each edge of the term that is a word character (letter, digit, `_`):
// `heron` is not in `heronry`, while `/home/x/` is in `/home/x/src`, which
// `grep -w` would have let through.
func Contains(text []byte, term string) bool {
	t := bytes.ToLower([]byte(term))
	s := bytes.ToLower(text)
	if len(t) == 0 {
		return false
	}
	first, _ := utf8.DecodeRune(t)
	last, _ := utf8.DecodeLastRune(t)
	needLeft, needRight := isWord(first), isWord(last)
	for from := 0; from <= len(s)-len(t); {
		i := bytes.Index(s[from:], t)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(t)
		before, _ := utf8.DecodeLastRune(s[:start])
		after, _ := utf8.DecodeRune(s[end:])
		if (!needLeft || start == 0 || !isWord(before)) && (!needRight || end == len(s) || !isWord(after)) {
			return true
		}
		from = start + 1
	}
	return false
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// hygiene is Check 6c: one failure per term found, naming the files.
func (r *run) hygiene() bool {
	files, err := Publishable(r.o, r.e)
	if err != nil {
		r.fail("Publication hygiene: cannot list the publishable surface: %v", err)
		return false
	}
	type text struct {
		path  string
		texts [][]byte
	}
	index, err := Index(r.o, r.root)
	if err != nil {
		r.fail("Publication hygiene: cannot read the index: %v", err)
		return false
	}
	var read []text
	clean := true
	for _, f := range files {
		texts, err := Texts(r.fs, filepath.Join(r.root, f))
		if err != nil {
			r.fail("Publication hygiene: cannot read %s, so it cannot be vouched for: %v", f, err)
			clean = false
		}
		texts = append(texts, []byte(f)) // the path itself is published
		read = append(read, text{f, texts})
	}
	for _, en := range index {
		if worktreeID(r.fs, filepath.Join(r.root, en.Path), en) == en.Object {
			continue
		}
		blob, errb, err := r.o.Exec(r.root, "git", "cat-file", "blob", en.Object)
		if err != nil {
			r.fail("Publication hygiene: cannot read the staged %s (%s): %v %s", en.Path, en.Object, err, strings.TrimSpace(string(errb)))
			clean = false
			continue
		}
		i := slices.IndexFunc(read, func(t text) bool { return t.path == en.Path })
		if i < 0 {
			read = append(read, text{en.Path, [][]byte{[]byte(en.Path)}})
			i = len(read) - 1
		}
		read[i].texts = append(read[i].texts, blob)
	}
	for _, term := range Terms(r.o, r.e) {
		var hits []string
		for _, t := range read {
			if slices.ContainsFunc(t.texts, func(b []byte) bool { return Contains(b, term) }) {
				hits = append(hits, t.path)
			}
		}
		if len(hits) == 0 {
			continue
		}
		r.fail("Publication hygiene: '%s' appears in publishable file(s) — the publishable surface publishes verbatim (see AGENTS.md: Publication Hygiene)", term)
		for _, h := range hits {
			fmt.Fprintf(r.errw, "    %s\n", h)
		}
		clean = false
	}
	return clean
}
