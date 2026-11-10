package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// ownURL is the swarm's own issues repository beside app's origin.
const ownURL = "git@forge.example.net:acme/swarm-x.git"

const ownBrief = "# M0\n\nStatus: active\nFirst.\n\n## M1 The board\n\nA persona sees the board.\n"

// ownUp makes acme/swarm-x the issues repository, the swarm's own: an
// upstream on main holding README.md, briefs/m0.md and notes/README.md,
// cloned at <ws>/swarm-x as the maintainer keeps it, swarm.json naming it
// with `notes: true` and its clone's briefs/ the one resource, and the
// Fake forge's issues and milestones there. It is the upstream.
func (a *applyWS) ownUp() string {
	a.t.Helper()
	up := filepath.Join(a.t.TempDir(), "swarm-x.git")
	src := a.t.TempDir()
	gitIn(a.t, "", "init", "-q", "-b", "main", src)
	os.MkdirAll(filepath.Join(src, "briefs"), 0o755)
	os.MkdirAll(filepath.Join(src, "notes"), 0o755)
	writes(map[string]string{"README.md": "# The swarm's repository\n", "briefs/m0.md": ownBrief, "notes/README.md": "# Notes\n\nOne file per subject.\n"})(src)
	gitIn(a.t, src, "add", "-A")
	gitIn(a.t, src, "commit", "-q", "-m", "first")
	gitIn(a.t, "", "clone", "-q", "--bare", src, up)
	a.upstreams[ownURL] = up
	clone := filepath.Join(a.ws, "swarm-x")
	gitIn(a.t, "", "clone", "-q", up, clone)
	gitIn(a.t, clone, "remote", "set-url", "origin", ownURL)
	a.forge = forge.NewFake(&ops.Fake{NowFn: a.Now}, "acme/swarm-x", []string{"app", "lib"}, forge.Account{Login: "swarm-bot", Email: "swarm@example.net"})
	a.forge.AddLabels(forge.Labels...)
	a.issues = "swarm-x"
	a.edit(func(c *declare.Config) {
		c.Forge.Repo, c.Notes, c.Resources = "acme/swarm-x", true, []string{"swarm-x/briefs/"}
	})
	return up
}

func ruleslibNote(by string) string {
	return "Subject: ruleslib's rules\nSource: https://example.net/ruleslib v2.0.0\nWritten: 2026-11-06 by " + by + "\nChecked: 2026-11-06\n\n## Rules\n\nThree.\n"
}

// The claim of Step 23 (docs/swarm.md section 14, item 83): with the
// swarm's own issues repository acme/swarm-x, holding no product code,
// and `notes: true`, the owner's batch reads the brief from its clone's
// briefs/ and proposes there, its milestone mirrored there; the batch's
// note and the architect's land in its notes/ on main, by each role's
// identity with the marker, the maintainer's files beside them untouched;
// the architect's plan is a comment on the issue there and its prompt
// indexes the batch's note but not notes/README.md; the worker builds in
// app, never in swarm-x, its pull request referring to acme/swarm-x#1;
// the host's clone is <state>/notes and the maintainer's clone is never
// written.
func TestSwarmApplyOwnIssuesRepository(t *testing.T) {
	a := newApplyWS(t)
	up := a.ownUp()
	first := gitIn(t, up, "rev-parse", "main")
	clone := filepath.Join(a.ws, "swarm-x")

	a.runner.Script(0, "owner", runner.Canned{Out: "## Board\nBrief: swarm-x/briefs/m0.md — M1 The board\nClass: routine\nShow the board.\n",
		Notes: writes(map[string]string{"Ruleslib.md": ruleslibNote("owner run 1")})})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted"}) || a.stderr.Len() != 0 {
		t.Fatalf("the batch's proposal: %v\nstdout:\n%s\nstderr:\n%s", got, a.stdout.String(), a.stderr.String())
	}
	if p := a.file("0-owner-1", "io/prompt.md"); !strings.Contains(p, "## swarm-x/briefs/m0.md (Status: active; mounted at /work/.resources/swarm-x/briefs/m0.md)") ||
		!strings.Contains(p, "- Research notes: /work/.notes, notes/ in acme/swarm-x, pushed by Daisy after the run\n\n# Research notes\n\nNone.\n") {
		t.Errorf("the batch's prompt:\n%s", p)
	}
	if got := a.milestoneTitles(); !reflect.DeepEqual(got, []string{"m0: M1 The board"}) || a.milestoneOf(1) != "m0: M1 The board" {
		t.Errorf("milestones %q, #1's %q", got, a.milestoneOf(1))
	}
	if got := gitIn(t, up, "log", "-1", "--format=%P%n%an <%ae>%n%B", "main"); got != first+"\nowner (Daisy swarm) <swarm@example.net>\nowner run 1: research notes\n\n<!-- daisy-swarm role=owner run=1 -->\n\nCo-Authored-By: claude-opus-5-5 <noreply@anthropic.com>" {
		t.Errorf("the batch's notes commit:\n%s", got)
	}

	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice, Notes: func(dir string) error {
		if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
			t.Errorf("the architect's copy lacks notes/README.md: %v", err)
		}
		for _, f := range []string{"briefs", ".git", "notes"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				t.Errorf("the architect's copy holds %s: it is notes/, not the repository", f)
			}
		}
		return writes(map[string]string{"Board-model.md": boardNote(gitIn(t, a.upstream("app"), "rev-parse", "main"))})(dir)
	}})
	a.pass()
	is := a.issue(1)
	if got := labels(is); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) || !slices.ContainsFunc(is.Comments, func(c forge.Comment) bool {
		return strings.HasPrefix(c.Body, "<!-- daisy-swarm role=architect run=1 -->\n**architect:**\n\n"+oneSlice)
	}) {
		t.Fatalf("the plan on acme/swarm-x#1: %v %+v", got, is.Comments)
	}
	if p := a.file("1-architect-1", "io/prompt.md"); !strings.Contains(p, "# Research notes\n\n- /work/.notes/Ruleslib.md: ruleslib's rules; source https://example.net/ruleslib v2.0.0, an external source") || strings.Contains(p, "/work/.notes/README.md") {
		t.Errorf("the architect's index:\n%s", p)
	}
	arch := a.runner.Launches()[1]
	if arch.Label != "acme/swarm-x#1/architect" || !slices.Contains(arch.Mounts, runner.Mount{Src: a.state("runs/1-architect-1/notes"), Dst: "/work/.notes"}) {
		t.Errorf("the architect's spec %+v", arch)
	}
	if files := gitIn(t, up, "ls-tree", "-r", "--name-only", "main"); files != "README.md\nbriefs/m0.md\nnotes/Board-model.md\nnotes/README.md\nnotes/Ruleslib.md" {
		t.Errorf("acme/swarm-x's main holds:\n%s", files)
	}
	if got := gitIn(t, up, "log", "-1", "--format=%an%n%s", "main"); got != "architect (Daisy swarm)\n#1 architect run 1: research notes" {
		t.Errorf("the architect's notes commit: %s", got)
	}
	if got := gitIn(t, up, "show", "main:briefs/m0.md"); got+"\n" != ownBrief {
		t.Errorf("the brief changed: %q", got)
	}
	if !strings.Contains(a.result("1-architect-1"), "\nnotes-base ") {
		t.Errorf("result:\n%s", a.result("1-architect-1"))
	}

	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	ps := a.pulls(1)
	if len(ps) != 1 || ps[0].Repo != "app" || !strings.HasSuffix(ps[0].Body, "\n\nRefs acme/swarm-x#1") {
		t.Fatalf("the pull request: %+v", ps)
	}
	var worker runner.Spec
	for _, l := range a.runner.Launches() {
		if strings.HasSuffix(l.Name, ".1.worker.1") {
			worker = l
		}
	}
	if worker.Image != "daisy-member:1" || worker.Label != "acme/swarm-x#1/worker" || !slices.Contains(worker.Mounts, runner.Mount{Src: a.state("runs/1-worker-1/notes"), Dst: "/work/.notes", ReadOnly: true}) {
		t.Errorf("the worker's spec %+v", worker)
	}
	if _, err := os.Stat(a.state("git/swarm-x")); err == nil {
		t.Error("a mirror of the issues repository: it is no product repository")
	}
	if _, err := os.Stat(a.state("notes/.git")); err != nil {
		t.Errorf("the host's clone: %v", err)
	}
	if _, err := os.Stat(a.state("wiki")); err == nil {
		t.Error("a wiki clone with notes")
	}
	if st := gitIn(t, clone, "status", "--porcelain"); st != "" || gitIn(t, clone, "rev-parse", "HEAD") != first {
		t.Errorf("the maintainer's clone written: %q", st)
	}
}

// A notes/ that is not a directory on the repository's main (the
// maintainer's link) is never replaced nor followed: the run's copy is
// empty, its note is kept with a warning, nothing is committed, and the
// run is collected, its proposal posted.
func TestSwarmApplyOwnNotesNotADirectory(t *testing.T) {
	a := newApplyWS(t)
	up := a.ownUp()
	dir := t.TempDir()
	gitIn(t, "", "clone", "-q", up, dir)
	os.RemoveAll(filepath.Join(dir, "notes"))
	os.Symlink("briefs", filepath.Join(dir, "notes"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "notes as a link")
	gitIn(t, dir, "push", "-q", "origin", "main")
	head := gitIn(t, up, "rev-parse", "main")
	a.runner.Script(0, "owner", runner.Canned{Out: "## Board\nBrief: swarm-x/briefs/m0.md — M1 The board\nClass: routine\nShow the board.\n", Notes: func(dir string) error {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("the copy holds %d entries: the link was followed", len(entries))
		}
		return writes(map[string]string{"Ruleslib.md": ruleslibNote("owner run 1")})(dir)
	}})
	a.pass()
	result := a.result("0-owner-1")
	if !strings.Contains(result, "\nnotes kept: notes/ is not a directory\n") || !strings.HasSuffix(result, "collected\n") || gitIn(t, up, "rev-parse", "main") != head {
		t.Errorf("result:\n%s", result)
	}
	if !strings.Contains(a.stderr.String(), "notes not pushed (notes/ is not a directory); kept in ") || len(a.issue(1).Labels) == 0 {
		t.Errorf("stderr %q", a.stderr.String())
	}
}

// A notes/ that is not a directory on main keeps nothing of a run that
// wrote no note: `notes unchanged`, no warning, the run's copy removed.
func TestSwarmApplyOwnNotesNotADirectoryNothingWritten(t *testing.T) {
	a := newApplyWS(t)
	up := a.ownUp()
	dir := t.TempDir()
	gitIn(t, "", "clone", "-q", up, dir)
	os.RemoveAll(filepath.Join(dir, "notes"))
	os.WriteFile(filepath.Join(dir, "notes"), []byte("a file\n"), 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "notes as a file")
	gitIn(t, dir, "push", "-q", "origin", "main")
	a.runner.Script(0, "owner", runner.Canned{Out: "## Board\nBrief: swarm-x/briefs/m0.md — M1 The board\nClass: routine\nShow the board.\n"})
	a.pass()
	result := a.result("0-owner-1")
	if !strings.Contains(result, "\nnotes unchanged\n") || !strings.HasSuffix(result, "collected\n") || strings.Contains(a.stderr.String(), "notes not pushed") {
		t.Errorf("result:\n%s\nstderr %q", result, a.stderr.String())
	}
	if _, err := os.Lstat(a.state("runs/0-owner-1/notes")); err == nil {
		t.Error("the run's copy is kept")
	}
}

// A notes/ that is a directory at the run's base but a link on main as
// fetched at the collection (the maintainer's, during the run) is never
// written through: the run's note does not reach the link's target, on
// main or in the host's clone, and is pushed to its own branch.
func TestSwarmApplyOwnNotesBecomeALink(t *testing.T) {
	a := newApplyWS(t)
	up := a.ownUp()
	brief := gitIn(t, up, "rev-parse", "main:briefs")
	a.runner.Script(0, "owner", runner.Canned{Out: "## Board\nBrief: swarm-x/briefs/m0.md — M1 The board\nClass: routine\nShow the board.\n", Notes: func(dir string) error {
		hand := t.TempDir()
		gitIn(t, "", "clone", "-q", up, hand)
		os.RemoveAll(filepath.Join(hand, "notes"))
		os.Symlink("briefs", filepath.Join(hand, "notes"))
		gitIn(t, hand, "add", "-A")
		gitIn(t, hand, "commit", "-q", "-m", "notes as a link")
		gitIn(t, hand, "push", "-q", "origin", "main")
		return writes(map[string]string{"Ruleslib.md": ruleslibNote("owner run 1")})(dir)
	}})
	a.pass()
	result := a.result("0-owner-1")
	if !strings.Contains(result, "\nnotes branch swarm/notes-0-owner-1 ") || !strings.HasSuffix(result, "collected\n") {
		t.Errorf("result:\n%s\nstderr %q", result, a.stderr.String())
	}
	if got := gitIn(t, up, "rev-parse", "main:briefs"); got != brief {
		t.Errorf("main's briefs/ changed: %s", gitIn(t, up, "ls-tree", "-r", "main"))
	}
	if _, err := os.Lstat(a.state("notes/briefs/Ruleslib.md")); err == nil {
		t.Error("the note was written through the link in the host's clone")
	}
	if got := gitIn(t, up, "show", "swarm/notes-0-owner-1:notes/Ruleslib.md"); got+"\n" != ruleslibNote("owner run 1") {
		t.Errorf("the branch's note %q", got)
	}
}
