package main_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// clock is what the time package offers that reads or waits on the clock:
// ops.Ops has Now and After, and the rest are those two in disguise.
var clock = map[string]bool{
	"Now": true, "Since": true, "Until": true, "After": true, "AfterFunc": true,
	"Sleep": true, "Tick": true, "NewTimer": true, "NewTicker": true,
}

// layer is a package's place in the dependency rule (AGENTS.md, Layout
// and the dependency rule): an import goes to a lower layer, and between
// domain packages (layer 1, internal/verb among them) either way; the two
// command packages, internal/commands and the swarm's internal/swarm/cmd,
// share layer 2 and import neither the other.
func layer(pkg string) int {
	switch pkg {
	case "cmd/daisy":
		return 4
	case "internal/cli":
		return 3
	case "internal/commands", "internal/swarm/cmd":
		return 2
	case "internal/ops":
		return 0
	}
	return 1
}

// TestDependencyRule holds AGENTS.md's dependency rule over every non-test
// Go file under cmd/ and internal/: imports run cli → {commands,
// swarm/cmd} → {domain packages} → ops, and nothing above ops imports os
// (or os/signal, os/user), names any of os/exec but its ExitError type, or
// reads or waits on the clock through the time package. internal/testenv sets the environment of
// a test process, and cmd/daisy/main.go hands the process's streams to
// cli.Run: those two import os. No non-test file imports internal/testenv.
func TestDependencyRule(t *testing.T) {
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, v := range ruleViolations(fset, filepath.ToSlash(path), f) {
				t.Error(v)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ruleViolations(fset *token.FileSet, path string, f *ast.File) []string {
	pkg := filepath.ToSlash(filepath.Dir(path))
	open := pkg == "internal/ops" // the one package the world is open to
	var out []string
	fail := func(n ast.Node, format string, args ...any) {
		out = append(out, fset.Position(n.Pos()).String()+": "+fmt.Sprintf(format, args...)+" (AGENTS.md, the dependency rule)")
	}
	watched := map[string]string{} // local name → import path, of the os family and time
	for _, spec := range f.Imports {
		imp, _ := strconv.Unquote(spec.Path.Value)
		switch {
		case imp == "os" || strings.HasPrefix(imp, "os/") && imp != "os/exec":
			if open || pkg == "internal/testenv" || path == "cmd/daisy/main.go" {
				continue
			}
			fail(spec, "imports %s outside internal/ops: go through ops.Ops", imp)
			fallthrough
		case !open && (imp == "os/exec" || imp == "time"):
			name := filepath.Base(imp)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name == "." {
				fail(spec, "dot-imports %s, which hides its calls from this check", imp)
			}
			watched[name] = imp
		case strings.HasPrefix(imp, "daisy/"):
			to := strings.TrimPrefix(imp, "daisy/")
			if to == "internal/testenv" {
				fail(spec, "imports internal/testenv, which sets a test process's environment: only a _test.go file imports it")
				continue
			}
			if a, b := layer(pkg), layer(to); b > a || b == a && a != 1 {
				fail(spec, "%s imports %s: imports run cli → {commands, swarm/cmd} → {domain packages} → ops", pkg, to)
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch name, imp := sel.Sel.Name, watched[x.Name]; {
		case imp == "os/exec" && name != "ExitError":
			fail(sel, "uses %s.%s outside internal/ops: run it through ops.Ops's Exec", x.Name, name)
		case imp == "time" && clock[name]:
			fail(sel, "uses %s.%s outside internal/ops: the clock is ops.Ops's Now and After", x.Name, name)
		case imp != "" && imp != "os/exec" && imp != "time":
			fail(sel, "uses %s.%s outside internal/ops: go through ops.Ops", x.Name, name)
		}
		return true
	})
	return out
}

// TestTestingDocQuotesTheSkill holds docs/testing.md's Part 1 to what it
// says it is: skills/testing/SKILL.md without its frontmatter, verbatim but
// for its headings, each one level deeper under the Part's own. The text
// between the Part's heading and its closing --- is compared whole, blank
// lines at its edges included.
func TestTestingDocQuotesTheSkill(t *testing.T) {
	skill, err := os.ReadFile("skills/testing/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut(strings.TrimPrefix(string(skill), "---\n"), "\n---\n")
	if !ok || !strings.HasPrefix(string(skill), "---\n") {
		t.Fatal("skills/testing/SKILL.md: no frontmatter between two --- lines")
	}
	var want []string
	fence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "#") {
			line = "#" + line
		}
		want = append(want, line)
	}
	doc, err := os.ReadFile("docs/testing.md")
	if err != nil {
		t.Fatal(err)
	}
	const start, end = "\n## Part 1 — The skill\n", "\n---\n\n## Part 2"
	before, part, ok := strings.Cut(string(doc), start)
	if ok {
		part, _, ok = strings.Cut(part, end)
	}
	if !ok {
		t.Fatalf("docs/testing.md: no Part 1 between %q and %q", start, end)
	}
	first := strings.Count(before, "\n") + 3
	got := strings.Split(part, "\n")
	for i := 0; i < max(len(got), len(want)); i++ {
		g, w := "(end of Part 1)", "(end of the skill)"
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Fatalf("docs/testing.md:%d: Part 1 has %q where skills/testing/SKILL.md has %q: copy the skill into Part 1 again, its headings one level deeper", first+i, g, w)
		}
	}
}

// TestChmodTestsSkipAsRoot holds every test that takes a file's or a
// directory's permissions away (a Chmod to mode 0) to skip when run as
// root, who reads, writes and removes it all the same: each function in a
// _test.go file under cmd/ and internal/ that calls Chmod with the mode 0
// calls testenv.SkipAsRoot too.
func TestChmodTestsSkipAsRoot(t *testing.T) {
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				var chmods []ast.Node
				skips := false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == "testenv" && sel.Sel.Name == "SkipAsRoot" {
						skips = true
					}
					if sel.Sel.Name != "Chmod" || len(call.Args) != 2 {
						return true
					}
					if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.INT {
						if v, err := strconv.ParseInt(lit.Value, 0, 64); err == nil && v == 0 {
							chmods = append(chmods, call)
						}
					}
					return true
				})
				if !skips {
					for _, c := range chmods {
						t.Errorf("%s: %s sets mode 0 but does not call testenv.SkipAsRoot(t): run as root, the test reads and writes the file all the same", fset.Position(c.Pos()), fn.Name.Name)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
