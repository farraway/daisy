package reconcile

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// appFacts is a head with a root module, two modules below the root
// (one under the root module's path, one under its own), a vendored
// dependency and a directory whose imports were not read.
func appFacts() GoFacts {
	return GoFacts{
		Modules: []GoModule{{"", "example.com/app"}, {"lib", "example.com/lib"}, {"tools", "example.com/app/tools"}},
		Packages: map[string]GoPackage{
			"api":                     {Imports: []string{"example.com/app/store", "net/http"}},
			"cli":                     {Imports: []string{"example.com/app/api"}},
			"legacy":                  {Unread: true},
			"lib":                     {},
			"report":                  {Imports: []string{"example.com/lib"}},
			"store":                   {Imports: []string{"fmt"}, TestImports: []string{"example.com/app/testutil", "testing"}},
			"testutil":                {},
			"tools/gen":               {},
			"vendor/github.com/dep/x": {},
			"web":                     {Imports: []string{"github.com/dep/x"}},
		},
	}
}

func fail(pkg string) string { return "FAIL\t" + pkg + "\t0.012s\n" }

const (
	appStore    = "example.com/app/store"
	appAPI      = "example.com/app/api"
	appWeb      = "example.com/app/web"
	appTestutil = "example.com/app/testutil"
)

// Section 7.9, The classification: a failure whose output names a
// failing package the change touches (by its directory, below it or
// above it, by its imports, its test files' included, or by lying
// outside the root module) is never classified, whatever signature the
// output holds; any other is classified when it is in untouched
// packages (the step having ended by itself) or holds a signature.
func TestClassify(t *testing.T) {
	rootPkg := appFacts()
	rootPkg.Packages[""] = GoPackage{}
	work := appFacts()
	work.Workspaces = []string{""}
	workBelow := appFacts()
	workBelow.Workspaces = []string{"web"}
	noRoot := appFacts()
	noRoot.Modules = noRoot.Modules[1:]
	unreadBelow := appFacts()
	unreadBelow.Modules[2].Path = ""
	unreadRoot := appFacts()
	unreadRoot.Modules[0].Path = ""
	storeGo := []string{"store/store.go"}
	refused := "    store_test.go:12: dial tcp 127.0.0.1:27017: connect: connection refused\n"
	for _, tc := range []struct {
		name     string
		out      string
		changed  []string
		facts    GoFacts
		timedOut bool
		want     Classification
	}{
		{"a touched package's connection refused", "--- FAIL: TestFind (5.00s)\n" + refused + "FAIL\n" + fail(appStore), storeGo, appFacts(), false, Classification{}},
		{"a timeout naming a touched FAIL", fail(appStore) + "--- FAIL: TestSlow\n    slow_test.go:3: read tcp: i/o timeout\n", storeGo, appFacts(), true, Classification{}},
		{"an untouched package", fail(appWeb), storeGo, appFacts(), false, Classification{Untouched: []string{appWeb}}},
		{"untouched packages and a signature", fail(appWeb) + "too many open files\n" + fail(appTestutil), storeGo, appFacts(), false, Classification{Untouched: []string{appWeb, appTestutil}, Signature: "too many open files"}},
		{"a package failing twice, named once", fail(appWeb) + fail(appWeb), storeGo, appFacts(), false, Classification{Untouched: []string{appWeb}}},
		{"one touched among untouched, with a signature", fail(appWeb) + refused + fail(appAPI), storeGo, appFacts(), false, Classification{}},
		{"touched by its directory", fail(appStore), storeGo, appFacts(), false, Classification{}},
		{"touched by an import", fail(appAPI), storeGo, appFacts(), false, Classification{}},
		{"touched through two imports", fail("example.com/app/cli") + refused, storeGo, appFacts(), false, Classification{}},
		{"touched by a test file's import", fail(appStore) + refused, []string{"testutil/util.go"}, appFacts(), false, Classification{}},
		{"a test file's import does not pass to an importer", fail(appAPI), []string{"testutil/util.go"}, appFacts(), false, Classification{Untouched: []string{appAPI}}},
		{"touched below its directory", fail(appStore), []string{"store/testdata/a.json"}, appFacts(), false, Classification{}},
		{"touched above its directory", fail(appWeb), []string{"go.sum"}, appFacts(), false, Classification{}},
		{"the root package touched by every change", fail("example.com/app") + refused, []string{"web/web.go"}, rootPkg, false, Classification{}},
		{"a sibling directory does not touch", fail(appWeb), []string{"webs/a.go", "we/b.go"}, appFacts(), false, Classification{Untouched: []string{appWeb}}},
		{"outside the root module", fail("example.com/other/pkg") + refused, storeGo, appFacts(), false, Classification{}},
		{"a module below the root, under the root's path", fail("example.com/app/tools/gen") + refused, storeGo, appFacts(), false, Classification{}},
		{"a module below the root, under its own path", fail("example.com/lib") + refused, storeGo, appFacts(), false, Classification{}},
		{"a module below the root, its path not read", fail("example.com/app/tools/gen") + refused, storeGo, unreadBelow, false, Classification{}},
		{"imported from a module below the root", fail("example.com/app/report"), []string{"lib/lib.go"}, appFacts(), false, Classification{}},
		{"imported from vendor", fail(appWeb), []string{"vendor/github.com/dep/x/x.go"}, appFacts(), false, Classification{}},
		{"a go.work at the root", fail(appWeb) + refused, storeGo, work, false, Classification{}},
		{"a go.work above the package", fail(appWeb) + refused, storeGo, workBelow, false, Classification{}},
		{"a go.work elsewhere", fail(appTestutil), storeGo, workBelow, false, Classification{Untouched: []string{appTestutil}}},
		{"no root module", fail(appWeb) + refused, storeGo, noRoot, false, Classification{}},
		{"the root module's path not read", fail(appWeb) + refused, storeGo, unreadRoot, false, Classification{}},
		{"a package not at the head", fail("example.com/app/gone") + refused, storeGo, appFacts(), false, Classification{}},
		{"a package whose imports were not read", fail("example.com/app/legacy") + refused, storeGo, appFacts(), false, Classification{}},
		{"no failing package, a signature", "store/store.go\nwrite tcp 10.0.0.2:5432: i/o timeout\n", storeGo, appFacts(), false, Classification{Signature: "i/o timeout"}},
		{"a signature in a passing test's own output", "=== RUN   TestDial\n    dial_test.go:9: want connection refused\n--- PASS: TestDial (0.00s)\nok  \texample.com/app/web\t0.1s\nstore/store.go: not formatted\n", storeGo, appFacts(), false, Classification{Signature: "connection refused"}},
		{"no failing package, no signature", "store/store.go: not formatted\n", storeGo, appFacts(), false, Classification{}},
		{"a timeout in untouched packages alone", fail(appWeb), storeGo, appFacts(), true, Classification{}},
		{"a timeout in untouched packages, a signature", fail(appWeb) + refused, storeGo, appFacts(), true, Classification{Signature: "connection refused"}},
		{"a timeout naming no package, a signature", "=== RUN TestMongo\nserver selection error: context deadline exceeded\n", storeGo, appFacts(), true, Classification{Signature: "server selection error"}},
		{"another language's FAIL line is no package", fail(appStore) + refused, storeGo, GoFacts{}, false, Classification{Signature: "connection refused"}},
		{"another language without a signature", "FAIL\tsrc/a.test.js\n", storeGo, GoFacts{}, false, Classification{}},
		{"the earliest signature", "Too many open files\nnetwork not found\n", storeGo, appFacts(), false, Classification{Signature: "too many open files"}},
		{"a FAIL line ending CRLF", "FAIL\texample.com/app/web\r\nFAIL\r\n", storeGo, appFacts(), false, Classification{Untouched: []string{appWeb}}},
		{"a FAIL line with no line end", refused + strings.TrimSuffix(fail(appStore), "\n"), storeGo, appFacts(), false, Classification{}},
	} {
		if got := Classify([]byte(tc.out), tc.changed, tc.facts, tc.timedOut); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// A failing package is read from a line `FAIL<tab><import path>`, with
// or without a time or a bracketed note, and from no other line: a
// touched package's line read holds back the signature beside it.
func TestClassifyFailLines(t *testing.T) {
	for _, tc := range []struct {
		line string
		read bool
	}{
		{"FAIL\texample.com/app/store\t0.012s", true},
		{"FAIL\texample.com/app/store [build failed]", true},
		{"FAIL\texample.com/app/store [setup failed]", true},
		{"FAIL\texample.com/app/store", true},
		{"FAIL\texample.com/app/store\t0.012s\r", true},
		{"FAIL\texample.com/app/store\r", true},
		{"FAIL", false},
		{"FAIL\t", false},
		{"--- FAIL: TestFind (0.00s)", false},
		{"    FAIL\texample.com/app/store\t0.012s", false},
		{"FAIL example.com/app/store", false},
		{"fail\texample.com/app/store", false},
		{"xFAIL\texample.com/app/store", false},
		{"ok  \texample.com/app/store\t0.012s", false},
	} {
		got := Classify([]byte("connection refused\n"+tc.line+"\n"), []string{"store/store.go"}, appFacts(), false)
		want := Classification{Signature: "connection refused"}
		if tc.read {
			want = Classification{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %+v, want %+v", tc.line, got, want)
		}
	}
}

// The five signatures, matched ignoring case anywhere in the whole
// output, and nothing near them.
func TestClassifySignatures(t *testing.T) {
	for _, tc := range []struct{ out, want string }{
		{"dial tcp: i/o timeout", "i/o timeout"},
		{"I/O TIMEOUT", "i/o timeout"},
		{"server selection error: context deadline exceeded", "server selection error"},
		{"Server Selection Error", "server selection error"},
		{"connect: connection refused", "connection refused"},
		{"Connection refused", "connection refused"},
		{"Error response from daemon: network not found", "network not found"},
		{"Network Not Found", "network not found"},
		{"accept4: too many open files", "too many open files"},
		{"Too many open files", "too many open files"},
		{"xconnection refusedx", "connection refused"},
		{"i/o  timeout", ""},
		{"io timeout", ""},
		{"i/o time out", ""},
		{"server selection", ""},
		{"server-selection error", ""},
		{"connection refuse", ""},
		{"connection-refused", ""},
		{"connection\nrefused", ""},
		{"network not-found", ""},
		{"network notfound", ""},
		{"networK not found", ""},
		{"too many open file", ""},
		{"too many  open files", ""},
		{"", ""},
	} {
		got := Classify([]byte(tc.out), []string{"store/store.go"}, appFacts(), false)
		if got.Signature != tc.want || got.Untouched != nil {
			t.Errorf("%q: %+v, want signature %q", tc.out, got, tc.want)
		}
	}
}

// `check rerun`'s reason (section 7.9): the untouched packages, the
// signature, or both separated by "; ".
func TestClassificationReason(t *testing.T) {
	for _, tc := range []struct {
		c    Classification
		want string
	}{
		{Classification{Untouched: []string{appWeb, appTestutil}}, "untouched packages example.com/app/web, example.com/app/testutil"},
		{Classification{Signature: "i/o timeout"}, `signature "i/o timeout"`},
		{Classification{Untouched: []string{appWeb}, Signature: "i/o timeout"}, `untouched packages example.com/app/web; signature "i/o timeout"`},
		{Classification{}, ""},
	} {
		if got := tc.c.Reason(); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.c, got, tc.want)
		}
	}
}

// fakeTree is an ops.Fake whose git answers `ls-tree -r -t -z <commit>
// -- .` and `cat-file blob <object>` in the mirror m from files, each
// path's mode and content (a mode 040000 a tree, 160000 a submodule).
func fakeTree(m, commit string, files map[string][2]string) *ops.Fake {
	return &ops.Fake{ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
		switch {
		case dir != m || name != "git":
		case reflect.DeepEqual(args, []string{"ls-tree", "-r", "-t", "-z", commit, "--", "."}):
			var b strings.Builder
			for p, f := range files {
				typ := map[string]string{"040000": "tree", "160000": "commit"}[f[0]]
				if typ == "" {
					typ = "blob"
				}
				b.WriteString(f[0] + " " + typ + " obj:" + p + "\t" + p + "\x00")
			}
			return []byte(b.String()), nil, nil
		case len(args) == 3 && args[0] == "cat-file" && args[1] == "blob" && strings.HasPrefix(args[2], "obj:"):
			if f, ok := files[strings.TrimPrefix(args[2], "obj:")]; ok {
				return []byte(f[1]), nil, nil
			}
		}
		return nil, []byte("fatal: no such thing"), errors.New("exit status 128")
	}}
}

// The facts are read from the head's tree in the mirror: each go.mod's
// module path, each go.work's directory, and each directory's imports
// from every .go file whatever its build tags (Go's parser, imports
// only), its test files' apart; a file whose imports cannot be read
// marks its directory.
func TestGoFacts(t *testing.T) {
	state := t.TempDir()
	g := Git{State: state}
	m := g.Mirror("app")
	files := map[string][2]string{
		"go.mod":               {"100644", "// the app\nmodule example.com/app // its path\n\ngo 1.27\n"},
		"go.work":              {"100644", "go 1.27\n\nuse .\n"},
		"main.go":              {"100644", "package main\n\nimport \"os\"\n"},
		"README.md":            {"100644", "import \"nothing\"\n"},
		"store":                {"040000", ""},
		"store/store.go":       {"100644", "package store\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n"},
		"store/store_linux.go": {"100755", "//go:build linux\n\npackage store\n\nimport \"golang.org/x/sys/unix\"\nimport \"fmt\"\n"},
		"store/store_test.go":  {"100644", "package store_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/app/store\"\n\tu \"example.com/app/testutil\"\n)\n"},
		"api":                  {"040000", ""},
		"api/api.go":           {"100644", "package api\n\nimport _ \"embed\"\nimport \"example.com/app/store\"\n\nfunc f() { this is not go }\n"},
		"broken":               {"040000", ""},
		"broken/broken.go":     {"100644", "packag broken\n"},
		"link":                 {"040000", ""},
		"link/l.go":            {"120000", "../api/api.go"},
		"web":                  {"040000", ""},
		"web/testdata":         {"040000", ""},
		"web/testdata/x.go":    {"100644", "package x\n\nimport \"net/http\"\n"},
		"tools":                {"040000", ""},
		"tools/go.mod":         {"100644", "module \"example.com/app/tools\"\n"},
		"tools/gen":            {"040000", ""},
		"tools/gen/gen.go":     {"100644", "package gen\n"},
		"odd":                  {"040000", ""},
		"odd/go.mod":           {"100644", "go 1.27\n"},
		"linked":               {"040000", ""},
		"linked/go.mod":        {"120000", "../go.mod"},
		"ext":                  {"160000", ""},
	}
	g.O = fakeTree(m, "abc123", files)
	got, err := g.GoFacts("app", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	want := GoFacts{
		Modules:    []GoModule{{"", "example.com/app"}, {"linked", ""}, {"odd", ""}, {"tools", "example.com/app/tools"}},
		Workspaces: []string{""},
		Packages: map[string]GoPackage{
			"":             {Imports: []string{"os"}},
			"api":          {Imports: []string{"embed", "example.com/app/store"}},
			"broken":       {Unread: true},
			"link":         {Unread: true},
			"store":        {Imports: []string{"fmt", "golang.org/x/sys/unix", "strings"}, TestImports: []string{"example.com/app/store", "example.com/app/testutil", "testing"}},
			"tools/gen":    {},
			"web/testdata": {Imports: []string{"net/http"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("facts\n%+v\nwant\n%+v", got, want)
	}
	if _, err := g.GoFacts("app", "def456"); err == nil || !strings.Contains(err.Error(), "ls-tree") {
		t.Errorf("an unread tree: %v", err)
	}
	delete(files, "main.go")
	files["store/store.go"] = [2]string{"100644", "package store\n"}
	for _, bad := range []string{"obj:store/store.go", "obj:tools/go.mod"} {
		g.O = &ops.Fake{ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			if args[0] == "cat-file" && args[2] == bad {
				return nil, []byte("fatal: bad object"), errors.New("exit status 128")
			}
			return fakeTree(m, "abc123", files).Exec(dir, name, args...)
		}}
		if _, err := g.GoFacts("app", "abc123"); err == nil || !strings.Contains(err.Error(), "cat-file blob "+bad) {
			t.Errorf("an unread %s: %v", bad, err)
		}
	}
}
