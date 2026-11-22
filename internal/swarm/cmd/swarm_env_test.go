package cmd

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// A repository's env (sections 3, 7.2 and 7.9): the worker's run, the
// dependency step and the check are each given its variables, one
// --env per name in name order, before deps.env's (the dependency step
// and the check) and before x-daisy-env's (beside the services), so a
// name in env and x-daisy-env takes the file's, given last. Without env
// each argv is as before (TestSwarmDepsBeforeTheCheck,
// TestSwarmCheckWithoutDepsUnchanged, TestSwarmNoServicesFile).
func TestSwarmEnvGivenToEveryRunOfTheRepository(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Repos[0].Env = map[string]string{"GRIDAPP_TEST_REDIS_URI": "redis://elsewhere:6379", "CGO_ENABLED": "0"}
	})
	a.withDeps(goDeps)
	a.land("app", map[string]string{"compose.test.yaml": gridappFile}, "test services")
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.runner.Script(1, "deps", runner.Canned{})
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
	a.pass()

	env := []string{"CGO_ENABLED=0", "GRIDAPP_TEST_REDIS_URI=redis://elsewhere:6379"}
	deps := []string{"GOFLAGS=-modcacherw", "GOMODCACHE=/deps/go/mod"}
	cat := func(parts ...[]string) []string { return slices.Concat(parts...) }
	flags := func(vars []string) []string {
		var out []string
		for _, v := range vars {
			out = append(out, "--env", v)
		}
		return out
	}
	workers, depSteps, checks := a.launched("worker"), a.launched("deps"), a.launched("check")
	if len(workers) != 1 || len(depSteps) != 1 || len(checks) != 1 {
		t.Fatalf("launched %d workers, %d deps, %d checks", len(workers), len(depSteps), len(checks))
	}
	if got, want := workers[0].Env, cat(env, gridappEnv); !reflect.DeepEqual(got, want) {
		t.Errorf("the worker's env:\n got %q\nwant %q", got, want)
	}
	if argv, want := workers[0].Args(), strings.Join(flags(cat(env, gridappEnv)), "\x00"); !strings.Contains(strings.Join(argv, "\x00"), want) {
		t.Errorf("the worker's argv lacks %q in order:\n%q", want, argv)
	}

	run, cache := a.state("runs/1-worker-1"), a.state("deps/app")
	head := func(role string) []string {
		return []string{"run", "--detach", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.swarm-app.1." + role + ".1", "--label", "daisy.swarm=acme/swarm-app#1/" + role,
			"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30"}
	}
	want := cat(head("deps"), flags(cat(env, deps)), []string{"--mount", "type=bind,src=" + run + "/check,dst=/job/work/app,readonly", "--mount", "type=bind,src=" + cache + ",dst=/deps"},
		stepMount(run), []string{"--workdir", "/job/work/app", "daisy-member:1"}, stepPrefix, []string{"sh", "-c", "go mod download && go list -deps -test ./... >/dev/null"})
	if got := depSteps[0].Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("deps:\n got %q\nwant %q", got, want)
	}
	want = cat(head("check"), []string{"--network", checkNet}, flags(cat(env, deps, gridappEnv)), []string{"--mount", "type=bind,src=" + run + "/check,dst=/job/work/app",
		"--mount", "type=bind,src=" + cache + ",dst=/deps,readonly"}, stepMount(run), []string{"--workdir", "/job/work/app", "daisy-member:1"}, stepPrefix, []string{"make", "check"})
	if got := checks[0].Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("check:\n got %q\nwant %q", got, want)
	}
	inOrder(t, a.result("1-worker-1"), "\nfetched ", "\ndeps exited 0 ", "\ncheck exited 0 ", "\npushed ", "\noutcome posted\n")
}

// Without services, the worker's and the reviewers' runs are given env
// alone, on Docker's default network, and the check env alone, offline
// (section 7.2).
func TestSwarmEnvWithoutDepsOrServices(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Repos[0].Env = map[string]string{"TZ": "UTC"} })
	a.publish("env")
	a.fetchMirror()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "v1\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v1\n"}, "v1"))})
	a.pass()
	workers, checks := a.launched("worker"), a.launched("check")
	if len(workers) != 1 || workers[0].Network != "" || !reflect.DeepEqual(workers[0].Env, []string{"TZ=UTC"}) {
		t.Errorf("workers %+v", workers)
	}
	run := a.state("runs/1-worker-1")
	want := slices.Concat([]string{"run", "--detach", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.swarm-app.1.check.1", "--label", "daisy.swarm=acme/swarm-app#1/check",
		"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30", "--network", "none", "--env", "TZ=UTC",
		"--mount", "type=bind,src=" + run + "/check,dst=/job/work/app"}, stepMount(run), []string{"--workdir", "/job/work/app", "daisy-member:1"}, stepPrefix, []string{"make", "check"})
	if len(checks) != 1 || !reflect.DeepEqual(checks[0].Args(), want) {
		t.Errorf("checks %+v", checks)
	}
	// The reviewers too, on the default network (section 7.9, Beside a
	// reviewer).
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved
	for _, role := range []string{"adversary", "sympathetic"} {
		rs := a.launched(role)
		if len(rs) != 1 || rs[0].Network != "" || !reflect.DeepEqual(rs[0].Env, []string{"TZ=UTC"}) {
			t.Errorf("%s %+v", role, rs)
		}
		after := "--mount\n" // the adversary's slot home after it (section 7.2)
		if role == "adversary" {
			after = "--env\nCODEX_HOME=/credential\n--mount\n"
		}
		if argv := a.file("1-"+role+"-1", "argv"); !strings.Contains(argv, "\n--env\nTZ=UTC\n"+after) || strings.Contains(argv, "--network") {
			t.Errorf("the %s's argv:\n%s", role, argv)
		}
	}
}
