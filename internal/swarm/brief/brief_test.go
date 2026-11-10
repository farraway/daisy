package brief

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"daisy/internal/ops"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFind(t *testing.T) {
	ws := t.TempDir()
	write(t, filepath.Join(ws, "docs/briefs/b-second.md"), "# Second\n\nStatus: done\n\n## M0\n")
	write(t, filepath.Join(ws, "docs/briefs/a-roadmap.md"), "# Roadmap\nStatus: active\nPriority: first.\n\n## M0\n")
	write(t, filepath.Join(ws, "docs/briefs/c-late.md"), "# Late\n\n## M0\n\nStatus: active\n") // after the first milestone: no header line
	write(t, filepath.Join(ws, "docs/notes.md"), "# Notes\n")
	write(t, filepath.Join(ws, "docs/briefs/d.txt"), "Status: active\n")
	write(t, filepath.Join(ws, "lib/docs/briefs/x.md"), "Status:   active  \n")
	write(t, filepath.Join(ws, "outside/y.md"), "Status: active\n")
	os.Symlink(filepath.Join(ws, "outside"), filepath.Join(ws, "docs/briefs/linked"))
	os.Symlink(filepath.Join(ws, "outside/y.md"), filepath.Join(ws, "docs/briefs/z.md"))

	got, err := Find(ops.Real().FS(), ws, []string{"docs/", "lib/docs", "docs/briefs/"})
	want := []Brief{{"docs/briefs/a-roadmap.md", "active", "# Roadmap\nStatus: active\nPriority: first.\n\n## M0\n", "docs/briefs/a-roadmap.md"}, {"docs/briefs/b-second.md", "done", "# Second\n\nStatus: done\n\n## M0\n", "docs/briefs/b-second.md"}, {"lib/docs/briefs/x.md", "active", "Status:   active  \n", "lib/docs/briefs/x.md"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Find = %+v, %v", got, err)
	}
	if got, err := Find(ops.Real().FS(), ws, nil); err != nil || got != nil {
		t.Errorf("no resources: %+v, %v", got, err)
	}
	if _, err := Find(ops.Real().FS(), ws, []string{"gone/"}); err == nil {
		t.Error("a missing resource is no error")
	}
	// A resource that is a brief itself.
	if got, _ := Find(ops.Real().FS(), ws, []string{"outside/y.md"}); !reflect.DeepEqual(got, []Brief{{"outside/y.md", "active", "Status: active\n", "outside/y.md"}}) {
		t.Errorf("a file resource: %+v", got)
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
	ws := t.TempDir()
	text := "# Brief\n\nProposals look like:\n\n```markdown\n## <title>\nStatus: done\n```\n\nStatus: active\n\n## M0\n"
	write(t, filepath.Join(ws, "docs/briefs/b.md"), text)
	got, err := Find(ops.Real().FS(), ws, []string{"docs/"})
	if err != nil || !reflect.DeepEqual(got, []Brief{{"docs/briefs/b.md", "active", text, "docs/briefs/b.md"}}) {
		t.Fatalf("Find = %+v, %v", got, err)
	}
	if ms := got[0].Milestones(); !reflect.DeepEqual(ms, []string{"M0"}) {
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
