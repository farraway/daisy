package reconcile

import (
	"strings"
	"testing"
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
