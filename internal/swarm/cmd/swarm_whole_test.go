package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
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
	t.Parallel()
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
	t.Parallel()
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
// note, its whole text attached as 1-gate-<time>.md or, refused, continued; the
// continuations are no gate note, so no second note is posted and the
// issue waits once (sections 5 and 7.5).
func TestSwarmLongGateNote(t *testing.T) {
	t.Parallel()
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
				i := slices.IndexFunc(a.forge.Attachments(issue1), func(f forge.Attachment) bool { return f.Name == "1-gate-20261106T150100Z.md" })
				if i < 0 {
					t.Fatalf("attached: %v", names(a.forge.Attachments(issue1)))
				}
				file := a.forge.Attachments(issue1)[i]
				if !strings.Contains(post, "[truncated by Daisy; the whole note is attached as [1-gate-20261106T150100Z.md]("+file.URL+")]"+decide) {
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

// The claim of Step R1-14 (an upload that fails): with every Attach
// failing as a forge's 5xx does, not refused, the architect's long plan
// and the plan gate's long note are each posted cut at the first pass,
// their rest in continuations, the failure warned of and, for the run,
// recorded `not attached`: the run is collected and the gate note posted
// on its first gating (section 7.5).
func TestGateNotePostedWhenAttachFails(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: openQuestions})
	f := &failing{Fake: a.forge, uploads: true}
	a.tick()
	if err := a.apply(f, "--once"); err != nil {
		t.Fatalf("pass 1: %v\n%s", err, a.stderr.String())
	}
	upload := "forge: POST /repos/acme/swarm-app/issues/1/assets: 502 Bad Gateway"
	if r := a.result("1-architect-1"); !strings.Contains(r, "\nnot attached 1-architect-1.md: "+upload+"\n") || !strings.HasSuffix(r, "\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
	for _, w := range []string{
		"⚠ #1 architect run 1: 1-architect-1.md not attached: " + upload + "; the rest follows in comments\n",
		"⚠ #1: 1-gate-20261106T150100Z.md not attached: " + upload + "; the rest follows in comments\n",
	} {
		if !strings.Contains(a.stderr.String(), w) {
			t.Errorf("stderr lacks %q:\n%s", w, a.stderr.String())
		}
	}
	notes := a.gateNotes(1)
	decide := "\n\nDecide: `daisy swarm accept 1`, `daisy swarm class 1 routine`, after which Daisy accepts it, or a comment here, from which the architect revises the plan."
	if len(notes) != 1 || !strings.Contains(notes[0], "[truncated by Daisy; the rest follows in 1 comment]"+decide) {
		t.Fatalf("gate notes: %d\n%s", len(notes), a.stdout.String())
	}
	var gate []string
	for _, c := range a.continuations(issue1) {
		if strings.HasPrefix(c, continuedMark+"Continued from the gate note") {
			gate = append(gate, c)
		}
	}
	if whole := rejoin(t, strings.TrimPrefix(notes[0], gateNote), "the gate note", gate); !strings.HasSuffix(whole, "  - The last question.\n- Risks: none in the plan.") {
		t.Errorf("the whole note (%d bytes) ends:\n%s", len(whole), whole[max(0, len(whole)-200):])
	}
}

// Each gating's note that does not fit attaches its own file (section
// 7.5): the plan gate's note on the plan, then on the architect's revised
// plan, attached as 1-gate-<time>.md at each pass's time, two files.
func TestSwarmGateNoteFilePerGating(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.forge.CreateIssue("Planned", "Plan it.", []string{"swarm:plan-0-posted"})
	a.forge.Comment(issue1, mark("architect", 1)+openQuestions)
	a.pass()
	revised := strings.Replace(openQuestions, "- The last question.", "- The last question, revised.", 1)
	a.tick()
	a.forge.Comment(issue1, mark("architect", 2)+revised)
	os.WriteFile(a.state("jobs/1/plan.md"), []byte(revised), 0o644)
	a.pass()
	files := a.forge.Attachments(issue1)
	if got := names(files); !slices.Equal(got, []string{"1-gate-20261106T150100Z.md", "1-gate-20261106T150300Z.md"}) {
		t.Fatalf("attached: %v\n%s", got, a.stderr.String())
	}
	notes := a.gateNotes(1)
	if len(notes) != 2 || !strings.Contains(notes[1], "[1-gate-20261106T150300Z.md]("+files[1].URL+")") || !strings.HasSuffix(string(files[1].Data), "  - The last question, revised.\n- Risks: none in the plan.") {
		t.Errorf("%d gate notes; the second file ends:\n%s", len(notes), files[len(files)-1].Data[max(0, len(files[len(files)-1].Data)-100):])
	}
}

// A text within the limit is posted as before: no file attached, no
// continuation, nothing recorded of either (section 7.5).
func TestSwarmShortTextIsPostedAsBefore(t *testing.T) {
	t.Parallel()
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

// The claim of Step R1-14b (the merge step's notes): T26's note naming
// more failed checks than one comment holds is posted cut on the pull
// request, its last sentence kept after the note, its whole text
// attached to the issue as 1-merge-<time>.md or, the file refused, its
// rest continued on the pull request; a merge step interrupted after the
// note (its relabel refused) posts neither again at the next pass
// (sections 7.5 and 12).
func TestLongMergeNoteAttached(t *testing.T) {
	t.Parallel()
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "refused"}[refused], func(t *testing.T) {
			a := newApplyWS(t)
			head := a.approvedGateOff()
			if refused {
				a.forge.RefuseAttachments()
			}
			var named []string
			for i := range 1500 {
				ctx := fmt.Sprintf("ci / test %04d of the long matrix", i)
				a.forge.SetStatus("app", head, ctx, "failure")
				named = append(named, "`"+ctx+"` (failure)")
			}
			tail := " Fix what they report on swarm/1-1 and commit; the reviewers then review the new head."
			text := strings.TrimSuffix(checksFailed(head, strings.Join(named, ", ")), tail)
			a.tick()
			if err := a.apply(&failing{Fake: a.forge, relabels: 1}, "--once"); err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
				t.Fatalf("pass 1: %v", err)
			}
			a.pass()
			pr := forge.Ref{Repo: "app", N: 1}
			comments, _ := a.forge.PullComments(pr)
			var notes []string
			for _, c := range comments {
				if after, ok := strings.CutPrefix(c.Body, mark("reconciler", 1)); ok && c.Author == "swarm-bot" {
					notes = append(notes, after)
				}
			}
			if len(notes) != 1 || !strings.HasSuffix(notes[0], "]"+tail) || len(notes[0]) > 60000 {
				t.Fatalf("%d notes; the first ends:\n%s", len(notes), notes[0][max(0, len(notes[0])-300):])
			}
			post := strings.TrimSuffix(notes[0], tail)
			cut := strings.LastIndex(post, "\n[truncated by Daisy; ") // within the one line: the note on its own
			files, conts := a.forge.Attachments(issue1), a.continuations(pr)
			if refused {
				got := post[:cut]
				for k, c := range conts {
					head := continuedMark + fmt.Sprintf("Continued from the merge step's note (part %d of %d):\n\n", k+1, len(conts))
					got += strings.TrimPrefix(c, head)
				}
				if len(files) != 0 || len(conts) != 1 || post[cut:] != "\n[truncated by Daisy; the rest follows in 1 comment]" || got != text {
					t.Errorf("%d files, %d continuations; the note's end %q; rejoined %d bytes of %d", len(files), len(conts), post[cut:], len(got), len(text))
				}
			} else if len(files) != 1 || !regexp.MustCompile(`^1-merge-20261106T\d{6}Z\.md$`).MatchString(files[0].Name) || string(files[0].Data) != text ||
				post[cut:] != "\n[truncated by Daisy; the whole note is attached as ["+files[0].Name+"]("+files[0].URL+")]" || len(conts) != 0 || !strings.HasPrefix(text, post[:cut]) {
				t.Errorf("attached %v, %d continuations; the note's end %q", names(files), len(conts), post[cut:])
			}
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested}) {
				t.Errorf("labels: %v\n%s", got, a.stderr.String())
			}
		})
	}
}

// created: issue n exists on the forge.
func (a *applyWS) created(n int) bool { _, err := a.forge.Issue(n); return err == nil }

// The claim of Step R1-14b (a proposal's body): the owner's proposal
// longer than one comment opens its issue cut, its note naming the batch
// run, its rest in a continuation on the new issue, posted once across a
// collection retried after it failed; the body and the continuation hold
// the proposal whole, and the architect's prompt shows it whole, with no
// note and no continuation (sections 6, 7.3 and 7.5).
func TestLongProposalBodyContinued(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Triage = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.release()
	body := "Brief: docs/briefs/a.md — M1 The drift\nClass: architecture\n" + strings.Repeat("Check the drift in each module, then report it.\n", 1500) + "The last line."
	a.runner.Script(0, "owner", runner.Canned{Out: "## Drift\n" + body + "\n"})
	a.tick()
	if err := a.apply(&failing{Fake: a.forge, comments: 1}, "--once"); err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("pass 1: %v\n%s", err, a.stderr.String())
	}
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	is := a.issue(1)
	post, ok := strings.CutPrefix(is.Body, mark("owner", 1))
	conts := a.continuations(issue1)
	if !ok || len(post) > 60000 || !strings.HasSuffix(post, "\n[truncated by Daisy; the whole output is in runs/0-owner-1; the rest follows in 1 comment]") || len(conts) != 1 {
		t.Fatalf("%d continuations; the body ends:\n%s", len(conts), post[max(0, len(post)-300):])
	}
	if got := rejoin(t, post, "proposal 1 of owner run 1", conts); got != body {
		t.Errorf("the issue holds %d bytes of the proposal's %d", len(got), len(body))
	}
	if r := a.result("0-owner-1"); strings.Count(r, "\ncreated 1 #1\n") != 1 || strings.Count(r, "\ncontinued 0-owner-1-1.md 1 of 1\n") != 1 || a.created(2) {
		t.Errorf("result:\n%s", r)
	}
	p := a.file("1-architect-1", "io/prompt.md")
	if !strings.Contains(p, "\nThe last line.\n") || strings.Contains(p, "truncated by Daisy") || strings.Contains(p, "Continued from") {
		t.Errorf("the architect's prompt:\n%s", p[:min(len(p), 2000)])
	}
}
