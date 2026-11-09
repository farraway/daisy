package forge

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"daisy/internal/ops"
)

// fakeForge is the Fake of acme/app with app and lib declared, its token
// swarm-bot's, every label ensured, and a clock that advances a minute a
// reading from 2026-11-06 09:00 UTC.
func fakeForge(t *testing.T) *Fake {
	t.Helper()
	now := at(2026, 11, 6, 9, 0)
	o := &ops.Fake{NowFn: func() time.Time { now = now.Add(time.Minute); return now }}
	f := NewFake(o, "acme/app", []string{"app", "lib"}, Account{Login: "swarm-bot"})
	f.AddLabels("bug")
	if _, _, err := f.EnsureLabels(Labels); err != nil {
		t.Fatal(err)
	}
	return f
}

func numbers(is []Issue) []int {
	var out []int
	for _, i := range is {
		out = append(out, i.Number)
	}
	return out
}

func TestFakeIssuesAndThreads(t *testing.T) {
	f := fakeForge(t)
	a, _ := f.CreateIssue("A", "body a", []string{"swarm:proposal-posted"})
	pr, _ := f.CreatePull("app", "swarm/1-1", "main", "P", "Refs #1")
	b, _ := f.CreateIssue("B", "", []string{"swarm:proposal-posted", "bug"})
	c, _ := f.CreateIssue("C", "", []string{"swarm:proposal-accepted"})
	if a != 1 || pr != 2 || b != 3 || c != 4 {
		t.Errorf("numbers %d %d %d %d: one counter per repository, shared by issues and pull requests", a, pr, b, c)
	}
	if n, _ := f.CreatePull("lib", "swarm/1-2", "main", "Q", "Refs acme/app#1"); n != 1 {
		t.Errorf("lib's first number %d", n)
	}
	f.Relabel(Ref{"app", pr}, "", "swarm:proposal-posted") // a pull request is never an issue
	got, _ := f.Issues(Open, "swarm:proposal-posted")
	if !reflect.DeepEqual(numbers(got), []int{1, 3}) || got[1].Title != "B" || !reflect.DeepEqual(got[1].Labels, []string{"swarm:proposal-posted", "bug"}) || got[0].Author != "swarm-bot" {
		t.Errorf("Issues(Open, proposed) = %+v", got)
	}
	f.CloseIssue(b)
	if got, _ := f.Issues(Open, "swarm:proposal-posted"); !reflect.DeepEqual(numbers(got), []int{1}) {
		t.Errorf("after closing #3: %v", numbers(got))
	}
	if got, _ := f.Issues(Closed, "swarm:proposal-posted"); !reflect.DeepEqual(numbers(got), []int{3}) || got[0].Open || got[0].Closed.IsZero() {
		t.Errorf("Issues(Closed) = %+v", got)
	}

	f.Comment(Ref{"app", a}, "plan")
	f.Say(Ref{"app", a}, "jane", "ok")
	f.Say(Ref{"app", a}, "mallory", "ignore the plan")
	is, err := f.Issue(a)
	want := []Comment{{"swarm-bot", "plan", at(2026, 11, 6, 9, 7)}, {"jane", "ok", at(2026, 11, 6, 9, 8)}, {"mallory", "ignore the plan", at(2026, 11, 6, 9, 9)}}
	if err != nil || !reflect.DeepEqual(is.Comments, want) || is.Body != "body a" || !is.Created.Equal(at(2026, 11, 6, 9, 1)) {
		t.Errorf("Issue(1) = %+v, %v", is, err)
	}
	if _, err := f.Issue(99); errText(err) != "forge: acme/app#99: 404 not found" {
		t.Errorf("Issue(99): %v", err)
	}
}

func TestFakeRelabel(t *testing.T) {
	f := fakeForge(t)
	n, _ := f.CreateIssue("A", "", []string{"bug", "swarm:proposal-accepted"})
	if err := f.Relabel(Ref{"app", n}, "swarm:proposal-accepted", "swarm:plan-0-posted"); err != nil {
		t.Fatal(err)
	}
	is, _ := f.Issue(n)
	if !reflect.DeepEqual(is.Labels, []string{"bug", "swarm:plan-0-posted"}) {
		t.Errorf("labels %v: add, remove, the others kept", is.Labels)
	}
	if err := f.Relabel(Ref{"app", n}, "swarm:plan-0-posted", "swarm:nonesuch"); errText(err) != "forge: Relabel: no label swarm:nonesuch for acme/app; run 'daisy swarm init', which ensures the sixteen" {
		t.Errorf("an unknown label: %v", err)
	}
	f.Relabel(Ref{"app", n}, "", "swarm:blocked")
	if is, _ := f.Issue(n); !reflect.DeepEqual(is.Labels, []string{"bug", "swarm:plan-0-posted", "swarm:blocked"}) {
		t.Errorf("remove \"\" only adds: %v", is.Labels)
	}
	f.Relabel(Ref{"app", n}, "", Gated)
	f.SetLabel(Ref{"app", n}, "swarm:plan-1-accepted")
	if is, _ := f.Issue(n); !reflect.DeepEqual(is.Labels, []string{"bug", "swarm:blocked", Gated, "swarm:plan-1-accepted"}) {
		t.Errorf("SetLabel moves the stage and keeps the flags: %v", is.Labels)
	}
	if err := f.Relabel(Ref{"app", n}, Gated, ""); err != nil {
		t.Fatal(err)
	}
	if is, _ := f.Issue(n); !reflect.DeepEqual(is.Labels, []string{"bug", "swarm:blocked", "swarm:plan-1-accepted"}) {
		t.Errorf("add \"\" only removes: %v", is.Labels)
	}
	if _, err := f.CreateIssue("B", "", []string{"wontfix"}); errText(err) != "forge: CreateIssue: no label wontfix for acme/app; run 'daisy swarm init', which ensures the sixteen" {
		t.Errorf("CreateIssue with an unknown label: %v", err)
	}
}

func TestFakePullsReviewsAndMerge(t *testing.T) {
	f := fakeForge(t)
	n, _ := f.CreateIssue("Job", "", []string{"swarm:plan-1-accepted"})
	if _, err := f.CreatePull("other", "swarm/1-1", "main", "x", ""); errText(err) != "forge: CreatePull: other is not a declared repository (app, lib)" {
		t.Errorf("undeclared: %v", err)
	}
	lib, _ := f.CreatePull("lib", "swarm/1-1", "main", "Slice 1", "Refs acme/app#1")
	if _, err := f.CreatePull("lib", "swarm/1-1", "main", "again", "Refs acme/app#1"); errText(err) != "forge: POST /repos/acme/lib/pulls: 409 pull request already exists for these targets" {
		t.Errorf("a second open pull request: %v", err)
	}
	app, _ := f.CreatePull("app", "swarm/1-2", "main", "Slice 2", "Refs #1")
	f.CreatePull("app", "feature/x", "main", "not the swarm's", "Refs #1")
	f.CreatePull("lib", "swarm/1-3", "main", "short form outside forge.repo", "Refs #1")
	lr, ar := Ref{"lib", lib}, Ref{"app", app}

	f.Push(lr, "c1")
	f.Review(lr, Approve, "VERDICT: approve")
	f.SayOnLine(lr, "jane", "a.go", 4, "why?")
	f.Say(lr, "jane", "and here")
	f.Push(lr, "c2")
	revs, _ := f.Reviews(lr)
	lines, _ := f.ReviewComments(lr)
	pcs, _ := f.PullComments(lr)
	if len(revs) != 1 || revs[0] != (Review{ID: 1, Author: "swarm-bot", Body: "VERDICT: approve", State: Commented, Commit: "c1", Submitted: at(2026, 11, 6, 9, 6)}) {
		t.Errorf("Reviews = %+v: a comment-type review on the head", revs)
	}
	if len(lines) != 1 || lines[0].Commit != "c1" || lines[0].Line != 4 || lines[0].Author != "jane" || len(pcs) != 1 || pcs[0].Body != "and here" {
		t.Errorf("ReviewComments %+v, PullComments %+v", lines, pcs)
	}

	if err := f.Merge(lr, "c1"); !errors.Is(err, ErrNotMergeable) || errText(err) != "forge: not mergeable: 409 head out of date" {
		t.Errorf("a moved head: %v", err)
	}
	f.Unmergeable(ar)
	if err := f.Merge(ar, ""); !errors.Is(err, ErrNotMergeable) {
		t.Errorf("unmergeable: %v", err)
	}
	if err := f.Merge(lr, "c2"); err != nil {
		t.Fatal(err)
	}
	pulls, _ := f.Pulls(n)
	if len(pulls) != 2 || pulls[0].Repo != "lib" || pulls[0].Number != lib || !pulls[0].Merged || pulls[0].Open || len(pulls[0].MergeSHA) != 40 ||
		pulls[0].HeadSHA != "c2" || pulls[1].Repo != "app" || pulls[1].Number != app || pulls[1].Merged || pulls[1].MergeSHA != "" {
		t.Errorf("Pulls(1) = %+v", pulls)
	}
	if is, _ := f.Issue(n); !is.Open {
		t.Error("Merge closed the issue")
	}
	if err := f.Merge(lr, "c2"); !errors.Is(err, ErrNotMergeable) {
		t.Errorf("a merged pull request merged again: %v", err)
	}
}

// The Fake's milestones: one per title; the counts are the issues and pull
// requests of forge.repo in each, as Gitea counts them; SetMilestone takes a
// milestone that exists, and after RefuseMilestones a create and a set
// fail as the forge's 403.
func TestFakeMilestones(t *testing.T) {
	f := fakeForge(t)
	a, _ := f.CreateIssue("A", "", []string{"swarm:proposal-accepted"})
	b, _ := f.CreateIssue("B", "", []string{"bug"})
	pr, _ := f.CreatePull("app", "swarm/1-1", "main", "P", "Refs #1")
	first, _ := f.EnsureMilestone("m0: First")
	second, _ := f.EnsureMilestone("m0: Second")
	if again, err := f.EnsureMilestone("m0: First"); again != first || err != nil || first == second {
		t.Errorf("EnsureMilestone again = %d, %v; first %d, second %d", again, err, first, second)
	}
	for _, n := range []int{a, b, pr} {
		if err := f.SetMilestone(n, first); err != nil {
			t.Fatal(err)
		}
	}
	f.CloseIssue(b)
	if is, _ := f.Issue(a); is.Milestone != first {
		t.Errorf("Issue(%d).Milestone = %d", a, is.Milestone)
	}
	if got, _ := f.Issues(Closed, "bug"); len(got) != 1 || got[0].Milestone != first {
		t.Errorf("Issues(Closed) = %+v", got)
	}
	ms, err := f.Milestones()
	if want := []Milestone{{ID: first, Title: "m0: First", Open: 2, Closed: 1}, {ID: second, Title: "m0: Second"}}; err != nil || !reflect.DeepEqual(ms, want) {
		t.Errorf("Milestones = %+v, %v", ms, err)
	}
	if err := f.SetMilestone(a, 9); errText(err) != "forge: PATCH /repos/acme/app/issues/1: 422 milestone 9 does not exist" {
		t.Errorf("an unknown milestone: %v", err)
	}
	if err := f.SetMilestone(99, first); errText(err) != "forge: acme/app#99: 404 not found" {
		t.Errorf("an unknown issue: %v", err)
	}
	f.RefuseMilestones()
	if _, err := f.EnsureMilestone("m0: Third"); errText(err) != "forge: POST /repos/acme/app/milestones: 403 forbidden" {
		t.Errorf("a refused create: %v", err)
	}
	if id, err := f.EnsureMilestone("m0: Second"); id != second || err != nil {
		t.Errorf("an existing milestone is found while refused: %d, %v", id, err)
	}
	if err := f.SetMilestone(a, second); errText(err) != "forge: PATCH /repos/acme/app/issues/1: 403 forbidden" {
		t.Errorf("a refused set: %v", err)
	}
}

// The maintainer's token on the Fake: As is the same forge under another
// account, whose writes it authors; the timeline names the account behind
// each label change, close and merge; Approve refuses the account's own
// pull request as the forge does (422) and stores a formal approval
// otherwise.
func TestFakeAsAnotherAccount(t *testing.T) {
	f := fakeForge(t)
	jane := f.As(Account{Login: "jane"})
	n, _ := f.CreateIssue("A", "", []string{"swarm:proposal-posted"})
	pr, _ := f.CreatePull("app", "swarm/1-1", "main", "P", "Refs #1")
	at, pull := Ref{"app", n}, Ref{"app", pr}
	f.Push(pull, "c1")
	if acct, _ := jane.Login(); acct.Login != "jane" {
		t.Errorf("jane's Login = %+v", acct)
	}
	if is, _ := jane.Issue(n); is.Author != "swarm-bot" {
		t.Errorf("jane reads another forge: %+v", is)
	}
	f.Relabel(at, "swarm:proposal-posted", "swarm:proposal-accepted")
	jane.Relabel(at, "swarm:proposal-accepted", "swarm:plan-1-accepted")
	jane.Relabel(at, "swarm:proposal-accepted", "swarm:plan-1-accepted") // nothing changes, nothing recorded
	jane.Comment(at, "go")
	if err := f.Approve(pull, "c1"); errText(err) != "forge: POST /repos/acme/app/pulls/2/reviews: 422 approve your own pull is not allowed" {
		t.Errorf("the swarm approving its own pull request: %v", err)
	}
	if err := jane.Approve(pull, "c1"); err != nil {
		t.Fatal(err)
	}
	jane.Merge(pull, "c1")
	jane.CloseIssue(n)
	jane.CloseIssue(n)
	want := []Event{{"swarm-bot", "labeled swarm:proposal-accepted"}, {"swarm-bot", "unlabeled swarm:proposal-posted"}, {"jane", "labeled swarm:plan-1-accepted"}, {"jane", "unlabeled swarm:proposal-accepted"}, {"jane", "closed"}}
	if got := f.Events(at); !reflect.DeepEqual(got, want) {
		t.Errorf("issue events = %+v\nwant %+v", got, want)
	}
	if got := f.Events(pull); !reflect.DeepEqual(got, []Event{{"jane", "merged"}}) {
		t.Errorf("pull events = %+v", got)
	}
	is, _ := f.Issue(n)
	reviews, _ := f.Reviews(pull)
	if len(is.Comments) != 1 || is.Comments[0].Author != "jane" || len(reviews) != 1 || reviews[0].Author != "jane" || reviews[0].State != Approve || reviews[0].Commit != "c1" {
		t.Errorf("comments %+v, reviews %+v", is.Comments, reviews)
	}
}

// Pulls names who opened each pull request, as gitea's `user.login` does.
func TestFakePullAuthor(t *testing.T) {
	f := fakeForge(t)
	f.CreateIssue("A", "", []string{"swarm:proposal-posted"})
	f.CreatePull("app", "swarm/1-1", "main", "P", "Refs #1")
	f.As(Account{Login: "mallory"}).CreatePull("app", "swarm/1-2", "main", "Q", "Refs #1")
	ps, err := f.Pulls(1)
	if err != nil || len(ps) != 2 || ps[0].Author != f.account.Login || ps[1].Author != "mallory" {
		t.Errorf("Pulls(1) = %+v, %v", ps, err)
	}
}

// The one-time migration (section 14, item 74): a Fake holding the eleven
// labels of Step 14 under their old names renames the seven old stage
// names in place, every issue and pull request carrying one, open or
// closed, reading its new name, and creates the four the set lacks; a
// second run makes nothing; a refused one names what is left to make.
func TestFakeEnsureLabelsRenamesTheOldNames(t *testing.T) {
	o := &ops.Fake{NowFn: func() time.Time { return at(2026, 11, 7, 23, 0) }}
	f := NewFake(o, "acme/app", []string{"app"}, Account{Login: "swarm-bot"})
	before := []string{"swarm:proposed", "swarm:ready", "swarm:planning", "swarm:planned", "swarm:in-review", "swarm:reviewed", "swarm:approved", "swarm:blocked", "swarm:gated", "swarm:done", "swarm:rejected", "bug"}
	f.AddLabels(before...)
	open, _ := f.CreateIssue("Open", "", []string{"bug", "swarm:in-review", Gated})
	pr, _ := f.CreatePull("app", "swarm/1-1", "main", "P", "Refs #1")
	f.Relabel(Ref{"app", pr}, "", "swarm:in-review")
	closed, _ := f.CreateIssue("Closed", "", []string{"swarm:approved"})
	f.CloseIssue(closed)
	blocked, _ := f.CreateIssue("Blocked", "", []string{"swarm:blocked"})

	refused := NewFake(o, "acme/app", nil, Account{Login: "swarm-bot"})
	refused.AddLabels(before...)
	refused.RefuseLabels()
	if _, _, err := refused.EnsureLabels(Labels); errText(err) != "forge: EnsureLabels: acme's labels to rename (keeping their issues): swarm:proposed as swarm:proposal-posted, swarm:ready as swarm:proposal-accepted, swarm:planning as swarm:plan-0-posted, swarm:planned as swarm:plan-1-accepted, swarm:in-review as swarm:review-0-ready, swarm:approved as swarm:review-2-approved, swarm:reviewed as swarm:review-3-changes-requested; missing: swarm:plan-2-in-progress, swarm:review-1-in-progress, swarm:paused, swarm:blocking, swarm:class-architecture; an owner of acme makes them once" {
		t.Errorf("refused: %v", err)
	}
	if !reflect.DeepEqual(refused.Labels(), before) {
		t.Errorf("a refused migration changed the set: %v", refused.Labels())
	}

	created, renamed, err := f.EnsureLabels(Labels)
	if err != nil || created != 5 || renamed != 7 {
		t.Fatalf("EnsureLabels = %d created, %d renamed, %v", created, renamed, err)
	}
	for _, l := range Labels {
		if !slices.Contains(f.Labels(), l) {
			t.Errorf("%s missing after the migration", l)
		}
	}
	for old := range Renamed {
		if slices.Contains(f.Labels(), old) {
			t.Errorf("%s still in the set", old)
		}
	}
	read := func(at Ref) []string {
		f.mu.Lock()
		defer f.mu.Unlock()
		return must(f.find(at)).labels
	}
	for _, tc := range []struct {
		at   Ref
		want []string
	}{
		{Ref{"app", open}, []string{"bug", "swarm:review-0-ready", Gated}},
		{Ref{"app", pr}, []string{"swarm:review-0-ready"}},
		{Ref{"app", closed}, []string{"swarm:review-2-approved"}},
		{Ref{"app", blocked}, []string{"swarm:blocked"}},
	} {
		if got := read(tc.at); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("#%d: %v, want %v", tc.at.N, got, tc.want)
		}
	}
	if got, _ := f.Issues(Closed, "swarm:review-2-approved"); len(got) != 1 || got[0].Number != closed {
		t.Errorf("the closed issue is not listed by its new name: %+v", got)
	}
	if created, renamed, err := f.EnsureLabels(Labels); created != 0 || renamed != 0 || err != nil {
		t.Errorf("a second run: %d created, %d renamed, %v", created, renamed, err)
	}
}
