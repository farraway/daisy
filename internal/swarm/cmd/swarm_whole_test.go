package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// continuedMark is a continuation's first line (section 7.5).
const continuedMark = "<!-- daisy-swarm role=reconciler kind=continued -->\n"

// continuations is the continuation comments on the issue or pull
// request at, oldest first.
func (a *applyWS) continuations(at forge.Ref) []string {
	a.t.Helper()
	var out []string
	for _, c := range a.comments(at) {
		if strings.HasPrefix(c.Body, continuedMark) && c.Author == "swarm-bot" {
			out = append(out, c.Body)
		}
	}
	return out
}

// rejoin is the text a post and its continuations hold together (section
// 7.5): the post's text up to its note's line, then each continuation's
// text after its heading line and the blank line, what names the text in
// the headings and parts their count.
func rejoin(t *testing.T, post, what string, conts []string) string {
	t.Helper()
	i := strings.LastIndex(post, "[truncated by Daisy; ")
	if i < 0 {
		t.Fatalf("no note in the post:\n%s", post[max(0, len(post)-300):])
	}
	text := post[:i]
	for k, c := range conts {
		head := continuedMark + "Continued from " + what + " (part " + strconv.Itoa(k+1) + " of " + strconv.Itoa(len(conts)) + "):\n\n"
		if !strings.HasPrefix(c, head) {
			t.Fatalf("continuation %d begins:\n%s", k+1, c[:min(len(c), 200)])
		}
		if len(c)-len(head) > 60000 {
			t.Errorf("continuation %d holds %d bytes", k+1, len(c)-len(head))
		}
		text += strings.TrimPrefix(c, head)
	}
	return text
}

// hugePlan is an accepted plan longer than two forge comments, so that
// its rest takes more than one continuation.
var hugePlan = oneSlice + "\n## For the builder\n\n" + strings.Repeat("Write main.go so that it says v1, then run the check again.\n", 2300) + "Last: the builder's final step.\n"

// The claim of Step 38d (request F40, the forge's side): a 70 KB plan is
// posted cut, and the comment ends in a link to the whole plan, attached
// to the issue as 1-architect-1.md; a collection interrupted after the
// file was attached attaches it once (its `attached` line in the result
// read at the retry), and the comment is posted once (section 7.5).
func TestSwarmLongTextIsAttachedWhole(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: bigPlan})
	f := &failing{Fake: a.forge, comments: 1}
	a.tick()
	if err := a.apply(f, "--once"); err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("pass 1: %v", err)
	}
	a.tick()
	if err := a.apply(f, "--once"); err != nil {
		t.Fatalf("pass 2: %v\n%s", err, a.stderr.String())
	}
	files := a.forge.Attachments(issue1)
	if len(files) != 1 || files[0].Name != "1-architect-1.md" || string(files[0].Data) != bigPlan {
		t.Fatalf("attached to #1: %d files %v", len(files), names(files))
	}
	var posted []string
	for _, b := range a.bodies(1) {
		if strings.HasPrefix(b, mark("architect", 1)) {
			posted = append(posted, strings.TrimPrefix(b, mark("architect", 1)))
		}
	}
	note := "\n[truncated by Daisy; the whole output is in runs/1-architect-1 and attached as [1-architect-1.md](" + files[0].URL + ")]"
	if len(posted) != 1 || !strings.HasSuffix(posted[0], note) || !strings.HasPrefix(bigPlan, strings.TrimSuffix(posted[0], note)+"\n") || len(posted[0]) > 60000 {
		t.Fatalf("the plan as posted (%d comments) ends:\n%s", len(posted), posted[0][max(0, len(posted[0])-300):])
	}
	if c := a.continuations(issue1); len(c) != 0 {
		t.Errorf("continuations with the file attached: %d", len(c))
	}
	if r := a.result("1-architect-1"); strings.Count(r, "\nattached 1-architect-1.md "+files[0].URL+"\n") != 1 || strings.Count(r, "posted comment") != 1 {
		t.Errorf("result:\n%s", r)
	}
}

func names(files []forge.Attachment) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

// failNth is the Fake whose nth Comment from now fails once, as a network
// error interrupting a collection between two of its comments does.
type failNth struct {
	*forge.Fake
	nth int
}

func (f *failNth) Comment(at forge.Ref, body string) error {
	if f.nth--; f.nth == 0 {
		return (&failing{comments: 1}).Comment(at, body)
	}
	return f.Fake.Comment(at, body)
}

// Where the forge refuses the file, the plan's rest follows the cut
// comment in numbered continuations, each within the limit, and the
// comments together hold the whole plan; a collection interrupted
// between two continuations posts each once, the refusal recorded and
// the file not tried again. Daisy's reads skip the continuations: no
// second plan is counted, the architect is not run again, and the
// worker's prompt shows the plan once, its note pointing at its copy
// (sections 7.3 and 7.5).
func TestSwarmLongTextContinuedWhenRefused(t *testing.T) {
	a := newApplyWS(t)
	a.forge.RefuseAttachments()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: hugePlan})
	f := &failNth{Fake: a.forge, nth: 3} // the main comment, continuation 1, then continuation 2 fails
	a.tick()
	if err := a.apply(f, "--once"); err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("pass 1: %v", err)
	}
	if !strings.Contains(a.stderr.String(), "⚠ #1 architect run 1: 1-architect-1.md not attached: forge: not attached: ") {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
	a.tick()
	if err := a.apply(f, "--once"); err != nil {
		t.Fatalf("pass 2: %v\n%s", err, a.stderr.String())
	}
	if files := a.forge.Attachments(issue1); len(files) != 0 {
		t.Errorf("attached: %v", names(files))
	}
	var posted []string
	for _, b := range a.bodies(1) {
		if strings.HasPrefix(b, mark("architect", 1)) {
			posted = append(posted, strings.TrimPrefix(b, mark("architect", 1)))
		}
	}
	conts := a.continuations(issue1)
	if len(posted) != 1 || len(conts) != 2 || !strings.HasSuffix(posted[0], "\n[truncated by Daisy; the whole output is in runs/1-architect-1; the rest follows in 2 comments]") || len(posted[0]) > 60000 {
		t.Fatalf("%d posts, %d continuations; the post ends:\n%s", len(posted), len(conts), posted[0][max(0, len(posted[0])-300):])
	}
	if got := rejoin(t, posted[0], "architect run 1's text", conts); got != hugePlan {
		t.Errorf("the comments hold %d bytes of the plan's %d", len(got), len(hugePlan))
	}
	r := a.result("1-architect-1")
	if strings.Count(r, "not attached 1-architect-1.md: ") != 1 || strings.Count(r, "\ncontinued 1-architect-1.md 1 of 2\n") != 1 || strings.Count(r, "\ncontinued 1-architect-1.md 2 of 2\n") != 1 {
		t.Errorf("result:\n%s", r)
	}

	// The thread read: one plan, the architect not woken by the
	// continuations, and the worker pointed at the plan whole in the
	// job's workspace (section 7.8, The prompt).
	launched := len(a.runner.Launched)
	a.pass()
	if len(a.runner.Launched) != launched {
		t.Errorf("a pass at plan-0-posted launched %d more", len(a.runner.Launched)-launched)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "\n  #1           plan revisions 0, blocks 0\n") {
		t.Errorf("status counts a revision:\n%s", out)
	}
	a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	p := a.file("1-worker-1", "io/prompt.md")
	if strings.Contains(p, "## For the builder") || strings.Contains(p, "Continued from") || strings.Contains(p, "truncated") ||
		!strings.Contains(p, fmt.Sprintf("\n- Plan: /job/memory/plan.md, architect run 1, %d bytes, 1 slice\n", len(hugePlan))) {
		t.Errorf("worker run 1's prompt:\n%s", p)
	}
	if got := readFile(t, a.state("jobs/1/plan.md")); got != hugePlan {
		t.Errorf("the job's plan.md: %d bytes, want %d", len(got), len(hugePlan))
	}
}

// openQuestions is a plan whose Open Questions alone make its plan gate's
// note longer than the limit.
var openQuestions = oneSlice + "\n## Open Questions\n\n" + strings.Repeat("- Should main.go say v1 or v2, and who decides it?\n", 1300) + "- The last question.\n"

// Daisy's own note is posted by the same rule: the plan gate's note
// longer than the limit is cut, its decision and mention kept after the
// note, its whole text attached as 1-gate.md or, refused, continued; the
// continuations are no gate note, so no second note is posted and the
// issue waits once (sections 5 and 7.5).
func TestSwarmLongGateNote(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "refused"}[refused], func(t *testing.T) {
			a := newApplyWS(t)
			if refused {
				a.forge.RefuseAttachments()
			}
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: openQuestions})
			a.pass()
			notes := a.gateNotes(1)
			if len(notes) != 1 {
				t.Fatalf("gate notes: %d\n%s", len(notes), a.stderr.String())
			}
			post := strings.TrimPrefix(notes[0], gateNote)
			decide := "\n\nDecide: `daisy swarm accept 1`, `daisy swarm class 1 routine`, after which Daisy accepts it, or a comment here, from which the architect revises the plan."
			if i := strings.Index(post, "]"+decide); i < 0 || len(post) > 60000 || !strings.HasPrefix(post, "**reconciler:** gated at plan-0-posted: ") || strings.Contains(post, "The last question.") {
				t.Fatalf("the gate note (%d bytes) ends:\n%s", len(post), post[max(0, len(post)-400):])
			}
			var whole string
			if refused {
				conts := a.continuations(issue1)
				var gate []string
				for _, c := range conts {
					if strings.HasPrefix(c, continuedMark+"Continued from the gate note") {
						gate = append(gate, c)
					}
				}
				if !strings.Contains(post, "[truncated by Daisy; the rest follows in 1 comment]"+decide) {
					t.Errorf("the note's cut:\n%s", post[max(0, len(post)-400):])
				}
				whole = rejoin(t, post, "the gate note", gate)
			} else {
				i := slices.IndexFunc(a.forge.Attachments(issue1), func(f forge.Attachment) bool { return f.Name == "1-gate.md" })
				if i < 0 {
					t.Fatalf("attached: %v", names(a.forge.Attachments(issue1)))
				}
				file := a.forge.Attachments(issue1)[i]
				if !strings.Contains(post, "[truncated by Daisy; the whole note is attached as [1-gate.md]("+file.URL+")]"+decide) {
					t.Errorf("the note's cut:\n%s", post[max(0, len(post)-400):])
				}
				whole = string(file.Data)
				if !strings.HasPrefix(whole, post[:strings.LastIndex(post, "[truncated by Daisy; ")]) {
					t.Errorf("the file does not begin with the note's text")
				}
			}
			if !strings.HasPrefix(whole, "**reconciler:** gated at plan-0-posted: ") || !strings.HasSuffix(whole, "  - The last question.\n- Risks: none in the plan.") {
				t.Errorf("the whole note (%d bytes) ends:\n%s", len(whole), whole[max(0, len(whole)-200):])
			}
			a.pass()
			if n := len(a.gateNotes(1)); n != 1 {
				t.Errorf("gate notes after another pass: %d", n)
			}
			if err := a.status(a.forge); err != nil {
				t.Fatal(err)
			}
			if out := a.stdout.String(); !strings.Contains(out, "\nWaiting for you: plan #1\n") {
				t.Errorf("status:\n%s", out)
			}
		})
	}
}

// A text within the limit is posted as before: no file attached, no
// continuation, nothing recorded of either (section 7.5).
func TestSwarmShortTextIsPostedAsBefore(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if files := a.forge.Attachments(issue1); len(files) != 0 {
		t.Errorf("attached: %v", names(files))
	}
	p := a.pulls(1)[0]
	if c := append(a.continuations(issue1), a.continuations(forge.Ref{Repo: p.Repo, N: p.Number})...); len(c) != 0 {
		t.Errorf("continuations: %q", c)
	}
	for _, run := range []string{"1-architect-1", "1-worker-1"} {
		if r := a.result(run); strings.Contains(r, "attached") || strings.Contains(r, "continued") {
			t.Errorf("%s's result:\n%s", run, r)
		}
	}
	if !strings.HasPrefix(p.Body, mark("worker", 1)+"v1\n\nNot established: none\n\n**Check:** ") {
		t.Errorf("the description: %q", p.Body)
	}
	if _, err := os.Stat(a.state(filepath.Join("runs", "1-worker-1", "io", "outputs"))); !os.IsNotExist(err) {
		t.Errorf("outputs: %v", err)
	}
}
