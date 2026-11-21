package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// ciWorkflow is a product's CI before it adopts the declared image: a
// lint job, and a job that runs the check's target with make on the
// runner beside a database service (section 7.9, The comparison with CI).
const ciWorkflow = `name: ci
on:
  push:
    branches: [main]
  pull_request:
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make lint
  test:
    runs-on: ubuntu-latest
    services:
      db:
        image: postgres:16
    steps:
      - uses: actions/checkout@v4
      - name: test
        run: make check
`

// ciOnPush is the same CI run on a push alone: no workflow runs on a
// pull request, so nothing is compared.
var ciOnPush = strings.Replace(ciWorkflow, "  pull_request:\n", "", 1)

// pushUp commits files to main in the upstream up, as a merge on the
// product's forge moves its base.
func pushUp(t *testing.T, up string, files map[string]string) {
	t.Helper()
	src := t.TempDir()
	gitIn(t, "", "clone", "-q", up, src)
	gitIn(t, src, "config", "user.name", "T")
	gitIn(t, src, "config", "user.email", "t@example.net")
	if err := commit(files, "ci")(src); err != nil {
		t.Fatal(err)
	}
	gitIn(t, src, "push", "-q", "origin", "main")
}

// fetchProduct brings the declared repository name's mirror up to date
// with its upstream, as a pass's fetch does.
func (a *applyWS) fetchProduct(name string) {
	a.t.Helper()
	if _, errb, err := a.exec(a.state("git/"+name), "git", "fetch", "-q", "--prune", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		a.t.Fatalf("git fetch: %v %s", err, errb)
	}
}

// breakTree removes the tree object of base's commit in the mirror at
// m, so that its commit reads and the commit's tree does not: a mirror
// git cannot read. It returns what puts it back.
func breakTree(t *testing.T, m string) func() {
	t.Helper()
	out, errb, err := ops.Real().Exec(m, "git", "rev-parse", "refs/heads/main^{tree}")
	if err != nil {
		t.Fatalf("rev-parse: %v %s", err, errb)
	}
	tree := strings.TrimSpace(string(out))
	loose := filepath.Join(m, "objects", tree[:2], tree[2:])
	data, err := os.ReadFile(loose)
	if err != nil {
		t.Fatalf("the tree is not a loose object: %v", err)
	}
	if err := os.Remove(loose); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.WriteFile(loose, data, 0o444); err != nil {
			t.Fatal(err)
		}
	}
}

// ciLines is CompareCI's lines for app as swarm.json declares it, over
// the workflow at .gitea/workflows/ci.yml and the test services file
// svcs: what each surface must show.
func (a *applyWS) ciLines(workflow string, svcs *declare.TestServices) []string {
	a.t.Helper()
	return declare.CompareCI("gitea", a.config().Repos[0], svcs, []declare.Workflow{{Path: ".gitea/workflows/ci.yml", Data: []byte(workflow)}})
}

// told is the comparison's lines in out, in order.
func told(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "CI differs from the declared check environment") {
			lines = append(lines, l)
		}
	}
	return lines
}

const ciDiffers = "⚠ app: CI differs from the declared check environment: "

// Step R3-5's claim (plan implementing the 2026-11-19 swarm roadmap):
// the comparison with CI at app's base, as its mirror holds it, is told
// on every surface section 7.9 names, each showing CompareCI's lines and
// writing nothing: apply prints them on stderr at the process's first
// pass and again only when the set changes, reading app's mirror with
// git's read commands alone; notify tells `ci differs app` once; status
// lists it under Waiting for you after `no check` (waiting.ci_differs
// with --json), as connect does; a workflow that runs on no pull request
// tells nothing and takes the item away. The kind's first directory
// holding a workflow is read alone (.gitea's, not .github's broken one).
func TestSwarmCIDiffersTold(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	notified := a.notifying()
	a.pass()
	if got := told(a.stderr.String()); got != nil {
		t.Fatalf("no workflow yet: %q", got)
	}

	pushUp(t, a.upstream("app"), map[string]string{".gitea/workflows/ci.yml": ciWorkflow, ".github/workflows/broken.yml": "on: [\n"})
	a.fetchProduct("app")
	want := a.ciLines(ciWorkflow, nil)
	if !slices.Equal(want, []string{
		ciDiffers + "no dockerfile is declared, so CI does not build the check's image",
		ciDiffers + ".gitea/workflows/ci.yml, job lint runs make lint, which the check does not",
		ciDiffers + ".gitea/workflows/ci.yml, job test: service db is not in the test services file",
	}) {
		t.Fatalf("the fixture's lines: %q", want)
	}
	var mu sync.Mutex
	var verbs []string
	mirror := a.state("git/app")
	a.beforeGit = func(dir string, args []string) {
		if dir == mirror {
			mu.Lock()
			verbs = append(verbs, args[0])
			mu.Unlock()
		}
	}
	for range 2 { // each --once a process of its own: its first pass tells them
		a.pass()
		if got := told(a.stderr.String()); !slices.Equal(got, want) {
			t.Fatalf("apply's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(want, "\n"))
		}
	}
	a.beforeGit = nil
	// A paused pass tells them too.
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.pass()
	os.Remove(a.state("paused"))
	if got := told(a.stderr.String()); !slices.Equal(got, want) || !strings.Contains(a.stdout.String(), "launching nothing") {
		t.Fatalf("a paused pass's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(want, "\n"))
	}
	if !slices.Contains(verbs, "ls-tree") || !slices.Contains(verbs, "cat-file") {
		t.Errorf("app's mirror not read: %q", verbs)
	}
	for _, v := range verbs {
		if v != "rev-parse" && v != "ls-tree" && v != "cat-file" {
			t.Errorf("app's mirror: git %s during the passes (%q)", v, verbs)
		}
	}
	if got := notified(); !reflect.DeepEqual(got, []string{"acme/swarm-app waits for you: ci differs app"}) {
		t.Errorf("notified %q", got)
	}
	if b, _ := os.ReadFile(a.state("notified")); string(b) != "ci differs app\n" {
		t.Errorf("<state>/notified: %q", b)
	}
	// A read git refuses is one warning: nothing told, and notify keeps
	// its record, as for any read it cannot make.
	objects := filepath.Join(mirror, "objects")
	a.beforeGit = func(dir string, args []string) {
		if dir == mirror && args[0] == "ls-tree" {
			os.Rename(objects, objects+".off")
		}
	}
	a.pass()
	a.beforeGit = nil
	os.Rename(objects+".off", objects)
	if got := a.stderr.String(); told(got) != nil || !strings.Contains(got, "⚠ comparing app's CI: git ls-tree ") || !strings.Contains(got, "⚠ notify: comparing app's CI: ") {
		t.Errorf("a refused read:\n%s", got)
	}
	if b, _ := os.ReadFile(a.state("notified")); string(b) != "ci differs app\n" || len(notified()) != 1 {
		t.Errorf("<state>/notified: %q, notified %q", b, notified())
	}
	restore := breakTree(t, mirror)
	if err := a.status(a.forge); err == nil || !strings.Contains(err.Error(), "comparing app's CI: git ls-tree ") {
		t.Errorf("status over a mirror git cannot read: %v\n%s", err, a.stdout.String())
	}
	restore()

	// Lines that change, the repository still differing, are printed and
	// not told again; a mirror with no base yet gives no line, and status
	// reads it.
	a.edit(func(c *declare.Config) { c.Repos[0].Env = map[string]string{"APP_MODE": "test"} })
	a.pass()
	if got := told(a.stderr.String()); !slices.Equal(got, append(slices.Clone(want), ciDiffers+"variable APP_MODE is set by no job that runs the check's targets")) || len(notified()) != 1 {
		t.Errorf("changed lines: %q, notified %q", got, notified())
	}
	a.edit(func(c *declare.Config) { c.Repos[0].Env = nil; c.Base = "trunk" })
	a.pass()
	if got := a.stderr.String(); told(got) != nil || strings.Contains(got, "comparing") {
		t.Errorf("no base in the mirror:\n%s", got)
	}
	if _, line := a.waitingSet(); line != "Waiting for you: nothing" {
		t.Errorf("status with no base: %q", line)
	}
	a.edit(func(c *declare.Config) { c.Base = "main" })
	if _, line := a.waitingSet(); line != "Waiting for you: ci differs app" {
		t.Errorf("status: %q", line)
	}
	a.flags = []string{"--waiting", "--json"}
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), `"ci_differs": [`+"\n"+`      "app"`+"\n") {
		t.Errorf("--waiting --json: %v\n%s", err, a.stdout.String())
	}
	a.flags = []string{"--json"}
	if err := a.readOnly(swarmConnect, a.forge); err != nil || !strings.Contains(a.stdout.String(), `"ci_differs": [`+"\n"+`      "app"`) {
		t.Errorf("connect --json: %v\n%s", err, a.stdout.String())
	}
	a.flags = nil
	// After no check, which names lib with the merge gate off.
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	if _, line := a.waitingSet(); line != "Waiting for you: no check lib; ci differs app" {
		t.Errorf("status: %q", line)
	}
	if err := a.readOnly(swarmConnect, a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nWaiting for you: no check lib; ci differs app\n") {
		t.Errorf("connect: %v\n%s", err, a.stdout.String())
	}
	a.edit(func(c *declare.Config) { c.Gates.Merge = true; c.Budget = nil })

	// The loop tells the lines at its first pass and again only when the
	// set changes: pass 2 the same, pass 3 with a Dockerfile declared.
	a.edit(func(c *declare.Config) { c.Notify = nil })
	a.signals = make(chan os.Signal, 1)
	waits := 0
	var wmu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		wmu.Lock()
		defer wmu.Unlock()
		waits++
		switch waits {
		case 2:
			a.edit(func(c *declare.Config) { c.Repos[0].Dockerfile = new("ci/check.Dockerfile") })
		case 3:
			a.signals <- os.Interrupt
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	before := want
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	a.after, a.signals = nil, nil
	after := a.ciLines(ciWorkflow, nil)
	if !slices.Equal(after, []string{
		ciDiffers + "no workflow job builds the image from ci/check.Dockerfile",
		ciDiffers + "the check runs make check, which no workflow step runs in the declared image",
	}) {
		t.Fatalf("the declared image's lines: %q", after)
	}
	if got := told(a.stderr.String()); !slices.Equal(got, append(slices.Clone(before), after...)) || strings.Count(a.stdout.String(), "Pass ") != 3 {
		t.Errorf("the loop's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(append(before, after...), "\n"))
	}

	// A workflow on a push alone compares nothing: no line, and the item
	// leaves Waiting for you and notify's record.
	notified = a.notifying()
	pushUp(t, a.upstream("app"), map[string]string{".gitea/workflows/ci.yml": ciOnPush})
	a.fetchProduct("app")
	a.pass()
	if got := told(a.stderr.String()); got != nil || notified() != nil {
		t.Errorf("on a push alone: %q, notified %q", got, notified())
	}
	if b, _ := os.ReadFile(a.state("notified")); len(b) != 0 {
		t.Errorf("<state>/notified: %q", b)
	}
	if _, line := a.waitingSet(); line != "Waiting for you: nothing" {
		t.Errorf("status: %q", line)
	}
	pushUp(t, a.upstream("app"), map[string]string{".gitea/workflows/ci.yml": ciWorkflow})
	a.fetchProduct("app")
	a.pass()
	if got := notified(); !reflect.DeepEqual(got, []string{"acme/swarm-app waits for you: ci differs app"}) {
		t.Errorf("back again: notified %q", got)
	}
}

// A gitea product whose workflows are in .github/workflows alone, the
// kind's third directory, has them compared; a .gitea/workflows holding
// no .yml or .yaml file is passed over.
func TestSwarmCIDiffersLaterDirectory(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	pushUp(t, a.upstream("lib"), map[string]string{".github/workflows/ci.yml": ciWorkflow, ".gitea/workflows/README.md": "Not here.\n"})
	a.fetchProduct("lib")
	a.pass()
	want := declare.CompareCI("gitea", a.config().Repos[1], nil, []declare.Workflow{{Path: ".github/workflows/ci.yml", Data: []byte(ciWorkflow)}})
	if len(want) == 0 || want[0] != "⚠ lib: CI differs from the declared check environment: no dockerfile is declared, so CI does not build the check's image" {
		t.Fatalf("the fixture's lines: %q", want)
	}
	if got := told(a.stderr.String()); !slices.Equal(got, want) {
		t.Errorf("apply's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(want, "\n"))
	}
	if _, line := a.waitingSet(); line != "Waiting for you: ci differs lib" {
		t.Errorf("status: %q", line)
	}
}

// The test services file at base is compared with the workflow's
// services; one refused (section 7.9, As built) is one line of its own,
// first, and the comparison is made without the file; with no workflow
// on a pull request, neither is told.
func TestSwarmCIDiffersServicesFile(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	const refused = "services:\n  db:\n    image: [postgres\n"
	_, err := declare.ParseServices([]byte(refused))
	if err == nil {
		t.Fatal("the fixture's services file is read")
	}
	pushUp(t, a.upstream("app"), map[string]string{".gitea/workflows/ci.yml": ciWorkflow, declare.ServicesFile: refused})
	a.fetchProduct("app")
	a.pass()
	want := append([]string{ciDiffers + declare.ServicesFile + ": not compared: " + err.Error()}, a.ciLines(ciWorkflow, nil)...)
	if got := told(a.stderr.String()); !slices.Equal(got, want) {
		t.Errorf("apply's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(want, "\n"))
	}
	if _, line := a.waitingSet(); line != "Waiting for you: ci differs app" {
		t.Errorf("status: %q", line)
	}

	const read = "services:\n  db:\n    image: postgres:16\n"
	svcs, err := declare.ParseServices([]byte(read))
	if err != nil {
		t.Fatal(err)
	}
	pushUp(t, a.upstream("app"), map[string]string{declare.ServicesFile: read})
	a.fetchProduct("app")
	a.pass()
	want = a.ciLines(ciWorkflow, &svcs)
	if got := told(a.stderr.String()); !slices.Equal(got, want) || slices.Contains(got, ciDiffers+".gitea/workflows/ci.yml, job test: service db is not in the test services file") {
		t.Errorf("apply's stderr:\n%s\nwant:\n%s", a.stderr.String(), strings.Join(want, "\n"))
	}

	pushUp(t, a.upstream("app"), map[string]string{".gitea/workflows/ci.yml": ciOnPush, declare.ServicesFile: refused})
	a.fetchProduct("app")
	a.pass()
	if got := told(a.stderr.String()); got != nil {
		t.Errorf("on a push alone: %q", got)
	}
	if _, line := a.waitingSet(); line != "Waiting for you: nothing" {
		t.Errorf("status: %q", line)
	}
}

// init's readiness prints each of the comparison's lines at base as
// `⚠ Readiness: …`, after the repository's own warnings, and exits 0: a
// warning, never a refusal (sections 7.9 and 11).
func TestSwarmInitReadinessCIDiffers(t *testing.T) {
	t.Parallel()
	s := newSwarmWS(t)
	s.vars["CLAUDE_CODE_OAUTH_TOKEN"] = "oauth-token"
	pushUp(t, s.upstreams["git@forge.example.net:acme/app.git"], map[string]string{".gitea/workflows/ci.yml": ciWorkflow})
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	c, _, err := declare.Load(ops.Real().FS(), s.place.Root)
	if err != nil {
		t.Fatal(err)
	}
	lines := declare.CompareCI("gitea", c.Repos[0], nil, []declare.Workflow{{Path: ".gitea/workflows/ci.yml", Data: []byte(ciWorkflow)}})
	if len(lines) == 0 || lines[0] != ciDiffers+"no dockerfile is declared, so CI does not build the check's image" {
		t.Fatalf("the fixture's lines: %q", lines)
	}
	wantErr := "⚠ Readiness: app has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: app: check is empty; no member's work is executed (set it in swarm.json)\n"
	for _, l := range lines {
		wantErr += "⚠ Readiness: " + strings.TrimPrefix(l, "⚠ ") + "\n"
	}
	wantErr += "⚠ Readiness: lib has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: lib: check is empty; no member's work is executed (set it in swarm.json)\n" +
		"⚠ Readiness: trusted holds no maintainer login; no comment of yours reaches a member\n"
	if s.stderr.String() != wantErr {
		t.Errorf("stderr:\n%s\nwant:\n%s", s.stderr.String(), wantErr)
	}

	// A mirror git cannot read is a warning, and init goes on.
	s.cwd = s.ws
	restore := breakTree(t, filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "git", "app"))
	defer restore()
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if got := s.stderr.String(); !strings.Contains(got, "⚠ Readiness: comparing app's CI: git ls-tree ") || strings.Contains(got, "CI differs") ||
		!strings.HasSuffix(got, "⚠ Readiness: lib: check is empty; no member's work is executed (set it in swarm.json)\n"+
			"⚠ Readiness: trusted holds no maintainer login; no comment of yours reaches a member\n") {
		t.Errorf("an unreadable mirror:\n%s", got)
	}
}
