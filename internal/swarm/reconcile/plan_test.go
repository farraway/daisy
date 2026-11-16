package reconcile

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"daisy/internal/swarm/fence"
)

// The slices of a plan (section 7.5): the numbered lines under ## Steps
// only, a fenced block's lines excepted; each in a declared repository,
// and with one repository declared, a prefix naming no repository the
// swarm knows (no mirror) is the slice's text, the slice in that
// repository, while one naming a repository removed since is not.
func TestSliceRepos(t *testing.T) {
	plan := "## Research\n\n1. app: not a slice\n\n## Steps\n\n1. lib: first\n   2. app: indented, not a slice\n```\n2. app: fenced\n```\n2. app: second\n3. third\n\n## Test Plan\n\n1. app: a claim\n"
	if got := Slices(plan); !reflect.DeepEqual(got, []string{"lib", "app", ""}) {
		t.Errorf("Slices = %q", got)
	}
	for _, tc := range []struct {
		name  string
		plan  string
		names []string
		known []string
		repos []string
		bad   []int
	}{
		{"one unnamed with two declared", plan, []string{"app", "lib"}, nil, []string{"lib", "app", ""}, []int{3}},
		{"one unnamed with one declared", plan, []string{"app"}, []string{"app", "swarm-app"}, []string{"app", "app", "app"}, nil},
		{"a prefix naming no repository, one declared", "## Steps\n\n1. Parser: read the header.\n2. app: print it.\n", []string{"app"}, []string{"app", "swarm-app"}, []string{"app", "app"}, nil},
		{"a repository removed since, one declared", "## Steps\n\n1. lib: x.\n2. Parser: y.\n", []string{"app"}, []string{"app", "lib", "swarm-app"}, []string{"lib", "app"}, []int{1}},
		{"a prefix naming no repository, two declared", "## Steps\n\n1. Parser: read the header.\n", []string{"app", "lib"}, nil, []string{"Parser"}, []int{1}},
		{"no slice lines, one declared", "## Steps\n\nDo it.\n", []string{"app"}, nil, []string{"app"}, nil},
		{"no slice lines, two declared", "## Steps\n\nDo it.\n", []string{"app", "lib"}, nil, nil, []int{1}},
		{"an undeclared repository", "## Steps\n\n1. other: x\n", []string{"app", "lib"}, nil, []string{"other"}, []int{1}},
	} {
		repos, bad := SliceRepos(tc.plan, tc.names, tc.known)
		if !reflect.DeepEqual(repos, tc.repos) || !reflect.DeepEqual(bad, tc.bad) {
			t.Errorf("%s: %q %v", tc.name, repos, bad)
		}
	}
}

// Headings gain the level they sit under, at most six; a fence's lines,
// a `#tag` and a heading already at six stay.
func TestShift(t *testing.T) {
	in := "# A\n#tag\n## B\n~~~\n# in a fence\n~~~\n##### E\n###### F\n#\n   # three in\n    # code\n"
	want := "### A\n#tag\n#### B\n~~~\n# in a fence\n~~~\n###### E\n###### F\n###\n   ### three in\n    # code\n"
	if got := Shift(in, 2); got != want {
		t.Errorf("Shift:\n%s", got)
	}
	if got := Shift("a log:\n````\n# x\n```", 1); got != "a log:\n````\n# x\n```\n````" {
		t.Errorf("an open fence is not closed:\n%s", got)
	}
	// A fence line with an info string inside a block closes nothing
	// (CommonMark): the heading after it is the block's text.
	if got := Shift("```\n```go\n# in\n```\n# out", 1); got != "```\n```go\n# in\n```\n## out" {
		t.Errorf("an info string closed the fence:\n%s", got)
	}
}

// The plan's slices are read by the same fence: an info-string fence
// line inside a block does not end it, so a numbered line after it is
// no slice.
func TestSlicesInsideAFenceWithAnInfoLine(t *testing.T) {
	plan := "## Steps\n\n1. app: one\n   ```\n   ```mermaid\n9. app: not a slice\n   ```\n2. lib: two\n\n## Risks\n"
	if got := Slices(plan); !reflect.DeepEqual(got, []string{"app", "lib"}) {
		t.Errorf("Slices = %q", got)
	}
}

// FuzzShift: whatever the text, Shift keeps every line but a heading's
// as it is, gives each heading outside a fenced block the levels it is
// shifted by, at most six `#` in all, so it stays a heading, and leaves
// no fenced block open: the one line it may add closes the block the
// text left open (section 7.3).
func FuzzShift(f *testing.F) {
	for _, s := range []string{"# A\n#tag\n## B\n~~~\n# in a fence\n~~~\n#\n   # three in\n    # code\n", "a log:\n````\n# x\n```", "```\n```go\n# in\n```\n# out", "# Owner\r\n\r\nText.\r\n"} {
		f.Add(s, uint8(1))
	}
	f.Fuzz(func(t *testing.T, text string, by uint8) {
		n := int(by % 7)
		in, out := strings.Split(text, "\n"), strings.Split(Shift(text, n), "\n")
		if len(out) != len(in) && len(out) != len(in)+1 {
			t.Fatalf("Shift(%q, %d) has %d lines, not %d or one more", text, n, len(out), len(in))
		}
		var before, after fence.Scan
		for i, line := range in {
			want := line
			if lead, level, rest, ok := atxHeading(line); !before.Line(line) && ok {
				want = lead + strings.Repeat("#", min(6, level+n)) + rest
			}
			if out[i] != want {
				t.Fatalf("Shift(%q, %d): line %d is %q, want %q", text, n, i+1, out[i], want)
			}
		}
		if len(out) > len(in) && out[len(in)] != before.Open() {
			t.Fatalf("Shift(%q, %d) adds %q, not the open block's run %q", text, n, out[len(in)], before.Open())
		}
		for _, line := range out {
			after.Line(line)
		}
		if after.Open() != "" {
			t.Fatalf("Shift(%q, %d) leaves a %q block open", text, n, after.Open())
		}
	})
}

// atxHeading is line read as an ATX heading (CommonMark 0.31, 4.2): at
// most three spaces, one to six `#`, then the line's end (a trailing `\r`
// is a CRLF ending's), a space or a tab.
func atxHeading(line string) (lead string, level int, rest string, ok bool) {
	t := strings.TrimLeft(line, " ")
	lead = line[:len(line)-len(t)]
	level = len(t) - len(strings.TrimLeft(t, "#"))
	rest = t[level:]
	ok = len(lead) <= 3 && level >= 1 && level <= 6 && (rest == "" || rest == "\r" || rest[0] == ' ' || rest[0] == '\t')
	return lead, level, rest, ok
}

// FuzzSlices: a plan's slices name a repository or none, a plan written
// with CRLF endings has the slices it has with LF ones, and a plan whose
// `## Steps` lines name repositories has those repositories as its
// slices, in order, whatever a fenced block among them holds and
// whatever follows its next heading (section 7.5).
func FuzzSlices(f *testing.F) {
	f.Add("## Steps\n\n1. app: one\n   ```\n   ```mermaid\n9. app: not a slice\n   ```\n2. lib: two\n\n## Risks\n", "app lib", "the slice")
	f.Add("## Research\n\n1. app: no\n\n## Steps\n\n1. lib: first\n```\n2. app: fenced\n```\n3. third\n", "lib", "x: y")
	f.Fuzz(func(t *testing.T, plan, names, text string) {
		got := Slices(plan)
		for _, name := range got {
			if name != "" && !sliceName.MatchString(name) {
				t.Fatalf("Slices(%q) names %q", plan, name)
			}
		}
		if crlf := Slices(strings.ReplaceAll(plan, "\n", "\r\n")); !reflect.DeepEqual(crlf, got) {
			t.Fatalf("Slices(%q) = %q, but %q with CRLF endings", plan, got, crlf)
		}
		if strings.Contains(text, "\n") || strings.Contains(plan, "~~~~~~~~") {
			return
		}
		var want []string
		var b strings.Builder
		b.WriteString("## Steps\n\n~~~~~~~~\n" + plan + "\n~~~~~~~~\n")
		for _, name := range strings.Fields(names) {
			if sliceName.MatchString(name) {
				want = append(want, name)
				fmt.Fprintf(&b, "%d. %s: %s\n", len(want), name, text)
			}
		}
		b.WriteString("\n## Risks\n")
		if !strings.Contains(plan, "Steps") {
			b.WriteString(plan)
		}
		if got := Slices(b.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("Slices(%q) = %q, want %q", b.String(), got, want)
		}
	})
}

var sliceName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
