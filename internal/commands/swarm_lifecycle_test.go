package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// gitForge is the Fake forge whose open pull requests' heads follow their
// branches in the upstreams, as a push moves them on the real forge.
type gitForge struct {
	*forge.Fake
	a *applyWS
}

func (g gitForge) Pulls(n int) ([]forge.Pull, error) {
	ps, err := g.Fake.Pulls(n)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if !p.Open {
			continue
		}
		out, _, err := ops.Real().Exec(g.a.upstream(p.Repo), "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+p.Head)
		if sha := strings.TrimSpace(string(out)); err == nil && sha != p.HeadSHA {
			g.Fake.Push(forge.Ref{Repo: p.Repo, N: p.Number}, sha)
		}
	}
	return g.Fake.Pulls(n)
}

func (a *applyWS) upstream(name string) string {
	return a.upstreams["git@forge.example.net:acme/"+name+".git"]
}

func (a *applyWS) git() gitForge { return gitForge{a.forge, a} }

// pass is one `apply --once` a minute after the last, which must exit 0.
func (a *applyWS) pass() {
	a.t.Helper()
	a.tick()
	if err := a.apply(a.git(), "--once"); err != nil {
		a.t.Fatalf("%v\nstdout:\n%s\nstderr:\n%s", err, a.stdout.String(), a.stderr.String())
	}
}

func (a *applyWS) issue(n int) forge.Issue {
	a.t.Helper()
	is, err := a.forge.Issue(n)
	if err != nil {
		a.t.Fatal(err)
	}
	return is
}

func (a *applyWS) pulls(n int) []forge.Pull {
	a.t.Helper()
	ps, err := a.git().Pulls(n)
	if err != nil {
		a.t.Fatal(err)
	}
	return ps
}

func (a *applyWS) file(run, rel string) string {
	b, _ := os.ReadFile(a.state(filepath.Join("runs", run, rel)))
	return string(b)
}

func (a *applyWS) release() { os.Remove(a.state("owner-held")) }

// commit is a worker's work: files written and committed in its clone,
// through the clone's own hook unless args say --no-verify.
func commit(files map[string]string, msg string, args ...string) func(string) error {
	return func(work string) error {
		for f, c := range files {
			os.MkdirAll(filepath.Dir(filepath.Join(work, f)), 0o755)
			if err := os.WriteFile(filepath.Join(work, f), []byte(c), 0o644); err != nil {
				return err
			}
		}
		for _, git := range [][]string{{"add", "-A"}, append([]string{"commit", "-q", "-m", msg}, args...)} {
			if _, errb, err := ops.Real().Exec(work, "git", git...); err != nil {
				return &execErr{git, string(errb)}
			}
		}
		return nil
	}
}

type execErr struct {
	args []string
	errb string
}

func (e *execErr) Error() string { return strings.Join(e.args, " ") + ": " + e.errb }

// labels is the issue's labels with the class flag and then the
// maintainer's flag, when it carries them, last: a test's expected labels
// say where the flags are.
func labels(is forge.Issue) []string {
	out := slices.DeleteFunc(slices.Clone(is.Labels), func(l string) bool { return l == forge.Gated || l == forge.ClassArchitecture })
	for _, l := range []string{forge.ClassArchitecture, forge.Gated} {
		if slices.Contains(is.Labels, l) {
			out = append(out, l)
		}
	}
	return out
}

func mark(role string, k int) string {
	return "<!-- daisy-swarm role=" + role + " run=" + strconv.Itoa(k) + " -->\n**" + role + ":**\n\n"
}

const twoSlices = `## Research

- /work/app/main.go: the entry point.

## Steps

1. app: change main.go to v1; the test proves it.
2. app: add notes.md.

## Test Plan

1. main.go says v1.
`

const (
	approve        = "VERDICT: approve\n\nFine.\n"
	requestChanges = "VERDICT: request-changes\n\n[blocking] main.go:1 says v1; the plan wants v2.\n"
)

// The lifecycle with every gate on (docs/swarm.md section 13, Step 29's
// first row): the batch over two active briefs proposes #1 with its
// Brief: line (T1), the maintainer triages it (T3), the architect plans
// (T5), the maintainer accepts (T7), the worker pushes swarm/1-1 and opens
// app#2 with Refs #1 after its check (T8), the issue at
// plan-2-in-progress from the worker's launch to its collection, the pair
// reviews in two rounds (T10, T12, T9), both reviewers launched in one
// pass and the issue at review-1-in-progress, written once, from the
// first one's launch until the pair completes, the
// maintainer merges and relabels plan-1-accepted (T13), and the next slice
// starts on swarm/1-2. The maintainer's flag rides along: on at the gated
// stages, proposal-posted, plan-0-posted and review-2-approved, and cleared
// by the pass that moves the issue on from each.
func TestSwarmApplyLifecycle(t *testing.T) {
	a := newApplyWS(t)
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nThe first priority.\n\n## M1 The drift\n\nA persona sees no drift.\n")
	a.write("docs/briefs/b.md", "# Old\n\nStatus: active\nSecond.\n\n## M1 Done\n")
	a.write("docs/briefs/c.md", "# Gone\n\nStatus: done\n\n## M1\n")
	a.release()
	ref := issue1
	a.runner.Script(0, "owner", runner.Canned{Out: "Met: docs/briefs/b.md\n\n## Drift check\nBrief: docs/briefs/a.md — M1 The drift\nCheck the drift.\n", Stdout: claudeResult})
	a.pass()
	is := a.issue(1)
	if is.Title != "Drift check" || is.Body != mark("owner", 1)+"Brief: docs/briefs/a.md — M1 The drift\nCheck the drift." || is.Author != "swarm-bot" ||
		!reflect.DeepEqual(labels(is), []string{"swarm:proposal-posted", forge.ClassArchitecture, forge.Gated}) { // unclassed: architecture
		t.Fatalf("T1: %+v", is)
	}
	if !strings.Contains(a.stdout.String(), "owner run 1: launched\nowner run 1: proposed #1 Drift check\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
	prompt := a.file("0-owner-1", "io/prompt.md")
	if !strings.Contains(prompt, "# Briefs\n\n## docs/briefs/a.md (Status: active; mounted at /work/.resources/docs/briefs/a.md)\n") ||
		!strings.Contains(prompt, "## docs/briefs/b.md (Status: active") || strings.Contains(prompt, "docs/briefs/c.md") {
		t.Errorf("batch prompt:\n%s", prompt)
	}
	if !strings.Contains(a.file("0-owner-1", "io/out.md"), "Met: docs/briefs/b.md") || strings.Contains(a.result("0-owner-1"), "creating 2") {
		t.Errorf("Met: line: %s", a.result("0-owner-1"))
	}

	a.pass() // G0: #1 is proposed, so no batch; the owner has nothing to answer
	if n := len(a.runner.Launches()); n != 1 {
		t.Fatalf("pass 2 launched %d", n)
	}

	a.forge.SetLabel(ref, "swarm:proposal-accepted") // T3
	a.runner.Script(1, "architect", runner.Canned{Out: twoSlices})
	a.pass()
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:plan-0-posted", forge.ClassArchitecture, forge.Gated}) {
		t.Fatalf("T5: %v", labels(a.issue(1)))
	}

	a.forge.SetLabel(ref, "swarm:plan-1-accepted") // T7
	a.runner.Script(1, "worker", runner.Canned{Out: "Slice 1: main.go says v1.\n\nNot established: none\n", Work: commit(map[string]string{"main.go": "v1\n"}, "Say v1")})
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
	a.pass()
	ps := a.pulls(1)
	if len(ps) != 1 {
		t.Fatalf("T8: pulls %+v", ps)
	}
	pr := ps[0]
	wantTail := "\n\n**Check:** `make check` on " + pr.HeadSHA[:7] + ": exit 0\n\n```\nok 1 test\n```\n\nRefs #1"
	if pr.Repo != "app" || pr.Number != 2 || pr.Head != "swarm/1-1" || pr.Title != "#1 slice 1: Drift check" ||
		pr.Body != mark("worker", 1)+"Slice 1: main.go says v1.\n\nNot established: none"+wantTail {
		t.Errorf("T8 pull: %+v", pr)
	}
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-0-ready", forge.ClassArchitecture}) {
		t.Errorf("T8: %v", labels(a.issue(1)))
	}
	msg, _, _ := ops.Real().Exec(a.upstream("app"), "git", "log", "-1", "--format=%an <%ae>%n%B", "swarm/1-1")
	if !strings.Contains(string(msg), "worker (Daisy swarm) <swarm@example.net>\nSay v1\n\nCo-Authored-By: claude-sonnet-5-5 <noreply@anthropic.com>") {
		t.Errorf("the pushed commit:\n%s", msg)
	}
	launches := a.runner.Launches()
	check := launches[len(launches)-1]
	run := a.state("runs/1-worker-1")
	if check.Name != "daisy-swarm.acme.app.1.check.1" || check.Label != "acme/app#1/check" || !check.NoNetwork || check.EnvVar != "" || check.Credential != "" ||
		!reflect.DeepEqual(check.Mounts, []runner.Mount{{Src: run + "/check", Dst: "/work"}}) || !reflect.DeepEqual(check.Argv, []string{"make", "check"}) || check.Workdir != "/work" {
		t.Errorf("check spec %+v", check)
	}
	for _, l := range []string{"branch swarm/1-1 in app", "label swarm:plan-1-accepted", "check started ", "check exited 0 ", "pushed " + pr.HeadSHA, "relabelled swarm:plan-1-accepted swarm:plan-2-in-progress", "created pull 2", "relabelled swarm:plan-2-in-progress swarm:review-0-ready", "outcome posted"} {
		if !strings.Contains(a.result("1-worker-1"), "\n"+l) {
			t.Errorf("worker result lacks %q:\n%s", l, a.result("1-worker-1"))
		}
	}
	if _, err := os.Stat(run + "/check"); err == nil {
		t.Error("check/ kept after the collection")
	}
	if !strings.Contains(a.stdout.String(), "#1 worker run 1: launched\n#1 worker run 1: swarm:plan-1-accepted → swarm:plan-2-in-progress\n") ||
		!strings.Contains(a.stdout.String(), "#1 worker run 1: swarm:plan-2-in-progress → swarm:review-0-ready\n") {
		t.Errorf("T8 through plan-2-in-progress:\n%s", a.stdout.String())
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve}, runner.Canned{Out: approve})
	before := len(a.runner.Launches())
	a.pass() // the pair, round 1, in one pass: T21 by the first launched, then T10
	var roles []string
	for _, l := range a.runner.Launches()[before:] {
		roles = append(roles, l.Label)
	}
	if !reflect.DeepEqual(roles, []string{"acme/app#1/adversary", "acme/app#1/sympathetic"}) {
		t.Errorf("round 1's pass launched %q", roles)
	}
	pr = a.pulls(1)[0]
	if !slices.Contains(pr.Labels, "swarm:review-3-changes-requested") { // the issue's as the pass left it, not a pass behind (section 5)
		t.Errorf("the pull request's label: %v", pr.Labels)
	}
	if sys := a.file("1-adversary-1", "io/system.md"); !strings.Contains(sys, "check that its test exists on this head and that it fails against main's code") {
		t.Errorf("adversary's Task:\n%s", sys)
	}
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-3-changes-requested", forge.ClassArchitecture}) || !strings.Contains(a.result("1-adversary-1"), "\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n") ||
		!strings.Contains(a.result("1-sympathetic-1"), "\nlabel swarm:review-1-in-progress\n") || strings.Contains(a.result("1-sympathetic-1"), "relabelled swarm:review-0-ready") ||
		!strings.Contains(a.result("1-sympathetic-1"), "\nrelabelled swarm:review-1-in-progress swarm:review-3-changes-requested\n") {
		t.Fatalf("T21 once, then T10: %v\n%s\n%s", labels(a.issue(1)), a.result("1-adversary-1"), a.result("1-sympathetic-1"))
	}
	if n := slices.Index(a.forge.Events(ref), forge.Event{Actor: "swarm-bot", What: "labeled swarm:review-1-in-progress"}); n < 0 ||
		slices.Contains(a.forge.Events(ref)[n+1:], forge.Event{Actor: "swarm-bot", What: "labeled swarm:review-1-in-progress"}) {
		t.Errorf("review-1-in-progress written other than once: %v", a.forge.Events(ref))
	}
	if sys := a.file("1-sympathetic-1", "io/prompt.md"); strings.Contains(sys, "says v1; the plan wants v2") {
		t.Errorf("the sympathetic reviewer read the adversary's review of its head:\n%s", sys)
	}

	a.runner.Script(1, "worker", runner.Canned{Out: "Now v2.\n\nNot established: none\n", Work: commit(map[string]string{"main.go": "v2\n"}, "Say v2")})
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 2 tests\n"})
	a.pass() // T12
	pr = a.pulls(1)[0]
	cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 2})
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-0-ready", forge.ClassArchitecture}) || !strings.Contains(a.result("1-worker-2"), "\nrelabelled swarm:review-3-changes-requested swarm:plan-2-in-progress\n") ||
		len(cs) != 1 || !strings.HasPrefix(cs[0].Body, mark("worker", 2)+"Now v2.") ||
		!strings.Contains(cs[0].Body, "**Check:** `make check` on "+pr.HeadSHA[:7]+": exit 0") {
		t.Fatalf("T12: %v %+v", labels(a.issue(1)), cs)
	}
	a.pass() // the pair, round 2: T9
	if sys := a.file("1-adversary-2", "io/system.md"); !strings.Contains(sys, "This is round 2.") {
		t.Errorf("round 2's Task:\n%s", sys)
	}
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-2-approved", forge.ClassArchitecture, forge.Gated}) {
		t.Fatalf("T9: %v", labels(a.issue(1)))
	}
	a.pass() // approved runs nobody
	on1 := 0
	for _, l := range a.runner.Launches() {
		if strings.HasPrefix(l.Label, "acme/app#1/") {
			on1++
		}
	}
	if on1 != 9 { // the architect, two workers with their checks, two pairs
		t.Errorf("approved launched: %d launches on #1", on1)
	}

	pr = a.pulls(1)[0]
	if err := a.forge.Merge(forge.Ref{Repo: "app", N: 2}, pr.HeadSHA); err != nil { // the maintainer merges
		t.Fatal(err)
	}
	a.forge.SetLabel(ref, "swarm:plan-1-accepted") // T13
	a.runner.Script(1, "worker", runner.Canned{Out: "Notes.\n\nNot established: none\n", Work: commit(map[string]string{"notes.md": "n\n"}, "Add notes")})
	a.pass()
	ps = a.pulls(1)
	merged := a.pulls(1)[0]
	if len(ps) != 2 || ps[1].Head != "swarm/1-2" || ps[1].Number != 3 || ps[1].Title != "#1 slice 2: Drift check" {
		t.Fatalf("slice 2: %+v", ps)
	}
	if !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-0-ready", forge.ClassArchitecture}) || !strings.Contains(a.stdout.String(), "#1 gated cleared\n") {
		t.Errorf("T13 by hand, then T8: %v\n%s", labels(a.issue(1)), a.stdout.String())
	}
	if p := a.file("1-worker-3", "io/prompt.md"); !strings.Contains(p, "# Earlier pull requests\n\n- app#2 swarm/1-1 merged "+merged.MergeSHA+": #1 slice 1: Drift check\n") {
		t.Errorf("slice 2's prompt:\n%s", p)
	}
}
