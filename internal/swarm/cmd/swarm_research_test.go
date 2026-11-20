package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// notesUp declares `notes: true` and pushes notes/Home.md and the pages
// given under notes/ to the swarm repository's main, as the maintainer
// does; it is the swarm repository's upstream.
func (a *applyWS) notesUp(pages map[string]string) string {
	a.t.Helper()
	pages["Home.md"] = "# Home\n"
	for f, c := range pages {
		a.write(filepath.Join("notes", f), c)
	}
	a.edit(func(c *declare.Config) { c.Notes = true })
	a.publish("notes")
	a.fetchMirror()
	return a.upstreams[swarmURL]
}

// gitIn is real git in dir, failing the test on an error; its output
// trimmed.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	args = append([]string{"-c", "user.name=T", "-c", "user.email=t@example.net"}, args...)
	out, errb, err := ops.Real().Exec(dir, "git", args...)
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, errb)
	}
	return strings.TrimSpace(string(out))
}

// editUp commits pages to the bare upstream up's branch, as the
// maintainer editing it by hand.
func editUp(t *testing.T, up, branch string, pages map[string]string) {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, "", "clone", "-q", "--branch", branch, up, dir)
	writes(pages)(dir)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "by hand")
	gitIn(t, dir, "push", "-q", "origin", branch)
}

// writes is a member's work in its copy of the notes: the pages written.
func writes(pages map[string]string) func(string) error {
	return func(dir string) error {
		for f, c := range pages {
			if err := os.WriteFile(filepath.Join(dir, f), []byte(c), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func boardNote(pin string) string {
	return "Subject: the board model\nSource: app " + pin + "\nWritten: 2026-11-06 by architect run 1 on #1\nChecked: 2026-11-06\n\n## Findings\n\nThe board is a grid.\n"
}

// The claim of Step 19 (docs/swarm/decisions/0079-the-research-wiki.md): an architect
// that researches writes a note into /job/notes, writable for it, which
// Daisy commits by the role's identity with the marker in the message and
// pushes after the run; a second architect, on another issue, is given
// the note in its prompt's # Research notes, with the commits its source
// has moved since the pin, and told to read it before it researches; a
// worker sees the notes read-only, without the section, and its mount
// point is never committed.
func TestSwarmApplyResearchNotes(t *testing.T) {
	a := newApplyWS(t)
	up := a.notesUp(map[string]string{})
	base := gitIn(t, up, "rev-parse", "main")
	pin := gitIn(t, a.upstream("app"), "rev-parse", "main")
	a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
	// The member's copy holds no .git, and what it plants in one, or a
	// link to a host file, reaches neither the host's clone nor notes/.
	hostFile := filepath.Join(t.TempDir(), "host.txt")
	os.WriteFile(hostFile, []byte("the host's\n"), 0o644)
	pwned := filepath.Join(t.TempDir(), "pwned")
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Notes: func(dir string) error {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			t.Error("the run's copy of the notes holds .git")
		}
		os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755)
		os.WriteFile(filepath.Join(dir, ".git", "hooks", "post-commit"), []byte("#!/bin/sh\ntouch "+pwned+"\n"), 0o755)
		os.Symlink(hostFile, filepath.Join(dir, "Leak.md"))
		return writes(map[string]string{"Board-model.md": boardNote(pin)})(dir)
	}})
	a.pass()
	if _, err := os.Stat(pwned); err == nil {
		t.Error("a hook the member planted ran on the host")
	}
	if files := gitIn(t, up, "ls-tree", "--name-only", "main", "notes/"); files != "notes/Board-model.md\nnotes/Home.md\nnotes/README.md" {
		t.Errorf("notes/ holds:\n%s", files)
	}

	head := notesTip(t, up)
	if !strings.Contains(a.stdout.String(), "#1 architect run 1: pushed notes "+head[:7]+"\n") || a.stderr.Len() != 0 {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	if got := gitIn(t, up, "log", "-1", "--format=%P%n%an <%ae>%n%B", head); got != base+"\narchitect (Daisy swarm) <swarm@example.net>\n#1 architect run 1: research notes\n\n<!-- daisy-swarm role=architect run=1 -->\n\nCo-Authored-By: claude-fable-5-1 <noreply@anthropic.com>" {
		t.Errorf("the notes' commit:\n%s", got)
	}
	if got := gitIn(t, up, "show", "main:notes/Board-model.md"); got+"\n" != boardNote(pin) {
		t.Errorf("the note pushed:\n%s", got)
	}
	result := a.result("1-architect-1")
	if !strings.Contains(result, "\nnotes-base "+base+"\n") || !strings.Contains(result, "\nnotes pushed "+head+"\n") {
		t.Errorf("result:\n%s", result)
	}
	spec := a.runner.Launches()[0]
	run := a.state("runs/1-architect-1")
	if !slices.Contains(spec.Mounts, runner.Mount{Src: run + "/notes", Dst: "/job/notes"}) || !slices.Contains(spec.Argv, "/job") {
		t.Errorf("the architect's mounts %+v, argv %q", spec.Mounts, spec.Argv)
	}
	if _, err := os.Stat(filepath.Join(run, "notes")); err == nil {
		t.Error("the run's copy of the notes kept after they were pushed")
	}
	if p := a.file("1-architect-1", "io/prompt.md"); !strings.Contains(p, "- Research notes: /job/notes, notes/ in acme/swarm-app, pushed by Daisy after the run\n\n# Research notes\n\nNone.\n\n# Issue #1") {
		t.Errorf("run 1's prompt:\n%s", p)
	}
	if s := a.file("1-architect-1", "io/system.md"); !strings.Contains(s, `"Written: 2026-11-06 by architect run 1 on #1"`) || !strings.Contains(s, `Your plan's "## Research" then names the note`) {
		t.Errorf("run 1's task:\n%s", s)
	}

	// A commit on app since the pin, then a second architect on another
	// issue: given the note, one commit behind.
	editUp(t, a.upstream("app"), "main", map[string]string{"board.go": "package board\n"})
	a.forge.CreateIssue("Board moves", "Add moves.", []string{"swarm:proposal-accepted"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	want := "# Research notes\n\n- /job/notes/Board-model.md: the board model; source app " + pin + ", 1 commit on app's main since the pin; written 2026-11-06 by architect run 1 on #1; checked 2026-11-06 (today)\n\n# Issue #2"
	if p := a.file("2-architect-1", "io/prompt.md"); !strings.Contains(p, want) {
		t.Errorf("the second architect's prompt lacks the note:\n%s", p)
	}
	if s := a.file("2-architect-1", "io/system.md"); !strings.Contains(s, `Before you research a source, read its note if "# Research notes" lists one`) {
		t.Errorf("the second architect's task:\n%s", s)
	}
	if !strings.Contains(a.result("2-architect-1"), "\nnotes unchanged\n") || notesTip(t, up) != head {
		t.Errorf("a run that wrote nothing pushed:\n%s", a.result("2-architect-1"))
	}

	// The worker: the notes read-only, outside its checkout, no section.
	a.forge.SetLabel(a.ref(2), "swarm:plan-1-accepted")
	a.runner.Script(2, "worker", runner.Canned{Out: "Done.\n", Work: func(work string) error {
		work = filepath.Join(work, "app") // the checkout, under /job/work
		if _, err := os.Lstat(filepath.Join(work, ".notes")); err == nil {
			return fmt.Errorf("a mount point in the checkout")
		}
		return commit(map[string]string{"main.go": "v1\n"}, "v1")(work)
	}})
	a.pass()
	var worker runner.Spec
	for _, l := range a.runner.Launches() {
		if strings.HasSuffix(l.Name, ".2.worker.1") {
			worker = l
		}
	}
	if !slices.Contains(worker.Mounts, runner.Mount{Src: a.state("runs/2-worker-1/notes"), Dst: "/job/notes", ReadOnly: true}) {
		t.Errorf("the worker's mounts %+v", worker.Mounts)
	}
	if p := a.file("2-worker-1", "io/prompt.md"); !strings.Contains(p, "- Research notes (read-only): /job/notes\n") || strings.Contains(p, "# Research notes") {
		t.Errorf("the worker's prompt:\n%s", p)
	}
	if r := a.result("2-worker-1"); strings.Contains(r, "launch-failed") {
		t.Errorf("the worker's run:\n%s", r)
	}
}

// The notes are the maintainer's to edit at will (section 7.6, The notes):
// a page changed during the run is kept, the run's commit put on top of
// it; the same page changed is a conflict, the run's commit pushed to its
// own branch with a warning; a note holding a secret the member was given
// is not pushed and its copy kept, the host's clone left without it; a
// host's clone deleted during the run is made again for the push.
func TestSwarmApplyResearchNotesMerges(t *testing.T) {
	secret := "oauth-token-0123456789abcdef0123456789"
	for _, tc := range []struct {
		name     string
		hand     map[string]string // the maintainer's edit during the run
		note     string            // what the architect writes to Board-model.md
		line     string            // the result's notes line, without its sha
		warn     string
		Board    string // main's notes/Board-model.md after
		kept     bool
		onBranch bool
		gone     bool // the host's clone deleted during the run
	}{
		{name: "another page", hand: map[string]string{"notes/Home.md": "# Home, by hand\n"}, note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes pushed ", Board: "Subject: x\nSource: app main\n\nNew.\n"},
		{name: "the same page", hand: map[string]string{"notes/Board-model.md": "By hand.\n"}, note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes branch swarm/notes-1-architect-1 ",
			warn: "⚠ #1 architect run 1: notes conflict with acme/swarm-app's head; pushed to swarm/notes-1-architect-1\n", Board: "By hand.\n", onBranch: true},
		{name: "a secret", note: "Subject: x\nSource: app main\n\n" + secret + "\n", line: "notes kept: secret in a note", Board: "Old.\n", kept: true},
		{name: "the host's clone deleted", note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes pushed ", Board: "Subject: x\nSource: app main\n\nNew.\n", gone: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			up := a.notesUp(map[string]string{"Board-model.md": "Old.\n"})
			base := gitIn(t, up, "rev-parse", "main")
			a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Notes: func(dir string) error {
				if tc.hand != nil {
					editUp(t, up, "main", tc.hand)
				}
				if tc.gone {
					os.RemoveAll(a.state("notes"))
				}
				return writes(map[string]string{"Board-model.md": tc.note})(dir)
			}})
			a.pass()
			result := a.result("1-architect-1")
			if !strings.Contains(result, "\n"+tc.line) || !strings.HasSuffix(result, "outcome posted\ncollected\n") {
				t.Errorf("result:\n%s", result)
			}
			if tc.warn != "" && a.stderr.String() != tc.warn {
				t.Errorf("stderr %q", a.stderr.String())
			}
			if got := gitIn(t, up, "show", "main:notes/Board-model.md"); got+"\n" != tc.Board {
				t.Errorf("master's note %q", got)
			}
			if tc.hand != nil {
				for f, c := range tc.hand {
					if got := gitIn(t, up, "show", "main:"+f); got+"\n" != c {
						t.Errorf("the maintainer's %s lost: %q", f, got)
					}
				}
			}
			if tc.name == "another page" {
				if parent := gitIn(t, up, "rev-parse", "main^"); parent == base {
					t.Error("the run's commit is not on top of the maintainer's")
				}
			}
			if tc.onBranch {
				if got := gitIn(t, up, "log", "-1", "--format=%P", "swarm/notes-1-architect-1"); got != base {
					t.Errorf("the branch's commit's parent %s, want the run's base %s", got, base)
				}
				if got := gitIn(t, up, "show", "swarm/notes-1-architect-1:notes/Board-model.md"); got+"\n" != tc.note {
					t.Errorf("the branch's note %q", got)
				}
			}
			_, err := os.Stat(a.state("runs/1-architect-1/notes/Board-model.md"))
			if tc.kept != (err == nil) {
				t.Errorf("the run's copy kept: %v, want %v", err == nil, tc.kept)
			}
			if tc.kept && (!strings.Contains(a.stderr.String(), "notes not pushed (secret in a note); kept in ") || strings.Contains(a.stderr.String(), secret)) {
				t.Errorf("stderr %q", a.stderr.String())
			}
			// The host's clone holds no page of the run's but what the
			// swarm repository has: nothing kept is left for a later export.
			if st := gitIn(t, a.state("notes"), "status", "--porcelain"); st != "" {
				t.Errorf("the host's clone is left dirty: %q", st)
			}
			if tc.kept {
				if b, _ := os.ReadFile(a.state("notes/notes/Board-model.md")); string(b) != "Old.\n" {
					t.Errorf("the host's clone holds the kept note: %q", b)
				}
			}
		})
	}
}

// The claim of Step 8 of the plan for the inbox requests F27-F35 (F32;
// docs/swarm.md section 7.6, The notes, and section 11, Waiting for
// you): two architects that add different sections to one note in one
// pass both land on main, the second's commit put on the first's, as
// two runs appending at a note's end do; two that change the same line
// make the second's branch, which status lists under Waiting for you
// once a pass has fetched it, and no more once it is deleted.
func TestSwarmApplyResearchNotesRebased(t *testing.T) {
	const old = "Subject: the board model\nSource: app main\nWritten: 2026-11-05 by hand\nChecked: 2026-11-05\n\n## Findings\n\nThe board is a grid.\n"
	checked := func(day string) string { return strings.Replace(old, "Checked: 2026-11-05", "Checked: "+day, 1) }
	const moves, scores = "\n## Moves (#1)\n\nA move is a pair.\n", "\n## Scores (#2)\n\nA score is a sum.\n"
	for _, tc := range []struct {
		name         string
		one, two     string // what #1's and #2's architects leave in Board-model.md
		Board, line2 string // main's note after, #2's notes line without its sha
	}{
		{name: "sections added", one: checked("2026-11-06") + moves, two: old + scores,
			Board: checked("2026-11-06") + moves + scores, line2: "notes pushed "},
		{name: "the same lines", one: checked("2026-11-06") + moves, two: checked("2026-11-07") + scores,
			Board: checked("2026-11-06") + moves, line2: "notes branch swarm/notes-2-architect-1 "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			up := a.notesUp(map[string]string{"Board-model.md": old})
			for n, note := range map[int]string{1: tc.one, 2: tc.two} {
				a.forge.CreateIssue(fmt.Sprintf("Board %d", n), "Change the board.", []string{"swarm:proposal-accepted"})
				a.runner.Script(n, "architect", runner.Canned{Out: archPlan, Notes: writes(map[string]string{"Board-model.md": note})})
			}
			a.pass()
			one, two := a.result("1-architect-1"), a.result("2-architect-1")
			if base := strings.SplitN(one, "notes-base ", 2); len(base) < 2 || !strings.Contains(two, "notes-base "+base[1][:40]) {
				t.Fatalf("the two runs were not given the same notes:\n%s\n%s", one, two)
			}
			if !strings.Contains(one, "\nnotes pushed ") || !strings.Contains(two, "\n"+tc.line2) {
				t.Errorf("results:\n%s\n%s", one, two)
			}
			if got := gitIn(t, up, "show", "main:notes/Board-model.md"); got+"\n" != tc.Board {
				t.Errorf("main's note:\n%s", got)
			}
			branches := gitIn(t, up, "for-each-ref", "--format=%(refname)", "refs/heads/swarm/")
			waiting := func() string {
				t.Helper()
				if err := a.status(a.forge); err != nil {
					t.Fatal(err)
				}
				for _, l := range strings.Split(a.stdout.String(), "\n") {
					if w, ok := strings.CutPrefix(l, "Waiting for you: "); ok {
						return w
					}
				}
				t.Fatalf("no Waiting for you line:\n%s", a.stdout.String())
				return ""
			}
			if tc.name == "sections added" {
				pushed := func(result string) string {
					_, sha, _ := strings.Cut(result, "\nnotes pushed ")
					sha, _, _ = strings.Cut(sha, "\n")
					return sha
				}
				if got := gitIn(t, up, "log", "-1", "--format=%P%n%an%n%B", pushed(two)+"^{commit}"); got != pushed(one)+
					"\narchitect (Daisy swarm)\n#2 architect run 1: research notes\n\n<!-- daisy-swarm role=architect run=1 -->\n\nCo-Authored-By: claude-fable-5-1 <noreply@anthropic.com>" {
					t.Errorf("#2's commit is not the run's, on #1's:\n%s", got)
				}
				if branches != "" || a.stderr.Len() != 0 {
					t.Errorf("branches %q, stderr %q", branches, a.stderr.String())
				}
				a.pass()
				if w := waiting(); strings.Contains(w, "notes") {
					t.Errorf("Waiting for you: %s", w)
				}
				return
			}
			if branches != "refs/heads/swarm/notes-2-architect-1" || !strings.Contains(a.stderr.String(), "⚠ #2 architect run 1: notes conflict with acme/swarm-app's head; pushed to swarm/notes-2-architect-1\n") {
				t.Errorf("branches %q, stderr %q", branches, a.stderr.String())
			}
			if got := gitIn(t, up, "show", "swarm/notes-2-architect-1:notes/Board-model.md"); got+"\n" != tc.two {
				t.Errorf("the branch's note:\n%s", got)
			}
			a.pass() // the swarm repository's mirror fetched
			if w := waiting(); !strings.HasSuffix(w, "; notes swarm/notes-2-architect-1") && w != "notes swarm/notes-2-architect-1" {
				t.Errorf("Waiting for you: %s", w)
			}
			gitIn(t, up, "branch", "-D", "swarm/notes-2-architect-1") // merged by hand and deleted
			a.pass()
			if w := waiting(); strings.Contains(w, "notes") {
				t.Errorf("a deleted branch still waits: %s", w)
			}
		})
	}
}

// A push of the notes refused (section 7.6, The notes): when the
// branch's head moved since the fetch the commit was put on (another
// writer's push between the two, a journal's say), the commit is put on
// the new head and pushed again; refused with the head unmoved (a hook),
// it is `notes kept: push rejected`, the run's copy kept.
func TestSwarmApplyResearchNotesPushRefused(t *testing.T) {
	note := "Subject: x\nSource: app main\n\nNew.\n"
	for _, tc := range []struct {
		name  string
		moved bool // another writer pushes to main before the notes' push
		line  string
	}{
		{name: "a journal commit first", moved: true, line: "notes pushed "},
		{name: "refused, the head unmoved", line: "notes kept: push rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			up := a.notesUp(map[string]string{"Board-model.md": "Old.\n"})
			var other string
			if tc.moved {
				a.beforeGit = func(dir string, args []string) {
					if other == "" && dir == a.state("notes") && slices.Contains(args, "push") && strings.HasSuffix(args[len(args)-1], ":refs/heads/main") {
						hand := t.TempDir()
						gitIn(t, "", "clone", "-q", up, hand)
						os.MkdirAll(filepath.Join(hand, "journal"), 0o755)
						os.WriteFile(filepath.Join(hand, "journal", "2026-11-06.md"), []byte("# 2026-11-06\n\n- 1400 0-owner-9: posted\n"), 0o644)
						gitIn(t, hand, "add", "-A")
						gitIn(t, hand, "commit", "-q", "-m", "swarm: journal of 1 run")
						gitIn(t, hand, "push", "-q", "origin", "main")
						other = gitIn(t, hand, "rev-parse", "HEAD")
					}
				}
			} else {
				os.WriteFile(filepath.Join(up, "hooks", "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
			}
			a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Notes: writes(map[string]string{"Board-model.md": note})})
			a.pass()
			result := a.result("1-architect-1")
			if !strings.Contains(result, "\n"+tc.line) || !strings.HasSuffix(result, "outcome posted\ncollected\n") {
				t.Errorf("result:\n%s\nstderr %q", result, a.stderr.String())
			}
			_, err := os.Stat(a.state("runs/1-architect-1/notes/Board-model.md"))
			if !tc.moved {
				if err != nil || !strings.Contains(a.stderr.String(), "⚠ #1 architect run 1: notes not pushed (push rejected); kept in ") {
					t.Errorf("the run's copy kept: %v; stderr %q", err, a.stderr.String())
				}
				return
			}
			if other == "" {
				t.Fatal("no notes' push to main")
			}
			if got := gitIn(t, up, "show", "main:notes/Board-model.md"); got+"\n" != note || err == nil {
				t.Errorf("main's note %q; the run's copy kept: %v", got, err == nil)
			}
			if got := gitIn(t, up, "log", "-1", "--format=%P", notesTip(t, up)); got != other {
				t.Errorf("the notes' commit's parent %s, want the other writer's %s", got, other)
			}
			if got := gitIn(t, up, "show", "main:journal/2026-11-06.md"); !strings.HasPrefix(got, "# 2026-11-06\n\n- 1400 0-owner-9: posted\n") {
				t.Errorf("the other writer's journal lost:\n%s", got)
			}
		})
	}
}
