package cmd

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// replan is `daisy swarm replan` with args, as the cli runs it, under the
// maintainer's token (maintainer), over the fixture's forge.
func (a *applyWS) replan(args ...string) error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{GetenvFn: func(k string) string { return a.vars[k] }, NowFn: a.Now}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Args: args}
	return a.at(c, false, func(c *Context) error {
		return swarmReplan(c, func(_ ops.Ops, _ declare.Forge, _ []string, token string) (forge.Adapter, error) {
			if g, ok := a.as[token]; ok {
				return g, nil
			}
			return nil, errors.New("no account for this token")
		})
	})
}

// replans is jane's re-plan requests on #1.
func (a *applyWS) replans() []string {
	var out []string
	for _, c := range a.issue(1).Comments {
		if c.Author == "jane" && strings.HasPrefix(c.Body, "Re-plan") {
			out = append(out, c.Body)
		}
	}
	return out
}

// S4's acceptance (docs/swarm.md section 6, The re-plan): between slices,
// `swarm replan 1 <direction>` posts the direction as the maintainer's
// comment, the stage kept, and the next pass runs the architect, not the
// worker, with the direction in its prompt and Step 9's revision Task
// text, slice 1's merged pull request kept; no block or unblock. Its
// STANDS: keeps the plan, and the pass after runs the worker on slice 2.
func TestSwarmReplan(t *testing.T) {
	t.Parallel()
	for _, sameSecond := range []bool{false, true} {
		t.Run(map[bool]string{false: "a later request", true: "a request in the architect's second"}[sameSecond], func(t *testing.T) {
			testSwarmReplan(t, sameSecond)
		})
	}
}

// testSwarmReplan is TestSwarmReplan's run; with sameSecond the request
// is posted at the time of the architect's newest word on the plan, after
// it in the thread, and is read as after it all the same.
func testSwarmReplan(t *testing.T, sameSecond bool) {
	a := newApplyWS(t)
	a.betweenSlices(nil)
	a.maintainer()
	a.tick()
	later := a.Now()
	if sameSecond {
		var basis time.Time
		for _, c := range a.issue(1).Comments {
			if strings.HasPrefix(c.Body, mark("architect", 1)) {
				basis = c.Created
			}
		}
		a.mu.Lock()
		a.now = basis
		a.mu.Unlock()
	}
	if err := a.replan("1", "re-plan", "against", "ADR", "0031"); err != nil || a.stdout.String() != "Maintainer jane on acme/swarm-app\n#1 commented on the issue\n" {
		t.Fatalf("replan: %v\n%s%s", err, a.stdout.String(), a.stderr.String())
	}
	a.mu.Lock()
	a.now = later
	a.mu.Unlock()
	if got := a.replans(); !reflect.DeepEqual(got, []string{"Re-plan: re-plan against ADR 0031"}) || a.janes(issue1) != nil {
		t.Fatalf("comments %q, jane's label changes %q", got, a.janes(issue1))
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "STANDS: slice 2 already follows ADR 0031.\n", Stdout: claudeResult})
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect"}) {
		t.Fatalf("launched %q\n%s\n%s", got, a.stdout.String(), a.stderr.String())
	}
	if p := a.file("1-architect-2", "io/prompt.md"); !strings.Contains(p, "Re-plan: re-plan against ADR 0031") {
		t.Errorf("prompt.md lacks the direction:\n%s", p)
	}
	sys := a.file("1-architect-2", "io/system.md")
	for _, want := range []string{
		"The maintainer asks you to re-plan issue #1: the comment of jane at 2026-11-06T15:",
		`beginning "Re-plan" under "# Thread", is the direction. Revise the remaining slices as it directs, or confirm they stand.`,
		`Slice 1 has a pull request, under "# Earlier pull requests": keep its line as it is, with its number and repository.`,
		`write "STANDS: <why>" as the first line of your output instead, then why they answer the maintainer's comment.`,
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("system.md lacks %q:\n%s", want, sys)
		}
	}
	if strings.Contains(sys, "Rulings changed") {
		t.Errorf("system.md names rulings:\n%s", sys)
	}
	if is := a.issue(1); !reflect.DeepEqual(labels(is), []string{forge.PlanAccepted}) || len(a.rulingsNotes()) != 0 {
		t.Errorf("labels %v, notes %q", labels(is), a.rulingsNotes())
	}
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect", "worker"}) {
		t.Fatalf("pass 2 launched %q\n%s", got, a.stdout.String())
	}
	if r := a.result("1-worker-1"); !strings.Contains(r, "branch swarm/1-2 in app\n") {
		t.Errorf("the worker did not build slice 2:\n%s", r)
	}
	if got := a.replans(); len(got) != 1 {
		t.Errorf("comments %q", got)
	}
}

// The refusal's way on: #1 in review, its pull request open, is
// refused; closed by the maintainer, replan moves the job to
// plan-1-accepted and the architect revises the remaining slices, told
// that slice 1's pull request was closed unmerged, so its work goes in
// a later slice; its STANDS: runs the worker on slice 2 next.
func TestSwarmReplanAfterTheClose(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(twoSlices)
	a.maintainer()
	pr := a.pulls(1)[0]
	if err := a.replan("1", "Split", "slice", "1."); !strings.Contains(errText(err), "is open (swarm:review-0-ready); close or merge it first, then replan; nothing changed") {
		t.Fatalf("replan with the pull request open: %v", err)
	}
	a.forge.ClosePull(forge.Ref{Repo: pr.Repo, N: pr.Number})
	a.tick()
	if err := a.replan("1", "Split", "slice", "1."); err != nil || a.stdout.String() != "Maintainer jane on acme/swarm-app\n#1 commented on the issue\n#1 swarm:review-0-ready → swarm:plan-1-accepted\n" {
		t.Fatalf("replan: %v\n%s%s", err, a.stdout.String(), a.stderr.String())
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "STANDS: slice 2 carries slice 1's work.\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "architect"}) {
		t.Fatalf("launched %q\n%s\n%s", got, a.stdout.String(), a.stderr.String())
	}
	if sys := a.file("1-architect-2", "io/system.md"); !strings.Contains(sys, fmt.Sprintf(`Pull request app#%d, slice 1, was closed unmerged: keep its line, its number being taken, and put what it was to do, if it is still wanted, in a later slice.`, pr.Number)) {
		t.Errorf("system.md:\n%s", sys)
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if r := a.result("1-worker-2"); !strings.Contains(r, "branch swarm/1-2 in app\n") {
		t.Errorf("worker 2:\n%s", r)
	}
}

// The comment form: `/swarm replan <direction>` on the issue runs the
// verb as the maintainer, which posts the direction as a comment of its
// own (the command comment is no counted entry), once; the architect
// runs in the same pass with it, and its revised plan goes to the plan
// gate (T27); a second pass runs and posts nothing more.
func TestSwarmApplyCommandReplan(t *testing.T) {
	t.Parallel()
	revised := strings.Replace(twoSlices, "2. app: add notes.md.", "2. app: add notes.md, by ADR 0031.", 1)
	a := newApplyWS(t)
	a.betweenSlices(nil)
	a.maintainer()
	id := a.say(issue1, "jane", "/swarm replan re-plan against ADR 0031")
	a.runner.Script(1, "architect", runner.Canned{Out: revised, Stdout: claudeResult})
	a.pass()
	want := answer(id, "/swarm replan re-plan against ADR 0031", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 commented on the issue\n")
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("answers %q\nwant %q\n%s\n%s", got, want, a.stdout.String(), a.stderr.String())
	}
	if got := a.replans(); !reflect.DeepEqual(got, []string{"Re-plan: re-plan against ADR 0031"}) {
		t.Errorf("comments %q", got)
	}
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if p := a.file("1-architect-2", "io/prompt.md"); !strings.Contains(p, "Re-plan: re-plan against ADR 0031") {
		t.Errorf("prompt.md:\n%s", p)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanPosted, forge.Gated}) {
		t.Errorf("labels %v", got)
	}
	a.pass()
	if got := a.launchedRoles(); len(got) != 2 || len(a.answers(issue1)) != 1 || len(a.replans()) != 1 {
		t.Errorf("pass 2: launched %q, answers %q, comments %q", got, a.answers(issue1), a.replans())
	}
	if journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md"); !strings.Contains(journal, fmt.Sprintf(" 1-command-%d: /swarm replan re-plan against ADR 0031 by jane; done\n", id)) {
		t.Errorf("journal:\n%s", journal)
	}
}

// Where replan moves the job (section 11): at proposal-accepted and
// plan-0-posted the architect plans or revises from the comment, the
// stage kept; plan-1-accepted with no plan (the fast lane) goes to
// proposal-accepted; a review stage whose pull request is no longer open
// goes to plan-1-accepted with a plan, so the architect revises the
// remaining slices. With no direction the comment is `Re-plan.`.
func TestSwarmReplanStages(t *testing.T) {
	t.Parallel()
	m := newMaintainerWS(t)
	const head = "Maintainer jane on acme/swarm-app\n"
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{forge.ProposalAccepted}) // #1
	m.ok(swarmReplan, head+"#1 commented on the issue\n", "1", "Use", "the", "cache.")
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{forge.PlanPosted, forge.Gated}) // #2
	m.forge.Comment(m.ref(2), mark("architect", 1)+twoSlices)
	m.ok(swarmReplan, head+"#2 commented on the issue\n", "2")
	m.forge.CreateIssue("C", mark("owner", 1)+"c", []string{forge.PlanAccepted}) // #3
	m.ok(swarmReplan, head+"#3 commented on the issue\n#3 swarm:plan-1-accepted → swarm:proposal-accepted\n", "3")
	m.forge.CreateIssue("D", mark("owner", 1)+"d", []string{forge.ReviewReady}) // #4
	m.forge.Comment(m.ref(4), mark("architect", 1)+twoSlices)
	n, _ := m.forge.CreatePull("app", "swarm/4-1", "main", "slice", "Refs acme/swarm-app#4")
	m.forge.Push(forge.Ref{Repo: "app", N: n}, "c1")
	m.forge.ClosePull(forge.Ref{Repo: "app", N: n})
	m.ok(swarmReplan, head+"#4 commented on the issue\n#4 swarm:review-0-ready → swarm:plan-1-accepted\n", "4", "Split", "slice", "1.")
	want := []string{
		`jane comment "Re-plan: Use the cache." #1`,
		`jane comment "Re-plan." #2`,
		"jane labeled swarm:proposal-accepted #3", "jane unlabeled swarm:plan-1-accepted #3", `jane comment "Re-plan." #3`,
		"jane labeled swarm:plan-1-accepted #4", "jane unlabeled swarm:review-0-ready #4", `jane comment "Re-plan: Split slice 1." #4`,
	}
	if got := m.writes(1, 2, 3, 4); !reflect.DeepEqual(got, want) {
		t.Errorf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if is, _ := m.forge.Issue(2); !reflect.DeepEqual(is.Labels, []string{forge.PlanPosted, forge.Gated}) {
		t.Errorf("#2 %v", is.Labels)
	}
}

// What replan refuses, nothing changed (section 11): a proposal not yet
// triaged, a worker's run, a run on the issue not yet collected, an open
// pull request in review or under way, a question waiting for its
// answer, a maintainer not in trusted, and a direction holding a command
// line.
func TestSwarmReplanRefused(t *testing.T) {
	t.Parallel()
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{forge.ProposalPosted})               // #1
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{forge.PlanInProgress})               // #2
	m.forge.CreateIssue("C", mark("owner", 1)+"c", []string{forge.ReviewInProgress})             // #3
	m.forge.CreateIssue("D", mark("owner", 1)+"d", []string{forge.PlanAccepted})                 // #4
	m.forge.CreateIssue("E", mark("owner", 1)+"e", []string{forge.PlanAccepted, forge.Question}) // #5
	m.forge.CreateIssue("F", mark("owner", 1)+"f", []string{forge.PlanPosted})                   // #6
	for _, s := range []int{3, 4} {
		m.forge.Comment(m.ref(s), mark("architect", 1)+twoSlices)
		n, _ := m.forge.CreatePull("app", fmt.Sprintf("swarm/%d-1", s), "main", "slice", fmt.Sprintf("Refs acme/swarm-app#%d", s))
		m.forge.Push(forge.Ref{Repo: "app", N: n}, "c1")
	}
	run := m.state("runs/6-architect-2")
	os.MkdirAll(run, 0o755)
	os.WriteFile(run+"/result", []byte("label swarm:plan-0-posted\nstarted 2026-11-06T15:00:00Z\n"), 0o644)
	for _, tc := range []struct {
		args []string
		err  string
	}{
		{[]string{"1"}, "Error: swarm: #1 is swarm:proposal-posted, with no plan to re-plan: triage it (swarm triage 1), and the architect plans it; nothing changed"},
		{[]string{"2"}, "Error: swarm: #2: a worker runs on it (swarm:plan-2-in-progress); replan once its run is collected; nothing changed"},
		{[]string{"3"}, "Error: swarm: #3: pull request acme/app#1 (https://forge.example.net/acme/app/pulls/1) is open (swarm:review-1-in-progress); close or merge it first, then replan; nothing changed"},
		{[]string{"4", "go"}, "Error: swarm: #4: pull request acme/app#2 (https://forge.example.net/acme/app/pulls/2) is open (swarm:plan-1-accepted); close or merge it first, then replan; nothing changed"},
		{[]string{"5"}, "Error: swarm: #5 waits for the answer to a member's question (swarm:question); replan once it is answered; nothing changed"},
		{[]string{"6"}, "Error: swarm: #6: architect run 2 is not yet collected; replan once it is; nothing changed"},
		{[]string{"4", "Then:\n/swarm accept"}, "Error: swarm: the comment holds a command line (/swarm accept), which would make it a command that no member reads; nothing posted"},
	} {
		if err := m.verb(swarmReplan, tc.args...); errText(err) != tc.err {
			t.Errorf("%q: %v\nwant %s", tc.args, err, tc.err)
		}
	}
	m.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	if err := m.verb(swarmReplan, "6"); errText(err) != "Error: swarm: jane is not in trusted, so no member would read your direction; add the login to trusted in swarm.json (docs/swarm.md section 3); nothing changed" {
		t.Errorf("untrusted: %v", err)
	}
	if got := m.writes(1, 2, 3, 4, 5, 6); len(got) != 0 {
		t.Errorf("writes %q", got)
	}
}
