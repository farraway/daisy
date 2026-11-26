package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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
	// The signal ends the loop without a drain (section 7.10, rule 2):
	// the next apply collects the run, the member having ended.
	for running, _ := a.runner.Running(); len(running) > 0; running, _ = a.runner.Running() {
		time.Sleep(time.Millisecond)
	}
	if err == nil {
		err = a.apply(a.git(), "--once")
	}
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
	// claudeCredits is claude's result for a model whose usage credits are
	// spent (Step SF-2), a capacity fault stating no reset.
	claudeCredits = runner.Canned{Code: 1, Stdout: `{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"api_error":"model_requires_usage_credits","num_turns":1,"result":"You're out of usage credits. Switch to another model to continue.","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0}}`}
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
	for _, tc := range []struct {
		name   string
		canned runner.Canned
		reason string
	}{
		{"a quota", claudeQuota, "Claude AI usage limit reached"},
		{"the usage credits", claudeCredits, "You're out of usage credits. Switch to another model to continue."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newApplyWS(t)
			a.withFallback("architect", opusEntry)
			a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
			a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", tc.canned, runner.Canned{Out: archPlan, Stdout: claudeResult})
			a.runner.Script(2, "architect", tc.canned, runner.Canned{Out: archPlan, Stdout: claudeResult})
			a.pass()
			for _, run := range []string{"1-architect-1", "2-architect-1"} {
				if r := a.result(run); !strings.Contains(r, "\npersona anthropic claude-fable-5-1 high\n") || !strings.HasSuffix(r, "fault: provider: "+tc.reason+"\nrecorded\noutcome fault\ncollected\n") {
					t.Fatalf("%s:\n%s", run, r)
				}
			}
			if b, err := os.ReadFile(a.state("capacity")); err != nil || string(b) != "anthropic/env claude-fable-5-1 until resume: "+tc.reason+"\n" {
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
		})
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

// withSlots gives openai the slots count, each logged in, and role's
// persona gpt-6.1-sol on openai taking only the slots listed (section 3,
// personas.<role>.slots).
func (a *applyWS) withSlots(count int, role string, slots ...int) {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		c.Keys["openai"] = declare.Key{Slots: &count}
		c.Personas[role] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>", Slots: slots}
	})
	for j := 1; j <= count; j++ {
		slot := filepath.Join(a.local, "swarm", "credentials", "openai", strconv.Itoa(j))
		os.MkdirAll(slot, 0o700)
		os.WriteFile(filepath.Join(slot, "auth.json"), []byte(`{"tokens":{"access_token":"x"}}`), 0o600)
	}
}

// A role takes only the slots its persona's slots lists (section 7.10,
// rule 3): the architect, given slot 2 of two, launches on slot 2 with
// slot 1 free; given slot 1 alone while another swarm's container holds
// it, it waits, slot 2 free, nothing launched or recorded.
func TestTakeSlotForARole(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withSlots(2, "architect", 2)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if l := a.runner.Launches(); len(l) != 1 || l[0].Credential != "openai/2" || !strings.Contains(a.result("1-architect-1"), "\ncredential openai slot 2\n") {
		t.Fatalf("launches %+v\n%s", l, a.stdout.String())
	}

	b := newApplyWS(t)
	b.withSlots(2, "architect", 1)
	b.runner.Others = []runner.Running{{Name: "daisy-other.x.2.worker.1", Swarm: "other/x#2/worker", Credential: "openai/1"}}
	b.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	b.pass()
	if l := b.runner.Launches(); len(l) != 0 || fileExists(b.state("runs/1-architect-1")) {
		t.Errorf("launched on a slot the persona does not list: %+v\n%s", l, b.stdout.String())
	}
}

// `status`'s Capacity: block (section 7.10, rule 3): a line per role, its
// queue (the issues it would launch on at this pass but for section 6's
// rule 4) and its running members, and what holds a queue that waits; a
// line per line of <state>/capacity; and the bottleneck, the role with
// the most issues waiting for a credential. The owner on slot 1 alone,
// which another swarm's container holds, waits for it; the architect's
// model is unavailable on its credential; with max_members' containers
// running, every queue waits for it.
func TestSwarmStatusCapacity(t *testing.T) {
	t.Parallel()
	s := newStatusWS(t)
	two := 2
	s.edit(func(c *declare.Config) {
		c.Trusted = []string{"jane", "swarm-bot"}
		c.Keys["openai"] = declare.Key{Slots: &two}
		c.Personas["owner"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>", Slots: []int{1}}
	})
	s.forge.CreateIssue("One", "x", []string{"swarm:proposal-posted"})
	s.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	s.forge.CreateIssue("Three", "x", []string{"swarm:proposal-accepted"})
	plan := s.ref(4)
	s.forge.CreateIssue("Four", "x", []string{"swarm:plan-0-posted"}) // the maintainer's turn: no queue
	s.forge.Comment(plan, archMark)
	os.MkdirAll(s.state(""), 0o755)
	os.WriteFile(s.state("capacity"), []byte("anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached\n"), 0o644)
	s.docker = "daisy-other-x-2-worker-1 other/x#2/worker openai/1\n"
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(statusPersonas, "owner        anthropic  claude-opus-5-5", "owner        openai     gpt-6.1-sol", 1) + `Capacity:
  owner        queue 1, running 0: no free slot of openai 1
  architect    queue 2, running 0: anthropic/env claude-fable-5-1 unavailable until resume
  worker       queue 0, running 0
  adversary    queue 0, running 0
  sympathetic  queue 0, running 0
  anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached
  Bottleneck: architect (anthropic/env claude-fable-5-1 unavailable until resume)
Throughput:
`
	if out := s.stdout.String(); !strings.Contains(out, want) {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}

	s.docker = "daisy-acme-app-9-worker-1 acme/swarm-app#9/worker \n" +
		"daisy-acme-app-8-worker-1 acme/swarm-app#8/worker \n" +
		"daisy-acme-app-7-worker-1 acme/swarm-app#7/worker \n"
	s.flags = []string{"--json"}
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(s.stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	want = `{"bottleneck":{"role":"architect","why":"max_members reached"},"roles":[` +
		`{"queue":[1],"role":"owner","running":0,"why":"max_members reached"},{"queue":[2,3],"role":"architect","running":0,"why":"max_members reached"},` +
		`{"queue":[],"role":"worker","running":3,"why":null},{"queue":[],"role":"adversary","running":0,"why":null},{"queue":[],"role":"sympathetic","running":0,"why":null}],"tokens":[],` +
		`"unavailable":["anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached"]}`
	var v any
	json.Unmarshal(doc["capacity"], &v)
	if got, _ := json.Marshal(v); string(got) != want {
		t.Errorf("capacity:\n%s\nwant\n%s", got, want)
	}

	// Paused, the pause holds every waiting queue (the maintainer's
	// ruling of 2026-11-26), before max_members.
	os.WriteFile(s.state("paused"), []byte("paused 2026-11-06T10:00:00Z\n"), 0o644)
	s.flags = nil
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{"  owner        queue 1, running 0: swarm paused\n", "  architect    queue 2, running 0: swarm paused\n", "  Bottleneck: architect (swarm paused)\n"} {
		if !strings.Contains(s.stdout.String(), l) {
			t.Errorf("paused: no %q in\n%s", l, s.stdout.String())
		}
	}
}

// A credential is paced to its provider's token budget (section 7.10,
// rule 4): openai's slot 1 has taken 1100 tokens in a run started two
// hours ago, over per_day's 1000, and 2100 in the last 7 days with a run
// six days old, over per_week's 2000. The architect on it alone waits,
// status naming the budget's time, the later of the run leaving the day
// (in 22 hours) and the older leaving the week (in 24); with a fallback
// it launches there; 23 hours on, the day's budget free and the week's
// not, it is still passed over; 25 hours on, it launches on the slot.
func TestSwarmPacedByATokenBudget(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withSlots(1, "architect")
	day, week := 1000, 2000
	a.edit(func(c *declare.Config) {
		k := c.Keys["openai"]
		k.Tokens = &declare.Tokens{PerDay: &day, PerWeek: &week}
		c.Keys["openai"] = k
	})
	now := a.Now()
	ran := func(name string, started time.Time, cost string) {
		os.MkdirAll(a.state("runs/"+name), 0o755)
		os.WriteFile(a.state("runs/"+name+"/result"), []byte("label swarm:plan-1-accepted\nstarted "+started.Format(time.RFC3339)+
			"\ncredential openai slot 1\npersona openai gpt-6.1-sol high\nexited 0 "+started.Add(time.Minute).Format(time.RFC3339)+"\n"+cost+"\noutcome posted\ncollected\n"), 0o644)
	}
	ran("9-worker-1", now.Add(-6*24*time.Hour), "cost 0.1 tokens 900/100 turns 1")
	ran("9-worker-2", now.Add(-2*time.Hour), "cost 0.1 tokens 800/300 turns 1 cached 500")
	ran("9-worker-3", now.Add(-time.Hour), "launch-failed") // no cost line: nothing counted
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if l := a.runner.Launches(); len(l) != 0 || fileExists(a.state("runs/1-architect-1")) {
		t.Fatalf("launched over the budget: %+v\n%s", l, a.stdout.String())
	}
	until := now.Add(24 * time.Hour).Format(time.RFC3339)
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "  architect    queue 1, running 0: openai/1 at its token budget until "+until+"\n") ||
		!strings.Contains(out, "  openai/1 tokens 1100 of 1000 per day, 2100 of 2000 per week\n") {
		t.Errorf("status:\n%s", out)
	}

	a.withFallback("architect", opusEntry)
	a.pass()
	if l := a.runner.Launches(); len(l) != 1 || model(l[0]) != "claude-opus-5-5" {
		t.Fatalf("the fallback: %+v\n%s", l, a.stdout.String())
	}
	a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	a.mu.Lock()
	a.now = now.Add(23 * time.Hour)
	a.mu.Unlock()
	a.pass()
	if l := a.runner.Launches(); len(l) != 2 || model(l[1]) != "claude-opus-5-5" {
		t.Fatalf("23 hours on, the week's budget reached: %+v\n%s", l, a.stdout.String())
	}
	a.forge.CreateIssue("Three", "x", []string{"swarm:proposal-accepted"})
	a.mu.Lock()
	a.now = now.Add(25 * time.Hour)
	a.mu.Unlock()
	a.pass()
	if l := a.runner.Launches(); len(l) != 3 || model(l[2]) != "gpt-6.1-sol" || l[2].Credential != "openai/1" {
		t.Errorf("25 hours on: %+v\n%s", l, a.stdout.String())
	}
}

// A launch takes a free slot under its budget (section 7.10, rule 4): with
// slot 1 at its day's budget and slot 2 under it, the run is on slot 2.
func TestSwarmPacedTakesTheSlotUnderBudget(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withSlots(2, "architect")
	day := 1000
	a.edit(func(c *declare.Config) {
		k := c.Keys["openai"]
		k.Tokens = &declare.Tokens{PerDay: &day}
		c.Keys["openai"] = k
	})
	os.MkdirAll(a.state("runs/9-worker-1"), 0o755)
	os.WriteFile(a.state("runs/9-worker-1/result"), []byte("label swarm:plan-1-accepted\nstarted "+a.Now().Add(-time.Hour).Format(time.RFC3339)+
		"\ncredential openai slot 1\npersona openai gpt-6.1-sol high\nexited 0 "+a.Now().Format(time.RFC3339)+"\ncost 0.1 tokens 900/200 turns 1\noutcome posted\ncollected\n"), 0o644)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if l := a.runner.Launches(); len(l) != 1 || l[0].Credential != "openai/2" {
		t.Fatalf("launches %+v\n%s", l, a.stdout.String())
	}
}

// Every Task text names the run's deadline, the one its record holds,
// and /job/io/deadline (section 7.10, rule 5), in its closing paragraph.
func TestTaskTextNamesTheDeadline(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	r := a.result("1-architect-1")
	started, deadline := line(r, "started "), line(r, "deadline ")
	if started == "" || deadline == "" {
		t.Fatalf("result:\n%s", r)
	}
	if s, d := mustTime(t, started), mustTime(t, deadline); d.Sub(s) != 45*time.Minute {
		t.Errorf("deadline %s, started %s", deadline, started)
	}
	system, _ := os.ReadFile(a.state("runs/1-architect-1/io/system.md"))
	want := "Your run is stopped at " + deadline + " (45m after its start). If /job/io/deadline appears, 80% of that time is used: write your output now, or, if the work cannot finish in the time left, write what you have and propose how to split it.\n\n"
	if !strings.Contains(string(system), want) {
		t.Errorf("system.md lacks %q:\n%s", want, system)
	}
}

// line is the value of r's first line beginning prefix, "" for none.
func line(r, prefix string) string {
	for _, l := range strings.Split(r, "\n") {
		if v, ok := strings.CutPrefix(l, prefix); ok {
			return v
		}
	}
	return ""
}

// At 80% of its timeout the process holding a run writes <run>/io/deadline
// for the member and records `warned <time>` (section 7.10, rule 5); a
// run that ends before it is not warned.
func TestSwarmDeadlineWarnedAt80Percent(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Warned: true})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	r := a.result("1-architect-1")
	deadline := line(r, "deadline ")
	b, err := os.ReadFile(a.state("runs/1-architect-1/io/deadline"))
	want := "80% of your time is used (36m0s of 45m); you are stopped at " + deadline + ". Write your output now; if the work cannot finish, write what you have and propose how to split it.\n"
	if err != nil || string(b) != want {
		t.Errorf("io/deadline %q %v, want %q", b, err, want)
	}
	if !strings.Contains(r, "\nwarned "+a.Now().Format(time.RFC3339)+"\n") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
	if fileExists(a.state("runs/2-architect-1/io/deadline")) || strings.Contains(a.result("2-architect-1"), "warned") {
		t.Errorf("#2 warned:\n%s", a.result("2-architect-1"))
	}
}

// A run stopped at its deadline has the output it had written posted
// with its failure, after the secret check, never read as the role's
// text (section 7.10, rule 5): #1's half plan ends the T17 note, no plan
// written; #2's, holding the credential it was given, is not posted.
func TestSwarmTimeoutPostsItsPartialOutput(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("One", "x", []string{"swarm:proposal-accepted"})
	a.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: "## Research\n\nHalf of it.\n", TimedOut: true})
	a.runner.Script(2, "architect", runner.Canned{Out: "## Research\n\nThe token is " + a.vars["CLAUDE_CODE_OAUTH_TOKEN"] + ".\n", TimedOut: true})
	a.pass()
	want := "<!-- daisy-swarm role=reconciler run=1 -->\n**reconciler:**\n\narchitect run 1 failed: timeout after 45m (at swarm:proposal-accepted).\n\nIts output when it was stopped:\n\n## Research\n\nHalf of it.\n\nRemove swarm:blocked to resume.\n\ncc @jane"
	if b := a.bodies(1); len(b) == 0 || b[len(b)-1] != want {
		t.Errorf("#1's note:\n%q\nwant\n%q", b, want)
	}
	if fileExists(a.state("jobs/1/plan.md")) || !strings.Contains(a.result("1-architect-1"), "failed: timeout after 45m\n") {
		t.Errorf("#1 read as a plan, or not a timeout:\n%s", a.result("1-architect-1"))
	}
	want = "<!-- daisy-swarm role=reconciler run=1 -->\n**reconciler:**\n\narchitect run 1 failed: timeout after 45m (at swarm:proposal-accepted). Its output when it was stopped held a secret the member was given and is not posted. Remove swarm:blocked to resume.\n\ncc @jane"
	if b := a.bodies(2); len(b) == 0 || b[len(b)-1] != want || strings.Contains(strings.Join(b, "\n"), a.vars["CLAUDE_CODE_OAUTH_TOKEN"]) {
		t.Errorf("#2's note:\n%q", b)
	}
}
