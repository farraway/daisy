package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/verb"
)

// run is Run over a Fake that reads no variable of the machine's: the
// help and the usages need none.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cwd := t.TempDir()
	o := &ops.Fake{
		GetenvFn: func(string) string { return "" },
		GetwdFn:  func() (string, error) { return cwd, nil },
	}
	var out, errb bytes.Buffer
	code = Run(o, strings.NewReader(""), &out, &errb, args)
	return code, out.String(), errb.String()
}

// The marker is the table's: every rendering of a command the table runs
// under ReadOnly (its --help, the tree, the Command Reference) ends its
// description with ` (read-only)`, no other command's does, and no
// description carries it by hand.
func TestReadOnlyMarker(t *testing.T) {
	_, tree, _ := run(t)
	_, ref, _ := run(t, "help", "--markdown")
	marked := 0
	for _, c := range table {
		if strings.Contains(c.Help, "(read-only)") {
			t.Errorf("%s: (read-only) written into Help; set ReadOnly instead", c.Name)
		}
		desc := c.Help
		if c.ReadOnly {
			desc += " (read-only)"
			marked++
		}
		if code, out, _ := run(t, append(strings.Fields(c.Name), "--help")...); code != 0 || !strings.HasPrefix(out, "Usage: daisy "+c.Usage+"\n\n"+desc+"\n") {
			t.Errorf("%s --help: exit %d, want the description %q:\n%s", c.Name, code, desc, out)
		}
		if !strings.Contains(tree, " "+desc+"\n") {
			t.Errorf("%s: the tree has no line ending %q", c.Name, desc)
		}
		if row := " | " + strings.ReplaceAll(desc, "|", `\|`) + " |\n"; !strings.Contains(ref, row) {
			t.Errorf("%s: the Command Reference has no row ending %q", c.Name, row)
		}
	}
	if marked < 10 {
		t.Errorf("%d commands run under ReadOnly; the table has lost its marks", marked)
	}
}

// A command the table marks ReadOnly runs under ops.ReadOnly: the write
// it tries is refused, and the same command unmarked makes it.
func TestTableReadOnlyIsEnforced(t *testing.T) {
	file := filepath.Join(t.TempDir(), "written")
	write := func(c *verb.Context) error { return c.Ops.FS().WriteFile(file, []byte("x\n"), 0o644) }
	saved := table
	t.Cleanup(func() { table = saved })
	for _, readOnly := range []bool{true, false} {
		table = append(slices.Clone(saved), Command{Name: "probe", Usage: "probe", ReadOnly: readOnly, Run: write})
		code, _, errs := run(t, "probe")
		_, err := os.Stat(file)
		if readOnly && (code != Failure || err == nil) {
			t.Errorf("ReadOnly: exit %d, file there %v, err %q", code, err == nil, errs)
		}
		if !readOnly && (code != OK || err != nil) {
			t.Errorf("Real: exit %d, file there %v, err %q", code, err == nil, errs)
		}
	}
}

// exampleWord is a word of an example as a shell splits it: double quotes
// around a word with spaces, the only quoting the examples use.
var exampleWord = regexp.MustCompile(`"[^"]*"|[^ ]+`)

// Every command has an example, and it is real usage: its words name the
// command and pass the command's flags and arity as Run checks them
// (help's names a command), so an example cannot outlive a flag.
func TestExamplesParse(t *testing.T) {
	for _, c := range table {
		var words []string
		for _, w := range exampleWord.FindAllString(c.Example, -1) {
			words = append(words, strings.Trim(w, `"`))
		}
		got, rest, ok := match(words)
		if !ok || got.Name != c.Name {
			t.Errorf("%s: example %q names %q", c.Name, c.Example, got.Name)
			continue
		}
		if c.Name == "help" {
			if _, r, ok := match(rest); !ok || len(r) > 0 {
				t.Errorf("help: example %q names no command", c.Example)
			}
			continue
		}
		if err := parse(&verb.Context{}, c, rest); err != nil {
			t.Errorf("%s: example %q: %v", c.Name, c.Example, err)
		}
	}
}
