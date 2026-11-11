package brief

import (
	"reflect"
	"testing"
)

// Read takes a file as a brief when its header, the lines before its
// first `## ` heading, holds a Status: line, its value trimmed; a
// Status: line after the first milestone makes no brief (section 4).
func TestRead(t *testing.T) {
	for _, c := range []struct {
		text, status string
		ok           bool
	}{
		{"# Roadmap\nStatus: active\nPriority: first.\n\n## M0\n", "active", true},
		{"# Second\n\nStatus: done\n\n## M0\n", "done", true},
		{"Status:   active  \n", "active", true},
		{"# Late\n\n## M0\n\nStatus: active\n", "", false},
		{"# Notes\n", "", false},
	} {
		b, ok := Read("briefs/b.md", "swarm-app/briefs/b.md", []byte(c.text))
		if ok != c.ok || (ok && b != (Brief{"briefs/b.md", c.status, c.text, "swarm-app/briefs/b.md"})) {
			t.Errorf("Read(%q) = %+v, %v", c.text, b, ok)
		}
	}
}

func TestMilestones(t *testing.T) {
	b := Brief{Text: "# M0\n\nStatus: active\n\n## First one \r\n\ntext\n### not one\n##not one\n## Second\n"}
	if got := b.Milestones(); !reflect.DeepEqual(got, []string{"First one", "Second"}) {
		t.Errorf("Milestones = %q", got)
	}
	if got := (Brief{Text: "Status: active\n"}).Milestones(); got != nil {
		t.Errorf("no milestones: %q", got)
	}
}

// A `## ` line in a fenced block is no milestone, nor is an empty heading
// or a second heading of one text; the forge milestone's title is the
// brief's file name and the milestone.
func TestMilestonesAndTitles(t *testing.T) {
	b := Brief{Path: "docs/briefs/m0-libraries.md", Text: "Status: active\n\n## First\n\n```md\n## not one\n```\n\n~~~~\n```\n## not one either\n~~~~\n\n## \n\n## Second\n\n## First\n"}
	if got := b.Milestones(); !reflect.DeepEqual(got, []string{"First", "Second"}) {
		t.Errorf("Milestones = %q", got)
	}
	// A fence line with an info string closes nothing (CommonMark).
	if got := (Brief{Text: "Status: active\n\n```\n```md\n## not one\n```\n## Third\n"}).Milestones(); !reflect.DeepEqual(got, []string{"Third"}) {
		t.Errorf("an info string closed the fence: %q", got)
	}
	if got := b.MilestoneTitle("First"); got != "m0-libraries: First" {
		t.Errorf("MilestoneTitle = %q", got)
	}
}

// The header ends at the first `## ` heading outside a fence: an example
// in a fenced block, a `## ` line or a `Status:` line in it, neither ends
// the header nor is its Status (section 4).
func TestStatusAfterAFencedHeading(t *testing.T) {
	text := "# Brief\n\nProposals look like:\n\n```markdown\n## <title>\nStatus: done\n```\n\nStatus: active\n\n## M0\n"
	got, ok := Read("briefs/b.md", "swarm-app/briefs/b.md", []byte(text))
	if !ok || got.Status != "active" {
		t.Fatalf("Read = %+v, %v", got, ok)
	}
	if ms := got.Milestones(); !reflect.DeepEqual(ms, []string{"M0"}) {
		t.Errorf("Milestones = %q", ms)
	}
}

// One rule says what a `## ` heading is, for where the header ends and for
// the milestones: at the left margin. An indented one is neither.
func TestIndentedHeadingIsNeither(t *testing.T) {
	text := "# Brief\n\n  ## Context\n\n## \n\nStatus: active\n\n  ## M0\n\n## M1\n"
	if v, ok := statusOf(text); !ok || v != "active" {
		t.Errorf("statusOf = %q, %v", v, ok)
	}
	if ms := (Brief{Text: text}).Milestones(); !reflect.DeepEqual(ms, []string{"M1"}) {
		t.Errorf("Milestones = %q", ms)
	}
}
