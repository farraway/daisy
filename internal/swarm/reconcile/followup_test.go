package reconcile

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// TestFollowUpFindings: a follow-up holds each [non-blocking] line of
// every swarm review on the pull request, every round's, the approving
// ones included, each once (section 7.11, Follow-ups): a finding a later
// round repeats, its line number, whitespace and case aside, keeps its
// earliest round; a review Daisy cut on the forge is read whole from its
// run's out.md (decision 149); a blocking line and a review not the
// swarm's are left out.
func TestFollowUpFindings(t *testing.T) {
	f, at := fixture()
	m, _ := f.CreatePull("lib", "swarm/1-1", "main", "#1 slice 1: Drift check", marker("worker", 1)+"Report 1.\n\nRefs acme/app#1")
	pr := forge.Ref{Repo: "lib", N: m}
	f.Push(pr, "c1")
	f.Review(pr, forge.Commented, marker("adversary", 1)+"VERDICT: request-changes\n\n[blocking] a.go:3 fails on an empty input\n[non-blocking] a.go:10: name the constant\n")
	f.Push(pr, "c2")
	second := "VERDICT: approve\n\n[non-blocking]  A.go:12:  Name the  constant \n[non-blocking] b.go:4: a test for the empty input\n"
	f.Review(pr, forge.Commented, marker("adversary", 2)+second+"\n[truncated by Daisy; the whole output is in runs/1-adversary-2]")
	f.Review(pr, forge.Commented, marker("sympathetic", 3)+"VERDICT: approve\n\n[non-blocking] c.go:1: the doc comment is stale\n[non-blocking] b.go:7: a test for the empty input\n")
	f.As(forge.Account{Login: "jane"}).Review(pr, forge.Approve, "[non-blocking] d.go:1: not the swarm's\n")
	reviews, _ := f.Reviews(pr)
	outputs := outputsAt(t, map[string]string{"1-adversary-2": second + filler + "\n[non-blocking] z.go:9: past the cut\n"})
	got, err := followUpFindings(at.N, outputs, reviews, nil, time.Time{}, "swarm-bot")
	want := []string{
		"adversary run 1, round 1: [non-blocking] a.go:10: name the constant",
		"adversary run 2, round 2: [non-blocking] b.go:4: a test for the empty input",
		"adversary run 2, round 2: [non-blocking] z.go:9: past the cut",
		"sympathetic run 3, round 2: [non-blocking] c.go:1: the doc comment is stale",
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("findings, %v:\n%s\nwant:\n%s", err, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
