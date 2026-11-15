package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// notifying is the swarm with notify set to a command appending its
// stdin to a file, whose lines it returns.
func (a *applyWS) notifying() func() []string {
	a.t.Helper()
	log := filepath.Join(a.t.TempDir(), "notified")
	a.edit(func(c *declare.Config) { c.Notify = []string{"sh", "-c", `cat >>"$0"`, log} })
	return func() []string {
		b, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			a.t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	}
}

// The maintainer is told, not polled for (section 5, The maintainer's
// mention): the reconciler's note that blocks an issue mentions the
// first login of trusted that is not the swarm's, the blocked gate's note
// after it no one, and notify runs once for the issue now waiting, its
// one line naming it; a second pass runs it again never; the block
// answered and set again by a new note is a new item, notified again;
// the gate notes mention the maintainer.
func TestSwarmApplyNotifies(t *testing.T) {
	a := newApplyWS(t)
	notified := a.notifying()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which repository?\n"})
	a.pass()
	b, g := a.bodies(1), a.gateNotes(1)
	if len(b) != 2 || b[1] != mark("reconciler", 1)+"architect run 1 is blocked (at swarm:proposal-accepted). Answer, then remove swarm:blocked to resume.\n\ncc @jane" ||
		len(g) != 1 || strings.Contains(g[0], "@jane") {
		t.Fatalf("the notes: %q %q", b, g)
	}
	want := []string{"acme/swarm-app#1 waits for you: blocked: Drift check"}
	if got := notified(); !reflect.DeepEqual(got, want) || !strings.Contains(a.stdout.String(), "\nNotified: acme/swarm-app#1 waits for you: blocked: Drift check\n") {
		t.Fatalf("notified %q\n%s", got, a.stdout.String())
	}
	if _, line := a.waitingSet(); line != "Waiting for you: blocked #1 (architecture)" {
		t.Errorf("the note with its mention is not read as blocking: %q", line)
	}
	if !strings.Contains(g[0], ": architect run 1 is blocked (at swarm:proposal-accepted). Answer, then remove swarm:blocked to resume.\n") {
		t.Errorf("the blocked gate note does not name the note: %q", g[0])
	}

	a.pass()
	if got := notified(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a second pass notified again: %q", got)
	}

	// The maintainer's answer lifts the block (section 6, The maintainer's
	// word); the architect blocks again in the same pass, a new note.
	a.tick()
	a.say(issue1, "jane", "The app repository.")
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which branch?\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted", forge.Blocked, forge.Gated}) || len(a.runner.Launches()) != 2 {
		t.Fatalf("not lifted and blocked again: %v\n%s", got, a.stdout.String())
	}
	if got := notified(); !reflect.DeepEqual(got, append(want, want[0])) {
		t.Fatalf("a new block's note: %q", got)
	}

	// Unblocked by hand: the item leaves, and the plan gate is another.
	a.forge.Relabel(issue1, forge.Blocked, "")
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	gates := a.gateNotes(1)
	if g := gates[len(gates)-1]; !strings.Contains(g, "gated at plan-0-posted: ") || !strings.HasSuffix(g, ".\n\ncc @jane") {
		t.Errorf("the plan gate's note: %q", g)
	}
	if got := notified(); !reflect.DeepEqual(got, append(want, want[0], "acme/swarm-app#1 waits for you: plan: Drift check")) {
		t.Fatalf("the plan gate: %q", got)
	}
	if rec, _ := os.ReadFile(a.state("notified")); strings.Count(string(rec), "\n") != 1 {
		t.Errorf("the record keeps an item that left: %q", rec)
	}
}

// With no login in trusted but the swarm's, the notes mention no one.
func TestSwarmApplyMentionsNoOne(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which repository?\n"})
	a.pass()
	if b := a.bodies(1); len(b) != 2 || b[1] != mark("reconciler", 1)+"architect run 1 is blocked (at swarm:proposal-accepted). Answer, then remove swarm:blocked to resume." {
		t.Fatalf("the notes: %q", b)
	}
}

// A swarm fault's pause has no issue to comment on: notify alone tells it,
// once; a pause of the maintainer's tells nothing; a notify command that
// fails is a warning, its item not tried again.
func TestSwarmApplyNotifiesAFaultPause(t *testing.T) {
	a := newApplyWS(t)
	notified := a.notifying()
	a.collectedRun("8-architect-1", "launch-failed 2026-11-06T14:59:00Z", "clone: gone")
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Code: 125})
	a.pass()
	paused, _ := os.ReadFile(a.state("paused"))
	want := []string{"acme/swarm-app waits for you: " + strings.TrimSuffix(string(paused), "\n")}
	if got := notified(); !strings.Contains(string(paused), ": swarm fault: ") || !reflect.DeepEqual(got, want) {
		t.Fatalf("paused %q, notified %q", paused, got)
	}
	a.pass()
	if got := notified(); !reflect.DeepEqual(got, want) {
		t.Fatalf("notified again: %q", got)
	}
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T16:00:00Z\n"), 0o644) // the maintainer's
	a.pass()
	if got := notified(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the maintainer's pause notified: %q", got)
	}

	os.Remove(a.state("paused"))
	a.edit(func(c *declare.Config) { c.Notify = []string{"sh", "-c", "echo no route >&2; exit 3"} })
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which repository?\n"})
	a.pass()
	if w := "⚠ notify: acme/swarm-app#1 waits for you: blocked: Drift check: exit status 3: no route\n"; !strings.Contains(a.stderr.String(), w) {
		t.Fatalf("stderr:\n%s\nwant %q", a.stderr.String(), w)
	}
	a.pass()
	if strings.Contains(a.stderr.String(), "notify") {
		t.Errorf("a failed notify ran again:\n%s", a.stderr.String())
	}
}

// A notify command that outlives the swarm's timeout is stopped, a
// warning.
func TestSwarmApplyNotifyTimeout(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Notify = []string{"sleep", "60"} })
	a.after = func(d time.Duration) <-chan time.Time {
		c := make(chan time.Time, 1)
		if d == 45*time.Minute {
			c <- time.Time{}
		}
		return c
	}
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which repository?\n"})
	a.pass()
	if w := "⚠ notify: acme/swarm-app#1 waits for you: blocked: Drift check: stopped after 45m\n"; !strings.Contains(a.stderr.String(), w) {
		t.Fatalf("stderr:\n%s\nwant %q", a.stderr.String(), w)
	}
}
