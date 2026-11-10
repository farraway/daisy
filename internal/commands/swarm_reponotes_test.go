package commands

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"daisy/internal/swarm/runner"
)

// notesOf is the `# Repository notes: <name>` sections of a system.md, by
// their heading lines, in order.
func notesOf(sys string) []string {
	var out []string
	for _, l := range strings.Split(sys, "\n") {
		if strings.HasPrefix(l, "# Repository notes: ") {
			out = append(out, l)
		}
	}
	return out
}

// The claim of revision 6's plan, Step 5, its first half (section 13,
// Step 5's row; section 4, The swarm's notes about a repository): a
// worker in lib gets repos/lib.md and not repos/app.md, as pushed (an
// edit in the clone not pushed reaches no member), under `# Repository
// notes: lib` after `# Task`, its headings shifted; its reviewers get
// lib's; the architect and the owner every declared repository's, in
// repos' order; a file absent at the pass's commit, or a link there, adds
// nothing.
func TestSwarmRepositoryNotes(t *testing.T) {
	a := newApplyWSIn(t, true)
	a.write("repos/app.md", "# App\n\nApp's notes.\n")
	a.write("repos/lib.md", "# Lib\n\nLib's notes.\n\n## Testing\n\nRun go test.\n")
	a.publish("notes")
	a.write("repos/lib.md", "# Lib\n\nUNPUSHED\n")

	if _, err := a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"}); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "## Steps\n\n1. lib: add the check.\n"})
	a.pass()
	sys := a.file("1-architect-1", "io/system.md")
	if got := notesOf(sys); strings.Join(got, "|") != "# Repository notes: app|# Repository notes: lib" {
		t.Fatalf("architect's notes %q:\n%s\nstderr:\n%s", got, sys, a.stderr.String())
	}
	task := strings.Index(sys, "\n# Task\n")
	app := strings.Index(sys, "\n# Repository notes: app\n\n## App\n\nApp's notes.\n\n# Repository notes: lib\n\n## Lib\n\nLib's notes.\n\n### Testing\n\nRun go test.\n")
	if task < 0 || app < task || !strings.HasSuffix(sys, "Run go test.\n") || strings.Contains(sys, "UNPUSHED") {
		t.Errorf("architect's system.md:\n%s", sys)
	}

	a.forge.SetLabel(a.ref(1), "swarm:plan-1-accepted")
	a.runner.Script(1, "worker", work("lib v1"))
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	sys = a.file("1-worker-1", "io/system.md")
	if got := notesOf(sys); strings.Join(got, "|") != "# Repository notes: lib" || strings.Contains(sys, "App's notes") || strings.Contains(sys, "UNPUSHED") ||
		!strings.Contains(sys, "in acme/lib, on the branch checked out in /work (swarm/1-1)") {
		t.Fatalf("worker's notes %q:\n%s\nstderr:\n%s", got, sys, a.stderr.String())
	}
	a.pass()
	for _, role := range []string{"adversary", "sympathetic"} {
		if sys := a.file("1-"+role+"-1", "io/system.md"); strings.Join(notesOf(sys), "|") != "# Repository notes: lib" {
			t.Errorf("%s's notes %q:\n%s", role, notesOf(sys), sys)
		}
	}

	// app's notes removed and lib's a link at the pass's commit: the
	// owner's batch reads neither.
	os.Remove(filepath.Join(a.ws, "repos/app.md"))
	os.Remove(filepath.Join(a.ws, "repos/lib.md"))
	os.Symlink("../README.md", filepath.Join(a.ws, "repos/lib.md"))
	a.release()
	a.publish("no notes")
	a.runner.Script(0, "owner", runner.Canned{Out: "BLOCKED: nothing to propose\n"})
	a.pass()
	if sys := a.file("0-owner-1", "io/system.md"); sys == "" || len(notesOf(sys)) != 0 {
		t.Errorf("owner's batch with no notes file:\n%s\nstderr:\n%s", sys, a.stderr.String())
	}
	os.Remove(filepath.Join(a.ws, "repos/lib.md"))
	a.write("repos/lib.md", "# Lib\n\nBack.\n")
	a.publish("lib's notes again")
	os.Remove(a.state("owner-held"))
	a.runner.Script(0, "owner", runner.Canned{Out: "BLOCKED: nothing to propose\n"})
	a.pass()
	if sys := a.file("0-owner-2", "io/system.md"); strings.Join(notesOf(sys), "|") != "# Repository notes: lib" || !strings.HasSuffix(sys, "\n## Lib\n\nBack.\n") {
		t.Errorf("owner's batch with lib's notes:\n%s", sys)
	}
}

// The old layout has no repos/<name>.md: nothing is read for a member,
// even a file at that path in the workspace, and init seeds none.
func TestSwarmRepositoryNotesOldLayout(t *testing.T) {
	a := newApplyWS(t)
	if _, err := os.Stat(filepath.Join(a.ws, "repos")); err == nil || strings.Contains(a.stdout.String(), "repos/") {
		t.Errorf("init wrote repos/ in the old layout")
	}
	a.write("repos/app.md", "# App\n\nApp's notes.\n")
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if sys := a.file("1-architect-1", "io/system.md"); sys == "" || len(notesOf(sys)) != 0 {
		t.Errorf("old layout's architect:\n%s", sys)
	}
}

// init seeds repos/<name>.md per declared repository when absent
// (section 4; section 13, Step 5's row): the heading, what the file is
// for, and the readiness facts, the product's instruction files read in
// its mirror at base, its check and protected paths as swarm.json
// declares them; a second run keeps every one, a maintainer's edit
// included.
func TestSwarmInitSeedsRepositoryNotes(t *testing.T) {
	s := newSwarmWSIn(t, true)
	pushFiles(t, s.upstreams["git@forge.example.net:acme/app.git"], map[string]string{"AGENTS.md": "# App\n", "CLAUDE.md": "# App\n"})
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if !strings.Contains(s.stdout.String(), "  Kept prompts/sympathetic.md (exists)\n✓ Wrote repos/app.md\n✓ Wrote repos/lib.md\n✓ Mirrors: ") &&
		!strings.Contains(s.stdout.String(), "✓ Wrote prompts/sympathetic.md\n✓ Wrote repos/app.md\n✓ Wrote repos/lib.md\n✓ Mirrors: ") {
		t.Errorf("init stdout:\n%s", s.stdout.String())
	}
	protected := "- Protected: `.github/`, `.gitea/`, `.forgejo/`, `.claude/`, `.grok/`, `AGENTS.md`, `CLAUDE.md`, as swarm.json declares them.\n"
	head := func(name string) string {
		return "# " + name + "\n\nThe swarm's notes about acme/" + name + ". Daisy adds this file to the system prompt of every member working in " + name +
			", under `# Repository notes: " + name + "`; the repository itself carries no swarm material. Replace what follows with what a member should know about it.\n\nWhat `daisy swarm init` found:\n\n"
	}
	want := map[string]string{
		"app": head("app") + "- Instructions: AGENTS.md and CLAUDE.md on main.\n- Check: none declared in swarm.json; no member's work is executed.\n" + protected,
		"lib": head("lib") + "- Instructions: no AGENTS.md or CLAUDE.md on main; members start without the product's instructions.\n- Check: none declared in swarm.json; no member's work is executed.\n" + protected,
	}
	for name, w := range want {
		if got := s.read("repos/" + name + ".md"); got != w {
			t.Errorf("repos/%s.md:\n%s\nwant:\n%s", name, got, w)
		}
	}

	// A check declared: the seed names it, shell-quoted; an existing
	// file, edited or not, is kept.
	s.write("repos/app.md", "# App\n\nMine.\n")
	os.Remove(filepath.Join(s.ws, "repos/lib.md"))
	cfg := s.read("swarm.json")
	s.write("swarm.json", strings.Replace(cfg, `"name": "lib", "repo": "acme/lib", "check": []`, `"name": "lib", "repo": "acme/lib", "check": ["go", "test", "./a b"]`, 1))
	if s.read("swarm.json") == cfg {
		t.Fatalf("swarm.json not edited:\n%s", cfg)
	}
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if !strings.Contains(s.stdout.String(), "  Kept repos/app.md (exists)\n✓ Wrote repos/lib.md\n") || s.read("repos/app.md") != "# App\n\nMine.\n" ||
		!strings.Contains(s.read("repos/lib.md"), "\n- Check: `go test './a b'`, as swarm.json declares it.\n") {
		t.Errorf("second init:\n%s\nrepos/lib.md:\n%s", s.stdout.String(), s.read("repos/lib.md"))
	}
}

// The claim of revision 6's plan, Step 5, its second half (section 11,
// Drift; section 13, Step 5's row): status prints a line per drifting
// path of swarm.json and prompts/ each time, a file git ignores never;
// apply's loop prints them at its first pass and again only when the set
// changes, then, once a commit and a push clear it, `✓ Drift cleared`
// once; nothing changes an exit code.
func TestSwarmDrift(t *testing.T) {
	a := newApplyWSIn(t, true)
	if err := a.status(a.forge); err != nil || strings.Contains(a.stderr.String(), "Drift") {
		t.Fatalf("status with nothing drifting: %v\n%s", err, a.stderr.String())
	}
	a.write("prompts/old.md", "Old.\n")
	a.publish("old")
	a.pass()
	if strings.Contains(a.stderr.String(), "Drift") || strings.Contains(a.stdout.String(), "Drift") {
		t.Fatalf("a pass with nothing drifting:\n%s%s", a.stdout.String(), a.stderr.String())
	}
	head := gitIn(t, a.upstreams[swarmURL], "rev-parse", "--short=7", "main")

	os.Remove(filepath.Join(a.ws, "prompts/old.md"))
	a.write("prompts/worker.md", a.read("prompts/worker.md")+"\nAnd test first.\n")
	a.write("prompts/new.md", "New.\n")
	a.write(".gitignore", "*.swp\n")
	a.write("prompts/.worker.md.swp", "x")
	at := "acme/swarm-app's main at " + head + "; commit and push it\n"
	drift := "⚠ Drift: prompts/new.md is not in " + at + "⚠ Drift: prompts/old.md is only in " + at + "⚠ Drift: prompts/worker.md differs from " + at
	for i := range 2 {
		if err := a.status(a.forge); err != nil || a.stderr.String() != drift {
			t.Errorf("status %d: %v\n%s\nwant:\n%s", i, err, a.stderr.String(), drift)
		}
	}

	// The loop: pass 1 drifting, pass 2 the same, pass 3 one more path,
	// pass 4 after the commit and the push, pass 5 still clear.
	a.signals = make(chan os.Signal, 1)
	waits := 0
	var mu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		waits++
		switch waits {
		case 2:
			a.write("swarm.json", a.read("swarm.json")+"\n")
		case 3:
			a.publish("the configuration")
		case 5:
			a.signals <- os.Interrupt
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	head2 := gitIn(t, a.upstreams[swarmURL], "rev-parse", "--short=7", "main")
	if want := drift + strings.Replace(drift, "prompts/worker.md differs from "+at, "prompts/worker.md differs from "+at+"⚠ Drift: swarm.json differs from "+at, 1); a.stderr.String() != want {
		t.Errorf("apply's stderr:\n%s\nwant:\n%s", a.stderr.String(), want)
	}
	cleared := "✓ Drift cleared: swarm.json and prompts/ are acme/swarm-app's main at " + head2 + "\n"
	if out := a.stdout.String(); strings.Count(out, "Pass ") != 5 || strings.Count(out, "Drift") != 1 || !strings.Contains(out, cleared) ||
		strings.Index(out, cleared) < nthIndex(out, "Pass ", 3) || strings.Index(out, cleared) > nthIndex(out, "Pass ", 4) {
		t.Errorf("apply's stdout:\n%s", out)
	}
	if err := a.status(a.forge); err != nil || strings.Contains(a.stderr.String(), "Drift") {
		t.Errorf("status once pushed: %v\n%s", err, a.stderr.String())
	}
}

// nthIndex is the index of the n-th (from 1) s in text, -1 when fewer.
func nthIndex(text, s string, n int) int {
	at := -1
	for range n {
		i := strings.Index(text[at+1:], s)
		if i < 0 {
			return -1
		}
		at += 1 + i
	}
	return at
}
