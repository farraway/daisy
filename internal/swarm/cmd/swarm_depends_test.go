package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// proposal is the owner's proposal body with lines after its Class: line.
func proposal(lines ...string) string {
	return mark("owner", 1) + "Brief: docs/briefs/a.md — M1 The drift\nClass: architecture\n" + strings.Join(lines, "\n") + "\n\nThe job.\n"
}

// A dependent job is held (section 12.1, rule 1): while an issue its
// newest `Depends:` declaration names is open (the proposal's, the
// accepted plan's, or the maintainer's comment that is the line alone),
// nothing launches on it and the merge step merges nothing of it; once
// it is closed, its role runs. A declaration naming no issue holds
// nothing, with a warning; one naming none holds nothing.
func TestSwarmApplyHoldsADependentJob(t *testing.T) {
	t.Parallel()
	t.Run("the proposal's", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.AddLabels("bug")
		a.forge.CreateIssue("The library", "", []string{"bug"})                                   // #1
		a.forge.CreateIssue("Held", proposal("Depends: #1"), []string{"swarm:proposal-accepted"}) // #2
		a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 0 {
			t.Fatalf("a dependent job launched with #1 open:\n%s", a.stdout.String())
		}
		a.forge.CloseIssue(1)
		a.pass()
		if n := len(a.runner.Launches()); n != 1 {
			t.Errorf("#1 closed: %d launches\n%s", n, a.stdout.String())
		}
	})
	t.Run("the plan's", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.Gates.Plan = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
		a.forge.AddLabels("bug")
		a.forge.CreateIssue("The library", "", []string{"bug"})                                     // #1
		a.forge.CreateIssue("Next", proposal("Depends: none"), []string{"swarm:proposal-accepted"}) // #2
		a.runner.Script(2, "architect", runner.Canned{Out: "Depends: #1\n\n" + oneSlice})
		a.pass()
		if got := labels(a.issue(2)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) {
			t.Fatalf("planned: %v\n%s", got, a.stdout.String())
		}
		a.runner.Script(2, "worker", work("v1"))
		a.pass()
		if n := len(a.runner.Launches()); n != 1 {
			t.Fatalf("the worker launched with #1 open: %d launches\n%s", n, a.stdout.String())
		}
		a.forge.CloseIssue(1)
		a.pass()
		if l := a.runner.Launches(); len(l) < 2 || l[1].Label != "acme/swarm-app#2/worker" {
			t.Errorf("#1 closed: launches %+v\n%s", l, a.stdout.String())
		}
	})
	t.Run("the maintainer's overrides", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.AddLabels("bug")
		a.forge.CreateIssue("The library", "", []string{"bug"})                                   // #1
		a.forge.CreateIssue("Held", proposal("Depends: #1"), []string{"swarm:proposal-accepted"}) // #2
		a.forge.Say(a.ref(2), "jane", "Depends: none")
		a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 1 {
			t.Errorf("Depends: none did not release it: %d launches\n%s", n, a.stdout.String())
		}
	})
	t.Run("naming no issue", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.CreateIssue("Held", proposal("Depends: #99"), []string{"swarm:proposal-accepted"}) // #1
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 1 || !strings.Contains(a.stderr.String(), "⚠ #1: Depends: #99 names no issue; not held\n") {
			t.Errorf("%d launches\nstderr:\n%s", n, a.stderr.String())
		}
	})
	t.Run("a cycle", func(t *testing.T) {
		// Two issues that depend on each other, both open: both held,
		// and the pass ends.
		a := newApplyWS(t)
		a.forge.CreateIssue("One", proposal("Depends: #2"), []string{"swarm:proposal-accepted"}) // #1
		a.forge.CreateIssue("Two", proposal("Depends: #1"), []string{"swarm:proposal-accepted"}) // #2
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
		a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		a.pass()
		if n := len(a.runner.Launches()); n != 0 {
			t.Errorf("a cycle launched %d runs\n%s", n, a.stdout.String())
		}
	})
	t.Run("the merge step", func(t *testing.T) {
		a := newApplyWS(t)
		head := a.approvedGateOff()
		a.forge.AddLabels("bug")
		a.forge.CreateIssue("The library", "", []string{"bug"}) // #2
		a.forge.Say(issue1, "jane", "Depends: #2")
		launched := len(a.runner.Launches())
		a.pass()
		if ps := a.pulls(1); ps[0].Merged || !slices.Contains(labels(a.issue(1)), forge.ReviewApproved) || len(a.runner.Launches()) != launched {
			t.Fatalf("merged, or a role woken, with #2 open: %v\n%s", labels(a.issue(1)), a.stdout.String())
		}
		a.forge.CloseIssue(2)
		a.pass()
		if ps := a.pulls(1); !ps[0].Merged || ps[0].HeadSHA != head || a.issue(1).Open {
			t.Errorf("#2 closed, not merged:\n%s", a.stdout.String())
		}
	})
}

// `swarm block <n> --by <m>` run twice holds by both (section 12.1, rule
// 1): nothing launches on #n while either is open, status names both,
// and once both are closed its role runs.
func TestSwarmMaintainerHoldsByTwo(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	m := &maintainerWS{statusWS: a.statusWS}
	m.secrets(bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
	a.forge.AddLabels("bug")
	a.forge.CreateIssue("The library", "", []string{"bug"})                              // #1
	a.forge.CreateIssue("The editor", "", []string{"bug"})                               // #2
	a.forge.CreateIssue("Held", "Check the drift.", []string{"swarm:proposal-accepted"}) // #3
	m.ok(maintainerVerb("block"), "Maintainer jane on acme/swarm-app\n#1 blocking\n#3 commented on the issue\n", "3", "--by", "1")
	m.ok(maintainerVerb("block"), "Maintainer jane on acme/swarm-app\n#2 blocking\n#3 commented on the issue\n", "3", "--by", "2")
	a.runner.Script(3, "architect", runner.Canned{Out: oneSlice})
	a.pass()
	if n := len(a.runner.Launches()); n != 0 {
		t.Fatalf("launched with #1 and #2 open:\n%s", a.stdout.String())
	}
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nHeld: #3 blocked by #1; #3 blocked by #2\n") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
	a.forge.CloseIssue(1)
	a.pass()
	if n := len(a.runner.Launches()); n != 0 {
		t.Fatalf("launched with #2 open:\n%s", a.stdout.String())
	}
	a.forge.CloseIssue(2)
	a.pass()
	if n := len(a.runner.Launches()); n != 1 {
		t.Errorf("both closed: %d launches\n%s", n, a.stdout.String())
	}
}

// What `status` shows of the dependencies (section 12.1): `Held:` names
// each open dependency of an issue, a declaration's and a `Blocked by`'s
// alike, as JSON's held.blocked_by does; a closed one is not named, and
// a held issue is in no role's queue.
func TestSwarmStatusHeldByDependencies(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.AddLabels("bug")
	a.forge.CreateIssue("The library", "", []string{"bug"})                                       // #1
	a.forge.CreateIssue("The upload", "", []string{"bug"})                                        // #2
	a.forge.CreateIssue("Held", proposal("Depends: #1, #2"), []string{"swarm:proposal-accepted"}) // #3
	a.forge.CreateIssue("Held by hand", "Check the drift.", []string{"swarm:proposal-accepted"})  // #4
	a.forge.CreateIssue("Free", proposal("Depends: #2"), []string{"swarm:proposal-accepted"})     // #5
	a.forge.Say(a.ref(4), "jane", "Blocked by #1")
	a.forge.Say(a.ref(4), "jane", "Depends: #2")
	a.forge.CloseIssue(2)
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nHeld: #3 blocked by #1; #4 blocked by #1\n") {
		t.Fatalf("status: %v\n%s", err, a.stdout.String())
	}
	a.flags = []string{"--json"}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Held struct {
			BlockedBy []struct{ Issue, By int } `json:"blocked_by"`
		} `json:"held"`
		Capacity struct {
			Roles []struct {
				Role  string `json:"role"`
				Queue []int  `json:"queue"`
			} `json:"roles"`
		} `json:"capacity"`
	}
	if err := json.Unmarshal(a.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("%v\n%s", err, a.stdout.String())
	}
	if got := fmt.Sprint(doc.Held.BlockedBy); got != "[{3 1} {4 1}]" {
		t.Errorf("held.blocked_by: %s", got)
	}
	for _, r := range doc.Capacity.Roles {
		if r.Role == "architect" && !slices.Equal(r.Queue, []int{5}) {
			t.Errorf("the architect's queue: %v", r.Queue)
		}
	}
}

// The triage and plan gates' notes name the declarations (section 12.1,
// What `status` shows): `- Depends:` each dependency with where it stands,
// and `- Touches:` the patterns, the proposal's at triage and the plan's
// slices' at the plan gate; `none declared` for either with none.
func TestGateNoteNamesDependencies(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.AddLabels("bug")
	a.forge.CreateIssue("The library", "", []string{"bug"})                                                                                           // #1
	a.forge.CreateIssue("The upload", "", []string{"bug"})                                                                                            // #2
	a.forge.CreateIssue("Next", proposal("Depends: #1, #2, #3", "Touches: src/, *.md"), []string{"swarm:proposal-posted"})                            // #3
	a.forge.CreateIssue("Plain", mark("owner", 1)+"Brief: docs/briefs/a.md — M1 The drift\nClass: architecture\n", []string{"swarm:proposal-posted"}) // #4
	a.forge.CreateIssue("Planned", proposal("Depends: #2", "Touches: lib/"), []string{"swarm:plan-0-posted"})                                         // #5
	a.forge.Comment(a.ref(5), mark("architect", 1)+"Depends: #1, #4\n\n## Steps\n\n1. app: one.\n   Touches: src/a/, docs/\n2. app: two.\n   Touches: docs/, src/b/\n3. app: three.\n")
	a.forge.CloseIssue(2)
	a.pass()
	if got := a.gateNotes(3); len(got) != 1 || !strings.Contains(got[0], "- Issues it names, its dependencies among them: #1 (no open swarm issue), #2 (no open swarm issue).\n- Depends: #1 (open), #2 (closed).\n- Touches: src/, *.md.\n\n") {
		t.Errorf("#3's triage note: %q", got)
	}
	if got := a.gateNotes(4); len(got) != 1 || !strings.Contains(got[0], "- Depends: none declared.\n- Touches: none declared.\n\n") {
		t.Errorf("#4's triage note: %q", got)
	}
	if got := a.gateNotes(5); len(got) != 1 || !strings.Contains(got[0], "- Class: architecture.\n- Depends: #1 (open), #4 (open, proposal-posted).\n- Touches: src/a/, docs/, src/b/, lib/.\n- Slices: 3.\n") {
		t.Errorf("#5's plan note: %q", got)
	}
}

// A slice's first build waits while it overlaps an open slice of another
// issue in its repository (section 12.1, rule 2): in one pass the first
// of two overlapping slices launches and the second, seeing it running,
// is held, and stays held while the first's pull request is open, status
// naming it `#<n> overlaps #<m>`; once that pull request is merged, the
// second launches. A `Safe with:` line naming the other's issue, or
// patterns apart, hold nothing.
func TestSwarmWorkerHeldByAnOverlap(t *testing.T) {
	t.Parallel()
	slice := func(touches string, more ...string) string {
		return "## Steps\n\n1. app: change main.go.\n   Touches: " + touches + "\n" + strings.Join(more, "\n") + "\n## Test Plan\n\n1. main.go says v1.\n"
	}
	planned := func(a *applyWS, plans ...string) {
		for i, p := range plans {
			n, _ := a.forge.CreateIssue(fmt.Sprintf("Job %d", i+1), "Do it.", []string{"swarm:proposal-accepted"})
			a.runner.Script(n, "architect", runner.Canned{Out: p})
		}
		a.pass()
		for i := range plans {
			a.forge.SetLabel(a.ref(i+1), "swarm:plan-1-accepted")
			a.runner.Script(i+1, "worker", work(fmt.Sprintf("v%d", i+1)))
		}
	}
	workers := func(a *applyWS) []string {
		var on []string
		for _, l := range a.runner.Launches() {
			if strings.HasSuffix(l.Label, "/worker") {
				on = append(on, l.Label)
			}
		}
		return on
	}
	t.Run("held until the first merges", func(t *testing.T) {
		a := newApplyWS(t)
		planned(a, slice("main.go"), slice("*.go"))
		a.pass()
		if got := workers(a); !reflect.DeepEqual(got, []string{"acme/swarm-app#1/worker"}) {
			t.Fatalf("workers %q\n%s", got, a.stdout.String())
		}
		a.pass()
		if got := workers(a); len(got) != 1 || len(a.pulls(1)) != 1 || !a.pulls(1)[0].Open {
			t.Fatalf("with #1's pull request open: workers %q\n%s", got, a.stdout.String())
		}
		if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nHeld: #2 overlaps #1\n") {
			t.Errorf("status: %v\n%s", err, a.stdout.String())
		}
		a.forge.Merge(forge.Ref{Repo: "app", N: a.pulls(1)[0].Number}, a.pulls(1)[0].HeadSHA)
		a.pass()
		if got := workers(a); !reflect.DeepEqual(got, []string{"acme/swarm-app#1/worker", "acme/swarm-app#2/worker"}) {
			t.Errorf("#1 merged: workers %q\n%s", got, a.stdout.String())
		}
	})
	t.Run("reads nothing with no slot free", func(t *testing.T) {
		// The overlap is read for a launch that can follow: with every
		// member slot taken, the held pass reads #1's issue once less.
		a := newApplyWS(t)
		planned(a, slice("main.go"), slice("*.go"))
		a.pass() // #1's worker builds; #2 is held, its pull request open
		a.passReads()
		free := a.passReads()
		a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
		a.runner.Others = []runner.Running{{Name: "daisy-swarm.acme.swarm-app.9.architect.9", Swarm: "acme/swarm-app#9/architect"}}
		a.publish("full")
		full := a.passReads()
		count := func(reads []string) int { return strings.Count(strings.Join(reads, "\n")+"\n", "Issue 1\n") }
		if count(full) >= count(free) {
			t.Errorf("with no slot free the pass read #1 %d times, with one free %d:\n%q\n%q", count(full), count(free), full, free)
		}
	})
	t.Run("by the open branch's files", func(t *testing.T) {
		// #1 declares nothing: its branch's main.go holds #2's main.go.
		a := newApplyWS(t)
		planned(a, oneSlice, slice("main.go"))
		a.forge.SetLabel(a.ref(2), "swarm:plan-0-posted")
		a.pass()
		a.forge.SetLabel(a.ref(2), "swarm:plan-1-accepted")
		a.pass()
		if got := workers(a); !reflect.DeepEqual(got, []string{"acme/swarm-app#1/worker"}) {
			t.Errorf("workers %q\n%s", got, a.stdout.String())
		}
	})
	for _, tc := range []struct{ name, first, second string }{
		{"safe with", slice("main.go"), slice("main.go", "   Safe with: #1")},
		{"apart", slice("src/"), slice("docs/")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			planned(a, tc.first, tc.second)
			a.pass()
			if got := workers(a); !reflect.DeepEqual(got, []string{"acme/swarm-app#1/worker", "acme/swarm-app#2/worker"}) {
				t.Errorf("workers %q\n%s", got, a.stdout.String())
			}
		})
	}
}

// A job held by an open dependency costs the forge no read of its pull
// request's thread (section 12.1, rule 1): the hold stands before the
// thread is read, and once the dependency is closed the thread is read.
func TestSwarmHeldJobReadsNoPullThread(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice) // #1, review-0-ready, its pull request open
	a.forge.AddLabels("bug")
	a.forge.CreateIssue("The library", "", []string{"bug"}) // #2
	a.forge.Say(issue1, "jane", "Depends: #2")
	a.passReads()
	reads := a.passReads()
	if slices.Contains(reads, "ReviewComments app#1") {
		t.Errorf("a held job's pull request thread was read: %q", reads)
	}
	a.forge.CloseIssue(2)
	if reads = a.passReads(); !slices.Contains(reads, "ReviewComments app#1") {
		t.Errorf("a released job's pull request thread was not read: %q", reads)
	}
}

// An issue the forge does not hold holds nothing, for the owner's batch
// hold as for a dependency (section 6, The hold released by its issues;
// section 12.1, rules 1 and 5; the maintainer's ruling of 2026-11-26 on
// R2-A's second open question): a hold waiting on #1 and on a #9 that
// names no issue is released once #1 is closed, the pass warning that #9
// is not waited on.
func TestSwarmOwnerHoldIgnoresAMissingIssue(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.CreateIssue("Library", "x", nil) // #1
	os.WriteFile(a.state("owner-held"), []byte("held 2026-11-06T14:00:00Z: blocked: waiting on #1, #9: x\n0-owner-1 waits #1 #9\n"), 0o644)
	a.pass()
	if !fileExists(a.state("owner-held")) {
		t.Fatalf("released with #1 open:\n%s", a.stdout.String())
	}
	a.forge.CloseIssue(1)
	a.runner.Script(0, "owner", runner.Canned{Out: "## Next\nA.\n"})
	a.pass()
	if fileExists(a.state("owner-held")) || !strings.Contains(a.stdout.String(), "Owner batch released: #1 #9 closed\n") {
		t.Errorf("not released with #9 naming no issue:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	if want := "⚠ owner batch: #9, which its hold waits on, names no issue; not waited on\n"; !strings.Contains(a.stderr.String(), want) {
		t.Errorf("stderr lacks %q:\n%s", want, a.stderr.String())
	}
}
