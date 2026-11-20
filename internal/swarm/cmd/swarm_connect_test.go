package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/commands"
	"daisy/internal/ops"
	"daisy/internal/swarm/runner"
	"daisy/internal/verb"
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

	since := Value{Flag: "--since", Value: "2026-11-06T15:02:00Z"}
	if err := a.connect(commands.Metrics, a.ws, home, Value{Flag: "--swarm", Value: "acme/swarm-app"}, since); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "  Rows taken: --swarm acme/swarm-app --since 2026-11-06T15:02:00Z\n  1 rows, 0 superseded") ||
		!strings.Contains(out, "  swarm-run  claude-fable-5-1  1 ") {
		t.Errorf("metrics:\n%s", out)
	}
	if err := a.connect(commands.Metrics, a.ws, home, Value{Flag: "--since", Value: "yesterday"}); err == nil ||
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

	if err := a.connect(SwarmJournal, a.ws, "", Value{Flag: "--mark", Value: "9-worker-1"}); err == nil ||
		err.Error() != "Error: swarm journal --mark: the journal holds no line of 9-worker-1; mark a run its lines name" {
		t.Errorf("an unknown run: %v", err)
	}
	if _, err := os.Stat(a.state("connect-seen")); err == nil {
		t.Error("a refused mark was written")
	}
	if err := a.connect(SwarmJournal, a.ws, "", Value{Flag: "--mark", Value: "2-architect-1"}); err != nil ||
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

// authorities is an AUTHORITIES.md whose bytes the briefing must keep:
// indentation, trailing blanks, no newline at its end.
const authorities = "  # Authorities\n\nA1. Accept a plan whose `## Research` names only files under app/.\n   A2. Merge a routine job's pull request.  \n\nNothing else.\t "

// Step 33's claim (the plan remediating the 2026-11-15 design review,
// P21; docs/swarm.md section 11, swarm connect): over a fake swarm,
// `swarm connect` prints prompts/swarm-connect.md rule 2's briefing as
// one report, under ReadOnly: the State and Running lines; the journal
// since the mark grouped by issue, the lines of no issue last, and the
// Mark line; Waiting for you with each waiting issue's gate note, its
// Decide: line among it; Held; the runs; the open questions the swarm
// answers; AUTHORITIES.md quoted byte for byte; the metrics of the swarm's
// runs in the home <state>/home names, since the clone's newest commit to
// swarm.json or prompts/ and in all. --json carries the same facts. No
// file changes. Without AUTHORITIES.md nothing is granted; without a
// home the metrics say so.
func TestSwarmConnect(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice, Stdout: claudeResult})
	a.pass() // 15:01, 1-architect-1
	a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	// The persona prompts changed after the first run.
	// The persona prompts changed after the first run, committed at
	// 15:01:30 (GIT_COMMITTER_DATE, which ops.Real's git does not pass).
	if _, errb, err := a.exec(a.ws, "git", "-c", "user.name=T", "-c", "user.email=t@example.net", "pull", "-q", "--rebase", "origin", "main"); err != nil {
		t.Fatalf("pull: %v %s", err, errb)
	}
	a.write("prompts/worker.md", a.read("prompts/worker.md")+"\nKeep main.go short.\n")
	commit := osexec.Command("git", "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-qam", "Worker: keep main.go short")
	commit.Dir, commit.Env = a.ws, append(os.Environ(), "GIT_COMMITTER_DATE=2026-11-06T15:01:30Z")
	if b, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v %s", err, b)
	}
	if _, errb, err := a.exec(a.ws, "git", "push", "-q", "origin", "HEAD:main"); err != nil {
		t.Fatalf("push: %v %s", err, errb)
	}
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go say v1 or v1.0?\n"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass() // 15:02, 1-worker-1 asks, 2-architect-1 plans: #2 gated at the plan
	a.forge.CreateIssue("Docs check", "Check the docs.", []string{"swarm:proposal-accepted", "swarm:paused"})
	q, _ := os.ReadFile(a.state("journal"))
	os.WriteFile(a.state("journal"), append(q, "2026-11-06 - 1503 0-release-1: the owner's batch released\n"...), 0o644)
	a.write("AUTHORITIES.md", authorities)

	before := snapshot(t, a.state(""), a.ws, home)
	if err := a.readOnly(swarmConnect, a.forge); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	out := a.stdout.String()
	for _, want := range []string{
		"Swarm: acme/swarm-app on forge.example.net, clone " + a.ws + "\n" +
			"State: " + a.state("") + " — owner batch held: test — owner batch held: no brief is active\n" +
			"Running: none\n",
		"\nSince the last connect: Journal of acme/swarm-app, 2026-11-06 (no line marked seen): 4 lines\n  #1\n    2026-11-06 - 1501 1-architect-1: posted; ",
		"\n    2026-11-06 - 1502 1-worker-1: question; ",
		"\n  #2\n    2026-11-06 - 1502 2-architect-1: posted; ",
		"\n  no issue\n    2026-11-06 - 1503 0-release-1: the owner's batch released\nMark them seen: daisy swarm journal --mark 0-release-1\n",
		"\nWaiting for you: plan #2\n  #2, its gate note:\n    **reconciler:** gated at plan-0-posted: the plan gate holds the plan until the maintainer accepts it.\n",
		"\n    Decide: `daisy swarm accept 2`, `daisy swarm class 2 routine`, after which Daisy accepts it, or a comment here, from which the architect revises the plan.",
		"\nHeld: paused #3\nRuns: 3 (2 posted, 0 blocked, 1 questions, 0 failed, 0 faults, 0 other)\n",
		"\nQuestions the swarm answers: #1 for the architect\n  #1, worker run 1 at 2026-11-06T15:02:00Z:\n    QUESTION: Should main.go say v1 or v1.0?\n",
		"\nAuthorities: " + filepath.Join(a.ws, "AUTHORITIES.md") + ", quoted verbatim; Daisy acts on none of it\n" + authorities + "\nEnd of AUTHORITIES.md\n",
		"\nMetrics of the swarm's runs: " + filepath.Join(home, "metrics", "metrics.csv") + " (home testhome)\n",
		"\n  since the last model or prompt change, 2026-11-06T15:01:30Z ",
		" Worker: keep main.go short (the clone's newest commit to swarm.json or prompts/):\n" +
			"    claude-fable-5-1   1 row, tokens 30, cost $0.25, estimated $0.00\n" +
			"    claude-sonnet-5-5  1 row, tokens —, cost —, estimated —\n" +
			"  in all:\n" +
			"    claude-fable-5-1   2 rows, tokens 60, cost $0.50, estimated $0.00\n" +
			"    claude-sonnet-5-5  1 row, tokens —, cost —, estimated —\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
	// The sections in rule 2's order.
	last := -1
	for _, head := range []string{"Swarm: ", "\nSince the last connect: ", "\nWaiting for you: ", "\nHeld: ", "\nRuns: ", "\nQuestions the swarm answers: ", "\nAuthorities: ", "\nMetrics of the swarm's runs: "} {
		i := strings.Index(out, head)
		if i <= last {
			t.Errorf("%q at %d, after %d:\n%s", head, i, last, out)
		}
		last = i
	}

	a.flags = []string{verb.JSONFlag}
	if err := a.readOnly(swarmConnect, a.forge); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Journal struct {
			Issues []struct {
				Issue *int `json:"issue"`
				Lines []struct{ Run string }
			}
			Mark *string
		}
		Waiting struct {
			Issues []struct {
				Item     string
				Issue    int
				GateNote *string `json:"gate_note"`
			}
		}
		Held      struct{ Paused []int }
		Runs      struct{ Total int }
		Questions []struct {
			Issue    int
			Answerer string
			Asked    *struct{ Role, Text string }
		}
		Authorities *struct{ Path, Text string }
		Metrics     struct {
			Home        *string
			Change      *struct{ At, Subject string }
			SinceChange []struct{ Rows int } `json:"since_change"`
			All         []struct {
				Model *string
				Rows  int
			}
			Problem *string
		}
		Warnings []string
	}
	if err := json.Unmarshal(a.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("%v\n%s", err, a.stdout.String())
	}
	groups := func() (s []string) {
		for _, g := range doc.Journal.Issues {
			n := "none"
			if g.Issue != nil {
				n = fmt.Sprint(*g.Issue)
			}
			s = append(s, fmt.Sprintf("%s:%d", n, len(g.Lines)))
		}
		return s
	}()
	if !slices.Equal(groups, []string{"1:2", "2:1", "none:1"}) || doc.Journal.Mark == nil || *doc.Journal.Mark != "0-release-1" {
		t.Errorf("journal: %v %v", groups, doc.Journal.Mark)
	}
	if w := doc.Waiting.Issues; len(w) != 1 || w[0].Item != "plan" || w[0].Issue != 2 || w[0].GateNote == nil ||
		!strings.HasPrefix(*w[0].GateNote, "**reconciler:** gated at plan-0-posted: ") || !strings.Contains(*w[0].GateNote, "\n\nDecide: `daisy swarm accept 2`") {
		t.Errorf("waiting: %+v", w)
	}
	if q := doc.Questions; len(q) != 1 || q[0].Issue != 1 || q[0].Answerer != "architect" || q[0].Asked == nil ||
		q[0].Asked.Role != "worker" || q[0].Asked.Text != "QUESTION: Should main.go say v1 or v1.0?\n" {
		t.Errorf("questions: %+v", q)
	}
	if !slices.Equal(doc.Held.Paused, []int{3}) || doc.Runs.Total != 3 {
		t.Errorf("held %v, runs %d", doc.Held.Paused, doc.Runs.Total)
	}
	if au := doc.Authorities; au == nil || au.Text != authorities || au.Path != filepath.Join(a.ws, "AUTHORITIES.md") {
		t.Errorf("authorities: %+v", au)
	}
	if m := doc.Metrics; m.Home == nil || *m.Home != "testhome" || m.Change == nil || m.Change.At != "2026-11-06T15:01:30Z" || m.Change.Subject != "Worker: keep main.go short" ||
		len(m.SinceChange) != 2 || len(m.All) != 2 || m.All[0].Rows != 2 || m.Problem != nil {
		t.Errorf("metrics: %+v", m)
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("warnings: %q", doc.Warnings)
	}

	after := snapshot(t, a.state(""), a.ws, home)
	if !maps.Equal(before, after) {
		t.Error("the briefing changed a file")
	}

	// No AUTHORITIES.md: nothing granted. No home: no metrics, and why.
	os.Remove(filepath.Join(a.ws, "AUTHORITIES.md"))
	os.Remove(a.state("home"))
	a.flags = nil
	if err := a.readOnly(swarmConnect, a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "\nAuthorities: none granted (no AUTHORITIES.md in "+a.ws+")\n") ||
		!strings.HasSuffix(out, "\nMetrics of the swarm's runs: none ("+a.state("home")+" names no Daisy home)\n") {
		t.Errorf("no authorities, no home:\n%s", out)
	}
	// An AUTHORITIES.md it cannot read is no "none granted": the briefing
	// fails, and prints nothing.
	os.Mkdir(filepath.Join(a.ws, "AUTHORITIES.md"), 0o755)
	if err := a.readOnly(swarmConnect, a.forge); err == nil || !strings.HasPrefix(err.Error(), "Error: swarm connect: read "+filepath.Join(a.ws, "AUTHORITIES.md")+": ") || a.stdout.Len() != 0 {
		t.Errorf("unreadable: %v\n%s", err, a.stdout.String())
	}
}
