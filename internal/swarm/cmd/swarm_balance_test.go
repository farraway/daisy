package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
)

// The balance (section 7.11, rule 7; Step R6-C of the plan implementing
// the 2026-11-19 swarm roadmap): the scorecard reads each merged slice's
// escapes and its follow-up items beside its cost.

// readScorecard is the scorecard `swarm metrics --scorecard` reads with
// the values given, over the scratch state and the Fake forge.
func (a *applyWS) readScorecard(values ...Value) reconcile.Scorecard {
	a.t.Helper()
	o := &ops.Fake{NowFn: a.Now, ExecFn: a.exec, GetenvFn: func(k string) string { return a.vars[k] }}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Flags: []string{"--scorecard"}, Values: values}
	var sc reconcile.Scorecard
	err := a.at(c, false, func(c *Context) error {
		var err error
		_, sc, err = readScorecard(c, func(ops.Ops, declare.Forge, []string, string) (forge.Adapter, error) { return a.forge, nil })
		return err
	})
	if err != nil {
		a.t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	return sc
}

// balanceRepo is app's history for the balance's tests: a work tree
// whose main merges each branch given (`swarm/<n>-<s>`, a commit of its
// own on main) with a merge commit at its time, as the forge merges; its
// heads by branch.
type balanceRepo struct {
	a     *applyWS
	work  string
	heads map[string]string
}

func newBalanceRepo(a *applyWS, branches ...string) *balanceRepo {
	t := a.t
	r := &balanceRepo{a: a, work: t.TempDir(), heads: map[string]string{}}
	gitIn(t, r.work, "init", "-q", "-b", "main")
	gitIn(t, r.work, "commit", "-q", "--allow-empty", "-m", "first")
	for _, b := range branches {
		gitIn(t, r.work, "checkout", "-q", "-b", b, "main")
		os.WriteFile(filepath.Join(r.work, strings.ReplaceAll(b, "/", "-")), []byte(b), 0o644)
		gitIn(t, r.work, "add", "-A")
		gitIn(t, r.work, "commit", "-q", "-m", b)
		r.heads[b] = gitIn(t, r.work, "rev-parse", "HEAD")
		gitIn(t, r.work, "checkout", "-q", "main")
	}
	return r
}

// git runs git in the work tree with its commits dated at (RFC 3339):
// the new HEAD.
func (r *balanceRepo) git(at string, args ...string) string {
	r.a.t.Helper()
	cmd := osexec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.net"}, args...)...)
	cmd.Dir, cmd.Env = r.work, append(os.Environ(), "GIT_COMMITTER_DATE="+at, "GIT_AUTHOR_DATE="+at)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.a.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return gitIn(r.a.t, r.work, "rev-parse", "HEAD")
}

// merge merges branch into main at at, its message `Merge <branch>` and
// the lines given: the merge commit.
func (r *balanceRepo) merge(branch, at string, lines ...string) string {
	return r.git(at, "merge", "-q", "--no-ff", "-m", strings.Join(append([]string{"Merge " + branch}, lines...), "\n\n"), branch)
}

// mirror fetches the work tree into app's mirror.
func (r *balanceRepo) mirror() {
	gitIn(r.a.t, r.a.state("git/app"), "fetch", "-q", r.work, "+refs/heads/*:refs/heads/*")
}

// TestEscapes: the claim of Step R6-C of the plan implementing the
// 2026-11-19 swarm roadmap, escapes (section 7.11, The balance): a
// merged swarm pull request, `<name>#<m>` as its worker run's `created
// pull <m>` names it, has escaped once for its merge reverted, once for
// each issue of forge.repo, open or closed, a line of whose body is
// `Defect in: <name>#<m>` (an issue naming it twice counts once; another
// repository's pull request of that number, or the line inside a
// sentence, none), and once for each commit on base's first-parent line
// that is no swarm merge and whose message holds that line (a swarm
// merge's, or one on another branch, none); a slice whose pull request no run names has none
// but its revert. With --until, an escape made after it is not counted:
// an issue opened, a hand commit or a revert.
func TestEscapes(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.metricsHome()
	a.setNow("2026-11-06T15:00:00Z")
	for _, title := range []string{"One", "Two", "Three", "Four"} {
		a.forge.CreateIssue(title, strings.ToLower(title), []string{forge.ProposalAccepted})
	}
	r := newBalanceRepo(a, "swarm/1-1", "swarm/2-1", "swarm/3-1", "swarm/4-1")
	r.merge("swarm/1-1", "2026-11-06T18:00:00Z")
	m2 := r.merge("swarm/2-1", "2026-11-06T19:00:00Z")
	r.merge("swarm/3-1", "2026-11-07T10:00:00Z", "Defect in: app#11")
	r.merge("swarm/4-1", "2026-11-07T11:00:00Z")
	r.git("2026-11-08T09:00:00Z", "revert", "--no-edit", "-m", "1", m2)
	os.WriteFile(filepath.Join(r.work, "fix"), []byte("fix\n"), 0o644)
	r.git("2026-11-08T10:00:00Z", "add", "-A")
	r.git("2026-11-08T10:00:00Z", "commit", "-q", "-m", "Fix the empty title\n\nDefect in: app#13\n")
	r.git("2026-11-08T11:00:00Z", "checkout", "-q", "-b", "elsewhere")
	r.git("2026-11-08T11:00:00Z", "commit", "-q", "--allow-empty", "-m", "Not on base\n\nDefect in: app#11\n")
	r.git("2026-11-08T11:00:00Z", "checkout", "-q", "main")
	r.git("2026-11-09T12:00:00Z", "commit", "-q", "--allow-empty", "-m", "Fix it by hand\n\nDefect in: app#11\n")
	r.mirror()
	for run, rs := range map[string]string{
		"1-worker-1": "branch swarm/1-1 in app\ncreated pull 11\n",
		"1-worker-2": "branch swarm/1-1 in app\npushed " + r.heads["swarm/1-1"] + "\n",
		"2-worker-1": "branch swarm/2-1 in app\npushed " + r.heads["swarm/2-1"] + "\ncreated pull 12\n",
		"3-worker-1": "branch swarm/3-1 in app\npushed " + r.heads["swarm/3-1"] + "\ncreated pull 13\n",
		"4-worker-1": "branch swarm/4-1 in app\npushed " + r.heads["swarm/4-1"] + "\n",
	} {
		a.runResult(run, rs+"outcome posted\ncollected\n", "")
	}
	a.setNow("2026-11-08T08:00:00Z")
	a.forge.CreateIssue("Empty title", "**owner:** proposal\n\nDefect in: app#11\n", nil)
	a.setNow("2026-11-08T09:30:00Z")
	closed, _ := a.forge.CreateIssue("Again", " Defect in: app#11\nDefect in: app#11\r\nDefect in: lib#12\nSee Defect in: app#13\n", nil)
	a.forge.CloseIssue(closed)
	a.setNow("2026-11-09T12:00:00Z")
	a.forge.CreateIssue("Reverted", "Defect in: app#12\n", nil)
	a.setNow("2026-11-10T00:00:00Z")

	escapes := func(sc reconcile.Scorecard) map[string]reconcile.Escapes {
		out := map[string]reconcile.Escapes{}
		for _, s := range sc.Slices {
			out[fmt.Sprintf("%s#%d of #%d", s.Repo, s.Pull, s.N)] = s.Escapes
		}
		return out
	}
	for _, c := range []struct {
		until string
		want  map[string]reconcile.Escapes
	}{
		{"", map[string]reconcile.Escapes{
			"app#11 of #1": {Filed: 2, ByHand: 1},
			"app#12 of #2": {Reverted: true, Filed: 1},
			"app#13 of #3": {ByHand: 1},
			"app#0 of #4":  {},
		}},
		{"2026-11-09T00:00:00Z", map[string]reconcile.Escapes{
			"app#11 of #1": {Filed: 2},
			"app#12 of #2": {Reverted: true},
			"app#13 of #3": {ByHand: 1},
			"app#0 of #4":  {},
		}},
		{"2026-11-08T08:30:00Z", map[string]reconcile.Escapes{
			"app#11 of #1": {Filed: 1},
			"app#12 of #2": {},
			"app#13 of #3": {},
			"app#0 of #4":  {},
		}},
	} {
		var values []Value
		if c.until != "" {
			values = append(values, Value{Flag: "--until", Value: c.until})
		}
		got := escapes(a.readScorecard(values...))
		if len(got) != len(c.want) {
			t.Errorf("until %q: slices %v, want %v", c.until, got, c.want)
		}
		for k, w := range c.want {
			if got[k] != w {
				t.Errorf("until %q: %s escapes %+v, want %+v", c.until, k, got[k], w)
			}
		}
	}
	// The count: a revert, each issue filed and each hand fix.
	if n := (reconcile.Escapes{Reverted: true, Filed: 2, ByHand: 1}).Count(); n != 4 {
		t.Errorf("Count = %d, want 4", n)
	}
}

// TestFollowUpsProvedReal: the claim of Step R6-C of the plan
// implementing the 2026-11-19 swarm roadmap, follow-up items proved real
// (section 7.11, The balance): a merged slice's follow-up is the issue
// carrying swarm:follow-up, open or closed, that the reconciler filed for
// its pull request (one without the flag is none), its items each raised by the run it names (`<n>-<role>-<k>`);
// an item is proved real when a `Follow-up: #<f> <i>` line names it in
// the body of an issue with a merged slice: one whose issue is not merged,
// a number no item has, or an issue no follow-up is, proves nothing. A
// slice with no follow-up has none. With --until, a merge after it
// proves nothing.
func TestFollowUpsProvedReal(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.metricsHome()
	a.setNow("2026-11-06T15:00:00Z")
	a.forge.CreateIssue("One", "one", []string{forge.ProposalAccepted})
	r := newBalanceRepo(a, "swarm/1-1", "swarm/1-2", "swarm/4-1", "swarm/5-1")
	r.merge("swarm/1-1", "2026-11-06T18:00:00Z")
	a.setNow("2026-11-06T18:05:00Z")
	body := followUpBody("11", r.heads["swarm/1-1"], 1, 1,
		"adversary run 4, round 1: [non-blocking] a.go:10: name the constant",
		"adversary run 5, round 2: [non-blocking] b.go:4: a test for the empty input",
		"sympathetic run 6, round 2: [non-blocking] c.go:1: the doc comment is stale")
	a.forge.CreateIssue("Not a follow-up: no flag", body, nil)
	f, _ := a.forge.CreateIssue("Follow-up: acme/app#11 (#1, slice 1)", body, []string{forge.FollowUp})
	a.forge.CloseIssue(f)
	a.setNow("2026-11-07T09:00:00Z")
	a.forge.CreateIssue("Name the constant", fmt.Sprintf("**owner:** proposal\n\nClass: routine\nFollow-up: #%d 1, 3, 9\nFollow-up: #1 1\n", f), []string{forge.ProposalAccepted})
	a.forge.CreateIssue("A test for the empty input", fmt.Sprintf("Follow-up: #%d 2\n", f), []string{forge.ProposalAccepted})
	r.merge("swarm/1-2", "2026-11-07T10:00:00Z")
	r.merge("swarm/4-1", "2026-11-07T12:00:00Z")
	a.setNow("2026-11-07T12:05:00Z")
	a.forge.CreateIssue("Follow-up: acme/app#13 (#4, slice 1)", followUpBody("13", r.heads["swarm/4-1"], 1, 4, "adversary run 2, round 1: [non-blocking] d.go:2: a stale name"), []string{forge.FollowUp})
	r.mirror()
	for run, rs := range map[string]string{
		"1-worker-1": "branch swarm/1-1 in app\npushed " + r.heads["swarm/1-1"] + "\ncreated pull 11\n",
		"1-worker-2": "branch swarm/1-2 in app\npushed " + r.heads["swarm/1-2"] + "\ncreated pull 14\n",
		"4-worker-1": "branch swarm/4-1 in app\npushed " + r.heads["swarm/4-1"] + "\ncreated pull 13\n",
		"5-worker-1": "branch swarm/5-1 in app\npushed " + r.heads["swarm/5-1"] + "\ncreated pull 15\n",
	} {
		a.runResult(run, rs+"outcome posted\ncollected\n", "")
	}
	a.setNow("2026-11-08T00:00:00Z")

	type slice struct {
		followUp int
		items    []reconcile.FollowUpItem
		real     int
	}
	got := func(sc reconcile.Scorecard) map[string]slice {
		out := map[string]slice{}
		for _, s := range sc.Slices {
			out[fmt.Sprintf("app#%d", s.Pull)] = slice{s.FollowUp, s.Items, s.ProvedReal()}
		}
		return out
	}
	want := map[string]slice{
		"app#11": {f, []reconcile.FollowUpItem{{Run: "1-adversary-4", Real: true}, {Run: "1-adversary-5"}, {Run: "1-sympathetic-6", Real: true}}, 2},
		"app#14": {},
		"app#13": {f + 3, []reconcile.FollowUpItem{{Run: "4-adversary-2"}}, 0},
	}
	if g := got(a.readScorecard()); !reflect.DeepEqual(g, want) {
		t.Errorf("follow-ups:\n%+v\nwant:\n%+v", g, want)
	}
	// Until #4's merge: its proposal has no merged slice yet.
	want = map[string]slice{
		"app#11": {f, []reconcile.FollowUpItem{{Run: "1-adversary-4"}, {Run: "1-adversary-5"}, {Run: "1-sympathetic-6"}}, 0},
		"app#14": {},
	}
	if g := got(a.readScorecard(Value{Flag: "--until", Value: "2026-11-07T11:00:00Z"})); !reflect.DeepEqual(g, want) {
		t.Errorf("until 2026-11-07T11:00:00Z, follow-ups:\n%+v\nwant:\n%+v", g, want)
	}
}
