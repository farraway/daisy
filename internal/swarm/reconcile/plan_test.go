package reconcile

import (
	"reflect"
	"testing"
)

// The slices of a plan (section 7.5): the numbered lines under ## Steps
// only, a fenced block's lines excepted; each in a declared repository.
func TestSliceRepos(t *testing.T) {
	plan := "## Research\n\n1. app: not a slice\n\n## Steps\n\n1. lib: first\n   2. app: indented, not a slice\n```\n2. app: fenced\n```\n2. app: second\n3. third\n\n## Test Plan\n\n1. app: a claim\n"
	if got := Slices(plan); !reflect.DeepEqual(got, []string{"lib", "app", ""}) {
		t.Errorf("Slices = %q", got)
	}
	for _, tc := range []struct {
		name  string
		plan  string
		names []string
		repos []string
		bad   []int
	}{
		{"one unnamed with two declared", plan, []string{"app", "lib"}, []string{"lib", "app", ""}, []int{3}},
		{"one unnamed with one declared", plan, []string{"app", "lib"}[:1], []string{"lib", "app", "app"}, []int{1}},
		{"no slice lines, one declared", "## Steps\n\nDo it.\n", []string{"app"}, []string{"app"}, nil},
		{"no slice lines, two declared", "## Steps\n\nDo it.\n", []string{"app", "lib"}, nil, []int{1}},
		{"an undeclared repository", "## Steps\n\n1. other: x\n", []string{"app", "lib"}, []string{"other"}, []int{1}},
	} {
		repos, bad := SliceRepos(tc.plan, tc.names)
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
