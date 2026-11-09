package reconcile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

var t0 = time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)

// clock is an Ops whose clock starts at t0 and moves a minute per read,
// so the Fake's entries are ordered.
func clock() ops.Ops {
	n := 0
	return &ops.Fake{NowFn: func() time.Time { n++; return t0.Add(time.Duration(n) * time.Minute) }}
}

func cfg() declare.Config {
	return declare.Config{
		Forge: declare.Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/app", CommitEmail: "swarm@example.net"},
		Repos: []declare.Repo{
			{Name: "app", Path: "app", Check: []string{"make", "check"}, Image: "daisy-member-app:2", Protected: []string{".gitea/", "AGENTS.md"}},
			{Name: "lib", Path: "lib", Check: []string{}, Image: "daisy-member:1", Protected: []string{}},
		},
		Base: "main", MaxRounds: 5, Timeout: "45m", Resources: []string{"docs/"}, Trusted: []string{"jane", "swarm-bot"},
		Gates: declare.Gates{Triage: true, Plan: true, Merge: true},
	}
}

// fixture is the Fake with issue 1, opened by the owner's run 2 toward a
// brief, a trusted comment carrying a heading and an untrusted one.
func fixture() (*forge.Fake, forge.Ref) {
	f := forge.NewFake(clock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	f.AddLabels(forge.Labels...)
	n, _ := f.CreateIssue("Drift check", "<!-- daisy-swarm role=owner run=2 -->\n**owner:**\n\nBrief: docs/briefs/b.md — M0\n\n# Why\n\nIt drifts.", []string{"swarm:proposal-accepted"})
	at := forge.Ref{Repo: "app", N: n}
	f.Say(at, "jane", "Yes.\n## Scope\nsmall")
	f.Say(at, "mallory", "evil")
	return f, at
}

func job(f *forge.Fake, n int, role, label string) *Job {
	is, _ := f.Issue(n)
	pulls, _ := f.Pulls(n)
	return &Job{Cfg: cfg(), Host: "forge.example.net", SwarmLogin: "swarm-bot", N: n, K: 1, Role: role, Label: label, Time: t0,
		Persona: "# " + role + "\n\nYou do it.\n\n```\n# not a heading\n```\n", Issue: is, Pulls: pulls}
}

// The architect's two files on ready, byte for byte (section 7.3 and 7.4):
// the persona's headings shifted and its fence left alone; the Daisy-plan
// Task with the repository prefix and each repository's protected paths;
// one Clone line per repository; Opened by owner without the marker
// lines; the trusted comment's heading shifted under its entry; the
// untrusted one's content omitted.
func TestRenderArchitectReady(t *testing.T) {
	f, at := fixture()
	sys, prompt := Render(job(f, at.N, "architect", "swarm:proposal-accepted"))
	if sys != wantArchitectSystem {
		t.Errorf("system.md:\n%s", sys)
	}
	if prompt != wantArchitectPrompt {
		t.Errorf("prompt.md:\n%s", prompt)
	}
}

const wantArchitectSystem = `<!-- daisy-swarm system: acme/app#1 architect run 1, 2026-11-06T15:00:00Z -->

# Persona

## architect

You do it.

` + "```" + `
# not a heading
` + "```" + `

# Task

Write the plan for issue #1 as a Daisy plan, under these "## " headings in this order: "## Research" (what exists: the files and seams the job touches, what connects to what, and what the change affects, each line naming a path under /work), "## Goal", "## Non-goals", "## Constraints", "## Steps" (the slices: a numbered list, "1. " and so on, one item per pull request in the order they are built, each beginning with the name of the repository it changes and a colon ("1. <name>: "), one of app, lib, then the files it changes and the new or changed types and signatures, its claim, and the test that proves the claim, its lines after the first indented so that under "## Steps" only a slice begins a line with a number and a dot; one item when the job is one pull request; a slice that moves a pin to another slice's change comes after that slice), "## Risks", "## Test Plan" (the plan's acceptance: numbered claims, each with the check that proves it and where its test lives), "## Decisions" (each design decision the resources do not record, with its alternative), "## Deferred" and "## Open Questions". When a slice touches more than one component, add a Mermaid diagram of it (a flowchart or a sequence diagram) in a fenced "mermaid" block under that step. The maintainer reads "## Research" first, then the plan, before any code is written. These paths are protected: app: .gitea/, AGENTS.md. No change to them is pushed; the maintainer makes it. Leave such a change out and say so.

Write your output to /work/.swarm/out.md; that file is the only thing read back, and nothing you write elsewhere outside /work survives. If you cannot do this without an answer from the maintainer, write ` + "`" + `BLOCKED: <your question>` + "`" + ` as the first line of /work/.swarm/out.md instead.
`

const wantArchitectPrompt = `<!-- daisy-swarm prompt: acme/app#1 architect run 1, 2026-11-06T15:00:00Z -->

# Repository

- Issues: acme/app on forge.example.net
- Clone /work/app: acme/app, branch main, base main, check make check
- Clone /work/lib: acme/lib, branch main, base main, no check declared
- Resources (read-only): /resources/docs

# Issue #1: Drift check

Opened by owner, run 2, at 2026-11-06T15:01:00Z. Labels: swarm:proposal-accepted.

Brief: docs/briefs/b.md — M0

## Why

It drifts.

# Thread

## jane at 2026-11-06T15:02:00Z

Yes.
#### Scope
small

## mallory at 2026-11-06T15:03:00Z (not trusted; content omitted)
`

// has reports each part missing from text, and each forbidden one in it.
func has(t *testing.T, what, text string, parts []string, forbidden ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("%s lacks %q:\n%s", what, p, text)
		}
	}
	for _, p := range forbidden {
		if strings.Contains(text, p) {
			t.Errorf("%s holds %q:\n%s", what, p, text)
		}
	}
}

const archPlan = "## Research\n\n- /work/app/x.go\n\n## Steps\n\n1. lib: the library.\n2. app: the pin.\n\n```mermaid\n1. app: not a slice\n```\n\n## Test Plan\n\n1. it works.\n"

// planned is the fixture with the architect's plan accepted, slice 1 open
// in lib on head aaa, reviewed once by both, a line comment by jane, and
// the check of the worker run that pushed it under state.
func planned(t *testing.T) (*forge.Fake, forge.Ref, forge.Ref, string) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	f.Say(at, "jane", "Go ahead.")
	m, err := f.CreatePull("lib", "swarm/1-1", "main", "#1 slice 1: Drift check", marker("worker", 1)+"Did it.\n\nRefs acme/app#1")
	if err != nil {
		t.Fatal(err)
	}
	pr := forge.Ref{Repo: "lib", N: m}
	f.Push(pr, "aaa")
	f.Review(pr, forge.Commented, marker("adversary", 1)+"VERDICT: request-changes\n[blocking] x.go:1 wrong")
	f.Review(pr, forge.Commented, marker("sympathetic", 1)+"VERDICT: approve\nfine")
	f.SayOnLine(pr, "jane", "x.go", 3, "why?")
	state := t.TempDir()
	run := filepath.Join(state, "runs", "1-worker-1")
	os.MkdirAll(run, 0o755)
	os.WriteFile(filepath.Join(run, "result"), []byte("pushed aaa\ncheck exited 2 2026-11-06T15:00:00Z\n"), 0o644)
	var out []string
	for i := 1; i <= 25; i++ {
		out = append(out, "line "+itoa(i))
	}
	os.WriteFile(filepath.Join(run, "check.out"), []byte(strings.Join(out, "\n")+"\n"), 0o644)
	return f, at, pr, state
}

func thread(f *forge.Fake, pr forge.Ref) *PullThread {
	cs, _ := f.PullComments(pr)
	rs, _ := f.Reviews(pr)
	ls, _ := f.ReviewComments(pr)
	return &PullThread{cs, rs, ls}
}

// The architect on planning while its slice is open (after T11): the
// revise text with the open-slice sentence; the pull request, its thread,
// the check's last 20 lines and the earlier pull requests in its prompt.
func TestRenderArchitectOpenSlice(t *testing.T) {
	f, at, pr, state := planned(t)
	j := job(f, at.N, "architect", "swarm:plan-0-posted")
	j.Pull = thread(f, pr)
	runs, _ := Runs(ops.Real().FS(), state)
	j.Check = CheckSection(ops.Real().FS(), runs, 1, "aaa", []string{"make", "check"}, "45m")
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{`Revise your plan for issue #1 in answer to the latest comment.`,
		`Pull request lib#1, slice 1, is open: it reached the review limit, or the maintainer returned the issue to you, and its reviews under "# Pull request" are the input.`})
	has(t, "prompt.md", prompt, []string{"# Earlier pull requests\n\nNone.\n\n# Pull request lib#1: #1 slice 1: Drift check\n\nBranch swarm/1-1 into main; head aaa.\n\nDid it.\n\nRefs acme/app#1\n\n## Thread\n\n",
		"### adversary, run 1, at 2026-11-06T15:07:00Z, on aaa: request-changes\n\n[blocking] x.go:1 wrong\n\n### sympathetic, run 1, at 2026-11-06T15:08:00Z, on aaa: approve\n\n### jane at 2026-11-06T15:09:00Z, on x.go:3 at aaa\n\nwhy?",
		"# Check\n\nmake check on aaa: exit 2\n\n    line 6\n", "    line 25\n"},
		"line 5\n", "<!-- daisy-swarm role=worker", "VERDICT:", "## Earlier rounds")
}

// The worker on planned, slice 2 after slice 1 merged: its repository from
// the plan, the pin sentence, Not established, the protected paths, the
// plan and the Direction since it, slice 1 under Earlier with its merge
// commit, no pull request yet.
func TestRenderWorkerNextSlice(t *testing.T) {
	f, at, pr, _ := planned(t)
	if err := f.Merge(pr, "aaa"); err != nil {
		t.Fatal(err)
	}
	pulls, _ := f.Pulls(1)
	j := job(f, at.N, "worker", "swarm:plan-1-accepted")
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{
		`Implement slice 2 of the accepted plan for issue #1 (it declares 2) in acme/app, on the branch checked out in /work (swarm/1-2).`,
		`Commit with git; do not push.`, `Run "make check" before you finish and quote its last result.`,
		`End what you write with a line "Not established:"`,
		`write "BLOCKED: slice <i> is not merged".`, `These paths are protected: .gitea/, AGENTS.md.`, mergeBase})
	has(t, "prompt.md", prompt, []string{"- Clone /work: acme/app, branch swarm/1-2, base main, check make check\n",
		"# Plan\n\n## architect, run 1, at 2026-11-06T15:04:00Z: slice 2 of 2, in app\n\n#### Research\n",
		"```mermaid\n1. app: not a slice\n```",
		"# Direction\n\n## jane at 2026-11-06T15:05:00Z\n\nGo ahead.\n\n# Earlier pull requests\n\n- lib#1 swarm/1-1 merged " + pulls[0].MergeSHA + ": #1 slice 1: Drift check\n"},
		"# Pull request", "# Check", "Yes.")
}

// The fast lane: no plan, the issue body and the comments are the job.
func TestRenderWorkerFastLane(t *testing.T) {
	f, at := fixture()
	j := job(f, at.N, "worker", "swarm:plan-1-accepted")
	j.Cfg.Gates.Merge = false
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{`There is no plan: the issue body and the maintainer's comments under Direction are the job of issue #1.`,
		mergeBase + ` Commit with git`},
		"slice <i>")
	has(t, "prompt.md", prompt, []string{"# Plan\n\nNone: there is no plan.", "# Direction\n\n## jane at", "- Clone /work: acme/app, branch swarm/1-1,"})
}

// The adversary on round 2: the other reviewer's review of the head left
// out, its own of the previous head and the round sentence in, the
// named-test sentence.
func TestRenderAdversaryRoundTwo(t *testing.T) {
	f, at, pr, _ := planned(t)
	f.Push(pr, "bbb")
	f.Review(pr, forge.Commented, marker("sympathetic", 2)+"VERDICT: approve\nstill fine")
	pulls, _ := f.Pulls(1)
	j := job(f, at.N, "adversary", "swarm:review-0-ready")
	j.Pulls, j.Pull, j.Check = pulls, thread(f, pr), "No check result for this head."
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{`Review pull request lib#1, slice 1 of issue #1. The change is "git diff main...HEAD" in /work`,
		`For each test that slice 1's item under the plan's Steps names`,
		`This is round 2. First say, for each blocking finding of your review of aaa, whether it is resolved, or answered with a reason you accept; raise new blocking findings only on code changed since ("git diff aaa..HEAD").`},
		"These paths are protected")
	has(t, "prompt.md", prompt, []string{"- Clone /work: acme/lib, branch swarm/1-1, base main, no check declared", "on aaa: request-changes", "# Check\n\nNo check result for this head."},
		"still fine", "# Direction")
}

// A head the worker answered without a commit (NO CHANGE:) is reviewed
// again: round 2 on the same head, its Task text telling the reviewer to
// judge the answer against its previous review of that head and to raise
// nothing new; the worker's Task text names the form.
func TestRenderAdversaryNoChange(t *testing.T) {
	f, at, pr, _ := planned(t)
	f.Comment(pr, marker("worker", 2)+"NO CHANGE: x.go:1 is right; the plan says so.\n\nNot established: none")
	pulls, _ := f.Pulls(1)
	j := job(f, at.N, "adversary", "swarm:review-0-ready")
	j.Pulls, j.Pull = pulls, thread(f, pr)
	sys, _ := Render(j)
	has(t, "system.md", sys, []string{`This is round 2, on the head you reviewed before (aaa): the worker answered in the thread with "NO CHANGE:" and no commit. First say, for each blocking finding of your previous review, whether the answer resolves it with a reason you accept; raise no new blocking finding, since the code has not changed.`},
		"This is round 2. First say")
	w := job(f, at.N, "worker", "swarm:review-3-changes-requested")
	w.Pulls, w.Pull = pulls, thread(f, pr)
	sys, _ = Render(w)
	has(t, "worker system.md", sys, []string{`If none needs a change to the code, make no commit and write "NO CHANGE: <why>" as the first line of your output, then your answer point by point: the reviewers judge it against the unchanged head.`})
}

// filler is an entry's prose: about 1.8kB, the size of a real review's
// or report's reasoning, never a finding.
var filler = strings.Repeat("The run explains what it read and why at length. ", 36)

// reviewed is the planned fixture's issue with n rounds on its pull
// request: on head c<r>, the adversary's review with two blocking
// findings, the sympathetic's with one, each with a non-blocking one and
// its reasoning, then the worker's report <r+1> on head c<r+1>; jane's
// comment and line comment after round 1. The description is report 1.
func reviewed(t *testing.T, n int) (*forge.Fake, forge.Ref, forge.Ref) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	m, err := f.CreatePull("lib", "swarm/1-1", "main", "#1 slice 1: Drift check", marker("worker", 1)+"Report 1.\n\n"+filler+"\n\nRefs acme/app#1")
	if err != nil {
		t.Fatal(err)
	}
	pr := forge.Ref{Repo: "lib", N: m}
	f.Push(pr, "c1")
	for r := 1; r <= n; r++ {
		f.Review(pr, forge.Commented, marker("adversary", r)+"VERDICT: request-changes\n[blocking] a.go:"+itoa(r)+" finding A"+itoa(r)+"\n[blocking] b.go:"+itoa(r)+" finding B"+itoa(r)+"\n[non-blocking] c.go:1 a nit\n\n"+filler)
		f.Review(pr, forge.Commented, marker("sympathetic", r)+"VERDICT: request-changes\n[blocking] s.go:"+itoa(r)+" finding S"+itoa(r)+"\n[non-blocking] c.go:2 a nit\n\n"+filler)
		if r == 1 {
			f.Say(pr, "jane", "Keep the API as it is.")
			f.SayOnLine(pr, "jane", "a.go", 7, "Not here.")
		}
		f.Push(pr, "c"+itoa(r+1))
		f.Comment(pr, marker("worker", r+1)+"Report "+itoa(r+1)+".\n\n"+filler)
	}
	return f, at, pr
}

// jobOn is the role's job on the reviewed fixture.
func jobOn(f *forge.Fake, at forge.Ref, pr forge.Ref, role, label string) *Job {
	pulls, _ := f.Pulls(at.N)
	j := job(f, at.N, role, label)
	j.Pulls, j.Pull, j.Check = pulls, thread(f, pr), "No check result for this head."
	return j
}

// fullThread is the length of every entry's text in the pull request's
// thread: what a prompt carrying the whole thread would hold.
func fullThread(j *Job) int {
	n := 0
	cur, _ := j.current()
	for _, e := range j.pullEntries(cur) {
		n += len(e.Body)
	}
	return n
}

// The review prompt on round 7 (section 7.3): the plan, the description,
// the worker's newest report, the previous round's blocking findings
// without the reviews' bodies, every earlier round as one line, and the
// maintainer's comment and line comment in full; under a quarter of the
// thread it summarises.
func TestRenderReviewCompact(t *testing.T) {
	f, at, pr := reviewed(t, 6)
	j := jobOn(f, at, pr, "adversary", "swarm:review-0-ready")
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{"This is round 7. First say, for each blocking finding of your review of c6,"})
	has(t, "prompt.md", prompt, []string{"# Plan\n\n## architect, run 1,",
		"Branch swarm/1-1 into main; head c7.\n\nReport 1.\n\n",
		"## Earlier rounds\n\n- round 1: 3 blocking, revised in c2\n- round 2: 3 blocking, revised in c3\n- round 3: 3 blocking, revised in c4\n- round 4: 3 blocking, revised in c5\n- round 5: 3 blocking, revised in c6\n\n## Thread\n\n### jane at ",
		"\n\nKeep the API as it is.\n\n### jane at ", ", on a.go:7 at c1\n\nNot here.\n\n### adversary, run 6, at ",
		", on c6: request-changes\n\n[blocking] a.go:6 finding A6\n[blocking] b.go:6 finding B6\n\n### sympathetic, run 6, at ",
		", on c6: request-changes\n\n[blocking] s.go:6 finding S6\n\n### worker, run 7, at ",
		"\n\nReport 7.\n\n" + filler + "\n\n# Check\n"},
		"Report 6.", "finding A5", "[non-blocking]", "VERDICT:", "round 6:")
	if full := fullThread(j); len(prompt)*4 >= full {
		t.Errorf("prompt.md is %d bytes, not under a quarter of the thread's %d", len(prompt), full)
	} else {
		t.Logf("round 7: prompt.md %d bytes, the thread %d", len(prompt), full)
	}
}

// A review prompt's length stops growing with the rounds: from round 5
// to 7 it grows by the two round lines, while the thread grows by four
// reviews and two reports.
func TestRenderReviewStopsGrowing(t *testing.T) {
	var sizes, threads []int
	for r := 5; r <= 7; r++ {
		f, at, pr := reviewed(t, r-1)
		j := jobOn(f, at, pr, "sympathetic", "swarm:review-1-in-progress")
		_, prompt := Render(j)
		sizes, threads = append(sizes, len(prompt)), append(threads, fullThread(j))
	}
	t.Logf("rounds 5, 6, 7: prompt.md %v bytes, the thread %v", sizes, threads)
	if d := sizes[2] - sizes[0]; d < 0 || d > 2*len("- round 5: 3 blocking, revised in c6\n") {
		t.Errorf("prompt.md grew by %d bytes from round 5 to 7: %v", d, sizes)
	}
	if d := threads[2] - threads[0]; d < 4*len(filler) {
		t.Errorf("the fixture's thread grew by %d bytes only: %v", d, threads)
	}
}

// The worker's revision prompt: the latest round's reviews in full, the
// rest as lines, its own newest report and the maintainer's entries.
func TestRenderWorkerRevisionCompact(t *testing.T) {
	f, at, pr := reviewed(t, 3)
	f.Review(pr, forge.Commented, marker("adversary", 4)+"VERDICT: request-changes\n[blocking] a.go:4 finding A4\n\nWhy A4.")
	f.Review(pr, forge.Commented, marker("sympathetic", 4)+"VERDICT: approve\n[non-blocking] c.go:9 later\n\nWhy not.")
	j := jobOn(f, at, pr, "worker", "swarm:review-3-changes-requested")
	_, prompt := Render(j)
	has(t, "prompt.md", prompt, []string{
		"## Earlier rounds\n\n- round 1: 3 blocking, revised in c2\n- round 2: 3 blocking, revised in c3\n- round 3: 3 blocking, revised in c4\n\n## Thread\n\n### jane at ",
		"\n\nKeep the API as it is.\n\n", "\n\nNot here.\n\n### worker, run 4, at ", "\n\nReport 4.\n\n",
		", on c4: request-changes\n\nVERDICT: request-changes\n[blocking] a.go:4 finding A4\n\nWhy A4.\n\n### sympathetic, run 4, at ",
		", on c4: approve\n\nVERDICT: approve\n[non-blocking] c.go:9 later\n\nWhy not.\n\n# Check"},
		"Report 3.", "finding A3", "### adversary, run 3")
}

// The architect's revision prompt keeps every round's blocking findings
// (it reworks the plan from them) but not the reviews' bodies, and has
// no round lines.
func TestRenderArchitectKeepsEveryFinding(t *testing.T) {
	f, at, pr := reviewed(t, 5)
	j := jobOn(f, at, pr, "architect", "swarm:plan-0-posted")
	_, prompt := Render(j)
	var findings []string
	for r := 1; r <= 5; r++ {
		findings = append(findings, ", on c"+itoa(r)+": request-changes\n\n[blocking] a.go:"+itoa(r)+" finding A"+itoa(r)+"\n[blocking] b.go:"+itoa(r)+" finding B"+itoa(r)+"\n\n### sympathetic, run "+itoa(r)+", at ")
	}
	has(t, "prompt.md", prompt, append(findings, "\n\nKeep the API as it is.\n\n", "\n\nReport 6.\n\n"),
		"## Earlier rounds", "[non-blocking]", "VERDICT:", "Report 5.", "The run explains what it read and why at length. ###")
	if strings.Count(prompt, filler) != 2 { // the description and the newest report
		t.Errorf("prompt.md holds %d bodies:\n%s", strings.Count(prompt, filler), prompt)
	}
}

// An answer without a commit ends its round as a line says, and the
// previous round on the same head shows both reviewers' findings: the
// other reviewer's review of the head before the answer is the previous
// round's, not this one's.
func TestRenderRoundLinesAnswer(t *testing.T) {
	f, at, pr, _ := planned(t)
	f.Comment(pr, marker("worker", 2)+"NO CHANGE: x.go:1 is right.\n\nNot established: none")
	j := jobOn(f, at, pr, "sympathetic", "swarm:review-0-ready")
	_, prompt := Render(j)
	has(t, "prompt.md", prompt, []string{"## Thread\n\n### adversary, run 1, at 2026-11-06T15:07:00Z, on aaa: request-changes\n\n[blocking] x.go:1 wrong\n\n### sympathetic, run 1, at 2026-11-06T15:08:00Z, on aaa: approve\n\n### jane at 2026-11-06T15:09:00Z, on x.go:3 at aaa\n\nwhy?\n\n### worker, run 2, at 2026-11-06T15:10:00Z\n\nNO CHANGE: x.go:1 is right."},
		"## Earlier rounds")
	f.Review(pr, forge.Commented, marker("adversary", 2)+"VERDICT: request-changes\n[blocking] x.go:1 still wrong")
	f.Review(pr, forge.Commented, marker("sympathetic", 2)+"VERDICT: approve\nfine")
	f.Push(pr, "bbb")
	f.Comment(pr, marker("worker", 3)+"Fixed x.go:1.")
	j = jobOn(f, at, pr, "adversary", "swarm:review-0-ready")
	_, prompt = Render(j)
	has(t, "prompt.md", prompt, []string{"## Earlier rounds\n\n- round 1: 1 blocking, answered without a commit\n\n## Thread\n\n### jane at 2026-11-06T15:09:00Z, on x.go:3 at aaa\n\nwhy?\n\n### adversary, run 2, at 2026-11-06T15:11:00Z, on aaa: request-changes\n\n[blocking] x.go:1 still wrong\n\n### sympathetic, run 2, at 2026-11-06T15:12:00Z, on aaa: approve\n\n### worker, run 3, at 2026-11-06T15:13:00Z\n\nFixed x.go:1.\n\n# Check"},
		"NO CHANGE:", "x.go:1 wrong")
}

// The owner on a proposed issue without a counted comment, and the batch
// over a resources tree: the active brief in, a done one and a file
// without Status left out; None. for each empty list.
func TestRenderOwner(t *testing.T) {
	f := forge.NewFake(clock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	f.AddLabels(forge.Labels...)
	n, _ := f.CreateIssue("Idea", "Do it.", []string{"swarm:proposal-posted"})
	f.Say(forge.Ref{Repo: "app", N: n}, "mallory", "hi")
	j := job(f, n, "owner", "swarm:proposal-posted")
	j.Cfg.Gates.Triage = false
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{`Issue #1 has no comment yet: say whether its body is a job the swarm can do and is worth doing next, and what is unclear in it. Then restate the whole job as it now stands: the architect plans from your answer.`})
	has(t, "prompt.md", prompt, []string{"Opened by swarm-bot at "})

	ws := t.TempDir()
	for name, text := range map[string]string{"docs/briefs/a.md": "# A\nStatus: active\nFirst.\n\n## M1\n\nDo.\n", "docs/briefs/b.md": "Status: done\n", "docs/notes.md": "# Notes\n"} {
		os.MkdirAll(filepath.Join(ws, filepath.Dir(name)), 0o755)
		os.WriteFile(filepath.Join(ws, name), []byte(text), 0o644)
	}
	briefs, err := brief.Find(ops.Real().FS(), ws, []string{"docs/"})
	if err != nil {
		t.Fatal(err)
	}
	b := job(f, 0, "owner", "")
	b.Batch = &Batch{Briefs: briefs, Propose: 3}
	sys, prompt = Render(b)
	has(t, "batch system.md", sys, []string{`Propose up to 3 new jobs for acme/app, most important first, toward the first milestone`})
	has(t, "batch prompt.md", prompt, []string{"# Briefs\n\n## docs/briefs/a.md (Status: active; mounted at /resources/docs/briefs/a.md)\n\n### A\nStatus: active\nFirst.\n\n#### M1\n",
		"# Open swarm issues\n\nNone.\n\n# Recently closed\n\nNone.\n"}, "b.md", "notes.md", "# Issue", "# Thread")
	b.Batch.Briefs = nil
	sys, prompt = Render(b)
	has(t, "batch without briefs", sys+prompt, []string{"from the resources and the code. Start each", "# Briefs\n\nNone."})
}

// mergeBase is the worker's first instruction in every Task text, the
// merge gate on or off (section 7.4).
const mergeBase = `Before anything else, run "git merge --no-edit main"; if it conflicts, resolve each conflict, keeping what both sides meant, and commit the merge, or, if you cannot, write "BLOCKED: main conflicts in <files>" instead, naming the conflicting files.`

// Every worker run merges base first, whatever its stage and the merge
// gate: a slice with a plan, the fast lane, and an answer to the reviews.
func TestRenderWorkerMergesBase(t *testing.T) {
	for _, gate := range []bool{true, false} {
		f, at, pr, _ := planned(t)
		fast, fastAt := fixture()
		pulls, _ := f.Pulls(1)
		answer := job(f, at.N, "worker", "swarm:review-3-changes-requested")
		answer.Pulls, answer.Pull = pulls, thread(f, pr)
		for name, j := range map[string]*Job{
			"slice":     job(f, at.N, "worker", "swarm:plan-1-accepted"),
			"fast lane": job(fast, fastAt.N, "worker", "swarm:plan-1-accepted"),
			"answer":    answer,
		} {
			j.Cfg.Gates.Merge = gate
			sys, _ := Render(j)
			has(t, fmt.Sprintf("%s, merge gate %v", name, gate), sys, []string{mergeBase})
		}
	}
}

// The worker's first slice of a plan: no pin sentence.
func TestRenderWorkerFirstSlice(t *testing.T) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	sys, prompt := Render(job(f, at.N, "worker", "swarm:plan-1-accepted"))
	has(t, "system.md", sys, []string{"Implement slice 1 of the accepted plan for issue #1 (it declares 2) in acme/lib"}, "slice <i>", "These paths are protected")
	has(t, "prompt.md", prompt, []string{"- Clone /work: acme/lib, branch swarm/1-1, base main, no check declared"})
}

// The mounts a member is given leave to reach: those outside its working
// directory.
func TestOutside(t *testing.T) {
	dirs := []string{"/work/.swarm", "/work/lib", "/resources/docs", "/workshop"}
	if got := outside("/work", dirs); !reflect.DeepEqual(got, []string{"/resources/docs", "/workshop"}) {
		t.Errorf("in /work: %q", got)
	}
	if got := outside("/work/app", dirs); !reflect.DeepEqual(got, dirs) {
		t.Errorf("in /work/app: %q", got)
	}
}

// The empty-field check reads Daisy's own lines only (section 7.1, step
// 2): a quoted body's `- Clone ` or `- Issues:` bullet, or a fenced
// `# Issue` line, is no field of the prompt.
func TestEmptyField(t *testing.T) {
	head := "# Repository\n\n- Issues: acme/app on forge.example.net\n- Clone /work: acme/app, branch main, base main, no check declared\n\n"
	for _, c := range []struct {
		prompt string
		empty  bool
	}{
		{head + "# Issue #1: Drift\n\n- Clone the repo first.\n- Issues: none\n\n```sh\n# Issue tracker\n# Pull request\n```\n", false},
		{head + "# Issue #1: \n", true},
		{"# Repository\n\n- Issues: acme/app on \n", true},
		{"# Repository\n\n- Clone /work: acme/app, branch , base main, x\n", true},
		{head + "# Issue #1: Drift\n\n# Pull request app#2: \n", true},
		{head + "# Issue #1: Drift\n\n~~~\n```\n# Issue x\n~~~\n# Issue #1: \n", true},
		{head + "# Issue #1: Drift\n\n" + Shift("a pasted log, its fence left open:\n```\n# Pull request x", 1) + "\n\n# Pull request app#2: \n", true},
	} {
		if got := emptyField(c.prompt); got != c.empty {
			t.Errorf("emptyField = %v, want %v:\n%s", got, c.empty, c.prompt)
		}
	}
}
