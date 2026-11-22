package cmd

import (
	"os"
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
