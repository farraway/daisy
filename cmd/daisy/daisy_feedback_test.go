package main_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"daisy/internal/cli"
	"daisy/internal/ops"
)

const widgetPrompt = "## Trigger\n\nRead this when relevant.\n\n## Rules\n\n1. Measure before cutting.\n\n# Document Title\n\n## Next Section\n\nBody text.\n"

const widgetFeedback = "# Feedback Log\n\n---\n\n## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nWidget once.\n\n## 2026-08-02 1100\n**Workflow:** plan\n\nPlan, not this target's.\n\n## 2026-08-03 1200\n**Workflow:** widgetflow\n\nWidget again.\n"

// loop is a machine set up for the feedback loop: root prompts, a home
// prompt with rules, pending entries, the home a repository, and a
// workspace AGENTS.md as init leaves one. The clock is pinned; answers is
// what each approval prompt reads, in turn (EOF past the last).
type loop struct {
	*machine
	answers []string
	asked   []string
}

func newLoop(t *testing.T) *loop {
	t.Helper()
	m := newMachine(t)
	writeFile(t, filepath.Join(m.root, "prompts", "daisy.md"), "## Trigger\n\nDaisy.\n\n## Rules\n\n1. a\n")
	writeFile(t, filepath.Join(m.root, "prompts", "plan.md"), "## Trigger\n\nPlan.\n\n## Rules\n\n1. a\n")
	writeFile(t, filepath.Join(m.root, "prompts", "retrospective.md"), "## Trigger\n\nRetro.\n\n## Rules\n\n1. a\n")
	writeFile(t, filepath.Join(m.home, "prompts", "widgetflow.md"), widgetPrompt)
	writeFile(t, filepath.Join(m.home, "feedback", "feedback.md"), widgetFeedback)
	writeFile(t, filepath.Join(m.home, "include.txt"), "daisy\n~retrospective\n~plan\n~widgetflow\n")
	writeFile(t, filepath.Join(m.ws, ".daisy", "AGENTS.md"), "stale\n")
	m.initHomeRepo(t)
	return &loop{machine: m}
}

func (l *loop) ops() *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string { return l.vars[k] },
		GetwdFn:  func() (string, error) { return l.ws, nil },
		NowFn:    func() time.Time { return time.Date(2026, 11, 1, 9, 30, 0, 0, time.Local) },
		PromptFn: func(r io.Reader, w io.Writer, q string) (string, error) {
			l.asked = append(l.asked, q)
			if len(l.answers) == 0 {
				return "", io.EOF
			}
			a := l.answers[0]
			l.answers = l.answers[1:]
			return a, nil
		},
	}
}

func (l *loop) run(args ...string) (int, string, string) { return runWith(l.ops(), args...) }

func (l *loop) proposal(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(l.ws, name)
	writeFile(t, p, text)
	return p
}

// Approval is through Ops.Prompt: y applies; N, anything else and EOF
// write nothing and are not failures; --yes never asks.
func TestOptimizeApplyApproval(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers []string
		yes     bool
		applied bool
		says    string
	}{
		{"y", []string{"y"}, false, true, "✓ Updated"},
		{"Yes", []string{"Yes"}, false, true, "✓ Updated"},
		{"N", []string{"N"}, false, false, "Cancelled — prompts/widgetflow left unchanged."},
		{"empty", []string{""}, false, false, "Cancelled"},
		{"EOF", nil, false, false, "Cancelled — no answer on stdin (EOF); nothing written."},
		{"--yes", nil, true, true, "✓ Updated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newLoop(t)
			l.answers = tc.answers
			args := []string{"optimize", "apply", "--workflow", "prompts/widgetflow", "--rules", l.proposal(t, "rules.md", "## Rules\n\n1. Measure twice.\n")}
			if tc.yes {
				args = append(args, "--yes")
			}
			before := snapshot(t, l.home, l.ws)
			code, out, errs := l.run(args...)
			if code != 0 || !strings.Contains(out, tc.says) {
				t.Fatalf("exit %d\nout %s\nerr %s", code, out, errs)
			}
			if tc.yes != (len(l.asked) == 0) {
				t.Errorf("asked %q", l.asked)
			}
			if !tc.yes && l.asked[0] != "Apply these changes to prompts/widgetflow? [y/N] " {
				t.Errorf("question %q", l.asked[0])
			}
			if !strings.Contains(out, "--- a/prompts/widgetflow\n+++ b/prompts/widgetflow\n") || !strings.Contains(out, "\n-1. Measure before cutting.\n+1. Measure twice.\n") {
				t.Errorf("diff:\n%s", out)
			}
			if tc.applied {
				if got := l.read(t, "prompts/widgetflow.md"); got != strings.Replace(widgetPrompt, "Measure before cutting", "Measure twice", 1) {
					t.Errorf("target:\n%s", got)
				}
				if l.commits(t) != "2" || l.git(t, "status", "--porcelain") != "" {
					t.Errorf("commits %s, status %q", l.commits(t), l.git(t, "status", "--porcelain"))
				}
				if msg := l.git(t, "log", "-1", "--format=%s"); msg != "Optimize prompts/widgetflow (rules)" {
					t.Errorf("subject %q", msg)
				}
				return
			}
			sameFiles(t, "declined", before, snapshot(t, l.home, l.ws))
			if l.commits(t) != "1" {
				t.Errorf("a declined apply committed")
			}
		})
	}
}

// The archive gains each of the target's entries — every tag naming it —
// with its provenance; feedback.md keeps the rest; the stamp names the
// run; both AGENTS.md files are rebuilt; one commit holds it all.
func TestOptimizeApplyArchivesAndCommitsOnce(t *testing.T) {
	l := newLoop(t)
	archive := l.read(t, "feedback/archive.md")
	code, out, errs := l.run("optimize", "apply", "--workflow", "widgetflow", "--rewrite", l.proposal(t, "new.md", "## Trigger\n\nNew.\n"), "--yes")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	prov := "**Addressed by:** `daisy optimize` 2026-11-01 — `prompts/widgetflow.md` (rewrite)"
	if got, want := l.read(t, "feedback/archive.md"), archive+"\n## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nWidget once.\n\n"+prov+"\n\n## 2026-08-03 1200\n**Workflow:** widgetflow\n\nWidget again.\n\n"+prov+"\n"; got != want {
		t.Errorf("archive\n%q\nwant\n%q", got, want)
	}
	if got, want := l.read(t, "feedback/feedback.md"), "# Feedback Log\n\n---\n\n## 2026-08-02 1100\n**Workflow:** plan\n\nPlan, not this target's.\n"; got != want {
		t.Errorf("feedback\n%q", got)
	}
	if got := l.read(t, "feedback/.last-optimized"); got != "2026-11-01 0930 prompts/widgetflow\n" {
		t.Errorf("stamp %q", got)
	}
	if got := l.read(t, "prompts/widgetflow.md"); got != "## Trigger\n\nNew.\n" {
		t.Errorf("target %q", got)
	}
	ws, _ := os.ReadFile(filepath.Join(l.ws, ".daisy", "AGENTS.md"))
	if !strings.Contains(string(ws), "New.") || !strings.Contains(l.read(t, "AGENTS.md"), "New.") {
		t.Errorf("AGENTS.md not rebuilt:\n%s", ws)
	}
	if l.commits(t) != "2" || l.git(t, "status", "--porcelain") != "" {
		t.Errorf("commits %s status %q", l.commits(t), l.git(t, "status", "--porcelain"))
	}
	for _, want := range []string{"  ✓ Updated " + filepath.Join(l.home, "prompts", "widgetflow.md"), "  ✓ Archived 2 entries to ", "  ✓ AGENTS.md rebuilt", "📝 Committed: Optimize prompts/widgetflow (rewrite) ("} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	// Applying again: nothing pending and the text already there.
	code, out, _ = l.run("optimize", "apply", "--workflow", "widgetflow", "--rewrite", filepath.Join(l.ws, "new.md"), "--yes")
	if code != 0 || !strings.HasPrefix(out, "Nothing to apply: ") || l.commits(t) != "2" {
		t.Errorf("second apply: exit %d out %q commits %s", code, out, l.commits(t))
	}
}

// A failure after the first write takes every write back; one before it
// writes nothing.
func TestOptimizeApplyRollsBack(t *testing.T) {
	l := newLoop(t)
	// A malformed include.txt fails the AGENTS.md build, which comes after
	// the target, the archive, feedback.md and the stamp are written.
	writeFile(t, filepath.Join(l.home, "include.txt"), "\"unmatched\n")
	proposal := l.proposal(t, "r.md", "## Rules\n\n1. x\n")
	before := snapshot(t, l.home, l.ws)
	code, _, errs := l.run("optimize", "apply", "--workflow", "widgetflow", "--rules", proposal, "--yes")
	if code != 1 || !strings.Contains(errs, "AGENTS.md cannot be rebuilt") || !strings.Contains(errs, "Nothing was applied.") {
		t.Fatalf("exit %d err %q", code, errs)
	}
	sameFiles(t, "rolled back", before, snapshot(t, l.home, l.ws))

	// The home not a repository: refused before any write.
	l = newLoop(t)
	os.RemoveAll(filepath.Join(l.home, ".git"))
	before = snapshot(t, l.home)
	if code, _, errs := l.run("optimize", "apply", "--workflow", "widgetflow", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. x\n"), "--yes"); code != 1 || !strings.Contains(errs, "not in its own git repository") {
		t.Errorf("no repo: exit %d err %q", code, errs)
	}
	sameFiles(t, "no repo", before, snapshot(t, l.home))
}

// Syntax: bare apply, a missing value, both modes, a stray word are usage
// errors (exit 2); the briefing takes no other flag.
func TestOptimizeUsage(t *testing.T) {
	l := newLoop(t)
	r := l.proposal(t, "r.md", "## Rules\n")
	for _, args := range [][]string{
		{"optimize", "apply"},
		{"optimize", "apply", "--rules", r},
		{"optimize", "apply", "--workflow", "plan"},
		{"optimize", "apply", "--workflow", "plan", "--rules", r, "--rewrite", r},
		{"optimize", "apply", "--workflow", "--rules", r},
		{"optimize", "apply", "--workflow", "plan", "--rules", "--yes"},
		{"optimize", "apply", "--workflow", "", "--rules", r},
		{"optimize", "apply", "--workflow", "plan", "--rules", r, "stray"},
		{"optimize", "--dry-run"},
		{"optimize", "--workflow"},
		{"optimize", "extra"},
	} {
		code, out, errs := l.run(args...)
		if code != 2 || out != "" || !strings.Contains(errs, "Usage: daisy optimize") {
			t.Errorf("%q: exit %d out %q err %q", args, code, out, errs)
		}
	}
	if code, _, _ := l.run("optimize", "apply", "--help"); code != 0 {
		t.Errorf("apply --help: %d", code)
	}
}

// The briefing runs read-only: under ops.ReadOnly a write would fail it,
// so passing proves it wrote nothing; the scope narrows to one target.
func TestOptimizeBriefing(t *testing.T) {
	l := newLoop(t)
	before := snapshot(t, l.home, l.ws, l.root)
	code, out, errs := l.run("optimize")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d err %q", code, errs)
	}
	sameFiles(t, "briefing", before, snapshot(t, l.home, l.ws, l.root))
	for _, want := range []string{
		"Pending feedback: 3 entries for 2 targets (" + filepath.Join(l.home, "feedback", "feedback.md") + ")\n",
		"\n[prompts/widgetflow] 2 entries\nTarget: " + filepath.Join(l.home, "prompts", "widgetflow.md") + "\nRules: 1\n--- entries ---\n## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nWidget once.\n\n## 2026-08-03 1200\n**Workflow:** widgetflow\n\nWidget again.\n--- current ## Rules ---\n## Rules\n\n1. Measure before cutting.\n--- end ---\n",
		"\n[prompts/plan] 1 entry\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("briefing lacks %q:\n%s", want, out)
		}
	}
	_, out, _ = l.run("optimize", "-w", "plan")
	if strings.Contains(out, "widgetflow") || !strings.Contains(out, "Pending feedback: 1 entry for 1 target") {
		t.Errorf("scoped:\n%s", out)
	}
	os.Remove(filepath.Join(l.home, "feedback", "feedback.md"))
	if code, _, errs := l.run("optimize"); code != 1 || !strings.HasPrefix(errs, "Error: No feedback file found at ") {
		t.Errorf("no feedback.md: exit %d err %q", code, errs)
	}
}

// feedback appends the bash's bytes and prints its lines; a tag naming no
// file, and no message, are refused before any write.
func TestFeedback(t *testing.T) {
	l := newLoop(t)
	file := filepath.Join(l.home, "feedback", "feedback.md")
	code, out, errs := l.run("feedback", "-w", "prompts/plan", "Plan", "was", "wrong.")
	if code != 0 || errs != "" || out != "  ✓ Feedback recorded (workflow: prompts/plan, 2026-11-01 09:30)\n  File: "+file+"\n" {
		t.Fatalf("exit %d out %q err %q", code, out, errs)
	}
	if got := l.read(t, "feedback/feedback.md"); got != widgetFeedback+"\n## 2026-11-01 0930\n**Workflow:** prompts/plan\n\nPlan was wrong.\n" {
		t.Errorf("feedback.md\n%q", got)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run(l.ops(), strings.NewReader("Piped.\n\n"), &stdout, &stderr, []string{"feedback"}); code != 0 || stdout.String() != "  ✓ Feedback recorded (2026-11-01 09:30)\n  File: "+file+"\n" {
		t.Errorf("piped: exit %d out %q err %q", code, stdout.String(), stderr.String())
	}
	if !strings.HasSuffix(l.read(t, "feedback/feedback.md"), "\n## 2026-11-01 0930\n\nPiped.\n") {
		t.Errorf("piped entry:\n%s", l.read(t, "feedback/feedback.md"))
	}
	before := l.read(t, "feedback/feedback.md")
	if code, _, errs := l.run("feedback", "--workflow", "nope", "x"); code != 1 || !strings.Contains(errs, "Error: workflow 'nope' names no prompt or skill\n") || !strings.Contains(errs, "\n    prompts/widgetflow\n") {
		t.Errorf("unknown tag: exit %d err %q", code, errs)
	}
	if code, _, errs := l.run("feedback"); code != 1 || errs != "Error: feedback message cannot be empty\n" {
		t.Errorf("empty: exit %d err %q", code, errs)
	}
	// An empty word is an empty message, not a reason to read stdin
	// (review pass 3, F13).
	var so, se bytes.Buffer
	if code := cli.Run(l.ops(), strings.NewReader("piped\n"), &so, &se, []string{"feedback", ""}); code != 1 || se.String() != "Error: feedback message cannot be empty\n" {
		t.Errorf("feedback \"\": exit %d err %q", code, se.String())
	}
	if l.read(t, "feedback/feedback.md") != before {
		t.Error("a refusal wrote")
	}
	if l.commits(t) != "1" {
		t.Error("feedback committed")
	}
}

// eval lists, shows and records as the bash did.
func TestEval(t *testing.T) {
	l := newLoop(t)
	writeFile(t, filepath.Join(l.root, "prompts", "evals", "new-day.md"), "# New day starts\n\nCriteria.\n")
	writeFile(t, filepath.Join(l.root, "prompts", "evals", "a.md"), "No title.\n")
	code, out, _ := l.run("eval")
	if want := "Available eval cases:\n\n  a                     \n  new-day               New day starts\n\n  Usage: daisy eval <case>                           # display criteria\n         daisy eval --record <case> pass|fail        # record a result\n"; code != 0 || out != want {
		t.Errorf("list: exit %d\n%q\nwant\n%q", code, out, want)
	}
	if code, out, _ := l.run("eval", "new-day"); code != 0 || out != "# New day starts\n\nCriteria.\n\n  Record result: daisy eval --record new-day pass|fail \"notes\"\n" {
		t.Errorf("show: exit %d %q", code, out)
	}
	results := filepath.Join(l.home, "feedback", "eval", "results.md")
	if code, out, _ := l.run("eval", "--record", "new-day", "pass", "Fine."); code != 0 || out != "  ✓ Recorded pass for eval 'new-day' (2026-11-01 09:30)\n  File: "+results+"\n" {
		t.Errorf("record: exit %d %q", code, out)
	}
	l.run("eval", "--record", "new-day", "fail")
	if got := l.read(t, "feedback/eval/results.md"); !strings.HasPrefix(got, "# Eval Results\n") || !strings.HasSuffix(got, "---\n\n## 2026-11-01 0930\n**Case:** new-day\n**Result:** pass\n\nFine.\n\n## 2026-11-01 0930\n**Case:** new-day\n**Result:** fail\n") {
		t.Errorf("results.md\n%q", got)
	}
	for _, tc := range []struct {
		args []string
		code int
		err  string
	}{
		{[]string{"eval", "nope"}, 1, "Error: Eval case not found: nope\n"},
		{[]string{"eval", "../plan"}, 1, "Error: Eval case not found: ../plan\n"},
		{[]string{"eval", "--record", "new-day", "maybe"}, 1, "Error: result must be 'pass' or 'fail' (got: maybe)\n"},
		{[]string{"eval", "--record"}, 1, "Usage: daisy eval --record <case> pass|fail [\"notes\"]\n"},
		{[]string{"eval", "new-day", "extra"}, 2, "Error: unexpected argument: extra\n"},
	} {
		if code, _, errs := l.run(tc.args...); code != tc.code || !strings.HasPrefix(errs, tc.err) {
			t.Errorf("%q: exit %d err %q", tc.args, code, errs)
		}
	}
	os.RemoveAll(filepath.Join(l.root, "prompts", "evals"))
	if code, _, errs := l.run("eval"); code != 1 || errs != "Error: No evals directory found at "+filepath.Join(l.root, "prompts", "evals")+"\n" {
		t.Errorf("no evals: exit %d err %q", code, errs)
	}
}

// The three hooks answer --healthcheck; the briefing's needs feedback/.
func TestFeedbackHealthHooks(t *testing.T) {
	l := newLoop(t)
	for _, cmd := range []string{"feedback", "optimize", "eval"} {
		if code, out, errs := l.run(cmd, "--healthcheck"); code != 0 || out != "" || errs != "" {
			t.Errorf("%s: exit %d out %q err %q", cmd, code, out, errs)
		}
	}
	os.RemoveAll(filepath.Join(l.home, "feedback"))
	if code, _, errs := l.run("optimize", "--healthcheck"); code != 1 || !strings.HasPrefix(errs, "Error: feedback directory not found") {
		t.Errorf("optimize without feedback/: exit %d err %q", code, errs)
	}
}

// The writes are computed before the question; a file changed while it
// waits — an entry recorded meanwhile — refuses the apply rather than
// losing the change (review pass 1, F4).
func TestOptimizeApplyRefusesAChangeWhileAsking(t *testing.T) {
	for _, rel := range []string{"feedback/feedback.md", "feedback/archive.md", "prompts/widgetflow.md"} {
		l := newLoop(t)
		o := l.ops()
		o.PromptFn = func(io.Reader, io.Writer, string) (string, error) {
			p := filepath.Join(l.home, rel)
			b, _ := os.ReadFile(p)
			os.WriteFile(p, append(b, "\n## 2026-11-01 0931\n\nRecorded meanwhile.\n"...), 0o644)
			return "y", nil
		}
		proposal := l.proposal(t, "r.md", "## Rules\n\n1. x\n")
		before := snapshot(t, l.home)
		code, _, errs := runWith(o, "optimize", "apply", "--workflow", "widgetflow", "--rules", proposal)
		if code != 1 || !strings.Contains(errs, filepath.Join(l.home, rel)+" changed while waiting for the answer; nothing was written.") {
			t.Errorf("%s: exit %d err %q", rel, code, errs)
		}
		after := snapshot(t, l.home)
		if !strings.HasSuffix(after[filepath.Join(l.home, rel)], "Recorded meanwhile.\n") {
			t.Errorf("%s: the change was lost", rel)
		}
		delete(before, filepath.Join(l.home, rel))
		delete(after, filepath.Join(l.home, rel))
		sameFiles(t, rel, before, after)
		if l.commits(t) != "1" {
			t.Errorf("%s: committed", rel)
		}
	}
}

// Only EOF cancels quietly; another failure to read the answer is an
// error (review pass 3, F14).
func TestOptimizeApplyAnswerUnreadable(t *testing.T) {
	l := newLoop(t)
	o := l.ops()
	o.PromptFn = func(io.Reader, io.Writer, string) (string, error) { return "", syscall.EISDIR }
	before := snapshot(t, l.home)
	code, out, errs := runWith(o, "optimize", "apply", "-w", "widgetflow", "--rules", l.proposal(t, "r.md", "## Rules\n"))
	if code != 1 || strings.Contains(out, "Cancelled") || errs != "Error: cannot read the answer: is a directory\nNothing was written.\n" {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	sameFiles(t, "unreadable answer", before, snapshot(t, l.home))
}

// What cannot be read is said, not passed over: an unreadable target fails
// the briefing after it prints (F3); a missing archive template fails the
// apply before any write (F8); a missing feedback.md names the fix that
// works (F7).
func TestOptimizeSaysWhatItCannotRead(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads anything")
	}
	l := newLoop(t)
	target := filepath.Join(l.home, "prompts", "widgetflow.md")
	os.Chmod(target, 0)
	code, out, errs := l.run("optimize")
	os.Chmod(target, 0o644)
	if code != 1 || !strings.Contains(out, "Target: "+target+" — cannot read "+target) || !strings.Contains(errs, "Error: cannot read "+target) || !strings.HasSuffix(errs, "The briefing above is incomplete.\n") {
		t.Errorf("unreadable: exit %d\nout %s\nerr %s", code, out, errs)
	}

	os.Remove(filepath.Join(l.home, "feedback", "archive.md"))
	os.Remove(filepath.Join(l.root, "templates", "home", "feedback", "archive.md"))
	before := snapshot(t, l.home)
	if code, _, errs := l.run("optimize", "apply", "-w", "widgetflow", "--rules", l.proposal(t, "r.md", "## Rules\n"), "--yes"); code != 1 || !strings.Contains(errs, "does not exist and cannot be started from") {
		t.Errorf("no template: exit %d err %q", code, errs)
	}
	sameFiles(t, "no template", before, snapshot(t, l.home))

	file := filepath.Join(l.home, "feedback", "feedback.md")
	os.Remove(file)
	if _, _, errs := l.run("optimize"); errs != "Error: No feedback file found at "+file+"\n  Run: mkdir -p \""+filepath.Dir(file)+"\" && touch \""+file+"\"\n" {
		t.Errorf("no feedback.md: %q", errs)
	}
}

// An existing target keeps its mode through apply and through a rollback
// (the Step 22 rule for WriteFile, reached through writeAll).
func TestOptimizeApplyKeepsModes(t *testing.T) {
	l := newLoop(t)
	target := filepath.Join(l.home, "prompts", "widgetflow.md")
	os.Chmod(target, 0o600)
	if code, _, errs := l.run("optimize", "apply", "-w", "widgetflow", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. x\n"), "--yes"); code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o600 {
		t.Errorf("apply: mode %v", fi.Mode().Perm())
	}
	writeFile(t, filepath.Join(l.home, "include.txt"), "\"unmatched\n")
	l.run("optimize", "apply", "-w", "widgetflow", "--rules", l.proposal(t, "s.md", "## Rules\n\n1. y\n"), "--yes")
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o600 {
		t.Errorf("rollback: mode %v", fi.Mode().Perm())
	}
}

// The briefing says when a target's section cannot take --rules (F20).
func TestOptimizeBriefingFlagsAnOpenFence(t *testing.T) {
	l := newLoop(t)
	writeFile(t, filepath.Join(l.home, "prompts", "widgetflow.md"), "## Rules\n\n1. a\n\n```sh\nx\n\n## Examples\n\nlots\n")
	code, out, _ := l.run("optimize", "-w", "widgetflow")
	if code != 0 || !strings.Contains(out, "\n--rules refused: ") || !strings.Contains(out, "fence opened at line 5") {
		t.Errorf("exit %d\n%s", code, out)
	}
	// No section, the rules would be appended into an open fence (pass 6,
	// F23): briefed as apply would refuse it.
	writeFile(t, filepath.Join(l.home, "prompts", "widgetflow.md"), "## Trigger\n\nx\n\n```sh\ncode\n")
	if _, out, _ := l.run("optimize", "-w", "widgetflow"); !strings.Contains(out, "--rules refused: the code fence opened at line 5 of the file never closes, and the rules would be appended inside it") {
		t.Errorf("no section:\n%s", out)
	}
}
