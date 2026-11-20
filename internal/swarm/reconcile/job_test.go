package reconcile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"daisy/internal/ops"
)

// jobFiles writes each file of issue n's workspace under state, as the
// collections would have written it (section 7.8, The files).
func jobFiles(t *testing.T, state string, n int, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		p := filepath.Join(state, "jobs", itoa(n), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const (
	keptSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	// builderPlan is the builder plan of archPlan, 66 bytes.
	builderPlan = "## Slice 1\n\nBuild the library in lib.\n\n## Slice 2\n\nPin it in app.\n"
	// round1 is slice 1's first round, as writeRound writes it.
	round1 = "# Round 1 on aaa\n\nSlice 1, pull request lib#1. The round before: none.\n\n" +
		"## adversary, run 1: request-changes\n\nWhole: /job/memory/texts/1-adversary-1.md\n\n[blocking] x.go:1 wrong\n[non-blocking] y.go:2 a nit\n\n" +
		"## sympathetic, run 1: approve\n\nWhole: /job/memory/texts/1-sympathetic-1.md\n"
)

// planWorkspace is the planned fixture's workspace: the plan, its builder
// plan and slice 1's files, a completed round on aaa, the failed check of
// aaa, the worker's report, and a kept commit of worker run 2.
func planWorkspace(t *testing.T, state string) {
	jobFiles(t, state, 1, map[string]string{
		"plan.md":              archPlan,
		"builder.md":           builderPlan,
		"slices/1/slice.md":    "1. lib: the library.\n\n## Slice 1\n\nBuild the library in lib.\n",
		"slices/2/slice.md":    "2. app: the pin.\n\n## Slice 2\n\nPin it in app.\n",
		"slices/1/kept":        "1-worker-2 " + keptSHA + "\n",
		"slices/1/rounds/1.md": round1,
		"checks/aaa.log":       "make check on aaa: exit 2\n\nline 25\n",
		"texts/1-worker-1.md":  "Did it.\n\nNot established: none\n",
	})
}

// memoryJob is role's job on the planned fixture, its workspace read as
// the launch reads it (readMemory), the check of aaa failed, two other
// branches open and, for the worker, a start from worker run 2's kept
// commit.
func memoryJob(t *testing.T, role, label string) *Job {
	t.Helper()
	f, at, pr, state := planned(t)
	planWorkspace(t, state)
	j := jobOn(f, at, pr, role, label)
	j.Outputs = Outputs{FS: ops.Real().FS(), State: state}
	j.Check = "make check on aaa: exit 2\n\n    line 25"
	j.Branches = 2
	if role == "worker" {
		j.Start = Start{From: "kept", SHA: keptSHA, Ref: "refs/swarm-runs/1-worker-2", Kept: "1-worker-2", KeptSHA: keptSHA}
	}
	if err := j.readMemory(); err != nil {
		t.Fatal(err)
	}
	return j
}

// The worker's prompt.md, byte for byte, continuing its open pull request
// (section 7.3; 7.8, The prompt): # Repository names the workspace; # Job,
// after # Issue, points into it, every line the worker reads; no # Plan,
// no description and no report of the worker's; the latest round's
// reviews as their headings and [blocking] lines alone.
func TestRenderWorkerJob(t *testing.T) {
	j := memoryJob(t, "worker", "swarm:review-3-changes-requested")
	if _, prompt := Render(j); prompt != wantWorkerPrompt {
		t.Errorf("prompt.md:\n%s", prompt)
	}
}

const wantWorkerPrompt = `<!-- daisy-swarm prompt: acme/app#1 worker run 1, 2026-11-06T15:00:00Z -->

# Repository

- Issues: acme/app on forge.example.net
- Clone /job/work/lib: acme/lib, branch swarm/1-1, base main, no check declared
- Job workspace (read-only): /job/memory

# Issue #1: Drift check

Opened by owner, run 2, at 2026-11-06T15:01:00Z. Labels: swarm:proposal-accepted.

Brief: docs/briefs/b.md — M0

## Why

It drifts.

# Job

- Workspace: /job/memory, read-only, kept by Daisy; the forge holds its texts for people, cut when long
- Plan: /job/memory/plan.md, architect run 1, 143 bytes, 2 slices
- Builder plan: /job/memory/builder.md, architect run 1, 66 bytes
- Slice: /job/memory/slices/1/slice.md, slice 1 of 2 in lib: the library.
- Start: bbbbbbb, worker run 2's kept commits
- Report: /job/memory/texts/1-worker-1.md, the worker's newest report on slice 1
- Rounds: /job/memory/slices/1/rounds/, 1 completed; the latest, 1 on aaa: adversary request-changes, sympathetic approve, 1 blocking
- Check: /job/memory/checks/aaa.log, make check on aaa: exit 2
- Branches: /job/io/branches.md, 2 other open swarm branches

# Earlier pull requests

None.

# Pull request lib#1: #1 slice 1: Drift check

Branch swarm/1-1 into main; head aaa.

## Thread

### adversary, run 1, at 2026-11-06T15:07:00Z, on aaa: request-changes

[blocking] x.go:1 wrong

### sympathetic, run 1, at 2026-11-06T15:08:00Z, on aaa: approve

### jane at 2026-11-06T15:09:00Z, on x.go:3 at aaa

why?

# Check

make check on aaa: exit 2

    line 25
`

// The reviewers', the architect's and the owner's # Job (section 7.8,
// The prompt, by role): a reviewer's Report, Rounds and Check and no
// Start; the architect's Kept, a line per slice with a kept commit, and,
// its pull request open, Rounds and Check, but no Slice or Report; the
// owner's Workspace and Plan alone. None has # Plan; the architect's and
// the owner's # Thread show the accepted plan as its heading and pointer.
func TestRenderJobByRole(t *testing.T) {
	const (
		workspace = "# Job\n\n- Workspace: /job/memory, read-only, kept by Daisy; the forge holds its texts for people, cut when long\n- Plan: /job/memory/plan.md, architect run 1, 143 bytes, 2 slices\n"
		builder   = "- Builder plan: /job/memory/builder.md, architect run 1, 66 bytes\n"
		slice     = "- Slice: /job/memory/slices/1/slice.md, slice 1 of 2 in lib: the library.\n"
		report    = "- Report: /job/memory/texts/1-worker-1.md, the worker's newest report on slice 1\n"
		rounds    = "- Rounds: /job/memory/slices/1/rounds/, 1 completed; the latest, 1 on aaa: adversary request-changes, sympathetic approve, 1 blocking\n- Check: /job/memory/checks/aaa.log, make check on aaa: exit 2\n"
		branches  = "- Branches: /job/io/branches.md, 2 other open swarm branches\n"
		pointer   = "## architect, run 1, at 2026-11-06T15:04:00Z (the accepted plan: whole in /job/memory/plan.md)\n\n## jane at"
	)
	for _, role := range []string{"adversary", "sympathetic"} {
		_, prompt := Render(memoryJob(t, role, "swarm:review-0-ready"))
		has(t, role+"'s prompt.md", prompt, []string{"\n\n" + workspace + builder + slice + report + rounds + branches + "\n# Earlier pull requests\n",
			"Branch swarm/1-1 into main; head aaa.\n\n## Thread\n\n### jane at"},
			"# Plan", "- Start:", "- Kept:", "Did it.")
	}
	_, prompt := Render(memoryJob(t, "architect", "swarm:plan-0-posted"))
	has(t, "architect's prompt.md", prompt, []string{"\n\n" + workspace + builder + rounds + "- Kept: slice 1: bbbbbbb, worker run 2\n" + branches + "\n# Thread\n",
		pointer, "Branch swarm/1-1 into main; head aaa.\n\nDid it."},
		"- Slice:", "- Report:", "- Start:", "#### Research", "it works.")
	_, prompt = Render(memoryJob(t, "owner", "swarm:proposal-posted"))
	has(t, "owner's prompt.md", prompt, []string{"\n\n" + workspace + "\n# Thread\n", pointer}, "- Builder plan:", "- Branches:", "#### Research")
}

// A job with no plan (the fast lane): Plan says so, and no slice, builder
// plan or kept commit is pointed to; the owner's has no Plan line.
func TestRenderJobWithoutPlan(t *testing.T) {
	f, at := fixture()
	state := t.TempDir()
	for _, c := range []struct {
		role, label string
		want        []string
		not         []string
	}{
		{"worker", "swarm:plan-1-accepted", []string{"\n- Plan: none: there is no plan.\n- Start: main\n- Branches: /job/io/branches.md, 0 other open swarm branches\n\n# Direction\n"}, []string{"- Slice:", "# Plan"}},
		{"architect", "swarm:proposal-accepted", []string{"\n- Plan: none: there is no plan.\n- Branches: /job/io/branches.md, 1 other open swarm branch\n\n# Thread\n"}, nil},
		{"owner", "swarm:proposal-posted", []string{"cut when long\n\n# Thread\n"}, []string{"- Plan:"}},
	} {
		j := job(f, at.N, c.role, c.label)
		j.Outputs = Outputs{FS: ops.Real().FS(), State: state}
		j.Start = Start{From: "base"}
		if c.role == "architect" {
			j.Branches = 1
		}
		if err := j.readMemory(); err != nil {
			t.Fatal(err)
		}
		_, prompt := Render(j)
		has(t, c.role+"'s prompt.md", prompt, c.want, c.not...)
	}
}

// The worker's Start line names where its clone starts and why (section
// 7.8, The start), in each of the rule's cases.
func TestRenderStartLine(t *testing.T) {
	at := time.Date(2026, 11, 6, 16, 0, 0, 0, time.UTC)
	const head = "cccccccccccccccccccccccccccccccccccccccc"
	for _, c := range []struct {
		st   Start
		open bool
		want string
	}{
		{Start{From: "kept", SHA: keptSHA, Kept: "1-worker-2", KeptSHA: keptSHA}, false, "- Start: bbbbbbb, worker run 2's kept commits\n"},
		{Start{From: "base"}, false, "- Start: main\n"},
		{Start{From: "head"}, true, "- Start: the pull request's head aaa\n"},
		{Start{From: "base", Kept: "1-worker-2", KeptSHA: keptSHA, By: "jane", At: at}, false,
			"- Start: main (Start: base, jane at 2026-11-06T16:00:00Z); worker run 2's kept commits bbbbbbb are left as they are\n"},
		{Start{From: "head", Kept: "1-worker-2", KeptSHA: keptSHA, By: "architect run 3", At: at}, true,
			"- Start: the pull request's head aaa (Start: base, architect run 3 at 2026-11-06T16:00:00Z); worker run 2's kept commits bbbbbbb are left as they are\n"},
		{Start{From: "moved", SHA: head, Ref: "refs/heads/swarm/1-1", Kept: "1-worker-2", KeptSHA: keptSHA}, true,
			"- Start: the pull request's head ccccccc, which moved past worker run 2's kept commits bbbbbbb\n"},
	} {
		f, at, pr, state := planned(t)
		if !c.open {
			if err := f.Merge(pr, "aaa"); err != nil {
				t.Fatal(err)
			}
		}
		j := job(f, at.N, "worker", "swarm:plan-1-accepted")
		if c.open {
			j = jobOn(f, at, pr, "worker", "swarm:review-3-changes-requested")
		}
		j.Outputs, j.Start = Outputs{FS: ops.Real().FS(), State: state}, c.st
		if err := j.readMemory(); err != nil {
			t.Fatal(err)
		}
		_, prompt := Render(j)
		has(t, c.st.From+"'s prompt.md", prompt, []string{"\n" + c.want})
	}
}

// # Direction holds the maintainer's comments since the plan's newest
// basis (section 7.8, The prompt): a re-plan request the architect
// answered STANDS: is no longer direction; a comment after it is.
func TestRenderDirectionSinceStands(t *testing.T) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	f.Say(at, "jane", "Before the plan stood.")
	f.Comment(at, marker("architect", 2)+"STANDS: the slices answer it.")
	f.Say(at, "jane", "After it stood.")
	_, prompt := Render(job(f, at.N, "worker", "swarm:plan-1-accepted"))
	has(t, "prompt.md", prompt, []string{"# Direction\n\n## jane at 2026-11-06T15:07:00Z\n\nAfter it stood.\n"}, "Before the plan stood.")
}

// The worker's and the reviewers' Task texts send them to the slice
// first, then to the builder plan and the plan (section 7.4); without a
// builder plan, to the plan alone; the fast lane, to neither. The
// architect's builder sentence says that Daisy gives each worker its
// slice's step and section.
func TestTaskSendsToTheSlice(t *testing.T) {
	const withBuilder = `Read slice 1, /job/memory/slices/1/slice.md, first; then the builder plan, /job/memory/builder.md, and the plan, /job/memory/plan.md, as you need them.`
	for _, c := range []struct{ role, label string }{{"worker", "swarm:review-3-changes-requested"}, {"adversary", "swarm:review-0-ready"}, {"sympathetic", "swarm:review-0-ready"}} {
		sys, _ := Render(memoryJob(t, c.role, c.label))
		has(t, c.role+"'s system.md", sys, []string{withBuilder})
	}
	j := memoryJob(t, "worker", "swarm:review-3-changes-requested")
	os.Remove(filepath.Join(j.Outputs.State, "jobs", "1", "builder.md"))
	if err := j.readMemory(); err != nil {
		t.Fatal(err)
	}
	sys, prompt := Render(j)
	has(t, "system.md without a builder plan", sys, []string{`Read slice 1, /job/memory/slices/1/slice.md, first; then the plan, /job/memory/plan.md, as you need it.`}, "builder.md")
	has(t, "prompt.md without a builder plan", prompt, nil, "- Builder plan:")
	f, at := fixture()
	fast := job(f, at.N, "worker", "swarm:plan-1-accepted")
	fast.Outputs = Outputs{FS: ops.Real().FS(), State: t.TempDir()}
	if err := fast.readMemory(); err != nil {
		t.Fatal(err)
	}
	sys, _ = Render(fast)
	has(t, "the fast lane's system.md", sys, nil, "/job/memory/slices")
	sys, _ = Render(job(f, at.N, "architect", "swarm:proposal-accepted"))
	has(t, "architect's system.md", sys, []string{`so that it reads without the plan. Daisy gives each worker its slice's step and section. Write it whole each time you write the plan.`})
}

// A file of the workspace # Job reads that is there and cannot be read is
// the job's own failure (section 7.7), never a line left out.
func TestReadMemoryUnusable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads a file of mode 000")
	}
	for _, rel := range []string{"slices/1/rounds/1.md", "slices/1/kept"} {
		f, at, pr, state := planned(t)
		planWorkspace(t, state)
		os.Chmod(filepath.Join(state, "jobs", "1", filepath.FromSlash(rel)), 0o000)
		j := jobOn(f, at, pr, "architect", "swarm:plan-0-posted")
		j.Outputs = Outputs{FS: ops.Real().FS(), State: state}
		err := j.readMemory()
		var u *unusableError
		if !errors.As(err, &u) || u.file != "jobs/1/"+rel {
			t.Errorf("%s unreadable: %v", rel, err)
		}
	}
}
