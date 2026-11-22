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
