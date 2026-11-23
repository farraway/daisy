package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/metrics"
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

// TestScorecardEscapesAndFollowUps: the claim of Step R6-C of the plan
// implementing the 2026-11-19 swarm roadmap, the scorecard shows escapes
// and follow-ups (section 11, The scorecard; section 7.11, The balance):
// each slice's line ends `; escapes <e>`, with ` (reverted, <b> filed,
// <h> fixed by hand)`, each part it holds, when it has one, and `;
// follow-up <k> items, <p> proved real`, `follow-up none` with no
// follow-up; the summary ends `; escapes <e> on <m> merged slices (<rv>
// reverted, <b> filed, <h> fixed by hand); follow-up items <f>, <p>
// proved real`, the totals of the window's slices.
func TestScorecardEscapesAndFollowUps(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.setNow("2026-11-06T15:00:00Z")
	for _, title := range []string{"One", "Two", "Three"} {
		a.forge.CreateIssue(title, strings.ToLower(title), []string{forge.ProposalAccepted})
	}
	r := newBalanceRepo(a, "swarm/1-1", "swarm/2-1", "swarm/3-1")
	r.merge("swarm/1-1", "2026-11-06T18:00:00Z")
	m2 := r.merge("swarm/2-1", "2026-11-06T19:00:00Z")
	r.git("2026-11-07T08:00:00Z", "revert", "--no-edit", "-m", "1", m2)
	r.merge("swarm/3-1", "2026-11-07T10:00:00Z")
	r.git("2026-11-07T11:00:00Z", "commit", "-q", "--allow-empty", "-m", "Fix it by hand\n\nDefect in: app#12\n")
	r.mirror()
	for run, rs := range map[string]string{
		"1-worker-1": "branch swarm/1-1 in app\npushed " + r.heads["swarm/1-1"] + "\ncreated pull 11\n",
		"2-worker-1": "branch swarm/2-1 in app\npushed " + r.heads["swarm/2-1"] + "\ncreated pull 12\n",
		"3-worker-1": "branch swarm/3-1 in app\npushed " + r.heads["swarm/3-1"] + "\ncreated pull 13\n",
	} {
		a.runResult(run, rs+"outcome posted\ncollected\n", "")
	}
	a.setNow("2026-11-06T18:05:00Z")
	f, _ := a.forge.CreateIssue("Follow-up: acme/app#11 (#1, slice 1)", followUpBody("11", r.heads["swarm/1-1"], 1, 1,
		"adversary run 2, round 1: [non-blocking] a.go:10: name the constant",
		"sympathetic run 3, round 1: [non-blocking] c.go:1: the doc comment is stale"), []string{forge.FollowUp})
	a.forge.EditIssue(3, fmt.Sprintf("three\n\nFollow-up: #%d 2\n", f))
	a.setNow("2026-11-07T09:00:00Z")
	a.forge.CreateIssue("Reverted", "Defect in: app#12\n", nil)
	a.forge.CreateIssue("Filed", "Defect in: app#13\n", nil)
	a.setNow("2026-11-08T18:00:00Z")

	csv := filepath.Join(home, "metrics", "metrics.csv")
	cost := "runs 0; tokens 0 uncached, 0 cached, 0 out; estimated $0.00 at list price; review rounds 0; stops none; interventions none"
	one := "  #1 slice 1 in app, merged 2026-11-06T18:00:00Z, 3h0m0s after the issue's opening: " + cost + "; escapes 0; follow-up 2 items, 1 proved real\n"
	two := "  #2 slice 1 in app, merged 2026-11-06T19:00:00Z, 4h0m0s after the issue's opening: " + cost + "; escapes 3 (reverted, 1 filed, 1 fixed by hand); follow-up none\n"
	three := "  #3 slice 1 in app, merged 2026-11-07T10:00:00Z, 19h0m0s after the issue's opening: " + cost + "; escapes 1 (1 filed); follow-up none\n"
	head := "Scorecard of acme/swarm-app: runs and interventions from " + csv + ", merges from the mirrors as last fetched, "
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	want := one + two + three +
		"Summary, every record (2.0 days): 3 slices merged; 0 runs, 0.0 per merge; input 0 tokens per merge; worker stop rate — (0 of 0 worker runs); 0 interventions, 0.0 a day; " +
		"escapes 4 on 3 merged slices (1 reverted, 2 filed, 1 fixed by hand); " + followUpTotals(2, 1) + "; blocking findings by kind none\n" +
		"By configuration, every record:\n  no run\n"
	if got := a.stdout.String(); !strings.HasPrefix(got, head) || !strings.HasSuffix(got, want) {
		t.Errorf("stdout:\n%s\nwant after its first line:\n%s", got, want)
	}
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T19:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	want = two + three +
		"Summary, since 2026-11-06T19:00:00Z (2.0 days): 2 slices merged; 0 runs, 0.0 per merge; input 0 tokens per merge; worker stop rate — (0 of 0 worker runs); 0 interventions, 0.0 a day; " +
		"escapes 4 on 2 merged slices (1 reverted, 2 filed, 1 fixed by hand); " + followUpTotals(0, 0) + "; blocking findings by kind none\n" +
		"By configuration, since 2026-11-06T19:00:00Z:\n  no run\n"
	if got := a.stdout.String(); !strings.HasSuffix(got, want) {
		t.Errorf("--since: stdout:\n%s\nwant after its first line:\n%s", got, want)
	}
}

// TestScorecardEscapesByConfiguration: the claim of Step R6-C of the
// plan implementing the 2026-11-19 swarm roadmap, escapes by
// configuration (section 11, The scorecard, By configuration; section
// 7.11, The balance): an adversary's or a sympathetic reviewer's line
// adds `; merged heads approved <a>, escaped <e>; follow-up items raised
// <f>, <p> proved real`: a slice's escapes count against each reviewer
// run whose approval of the merged head stood (its role's newest posted
// review of the slice, approving the head merged: neither an earlier
// approval, one of another head, a run that failed, nor a newest review
// requesting changes on a head merged anyway), and a
// follow-up item against the run that raised it, each by the run's
// config_hash; a run before --since is no line's.
func TestScorecardEscapesByConfiguration(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.setNow("2026-11-06T15:00:00Z")
	for _, title := range []string{"One", "Two", "Three"} {
		a.forge.CreateIssue(title, strings.ToLower(title), []string{forge.ProposalAccepted})
	}
	r := newBalanceRepo(a, "swarm/1-1", "swarm/2-1", "swarm/3-1")
	m1 := r.merge("swarm/1-1", "2026-11-06T18:00:00Z")
	r.merge("swarm/2-1", "2026-11-06T19:00:00Z")
	r.git("2026-11-07T08:00:00Z", "revert", "--no-edit", "-m", "1", m1)
	r.merge("swarm/3-1", "2026-11-07T10:00:00Z")
	r.mirror()
	h1, h2 := r.heads["swarm/1-1"], r.heads["swarm/2-1"]
	for run, rs := range map[string]string{
		"1-worker-1": "branch swarm/1-1 in app\npushed " + h1 + "\ncreated pull 11\n",
		"2-worker-1": "branch swarm/2-1 in app\npushed " + h2 + "\ncreated pull 12\n",
		"3-worker-1": "branch swarm/3-1 in app\npushed " + r.heads["swarm/3-1"] + "\ncreated pull 13\n",
	} {
		a.runResult(run, rs+"outcome posted\ncollected\n", "")
	}
	review := func(run, head, outcome, verdict string) {
		a.runResult(run, "start "+head+"\nround 1\noutcome "+outcome+"\ncollected\n", "VERDICT: "+verdict+"\n\n[non-blocking] a.go:1: a finding\n")
	}
	review("1-adversary-1", h1, "posted", "request-changes")
	review("1-adversary-2", h1, "posted", "approve")
	review("1-sympathetic-3", h1, "posted", "approve")
	review("2-adversary-1", h2, "posted", "approve")
	review("2-sympathetic-2", r.heads["swarm/1-1"], "posted", "approve")
	review("2-adversary-3", h2, "failed", "request-changes")
	review("3-adversary-1", r.heads["swarm/3-1"], "posted", "request-changes")
	a.setNow("2026-11-06T19:05:00Z")
	f, _ := a.forge.CreateIssue("Follow-up: acme/app#12 (#2, slice 1)", followUpBody("12", h2, 1, 2,
		"adversary run 1, round 1: [non-blocking] a.go:10: name the constant",
		"adversary run 1, round 1: [non-blocking] b.go:4: a test for the empty input",
		"sympathetic run 2, round 1: [non-blocking] c.go:1: the doc comment is stale"), []string{forge.FollowUp})
	a.forge.EditIssue(3, fmt.Sprintf("three\n\nFollow-up: #%d 1\n", f))
	a.forge.CreateIssue("Filed", "Defect in: app#11\n", nil)
	ha, hb, hs := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("5", 64)
	row := func(at, run, role, outcome, hash string) string {
		n, _, _ := strings.Cut(run, "-")
		return at + ",swarm-run,acme/swarm-app," + run + "," + role + ",openai,gpt-6.2-astra,high,,,p," + outcome + ",1,100,10,110,,0.01,,,,,,,,,#" + n + ",0,,," + hash + "\n"
	}
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	os.WriteFile(csvFile, []byte(strings.Join(metrics.Header, ",")+"\n"+
		row("2026-11-06T17:00:00Z", "1-adversary-1", "adversary", "posted", ha)+
		row("2026-11-06T17:30:00Z", "1-adversary-2", "adversary", "posted", ha)+
		row("2026-11-06T17:40:00Z", "1-sympathetic-3", "sympathetic", "posted", hs)+
		row("2026-11-06T18:20:00Z", "2-adversary-1", "adversary", "posted", hb)+
		row("2026-11-06T18:30:00Z", "2-sympathetic-2", "sympathetic", "posted", hs)+
		row("2026-11-06T18:40:00Z", "2-adversary-3", "adversary", "failed", hb)+
		row("2026-11-07T09:00:00Z", "3-adversary-1", "adversary", "posted", ha)), 0o644)
	a.setNow("2026-11-08T00:00:00Z")

	lines := func() map[string]string {
		out := map[string]string{}
		_, by, _ := strings.Cut(a.stdout.String(), "\nBy configuration")
		for _, l := range strings.Split(by, "\n")[1:] {
			if k, rest, ok := strings.Cut(l, ": runs "); ok {
				_, tail, _ := strings.Cut(rest, "; review effect ")
				_, tail, _ = strings.Cut(tail, "; ")
				out[strings.TrimSpace(k)] = tail
			}
		}
		return out
	}
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	want := map[string]string{
		"adversary openai gpt-6.2-astra high, config aaaaaaaaaaaa":   "merged heads approved 1, escaped 2; follow-up items raised 0, 0 proved real",
		"adversary openai gpt-6.2-astra high, config bbbbbbbbbbbb":   "merged heads approved 1, escaped 0; follow-up items raised 2, 1 proved real",
		"sympathetic openai gpt-6.2-astra high, config 555555555555": "merged heads approved 1, escaped 2; follow-up items raised 1, 0 proved real",
	}
	if got := lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("by configuration:\n%v\nwant:\n%v\nstdout:\n%s", got, want, a.stdout.String())
	}
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T18:10:00Z"}); err != nil {
		t.Fatal(err)
	}
	want = map[string]string{
		"adversary openai gpt-6.2-astra high, config aaaaaaaaaaaa":   "merged heads approved 0, escaped 0; follow-up items raised 0, 0 proved real",
		"adversary openai gpt-6.2-astra high, config bbbbbbbbbbbb":   "merged heads approved 1, escaped 0; follow-up items raised 2, 1 proved real",
		"sympathetic openai gpt-6.2-astra high, config 555555555555": "merged heads approved 0, escaped 0; follow-up items raised 1, 0 proved real",
	}
	if got := lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("--since: by configuration:\n%v\nwant:\n%v\nstdout:\n%s", got, want, a.stdout.String())
	}
}

// TestScorecardBlockingByKind: the claim of Step R6-C of the plan
// implementing the 2026-11-19 swarm roadmap, blocking findings by kind
// (section 11, The scorecard; section 7.11, The list, in the Task): the
// summary ends `; blocking findings by kind <kind> <k>, …`, the
// `[blocking]` lines of the window's posted reviews (a reviewer run's
// out.md) counted by the word after the marker, case and a trailing
// colon aside, in the blocking sentence's order (claim, test, defect,
// introduced, unclosed), a line naming none of them `unnamed`, and `none`
// for no blocking finding: a non-blocking line, a review not posted, a
// run not a reviewer's and a run before --since count none.
func TestScorecardBlockingByKind(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	for run, out := range map[string]string{
		"1-adversary-1":   "VERDICT: request-changes\n\n[blocking] claim a.go:3: the empty input is refused; reproduced by go test ./...\n[blocking] defect: b.go:4: a percent-encoded path escapes; reproduced by curl\n[blocking] a.go:5: no kind named\n[non-blocking] test c.go:1: a test could be added\n",
		"1-adversary-2":   "VERDICT: request-changes\n\n[blocking] unclosed a.go:3: still refused\n[blocking] introduced d.go:1: the old flag fails\n[blocking] Test e.go:2: TestDrift is missing\n",
		"1-sympathetic-3": "VERDICT: request-changes\n\n[blocking] claim f.go:1: not posted\n",
		"1-worker-1":      "[blocking] claim g.go:1: a worker's line\n",
		"2-adversary-1":   "VERDICT: request-changes\n\n[blocking] claim h.go:1: before --since\n",
	} {
		a.runResult(run, "outcome posted\ncollected\n", out)
	}
	row := func(at, run, role, outcome string) string {
		n, _, _ := strings.Cut(run, "-")
		return at + ",swarm-run,acme/swarm-app," + run + "," + role + ",openai,gpt-6.2-astra,high,,,p," + outcome + ",1,100,10,110,,0.01,,,,,,,,,#" + n + ",0,,,\n"
	}
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	os.WriteFile(csvFile, []byte(strings.Join(metrics.Header, ",")+"\n"+
		row("2026-11-06T08:00:00Z", "2-adversary-1", "adversary", "posted")+
		row("2026-11-06T09:30:00Z", "1-worker-1", "worker", "posted")+
		row("2026-11-06T10:00:00Z", "1-adversary-1", "adversary", "posted")+
		row("2026-11-06T11:00:00Z", "1-adversary-2", "adversary", "posted")+
		row("2026-11-06T11:30:00Z", "1-sympathetic-3", "sympathetic", "failed")), 0o644)
	a.setNow("2026-11-07T00:00:00Z")
	for _, c := range []struct{ since, want string }{
		{"", "; blocking findings by kind claim 2, test 1, defect 1, introduced 1, unclosed 1, unnamed 1\n"},
		{"2026-11-06T09:00:00Z", "; blocking findings by kind claim 1, test 1, defect 1, introduced 1, unclosed 1, unnamed 1\n"},
		{"2026-11-06T12:00:00Z", "; blocking findings by kind none\n"},
	} {
		var values []Value
		if c.since != "" {
			values = append(values, Value{Flag: "--since", Value: c.since})
		}
		if err := a.scorecard(values...); err != nil {
			t.Fatalf("%v\n%s", err, a.stderr.String())
		}
		_, summary, _ := strings.Cut(a.stdout.String(), "\nSummary, ")
		summary, _, _ = strings.Cut(summary, "\nBy configuration")
		if !strings.HasSuffix(summary+"\n", c.want) {
			t.Errorf("since %q: summary %q, want it ending %q", c.since, summary, c.want)
		}
	}
}

// followUpTotals is the summary's `follow-up items <f>, <p> proved
// real`, written from its numbers: the words before them, as literal
// text, would read as a citation of decisions <f> and <p>
// (TestCodeCitesExistingSpecSections).
func followUpTotals(f, p int) string { return fmt.Sprintf("follow-up items %d, %d proved real", f, p) }
