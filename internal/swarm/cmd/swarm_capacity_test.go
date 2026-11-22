package cmd

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// The collection reads a run's output with the adapter of its own
// persona line, never of the role's persona as swarm.json has it by then
// (section 7.10, The fallback): #1's architect, launched on anthropic, is
// moved to openai in swarm.json while it runs; its collection, at the
// drain after the signal, reads claude's result, its cost line written.
func TestSwarmCollectsWithTheLaunchedPersona(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.signals = make(chan os.Signal, 2)
	a.forge.CreateIssue("Long", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	gate := make(chan struct{})
	waits := 0
	a.after = func(time.Duration) <-chan time.Time {
		waits++
		a.tick()
		if waits == 1 {
			a.edit(func(c *declare.Config) {
				c.Personas["architect"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>"}
			})
			c := make(chan time.Time, 1)
			c <- time.Time{}
			return c
		}
		close(gate) // the member ends after the second pass, which read the edit
		a.signals <- os.Interrupt
		return make(chan time.Time)
	}
	err := a.applyTo(a.git(), held{a.runner, map[string]chan struct{}{"daisy-swarm.acme.swarm-app.1.architect.1": gate}}, &a.stdout, &a.stderr)
	r := a.result("1-architect-1")
	if err != nil || waits != 2 || !strings.Contains(r, "\npersona anthropic claude-fable-5-1 high\n") ||
		!strings.Contains(r, "\ncost 0.25 tokens 10/20 turns 4 cached 0\n") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("%v, %d waits:\n%s\nstdout:\n%s", err, waits, r, a.stdout.String())
	}
}

// withFallback gives role's persona the fallback entries (section 3,
// personas.<role>.fallback).
func (a *applyWS) withFallback(role string, entries ...declare.Entry) {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		p := c.Personas[role]
		p.Fallback = entries
		c.Personas[role] = p
	})
}

var (
	opusEntry = declare.Entry{Provider: "anthropic", Model: "claude-opus-5-5", Effort: "high", CoAuthor: "claude-opus-5-5 <noreply@anthropic.com>"}
	solEntry  = declare.Entry{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>"}
	// claudeQuota is claude's result for a spent quota, a capacity fault
	// stating no reset (section 9).
	claudeQuota = runner.Canned{Code: 1, Stdout: `{"type":"result","is_error":true,"result":"Claude AI usage limit reached"}`}
	// codexLimit is codex's usage limit as the live journal holds it
	// (2026-11-19), stating its reset.
	codexLimit = runner.Canned{Code: 1, Stdout: `{"type":"turn.failed","error":{"message":"You’ve hit your usage limit. Visit https://chatgpt.com/settings/usage to purchase more credits or try again at 3:53 PM."}}` + "\n"}
)

// model is the --model a launch's argv names.
func model(s runner.Spec) string {
	i := slices.Index(s.Argv, "--model")
	if i < 0 || i+1 >= len(s.Argv) {
		return ""
	}
	return s.Argv[i+1]
}

// A capacity fault (section 7.10, The fallback) writes its line to
// <state>/capacity, `<provider>/<credential> <model> until <time>:
// <reason>`, `resume` for a message stating no reset, the line for a
// credential and model written once; the role is launched again on its
// next entry, its record naming it, and the faults pause nothing, two in
// a row included (7.7).
func TestSwarmFallbackOnACapacityFault(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withFallback("architect", opusEntry)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", claudeQuota, runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.runner.Script(2, "architect", claudeQuota, runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass()
	for _, run := range []string{"1-architect-1", "2-architect-1"} {
		if r := a.result(run); !strings.Contains(r, "\npersona anthropic claude-fable-5-1 high\n") || !strings.HasSuffix(r, "fault: provider: Claude AI usage limit reached\nrecorded\noutcome fault\ncollected\n") {
			t.Fatalf("%s:\n%s", run, r)
		}
	}
	if b, err := os.ReadFile(a.state("capacity")); err != nil || string(b) != "anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached\n" {
		t.Fatalf("capacity %q %v", b, err)
	}
	if _, err := os.Stat(a.state("paused")); err == nil {
		t.Fatal("two capacity faults in a row paused the swarm")
	}
	a.pass()
	launches := a.runner.Launches()
	if len(launches) != 4 || model(launches[2]) != "claude-opus-5-5" || model(launches[3]) != "claude-opus-5-5" {
		t.Fatalf("%d launches, %v\n%s", len(launches), launches, a.stdout.String())
	}
	if r := a.result("1-architect-2"); !strings.Contains(r, "\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\npersona anthropic claude-opus-5-5 high\n") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("run 2:\n%s", r)
	}
	if b := a.bodies(1); len(b) == 0 || b[len(b)-1] != mark("architect", 2)+archPlan {
		t.Errorf("#1's plan: %q", b)
	}
}

// A capacity fault's line holds until the reset its message states
// (section 7.10, The fallback): codex's "try again at 3:53 PM", read in
// UTC, leaves the role with no other entry waiting, nothing launched or
// posted for it, through the pass at 15:53; the first pass after it
// removes the line, says so, and launches the role on the model again.
func TestSwarmCapacityUntilTheReset(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Personas["architect"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>"}
	})
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", codexLimit, runner.Canned{Out: archPlan})
	a.pass()
	want := "openai/1 gpt-6.1-sol until 2026-11-06T15:53:00Z: You’ve hit your usage limit. Visit https://chatgpt.com/settings/usage to purchase more credits or try again at 3:53 PM.\n"
	if b, err := os.ReadFile(a.state("capacity")); err != nil || string(b) != want {
		t.Fatalf("capacity %q %v\n%s", b, err, a.result("1-architect-1"))
	}
	before := a.bodies(1)
	a.mu.Lock()
	a.now = time.Date(2026, 11, 6, 15, 52, 0, 0, time.UTC)
	a.mu.Unlock()
	a.pass() // 15:53
	if len(a.runner.Launches()) != 1 || fileExists(a.state("runs/1-architect-2")) || !reflect.DeepEqual(a.bodies(1), before) {
		t.Fatalf("launched before the reset:\n%s", a.stdout.String())
	}
	if b, _ := os.ReadFile(a.state("capacity")); string(b) != want {
		t.Fatalf("capacity at the reset's minute %q", b)
	}
	a.pass() // 15:54
	if b, err := os.ReadFile(a.state("capacity")); err == nil {
		t.Errorf("capacity after the reset %q", b)
	}
	if out := a.stdout.String(); !strings.HasPrefix(out, "Capacity: openai/1 gpt-6.1-sol available again\n") || !strings.Contains(out, "#1 architect run 2: launched\n") ||
		len(a.runner.Launches()) != 2 || model(a.runner.Launches()[1]) != "gpt-6.1-sol" {
		t.Errorf("after the reset:\n%s", out)
	}
}

// The bare `swarm resume` removes <state>/capacity, so a model whose
// fault stated no reset is tried again at the next pass (section 11,
// pause and resume; section 7.10).
func TestSwarmResumeClearsCapacity(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", claudeQuota, runner.Canned{Out: archPlan})
	a.pass()
	a.mu.Lock()
	a.now = a.now.Add(72 * time.Hour)
	a.mu.Unlock()
	a.pass()
	if len(a.runner.Launches()) != 1 || !fileExists(a.state("capacity")) {
		t.Fatalf("launched with no model available, or the line gone, three days on:\n%s", a.stdout.String())
	}
	if err := a.verb(SwarmResume); err != nil || a.stdout.String() != "Owner batch released\nCapacity released: every model is tried again at the next pass\n" {
		t.Fatalf("resume: %v %q", err, a.stdout.String())
	}
	if fileExists(a.state("capacity")) {
		t.Fatal("capacity kept by resume")
	}
	a.pass()
	if l := a.runner.Launches(); len(l) != 2 || model(l[1]) != "claude-fable-5-1" || !strings.HasSuffix(a.result("1-architect-2"), "outcome posted\ncollected\n") {
		t.Errorf("after resume:\n%s", a.stdout.String())
	}
}

// With no entry on a free credential whose model is available, the role
// waits (section 7.10, The fallback; section 6, rule 4): nothing is
// launched or posted for it, no run is recorded, and the pass goes on
// with the other roles, the worker beside it on the same credential with
// another model.
func TestSwarmRoleWaitsWithNoModelAvailable(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice) // #1: the architect's plan accepted, the worker's turn
	a.withFallback("architect", opusEntry, solEntry)
	a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	os.WriteFile(a.state("capacity"), []byte("anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached\n"+
		"anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached\n"+
		"openai/1 gpt-6.1-sol until 2026-11-07T15:53:00Z: try again at 3:53 PM\n"), 0o644)
	a.runner.Script(1, "worker", work("v1"))
	before := a.bodies(2)
	a.pass()
	out := a.stdout.String()
	if strings.Contains(out, "#2 architect") || len(a.launched("architect")) != 1 || fileExists(a.state("runs/2-architect-1")) || !reflect.DeepEqual(a.bodies(2), before) {
		t.Errorf("#2's architect launched or posted:\n%s", out)
	}
	if w := a.launched("worker"); len(w) != 1 || !strings.Contains(out, "#1 worker run 1: launched\n") || model(w[0]) != "claude-sonnet-5-5" {
		t.Errorf("the worker beside it:\n%s", out)
	}
}

// capacityNotes is issue n's capacity notes, in order.
func (a *applyWS) capacityNotes(n int) []string {
	var out []string
	for _, b := range a.bodies(n) {
		if text, ok := strings.CutPrefix(b, "<!-- daisy-swarm role=reconciler kind=capacity -->\n"); ok {
			out = append(out, text)
		}
	}
	return out
}

// A run on an issue launched on another entry than the role's newest run
// there gets one kind=capacity note, naming the pairs unavailable, and
// one more when it is back on the persona (section 7.10, The note); a
// run on the same entry as the one before it gets none. The note is no
// counted entry: no prompt shows it.
func TestSwarmCapacityNoteOnTheIssue(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withFallback("architect", opusEntry)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	blocked := runner.Canned{Out: "BLOCKED: which repository?\n"}
	a.runner.Script(1, "architect", claudeQuota, blocked, blocked, runner.Canned{Out: archPlan})
	a.pass()
	if notes := a.capacityNotes(1); len(notes) != 0 {
		t.Fatalf("a note at the persona's own run: %q", notes)
	}
	a.pass()
	in := "**reconciler:** the architect runs on anthropic claude-opus-5-5 (high) in place of anthropic claude-fable-5-1 (high): anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached."
	if notes := a.capacityNotes(1); !reflect.DeepEqual(notes, []string{in}) || !strings.Contains(a.stdout.String(), "#1 capacity note posted\n") {
		t.Fatalf("notes %q\n%s", notes, a.stdout.String())
	}
	// jane's answer wakes the architect, on opus still: no second note,
	// and its prompt shows none.
	a.tick()
	a.say(issue1, "jane", "The app repository.")
	a.pass()
	if notes := a.capacityNotes(1); len(notes) != 1 || !strings.Contains(a.result("1-architect-3"), "\npersona anthropic claude-opus-5-5 high\n") {
		t.Fatalf("notes %q\n%s", notes, a.stdout.String())
	}
	if p := a.file("1-architect-3", "io/prompt.md"); !strings.Contains(p, "The app repository.") || strings.Contains(p, "runs on anthropic") {
		t.Errorf("the prompt:\n%s", p)
	}
	a.resume()
	a.tick()
	a.say(issue1, "jane", "The app repository, I said.")
	a.pass()
	again := "**reconciler:** the architect runs on anthropic claude-fable-5-1 (high) again."
	if notes := a.capacityNotes(1); !reflect.DeepEqual(notes, []string{in, again}) || !strings.HasSuffix(a.result("1-architect-4"), "outcome posted\ncollected\n") {
		t.Errorf("notes %q\n%s", notes, a.stdout.String())
	}
}

// Notify tells each line of <state>/capacity once, known by the line
// (section 5, Notify; section 7.10): told at the pass that wrote it, not
// again while it holds, and dropped from <state>/notified once resume
// removes it.
func TestSwarmNotifiesCapacity(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	notified := a.notifying()
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", claudeQuota, runner.Canned{Out: archPlan})
	a.pass()
	want := []string{"acme/swarm-app capacity: anthropic/env claude-fable-5-1 unavailable until resume: Claude AI usage limit reached"}
	if got := notified(); !reflect.DeepEqual(got, want) || !strings.Contains(a.stdout.String(), "\nNotified: "+want[0]+"\n") {
		t.Fatalf("notified %q\n%s", got, a.stdout.String())
	}
	a.pass()
	if got := notified(); !reflect.DeepEqual(got, want) {
		t.Errorf("told again: %q", got)
	}
	a.resume()
	a.pass()
	if b, _ := os.ReadFile(a.state("notified")); strings.Contains(string(b), "capacity") || strings.Contains(string(b), "claude-fable-5-1") {
		t.Errorf("the record keeps the line: %q", b)
	}
}
