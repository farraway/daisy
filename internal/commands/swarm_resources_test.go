package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// pushFiles commits files (a value "-> <target>" a link to target) to
// the upstream up's main and returns the new head.
func pushFiles(t *testing.T, up string, files map[string]string) string {
	t.Helper()
	tree := t.TempDir()
	gitIn(t, "", "clone", "-q", up, tree)
	for f, c := range files {
		p := filepath.Join(tree, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if target, ok := strings.CutPrefix(c, "-> "); ok {
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, tree, "add", "-A")
	gitIn(t, tree, "commit", "-q", "-m", "files")
	gitIn(t, tree, "push", "-q", "origin", "HEAD:main")
	return gitIn(t, tree, "rev-parse", "HEAD")
}

// newUpstream is a bare repository holding files on main, reached at url
// in place of the forge.
func (a *applyWS) newUpstream(url string, files map[string]string) string {
	a.t.Helper()
	up := filepath.Join(a.t.TempDir(), "up.git")
	src := a.t.TempDir()
	gitIn(a.t, "", "init", "-q", "-b", "main", src)
	gitIn(a.t, src, "commit", "-q", "--allow-empty", "-m", "first")
	gitIn(a.t, "", "clone", "-q", "--bare", src, up)
	a.upstreams[url] = up
	pushFiles(a.t, up, files)
	return up
}

// tree is every file and link under dir, by its path, a link as
// "-> <target>".
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.Type()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(p)
			out[rel] = "-> " + target
			return nil
		}
		b, _ := os.ReadFile(p)
		out[rel] = string(b)
		return nil
	})
	return out
}

// The claim of revision 6's plan, Step 4 (section 13, its row): an edit
// to a brief in the swarm repository's clone, not pushed, reaches no
// prompt and no mount, and pushed it reaches the next pass's; the brief
// mounted at /work/.resources/<repo>/<path>, a checkout without .git,
// the run's result naming the commit it was read at, the checkout gone
// once the run is collected; status reads the briefs in the mirror as the
// last pass fetched it.
func TestSwarmMembersReadPushedContent(t *testing.T) {
	a := newApplyWSIn(t, true)
	up := a.upstreams[swarmURL]
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nv1\n\n## M1 The drift\n")
	a.publish("brief v1")
	v1 := gitIn(t, up, "rev-parse", "main")

	var seen []map[string]string
	owner := func() runner.Canned {
		return runner.Canned{Out: "BLOCKED: nothing to propose\n", Resources: func(dir string) error {
			seen = append(seen, tree(t, dir))
			return nil
		}}
	}
	batch := func(k int) (prompt, result string) {
		t.Helper()
		os.Remove(a.state("owner-held"))
		a.runner.Script(0, "owner", owner())
		a.pass()
		run := "0-owner-" + strconv.Itoa(k)
		if _, err := os.Stat(a.state(filepath.Join("runs", run, "resources"))); err == nil {
			t.Errorf("run %d: resources/ kept after the collection", k)
		}
		return a.file(run, "io/prompt.md"), a.result(run)
	}

	prompt, result := batch(1)
	heading := "## docs/briefs/a.md (Status: active; mounted at /work/.resources/swarm-app/docs/briefs/a.md)"
	if !strings.Contains(prompt, heading) || !strings.Contains(prompt, "\nv1\n") || !strings.Contains(prompt, "- Resources (read-only): /work/.resources/swarm-app/docs\n") {
		t.Fatalf("run 1's prompt:\n%s\nstdout:\n%s\nstderr:\n%s", prompt, a.stdout.String(), a.stderr.String())
	}
	if !strings.Contains(result, "\nresource swarm-app "+v1+"\n") {
		t.Errorf("run 1's result:\n%s", result)
	}
	if want := map[string]string{"docs/README.md": "# Docs\n", "docs/briefs/a.md": "# Drift\n\nStatus: active\nv1\n\n## M1 The drift\n"}; len(seen) != 1 || !reflect.DeepEqual(seen[0], map[string]string{
		"swarm-app/docs/README.md": want["docs/README.md"], "swarm-app/docs/briefs/a.md": want["docs/briefs/a.md"]}) {
		t.Errorf("run 1's mount: %q", seen)
	}

	// Edited in the clone, not pushed: the next pass reads v1 still.
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nv2\n\n## M1 The drift\n")
	prompt, result = batch(2)
	if !strings.Contains(prompt, "\nv1\n") || strings.Contains(prompt, "v2") || !strings.Contains(result, "\nresource swarm-app "+v1+"\n") ||
		len(seen) != 2 || !strings.Contains(seen[1]["swarm-app/docs/briefs/a.md"], "v1") {
		t.Errorf("run 2 read the unpushed edit:\n%s\n%s\n%q", prompt, result, seen)
	}

	// Pushed: status reads the mirror as the last pass left it, and the
	// next pass fetches and reads v2.
	a.publish("brief v2")
	v2 := gitIn(t, up, "rev-parse", "main")
	a.write("docs/briefs/b.md", "# B\n\nStatus: active\n\n## M1 Unpushed\n")
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "Briefs:\n  docs/briefs/a.md: 1 of 1 milestones with no issue\n") {
		t.Errorf("status before the pass: %v\n%s", err, a.stdout.String())
	}
	prompt, result = batch(3)
	if !strings.Contains(prompt, "\nv2\n") || strings.Contains(prompt, "v1") || strings.Contains(prompt, "docs/briefs/b.md") ||
		!strings.Contains(result, "\nresource swarm-app "+v2+"\n") || len(seen) != 3 || !strings.Contains(seen[2]["swarm-app/docs/briefs/a.md"], "v2") {
		t.Errorf("run 3 did not read the pushed edit:\n%s\n%s\n%q", prompt, result, seen)
	}
}

// The resources and references of a swarm repository's clone (section
// 13, revision 6's Step 4): a swarm-repository path, a path in a declared
// repository (<name>:<path>, at base, a link in it checked out as a
// link), a reference at a tag and one at a commit, each at
// /work/.resources/<repository>/<path> under one read-only mount, each
// repository's commit in the result; a reference fetched once, so its
// tag moved or its upstream gone changes nothing; a brief in a declared repository named
// <name>:<path>.
func TestSwarmResourcesAtTheirCommits(t *testing.T) {
	a := newApplyWSIn(t, true)
	libHead := pushFiles(t, a.upstream("lib"), map[string]string{"guide/x.md": "# Guide\n", "guide/link": "-> ../README.md",
		"guide/b.md": "# B\n\nStatus: active\n\n## M1 Lib\n", "other.md": "not a resource\n"})
	specUp := a.newUpstream("git@forge.example.net:std/spec.git", map[string]string{"SPEC.md": "spec v1\n"})
	gitIn(t, specUp, "tag", "v1", "main")
	specAt := gitIn(t, specUp, "rev-parse", "v1^{commit}")
	stdUp := a.newUpstream("https://forge.example.net/std/std.git", map[string]string{"lib/a.md": "std a\n", "lib/b.md": "std b\n"})
	stdAt := gitIn(t, stdUp, "rev-parse", "main")
	pushFiles(t, stdUp, map[string]string{"lib/a.md": "std a, later\n"}) // the pin stays before it
	a.edit(func(c *declare.Config) {
		c.Resources = []string{"docs/", "lib:guide/"}
		c.References = []declare.Reference{
			{Name: "spec", URL: "git@forge.example.net:std/spec.git", Pin: "v1", Paths: []string{"SPEC.md"}},
			{Name: "std", URL: "https://forge.example.net/std/std.git", Pin: stdAt, Paths: []string{"lib/"}},
		}
	})
	swarmAt := gitIn(t, a.upstreams[swarmURL], "rev-parse", "main")

	var seen []map[string]string
	arch := func() runner.Canned {
		return runner.Canned{Out: archPlan, Resources: func(dir string) error { seen = append(seen, tree(t, dir)); return nil }}
	}
	if _, err := a.forge.CreateIssue("Board", "Show the board.", []string{"swarm:proposal-accepted"}); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "architect", arch(), arch())
	a.pass()
	want := map[string]string{
		"swarm-app/docs/README.md": "# Docs\n",
		"lib/guide/x.md":           "# Guide\n", "lib/guide/link": "-> ../README.md", "lib/guide/b.md": "# B\n\nStatus: active\n\n## M1 Lib\n",
		"spec/SPEC.md": "spec v1\n",
		"std/lib/a.md": "std a\n", "std/lib/b.md": "std b\n",
	}
	if len(seen) != 1 || !reflect.DeepEqual(seen[0], want) {
		t.Fatalf("the mount: %q\nstdout:\n%s\nstderr:\n%s", seen, a.stdout.String(), a.stderr.String())
	}
	spec := a.runner.Launched[0]
	var mounts []string
	for _, m := range spec.Mounts {
		if strings.HasPrefix(m.Dst, "/work/.resources") {
			mounts = append(mounts, m.Dst+" "+strings.TrimPrefix(m.Src, a.state("runs/1-architect-1")+"/")+" "+map[bool]string{true: "ro", false: "rw"}[m.ReadOnly])
		}
	}
	if !reflect.DeepEqual(mounts, []string{"/work/.resources resources ro"}) {
		t.Errorf("mounts %q", mounts)
	}
	if prompt := a.file("1-architect-1", "io/prompt.md"); !strings.Contains(prompt, "- Resources (read-only): /work/.resources/swarm-app/docs, /work/.resources/lib/guide, /work/.resources/spec/SPEC.md, /work/.resources/std/lib\n") {
		t.Errorf("prompt:\n%s", prompt)
	}
	lines := "\nresource swarm-app " + swarmAt + "\nresource lib " + libHead + "\nresource spec " + specAt + "\nresource std " + stdAt + "\n"
	if result := a.result("1-architect-1"); !strings.Contains(result, lines) {
		t.Errorf("result:\n%s\nwant the lines%s", result, lines)
	}

	// Fetched once: the tag moved on its upstream, and the commit's
	// upstream gone, the pins are read from their mirrors as first
	// fetched.
	moved := pushFiles(t, specUp, map[string]string{"SPEC.md": "spec v2\n"})
	gitIn(t, specUp, "tag", "-f", "v1", moved)
	os.RemoveAll(stdUp)
	a.forge.Say(a.ref(1), "jane", "Say more.")
	a.pass()
	if len(seen) != 2 || !reflect.DeepEqual(seen[1], want) || !strings.Contains(a.result("1-architect-2"), lines) {
		t.Errorf("the second run: %q\n%s\n%s", seen, a.stdout.String(), a.stderr.String())
	}

	// The brief in lib, named <name>:<path>, mounted at lib/<path>.
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "Briefs:\n  lib:guide/b.md: 1 of 1 milestones with no issue\n") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
}

// *resource refused* (sections 7.1, step 3, and 7.7): a path the pass's
// commit does not hold, one that contains .env.sh, and a reference's pin
// its fetch did not bring each refuse the launch as a fault naming
// section 3's field, nothing launched and the run collected.
func TestSwarmResourceRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(a *applyWS, c *declare.Config)
		want func(a *applyWS) string
	}{
		{"not at the commit", func(a *applyWS, c *declare.Config) {
			a.write("gone/x.md", "only in the clone\n")
			c.Resources = []string{"docs/", "gone/"}
		}, func(a *applyWS) string {
			return `fault: swarm.json: resources[1]: "gone/" does not exist at ` + gitIn(a.t, a.upstreams[swarmURL], "rev-parse", "--short=7", "main")
		}},
		{"a secret", func(a *applyWS, c *declare.Config) {
			pushFiles(a.t, a.upstream("lib"), map[string]string{"guide/deep/.env.sh": "export A=1\n"})
			c.Resources = []string{"lib:guide"}
		}, func(*applyWS) string { return `fault: swarm.json: resources[0]: "lib:guide" contains .env.sh` }},
		{"a pin not fetched", func(a *applyWS, c *declare.Config) {
			a.newUpstream("git@forge.example.net:std/spec.git", map[string]string{"SPEC.md": "spec\n"})
			c.References = []declare.Reference{{Name: "spec", URL: "git@forge.example.net:std/spec.git", Pin: "v9", Paths: []string{"SPEC.md"}}}
		}, func(*applyWS) string {
			return "fault: swarm.json: references[0]: pin v9 not found at git@forge.example.net:std/spec.git: git fetch --quiet --no-tags origin +refs/tags/v9:refs/tags/v9: exit status 128: fatal: couldn't find remote ref refs/tags/v9"
		}},
		{"a reference's path", func(a *applyWS, c *declare.Config) {
			up := a.newUpstream("git@forge.example.net:std/spec.git", map[string]string{"SPEC.md": "spec\n"})
			gitIn(a.t, up, "tag", "v1", "main")
			c.References = []declare.Reference{{Name: "spec", URL: "git@forge.example.net:std/spec.git", Pin: "v1", Paths: []string{"SPEC.md", "NOTES.md"}}}
		}, func(*applyWS) string {
			return `fault: swarm.json: references[0].paths[1]: "NOTES.md" does not exist at v1`
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWSIn(t, true)
			a.edit(func(c *declare.Config) { tc.edit(a, c) })
			if _, err := a.forge.CreateIssue("Board", "Show the board.", []string{"swarm:proposal-accepted"}); err != nil {
				t.Fatal(err)
			}
			a.pass()
			want := tc.want(a)
			result := a.result("1-architect-1")
			if len(a.runner.Launched) != 0 || !strings.Contains(result, "\nlaunch-failed ") || !strings.Contains(result, "\n"+want+"\noutcome fault\ncollected\n") {
				t.Errorf("result:\n%s\nwant %s\nstdout:\n%s", result, want, a.stdout.String())
			}
			if _, err := os.Stat(a.state("runs/1-architect-1/resources")); err == nil {
				t.Error("resources/ kept")
			}
		})
	}
}

// An issue opened before the swarm moved names its brief by the old
// layout's workspace path, swarm-app/<path> (section 4; the plan's risk
// R2): until Step 7 it names the brief at <path>, so the milestone step
// puts it in its milestone, the batch counts it against the brief, and a
// Met: line naming it so is read by status.
func TestSwarmOldBriefPathMatches(t *testing.T) {
	a := newApplyWSIn(t, true)
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\n\n## M1 The drift\n")
	a.publish("brief")
	n, err := a.forge.CreateIssue("Old", mark("owner", 1)+"Brief: swarm-app/docs/briefs/a.md — M1 The drift\nAn old job.\n", []string{"swarm:plan-1-accepted", "swarm:blocked"})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(a.state("owner-held"))
	a.runner.Script(0, "owner", runner.Canned{Out: "Met: swarm-app/docs/briefs/a.md\n\nBLOCKED: the brief is met\n"})
	a.pass()
	if got := a.milestoneOf(n); got != "a: M1 The drift" || strings.Contains(a.stderr.String(), "no brief") {
		t.Errorf("#%d in %q\nstdout:\n%s\nstderr:\n%s", n, got, a.stdout.String(), a.stderr.String())
	}
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), "  docs/briefs/a.md: M1 The drift (1 open, 0 closed); met (owner run 1)\n") {
		t.Errorf("status: %v\n%s", err, a.stdout.String())
	}
}
