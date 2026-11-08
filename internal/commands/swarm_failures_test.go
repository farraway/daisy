package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// planned is a new issue #1 with an accepted one-slice plan at planned.
func (a *applyWS) planned(plan string) {
	a.t.Helper()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
	a.runner.Script(1, "architect", runner.Canned{Out: plan})
	a.pass()
	a.forge.SetLabel(issue1, "swarm:planned")
}

// Every work-failure row of section 7.7 a worker or a reviewer can meet,
// then T17 with the prior label in the note and a failed worker's commits
// kept in the mirror; then T18, the maintainer's relabel, runs the role
// again.
func TestSwarmApplyWorkFailures(t *testing.T) {
	secret := "oauth-token-0123456789abcdef0123456789"
	for _, tc := range []struct {
		name   string
		review bool // the failure is the adversary's, #1 in review
		canned runner.Canned
		check  runner.Canned
		setup  func(a *applyWS)
		reason string
		kept   bool
	}{
		{name: "timeout", canned: runner.Canned{Out: "half\n", TimedOut: true, Code: 137, Work: commit(map[string]string{"main.go": "half\n"}, "Half")}, reason: "timeout after 45m", kept: true},
		{name: "non-zero exit", canned: runner.Canned{Out: "x\n", Code: 2}, reason: "exit 2"},
		{name: "no output", canned: runner.Canned{Work: commit(map[string]string{"main.go": "v1\n"}, "v1")}, reason: "no output", kept: true},
		{name: "no commits", canned: runner.Canned{Out: "Nothing to do.\n"}, reason: "no commits"},
		{name: "protected path", canned: runner.Canned{Out: "Done.\n", Work: commit(map[string]string{"main.go": "v1\n", ".gitea/workflows/ci.yml": "on: push\n", "docs/AGENTS.md": "x\n"}, "v1")},
			reason: "protected path changed: .gitea/workflows/ci.yml, docs/AGENTS.md", kept: true},
		{name: "a clone reaching outside itself", canned: runner.Canned{Out: "Done.\n", Work: func(work string) error {
			if err := commit(map[string]string{"main.go": "v1\n"}, "v1")(work); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(work, ".git", "objects", "info", "alternates"), []byte("/elsewhere/objects\n"), 0o644)
		}}, reason: "the clone reaches outside itself: it holds .git/objects/info/alternates"},
		{name: "a clone with a link named by a secret", canned: runner.Canned{Out: "Done.\n", Work: func(work string) error {
			if err := commit(map[string]string{"main.go": "v1\n"}, "v1")(work); err != nil {
				return err
			}
			return os.Symlink("x", filepath.Join(work, ".git", "refs", secret))
		}}, reason: "the clone reaches outside itself: .git holds a link"},
		{name: "a clone with an unreadable directory", canned: runner.Canned{Out: "Done.\n", Work: func(work string) error {
			if err := commit(map[string]string{"main.go": "v1\n"}, "v1")(work); err != nil {
				return err
			}
			os.Mkdir(filepath.Join(work, ".git", "closed"), 0o755)
			return os.Chmod(filepath.Join(work, ".git", "closed"), 0)
		}}, reason: "the clone reaches outside itself: a directory in .git cannot be read"},
		{name: "a protected path with a newline", canned: runner.Canned{Out: "Done.\n", Work: commit(map[string]string{"main.go": "v1\n", ".gitea/x\ncost 4242.5": "x\n"}, "v1")},
			reason: `protected path changed: .gitea/x\ncost 4242.5`, kept: true},
		{name: "secret in a blob", canned: runner.Canned{Out: "Done.\n", Work: commit(map[string]string{"sub/key.txt": secret + "\n"}, "v1")}, reason: "secret in output: commits", kept: true},
		{name: "secret in a protected path's name", canned: runner.Canned{Out: "Done.\n", Work: commit(map[string]string{".gitea/workflows/" + secret + ".yml": "on: push\n"}, "v1")}, reason: "secret in output: commits", kept: true},
		{name: "secret in a commit message", canned: runner.Canned{Out: "Done.\n", Work: commit(map[string]string{"main.go": "v1\n"}, "token "+secret)}, reason: "secret in output: commits", kept: true},
		{name: "secret in the check's tail", canned: work("v1"), check: runner.Canned{Stdout: "env: " + secret + "\n"}, reason: "secret in output: check.out", kept: true},
		{name: "push rejected", canned: work("v1"), setup: func(a *applyWS) {
			// a branch of the name the forge holds already, unrelated
			src := a.t.TempDir()
			for _, args := range [][]string{{"init", "-q", "-b", "x", src}, {"-C", src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "other"},
				{"-C", src, "push", "-q", a.upstream("app"), "x:refs/heads/swarm/1-1"}} {
				if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
					a.t.Fatalf("git %v: %v %s", args, err, errb)
				}
			}
		}, reason: "push rejected", kept: true},
		{name: "bad verdict: changes without a blocking finding", review: true, canned: runner.Canned{Out: "VERDICT: request-changes\n\n[non-blocking] main.go:1 style.\n"}, reason: "bad verdict"},
		{name: "bad verdict: approval with a blocking finding", review: true, canned: runner.Canned{Out: "VERDICT: approve\n\n[blocking] main.go:1 wrong.\n"}, reason: "bad verdict"},
		{name: "bad verdict: no verdict line", review: true, canned: runner.Canned{Out: "Looks fine.\n"}, reason: "bad verdict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			role, label := "worker", "swarm:planned"
			if tc.review {
				a.inReview(oneSlice)
				role, label = "adversary", "swarm:in-review"
			} else {
				a.planned(oneSlice)
			}
			if tc.setup != nil {
				tc.setup(a)
			}
			a.runner.Script(1, role, tc.canned)
			a.runner.Script(1, "check", tc.check)
			a.pass()
			k := map[string]string{"worker": "1", "adversary": "1"}[role]
			note := mark("reconciler", 1) + role + " run " + k + " failed: " + tc.reason + " (was " + label + "). Relabel the issue to resume."
			if tc.kept {
				note += " Its commits are kept in the swarm's mirror as refs/swarm-runs/1-worker-1."
			}
			b := a.bodies(1)
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:blocked", forge.Gated}) || b[len(b)-1] != note {
				t.Fatalf("T17: %v %q\n%s", got, b, a.result("1-"+role+"-1"))
			}
			r := a.result("1-" + role + "-1")
			if !strings.Contains(r, "failed: "+tc.reason+"\n") || !strings.HasSuffix(r, "outcome failed\ncollected\n") {
				t.Errorf("result:\n%s", r)
			}
			mirror := a.state("git/app")
			_, _, err := ops.Real().Exec(mirror, "git", "rev-parse", "--verify", "--quiet", "refs/swarm-runs/1-worker-1")
			if tc.kept != (err == nil) || tc.kept != strings.Contains(r, "kept refs/swarm-runs/1-worker-1\n") {
				t.Errorf("kept %v, ref present %v:\n%s", tc.kept, err == nil, r)
			}
			if !tc.review {
				if ps := a.pulls(1); len(ps) != 0 {
					t.Errorf("a pull request was opened: %+v", ps)
				}
				if out, _, _ := ops.Real().Exec(a.upstream("app"), "git", "log", "--all", "--format=%B"); strings.Contains(string(out), secret) {
					t.Error("the secret was pushed")
				}
			}
			if strings.Contains(strings.Join(b, "\n"), secret) {
				t.Error("the secret was posted")
			}

			a.forge.SetLabel(issue1, label) // T18
			a.runner.Script(1, role, runner.Canned{Out: "BLOCKED: what now?\n"})
			a.pass()
			if _, err := os.Stat(a.state("runs/1-" + role + "-2/result")); err != nil {
				t.Errorf("T18 did not run the %s again", role)
			}
		})
	}
}

// The trailer (section 7.6): a commit the member made without its hook is
// reported on stderr and as `unstamped <sha>`, and pushed as it is.
func TestSwarmApplyReportsAnUnstampedCommit(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n", Work: commit(map[string]string{"main.go": "v1\n"}, "Unhooked", "--no-verify")})
	a.pass()
	ps := a.pulls(1)
	if len(ps) != 1 {
		t.Fatalf("the run did not post: %s", a.result("1-worker-1"))
	}
	sha := ps[0].HeadSHA
	if !strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: commit "+sha[:7]+" has no Co-Authored-By: claude-sonnet-5-5 <noreply@anthropic.com>\n") ||
		!strings.Contains(a.result("1-worker-1"), "\nunstamped "+sha+"\n") || !strings.Contains(a.result("1-worker-1"), "\npushed "+sha+"\n") {
		t.Errorf("stderr %q\n%s", a.stderr.String(), a.result("1-worker-1"))
	}
}

// A secret read from the slot when docker run returned (a refresh during
// the run) is refused in a reviewer's out.md: nothing posted.
func TestSwarmApplySecretFromTheSlotAtReturn(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	refreshed := "refreshed-access-token-0123456789abcdefghij"
	slot := filepath.Join(a.local, "swarm", "credentials", "openai", "1")
	a.runner.Script(1, "adversary", runner.Canned{Out: "VERDICT: approve\n\nI saw " + refreshed + "\n", Work: func(string) error {
		return os.WriteFile(filepath.Join(slot, "auth.json"), []byte(`{"tokens":{"access_token":"`+refreshed+`"}}`), 0o600)
	}})
	a.pass()
	rs, _ := a.forge.Reviews(prRef(2))
	if len(rs) != 0 || !strings.Contains(a.result("1-adversary-1"), "failed: secret in output: out.md\n") {
		t.Errorf("reviews %+v\n%s", rs, a.result("1-adversary-1"))
	}
}

// Swarm faults (section 7.7): nothing on the forge, the role relaunched;
// two in a row write paused with the second's reason; resume clears it.
// A slot without a login is its run's fault, met at the launch that needs
// the slot, so a pass on other providers is unaffected.
func TestSwarmApplyFaults(t *testing.T) {
	a := newApplyWS(t)
	slot := filepath.Join(a.local, "swarm", "credentials", "openai", "1")
	os.Remove(filepath.Join(slot, "auth.json"))
	a.inReview(oneSlice) // the architect and the worker are anthropic's
	before := a.bodies(1)
	a.pass()
	r := a.result("1-adversary-1")
	want := "fault: slot openai/1: no login; run 'CODEX_HOME=" + slot + " codex login -c cli_auth_credentials_store=file'\noutcome fault\ncollected\n"
	if !strings.HasSuffix(r, want) || len(a.runner.Launches()) != 3 || !reflect.DeepEqual(a.bodies(1), before) {
		t.Fatalf("slot without login:\n%s", r)
	}
	if _, err := os.Stat(a.state("paused")); err == nil {
		t.Fatal("one fault paused the swarm")
	}
	a.pass()
	paused, err := os.ReadFile(a.state("paused"))
	if err != nil || !strings.HasSuffix(string(paused), ": swarm fault: slot openai/1: no login; run 'CODEX_HOME="+slot+" codex login -c cli_auth_credentials_store=file'\n") {
		t.Fatalf("paused: %q %v", paused, err)
	}
	if !reflect.DeepEqual(a.bodies(1), before) || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:in-review"}) {
		t.Errorf("a fault reached the forge: %q", a.bodies(1))
	}
	a.pass()
	if !strings.Contains(a.stdout.String(), "Paused since ") || strings.Contains(a.stdout.String(), "launched\n") {
		t.Errorf("paused pass:\n%s", a.stdout.String())
	}
	os.WriteFile(filepath.Join(slot, "auth.json"), []byte(`{"tokens":{"access_token":"x"}}`), 0o600)
	a.resume()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.pass()
	if _, err := os.Stat(a.state("paused")); err == nil || !strings.Contains(a.stdout.String(), "#1 adversary run 3: posted review (approve)\n") {
		t.Errorf("after resume:\n%s", a.stdout.String())
	}
}

// Two faults in a row within one pass pause the swarm at once: the pass
// launches nothing after them.
func TestSwarmApplyFaultsInOnePass(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Personas["architect"] = declare.Persona{Provider: "openai", Model: "gpt-6.1-sol", Effort: "high", CoAuthor: "gpt-6.1-sol <noreply@openai.com>"}
	})
	os.Remove(filepath.Join(a.local, "swarm", "credentials", "openai", "1", "auth.json"))
	for _, title := range []string{"One", "Two"} {
		a.forge.CreateIssue(title, "x", []string{"swarm:ready"})
	}
	a.forge.CreateIssue("Three", "x", []string{"swarm:planned"})
	a.runner.Script(3, "worker", work("v1"))
	a.pass()
	if _, err := os.Stat(a.state("paused")); err != nil || len(a.runner.Launches()) != 0 {
		t.Errorf("paused %v, launched %d\n%s", err == nil, len(a.runner.Launches()), a.stdout.String())
	}
}

// The batch trigger (section 6, G0): with the triage gate on it waits for
// no proposed issue; off, it tops the proposed and ready issues up to
// owner.max_open_proposals; a batch that writes BLOCKED:, proposes
// nothing or fails holds the batch, and a fault does not.
func TestSwarmApplyBatchTrigger(t *testing.T) {
	t.Run("pacing, triage off", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) {
			c.Gates.Triage = false
			c.Owner.MaxOpenProposals = 3
			c.Budget = &declare.Budget{PerDay: 10, Total: 100}
		})
		a.forge.CreateIssue("Old", "x", []string{"swarm:ready"})
		a.runner.Others = []runner.Running{{Name: "daisy-swarm.acme.app.1.architect.9", Swarm: "acme/app#1/architect"}} // #1 stays ready
		a.release()
		a.runner.Script(0, "owner", runner.Canned{Out: "## One\nA.\n## Two\nB.\n## Three\nC.\n"})
		a.pass()
		if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, "Propose up to 2 new jobs for acme/app, most important first, from the resources and the code.") {
			t.Errorf("batch Task:\n%s", sys)
		}
		ready, _ := a.forge.Issues(forge.Open, "swarm:ready")
		if len(ready) != 3 || ready[2].Title != "Two" {
			t.Errorf("ready: %+v", ready)
		}
		a.pass()
		if _, err := os.Stat(a.state("runs/0-owner-2")); err == nil {
			t.Error("the batch ran with three ready")
		}
	})
	for _, tc := range []struct {
		name, held string
		canned     runner.Canned
	}{
		{"blocked", "blocked: nothing left in the brief", runner.Canned{Out: "Met: docs/briefs/a.md\nBLOCKED: nothing left in the brief\n"}},
		{"blocked with no question", "blocked: ", runner.Canned{Out: "BLOCKED:\n"}},
		{"proposed nothing", "proposed nothing", runner.Canned{Out: "Met: docs/briefs/a.md\n"}},
		{"failed", "exit 1", runner.Canned{Out: "## x\n", Code: 1}},
		{"fault", "", runner.Canned{Code: 125}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.release()
			a.runner.Script(0, "owner", tc.canned)
			a.pass()
			held, err := os.ReadFile(a.state("owner-held"))
			if tc.held == "" {
				if err == nil {
					t.Errorf("a fault held the batch: %s", held)
				}
				return
			}
			if string(held) != "held 2026-11-06T15:01:00Z: "+tc.held+"\n" {
				t.Errorf("owner-held: %q", held)
			}
			if is, _ := a.forge.Issues(forge.Open, "swarm:proposed"); len(is) != 0 {
				t.Errorf("created %+v", is)
			}
			a.pass()
			if _, err := os.Stat(a.state("runs/0-owner-2")); err == nil {
				t.Error("a held batch ran")
			}
			a.resume()
			if !strings.Contains(a.stdout.String(), "Owner batch released\n") {
				t.Errorf("resume: %s", a.stdout.String())
			}
		})
	}
}

// Runs another apply started (section 7.7): one with no container is
// abandoned; one whose container has outlived timeout is stopped, then
// abandoned; one within its time is left running.
func TestSwarmApplyAbandonedAndOrphans(t *testing.T) {
	a := newApplyWS(t)
	for run, started := range map[string]string{"1-architect-1": "2026-11-06T14:59:00Z", "2-architect-1": "2026-11-06T14:00:00Z", "3-architect-1": "2026-11-06T14:59:00Z"} {
		os.MkdirAll(a.state("runs/"+run+"/work"), 0o755)
		os.WriteFile(a.state("runs/"+run+"/result"), []byte("label swarm:ready\nstarted "+started+"\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\n"), 0o644)
	}
	a.runner.Others = []runner.Running{{Name: "daisy-swarm.acme.app.2.architect.1", Swarm: "acme/app#2/architect"}, {Name: "daisy-swarm.acme.app.3.architect.1", Swarm: "acme/app#3/architect"}}
	a.pass()
	if !reflect.DeepEqual(a.runner.Stopped, []string{"daisy-swarm.acme.app.2.architect.1"}) {
		t.Errorf("stopped %v", a.runner.Stopped)
	}
	for run, abandoned := range map[string]bool{"1-architect-1": true, "2-architect-1": true, "3-architect-1": false} {
		if got := strings.HasSuffix(a.result(run), "outcome abandoned\ncollected\n"); got != abandoned {
			t.Errorf("%s:\n%s", run, a.result(run))
		}
	}
}
