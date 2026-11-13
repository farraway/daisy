package feedback

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

const log = "# Feedback Log\n\n---\n\n## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nOne.\n\n## 2026-08-02 1100\n\nUnheadered.\n  \n\n## 2026-08-03 1200 extra\n**Workflow:**   plan  \n\nThree.\nlast line"

// Entries start at `## YYYY-MM-DD HHMM`; each runs to the next, trailing
// newlines dropped (a line of spaces is kept, as the bash's command
// substitution kept it); the header is what comes before the first.
func TestParse(t *testing.T) {
	l := Parse([]byte(log))
	if l.Header != "# Feedback Log\n\n---" {
		t.Errorf("header %q", l.Header)
	}
	want := []Entry{
		{Text: "## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nOne.", Workflow: "prompts/widgetflow"},
		{Text: "## 2026-08-02 1100\n\nUnheadered.\n  ", Workflow: ""},
		{Text: "## 2026-08-03 1200 extra\n**Workflow:**   plan  \n\nThree.\nlast line", Workflow: "plan"},
	}
	if !reflect.DeepEqual(l.Entries, want) {
		t.Errorf("entries\n%q\nwant\n%q", l.Entries, want)
	}
	if got := (Entry{}).Tag(); got != "daisy" {
		t.Errorf("an unheadered entry's tag is %q", got)
	}
	if l := Parse([]byte("# Feedback Log\n\n---\n")); len(l.Entries) != 0 || l.Header != "# Feedback Log\n\n---" {
		t.Errorf("no entries: %+v", l)
	}
}

// Keep writes the header and the kept entries one blank line apart (case
// 66); Archive appends each entry and its provenance line.
func TestKeepAndArchive(t *testing.T) {
	l := Parse([]byte(log))
	got := string(l.Keep(func(i int) bool { return i != 0 }))
	want := "# Feedback Log\n\n---\n\n## 2026-08-02 1100\n\nUnheadered.\n  \n\n## 2026-08-03 1200 extra\n**Workflow:**   plan  \n\nThree.\nlast line\n"
	if got != want {
		t.Errorf("keep\n%q\nwant\n%q", got, want)
	}
	headless := Parse([]byte("## 2026-01-01 0000\nA.\n\n## 2026-01-02 0000\nB.\n"))
	if got := string(headless.Keep(func(int) bool { return true })); got != "## 2026-01-01 0000\nA.\n\n## 2026-01-02 0000\nB.\n" {
		t.Errorf("no header: %q", got)
	}
	if got := string(headless.Keep(func(i int) bool { return i == 1 })); got != "## 2026-01-02 0000\nB.\n" {
		t.Errorf("no header, first dropped: %q", got)
	}
	prov := "**Addressed by:** `daisy optimize` 2026-11-01 — `prompts/widgetflow.md` (rules)"
	if got, want := string(Archive([]byte("# Archive\n\n---\n"), l.Entries[:1], prov)), "# Archive\n\n---\n\n## 2026-08-01 1000\n**Workflow:** prompts/widgetflow\n\nOne.\n\n"+prov+"\n"; got != want {
		t.Errorf("archive\n%q\nwant\n%q", got, want)
	}
	if got := string(Archive([]byte("# Archive"), l.Entries[:1], prov)); !strings.HasPrefix(got, "# Archive\n\n## 2026-08-01") {
		t.Errorf("an archive without a final newline gets one: %q", got)
	}
}

// Record appends an entry as the bash did.
func TestRecord(t *testing.T) {
	if got, want := string(Record([]byte("# Log\n"), "2026-11-01 0930", "skills/git", "Wrong.")), "# Log\n\n## 2026-11-01 0930\n**Workflow:** skills/git\n\nWrong.\n"; got != want {
		t.Errorf("%q", got)
	}
	if got, want := string(Record([]byte("# Log\n"), "2026-11-01 0930", "", "a\nb")), "# Log\n\n## 2026-11-01 0930\n\na\nb\n"; got != want {
		t.Errorf("%q", got)
	}
}

type tree struct{ root, home string }

func newTree(t *testing.T) tree {
	tr := tree{root: t.TempDir(), home: t.TempDir()}
	for _, f := range []string{
		"root/prompts/daisy.md", "root/prompts/github.md", "root/prompts/retrospective.md",
		"root/skills/git/SKILL.md", "root/skills/testing/SKILL.md", "home/prompts/personal.md",
		"home/skills/widget/SKILL.md", "root/prompts/evals/new-day.md",
	} {
		write(t, tr.path(f), "## Rules\n\n1. a\n")
	}
	return tr
}

func (tr tree) path(f string) string {
	if rest, ok := strings.CutPrefix(f, "root/"); ok {
		return filepath.Join(tr.root, rest)
	}
	return filepath.Join(tr.home, strings.TrimPrefix(f, "home/"))
}

func write(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Tag resolution (case 67): `prompts/<n>` the home's then the root's
// prompt; `skills/<n>` the skill source (the home's over the root's); a
// bare name a prompt, then a skill, then the aliases daisy.md owns; a name
// holding a slash or naming nothing resolves to nothing.
func TestResolve(t *testing.T) {
	tr := newTree(t)
	fsys := ops.Real().FS()
	for tag, want := range map[string]string{
		"prompts/github":           "root/prompts/github.md",
		"github":                   "root/prompts/github.md",
		"skills/git":               "root/skills/git/SKILL.md",
		"git":                      "root/skills/git/SKILL.md",
		"daisy":                    "root/prompts/daisy.md",
		"new-day":                  "root/prompts/daisy.md",
		"home":                     "root/prompts/daisy.md",
		"personal":                 "home/prompts/personal.md",
		"prompts/personal":         "home/prompts/personal.md",
		"widget":                   "home/skills/widget/SKILL.md",
		"daisy-write":              "",
		"prompts/nope":             "",
		"skills/nope":              "",
		"prompts/evals/new-day":    "",
		"prompts/../prompts/daisy": "",
		"":                         "",
		"prompts/":                 "",
	} {
		got, ok := Resolve(fsys, tr.root, tr.home, tag)
		if want == "" {
			if ok {
				t.Errorf("%q resolved to %+v", tag, got)
			}
			continue
		}
		if !ok || got.Path != tr.path(want) {
			t.Errorf("%q: %+v %v, want %s", tag, got, ok, want)
		}
	}
	write(t, tr.path("home/prompts/github.md"), "x")
	if got, _ := Resolve(fsys, tr.root, tr.home, "prompts/github"); got.Path != tr.path("home/prompts/github.md") || got.Label != "prompts/github" {
		t.Errorf("the home's prompt shadows the root's: %+v", got)
	}
	if got, _ := Resolve(fsys, tr.root, tr.home, "new-day"); got.Label != "prompts/daisy" || got.Skill {
		t.Errorf("an alias's label: %+v", got)
	}
	// An alias is prompts/daisy, the home's shadowing the root's (review
	// pass 1, F2).
	write(t, tr.path("home/prompts/daisy.md"), "x")
	for _, tag := range []string{"new-day", "daisy", "prompts/daisy"} {
		if got, _ := Resolve(fsys, tr.root, tr.home, tag); got.Path != tr.path("home/prompts/daisy.md") {
			t.Errorf("%s: %+v", tag, got)
		}
	}
	if got, _ := Resolve(fsys, tr.root, tr.home, "git"); got.Label != "skills/git" || !got.Skill {
		t.Errorf("a skill's label: %+v", got)
	}
	want := []string{"prompts/daisy", "prompts/github", "prompts/personal", "prompts/retrospective", "skills/git", "skills/testing", "skills/widget"}
	if got := Labels(fsys, tr.root, tr.home); !reflect.DeepEqual(got, want) {
		t.Errorf("labels %q", got)
	}
}

// A skill carrying LICENSE or NOTICE.md is vendored; a prompt never is.
func TestVendored(t *testing.T) {
	tr := newTree(t)
	fsys := ops.Real().FS()
	git, _ := Resolve(fsys, tr.root, tr.home, "skills/git")
	if Vendored(fsys, git) {
		t.Error("git is not vendored")
	}
	write(t, filepath.Join(tr.root, "skills", "git", "NOTICE.md"), "x")
	if !Vendored(fsys, git) {
		t.Error("NOTICE.md makes it vendored")
	}
	// A prompt is not, even beside a LICENSE or NOTICE.md of its directory.
	write(t, filepath.Join(tr.root, "prompts", "LICENSE"), "x")
	write(t, filepath.Join(tr.root, "prompts", "NOTICE.md"), "x")
	if daisy, ok := Resolve(fsys, tr.root, tr.home, "prompts/daisy"); !ok || Vendored(fsys, daisy) {
		t.Error("a prompt is vendored")
	}
}

// Grouping: entries go to the target their tag resolves to (two tags for
// one file are one target), in order of first appearance; an unheadered
// entry is daisy's; a tag naming nothing is its own group, unresolved;
// a scope keeps one target's entries; every active target and every pending
// one over 3 rules is flagged (4 are, 3 are not), and a pending one
// carries its flag inline.
func TestBrief(t *testing.T) {
	tr := newTree(t)
	fsys := ops.Real().FS()
	many := "## Rules\n\n" + strings.Repeat("1. r\n", 4)
	write(t, tr.path("root/prompts/plan.md"), many)
	write(t, tr.path("root/prompts/retrospective.md"), many)
	write(t, tr.path("root/prompts/github.md"), "## Rules\n\n"+strings.Repeat("1. r\n", 3))
	write(t, tr.path("root/skills/testing/SKILL.md"), many)
	l := Parse([]byte("# L\n\n## 2026-08-01 1000\n**Workflow:** plan\n\na\n\n## 2026-08-02 1000\n\nb\n\n## 2026-08-03 1000\n**Workflow:** prompts/plan\n\nc\n\n## 2026-08-04 1000\n**Workflow:** daisy-write\n\nd\n"))
	active := []string{"retrospective", "github", "plan", "daisy"}
	b := Brief(fsys, tr.root, tr.home, l, active, nil)
	var got []string
	for _, g := range b.Pending {
		got = append(got, g.Tag+"="+g.Target.Label+":"+strings.Trim(strings.Repeat("x", len(g.Entries)), ""))
	}
	if want := []string{"plan=prompts/plan:xx", "daisy=prompts/daisy:x", "daisy-write=:x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("groups %q", got)
	}
	if g := b.Pending[0]; g.Rules != 4 || !g.Flagged() || !g.HasRules {
		t.Errorf("plan: %+v", g)
	}
	if g := b.Pending[1]; g.Rules != 1 || g.Flagged() {
		t.Errorf("daisy: %+v", g)
	}
	if g := b.Pending[2]; g.Resolved {
		t.Errorf("daisy-write resolved: %+v", g)
	}
	var flagged []string
	for _, f := range b.Flagged {
		flagged = append(flagged, f.Target.Label)
	}
	if want := []string{"prompts/retrospective", "skills/testing"}; !reflect.DeepEqual(flagged, want) {
		t.Errorf("flagged %q (github has 3, plan is pending)", flagged)
	}

	plan, _ := Resolve(fsys, tr.root, tr.home, "plan")
	scoped := Brief(fsys, tr.root, tr.home, l, active, &plan)
	if len(scoped.Pending) != 1 || scoped.Pending[0].Target.Label != "prompts/plan" || len(scoped.Flagged) != 0 {
		t.Errorf("scoped to plan: %+v", scoped)
	}
	if len(b.Errors) != 0 {
		t.Errorf("errors %v", b.Errors)
	}
	// A target that cannot be read is an error of the briefing, not a
	// target without rules (review pass 1, F3).
	os.Chmod(tr.path("root/prompts/plan.md"), 0)
	os.Chmod(tr.path("root/skills/testing/SKILL.md"), 0)
	if os.Getuid() != 0 {
		if e := Brief(fsys, tr.root, tr.home, l, active, nil).Errors; len(e) != 2 || !strings.Contains(e[0].Error(), "cannot read "+tr.path("root/prompts/plan.md")) {
			t.Errorf("unreadable: %v", e)
		}
	}
	os.Chmod(tr.path("root/prompts/plan.md"), 0o644)
	os.Chmod(tr.path("root/skills/testing/SKILL.md"), 0o644)
	retro, _ := Resolve(fsys, tr.root, tr.home, "retrospective")
	if s := Brief(fsys, tr.root, tr.home, l, active, &retro); len(s.Pending) != 0 || len(s.Flagged) != 1 {
		t.Errorf("scoped to an unpending flagged target: %+v", s)
	}
}

// The diff is unified, three lines of context, a hunk per change region.
func TestDiff(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	b := "1\n2\nthree\n4\n5\n6\n7\n8\n9\n10\neleven\n"
	want := "--- a/x\n+++ b/x\n@@ -1,6 +1,6 @@\n 1\n 2\n-3\n+three\n 4\n 5\n 6\n@@ -8,3 +8,4 @@\n 8\n 9\n 10\n+eleven\n"
	if got := Diff("a/x", "b/x", []byte(a), []byte(b)); got != want {
		t.Errorf("diff\n%s\nwant\n%s", got, want)
	}
	if got := Diff("a/x", "b/x", []byte(a), []byte(a)); got != "" {
		t.Errorf("no change: %q", got)
	}
	if got := Diff("a", "b", nil, []byte("x\n")); got != "--- a\n+++ b\n@@ -0,0 +1,1 @@\n+x\n" {
		t.Errorf("from empty: %q", got)
	}
	if got := Diff("a", "b", []byte("x"), []byte("x\n")); !strings.Contains(got, "\\ No newline at end of file") {
		t.Errorf("a missing final newline is shown: %q", got)
	}
}

// Every root skill is a target `daisy optimize` briefs with a `## Rules`
// section of its own, so `apply --rules` replaces that section rather than
// inserting one above the skill's first heading (for git, above `## 1. See
// the current state`).
func TestRootSkillsHaveARulesSection(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	fsys := ops.Real().FS()
	dirs, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var doc strings.Builder
	doc.WriteString("# L\n")
	for i, d := range dirs {
		fmt.Fprintf(&doc, "\n## 2026-08-%02d 1000\n**Workflow:** skills/%s\n\nx\n", i+1, d.Name())
	}
	b := Brief(fsys, root, t.TempDir(), Parse([]byte(doc.String())), nil, nil)
	if len(b.Pending) != len(dirs) || len(b.Errors) != 0 {
		t.Fatalf("%d skills, %d briefed, errors %v", len(dirs), len(b.Pending), b.Errors)
	}
	for _, g := range b.Pending {
		if !g.Resolved || !g.HasRules || g.Problem != nil {
			t.Errorf("%s: resolved %v, a ## Rules section %v, problem %v", g.Tag, g.Resolved, g.HasRules, g.Problem)
		}
	}
}
