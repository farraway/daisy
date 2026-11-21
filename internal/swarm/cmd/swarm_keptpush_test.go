package cmd

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// nothing is a worker run that commits nothing and reports text.
func nothing(text string) runner.Canned {
	return runner.Canned{Out: text + "\n\nNot established: none\n"}
}

// A kept start pushed without new commits (section 7.9, The
// classification; 7.6, step 1; the maintainer's answer of 2026-11-19 at
// the plan's Step 12 checkpoint): a worker run that starts from kept
// commits not yet on the slice's branch (for a slice not yet pushed, on
// no branch) and fetches its start unchanged has those commits to push,
// so it is not the failure no commits: its collection goes on to the
// secret check, the check and the push, the pull request opened on the
// kept commit or moved forward to it, and its report posted as any. Its
// fetched commit is not past its start, so kept still names the kept
// run, as the replay writes it. A NO CHANGE: answer from such a start is
// a revision, pushed. A start the branch already holds (the kept commit
// pushed, or the branch moved past it) has nothing to push: no commits
// as before, with a pull request open or not.
func TestSwarmKeptStartPushedWithoutNewCommits(t *testing.T) {
	t.Parallel()
	pushedKept := func(t *testing.T, a *applyWS, run, kept string) {
		t.Helper()
		r := a.result(run)
		for _, l := range []string{"\nunpushed " + kept + "\n", "\nfetched " + kept + "\n", "\npushed " + kept + "\n", "\noutcome posted\n"} {
			if !strings.Contains(r, l) {
				t.Errorf("%s's result lacks %q:\n%s", run, strings.TrimSpace(l), r)
			}
		}
		if strings.Contains(r, "failed") {
			t.Errorf("%s failed:\n%s", run, r)
		}
		if p := a.pulls(1); len(p) != 1 || p[0].HeadSHA != kept || !p[0].Open {
			t.Errorf("the pull request %+v, want one open on the kept %s", p, kept)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) {
			t.Errorf("labels %v, want review-0-ready\n%s", got, a.stdout.String())
		}
	}
	// The replayed workspace is the collections': kept still names the
	// kept run.
	replayed := func(t *testing.T, a *applyWS, keptRun string) {
		t.Helper()
		collected := a.tree(1)
		if k := collected["slices/1/kept"]; !strings.HasPrefix(k, keptRun+" ") {
			t.Errorf("kept %q, want the kept run %s's", k, keptRun)
		}
		os.RemoveAll(a.state("jobs/1"))
		os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
		a.pass()
		if got := a.tree(1); !reflect.DeepEqual(got, collected) || !a.built(1) {
			t.Errorf("replayed (built %v):\n%v\nthe collection's:\n%v", a.built(1), got, collected)
		}
	}

	t.Run("a slice not yet pushed", func(t *testing.T) {
		a := newApplyWS(t)
		home := a.metricsHome()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		if len(a.pulls(1)) != 0 {
			t.Fatalf("worker run 1 pushed:\n%s", a.result("1-worker-1"))
		}
		a.say(issue1, "jane", "v1; half is it.")
		a.runner.Script(1, "worker", nothing("main.go as kept is v1's."))
		a.pass()
		pushedKept(t, a, "1-worker-2", kept)
		if p := a.pulls(1); len(p) == 1 && !strings.Contains(p[0].Body, "main.go as kept is v1's.") {
			t.Errorf("the pull request's description:\n%s", p[0].Body)
		}
		// Its row's lines are counted over start..pushed, empty.
		var row map[string]string
		for _, rw := range rows(t, home) {
			if rw["step_or_run"] == "1-worker-2" {
				row = rw
			}
		}
		if row == nil || row["code_added"] != "0" || row["range"] != kept+".."+kept {
			t.Errorf("worker run 2's row %v", row)
		}
		replayed(t, a, "1-worker-1")
	})

	t.Run("an open pull request", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.changesRequested()
		a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
		a.pass()
		kept := a.keptOf()
		a.say(app1Pull, "jane", "v2; half is it.")
		a.runner.Script(1, "worker", nothing("main.go as kept is v2's."))
		a.pass()
		pushedKept(t, a, "1-worker-3", kept)
		cs := a.comments(app1Pull)
		if last := cs[len(cs)-1].Body; !strings.HasPrefix(last, mark("worker", 3)+"main.go as kept is v2's.") || !strings.Contains(last, "**Check:** `make check` on "+kept[:7]) {
			t.Errorf("the pull request's last comment:\n%s", last)
		}
		replayed(t, a, "1-worker-2")
	})

	t.Run("NO CHANGE: from kept commits not pushed", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.changesRequested()
		a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
		a.pass()
		kept := a.keptOf()
		a.say(app1Pull, "jane", "v2; half is it.")
		a.runner.Script(1, "worker", nothing("NO CHANGE: main.go as kept is v2's."))
		a.pass()
		pushedKept(t, a, "1-worker-3", kept)
		cs := a.comments(app1Pull)
		if last := cs[len(cs)-1].Body; !strings.Contains(last, "**Check:** `make check` on "+kept[:7]) {
			t.Errorf("the answer was not posted as a revision:\n%s", last)
		}
		replayed(t, a, "1-worker-2")
	})

	t.Run("a kept commit the branch holds", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		head := a.pulls(1)[0].HeadSHA
		if kept := a.keptOf(); kept != head {
			t.Fatalf("kept %s, want the pushed head %s", kept, head)
		}
		a.changesRequested()
		a.runner.Script(1, "worker", nothing("Nothing to change."))
		a.pass()
		r := a.result("1-worker-2")
		if !strings.HasPrefix(r, "start "+head+"\n") || strings.Contains(r, "unpushed") || !strings.Contains(r, "\nfailed: no commits\n") || strings.Contains(r, "pushed") {
			t.Errorf("a start the branch holds, nothing committed:\n%s", r)
		}
	})

	// A collection retried after its push (here, the pull request
	// refused once) reads its launch's line again: pushed once, then
	// posted, not no commits.
	t.Run("retried after its push", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		a.say(issue1, "jane", "v1; half is it.")
		a.runner.Script(1, "worker", nothing("main.go as kept is v1's."))
		refuse := true
		a.tick()
		if err := a.apply(pullRefused{a.git(), &refuse}, "--once"); err == nil || !strings.Contains(err.Error(), "the forge is down") {
			t.Fatalf("apply: %v\n%s", err, a.stdout.String())
		}
		if r := a.result("1-worker-2"); !strings.Contains(r, "\npushed "+kept+"\n") || strings.Contains(r, "outcome") {
			t.Fatalf("not pushed, or collected:\n%s", r)
		}
		refuse = false
		a.pass() // the collection, then the pair, unscripted
		r := a.result("1-worker-2")
		if strings.Count(r, "\npushed ") != 1 || strings.Contains(r, "failed") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
			t.Errorf("not pushed once and posted:\n%s", r)
		}
		if p := a.pulls(1); len(p) != 1 || p[0].HeadSHA != kept {
			t.Errorf("the pull request %+v, want one on the kept %s", p, kept)
		}
		if k, _ := a.jobFile(1, "slices/1/kept"); !strings.HasPrefix(k, "1-worker-1 ") {
			t.Errorf("kept %q, want the kept run 1-worker-1's", k)
		}
	})

	// A check held in the environment (section 7.9, The classification):
	// nothing pushed, the kept commits held as the run's ref, which the
	// note names, and kept still naming the kept run.
	t.Run("the check held in the environment", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: which version?\n", Work: in("app", commit(map[string]string{"store/store.go": "package store\n\n// half\n"}, "Half"))})
		a.pass()
		kept := a.keptOf()
		a.say(issue1, "jane", "v1; half is it.")
		a.runner.Script(1, "worker", nothing("store.go as kept is v1's."))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Code: 1, Stdout: webFailed})
		a.pass()
		r := a.result("1-worker-2")
		inOrder(t, r, "\nunpushed "+kept+"\n", "\ncheck rerun: untouched packages example.com/app/web\n", "\nfailed: check environment: untouched packages example.com/app/web\n", "\nkept refs/swarm-runs/1-worker-2\n", "\noutcome failed\n")
		if strings.Contains(r, "\npushed ") || len(a.pulls(1)) != 0 {
			t.Errorf("pushed or posted:\n%s", r)
		}
		b := a.bodies(1)
		if note := "Its commits are kept in the swarm's mirror as refs/swarm-runs/1-worker-2."; !strings.Contains(b[len(b)-1], note) {
			t.Errorf("note %q", b[len(b)-1])
		}
		if ref := a.gitRunIn(a.state("git/app"), "rev-parse", "refs/swarm-runs/1-worker-2"); ref != kept {
			t.Errorf("the run's ref %q, want the kept %s", ref, kept)
		}
		replayed(t, a, "1-worker-1")
	})

	// The secret check and protected paths (section 7.6, step 3) read
	// the kept commits again: a path protected since the kept run is
	// refused, nothing pushed, the commits held as the run's ref.
	t.Run("a protected path since the kept run", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		a.edit(func(c *declare.Config) { c.Repos[0].Protected = append(c.Repos[0].Protected, "main.go") })
		a.say(issue1, "jane", "v1; half is it.")
		a.runner.Script(1, "worker", nothing("main.go as kept is v1's."))
		a.pass()
		r := a.result("1-worker-2")
		inOrder(t, r, "\nunpushed "+kept+"\n", "\nfailed: protected path changed: main.go\n", "\nkept refs/swarm-runs/1-worker-2\n", "\noutcome failed\n")
		if strings.Contains(r, "\npushed ") || len(a.pulls(1)) != 0 || len(a.launched("check")) != 0 {
			t.Errorf("checked, pushed or posted:\n%s", r)
		}
		b := a.bodies(1)
		if note := "Its commits are kept in the swarm's mirror as refs/swarm-runs/1-worker-2."; !strings.Contains(b[len(b)-1], note) {
			t.Errorf("note %q", b[len(b)-1])
		}
		if k, _ := a.jobFile(1, "slices/1/kept"); !strings.HasPrefix(k, "1-worker-1 ") {
			t.Errorf("kept %q, want the kept run 1-worker-1's", k)
		}
	})

	// The slice's branch already holds the kept commit, pushed by hand
	// with no pull request opened: nothing to push.
	t.Run("a kept commit pushed by hand, no pull request", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		gitIn(t, a.state("git/app"), "push", "-q", a.upstream("app"), kept+":refs/heads/swarm/1-1")
		a.say(issue1, "jane", "v1; half is it.")
		a.runner.Script(1, "worker", nothing("Nothing to change."))
		a.pass()
		r := a.result("1-worker-2")
		if !strings.HasPrefix(r, "start "+kept+"\n") || strings.Contains(r, "unpushed") || !strings.Contains(r, "\nfailed: no commits\n") || len(a.pulls(1)) != 0 {
			t.Errorf("a start the branch holds, no pull request, nothing committed:\n%s", r)
		}
	})

	t.Run("the branch moved past the kept run", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.changesRequested()
		a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
		a.pass()
		src := t.TempDir()
		gitIn(t, "", "clone", "-q", "--branch", "swarm/1-1", a.upstream("app"), src)
		if err := commit(map[string]string{"notes.md": "by hand\n"}, "by hand", "--no-verify")(src); err != nil {
			t.Fatal(err)
		}
		gitIn(t, src, "push", "-q", "origin", "swarm/1-1")
		moved := a.gitRunIn(src, "rev-parse", "HEAD")
		a.say(app1Pull, "jane", "I pushed notes.md.")
		a.runner.Script(1, "worker", nothing("Nothing to change."))
		a.pass()
		r := a.result("1-worker-3")
		if !strings.HasPrefix(r, "start "+moved+"\n") || strings.Contains(r, "unpushed") || !strings.Contains(r, "\nfailed: no commits\n") {
			t.Errorf("a start at the moved head, nothing committed:\n%s", r)
		}
	})
}
