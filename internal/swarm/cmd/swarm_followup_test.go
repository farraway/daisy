package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// The reviews of the follow-up tests: each approves with a non-blocking
// finding of its own (section 7.11, Follow-ups).
const (
	approveNoting   = "VERDICT: approve\n[checked] claim 1: main.go says v1 on this head.\n\n[non-blocking] main.go:1: the version could be a constant\n"
	approveDocument = "VERDICT: approve\n[checked] claim 1: main.go says v1 on this head.\n\n[non-blocking] main.go:1: no doc comment\n"
)

// followUpBody is the follow-up of app#m, merged at sha, slice s of #n,
// holding items.
func followUpBody(m, sha string, s, n int, items ...string) string {
	body := fmt.Sprintf("<!-- daisy-swarm role=reconciler kind=follow-up -->\n**reconciler:** follow-up of %s, merged at %s, slice %d of #%d: the reviews' findings that did not block it.\n", appPull(m), sha[:7], s, n)
	for i, it := range items {
		body += fmt.Sprintf("\n%d. %s", i+1, it)
	}
	return body
}

// followUps is every issue carrying swarm:follow-up, open and closed.
func followUps(t *testing.T, f *forge.Fake) []forge.Issue {
	t.Helper()
	open, err := f.Issues(forge.Open, forge.FollowUp)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := f.Issues(forge.Closed, forge.FollowUp)
	if err != nil {
		t.Fatal(err)
	}
	return append(open, closed...)
}

// approvedNoting takes issue 1, of the plan given, through the pair's
// approval of slice 1, each review noting a non-blocking finding.
func (a *applyWS) approvedNoting(plan string) {
	a.t.Helper()
	a.inReview(plan)
	a.runner.Script(1, "adversary", runner.Canned{Out: approveNoting})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approveDocument})
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
}

// A merged pull request's non-blocking findings are filed as one
// follow-up issue, the reviews' findings numbered, carrying
// swarm:follow-up alone, titled "<k> findings left by #<n>: <the
// issue's title>" (section 7.11, Follow-ups): by the merge step,
// which closes the issue (T15); by step 8 of the pass after a merge on
// the forge, the issue left at review-2-approved; and by `swarm merge`,
// under the maintainer's token, so the issue is the maintainer's.
func TestSwarmFollowUpFiledAtTheMerge(t *testing.T) {
	t.Parallel()
	items := []string{
		"adversary run 1, round 1: [non-blocking] main.go:1: the version could be a constant",
		"sympathetic run 1, round 1: [non-blocking] main.go:1: no doc comment",
	}
	check := func(t *testing.T, f *forge.Fake, author, stdout string) {
		t.Helper()
		p, _ := f.Pulls(1, []string{"swarm-bot"})
		fus := followUps(t, f)
		if len(fus) != 1 {
			t.Fatalf("follow-ups: %+v\n%s", fus, stdout)
		}
		fu := fus[0]
		if fu.Title != "2 findings left by #1: Drift check" || fu.Body != followUpBody("1", p[0].MergeSHA, 1, 1, items...) || fu.Author != author ||
			!fu.Open || !reflect.DeepEqual(fu.Labels, []string{forge.FollowUp}) {
			t.Errorf("the follow-up: %q by %s, open %v, %v:\n%s", fu.Title, fu.Author, fu.Open, fu.Labels, fu.Body)
		}
		if !strings.Contains(stdout, fmt.Sprintf("#1 follow-up of acme/app#1 filed as #%d\n", fu.Number)) {
			t.Errorf("stdout:\n%s", stdout)
		}
	}

	t.Run("the merge step", func(t *testing.T) {
		t.Parallel()
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
		a.approvedNoting(oneSlice)
		if fus := followUps(t, a.forge); len(fus) != 0 {
			t.Fatalf("filed before the merge: %+v", fus)
		}
		a.pass() // T15: merged and closed, its pull requests listed
		if a.issue(1).Open || !strings.Contains(a.issue(1).Body, appPull("1")+" merged") {
			t.Fatalf("not merged and closed:\n%s", a.stdout.String())
		}
		check(t, a.forge, "swarm-bot", a.stdout.String())
	})

	t.Run("step 8, after a merge on the forge", func(t *testing.T) {
		t.Parallel()
		a := newApplyWS(t)
		a.approvedNoting(oneSlice)
		if err := a.forge.Merge(forge.Ref{Repo: "app", N: 1}, a.pulls(1)[0].HeadSHA); err != nil { // the maintainer merges
			t.Fatal(err)
		}
		a.pass()
		check(t, a.forge, "swarm-bot", a.stdout.String())
	})

	t.Run("swarm merge", func(t *testing.T) {
		t.Parallel()
		m := newMaintainerWS(t)
		m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"Brief: docs/briefs/m0.md — First\n", []string{forge.PlanAccepted})
		m.forge.Comment(m.ref(1), mark("architect", 1)+oneSlice)
		pr := m.approvedPull(1, "c1")
		m.forge.Push(pr, "c2") // round 2, its reviews noting a finding each
		m.forge.Review(pr, forge.Commented, mark("adversary", 2)+approveNoting)
		m.forge.Review(pr, forge.Commented, mark("sympathetic", 2)+approveDocument)
		if err := m.verb(swarmMerge, "1"); err != nil {
			t.Fatalf("%v\n%s", err, m.stderr.String())
		}
		items := []string{
			"adversary run 2, round 2: [non-blocking] main.go:1: the version could be a constant",
			"sympathetic run 2, round 2: [non-blocking] main.go:1: no doc comment",
		}
		fus := followUps(t, m.forge)
		p, _ := m.forge.Pulls(1, []string{"swarm-bot"})
		if len(fus) != 1 || fus[0].Title != "2 findings left by #1: Pin the corpus" || fus[0].Body != followUpBody("1", p[0].MergeSHA, 1, 1, items...) || fus[0].Author != "jane" ||
			!strings.Contains(m.stdout.String(), fmt.Sprintf("#1 follow-up of acme/app#1 filed as #%d\n", fus[0].Number)) {
			t.Errorf("the verb's follow-up: %+v\n%s", fus, m.stdout.String())
		}
	})
}

// A follow-up of one finding is titled "1 finding left by #<n>: <the
// issue's title>" (section 7.11, Follow-ups): the two reviews noting one
// finding, it is listed once.
func TestSwarmFollowUpTitleOfOneFinding(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approveNoting})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approveNoting})
	a.pass()
	a.pass()
	if err := a.forge.Merge(forge.Ref{Repo: "app", N: 1}, a.pulls(1)[0].HeadSHA); err != nil {
		t.Fatal(err)
	}
	a.pass()
	p, _ := a.forge.Pulls(1, []string{"swarm-bot"})
	fus := followUps(t, a.forge)
	if len(fus) != 1 || fus[0].Title != "1 finding left by #1: Drift check" ||
		fus[0].Body != followUpBody("1", p[0].MergeSHA, 1, 1, "adversary run 1, round 1: [non-blocking] main.go:1: the version could be a constant") {
		t.Errorf("the follow-up: %+v\n%s", fus, a.stdout.String())
	}
}

// refusingEdits is the forge refusing every issue body's edit, so a
// pass's listing files a follow-up and then fails to write the block.
type refusingEdits struct{ forge.Adapter }

func (refusingEdits) EditIssue(int, string) error {
	return errors.New("forge: PATCH /repos/acme/swarm-app/issues/1: 500 refused")
}

// A follow-up is filed once (section 7.11, Follow-ups): a pass whose
// block write fails after it filed one files none at the next pass,
// which finds it among the issues carrying swarm:follow-up, closed as
// well as open; the pass that writes the block files none either.
func TestSwarmFollowUpFiledOnce(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedNoting(oneSlice)
	if err := a.forge.Merge(forge.Ref{Repo: "app", N: 1}, a.pulls(1)[0].HeadSHA); err != nil {
		t.Fatal(err)
	}
	refused := func() {
		t.Helper()
		a.tick()
		if err := a.apply(refusingEdits{a.git()}, "--once"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(a.stderr.String(), "⚠ #1: pull requests: ") || strings.Contains(a.issue(1).Body, " merged") {
			t.Fatalf("the block written:\n%s\nstderr:\n%s", a.issue(1).Body, a.stderr.String())
		}
		if fus := followUps(t, a.forge); len(fus) != 1 {
			t.Fatalf("follow-ups after a refused write: %d\n%s", len(fus), a.stdout.String())
		}
	}
	refused()
	fus := followUps(t, a.forge)
	if len(fus) != 1 || !strings.Contains(a.stdout.String(), fmt.Sprintf("#1 follow-up of acme/app#1 filed as #%d\n", fus[0].Number)) {
		t.Fatalf("filed: %+v\n%s", fus, a.stdout.String())
	}
	refused() // the follow-up open
	a.forge.CloseIssue(fus[0].Number)
	refused() // the follow-up closed
	a.pass()  // the block written
	if got := followUps(t, a.forge); len(got) != 1 || !strings.Contains(a.issue(1).Body, appPull("1")+" merged") || strings.Contains(a.stdout.String(), "follow-up") {
		t.Errorf("follow-ups %+v, body:\n%s\nstdout:\n%s", got, a.issue(1).Body, a.stdout.String())
	}
}

// A pull request whose item the block already shows merged files no
// follow-up (section 7.11, Follow-ups): a merge listed before the build
// files nothing, though the pass edits the block for the issue's next
// pull request.
func TestSwarmFollowUpNotFiledForPastMerges(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedNoting(twoSlices)
	if err := a.forge.Merge(forge.Ref{Repo: "app", N: 1}, a.pulls(1)[0].HeadSHA); err != nil {
		t.Fatal(err)
	}
	body := pullsBlock("slice 1 "+appPull("1")+" merged") + "Check the drift."
	if err := a.forge.EditIssue(1, body); err != nil { // listed merged before the build
		t.Fatal(err)
	}
	a.forge.CreatePull("app", "swarm/1-2", "main", "acme/swarm-app#1 slice 2: Drift check", "Refs acme/swarm-app#1")
	a.pass()
	if got := a.issue(1).Body; got != pullsBlock("slice 1 "+appPull("1")+" merged · slice 2 "+appPull("2")+" open, review round 1")+"Check the drift." {
		t.Errorf("the block not listed:\n%s", got)
	}
	if fus := followUps(t, a.forge); len(fus) != 0 || strings.Contains(a.stdout.String(), "follow-up") {
		t.Errorf("filed for a past merge: %+v\n%s", fus, a.stdout.String())
	}
}

// The owner's batch reads the open follow-ups (sections 7.3 and 7.11,
// The owner's triage): its prompt holds, after "# Open swarm issues",
// "# Follow-ups" with each open follow-up, oldest first, as
// "## #<f>: <title>" and its numbered items, the marker lines and the
// first text line left out; a closed one is not there; its Task text asks
// it to triage them, and lets it retitle one it leaves open; and a follow-up holds no proposal's place, the batch
// proposing with owner.max_open_proposals 1.
func TestSwarmOwnerBatchReadsFollowUps(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Owner.MaxOpenProposals = 1 })
	sha := "0123456789abcdef0123456789abcdef01234567"
	first := "adversary run 2, round 1: [non-blocking] main.go:1: the version could be a constant"
	second := "sympathetic run 3, round 2: [non-blocking] main.go:1: no doc comment"
	for _, fu := range []struct {
		title, body string
		closed      bool
	}{
		{"1 finding left by #9: Name the version", followUpBody("1", sha, 1, 9, "adversary run 1, round 1: [non-blocking] a.go:1: triaged already"), true},
		{"1 finding left by #9: Name the version", followUpBody("2", sha, 2, 9, first), false},
		{"2 findings left by #10: Document the drift", followUpBody("3", sha, 1, 10, first, second), false},
	} {
		n, err := a.forge.CreateIssue(fu.title, fu.body, []string{forge.FollowUp})
		if err != nil {
			t.Fatal(err)
		}
		if fu.closed {
			a.forge.CloseIssue(n)
		}
	}
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Out: "## Drift check\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nCheck the drift.\n", Stdout: claudeResult})
	a.pass()
	want := "# Open swarm issues\n\nNone.\n\n# Follow-ups\n\n" +
		"## #2: 1 finding left by #9: Name the version\n\n1. " + first + "\n\n" +
		"## #3: 2 findings left by #10: Document the drift\n\n1. " + first + "\n2. " + second + "\n\n# Recently closed\n"
	if prompt := a.file("0-owner-1", "io/prompt.md"); !strings.Contains(prompt, want) || strings.Contains(prompt, "triaged already") {
		t.Errorf("prompt.md:\n%s\nwant within:\n%s", prompt, want)
	}
	task := `Each issue under "# Follow-ups" holds review findings that did not block a merge. Check each item against main under /job/work;`
	if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, task) || strings.Index(sys, task) > strings.Index(sys, `If there is nothing worth proposing, write "BLOCKED: <why>"`) {
		t.Errorf("system.md lacks the triage sentence before the BLOCKED: one:\n%s", sys)
	}
	retitle := `Daisy closes the issue with your reason. For a follow-up you leave open, you may write a line "Retitled: #<f>: <title>" there too, its title naming the findings' cause; Daisy sets the issue's title to it.`
	if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, retitle) {
		t.Errorf("system.md lacks the retitle sentence after the Triaged: one:\n%s", sys)
	}
	if !strings.Contains(a.stdout.String(), "owner run 1: proposed #4 Drift check\n") {
		t.Errorf("the batch proposed nothing beside the follow-ups:\n%s", a.stdout.String())
	}
}

// The owner's Triaged: line closes the follow-up it names (sections 7.5,
// 7.6 and 7.11, The owner's triage): the reconciler's note, the owner's
// reason and the batch's proposals that name it on a Follow-up: line, is
// posted on it, then it is closed, the flag kept; a follow-up the batch
// does not name stays open; a Triaged: line naming a closed follow-up,
// an issue that is no follow-up or a number the forge does not hold is
// ignored with a warning. Triaged: lines may come before the BLOCKED:
// line, which still holds the batch, and their follow-ups are closed.
func TestSwarmTriagedClosesTheFollowUp(t *testing.T) {
	t.Parallel()
	sha := "0123456789abcdef0123456789abcdef01234567"
	item := "adversary run 2, round 1: [non-blocking] main.go:1: the version could be a constant"
	// setup files follow-ups #1 and #2 open, #3 closed, and #4, an issue
	// with no label.
	setup := func(t *testing.T) *applyWS {
		a := newApplyWS(t)
		for i, closed := range []bool{false, false, true} {
			n, err := a.forge.CreateIssue("1 finding left by #9: Name the version", followUpBody(fmt.Sprint(i+1), sha, 1, 9, item), []string{forge.FollowUp})
			if err != nil {
				t.Fatal(err)
			}
			if closed {
				a.forge.CloseIssue(n)
			}
		}
		a.forge.CreateIssue("The maintainer's own", "Something.", nil)
		a.release()
		return a
	}
	triaged := func(t *testing.T, a *applyWS, n int, note string) {
		t.Helper()
		is := a.issue(n)
		if len(is.Comments) == 0 {
			t.Fatalf("#%d: no note; open %v\n%s\n%s", n, is.Open, a.stdout.String(), a.stderr.String())
		}
		last := is.Comments[len(is.Comments)-1]
		if is.Open || !reflect.DeepEqual(is.Labels, []string{forge.FollowUp}) || last.Body != note || last.Author != "swarm-bot" ||
			!strings.Contains(a.stdout.String(), fmt.Sprintf("owner run 1: follow-up #%d closed (triaged)\n", n)) {
			t.Errorf("#%d: open %v, %v, %d comments, the last %q\n%s", n, is.Open, is.Labels, len(is.Comments), last.Body, a.stdout.String())
		}
	}

	t.Run("with a proposal", func(t *testing.T) {
		t.Parallel()
		a := setup(t)
		out := "Triaged: #1: each item is proposed below\nTriaged: #3: closed already\nTriaged: #4: no follow-up\nTriaged: #99: no issue\n\n" +
			"## Name the version constant\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nFollow-up: #1 1\nName it.\n\n" +
			"## Another job\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nNot a follow-up.\n"
		a.runner.Script(0, "owner", runner.Canned{Out: out, Stdout: claudeResult})
		a.pass()
		if !strings.Contains(a.stdout.String(), "owner run 1: proposed #5 Name the version constant\n") {
			t.Fatalf("not proposed:\n%s\n%s", a.stdout.String(), a.stderr.String())
		}
		triaged(t, a, 1, mark("reconciler", 1)+`The owner triaged this follow-up in its batch run 1: each item is proposed below; proposed for it: "Name the version constant".`)
		if is := a.issue(2); !is.Open || len(is.Comments) != 0 {
			t.Errorf("#2, not triaged, changed: %+v", is)
		}
		for _, n := range []int{3, 4, 99} {
			if w := fmt.Sprintf("⚠ #%d: Triaged names no open follow-up issue; ignored\n", n); !strings.Contains(a.stderr.String(), w) {
				t.Errorf("stderr lacks %q:\n%s", w, a.stderr.String())
			}
		}
		if is := a.issue(4); !is.Open || len(is.Comments) != 0 {
			t.Errorf("#4, no follow-up, changed: %+v", is)
		}
	})

	t.Run("before BLOCKED:", func(t *testing.T) {
		t.Parallel()
		a := setup(t)
		a.runner.Script(0, "owner", runner.Canned{Out: "Triaged: #2: met only where the notes trust\nBLOCKED: nothing worth proposing\nCause: scope\n", Stdout: claudeResult})
		a.pass()
		if !strings.Contains(a.stdout.String(), "owner run 1: blocked: nothing worth proposing; batch held") {
			t.Fatalf("not held as blocked:\n%s", a.stdout.String())
		}
		triaged(t, a, 2, mark("reconciler", 1)+"The owner triaged this follow-up in its batch run 1: met only where the notes trust; no proposal of the batch names it.")
		if is := a.issue(1); !is.Open {
			t.Errorf("#1, not triaged, closed")
		}
	})
}

// The owner's Retitled: line sets the title of the follow-up it names and
// changes nothing else of it (sections 7.5, 7.6 and 7.11, The owner's
// triage): among the report lines, before the proposals or before the
// BLOCKED: line, beside the Triaged: lines. A Retitled: line naming an
// issue not under the run's "# Follow-ups", or with an empty title, fails
// the run: no title is changed, no proposal created and no follow-up
// closed.
func TestSwarmRetitledRetitlesTheFollowUp(t *testing.T) {
	t.Parallel()
	sha := "0123456789abcdef0123456789abcdef01234567"
	item := "adversary run 2, round 1: [non-blocking] main.go:1: the version could be a constant"
	const old = "1 finding left by #9: Name the version"
	// setup files follow-ups #1 and #2 open and #3 closed, and #4, an
	// issue with no label; the batch's "# Follow-ups" holds #1 and #2.
	setup := func(t *testing.T) *applyWS {
		a := newApplyWS(t)
		for i, closed := range []bool{false, false, true} {
			n, err := a.forge.CreateIssue(old, followUpBody(fmt.Sprint(i+1), sha, 1, 9, item), []string{forge.FollowUp})
			if err != nil {
				t.Fatal(err)
			}
			if closed {
				a.forge.CloseIssue(n)
			}
		}
		a.forge.CreateIssue("The maintainer's own", "Something.", nil)
		a.release()
		return a
	}
	issues := func(a *applyWS) []forge.Issue {
		var is []forge.Issue
		for n := 1; n <= 4; n++ {
			is = append(is, a.issue(n))
		}
		return is
	}
	proposal := "## Name the version constant\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nFollow-up: #1 1\nName it.\n"

	t.Run("before the proposals", func(t *testing.T) {
		t.Parallel()
		a := setup(t)
		before := issues(a)
		a.runner.Script(0, "owner", runner.Canned{Out: "Retitled: #1: The version is a literal in three places\n\n" + proposal, Stdout: claudeResult})
		a.pass()
		if !strings.Contains(a.stdout.String(), "owner run 1: proposed #5 Name the version constant\n") ||
			!strings.Contains(a.stdout.String(), "owner run 1: follow-up #1 retitled\n") {
			t.Fatalf("stdout:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
		}
		after := issues(a)
		want := before[0]
		want.Title = "The version is a literal in three places"
		if !reflect.DeepEqual(after[0], want) {
			t.Errorf("#1:\n got %+v\nwant %+v", after[0], want)
		}
		if !reflect.DeepEqual(after[1:], before[1:]) {
			t.Errorf("another issue changed:\n got %+v\nwant %+v", after[1:], before[1:])
		}
	})

	t.Run("before BLOCKED:", func(t *testing.T) {
		t.Parallel()
		a := setup(t)
		a.runner.Script(0, "owner", runner.Canned{Out: "Triaged: #1: met only where the notes trust\nRetitled: #2: A stale doc comment\nBLOCKED: nothing worth proposing\n", Stdout: claudeResult})
		a.pass()
		if !strings.Contains(a.stdout.String(), "owner run 1: blocked: nothing worth proposing; batch held") || a.issue(1).Open || a.issue(2).Title != "A stale doc comment" {
			t.Errorf("#1 open %v, #2 %q\n%s", a.issue(1).Open, a.issue(2).Title, a.stdout.String())
		}
	})

	for _, tc := range []struct{ name, line, reason string }{
		{"a closed follow-up", "Retitled: #3: Closed already", "bad Retitled line: #3 is not under # Follow-ups"},
		{"an issue that is no follow-up", "Retitled: #4: The maintainer's", "bad Retitled line: #4 is not under # Follow-ups"},
		{"a number the forge does not hold", "Retitled: #99: No issue", "bad Retitled line: #99 is not under # Follow-ups"},
		{"an empty title", "Retitled: #2:   ", "bad Retitled line: #2 has an empty title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := setup(t)
			before := issues(a)
			a.runner.Script(0, "owner", runner.Canned{Out: "Retitled: #1: The version is a literal\nTriaged: #2: fixed already\n" + tc.line + "\n\n" + proposal, Stdout: claudeResult})
			a.pass()
			if line := "owner run 1: " + tc.reason + "; batch held (" + a.state("owner-held") + ")\n"; !strings.Contains(a.stdout.String(), line) {
				t.Errorf("stdout lacks %q:\n%s", line, a.stdout.String())
			}
			if after := issues(a); !reflect.DeepEqual(after, before) {
				t.Errorf("an issue changed:\n got %+v\nwant %+v", after, before)
			}
			if _, err := a.forge.Issue(5); err == nil {
				t.Errorf("a proposal created")
			}
		})
	}
}

// The findings the architect overruled at the round limit are filed in
// the follow-up, marked "outstanding at the round limit" (Step R6-D of
// the plan implementing the 2026-11-19 swarm roadmap; section 7.11, rule
// 8 and Follow-ups): at the merge step's merge of the head the
// sympathetic reviewer approved after the overruling, the last round's
// blocking finding is an item of its round and review, before the
// review's non-blocking one; an earlier round's blocking finding, which
// the revision answered and no one overruled, is none.
func TestFollowUpMarksOutstanding(t *testing.T) {
	t.Parallel()
	const (
		round1 = "VERDICT: request-changes\n\n[blocking] claim main.go:1 says v1; the plan wants v2.\n"
		round2 = "VERDICT: request-changes\n\n[blocking] defect main.go:1 panics on an empty input; reproduced by go run . ''\n[non-blocking] main.go:1: the version could be a constant\n"
	)
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.MaxRounds = 2
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: round1}, runner.Canned{Out: round2})
	a.runner.Script(1, "worker", work("v2"))
	a.runner.Script(1, "architect", runner.Canned{Out: overruling})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approveDocument})
	a.pass() // round 1: T10
	a.pass() // the worker's revision: T12
	a.pass() // round 2 at the limit: T11
	a.pass() // the architect overrules: T33
	a.pass() // the sympathetic reviewer approves: T9
	a.pass() // the merge step: T15, the follow-up filed
	p := a.pulls(1)
	if a.issue(1).Open || !p[0].Merged {
		t.Fatalf("not merged and closed: %v\n%s", labels(a.issue(1)), a.stdout.String())
	}
	fus := followUps(t, a.forge)
	want := followUpBody("1", p[0].MergeSHA, 1, 1,
		"adversary run 2, round 2: outstanding at the round limit: [blocking] defect main.go:1 panics on an empty input; reproduced by go run . ''",
		"adversary run 2, round 2: [non-blocking] main.go:1: the version could be a constant",
		"sympathetic run 1, round 2: [non-blocking] main.go:1: no doc comment")
	if len(fus) != 1 || fus[0].Body != want || fus[0].Title != "3 findings left by #1: Drift check" {
		t.Fatalf("the follow-up: %+v\nwant:\n%s\n%s", fus, want, a.stdout.String())
	}
}
