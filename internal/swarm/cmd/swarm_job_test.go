package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// The run's folder (sections 7.1, 7.2, 7.8 and 9; decision 159): every
// member mounts exactly <run>/job at /job and its children in it, the
// checkouts' folder at /job/work, the io at /job/io, the notes at
// /job/notes and the resources at /job/resources (and a slot at
// /credential); a CLI that is not confined starts in its checkout,
// /job/work/<name>, given /job alone by --add-dir, and grok starts in
// /job with no --add-dir; the mount points are made in <run>/job, and no
// clone holds one, nor a line for one in its .git/info/exclude.
func TestMemberArgvMountsTheRunsFolder(t *testing.T) {
	a := newApplyWS(t)
	a.notesUp(map[string]string{})
	env := "XAI_API_KEY"
	a.edit(func(c *declare.Config) {
		c.Personas["sympathetic"] = declare.Persona{Provider: "xai", Model: "grok-4.5", Effort: "high", CoAuthor: "grok-4.5 <noreply@x.ai>"}
		c.Keys["xai"] = declare.Key{Env: &env}
	})
	a.vars["XAI_API_KEY"] = "xai-key-0123456789abcdef0123456789abcdef"

	// What a member sees of the host at its launch: <run>/job's entries and
	// each clone's mount points and exclude lines.
	seen := map[string]string{}
	look := func(run string, then func(string) error) func(string) error {
		return func(work string) error {
			var b strings.Builder
			entries, _ := os.ReadDir(filepath.Join(filepath.Dir(work), "job"))
			for _, e := range entries {
				b.WriteString("job/" + e.Name() + "\n")
			}
			clones, _ := os.ReadDir(work)
			for _, c := range clones {
				for _, m := range []string{".swarm", ".notes", ".resources", ".job"} {
					if _, err := os.Lstat(filepath.Join(work, c.Name(), m)); err == nil {
						b.WriteString("mount point " + c.Name() + "/" + m + "\n")
					}
				}
				ex, _ := os.ReadFile(filepath.Join(work, c.Name(), ".git", "info", "exclude"))
				for _, l := range strings.Split(string(ex), "\n") {
					if strings.HasPrefix(l, "/.") {
						b.WriteString("excluded " + c.Name() + " " + l + "\n")
					}
				}
			}
			seen[run] = b.String()
			if then != nil {
				return then(work)
			}
			return nil
		}
	}
	if _, err := a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"}); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice, Work: look("1-architect-1", nil)})
	a.pass()
	if a.Cfg().Gates.Plan {
		a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	}
	v1 := work("v1")
	a.runner.Script(1, "worker", runner.Canned{Out: v1.Out, Work: look("1-worker-1", v1.Work)})
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve, Work: look("1-adversary-1", nil)})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve, Work: look("1-sympathetic-1", nil)})
	a.pass()

	children := "job/io\njob/notes\njob/resources\njob/work\n"
	for _, tc := range []struct {
		run, workdir string
		addDir       []string // nil: grok, confined
		notesRW      bool
		slot         bool
	}{
		{run: "1-architect-1", workdir: "/job/work/app", addDir: []string{"/job"}, notesRW: true},
		{run: "1-worker-1", workdir: "/job/work/app", addDir: []string{"/job"}},
		{run: "1-adversary-1", workdir: "/job/work/app", addDir: []string{"/job"}, slot: true},
		{run: "1-sympathetic-1", workdir: "/job"},
	} {
		i := slices.IndexFunc(a.runner.Launched, func(s runner.Spec) bool {
			return s.Name == "daisy-swarm.acme."+a.iss()+"."+strings.ReplaceAll(tc.run, "-", ".")
		})
		if i < 0 {
			t.Errorf("%s was not launched", tc.run)
			continue
		}
		spec, run := a.runner.Launched[i], a.state("runs/"+tc.run)
		want := []runner.Mount{{Src: run + "/job", Dst: "/job"}, {Src: run + "/work", Dst: "/job/work"}, {Src: run + "/io", Dst: "/job/io"},
			{Src: run + "/notes", Dst: "/job/notes", ReadOnly: !tc.notesRW}, {Src: run + "/resources", Dst: "/job/resources", ReadOnly: true}}
		got := slices.DeleteFunc(slices.Clone(spec.Mounts), func(m runner.Mount) bool { return tc.slot && m.Dst == "/credential" })
		if !reflect.DeepEqual(got, want) || spec.Workdir != tc.workdir {
			t.Errorf("%s: mounts %+v in %s\nwant %+v in %s", tc.run, spec.Mounts, spec.Workdir, want, tc.workdir)
		}
		argv := spec.Argv[len(exitPrefix):]
		var dirs []string
		for k, w := range argv {
			if w == "--add-dir" {
				dirs = append(dirs, argv[k+1])
			}
		}
		if !slices.Equal(dirs, tc.addDir) {
			t.Errorf("%s: --add-dir %q, want %q: %q", tc.run, dirs, tc.addDir, argv)
		}
		if tc.addDir == nil && argv[slices.Index(argv, "--cwd")+1] != "/job" {
			t.Errorf("%s: grok's --cwd: %q", tc.run, argv)
		}
		if !strings.Contains(spec.Argv[2], ">/job/io/exit;") || !strings.HasSuffix(argv[len(argv)-1], "Write your output to /job/io/out.md.") ||
			!strings.Contains(strings.Join(argv, " "), "/job/io/prompt.md") {
			t.Errorf("%s: the io's paths in %q", tc.run, spec.Argv)
		}
		if seen[tc.run] != children {
			t.Errorf("%s at its launch:\n%swant:\n%s", tc.run, seen[tc.run], children)
		}
	}
}

// A worker run launched before the run's folder, its clone at <run>/work
// and no <run>/job, is collected from that clone (section 7.6, step 1):
// its commit fetched, pushed and its pull request opened, as a run of the
// new layout's is.
func TestSwarmCollectsAWorkerOfTheEarlierLayout(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	v1 := work("v1")
	a.runner.Script(1, "worker", runner.Canned{Out: v1.Out, Work: func(work string) error {
		if err := v1.Work(work); err != nil {
			return err
		}
		// The earlier layout: the one clone is <run>/work, no <run>/job.
		run := filepath.Dir(work)
		if err := os.Rename(filepath.Join(work, "app"), filepath.Join(run, "clone")); err != nil {
			return err
		}
		if err := os.Remove(work); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(run, "clone"), work); err != nil {
			return err
		}
		return os.RemoveAll(filepath.Join(run, "job"))
	}})
	a.pass()
	r := a.result("1-worker-1")
	if ps := a.pulls(1); len(ps) != 1 || !strings.Contains(r, "\nfetched ") || !strings.Contains(r, "\npushed ") {
		t.Errorf("pulls %+v, result:\n%s", ps, r)
	}
}

// A worker run of the run's folder whose apply died is abandoned as one
// of the earlier layout is (sections 2, 7.6 and 7.7): its commits fetched
// from its clone at <run>/work/<name> and kept, and <run>/job, where the
// member may have written at its top, removed with <run>/work.
func TestSwarmAbandonedWorkerOfTheRunsFolder(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	head := a.orphanWorker()
	run := a.state("runs/1-worker-1")
	if err := os.Rename(filepath.Join(run, "work"), filepath.Join(run, "clone")); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"work", "job/work", "job/io"} {
		os.MkdirAll(filepath.Join(run, d), 0o755)
	}
	if err := os.Rename(filepath.Join(run, "clone"), filepath.Join(run, "work", "app")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(run, "job", "scratch.txt"), []byte("the member's\n"), 0o644)
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if r := a.result("1-worker-1"); !strings.Contains(r, "\nfetched "+head+"\n") || !strings.HasSuffix(r, "outcome abandoned\ncollected\n") {
		t.Errorf("result:\n%s", r)
	}
	for _, d := range []string{"work", "job"} {
		if _, err := os.Lstat(filepath.Join(run, d)); err == nil {
			t.Errorf("%s/ kept after the abandoned run's collection", d)
		}
	}
}
