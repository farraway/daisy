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
	in := "# A\n#tag\n## B\n~~~\n# in a fence\n~~~\n##### E\n###### F\n#\n"
	want := "### A\n#tag\n#### B\n~~~\n# in a fence\n~~~\n###### E\n###### F\n###\n"
	if got := Shift(in, 2); got != want {
		t.Errorf("Shift:\n%s", got)
	}
}
