package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
	"daisy/internal/testenv"
)

// The run's folder (sections 7.1, 7.2, 7.8 and 9; decision 159): every
// member mounts exactly <run>/job at /job and its children in it, the
// checkouts' folder at /job/work, the io at /job/io, the notes at
// /job/notes, the resources at /job/resources and the job's workspace
// at /job/memory (and a slot at /credential); a CLI that is not
// confined starts in its checkout, /job/work/<name>, given /job alone
// by --add-dir, and grok starts in
// /job with no --add-dir; the mount points are made in <run>/job, and no
// clone holds one, nor a line for one in its .git/info/exclude.
func TestMemberArgvMountsTheRunsFolder(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.notesUp(map[string]string{})
	env := "XAI_API_KEY"
	a.edit(func(c *declare.Config) {
		c.Personas["sympathetic"] = declare.Persona{Provider: "xai", Model: "grok-4.5", Effort: "high"}
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
	a.pass() // the sympathetic reviewer, once the adversary approved

	children := "job/io\njob/memory\njob/notes\njob/resources\njob/work\n"
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
			{Src: run + "/notes", Dst: "/job/notes", ReadOnly: !tc.notesRW}, {Src: run + "/resources", Dst: "/job/resources", ReadOnly: true},
			{Src: a.state("jobs/1"), Dst: "/job/memory", ReadOnly: true}}
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
	t.Parallel()
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
	t.Parallel()
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

// The job's workspace at /job/memory (sections 7.1, 7.2 and 7.8, The
// mount): a run on an issue mounts <state>/jobs/<n> read-only at
// /job/memory, after the resources' line, its mount point made in
// <run>/job and the workspace there (built) at its launch; a batch run,
// the owner's, has no job, so no such mount and no mount point.
func TestArgvMountsJobMemory(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.release()
	seen := map[string]string{}
	look := func(run string) func(string) error {
		return func(work string) error {
			var b strings.Builder
			if _, err := os.Lstat(filepath.Join(filepath.Dir(work), "job", "memory")); err == nil {
				b.WriteString("mount point job/memory\n")
			}
			if n, _, _ := strings.Cut(run, "-"); n != "0" {
				if _, err := os.Lstat(a.state("jobs/" + n + "/built")); err == nil {
					b.WriteString("jobs/" + n + "/built\n")
				}
			}
			seen[run] = b.String()
			return nil
		}
	}
	a.runner.Script(0, "owner", runner.Canned{Out: "## One\nA.\n", Work: look("0-owner-1")})
	a.pass()
	a.forge.SetLabel(issue1, "swarm:proposal-accepted")
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice, Work: look("1-architect-1")})
	a.pass()

	for _, tc := range []struct {
		run, seen string
		memory    *runner.Mount
	}{
		{run: "0-owner-1", seen: ""},
		{run: "1-architect-1", seen: "mount point job/memory\njobs/1/built\n", memory: &runner.Mount{Src: a.state("jobs/1"), Dst: "/job/memory", ReadOnly: true}},
	} {
		i := slices.IndexFunc(a.runner.Launched, func(s runner.Spec) bool {
			return s.Name == "daisy-swarm.acme."+a.iss()+"."+strings.ReplaceAll(tc.run, "-", ".")
		})
		if i < 0 {
			t.Errorf("%s was not launched:\n%s", tc.run, a.stdout.String())
			continue
		}
		ms := a.runner.Launched[i].Mounts
		var got *runner.Mount
		for k, m := range ms {
			if m.Dst == "/job/memory" {
				got = &ms[k]
				if k == 0 || ms[k-1].Dst != "/job/resources" {
					t.Errorf("%s: /job/memory not after the resources: %+v", tc.run, ms)
				}
			}
		}
		if !reflect.DeepEqual(got, tc.memory) {
			t.Errorf("%s: /job/memory mounted %+v, want %+v", tc.run, got, tc.memory)
		}
		if seen[tc.run] != tc.seen {
			t.Errorf("%s at its launch:\n%swant:\n%s", tc.run, seen[tc.run], tc.seen)
		}
	}
}

// branch pushes branch, made at base's head with files committed (none:
// base's head itself), to the upstream of the declared repository name.
func (a *applyWS) branch(name, branch string, files map[string]string) string {
	a.t.Helper()
	src := a.t.TempDir()
	for _, args := range [][]string{{"clone", "-q", a.upstream(name), src}, {"-C", src, "config", "user.name", "T"},
		{"-C", src, "config", "user.email", "t@example.net"}, {"-C", src, "checkout", "-q", "-b", branch}} {
		if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
			a.t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	if files != nil {
		if err := commit(files, "on "+branch)(src); err != nil {
			a.t.Fatal(err)
		}
	}
	if _, errb, err := ops.Real().Exec(src, "git", "push", "-q", "origin", branch); err != nil {
		a.t.Fatalf("push: %v %s", err, errb)
	}
	return a.gitRunIn(src, "rev-parse", "HEAD")
}

// The open branches (section 7.8, The open branches): before the launch
// of a worker's, a reviewer's or the architect's run, <run>/io/branches.md
// holds a line per swarm branch of each declared repository, as the pass
// fetched it, that is neither the run's own nor merged into main, with
// the files it changes against main, by repository, then issue and
// slice (#3 before #12), a branch sharing no history with main saying so
// in place of its files; None. when there is none. A branch
// at main's head (merged) and a branch not named swarm/<n>-<s> are left
// out, and the reviewers of #1 do not see swarm/1-1, their own.
func TestBranchesFile(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	h1 := a.pulls(1)[0].HeadSHA
	for _, run := range []string{"1-architect-1", "1-worker-1"} {
		if got := a.file(run, "io/branches.md"); got != "None.\n" {
			t.Errorf("%s's branches.md: %q", run, got)
		}
	}
	other := a.branch("lib", "swarm/3-2", map[string]string{"lib.go": "x\n", "sub/b.txt": "y\n"})
	later := a.branch("lib", "swarm/12-1", map[string]string{"c.txt": "c\n"})
	a.branch("app", "swarm/4-1", nil)
	// a branch of the name, sharing no history with main
	src := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "x", src}, {"-C", src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "other"},
		{"-C", src, "push", "-q", a.upstream("app"), "x:refs/heads/swarm/5-1"}} {
		if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	unrelated := a.gitRunIn(src, "rev-parse", "HEAD")
	a.branch("app", "feature-1", map[string]string{"f.txt": "z\n"})
	if _, err := a.forge.CreateIssue("Another", "Another job.", []string{"swarm:proposal-accepted"}); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved

	app5 := "- app swarm/5-1 (#5, slice 1) at " + unrelated[:7] + ": no history in common with main\n"
	lib := "- lib swarm/3-2 (#3, slice 2) at " + other[:7] + ": lib.go, sub/b.txt\n" +
		"- lib swarm/12-1 (#12, slice 1) at " + later[:7] + ": c.txt\n"
	for run, want := range map[string]string{
		"2-architect-1":   "- app swarm/1-1 (#1, slice 1) at " + h1[:7] + ": main.go\n" + app5 + lib,
		"1-adversary-1":   app5 + lib,
		"1-sympathetic-1": app5 + lib,
	} {
		if got := a.file(run, "io/branches.md"); got != want {
			t.Errorf("%s's branches.md:\n%s\nwant:\n%s", run, got, want)
		}
	}
	// The prompt's # Job counts them (section 7.8, The prompt): the file
	// is written before the prompt.
	for run, c := range map[string]string{"1-worker-1": "0 other open swarm branches", "2-architect-1": "4 other open swarm branches", "1-adversary-1": "3 other open swarm branches"} {
		if p := a.file(run, "io/prompt.md"); !strings.Contains(p, "\n- Branches: /job/io/branches.md, "+c+"\n") {
			t.Errorf("%s's prompt does not count %s:\n%s", run, c, p)
		}
	}
}

// A file # Job reads that is there and is not Daisy's, a round's file
// whose first line is not its heading, blocks the issue before the run is
// launched, the reconciler's note naming it (sections 7.7 and 7.8, The
// prompt), rather than a prompt summarising it wrong or the pass failing.
func TestJobSectionBlocksOnAFileItCannotUse(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.changesRequested()
	if err := os.WriteFile(a.state("jobs/1/slices/1/rounds/1.md"), []byte("not a round\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if _, err := os.Stat(a.state("runs/1-worker-2")); err == nil {
		t.Errorf("worker run 2 launched:\n%s", a.result("1-worker-2"))
	}
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !strings.Contains(a.lastComment(1), "Daisy cannot use jobs/1/slices/1/rounds/1.md: not a round's file: its first line is not `# Round <r> on <sha>`") {
		t.Errorf("not blocked: %v, last comment:\n%s", got, a.lastComment(1))
	}
}

// A pair of reviewers launched on one pass meets the same file # Job
// cannot use: the issue is blocked once, the reconciler's note posted
// once (section 7.8, Rollout), neither reviewer launched.
func TestJobSectionBlocksAPairOnce(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.changesRequested()
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if err := os.WriteFile(a.state("jobs/1/slices/1/rounds/1.md"), []byte("not a round\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	before := len(a.bodies(1))
	a.pass()
	notes := 0
	for _, b := range a.bodies(1)[before:] {
		if strings.Contains(b, "Daisy cannot use jobs/1/slices/1/rounds/1.md") {
			notes++
		}
	}
	if notes != 1 || !slices.Contains(labels(a.issue(1)), forge.Blocked) || a.result("1-adversary-2") != "" || a.result("1-sympathetic-2") != "" {
		t.Errorf("%d notes, labels %v\n%s", notes, labels(a.issue(1)), a.stdout.String())
	}
}

// A member text the prompt shows that is in the job's texts/ and cannot
// be read, the review whose [blocking] lines the worker answers, blocks
// the issue before the run is launched, the reconciler's note naming it
// (sections 7.7 and 7.8, The prompt), rather than a prompt showing the
// review cut or the pass failing.
func TestUnreadableTextBlocksTheLaunch(t *testing.T) {
	testenv.SkipAsRoot(t)
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.changesRequested()
	closed := a.state("jobs/1/texts/1-adversary-1.md")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o644) })
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if _, err := os.Stat(a.state("runs/1-worker-2")); err == nil {
		t.Errorf("worker run 2 launched:\n%s", a.result("1-worker-2"))
	}
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !strings.Contains(a.lastComment(1), "Daisy cannot use jobs/1/texts/1-adversary-1.md: permission denied") {
		t.Errorf("not blocked: %v, last comment:\n%s", got, a.lastComment(1))
	}
}

// A pair of reviewers whose prompts show a text in texts/ that cannot be
// read, the round's review, blocks the issue once, the note posted once,
// neither reviewer launched (sections 7.7 and 7.8, The prompt).
func TestUnreadableTextBlocksAPairOnce(t *testing.T) {
	testenv.SkipAsRoot(t)
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.changesRequested()
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	closed := a.state("jobs/1/texts/1-adversary-1.md")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o644) })
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	before := len(a.bodies(1))
	a.pass()
	notes := 0
	for _, b := range a.bodies(1)[before:] {
		if strings.Contains(b, "Daisy cannot use jobs/1/texts/1-adversary-1.md") {
			notes++
		}
	}
	if notes != 1 || !slices.Contains(labels(a.issue(1)), forge.Blocked) || a.result("1-adversary-2") != "" || a.result("1-sympathetic-2") != "" {
		t.Errorf("%d notes, labels %v\n%s", notes, labels(a.issue(1)), a.stdout.String())
	}
}
