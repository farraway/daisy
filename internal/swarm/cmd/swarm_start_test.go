package cmd

import (
	"os"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// app1Pull is issue #1's first pull request, on app.
var app1Pull = forge.Ref{Repo: "app", N: 1}

// seen is what a worker run found in its checkout before it committed:
// its branch and its head.
type seen struct{ branch, head, main string }

// look is a worker's run that records what its checkout of app holds,
// then commits main.go with text and reports it.
func look(into *seen, text string) runner.Canned {
	return runner.Canned{Out: text + "\n\nNot established: none\n", Work: in("app", func(work string) error {
		into.branch = gitOut(work, "rev-parse", "--abbrev-ref", "HEAD")
		into.head = gitOut(work, "rev-parse", "HEAD")
		b, _ := os.ReadFile(work + "/main.go")
		into.main = string(b)
		return commit(map[string]string{"main.go": text + "\n"}, text)(work)
	})}
}

func gitOut(dir string, args ...string) string {
	out, _, _ := ops.Real().Exec(dir, "git", args...)
	return strings.TrimSpace(string(out))
}

// half is a worker run that commits main.go "half" and stops blocked
// (or with the text out), its commits kept.
func half(out string) runner.Canned {
	return runner.Canned{Out: out, Work: in("app", commit(map[string]string{"main.go": "half\n"}, "Half"))}
}

// keptOf is slice 1's kept commit on issue #1, from its workspace.
func (a *applyWS) keptOf() string {
	a.t.Helper()
	k, ok := a.jobFile(1, "slices/1/kept")
	if !ok {
		a.t.Fatalf("no kept:\n%s", a.stdout.String())
	}
	_, sha, _ := strings.Cut(strings.TrimSpace(k), " ")
	return sha
}

// changesRequested takes #1, in review on app#1, through a round that
// requests changes: review-3-changes-requested.
func (a *applyWS) changesRequested() {
	a.t.Helper()
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := labels(a.issue(1)); len(got) == 0 || got[0] != forge.ChangesRequested {
		a.t.Fatalf("not at changes requested: %v\n%s", got, a.stdout.String())
	}
}

// The start (section 7.8): the next worker run of a slice starts from
// the slice's kept commit. A blocked worker's commits on a slice not yet
// pushed are on the next run's new branch swarm/1-1, whose push then
// holds them; on a slice whose pull request is open, a blocked worker's
// commits past the head move the branch forward to them; and a branch
// moved past the kept run by a push not the run's starts the next run at
// the branch's head, which the push did not overwrite.
func TestWorkerStartsFromKept(t *testing.T) {
	t.Run("a slice not yet pushed", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		a.say(issue1, "jane", "v1.")
		var got seen
		a.runner.Script(1, "worker", look(&got, "v1"))
		a.pass()
		if got.branch != "swarm/1-1" || got.head != kept || got.main != "half\n" {
			t.Fatalf("worker run 2 started on %q at %s (main.go %q), want swarm/1-1 at the kept %s\n%s", got.branch, got.head, got.main, kept, a.result("1-worker-2"))
		}
		if r := a.result("1-worker-2"); !strings.HasPrefix(r, "start "+kept+"\n") {
			t.Errorf("worker run 2's start line:\n%s", r)
		}
		head := a.pulls(1)[0].HeadSHA
		if parent := a.gitRunIn(a.state("git/app"), "rev-parse", head+"^"); parent != kept {
			t.Errorf("the pushed head's parent %s, want the kept %s", parent, kept)
		}
	})

	t.Run("an open pull request moved forward", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		h1 := a.pulls(1)[0].HeadSHA
		a.changesRequested()
		a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
		a.pass()
		kept := a.keptOf()
		if kept == h1 || a.pulls(1)[0].HeadSHA != h1 {
			t.Fatalf("worker run 2 pushed or kept nothing: kept %s, head %s\n%s", kept, a.pulls(1)[0].HeadSHA, a.result("1-worker-2"))
		}
		a.say(app1Pull, "jane", "v2.")
		var got seen
		a.runner.Script(1, "worker", look(&got, "v2"))
		a.pass()
		if got.branch != "swarm/1-1" || got.head != kept {
			t.Fatalf("worker run 3 started on %q at %s, want swarm/1-1 at the kept %s (the head %s)\n%s", got.branch, got.head, kept, h1, a.result("1-worker-3"))
		}
		head := a.pulls(1)[0].HeadSHA
		if parent := a.gitRunIn(a.state("git/app"), "rev-parse", head+"^"); parent != kept {
			t.Errorf("the pushed head's parent %s, want the kept %s\n%s", parent, kept, a.result("1-worker-3"))
		}
	})

	t.Run("a kept commit gone from the mirror", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		base := a.gitRunIn(a.state("git/app"), "rev-parse", "refs/heads/main")
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		kept := a.keptOf()
		mirror := a.state("git/app")
		a.gitRunIn(mirror, "update-ref", "-d", "refs/swarm-runs/1-worker-1")
		a.gitRunIn(mirror, "reflog", "expire", "--expire=now", "--all")
		a.gitRunIn(mirror, "gc", "-q", "--prune=now")
		a.say(issue1, "jane", "v1.")
		var got seen
		a.runner.Script(1, "worker", look(&got, "v1"))
		a.pass()
		if got.head != base {
			t.Fatalf("worker run 2 started at %s, want base %s\n%s", got.head, base, a.stderr.String())
		}
		if w := "⚠ #1 worker run 2: kept commit " + kept[:7] + " of 1-worker-1 is not in app's mirror; started as with no kept run\n"; !strings.Contains(a.stderr.String(), w) {
			t.Errorf("stderr lacks %q:\n%s", w, a.stderr.String())
		}
	})

	t.Run("a kept file Daisy cannot use", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		if err := os.WriteFile(a.state("jobs/1/slices/1/kept"), []byte("not a kept line\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.say(issue1, "jane", "v1.")
		a.runner.Script(1, "worker", work("v1"))
		a.pass()
		if _, err := os.Stat(a.state("runs/1-worker-2")); err == nil {
			t.Errorf("worker run 2 launched:\n%s", a.result("1-worker-2"))
		}
		if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !strings.Contains(a.lastComment(1), "Daisy cannot use jobs/1/slices/1/kept: not one line `<n>-worker-<k> <sha>`") {
			t.Errorf("not blocked: %v, last comment:\n%s", got, a.lastComment(1))
		}
	})

	t.Run("the branch moved past the kept run", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.changesRequested()
		a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
		a.pass()
		kept := a.keptOf()
		// The maintainer pushes to the slice's branch, past the head the
		// kept run started from but not past the kept commit.
		src := t.TempDir()
		gitIn(t, "", "clone", "-q", "--branch", "swarm/1-1", a.upstream("app"), src)
		if err := commit(map[string]string{"notes.md": "by hand\n"}, "by hand", "--no-verify")(src); err != nil {
			t.Fatal(err)
		}
		gitIn(t, src, "push", "-q", "origin", "swarm/1-1")
		moved := a.gitRunIn(src, "rev-parse", "HEAD")
		a.say(app1Pull, "jane", "I pushed notes.md.")
		var got seen
		a.runner.Script(1, "worker", look(&got, "v2"))
		a.pass()
		if got.branch != "swarm/1-1" || got.head != moved {
			t.Fatalf("worker run 3 started on %q at %s, want swarm/1-1 at the moved head %s (kept %s)\n%s", got.branch, got.head, moved, kept, a.result("1-worker-3"))
		}
	})
}

// The start's reset (section 7.8, The start): a line `Start: base`,
// alone on a line, whitespace aside, outside a fenced block, written
// since the kept run ended, in the architect's answer, its revised plan
// or the maintainer's counted comment that lifts the block, starts the
// next worker run where it would with no kept run: base, for a slice not
// yet pushed. A line inside a fence, one with more words, one written
// before the kept run ended, and one by a login not trusted start it
// from the kept commit.
func TestStartBaseLine(t *testing.T) {
	blocked := func(t *testing.T) (*applyWS, string, string) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		base := a.gitRunIn(a.state("git/app"), "rev-parse", "refs/heads/main")
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		return a, base, a.keptOf()
	}
	next := func(t *testing.T, a *applyWS, k int, want string) {
		t.Helper()
		var got seen
		a.runner.Script(1, "worker", look(&got, "v1"))
		a.pass()
		if got.branch != "swarm/1-1" || got.head != want {
			t.Fatalf("worker run %d started on %q at %s, want %s\n%s\n%s", k, got.branch, got.head, want, a.result("1-worker-"+string(rune('0'+k))), a.stdout.String())
		}
	}

	for _, c := range []struct {
		name, comment string
		base          bool
	}{
		{"alone", "v1, from scratch.\n\nStart: base\n", true},
		{"whitespace at either end", "v1.\n  Start: base \r\n", true},
		{"in a fence", "v1.\n\n```\nStart: base\n```\n", false},
		{"more words", "v1. Start: base\n", false},
		{"a trailing full stop", "v1.\nStart: base.\n", false},
	} {
		t.Run("the maintainer's word: "+c.name, func(t *testing.T) {
			a, base, kept := blocked(t)
			a.say(issue1, "jane", c.comment)
			want := kept
			if c.base {
				want = base
			}
			next(t, a, 2, want)
		})
	}

	t.Run("swarm unblock's comment", func(t *testing.T) {
		a, base, _ := blocked(t)
		a.tick()
		m := &maintainerWS{statusWS: a.statusWS}
		m.secrets(bothTokens)
		m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
		if err := m.verb(swarmUnblock, "1", "Start over.\n\nStart: base"); err != nil {
			t.Fatal(err)
		}
		next(t, a, 2, base)
	})

	t.Run("before the kept run ended", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("BLOCKED: which version?\n"))
		a.pass()
		a.say(issue1, "jane", "v1.\n\nStart: base\n")
		a.runner.Script(1, "worker", half("BLOCKED: still unsure.\n"))
		a.pass()
		kept := a.keptOf()
		if r := a.result("1-worker-2"); !strings.Contains(r, "outcome blocked") {
			t.Fatalf("worker run 2:\n%s", r)
		}
		a.say(issue1, "jane", "v1, really.")
		next(t, a, 3, kept)
	})

	t.Run("a login not trusted", func(t *testing.T) {
		a, _, kept := blocked(t)
		a.say(issue1, "mallory", "Start: base\n")
		a.say(issue1, "jane", "v1.")
		next(t, a, 2, kept)
	})

	t.Run("the architect's answer", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		base := a.gitRunIn(a.state("git/app"), "rev-parse", "refs/heads/main")
		a.runner.Script(1, "worker", half("QUESTION: v1 or v1.0?\n"))
		a.pass()
		if _, ok := a.jobFile(1, "slices/1/kept"); !ok {
			t.Fatalf("no kept after the question:\n%s", a.result("1-worker-1"))
		}
		a.runner.Script(1, "architect", runner.Canned{Out: "v1; the half-done main.go is wrong.\n\nStart: base\n", Stdout: claudeResult})
		a.pass()
		next(t, a, 2, base)
	})

	t.Run("the architect's answer without the line", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", half("QUESTION: v1 or v1.0?\n"))
		a.pass()
		kept := a.keptOf()
		a.runner.Script(1, "architect", runner.Canned{Out: "v1.\n", Stdout: claudeResult})
		a.pass()
		next(t, a, 2, kept)
	})

	t.Run("a revised plan", func(t *testing.T) {
		a, base, _ := blocked(t)
		a.forge.Relabel(issue1, forge.Blocked, "")
		a.forge.SetLabel(issue1, forge.ProposalAccepted)
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nStart: base\n"})
		a.pass()
		a.forge.SetLabel(issue1, forge.PlanAccepted)
		next(t, a, 2, base)
	})
}

// With the slice's pull request open, `Start: base` leaves out only the
// kept commits not pushed (section 7.8, The start, 1): the run starts
// from the pull request's head, since the swarm never forces a push, and
// the kept commit stays in the mirror and in kept.
func TestStartBaseWithOpenPullKeepsHead(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	h1 := a.pulls(1)[0].HeadSHA
	a.changesRequested()
	a.runner.Script(1, "worker", half("BLOCKED: v2 or v2.0?\n"))
	a.pass()
	kept := a.keptOf()
	a.say(app1Pull, "jane", "v2.\n\nStart: base\n")
	var got seen
	a.runner.Script(1, "worker", look(&got, "v2"))
	a.pass()
	if got.branch != "swarm/1-1" || got.head != h1 {
		t.Fatalf("worker run 3 started on %q at %s, want swarm/1-1 at the head %s (kept %s)\n%s", got.branch, got.head, h1, kept, a.result("1-worker-3"))
	}
	if held := a.gitRunIn(a.state("git/app"), "cat-file", "-t", kept); held != "commit" {
		t.Errorf("the kept commit %s is gone from the mirror: %q", kept, held)
	}
}
