package commands

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// relabelOnList stands in for the maintainer relabelling issue id to the
// label to while a pass runs (section 5: "at any time"): once, when the
// pass lists the open issues of on, before the list is read or after.
type relabelOnList struct {
	*forge.Fake
	on, to string
	id     forge.Ref
	after  bool
	done   bool
}

func (r *relabelOnList) Issues(st forge.State, label string) ([]forge.Issue, error) {
	fire := !r.done && st == forge.Open && label == r.on
	if fire && !r.after {
		r.done = true
		r.Fake.SetLabel(r.id, r.to)
	}
	out, err := r.Fake.Issues(st, label)
	if fire && r.after {
		r.done = true
		r.Fake.SetLabel(r.id, r.to)
	}
	return out, err
}

// The pass acts on an issue as Issue(n) reads it, not as a list read
// earlier did (section 6, rule 1: n carries exactly one swarm:* label and
// R is its role): an issue moved from ready to planned after its stage was
// listed launches no architect.
func TestSwarmPassActsOnTheIssueAsItIs(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.tick()
	f := &relabelOnList{Fake: a.forge, on: "swarm:proposal-accepted", to: "swarm:plan-1-accepted", id: issue1, after: true}
	if err := a.apply(f, "--once"); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); strings.Contains(out, "#1 architect run 1: launched") {
		t.Errorf("the architect launched on #1 at %v:\n%s", labels(a.issue(1)), out)
	}
}

// Rule 2 counts this pass's own launches: an issue the pass launched a
// member on, then relabelled by the maintainer to a later stage before
// that stage is listed, gets no second member in the same pass.
func TestSwarmPassLaunchesOnceAnIssue(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.tick()
	f := &relabelOnList{Fake: a.forge, on: "swarm:plan-0-posted", to: "swarm:plan-1-accepted", id: issue1}
	if err := a.apply(f, "--once"); err != nil {
		t.Fatal(err)
	}
	if n := len(a.runner.Launches()); n != 1 {
		t.Errorf("%d members launched on #1 in one pass:\n%s", n, a.stdout.String())
	}
}

// The merge step merges only an open issue whose one swarm:* label is
// approved: one the maintainer also labelled blocked is skipped, as step
// (5) says, and neither merged nor closed.
func TestSwarmMergeStepSkipsTwoLabels(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	if got := labels(a.issue(1)); len(got) != 1 || got[0] != "swarm:review-2-approved" {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	a.forge.As(forge.Account{Login: "jane"}).Relabel(issue1, "", "swarm:blocked")
	a.pass()
	if is, ps := a.issue(1), a.pulls(1); ps[0].Merged || !is.Open {
		t.Errorf("merged %v, open %v, labels %v\n%s", ps[0].Merged, is.Open, is.Labels, a.stdout.String())
	}
}

// failIssue fails the forge's Issue(n) at its call'th call, once.
type failIssue struct {
	gitForge
	call, calls int
}

func (f *failIssue) Issue(n int) (forge.Issue, error) {
	f.calls++
	if f.calls == f.call {
		return forge.Issue{}, errors.New("forge: 502 bad gateway")
	}
	return f.gitForge.Issue(n)
}

// max_members counts every container of the swarm running at once, a
// worker run's check included (section 6, rule 4): with max_members 1, a
// check started in step (3), collecting a run left uncollected, leaves no
// room for a launch in step (5) of the same pass.
func TestSwarmPassCountsAStartedCheck(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.tick()
	// The third Issue read is the collection's: the first two are step
	// (5)'s, at plan-1-accepted and at the plan-2-in-progress the launch
	// wrote.
	if err := a.apply(&failIssue{gitForge: a.git(), call: 3}, "--once"); err == nil {
		t.Fatalf("the worker's collection did not fail:\n%s", a.stdout.String())
	}
	a.forge.CreateIssue("Second", "Another job.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if out := a.stdout.String(); !strings.Contains(out, "#1 worker run 1: check started") || strings.Contains(out, "#2 architect run 1: launched") {
		t.Errorf("a check and a member at once under max_members 1:\n%s", out)
	}
}

// out.md is the member's: a link it leaves there is never followed (section
// 7.5), so it cannot make apply read a host file and post it; the run has no
// output.
func TestSwarmOutputLinkIsNotFollowed(t *testing.T) {
	a := newApplyWS(t)
	host := filepath.Join(a.ws, "host-secret.txt")
	if err := os.WriteFile(host, []byte("HOST-SECRET-0123456789\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Work: func(work string) error {
		return os.Symlink(host, filepath.Join(filepath.Dir(work), "io", "out.md"))
	}})
	a.pass()
	if body := strings.Join(a.bodies(1), "\n"); strings.Contains(body, "HOST-SECRET") || !strings.Contains(body, "failed: no output") {
		t.Errorf("the issue's thread:\n%s", body)
	}
}

// apply reports a problem of <state>/env once, at its start, however
// often the pass then reads a secret (section 11, Stderr: as init).
func TestSwarmApplyWarnsOnceAboutTheSecretsFile(t *testing.T) {
	a := newApplyWS(t)
	a.secrets("export GITEA_TOKEN=\"tok-123\"\nsource ./other.sh\n")
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if n := strings.Count(a.stderr.String(), a.secretsFile()+" line 2"); n != 1 {
		t.Errorf("warned %d times:\n%s", n, a.stderr.String())
	}
}

// A paused pass merges nothing (section 11, pause): with the merge gate
// off, an approved issue stays approved and its pull request open until
// resume.
func TestSwarmPausedPassMergesNothing(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9, approved
	if err := a.verb(SwarmPause); err != nil {
		t.Fatal(err)
	}
	a.pass()
	if ps := a.pulls(1); ps[0].Merged || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-2-approved"}) {
		t.Errorf("a paused pass merged: %+v %v\n%s", ps[0], labels(a.issue(1)), a.stdout.String())
	}
	a.resume()
	a.pass()
	if ps := a.pulls(1); !ps[0].Merged {
		t.Errorf("not merged after resume:\n%s", a.stdout.String())
	}
}

// A worker that writes BLOCKED: in swarm:review-3-changes-requested posts it on the pull
// request, whose thread the worker reads there; unblock comments there,
// so the worker runs again (section 6, rule 3), not on the issue, where
// it would see its own BLOCKED: as the newest entry and run no more.
func TestSwarmUnblockReviewedRunsTheWorker(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T10, reviewed
	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: should main.go say v2 or v3?\n"})
	a.pass() // T17: blocked
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested", forge.Blocked, forge.Gated}) {
		t.Fatalf("not blocked: %v\n%s", got, a.stdout.String())
	}
	m := &maintainerWS{statusWS: a.statusWS}
	m.secrets(bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
	if err := m.verb(swarmUnblock, "1", "v2, as the plan says."); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.stdout.String(), "#1 commented on pull app#1\n#1 blocked cleared\n#1 gated cleared\n") {
		t.Errorf("unblock:\n%s", m.stdout.String())
	}
	before := len(a.runner.Launches())
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if len(a.runner.Launches()) == before {
		t.Errorf("no worker after unblock to swarm:review-3-changes-requested:\n%s", a.stdout.String())
	}
}

// A check another apply started, outlived its timeout: stopped, recorded
// and said on stdout as any collection step is (section 11, Stdout).
func TestSwarmOrphanCheckTimeoutIsSaid(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:plan-1-accepted"})
	run := "1-worker-1"
	os.MkdirAll(a.state("runs/"+run+"/work"), 0o755)
	os.WriteFile(a.state("runs/"+run+"/result"), []byte("start aaaa\nbranch swarm/1-1 in app\nlabel swarm:plan-1-accepted\nstarted 2026-11-06T13:00:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\nexited 0 2026-11-06T13:10:00Z\nfetched bbbb\ncheck started 2026-11-06T13:11:00Z\n"), 0o644)
	a.runner.Others = []runner.Running{{Name: "daisy-swarm.acme.swarm-app.1.check.1", Swarm: "acme/swarm-app#1/check"}}
	a.tick()
	a.apply(a.git(), "--once")
	if !strings.Contains(a.result(run), "check timeout ") || !strings.Contains(a.stdout.String(), "#1 worker run 1: check timeout\n") {
		t.Errorf("result:\n%s\nstdout:\n%s", a.result(run), a.stdout.String())
	}
}

// relabelOnComment is the maintainer relabelling issue 1 to `to` while the
// collection posts: after its read of the issue, before its relabel.
type relabelOnComment struct {
	*forge.Fake
	to   string
	done bool
}

func (r *relabelOnComment) Comment(at forge.Ref, body string) error {
	err := r.Fake.Comment(at, body)
	if !r.done && at == issue1 {
		r.done = true
		r.Fake.SetLabel(issue1, r.to)
	}
	return err
}

// A collection reads the labels right before it relabels: the maintainer's
// move while the run was posted is kept, with the warning, never joined by
// a second swarm:* label (section 5).
func TestSwarmCollectionRelabelReadsTheLabelsAgain(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.tick()
	if err := a.apply(&relabelOnComment{Fake: a.forge, to: "swarm:plan-1-accepted"}, "--once"); err != nil {
		t.Fatal(err)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) || !strings.Contains(a.stderr.String(), "relabelled to swarm:plan-1-accepted during the run") {
		t.Errorf("labels %v\nstderr:\n%s", got, a.stderr.String())
	}
}

// relabelOnMerge is the maintainer relabelling issue 1 while the merge
// step merges.
type relabelOnMerge struct {
	gitForge
	done bool
}

func (r *relabelOnMerge) Merge(pr forge.Ref, head string) error {
	if !r.done {
		r.done = true
		r.Fake.SetLabel(issue1, "swarm:proposal-accepted")
	}
	return r.gitForge.Merge(pr, head)
}

// The merge step reads the labels again right before its relabel: the
// maintainer's move during the merge is kept, with the warning.
func TestSwarmMergeStepRelabelReadsTheLabelsAgain(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	a.forge.RefuseMerge(forge.Ref{Repo: "app", N: 1})
	a.tick()
	a.apply(&relabelOnMerge{gitForge: a.git()}, "--once")
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted"}) || !strings.Contains(a.stderr.String(), "relabelled to swarm:proposal-accepted during the merge step") {
		t.Errorf("labels %v\nstdout:\n%s\nstderr:\n%s", got, a.stdout.String(), a.stderr.String())
	}
}

// An issue's roles are decided once a pass (section 6): with the
// adversary's launch refused (its slot has no login) and the sympathetic
// reviewer's moving the issue to review-1-in-progress, which the pass
// reads later, the adversary is not launched again in that pass.
func TestSwarmPassDecidesAnIssueOnce(t *testing.T) {
	a := newApplyWS(t)
	os.Remove(filepath.Join(a.local, "swarm", "credentials", "openai", "1", "auth.json"))
	a.inReview(oneSlice)
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if r := a.result("1-adversary-1"); strings.Count(r, "launch-failed") != 1 || !strings.HasSuffix(r, "outcome fault\ncollected\n") {
		t.Errorf("the adversary launched again in the pass:\n%s", r)
	}
	if _, err := os.Stat(a.state("runs/1-adversary-2")); err == nil {
		t.Error("adversary run 2 in the same pass")
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress"}) || !strings.Contains(a.result("1-sympathetic-1"), "\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n") {
		t.Errorf("labels %v\n%s", got, a.result("1-sympathetic-1"))
	}
}
