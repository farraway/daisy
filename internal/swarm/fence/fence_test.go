package fence

import (
	"reflect"
	"testing"
)

// Each line's reading, CommonMark's: a fence closes only on a run of its
// own character at least as long with nothing after it but whitespace,
// so a fence line with an info string inside a block, a shorter run or
// the other character is the block's text; a line indented four spaces
// neither opens nor closes.
func TestLine(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  []bool
		open  string
	}{
		{"outside, open, inside, close", []string{"a", "```go", "x", "```", "b"}, []bool{false, true, true, true, false}, ""},
		{"an info string closes nothing", []string{"```", "```go", "## x", "```"}, []bool{true, true, true, true}, ""},
		{"an info string left open", []string{"```", "```go", "## x"}, []bool{true, true, true}, "```"},
		{"a shorter run or the other character", []string{"````", "```", "~~~~", "`````  \r", "y"}, []bool{true, true, true, true, false}, ""},
		{"tildes, trailing whitespace", []string{"~~~ sh", "x", "~~~~ \t", "y"}, []bool{true, true, true, false}, ""},
		{"indented four spaces is no fence", []string{"    ```", "x", "   ```", "y"}, []bool{false, false, true, true}, "```"},
		{"two backticks is no fence", []string{"``", "x"}, []bool{false, false}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s Scan
			var got []bool
			for _, l := range tc.lines {
				got = append(got, s.Line(l))
			}
			if !reflect.DeepEqual(got, tc.want) || s.Open() != tc.open {
				t.Errorf("%v, open %q; want %v, open %q", got, s.Open(), tc.want, tc.open)
			}
		})
	}
}
