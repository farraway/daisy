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

// The claim of revision 6's plan, Step 6 (section 11, init; section 13,
// its row): `init --defaults` in a folder holding the products' clones and
// no swarm repository's clone creates acme/swarm-app with the maintainer's
// token, private, gives the swarm's account write access, clones it
// beside the products', seeds the tree, commits it as the maintainer and
// pushes it, writes <state>/env 0600, makes the mirrors (the swarm
// repository's holding the push) and the labels with the swarm's token,
// each write's line naming its token; a second run, from the clone,
// writes nothing; then `apply --once` runs a pass that reads what init
// pushed.
func TestSwarmInitCreatesTheSwarmRepository(t *testing.T) {
	s := newSwarmWSIn(t, true)
	if _, err := os.Stat(s.ws); err == nil {
		t.Fatal("a clone before init")
	}
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	up := s.upstreams[swarmURL]
	env := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "env")
	seeded := []string{"repos/app.md", "repos/lib.md", "README.md", "briefs/README.md", "notes/README.md", "docs/README.md", ".gitignore"}
	want := "✓ Maintainer jane: acme/swarm-app created\n✓ Maintainer jane: swarm-bot may write to acme/swarm-app\n✓ Cloned acme/swarm-app into " + s.ws +
		"\n✓ Token $GITEA_TOKEN reads acme/swarm-app as swarm-bot\n✓ Wrote swarm.json\n"
	for _, r := range declare.Roles {
		want += "✓ Wrote prompts/" + r + ".md\n"
	}
	for _, f := range seeded {
		want += "✓ Wrote " + f + "\n"
	}
	want += "✓ Committed and pushed " + gitIn(t, up, "rev-parse", "--short=7", "main") + " to acme/swarm-app's main as Jane <jane@example.net>\n✓ Wrote " + env +
		" (mode 0600)\n✓ Mirrors: swarm-app, app, lib fetched\n✓ Token $GITEA_TOKEN: labels 16 present (16 created, 0 renamed)\n" +
		strings.Replace(s.units(), "for the workspace %f", "for the swarm repository %f", 1)
	if s.stdout.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", s.stdout.String(), want)
	}
	if !strings.Contains(s.stderr.String(), "⚠ "+env+": $CLAUDE_CODE_OAUTH_TOKEN is empty; set it before 'daisy swarm apply'\n") {
		t.Errorf("stderr:\n%s", s.stderr.String())
	}

	// The forge: created by jane, private, swarm-bot a writer by jane's
	// token; the labels made under the swarm's.
	if h, ok := s.forge.Held(); !ok || !reflect.DeepEqual(h, forge.HeldRepository{Private: true, CreatedBy: "jane", Collaborators: map[string]string{"swarm-bot": "write by jane"}}) {
		t.Errorf("the forge holds %+v %v", h, ok)
	}
	if s.opened[4] != "tok-123" || len(s.forge.Labels()) != 16 {
		t.Errorf("labels %q under %q", s.forge.Labels(), s.opened)
	}
	// The clone and the push: one commit by the clone's identity, of
	// exactly the files init wrote, nothing left over.
	if got := gitIn(t, s.ws, "config", "remote.origin.url"); got != swarmURL {
		t.Errorf("the clone's origin: %s", got)
	}
	if got := gitIn(t, up, "log", "--format=%an <%ae> %s", "main"); got != "Jane <jane@example.net> swarm: written by init" {
		t.Errorf("pushed: %s", got)
	}
	files := "swarm.json"
	for _, r := range declare.Roles {
		files += "\nprompts/" + r + ".md"
	}
	files += "\n" + strings.Join(seeded, "\n")
	if got := gitIn(t, up, "ls-tree", "-r", "--name-only", "main"); sortedLines(got) != sortedLines(files) {
		t.Errorf("the pushed tree:\n%s", got)
	}
	if got := gitIn(t, s.ws, "status", "--porcelain"); got != "" || s.read(".gitignore") != ".env.sh\n" {
		t.Errorf("the clone: %q, .gitignore %q", got, s.read(".gitignore"))
	}
	if c, _, err := declare.Load(ops.Real().FS(), s.ws); err != nil || !reflect.DeepEqual(c.Resources, []string{"briefs/", "docs/"}) || !reflect.DeepEqual(c.Trusted, []string{"swarm-bot"}) {
		t.Errorf("swarm.json: %v %+v", err, c)
	}
	// <state>/env: mode 0600, the configuration's variables and the
	// maintainer's, from the environment.
	if fi, err := os.Stat(env); err != nil || fi.Mode().Perm() != 0o600 ||
		readAbs(env) != "export GITEA_TOKEN='tok-123'\nexport CLAUDE_CODE_OAUTH_TOKEN=''\nexport DAISY_MAINTAINER_TOKEN='jane-456'\n" {
		t.Errorf("<state>/env: %v %q", err, readAbs(env))
	}
	// The swarm repository's mirror holds the push.
	if got := gitIn(t, filepath.Join(filepath.Dir(env), "git", "swarm-app"), "rev-parse", "HEAD"); got != gitIn(t, up, "rev-parse", "main") {
		t.Errorf("the mirror at %s", got)
	}

	// A second run, from the clone: the repository adopted, nothing
	// written, committed or created.
	s.cwd = s.ws
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	out := s.stdout.String()
	if !strings.HasPrefix(out, "  Maintainer jane: acme/swarm-app exists\n✓ Maintainer jane: swarm-bot may write to acme/swarm-app\n✓ Token $GITEA_TOKEN reads acme/swarm-app as swarm-bot\n  Unchanged swarm.json\n") ||
		strings.Contains(out, "✓ Wrote") || !strings.Contains(out, "\n  Nothing to commit\n  Kept "+env+" (exists)\n") || !strings.Contains(out, "(0 created, 0 renamed)") ||
		gitIn(t, up, "rev-list", "--count", "main") != "1" {
		t.Errorf("second run:\n%s", out)
	}

	// apply --once on what init left, once the maintainer has set the
	// architect's token in <state>/env: the architect reads the pushed
	// commit.
	a := &applyWS{statusWS: &statusWS{swarmWS: s, issues: "swarm-app"}, runner: &runner.Fake{}, now: fixedClock().Now()}
	os.WriteFile(env, []byte(strings.Replace(readAbs(env), "CLAUDE_CODE_OAUTH_TOKEN=''", "CLAUDE_CODE_OAUTH_TOKEN='oauth-token-0123456789abcdef0123456789'", 1)), 0o600)
	os.WriteFile(a.state("owner-held"), []byte("held 2026-11-06T14:00:00Z: test\n"), 0o644)
	a.cloned()
	if err := a.status(a.forge); err != nil || a.stderr.Len() != 0 {
		t.Errorf("status: %v\n%s", err, a.stderr.String())
	}
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.Gated}) ||
		!strings.Contains(a.result("1-architect-1"), "\nresource swarm-app "+gitIn(t, up, "rev-parse", "main")+"\n") {
		t.Errorf("the pass: %v\n%s\n%s", got, a.result("1-architect-1"), a.stderr.String())
	}
}

// sortedLines is text's lines in order, to compare two listings.
func sortedLines(text string) string {
	lines := strings.Split(text, "\n")
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// readAbs is the file at p, "" when it cannot be read.
func readAbs(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

// init adopts a swarm repository the forge holds (section 11, init): its
// clone made from it, a file the maintainer pushed kept, the rest
// committed on top; and a clone of it already in the folder, holding no
// swarm.json, taken as it is.
func TestSwarmInitAdoptsTheSwarmRepository(t *testing.T) {
	s := newSwarmWSIn(t, true)
	s.forge.AddRepository()
	up := s.upstreams[swarmURL]
	pushFiles(t, up, map[string]string{"README.md": "# Ours\n"})
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	out := s.stdout.String()
	if !strings.HasPrefix(out, "  Maintainer jane: acme/swarm-app exists\n✓ Maintainer jane: swarm-bot may write to acme/swarm-app\n✓ Cloned acme/swarm-app into "+s.ws+"\n") ||
		!strings.Contains(out, "\n  Kept README.md (exists)\n") || s.read("README.md") != "# Ours\n" || gitIn(t, up, "rev-list", "--count", "main") != "2" {
		t.Errorf("adopted:\n%s", out)
	}
	if h, _ := s.forge.Held(); h.CreatedBy != "" || h.Collaborators["swarm-bot"] != "write by jane" {
		t.Errorf("the forge holds %+v", h)
	}

	// A clone of it already in the folder, without swarm.json: no clone
	// made, its files committed.
	s = newSwarmWSIn(t, true)
	s.forge.AddRepository()
	git(t, "", "clone", "-q", s.upstreams[swarmURL], s.ws)
	git(t, s.ws, "remote", "set-url", "origin", swarmURL)
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if out := s.stdout.String(); strings.Contains(out, "Cloned") || !strings.Contains(out, "✓ Committed and pushed ") {
		t.Errorf("an existing clone:\n%s", out)
	}
}

// Each refusal of init in revision 6 (section 11, init, its exit codes),
// each before the write it guards: nothing created or cloned before the
// tokens are checked; nothing after the refused write.
func TestSwarmInitRefusesInAFolder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		set     func(s *swarmWS)
		want    string
		created bool // the forge holds the repository after
		cloned  bool // the clone is made
	}{
		{"the maintainer's token unset", func(s *swarmWS) { delete(s.vars, "DAISY_MAINTAINER_TOKEN") },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN is not set (in <state>/env or the environment)", false, false},
		{"the maintainer's token the swarm's", func(s *swarmWS) { s.vars["DAISY_MAINTAINER_TOKEN"] = "tok-123" },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN holds the swarm's token ($GITEA_TOKEN); the maintainer's writes act with your own (docs/swarm.md section 8)", false, false},
		{"the maintainer's token the swarm's account", func(s *swarmWS) { s.maintainer = forge.Account{Login: "Swarm-Bot"} },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN is Swarm-Bot's token, the swarm's account ($GITEA_TOKEN's); the maintainer's writes act with your own (docs/swarm.md section 8)", false, false},
		{"the read refused", func(s *swarmWS) { s.repoReadFails = true },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN cannot read acme/swarm-app: forge: GET /repos/acme/swarm-app: 403 forbidden", false, false},
		{"the creation refused", func(s *swarmWS) { s.forge.RefuseRepository() },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN cannot create acme/swarm-app: forge: POST /orgs/acme/repos: 403 forbidden", false, false},
		{"the access refused", func(s *swarmWS) { s.forge.AddRepository(); s.forge.RefuseRepository() },
			"Error: swarm: $DAISY_MAINTAINER_TOKEN cannot give swarm-bot write access to acme/swarm-app: forge: PUT /repos/acme/swarm-app/collaborators/swarm-bot: 403 forbidden", true, false},
		{"<dir>/<repo> not a clone of it", func(s *swarmWS) { os.MkdirAll(filepath.Join(s.ws, "notes"), 0o755) },
			"Error: swarm: <ws> exists and is not a clone of acme/swarm-app", false, false},
		{"<dir>/<repo> a clone holding swarm.json", func(s *swarmWS) {
			git(t, "", "clone", "-q", s.upstreams[swarmURL], s.ws)
			git(t, s.ws, "remote", "set-url", "origin", swarmURL)
			os.WriteFile(filepath.Join(s.ws, "swarm.json"), []byte("{}\n"), 0o644)
		}, "Error: swarm: <ws> is the swarm's clone; run 'daisy swarm init' in it, where its swarm.json gives the answers", false, false},
		{"the push refused", func(s *swarmWS) {
			hook := filepath.Join(s.upstreams[swarmURL], "hooks", "pre-receive")
			os.WriteFile(hook, []byte("#!/bin/sh\necho refused >&2\nexit 1\n"), 0o755)
		}, "Error: swarm: push <ws>: git push: remote: refused        \nTo git@forge.example.net:acme/swarm-app.git\n ! [remote rejected] HEAD -> main (pre-receive hook declined)\nerror: failed to push some refs to 'git@forge.example.net:acme/swarm-app.git'; the commit stays in the clone: pull, then push it", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSwarmWSIn(t, true)
			tc.set(s)
			err := s.run("--defaults")
			state := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app")
			if got := strings.NewReplacer(s.ws, "<ws>", state, "<state>", s.upstreams[swarmURL], swarmURL).Replace(errText(err)); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
			_, created := s.forge.Held()
			_, cerr := os.Stat(filepath.Join(s.ws, ".git"))
			if created != tc.created || (cerr == nil) != (tc.cloned || strings.Contains(tc.name, "clone holding")) {
				t.Errorf("created %v, cloned %v", created, cerr == nil)
			}
			if _, err := os.Stat(filepath.Join(state, "env")); err == nil || len(s.forge.Labels()) != 0 {
				t.Errorf("<state>/env written or labels made: %q", s.forge.Labels())
			}
			if tc.cloned && gitIn(t, s.ws, "log", "--format=%s", "-1") != "swarm: written by init" {
				t.Error("the commit is not in the clone")
			}
		})
	}
}

// A re-run takes up where a failed commit stopped (section 11, init):
// the files the first run wrote and could not commit, kept by the second,
// are committed and pushed by it, and nothing else of the clone.
func TestSwarmInitCommitsWhatAFailedRunLeft(t *testing.T) {
	s := newSwarmWSIn(t, true)
	s.forge.AddRepository()
	git(t, "", "clone", "-q", s.upstreams[swarmURL], s.ws)
	git(t, s.ws, "remote", "set-url", "origin", swarmURL)
	hook := filepath.Join(s.ws, ".git", "hooks", "pre-commit")
	os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755)
	if err := s.run("--defaults"); !strings.HasPrefix(errText(err), "Error: swarm: commit in "+s.ws+": git commit: ") {
		t.Fatalf("the failed commit: %v", err)
	}
	os.Remove(hook)
	s.write("mine.txt", "not init's\n")
	s.cwd = s.ws
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	up := s.upstreams[swarmURL]
	if out := s.stdout.String(); strings.Contains(out, "✓ Wrote swarm.json") || !strings.Contains(out, "✓ Committed and pushed ") ||
		len(strings.Split(gitIn(t, up, "ls-tree", "-r", "--name-only", "main"), "\n")) != 13 || gitIn(t, s.ws, "status", "--porcelain") != "?? mine.txt" {
		t.Errorf("the re-run:\n%s\n%s", out, gitIn(t, up, "ls-tree", "-r", "--name-only", "main"))
	}
}
