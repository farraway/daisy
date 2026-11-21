package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// gridappFile is gridapp's test services (request F29): MongoDB on tmpfs
// and Redis, and the two variables its tests read.
const gridappFile = `services:
  mongo: { image: "mongo:7", tmpfs: [/data/db] }
  redis: { image: "redis:7" }
x-daisy-env:
  GRIDAPP_TEST_MONGO_URI: mongodb://mongo:27017
  GRIDAPP_TEST_REDIS_URI: redis://redis:6379
`

var (
	gridappList = []runner.Service{{Name: "mongo", Image: "mongo:7", Tmpfs: []string{"/data/db"}}, {Name: "redis", Image: "redis:7"}}
	gridappEnv  = []string{"GRIDAPP_TEST_MONGO_URI=mongodb://mongo:27017", "GRIDAPP_TEST_REDIS_URI=redis://redis:6379"}
)

const (
	workerNet = "daisy-swarm.acme.swarm-app.1.worker.1"
	checkNet  = "daisy-swarm.acme.swarm-app.1.check.1"
)

// inOrder fails unless r holds each of lines, in order.
func inOrder(t *testing.T, r string, lines ...string) {
	t.Helper()
	at := 0
	for _, l := range lines {
		i := strings.Index(r[at:], l)
		if i < 0 {
			t.Fatalf("result lacks %q after byte %d:\n%s", l, at, r)
		}
		at += i + 1
	}
}

// A product with compose.test.yaml (sections 3, 7.2 and 7.6, step 4):
// the worker runs beside its services on a private network of its own;
// the check, on the head, beside them on an internal network, in place
// of --network none, with x-daisy-env's variables; and once the check
// has ended, failing or timed out, the services and the network are
// gone (the Fake refuses a network's removal while its check runs).
func TestSwarmServicesBesideTheCheck(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		canned runner.Canned
		line   string
		result string
	}{
		{"failing", runner.Canned{Code: 1, Stdout: "--- FAIL: TestImportFixture (0.20s)\nFAIL\tgridapp/cmd/gridapp\n"}, "\ncheck exited 1 ", "exit 1"},
		{"timeout", runner.Canned{Code: 137, TimedOut: true, Stdout: "=== RUN   TestImportFixture\n"}, "\ncheck timeout ", "timeout after 45m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.planned(oneSlice)
			a.land("app", map[string]string{"compose.test.yaml": gridappFile}, "test services")
			a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
			a.runner.Script(1, "check", tc.canned)
			a.pass()
			want := []runner.Services{{Network: workerNet, List: gridappList}, {Network: checkNet, Internal: true, List: gridappList}}
			if !reflect.DeepEqual(a.runner.Ups, want) {
				t.Errorf("ups:\n got %+v\nwant %+v", a.runner.Ups, want)
			}
			if len(a.runner.Networks) != 0 {
				t.Errorf("networks left up: %+v", a.runner.Networks)
			}
			workers, checks := a.launched("worker"), a.launched("check")
			if len(workers) != 1 || workers[0].Network != workerNet || !reflect.DeepEqual(workers[0].Env, gridappEnv) {
				t.Errorf("workers %+v", workers)
			}
			run := a.state("runs/1-worker-1")
			args := []string{"run", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.swarm-app.1.check.1", "--label", "daisy.swarm=acme/swarm-app#1/check",
				"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30", "--network", checkNet,
				"--env", gridappEnv[0], "--env", gridappEnv[1], "--mount", "type=bind,src=" + run + "/check,dst=/job/work/app", "--workdir", "/job/work/app", "daisy-member:1", "make", "check"}
			if len(checks) != 1 || !reflect.DeepEqual(checks[0].Args(), args) {
				t.Errorf("checks %+v", checks)
			}
			inOrder(t, a.result("1-worker-1"), "\nservices worker up mongo redis\n", "\nservices worker down\n", "\nfetched ",
				"\nservices check up mongo redis\n", "\ncheck started ", tc.line, "\nservices check down\n", "\npushed ", "\noutcome posted\n")
			ps := a.pulls(1)
			if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": "+tc.result+"\n\n```\n") {
				t.Errorf("pulls %+v", ps)
			}
		})
	}
}

// Without a services file, nothing of this: no network, no service, the
// worker launched on Docker's default network without variables, the
// check offline as before (its argv is TestSwarmCheckWithoutDepsUnchanged's),
// and the reviewers launched as before Step R3-3, byte for byte.
func TestSwarmNoServicesFile(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.pass()
	workers, checks := a.launched("worker"), a.launched("check")
	if len(a.runner.Ups) != 0 || len(workers) != 1 || workers[0].Network != "" || workers[0].Env != nil || len(checks) != 1 || checks[0].Network != "none" ||
		strings.Contains(a.result("1-worker-1"), "services") {
		t.Errorf("ups %+v, workers %+v, checks %+v\n%s", a.runner.Ups, workers, checks, a.result("1-worker-1"))
	}
	if argv := a.file("1-worker-1", "argv"); strings.Contains(argv, "--network") || strings.Contains(argv, "--env\n") {
		t.Errorf("the worker's argv:\n%s", argv)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	// The reviewers', without services or env, as before R3-3, byte for
	// byte to their image: the adversary's slot, the sympathetic
	// reviewer's env credential, and no --network or --env of their own.
	slot := filepath.Join(a.local, "swarm", "credentials", "openai", "1")
	for _, tc := range []struct{ role, credential string }{
		{"adversary", "--label\ndaisy.credential=openai/1\n--mount\ntype=bind,src=" + slot + ",dst=/credential\n--env\nCODEX_HOME=/credential\n"},
		{"sympathetic", "--env-file\n/dev/stdin\n"},
	} {
		run, jobs := a.state("runs/1-"+tc.role+"-1"), a.state("jobs/1")
		want := "docker\nrun\n--rm\n--pull\nnever\n--name\ndaisy-swarm.acme.swarm-app.1." + tc.role + ".1\n--label\ndaisy.swarm=acme/swarm-app#1/" + tc.role +
			"\n--user\n1000:1000\n--security-opt\nno-new-privileges\n--stop-timeout\n30\n" + tc.credential +
			"--mount\ntype=bind,src=" + run + "/job,dst=/job\n--mount\ntype=bind,src=" + run + "/work,dst=/job/work\n--mount\ntype=bind,src=" + run + "/io,dst=/job/io\n" +
			"--mount\ntype=bind,src=" + run + "/resources,dst=/job/resources,readonly\n--mount\ntype=bind,src=" + jobs + ",dst=/job/memory,readonly\n" +
			"--workdir\n/job/work/app\ndaisy-member:1\nsh\n-c\n"
		if argv := a.file("1-"+tc.role+"-1", "argv"); !strings.HasPrefix(argv, want) {
			t.Errorf("the %s's argv:\n%s\nwant it to begin:\n%s", tc.role, argv, want)
		}
		if r := a.result("1-" + tc.role + "-1"); strings.Contains(r, "services") {
			t.Errorf("the %s's result:\n%s", tc.role, r)
		}
	}
}

// A file outside the subset (section 3) runs nothing of it: the worker,
// on main's, is launched without services and says so on stderr; the
// check, on the head's, does not run, its result the refusal, posted as
// a check's for the reviewers. A file the repository's services field
// names is read in place of compose.test.yaml.
func TestSwarmServicesRefused(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	refused := "services:\n  mongo:\n    image: mongo:7\n    ports: ['27017:27017']\n"
	a.land("app", map[string]string{"test/compose.yaml": refused, "compose.test.yaml": gridappFile}, "test services")
	file := "test/compose.yaml"
	a.edit(func(c *declare.Config) { c.Repos[0].Services = &file })
	a.publish("services")
	a.fetchMirror()
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.pass()
	reason := "test/compose.yaml: services.mongo.ports: not in the subset Daisy runs (image, tmpfs, ulimits, environment, command)"
	workers := a.launched("worker")
	if len(a.runner.Ups) != 0 || len(a.launched("check")) != 0 || len(workers) != 1 || workers[0].Network != "" || workers[0].Env != nil {
		t.Fatalf("ups %+v, checks %d, workers %+v", a.runner.Ups, len(a.launched("check")), workers)
	}
	if want := "⚠ #1 worker run 1: test services refused (" + reason + "): launched without them\n"; !strings.Contains(a.stderr.String(), want) {
		t.Errorf("stderr lacks %q:\n%s", want, a.stderr.String())
	}
	inOrder(t, a.result("1-worker-1"), "\nservices worker refused: "+reason+"\n", "\nfetched ", "\nservices check refused: "+reason+"\n", "\npushed ", "\noutcome posted\n")
	ps := a.pulls(1)
	if len(ps) != 1 || !strings.Contains(ps[0].Body, "**Check:** `make check` on "+ps[0].HeadSHA[:7]+": test services refused; the check did not run\n\n```\n"+reason+"\n```") {
		t.Errorf("pulls %+v", ps)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if p := a.file("1-adversary-1", "io/prompt.md"); !strings.Contains(p, "# Check\n\nmake check on "+ps[0].HeadSHA+": test services refused; the check did not run\n\n    "+reason) {
		t.Errorf("the reviewer's # Check:\n%s", p)
	}
	// Each reviewer, on the head's file, is launched without services
	// and warned of it, as the worker is (section 7.9, Beside a reviewer).
	for _, role := range []string{"adversary", "sympathetic"} {
		rs := a.launched(role)
		if len(a.runner.Ups) != 0 || len(rs) != 1 || rs[0].Network != "" || rs[0].Env != nil {
			t.Errorf("ups %+v, %s %+v", a.runner.Ups, role, rs)
		}
		if want := "⚠ #1 " + role + " run 1: test services refused (" + reason + "): launched without them\n"; !strings.Contains(a.stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, a.stderr.String())
		}
		inOrder(t, a.result("1-"+role+"-1"), "\nservices "+role+" refused: "+reason+"\n", "\nstarted ", "\noutcome posted\n")
	}
}

// Services that cannot start (an image not on the host: --pull never)
// are a swarm fault, named as the services' (section 7.7): at the
// worker's launch, a launch's; at the check, as a check that cannot
// start; nothing posted, T22, and what was made of them taken down.
func TestSwarmServicesCannotStart(t *testing.T) {
	t.Parallel()
	missing := errors.New("redis: docker: Error response from daemon: No such image: redis:7.")
	t.Run("worker", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.land("app", map[string]string{"compose.test.yaml": gridappFile}, "test services")
		a.runner.UpErr = missing
		a.pass()
		r := a.result("1-worker-1")
		if len(a.launched("worker")) != 0 || len(a.runner.Networks) != 0 || !strings.Contains(r, "\nlaunch-failed ") ||
			!strings.Contains(r, "\nfault: services: "+missing.Error()+"\n") || !strings.Contains(r, "\nservices worker down\n") || !strings.Contains(r, "\noutcome fault\n") ||
			!reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:plan-1-accepted"}) {
			t.Errorf("workers %d, networks %+v, labels %v\n%s", len(a.launched("worker")), a.runner.Networks, labels(a.issue(1)), r)
		}
	})
	t.Run("check", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.UpErr = missing
		a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n", "compose.test.yaml": gridappFile}, "v1"))})
		a.pass()
		r := a.result("1-worker-1")
		if len(a.launched("check")) != 0 || len(a.runner.Ups) != 1 || len(a.runner.Networks) != 0 || !strings.Contains(r, "\nfault: services: "+missing.Error()+"\n") ||
			!strings.Contains(r, "\nservices check down\n") || !strings.Contains(r, "\noutcome fault\n") || len(a.pulls(1)) != 0 ||
			!reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:plan-1-accepted"}) {
			t.Errorf("checks %d, ups %d, networks %+v, labels %v\n%s", len(a.launched("check")), len(a.runner.Ups), a.runner.Networks, labels(a.issue(1)), r)
		}
	})
}

// A check whose apply died left its services up: the next pass takes
// them down before it starts the check again beside new ones, and takes
// those down once it has ended (section 7.6, step 4).
func TestSwarmServicesRestartedAfterTheirApplyDied(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.land("app", map[string]string{"compose.test.yaml": gridappFile}, "test services")
	head := a.orphanWorker("exited 0 2026-11-06T15:10:00Z")
	a.gitRunIn(a.state("git/app"), "fetch", "-q", "--no-tags", a.state("runs/1-worker-1/work"), "+refs/heads/swarm/1-1:refs/swarm-runs/1-worker-1")
	run := a.state("runs/1-worker-1")
	os.WriteFile(filepath.Join(run, "io", "out.md"), []byte("v1\n\nNot established: none\n"), 0o644)
	f, _ := os.OpenFile(filepath.Join(run, "result"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("fetched " + head + "\nservices check up mongo redis\ncheck started 2026-11-06T15:11:00Z\n")
	f.Close()
	a.runner.Networks = map[string]runner.Services{checkNet: {Network: checkNet, Internal: true, List: gridappList}}
	a.pass()
	r := a.result("1-worker-1")
	if len(a.runner.Ups) != 1 || len(a.runner.Networks) != 0 || len(a.launched("check")) != 1 || strings.Count(r, "\nservices check down\n") != 2 {
		t.Fatalf("ups %d, networks %+v, checks %d\n%s", len(a.runner.Ups), a.runner.Networks, len(a.launched("check")), r)
	}
	inOrder(t, r, "\ncheck started ", "\nservices check down\n", "\nservices check up mongo redis\n", "\ncheck started ", "\ncheck exited 0 ", "\nservices check down\n", "\npushed "+head+"\n")
}

// reviewerNet is the network of issue 1's reviewer run k (section 7.9,
// Beside a reviewer).
func reviewerNet(role string, k int) string {
	return fmt.Sprintf("daisy-swarm.acme.swarm-app.1.%s.%d", role, k)
}

// A reviewer's run (sections 7.2, 7.6, step 4, and 7.9, Beside a
// reviewer): each of the pair, launched in one pass, runs beside the
// test services file read at the head it reviews (main holds none: the
// worker's commit adds it, so worker run 1 has none), on a network of
// its own, not internal, named as its container, given the
// repository's env then x-daisy-env's, as the worker is; the
// collection takes each down. Round 2's pair, after changes were
// requested, runs on networks of its own again, beside worker run 2's
// and its check's, and nothing is left up.
func TestSwarmServicesBesideAReviewer(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Repos[0].Env = map[string]string{"GRIDAPP_TEST_REDIS_URI": "redis://elsewhere:6379", "CGO_ENABLED": "0"}
	})
	a.publish("env")
	a.fetchMirror()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n", "compose.test.yaml": gridappFile}, "v1"))},
		runner.Canned{Out: "v2\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v2\n"}, "v2"))})
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve}, runner.Canned{Out: approve})
	a.pass() // worker run 1 and its check
	a.pass() // the pair, round 1
	a.pass() // worker run 2 and its check
	a.pass() // the pair, round 2
	if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-2-approved") {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	net := func(role string, k int) runner.Services {
		return runner.Services{Network: fmt.Sprintf("daisy-swarm.acme.swarm-app.1.%s.%d", role, k), Internal: role == "check", List: gridappList}
	}
	want := []runner.Services{net("check", 1), net("adversary", 1), net("sympathetic", 1), net("worker", 2), net("check", 2), net("adversary", 2), net("sympathetic", 2)}
	if !reflect.DeepEqual(a.runner.Ups, want) {
		t.Errorf("ups:\n got %+v\nwant %+v", a.runner.Ups, want)
	}
	if len(a.runner.Networks) != 0 {
		t.Errorf("networks left up: %+v", a.runner.Networks)
	}
	env := append([]string{"CGO_ENABLED=0", "GRIDAPP_TEST_REDIS_URI=redis://elsewhere:6379"}, gridappEnv...)
	var flags []string
	for _, v := range env {
		flags = append(flags, "--env", v)
	}
	for _, role := range []string{"adversary", "sympathetic"} {
		rs := a.launched(role)
		if len(rs) != 2 {
			t.Fatalf("%d %s runs", len(rs), role)
		}
		for i, r := range rs {
			k := i + 1
			if r.Network != reviewerNet(role, k) || !reflect.DeepEqual(r.Env, env) {
				t.Errorf("%s run %d: network %q, env %q", role, k, r.Network, r.Env)
			}
			argv := a.file(fmt.Sprintf("1-%s-%d", role, k), "argv")
			if !strings.Contains(argv, "\n--stop-timeout\n30\n--network\n"+reviewerNet(role, k)+"\n") || !strings.Contains(argv, "\n"+strings.Join(flags, "\n")+"\n--mount\n") {
				t.Errorf("%s run %d's argv:\n%s", role, k, argv)
			}
			inOrder(t, a.result(fmt.Sprintf("1-%s-%d", role, k)), "\nstarted ", "\nservices "+role+" up mongo redis\n", "\nexited 0 ", "\nservices "+role+" down\n", "\noutcome posted\n")
		}
	}
	if !strings.Contains(a.stdout.String(), "#1 sympathetic run 2: services sympathetic up mongo redis\n") ||
		!strings.Contains(a.stdout.String(), "#1 sympathetic run 2: services sympathetic down\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
}

// A reviewer's services are removed whatever ends its run (sections 7.6,
// step 4, and 7.7): a run whose apply died after they were up (the
// pair's second, abandoned at the next pass), a launch refused once they
// were up, and services that cannot start, the launch's fault; each a
// fault or abandoned, the issue taken home, nothing left up.
func TestSwarmServicesOfAReviewerRemoved(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) *applyWS {
		a := newApplyWS(t)
		a.land("app", map[string]string{"compose.test.yaml": gridappFile}, "test services")
		a.inReview(oneSlice)
		if len(a.runner.Networks) != 0 {
			t.Fatalf("networks up before the review: %+v", a.runner.Networks)
		}
		return a
	}
	home := func(t *testing.T, a *applyWS) {
		t.Helper()
		if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-0-ready") || slices.Contains(got, "swarm:review-1-in-progress") {
			t.Errorf("labels %v", got)
		}
		if len(a.runner.Networks) != 0 {
			t.Errorf("networks left up: %+v", a.runner.Networks)
		}
	}
	t.Run("abandoned", func(t *testing.T) {
		a := setup(t)
		net := reviewerNet("sympathetic", 1)
		run := a.state("runs/1-sympathetic-1")
		os.MkdirAll(filepath.Join(run, "io"), 0o755)
		lines := []string{"start " + a.pulls(1)[0].HeadSHA, "label swarm:review-1-in-progress", "started 2026-11-06T14:59:00Z", "credential anthropic env CLAUDE_CODE_OAUTH_TOKEN",
			"persona anthropic claude-opus-5-5 high", "round 1", "services sympathetic up mongo redis"}
		os.WriteFile(filepath.Join(run, "result"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		a.forge.SetLabel(issue1, "swarm:review-1-in-progress")
		a.runner.Networks = map[string]runner.Services{net: {Network: net, List: gridappList}}
		if err := a.verb(SwarmPause); err != nil {
			t.Fatal(err)
		}
		a.pass()
		home(t, a)
		inOrder(t, a.result("1-sympathetic-1"), "\nservices sympathetic up mongo redis\n", "\nabandoned ", "\nservices sympathetic down\n", "\noutcome abandoned\ncollected\n")
	})
	t.Run("launch refused", func(t *testing.T) {
		a := setup(t)
		a.runner.StartErr = errors.New("no socket")
		a.pass()
		home(t, a)
		for _, role := range []string{"adversary", "sympathetic"} {
			inOrder(t, a.result("1-"+role+"-1"), "\nservices "+role+" up mongo redis\n", "\nlaunch-failed ", "\nfault: launch: no socket\n", "\nservices "+role+" down\n", "\noutcome fault\ncollected\n")
		}
	})
	t.Run("cannot start", func(t *testing.T) {
		a := setup(t)
		missing := errors.New("redis: docker: Error response from daemon: No such image: redis:7.")
		a.runner.UpErr = missing
		a.pass()
		home(t, a)
		if n := len(a.launched("adversary")) + len(a.launched("sympathetic")); n != 0 {
			t.Errorf("%d reviewers launched", n)
		}
		for _, role := range []string{"adversary", "sympathetic"} {
			inOrder(t, a.result("1-"+role+"-1"), "\nservices "+role+" up mongo redis\n", "\nlaunch-failed ", "\nfault: services: "+missing.Error()+"\n", "\nservices "+role+" down\n", "\noutcome fault\ncollected\n")
		}
	})
}
