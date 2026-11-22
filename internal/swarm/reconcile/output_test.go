package reconcile

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
)

// block is one `# ` block of a rendered system.md: its heading line and
// the text under it, up to the next.
type sysBlock struct{ heading, text string }

// blocksOf is system.md's `# ` blocks in order, a heading in a fenced
// block not being one.
func blocksOf(sys string) []sysBlock {
	var out []sysBlock
	fenced := false
	for _, l := range strings.Split(sys, "\n") {
		if strings.HasPrefix(l, "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(l, "# ") {
			out = append(out, sysBlock{heading: l})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].text += l + "\n"
		}
	}
	return out
}

// headings is the blocks' heading lines.
func headings(bs []sysBlock) []string {
	var hs []string
	for _, b := range bs {
		hs = append(hs, b.heading)
	}
	return hs
}

// lastLine is text's last non-empty line.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return lines[len(lines)-1]
}

// reviewerRuns is each reviewer's system.md on lib#1, slice 1 of
// issue 1, in round 1 (no review yet) and in round 2, a delta review
// (a review of aaa, the head now bbb).
func reviewerRuns(t *testing.T) map[string]string {
	t.Helper()
	runs := map[string]string{}
	for _, role := range []string{"adversary", "sympathetic"} {
		f, at := fixture()
		f.Comment(at, marker("architect", 1)+archPlan)
		m, err := f.CreatePull("lib", "swarm/1-1", "main", "#1 slice 1: Drift check", marker("worker", 1)+"Did it.\n\nRefs acme/app#1")
		if err != nil {
			t.Fatal(err)
		}
		pr := forge.Ref{Repo: "lib", N: m}
		f.Push(pr, "aaa")
		render := func() string {
			pulls, _ := f.Pulls(1, []string{"swarm-bot"})
			j := job(f, at.N, role, "swarm:review-0-ready")
			j.Pulls, j.Pull, j.Check = pulls, thread(f, pr), "No check result for this head."
			sys, _ := Render(j)
			return sys
		}
		runs[role+", round 1"] = render()
		f.Review(pr, forge.Commented, marker(role, 1)+"VERDICT: request-changes\n[blocking] x.go:1 wrong")
		f.Push(pr, "bbb")
		runs[role+", round 2"] = render()
	}
	return runs
}

// The verdict fix's claim (the plan implementing the 2026-11-19 swarm
// roadmap; sections 7.3 and 7.4, decision 167 as amended): a reviewer's
// system.md, in round 1 and in a delta round alike, holds its output
// contract as the first `# ` block after `# Persona`, `# Output`, with
// the literal verdict lines and a finding line, and its Task ends with
// the line restating the verdict line, the Task stating the verdict
// nowhere else.
func TestRenderReviewerOutputContractFirst(t *testing.T) {
	const reminder = `Begin your output with the line "VERDICT: approve" or "VERDICT: request-changes".`
	runs := reviewerRuns(t)
	if !strings.Contains(runs["adversary, round 2"], "This is round 2 of pull request lib#1") {
		t.Fatalf("round 2 is no delta review:\n%s", runs["adversary, round 2"])
	}
	for name, sys := range runs {
		bs := blocksOf(sys)
		if len(bs) < 3 || bs[0].heading != "# Persona" || bs[1].heading != "# Output" {
			t.Errorf("%s: the blocks are %q, want # Persona, # Output first", name, headings(bs))
			continue
		}
		out := bs[1].text
		lines := strings.Split(out, "\n")
		for _, l := range []string{"VERDICT: approve", "VERDICT: request-changes"} {
			if !slices.Contains(lines, l) {
				t.Errorf("%s: # Output has no line %q alone:\n%s", name, l, out)
			}
		}
		if !slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "[blocking] ") }) ||
			!slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "[non-blocking] ") }) {
			t.Errorf("%s: # Output has no finding line in its template:\n%s", name, out)
		}
		has(t, name+"'s # Output", out, []string{"Request changes only when a finding is blocking"})
		task := bs[len(bs)-1]
		if task.heading != "# Task" || lastLine(task.text) != reminder {
			t.Errorf("%s: the last block %q ends %q, want # Task ending %q", name, task.heading, lastLine(task.text), reminder)
		}
		if n := strings.Count(task.text, "VERDICT: approve"); n != 1 {
			t.Errorf("%s: the Task states the verdict %d times, want once, in its last line:\n%s", name, n, task.text)
		}
	}
	if runs["adversary, round 1"] == "" || blocksOf(runs["adversary, round 1"])[1] != blocksOf(runs["adversary, round 2"])[1] {
		t.Errorf("the adversary's # Output differs between its rounds")
	}
}

// The other first-line contracts (the verdict fix; section 7.4): each
// role's system.md holds `# Output` right after `# Persona`, one text
// for every run of the role, naming every first line its Task texts ask
// for (each upper-case `<WORD>:` a rendered Task names, and the owner's
// `Met:`); a question run's Task, whose first line is required, ends
// restating it.
func TestRenderOutputContractsEveryRole(t *testing.T) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	type run struct {
		role, label string
		question    bool
		sys         string
	}
	var runs []run
	add := func(role, label string, set func(j *Job)) {
		j := job(f, at.N, role, label)
		if set != nil {
			set(j)
		}
		sys, _ := Render(j)
		runs = append(runs, run{role, label, j.Question, sys})
	}
	add("owner", "swarm:proposal-posted", nil)
	add("owner", "swarm:plan-1-accepted", func(j *Job) { j.Question = true })
	add("owner", "", func(j *Job) { j.N, j.Batch = 0, &Batch{Propose: 3} })
	add("architect", "swarm:proposal-accepted", nil)
	add("architect", "swarm:plan-1-accepted", nil)
	add("architect", "swarm:plan-1-accepted", func(j *Job) { j.Question = true })
	add("worker", "swarm:plan-1-accepted", nil)
	add("worker", forge.ChangesRequested, nil)
	add("qa", "", func(j *Job) { j.N, j.Batch = 0, &Batch{Propose: 2} })
	for name, sys := range reviewerRuns(t) {
		role, label, _ := strings.Cut(name, ", ")
		runs = append(runs, run{role, label, false, sys})
	}
	contracts := map[string][]string{
		"owner":       {"`BLOCKED: ", "`QUESTION: ", "`ANSWER: ", "`Met: ", "`## <title>`"},
		"architect":   {"`BLOCKED: ", "`QUESTION: ", "`ANSWER: ", "`STANDS: "},
		"worker":      {"`BLOCKED: ", "`QUESTION: ", "`NO CHANGE: "},
		"adversary":   {"`BLOCKED: ", "`QUESTION: ", "VERDICT: approve"},
		"sympathetic": {"`BLOCKED: ", "`QUESTION: ", "VERDICT: approve"},
		"qa":          {"`BLOCKED: ", "`NO FAILURE: ", "`## <title>`"},
	}
	reminders := map[string]string{
		"owner":     `Begin your output with the line "ANSWER: <your answer>", or with "QUESTION: <the question>" when only the maintainer can settle it.`,
		"architect": `Begin your output with the line "ANSWER: <your answer>", or with "QUESTION: <the question>" when the plan, the thread and the resources do not settle it.`,
	}
	word := regexp.MustCompile(`\b[A-Z]{2,}(?: [A-Z]{2,})*:`)
	seen, named := map[string]string{}, map[string][]string{}
	for _, j := range runs {
		sys := j.sys
		bs := blocksOf(sys)
		if len(bs) < 3 || bs[0].heading != "# Persona" || bs[1].heading != "# Output" {
			t.Errorf("%s %s: the blocks are %q, want # Persona, # Output first", j.role, j.label, headings(bs))
			continue
		}
		out, task := bs[1].text, bs[len(bs)-1].text
		if prev, ok := seen[j.role]; ok && prev != out {
			t.Errorf("%s: two runs' # Output differ:\n%s\n----\n%s", j.role, prev, out)
		}
		seen[j.role] = out
		if strings.TrimSpace(out) != Output(j.role) {
			t.Errorf("%s %s: # Output is not Output(%q), the role's alone:\n%s", j.role, j.label, j.role, out)
		}
		has(t, j.role+"'s # Output", out, contracts[j.role])
		// The closing's Cause sentence names BLOCKED: and QUESTION: for
		// every role, as lines it may write, not as its Task's first line.
		for _, w := range word.FindAllString(strings.ReplaceAll(task, causeSentence, ""), -1) {
			named[j.role] = append(named[j.role], w)
			if !strings.Contains(out, w) {
				t.Errorf("%s %s: the Task names %q, which # Output does not:\n%s", j.role, j.label, w, out)
			}
		}
		if j.question {
			if got := lastLine(task); got != reminders[j.role] {
				t.Errorf("%s question run: the Task ends %q, want %q", j.role, got, reminders[j.role])
			}
			if n := strings.Count(task, `"ANSWER: <your answer>"`); n != 1 {
				t.Errorf("%s question run: the Task states the ANSWER line %d times, want once:\n%s", j.role, n, task)
			}
		}
	}
	if len(seen) != 6 {
		t.Errorf("roles seen %v, want all six", seen)
	}
	// The runs above reach each contract a Task names.
	for role, ws := range map[string][]string{"owner": {"ANSWER:", "BLOCKED:"}, "architect": {"ANSWER:", "STANDS:"}, "worker": {"NO CHANGE:", "QUESTION:", "BLOCKED:"}, "qa": {"NO FAILURE:", "BLOCKED:"}} {
		for _, w := range ws {
			if !slices.Contains(named[role], w) {
				t.Errorf("no %s Task rendered here names %q: %v", role, w, named[role])
			}
		}
	}
}

// config_hash cuts the # Output block out by finding the next `# ` line
// after it (section 7.6), so no Output text may hold a line that begins
// `# `, in a fence or not, nor leave a fence open; every role has one,
// and a role without a text gets no block.
func TestOutputTextsHoldNoHeadingLine(t *testing.T) {
	roles := []string{"owner", "architect", "worker", "adversary", "sympathetic", "qa"}
	for _, role := range roles {
		out := Output(role)
		if out == "" {
			t.Errorf("%s has no # Output text", role)
		}
		fenced := false
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "# ") {
				t.Errorf("%s: a line of # Output begins `# `: %q", role, l)
			}
			if strings.HasPrefix(l, "```") {
				fenced = !fenced
			}
		}
		if fenced {
			t.Errorf("%s: # Output leaves a fence open:\n%s", role, out)
		}
	}
	if Output("reconciler") != "" {
		t.Errorf("a role the swarm does not run has an # Output")
	}
}

// Section 7.4 quotes the reviewers' # Output and the three reminders as
// the code has them: the spec's block quote, its wrapping undone, is
// reviewerOutput word for word, and each reminder is in the spec.
func TestSpecQuotesTheOutputTexts(t *testing.T) {
	b, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	spec := string(b)
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	_, rest, ok := strings.Cut(spec, "reads:\n\n> The first line of your output is exactly")
	if !ok {
		t.Fatal("section 7.4 has no quote of the reviewers' # Output")
	}
	var quote []string
	for _, l := range strings.Split("> The first line of your output is exactly"+rest, "\n") {
		if !strings.HasPrefix(l, ">") {
			break
		}
		quote = append(quote, strings.TrimPrefix(l, ">"))
	}
	if got, want := flat(strings.Join(quote, "\n")), flat(reviewerOutput); got != want {
		t.Errorf("section 7.4's quote of the reviewers' # Output:\n%s\nthe code's:\n%s", got, want)
	}
	for _, r := range []string{verdictReminder, architectReminder, ownerReminder} {
		if !strings.Contains(flat(spec), flat(r)) {
			t.Errorf("section 7.4 does not quote the reminder %q", r)
		}
	}
}
