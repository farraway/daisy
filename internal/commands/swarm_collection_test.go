package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// gitRunIn runs git in dir, failing the test on an error, and returns its
// output trimmed.
func (a *applyWS) gitRunIn(dir string, args ...string) string {
	a.t.Helper()
	out, errb, err := ops.Real().Exec(dir, "git", args...)
	if err != nil {
		a.t.Fatalf("git %v: %v %s", args, err, errb)
	}
	return strings.TrimSpace(string(out))
}

// orphanWorker is worker run 1 of #1 as an apply that died left it: a
// clone of app on swarm/1-1 holding one commit past main, the result's
// lines up to its in-progress relabel and then more, #1 at
// plan-2-in-progress, no container. It returns the clone's head.
func (a *applyWS) orphanWorker(more ...string) string {
	a.t.Helper()
	run := a.state("runs/1-worker-1")
	work := filepath.Join(run, "work")
	os.MkdirAll(filepath.Join(run, "io"), 0o755)
	a.gitRunIn("", "clone", "-q", a.upstream("app"), work)
	start := a.gitRunIn(work, "rev-parse", "HEAD")
	a.gitRunIn(work, "checkout", "-q", "-b", "swarm/1-1")
	a.gitRunIn(work, "config", "user.name", "worker (Daisy swarm)")
	a.gitRunIn(work, "config", "user.email", "swarm@example.net")
	if err := commit(map[string]string{"main.go": "v1\n"}, "v1")(work); err != nil {
		a.t.Fatal(err)
	}
	head := a.gitRunIn(work, "rev-parse", "HEAD")
	lines := append([]string{"start " + start, "branch swarm/1-1 in app", "label swarm:plan-1-accepted", "started 2026-11-06T14:59:00Z",
		"credential anthropic env CLAUDE_CODE_OAUTH_TOKEN", "relabelled swarm:plan-1-accepted swarm:plan-2-in-progress"}, more...)
	os.WriteFile(filepath.Join(run, "result"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	a.forge.SetLabel(issue1, "swarm:plan-2-in-progress")
	return head
}

// A worker run whose apply died is abandoned with its commits kept in the
// mirror as the run's ref (section 7.6, steps 1 and 2; 7.7, abandoned),
// and its clone removed though the member left a directory in it that
// cannot be read.
func TestSwarmAbandonedWorkerKeepsItsCommits(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker()
	closed := filepath.Join(a.state("runs/1-worker-1/work"), "build")
	os.MkdirAll(closed, 0o755)
	os.WriteFile(filepath.Join(closed, "out.o"), []byte("x"), 0o644)
	os.Chmod(closed, 0)
	t.Cleanup(func() { os.Chmod(closed, 0o755) })
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	r := a.result("1-worker-1")
	if !strings.Contains(r, "\nabandoned ") || !strings.Contains(r, "\nfetched "+head+"\n") || !strings.Contains(r, "\nkept refs/swarm-runs/1-worker-1\n") ||
		!strings.HasSuffix(r, "outcome abandoned\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
	if got := a.gitRunIn(a.state("git/app"), "rev-parse", "refs/swarm-runs/1-worker-1"); got != head {
		t.Errorf("the run's ref is %q, want %s", got, head)
	}
	if _, err := os.Lstat(a.state("runs/1-worker-1/work")); err == nil {
		t.Error("work/ kept after the collection")
	}
}

// An abandoned worker run whose clone holds no branch (its member killed
// mid-run): nothing kept, the warning, and the run still collected and
// its issue taken back (section 7.7).
func TestSwarmAbandonedWorkerWithoutItsBranch(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.orphanWorker()
	work := a.state("runs/1-worker-1/work")
	a.gitRunIn(work, "checkout", "-q", "--detach")
	a.gitRunIn(work, "branch", "-q", "-D", "swarm/1-1")
	a.pass()
	r := a.result("1-worker-1")
	if !strings.HasSuffix(r, "outcome abandoned\ncollected\n") || strings.Contains(r, "\nkept ") ||
		!strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: abandoned; commits not kept (") {
		t.Errorf("result:\n%s\nstderr:\n%s", r, a.stderr.String())
	}
}

// An abandoned worker run whose fetch fails with git's error (its clone's
// .git left with no HEAD, which git reads as no repository) is retried as
// a worker run's fetch is (section 7.6, step 1): the first failure is the
// pass's error, recorded, the run left uncollected and its issue at its
// in-progress stage; a fetch that then succeeds keeps the commits, and a
// second failure in a row keeps nothing, with the warning, and the run is
// collected (7.7, abandoned).
func TestSwarmAbandonedWorkerFetchRetried(t *testing.T) {
	for _, mended := range []bool{true, false} {
		t.Run(map[bool]string{true: "mended", false: "failing again"}[mended], func(t *testing.T) {
			a := newApplyWS(t)
			a.planned(oneSlice)
			head := a.orphanWorker()
			headFile := filepath.Join(a.state("runs/1-worker-1/work"), ".git", "HEAD")
			os.Rename(headFile, headFile+".aside")
			a.tick()
			if err := a.apply(a.git(), "--once"); err == nil || !strings.Contains(err.Error(), "git fetch") {
				t.Fatalf("the first failed fetch: %v\n%s", err, a.stderr.String())
			}
			r := a.result("1-worker-1")
			if strings.Count(r, "\nfetch failed: git fetch ") != 1 || strings.Contains(r, "collected") || strings.Contains(a.stderr.String(), "commits not kept") ||
				!reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:plan-2-in-progress"}) {
				t.Fatalf("after the first:\n%s\n%v\n%s", r, labels(a.issue(1)), a.stderr.String())
			}
			if mended {
				os.Rename(headFile+".aside", headFile)
			}
			a.pass()
			r = a.result("1-worker-1")
			kept := strings.Contains(r, "\nfetched "+head+"\n") && strings.Contains(r, "\nkept refs/swarm-runs/1-worker-1\n")
			warned := strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: abandoned; commits not kept (git fetch ")
			if !strings.HasSuffix(r, "outcome abandoned\ncollected\n") || kept != mended || warned == mended || strings.Count(r, "\nfetch failed: ") != map[bool]int{true: 1, false: 2}[mended] {
				t.Errorf("after the second:\n%s\nstderr:\n%s", r, a.stderr.String())
			}
		})
	}
}

// An abandoned worker run whose fetch succeeds and whose trailer's log
// then fails (base gone from the mirror) keeps its commits, the fetch
// being done: the log's error is the pass's, and the next pass collects
// the run (section 7.6, steps 1 and 2).
func TestSwarmAbandonedWorkerKeepsWhatItFetched(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker()
	mirror := a.state("git/app")
	base := a.gitRunIn(mirror, "rev-parse", "refs/heads/main")
	a.gitRunIn(mirror, "update-ref", "-d", "refs/heads/main")
	a.tick()
	if err := a.apply(a.git(), "--once"); err == nil || !strings.Contains(err.Error(), "git log") {
		t.Fatalf("the trailer's log: %v", err)
	}
	r := a.result("1-worker-1")
	if !strings.Contains(r, "\nfetched "+head+"\nkept refs/swarm-runs/1-worker-1\n") || strings.Contains(r, "collected") || strings.Contains(a.stderr.String(), "commits not kept") {
		t.Fatalf("after the first:\n%s\nstderr:\n%s", r, a.stderr.String())
	}
	a.gitRunIn(mirror, "update-ref", "refs/heads/main", base)
	a.pass()
	r = a.result("1-worker-1")
	if !strings.HasSuffix(r, "outcome abandoned\ncollected\n") || strings.Count(r, "\nkept ") != 1 {
		t.Errorf("after the second:\n%s", r)
	}
	if got := a.gitRunIn(mirror, "rev-parse", "refs/swarm-runs/1-worker-1"); got != head {
		t.Errorf("the run's ref is %q, want %s", got, head)
	}
}

// A run another apply started, its container gone (section 7.7): ended
// only when the sh its argv runs under wrote the CLI's exit status to
// io/exit, whatever out.md holds; a run launched before the mark, by its
// out.md.
func TestSwarmOrphanEndedByTheExitMark(t *testing.T) {
	marked := strings.Join(append([]string{"docker", "run"}, exitPrefix...), "\n") + "\nclaude\n"
	for _, tc := range []struct {
		name, argv, exit, out string
		want                  string // the result's lines after the end line
	}{
		{name: "out.md without the mark", argv: marked, out: archPlan, want: "abandoned"},
		{name: "a partial out.md without the mark", argv: marked, out: "## Research\n\n- /work/app/ma", want: "abandoned"},
		{name: "the mark not the script's", argv: marked, exit: "done\n", out: archPlan, want: "abandoned"},
		{name: "the mark and out.md", argv: marked, exit: "0\n", out: archPlan, want: "ended posted"},
		{name: "the mark and an empty out.md", argv: marked, exit: "0\n", out: "", want: "ended failed: no output"},
		{name: "the mark of a non-zero exit", argv: marked, exit: "3\n", out: archPlan, want: "ended failed: exit 3"},
		{name: "the mark of a command that cannot run", argv: marked, exit: "127\n", out: "", want: "ended fault: exit 127"},
		{name: "launched before the mark", argv: "docker\nrun\nclaude\n", out: archPlan, want: "ended posted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			run := a.state("runs/1-architect-1")
			os.MkdirAll(filepath.Join(run, "io"), 0o755)
			os.MkdirAll(filepath.Join(run, "work"), 0o755)
			os.WriteFile(filepath.Join(run, "argv"), []byte(tc.argv), 0o644)
			os.WriteFile(filepath.Join(run, "io", "out.md"), []byte(tc.out), 0o644)
			if tc.exit != "" {
				os.WriteFile(filepath.Join(run, "io", "exit"), []byte(tc.exit), 0o644)
			}
			os.WriteFile(filepath.Join(run, "result"), []byte("start app 0123\nlabel swarm:proposal-accepted\nstarted 2026-11-06T14:30:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\n"), 0o644)
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan}) // run 2, after an abandoned run 1
			a.tick()
			a.apply(a.git(), "--once")
			r := a.result("1-architect-1")
			end, rest, _ := strings.Cut(strings.TrimPrefix(r, "start app 0123\nlabel swarm:proposal-accepted\nstarted 2026-11-06T14:30:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\n"), "\n")
			got := strings.Fields(end)[0]
			switch {
			case strings.HasPrefix(tc.want, "ended posted"):
				if got != "ended" || !strings.HasPrefix(rest, "posted comment\n") {
					t.Errorf("result:\n%s", r)
				}
			case strings.HasPrefix(tc.want, "ended "):
				if got != "ended" || !strings.HasPrefix(rest, strings.TrimPrefix(tc.want, "ended ")+"\n") {
					t.Errorf("result:\n%s", r)
				}
			default:
				if got != "abandoned" || !strings.HasSuffix(r, "outcome abandoned\ncollected\n") ||
					slices.ContainsFunc(a.bodies(1), func(b string) bool { return strings.HasPrefix(b, mark("architect", 1)) }) {
					t.Errorf("result:\n%s\ncomments %q", r, a.bodies(1))
				}
			}
		})
	}
}

// A worker's post longer than the limit: its text cut so that the text,
// one blank line and the check block hold to 60 000 bytes (section 7.5),
// the block whole after it.
func TestSwarmWorkerPostHoldsToTheLimitWithItsCheckBlock(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	long := strings.Repeat("A line of the worker's report, long enough to count.\n", 1300) // about 69 KB
	a.runner.Script(1, "worker", runner.Canned{Out: long, Work: commit(map[string]string{"main.go": "v1\n"}, "v1")})
	a.runner.Script(1, "check", runner.Canned{Stdout: strings.Repeat("ok\n", 30)})
	a.pass()
	ps := a.pulls(1)
	if len(ps) != 1 {
		t.Fatalf("pulls %+v", ps)
	}
	posted := strings.TrimSuffix(strings.TrimPrefix(ps[0].Body, mark("worker", 1)), "\n\nRefs acme/swarm-app#1")
	note := "\n[truncated by Daisy; the whole output is in runs/1-worker-1]\n\n**Check:** `make check` on "
	if len(posted) > 60000 || !strings.Contains(posted, note) || !strings.HasSuffix(posted, ": exit 0\n\n```\n"+strings.Repeat("ok\n", 19)+"ok\n```") {
		t.Errorf("posted %d bytes:\n%s", len(posted), posted[max(0, len(posted)-400):])
	}
}

// A worker's text ending in CRLF line ends: the line break before the
// blank line and the check block is taken off whole, no "\r" left
// before it (section 7.5).
func TestSwarmWorkerPostTakesOffACRLFEnd(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\r\nNot established: none\r\n", Work: commit(map[string]string{"main.go": "v1\n"}, "v1")})
	a.pass()
	ps := a.pulls(1)
	if len(ps) != 1 || !strings.HasPrefix(ps[0].Body, mark("worker", 1)+"Done.\r\nNot established: none\n\n**Check:** ") {
		t.Errorf("pulls %+v", ps)
	}
}

// moveOnReview is the maintainer moving issue 1 to review-0-ready while
// the pair's second reviewer posts its review.
type moveOnReview struct {
	gitForge
	reviews int
}

func (m *moveOnReview) Review(pr forge.Ref, v forge.Verdict, body string) error {
	if m.reviews++; m.reviews == 2 {
		m.Fake.SetLabel(issue1, "swarm:review-0-ready")
	}
	return m.gitForge.Review(pr, v, body)
}

// The pair's second reviewer, launched on review-1-in-progress, finds the
// issue moved to review-0-ready during its run: T9 is skipped with the
// warning, not made from review-0-ready (section 7.6).
func TestSwarmSecondReviewerSkipsAMoveToReviewReady(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.tick()
	if err := a.apply(&moveOnReview{gitForge: a.git()}, "--once"); err != nil {
		t.Fatal(err)
	}
	r := a.result("1-sympathetic-1")
	if got := labels(a.issue(1)); !strings.Contains(r, "\nlabel swarm:review-1-in-progress\n") || slices.Contains(got, "swarm:review-2-approved") ||
		!slices.Contains(got, "swarm:review-0-ready") || !strings.Contains(a.stderr.String(), "#1: relabelled to swarm:review-0-ready during the run; not relabelled") {
		t.Errorf("labels %v\n%s\nstderr:\n%s", got, r, a.stderr.String())
	}
}

// A worker run's check whose apply died, gone with no end line: started
// again by the next pass and the run collected from it (section 7.6,
// step 4).
func TestSwarmCheckRestartedAfterItsApplyDied(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker("exited 0 2026-11-06T15:10:00Z")
	a.gitRunIn(a.state("git/app"), "fetch", "-q", "--no-tags", a.state("runs/1-worker-1/work"), "+refs/heads/swarm/1-1:refs/swarm-runs/1-worker-1")
	run := a.state("runs/1-worker-1")
	os.WriteFile(filepath.Join(run, "io", "out.md"), []byte("v1\n\nNot established: none\n"), 0o644)
	f, _ := os.OpenFile(filepath.Join(run, "result"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("fetched " + head + "\ncheck started 2026-11-06T15:11:00Z\n")
	f.Close()
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
	a.pass()
	r := a.result("1-worker-1")
	var checks []runner.Spec
	for _, s := range a.runner.Launches() {
		if strings.HasSuffix(s.Label, "/check") {
			checks = append(checks, s)
		}
	}
	if strings.Count(r, "\ncheck started ") != 2 || !strings.Contains(r, "\ncheck exited 0 ") || !strings.Contains(r, "\npushed "+head+"\n") ||
		len(checks) != 1 || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-0-ready"}) {
		t.Errorf("checks %d, labels %v\n%s", len(checks), labels(a.issue(1)), r)
	}
}

// A worker run whose member left a directory in its clone that cannot be
// read (mode 000, holding a file): the run is collected and its clone
// removed, the pass exiting 0; a link in that directory to one outside
// the run that cannot be read either is removed, never followed, so the
// directory outside keeps its mode (section 7.6).
func TestSwarmCollectionRemovesAnUnreadableDirectory(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	var closed string
	outside := filepath.Join(t.TempDir(), "outside")
	os.MkdirAll(filepath.Join(outside, "sub"), 0o755)
	os.Chmod(outside, 0)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n", Work: func(work string) error {
		if err := commit(map[string]string{"main.go": "v1\n"}, "v1")(work); err != nil {
			return err
		}
		closed = filepath.Join(work, "build")
		os.MkdirAll(closed, 0o755)
		os.WriteFile(filepath.Join(closed, "out.o"), []byte("x"), 0o644)
		if err := os.Symlink(outside, filepath.Join(closed, "outside")); err != nil {
			return err
		}
		return os.Chmod(closed, 0)
	}})
	t.Cleanup(func() { os.Chmod(closed, 0o755); os.Chmod(outside, 0o755) })
	a.pass()
	if r := a.result("1-worker-1"); !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
	if _, err := os.Lstat(a.state("runs/1-worker-1/work")); err == nil {
		t.Error("work/ kept after the collection")
	}
	if fi, err := os.Lstat(outside); err != nil {
		t.Error(err)
	} else if fi.Mode().Perm() != 0 {
		t.Errorf("the directory outside the run was opened through the link: %v", fi.Mode())
	}
}
