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
	want := []Brief{{"docs/briefs/a-roadmap.md", "active", "# Roadmap\nStatus: active\nPriority: first.\n\n## M0\n"}, {"docs/briefs/b-second.md", "done", "# Second\n\nStatus: done\n\n## M0\n"}, {"lib/docs/briefs/x.md", "active", "Status:   active  \n"}}
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
	if got, _ := Find(ops.Real().FS(), ws, []string{"outside/y.md"}); !reflect.DeepEqual(got, []Brief{{"outside/y.md", "active", "Status: active\n"}}) {
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
	if got := b.MilestoneTitle("First"); got != "m0-libraries: First" {
		t.Errorf("MilestoneTitle = %q", got)
	}
}
