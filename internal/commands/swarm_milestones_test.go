package commands

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

func (a *applyWS) milestoneTitles() []string {
	a.t.Helper()
	ms, err := a.forge.Milestones()
	if err != nil {
		a.t.Fatal(err)
	}
	var out []string
	for _, m := range ms {
		out = append(out, m.Title)
	}
	return out
}

// milestoneOf is issue n's milestone's title, "" for none.
func (a *applyWS) milestoneOf(n int) string {
	a.t.Helper()
	id := a.issue(n).Milestone
	if id == 0 {
		return ""
	}
	return a.milestoneTitles()[id-1]
}

// The forge milestones mirror the briefs (docs/swarm.md section 11,
// apply's step 8): two active briefs with three milestones give three
// milestones on the forge, and each proposal of the owner's batch is put
// in the one its Brief: line names, in the pass that ran the batch. The
// swarm issues from before the milestones, open and closed, are put in
// theirs; a done brief's issue, an issue with no Brief: line and a closed
// issue naming a milestone or a brief that is not there are left as they
// are, and an open one naming either is a warning.
// A second pass writes nothing and leaves the milestone the maintainer
// moved an issue to; a paused pass mirrors too.
func TestSwarmApplyMilestones(t *testing.T) {
	a := newApplyWS(t)
	a.write("docs/briefs/a.md", "# A\n\nStatus: active\n\n## M1 First\n\n```\n## not a milestone\n```\n\n## M2 Second\n")
	a.write("docs/briefs/b.md", "# B\n\nStatus: active\n\n## M1 Only\n")
	a.write("docs/briefs/c.md", "# C\n\nStatus: done\n\n## M1 Gone\n")
	f := a.forge
	create := func(label, body string) int {
		n, err := f.CreateIssue("t", body, []string{label})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	create("swarm:blocked", mark("owner", 1)+"Brief: docs/briefs/a.md — M2 Second\nA job.\n")   // 1
	f.CloseIssue(create("swarm:ready", mark("owner", 1)+"Brief: docs/briefs/b.md — M1 Only\n")) // 2
	create("swarm:blocked", mark("owner", 1)+"Brief: docs/briefs/a.md — M1 Frist\n")            // 3: a heading renamed since
	create("swarm:approved", "Brief: docs/briefs/c.md — M1 Gone\n")                             // 4: the maintainer's, a done brief
	create("swarm:blocked", "A job of the maintainer's.\n")                                     // 5
	create("swarm:blocked", "Brief: docs/brief/a.md — M1 First\n")                              // 6: no such brief
	f.CloseIssue(create("swarm:approved", mark("owner", 1)+"Brief: docs/briefs/b.md — Gone\n")) // 7
	f.CloseIssue(create("swarm:approved", "Brief: docs/brief/b.md — M1 Only\n"))                // 8

	a.release()
	a.runner.Script(0, "owner", runner.Canned{Stdout: claudeResult, Out: "## One\nBrief: docs/briefs/a.md — M1 First\nDo one.\n\n" +
		"## Two\nBrief: docs/briefs/b.md — M1 Only\nDo two.\n\n## Three\nBrief: docs/briefs/a.md — M2 Second\nDo three.\n"})
	a.pass()
	if want := []string{"a: M1 First", "a: M2 Second", "b: M1 Only"}; !reflect.DeepEqual(a.milestoneTitles(), want) {
		t.Fatalf("milestones %q, want %q\nstdout:\n%s\nstderr:\n%s", a.milestoneTitles(), want, a.stdout.String(), a.stderr.String())
	}
	for n, want := range map[int]string{1: "a: M2 Second", 2: "b: M1 Only", 3: "", 4: "", 5: "", 6: "", 7: "", 8: "", 9: "a: M1 First", 10: "b: M1 Only", 11: "a: M2 Second"} {
		if got := a.milestoneOf(n); got != want {
			t.Errorf("#%d in %q, want %q", n, got, want)
		}
	}
	for _, line := range []string{"owner run 1: proposed #11 Three\n", "milestone created: a: M1 First\nmilestone created: a: M2 Second\nmilestone created: b: M1 Only\n",
		"#1 milestone: a: M2 Second\n#9 milestone: a: M1 First\n#10 milestone: b: M1 Only\n#11 milestone: a: M2 Second\n#2 milestone: b: M1 Only\nPass "} {
		if !strings.Contains(a.stdout.String(), line) {
			t.Errorf("no %q in stdout:\n%s", line, a.stdout.String())
		}
	}
	warnings := "⚠ #3: its Brief: line names \"M1 Frist\", which is no milestone of docs/briefs/a.md; no milestone set\n" +
		"⚠ #6: its Brief: line names docs/brief/a.md, which is no brief under resources; no milestone set\n"
	if a.stderr.String() != warnings {
		t.Errorf("stderr:\n%s\nwant:\n%s", a.stderr.String(), warnings)
	}
	ms, _ := f.Milestones()
	if want := []forge.Milestone{{ID: 1, Title: "a: M1 First", Open: 1}, {ID: 2, Title: "a: M2 Second", Open: 2}, {ID: 3, Title: "b: M1 Only", Open: 1, Closed: 1}}; !reflect.DeepEqual(ms, want) {
		t.Errorf("counts %+v", ms)
	}

	hand, _ := f.EnsureMilestone("release 1") // the maintainer moves #9
	f.SetMilestone(9, hand)
	a.pass()
	if strings.Contains(a.stdout.String(), "milestone") || a.stderr.String() != warnings || a.milestoneOf(9) != "release 1" || len(a.milestoneTitles()) != 4 {
		t.Errorf("the second pass:\n%s\n%s\n#9 in %q", a.stdout.String(), a.stderr.String(), a.milestoneOf(9))
	}

	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T16:00:00Z\n"), 0o644)
	create("swarm:blocked", "Brief: docs/briefs/b.md — M1 Only\n") // 12
	a.pass()
	if !strings.Contains(a.stdout.String(), "#12 milestone: b: M1 Only\n") || a.milestoneOf(12) != "b: M1 Only" {
		t.Errorf("paused:\n%s", a.stdout.String())
	}
}

// A forge that refuses the milestones (an account without write access to
// the issues) is a warning each pass, never a fault or an exit code: the
// pass goes on and nothing is paused.
func TestSwarmApplyMilestonesRefused(t *testing.T) {
	a := newApplyWS(t)
	a.write("docs/briefs/a.md", "Status: active\n\n## M1\n\n## M2\n")
	f := a.forge
	f.CreateIssue("t", "Brief: docs/briefs/a.md — M1\n", []string{"swarm:blocked"})
	f.EnsureMilestone("a: M1")
	f.RefuseMilestones()
	a.pass()
	warnings := "⚠ milestone a: M2: forge: POST /repos/acme/app/milestones: 403 forbidden\n" +
		"⚠ #1: milestone a: M1: forge: PATCH /repos/acme/app/issues/1: 403 forbidden\n"
	if a.stderr.String() != warnings || a.issue(1).Milestone != 0 || !reflect.DeepEqual(a.milestoneTitles(), []string{"a: M1"}) {
		t.Errorf("stderr:\n%s\nwant:\n%s", a.stderr.String(), warnings)
	}
	if !strings.HasSuffix(a.stdout.String(), "Pass 15:01:00: launched 0, collected 0, running 0\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
	if _, err := os.Stat(a.state("paused")); err == nil {
		t.Error("a refused milestone paused the swarm")
	}
}
