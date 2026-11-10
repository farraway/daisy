package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// refuseFlag is the forge refusing every removal of one flag.
type refuseFlag struct {
	*forge.Fake
	flag string
}

func (f *refuseFlag) Relabel(at forge.Ref, remove, add string) error {
	if remove == f.flag && add == "" {
		return errors.New("forge: DELETE /repos/acme/app/issues/1/labels/7: 502 Bad Gateway")
	}
	return f.Fake.Relabel(at, remove, add)
}

// refuseOnce is the forge refusing the next Relabel that removes remove,
// and the next CloseIssue when close is set.
type refuseOnce struct {
	*forge.Fake
	remove string
	close  bool
}

func (f *refuseOnce) Relabel(at forge.Ref, remove, add string) error {
	if f.remove != "" && remove == f.remove {
		f.remove = ""
		return errors.New("forge: POST /repos/acme/app/issues/1/labels: 502 Bad Gateway")
	}
	return f.Fake.Relabel(at, remove, add)
}

func (f *refuseOnce) CloseIssue(n int) error {
	if f.close {
		f.close = false
		return errors.New("forge: PATCH /repos/acme/app/issues/1: 502 Bad Gateway")
	}
	return f.Fake.CloseIssue(n)
}

// status writes its report whole, or nothing: a run's result that cannot
// be read, read last, leaves stdout empty beside the error.
func TestSwarmStatusWritesNothingOnAFailure(t *testing.T) {
	s := newStatusWS(t)
	if err := os.MkdirAll(s.state("runs/1-worker-1/result"), 0o755); err != nil { // a directory: ReadFile fails
		t.Fatal(err)
	}
	if err := s.status(s.forge); err == nil || !strings.HasPrefix(err.Error(), "Error: swarm: reading the runs: ") {
		t.Fatalf("err %v", err)
	}
	if s.stdout.Len() != 0 {
		t.Errorf("a partial report:\n%s", s.stdout.String())
	}
}

// An approved issue whose current pull request was closed unmerged waits
// under `pull closed`, which neither `swarm merge` nor the merge step
// merges, not under `merge`; the class is named under every item but
// triage and plan, where only an architecture job waits.
func TestSwarmStatusPullClosedUnmerged(t *testing.T) {
	s := newStatusWS(t)
	f := s.forge
	n, _ := f.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:review-2-approved"})
	m, _ := f.CreatePull("app", fmt.Sprintf("swarm/%d-1", n), "main", "slice", "Refs #1")
	f.CloseIssue(m)
	f.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:proposal-posted"}) // #3
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.stdout.String(), "\nWaiting for you: triage #3; pull closed #1 (architecture)\n") {
		t.Errorf("stdout:\n%s", s.stdout.String())
	}
}

// Two active briefs of one file name: apply mirrors the second to no
// milestone, and status says so on its line, with none of the first's
// counts.
func TestSwarmStatusBriefsShareAName(t *testing.T) {
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) { c.Resources = []string{"docs/"} })
	s.write("docs/a/plan.md", "Status: active\n\n## M1\n")
	s.write("docs/b/plan.md", "Status: active\n\n## M1\n")
	n, _ := s.forge.CreateIssue("A", "", []string{"swarm:proposal-accepted"})
	inMilestone(t, s.forge, "plan: M1", n)
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	want := "Briefs:\n  docs/a/plan.md: M1 (1 open, 0 closed)\n  docs/b/plan.md: shares the name plan with docs/a/plan.md; mirrored to no milestone\n"
	if !strings.HasSuffix(s.stdout.String(), want) {
		t.Errorf("stdout:\n%s\nwant it to end:\n%s", s.stdout.String(), want)
	}
}

// A reconciler note on an issue the maintainer moved during the run, which
// T17 leaves as it is, says it blocked nothing, and status counts no
// block for it.
func TestSwarmApplyNoteOnAMovedIssueBlocksNothing(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which corpus?\n"})
	if err := a.apply(&moveOnComment{Fake: a.forge, role: "architect", to: "swarm:plan-1-accepted"}, "--once"); err != nil {
		t.Fatal(err)
	}
	bodies := a.bodies(1)
	note := bodies[len(bodies)-1]
	if want := "architect run 1 is blocked (at swarm:proposal-accepted). The issue was moved to swarm:plan-1-accepted during the run, so it is not blocked."; !strings.HasSuffix(note, want) {
		t.Errorf("note:\n%s\nwant it to end:\n%s", note, want)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) {
		t.Errorf("labels %v", got)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(a.stdout.String(), "\n  #1 ") {
		t.Errorf("a block counted:\n%s", a.stdout.String())
	}
}

// moveOnComment is the maintainer moving issue 1 to to as the collection
// posts role's comment.
type moveOnComment struct {
	*forge.Fake
	role, to string
}

func (m *moveOnComment) Comment(at forge.Ref, body string) error {
	if strings.Contains(body, "role="+m.role+" ") {
		m.Fake.SetLabel(issue1, m.to)
	}
	return m.Fake.Comment(at, body)
}

// .env.sh is read for the names asked alone: every other assignment, the
// maintainer's token among them, is dropped with the parse; a name it does
// not assign is the environment's.
func TestReadSecretsKeepsTheNamesAsked(t *testing.T) {
	for _, clone := range []bool{false, true} {
		s := newStatusWSIn(t, clone)
		s.secrets(bothTokens + "export OTHER=\"$GITEA_TOKEN-x\"\n")
		o := &ops.Fake{GetenvFn: func(k string) string { return map[string]string{"FROM_ENV": "e"}[k] }}
		c := &Context{Ops: o, Env: env.Env{Local: s.local}, Swarm: s.place}
		var warn bytes.Buffer
		got, err := readSecrets(c, s.config(), &warn, "OTHER", "FROM_ENV")
		if want := map[string]string{"OTHER": "tok-123-x", "FROM_ENV": "e"}; err != nil || !reflect.DeepEqual(got, want) || warn.Len() != 0 {
			t.Errorf("clone %v: got %q (%v), want %q; warn %q", clone, got, err, want, warn.String())
		}
	}
}

// A blank comment is none: `unblock` with one is usage, writing nothing;
// `close` with one posts nothing before the close.
func TestSwarmMaintainerBlankComment(t *testing.T) {
	bothLayouts(t, func(t *testing.T, clone bool) {
		m := newMaintainerWSIn(t, clone)
		m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:plan-1-accepted", forge.Blocked})
		var u UsageError
		if err := m.verb(swarmUnblock, "1", " ", "\t"); !errors.As(err, &u) || len(m.writes(1)) != 0 {
			t.Errorf("unblock: %v; writes %q", err, m.writes(1))
		}
		m.ok(swarmClose, "Maintainer jane on acme/"+m.iss()+"\n#1 closed\n#1 swarm:plan-1-accepted → swarm:rejected\n#1 blocked cleared\n", "1", "  ")
		if is, _ := m.forge.Issue(1); len(is.Comments) != 0 {
			t.Errorf("close posted %q", is.Comments)
		}
	})
}

// A verb run again after a write that followed its comment was refused
// posts the comment once: `unblock` whose flag removal, and `close` whose
// close, failed; `merge` sending a pull request back whose relabel failed.
func TestSwarmMaintainerRunAgainPostsOnce(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:plan-1-accepted", forge.Blocked}) // #1
	m.wrap = func(f *forge.Fake) forge.Adapter { return &refuseOnce{Fake: f, remove: forge.Blocked} }
	if err := m.verb(swarmUnblock, "1", "Carry", "on."); err == nil {
		t.Fatal("the refused flag removal: no error")
	}
	m.wrap = nil
	m.ok(swarmUnblock, "Maintainer jane on acme/app\n#1 comment already posted\n#1 blocked cleared\n", "1", "Carry", "on.")
	if got := m.writes(1); !reflect.DeepEqual(got, []string{"jane unlabeled swarm:blocked #1", `jane comment "Carry on." #1`}) {
		t.Errorf("unblock writes %q", got)
	}

	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:plan-1-accepted"}) // #2
	m.wrap = func(f *forge.Fake) forge.Adapter { return &refuseOnce{Fake: f, close: true} }
	if err := m.verb(swarmClose, "2", "Not", "now."); err == nil {
		t.Fatal("the refused close: no error")
	}
	m.wrap = nil
	m.ok(swarmClose, "Maintainer jane on acme/app\n#2 comment already posted\n#2 closed\n#2 swarm:plan-1-accepted → swarm:rejected\n", "2", "Not", "now.")
	if is, _ := m.forge.Issue(2); len(is.Comments) != 1 {
		t.Errorf("close posted %d comments", len(is.Comments))
	}
}

// T25 by `swarm merge` run again after its relabel was refused finds the
// direction posted on the pull request and goes on to the relabel.
func TestSwarmMaintainerMergeBackRunAgain(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	pr := m.approvedPull(1, "c1abcdef99")
	m.forge.SetMergeable(pr, forge.MergeableNo)
	m.wrap = func(f *forge.Fake) forge.Adapter { return &refuseOnce{Fake: f, remove: forge.ReviewApproved} }
	if err := m.verb(swarmMerge, "1"); err == nil {
		t.Fatal("the refused relabel: no error")
	}
	m.wrap = nil
	m.ok(swarmMerge, "Maintainer jane on acme/app\n#1 not mergeable: back to the worker\n#1 comment already posted on pull app#2\n#1 swarm:review-2-approved → swarm:review-3-changes-requested\n", "1")
	comments, _ := m.forge.PullComments(pr)
	if len(comments) != 1 {
		t.Errorf("%d directions", len(comments))
	}
}

// The verbs keep swarm:gated where the new labels are gated, as the pass
// sets it (section 5), and clear it elsewhere: `unblock` at the triage
// gate keeps it, at plan-1-accepted clears it; `triage` of an issue still
// blocked keeps it.
func TestSwarmMaintainerKeepsTheFlagWhereGated(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted", forge.Blocked, forge.Gated}) // #1
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:plan-1-accepted", forge.Blocked, forge.Gated}) // #2
	m.forge.CreateIssue("C", mark("owner", 1)+"c", []string{"swarm:proposal-posted", forge.Blocked, forge.Gated}) // #3
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmUnblock, head+"#1 commented\n#1 blocked cleared\n", "1", "go")
	m.ok(swarmUnblock, head+"#2 commented\n#2 blocked cleared\n#2 gated cleared\n", "2", "go")
	m.ok(swarmTriage, head+"#3 swarm:proposal-posted → swarm:proposal-accepted\n", "3")
	for n, want := range map[int][]string{
		1: {"swarm:proposal-posted", forge.Gated},
		2: {"swarm:plan-1-accepted"},
		3: {forge.Blocked, forge.Gated, "swarm:proposal-accepted"},
	} {
		if is, _ := m.forge.Issue(n); !reflect.DeepEqual(is.Labels, want) {
			t.Errorf("#%d: %v, want %v", n, is.Labels, want)
		}
	}
}

// A block lifted at a gated stage with the flag left on, as `unblock`
// leaves it, is gated again: the next pass posts that gate's note, the
// newest gate note being the blocked gate's at the same stage.
func TestSwarmApplyGateNoteAfterABlockLifted(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644) // no role runs; the flag step still does
	a.forge.CreateIssue("Plan", mark("owner", 1)+"a", []string{"swarm:plan-0-posted", forge.Blocked})
	a.forge.Comment(issue1, mark("architect", 1)+gatePlan)
	a.forge.Comment(issue1, mark("reconciler", 1)+"worker run 1 failed: x (at swarm:plan-0-posted). Remove swarm:blocked to resume.")
	a.pass()
	if got := a.gateNotes(1); len(got) != 1 || !strings.Contains(got[0], "gated at plan-0-posted: "+"it is blocked") {
		t.Fatalf("the blocked gate: %q", got)
	}
	a.forge.Relabel(issue1, forge.Blocked, "")
	a.pass()
	if got := a.gateNotes(1); len(got) != 2 || !strings.Contains(got[1], "gated at plan-0-posted: the plan gate holds the plan") {
		t.Errorf("after the block lifted: %q\n%s", got, a.stdout.String())
	}
}

// A flag removal refused after the verb's own write is a warning: the
// relabel is made (exit 0), and the next pass clears the flag; after a
// close, with the terminal set, it is named for what it left undone.
func TestSwarmMaintainerFlagRemovalRefused(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted", forge.Gated}) // #1
	m.wrap = func(f *forge.Fake) forge.Adapter { return &refuseFlag{Fake: f, flag: forge.Gated} }
	m.ok(swarmTriage, "Maintainer jane on acme/app\n#1 swarm:proposal-posted → swarm:proposal-accepted\n", "1")
	if want := "⚠ #1: swarm:gated not cleared (forge: DELETE /repos/acme/app/issues/1/labels/7: 502 Bad Gateway); the next pass of apply clears it\n"; m.stderr.String() != want {
		t.Errorf("stderr %q", m.stderr.String())
	}
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:plan-1-accepted", forge.Gated}) // #2
	if err := m.verb(swarmClose, "2"); errText(err) != "Error: swarm: #2 is closed and relabelled swarm:rejected, but swarm:gated is not cleared: forge: DELETE /repos/acme/app/issues/1/labels/7: 502 Bad Gateway; remove it on the forge" {
		t.Errorf("close: %v", err)
	}
}

// The maintainer's refusals and warnings: an issue carrying only the
// gated flag carries no stage label; `block` by a login not in trusted
// warns that the comment holds nothing; `unblock` is refused at
// review-0-ready too when the round limit's second note blocked it (T21's
// write refused), where no role runs.
func TestSwarmMaintainerStep29Refusals(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", "", []string{forge.Gated}) // #1
	if err := m.verb(swarmTriage, "1"); errText(err) != "Error: swarm: #1 carries no stage label; nothing changed" {
		t.Errorf("only gated: %v", err)
	}

	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:review-0-ready", forge.Blocked}) // #2
	m.forge.Comment(forge.Ref{Repo: "app", N: 2}, mark("reconciler", 3)+"Review round 3 of 3 on app#9 requested changes a second time (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked.")
	if err := m.verb(swarmUnblock, "2", "go"); !strings.HasPrefix(errText(err), "Error: swarm: #2: blocked at the review round limit, where both reviewers have reviewed the head and swarm:review-0-ready runs no one;") || len(m.writes(2)) != 0 {
		t.Errorf("unblock at the round limit, at review-0-ready: %v; writes %q", err, m.writes(2))
	}

	m.forge.CreateIssue("C", mark("owner", 1)+"c", []string{"swarm:plan-1-accepted"}) // #3
	m.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	m.ok(swarmBlock, "Maintainer jane on acme/app\n#2 blocking\n#3 commented\n", "3", "--by", "2")
	if want := "⚠ jane is not in trusted: the role rule does not read your Blocked by #2, so it holds nothing (docs/swarm.md section 6)\n"; m.stderr.String() != want {
		t.Errorf("stderr %q", m.stderr.String())
	}
}

// The gated flag's removal on the transitions no test drove on a flagged
// issue (section 5): `accept` (T7) and `merge`'s T13 clear it with the
// relabel, and the merge step's T15, the merge gate since turned off,
// clears it with the close.
func TestSwarmFlagClearedOnEveryTransition(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:plan-0-posted", forge.Gated})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmAccept, head+"#1 swarm:plan-0-posted → swarm:plan-1-accepted\n#1 gated cleared\n", "1")
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	m.approvedPull(1, "c1")
	m.forge.Relabel(issue1, "", forge.Gated)
	m.ok(swarmMerge, head+"#1 merged pull app#2 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n#1 gated cleared\n", "1")
	if is, _ := m.forge.Issue(1); !reflect.DeepEqual(is.Labels, []string{"swarm:plan-1-accepted"}) {
		t.Errorf("T13: %v", is.Labels)
	}

	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:review-2-approved", forge.Gated})
	a.forge.Comment(issue1, mark("architect", 1)+oneSlice)
	n, _ := a.forge.CreatePull("app", "swarm/1-1", "main", "slice", "Refs #1")
	pr := forge.Ref{Repo: "app", N: n}
	a.forge.Push(pr, "c1")
	a.forge.Review(pr, forge.Approve, mark("adversary", 1)+approve)
	a.forge.Review(pr, forge.Approve, mark("sympathetic", 1)+approve)
	a.tick()
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatal(err)
	}
	if is := a.issue(1); is.Open || !reflect.DeepEqual(is.Labels, []string{forge.Done}) || !strings.Contains(a.stdout.String(), "#1 swarm:review-2-approved → swarm:done\n#1 gated cleared\n") {
		t.Errorf("T15: open %v, %v\n%s", is.Open, is.Labels, a.stdout.String())
	}
}

// The auto count keeps an issue gone from the forge's window: each gate
// Daisy passes is recorded in <state>/auto, and status counts per issue
// the larger of the record and the forge's auto notes.
func TestSwarmStatusAutoKeepsOldJobs(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", mark("owner", 1)+"Brief: docs/briefs/a.md — M1\nClass: routine\nCheck.\n", []string{"swarm:proposal-posted"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted"}) {
		t.Fatalf("not passed: %v\n%s", got, a.stdout.String())
	}
	if b, _ := os.ReadFile(a.state("auto")); string(b) != a.Now().UTC().Format(time.RFC3339)+" #1 proposal-posted\n" {
		t.Errorf("<state>/auto: %q", b)
	}
	a.forge.CloseIssue(1)
	a.statusWS.now = a.Now().Add(31 * 24 * time.Hour) // closed 31 days ago: out of the window
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if want := "\n  auto         1 gates passed for routine jobs (triage 1, plan 0)\n"; !strings.Contains(a.stdout.String(), want) {
		t.Errorf("status lacks %q:\n%s", want, a.stdout.String())
	}
	if slices.Contains(a.asked, MaintainerTokenEnv) {
		t.Error("asked for the maintainer's token")
	}
}
