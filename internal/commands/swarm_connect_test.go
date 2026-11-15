package commands

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/runner"
)

// connect runs a command of the connect's briefing, as the cli does, from
// cwd: $DAISY_ROOT and $DAISY_LOCAL the scratch machine's, home the
// workspace's home, values the value flags.
func (a *applyWS) connect(fn func(*Context) error, cwd, home string, values ...Value) error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{
		NowFn:   a.Now,
		ExecFn:  a.exec,
		GetwdFn: func() (string, error) { return cwd, nil },
		GetenvFn: func(k string) string {
			return map[string]string{"DAISY_ROOT": a.root, "DAISY_LOCAL": a.local}[k]
		},
	}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Values: values}
	c.Env.Home = home
	return fn(c)
}

// snapshot is every file under the directories, by path: its mode and bytes.
func snapshot(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, d := range dirs {
		filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				t.Fatal(err)
			}
			fi, err := os.Lstat(p)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(p)
			files[p] = fi.Mode().String() + " " + string(b)
			return nil
		})
	}
	return files
}

// Step 15's claim (plan for the inbox requests F27-F35; docs/swarm.md
// section 11, swarm list and swarm journal): the briefing's commands,
// `swarm list` from outside any clone and from inside one, `swarm
// journal`, `swarm status` and `metrics --swarm --since`, run on a scratch
// swarm whose journal has a line pushed and one still queued, and change
// no file of the state, the clone or the home; `swarm journal --mark`
// then moves the mark, so the next connect shows the line collected after
// it alone.
func TestSwarmConnectBriefing(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	up := a.upstreams[swarmURL]
	refuse := filepath.Join(t.TempDir(), "refuse")
	os.WriteFile(filepath.Join(up, "hooks", "pre-receive"), []byte("#!/bin/sh\n[ -e "+refuse+" ] && exit 1\nexit 0\n"), 0o755)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass() // 1-architect-1's line pushed
	os.WriteFile(refuse, nil, 0o644)
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass() // 2-architect-1's line queued: its push refused
	if b, _ := os.ReadFile(a.state("journal")); !strings.Contains(string(b), " 2-architect-1: posted;") {
		t.Fatalf("queue %q\n%s", b, a.stderr.String())
	}
	if b, _ := os.ReadFile(a.state("clone")); string(b) != a.ws+"\n" {
		t.Errorf("<state>/clone %q, want the clone apply ran from", b)
	}

	before := snapshot(t, a.state(""), a.ws, home)
	elsewhere := t.TempDir()
	if err := a.connect(SwarmList, elsewhere, ""); err != nil {
		t.Fatal(err)
	}
	want := "Swarms on this machine (" + filepath.Join(a.local, "swarm") + "):\n" +
		"  acme/swarm-app on forge.example.net: clone " + a.ws + "; home testhome; not paused\n"
	if a.stdout.String() != want || a.stderr.Len() != 0 {
		t.Errorf("list from elsewhere:\n%s\nwant:\n%s\n%s", a.stdout.String(), want, a.stderr.String())
	}
	if err := a.connect(SwarmList, filepath.Join(a.ws, "prompts"), ""); err != nil ||
		a.stdout.String() != strings.TrimSuffix(want, "\n")+"; here "+a.ws+"\n" {
		t.Errorf("list from the clone: %v\n%s", err, a.stdout.String())
	}

	if err := a.connect(SwarmJournal, a.ws, ""); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(a.stdout.String(), "\n")
	if len(lines) != 5 || lines[0] != "Journal of acme/swarm-app, 2026-11-06 (no line marked seen): 2 lines" ||
		!strings.HasPrefix(lines[1], "2026-11-06 - 1501 1-architect-1: posted; ") || !strings.HasPrefix(lines[2], "2026-11-06 - 1502 2-architect-1: posted; ") ||
		lines[3] != "Mark them seen: daisy swarm journal --mark 2-architect-1" {
		t.Errorf("journal:\n%s", a.stdout.String())
	}

	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "Waiting for you: ") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}

	since := Value{"--since", "2026-11-06T15:02:00Z"}
	if err := a.connect(Metrics, a.ws, home, Value{"--swarm", "acme/swarm-app"}, since); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "  Rows taken: --swarm acme/swarm-app --since 2026-11-06T15:02:00Z\n  1 rows, 0 superseded") ||
		!strings.Contains(out, "  swarm-run  claude-fable-5-1  1 ") {
		t.Errorf("metrics:\n%s", out)
	}
	if err := a.connect(Metrics, a.ws, home, Value{"--since", "yesterday"}); err == nil ||
		err.Error() != "Error: --since must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: yesterday)" {
		t.Errorf("a bad --since: %v", err)
	}

	after := snapshot(t, a.state(""), a.ws, home)
	for p, v := range before {
		if after[p] != v {
			t.Errorf("the briefing changed %s", p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("the briefing wrote %s", p)
		}
	}

	if err := a.connect(SwarmJournal, a.ws, "", Value{"--mark", "9-worker-1"}); err == nil ||
		err.Error() != "Error: swarm journal --mark: the journal holds no line of 9-worker-1; mark a run its lines name" {
		t.Errorf("an unknown run: %v", err)
	}
	if _, err := os.Stat(a.state("connect-seen")); err == nil {
		t.Error("a refused mark was written")
	}
	if err := a.connect(SwarmJournal, a.ws, "", Value{"--mark", "2-architect-1"}); err != nil ||
		a.stdout.String() != "✓ Marked the journal seen through 2-architect-1 ("+a.state("connect-seen")+")\n" {
		t.Fatalf("mark: %v %q", err, a.stdout.String())
	}
	if err := a.connect(SwarmJournal, a.ws, ""); err != nil ||
		a.stdout.String() != "Journal of acme/swarm-app: nothing since 2-architect-1 (marked seen 2026-11-06T15:02:00Z)\n" {
		t.Errorf("nothing new: %v %q", err, a.stdout.String())
	}

	os.Remove(refuse)
	a.forge.CreateIssue("Docs check", "Check the docs.", []string{"swarm:proposal-accepted"})
	a.runner.Script(3, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass() // both queued lines pushed: 2-architect-1's once
	if err := a.connect(SwarmJournal, a.ws, ""); err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(a.stdout.String(), "\n")
	if len(lines) != 4 || lines[0] != "Journal of acme/swarm-app since 2-architect-1 (marked seen 2026-11-06T15:02:00Z): 1 line" ||
		!strings.HasPrefix(lines[1], "2026-11-06 - 1503 3-architect-1: posted; ") || lines[2] != "Mark them seen: daisy swarm journal --mark 3-architect-1" {
		t.Errorf("since the mark:\n%s", a.stdout.String())
	}

	// A line pushed whose queue was not yet removed (apply stopped
	// between the push and the removal) is one line.
	pushed := strings.Split(gitIn(t, up, "show", "main:journal/2026-11-06.md"), "\n")[4]
	os.WriteFile(a.state("journal"), []byte("2026-11-06 "+pushed+"\n"), 0o644)
	if err := a.connect(SwarmJournal, a.ws, ""); err != nil || a.stdout.String() != strings.Join(lines, "\n") {
		t.Errorf("pushed and queued:\n%s", a.stdout.String())
	}

	// A mark the journal does not hold: the newest day, with a warning.
	os.WriteFile(a.state("connect-seen"), []byte("7-owner-1 2026-11-01T00:00:00Z\n"), 0o644)
	if err := a.connect(SwarmJournal, a.ws, ""); err != nil || !strings.HasPrefix(a.stdout.String(), "Journal of acme/swarm-app, 2026-11-06 (no line marked seen): 3 lines\n") ||
		a.stderr.String() != "⚠ "+a.state("connect-seen")+" names 7-owner-1, which the journal does not hold: showing its newest day\n" {
		t.Errorf("a lost mark: %v\n%s\n%s", err, a.stdout.String(), a.stderr.String())
	}
}

// `swarm list` with no state, and a recorded clone that moved: said, not
// hidden; a paused swarm says since when and why.
func TestSwarmListStates(t *testing.T) {
	a := newApplyWS(t)
	elsewhere := t.TempDir()
	os.RemoveAll(filepath.Join(a.local, "swarm"))
	if err := a.connect(SwarmList, elsewhere, ""); err != nil || a.stdout.String() != "No swarm on this machine ("+filepath.Join(a.local, "swarm")+" holds no swarm's state)\n" {
		t.Errorf("none: %v %q", err, a.stdout.String())
	}
	os.MkdirAll(a.state(""), 0o755)
	os.MkdirAll(filepath.Join(a.local, "swarm", "credentials", "openai", "1"), 0o755)
	if err := a.connect(SwarmList, elsewhere, ""); err != nil ||
		a.stdout.String() != "Swarms on this machine ("+filepath.Join(a.local, "swarm")+"):\n  acme/swarm-app on forge.example.net: clone not recorded; no home; not paused\n" {
		t.Errorf("unrecorded: %v %q", err, a.stdout.String())
	}
	os.WriteFile(a.state("clone"), []byte(elsewhere+"\n"), 0o644)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T15:00:00Z: swarm fault: docker gone\n"), 0o644)
	if err := a.connect(SwarmList, elsewhere, ""); err != nil ||
		!strings.HasSuffix(a.stdout.String(), ": clone "+elsewhere+" (not a clone of it now); no home; paused since 2026-11-06T15:00:00Z: swarm fault: docker gone\n") {
		t.Errorf("moved and paused: %v %q", err, a.stdout.String())
	}
}
