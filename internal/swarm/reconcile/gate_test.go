package reconcile

import (
	"strings"
	"testing"
)

// The claim of Step S7a of the plan implementing the 2026-11-19 swarm
// roadmap (section 5, The gate note): a quoted line over 120 characters
// is cut before a Markdown link the 119-character cut would split, so the
// link is kept whole or left out; a link that ends within the 119 stays,
// a line of 120 or fewer is as it was, and a line with no link is cut at
// 119 with `…`, characters counted as runes.
func TestCutKeepsALinkWhole(t *testing.T) {
	link := "[acme/app#12](https://forge.example.net/acme/app/pulls/12)" // 58 runes
	for _, tc := range []struct{ name, in, want string }{
		{"a link across the cut", strings.Repeat("a", 100) + " see " + link + " now", strings.Repeat("a", 100) + " see …"},
		{"runes before it", strings.Repeat("é", 100) + " see " + link + " now", strings.Repeat("é", 100) + " see …"},
		{"a link ending within", strings.Repeat("a", 60) + link + strings.Repeat("b", 10), strings.Repeat("a", 60) + link + "b…"},
		{"a link after the cut", strings.Repeat("a", 125) + link, strings.Repeat("a", 119) + "…"},
		{"no link", strings.Repeat("a", 130), strings.Repeat("a", 119) + "…"},
		{"120 or fewer", strings.Repeat("a", 62) + link, strings.Repeat("a", 62) + link},
	} {
		if got := cut(tc.in); got != tc.want {
			t.Errorf("%s:\n%q\nwant\n%q", tc.name, got, tc.want)
		}
	}
}
