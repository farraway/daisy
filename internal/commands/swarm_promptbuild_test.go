package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// ownerAtB3ec5eb is the owner's first default, as
// `git show b3ec5eb:internal/swarm/declare/prompts.go` gives it (its
// resources under /resources): an earlier default.
const ownerAtB3ec5eb = `# Owner

You are the product owner of this repository. You decide what is worth
doing next, in what order, and why.

- Read the resources under /resources and the code under /work before you
  propose anything. Propose only what they support, and name the document
  or file that supports each proposal.
- A proposal is one job the swarm can finish in a few pull requests at
  most. Its title names the outcome. Its body says what is true when the
  job is done, why it is next, and what it depends on.
- Prefer the smallest job that moves the next milestone. Never propose
  what an open issue already covers. Do not re-propose a closed job unless
  the resources changed; say what changed.
- In a thread, answer the question asked. When the maintainer's comment
  changes the job, restate the whole job as it now stands, so that the
  thread's last word is all of it.
- You do not plan the implementation; the architect does.
`

// promptBuild is `swarm prompt build` with flags, found from s.cwd, its
// root s.root.
func (s *swarmWS) promptBuild(flags ...string) error {
	s.stdout.Reset()
	s.stderr.Reset()
	o := &ops.Fake{GetenvFn: func(k string) string { return s.vars[k] }, ExecFn: s.exec}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &s.stdout, Stderr: &s.stderr, Flags: flags}
	return s.at(c, false, promptBuild)
}

// The claim of Step 7 of the plan that clears the Deferred list
// (docs/swarm.md section 11, prompt build): in a swarm repository's clone
// whose owner prompt is an earlier default, whose sympathetic prompt is
// gone and whose worker prompt the maintainer edited, `swarm prompt build`
// rewrites the owner's and writes the sympathetic's, commits the two as
// the clone's identity and pushes them to the bare remote's default
// branch, keeps the worker's byte for byte with its diff from the default
// printed, and fails naming it; --dry-run writes nothing, --force with
// it included; --force rewrites the edited prompt; a last run finds
// nothing to write and commits nothing.
func TestSwarmPromptBuild(t *testing.T) {
	s := newSwarmWS(t)
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("init: %v\n%s", err, s.stderr.String())
	}
	s.cloned()
	up := s.upstreams[swarmURL]
	git(t, s.root, "init", "-q")
	git(t, s.root, "add", "-A")
	git(t, s.root, "commit", "-q", "-m", "the root")
	root := gitIn(t, s.root, "rev-parse", "--short=7", "HEAD")

	edited := declare.DefaultPrompts["worker"] + "- Be brief.\n"
	s.write("prompts/owner.md", ownerAtB3ec5eb)
	s.write("prompts/worker.md", edited)
	os.Remove(filepath.Join(s.ws, "prompts", "sympathetic.md"))
	s.publish("the maintainer's prompts")
	before := gitIn(t, up, "rev-parse", "main")

	// Edited: the others written, committed and pushed; the worker's kept,
	// its diff printed, and the verb fails naming it.
	err := s.promptBuild()
	wantErr := "Error: swarm: edited, so kept: prompts/worker.md (each diff above); merge the default's changes in by hand, or run 'daisy swarm prompt build --force' to overwrite them"
	if err == nil || err.Error() != wantErr {
		t.Errorf("err: %v", err)
	}
	pushed := gitIn(t, up, "rev-parse", "--short=7", "main")
	diff := "--- prompts/worker.md (default)\n+++ prompts/worker.md\n@@ -15,3 +15,4 @@\n" +
		" - The description you write is read by the reviewers and the maintainer:\n" +
		"   what changed, how it was tested, each acceptance claim's status, and\n   anything left undone.\n+- Be brief.\n"
	want := "✓ Rebuilt prompts/owner.md (an earlier default)\n  Unchanged prompts/architect.md\n" +
		"  Kept prompts/worker.md (edited), its diff from the default:\n" + diff +
		"  Unchanged prompts/adversary.md\n✓ Wrote prompts/sympathetic.md\n" +
		"✓ Committed and pushed " + pushed + " to acme/swarm-app's main as Jane <jane@example.net>\n✓ Mirrors: swarm-app fetched\n"
	if s.stdout.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", s.stdout.String(), want)
	}
	if s.read("prompts/owner.md") != declare.DefaultPrompts["owner"] || s.read("prompts/sympathetic.md") != declare.DefaultPrompts["sympathetic"] {
		t.Errorf("owner.md:\n%s\nsympathetic.md:\n%s", s.read("prompts/owner.md"), s.read("prompts/sympathetic.md"))
	}
	if s.read("prompts/worker.md") != edited {
		t.Errorf("the edited worker.md was changed:\n%s", s.read("prompts/worker.md"))
	}
	if got := gitIn(t, up, "log", "-1", "--format=%an <%ae> %s", "main"); got != "Jane <jane@example.net> swarm: prompts rebuilt from the root at "+root+" (owner, sympathetic)" {
		t.Errorf("pushed: %s", got)
	}
	if got := gitIn(t, up, "diff", "--name-only", before, "main"); got != "prompts/owner.md\nprompts/sympathetic.md" {
		t.Errorf("the commit changed:\n%s", got)
	}
	if got := gitIn(t, s.ws, "status", "--porcelain"); got != "" {
		t.Errorf("the clone: %q", got)
	}
	mirror := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "git", "swarm-app")
	if got := gitIn(t, mirror, "rev-parse", "--short=7", "HEAD"); got != pushed {
		t.Errorf("the mirror at %s, pushed %s", got, pushed)
	}

	// --dry-run, --force with it: what --force would do, nothing written.
	if err := s.promptBuild("--force", "--dry-run"); err != nil {
		t.Errorf("dry run: %v", err)
	}
	want = "  Unchanged prompts/owner.md\n  Unchanged prompts/architect.md\n" +
		"  Would rebuild prompts/worker.md (edited, overwritten by --force), its diff from the default:\n" + diff +
		"  Unchanged prompts/adversary.md\n  Unchanged prompts/sympathetic.md\n" +
		"  Dry run: nothing written; would commit \"swarm: prompts rebuilt from the root at " + root + " (worker)\" and push it\n"
	if s.stdout.String() != want || s.read("prompts/worker.md") != edited || gitIn(t, up, "rev-parse", "--short=7", "main") != pushed {
		t.Errorf("dry run:\n%s\nwant:\n%s", s.stdout.String(), want)
	}

	// --force: the edited prompt rewritten, committed and pushed.
	if err := s.promptBuild("--force"); err != nil {
		t.Errorf("--force: %v", err)
	}
	if !strings.Contains(s.stdout.String(), "✓ Rebuilt prompts/worker.md (edited, overwritten by --force), its diff from the default:\n"+diff) ||
		s.read("prompts/worker.md") != declare.DefaultPrompts["worker"] {
		t.Errorf("--force:\n%s\nworker.md:\n%s", s.stdout.String(), s.read("prompts/worker.md"))
	}
	if got := gitIn(t, up, "log", "-1", "--format=%s", "main"); got != "swarm: prompts rebuilt from the root at "+root+" (worker)" {
		t.Errorf("pushed: %s", got)
	}

	// Every prompt current: nothing written, nothing committed.
	head := gitIn(t, up, "rev-parse", "main")
	if err := s.promptBuild(); err != nil {
		t.Errorf("current: %v", err)
	}
	want = ""
	for _, r := range declare.Roles {
		want += "  Unchanged prompts/" + r + ".md\n"
	}
	if want += "  Nothing to commit\n"; s.stdout.String() != want || gitIn(t, up, "rev-parse", "main") != head {
		t.Errorf("current:\n%s", s.stdout.String())
	}
}
