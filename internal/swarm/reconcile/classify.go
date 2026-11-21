package reconcile

import (
	"bytes"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

// signatures are section 7.9's five infrastructure signatures, exactly,
// each matched ignoring case anywhere in the failed step's whole output.
var signatures = []string{"i/o timeout", "server selection error", "connection refused", "network not found", "too many open files"}

// GoModule is a go.mod of the head: its directory ("" the root's) and
// its module path ("" when its module line is not read).
type GoModule struct{ Dir, Path string }

// GoPackage is a directory of the head holding a .go file: the import
// paths its .go files import, its test files' (`_test.go`) apart, each
// sorted and once, read from every file whatever its build tags; Unread
// when a file's imports were not read, Go's parser refusing its package
// clause or import list (a symbolic link's text, read as git holds it,
// is refused so).
type GoPackage struct {
	Imports, TestImports []string
	Unread               bool
}

// GoFacts is what the classification knows of the head's Go code
// (section 7.9, The classification): its modules, the directory of each
// go.work, each submodule's (whose files the mirror does not hold) and
// every package directory. A head with none of them is
// another language's, whose output names no failing package.
type GoFacts struct {
	Modules    []GoModule
	Workspaces []string
	Submodules []string
	Packages   map[string]GoPackage
}

// Classification is a failed check classified as the environment's
// (section 7.9): by its untouched failing packages, by a signature, or
// both; neither when it is not classified.
type Classification struct {
	Untouched []string
	Signature string
}

// Classified: the failure is the environment's, to be run again once.
func (c Classification) Classified() bool { return len(c.Untouched) > 0 || c.Signature != "" }

// Reason is `check rerun`'s reason (section 7.9): `untouched packages
// <path>, …`, `signature "<signature>"` or both separated by `; `; ""
// for a failure not classified.
func (c Classification) Reason() string {
	var parts []string
	if len(c.Untouched) > 0 {
		parts = append(parts, "untouched packages "+strings.Join(c.Untouched, ", "))
	}
	if c.Signature != "" {
		parts = append(parts, `signature "`+c.Signature+`"`)
	}
	return strings.Join(parts, "; ")
}

// Classify is section 7.9's classification of a failed step's output
// out, given the paths the change alters (step 3's diff, a renamed file
// by both its paths), the head's Go facts, and whether the step timed
// out at the wall clock. A failure naming a failing package the change
// touches is never classified, whatever signature the output holds; any
// other is classified by its untouched failing packages when it names
// one and ended by itself, and by the earliest of the five signatures
// in the output. It reads nothing but its arguments.
func Classify(out []byte, changed []string, facts GoFacts, timedOut bool) Classification {
	failing := facts.failing(out)
	touched := facts.touched(changed)
	for _, p := range failing {
		if facts.touches(p, touched) {
			return Classification{}
		}
	}
	var c Classification
	if !timedOut && len(failing) > 0 {
		c.Untouched = failing
	}
	c.Signature = signature(out)
	return c
}

// failing is each package out names on a line `FAIL<tab><import path>`,
// with or without a time or a bracketed note after it, once, in the
// order first named; none for another language's head.
func (f GoFacts) failing(out []byte) []string {
	if len(f.Modules) == 0 && len(f.Workspaces) == 0 && len(f.Packages) == 0 {
		return nil
	}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSuffix(line, "\r"), "FAIL\t")
		if !ok {
			continue
		}
		if i := strings.IndexAny(rest, " \t"); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" && !slices.Contains(pkgs, rest) {
			pkgs = append(pkgs, rest)
		}
	}
	return pkgs
}

// touched is whether a directory the change touches: a changed file
// in it, below it or, for a package directory of the head, above it (a
// package the change deletes or moves away holds none, and is touched
// by its files, and so are its importers); a directory at or below a
// changed path (a submodule's, whose pointer is the change's path); or
// a package importing,
// directly or
// through other packages, a directory so touched, its test files'
// imports counted for itself alone (a package's tests are not built
// into its importers). A directory whose imports were not read imports
// one, so it is touched.
func (f GoFacts) touched(changed []string) func(string) bool {
	var dirs []string
	for _, c := range changed {
		cd := path.Dir(c)
		if cd == "." {
			cd = ""
		}
		dirs = append(dirs, cd, c)
	}
	t := map[string]bool{}
	near := func(d string) bool {
		if t[d] {
			return true
		}
		_, pkg := f.Packages[d]
		for i := 0; i < len(dirs); i += 2 {
			cd, c := dirs[i], dirs[i+1]
			if cd == d || within(cd, d) || pkg && within(d, cd) || d == c || within(d, c) {
				return true
			}
		}
		return false
	}
	for grew := true; grew; {
		grew = false
		for d, p := range f.Packages {
			if !t[d] && (p.Unread || near(d) || f.importsTouched(p.Imports, near)) {
				t[d], grew = true, true
			}
		}
	}
	tested := map[string]bool{}
	for d, p := range f.Packages {
		if t[d] || f.importsTouched(p.TestImports, near) {
			tested[d] = true
		}
	}
	return func(d string) bool { return tested[d] || near(d) }
}

// importsTouched: one of imports is a touched directory, each import
// path read as every module of the head and every vendor directory of
// a module or workspace could resolve it; and any import, when a
// directory whose import paths the reading does not know (a module
// below the root whose path is not read, a submodule) has a touched
// directory at or below it.
func (f GoFacts) importsTouched(imports []string, touched func(string) bool) bool {
	if len(imports) == 0 {
		return false
	}
	roots := []string{""}
	for _, m := range f.Modules {
		roots = append(roots, m.Dir)
	}
	for _, d := range f.opaque() {
		if f.subtreeTouched(d, touched) {
			return true
		}
	}
	roots = append(roots, f.Workspaces...)
	for _, imp := range imports {
		for _, m := range f.Modules {
			if m.Path == "" {
				continue
			}
			if imp == m.Path && touched(m.Dir) {
				return true
			}
			if rel, ok := strings.CutPrefix(imp, m.Path+"/"); ok && touched(path.Join(m.Dir, rel)) {
				return true
			}
		}
		for _, r := range roots {
			if touched(path.Join(r, "vendor", imp)) {
				return true
			}
		}
	}
	return false
}

// opaque is each directory whose import paths the reading does not
// know: a module below the root whose path is not read, a submodule.
func (f GoFacts) opaque() []string {
	var ds []string
	for _, m := range f.Modules {
		if m.Path == "" && m.Dir != "" {
			ds = append(ds, m.Dir)
		}
	}
	return append(ds, f.Submodules...)
}

// subtreeTouched: dir, or a package directory below it, is touched.
func (f GoFacts) subtreeTouched(dir string, touched func(string) bool) bool {
	if touched(dir) {
		return true
	}
	for d := range f.Packages {
		if within(d, dir) && touched(d) {
			return true
		}
	}
	return false
}

// touches: the failing package p is touched, its directory touched, or it
// lies outside the root module: no root go.mod or none whose path is
// read, a path not below the root module's, in another module of the
// head (by its path or its directory) or at or below a go.work, or no
// package directory of the head (a submodule's packages among them).
func (f GoFacts) touches(p string, touched func(string) bool) bool {
	i := slices.IndexFunc(f.Modules, func(m GoModule) bool { return m.Dir == "" })
	if i < 0 || f.Modules[i].Path == "" {
		return true
	}
	root := f.Modules[i].Path
	d, ok := "", p == root
	if !ok {
		d, ok = strings.CutPrefix(p, root+"/")
	}
	if !ok {
		return true
	}
	for _, m := range f.Modules {
		if m.Dir == "" {
			continue
		}
		if m.Path != "" && (p == m.Path || strings.HasPrefix(p, m.Path+"/")) || d == m.Dir || within(d, m.Dir) {
			return true
		}
	}
	for _, w := range f.Workspaces {
		if d == w || within(d, w) {
			return true
		}
	}
	if _, ok := f.Packages[d]; !ok {
		return true
	}
	return touched(d)
}

// within: directory a lies below directory b ("" the root).
func within(a, b string) bool {
	if b == "" {
		return a != ""
	}
	return strings.HasPrefix(a, b+"/")
}

// signature is the signature that comes first in out, matched ignoring
// ASCII case (the signatures are ASCII, so no other letter folds to
// one of theirs); "" when out holds none.
func signature(out []byte) string {
	lower := bytes.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, out)
	best, at := "", -1
	for _, s := range signatures {
		if i := bytes.Index(lower, []byte(s)); i >= 0 && (at < 0 || i < at) {
			best, at = s, i
		}
	}
	return best
}

// GoFacts reads the facts of the classification (section 7.9) from
// commit in the repository's mirror, with git's read commands alone
// (ls-tree, cat-file): each go.mod's module path, each go.work's
// directory, and every .go file's imports by Go's parser (imports only),
// whatever its build tags, its test files' apart.
func (g Git) GoFacts(name, commit string) (GoFacts, error) {
	m := g.Mirror(name)
	entries, err := g.Tree(m, commit, ".")
	if err != nil {
		return GoFacts{}, err
	}
	facts := GoFacts{Packages: map[string]GoPackage{}}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.Type == "commit" {
			facts.Submodules = append(facts.Submodules, e.Path)
		}
		if e.Type != "blob" {
			continue
		}
		dir, base := path.Split(e.Path)
		dir = strings.TrimSuffix(dir, "/")
		switch {
		case base == "go.mod":
			data, err := g.Blob(m, e.Object)
			if err != nil {
				return GoFacts{}, err
			}
			facts.Modules = append(facts.Modules, GoModule{dir, modulePath(data)})
		case base == "go.work":
			facts.Workspaces = append(facts.Workspaces, dir)
		case strings.HasSuffix(base, ".go"):
			p := facts.Packages[dir]
			data, err := g.Blob(m, e.Object)
			if err != nil {
				return GoFacts{}, err
			}
			file, err := parser.ParseFile(fset, e.Path, data, parser.ImportsOnly)
			if err != nil {
				p.Unread = true
			}
			if file != nil {
				for _, s := range file.Imports {
					imp, err := strconv.Unquote(s.Path.Value)
					switch {
					case err != nil:
						p.Unread = true
					case strings.HasSuffix(base, "_test.go"):
						p.TestImports = append(p.TestImports, imp)
					default:
						p.Imports = append(p.Imports, imp)
					}
				}
			}
			facts.Packages[dir] = p
		}
	}
	for d, p := range facts.Packages {
		slices.Sort(p.Imports)
		slices.Sort(p.TestImports)
		p.Imports, p.TestImports = slices.Compact(p.Imports), slices.Compact(p.TestImports)
		facts.Packages[d] = p
	}
	slices.SortFunc(facts.Modules, func(a, b GoModule) int { return strings.Compare(a.Dir, b.Dir) })
	slices.Sort(facts.Workspaces)
	slices.Sort(facts.Submodules)
	return facts, nil
}

// modulePath is the path of go.mod's module line, or of the first line
// of a `module ( … )` block, "" when it has none or one Go would not
// read.
func modulePath(data []byte) string {
	block := false
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		switch {
		case block && len(f) == 0:
			continue
		case block && len(f) == 1 && f[0] != ")":
			return pathWord(f[0])
		case block:
			return ""
		case len(f) == 2 && f[0] == "module" && f[1] == "(":
			block = true
		case len(f) == 2 && f[0] == "module":
			return pathWord(f[1])
		}
	}
	return ""
}

// pathWord is a module path as go.mod writes it, quoted or not; "" for
// a word Go would not read as one.
func pathWord(w string) string {
	if p, err := strconv.Unquote(w); err == nil {
		return p
	}
	if strings.ContainsAny(w, "\"`()") {
		return ""
	}
	return w
}
