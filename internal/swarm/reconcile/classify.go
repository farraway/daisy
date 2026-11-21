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
// go.work and every package directory. A head with none of them is
// another language's, whose output names no failing package.
type GoFacts struct {
	Modules    []GoModule
	Workspaces []string
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

// touched is every package directory the change touches: a changed
// file in it, below it or above it; or its importing, directly or
// through other packages, a package so touched, its test files'
// imports counted for itself alone (a package's tests are not built
// into its importers). A directory whose imports were not read imports
// one, so it is touched.
func (f GoFacts) touched(changed []string) map[string]bool {
	t := map[string]bool{}
	for d := range f.Packages {
		for _, c := range changed {
			cd := path.Dir(c)
			if cd == "." {
				cd = ""
			}
			if cd == d || within(cd, d) || within(d, cd) {
				t[d] = true
				break
			}
		}
	}
	for grew := true; grew; {
		grew = false
		for d, p := range f.Packages {
			if !t[d] && (p.Unread || f.importsTouched(p.Imports, t)) {
				t[d], grew = true, true
			}
		}
	}
	tested := map[string]bool{}
	for d, p := range f.Packages {
		if t[d] || f.importsTouched(p.TestImports, t) {
			tested[d] = true
		}
	}
	return tested
}

// importsTouched: one of imports is a touched package directory, each
// import path read as every module of the head and every vendor
// directory of a module or workspace could resolve it.
func (f GoFacts) importsTouched(imports []string, t map[string]bool) bool {
	roots := []string{""}
	for _, m := range f.Modules {
		roots = append(roots, m.Dir)
	}
	roots = append(roots, f.Workspaces...)
	for _, imp := range imports {
		for _, m := range f.Modules {
			if m.Path == "" {
				continue
			}
			if imp == m.Path && t[m.Dir] {
				return true
			}
			if rel, ok := strings.CutPrefix(imp, m.Path+"/"); ok && t[path.Join(m.Dir, rel)] {
				return true
			}
		}
		for _, r := range roots {
			if t[path.Join(r, "vendor", imp)] {
				return true
			}
		}
	}
	return false
}

// touches: the failing package p is touched, its directory in t, or it
// lies outside the root module: no root go.mod or none whose path is
// read, a path not below the root module's, in another module of the
// head (by its path or its directory) or at or below a go.work, or no
// package directory of the head.
func (f GoFacts) touches(p string, t map[string]bool) bool {
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
	return t[d]
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
	return facts, nil
}

// modulePath is the path of go.mod's module line, "" when it has none
// or one Go would not read.
func modulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != "module" {
			continue
		}
		if p, err := strconv.Unquote(f[1]); err == nil {
			return p
		}
		if strings.ContainsAny(f[1], "\"`") {
			return ""
		}
		return f[1]
	}
	return ""
}
