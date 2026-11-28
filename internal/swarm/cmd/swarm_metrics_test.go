package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
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
	t.Parallel()
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
		!strings.Contains(r, "\ncost - tokens 30/40 turns 2 cached 0\n") || !strings.HasSuffix(r, "\nrecorded\noutcome posted\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}

	up := a.upstreams[swarmURL]
	journal := gitIn(t, up, "show", "main:journal/2026-11-06.md")
	want := "# 2026-11-06\n\n" +
		"- 1500 1-architect-1: posted; anthropic claude-fable-5-1 high; tokens 10/20 (0 cached), 4 turns; cost $0.25; #1\n" +
		"- 1500 2-architect-1: posted; anthropic claude-fable-5-1 high; tokens 30/40 (0 cached), 2 turns; cost not reported (~$0.0023 at list price); #2"
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
		"outcome": "posted", "review_rounds": "", "tokens_cached": "0", "code_added": "", "range": ""}
	for i, w := range []map[string]string{
		{"step_or_run": "1-architect-1", "prompt_hash": promptHash(t, a, "1-architect-1"), "tokens_in": "10", "tokens_out": "20", "tokens_total": "30",
			"cost_reported": "0.25", "cost_estimated": "0.0011", "notes": "#1, 4 turns"},
		{"step_or_run": "2-architect-1", "prompt_hash": promptHash(t, a, "2-architect-1"), "tokens_in": "30", "tokens_out": "40", "tokens_total": "70",
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
	if !strings.Contains(a.stdout.String(), "  cost         $0.25 today, $0.25 in all (1 runs without a cost); estimated $0.00 in all at list price (0 runs without an estimate)\n") {
		t.Errorf("status:\n%s", a.stdout.String())
	}
}

// Step 11b's claim (plan for the inbox requests F27-F35; sections 2,
// 7.6 and 9): an architect on codex whose output is a live run's usage
// (93-worker-1, gpt-6.1-sol: input_tokens 3476833 holding
// cached_input_tokens 3390208, output_tokens 12969) records the cached
// count on its cost line, keeps tokens_in as codex reports it, puts the
// cached count in the row's tokens_cached (Step 28 of the plan
// remediating the 2026-11-15 design review; before it, the notes) and
// the journal line, and estimates
// 86625 uncached at $2, 3390208 cached at $0.10 and 12969 out at $10 per
// million: $0.641961.
func TestSwarmApplyPricesCachedInputApart(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.edit(func(c *declare.Config) {
		c.Personas["architect"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "medium"}
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
		"tokens_cached": "3390208", "tokens_total": "3489802", "cost_reported": "", "cost_estimated": "0.641961", "notes": "#1, 1 turns"} {
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
// <state>/home, the journal alone is written. From Step 7 of the plan
// implementing the 2026-11-19 swarm roadmap (L329): the row is dated by
// the run's end, its exited line's time, as a backfill dates it, though
// its check ran a minute past it; the journal line by its collection.
func TestSwarmApplyRecordsWorkerLinesAndReviewRounds(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n",
		Work: in("app", commit(map[string]string{"main.go": "v1\nv1b\n", "main_test.go": "t\n", "README.md": "r\n"}, "v1"))})
	a.runner.Script(1, "check", runner.Canned{Work: func(string) error { a.tick(); return nil }})
	a.pass()
	ended := strings.Fields(lineOf(a.result("1-worker-1"), "exited "))
	collected := a.Now().UTC()
	if len(ended) != 3 || !collected.After(mustTime(t, ended[2])) {
		t.Fatalf("the check did not move the clock past the worker's end: %v, now %s", ended, collected)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	byRun := map[string]map[string]string{}
	for _, r := range rows(t, home) {
		byRun[r["step_or_run"]] = r
	}
	w := byRun["1-worker-1"]
	if w == nil || w["when"] != ended[2] {
		t.Errorf("worker row's when %q, want its end %s", w["when"], ended[2])
	}
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
	if !strings.Contains(journal, "- "+collected.Format("1504")+" 1-worker-1: posted; anthropic claude-sonnet-5-5 medium; no usage reported; code +2/-0, tests +1/-0, prose +1/-0 (") ||
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

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return at
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
	t.Parallel()
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
	t.Parallel()
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
	return a.at(c, false, func(c *Context) error { return swarmMetrics(c, openForge) })
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
// `backfilled`; a cost line without its cached count, written before
// Step 28 of the plan remediating the 2026-11-15 design review, gets no
// estimate; a run not collected gets none; the rows' costs sum to
// status's; a second backfill writes nothing.
func TestSwarmMetricsBackfill(t *testing.T) {
	t.Parallel()
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
	a.oldRun("0-owner-1", false, "claude\n-p\n--model\nclaude-sonnet-5-5\n--effort\nlow\n--append-system-prompt-file\n/job/io/system.md",
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
	same := map[string]string{"kind": "swarm-run", "plan_or_swarm": "acme/swarm-app", "root_sha": "", "home_sha": "", "outcome": "posted", "review_rounds": "", "tokens_cached": ""}
	for i, w := range []map[string]string{
		{"when": "2026-11-05T10:05:00Z", "step_or_run": "0-owner-1", "role": "owner", "provider": "anthropic", "model": "claude-sonnet-5-5", "effort": "low",
			"prompt_hash": promptHash(t, a, "0-owner-1"), "tokens_in": "1000", "tokens_out": "3000", "tokens_total": "4000", "cost_reported": "0.5", "cost_estimated": "",
			"code_added": "", "range": "", "notes": "batch, 3 turns, backfilled"},
		{"when": "2026-11-05T11:00:00Z", "step_or_run": "2-adversary-1", "role": "adversary", "provider": "openai", "model": "gpt-6.1-sol", "effort": "medium",
			"tokens_in": "1000", "tokens_out": "2000", "tokens_total": "3000", "cost_reported": "", "cost_estimated": "", "code_added": "", "range": "", "notes": "#2, 5 turns, backfilled"},
		{"when": "2026-11-05T12:00:00Z", "step_or_run": "3-worker-1", "role": "worker", "provider": "xai", "model": "grok-4.7", "effort": "low",
			"tokens_in": "10", "tokens_out": "20", "tokens_total": "30", "cost_reported": "0.02", "cost_estimated": "",
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
	if fmt.Sprintf("$%.2f", sum) != "$0.77" || !strings.Contains(a.stdout.String(), "  cost         $0.25 today, $0.77 in all (1 runs without a cost); estimated $0.00 in all at list price (4 runs without an estimate)\n") {
		t.Errorf("rows sum to $%.2f; status:\n%s", sum, a.stdout.String())
	}

	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, b) || a.stdout.String() != "✓ Backfilled 0 rows into "+csvFile+" (4 runs collected, 4 with its row already)\n" {
		t.Errorf("second backfill: stdout:\n%s\ncsv:\n%s", a.stdout.String(), after)
	}

	// From Step 7 of the plan implementing the 2026-11-19 swarm roadmap
	// (L381): a review requesting changes is backfilled with its effect
	// unknown; its next review, approving a new head in the next round,
	// backfilled later, supersedes the first's row with its two blocking
	// findings, as a collection would, every other cell carried over.
	h1, h2 := strings.Repeat("a", 40), strings.Repeat("b", 40)
	review := func(round, head string) string {
		return "start " + head + "\nround " + round + "\nstarted 2026-11-05T13:00:00Z\nexited 0 2026-11-05T13:10:00Z\nposted review\noutcome posted\ncollected\n"
	}
	a.runResult("5-adversary-1", review("1", h1), "VERDICT: request-changes\n\n[blocking] main.go:1 wrong.\n[blocking] main.go:2 untested.\n")
	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	first := rows(t, home)
	if r := first[len(first)-1]; r["step_or_run"] != "5-adversary-1" || r["review_effect"] != "" {
		t.Fatalf("first review's row %v", r)
	}
	a.runResult("5-adversary-2", review("2", h2), approve)
	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if want := "✓ Backfilled 1 rows into " + csvFile + " (6 runs collected, 5 with its row already); 1 earlier reviews' rows superseded with their review_effect\n"; a.stdout.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", a.stdout.String(), want)
	}
	got = rows(t, home)
	if len(got) != len(first)+2 || got[len(got)-2]["step_or_run"] != "5-adversary-2" || got[len(got)-2]["review_effect"] != "0" {
		t.Fatalf("rows %v", got[len(first):])
	}
	sup, held := got[len(got)-1], first[len(first)-1]
	for _, col := range metrics.Header {
		if want := held[col]; col == "review_effect" && sup[col] != "2" || col != "review_effect" && sup[col] != want {
			t.Errorf("5-adversary-1's superseding %s: %q (held %q)", col, sup[col], want)
		}
	}
}

// A worker whose commits are gone from its mirror gets its row with the
// line cells empty and a note, and a warning; with no <state>/home the
// backfill is refused, nothing written.
func TestSwarmMetricsBackfillGoneShasAndNoHome(t *testing.T) {
	t.Parallel()
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

// metricsVerb is `swarm metrics` with flags over the scratch state.
func (a *applyWS) metricsVerb(flags ...string) error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{NowFn: a.Now, ExecFn: a.exec}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Flags: flags}
	return a.at(c, false, func(c *Context) error { return swarmMetrics(c, openForge) })
}

// Step 28's claim (the plan remediating the 2026-11-15 design review,
// P22; section 11, swarm metrics): over a fixture CSV under the first
// header, holding a codex worker's row recorded before gpt-6.1-sol was
// priced (93-worker-1's usage: no estimate, no cached count) and a
// claude architect's whose tokens_in left the cache out (44), a reprice
// reads each run's stdout through its provider's adapter and appends a
// row superseding each: the codex row's tokens_cached 3390208,
// tokens_total and its estimate, $0.641961; the claude row's tokens_in
// with the cache's 852044 and its estimate, $1.17044 (52044 uncached at
// $10, 800000 cached at $0.25, 9000 out at $50, where the held row priced
// the 44 alone); every other cell carried over, the notes ending
// `repriced`. A run without stdout is unread, one without a row or not
// collected not counted; --dry-run writes nothing; a second reprice
// writes nothing. From Step 7 of the plan implementing the 2026-11-19
// swarm roadmap (L381): the codex row's notes, recorded before Step 28
// with `3390208 cached input`, lose that part once tokens_cached holds it.
func TestSwarmMetricsReprice(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	held := "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds," +
		"tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed," +
		"binary_files,range,notes\n" +
		"2026-11-12T08:00:00Z,swarm-run,acme/swarm-app,0-owner-1,owner,anthropic,claude-opus-5-5,low,,,,posted,,30,40,,0.1,0.00092,,,,,,,,,\"batch, 2 turns\"\n" +
		"2026-11-12T09:00:00Z,swarm-run,acme/swarm-app,1-architect-1,architect,anthropic,claude-fable-5-1,high,abcdef1,1234567,ph,posted,,44,9000,,1.2,0.45044,,,,,,,,,\"#1, 20 turns\"\n" +
		"2026-11-12T10:00:00Z,swarm-run,acme/swarm-app,93-worker-1,worker,openai,gpt-6.1-sol,medium,abcdef1,1234567,ph,posted,,3476833,12969,,,,120,4,80,0,3,1,0,aaaaaaa..bbbbbbb,\"#93, 1 turns, 3390208 cached input\"\n"
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	os.WriteFile(csvFile, []byte(held), 0o644)
	for run, stdout := range map[string]string{
		"0-owner-1":     "",
		"1-architect-1": `{"type":"result","is_error":false,"num_turns":20,"result":"Done.","total_cost_usd":1.2,"usage":{"input_tokens":44,"cache_creation_input_tokens":52000,"cache_read_input_tokens":800000,"output_tokens":9000}}`,
		"4-adversary-1": `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":5,"output_tokens":1}}`,
		"6-worker-1":    `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":5,"output_tokens":1}}`,
		"93-worker-1": `{"type":"thread.started","thread_id":"t"}
{"type":"turn.completed","usage":{"input_tokens":3476833,"cached_input_tokens":3390208,"cache_write_input_tokens":0,"output_tokens":12969,"reasoning_output_tokens":2175}}
`,
	} {
		result := "started 2026-11-12T07:00:00Z\nexited 0 2026-11-12T07:30:00Z\noutcome posted\ncollected\n"
		if run == "6-worker-1" {
			result = "started 2026-11-12T07:00:00Z\n"
		}
		a.runResult(run, result, "")
		if stdout != "" {
			os.WriteFile(a.state("runs/"+run+"/stdout"), []byte(stdout), 0o644)
		}
	}
	a.runResult("7-merge-1", "merge app c1 into b1 at m1\npassed\ncollected\n", "") // a merge check has no member, no row and no count (section 12.1)
	counts := "(4 runs collected, 3 with a row: 0 current, 1 with no usage to read)"

	if err := a.metricsVerb("--reprice", "--dry-run"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if want := "  1-architect-1 anthropic claude-fable-5-1: tokens_in 44 → 852044, tokens_cached — → 800000, tokens_total — → 861044, cost_estimated 0.45044 → 1.17044\n" +
		"  93-worker-1 openai gpt-6.1-sol: tokens_in 3476833 → 3476833, tokens_cached — → 3390208, tokens_total — → 3489802, cost_estimated — → 0.641961\n" +
		"Would reprice 2 rows in " + csvFile + " " + counts + "; nothing written\n"; a.stdout.String() != want || a.stderr.Len() != 0 {
		t.Errorf("dry run: stdout:\n%s\nwant:\n%s\nstderr:\n%s", a.stdout.String(), want, a.stderr.String())
	}
	if b, _ := os.ReadFile(csvFile); string(b) != held {
		t.Fatalf("a dry run wrote:\n%s", b)
	}

	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if want := "✓ Repriced 2 rows into " + csvFile + " " + counts + "\n"; a.stdout.String() != want || a.stderr.Len() != 0 {
		t.Errorf("stdout:\n%s\nwant:\n%s\nstderr:\n%s", a.stdout.String(), want, a.stderr.String())
	}
	b, _ := os.ReadFile(csvFile)
	_, oldRows, _ := strings.Cut(held, "\n")
	want := strings.Join(metrics.Header, ",") + "\n" + oldRows +
		"2026-11-12T09:00:00Z,swarm-run,acme/swarm-app,1-architect-1,architect,anthropic,claude-fable-5-1,high,abcdef1,1234567,ph,posted,,852044,9000,861044,1.2,1.17044,,,,,,,,,\"#1, 20 turns, repriced\",800000,,,\n" +
		"2026-11-12T10:00:00Z,swarm-run,acme/swarm-app,93-worker-1,worker,openai,gpt-6.1-sol,medium,abcdef1,1234567,ph,posted,,3476833,12969,3489802,,0.641961,120,4,80,0,3,1,0,aaaaaaa..bbbbbbb,\"#93, 1 turns, repriced\",3390208,,,\n"
	if string(b) != want {
		t.Fatalf("csv:\n%s\nwant:\n%s", b, want)
	}
	s, err := metrics.Summarize(b, metrics.Filter{Swarm: "acme/swarm-app"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows != 5 || s.Superseded != 2 {
		t.Errorf("summary %+v", s)
	}
	for _, g := range s.Groups {
		if g.Model == "gpt-6.1-sol" && (g.Rows != 1 || g.Estimate != 0.641961 || g.Cached != 3390208 || g.Tokens != 3489802) {
			t.Errorf("gpt-6.1-sol group %+v", g)
		}
		if g.Model == "claude-fable-5-1" && (g.Rows != 1 || g.Estimate != 1.17044 || g.Tokens != 861044) {
			t.Errorf("claude-fable-5-1 group %+v", g)
		}
	}

	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, b) ||
		a.stdout.String() != "✓ Repriced 0 rows into "+csvFile+" (4 runs collected, 3 with a row: 2 current, 1 with no usage to read)\n" {
		t.Errorf("second reprice: stdout:\n%s\ncsv:\n%s", a.stdout.String(), after)
	}
	if err := a.metricsVerb("--reprice", "--dry-run"); err != nil ||
		a.stdout.String() != "Would reprice 0 rows in "+csvFile+" (4 runs collected, 3 with a row: 2 current, 1 with no usage to read); nothing written\n" {
		t.Errorf("a dry run once current = %v: stdout:\n%s", err, a.stdout.String())
	}

	for _, flags := range [][]string{nil, {"--backfill", "--reprice"}, {"--backfill", "--dry-run"}, {"--dry-run"}} {
		var usage UsageError
		if err := a.metricsVerb(flags...); !errors.As(err, &usage) {
			t.Errorf("swarm metrics %v = %v, want a usage error", flags, err)
		}
	}
}

// Step 30's claim, the swarm's side (the plan remediating the 2026-11-15
// design review, P29; section 7.6, The run's record): a pull request
// reviewed in two rounds, the adversary requesting changes with one
// blocking finding, which completes round 1 alone (decision 163), the
// worker pushing v2, both reviewers approving it, the sympathetic
// reviewer once the adversary approved: the adversary's first row is
// written with its effect unknown, and the collection of its second
// review supersedes it with effect 1, every other cell carried over;
// each approving review's effect is 0, the worker's row has none;
// `daisy metrics` sums them by role and model.
func TestSwarmApplyRecordsReviewEffect(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.pass() // T10, by the adversary alone
	first := map[string]map[string]string{}
	for _, r := range rows(t, home) {
		first[r["step_or_run"]] = r
	}
	if r := first["1-adversary-1"]; r == nil || r["review_effect"] != "" || r["review_rounds"] != "1" {
		t.Fatalf("first adversary row %v", r)
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-2-approved") {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	all := rows(t, home)
	last := map[string]map[string]string{}
	count := map[string]int{}
	for _, r := range all {
		last[r["step_or_run"]] = r
		count[r["step_or_run"]]++
	}
	if _, ok := last["1-sympathetic-2"]; ok {
		t.Errorf("a second sympathetic run: %v", last["1-sympathetic-2"])
	}
	for run, want := range map[string]string{"1-adversary-1": "1", "1-adversary-2": "0", "1-sympathetic-1": "0", "1-worker-1": "", "1-worker-2": ""} {
		if r := last[run]; r == nil || r["review_effect"] != want {
			t.Errorf("%s: review_effect %q, want %q", run, r["review_effect"], want)
		}
	}
	if count["1-adversary-1"] != 2 || count["1-sympathetic-1"] != 1 || count["1-adversary-2"] != 1 {
		t.Errorf("rows per run %v", count)
	}
	sup, held := last["1-adversary-1"], first["1-adversary-1"]
	for _, col := range metrics.Header {
		if col != "review_effect" && sup[col] != held[col] {
			t.Errorf("1-adversary-1's %s: %q, superseding %q", col, sup[col], held[col])
		}
	}
	if strings.Contains(a.stderr.String(), "review_effect") {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
	b, _ := os.ReadFile(filepath.Join(home, "metrics", "metrics.csv"))
	s, err := metrics.Summarize(b, metrics.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Reviews) != 2 || s.Reviews[0].Role != "adversary" || s.Reviews[0].Rows != 2 || s.Reviews[0].Effect != 1 || s.Reviews[0].WithEffect != 2 ||
		s.Reviews[1].Role != "sympathetic" || s.Reviews[1].Rows != 1 || s.Reviews[1].Effect != 0 || s.Reviews[1].WithEffect != 1 {
		t.Errorf("reviews %+v", s.Reviews)
	}
}

// R3-12's claim (the plan implementing the 2026-11-19 swarm roadmap;
// section 7.6, The run's record): with the sympathetic reviewer after
// the adversary's approval (decision 163), the sympathetic reviewer
// requests changes with one blocking finding in round 1, the worker
// pushes v2, the adversary requests changes alone in round 2, the worker
// pushes v3, both approve it in round 3: the sympathetic reviewer's
// first review is credited its finding, 1, read from its role's next
// review on the pull request though that review is of round 3, not 2.
func TestSwarmReviewEffectAcrossAnAdversaryOnlyRound(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: requestChanges})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved: T10
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.pass() // round 2, the adversary's alone: T10
	a.runner.Script(1, "worker", work("v3"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // T9
	if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-2-approved") {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	last := map[string]map[string]string{}
	for _, r := range rows(t, home) {
		last[r["step_or_run"]] = r
	}
	for run, want := range map[string][2]string{
		"1-sympathetic-1": {"1", "1"}, "1-sympathetic-2": {"3", "0"},
		"1-adversary-1": {"1", "0"}, "1-adversary-2": {"2", "1"}, "1-adversary-3": {"3", "0"},
	} {
		if r := last[run]; r == nil || r["review_rounds"] != want[0] || r["review_effect"] != want[1] {
			t.Errorf("%s: round %q, review_effect %q; want round %s, effect %q", run, r["review_rounds"], r["review_effect"], want[0], want[1])
		}
	}
}

// Step 30's reprice (section 11, swarm metrics): over a fixture CSV
// under Step 28's header, holding the rows a collection wrote before the
// review_effect column for one issue's three reviews (the adversary's
// request with two blocking findings, then its approval of the worker's
// new head; the sympathetic reviewer's approval), a reprice supersedes
// each with the effect the runs give, 2, 0 and 0, its usage cells as
// they were when its output reports none; the dry run names the effect;
// a second reprice writes nothing; an unknown effect keeps one set by
// hand.
func TestSwarmMetricsRepriceReviewEffect(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	held := "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds," +
		"tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed," +
		"binary_files,range,notes,tokens_cached\n" +
		"2026-11-12T08:00:00Z,swarm-run,acme/swarm-app,2-adversary-1,adversary,xai,grok-4.7,high,,,,posted,1,100,10,110,,,,,,,,,,,\"#2, 3 turns\",\n" +
		"2026-11-12T09:00:00Z,swarm-run,acme/swarm-app,2-sympathetic-1,sympathetic,xai,grok-4.7,high,,,,posted,1,200,20,220,,,,,,,,,,,\"#2, 3 turns\",\n" +
		"2026-11-12T10:00:00Z,swarm-run,acme/swarm-app,2-adversary-2,adversary,xai,grok-4.7,high,,,,posted,2,300,30,330,,,,,,,,,,,\"#2, 3 turns\",\n"
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	os.WriteFile(csvFile, []byte(held), 0o644)
	h1, h2 := strings.Repeat("a", 40), strings.Repeat("b", 40)
	result := func(round, head string) string {
		return "start " + head + "\nround " + round + "\nstarted 2026-11-12T07:00:00Z\nexited 0 2026-11-12T07:30:00Z\nposted review\nrecorded\noutcome posted\ncollected\n"
	}
	a.runResult("2-adversary-1", result("1", h1), "VERDICT: request-changes\n\n[blocking] main.go:1 wrong.\n[blocking] main.go:2 untested.\n")
	a.runResult("2-sympathetic-1", result("1", h1), approve)
	a.runResult("2-adversary-2", result("2", h2), approve)
	counts := "(3 runs collected, 3 with a row: 0 current, 3 with no usage to read)"

	if err := a.metricsVerb("--reprice", "--dry-run"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if want := "  2-adversary-1 xai grok-4.7: tokens_in 100 → 100, tokens_cached — → —, tokens_total 110 → 110, cost_estimated — → —, review_effect — → 2\n" +
		"  2-adversary-2 xai grok-4.7: tokens_in 300 → 300, tokens_cached — → —, tokens_total 330 → 330, cost_estimated — → —, review_effect — → 0\n" +
		"  2-sympathetic-1 xai grok-4.7: tokens_in 200 → 200, tokens_cached — → —, tokens_total 220 → 220, cost_estimated — → —, review_effect — → 0\n" +
		"Would reprice 3 rows in " + csvFile + " " + counts + "; nothing written\n"; a.stdout.String() != want || a.stderr.Len() != 0 {
		t.Errorf("dry run: stdout:\n%s\nwant:\n%s\nstderr:\n%s", a.stdout.String(), want, a.stderr.String())
	}
	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	b, _ := os.ReadFile(csvFile)
	_, oldRows, _ := strings.Cut(held, "\n")
	want := strings.Join(metrics.Header, ",") + "\n" + oldRows +
		"2026-11-12T08:00:00Z,swarm-run,acme/swarm-app,2-adversary-1,adversary,xai,grok-4.7,high,,,,posted,1,100,10,110,,,,,,,,,,,\"#2, 3 turns, repriced\",,2,,\n" +
		"2026-11-12T10:00:00Z,swarm-run,acme/swarm-app,2-adversary-2,adversary,xai,grok-4.7,high,,,,posted,2,300,30,330,,,,,,,,,,,\"#2, 3 turns, repriced\",,0,,\n" +
		"2026-11-12T09:00:00Z,swarm-run,acme/swarm-app,2-sympathetic-1,sympathetic,xai,grok-4.7,high,,,,posted,1,200,20,220,,,,,,,,,,,\"#2, 3 turns, repriced\",,0,,\n"
	if string(b) != want {
		t.Fatalf("csv:\n%s\nwant:\n%s", b, want)
	}
	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, b) || a.stdout.String() != "✓ Repriced 0 rows into "+csvFile+" "+counts+"\n" {
		t.Errorf("second reprice: stdout:\n%s\ncsv:\n%s", a.stdout.String(), after)
	}

	// From Step 7 of the plan implementing the 2026-11-19 swarm roadmap
	// (L381): a review whose effect the runs read unknown (changes
	// requested, no next review) keeps the effect its row holds, set by
	// hand: nothing written.
	held3 := string(b) + "2026-11-12T11:00:00Z,swarm-run,acme/swarm-app,3-adversary-1,adversary,xai,grok-4.7,high,,,,posted,1,100,10,110,,,,,,,,,,,\"#3, 3 turns\",,1,\n"
	os.WriteFile(csvFile, []byte(held3), 0o644)
	a.runResult("3-adversary-1", result("1", h1), "VERDICT: request-changes\n\n[blocking] main.go:1 wrong.\n")
	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(csvFile); string(after) != held3 || !strings.HasPrefix(a.stdout.String(), "✓ Repriced 0 rows into ") {
		t.Errorf("a hand-set effect: stdout:\n%s\ncsv:\n%s", a.stdout.String(), after)
	}
}

// Step 4's claim (the plan implementing the 2026-11-19 swarm roadmap,
// R0; section 7.5, The stop's cause): a member's stop, BLOCKED: or
// QUESTION:, records the word of the Cause: line under it in its row's
// cause column; a word outside the vocabulary records it empty with a
// warning, and a stop without the line records it empty without one.
// The owner's batch reads the line under its BLOCKED:, after any Met:
// lines; a run that did not stop records none, though its text holds
// the lines (an architect's QUESTION: on a stage that reads no question
// is the failure off its stage, whose cause is empty: Step S12). Every
// Task text asks for the line.
func TestSwarmApplyRecordsTheStopsCause(t *testing.T) {
	t.Parallel()
	warning := `⚠ #1 worker run 1: cause left empty: "whim" is not one of plan, prerequisite, main-red, environment, forge-fact, kept-work, ruling, scope, capacity` + "\n"
	for _, tc := range []struct {
		name, out, outcome, cause, stderr string
	}{
		{"blocked", "BLOCKED: the plan's fixture says v2, main.go says v1.\nCause: plan\n", "blocked", "plan", ""},
		{"question after a blank line", "QUESTION: May I rename main.go?\n\nCause: scope\n", "question", "scope", ""},
		{"an unknown word", "QUESTION: Should main.go say v1 or v1.0?\nCause: whim\n", "question", "", warning},
		{"no line", "BLOCKED: what now?\n", "blocked", "", ""},
		{"not a stop", "v1\n\nCause: plan\n\nNot established: none\n", "posted", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			home := a.metricsHome()
			a.planned(oneSlice)
			canned := runner.Canned{Out: tc.out}
			if tc.outcome == "posted" {
				canned.Work = in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))
			}
			a.runner.Script(1, "worker", canned)
			a.pass()
			if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, causeSentence) {
				t.Errorf("worker's Task lacks the cause sentence:\n%s", sys)
			}
			if r := a.result("1-worker-1"); !strings.HasSuffix(r, "outcome "+tc.outcome+"\ncollected\n") {
				t.Fatalf("result:\n%s\n%s", r, a.stdout.String())
			}
			var row map[string]string
			for _, r := range rows(t, home) {
				if r["step_or_run"] == "1-worker-1" {
					row = r
				}
			}
			if row == nil || row["cause"] != tc.cause || row["outcome"] != tc.outcome {
				t.Errorf("row %v: cause %q, want %q", row, row["cause"], tc.cause)
			}
			if got := a.stderr.String(); got != tc.stderr {
				t.Errorf("stderr:\n%s\nwant:\n%s", got, tc.stderr)
			}
		})
	}
	t.Run("a QUESTION: line off its stage is no member's stop", func(t *testing.T) {
		a := newApplyWS(t)
		home := a.metricsHome()
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
		a.runner.Script(1, "architect", runner.Canned{Out: "QUESTION: is this a plan?\nCause: plan\n\n" + oneSlice})
		a.pass()
		got := rows(t, home)
		if len(got) != 1 || got[0]["outcome"] != "failed" || got[0]["cause"] != "" {
			t.Errorf("rows %v", got)
		}
	})
	t.Run("the owner's batch", func(t *testing.T) {
		a := newApplyWS(t)
		home := a.metricsHome()
		a.release()
		a.runner.Script(0, "owner", runner.Canned{Out: "Met: docs/briefs/a.md\n\nBLOCKED: nothing to propose until the library lands\nCause: prerequisite\n"})
		a.pass()
		if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, causeSentence) {
			t.Errorf("owner's Task lacks the cause sentence:\n%s", sys)
		}
		got := rows(t, home)
		if len(got) != 1 || got[0]["step_or_run"] != "0-owner-1" || got[0]["outcome"] != "blocked" || got[0]["cause"] != "prerequisite" {
			t.Errorf("rows %v", got)
		}
	})
}

// causeSentence ends every Task text's closing paragraph (section 7.4).
const causeSentence = "Write one line `Cause: <word>` under any line beginning `BLOCKED:` or `QUESTION:` that you write, even one this text calls the only line, the word naming why you stopped: " +
	"`plan` (the plan, or the issue where there is none, is wrong, unclear or cannot be followed), " +
	"`prerequisite` (work the job depends on is not merged), " +
	"`main-red` (the base branch is broken: its check fails without your change), " +
	"`environment` (a tool, the image, a service or the network the run needs fails or is missing), " +
	"`forge-fact` (a fact on the forge you need cannot be read), " +
	"`kept-work` (work Daisy was to keep or give you is missing: a plan cut short, a kept commit not given back), " +
	"`ruling` (only the maintainer's ruling settles it), " +
	"`scope` (whether you may change what the plan or the job leaves out), " +
	"or `capacity` (a provider's limit, quota or credit stopped the work)."

// Step 5's claim, (a) (the plan implementing the 2026-11-19 swarm
// roadmap, R0; section 7.5, The stop's cause, Daisy's own stops): a run
// that ends in a stop of Daisy's own records the cause section 7.5 names
// for it, read from its fault: or failed: line: a check that cannot
// start, and any fault of the machine, `environment`; a provider's
// limit `capacity`, its sign-in `environment`; a worker's protected path
// `scope`; an architect's bad slices `plan`; a failure no word names (a
// non-zero exit) none.
func TestSwarmApplyRecordsDaisysOwnCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, role   string
		canned       runner.Canned
		check        runner.Canned
		outcome      string
		cause, fault string
	}{
		{name: "a check that cannot start", role: "worker", canned: work("v1"), check: runner.Canned{Code: 125}, outcome: "fault", cause: "environment", fault: "fault: check: exit 125"},
		{name: "the image missing", role: "worker", canned: runner.Canned{Code: 125}, outcome: "fault", cause: "environment", fault: "fault: exit 125"},
		{name: "a provider's limit", role: "worker", canned: runner.Canned{Code: 1, Stdout: `{"type":"result","is_error":true,"result":"Claude AI usage limit reached"}`}, outcome: "fault", cause: "capacity", fault: "fault: provider: Claude AI usage limit reached"},
		{name: "a provider's usage credits", role: "worker", canned: claudeCredits, outcome: "fault", cause: "capacity", fault: "fault: provider: You're out of usage credits. Switch to another model to continue."},
		{name: "a provider's sign-in", role: "worker", canned: runner.Canned{Code: 1, Stdout: `{"type":"result","is_error":true,"result":"Invalid API key · Please run /login"}`}, outcome: "fault", cause: "environment", fault: "fault: provider: Invalid API key · Please run /login"},
		{name: "a protected path", role: "worker", canned: runner.Canned{Out: "Done.\n", Work: in("app", commit(map[string]string{"main.go": "v1\n", ".gitea/workflows/ci.yml": "on: push\n"}, "v1"))}, outcome: "failed", cause: "scope", fault: "failed: protected path changed: .gitea/workflows/ci.yml"},
		{name: "bad slices", role: "architect", canned: runner.Canned{Out: "## Steps\n\n1. app: one.\n2. other: two.\n"}, outcome: "failed", cause: "plan", fault: "failed: bad slices: 2"},
		{name: "a non-zero exit", role: "worker", canned: runner.Canned{Out: "x\n", Code: 2}, outcome: "failed", cause: "", fault: "failed: exit 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			home := a.metricsHome()
			if tc.role == "worker" {
				a.planned(oneSlice)
			} else {
				a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
			}
			a.runner.Script(1, tc.role, tc.canned)
			a.runner.Script(1, "check", tc.check)
			a.pass()
			run := "1-" + tc.role + "-1"
			if r := a.result(run); !strings.Contains(r, "\n"+tc.fault+"\n") || !strings.HasSuffix(r, "outcome "+tc.outcome+"\ncollected\n") {
				t.Fatalf("result:\n%s\n%s", r, a.stdout.String())
			}
			var row map[string]string
			for _, r := range rows(t, home) {
				if r["step_or_run"] == run {
					row = r
				}
			}
			if row == nil || row["cause"] != tc.cause {
				t.Errorf("row %v: cause %q, want %q", row, row["cause"], tc.cause)
			}
		})
	}
}

// Step 5's claim, (b) (section 11, unblock): `swarm unblock <n> --cause
// <word>` supersedes the metrics row of the run whose stop it releases
// by the CSV's own rule, a later row of the run's key: the held row with
// the word in its cause cell and its notes ending `retagged`, every
// other cell carried over; a word outside the vocabulary, after <n> or
// leading as the cli reads a value flag, or no word, is usage before
// anything is written; a block no run's collection set (by hand) is
// refused, nothing changed.
func TestSwarmUnblockRetagsTheCause(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: may I rename main.go?\nCause: plan\n"})
	a.pass()
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	held, _ := os.ReadFile(csvFile)
	m := &maintainerWS{statusWS: a.statusWS}
	m.secrets(bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
	before := m.writes(1)
	var u UsageError
	for _, tc := range []struct {
		values []Value
		args   []string
	}{
		{nil, []string{"1", "--cause", "whim", "Rename it."}},
		{[]Value{{Flag: "--cause", Value: "whim"}}, []string{"1", "Rename it."}},
		{nil, []string{"1", "--cause"}},
		{nil, []string{"1", "--cause", "scope"}},
	} {
		m.values = tc.values
		if err := m.verb(swarmUnblock, tc.args...); !errors.As(err, &u) {
			t.Errorf("%v %q: %v, want usage", tc.values, tc.args, err)
		}
	}
	m.values = nil
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, held) || !slices.Equal(m.writes(1), before) {
		t.Fatalf("a usage error wrote:\n%s\n%q", after, m.writes(1)[len(before):])
	}
	if err := m.verb(swarmUnblock, "1", "--cause", "scope", "Rename", "it."); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.stdout.String(), "#1 1-worker-1 cause plan → scope\n#1 commented on the issue\n#1 blocked cleared\n#1 gated cleared\n") {
		t.Errorf("stdout:\n%s", m.stdout.String())
	}
	if b := a.bodies(1); b[len(b)-1] != "Rename it." {
		t.Errorf("comment %q", b[len(b)-1])
	}
	var got []map[string]string
	for _, r := range rows(t, home) {
		if r["step_or_run"] == "1-worker-1" {
			got = append(got, r)
		}
	}
	if len(got) != 2 || got[0]["cause"] != "plan" || got[1]["cause"] != "scope" || got[1]["notes"] != got[0]["notes"]+", retagged" {
		t.Fatalf("rows %v", got)
	}
	for _, c := range metrics.Header {
		if c != "cause" && c != "notes" && got[1][c] != got[0][c] {
			t.Errorf("%s: %q, held %q", c, got[1][c], got[0][c])
		}
	}

	// The worker runs again, and the block set by hand since is no run's.
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	a.forge.Relabel(issue1, "", forge.Blocked)
	held, _ = os.ReadFile(csvFile)
	before = m.writes(1)
	err := m.verb(swarmUnblock, "1", "--cause", "ruling", "Go on.")
	if want := "Error: swarm: #1: --cause: no run's stop set this block (run 1-worker-1, whose stop flagged it, ended before run 1-worker-2 started: the block was set again since, by the merge step or by hand); unblock it without --cause; nothing changed"; errText(err) != want {
		t.Errorf("%v\nwant %s", err, want)
	}
	if after, _ := os.ReadFile(csvFile); !bytes.Equal(after, held) || !slices.Equal(m.writes(1), before) {
		t.Errorf("a refusal wrote:\n%s\n%q", after, m.writes(1)[len(before):])
	}

	t.Run("a block set by hand", func(t *testing.T) {
		a := newApplyWS(t)
		home := a.metricsHome()
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:plan-1-accepted", forge.Blocked})
		m := &maintainerWS{statusWS: a.statusWS}
		m.secrets(bothTokens)
		m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
		before := m.writes(1)
		err := m.verb(swarmUnblock, "1", "--cause", "scope", "Go on.")
		if want := "Error: swarm: #1: --cause: no run's stop set this block (no run of the issue flagged it swarm:blocked); unblock it without --cause; nothing changed"; errText(err) != want {
			t.Errorf("%v\nwant %s", err, want)
		}
		if _, err := os.Stat(filepath.Join(home, "metrics", "metrics.csv")); err == nil || !slices.Equal(m.writes(1), before) {
			t.Errorf("wrote: %v %q", err, m.writes(1)[len(before):])
		}
	})
}
