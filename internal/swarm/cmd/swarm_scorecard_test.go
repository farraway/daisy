package cmd

import (
	"bytes"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// scorecard is `swarm metrics --scorecard` with the values given over the
// scratch state and the Fake forge, git run for real on the mirrors.
func (a *applyWS) scorecard(values ...Value) error {
	a.stdout.Reset()
	a.stderr.Reset()
	o := &ops.Fake{NowFn: a.Now, ExecFn: a.exec, GetenvFn: func(k string) string { return a.vars[k] }}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Flags: []string{"--scorecard"}, Values: values}
	return a.at(c, false, func(c *Context) error {
		return swarmMetrics(c, func(ops.Ops, declare.Forge, []string, string) (forge.Adapter, error) { return a.forge, nil })
	})
}

func (a *applyWS) setNow(at string) {
	a.t.Helper()
	t := mustTime(a.t, at)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.now = t
}

// scorecardCSV is the fixture of the scorecard's claim: the rows of
// acme/swarm-app's runs and interventions on issues 1 (two slices), 2
// (one) and 3 (none merged), the batch, an issue-0 intervention, a row
// superseded by a retag, and another swarm's row.
const scorecardCSV = "" +
	"2026-11-06T14:00:00Z,swarm-run,acme/swarm-app,0-owner-1,owner,anthropic,claude-opus-5-5,low,,,,posted,,5000,500,5500,,2,,,,,,,,,batch,4000,,\n" +
	"2026-11-06T15:30:00Z,swarm-run,acme/swarm-app,1-architect-1,architect,anthropic,claude-fable-5-2,high,,,,posted,,1000,100,1100,,0.5,,,,,,,,,#1,800,,\n" +
	"2026-11-06T16:00:00Z,swarm-run,acme/swarm-app,1-worker-1,worker,xai,grok-4.7,low,,,,blocked,,2000,200,2200,,1,,,,,,,,,#1,1500,,\n" +
	"2026-11-06T16:10:00Z,intervention,acme/swarm-app,1-intervention-1,maintainer,,,,,,,unblock,,,,,,,,,,,,,,,#1,,,I3\n" +
	"2026-11-06T16:20:00Z,swarm-run,acme/swarm-app,2-architect-1,architect,anthropic,claude-fable-5-2,high,,,,posted,,100,10,110,,0.01,,,,,,,,,#2,0,,\n" +
	"2026-11-06T16:30:00Z,swarm-run,acme/swarm-app,1-worker-2,worker,xai,grok-4.7,low,,,,posted,,3000,300,3300,,,,,,,,,,,#1,2000,,\n" +
	"2026-11-06T17:00:00Z,swarm-run,acme/swarm-app,1-adversary-1,adversary,openai,gpt-6.2-astra,high,,,,posted,1,500,50,550,,0.25,,,,,,,,,#1,0,1,\n" +
	"2026-11-06T17:10:00Z,swarm-run,acme/swarm-app,2-worker-1,worker,xai,grok-4.7,low,,,,question,,400,40,440,,0.02,,,,,,,,,#2,100,,scope\n" +
	"2026-11-06T17:30:00Z,swarm-run,acme/swarm-app,1-adversary-2,adversary,openai,gpt-6.2-astra,high,,,,posted,2,,,,,,,,,,,,,,#1,,0,\n" +
	"2026-11-06T17:40:00Z,swarm-run,acme/swarm-app,2-worker-2,worker,xai,grok-4.7,low,,,,failed,,,,,,,,,,,,,,,#2,,,capacity\n" +
	"2026-11-06T18:00:05Z,intervention,acme/swarm-app,1-intervention-2,maintainer,,,,,,,merge,,,,,,,,,,,,,,,#1,,,I7\n" +
	"2026-11-06T18:10:00Z,swarm-run,acme/swarm-app,2-worker-3,worker,xai,grok-4.7,low,,,,posted,,600,60,660,,0.03,,,,,,,,,#2,300,,\n" +
	"2026-11-06T16:00:00Z,swarm-run,acme/swarm-app,1-worker-1,worker,xai,grok-4.7,low,,,,blocked,,2000,200,2200,,1,,,,,,,,,\"#1, retagged\",1500,,plan\n" +
	"2026-11-06T20:00:00Z,intervention,acme/swarm-app,0-intervention-1,maintainer,,,,,,,intervene,,,,,,,,,,,,,,,the image rebuilt,,,I14\n" +
	"2026-11-07T09:00:00Z,swarm-run,acme/swarm-app,1-worker-3,worker,xai,grok-4.7,low,,,,posted,,700,70,770,,0.07,,,,,,,,,#1,0,,\n" +
	"2026-11-07T11:00:00Z,swarm-run,acme/swarm-app,3-architect-1,architect,anthropic,claude-fable-5-2,high,,,,posted,,50,5,55,,0.005,,,,,,,,,#3,0,,\n" +
	"2026-11-06T17:00:00Z,swarm-run,other/swarm,1-worker-9,worker,xai,grok-4.7,low,,,,blocked,,9000,900,9900,,9,,,,,,,,,#1,0,,plan\n"

// The claim of Step 7 of the plan implementing the 2026-11-19 swarm
// roadmap (R0; section 11, The scorecard): over a fixture CSV of runs,
// stops, interventions and a retag, merges in app's mirror (#1's slice 1
// at 18:00, #2's at 19:00, #1's slice 2 the next morning) and the Fake
// forge's issue openings, the scorecard prints a row per merged slice
// with the runs by role, the tokens, the estimate (a row without one
// counted $0 and named), the review rounds, the stops by cause (the
// retag's, a question's, Daisy's own capacity stop) and the
// interventions by type (the merge verb's on the slice it merged), and
// the summary over every record; with --since, the slices merged since
// then and a summary over the records since; with --until (Step 8a),
// neither the merges nor the records after it, in the rows or the
// summary, the window ending at it or now, whichever is earlier;
// dropping one run's row changes its slice's row; nothing is written.
func TestScorecardRowPerMergedSlice(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.setNow("2026-11-06T15:00:00Z")
	a.forge.CreateIssue("One", "one", []string{forge.ProposalAccepted})
	a.setNow("2026-11-06T16:00:00Z")
	a.forge.CreateIssue("Two", "two", []string{forge.ProposalAccepted})
	a.forge.CreateIssue("Three", "three", []string{forge.ProposalAccepted})

	work := t.TempDir()
	gitIn(t, work, "init", "-q", "-b", "main")
	gitIn(t, work, "commit", "-q", "--allow-empty", "-m", "first")
	heads := map[string]string{}
	for _, b := range []string{"swarm/1-1", "swarm/2-1", "swarm/1-2"} {
		gitIn(t, work, "checkout", "-q", "-b", b, "main")
		os.WriteFile(filepath.Join(work, strings.ReplaceAll(b, "/", "-")), []byte(b), 0o644)
		gitIn(t, work, "add", "-A")
		gitIn(t, work, "commit", "-q", "-m", b)
		heads[b] = strings.TrimSpace(gitIn(t, work, "rev-parse", "HEAD"))
		gitIn(t, work, "checkout", "-q", "main")
	}
	for _, m := range []struct{ branch, at string }{{"swarm/1-1", "2026-11-06T18:00:00Z"}, {"swarm/2-1", "2026-11-06T19:00:00Z"}, {"swarm/1-2", "2026-11-07T10:00:00Z"}} {
		cmd := osexec.Command("git", "-c", "user.name=T", "-c", "user.email=t@example.net", "merge", "-q", "--no-ff", "-m", "Merge "+m.branch, m.branch)
		cmd.Dir, cmd.Env = work, append(os.Environ(), "GIT_COMMITTER_DATE="+m.at)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git merge %s: %v %s", m.branch, err, out)
		}
	}
	gitIn(t, a.state("git/app"), "fetch", "-q", work, "+refs/heads/*:refs/heads/*")
	at := mustTime(t, "2026-11-07T12:00:00Z")
	os.Chtimes(a.state("git/app/FETCH_HEAD"), at, at)
	for run, branch := range map[string]string{"1-worker-2": "swarm/1-1", "2-worker-3": "swarm/2-1", "1-worker-3": "swarm/1-2"} {
		a.runResult(run, "branch "+branch+" in app\npushed "+heads[branch]+"\noutcome posted\ncollected\n", "")
	}

	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	held := strings.Join(metrics.Header, ",") + "\n" + scorecardCSV
	os.WriteFile(csvFile, []byte(held), 0o644)
	a.setNow("2026-11-08T14:00:00Z")

	head := "Scorecard of acme/swarm-app: runs and interventions from " + csvFile + ", merges from the mirrors as last fetched, 2026-11-07T12:00:00Z\n"
	one1 := "  #1 slice 1 in app, merged 2026-11-06T18:00:00Z, 3h0m0s after the issue's opening: runs 5 (architect 1, worker 2, adversary 2); " +
		"tokens 2200 uncached, 4300 cached, 650 out (1 run without usage); estimated $1.75 at list price (2 runs without an estimate); review rounds 2; stops plan 1; interventions I3 1, I7 1\n"
	two1 := "  #2 slice 1 in app, merged 2026-11-06T19:00:00Z, 3h0m0s after the issue's opening: runs 4 (architect 1, worker 3); " +
		"tokens 700 uncached, 400 cached, 110 out (1 run without usage); estimated $0.06 at list price (1 run without an estimate); review rounds 0; stops capacity 1, scope 1; interventions none\n"
	one2 := "  #1 slice 2 in app, merged 2026-11-07T10:00:00Z, 16h0m0s after the previous slice's merge: runs 1 (worker 1); " +
		"tokens 700 uncached, 0 cached, 70 out; estimated $0.07 at list price; review rounds 0; stops none; interventions none\n"
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	want := head + one1 + two1 + one2 +
		"Summary, every record (2.0 days): 3 slices merged; 12 runs, 4.0 per merge; input 4450 tokens per merge; worker stop rate 33% (2 of 6 worker runs); 3 interventions, 1.5 a day\n" +
		"By configuration, every record:\n" +
		"  owner anthropic claude-opus-5-5 low, prompt —: runs 1; tokens 1000 uncached, 4000 cached, 500 out; estimated $2.00 at list price; stops 0\n" +
		"  architect anthropic claude-fable-5-2 high, prompt —: runs 3; tokens 350 uncached, 800 cached, 115 out; estimated $0.52 at list price; stops 0\n" +
		"  worker xai grok-4.7 low, prompt —: runs 6; tokens 2800 uncached, 3900 cached, 670 out (1 run without usage); estimated $1.12 at list price (2 runs without an estimate); stops 3\n" +
		"  adversary openai gpt-6.2-astra high, prompt —: runs 2; tokens 500 uncached, 0 cached, 50 out (1 run without usage); estimated $0.25 at list price (1 run without an estimate); stops 0; review effect 1 (2 of 2 runs known)\n"
	if got := a.stdout.String(); got != want || a.stderr.Len() != 0 {
		t.Errorf("stdout:\n%s\nwant:\n%s\nstderr:\n%s", got, want, a.stderr.String())
	}

	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T19:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	want = head + two1 + one2 +
		"Summary, since 2026-11-06T19:00:00Z (1.8 days): 2 slices merged; 2 runs, 1.0 per merge; input 375 tokens per merge; worker stop rate 0% (0 of 1 worker runs); 1 interventions, 0.6 a day\n" +
		"By configuration, since 2026-11-06T19:00:00Z:\n" +
		"  architect anthropic claude-fable-5-2 high, prompt —: runs 1; tokens 50 uncached, 0 cached, 5 out; estimated $0.01 at list price; stops 0\n" +
		"  worker xai grok-4.7 low, prompt —: runs 1; tokens 700 uncached, 0 cached, 70 out; estimated $0.07 at list price; stops 0\n"
	if got := a.stdout.String(); got != want {
		t.Errorf("--since: stdout:\n%s\nwant:\n%s", got, want)
	}

	// --until at #1's first merge: the merges after it and the records
	// after it, the merge verb's intervention five seconds later among
	// them, are left out of the rows and the summary, the window ending
	// at it; an --until after now ends the window now.
	if err := a.scorecard(Value{Flag: "--until", Value: "2026-11-06T18:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	want = head + strings.Replace(one1, "interventions I3 1, I7 1", "interventions I3 1", 1) +
		"Summary, until 2026-11-06T18:00:00Z (0.2 days): 1 slices merged; 9 runs, 9.0 per merge; input 12000 tokens per merge; worker stop rate 50% (2 of 4 worker runs); 1 interventions, 6.0 a day\n" +
		"By configuration, until 2026-11-06T18:00:00Z:\n" +
		"  owner anthropic claude-opus-5-5 low, prompt —: runs 1; tokens 1000 uncached, 4000 cached, 500 out; estimated $2.00 at list price; stops 0\n" +
		"  architect anthropic claude-fable-5-2 high, prompt —: runs 2; tokens 300 uncached, 800 cached, 110 out; estimated $0.51 at list price; stops 0\n" +
		"  worker xai grok-4.7 low, prompt —: runs 4; tokens 1800 uncached, 3600 cached, 540 out (1 run without usage); estimated $1.02 at list price (2 runs without an estimate); stops 3\n" +
		"  adversary openai gpt-6.2-astra high, prompt —: runs 2; tokens 500 uncached, 0 cached, 50 out (1 run without usage); estimated $0.25 at list price (1 run without an estimate); stops 0; review effect 1 (2 of 2 runs known)\n"
	if got := a.stdout.String(); got != want {
		t.Errorf("--until: stdout:\n%s\nwant:\n%s", got, want)
	}
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T19:00:00Z"}, Value{Flag: "--until", Value: "2026-12-01T00:00:00+01:00"}); err != nil {
		t.Fatal(err)
	}
	want = head + two1 + one2 +
		"Summary, since 2026-11-06T19:00:00Z until 2026-11-30T23:00:00Z (1.8 days): 2 slices merged; 2 runs, 1.0 per merge; input 375 tokens per merge; worker stop rate 0% (0 of 1 worker runs); 1 interventions, 0.6 a day\n" +
		"By configuration, since 2026-11-06T19:00:00Z until 2026-11-30T23:00:00Z:\n" +
		"  architect anthropic claude-fable-5-2 high, prompt —: runs 1; tokens 50 uncached, 0 cached, 5 out; estimated $0.01 at list price; stops 0\n" +
		"  worker xai grok-4.7 low, prompt —: runs 1; tokens 700 uncached, 0 cached, 70 out; estimated $0.07 at list price; stops 0\n"
	if got := a.stdout.String(); got != want {
		t.Errorf("--until after now: stdout:\n%s\nwant:\n%s", got, want)
	}
	if b, _ := os.ReadFile(csvFile); string(b) != held {
		t.Errorf("the CSV was written:\n%s", b)
	}

	// One run's row dropped: its slice's row reads without it.
	os.WriteFile(csvFile, bytes.Replace([]byte(held), []byte("2026-11-06T17:10:00Z,swarm-run,acme/swarm-app,2-worker-1,worker,xai,grok-4.7,low,,,,question,,400,40,440,,0.02,,,,,,,,,#2,100,,scope\n"), nil, 1), 0o644)
	if err := a.scorecard(); err != nil {
		t.Fatal(err)
	}
	if want := "  #2 slice 1 in app, merged 2026-11-06T19:00:00Z, 3h0m0s after the issue's opening: runs 3 (architect 1, worker 2); " +
		"tokens 400 uncached, 300 cached, 70 out (1 run without usage); estimated $0.04 at list price (1 run without an estimate); review rounds 0; stops capacity 1; interventions none\n"; !strings.Contains(a.stdout.String(), want) {
		t.Errorf("one run dropped: stdout:\n%s\nwant within:\n%s", a.stdout.String(), want)
	}
}

// --scorecard is one action of the verb: with another, or --since or
// --until without it, usage; a --since or --until that is not RFC 3339,
// an error; no home named, the backfill's error; no slice merged, a
// line saying so; --until before --since, usage before anything is read.
func TestScorecardRefusals(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	for _, flags := range [][]string{{"--scorecard", "--reprice"}, {"--scorecard", "--dry-run"}} {
		var usage UsageError
		if err := a.metricsVerb(flags...); !errors.As(err, &usage) {
			t.Errorf("swarm metrics %v = %v, want a usage error", flags, err)
		}
	}
	o := &ops.Fake{NowFn: a.Now, ExecFn: a.exec}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &a.stdout, Stderr: &a.stderr, Flags: []string{"--backfill"}, Values: []Value{{Flag: "--since", Value: "2026-11-06T00:00:00Z"}}}
	var usage UsageError
	if err := a.at(c, false, func(c *Context) error { return swarmMetrics(c, openForge) }); !errors.As(err, &usage) || string(usage) != "--since goes with --scorecard" {
		t.Errorf("--since with --backfill = %v", err)
	}
	c.Values = []Value{{Flag: "--until", Value: "2026-11-06T00:00:00Z"}}
	if err := a.at(c, false, func(c *Context) error { return swarmMetrics(c, openForge) }); !errors.As(err, &usage) || string(usage) != "--until goes with --scorecard" {
		t.Errorf("--until with --backfill = %v", err)
	}
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06"}); errText(err) != "Error: --since must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: 2026-11-06)" {
		t.Errorf("a date for --since = %v", err)
	}
	if err := a.scorecard(Value{Flag: "--until", Value: "tomorrow"}); errText(err) != "Error: --until must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: tomorrow)" {
		t.Errorf("a word for --until = %v", err)
	}
	if err := a.scorecard(); err != nil || a.stdout.String() != "Scorecard of acme/swarm-app: runs and interventions from "+filepath.Join(home, "metrics", "metrics.csv")+
		", merges from no mirror fetched\n  no slice merged\nSummary, every record (no record): 0 slices merged; 0 runs, — per merge; input — tokens per merge; worker stop rate — (0 of 0 worker runs); 0 interventions, — a day\nBy configuration, every record:\n  no run\n" {
		t.Errorf("no record = %v\n%s", err, a.stdout.String())
	}
	os.Remove(a.state("home"))
	if err := a.scorecard(); errText(err) != "Error: swarm metrics --scorecard: "+a.state("home")+" names no Daisy home, whose metrics CSV would take the rows; write a registered home's name in it" {
		t.Errorf("no home = %v", err)
	}
	// Before anything is read: with no home named, the window still refused.
	usage = ""
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T12:00:00+01:00"}, Value{Flag: "--until", Value: "2026-11-06T10:59:59Z"}); !errors.As(err, &usage) || string(usage) != "--until 2026-11-06T10:59:59Z is before --since 2026-11-06T12:00:00+01:00: the window would hold nothing" {
		t.Errorf("--until before --since = %v", err)
	}
}

// R3-12's claim, the scorecard's side (the plan implementing the
// 2026-11-19 swarm roadmap; section 11, The scorecard): the window's
// runs grouped by role and configuration (provider, model, effort,
// prompt hash), the roles in their order and each role's
// configurations in the order of their first run: the adversary's runs
// under two prompt hashes are two rows, each with its runs, tokens,
// estimate, stops and review effect (its known effects summed, and how
// many are known); a run before --since is no row's.
func TestScorecardByConfiguration(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	ha, hb, hc := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	csvFile := filepath.Join(home, "metrics", "metrics.csv")
	os.MkdirAll(filepath.Dir(csvFile), 0o755)
	os.WriteFile(csvFile, []byte(strings.Join(metrics.Header, ",")+"\n"+
		"2026-11-06T10:00:00Z,swarm-run,acme/swarm-app,1-worker-1,worker,xai,grok-4.7,low,,,"+hc+",posted,,900,90,990,,0.09,,,,,,,,,#1,0,,\n"+
		"2026-11-06T17:00:00Z,swarm-run,acme/swarm-app,1-adversary-1,adversary,openai,gpt-6.2-astra,high,,,"+ha+",posted,1,500,50,550,,0.25,,,,,,,,,#1,100,1,\n"+
		"2026-11-06T17:10:00Z,swarm-run,acme/swarm-app,2-worker-1,worker,xai,grok-4.7,low,,,"+hc+",blocked,,200,20,220,,0.02,,,,,,,,,#2,0,,plan\n"+
		"2026-11-06T17:20:00Z,swarm-run,acme/swarm-app,2-adversary-1,adversary,openai,gpt-6.2-astra,high,,,"+hb+",posted,1,400,40,440,,,,,,,,,,,#2,0,,\n"+
		"2026-11-06T17:30:00Z,swarm-run,acme/swarm-app,1-adversary-2,adversary,openai,gpt-6.2-astra,high,,,"+ha+",posted,2,300,30,330,,0.1,,,,,,,,,#1,0,0,\n"), 0o644)
	a.setNow("2026-11-07T00:00:00Z")
	if err := a.scorecard(Value{Flag: "--since", Value: "2026-11-06T12:00:00Z"}); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	want := "By configuration, since 2026-11-06T12:00:00Z:\n" +
		"  worker xai grok-4.7 low, prompt cccccccccccc: runs 1; tokens 200 uncached, 0 cached, 20 out; estimated $0.02 at list price; stops 1\n" +
		"  adversary openai gpt-6.2-astra high, prompt aaaaaaaaaaaa: runs 2; tokens 700 uncached, 100 cached, 80 out; estimated $0.35 at list price; stops 0; review effect 1 (2 of 2 runs known)\n" +
		"  adversary openai gpt-6.2-astra high, prompt bbbbbbbbbbbb: runs 1; tokens 400 uncached, 0 cached, 40 out; estimated $0.00 at list price (1 run without an estimate); stops 0; review effect 0 (0 of 1 runs known)\n"
	if got := a.stdout.String(); !strings.HasSuffix(got, want) {
		t.Errorf("stdout:\n%s\nwant ending:\n%s", got, want)
	}
}
