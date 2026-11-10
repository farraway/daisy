package reconcile

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
)

// The notes' repository's URL beside each of section 10's three origin
// forms: the wiki's, and the swarm's own issues repository's.
func TestRepoURL(t *testing.T) {
	for origin, want := range map[string]string{
		"git@forge.example.net:acme/lib.git":            "git@forge.example.net:acme/app.wiki.git",
		"ssh://git@forge.example.net:2222/acme/lib.git": "ssh://git@forge.example.net:2222/acme/app.wiki.git",
		"https://forge.example.net/acme/lib/":           "https://forge.example.net/acme/app.wiki.git",
	} {
		if got := RepoURL(origin, "app.wiki"); got != want {
			t.Errorf("RepoURL(%q) = %q, want %q", origin, got, want)
		}
	}
	if got := RepoURL("https://forge.example.net/acme/lib", "swarm-x"); got != "https://forge.example.net/acme/swarm-x.git" {
		t.Errorf("RepoURL = %q", got)
	}
}

// A note is a top-level page whose header, as a brief's, holds Subject:
// and Source:; a line in a fence or after the first `## ` is not its
// header, and a link, a directory or another page is no note.
func TestReadNotes(t *testing.T) {
	dir := t.TempDir()
	for f, c := range map[string]string{
		"Home.md":     "# Home\n",
		"ruleslib.md": "Subject: ruleslib 2.0.0's rules\nSource: https://example.net/ruleslib v2.0.0\nWritten: 2026-11-01 by architect run 3 on #14\nChecked: 2026-11-04\n\n## Rules\n\nSubject: not this\n",
		"fenced.md":   "```\nSubject: in a fence\nSource: app main\n```\n",
		"late.md":     "## First\n\nSubject: after a heading\nSource: app main\n",
		"no-src.md":   "Subject: a source missing\n",
		"notes.txt":   "Subject: not Markdown\nSource: app main\n",
		"sub/deep.md": "Subject: below the top\nSource: app main\n",
	} {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
		os.WriteFile(filepath.Join(dir, f), []byte(c), 0o644)
	}
	os.Symlink(filepath.Join(dir, "ruleslib.md"), filepath.Join(dir, "link.md"))
	notes, err := ReadNotes(ops.Real().FS(), dir)
	if err != nil || len(notes) != 1 {
		t.Fatalf("%v %+v", err, notes)
	}
	n := notes[0]
	if n.File != "ruleslib.md" || n.Subject != "ruleslib 2.0.0's rules" || n.Source != "https://example.net/ruleslib v2.0.0" || n.Pin() != "v2.0.0" ||
		n.Written != "2026-11-01 by architect run 3 on #14" || n.Checked != "2026-11-04" || n.Behind != -1 {
		t.Errorf("%+v", n)
	}
}

// The index (section 7.3): each note's subject, source and pin, how far a
// declared repository's source has moved, or that an external one is
// dated by its pin alone, and its age; for the owner's batch and the
// architect, after # Repository, with the Task's sentences; the others
// see the mount read-only, and nothing without `wiki` or `notes`; with
// `notes` the line names notes/ in the swarm's own issues repository.
func TestRenderResearchNotes(t *testing.T) {
	f, at := fixture()
	j := job(f, at.N, "architect", forge.ProposalAccepted)
	j.Cfg.Wiki = true
	j.Notes = []Note{
		{File: "ruleslib.md", Subject: "ruleslib's rules", Source: "https://example.net/ruleslib v2.0.0", Written: "2026-11-01 by architect run 3 on #14", Checked: "2026-11-04", Behind: -1},
		{File: "board.md", Subject: "the board", Source: "app 1a2b3c4", Written: "2026-11-06 by owner run 1", Checked: "2026-11-06", Repo: "app", Behind: 12},
		{File: "gone.md", Subject: "a lost pin", Source: "lib deadbee", Repo: "lib", Behind: -1},
		{File: "loose.md", Subject: "no pin", Source: "lib", Repo: "lib", Behind: -1, Checked: "someday"},
	}
	system, prompt := Render(j)
	has(t, "prompt", prompt, []string{"- Resources (read-only): /work/.resources/docs\n- Research notes: /work/.notes, the wiki, pushed by Daisy after the run\n\n# Research notes\n\n" +
		"- /work/.notes/ruleslib.md: ruleslib's rules; source https://example.net/ruleslib v2.0.0, an external source (its pin and its date alone date it); written 2026-11-01 by architect run 3 on #14; checked 2026-11-04 (2 days ago)\n" +
		"- /work/.notes/board.md: the board; source app 1a2b3c4, 12 commits on app's main since the pin; written 2026-11-06 by owner run 1; checked 2026-11-06 (today)\n" +
		"- /work/.notes/gone.md: a lost pin; source lib deadbee, a pin not in lib's history; no Written: line; never checked\n" +
		"- /work/.notes/loose.md: no pin; source lib, no pin; no Written: line; checked someday\n\n# Issue #1"})
	has(t, "system", system, []string{`"Written: 2026-11-06 by architect run 1 on #1" and "Checked: 2026-11-06"`, "one of app, lib, or a URL", `Your plan's "## Research" then names the note rather than repeating it. Daisy commits and pushes /work/.notes after the run; delete no note.`})

	j.Notes = nil
	_, prompt = Render(j)
	has(t, "no notes", prompt, []string{"# Research notes\n\nNone.\n\n# Issue"})

	b := job(f, 0, "owner", "")
	b.N, b.K, b.Batch, b.Issue = 0, 4, &Batch{Propose: 2}, forge.Issue{}
	b.Cfg.Wiki = true
	system, prompt = Render(b)
	has(t, "batch", prompt, []string{"# Research notes\n\nNone.\n\n# Briefs"})
	has(t, "batch task", system, []string{`"Written: 2026-11-06 by owner run 4"`}, "on #0", "names the note")

	w := job(f, at.N, "worker", forge.PlanAccepted)
	w.Cfg.Wiki = true
	system, prompt = Render(w)
	has(t, "worker", prompt, []string{"- Research notes (read-only): /work/.notes\n"}, "# Research notes")
	has(t, "worker task", system, nil, "Research notes", ".notes")

	j.Cfg.Wiki, j.Cfg.Notes, j.Cfg.Forge.Repo = false, true, "acme/swarm-x"
	_, prompt = Render(j)
	has(t, "notes", prompt, []string{"- Research notes: /work/.notes, notes/ in acme/swarm-x, pushed by Daisy after the run\n\n# Research notes\n\nNone."})

	j.Cfg.Notes = false
	system, prompt = Render(j)
	has(t, "no notes kept", prompt+system, nil, "Research notes", ".notes")
}

func TestAge(t *testing.T) {
	for date, want := range map[string]string{"2026-11-06": " (today)", "2026-11-05 by hand": " (1 day ago)", "2026-10-27": " (10 days ago)", "06/11/2026": "", "": ""} {
		if got := age(date, t0); got != want {
			t.Errorf("age(%q) = %q, want %q", date, got, want)
		}
	}
}

// Export gives a run the clone's notes/ alone, without .git; notes/
// absent, or a link (to a host directory, say), is an empty copy, the
// link never followed.
func TestExportNotes(t *testing.T) {
	state := t.TempDir()
	clone := filepath.Join(state, "notes")
	g := Git{O: ops.Real(), State: state}
	if _, err := g.run("", "init", "-q", clone); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(clone, "README.md"), []byte("# Repo\n"), 0o644)
	if _, err := g.run(clone, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first"); err != nil {
		t.Fatal(err)
	}
	n := NotesRepo{Git: g, Sub: "notes", dir: "notes"}
	for _, tc := range []struct {
		name  string
		setup func()
		want  []string
	}{
		{"absent", func() {}, nil},
		{"a link", func() {
			host := t.TempDir()
			os.WriteFile(filepath.Join(host, "host.md"), []byte("the host's\n"), 0o644)
			os.Symlink(host, filepath.Join(clone, "notes"))
		}, nil},
		{"a directory", func() {
			os.Remove(filepath.Join(clone, "notes"))
			os.MkdirAll(filepath.Join(clone, "notes", ".git"), 0o755)
			os.WriteFile(filepath.Join(clone, "notes", "a.md"), []byte("Subject: a\nSource: app main\n"), 0o644)
		}, []string{"a.md"}},
	} {
		tc.setup()
		dst := filepath.Join(t.TempDir(), "copy")
		if _, err := n.Export(dst); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		entries, err := os.ReadDir(dst)
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: the copy holds %q (%v), want %q", tc.name, got, err, tc.want)
		}
	}
}
