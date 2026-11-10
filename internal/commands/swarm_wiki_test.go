package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// wikiURL is the issues repository's wiki beside app's origin.
const wikiURL = "git@forge.example.net:acme/app.wiki.git"

// wikiUp is the wiki's upstream: a bare repository on the branch master
// (a Gitea wiki's branch is read, never assumed) holding Home.md and the
// pages given, mapped from wikiURL; and `wiki: true` declared.
func (a *applyWS) wikiUp(pages map[string]string) string {
	a.t.Helper()
	up := filepath.Join(a.t.TempDir(), "app.wiki.git")
	src := a.t.TempDir()
	gitIn(a.t, "", "init", "-q", "-b", "master", src)
	pages["Home.md"] = "# Home\n"
	writes(pages)(src)
	gitIn(a.t, src, "add", "-A")
	gitIn(a.t, src, "commit", "-q", "-m", "first")
	gitIn(a.t, "", "clone", "-q", "--bare", src, up)
	a.upstreams[wikiURL] = up
	a.edit(func(c *declare.Config) { c.Wiki = true })
	return up
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
// maintainer editing the wiki by hand.
func editUp(t *testing.T, up, branch string, pages map[string]string) {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, "", "clone", "-q", "--branch", branch, up, dir)
	writes(pages)(dir)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "by hand")
	gitIn(t, dir, "push", "-q", "origin", branch)
}

// writes is a member's work in its copy of the wiki: the pages written.
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

// The claim of Step 19 (docs/swarm.md section 14, item 79): an architect
// that researches writes a note into /work/.notes, writable for it, which
// Daisy commits by the role's identity with the marker in the message and
// pushes after the run; a second architect, on another issue, is given
// the note in its prompt's # Research notes, with the commits its source
// has moved since the pin, and told to read it before it researches; a
// worker sees the wiki read-only, without the section, and its mount
// point is never committed.
func TestSwarmApplyResearchWiki(t *testing.T) {
	a := newApplyWS(t)
	up := a.wikiUp(map[string]string{})
	base := gitIn(t, up, "rev-parse", "master")
	pin := gitIn(t, a.upstream("app"), "rev-parse", "main")
	a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
	// The member's copy holds no .git, and what it plants in one, or a
	// link to a host file, reaches neither the host's clone nor the wiki.
	hostFile := filepath.Join(t.TempDir(), "host.txt")
	os.WriteFile(hostFile, []byte("the host's\n"), 0o644)
	pwned := filepath.Join(t.TempDir(), "pwned")
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Notes: func(dir string) error {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			t.Error("the run's copy of the wiki holds .git")
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
	if files := gitIn(t, up, "ls-tree", "--name-only", "master"); files != "Board-model.md\nHome.md" {
		t.Errorf("the wiki holds:\n%s", files)
	}

	head := gitIn(t, up, "rev-parse", "master")
	if !strings.Contains(a.stdout.String(), "#1 architect run 1: pushed notes "+head[:7]+"\n") || a.stderr.Len() != 0 {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	if got := gitIn(t, up, "log", "-1", "--format=%P%n%an <%ae>%n%B", "master"); got != base+"\narchitect (Daisy swarm) <swarm@example.net>\n#1 architect run 1: research notes\n\n<!-- daisy-swarm role=architect run=1 -->\n\nCo-Authored-By: claude-fable-5-1 <noreply@anthropic.com>" {
		t.Errorf("the wiki's commit:\n%s", got)
	}
	if got := gitIn(t, up, "show", "master:Board-model.md"); got+"\n" != boardNote(pin) {
		t.Errorf("the note pushed:\n%s", got)
	}
	result := a.result("1-architect-1")
	if !strings.Contains(result, "\nnotes-base "+base+"\n") || !strings.Contains(result, "\nnotes pushed "+head+"\n") {
		t.Errorf("result:\n%s", result)
	}
	spec := a.runner.Launches()[0]
	run := a.state("runs/1-architect-1")
	if !slices.Contains(spec.Mounts, runner.Mount{Src: run + "/notes", Dst: "/work/.notes"}) || !slices.Contains(spec.Argv, "/work/.notes") {
		t.Errorf("the architect's mounts %+v, argv %q", spec.Mounts, spec.Argv)
	}
	if _, err := os.Stat(filepath.Join(run, "notes")); err == nil {
		t.Error("the run's copy of the wiki kept after its notes were pushed")
	}
	if p := a.file("1-architect-1", "io/prompt.md"); !strings.Contains(p, "- Research notes: /work/.notes, the wiki, pushed by Daisy after the run\n\n# Research notes\n\nNone.\n\n# Issue #1") {
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
	want := "# Research notes\n\n- /work/.notes/Board-model.md: the board model; source app " + pin + ", 1 commit on app's main since the pin; written 2026-11-06 by architect run 1 on #1; checked 2026-11-06 (today)\n\n# Issue #2"
	if p := a.file("2-architect-1", "io/prompt.md"); !strings.Contains(p, want) {
		t.Errorf("the second architect's prompt lacks the note:\n%s", p)
	}
	if s := a.file("2-architect-1", "io/system.md"); !strings.Contains(s, `Before you research a source, read its note if "# Research notes" lists one`) {
		t.Errorf("the second architect's task:\n%s", s)
	}
	if !strings.Contains(a.result("2-architect-1"), "\nnotes unchanged\n") || gitIn(t, up, "rev-parse", "master") != head {
		t.Errorf("a run that wrote nothing pushed:\n%s", a.result("2-architect-1"))
	}

	// The worker: the wiki read-only, no section, its mount point never
	// committed.
	a.forge.SetLabel(forge.Ref{Repo: "app", N: 2}, "swarm:plan-1-accepted")
	a.runner.Script(2, "worker", runner.Canned{Out: "Done.\n", Work: func(work string) error {
		if err := os.WriteFile(filepath.Join(work, ".notes", "stray.md"), []byte("x\n"), 0o644); err != nil {
			return err
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
	if !slices.Contains(worker.Mounts, runner.Mount{Src: a.state("runs/2-worker-1/notes"), Dst: "/work/.notes", ReadOnly: true}) {
		t.Errorf("the worker's mounts %+v", worker.Mounts)
	}
	if p := a.file("2-worker-1", "io/prompt.md"); !strings.Contains(p, "- Research notes (read-only): /work/.notes\n") || strings.Contains(p, "# Research notes") {
		t.Errorf("the worker's prompt:\n%s", p)
	}
	if files := gitIn(t, a.upstream("app"), "ls-tree", "-r", "--name-only", "swarm/2-1"); strings.Contains(files, ".notes") {
		t.Errorf("the worker's branch holds the mount point:\n%s", files)
	}
}

// The wiki is the maintainer's to edit at will (section 7.6, The notes):
// a page changed during the run is kept, the run's commit put on top of
// it; the same page changed is a conflict, the run's commit pushed to its
// own branch with a warning; a note holding a secret the member was given
// is not pushed and its copy kept, the host's clone left without it; a
// host's clone deleted during the run is made again for the push; and an
// unreachable wiki stops the pass with what to do.
func TestSwarmApplyResearchWikiMerges(t *testing.T) {
	secret := "oauth-token-0123456789abcdef0123456789"
	for _, tc := range []struct {
		name     string
		hand     map[string]string // the maintainer's edit during the run
		note     string            // what the architect writes to Board-model.md
		line     string            // the result's notes line, without its sha
		warn     string
		Board    string // master's Board-model.md after
		kept     bool
		onBranch bool
		gone     bool // the host's clone deleted during the run
	}{
		{name: "another page", hand: map[string]string{"Home.md": "# Home, by hand\n"}, note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes pushed ", Board: "Subject: x\nSource: app main\n\nNew.\n"},
		{name: "the same page", hand: map[string]string{"Board-model.md": "By hand.\n"}, note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes branch swarm/notes-1-architect-1 ",
			warn: "⚠ #1 architect run 1: notes conflict with the wiki's head; pushed to swarm/notes-1-architect-1\n", Board: "By hand.\n", onBranch: true},
		{name: "a secret", note: "Subject: x\nSource: app main\n\n" + secret + "\n", line: "notes kept: secret in a note", Board: "Old.\n", kept: true},
		{name: "the host's clone deleted", note: "Subject: x\nSource: app main\n\nNew.\n", line: "notes pushed ", Board: "Subject: x\nSource: app main\n\nNew.\n", gone: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			up := a.wikiUp(map[string]string{"Board-model.md": "Old.\n"})
			base := gitIn(t, up, "rev-parse", "master")
			a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Notes: func(dir string) error {
				if tc.hand != nil {
					editUp(t, up, "master", tc.hand)
				}
				if tc.gone {
					os.RemoveAll(a.state("wiki"))
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
			if got := gitIn(t, up, "show", "master:Board-model.md"); got+"\n" != tc.Board {
				t.Errorf("master's note %q", got)
			}
			if tc.hand != nil {
				for f, c := range tc.hand {
					if got := gitIn(t, up, "show", "master:"+f); got+"\n" != c {
						t.Errorf("the maintainer's %s lost: %q", f, got)
					}
				}
			}
			if tc.name == "another page" {
				if parent := gitIn(t, up, "rev-parse", "master^"); parent == base {
					t.Error("the run's commit is not on top of the maintainer's")
				}
			}
			if tc.onBranch {
				if got := gitIn(t, up, "log", "-1", "--format=%P", "swarm/notes-1-architect-1"); got != base {
					t.Errorf("the branch's commit's parent %s, want the run's base %s", got, base)
				}
				if got := gitIn(t, up, "show", "swarm/notes-1-architect-1:Board-model.md"); got+"\n" != tc.note {
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
			// wiki has: nothing kept is left for a later export.
			if st := gitIn(t, a.state("wiki"), "status", "--porcelain"); st != "" {
				t.Errorf("the host's clone is left dirty: %q", st)
			}
			if tc.kept {
				if b, _ := os.ReadFile(a.state("wiki/Board-model.md")); string(b) != "Old.\n" {
					t.Errorf("the host's clone holds the kept note: %q", b)
				}
			}
		})
	}

	// A wiki that cannot be cloned: the pass's error, saying what to do.
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Wiki = true })
	a.upstreams[wikiURL] = filepath.Join(t.TempDir(), "none.wiki.git")
	a.forge.CreateIssue("Board model", "Change the board.", []string{"swarm:proposal-accepted"})
	a.tick()
	if err := a.apply(a.git(), "--once"); err == nil || !strings.Contains(err.Error(), "enable the issues repository's wiki and create its first page") || len(a.runner.Launches()) != 0 {
		t.Errorf("an absent wiki: %v", err)
	}
}
