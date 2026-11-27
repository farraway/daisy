package declare

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Each role's current default is in the table of earlier defaults, its
// last entry: a change to a default that does not append its hash fails
// here, so that `swarm prompt build` would not take the new default for
// an edited prompt (section 11, prompt build).
func TestDefaultHistoryHoldsEachCurrentDefault(t *testing.T) {
	for _, role := range AllRoles {
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
	if len(defaultHistory) != len(AllRoles) {
		t.Errorf("defaultHistory has %d roles, want %d", len(defaultHistory), len(AllRoles))
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

// absPath is an absolute path in a prompt: a slash after the start, a
// space or an opening mark, then the path's characters.
var absPath = regexp.MustCompile("(?:^|[\\s(`\"'])(/[A-Za-z0-9._/<>-]*[A-Za-z0-9_/>-])")

// No default names a resource's path: the owner, the architect and the QA read
// the resources listed below, system.md's # Resources, which Daisy writes
// from swarm.json (section 7.3). Every path a default names is under
// /job, the run's folder (sections 7.2 and 7.8), where a member's
// checkouts, /job/work/<name>, are (Step R1-6 of the plan implementing
// the 2026-11-19 swarm roadmap).
func TestDefaultPromptsNameNoResourcePath(t *testing.T) {
	seen := 0
	for _, role := range AllRoles {
		if strings.Contains(DefaultPrompts[role], "resources/") {
			t.Errorf("%s's default names a path under a resources mount", role)
		}
		for _, m := range absPath.FindAllStringSubmatch(DefaultPrompts[role], -1) {
			seen++
			if m[1] != "/job" && !strings.HasPrefix(m[1], "/job/") {
				t.Errorf("%s's default names %s, outside /job", role, m[1])
			}
		}
	}
	if seen == 0 {
		t.Error("no default names a path")
	}
	for _, role := range []string{"owner", "architect", "qa"} {
		if !strings.Contains(DefaultPrompts[role], "the resources listed below") {
			t.Errorf("%s's default does not read the resources listed below", role)
		}
	}
}

// No default reviewer persona tells a later round to read the whole diff
// regardless of the Task: since a delta review (decision 168), a later
// round's Task names the change since the reviewer's last head and says
// reviewing the whole change again is not the task, so each reviewer's
// reading instruction yields to it, the whole diff in round 1 (Step E2b
// of the plan implementing the 2026-11-19 swarm roadmap). A sentence
// that names the whole diff names round 1 too, and the instruction reads
// what the Task names in a later round.
func TestDefaultReviewersReadWhatTheTaskNames(t *testing.T) {
	for _, role := range []string{"adversary", "sympathetic"} {
		text := strings.Join(strings.Fields(DefaultPrompts[role]), " ")
		for _, s := range strings.SplitAfter(text, ". ") {
			if strings.Contains(strings.ToLower(s), "whole diff") && !strings.Contains(s, "in round 1") {
				t.Errorf("%s's default reads the whole diff in every round: %q", role, s)
			}
		}
		if !strings.Contains(text, "in a later round what the Task names") {
			t.Errorf("%s's default does not read what the Task names in a later round", role)
		}
	}
}

// The default owner proposes a job whose dependencies are still open,
// naming them on its "Depends:" line, which holds the job until they are
// closed (docs/swarm.md section 12.1, rule 5; the maintainer's ruling of
// 2026-11-26 on R2-A's first open question): no sentence of its default
// waits for them to close before proposing.
func TestDefaultOwnerProposesDependentWork(t *testing.T) {
	text := strings.Join(strings.Fields(DefaultPrompts["owner"]), " ")
	if !strings.Contains(text, "A job that depends on open jobs may be proposed all the same, naming them on its \"Depends:\" line") {
		t.Errorf("the owner's default does not propose dependent work:\n%s", DefaultPrompts["owner"])
	}
	if strings.Contains(text, "only when every job it depends on is closed") {
		t.Errorf("the owner's default still waits for its dependencies to close:\n%s", DefaultPrompts["owner"])
	}
}
