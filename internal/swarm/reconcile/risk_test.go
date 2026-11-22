package reconcile

import (
	"os"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
)

// boundaries is a repository's notes that declare its trust boundaries.
const boundaries = "# Lib\n\n## Trust boundaries\n\nTrusted: the environment. Not trusted: an upload.\n"

// blockingList is the blocking sentence (section 7.11, rule 1) of slice
// s, realistic as the repository notes declare it or, with none, the
// default.
func blockingList(s, realistic string) string {
	return "A finding is blocking only when it is one of these four kinds:\n\n" +
		"- claim: a claim of slice " + s + " that the head does not show, a test that survives the probe the plan names for it among them;\n" +
		"- test: a test the plan names for slice " + s + " that is missing on this head or fails;\n" +
		"- defect: a defect in the code the change adds or changes that a realistic user or input meets, \"realistic\" as " + realistic + ";\n" +
		"- introduced: a fault this revision introduced, behaviour that worked on a head reviewed before and fails on this one.\n\n" +
		`Every other finding is non-blocking, however sound: hardening against what the notes trust, a fault met only in a setup the slice does not support, a probe of your own that a test survives (unless the change that survives shows the head itself is wrong), and wording. ` +
		`Write a blocking finding's kind after its marker (from round 2, "unclosed" for an earlier one not closed) and, on the same line, the reproduction that shows it: a command, a test or the steps. ` +
		`Request changes only for a blocking finding; Daisy files every non-blocking one for the owner when the pull request merges, so none is lost.`
}

const (
	declaredBoundaries = `"Trust boundaries" in the repository notes and the slice's "Unsupported:" line define it`
	defaultBoundaries  = `the slice's "Unsupported:" line defines it, and, since the repository notes declare no trust boundaries, trusting the product's own process environment, configuration, filesystem and umask and the toolchain's settings, and nothing a user or a network peer sends`
)

// inSpec reports a text section 7.11 does not quote, its wrapping, its
// block quote's `>` and its backticks undone.
func inSpec(t *testing.T, texts ...string) {
	t.Helper()
	b, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	_, spec, _ := strings.Cut(string(b), "\n### 7.11 ")
	spec, _, _ = strings.Cut(spec, "\n## 8. ")
	flat := func(s string) string {
		return strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, "\n>", "\n"), "`", "")), " ")
	}
	for _, text := range texts {
		if !strings.Contains(flat(spec), flat(text)) {
			t.Errorf("section 7.11 does not quote %q", text)
		}
	}
}

// taskOf is the # Task block of a rendered system.md.
func taskOf(t *testing.T, sys string) string {
	t.Helper()
	bs := blocksOf(sys)
	if len(bs) == 0 || bs[len(bs)-1].heading != "# Task" {
		t.Fatalf("system.md does not end with # Task:\n%s", sys)
	}
	return bs[len(bs)-1].text
}

// paragraphs reports each text that is not a paragraph of its own in the
// Task, and one that comes before the text before it: an instruction in
// the middle of a paragraph is one a smaller model misses (decision 167).
func paragraphs(t *testing.T, what, task string, texts ...string) {
	t.Helper()
	at := 0
	for _, p := range texts {
		i := strings.Index(task, "\n\n"+p+"\n\n")
		if i < 0 {
			t.Errorf("%s: no paragraph %q in the Task:\n%s", what, p, task)
			continue
		}
		if i < at {
			t.Errorf("%s: %q is out of order in the Task:\n%s", what, p, task)
		}
		at = i
	}
}

// later is role's job on the planned fixture's slice 1, with the job's
// workspace, in round 2: what happens on its pull request after both
// reviewed aaa is then; repos are the repository notes.
func later(t *testing.T, role string, then func(f *forge.Fake, pr forge.Ref), repos ...RepoNote) *Job {
	t.Helper()
	f, at, pr, state := planned(t)
	planWorkspace(t, state)
	then(f, pr)
	j := jobOn(f, at, pr, role, "swarm:review-0-ready")
	j.Outputs, j.Repos = Outputs{FS: ops.Real().FS(), State: state}, repos
	if err := j.readMemory(); err != nil {
		t.Fatal(err)
	}
	return j
}

// delta is a delta review's, of bbb since aaa.
func delta(t *testing.T, role string, repos ...RepoNote) *Job {
	t.Helper()
	return later(t, role, func(f *forge.Fake, pr forge.Ref) { f.Push(pr, "bbb") }, repos...)
}

// sameHead is that of a round on the head both reviewed, which the
// maintainer's comment reopened.
func sameHead(t *testing.T, role string) *Job {
	t.Helper()
	return later(t, role, func(f *forge.Fake, pr forge.Ref) { f.Say(pr, "jane", "x.go:1 is right.") })
}

// The blocking sentence (Step R6-A of the plan implementing the
// 2026-11-19 swarm roadmap; section 7.11, rules 1, 2 and 4; decision
// 171): every reviewer's Task holds the four kinds of finding that
// block, "realistic" read by the repository notes' trust boundaries and
// the slice's unsupported setups, one text for both reviewers in every
// round, a paragraph of its own after the row's text and before the
// slice sentence; the # Output block, the same for every run of the
// role, defers to it and shows a blocking finding's line with its kind
// and its reproduction.
func TestTaskTextBlockingSentence(t *testing.T) {
	want := blockingList("1", declaredBoundaries)
	notes := RepoNote{Name: "lib", Text: boundaries}
	const slice = `Read slice 1, /job/memory/slices/1/slice.md, first; `
	for _, role := range []string{"adversary", "sympathetic"} {
		one := memoryJob(t, role, "swarm:review-0-ready")
		one.Pull.Reviews = nil
		rounds := map[string]*Job{"round 1": one, "an unchanged head's round": sameHead(t, role), "a delta round": delta(t, role)}
		for name, j := range rounds {
			j.Repos = []RepoNote{notes}
			sys, _ := Render(j)
			task := taskOf(t, sys)
			paragraphs(t, role+", "+name, task, want)
			if n := strings.Count(task, "Request changes only"); n != 1 {
				t.Errorf("%s, %s: the Task says when to request changes %d times, want once:\n%s", role, name, n, task)
			}
			row, list, read := strings.Index(task, `"Not established:" lists against what`), strings.Index(task, want), strings.Index(task, slice)
			if row < 0 || list < row || read < list {
				t.Errorf("%s, %s: the row's text at %d, the blocking sentence at %d, the slice sentence at %d:\n%s", role, name, row, list, read, task)
			}
			out := blocksOf(sys)[1].text
			has(t, role+"'s # Output", out, []string{
				"Request changes only for a blocking finding, of a kind the Task lists; approve otherwise.",
				"\n[blocking] <kind> <file>:<line>: <what is wrong>; reproduced by <a command, a test or the steps>\n"},
				"only when a finding is blocking", "lib", "slice 1")
		}
	}
	inSpec(t, blockingList("<s>", declaredBoundaries))
}

// With no `## Trust boundaries` heading in the repository notes the
// blocking sentence states the default (section 7.11, rule 2); a heading
// at another level, in a fenced block or in the prose is none.
func TestTaskTextBlockingSentenceWithoutTrustBoundaries(t *testing.T) {
	for name, repos := range map[string][]RepoNote{
		"no notes":          nil,
		"notes without it":  {{Name: "lib", Text: "# Lib\n\nThe library. Trust boundaries: none.\n"}},
		"in a fenced block": {{Name: "lib", Text: "# Lib\n\n```\n## Trust boundaries\n```\n"}},
		"at another level":  {{Name: "lib", Text: "# Trust boundaries\n\n### Trust boundaries\n"}},
		"a longer heading":  {{Name: "lib", Text: "## Trust boundaries of lib\n"}},
	} {
		for _, role := range []string{"adversary", "sympathetic"} {
			sys, _ := Render(delta(t, role, repos...))
			task := taskOf(t, sys)
			paragraphs(t, role+", "+name, task, blockingList("1", defaultBoundaries))
			has(t, role+", "+name, task, nil, declaredBoundaries)
		}
	}
	for name, text := range map[string]string{"": boundaries, "with CRLF": strings.ReplaceAll(boundaries, "\n", "\r\n"), "with spaces after it": "## Trust boundaries  \n"} {
		sys, _ := Render(delta(t, "adversary", RepoNote{Name: "lib", Text: text}))
		has(t, "notes that declare them "+name, taskOf(t, sys), []string{declaredBoundaries}, defaultBoundaries)
	}
	inSpec(t, defaultBoundaries)
}

// convergence is the convergence sentence of slice s (section 7.11, rule 5).
func convergence(s string) string {
	return `A blocking finding of the previous round or of your last review is closed when its own reproduction now passes on this head. ` +
		`From this round, only three kinds of finding block: an earlier blocking finding not closed, a fault this revision introduced, and a claim of slice ` + s + ` that the head does not show; any other new finding, a new variant of a closed one among them, is non-blocking. ` +
		`A non-blocking finding the previous round or your last review already raised stands as written there: do not raise it again.`
}

// lateFirst is the text of a reviewer whose first review falls in round r.
func lateFirst(r, s string) string {
	return `This is round ` + r + ` of the pull request: a new finding blocks only when it is a claim of slice ` + s + ` that the head does not show, or a fault the latest revision introduced; any other is non-blocking.`
}

// The convergence sentence (Step R6-A of the plan implementing the
// 2026-11-19 swarm roadmap; section 7.11, rule 5): a delta round's Task
// narrows the list of what blocks, in the paragraph after it, for both
// reviewers, in place of the sentence on code changed since the previous
// head; round 1 and the rounds on an unchanged head are not narrowed by
// it, and keep their own sentences.
func TestTaskTextConvergenceSentence(t *testing.T) {
	for _, role := range []string{"adversary", "sympathetic"} {
		sys, _ := Render(delta(t, role, RepoNote{Name: "lib", Text: boundaries}))
		task := taskOf(t, sys)
		paragraphs(t, role, task, blockingList("1", declaredBoundaries), convergence("1"))
		if strings.Index(task, "Read slice 1, ") < strings.Index(task, convergence("1")) {
			t.Errorf("%s: the slice sentence comes before the convergence sentence:\n%s", role, task)
		}
		has(t, role, task, nil, "only on code changed since", "on code unchanged", "This is round 2 of the pull request:")
		one := memoryJob(t, role, "swarm:review-0-ready")
		one.Pull.Reviews = nil
		for name, j := range map[string]*Job{"round 1": one, "an unchanged head's round": sameHead(t, role)} {
			sys, _ := Render(j)
			has(t, role+", "+name, taskOf(t, sys), nil, "only three kinds of finding block", "a new finding blocks only when")
		}
		sys, _ = Render(sameHead(t, role))
		has(t, role+", an unchanged head's round", taskOf(t, sys), []string{"This is round 2, on the head you reviewed before (aaa): the maintainer commented", "raise a new blocking finding only where the comment shows one."})
	}
	inSpec(t, convergence("<s>"))
}

// A reviewer whose first review falls in a later round (the sympathetic
// reviewer's, decision 163) reviews the whole change and is bound by the
// round (section 7.11, rule 5): its Task says so in the paragraph after
// the list of what blocks; in round 1, and with a review of its own, it
// has no such sentence.
func TestTaskTextLateFirstReviewIsBoundByTheRound(t *testing.T) {
	j := delta(t, "sympathetic", RepoNote{Name: "lib", Text: boundaries})
	j.Pull.Reviews = j.Pull.Reviews[:1] // the adversary's, on aaa
	sys, _ := Render(j)
	task := taskOf(t, sys)
	has(t, "the late first review", task, []string{`Review pull request lib#1, slice 1 of issue #1. The change is "git diff main...HEAD"`}, "only three kinds of finding block", "This is round 2 of pull request lib#1")
	paragraphs(t, "the late first review", task, blockingList("1", declaredBoundaries), lateFirst("2", "1"))
	one := delta(t, "sympathetic")
	one.Pull.Reviews = nil
	for name, j := range map[string]*Job{"round 1": one, "a review of its own": delta(t, "sympathetic")} {
		sys, _ := Render(j)
		has(t, name, taskOf(t, sys), nil, "a new finding blocks only when")
	}
	inSpec(t, lateFirst("<r>", "<s>"))
}

// The worker answers, rather than fixes, a blocking finding that is none
// of the kinds that block (section 7.11, rule 5): its Task on changes
// requested says so, a paragraph of its own after the row's text; its
// Task on a slice to build does not.
func TestTaskTextWorkerAnswersAFindingThatDoesNotBlock(t *testing.T) {
	const want = `A blocking finding that is none of the kinds that block (a claim of the slice the head does not show, a test the plan names missing or failing, a defect in the changed code a realistic user or input meets, a fault the revision introduced) is answered, not fixed: say which it is not, and why. A non-blocking finding needs no change; Daisy files it for the owner.`
	sys, _ := Render(memoryJob(t, "worker", "swarm:review-3-changes-requested"))
	task := taskOf(t, sys)
	paragraphs(t, "changes requested", task, want)
	if i := strings.Index(task, want); i < strings.Index(task, "the reviewers judge it against the unchanged head.") || i > strings.Index(task, "Read slice 1, ") {
		t.Errorf("the sentence is not between the row's text and the slice sentence:\n%s", task)
	}
	sys, _ = Render(memoryJob(t, "worker", "swarm:plan-1-accepted"))
	has(t, "a slice to build", taskOf(t, sys), nil, "is answered, not fixed")
	inSpec(t, want)
}

// planLines is what an architect's text asks of the plan (section 7.11,
// rules 3 and 4).
const planLines = `The plan holds two lines the worker and the reviewers read:

- under "## Test Plan", for each claim's test, "Probe: <the change to the code under test that the test must fail on>": a change the code can make and its test can tell (not an equivalent one, nor a fixture no test reads), or, for a guard of kept behaviour, the break of the live path it exercises;
- in each slice's item under "## Steps", "Unsupported: <the setups the slice does not support>", or "Unsupported: none".`

// The plan names its tests' probes and its slices' unsupported setups
// (Step R6-A of the plan implementing the 2026-11-19 swarm roadmap;
// section 7.11, rules 3 and 4): each architect's text that asks for a
// plan or a revised plan asks for the two lines, after the text that
// asks for the plan and before the builder sentence, whose slice section
// holds both; its answer to a question asks for neither.
func TestTaskTextPlanAsksForProbesAndUnsupported(t *testing.T) {
	const builder = `(its files, types and signatures, claim and test, its tests' probes and its "Unsupported:" line, and what of the research, constraints and decisions bears on it)`
	for _, c := range []struct{ name, label, ask string }{
		{"the plan", "swarm:proposal-accepted", `in a fenced "mermaid" block under that step.`},
		{"its revision", "swarm:plan-0-posted", `a diagram where a slice touches more than one component.`},
		{"the accepted plan's revision", "swarm:plan-1-accepted", `If the remaining slices stand as they are, write "STANDS: <why>"`},
	} {
		sys, _ := Render(memoryJob(t, "architect", c.label))
		task := taskOf(t, sys)
		paragraphs(t, c.name, task, planLines)
		has(t, c.name, task, []string{c.ask, "\n\nThen write the builder plan to /job/io/builder.md", builder})
		if i := strings.Index(task, planLines); i < strings.Index(task, c.ask) || i > strings.Index(task, "Then write the builder plan") {
			t.Errorf("%s: the two lines are not between the text that asks for the plan and the builder sentence:\n%s", c.name, task)
		}
	}
	j := memoryJob(t, "architect", "swarm:plan-1-accepted")
	j.Question = true
	sys, _ := Render(j)
	has(t, "an answer to a question", taskOf(t, sys), nil, "Probe:", "Unsupported:")
	inSpec(t, planLines)
}

// The worker applies each probe its slice's plan names and reports what
// it showed (section 7.11, rule 3): its Task on a slice of a plan says
// so, a paragraph of its own after the row's text; the fast lane has no
// plan to name one.
func TestTaskTextWorkerReportsProbes(t *testing.T) {
	const want = `For each test of this slice whose Test Plan claim names a Probe, apply the probe, run the test and restore the code: report, in the description or your comment, the test, the probe and the failure it showed, or that it survived.`
	sys, _ := Render(memoryJob(t, "worker", "swarm:plan-1-accepted"))
	task := taskOf(t, sys)
	paragraphs(t, "a slice of a plan", task, want)
	if i := strings.Index(task, want); i < strings.Index(task, "done, not in this slice, or failing.") || i > strings.Index(task, "Read slice 1, ") {
		t.Errorf("the sentence is not between the row's text and the slice sentence:\n%s", task)
	}
	f, at := fixture()
	sys, _ = Render(job(f, at.N, "worker", "swarm:plan-1-accepted"))
	has(t, "the fast lane", taskOf(t, sys), nil, "names a Probe")
	inSpec(t, want)
}

// The adversary checks the probes the plan names, after its rollback, and
// a probe of its own is no ground to block (section 7.11, rule 3), a
// paragraph of its own in every round; the sympathetic reviewer's Task
// asks for none.
func TestTaskTextAdversaryChecksTheNamedProbes(t *testing.T) {
	const want = `Then apply, for each such test, the probe its Test Plan claim names, run the test and restore the head, comparing the worker's report of it: a test that survives its named probe is a "[blocking]" finding of kind "claim". A test that survives a probe of your own is a "[non-blocking]" finding, unless the change that survives shows that the head itself is wrong, which is then a defect, judged as any other.`
	one := memoryJob(t, "adversary", "swarm:review-0-ready")
	one.Pull.Reviews = nil
	for name, j := range map[string]*Job{"round 1": one, "a delta round": delta(t, "adversary"), "an unchanged head's round": sameHead(t, "adversary")} {
		sys, _ := Render(j)
		task := taskOf(t, sys)
		paragraphs(t, name, task, want)
		if i := strings.Index(task, "A claim reported not in this slice is not checked here.\n\n"+want); i < 0 {
			t.Errorf("%s: the probes do not follow the rollback:\n%s", name, task)
		}
	}
	for _, j := range []*Job{memoryJob(t, "sympathetic", "swarm:review-0-ready"), delta(t, "sympathetic")} {
		sys, _ := Render(j)
		has(t, "the sympathetic reviewer", taskOf(t, sys), nil, "the probe its Test Plan claim names")
	}
	inSpec(t, want)
}
