package cmd

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// goApp lands a Go module on app's main, go.mod and the packages store
// and web, so that a change to store/ leaves web untouched (section 7.9,
// The classification).
func (a *applyWS) goApp() {
	a.t.Helper()
	a.land("app", map[string]string{"go.mod": "module example.com/app\n\ngo 1.27\n", "store/store.go": "package store\n", "web/web.go": "package web\n"}, "a Go module")
}

// storeWork is a worker run that changes store/store.go with text and
// reports it.
func storeWork(text string) runner.Canned {
	return runner.Canned{Out: text + "\n\nNot established: none\n", Work: in("app", commit(map[string]string{"store/store.go": "package store\n\n// " + text + "\n"}, text))}
}

const (
	webFailed   = "--- FAIL: TestPage (5.00s)\n    page_test.go:9: context deadline exceeded\nFAIL\nFAIL\texample.com/app/web\t5.012s\n"
	storeFailed = "--- FAIL: TestFind (0.00s)\n    store_test.go:4: got v0, want v1\nFAIL\nFAIL\texample.com/app/store\t0.004s\n"
	webPassed   = "ok  \texample.com/app/web\t0.104s\n"
	rerunLine   = "#1 worker run 1: check failed in the environment (untouched packages example.com/app/web): run again\n"
)

// noChecks fails the test when the job's workspace holds checks/.
func (a *applyWS) noChecks() {
	a.t.Helper()
	if _, err := os.Stat(a.state("jobs/1/checks")); !errors.Is(err, iofs.ErrNotExist) {
		a.t.Errorf("checks/ was written: %v", err)
	}
}

// Section 7.9, The classification: a worker's check that fails in
// packages the change does not touch (or, for another language's head,
// with an infrastructure signature) is run again once, on a clone made
// again, `check rerun: <reason>` recorded, its first output kept as
// check-1.out and a line on stdout; the rerun's result is the check's.
// Green, it is posted as any green check, nothing of the first failure
// on the pull request or in checks/; failed in a package the change
// touches, it is the change's, posted with the rerun's block and
// checks/<sha>.log the rerun's. A dependency step's failure runs the
// step and then the check again; a check's beside a dependency step, the
// check alone.
func TestSwarmCheckRerunOnceThenPosted(t *testing.T) {
	t.Parallel()

	t.Run("green on the rerun", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Stdout: webPassed})
		a.pass()
		r := a.result("1-worker-1")
		if n := len(a.launched("check")); n != 2 {
			t.Fatalf("%d checks launched, want 2:\n%s", n, r)
		}
		inOrder(t, r, "\ncheck started ", "\ncheck exited 1 ", "\ncheck rerun: untouched packages example.com/app/web\n", "\ncheck started ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n")
		if got := a.file("1-worker-1", "check-1.out"); got != webFailed {
			t.Errorf("check-1.out %q", got)
		}
		if !strings.Contains(a.stdout.String(), rerunLine) {
			t.Errorf("stdout:\n%s", a.stdout.String())
		}
		ps := a.pulls(1)
		if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": exit 0\n\n```\n"+webPassed+"```") || strings.Contains(ps[0].Body, "deadline") {
			t.Fatalf("pulls %+v", ps)
		}
		a.noChecks()
		if got := labels(a.issue(1)); !slices.Equal(got, []string{forge.ReviewReady}) {
			t.Errorf("labels %v", got)
		}
	})

	t.Run("failed again, in a package the change touches", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Code: 1, Stdout: storeFailed})
		a.pass()
		r := a.result("1-worker-1")
		if n := len(a.launched("check")); n != 2 {
			t.Fatalf("%d checks launched, want 2:\n%s", n, r)
		}
		inOrder(t, r, "\ncheck exited 1 ", "\ncheck rerun: untouched packages example.com/app/web\n", "\ncheck started ", "\ncheck exited 1 ", "\npushed ", "\noutcome posted\n")
		ps := a.pulls(1)
		if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": exit 1\n\n```\n"+storeFailed+"```") || strings.Contains(ps[0].Body, "deadline") {
			t.Fatalf("pulls %+v", ps)
		}
		head := ps[0].HeadSHA
		if got, ok := a.jobFile(1, "checks/"+head+".log"); !ok || got != "make check on "+head+": exit 1\n\n"+storeFailed {
			t.Errorf("checks/%s.log (%v):\n%s", head, ok, got)
		}
	})

	t.Run("the dependency step's failure", func(t *testing.T) {
		a := newApplyWS(t)
		a.withDeps(goDeps)
		a.planned(oneSlice)
		// Another language's head: no go.mod, go.work or .go file, so a
		// signature classifies the failure.
		a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"app.js": "v1\n"}, "v1"))})
		refused := "npm ERR! connect ECONNREFUSED: connection refused\n"
		a.runner.Script(1, "deps", runner.Canned{Code: 1, Stderr: refused}, runner.Canned{})
		a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
		a.pass()
		r := a.result("1-worker-1")
		if d, c := len(a.launched("deps")), len(a.launched("check")); d != 2 || c != 1 {
			t.Fatalf("%d dependency steps and %d checks launched, want 2 and 1:\n%s", d, c, r)
		}
		inOrder(t, r, "\ndeps exited 1 ", "\n"+`check rerun: signature "connection refused"`+"\n", "\ndeps started ", "\ndeps exited 0 ", "\ncheck started ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n")
		if got := a.file("1-worker-1", "deps-1.out"); got != refused {
			t.Errorf("deps-1.out %q", got)
		}
		if !strings.Contains(a.stdout.String(), `#1 worker run 1: check failed in the environment (signature "connection refused"): run again`+"\n") {
			t.Errorf("stdout:\n%s", a.stdout.String())
		}
		a.noChecks()
	})

	t.Run("a Go head's dependency step, on a signature", func(t *testing.T) {
		// The dependency step builds nothing of the change: a signature
		// classifies its failure whatever the head, though its output
		// names no failing package (Step R3-3c).
		a := newApplyWS(t)
		a.withDeps(goDeps)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		timeout := "go: example.com/dep@v1.2.0: Get \"https://proxy.golang.org/example.com/dep/@v/v1.2.0.mod\": dial tcp 142.250.0.1:443: i/o timeout\n"
		a.runner.Script(1, "deps", runner.Canned{Code: 1, Stderr: timeout}, runner.Canned{})
		a.runner.Script(1, "check", runner.Canned{Stdout: webPassed})
		a.pass()
		r := a.result("1-worker-1")
		if d, c := len(a.launched("deps")), len(a.launched("check")); d != 2 || c != 1 {
			t.Fatalf("%d dependency steps and %d checks launched, want 2 and 1:\n%s", d, c, r)
		}
		inOrder(t, r, "\ndeps exited 1 ", "\n"+`check rerun: signature "i/o timeout"`+"\n", "\ndeps started ", "\ndeps exited 0 ", "\ncheck started ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n")
		if got := a.file("1-worker-1", "deps-1.out"); got != timeout {
			t.Errorf("deps-1.out %q", got)
		}
		a.noChecks()
	})

	t.Run("the check's failure beside a dependency step", func(t *testing.T) {
		a := newApplyWS(t)
		a.withDeps(goDeps)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "deps", runner.Canned{}, runner.Canned{Code: 1, Stderr: "a second dependency step\n"})
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Stdout: webPassed})
		a.pass()
		r := a.result("1-worker-1")
		if d, c := len(a.launched("deps")), len(a.launched("check")); d != 1 || c != 2 {
			t.Fatalf("%d dependency steps and %d checks launched, want 1 and 2:\n%s", d, c, r)
		}
		inOrder(t, r, "\ndeps exited 0 ", "\ncheck exited 1 ", "\ncheck rerun: untouched packages example.com/app/web\n", "\ncheck started ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n")
	})
}

// Section 7.9: a check whose rerun fails classified again is the
// failure check environment (7.7): `failed: check environment:
// <reason>`, the reconciler's note on the issue, swarm:blocked, the
// run's row with cause environment and its commits kept; nothing pushed,
// nothing posted on the pull request, no checks/ file. Never a third
// check.
func TestSwarmCheckEnvironmentHeld(t *testing.T) {
	t.Parallel()
	failing := []runner.Canned{{Code: 1, Stdout: webFailed}, {Code: 1, Stdout: webFailed}, {Code: 1, Stdout: webFailed}}

	t.Run("a new slice", func(t *testing.T) {
		a := newApplyWS(t)
		home := a.metricsHome()
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "check", failing...)
		a.pass()
		r := a.result("1-worker-1")
		if n := len(a.launched("check")); n != 2 {
			t.Fatalf("%d checks launched, want 2:\n%s", n, r)
		}
		reason := "check environment: untouched packages example.com/app/web"
		inOrder(t, r, "\ncheck rerun: untouched packages example.com/app/web\n", "\ncheck started ", "\ncheck exited 1 ", "\nfailed: "+reason+"\n", "\nkept refs/swarm-runs/1-worker-1\n", "\noutcome failed\n")
		if strings.Contains(r, "\npushed ") || len(a.pulls(1)) != 0 {
			t.Errorf("pushed or posted:\n%s\n%+v", r, a.pulls(1))
		}
		if branches := a.gitRunIn(a.upstream("app"), "branch", "--list", "swarm/*"); branches != "" {
			t.Errorf("upstream branches %q", branches)
		}
		a.noChecks()
		if a.file("1-worker-1", "check-1.out") != webFailed || a.file("1-worker-1", "check.out") != webFailed {
			t.Errorf("the two outputs: %q, %q", a.file("1-worker-1", "check-1.out"), a.file("1-worker-1", "check.out"))
		}
		if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) {
			t.Errorf("labels %v", got)
		}
		b := a.bodies(1)
		note := "worker run 1 failed: " + reason + " (at swarm:plan-1-accepted). Remove swarm:blocked to resume. Its commits are kept in the swarm's mirror as refs/swarm-runs/1-worker-1."
		if !strings.Contains(b[len(b)-1], note) {
			t.Errorf("note %q", b[len(b)-1])
		}
		if k, ok := a.jobFile(1, "slices/1/kept"); !ok || !strings.HasPrefix(k, "1-worker-1 ") {
			t.Errorf("kept %q (%v)", k, ok)
		}
		var row map[string]string
		for _, rw := range rows(t, home) {
			if rw["step_or_run"] == "1-worker-1" {
				row = rw
			}
		}
		if row == nil || row["cause"] != "environment" {
			t.Errorf("row %v", row)
		}
	})

	t.Run("an untouched package naming a secret", func(t *testing.T) {
		a := newApplyWS(t)
		secret := a.vars["CLAUDE_CODE_OAUTH_TOKEN"]
		a.goApp()
		a.land("app", map[string]string{"web/" + secret + "/x.go": "package x\n"}, "a package")
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		out := "FAIL\texample.com/app/web/" + secret + "\t0.1s\n"
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: out}, runner.Canned{Code: 1, Stdout: out})
		a.pass()
		r := a.result("1-worker-1")
		withheld := "untouched packages withheld: one holds a secret the member was given"
		inOrder(t, r, "\ncheck rerun: "+withheld+"\n", "\nfailed: check environment: "+withheld+"\n")
		b := a.bodies(1)
		if strings.Contains(r, secret) || strings.Contains(a.stdout.String(), secret) || strings.Contains(strings.Join(b, "\n"), secret) || !strings.Contains(b[len(b)-1], withheld) {
			t.Errorf("the secret named, or the note without the reason:\n%s\n%s\n%q", r, a.stdout.String(), b)
		}
	})

	t.Run("on the pull request", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.pass()
		h1 := a.pulls(1)[0].HeadSHA
		a.changesRequested()
		before := len(a.comments(app1Pull))
		a.runner.Script(1, "worker", storeWork("v2"))
		a.runner.Script(1, "check", failing...)
		a.pass()
		r := a.result("1-worker-2")
		if n := len(a.launched("check")); n != 3 {
			t.Fatalf("%d checks launched, want 3 (run 1's, run 2's and its rerun):\n%s", n, r)
		}
		inOrder(t, r, "\ncheck rerun: untouched packages example.com/app/web\n", "\nfailed: check environment: untouched packages example.com/app/web\n", "\noutcome failed\n")
		if p := a.pulls(1)[0]; p.HeadSHA != h1 || len(a.comments(app1Pull)) != before {
			t.Errorf("the pull request moved (%s, want %s) or was commented on (%d comments, want %d)", p.HeadSHA, h1, len(a.comments(app1Pull)), before)
		}
		if got := a.gitRunIn(a.upstream("app"), "rev-parse", "swarm/1-1"); got != h1 {
			t.Errorf("swarm/1-1 at %s, want %s", got, h1)
		}
		a.noChecks()
		if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) {
			t.Errorf("labels %v", got)
		}
	})
}

// A job's workspace built by replay (section 7.8, Rollout) reads a
// check run again as its collection did: the lines after `check rerun`,
// so a rerun that passed writes no checks/ file, and none for a run that
// failed check environment.
func TestWorkspaceReplayRerunCheck(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		rerun runner.Canned
	}{
		{"green on the rerun", runner.Canned{Stdout: webPassed}},
		{"held in the environment", runner.Canned{Code: 1, Stdout: webFailed}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.goApp()
			a.planned(oneSlice)
			a.runner.Script(1, "worker", storeWork("v1"))
			a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: webFailed}, tc.rerun)
			a.pass()
			if r := a.result("1-worker-1"); !strings.Contains(r, "\ncheck rerun: ") {
				t.Fatalf("no rerun:\n%s", r)
			}
			collected := a.tree(1)
			a.noChecks()
			os.RemoveAll(a.state("jobs/1"))
			os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
			a.pass()
			if got := a.tree(1); !reflect.DeepEqual(got, collected) || !a.built(1) {
				t.Errorf("replayed (built %v):\n%v\nthe collection's:\n%v", a.built(1), got, collected)
			}
		})
	}
}

// pullRefused is a forge that refuses to open a pull request while
// refuse is set.
type pullRefused struct {
	gitForge
	refuse *bool
}

func (f pullRefused) CreatePull(repo, head, base, title, body string) (int, error) {
	if *f.refuse {
		return 0, errors.New("the forge is down")
	}
	return f.gitForge.CreatePull(repo, head, base, title, body)
}

// What is not classified (section 7.9) is not run again: a check whose
// test services file was refused, though its reason holds a signature,
// a check stopped at the wall clock whose output names untouched
// packages alone, and a check whose collection is retried after its push (here, a
// failure the change's at the push, its output then rewritten to one in
// an untouched package, as an older build's push would leave it), each
// posted as any failing check, one check launched.
func TestSwarmCheckNotRerun(t *testing.T) {
	t.Parallel()

	t.Run("a test services file refused", func(t *testing.T) {
		a := newApplyWS(t)
		file := "connection refused.yaml"
		a.edit(func(c *declare.Config) { c.Repos[0].Services = &file })
		a.publish("services")
		a.fetchMirror()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"app.js": "v1\n"}, "v1"))})
		a.pass()
		r := a.result("1-worker-1")
		inOrder(t, r, "\nservices check refused: connection refused.yaml: no such file at ", "\npushed ", "\noutcome posted\n")
		if strings.Contains(r, "\ncheck rerun") || len(a.launched("check")) != 0 {
			t.Errorf("run again:\n%s", r)
		}
	})

	t.Run("a timeout in untouched packages", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "check", runner.Canned{TimedOut: true, Code: 137, Stdout: webFailed}, runner.Canned{Stdout: webPassed})
		a.pass()
		r := a.result("1-worker-1")
		inOrder(t, r, "\ncheck timeout ", "\npushed ", "\noutcome posted\n")
		if strings.Contains(r, "\ncheck rerun") || len(a.launched("check")) != 1 {
			t.Errorf("run again:\n%s", r)
		}
	})

	t.Run("after the push", func(t *testing.T) {
		a := newApplyWS(t)
		a.goApp()
		a.planned(oneSlice)
		a.runner.Script(1, "worker", storeWork("v1"))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: storeFailed}, runner.Canned{Code: 1, Stdout: storeFailed})
		refuse := true
		a.tick()
		if err := a.apply(pullRefused{a.git(), &refuse}, "--once"); err == nil || !strings.Contains(err.Error(), "the forge is down") {
			t.Fatalf("apply: %v\n%s", err, a.stdout.String())
		}
		if r := a.result("1-worker-1"); !strings.Contains(r, "\npushed ") {
			t.Fatalf("not pushed:\n%s", r)
		}
		os.WriteFile(a.state("runs/1-worker-1/check.out"), []byte(webFailed), 0o644)
		refuse = false
		a.pass()
		r := a.result("1-worker-1")
		if strings.Contains(r, "\ncheck rerun") || len(a.launched("check")) != 1 || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
			t.Errorf("run again, or not posted:\n%s", r)
		}
	})
}

// A rerun whose apply died, gone with no end line (section 7.9; 7.6,
// step 4): read from the lines after `check rerun`, it is the step
// still running, started again by the next pass, whose result is the
// check's; the first output stays check-1.out.
func TestSwarmCheckRerunRestartedAfterItsApplyDied(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker("exited 0 2026-11-06T15:10:00Z")
	a.gitRunIn(a.state("git/app"), "fetch", "-q", "--no-tags", a.state("runs/1-worker-1/work"), "+refs/heads/swarm/1-1:refs/swarm-runs/1-worker-1")
	run := a.state("runs/1-worker-1")
	os.WriteFile(filepath.Join(run, "io", "out.md"), []byte("v1\n\nNot established: none\n"), 0o644)
	os.WriteFile(filepath.Join(run, "check-1.out"), []byte(webFailed), 0o644)
	f, _ := os.OpenFile(filepath.Join(run, "result"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("fetched " + head + "\ncheck started 2026-11-06T15:11:00Z\ncheck exited 1 2026-11-06T15:12:00Z\ncheck rerun: untouched packages example.com/app/web\ncheck started 2026-11-06T15:13:00Z\n")
	f.Close()
	a.runner.Script(1, "check", runner.Canned{Stdout: webPassed})
	a.pass()
	r := a.result("1-worker-1")
	inOrder(t, r, "\ncheck rerun: ", "\ncheck started ", "\ncheck started ", "\ncheck exited 0 ", "\npushed "+head+"\n", "\noutcome posted\n")
	if n := len(a.launched("check")); n != 1 || a.file("1-worker-1", "check-1.out") != webFailed {
		t.Errorf("%d checks launched, check-1.out %q\n%s", n, a.file("1-worker-1", "check-1.out"), r)
	}
	if ps := a.pulls(1); len(ps) != 1 || !strings.Contains(ps[0].Body, ": exit 0\n\n```\n"+webPassed+"```") {
		t.Errorf("pulls %+v", ps)
	}
}
