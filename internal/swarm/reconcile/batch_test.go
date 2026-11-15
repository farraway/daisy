package reconcile

import (
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// A proposal's title is 1 to 200 characters, not bytes (section 7.5): 150
// two-byte letters are a title; 201 letters are body text.
func TestProposalTitleInCharacters(t *testing.T) {
	long := strings.Repeat("é", 150)
	_, _, _, ps := Proposals("## Short job\n\nBody one.\n\n## " + long + "\n\nBody two.\n")
	if len(ps) != 2 || ps[1].Title != long || ps[1].Body != "Body two." {
		t.Errorf("150 characters, 300 bytes: %+v", ps)
	}
	_, _, _, ps = Proposals("## Short job\n\n## " + strings.Repeat("a", 201) + "\n")
	if len(ps) != 1 || !strings.Contains(ps[0].Body, strings.Repeat("a", 201)) {
		t.Errorf("201 characters: %+v", ps)
	}
}

// A first line of BLOCKED: alone is a block, with no question after it
// (section 7.5), not "proposed nothing".
func TestProposalsBareBlocked(t *testing.T) {
	met, blocked, isBlocked, ps := Proposals("Met: docs/b.md\n\nBLOCKED:\n")
	if !isBlocked || blocked != "" || len(ps) != 0 || len(met) != 1 {
		t.Errorf("met %q blocked %q %v proposals %+v", met, blocked, isBlocked, ps)
	}
}

// The result file is read line by line: a line Append writes stays one,
// whatever it quotes (a member's path may hold a newline).
func TestAppendKeepsOneLine(t *testing.T) {
	r := Run{Dir: t.TempDir()}
	fs := ops.Real().FS()
	if err := r.Append(fs, "failed: protected path changed: .gitea/x\ncost 4242.5", "outcome failed"); err != nil {
		t.Fatal(err)
	}
	lines, _ := r.Result(fs)
	if len(lines) != 2 || lines[0] != `failed: protected path changed: .gitea/x\ncost 4242.5` {
		t.Errorf("%q", lines)
	}
}

// WaitingOn reads the issues a batch's BLOCKED: names in its fixed form,
// `waiting on #<n>, #<m>: <why>`, and nothing from any other.
func TestWaitingOn(t *testing.T) {
	for blocked, want := range map[string][]int{
		"waiting on #99, #100: M2 needs them": {99, 100},
		"waiting on #7: one":                  {7},
		"waiting on #7,#8 , #9: spaced":       {7, 8, 9},
		"waiting on #7 and #8: not the form":  nil,
		"waiting on #7":                       nil,
		"M2 waits on #99 and #100":            nil,
		"Waiting on #7: capitalised":          {7},
		"":                                    nil,
	} {
		if got := WaitingOn(blocked); !slices.Equal(got, want) {
			t.Errorf("WaitingOn(%q) = %v, want %v", blocked, got, want)
		}
	}
}
