package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// A repository's declared dockerfile is protected by default (section 3,
// repos[].dockerfile; 7.6, step 3; decision 162): a worker's change to it,
// made, deleted, renamed away or created, is the failure "protected path
// changed", named by its path as a protected pattern's match is and once
// when protected matches it too, nothing pushed; a file of the same name
// elsewhere is not it. protected itself is as the file declares it, and
// the worker's Task text names the dockerfile among the protected paths.
func TestDeclaredDockerfileIsProtected(t *testing.T) {
	t.Parallel()
	const df = "images/ci/Dockerfile"
	mv := func(from, to string) func(string) error {
		return in("app", func(work string) error {
			if err := os.Rename(filepath.Join(work, from), filepath.Join(work, to)); err != nil {
				return err
			}
			return commit(map[string]string{"main.go": "v1\n"}, "v1")(work)
		})
	}
	for _, tc := range []struct {
		name      string
		base      string   // a file landed on base before the run
		protected []string // added to app's protected
		none      bool     // app's protected is []
		work      func(string) error
		reason    string // "" when the run is pushed
	}{
		{name: "changed", base: df, work: in("app", commit(map[string]string{"main.go": "v1\n", df: "FROM elsewhere:1\n"}, "v1")),
			reason: "protected path changed: " + df},
		{name: "deleted", base: df, work: in("app", func(work string) error {
			if err := os.Remove(filepath.Join(work, df)); err != nil {
				return err
			}
			return commit(map[string]string{"main.go": "v1\n"}, "v1")(work)
		}), reason: "protected path changed: " + df},
		{name: "renamed away", base: df, work: mv(df, "images/ci/Dockerfile.old"), reason: "protected path changed: " + df},
		{name: "created", work: in("app", commit(map[string]string{"main.go": "v1\n", df: "FROM daisy-member:1\n"}, "v1")),
			reason: "protected path changed: " + df},
		{name: "also a protected pattern's", base: df, protected: []string{"images/", "Dockerfile"},
			work:   in("app", commit(map[string]string{"main.go": "v1\n", df: "FROM elsewhere:1\n"}, "v1")),
			reason: "protected path changed: " + df},
		{name: "beside a protected path", base: df, work: in("app", commit(map[string]string{".gitea/workflows/ci.yml": "on: push\n", "main.go": "v1\n", df: "FROM elsewhere:1\n"}, "v1")),
			reason: "protected path changed: .gitea/workflows/ci.yml, " + df},
		{name: "a file of the same name elsewhere", base: df, work: in("app", commit(map[string]string{"main.go": "v1\n", "Dockerfile": "FROM x:1\n", "images/Dockerfile": "FROM x:1\n", "images/ci/dockerfile": "FROM x:1\n", "images/ci/Dockerfile.dev": "FROM x:1\n"}, "v1"))},
		{name: "protected empty", base: df, none: true, work: in("app", commit(map[string]string{"main.go": "v1\n", df: "FROM elsewhere:1\n"}, "v1")),
			reason: "protected path changed: " + df},
		{name: "renamed in", base: "images/ci/Dockerfile.new", work: mv("images/ci/Dockerfile.new", df), reason: "protected path changed: " + df},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newApplyWS(t)
			var declared []string
			a.edit(func(c *declare.Config) {
				c.Repos[0].Dockerfile = ptr(df)
				c.Repos[0].Protected = append(c.Repos[0].Protected, tc.protected...)
				if tc.none {
					c.Repos[0].Protected = []string{}
				}
				declared = c.Repos[0].Protected
			})
			if tc.base != "" {
				a.land("app", map[string]string{tc.base: "FROM daisy-member:1\n"}, "the image's Dockerfile")
			}
			a.planned(oneSlice)
			a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n\nNot established: none\n", Work: tc.work})
			a.runner.Script(1, "check", runner.Canned{Stdout: "ok\n"})
			a.pass()
			r := a.result("1-worker-1")
			if tc.reason == "" {
				if strings.Contains(r, "failed:") || len(a.pulls(1)) != 1 {
					t.Fatalf("not pushed:\n%s", r)
				}
			} else {
				if !strings.Contains(r, "failed: "+tc.reason+"\n") || len(a.pulls(1)) != 0 {
					t.Fatalf("want failed: %s, %d pull requests:\n%s", tc.reason, len(a.pulls(1)), r)
				}
				if b := a.bodies(1); !strings.Contains(b[len(b)-1], "worker run 1 failed: "+tc.reason+" (at swarm:plan-1-accepted).") {
					t.Errorf("the note: %q", b[len(b)-1])
				}
			}
			if got := a.Cfg().Repos[0].Protected; strings.Join(got, ",") != strings.Join(declared, ",") || strings.Contains(strings.Join(got, ","), df) {
				t.Errorf("protected changed: %q", got)
			}
			want := "These paths are protected: " + strings.Join(append(declared, df), ", ") + "."
			if tc.protected != nil {
				want = "These paths are protected: " + strings.Join(declared, ", ") + "."
			}
			if tc.none {
				want = "These paths are protected: images/ci/Dockerfile."
			}
			if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, want) {
				t.Errorf("the Task text lacks %q:\n%s", want, sys)
			}
			// The architect's names each repository's, as <name>: <paths>.
			want = "These paths are protected: app: " + strings.TrimSuffix(strings.TrimPrefix(want, "These paths are protected: "), ".") + ";"
			if sys := a.file("1-architect-1", "io/system.md"); !strings.Contains(sys, want) {
				t.Errorf("the architect's Task text lacks %q:\n%s", want, sys)
			}
		})
	}
}
