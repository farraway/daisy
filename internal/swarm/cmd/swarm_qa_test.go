package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// pageMap is the page map the maintainer keeps in the product's
// repository notes (section 4), which the QA drives.
const pageMap = "# app\n\n## Page map\n\n- /login: the sign-in form; Sign in goes to /boards.\n- /boards: the boards; New board adds one.\n"

// qa declares the QA with trigger e, its persona the owner's
// and its prompt the default, writes the page map into app's notes and
// publishes the clone; the owner's batch stays held.
func (a *applyWS) qa(e declare.QA) {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		c.QA = &e
		c.Personas[declare.QARole] = c.Personas["owner"]
	})
	a.write("prompts/qa.md", declare.DefaultPrompts[declare.QARole])
	a.write("repos/app.md", pageMap)
	a.publish("the QA")
}

// tagUp tags app's main in its upstream, as the maintainer's release
// does.
func (a *applyWS) tagUp(tag string) {
	a.t.Helper()
	if _, errb, err := ops.Real().Exec(a.upstream("app"), "git", "tag", tag, "main"); err != nil {
		a.t.Fatalf("tag: %v %s", err, errb)
	}
}

// qaRuns is the QA's launches, in order.
func (a *applyWS) qaRuns() []runner.Spec {
	var out []runner.Spec
	for _, s := range a.runner.Launches() {
		if strings.HasSuffix(s.Label, "#0/qa") {
			out = append(out, s)
		}
	}
	return out
}

// The QA after merges (section 6, The QA trigger): with
// qa.merges 2, it runs as issue 0 once base's first-parent line
// holds two commits its last completed exploration does not (with none,
// every commit), in the product's image over every repository's clone,
// its system.md carrying its Task text and the page map in app's notes;
// each failure it reports is a proposal issue marked as the QA's,
// classed by its first line, which the owner leaves unanswered as its
// own, a routine one passing triage by itself; it does not run again until two more
// merges, and then reports no failure, creating nothing.
func TestSwarmQAAfterMerges(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.qa(declare.QA{Merges: ptr(2)})
	a.runner.Script(0, "qa", runner.Canned{Out: "## Sign in loses the board\nClass: routine\nOn /login, Sign in goes to /boards with no board listed.\n\n## New board fails\nOn /boards, New board answers 500.\n"})
	a.pass()
	if n := len(a.qaRuns()); n != 0 {
		t.Fatalf("the QA ran after one commit: %d\n%s", n, a.stdout.String())
	}

	a.land("app", map[string]string{"login.go": "package app\n"}, "the login page")
	a.pass()
	runs := a.qaRuns()
	if len(runs) != 1 {
		t.Fatalf("QA launches after the merge: %d\n%s%s", len(runs), a.stdout.String(), a.stderr.String())
	}
	if s := runs[0]; s.Image != "daisy-member:1" || s.Workdir != "/job/work/app" {
		t.Errorf("launched in %s at %s", s.Image, s.Workdir)
	}
	sys, prompt := a.file("0-qa-1", "io/system.md"), a.file("0-qa-1", "io/prompt.md")
	for _, want := range []string{
		"# Persona\n\n## QA\n",
		`# Task` + "\n\n" + `Use app as its users do and report what fails. In its clone /job/work/app, at main, start the app and its browser as "# Repository notes: app" says, and drive each route and control of the page map those notes keep.`,
		`Report each failure as a job for acme/swarm-app, at most 5, the most serious first;`,
		`what you expected, what happened, and the error text or log line it left, then a browser test that reproduces the failure from a fresh start, failing now and passing once it is fixed: in the browser test framework the repository notes name, or else a Playwright test, in a fenced block, for the fix's slice to add to the repository's tests.`,
		"# Repository notes: app\n\n## app\n\n### Page map\n\n- /login: the sign-in form;",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("system.md lacks %q:\n%s", want, sys)
		}
	}
	for _, want := range []string{"- Clone /job/work/app: acme/app, branch main, base main", "- Clone /job/work/lib: acme/lib", "# Open swarm issues\n\nNone.", "# Recently closed\n\nNone."} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt.md lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "# Briefs") {
		t.Errorf("the QA's prompt shows the briefs:\n%s", prompt)
	}
	// The routine failure passes triage by itself in the pass that
	// created it (T23), its owner having nothing to answer; the other
	// waits for the maintainer.
	for i, want := range []struct {
		title, body, stage string
		architecture       bool
	}{
		{"Sign in loses the board", "<!-- daisy-swarm role=qa run=1 -->\n**qa:**\n\nClass: routine\nOn /login,", forge.ProposalAccepted, false},
		{"New board fails", "<!-- daisy-swarm role=qa run=1 -->\n**qa:**\n\nOn /boards, New board answers 500.", forge.ProposalPosted, true},
	} {
		full := a.issue(i + 1)
		if full.Title != want.title || !strings.HasPrefix(full.Body, want.body) || hasLabel(full, forge.ClassArchitecture) != want.architecture || !hasLabel(full, want.stage) {
			t.Errorf("proposal %d: %q %v\n%s", i+1, full.Title, full.Labels, full.Body)
		}
	}
	for _, want := range []string{"qa run 1: launched\n", "qa run 1: proposed #1 Sign in loses the board\n", "qa run 1: proposed #2 New board fails\n"} {
		if !strings.Contains(a.stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, a.stdout.String())
		}
	}

	a.pass()
	for _, s := range a.runner.Launches() {
		if strings.HasSuffix(s.Label, "/owner") {
			t.Errorf("the owner answered the QA's proposal: %s", s.Label)
		}
	}
	a.land("app", map[string]string{"boards.go": "package app\n"}, "the boards page")
	a.pass()
	if n := len(a.qaRuns()); n != 1 {
		t.Fatalf("the QA ran again before two more merges: %d", n)
	}
	a.runner.Script(0, "qa", runner.Canned{Out: "NO FAILURE: /login, /boards\n"})
	a.land("app", map[string]string{"board.go": "package app\n"}, "a board")
	a.pass()
	if n := len(a.qaRuns()); n != 2 || !strings.Contains(a.stdout.String(), "qa run 2: no failure\n") {
		t.Fatalf("after two more merges: %d launches\n%s", n, a.stdout.String())
	}
	if is, err := a.forge.Issue(3); err == nil {
		t.Errorf("a run with no failure proposed: %+v", is)
	}
	if _, err := os.Stat(a.state("qa-held")); err == nil {
		t.Error("a run with no failure held the QA")
	}

	// main force-pushed to a history without the explored commit, which
	// the mirror then no longer holds: warned of, the pass going on, the
	// merges counted from the branch's start, so one commit runs nothing
	// and a second runs the QA.
	explored := gitIn(t, a.upstream("app"), "rev-parse", "main")
	src := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main", src}, {"-C", src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "rewritten"},
		{"-C", src, "push", "-q", "-f", a.upstream("app"), "main"}} {
		if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	a.checkedGreen("app")
	a.fetchProduct("app")
	gitIn(t, a.state("git/app"), "gc", "-q", "--prune=now")
	if _, _, err := ops.Real().Exec(a.state("git/app"), "git", "cat-file", "-e", explored+"^{commit}"); err == nil {
		t.Fatal("the mirror still holds the explored commit")
	}
	warning := "⚠ qa: app's main no longer holds " + explored[:7] + ", the commit the last exploration explored (it was force-pushed): its merges are counted from the branch's start\n"
	a.runner.Script(0, "qa", runner.Canned{Out: "NO FAILURE: /login, /boards\n"})
	a.pass()
	if n := len(a.qaRuns()); n != 2 || strings.Count(a.stderr.String(), warning) != 1 {
		t.Fatalf("after the force-push: %d launches\n%s", n, a.stderr.String())
	}
	a.land("app", map[string]string{"again.go": "package app\n"}, "again")
	a.pass()
	if n := len(a.qaRuns()); n != 3 {
		t.Fatalf("after two commits on the rewritten main: %d launches\n%s%s", n, a.stdout.String(), a.stderr.String())
	}
}

func hasLabel(is forge.Issue, l string) bool {
	for _, x := range is.Labels {
		if x == l {
			return true
		}
	}
	return false
}

// The QA after a release tag (section 6): qa.tag runs it once
// base holds a tag matching the pattern that its last completed
// exploration did not; a tag of another pattern runs nothing; with the
// triage gate off its findings are proposal-accepted (T2).
func TestSwarmQAAfterATag(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Triage = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.qa(declare.QA{Tag: ptr("v*")})
	a.runner.Script(0, "qa", runner.Canned{Out: "## New board fails\nClass: routine\nOn /boards, New board answers 500.\n"})
	a.land("app", map[string]string{"login.go": "package app\n"}, "the login page")
	a.pass()
	if n := len(a.qaRuns()); n != 0 {
		t.Fatalf("the QA ran with no tag: %d", n)
	}
	a.tagUp("v1.0")
	a.pass()
	if n := len(a.qaRuns()); n != 1 {
		t.Fatalf("QA launches after v1.0: %d\n%s%s", n, a.stdout.String(), a.stderr.String())
	}
	if is, _ := a.forge.Issues(forge.Open, forge.ProposalAccepted); len(is) != 1 || hasLabel(a.issue(is[0].Number), forge.ClassArchitecture) {
		t.Errorf("triage off: %+v", is)
	}
	a.land("app", map[string]string{"boards.go": "package app\n"}, "the boards page")
	a.tagUp("nightly-1")
	a.pass()
	a.pass()
	if n := len(a.qaRuns()); n != 1 {
		t.Fatalf("the QA ran again with no release tag: %d", n)
	}
	a.tagUp("v1.1")
	a.pass()
	if n := len(a.qaRuns()); n != 2 {
		t.Errorf("QA launches after v1.1: %d", n)
	}
}

// The QA's hold (sections 2 and 6): a run that writes BLOCKED: or
// fails writes <state>/qa-held, which status names and which holds
// the trigger through further merges until `swarm resume` removes it;
// the merges are counted from the last completed exploration, none
// here, so the QA resumed with no merge since the held run runs
// at once; a fault holds nothing; the owner's hold is not the
// QA's.
func TestSwarmQAHeld(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, held string
		canned     runner.Canned
	}{
		{"blocked", "blocked: the image has no browser", runner.Canned{Out: "BLOCKED: the image has no browser\n"}},
		{"proposed nothing", "proposed nothing", runner.Canned{Out: "The pages look fine.\n"}},
		{"failed", "exit 1", runner.Canned{Out: "## x\n", Code: 1}},
		{"fault", "", runner.Canned{Code: 125}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.qa(declare.QA{Merges: ptr(2)})
			a.runner.Script(0, "qa", tc.canned)
			a.land("app", map[string]string{"login.go": "package app\n"}, "the login page")
			a.pass()
			path := a.state("qa-held")
			held, err := os.ReadFile(path)
			if tc.held == "" {
				if err == nil {
					t.Errorf("a fault held the QA: %s", held)
				}
				return
			}
			if string(held) != "held 2026-11-06T15:01:00Z: "+tc.held+"\n" {
				t.Errorf("qa-held: %q\n%s", held, a.stdout.String())
			}
			if want := "qa run 1: " + tc.held + "; qa held (" + path + ")\n"; !strings.Contains(a.stdout.String(), want) {
				t.Errorf("stdout lacks %q:\n%s", want, a.stdout.String())
			}
			if is, _ := a.forge.Issues(forge.Open, forge.ProposalPosted); len(is) != 0 {
				t.Errorf("created %+v", is)
			}
			if _, err := os.Stat(a.state("owner-held")); err != nil || !strings.HasPrefix(string(must(os.ReadFile(a.state("owner-held")))), "held 2026-11-06T14:00:00Z: test") {
				t.Errorf("the owner's hold changed: %v", err)
			}
			if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), " — qa held: "+tc.held+"\n") {
				t.Errorf("status: %v\n%s", err, a.stdout.String())
			}
			// Resumed with no merge since: the held run completed no
			// exploration, so the merges count from the branch's start.
			a.resume()
			if !strings.Contains(a.stdout.String(), "QA released\n") || !strings.Contains(a.stdout.String(), "Owner batch released\n") {
				t.Errorf("resume: %s", a.stdout.String())
			}
			os.WriteFile(a.state("owner-held"), []byte("held 2026-11-06T14:00:00Z: test\n"), 0o644)
			a.pass()
			if n := len(a.qaRuns()); n != 2 {
				t.Fatalf("QA launches after resume: %d", n)
			}
			a.land("app", map[string]string{"boards.go": "package app\n"}, "the boards page")
			a.land("app", map[string]string{"board.go": "package app\n"}, "a board")
			a.pass()
			if n := len(a.qaRuns()); n != 2 {
				t.Fatalf("a held QA ran: %d", n)
			}
			a.resume()
			a.runner.Script(0, "qa", runner.Canned{Out: "NO FAILURE: /login, /boards\n"})
			a.pass()
			if n := len(a.qaRuns()); n != 3 {
				t.Errorf("QA launches after the second resume: %d", n)
			}
		})
	}
}

func must(b []byte, _ error) []byte { return b }

func ptr[T any](v T) *T { return &v }

// A QA run not declared never runs, whatever lands on base; swarm.json
// declaring it names its persona in status (section 11).
func TestSwarmQAUndeclared(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.land("app", map[string]string{"login.go": "package app\n"}, "the login page")
	a.tagUp("v1.0")
	a.pass()
	if n := len(a.qaRuns()); n != 0 {
		t.Fatalf("an undeclared QA ran: %d", n)
	}
	if err := a.status(a.forge); err != nil || strings.Contains(a.stdout.String(), "qa") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
	a.qa(declare.QA{Merges: ptr(5)})
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "\n  qa           anthropic  claude-opus-5-5 high\n") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
	if _, err := os.Stat(filepath.Join(a.ws, "prompts", "qa.md")); err != nil {
		t.Error(err)
	}
}

// Issue 0 runs one batch run at a time (section 6): the owner's batch
// and the QA due in one pass, the owner's runs first and the
// QA at the next pass.
func TestSwarmQAAfterTheOwner(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.qa(declare.QA{Merges: ptr(1)})
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Out: "## One\nBrief: docs/briefs/a.md — M1 The drift\nA.\n"})
	a.runner.Script(0, "qa", runner.Canned{Out: "NO FAILURE: /login\n"})
	a.pass()
	if _, err := os.Stat(a.state("runs/0-owner-1")); err != nil || len(a.qaRuns()) != 0 {
		t.Fatalf("first pass: owner %v, QA %d\n%s", err, len(a.qaRuns()), a.stdout.String())
	}
	a.pass()
	if n := len(a.qaRuns()); n != 1 {
		t.Errorf("second pass: QA %d\n%s", n, a.stdout.String())
	}
}
