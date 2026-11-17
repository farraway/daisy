package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/swarm/runner"
)

// bigPlan is an accepted plan longer than a forge comment Daisy posts
// (60 000 bytes, section 7.5): its builder's section and its last line
// fall past the cut.
var bigPlan = oneSlice + "\n## For the builder\n\n" + strings.Repeat("Write main.go so that it says v1, then run the check.\n", 1300) + "Last: the builder's final step.\n"

// longReview is a reviewer's request-changes review longer than a forge
// comment: its last blocking finding falls past the cut.
var longReview = "VERDICT: request-changes\n\n[blocking] main.go:1 says v1; the plan wants v2.\n\n" + strings.Repeat("The reviewer explains the finding at length, line after line.\n", 1100) + "[blocking] main.go:2 the finding past the cut.\n"

// longReport is a worker run that commits main.go and writes a report
// longer than a forge comment: the pull request's description is cut.
var longReport = "v1\n\n" + strings.Repeat("The worker reports what it changed, line after line.\n", 1300) + "Not established: none\n"

// cutTask is the Task text's sentence naming the files that hold whole
// the texts its prompt shows cut (section 7.4).
func cutTask(texts string) string {
	t := `Daisy cuts a text longer than 60,000 bytes when it posts it on the forge, and these texts in your prompt were cut: ` + texts + `. In each, a line "[truncated by Daisy; the whole output is in <file>]" marks the cut and names the file that holds the member's whole output; read the file before you act on the text, since what was cut off may be what you must act on.`
	if strings.Contains(texts, "worker run") {
		t += ` A worker's text keeps Daisy's "**Check:**" result after that line, which the file does not hold.`
	}
	return t
}

func cutNote(where string) string {
	return "[truncated by Daisy; the whole output is in " + where + "]"
}

// The claim of Step 38b (request F40): an accepted plan longer than the
// forge's comment is posted cut, as before, and each member that acts on
// it, the worker and each reviewer of its slice, finds it whole as a
// file in its workspace, /work/.swarm/outputs/<run>.md, its Task text
// naming the file and the cut's note in its prompt pointing at it; the
// principle holds for any cut text a later member acts on, a reviewer's
// review that the worker answers among them (section 7.3).
func TestSwarmCutTextReachesTheMemberWhole(t *testing.T) {
	a := newApplyWS(t)
	a.planned(bigPlan)
	posted := ""
	for _, c := range a.issue(1).Comments {
		if strings.HasPrefix(c.Body, mark("architect", 1)) {
			posted = c.Body
		}
	}
	if !strings.HasSuffix(posted, "\n"+cutNote("runs/1-architect-1")) || strings.Contains(posted, "Last: the builder's final step.") {
		t.Fatalf("the plan as posted ends:\n%s", posted[max(0, len(posted)-200):])
	}
	planFile := "/work/.swarm/outputs/1-architect-1.md"
	whole := func(run, file, text, what string) {
		t.Helper()
		if got := a.file(run, "io/outputs/"+filepath.Base(file)); got != text {
			t.Errorf("%s's copy of %s: %d bytes, want %d", run, file, len(got), len(text))
		}
		prompt := a.file(run, "io/prompt.md")
		if !strings.Contains(prompt, "\n"+cutNote(file)+"\n") || strings.Contains(prompt, cutNote("runs/"+strings.TrimSuffix(filepath.Base(file), ".md"))) {
			t.Errorf("%s's prompt does not point the cut at %s:\n%s", run, file, prompt[max(0, strings.Index(prompt, "[truncated")-200):])
		}
		if sys := a.file(run, "io/system.md"); !strings.Contains(sys, cutTask(what)) {
			t.Errorf("%s's Task text does not name %s:\n%s", run, file, sys)
		}
	}

	reportFile, reviewFile := "/work/.swarm/outputs/1-worker-1.md", "/work/.swarm/outputs/1-adversary-1.md"
	plan := "the plan, whole in " + planFile
	report := "; worker run 1's text, whole in " + reportFile
	review := "; adversary run 1's text, whole in " + reviewFile

	a.runner.Script(1, "worker", runner.Canned{Out: longReport, Work: commit(map[string]string{"main.go": "v1\n"}, "v1")})
	a.pass()
	whole("1-worker-1", planFile, bigPlan, plan)

	// The reviewers act on the plan and on the worker's report, the pull
	// request's description, both cut.
	a.runner.Script(1, "adversary", runner.Canned{Out: longReview})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	for _, run := range []string{"1-adversary-1", "1-sympathetic-1"} {
		whole(run, planFile, bigPlan, plan+report)
		whole(run, reportFile, longReport, plan+report)
	}

	// The worker answers the review cut on the forge.
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	a.pass()
	whole("1-worker-2", planFile, bigPlan, plan+report+review)
	whole("1-worker-2", reviewFile, longReview, plan+report+review)

	// Round 2: each reviewer sees the previous round's blocking lines
	// alone, the cut review's with its note, so the findings past the cut
	// are found in its file.
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	for _, run := range []string{"1-adversary-2", "1-sympathetic-2"} {
		whole(run, reviewFile, longReview, plan+report+review)
		if p := a.file(run, "io/prompt.md"); !strings.Contains(p, "[blocking] main.go:1 says v1; the plan wants v2.\n"+cutNote(reviewFile)+"\n") {
			t.Errorf("%s's prompt does not end the cut review's findings with its note:\n%s", run, p)
		}
	}
}

// With a plan within the forge's limit nothing changes: no copy in the
// member's io, no Task sentence, and the prompt names no file of
// /work/.swarm/outputs/.
func TestSwarmUncutTextIsNotCopied(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	for _, run := range []string{"1-worker-1", "1-adversary-1", "1-sympathetic-1"} {
		if _, err := os.Lstat(a.state(filepath.Join("runs", run, "io", "outputs"))); !os.IsNotExist(err) {
			t.Errorf("%s's io holds outputs: %v", run, err)
		}
		for _, f := range []string{"io/prompt.md", "io/system.md"} {
			if text := a.file(run, f); text == "" || strings.Contains(text, "/work/.swarm/outputs") || strings.Contains(text, "truncated") {
				t.Errorf("%s's %s:\n%s", run, f, text)
			}
		}
	}
}
