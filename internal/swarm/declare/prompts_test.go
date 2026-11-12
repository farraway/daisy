package declare

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"testing"
)

// Each role's current default is in the table of earlier defaults, its
// last entry: a change to a default that does not append its hash fails
// here, so that `swarm prompt build` would not take the new default for
// an edited prompt (section 11, prompt build).
func TestDefaultHistoryHoldsEachCurrentDefault(t *testing.T) {
	for _, role := range Roles {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(DefaultPrompts[role])))
		h := defaultHistory[role]
		if len(h) == 0 || h[len(h)-1] != sum {
			t.Errorf("defaultHistory[%q] does not end with the current default's sha256 %s: append it (prompts.go says how)", role, sum)
		}
		for i, s := range h {
			if slices.Index(h, s) != i {
				t.Errorf("defaultHistory[%q] holds %s twice", role, s)
			}
		}
	}
	if len(defaultHistory) != len(Roles) {
		t.Errorf("defaultHistory has %d roles, want %d", len(defaultHistory), len(Roles))
	}
}

// workerAtB1021ce is the worker's default from b3ec5eb to b1021ce, as
// `git show b1021ce:internal/swarm/declare/prompts.go` gives it: the
// oracle that the table holds a default before the current one.
const workerAtB1021ce = `# Worker

You are the worker. You implement one slice of the accepted plan, on the
branch checked out in /work, and nothing else.

- Write the tests the plan names first, see them fail, then make them
  pass. Run the repository's own checks before you finish.
- Make small commits with messages that say why. Do not push; Daisy pushes.
- Change only what the slice needs. If the plan is wrong or cannot be
  followed, stop and say why instead of improvising.
- When reviews request changes, address each blocking point or say why
  you did not; do not rewrite what the reviewers did not question.
- The description you write is read by the reviewers and the maintainer:
  what changed, how it was tested, each acceptance claim's status, and
  anything left undone.
`

func TestComparePrompt(t *testing.T) {
	for _, c := range []struct {
		role, text string
		want       PromptKind
	}{
		{"worker", DefaultPrompts["worker"], PromptCurrent},
		{"worker", workerAtB1021ce, PromptEarlier},
		{"worker", workerAtB1021ce + "- Be brief.\n", PromptEdited},
		{"worker", workerAtB1021ce[:len(workerAtB1021ce)-1], PromptEdited}, // its trailing newline dropped
		{"owner", workerAtB1021ce, PromptEdited},                           // another role's default
		{"worker", "", PromptEdited},
	} {
		if got := ComparePrompt(c.role, []byte(c.text)); got != c.want {
			t.Errorf("ComparePrompt(%q, %.40q…) = %v, want %v", c.role, c.text, got, c.want)
		}
	}
}
