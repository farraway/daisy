package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// metricsHome makes testhome, a home with one commit registered in the
// registry, the root a clone with one commit, and <state>/home naming the
// home, as `swarm init` writes it in a workspace bound to testhome; it is
// the home's directory.
func (a *applyWS) metricsHome() string {
	a.t.Helper()
	home := filepath.Join(a.t.TempDir(), "testhome")
	os.MkdirAll(home, 0o755)
	for _, dir := range []string{home, a.root} {
		git(a.t, dir, "init", "-q")
		git(a.t, dir, "-c", "user.name=T", "-c", "user.email=t@example.net", "add", "-A")
		git(a.t, dir, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
	}
	os.WriteFile(filepath.Join(a.swarmWS.config, "homes"), []byte("testhome="+home+"\n"), 0o644)
	os.WriteFile(a.state("home"), []byte("testhome\n"), 0o644)
	return home
}

// rows is the home's metrics CSV, each row by its column names.
func rows(t *testing.T, home string) []map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, "metrics", "metrics.csv"))
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(records[0], metrics.Header) {
		t.Fatalf("header %q", records[0])
	}
	var out []map[string]string
	for _, rec := range records[1:] {
		m := map[string]string{}
		for i, c := range metrics.Header {
			m[c] = rec[i]
		}
		out = append(out, m)
	}
	return out
}

func headOf(t *testing.T, dir string) string {
	return strings.TrimSpace(gitIn(t, dir, "rev-parse", "HEAD"))
}

func promptHash(t *testing.T, a *applyWS, run string) string {
	sum := sha256.Sum256([]byte(a.file(run, "io/system.md")))
	return hex.EncodeToString(sum[:])
}

// Step 16b's claim (section 7.6, The run's record): two architect runs
// collected in one pass, the first's CLI reporting its cost and the
// second's not: two lines in the day's journal file, pushed to the swarm
// repository by the reconciler's identity, and two rows in the home
// <state>/home names, the second's cost_reported empty (not 0) and its
// estimate at the model's list price; `status` counts one run without a
// cost.
func TestSwarmApplyRecordsEachRun(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan, Stdout: `{"type":"result","is_error":false,"num_turns":2,"result":"done","usage":{"input_tokens":30,"output_tokens":40}}`})
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if a.stderr.Len() != 0 {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
	if r := a.result("2-architect-1"); !strings.Contains(r, "credential anthropic env CLAUDE_CODE_OAUTH_TOKEN\npersona anthropic claude-fable-5-1 high\n") ||
		!strings.Contains(r, "\ncost - tokens 30/40 turns 2\n") || !strings.HasSuffix(r, "\nrecorded\noutcome posted\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}

	up := a.upstreams[swarmURL]
	journal := gitIn(t, up, "show", "main:journal/2026-11-06.md")
	want := "# 2026-11-06\n\n" +
		"- 1500 1-architect-1: posted; anthropic claude-fable-5-1 high; tokens 10/20, 4 turns; cost $0.25; #1\n" +
		"- 1500 2-architect-1: posted; anthropic claude-fable-5-1 high; tokens 30/40, 2 turns; cost not reported (~$0.0023 at list price); #2"
	if journal != want {
		t.Errorf("journal:\n%s\nwant:\n%s", journal, want)
	}
	if got := gitIn(t, up, "log", "-1", "--format=%an <%ae>%n%s", "main"); got != "reconciler (Daisy swarm) <swarm@example.net>\nswarm: journal of 2 runs" {
		t.Errorf("journal commit: %q", got)
	}
	if _, err := os.Stat(a.state("journal")); err == nil {
		t.Error("<state>/journal kept after its push")
	}

	got := rows(t, home)
	if len(got) != 2 {
		t.Fatalf("%d rows: %v", len(got), got)
	}
	same := map[string]string{"when": "2026-11-06T15:00:00Z", "kind": "swarm-run", "plan_or_swarm": "acme/swarm-app", "role": "architect",
		"provider": "anthropic", "model": "claude-fable-5-1", "effort": "high", "root_sha": headOf(t, a.root), "home_sha": headOf(t, home),
		"outcome": "posted", "review_rounds": "", "tokens_total": "", "code_added": "", "range": ""}
	for i, w := range []map[string]string{
		{"step_or_run": "1-architect-1", "prompt_hash": promptHash(t, a, "1-architect-1"), "tokens_in": "10", "tokens_out": "20",
			"cost_reported": "0.25", "cost_estimated": "0.0011", "notes": "#1, 4 turns"},
		{"step_or_run": "2-architect-1", "prompt_hash": promptHash(t, a, "2-architect-1"), "tokens_in": "30", "tokens_out": "40",
			"cost_reported": "", "cost_estimated": "0.0023", "notes": "#2, 2 turns"},
	} {
		for k, v := range same {
			w[k] = v
		}
		for k, v := range w {
			if got[i][k] != v {
				t.Errorf("row %d: %s = %q, want %q", i+1, k, got[i][k], v)
			}
		}
	}

	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.stdout.String(), "  cost         $0.25 today, $0.25 in all (1 runs without a cost)\n") {
		t.Errorf("status:\n%s", a.stdout.String())
	}
}

// Step 11b's claim (plan for the inbox requests F27-F35; sections 2,
// 7.6 and 9): an architect on codex whose output is a live run's usage
// (93-worker-1, gpt-6.1-sol: input_tokens 3476833 holding
// cached_input_tokens 3390208, output_tokens 12969) records the cached
// count on its cost line, keeps tokens_in as codex reports it, puts the
// cached count in the row's notes and the journal line, and estimates
// 86625 uncached at $2, 3390208 cached at $0.10 and 12969 out at $10 per
// million: $0.641961.
func TestSwarmApplyPricesCachedInputApart(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.edit(func(c *declare.Config) {
		c.Personas["architect"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "medium", CoAuthor: "gpt-6.1-sol <noreply@openai.com>"}
	})
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: `{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"ok"}}
{"type":"turn.completed","usage":{"input_tokens":3476833,"cached_input_tokens":3390208,"cache_write_input_tokens":0,"output_tokens":12969,"reasoning_output_tokens":2175}}
`})
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if r := a.result("1-architect-1"); !strings.Contains(r, "\ncost - tokens 3476833/12969 turns 1 cached 3390208\n") {
		t.Errorf("result:\n%s", r)
	}
	got := rows(t, home)
	if len(got) != 1 {
		t.Fatalf("%d rows: %v", len(got), got)
	}
	for k, v := range map[string]string{"provider": "openai", "model": "gpt-6.1-sol", "tokens_in": "3476833", "tokens_out": "12969",
		"cost_reported": "", "cost_estimated": "0.641961", "notes": "#1, 1 turns, 3390208 cached input"} {
		if got[0][k] != v {
			t.Errorf("%s = %q, want %q", k, got[0][k], v)
		}
	}
	if journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md"); !strings.Contains(journal,
		" 1-architect-1: posted; openai gpt-6.1-sol medium; tokens 3476833/12969 (3390208 cached), 1 turns; cost not reported (~$0.641961 at list price); #1") {
		t.Errorf("journal:\n%s", journal)
	}
}

// A worker's row carries its pushed commits' lines per class, start to
// pushed in its repository's mirror, and a reviewer's its round; with no
// <state>/home, the journal alone is written.
func TestSwarmApplyRecordsWorkerLinesAndReviewRounds(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n",
		Work: commit(map[string]string{"main.go": "v1\nv1b\n", "main_test.go": "t\n", "README.md": "r\n"}, "v1")})
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	byRun := map[string]map[string]string{}
	for _, r := range rows(t, home) {
		byRun[r["step_or_run"]] = r
	}
	w := byRun["1-worker-1"]
	if w == nil || w["code_added"] != "2" || w["code_removed"] != "0" || w["tests_added"] != "1" || w["prose_added"] != "1" ||
		w["binary_files"] != "0" || !strings.HasSuffix(w["range"], ".."+strings.TrimPrefix(lineOf(a.result("1-worker-1"), "pushed "), "pushed ")) {
		t.Errorf("worker row %v\nresult:\n%s", w, a.result("1-worker-1"))
	}
	for _, run := range []string{"1-adversary-1", "1-sympathetic-1"} {
		if r := byRun[run]; r == nil || r["review_rounds"] != "1" || r["code_added"] != "" {
			t.Errorf("%s row %v", run, r)
		}
	}
	if r := byRun["1-architect-1"]; r == nil || r["review_rounds"] != "" {
		t.Errorf("architect row %v", r)
	}
	journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md")
	if !strings.Contains(journal, " 1-worker-1: posted; anthropic claude-sonnet-5-5 medium; no usage reported; code +2/-0, tests +1/-0, prose +1/-0 (") ||
		!strings.Contains(journal, " 1-adversary-1: posted; openai gpt-6.1-sol high; round 1; no usage reported; #1\n") {
		t.Errorf("journal:\n%s", journal)
	}

	// No home named: the journal alone, no row.
	os.Remove(a.state("home"))
	before, _ := os.ReadFile(filepath.Join(home, "metrics", "metrics.csv"))
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	after, _ := os.ReadFile(filepath.Join(home, "metrics", "metrics.csv"))
	if !bytes.Equal(before, after) {
		t.Error("a row written with no <state>/home")
	}
	if !strings.Contains(gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md"), " 2-architect-1: posted;") {
		t.Error("no journal line without a home")
	}
}

// lineOf is result's first line starting with prefix, "" for none.
func lineOf(result, prefix string) string {
	for _, l := range strings.Split(result, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

// init in a folder of a Daisy workspace bound to testhome records the
// home in <state>/home; a second init keeps the file as the maintainer
// left it (section 2).
func TestSwarmInitRecordsTheWorkspacesHome(t *testing.T) {
	s := newSwarmWS(t)
	os.MkdirAll(filepath.Join(s.dir, ".daisy"), 0o755)
	os.WriteFile(filepath.Join(s.dir, ".daisy", "home"), []byte("testhome\n"), 0o644)
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	home := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "home")
	if b, _ := os.ReadFile(home); string(b) != "testhome\n" || !strings.Contains(s.stdout.String(), "✓ Wrote "+home+" (testhome): each run's metrics row goes to that home\n") {
		t.Fatalf("<state>/home %q\n%s", b, s.stdout.String())
	}
	os.WriteFile(home, []byte("work\n"), 0o644)
	s.cloned()
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if b, _ := os.ReadFile(home); string(b) != "work\n" || !strings.Contains(s.stdout.String(), "  Kept "+home+" (exists)\n") {
		t.Errorf("<state>/home %q\n%s", b, s.stdout.String())
	}
}

// A journal the forge refuses (its hook declines the push) stays in
// <state>/journal with a warning, the run collected, and goes with the
// next pass's line; a journal/ that is not a directory on the head is
// never written through: the lines kept again (section 7.6, The run's
// record).
func TestSwarmJournalKeptForTheNextPass(t *testing.T) {
	a := newApplyWS(t)
	up := a.upstreams[swarmURL]
	refuse := filepath.Join(t.TempDir(), "refuse")
	os.WriteFile(filepath.Join(up, "hooks", "pre-receive"), []byte("#!/bin/sh\n[ -e "+refuse+" ] && exit 1\nexit 0\n"), 0o755)
	os.WriteFile(refuse, nil, 0o644)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	queue := a.state("journal")
	if !strings.Contains(a.stderr.String(), "⚠ journal not pushed (push rejected); its 1 line is kept in "+queue+" for the next pass\n") ||
		!strings.HasSuffix(a.result("1-architect-1"), "\nrecorded\noutcome posted\ncollected\n") {
		t.Fatalf("stderr:\n%s\nresult:\n%s", a.stderr.String(), a.result("1-architect-1"))
	}
	if b, _ := os.ReadFile(queue); !strings.HasPrefix(string(b), "2026-11-06 - 1501 1-architect-1: posted;") || strings.Count(string(b), "\n") != 1 {
		t.Fatalf("queue %q", b)
	}

	os.Remove(refuse)
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	journal := gitIn(t, up, "show", "main:journal/2026-11-06.md")
	if lines := strings.Split(journal, "\n"); len(lines) != 4 || !strings.HasPrefix(lines[2], "- 1501 1-architect-1: posted;") || !strings.HasPrefix(lines[3], "- 1502 2-architect-1: posted;") {
		t.Errorf("journal:\n%s", journal)
	}
	if _, err := os.Stat(queue); err == nil {
		t.Error("the queue kept after its push")
	}

	// journal/ replaced by a file on the head: not written, kept.
	dir := t.TempDir()
	gitIn(t, "", "clone", "-q", up, dir)
	os.RemoveAll(filepath.Join(dir, "journal"))
	os.WriteFile(filepath.Join(dir, "journal"), []byte("mine\n"), 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "journal as a file")
	gitIn(t, dir, "push", "-q", "origin", "main")
	head := gitIn(t, up, "rev-parse", "main")
	a.forge.CreateIssue("Docs check", "Check the docs.", []string{"swarm:proposal-accepted"})
	a.runner.Script(3, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if !strings.Contains(a.stderr.String(), "⚠ journal not pushed (journal/ is not a directory); its 1 line is kept in ") || gitIn(t, up, "rev-parse", "main") != head {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
}

// backfill is `swarm metrics --backfill` over the scratch state, git run
// for real on the mirrors.
func (a *applyWS) backfill() error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{NowFn: a.Now, ExecFn: a.exec}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Flags: []string{"--backfill"}}
	return a.at(c, false, swarmMetrics)
}

// oldRun writes run as a run collected before Step 16b of the plan that
// clears the Deferred list left it: its argv, launching the CLI argv cli
// (under the exit script when wrapped), its result, and its io/system.md.
func (a *applyWS) oldRun(run string, wrapped bool, cli, result string) {
	a.t.Helper()
	argv := "docker\nrun\n--rm\n--name\ndaisy-swarm.acme.swarm-app." + strings.ReplaceAll(run, "-", ".") + "\n--user\n1000:1000\n--workdir\n/work/app\ndaisy-member-app:1\n"
	if wrapped {
		argv += strings.Join(exitPrefix, "\n") + "\n"
	}
	a.runResult(run, result, "")
	os.WriteFile(a.state("runs/"+run+"/argv"), []byte(argv+cli+"\n"), 0o644)
	os.WriteFile(a.state("runs/"+run+"/io/system.md"), []byte("# "+run+"\n"), 0o644)
}

// Step 5's claim (plan for the inbox requests F27-F35; section 11, swarm
// metrics): over a scratch state with one run recorded live and three
// collected before Step 16b, with no persona line, a backfill writes
// three rows, the persona read from each run's argv (not swarm.json's),
// the old `cost 0` of codex, which reports no cost, as no cost, a
// worker's lines from start to pushed in its mirror, the time the run's
// end line's, the root's and home's commits empty and the notes
// `backfilled`; a run not collected gets none; the rows' costs sum to
// status's; a second backfill writes nothing.
func TestSwarmMetricsBackfill(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}

	start := gitIn(t, a.upstream("app"), "rev-parse", "main")
	a.land("app", map[string]string{"main.go": "a\nb\n", "main_test.go": "t\n", "docs/use.md": "p\n"}, "v1")
	pushed := gitIn(t, a.upstream("app"), "rev-parse", "main")
	if _, errb, err := a.exec(a.state("git/app"), "git", "fetch", "-q", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		t.Fatalf("fetch: %v %s", err, errb)
	}
	a.oldRun("0-owner-1", false, "claude\n-p\n--model\nclaude-sonnet-5-5\n--effort\nlow\n--append-system-prompt-file\n/work/.swarm/system.md",
		"start app "+start+"\npropose 5\nstarted 2026-11-05T10:00:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\nexited 0 2026-11-05T10:05:00Z\n"+
			"cost 0.5 tokens 1000/3000 turns 3\ncreating 1\noutcome posted\ncollected\n")
	a.oldRun("2-adversary-1", true, "codex\nexec\n--json\n--model\ngpt-6.1-sol\n-c\nmodel_reasoning_effort=medium\n--dangerously-bypass-approvals-and-sandbox\n--cd\n/work/app",
		"start "+pushed+"\nlabel swarm:review-0-ready\nstarted 2026-11-05T10:50:00Z\ncredential openai slot 1\nexited 0 2026-11-05T11:00:00.5Z\n"+
			"cost 0 tokens 1000/2000 turns 5\nposted comment\noutcome posted\ncollected\n")
	a.oldRun("3-worker-1", true, "grok\n--model\ngrok-4.7\n--reasoning-effort\nlow\n--cwd\n/work/app\n--trust",
		"start "+start+"\nbranch swarm/3-1 in app\nlabel swarm:plan-1-accepted\nstarted 2026-11-05T11:50:00Z\ncredential xai slot 1\nexited 0 2026-11-05T12:00:00Z\n"+
			"cost 0.02 tokens 10/20 turns 1\nfetched "+pushed+"\npushed "+pushed+"\ncreated pull 4\noutcome posted\ncollected\n")
	a.oldRun("4-worker-1", true, "grok\n--model\ngrok-4.7\n--reasoning-effort\nlow", "start "+start+"\nstarted 2026-11-05T12:30:00Z\nexited 0 2026-11-05T12:40:00Z\n")

	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	if a.stdout.String() != "✓ Backfilled 3 rows into "+csvFile+" (4 runs collected, 1 with its row already)\n" || a.stderr.Len() != 0 {
		t.Errorf("stdout:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	got := rows(t, home)
	if len(got) != 4 || got[0]["step_or_run"] != "1-architect-1" {
		t.Fatalf("%d rows: %v", len(got), got)
	}
	same := map[string]string{"kind": "swarm-run", "plan_or_swarm": "acme/swarm-app", "root_sha": "", "home_sha": "", "outcome": "posted", "review_rounds": "", "tokens_total": ""}
	for i, w := range []map[string]string{
		{"when": "2026-11-05T10:05:00Z", "step_or_run": "0-owner-1", "role": "owner", "provider": "anthropic", "model": "claude-sonnet-5-5", "effort": "low",
			"prompt_hash": promptHash(t, a, "0-owner-1"), "tokens_in": "1000", "tokens_out": "3000", "cost_reported": "0.5", "cost_estimated": "0.032",
			"code_added": "", "range": "", "notes": "batch, 3 turns, backfilled"},
		{"when": "2026-11-05T11:00:00Z", "step_or_run": "2-adversary-1", "role": "adversary", "provider": "openai", "model": "gpt-6.1-sol", "effort": "medium",
			"tokens_in": "1000", "tokens_out": "2000", "cost_reported": "", "cost_estimated": "", "code_added": "", "range": "", "notes": "#2, 5 turns, backfilled"},
		{"when": "2026-11-05T12:00:00Z", "step_or_run": "3-worker-1", "role": "worker", "provider": "xai", "model": "grok-4.7", "effort": "low",
			"tokens_in": "10", "tokens_out": "20", "cost_reported": "0.02", "cost_estimated": "",
			"code_added": "2", "code_removed": "0", "tests_added": "1", "prose_added": "1", "binary_files": "0", "range": start + ".." + pushed, "notes": "#3, 1 turns, backfilled"},
	} {
		for k, v := range same {
			w[k] = v
		}
		for k, v := range w {
			if got[i+1][k] != v {
				t.Errorf("row %d: %s = %q, want %q", i+2, k, got[i+1][k], v)
			}
		}
	}

	// The rows' costs are status's: the old `cost 0` is $0 to both.
	b, _ := os.ReadFile(csvFile)
	s, err := metrics.Summarize(b, metrics.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, g := range s.Groups {
		if g.Kind == metrics.SwarmRun {
			sum += g.Cost
		}
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("$%.2f", sum) != "$0.77" || !strings.Contains(a.stdout.String(), "  cost         $0.25 today, $0.77 in all (1 runs without a cost)\n") {
		t.Errorf("rows sum to $%.2f; status:\n%s", sum, a.stdout.String())
	}

	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, b) || a.stdout.String() != "✓ Backfilled 0 rows into "+csvFile+" (4 runs collected, 4 with its row already)\n" {
		t.Errorf("second backfill: stdout:\n%s\ncsv:\n%s", a.stdout.String(), after)
	}
}

// A worker whose commits are gone from its mirror gets its row with the
// line cells empty and a note, and a warning; with no <state>/home the
// backfill is refused, nothing written.
func TestSwarmMetricsBackfillGoneShasAndNoHome(t *testing.T) {
	a := newApplyWS(t)
	home := a.metricsHome()
	gone := strings.Repeat("ab", 20)
	a.oldRun("1-worker-1", true, "grok\n--model\ngrok-4.7\n--reasoning-effort\nlow",
		"start "+gone+"\nbranch swarm/1-1 in app\nstarted 2026-11-05T11:50:00Z\nexited 0 2026-11-05T12:00:00Z\npushed "+strings.Repeat("cd", 20)+"\noutcome posted\ncollected\n")
	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	got := rows(t, home)
	if len(got) != 1 || got[0]["code_added"] != "" || got[0]["range"] != "" || got[0]["notes"] != "#1, lines not counted, backfilled" ||
		!strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: lines not counted: git diff --numstat "+gone+"..") {
		t.Errorf("rows %v\nstderr:\n%s", got, a.stderr.String())
	}

	os.Remove(a.state("home"))
	os.Remove(filepath.Join(home, "metrics", "metrics.csv"))
	err := a.backfill()
	if err == nil || err.Error() != "Error: swarm metrics --backfill: "+a.state("home")+" names no Daisy home, whose metrics CSV would take the rows; write a registered home's name in it" {
		t.Errorf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "metrics", "metrics.csv")); err == nil {
		t.Error("a CSV written with no home named")
	}
}
