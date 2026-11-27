package reconcile

import (
	"slices"
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// An issue's dependencies (section 12.1, rule 1): its newest `Depends:`
// declaration, the proposal's (the swarm's body, marked role=owner or
// role=qa), the accepted plan's (dated by its post) or a maintainer's
// comment that is the line alone, outside a fence; then the maintainer's
// newest `Blocked by #<m>` comments; the issue itself left out.
func TestDependencies(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	owner := marker("owner", 1) + "Brief: docs/briefs/a.md — M1\nClass: routine\nDepends: #3, #4\nTouches: src/\n\nThe job.\n"
	plan := "## Steps\n\nDepends: #5\n\n1. app: the slice.\n"
	at := func(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }
	for _, tc := range []struct {
		name            string
		author, body    string
		plan            string
		planAt          time.Time
		comments        []forge.Comment
		declared, block []int
	}{
		{"the owner's proposal", "swarm-bot", owner, "", time.Time{}, nil, []int{3, 4}, nil},
		{"the QA's proposal", "swarm-bot", marker("qa", 1) + "Class: routine\nDepends: #6\n", "", time.Time{}, nil, []int{6}, nil},
		{"an architect's body is no proposal", "swarm-bot", marker("architect", 1) + "Depends: #6\n", "", time.Time{}, nil, nil, nil},
		{"a body the maintainer wrote", "jane", "Depends: #6\n", "", time.Time{}, nil, nil, nil},
		{"none declared", "swarm-bot", marker("owner", 1) + "Brief: x — M1\nClass: routine\n", "", time.Time{}, nil, nil, nil},
		{"Depends: none", "swarm-bot", marker("owner", 1) + "Depends: none\n", "", time.Time{}, nil, nil, nil},
		{"inside a fence", "swarm-bot", marker("owner", 1) + "```\nDepends: #3\n```\n", "", time.Time{}, nil, nil, nil},
		{"in a sentence", "swarm-bot", marker("owner", 1) + "It Depends: #3 first.\n", "", time.Time{}, nil, nil, nil},
		{"the plan refines the proposal's", "swarm-bot", owner, plan, at(2), nil, []int{5}, nil},
		{"the plan's none refines it", "swarm-bot", owner, "## Steps\n\nDepends: none\n", at(2), nil, nil, nil},
		{"a plan with none keeps the proposal's", "swarm-bot", owner, "## Steps\n\n1. app: x.\n", at(2), nil, []int{3, 4}, nil},
		{"the maintainer overrides the plan", "swarm-bot", owner, plan, at(2), []forge.Comment{{Author: "jane", Body: " Depends: #7\n", Created: at(3)}}, []int{7}, nil},
		{"a plan after the maintainer's", "swarm-bot", owner, plan, at(4), []forge.Comment{{Author: "jane", Body: "Depends: #7", Created: at(3)}}, []int{5}, nil},
		{"an untrusted login's", "swarm-bot", owner, "", time.Time{}, []forge.Comment{{Author: "mallory", Body: "Depends: none", Created: at(3)}}, []int{3, 4}, nil},
		{"the swarm's comment", "swarm-bot", owner, "", time.Time{}, []forge.Comment{{Author: "swarm-bot", Body: "Depends: none", Created: at(3)}}, []int{3, 4}, nil},
		{"a maintainer's comment with more", "swarm-bot", owner, "", time.Time{}, []forge.Comment{{Author: "jane", Body: "Depends: none\nfor now", Created: at(3)}}, []int{3, 4}, nil},
		{"itself", "swarm-bot", marker("owner", 1) + "Depends: #1, #3\n", "", time.Time{}, []forge.Comment{{Author: "jane", Body: "Blocked by #1", Created: at(3)}}, []int{3}, nil},
		{"with the maintainer's Blocked by", "swarm-bot", owner, "", time.Time{}, []forge.Comment{{Author: "jane", Body: "Blocked by #8", Created: at(3)}, {Author: "jane", Body: "Depends: #3", Created: at(4)}, {Author: "jane", Body: "Blocked by #9", Created: at(5)}}, []int{3}, []int{8, 9}},
	} {
		is := forge.Issue{Number: 1, Author: tc.author, Body: tc.body, Created: t0, Comments: tc.comments}
		d := Dependencies(is, tc.plan, tc.plan != "", tc.planAt, "swarm-bot", []string{"jane", "swarm-bot"})
		if !slices.Equal(d.Declared, tc.declared) || !slices.Equal(d.BlockedBy, tc.block) {
			t.Errorf("%s: Dependencies = %+v, want declared %v, blocked by %v", tc.name, d, tc.declared, tc.block)
		}
	}
	// A declaration wakes no role, as a class statement does not.
	if counted(Entry{"jane", "Depends: #3", t0}, "swarm-bot", []string{"jane"}) {
		t.Error("a declaration is counted")
	}
}

// Two slices overlap (section 12.1, rule 2) when a pattern of one matches
// a path the other's branch changes, or a pattern of each may match one
// path: literal prefixes that nest, or either a pattern without `/`.
func TestOverlap(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b Touch
		want bool
	}{
		{"one directory", Touch{Patterns: []string{"src/"}}, Touch{Patterns: []string{"src/"}}, true},
		{"nested directories", Touch{Patterns: []string{"src/"}}, Touch{Patterns: []string{"src/parser/"}}, true},
		{"apart", Touch{Patterns: []string{"src/"}}, Touch{Patterns: []string{"docs/"}}, false},
		{"a glob under a directory", Touch{Patterns: []string{"src/*.go"}}, Touch{Patterns: []string{"src/lexer/"}}, true},
		{"globs apart", Touch{Patterns: []string{"src/*.go"}}, Touch{Patterns: []string{"docs/*.md"}}, false},
		{"globs that nest but cannot meet hold", Touch{Patterns: []string{"src/*.go"}}, Touch{Patterns: []string{"src/*.md"}}, true},
		{"a pattern without a slash", Touch{Patterns: []string{"*.md"}}, Touch{Patterns: []string{"src/"}}, true},
		{"a file name at any depth", Touch{Patterns: []string{"main.go"}}, Touch{Patterns: []string{"src/"}}, true},
		{"a pattern against a branch file", Touch{Patterns: []string{"docs/"}}, Touch{Files: []string{"docs/a.md"}}, true},
		{"a branch file against a pattern", Touch{Files: []string{"go.mod"}}, Touch{Patterns: []string{"go.mod"}}, true},
		{"a pattern apart from the branch files", Touch{Patterns: []string{"docs/"}}, Touch{Files: []string{"src/a.go"}}, false},
		{"no patterns: branch files alone", Touch{Files: []string{"src/a.go"}}, Touch{Files: []string{"src/a.go"}}, false},
		{"no patterns at all", Touch{}, Touch{Patterns: []string{"src/"}}, false},
	} {
		if got := Overlap(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: Overlap = %v, want %v", tc.name, got, tc.want)
		}
		if got := Overlap(tc.b, tc.a); got != tc.want {
			t.Errorf("%s, swapped: Overlap = %v, want %v", tc.name, got, tc.want)
		}
	}
	// A slice's patterns: its item's line, else the proposal's; its item's
	// Safe with: line.
	is := forge.Issue{Author: "swarm-bot", Body: marker("owner", 1) + "Class: routine\nTouches: lib/, *.md\n"}
	plan := "## Steps\n\n1. app: one.\n   Touches: src/a/, src/b/\n   Safe with: #4, #5\n2. app: two.\n"
	if got := SliceTouch(is, plan, true, 1, "swarm-bot"); !slices.Equal(got.Patterns, []string{"src/a/", "src/b/"}) || !slices.Equal(got.Safe, []int{4, 5}) {
		t.Errorf("slice 1: %+v", got)
	}
	if got := SliceTouch(is, plan, true, 2, "swarm-bot"); !slices.Equal(got.Patterns, []string{"lib/", "*.md"}) || got.Safe != nil {
		t.Errorf("slice 2: %+v", got)
	}
	if got := SliceTouch(is, "", false, 1, "swarm-bot"); !slices.Equal(got.Patterns, []string{"lib/", "*.md"}) {
		t.Errorf("no plan: %+v", got)
	}
}

// The Task texts ask for the declarations (section 12.1, rules 1, 2 and
// 5): the owner's batch text for each proposal's `Depends:` and
// `Touches:` lines after its `Class:` line, and for work that waits on
// open issues to be proposed with them; the architect's plan texts, the
// first plan and a revision, for the plan's `Depends:` line and each
// slice's `Touches:` and `Safe with:` lines, a paragraph of its own after
// the probes; the other texts for neither.
func TestTaskTextsAskForDependencies(t *testing.T) {
	f, at := fixture()
	j := job(f, at.N, "owner", "")
	j.N, j.Batch = 0, &Batch{Propose: 2}
	sys, _ := Render(j)
	has(t, "owner batch", taskOf(t, sys), []string{
		`then a line "Class: routine" or "Class: architecture", then a line "Depends: none", or "Depends: #<a>, #<b>" naming the open issues the job waits on, then a line "Touches: <pattern>, <pattern>" naming the paths you expect it to change`,
		`so work that waits on open issues is proposed all the same, its Depends: line naming them.`,
		`If there is nothing worth proposing even so, write "BLOCKED: <why>"`,
	})
	for _, label := range []string{"swarm:proposal-accepted", "swarm:plan-0-posted"} {
		f, at := fixture()
		if label == "swarm:plan-0-posted" {
			f.Comment(at, marker("architect", 1)+archPlan)
		}
		sys, _ := Render(job(f, at.N, "architect", label))
		task := taskOf(t, sys)
		paragraphs(t, "architect at "+label, task, probesSentence, dependsSentence)
	}
	for _, c := range []struct{ role, label string }{{"owner", "swarm:proposal-posted"}, {"worker", "swarm:plan-1-accepted"}, {"adversary", "swarm:review-0-ready"}} {
		sys, _ := Render(memoryJob(t, c.role, c.label))
		has(t, c.role, taskOf(t, sys), nil, "Depends:", "Touches:", "Safe with:")
	}
}
