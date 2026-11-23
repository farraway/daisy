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
	approveNoting   = "VERDICT: approve\n\n[non-blocking] main.go:1: the version could be a constant\n"
	approveDocument = "VERDICT: approve\n\n[non-blocking] main.go:1: no doc comment\n"
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
// swarm:follow-up alone (section 7.11, Follow-ups): by the merge step,
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
		if fu.Title != "Follow-up: acme/app#1 (#1, slice 1)" || fu.Body != followUpBody("1", p[0].MergeSHA, 1, 1, items...) || fu.Author != author ||
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
		if len(fus) != 1 || fus[0].Body != followUpBody("1", p[0].MergeSHA, 1, 1, items...) || fus[0].Author != "jane" ||
			!strings.Contains(m.stdout.String(), fmt.Sprintf("#1 follow-up of acme/app#1 filed as #%d\n", fus[0].Number)) {
			t.Errorf("the verb's follow-up: %+v\n%s", fus, m.stdout.String())
		}
	})
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
// it to triage them; and a follow-up holds no proposal's place, the batch
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
		{"Follow-up: acme/app#1 (#9, slice 1)", followUpBody("1", sha, 1, 9, "adversary run 1, round 1: [non-blocking] a.go:1: triaged already"), true},
		{"Follow-up: acme/app#2 (#9, slice 2)", followUpBody("2", sha, 2, 9, first), false},
		{"Follow-up: acme/app#3 (#10, slice 1)", followUpBody("3", sha, 1, 10, first, second), false},
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
		"## #2: Follow-up: acme/app#2 (#9, slice 2)\n\n1. " + first + "\n\n" +
		"## #3: Follow-up: acme/app#3 (#10, slice 1)\n\n1. " + first + "\n2. " + second + "\n\n# Recently closed\n"
	if prompt := a.file("0-owner-1", "io/prompt.md"); !strings.Contains(prompt, want) || strings.Contains(prompt, "triaged already") {
		t.Errorf("prompt.md:\n%s\nwant within:\n%s", prompt, want)
	}
	task := `Each issue under "# Follow-ups" holds review findings that did not block a merge. Check each item against main under /job/work;`
	if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, task) || strings.Index(sys, task) > strings.Index(sys, `If there is nothing worth proposing, write "BLOCKED: <why>"`) {
		t.Errorf("system.md lacks the triage sentence before the BLOCKED: one:\n%s", sys)
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
			n, err := a.forge.CreateIssue(fmt.Sprintf("Follow-up: acme/app#%d (#9, slice 1)", i+1), followUpBody(fmt.Sprint(i+1), sha, 1, 9, item), []string{forge.FollowUp})
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
