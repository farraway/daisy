package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/metrics"
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
