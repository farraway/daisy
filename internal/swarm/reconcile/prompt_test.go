package reconcile

import (
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
		"### adversary, run 1, at 2026-11-06T15:07:00Z, on aaa: request-changes\n\nVERDICT: request-changes\n[blocking] x.go:1 wrong",
		"### sympathetic, run 1, at 2026-11-06T15:08:00Z, on aaa: approve",
		"### jane at 2026-11-06T15:09:00Z, on x.go:3 at aaa\n\nwhy?",
		"# Check\n\nmake check on aaa: exit 2\n\n    line 6\n", "    line 25\n"},
		"line 5\n", "<!-- daisy-swarm role=worker")
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
		`write "BLOCKED: slice <i> is not merged".`, `These paths are protected: .gitea/, AGENTS.md.`},
		`git merge --no-edit`)
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
		`Before anything else, run "git merge --no-edit main"; if it conflicts, write "BLOCKED: main conflicts" instead. Commit with git`},
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
