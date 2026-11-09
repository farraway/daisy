package reconcile

import (
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// A proposal's class is the line right after its opening Brief: line, and
// nothing else (section 7.5).
func TestProposalClass(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"Brief: docs/briefs/a.md — M1\nClass: routine\nDo it.", Routine},
		{"\nBrief: docs/briefs/a.md — M1\r\nClass: architecture  \nDo it.", Architecture},
		{"Brief: docs/briefs/a.md — M1\nDo it.\nClass: routine", ""},
		{"Class: routine\nBrief: docs/briefs/a.md — M1", ""},
		{"Do it.\nClass: routine", ""},
		{"Brief: docs/briefs/a.md — M1\nClass: Routine", ""},
		{"Brief: docs/briefs/a.md — M1\nClass: routine, small", ""},
		{"Brief: docs/briefs/a.md — M1", ""},
	} {
		if got := ProposalClass(tc.text); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.text, got, tc.want)
		}
	}
}

// An issue's class (section 5): the flag says architecture whatever the
// statements; without it, the newest statement of the owner's proposal
// line and the maintainers' comments decides, the swarm's and an
// untrusted login's comments being none, and an issue with none is
// architecture.
func TestClassOf(t *testing.T) {
	t0 := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	routine := marker("owner", 1) + "Brief: docs/briefs/a.md — M1\nClass: routine\nDo it."
	for _, tc := range []struct {
		name, author, body string
		labels             []string
		comments           []forge.Comment
		want               string
	}{
		{"the owner's routine", "swarm-bot", routine, nil, nil, Routine},
		{"the owner's routine, flagged", "swarm-bot", routine, []string{forge.ClassArchitecture}, nil, Architecture},
		{"the owner's unclassed", "swarm-bot", marker("owner", 1) + "Brief: docs/briefs/a.md — M1\nDo it.", nil, nil, Architecture},
		{"the line in a maintainer's body", "jane", "Brief: docs/briefs/a.md — M1\nClass: routine\n", nil, nil, Architecture},
		{"the line in an architect's entry", "swarm-bot", marker("architect", 1) + "Brief: docs/briefs/a.md — M1\nClass: routine\n", nil, nil, Architecture},
		{"a maintainer's routine", "jane", "Fix it.", nil, []forge.Comment{{Author: "Jane", Body: "Class: routine"}}, Routine},
		{"the newest statement", "swarm-bot", routine, nil, []forge.Comment{{Author: "jane", Body: "Class: architecture"}}, Architecture},
		{"and back", "swarm-bot", routine, nil, []forge.Comment{{Author: "jane", Body: "Class: architecture"}, {Author: "jane", Body: "Class: routine\n"}}, Routine},
		{"the swarm's and an untrusted login's", "jane", "Fix it.", nil, []forge.Comment{{Author: "swarm-bot", Body: "Class: routine"}, {Author: "mallory", Body: "Class: routine"}}, Architecture},
		{"a mention", "jane", "Fix it.", nil, []forge.Comment{{Author: "jane", Body: "Class: routine? Not yet."}}, Architecture},
	} {
		is := forge.Issue{Author: tc.author, Body: tc.body, Created: t0, Labels: append([]string{forge.ProposalPosted}, tc.labels...)}
		for i, c := range tc.comments {
			c.Created = t0.Add(time.Duration(i+1) * time.Hour)
			is.Comments = append(is.Comments, c)
		}
		if got := ClassOf(is, "swarm-bot", []string{"jane", "swarm-bot"}); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

// The auto passes status counts: the swarm's auto notes by the stage each
// names; a gate note, or an auto note by another login, is none.
func TestAutoPasses(t *testing.T) {
	note := func(stage string) string {
		return autoMarker + "\n**reconciler:** auto at " + stage + ": a routine job passes a gate by itself."
	}
	is := forge.Issue{Comments: []forge.Comment{
		{Author: "swarm-bot", Body: note("proposal-posted")},
		{Author: "swarm-bot", Body: note("plan-0-posted")},
		{Author: "swarm-bot", Body: note("plan-0-posted")},
		{Author: "jane", Body: note("proposal-posted")},
		{Author: "swarm-bot", Body: gateMarker + "\n**reconciler:** gated at proposal-posted: why."},
	}}
	if triage, plan := AutoPasses(is, "swarm-bot"); triage != 1 || plan != 2 {
		t.Errorf("AutoPasses = %d, %d", triage, plan)
	}
}
