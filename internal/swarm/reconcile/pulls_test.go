package reconcile

import (
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// block is the pulls block listing items, as the request's example has it.
func block(items string) string {
	return "<!-- daisy-swarm kind=pulls -->\n**Pull requests:** " + items + "\n<!-- /daisy-swarm kind=pulls -->"
}

// The block goes right after the marker lines of a body whose first line
// is the marker, so the marker stays first, and first in any other; it
// is replaced in place, the rest of the body as it was; without the
// markers whole, a fresh one goes at the top and nothing else changes;
// and every reader takes it out with the empty line after it, giving the
// body back as written (section 5, The pull requests on the issue).
func TestWithPulls(t *testing.T) {
	one, two := block("slice 1 x merged"), block("slice 1 x merged · slice 2 y open, review round 1")
	owner := marker("owner", 1) + "Brief: docs/briefs/a.md — M1\nClass: routine\nDo it."
	for _, tc := range []struct{ name, body, block, want string }{
		{"the owner's", owner, one, marker("owner", 1) + one + "\n\nBrief: docs/briefs/a.md — M1\nClass: routine\nDo it."},
		{"the maintainer's", "Fix it.\r\nNow.", one, one + "\n\nFix it.\r\nNow."},
		{"an empty body", "", one, one + "\n\n"},
		{"replaced in place", marker("owner", 1) + one + "\n\nDo it.", two, marker("owner", 1) + two + "\n\nDo it."},
		{"moved by hand, replaced where it is", "Fix it.\n\n" + one + "\nNow.", two, "Fix it.\n\n" + two + "\nNow."},
		{"CRLF markers", "<!-- daisy-swarm kind=pulls -->\r\n**Pull requests:** old\r\n<!-- /daisy-swarm kind=pulls -->\r\n\r\nFix it.", one, one + "\n\r\nFix it."}, // the block's lines replaced whole
		{"no closing marker", "<!-- daisy-swarm kind=pulls -->\nFix it.", one, one + "\n\n<!-- daisy-swarm kind=pulls -->\nFix it."},
		{"a closing marker alone", marker("owner", 1) + "<!-- /daisy-swarm kind=pulls -->\nDo it.", one, marker("owner", 1) + one + "\n\n<!-- /daisy-swarm kind=pulls -->\nDo it."},
		{"none to list, none there", owner, "", owner},
		{"none to list, removed", marker("owner", 1) + one + "\n\nDo it.", "", marker("owner", 1) + "Do it."},
	} {
		got := withPulls(tc.body, tc.block)
		if got != tc.want {
			t.Errorf("%s:\n%q\nwant\n%q", tc.name, got, tc.want)
		}
		if strings.Contains(tc.name, "marker") || strings.Contains(tc.name, "moved") || strings.Contains(tc.name, "CRLF") || strings.Contains(tc.name, "in place") {
			continue // the body held a block, or the rests of one, already
		}
		if back := withoutPulls(got); back != tc.body {
			t.Errorf("%s: without the block %q, want %q", tc.name, back, tc.body)
		}
	}
}

// A body carrying the block still reads as it did (sections 5 and 6):
// its marker, so its entry is the swarm's owner's and the owner at
// proposal-posted has nothing to answer, its Brief: and Class: lines, and
// the issue body a member is given, which leaves the block out.
func TestPullsBlockKeepsTheReaders(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	text := "Brief: docs/briefs/a.md — M1 The drift\nClass: routine\nCheck the drift."
	listed := withPulls(marker("owner", 1)+text, block("slice 1 [acme/app#1](https://forge.example.net/acme/app/pulls/1) merged"))
	is := forge.Issue{Number: 1, Title: "Drift check", Author: "swarm-bot", Body: listed, Created: t0, Labels: []string{forge.ProposalPosted}}
	trusted := []string{"jane", "swarm-bot"}
	if m, ok := Swarms(IssueThread(is)[0], "swarm-bot"); !ok || m.Role != "owner" || m.Run != 1 {
		t.Errorf("the marker: %+v %v", m, ok)
	}
	if roles := Launch(Turn{Label: forge.ProposalPosted, Thread: IssueThread(is)}, "swarm-bot", trusted); len(roles) != 0 {
		t.Errorf("the owner launched on its own proposal: %v", roles)
	}
	if path, milestone := BriefOf(is, "swarm-bot"); path != "docs/briefs/a.md" || milestone != "M1 The drift" {
		t.Errorf("Brief: %q %q", path, milestone)
	}
	if c := ClassOf(is, "swarm-bot", trusted); c != Routine {
		t.Errorf("Class: %s", c)
	}
	j := &Job{Cfg: cfg(), SwarmLogin: "swarm-bot", N: 1, Issue: is}
	if got := j.issue(); !strings.Contains(got, "swarm:proposal-posted.\n\n"+text) || strings.Contains(got, "Pull requests") {
		t.Errorf("the member's issue body:\n%s", got)
	}
	jane := forge.Issue{Number: 2, Author: "jane", Body: withPulls("Brief: docs/briefs/a.md — M1 The drift\nFix it.", block("x")), Created: t0}
	if path, _ := BriefOf(jane, "swarm-bot"); path != "docs/briefs/a.md" {
		t.Errorf("a maintainer's Brief: %q", path)
	}
}

// A pull request's state in the block: merged, closed (unmerged), or
// open, approved when both newest reviews approve its head, else in the
// round one more than those both reviewers have completed (section 6).
func TestPullState(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	review := func(role, verdict, commit string, h int) forge.Review {
		return forge.Review{Author: "swarm-bot", Body: marker(role, h) + "VERDICT: " + verdict + "\n", Commit: commit, Submitted: t0.Add(time.Duration(h) * time.Hour)}
	}
	open := forge.Pull{Open: true, HeadSHA: "c2"}
	for _, tc := range []struct {
		name    string
		p       forge.Pull
		reviews []forge.Review
		want    string
	}{
		{"merged", forge.Pull{Merged: true}, nil, "merged"},
		{"closed", forge.Pull{}, nil, "closed"},
		{"no review yet", open, nil, "open, review round 1"},
		{"one review of the first round", open, []forge.Review{review("adversary", "request-changes", "c1", 1)}, "open, review round 1"},
		{"a round done", open, []forge.Review{review("adversary", "request-changes", "c1", 1), review("sympathetic", "approve", "c1", 2)}, "open, review round 2"},
		{"approved", open, []forge.Review{review("adversary", "request-changes", "c1", 1), review("sympathetic", "approve", "c1", 2),
			review("adversary", "approve", "c2", 3), review("sympathetic", "approve", "c2", 4)}, "open, approved"},
		{"approved, then pushed", forge.Pull{Open: true, HeadSHA: "c3"}, []forge.Review{review("adversary", "approve", "c2", 3), review("sympathetic", "approve", "c2", 4)}, "open, review round 2"},
	} {
		if got := pullState(tc.p, tc.reviews, nil, "swarm-bot"); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A direction posted before the pull request was linked in it is still
// a direction on its head (SentBack), so a head sent back then, and
// approved again since, is not sent back a second time.
func TestSentBackReadsTheUnlinkedDirection(t *testing.T) {
	p := forge.Pull{Repo: "app", Number: 1, URL: "https://forge.example.net/acme/app/pulls/1", Head: "swarm/7-1"}
	old := marker("reconciler", 1) + "Pull request acme/app#1 was approved at c1c1c1c, but the forge reports it not mergeable."
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"unlinked", old, true},
		{"linked", marker("reconciler", 1) + Direction("acme", "main", 7, p, "c1c1c1c9", nil), true},
		{"another head", strings.Replace(old, "c1c1c1c", "d2d2d2d", 1), false},
	} {
		comments := []forge.Comment{{Author: "swarm-bot", Body: tc.body}, {Author: "swarm-bot", Body: "the last word"}}
		if got := SentBack(comments, "acme", p, "c1c1c1c9", "swarm-bot", nil); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
