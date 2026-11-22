package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/env"
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

// A command's own flag after its arguments is refused as a usage error,
// exit 2, before the command runs: `log "msg" --co-author X` would
// otherwise put the flag into the journal and commit with no trailer.
// The same words with the flag leading reach the command.
func TestATrailingFlagIsRefused(t *testing.T) {
	const line = "Jane Doe <jane@example.net>"
	saved := table
	t.Cleanup(func() { table = saved })
	ran := ""
	table = slices.Clone(saved)
	for i := range table {
		name := table[i].Name
		table[i].Workspace = false // the stand-in writes nothing, so needs no workspace
		table[i].Run = func(*verb.Context) error { ran = name; return nil }
	}
	for _, tc := range []struct{ name, arg, lead string }{
		{"log", "Reviewed the change", "daisy log --co-author <x> <message...>"},
		{"commit", "Notes", "daisy commit --co-author <x> <message>"},
		{"done", "parser", "daisy done --co-author <x> <pattern>"},
	} {
		ran = ""
		code, _, errs := run(t, tc.name, tc.arg, "--co-author", line)
		want := "Error: --co-author follows the arguments; flags lead: " + tc.lead + "\n"
		if code != Usage || !strings.HasPrefix(errs, want) || ran != "" {
			t.Errorf("%s with a trailing --co-author: exit %d, ran %q, stderr %q; want exit 2, nothing run, %q", tc.name, code, ran, errs, want)
		}
		if code, _, errs := run(t, tc.name, "--co-author", line, tc.arg); code != OK || ran != tc.name {
			t.Errorf("%s with --co-author leading: exit %d, ran %q, stderr %q", tc.name, code, ran, errs)
		}
	}
}

// A command that cannot resolve its workspace prints the error with the
// `Error: ` prefix the errors rule gives (AGENTS.md, Errors), adding it
// to a lower-case error and never doubling one already there.
func TestInvokePrefixesAResolveError(t *testing.T) {
	saved := resolve
	t.Cleanup(func() { resolve = saved })
	for _, tc := range []struct{ err, want string }{
		{"the registry cannot be read", "Error: the registry cannot be read\n"},
		{"Error: Not in a Daisy workspace", "Error: Not in a Daisy workspace\n"},
	} {
		resolve = func(ops.Ops) (env.Env, error) { return env.Env{}, errors.New(tc.err) }
		if code, _, errs := run(t, "status"); code != Failure || errs != tc.want {
			t.Errorf("resolve error %q: exit %d, stderr %q; want exit 1, %q", tc.err, code, errs, tc.want)
		}
	}
}

// usagePart is a word of a usage line with its brackets and alternatives
// taken off: `[--todo|--done]` is `--todo`, `[<n>]` is `<n>`.
func usagePart(word string) string {
	part, _, _ := strings.Cut(strings.Trim(word, "[]()"), "|")
	return part
}

// Flags lead (splitArgs), so a usage line puts none after a positional
// placeholder (`eval [<case>] [--record]` read as if --record could
// follow), but for a command marked FlagsAfter, which reads its value
// flags after <n> itself. A placeholder right after a value flag is that
// flag's value, not a positional.
func TestUsageFlagsLead(t *testing.T) {
	for _, c := range table {
		if c.FlagsAfter {
			continue
		}
		words := strings.Fields(strings.TrimPrefix(c.Usage, c.Name))
		positional := ""
		for i := 0; i < len(words); i++ {
			w := words[i]
			if strings.Trim(w, "[]()|") == "" {
				continue // a bare `|` between forms
			}
			part := usagePart(w)
			if !strings.HasPrefix(part, "-") {
				positional = w
				continue
			}
			if positional != "" {
				t.Errorf("%s: usage %q puts %s after the positional %s", c.Name, c.Usage, w, positional)
			}
			for _, f := range strings.Split(strings.Trim(w, "[]()"), "|") {
				if slices.Contains(c.values(), f) && !strings.HasSuffix(w, "]") && i+1 < len(words) {
					i++ // the flag's value
					break
				}
			}
		}
	}
}
