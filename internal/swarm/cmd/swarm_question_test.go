package cmd

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// lastComment is issue n's newest comment's body.
func (a *applyWS) lastComment(n int) string {
	a.t.Helper()
	cs := a.issue(n).Comments
	if len(cs) == 0 {
		return ""
	}
	return cs[len(cs)-1].Body
}

const askSentence = `When the plan, the issue and its thread leave open something your work depends on, do not guess: write "QUESTION: <one question>" as the first line of your output instead; the architect answers it, and you run again with the answer under "# Questions".`

// The question answered (section 6, The question): the worker's
// QUESTION: is posted on the issue, which gains swarm:question beside its
// stage and launches no one but the architect, at the next pass, on
// plan-1-accepted; its answer is posted marked ANSWER:, the flag
// cleared, the label kept and no plan taken from it; the worker then
// runs again, the answer under # Questions and the plan still the one
// before.
func TestSwarmApplyQuestionAnswered(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go say v1 or v1.0?\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted, forge.Question}) || a.lastComment(1) != mark("worker", 1)+"QUESTION: Should main.go say v1 or v1.0?\n" {
		t.Fatalf("labels %v, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, askSentence) {
		t.Errorf("worker's system.md:\n%s", sys)
	}
	if r := a.result("1-worker-1"); !strings.Contains(r, "relabelled swarm:plan-2-in-progress swarm:plan-1-accepted\nflagged swarm:question\n") || !strings.HasSuffix(r, "outcome question\ncollected\n") {
		t.Errorf("worker run 1:\n%s", r)
	}
	if !strings.Contains(a.stdout.String(), "#1 worker run 1: question\n") || !reflect.DeepEqual(a.launchedRoles(), []string{"architect", "worker"}) {
		t.Errorf("launched %q\n%s", a.launchedRoles(), a.stdout.String())
	}

	a.runner.Script(1, "architect", runner.Canned{Out: "v1, as the Test Plan's first claim says.\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect"}) {
		t.Fatalf("pass 2 launched %q\n%s", got, a.stdout.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted}) || a.lastComment(1) != mark("architect", 2)+"ANSWER: v1, as the Test Plan's first claim says.\n" {
		t.Fatalf("labels %v, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	if r := a.result("1-architect-2"); !strings.Contains(r, "label swarm:plan-1-accepted\nquestion\n") || strings.Contains(r, "relabelled") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("architect run 2:\n%s", r)
	}
	if sys := a.file("1-architect-2", "io/system.md"); !strings.Contains(sys, `Worker run 1 asks a question on issue #1, its comment beginning "QUESTION:" under "# Thread". Answer it from the plan, the thread and the resources`) ||
		strings.Contains(sys, "builder.md") || !strings.Contains(sys, "that file is the only thing read back") {
		t.Errorf("architect's system.md:\n%s", sys)
	}
	if !strings.Contains(a.stdout.String(), "#1 question cleared\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}

	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect", "worker", "check"}) {
		t.Fatalf("pass 3 launched %q\n%s", got, a.stdout.String())
	}
	prompt := a.file("1-worker-2", "io/prompt.md")
	_, questions, _ := strings.Cut(prompt, "\n# Questions\n\n")
	if !strings.Contains(questions, "## worker, run 1, at ") || !strings.Contains(questions, "\n\nQUESTION: Should main.go say v1 or v1.0?\n\n## architect, run 2, at ") ||
		!strings.Contains(questions, "\n\nANSWER: v1, as the Test Plan's first claim says.\n") || !strings.Contains(prompt, "\n- Plan: /job/memory/plan.md, architect run 1, ") || strings.Contains(prompt, "# Plan") {
		t.Errorf("worker run 2's prompt.md:\n%s", prompt)
	}
	if sys := a.file("1-worker-2", "io/system.md"); !strings.Contains(sys, "Implement slice 1 of the accepted plan for issue #1 (it declares 1)") {
		t.Errorf("worker run 2's system.md:\n%s", sys)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) {
		t.Errorf("labels %v", got)
	}
}

// The question passed on (section 6, The question): the architect's
// QUESTION: is posted and the flag kept, so the owner answers at the
// next pass; the owner's QUESTION: passes it to the maintainer, the issue
// blocked with the reconciler's note naming the question and the flag
// cleared; nothing runs until the maintainer answers, and status lists
// it as a question; unblock's comment then runs the worker, which reads
// the questions and the answer.
func TestSwarmApplyQuestionToTheMaintainer(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go keep the old flag?\n"})
	a.pass()
	a.runner.Script(1, "architect", runner.Canned{Out: "QUESTION: Does the brief want the old flag kept?\n", Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted, forge.Question}) || a.lastComment(1) != mark("architect", 2)+"QUESTION: Does the brief want the old flag kept?\n" {
		t.Fatalf("labels %v, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	if r := a.result("1-architect-2"); !strings.HasSuffix(r, "outcome question\ncollected\n") {
		t.Errorf("architect run 2:\n%s", r)
	}

	a.runner.Script(1, "owner", runner.Canned{Out: "QUESTION: Keep the old flag, or drop it?\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect", "owner"}) {
		t.Fatalf("pass 3 launched %q\n%s", got, a.stdout.String())
	}
	if sys := a.file("1-owner-1", "io/system.md"); !strings.Contains(sys, `The architect passed you a question on issue #1, its comment beginning "QUESTION:" under "# Thread"`) {
		t.Errorf("owner's system.md:\n%s", sys)
	}
	is := a.issue(1)
	note := mark("reconciler", 1) + "owner run 1 passed a question to the maintainer (at swarm:plan-1-accepted): Keep the old flag, or drop it? Answer, then remove swarm:blocked to resume.\n\ncc @jane"
	if got := labels(is); !reflect.DeepEqual(got, []string{forge.PlanAccepted, forge.Blocked, forge.Gated}) {
		t.Fatalf("labels %v\n%s", got, a.stdout.String())
	}
	if bs := a.bodies(1); len(bs) < 2 || bs[len(bs)-2] != mark("owner", 1)+"QUESTION: Keep the old flag, or drop it?\n" || bs[len(bs)-1] != note {
		t.Errorf("comments %q", bs)
	}
	if r := a.result("1-owner-1"); !strings.Contains(r, "flagged swarm:blocked\n") || !strings.HasSuffix(r, "outcome blocked\ncollected\n") {
		t.Errorf("owner run 1:\n%s", r)
	}

	a.pass()
	if got := len(a.runner.Launches()); got != 4 {
		t.Errorf("launched %q while blocked", a.launchedRoles())
	}
	if ns, line := a.waitingSet(); !reflect.DeepEqual(ns, []int{1}) || line != "Waiting for you: question #1 (architecture)" {
		t.Errorf("waiting %v: %q", ns, line)
	}

	m := &maintainerWS{statusWS: a.statusWS}
	m.secrets(bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
	if err := m.verb(swarmUnblock, "1", "Drop it: nothing reads it."); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect", "owner", "worker", "check"}) {
		t.Fatalf("after unblock launched %q\n%s", got, a.stdout.String())
	}
	_, questions, _ := strings.Cut(a.file("1-worker-2", "io/prompt.md"), "\n# Questions\n\n")
	for _, want := range []string{"QUESTION: Should main.go keep the old flag?", "QUESTION: Does the brief want the old flag kept?", "QUESTION: Keep the old flag, or drop it?", "## jane at ", "Drop it: nothing reads it."} {
		if !strings.Contains(questions, want) {
			t.Errorf("# Questions lacks %q:\n%s", want, questions)
		}
	}
}

// A reviewer's question: the sympathetic reviewer's QUESTION:, launched
// once the adversary approved (decision 163), takes the issue home to
// review-0-ready with the flag, the adversary's review standing; the
// architect answers it with the pull request in its prompt, and only the
// sympathetic reviewer runs again, with the answer under # Questions.
func TestSwarmApplyReviewerQuestion(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: "QUESTION: Is v1 the release's name or the file's?\n"})
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer, once the adversary approved, asks
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady, forge.Question}) || a.lastComment(1) != mark("sympathetic", 1)+"QUESTION: Is v1 the release's name or the file's?\n" {
		t.Fatalf("labels %v, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "ANSWER: the file's.\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "sympathetic", "architect"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if prompt := a.file("1-architect-2", "io/prompt.md"); !strings.Contains(prompt, "# Pull request app#1: ") {
		t.Errorf("architect's prompt.md:\n%s", prompt)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) || a.lastComment(1) != mark("architect", 2)+"ANSWER: the file's.\n" {
		t.Fatalf("labels %v, last %q", got, a.lastComment(1))
	}
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "sympathetic", "architect", "sympathetic"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if _, questions, _ := strings.Cut(a.file("1-sympathetic-2", "io/prompt.md"), "\n# Questions\n\n"); !strings.Contains(questions, "ANSWER: the file's.") {
		t.Errorf("sympathetic run 2's # Questions:\n%s", questions)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewApproved, forge.Gated}) {
		t.Errorf("labels %v", got)
	}
}

// A ruling that lands while a question is open between slices waits for
// the answer (section 6, The rulings): no note while the flag is on, the
// architect answering the question; the next pass notes the ruling and
// runs the architect on it, rather than reading its answer as its word on
// the plan and running no one.
func TestSwarmApplyQuestionBeforeRulings(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.betweenSlices(nil)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Does slice 2 keep notes.md's name?\n"})
	a.pass()
	a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27. Roles\n"}, "ADR 0027")
	a.runner.Script(1, "architect", runner.Canned{Out: "ANSWER: it does.\n", Stdout: claudeResult}, runner.Canned{Out: "STANDS: no slice names a role.\n", Stdout: claudeResult})
	a.pass()
	if notes := a.rulingsNotes(); len(notes) != 0 || a.lastComment(1) != mark("architect", 2)+"ANSWER: it does.\n" {
		t.Fatalf("notes %q, last %q\n%s", notes, a.lastComment(1), a.stdout.String())
	}
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect", "architect"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if notes := a.rulingsNotes(); len(notes) != 1 || a.lastComment(1) != mark("architect", 3)+"STANDS: no slice names a role.\n" {
		t.Errorf("notes %q, last %q", notes, a.lastComment(1))
	}
}

// An answer and an architect's question are no plan revisions (section
// 6, The question): the worker asks, the architect passes the question
// on, the owner answers, and status's Throughput counts no revision of
// the one plan.
func TestSwarmQuestionsAreNoPlanRevisions(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go keep the old flag?\n"})
	a.pass()
	a.runner.Script(1, "architect", runner.Canned{Out: "QUESTION: Does the brief want the old flag kept?\n", Stdout: claudeResult})
	a.pass()
	a.runner.Script(1, "owner", runner.Canned{Out: "ANSWER: drop it.\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "architect", "owner"}) || a.lastComment(1) != mark("owner", 1)+"ANSWER: drop it.\n" {
		t.Fatalf("launched %q, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.stdout.String(), "\n  #1           plan revisions 0, blocks 0") {
		t.Errorf("status:\n%s", a.stdout.String())
	}
}

// The adversary's question (section 6, The question): its QUESTION:
// takes the issue home to review-0-ready with the flag, the architect
// answers it, and the adversary runs again, the answer under
// # Questions.
func TestSwarmApplyAdversaryQuestion(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: "QUESTION: Is v1 the release's name or the file's?\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady, forge.Question}) || a.lastComment(1) != mark("adversary", 1)+"QUESTION: Is v1 the release's name or the file's?\n" {
		t.Fatalf("labels %v, last %q\n%s", got, a.lastComment(1), a.stdout.String())
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "ANSWER: the file's.\n", Stdout: claudeResult})
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "architect", "adversary"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if _, questions, _ := strings.Cut(a.file("1-adversary-2", "io/prompt.md"), "\n# Questions\n\n"); !strings.Contains(questions, "ANSWER: the file's.") {
		t.Errorf("adversary run 2's # Questions:\n%s", questions)
	}
}

// A question below the maintainer's level waits for no one but the
// swarm (section 6, The question; section 11): while the architect is
// to answer the worker's question, status's Waiting line names nothing.
func TestSwarmStatusQuestionBelowTheMaintainer(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go say v1 or v1.0?\n"})
	a.pass()
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Question) {
		t.Fatalf("labels %v", got)
	}
	if ns, line := a.waitingSet(); ns != nil || line != "Waiting for you: nothing" {
		t.Errorf("waiting %v: %q", ns, line)
	}
}

// The worker's question at review-3-changes-requested (section 6, The
// question): posted on the issue, its pull request's thread left as it
// was; the architect answers with the pull request in its prompt; the
// worker then runs again on the review it was to answer, and pushes.
func TestSwarmApplyQuestionAtChangesRequested(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.changesRequested()
	pull := forge.Ref{Repo: "app", N: 1}
	before, _ := a.forge.PullComments(pull)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Which of the two findings comes first?\n"})
	a.pass()
	after, _ := a.forge.PullComments(pull)
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested, forge.Question}) || a.lastComment(1) != mark("worker", 2)+"QUESTION: Which of the two findings comes first?\n" || len(after) != len(before) {
		t.Fatalf("labels %v, last %q, pull comments %d → %d\n%s", got, a.lastComment(1), len(before), len(after), a.stdout.String())
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "ANSWER: the blocking one.\n", Stdout: claudeResult})
	a.pass()
	if prompt := a.file("1-architect-2", "io/prompt.md"); !strings.Contains(prompt, "# Pull request app#1: ") {
		t.Errorf("architect's prompt.md:\n%s", prompt)
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) {
		t.Errorf("labels %v\n%s", got, a.stdout.String())
	}
	if _, questions, _ := strings.Cut(a.file("1-worker-3", "io/prompt.md"), "\n# Questions\n\n"); !strings.Contains(questions, "ANSWER: the blocking one.") {
		t.Errorf("worker run 3's # Questions:\n%s", questions)
	}
}

// An architect's BLOCKED: on a question is T17 (section 6, The
// question): the issue blocked and swarm:question cleared, the question
// the maintainer's.
func TestSwarmApplyQuestionBlocked(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go keep the old flag?\n"})
	a.pass()
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: neither the plan nor the brief says.\n", Stdout: claudeResult})
	a.pass()
	got := labels(a.issue(1))
	if !slices.Contains(got, forge.Blocked) || slices.Contains(got, forge.Question) || !slices.Contains(got, forge.PlanAccepted) {
		t.Errorf("labels %v\n%s", got, a.stdout.String())
	}
}
