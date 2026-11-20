package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
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

// forgeNote is the note that ends run's text on the forge, cut, its whole
// output attached to issue 1 as <run>.md (section 7.5).
func (a *applyWS) forgeNote(run string) string {
	a.t.Helper()
	for _, f := range a.forge.Attachments(issue1) {
		if f.Name == run+".md" {
			return "[truncated by Daisy; the whole output is in runs/" + run + " and attached as [" + run + ".md](" + f.URL + ")]"
		}
	}
	a.t.Fatalf("no %s.md attached to #1: %v", run, names(a.forge.Attachments(issue1)))
	return ""
}

// The claim of Step 38b (request F40): an accepted plan longer than the
// forge's comment is posted cut, as before, and each member that acts on
// it, the worker and each reviewer of its slice, finds it whole as a
// file in its workspace, /job/io/outputs/<run>.md, its Task text
// naming the file and the cut's note in its prompt pointing at it; the
// principle holds for any cut text a later member acts on, a reviewer's
// review that the worker answers among them (section 7.3). It holds as
// well where the forge refuses the files and the texts' rest follows in
// continuations (Step 38d), which no prompt shows: each text is shown
// once, cut, its note pointing at the member's copy (section 7.5).
func TestSwarmCutTextReachesTheMemberWhole(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "refused"}[refused], func(t *testing.T) { cutTextReachesTheMemberWhole(t, refused) })
	}
}

func cutTextReachesTheMemberWhole(t *testing.T, refused bool) {
	a := newApplyWS(t)
	if refused {
		a.forge.RefuseAttachments()
	}
	a.planned(bigPlan)
	posted := ""
	for _, c := range a.issue(1).Comments {
		if strings.HasPrefix(c.Body, mark("architect", 1)) {
			posted = c.Body
		}
	}
	note := "[truncated by Daisy; the whole output is in runs/1-architect-1; the rest follows in 1 comment]"
	if !refused {
		note = a.forgeNote("1-architect-1")
	}
	if !strings.HasSuffix(posted, "\n"+note) || strings.Contains(posted, "Last: the builder's final step.") {
		t.Fatalf("the plan as posted ends:\n%s", posted[max(0, len(posted)-200):])
	}
	planFile := "/job/io/outputs/1-architect-1.md"
	whole := func(run, file, text, what string) {
		t.Helper()
		if got := a.file(run, "io/outputs/"+filepath.Base(file)); got != text {
			t.Errorf("%s's copy of %s: %d bytes, want %d", run, file, len(got), len(text))
		}
		prompt := a.file(run, "io/prompt.md")
		if !strings.Contains(prompt, "\n"+cutNote(file)+"\n") || strings.Contains(prompt, "the whole output is in runs/") || strings.Contains(prompt, "Continued from") {
			t.Errorf("%s's prompt does not point the cut at %s:\n%s", run, file, prompt[max(0, strings.Index(prompt, "[truncated")-200):])
		}
		if sys := a.file(run, "io/system.md"); !strings.Contains(sys, cutTask(what)) {
			t.Errorf("%s's Task text does not name %s:\n%s", run, file, sys)
		}
	}

	reportFile, reviewFile := "/job/io/outputs/1-worker-1.md", "/job/io/outputs/1-adversary-1.md"
	plan := "the plan, whole in " + planFile
	report := "; worker run 1's text, whole in " + reportFile
	review := "; adversary run 1's text, whole in " + reviewFile

	a.runner.Script(1, "worker", runner.Canned{Out: longReport, Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
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
	// The review cut on the pull request, its rest in a comment there
	// when refused, its verdict the pair's one: T10.
	p := a.pulls(1)[0]
	pr := forge.Ref{Repo: p.Repo, N: p.Number}
	if conts := a.continuations(pr); refused != (len(conts) == 2) || refused && (!strings.HasPrefix(conts[0], continuedMark+"Continued from worker run 1's text (part 1 of 1):") || !strings.HasPrefix(conts[1], continuedMark+"Continued from adversary run 1's text (part 1 of 1):")) {
		t.Errorf("continuations on the pull request: %d", len(conts))
	}
	if got := labels(a.issue(1)); !slices.Equal(got, []string{"swarm:review-3-changes-requested"}) {
		t.Fatalf("after the pair: %v\n%s", got, a.stdout.String())
	}

	// The worker answers the review cut on the forge.
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	a.pass()
	whole("1-worker-2", planFile, bigPlan, plan+report+review)
	whole("1-worker-2", reviewFile, longReview, plan+report+review)

	// Round 2: each reviewer sees the previous round's blocking lines
	// alone, read by Daisy from the cut review whole, the finding past
	// the cut among them, so no copy of the review is needed (Step 38c).
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	for _, run := range []string{"1-adversary-2", "1-sympathetic-2"} {
		whole(run, planFile, bigPlan, plan+report)
		p := a.file(run, "io/prompt.md")
		if !strings.Contains(p, "[blocking] main.go:1 says v1; the plan wants v2.\n[blocking] main.go:2 the finding past the cut.\n") || strings.Contains(p, reviewFile) {
			t.Errorf("%s's prompt does not list the cut review's findings whole:\n%s", run, p)
		}
	}
}

// With a plan within the forge's limit nothing changes: no copy in the
// member's io, no Task sentence, and the prompt names no file of
// /job/io/outputs/.
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
			if text := a.file(run, f); text == "" || strings.Contains(text, "/job/io/outputs") || strings.Contains(text, "truncated") {
				t.Errorf("%s's %s:\n%s", run, f, text)
			}
		}
	}
}

// bigSlices is an accepted plan of two slices longer than a forge
// comment, its research so long that its `## Steps` fall past the cut
// (section 7.5).
var bigSlices = "## Research\n\n" + strings.Repeat("- /work/app/main.go: the entry point, read line after line.\n", 1200) +
	strings.TrimPrefix(twoSlices, "## Research\n\n- /work/app/main.go: the entry point.\n")

// The claim of Step 38c (request F40, Daisy's side): Daisy reads a plan
// it cut on the forge whole, from its run's out.md, wherever it acts on
// it. With `## Steps` past the cut, the plan gate's note counts two
// slices, the worker is given slice 1 of 2, the merge step merges it
// and hands the issue back for slice 2 instead of closing it, and the
// worker is given slice 2 of 2. With the run's out.md deleted after the
// gate's note, the same: Daisy's own reads of the plan, its slices
// counted for the worker's slice and Task text, the prompt's heading
// and the merge step, read the job's workspace, plan.md, which its
// collection wrote whole (section 7.8, Daisy's own reads; Step R1-9 of
// the plan implementing the 2026-11-19 swarm roadmap).
func TestSwarmCutPlanIsReadWhole(t *testing.T) {
	for _, gone := range []bool{false, true} {
		t.Run(map[bool]string{false: "whole", true: "out.md gone"}[gone], func(t *testing.T) {
			a := newApplyWS(t)
			a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: bigSlices})
			a.pass()
			plan := a.issue(1).Comments[0].Body
			if !strings.HasSuffix(plan, "\n"+a.forgeNote("1-architect-1")) || strings.Contains(plan, "## Steps") {
				t.Fatalf("the plan as posted ends:\n%s", plan[max(0, len(plan)-200):])
			}
			if notes := a.gateNotes(1); len(notes) == 0 || !strings.Contains(notes[len(notes)-1], "- Slices: 2.\n  1. app: change main.go to v1; the test proves it.\n  2. app: add notes.md.\n") {
				t.Errorf("the plan gate's note: %q", notes)
			}
			if gone {
				if err := os.Remove(a.state("runs/1-architect-1/io/out.md")); err != nil {
					t.Fatal(err)
				}
			}

			a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
			a.runner.Script(1, "worker", work("v1"))
			a.pass()
			if is := a.issue(1); slices.Contains(is.Labels, "swarm:blocked") {
				t.Fatalf("blocked: %v\n%s", labels(is), a.stdout.String())
			}
			if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, "Implement slice 1 of the accepted plan for issue #1 (it declares 2) in acme/app") {
				t.Errorf("worker run 1's Task text:\n%s\n%s", sys, a.stdout.String())
			}
			if p := a.file("1-worker-1", "io/prompt.md"); !strings.Contains(p, ": slice 1 of 2, in app\n") {
				t.Errorf("worker run 1's prompt:\n%s", p)
			}
			a.runner.Script(1, "adversary", runner.Canned{Out: approve})
			a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
			a.runner.Script(1, "worker", work("v2"))
			a.pass() // the pair: T9
			a.pass() // the merge step
			if is := a.issue(1); !is.Open || !strings.Contains(a.stdout.String(), "#1 merged pull app#1 (slice 1 of 2)\n") || strings.Contains(a.stdout.String(), "closed") {
				t.Fatalf("the merge step closed the job after slice 1: open %v, %v\n%s", is.Open, labels(is), a.stdout.String())
			}
			if a.file("1-worker-2", "io/system.md") == "" {
				a.pass()
			}
			if sys := a.file("1-worker-2", "io/system.md"); !strings.Contains(sys, "Implement slice 2 of the accepted plan for issue #1 (it declares 2) in acme/app") {
				t.Errorf("worker run 2's Task text:\n%s\n%s", sys, a.stdout.String())
			}
			if p := a.file("1-worker-2", "io/prompt.md"); !strings.Contains(p, ": slice 2 of 2, in app\n") {
				t.Errorf("worker run 2's prompt:\n%s", p)
			}
		})
	}
}

// With the merge gate on, what the maintainer is told of an approved
// slice reads the cut plan and the cut review whole: the gate note's
// slice count and the approving review's non-blocking finding past the
// cut, and, the pull request merged by hand, status's Waiting for you,
// which waits on the next slice, not the close (sections 7.5, 11 and 12).
// Each reads the job's workspace (section 7.8, Daisy's own reads), so
// with every run's out.md deleted before the gate note it holds.
func TestSwarmCutTextsTheMaintainerIsToldOf(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(bigSlices)
	long := "VERDICT: approve\n\n" + strings.Repeat("The reviewer explains its approval at length, line after line.\n", 1100) + "[non-blocking] main.go:1 a finding past the cut.\n"
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: long})
	a.pass() // the pair: T9, and the gate note
	// The outputs deleted and the note tidied away, so that the next pass
	// posts it again: the plan read from plan.md, the review from texts/
	// (section 7.8, Daisy's own reads).
	outs, _ := filepath.Glob(a.state("runs/1-*/io/out.md"))
	for _, o := range outs {
		if err := os.Remove(o); err != nil {
			t.Fatal(err)
		}
	}
	cs := a.issue(1).Comments
	a.forge.EditComment(issue1, cs[len(cs)-1].ID, "tidied")
	a.pass() // the gate note again
	if notes := a.gateNotes(1); len(notes) == 0 || !strings.Contains(notes[len(notes)-1], "- Slice 1 of 2.\n") ||
		!strings.Contains(notes[len(notes)-1], "  - sympathetic: [non-blocking] main.go:1 a finding past the cut.\n") {
		t.Errorf("the merge gate's note: %q", notes)
	}
	p := a.pulls(1)[0]
	if err := a.forge.Merge(forge.Ref{Repo: p.Repo, N: p.Number}, p.HeadSHA); err != nil {
		t.Fatal(err)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "\nWaiting for you: next slice #1 (architecture)\n") {
		t.Errorf("status:\n%s", out)
	}
}

// The merge verb counts the slices of a plan cut on the forge in its
// run's out.md, so slice 1 of 2 hands the issue back for slice 2; a note
// naming another run than the plan's own is left as posted, that run's
// output never read, and the plan as posted counts one slice, so its
// merge closes the issue; an out.md that cannot be read is the verb's
// error, nothing merged (section 7.5). A job whose workspace is built
// reads plan.md, its run's out.md gone, and a plan.md that cannot be
// read is the verb's error too (section 7.8, Daisy's own reads).
func TestSwarmMergeVerbReadsTheCutPlanWhole(t *testing.T) {
	const head = "Maintainer jane on acme/swarm-app\n"
	for _, c := range []struct {
		name, noted string
		built       bool // the job's workspace built, its plan.md holding the plan whole and no out.md
		mode        os.FileMode
		want        string
	}{
		{"its own run", "1-architect-1", false, 0o644, head + "#1 merged pull app#1 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n#1 pull requests listed\n"},
		{"another run", "1-architect-9", false, 0o644, head + "#1 merged pull app#1 (slice 1 of 1)\n#1 closed (last slice merged)\n#1 swarm:review-2-approved → swarm:done\n#1 pull requests listed\n"},
		{"unreadable", "1-architect-1", false, 0o000, ""},
		{"plan.md, out.md gone", "1-architect-1", true, 0o644, head + "#1 merged pull app#1 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n#1 pull requests listed\n"},
		{"plan.md unreadable", "1-architect-1", true, 0o000, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := newMaintainerWS(t)
			m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
			cut := bigSlices[:strings.LastIndex(bigSlices[:59000], "\n")] + "\n" + cutNote("runs/"+c.noted)
			m.forge.Comment(m.ref(1), mark("architect", 1)+cut)
			file, wantErr := filepath.Join(m.state("runs/"+c.noted+"/io"), "out.md"), "Error: swarm: #1: output 1-architect-1: "
			if c.built {
				file, wantErr = m.state("jobs/1/plan.md"), "Error: swarm: #1: "
				os.MkdirAll(m.state("jobs/1"), 0o755)
				os.WriteFile(m.state("jobs/1/built"), nil, 0o644)
			}
			os.MkdirAll(filepath.Dir(file), 0o755)
			os.WriteFile(file, []byte(bigSlices), c.mode)
			if c.noted == "1-architect-1" {
				// The plan's run, collected here (a run not collected is
				// left to its own collection, section 7.8, Rollout).
				os.WriteFile(m.state("runs/1-architect-1/result"), []byte("label swarm:proposal-accepted\noutcome posted\ncollected\n"), 0o644)
			}
			m.approvedPull(1, "c1")
			if c.want != "" {
				m.ok(swarmMerge, c.want, "1")
				return
			}
			err := m.verb(swarmMerge, "1")
			if err == nil || !strings.HasPrefix(err.Error(), wantErr) || !strings.Contains(err.Error(), "permission denied") || !strings.HasSuffix(err.Error(), "; nothing merged") {
				t.Errorf("an unreadable file: %v", err)
			}
			if p, _ := m.forge.Pulls(1, []string{"swarm-bot"}); len(p) != 1 || p[0].Merged {
				t.Errorf("merged with the plan unread: %+v", p)
			}
		})
	}
}

// A built job whose plan.md is not there has its plan read as the replay
// would write it, built in memory from its runs; with its texts/ file
// and its out.md gone too it is taken cut from its post, and the merge
// step blocks the issue as the replay would, nothing merged, rather than
// count its slices short and close it (section 7.8, Daisy's own reads
// and Rollout).
func TestSwarmMergeStepBlocksAPlanBuiltCut(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(bigSlices)
	for _, f := range []string{"jobs/1/plan.md", "jobs/1/texts/1-architect-1.md", "runs/1-architect-1/io/out.md"} {
		if err := os.Remove(a.state(f)); err != nil {
			t.Fatal(err)
		}
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	a.pass() // the merge step
	is := a.issue(1)
	if !slices.Contains(is.Labels, "swarm:blocked") || !slices.ContainsFunc(is.Comments, func(c forge.Comment) bool {
		return strings.Contains(c.Body, "The plan of 1-architect-1 is cut on the forge and runs/1-architect-1/io/out.md is gone, so its slices cannot be counted")
	}) {
		t.Errorf("not blocked: %v\n%s", labels(is), a.stdout.String())
	}
	if p := a.pulls(1); len(p) != 1 || p[0].Merged {
		t.Errorf("merged: %+v", p)
	}
}
