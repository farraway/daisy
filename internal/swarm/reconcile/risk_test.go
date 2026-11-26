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
		"- claim: a claim of slice " + s + " that the head does not show, a test that survives the probe the plan names for it among them; on a slice that changes only documentation, only a false statement the slice added or changed;\n" +
		"- test: a test the plan names for slice " + s + " that is missing on this head or fails;\n" +
		"- defect: a defect in the code the change adds or changes;\n" +
		"- introduced: a fault this revision introduced, behaviour that worked on a head reviewed before and fails on this one.\n\n" +
		realisticFor(realistic) + "\n\n" +
		`Every other finding is non-blocking, however sound: hardening against what the notes trust, a fault met only in a setup the slice does not support, a probe of your own that a test survives (unless the change that survives shows the head itself is wrong), and wording. ` +
		`Write a blocking finding's kind after its marker (from round 2, "unclosed" for an earlier one not closed) and, on the same line, the reproduction that shows it: a command, a test or the steps. ` +
		`Request changes only for a blocking finding; Daisy files every non-blocking one for the owner when the pull request merges, so none is lost.`
}

// realisticFor is the blocking sentence's paragraph that bounds every
// kind by what is realistic (section 7.11, rule 1 as built at Step R6-F;
// the maintainer's ruling A4 of 2026-11-22), "realistic" as given.
func realisticFor(realistic string) string {
	return `A finding of any kind blocks only when a realistic user, input or repository state meets it, "realistic" as ` + realistic + `; for tooling that builds or tests the code, a case the repository has today, not one it could have. ` +
		`A test that survives the probe the plan names for it still blocks: the plan named that case.`
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
	for name, text := range map[string]string{"": boundaries, "with CRLF": strings.ReplaceAll(boundaries, "\n", "\r\n"), "with spaces after it": "## Trust boundaries  \n",
		"as init seeds them": RepoNotesSeed(cfg(), cfg().Repos[1], nil)} {
		sys, _ := Render(delta(t, "adversary", RepoNote{Name: "lib", Text: text}))
		has(t, "notes that declare them "+name, taskOf(t, sys), []string{declaredBoundaries}, defaultBoundaries)
	}
	inSpec(t, defaultBoundaries)
}

// Every kind of blocking finding is bound by what is realistic (Step
// R6-F of the plan implementing the 2026-11-19 swarm roadmap; section
// 7.11, rule 1 as built; the maintainer's ruling A4 of 2026-11-22): the
// blocking sentence says so for every kind, in a paragraph of its own
// after the list, not in the defect's item alone, build and test tooling
// read as the repository has it today, and a test that survives its named
// probe still blocks; it does so for both reviewers in every round, with
// the notes' trust boundaries or the default; a later round's narrowing
// keeps the bound; and the worker answers, rather than fixes, a finding
// no realistic case meets.
func TestTaskTextRealisticForEveryKind(t *testing.T) {
	const oldDefect = `- defect: a defect in the code the change adds or changes that a realistic user or input meets`
	for name, c := range map[string]struct {
		repos     []RepoNote
		realistic string
	}{
		"declared": {[]RepoNote{{Name: "lib", Text: boundaries}}, declaredBoundaries},
		"default":  {nil, defaultBoundaries},
	} {
		for _, role := range []string{"adversary", "sympathetic"} {
			one := memoryJob(t, role, "swarm:review-0-ready")
			one.Pull.Reviews = nil
			rounds := map[string]*Job{"round 1": one, "an unchanged head's round": sameHead(t, role), "a delta round": delta(t, role)}
			if role == "sympathetic" {
				late := delta(t, role)
				late.Pull.Reviews = late.Pull.Reviews[:1] // the adversary's, on aaa
				rounds["a late first review"] = late
			}
			for round, j := range rounds {
				j.Repos = c.repos
				sys, _ := Render(j)
				task := taskOf(t, sys)
				what := name + ", " + role + ", " + round
				paragraphs(t, what, task, realisticFor(c.realistic))
				has(t, what, task, []string{"\n- defect: a defect in the code the change adds or changes;\n- introduced: "}, oldDefect)
				if strings.Index(task, realisticFor(c.realistic)) < strings.Index(task, "\n- introduced: ") {
					t.Errorf("%s: the bound comes before the list it bounds:\n%s", what, task)
				}
				switch round {
				case "a delta round":
					paragraphs(t, what, task, realisticFor(c.realistic), convergence("1"))
				case "a late first review":
					paragraphs(t, what, task, realisticFor(c.realistic), lateFirst("2", "1"))
				}
			}
		}
	}
	sys, _ := Render(memoryJob(t, "worker", "swarm:review-3-changes-requested"))
	has(t, "the worker on changes requested", taskOf(t, sys), []string{"or that no realistic user, input or repository state meets, is answered, not fixed"})
	inSpec(t, realisticFor(declaredBoundaries))
}

// On a slice that changes only documentation, a claim blocks only on a
// false statement the slice added or changed (Step R6-F of the plan
// implementing the 2026-11-19 swarm roadmap; section 7.11, rule 1 as
// built; the maintainer's ruling A3 of 2026-11-22): Daisy does not read
// which paths the head changes, so every reviewer's Task says it in the
// claim's item of the list, for the reviewer to apply, in every round,
// with the notes' trust boundaries or the default; the worker's sentence
// on a finding that does not block says it too.
func TestTaskTextDocsSliceClaim(t *testing.T) {
	const claim = "\n- claim: a claim of slice 1 that the head does not show, a test that survives the probe the plan names for it among them; on a slice that changes only documentation, only a false statement the slice added or changed;\n- test: "
	for name, repos := range map[string][]RepoNote{"declared": {{Name: "lib", Text: boundaries}}, "default": nil} {
		for _, role := range []string{"adversary", "sympathetic"} {
			one := memoryJob(t, role, "swarm:review-0-ready")
			one.Pull.Reviews = nil
			for round, j := range map[string]*Job{"round 1": one, "an unchanged head's round": sameHead(t, role), "a delta round": delta(t, role)} {
				j.Repos = repos
				sys, _ := Render(j)
				has(t, name+", "+role+", "+round, taskOf(t, sys), []string{claim})
			}
		}
	}
	sys, _ := Render(memoryJob(t, "worker", "swarm:review-3-changes-requested"))
	has(t, "the worker on changes requested", taskOf(t, sys), []string{"(a claim of the slice the head does not show, on a slice that changes only documentation only a false statement the slice added or changed; a test"})
}

// convergence is the convergence sentence of slice s (section 7.11, rule 5).
func convergence(s string) string {
	return `A blocking finding of the previous round or of your last review is closed when its own reproduction now passes on this head. ` +
		`From this round, only three kinds of finding block, each as the paragraphs above bound it: an earlier blocking finding not closed, a fault this revision introduced, and a claim of slice ` + s + ` that the head does not show; any other new finding, a new variant of a closed one among them, is non-blocking. ` +
		`A non-blocking finding the previous round or your last review already raised stands as written there: do not raise it again.`
}

// lateFirst is the text of a reviewer whose first review falls in round r.
func lateFirst(r, s string) string {
	return `This is round ` + r + ` of the pull request: a new finding blocks only when it is a claim of slice ` + s + ` that the head does not show, or a fault the latest revision introduced, each as the paragraphs above bound it; any other is non-blocking.`
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
	const want = `A blocking finding that is none of the kinds that block (a claim of the slice the head does not show, on a slice that changes only documentation only a false statement the slice added or changed; a test the plan names missing or failing; a defect in the changed code; a fault the revision introduced), or that no realistic user, input or repository state meets, is answered, not fixed: say why it does not block. A non-blocking finding needs no change; Daisy files it for the owner.`
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

// Every worker's Task says which gates are Daisy's (Step R6-A of the plan
// implementing the 2026-11-19 swarm roadmap; section 7.11, rule 9), a
// paragraph of its own after its check sentence: the declared check and
// the forge's checks, what the container cannot run being no reason to
// stop, or, with no check declared, the forge's alone.
func TestTaskTextWorkerGatesSentence(t *testing.T) {
	gates := func(check string) string {
		return `Daisy runs "` + check + `" on the head you push, in the repository's declared check environment with its test services, and the forge's required checks run before the merge: those gates are Daisy's, and a failure of theirs comes back to you. A part of them your container cannot run (a service or a network it lacks) goes under "Not established:"; it is no reason to write BLOCKED:.`
	}
	const none = `The forge's required checks run before the merge: they are Daisy's gates, and a failure of theirs comes back to you.`
	f, at := fixture()
	fast := job(f, at.N, "worker", "swarm:plan-1-accepted") // in app, whose check is declared
	for name, c := range map[string]struct {
		j            *Job
		after, gates string
		not          string
	}{
		"the fast lane, a check":           {fast, `Run "make check" before you finish and quote its last result.`, gates("make check"), none},
		"a slice, no check":                {memoryJob(t, "worker", "swarm:plan-1-accepted"), `No check command is declared; say how you tested.`, none, "Daisy runs"},
		"changes requested, with no check": {memoryJob(t, "worker", "swarm:review-3-changes-requested"), `No check command is declared; say how you tested.`, none, "Daisy runs"},
	} {
		sys, _ := Render(c.j)
		task := taskOf(t, sys)
		paragraphs(t, name, task, c.gates)
		has(t, name, task, []string{c.after + "\n\n" + c.gates + "\n\n" + `End what you write with a line "Not established:"`}, c.not)
	}
	for _, role := range []string{"architect", "adversary", "sympathetic"} {
		label := map[string]string{"architect": "swarm:plan-0-posted"}[role]
		if label == "" {
			label = "swarm:review-0-ready"
		}
		sys, _ := Render(memoryJob(t, role, label))
		has(t, role, taskOf(t, sys), nil, "those gates are Daisy's", "they are Daisy's gates")
	}
	inSpec(t, gates("<check>"), none)
}

// The sympathetic reviewer's default prompt reads and runs nothing (the
// maintainer's ruling of 2026-11-22; section 7.11, The defaults), so its
// Task does not offer its clone to build and test in, as the adversary's
// does, in round 1 and in a delta round.
func TestTaskTextSympatheticReadsItsClone(t *testing.T) {
	const build = "a throwaway clone you may build and test in; nothing you change there is kept"
	for role, parts := range map[string][2][]string{
		"adversary":   {{build}, nil},
		"sympathetic": {{"a throwaway clone; nothing you change there is kept"}, {"build and test"}},
	} {
		one := memoryJob(t, role, "swarm:review-0-ready")
		one.Pull.Reviews = nil
		for name, j := range map[string]*Job{"round 1": one, "a delta round": delta(t, role)} {
			sys, _ := Render(j)
			has(t, role+", "+name, taskOf(t, sys), parts[0], parts[1]...)
		}
	}
}

// The owner's and the QA's batch texts ask for a `Defect in:` line (Step
// R6-A of the plan implementing the 2026-11-19 swarm roadmap; section
// 7.11, rule 7), a paragraph of its own after the row's text; the owner's
// other texts and the other roles' do not.
func TestTaskTextsAskForDefectIn(t *testing.T) {
	const want = `When you find that a failure's cause is the change of a merged swarm pull request, name it in the job's body with a line "Defect in: <name>#<m>".`
	for role, row := range map[string]string{
		"owner": "and Daisy runs you again once they are all closed.",
		"qa":    "Daisy then holds the QA until the maintainer resumes the swarm.",
	} {
		f, at := fixture()
		j := job(f, at.N, role, "")
		j.N, j.Batch = 0, &Batch{Propose: 2}
		sys, _ := Render(j)
		task := taskOf(t, sys)
		paragraphs(t, role, task, want)
		has(t, role, task, []string{row + "\n\n" + want + "\n\nThe protected paths are "})
	}
	for _, c := range []struct{ role, label string }{{"owner", "swarm:proposal-posted"}, {"architect", "swarm:plan-0-posted"}, {"worker", "swarm:plan-1-accepted"}, {"adversary", "swarm:review-0-ready"}} {
		sys, _ := Render(memoryJob(t, c.role, c.label))
		has(t, c.role, taskOf(t, sys), nil, "Defect in:")
	}
	inSpec(t, want)
}

// judgement is the architect's text at the round limit's first time
// (section 7.11, rule 8), for pull request pull, slice s, "realistic" as
// given.
func judgement(pull, s, realistic string) string {
	return `This is the first time pull request ` + pull + ` reached the review round limit: before you revise, judge each blocking finding of its last round, under "# Pull request". ` +
		`A finding qualifies only when it is one of the kinds that block (a claim of slice ` + s + ` that the head does not show, a test that survives the probe the plan names for it among them, on a slice that changes only documentation only a false statement the slice added or changed; a test the plan names for slice ` + s + ` missing or failing; a defect in the code the change adds or changes; a fault the revision introduced), ` +
		`a realistic user, input or repository state meets it, "realistic" as ` + realistic + `, ` +
		`and, from round 2, it is an earlier blocking finding not closed, a fault the revision introduced or a claim of slice ` + s + ` that the head does not show. ` +
		`When none qualifies, write "OVERRULED: <why, finding by finding>" as the first line of your output instead of a plan: Daisy posts it on the pull request, the sympathetic reviewer reviews the head as it would after the adversary's approval, and the findings you overrule are filed for the owner, marked "outstanding at the round limit". When any qualifies, revise the plan as above.`
}

// The architect judges at the round limit (Step R6-D of the plan
// implementing the 2026-11-19 swarm roadmap; section 7.11, rule 8; the
// maintainer's ruling 1 of 2026-11-22, option (b) of decision 171): when
// the newest counted entry of the issue is the round limit's first note
// on the open pull request, the architect's Task holds the judgement, a
// paragraph of its own after the open-slice sentence, with the plan gate
// on (the revision) and off (the plan's text), "realistic" as the
// repository notes declare it or by the default; with the maintainer's
// comment after the note, the limit's later note, or no note, it does
// not.
func TestTaskTextArchitectJudgesAtTheLimit(t *testing.T) {
	const first = "Review round 1 of 1 on [acme/lib#1](https://forge.example.net/acme/lib/pulls/1) requested changes (was swarm:review-1-in-progress). The architect revises the plan from the reviews."
	const again = "Review round 1 of 1 on [acme/lib#1](https://forge.example.net/acme/lib/pulls/1) requested changes again (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked."
	const openSlice = `Revise the plan so that the slice can pass review: slice 1 keeps its number, its repository and its branch, and the worker continues on them; what it does, how it is tested and the slices after it may change.`
	at := func(t *testing.T, label string, then func(f *forge.Fake, at forge.Ref), repos ...RepoNote) string {
		t.Helper()
		f, at, pr, state := planned(t)
		planWorkspace(t, state)
		then(f, at)
		j := jobOn(f, at, pr, "architect", label)
		j.Outputs, j.Repos = Outputs{FS: ops.Real().FS(), State: state}, repos
		j.Cfg.Gates.Plan = label == "swarm:plan-0-posted"
		if err := j.readMemory(); err != nil {
			t.Fatal(err)
		}
		sys, _ := Render(j)
		return taskOf(t, sys)
	}
	note := func(text string) func(*forge.Fake, forge.Ref) {
		return func(f *forge.Fake, at forge.Ref) { f.Comment(at, marker("reconciler", 1)+text) }
	}
	for _, label := range []string{"swarm:plan-0-posted", "swarm:proposal-accepted"} {
		for name, c := range map[string]struct {
			repos     []RepoNote
			realistic string
		}{"the default": {nil, defaultBoundaries}, "the notes' trust boundaries": {[]RepoNote{{Name: "lib", Text: boundaries}}, declaredBoundaries}} {
			task := at(t, label, note(first), c.repos...)
			paragraphs(t, label+", "+name, task, judgement("lib#1", "1", c.realistic))
			if i := strings.Index(task, judgement("lib#1", "1", c.realistic)); i < strings.Index(task, openSlice) {
				t.Errorf("%s, %s: the judgement is not after the open-slice sentence:\n%s", label, name, task)
			}
		}
		for name, then := range map[string]func(*forge.Fake, forge.Ref){
			"no note": func(*forge.Fake, forge.Ref) {},
			"the maintainer's comment after the note": func(f *forge.Fake, at forge.Ref) {
				note(first)(f, at)
				f.Say(at, "jane", "Re-plan it smaller.")
			},
			"the limit's later note": note(again),
		} {
			has(t, label+", "+name, at(t, label, then), []string{openSlice}, "OVERRULED:", "round limit:")
		}
	}
	inSpec(t, judgement("<name>#<m>", "<s>", "<realistic>"))
}
