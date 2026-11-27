package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// checkedGreen records base's commit in the upstream of the declared
// repository name green, as a check that passed on its tree would (section
// 12.1, rule 4): a collected main check, `0-main-<k>`, passed on it. The
// workspace starts so, and land, a slice merged, keeps it so.
func (a *applyWS) checkedGreen(name string) {
	a.t.Helper()
	sha := gitIn(a.t, a.upstream(name), "rev-parse", "main")
	tree := gitIn(a.t, a.upstream(name), "rev-parse", "main^{tree}")
	k := 1
	for ; ; k++ {
		if _, err := os.Stat(a.state(filepath.Join("runs", fmt.Sprintf("0-main-%d", k)))); err != nil {
			break
		}
	}
	dir := a.state(filepath.Join("runs", fmt.Sprintf("0-main-%d", k)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.t.Fatal(err)
	}
	result := fmt.Sprintf("main %s %s tree %s\nbranch main in %s\nfetched %s\npassed\ncollected\n", name, sha, tree, name, sha)
	if err := os.WriteFile(filepath.Join(dir, "result"), []byte(result), 0o644); err != nil {
		a.t.Fatal(err)
	}
}

// push commits files to base (main) in the upstream of the declared
// repository name by hand, as no swarm merge moves it: no check has
// passed on its tree. It returns the commit.
func (a *applyWS) push(name string, files map[string]string, msg string) string {
	a.t.Helper()
	a.landUnchecked(name, files, msg)
	return a.mainAt()
}

// mains is the main checks launched, by their labels.
func (a *applyWS) mains() []string {
	var out []string
	for _, l := range a.runner.Launches() {
		if strings.HasSuffix(l.Label, "#0/main") {
			out = append(out, l.Name)
		}
	}
	return out
}

// launches is how many launches carry the label.
func (a *applyWS) launches(label string) int {
	n := 0
	for _, l := range a.runner.Launches() {
		if l.Label == label {
			n++
		}
	}
	return n
}

// forgeMerges merges head into base (main) in the upstream of the
// declared repository name as the forge merges a pull request, with a
// merge commit (git merge --no-ff), and returns the commit.
func (a *applyWS) forgeMerges(name, head string) string {
	a.t.Helper()
	src := a.t.TempDir()
	gitIn(a.t, "", "clone", "-q", a.upstream(name), src)
	gitIn(a.t, src, "fetch", "-q", "origin", head)
	gitIn(a.t, src, "merge", "-q", "--no-ff", "--no-edit", head)
	gitIn(a.t, src, "push", "-q", "origin", "main")
	return a.mainAt()
}

// broken is a main check's failing run: the check exits 2 with a
// compiler's error, which section 7.9 does not classify.
var broken = runner.Canned{Code: 2, Stdout: "other.go:1: syntax error: unexpected newline\n"}

// A merge the forge makes of a head whose merge check passed is green
// with no run (section 12.1, rule 4): its tree is the merge check's, so
// no main check runs on it.
func TestSwarmMainGreenByTheMergeCheck(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	a.pass() // the merge check, passed
	a.pass() // the merge step merges
	if !a.pulls(1)[0].Merged {
		t.Fatalf("not merged:\n%s", a.stdout.String())
	}
	merged := a.forgeMerges("app", head)
	tree := gitIn(t, a.upstream("app"), "rev-parse", merged+"^{tree}")
	if !strings.Contains(a.result("1-merge-1"), "\ntree "+tree+"\n") {
		t.Fatalf("the forge's merge is not the merge check's tree %s:\n%s", tree, a.result("1-merge-1"))
	}
	a.pass()
	a.pass()
	if ms := a.mains(); len(ms) != 0 || strings.Contains(a.stdout.String(), "main check") {
		t.Errorf("a main check ran on a tree the merge check passed: %q\n%s", ms, a.stdout.String())
	}
}

// A commit of base whose tree no check has passed, a push by hand, is
// checked (section 12.1, rule 4): the main check 0-main-<k> runs the
// repository's check on that commit, its container named and labelled
// for issue 0, its result naming the repository, the commit and its
// tree; once it passed, the commit is green and not checked again.
func TestSwarmMainCheckedAfterAHandPush(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	sha := a.push("app", map[string]string{"hand.go": "hand\n"}, "a push by hand")
	tree := gitIn(t, a.upstream("app"), "rev-parse", sha+"^{tree}")
	var checked string
	a.runner.Script(0, "main", runner.Canned{Work: func(work string) error {
		out, _, err := ops.Real().Exec(work, "git", "rev-parse", "HEAD")
		checked = strings.TrimSpace(string(out))
		return err
	}})
	a.pass()
	l := a.runner.Launches()
	if ms := a.mains(); len(ms) != 1 || ms[0] != "daisy-swarm.acme.swarm-app.0.main.2" || l[len(l)-1].Workdir != "/job/work/app" ||
		!strings.Contains(strings.Join(l[len(l)-1].Argv, " "), "make check") || checked != sha {
		t.Fatalf("the main check: %q, checked %q (want %s)\n%s", ms, checked, sha, a.stdout.String())
	}
	res := a.result("0-main-2")
	if !strings.HasPrefix(res, "main app "+sha+" tree "+tree+"\n") || !strings.HasSuffix(res, "\npassed\ncollected\n") ||
		!strings.Contains(a.stdout.String(), "main check of app: main at "+sha[:7]+"\n") {
		t.Errorf("the run's result:\n%s\n%s", res, a.stdout.String())
	}
	a.pass()
	if ms := a.mains(); len(ms) != 1 {
		t.Errorf("a green commit checked again: %q", ms)
	}
}

// While base is red, no worker launches on its repository and its queue
// checks no head, but the fix's (section 12.1, rule 4): a push by hand
// fails its main check; the approved head of #1 is not merge-checked,
// #2's worker is not launched on plan-1-accepted, and its architect, no
// worker, runs as before.
func TestSwarmRedMainHoldsWorkers(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedHead(false)
	red := a.push("app", map[string]string{"other.go": "broken\n"}, "a push by hand")
	a.runner.Script(0, "main", broken)
	a.pass() // the main check fails
	if res := a.result("0-main-2"); !strings.HasPrefix(res, "main app "+red+" ") || !strings.Contains(res, "\nfailed: main check: exit 2\n") ||
		!strings.Contains(a.stdout.String(), "main check of app failed (exit 2): main red at "+red[:7]+"\n") {
		t.Fatalf("the main check:\n%s\n%s", res, a.stdout.String())
	}
	a.forge.CreateIssue("Second", "Check the second.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
	a.pass() // #2's architect runs
	a.forge.SetLabel(a.ref(2), "swarm:plan-1-accepted")
	a.runner.Script(2, "worker", work("v2"))
	a.pass()
	if a.launches("acme/swarm-app#2/architect") != 1 || a.launches("acme/swarm-app#2/worker") != 0 || len(a.merges()) != 0 {
		t.Errorf("launched on a red main: architect %d, worker %d, merge checks %d\n%s", a.launches("acme/swarm-app#2/architect"),
			a.launches("acme/swarm-app#2/worker"), len(a.merges()), a.stdout.String())
	}
	if got := labels(a.issue(2)); !slices.Contains(got, forge.PlanAccepted) {
		t.Errorf("#2 moved: %v", got)
	}
}

// When a commit of base is green again, the hold lifts at that pass
// (section 12.1, The red main): a push by hand fails its main check, a
// second passes on the red base; the journal names both changes, red
// then green, and the queue checks the approved head again.
func TestSwarmMainGreenAgain(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedHead(false)
	red := a.push("app", map[string]string{"other.go": "broken\n"}, "a push by hand")
	a.runner.Script(0, "main", broken)
	a.pass() // red
	green := a.push("app", map[string]string{"other.go": "fixed\n"}, "the fix, by hand")
	a.pass() // the main check on the red base passes
	if res := a.result("0-main-3"); !strings.HasPrefix(res, "main app "+green+" ") || !strings.Contains(res, "\nred\n") || !strings.HasSuffix(res, "\npassed\ncollected\n") || len(a.merges()) != 0 {
		t.Fatalf("the second main check:\n%s\n%s", res, a.stdout.String())
	}
	a.pass()
	if ms := a.merges(); len(ms) != 1 || !strings.HasPrefix(a.result("1-merge-1"), "merge app "+a.pulls(1)[0].HeadSHA+" into "+green+" ") {
		t.Errorf("the queue not checking again: %d\n%s", len(ms), a.stdout.String())
	}
	journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md") + "\n"
	if !strings.Contains(journal, " 0-main-2: app red at "+red[:7]+"\n") || !strings.Contains(journal, " 0-main-3: app green at "+green[:7]+"\n") {
		t.Errorf("journal:\n%s", journal)
	}
}

// The fix is given to the worker of the job whose merge broke base when
// that job is open (section 12.1, The red main): #1's first slice merges
// onto a commit pushed by hand, a tree no check passed; its main check
// fails, its log goes to #1's workspace, the reconciler's note to #1's
// issue, and #1's next worker run, not held, is told to make its
// branch's first commit the fix.
func TestSwarmRedMainRoutedToTheBreakingJob(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(twoSlices)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary approves
	a.pass() // T9
	a.pass() // the merge check
	a.pass() // merged: T14
	head := a.pulls(1)[0].HeadSHA
	if !a.pulls(1)[0].Merged {
		t.Fatalf("not merged:\n%s", a.stdout.String())
	}
	a.forge.Relabel(issue1, "", forge.Paused) // slice 2 waits while the test moves main
	a.landUnchecked("app", map[string]string{"other.go": "other\n"}, "a push by hand")
	red := a.forgeMerges("app", head)
	a.runner.Script(0, "main", broken)
	a.pass()
	note := mark("reconciler", 2) + "main of app is red at " + red[:7] + ", after the merge of [acme/app#1](https://forge.example.net/acme/app/pulls/1) (slice 1): `make check` fails; its output is /job/memory/checks/" + red +
		".log. This job's next worker run fixes it first.\n\ncc @jane"
	if bodies := a.bodies(1); !slices.Contains(bodies, note) || !strings.Contains(a.result("0-main-2"), "\nrouted #1\n") {
		t.Fatalf("no note on #1: %q\n%s\n%s", bodies, a.result("0-main-2"), a.stdout.String())
	}
	if log, err := os.ReadFile(a.state(filepath.Join("jobs", "1", "checks", red+".log"))); err != nil || !strings.HasPrefix(string(log), "make check on "+red+": exit 2\n\nother.go:1: syntax error") {
		t.Errorf("the log: %q %v", log, err)
	}
	a.forge.Relabel(issue1, forge.Paused, "")
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if sys := a.file("1-worker-2", "io/system.md"); !strings.Contains(sys, "main of app is red at "+red[:7]+" since this job's merge: its check fails there, its output in /job/memory/checks/"+red+".log. Make your branch's first commit the fix") {
		t.Errorf("the worker's Task:\n%s\n%s", sys, a.stdout.String())
	}
}

// The fix is the owner's next proposal when no swarm merge broke base
// (section 12.1, The red main): a push by hand fails its main check; the
// owner's held batch is released, with its journal line; the batch's
// prompt gains # Main, pointing to the check's output written to
// io/main/, and its Task asks for a Fixes: line; the proposal that
// carries it is the fix's, its worker launched while another issue's is
// held.
func TestSwarmRedMainRoutedToTheOwner(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nFirst.\n\n## M1 The drift\n\nA persona sees no drift.\n")
	a.publish("briefs")
	red := a.push("app", map[string]string{"other.go": "broken\n"}, "a push by hand")
	a.runner.Script(0, "main", broken)
	a.pass()
	if fileExists(a.state("owner-held")) || !strings.Contains(a.stdout.String(), "Owner batch released: app red at "+red[:7]+"\n") || !strings.Contains(a.result("0-main-2"), "\nrouted owner\n") {
		t.Fatalf("not released:\n%s", a.stdout.String())
	}
	a.runner.Script(0, "owner", runner.Canned{Out: "## Fix main\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nFixes: app\nDepends: none\n\nFix other.go.\n\n## Other work\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nDepends: none\n\nMore.\n"})
	a.pass() // the owner's batch
	file := "app-" + red[:7] + ".log"
	if p := a.file("0-owner-1", "io/prompt.md"); !strings.Contains(p, "# Main\n\n## app: main red at "+red[:7]+"\n\n`make check` fails on main at "+red+"; its output is /job/io/main/"+file+".\nNo merge since the newest green commit: a push by hand broke it.\n") {
		t.Errorf("the batch prompt:\n%s", p)
	}
	if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, `with a line "Fixes: <name>" naming the repository after its "Class:" line`) {
		t.Errorf("the batch Task:\n%s", sys)
	}
	if log := a.file("0-owner-1", "io/main/"+file); !strings.HasPrefix(log, "make check on "+red+": exit 2\n\nother.go:1: syntax error") {
		t.Errorf("io/main/%s: %q", file, log)
	}
	journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md") + "\n"
	if !strings.Contains(journal, " 0-release-0: owner batch released: app red at "+red[:7]+"\n") {
		t.Errorf("journal:\n%s", journal)
	}
	for _, n := range []int{1, 2} {
		a.forge.SetLabel(a.ref(n), "swarm:plan-1-accepted")
		a.runner.Script(n, "worker", work("v"+strconv.Itoa(n)))
	}
	a.pass()
	if a.launches("acme/swarm-app#1/worker") != 1 || a.launches("acme/swarm-app#2/worker") != 0 {
		t.Errorf("workers: the fix's %d, the other's %d\n%q\n%s", a.launches("acme/swarm-app#1/worker"), a.launches("acme/swarm-app#2/worker"), a.issue(1).Body, a.stdout.String())
	}
}

// status shows each repository's base and its queue, and what of base
// waits for the maintainer (section 12.1, What `status` shows): the merge
// gate's note names the approved head's merge check; a `Queue app:` line
// follows Running, green with the head passed, then red with it waiting;
// Waiting for you names `main red app`, which notify tells; a main check
// failing in the environment twice leaves base unchecked, a line of
// <state>/main-unchecked, `main unchecked app`, until the bare resume
// removes it and the commit is checked again.
func TestSwarmStatusQueueAndMain(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	notified := a.notifying()
	a.approvedHead(false)
	base := a.mainAt()
	a.pass() // the merge check passes; the merge gate's note
	if bodies := a.gateNotes(1); !slices.ContainsFunc(bodies, func(b string) bool { return strings.Contains(b, "\n- Merge check: queued on main at "+base[:7]+".\n") }) {
		t.Errorf("the merge gate's note: %q", bodies)
	}
	status := func() string {
		t.Helper()
		if err := a.status(a.forge); err != nil {
			t.Fatal(err)
		}
		return a.stdout.String()
	}
	if out := status(); !strings.Contains(out, "\nRunning: none\nQueue app: green at "+base[:7]+"; #1 passed\nPersonas:\n") {
		t.Errorf("status, green:\n%s", out)
	}
	red := a.push("app", map[string]string{"other.go": "broken\n"}, "a push by hand")
	a.runner.Script(0, "main", broken)
	a.pass()
	out := status()
	if !strings.Contains(out, "\nQueue app: red at "+red[:7]+"; #1 waits\n") || !strings.Contains(out, "; main red app\n") {
		t.Errorf("status, red:\n%s", out)
	}
	if got := notified(); !slices.Contains(got, "acme/swarm-app waits for you: main red app") {
		t.Errorf("notified %q", got)
	}

	b := newApplyWS(t)
	b.goApp()
	sha := b.push("app", map[string]string{"store/store.go": "package store\n\n// by hand\n"}, "a push by hand")
	b.runner.Script(0, "main", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Code: 1, Stdout: webFailed})
	b.pass()
	if u, _ := os.ReadFile(b.state("main-unchecked")); string(u) != "app "+sha+"\n" || len(b.mains()) != 2 {
		t.Fatalf("main-unchecked %q, %d checks\n%s\n%s", u, len(b.mains()), b.result("0-main-3"), b.stdout.String())
	}
	if err := b.status(b.forge); err != nil || !strings.Contains(b.stdout.String(), "\nQueue app: unchecked at "+sha[:7]+"\n") || !strings.Contains(b.stdout.String(), "Waiting for you: main unchecked app\n") {
		t.Errorf("status, unchecked: %v\n%s", err, b.stdout.String())
	}
	b.pass()
	if len(b.mains()) != 2 {
		t.Errorf("an unchecked commit checked again before resume: %d", len(b.mains()))
	}
	b.resume()
	b.pass()
	if _, err := os.Stat(b.state("main-unchecked")); err == nil || len(b.mains()) != 3 {
		t.Errorf("resume: %v, %d checks", err, len(b.mains()))
	}
}

// The queue waits while base is being checked (section 12.1, rule 4):
// a push by hand leaves base's tree with no verdict, so no merge check
// starts on it in the pass that starts its main check; once that check
// passed, base is green and the next pass starts the head's.
func TestSwarmQueueWaitsForTheMainCheck(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedHead(false)
	a.push("app", map[string]string{"hand.go": "hand\n"}, "a push by hand")
	a.pass() // the main check starts and passes
	if len(a.mains()) != 1 || len(a.merges()) != 0 {
		t.Fatalf("main checks %d, merge checks %d in the pass that checks base\n%s", len(a.mains()), len(a.merges()), a.stdout.String())
	}
	a.pass()
	if len(a.merges()) != 1 {
		t.Errorf("the queue not checking the head on the green base: %d\n%s", len(a.merges()), a.stdout.String())
	}
}

// A running main check holds issue 0's batch runs (section 12.1, rule 4,
// by section 6's rule 2): with the main check's container running, the
// owner's batch is not launched, though a brief is active.
func TestSwarmMainCheckHoldsTheBatch(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	os.Remove(a.state("owner-held"))
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nFirst.\n\n## M1 The drift\n\nA persona sees no drift.\n")
	a.publish("briefs")
	a.runner.Script(0, "owner", runner.Canned{Out: "BLOCKED: nothing to propose\n"})
	a.runner.Others = []runner.Running{{Name: "daisy-swarm.acme.swarm-app.0.main.1", Swarm: "acme/swarm-app#0/main"}}
	a.pass()
	if got := a.launches("acme/swarm-app#0/owner"); got != 0 {
		t.Fatalf("the owner's batch launched with a main check running: %d\n%s", got, a.stdout.String())
	}
	a.runner.Others = nil
	a.pass()
	if got := a.launches("acme/swarm-app#0/owner"); got != 1 {
		t.Errorf("the batch not launched once the main check is gone: %d\n%s", got, a.stdout.String())
	}
}

// mergeBy merges a side branch committing files into base (main) in the
// upstream of the declared repository name with a merge commit, as a
// merge on the forge by hand, and returns the merge.
func (a *applyWS) mergeBy(name string, files map[string]string, msg string) string {
	a.t.Helper()
	src := a.t.TempDir()
	gitIn(a.t, "", "clone", "-q", a.upstream(name), src)
	gitIn(a.t, src, "checkout", "-q", "-b", "side")
	if err := commit(files, msg)(src); err != nil {
		a.t.Fatal(err)
	}
	gitIn(a.t, src, "checkout", "-q", "main")
	gitIn(a.t, src, "merge", "-q", "--no-ff", "--no-edit", "side")
	gitIn(a.t, src, "push", "-q", "origin", "main")
	return a.mainAt()
}

// recorded writes a main check on the commit sha of app that failed in
// the environment: a commit a check recorded, no verdict on its tree.
func (a *applyWS) recorded(sha string) {
	a.t.Helper()
	tree := gitIn(a.t, a.upstream("app"), "rev-parse", sha+"^{tree}")
	dir := a.state(filepath.Join("runs", "0-main-9"))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "result"), []byte("main app "+sha+" tree "+tree+"\nfailed: check environment: x\ncollected\n"), 0o644)
}

// recordedMerge writes a merge check of app whose base is the commit sha:
// a commit a check recorded, no verdict on its tree.
func (a *applyWS) recordedMerge(sha string) {
	a.t.Helper()
	dir := a.state(filepath.Join("runs", "7-merge-9"))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "result"), []byte("merge app "+sha+" into "+sha+" at "+sha+"\nfailed: check environment: x\ncollected\n"), 0o644)
}

// With no green commit known, # Main lists the merges back to the oldest
// commit any check recorded on base's first-parent line (a main check's
// commit or a merge check's base, the newest of the line aside, a commit
// off the line ignored), and with none recorded the newest merge alone
// (the maintainer's ruling of 2026-11-27 on R2-C's open question 2).
func TestSwarmMainListsMergesToTheOldestRecordedCommit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name             string
		main, merge, off string // which merge a main check / a merge check's base records; "off": a commit off the line
		wantMid, wantOld bool
	}{
		{name: "none"},
		{name: "main at the middle", main: "mid", wantMid: true},
		{name: "merge base at the middle", merge: "mid", wantMid: true},
		{name: "oldest wins over a newer record", main: "mid", merge: "old", wantMid: true, wantOld: true},
		{name: "off the line", off: "x"},
	} {
		a := newApplyWS(t)
		os.RemoveAll(a.state(filepath.Join("runs", "0-main-1"))) // no green commit known
		a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nFirst.\n\n## M1 The drift\n\nA persona sees no drift.\n")
		a.publish("briefs")
		old := a.mergeBy("app", map[string]string{"a.go": "a\n"}, "the oldest merge")
		mid := a.mergeBy("app", map[string]string{"b.go": "b\n"}, "a merge")
		newest := a.mergeBy("app", map[string]string{"c.go": "c\n"}, "the newest merge")
		at := map[string]string{"old": old, "mid": mid}
		if c.main != "" {
			a.recorded(at[c.main])
		}
		if c.merge != "" {
			a.recordedMerge(at[c.merge])
		}
		if c.off != "" {
			off := strings.Repeat("ab", 20) // on no line of base
			dir := a.state(filepath.Join("runs", "0-main-8"))
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, "result"), []byte("main app "+off+" tree "+off+"\nfailed: check environment: x\ncollected\n"), 0o644)
		}
		a.push("app", map[string]string{"other.go": "broken\n"}, "a push by hand")
		a.runner.Script(0, "main", broken)
		a.pass() // red, the owner's
		a.runner.Script(0, "owner", runner.Canned{Out: "BLOCKED: nothing\n"})
		a.pass()
		p := a.file("0-owner-1", "io/prompt.md")
		has := func(sha string) bool { return strings.Contains(p, "\n- "+sha[:7]+" merges ") }
		if !has(newest) || has(mid) != c.wantMid || has(old) != c.wantOld {
			t.Errorf("%s: newest %v, mid %v, old %v:\n%s", c.name, has(newest), has(mid), has(old), p)
		}
	}
}
