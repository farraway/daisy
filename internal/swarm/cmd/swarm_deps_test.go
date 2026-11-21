package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// goDeps is a Go product's dependency step, as section 3's example
// declares it.
var goDeps = &declare.Deps{Run: []string{"sh", "-c", "go mod download && go list -deps -test ./... >/dev/null"},
	Env: map[string]string{"GOMODCACHE": "/deps/go/mod", "GOFLAGS": "-modcacherw"}}

// withDeps declares deps for app and publishes it.
func (a *applyWS) withDeps(d *declare.Deps) {
	a.t.Helper()
	a.edit(func(c *declare.Config) { c.Repos[0].Deps = d })
	a.publish("deps")
	a.fetchMirror()
}

// launched is the specs launched for issue 1's role, in order.
func (a *applyWS) launched(role string) []runner.Spec {
	var out []runner.Spec
	for _, s := range a.runner.Launches() {
		if strings.HasSuffix(s.Label, "#1/"+role) {
			out = append(out, s)
		}
	}
	return out
}

// A repository with deps (sections 3, 7.2 and 7.6, step 4): after the
// worker's run, the dependency step runs on the fetched head with
// Docker's default network, the head's clone read-only and the swarm's
// cache for app, <state>/deps/app, read-write at /deps; then the check
// runs offline with the same cache read-only; both are given deps' env.
// The worker's own container is launched as without deps.
func TestSwarmDepsBeforeTheCheck(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withDeps(goDeps)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.runner.Script(1, "deps", runner.Canned{Stderr: "go: downloading example.net/m v1.0.0\n"})
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
	a.pass()
	run, cache := a.state("runs/1-worker-1"), a.state("deps/app")
	deps, checks, workers := a.launched("deps"), a.launched("check"), a.launched("worker")
	if len(deps) != 1 || len(checks) != 1 || len(workers) != 1 {
		t.Fatalf("launched %d deps, %d checks, %d workers", len(deps), len(checks), len(workers))
	}
	head := []string{"run", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.swarm-app.1.%s.1", "--label", "daisy.swarm=acme/swarm-app#1/%s",
		"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30"}
	args := func(role string, rest ...string) []string {
		var out []string
		for _, a := range head {
			out = append(out, strings.ReplaceAll(a, "%s", role))
		}
		return append(out, rest...)
	}
	env := []string{"--env", "GOFLAGS=-modcacherw", "--env", "GOMODCACHE=/deps/go/mod"}
	want := args("deps", append(env, "--mount", "type=bind,src="+run+"/check,dst=/work,readonly", "--mount", "type=bind,src="+cache+",dst=/deps",
		"--workdir", "/work", "daisy-member:1", "sh", "-c", "go mod download && go list -deps -test ./... >/dev/null")...)
	if got := deps[0].Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("deps:\n got %q\nwant %q", got, want)
	}
	want = args("check", append(append([]string{"--network", "none"}, env...), "--mount", "type=bind,src="+run+"/check,dst=/work",
		"--mount", "type=bind,src="+cache+",dst=/deps,readonly", "--workdir", "/work", "daisy-member:1", "make", "check")...)
	if got := checks[0].Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("check:\n got %q\nwant %q", got, want)
	}
	for _, m := range workers[0].Mounts {
		if m.Dst == "/deps" || m.Src == cache {
			t.Errorf("the worker was given the cache: %+v", m)
		}
	}
	if len(workers[0].Env) != 0 {
		t.Errorf("the worker was given deps' env: %q", workers[0].Env)
	}
	if fi, err := os.Stat(cache); err != nil || !fi.IsDir() {
		t.Errorf("the cache %s: %v", cache, err)
	}
	r := a.result("1-worker-1")
	order := []string{"\nfetched ", "\ndeps started ", "\ndeps exited 0 ", "\ncheck started ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n"}
	at := 0
	for _, l := range order {
		i := strings.Index(r[at:], l)
		if i < 0 {
			t.Fatalf("result lacks %q after byte %d:\n%s", l, at, r)
		}
		at += i
	}
	if got := a.file("1-worker-1", "deps.out"); got != "go: downloading example.net/m v1.0.0\n" {
		t.Errorf("deps.out %q", got)
	}
	ps := a.pulls(1)
	if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": exit 0\n\n```\nok 1 test\n```") {
		t.Errorf("pulls %+v", ps)
	}
	if !strings.Contains(a.stdout.String(), "#1 worker run 1: deps started\n#1 worker run 1: deps exited 0\n#1 worker run 1: check started\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
}

// Without deps, the check is launched exactly as before deps existed
// (section 7.2): its argv byte for byte.
func TestSwarmCheckWithoutDepsUnchanged(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.pass()
	run := a.state("runs/1-worker-1")
	checks := a.launched("check")
	want := []string{"run", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.swarm-app.1.check.1", "--label", "daisy.swarm=acme/swarm-app#1/check",
		"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30", "--network", "none",
		"--mount", "type=bind,src=" + run + "/check,dst=/work", "--workdir", "/work", "daisy-member:1", "make", "check"}
	if len(checks) != 1 || !reflect.DeepEqual(checks[0].Args(), want) {
		t.Errorf("checks %+v", checks)
	}
	if len(a.launched("deps")) != 0 || strings.Contains(a.result("1-worker-1"), "deps") {
		t.Errorf("a dependency step without deps:\n%s", a.result("1-worker-1"))
	}
	if _, err := os.Stat(a.state("deps")); err == nil {
		t.Error("a cache made without deps")
	}
}

// A dependency step that fails is the run's check result, named as the
// dependency step's (sections 7.6 and 7.7): no check runs, the post's
// check block and the reviewers' # Check say so with the tail of
// deps.out, and the run is posted as after a failing check; a timeout
// likewise. One Docker cannot run (125) is a swarm fault: nothing
// posted, T22.
func TestSwarmDepsFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		canned runner.Canned
		line   string
		result string
	}{
		{"exit", runner.Canned{Code: 1, Stderr: "go: example.net/m@v1.0.0: dial tcp: lookup proxy.golang.org: no such host\n"}, "\ndeps exited 1 ", "dependency step exited 1; the check did not run"},
		{"timeout", runner.Canned{Code: 137, TimedOut: true, Stderr: "go: downloading example.net/m v1.0.0\n"}, "\ndeps timeout ", "dependency step timed out after 45m; the check did not run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.withDeps(goDeps)
			a.planned(oneSlice)
			a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
			a.runner.Script(1, "deps", tc.canned)
			a.pass()
			r := a.result("1-worker-1")
			if len(a.launched("check")) != 0 || strings.Contains(r, "check started") || !strings.Contains(r, tc.line) || !strings.Contains(r, "\noutcome posted\n") {
				t.Fatalf("checks %d\n%s", len(a.launched("check")), r)
			}
			tail := strings.TrimSuffix(tc.canned.Stderr, "\n")
			ps := a.pulls(1)
			if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": "+tc.result+"\n\n```\n"+tail+"\n```") {
				t.Fatalf("pulls %+v", ps)
			}
			if got := a.file("1-worker-1", "deps.out"); got != tc.canned.Stderr {
				t.Errorf("deps.out %q", got)
			}
			a.runner.Script(1, "adversary", runner.Canned{Out: approve})
			a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
			a.pass()
			if p := a.file("1-adversary-1", "io/prompt.md"); !strings.Contains(p, "# Check\n\nmake check on "+ps[0].HeadSHA+": "+tc.result+"\n\n    "+tail) {
				t.Errorf("the reviewer's # Check:\n%s", p)
			}
		})
	}
	t.Run("fault", func(t *testing.T) {
		a := newApplyWS(t)
		a.withDeps(goDeps)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
		a.runner.Script(1, "deps", runner.Canned{Code: 125})
		a.pass()
		r := a.result("1-worker-1")
		if len(a.launched("check")) != 0 || !strings.Contains(r, "\nfault: deps: exit 125\n") || !strings.Contains(r, "\noutcome fault\n") ||
			len(a.pulls(1)) != 0 || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:plan-1-accepted"}) {
			t.Errorf("labels %v, pulls %d\n%s", labels(a.issue(1)), len(a.pulls(1)), r)
		}
	})
}

// A dependency step whose apply died, gone with no end line, is started
// again by the next pass, and the check follows it (section 7.6, step 4).
func TestSwarmDepsRestartedAfterItsApplyDied(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withDeps(goDeps)
	a.planned(oneSlice)
	head := a.orphanWorker("exited 0 2026-11-06T15:10:00Z")
	a.gitRunIn(a.state("git/app"), "fetch", "-q", "--no-tags", a.state("runs/1-worker-1/work"), "+refs/heads/swarm/1-1:refs/swarm-runs/1-worker-1")
	run := a.state("runs/1-worker-1")
	os.WriteFile(filepath.Join(run, "io", "out.md"), []byte("v1\n\nNot established: none\n"), 0o644)
	f, _ := os.OpenFile(filepath.Join(run, "result"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("fetched " + head + "\ndeps started 2026-11-06T15:11:00Z\n")
	f.Close()
	a.pass()
	r := a.result("1-worker-1")
	if strings.Count(r, "\ndeps started ") != 2 || !strings.Contains(r, "\ndeps exited 0 ") || !strings.Contains(r, "\ncheck exited 0 ") ||
		!strings.Contains(r, "\npushed "+head+"\n") || len(a.launched("deps")) != 1 || len(a.launched("check")) != 1 {
		t.Errorf("deps %d, checks %d\n%s", len(a.launched("deps")), len(a.launched("check")), r)
	}
}
