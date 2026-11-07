package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/provider"
	"daisy/internal/swarm/runner"
)

// launch prepares and starts the run of j (section 7.1): for the worker,
// its slice's repository resolved first; its directory and prompt files,
// read back; the clones; the credential; the argv and the result's first
// lines. It reports whether a container started. A run that cannot launch
// is recorded and collected at once; none is made when no slot is free.
func (s *Swarm) launch(j *Job, running []runner.Running) (bool, error) {
	fs := s.O.FS()
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-%s-%d", j.N, j.Role, j.K)), N: j.N, K: j.K, Role: j.Role}
	ioDir := filepath.Join(r.Dir, "io")
	if err := fs.MkdirAll(ioDir, 0o755); err != nil {
		return false, err
	}
	now := func() string { return stamp(s.O.Now()) }
	var labelLines []string
	if j.Label != "" {
		labelLines = []string{"label " + j.Label}
	}
	failed := func(lines ...string) (bool, error) {
		if err := r.Append(fs, append(labelLines, lines...)...); err != nil {
			return false, err
		}
		return false, s.collect(r)
	}

	// The run's repository and branch: the worker's slice, a reviewer's
	// pull request; the owner and the architect clone every repository.
	var repo declare.Repo
	branch, newBranch := s.Cfg.Base, false
	switch j.Role {
	case "worker":
		sl, name, bad := j.WorkerSlice()
		rp, declared := s.repo(name)
		if len(bad) == 0 && !declared {
			bad = []int{sl}
		}
		if len(bad) > 0 {
			return failed("launch-failed "+now(), "failed: bad slices: "+numbers(bad))
		}
		repo, branch = rp, fmt.Sprintf("swarm/%d-%d", j.N, sl)
		_, newBranch = j.current()
		newBranch = !newBranch
	case "adversary", "sympathetic":
		cur, _ := j.current()
		repo, _ = s.repo(cur.Repo)
		branch = cur.Head
	}

	system, prompt := Render(j)
	for _, f := range []struct{ name, text string }{{"system.md", system}, {"prompt.md", prompt}} {
		p := filepath.Join(ioDir, f.name)
		if err := fs.WriteFile(p, []byte(f.text), 0o644); err != nil {
			return false, err
		}
		back, err := fs.ReadFile(p)
		if err != nil || !bytes.Equal(back, []byte(f.text)) {
			return failed("launch-failed "+now(), "fault: prompt: "+f.name+": differs")
		}
	}
	if emptyField(prompt) {
		return failed("launch-failed "+now(), "fault: prompt: prompt.md: empty field")
	}

	persona := s.Cfg.Personas[j.Role]
	work := filepath.Join(r.Dir, "work")
	var startLines, reads []string
	workdir, image := "/work", repo.Image
	clone := func(rp declare.Repo, branch, dir string) (string, error) {
		sha, err := s.git().Clone(rp, branch, s.Cfg.Base, dir, j.Role, s.Cfg.Forge.CommitEmail, persona.CoAuthor)
		if err != nil {
			fs.RemoveAll(r.Dir)
		}
		return sha, err
	}
	if j.Role == "owner" || j.Role == "architect" {
		for _, rp := range s.Cfg.Repos {
			sha, err := clone(rp, s.Cfg.Base, filepath.Join(work, rp.Name))
			if err != nil {
				return false, err
			}
			startLines = append(startLines, "start "+rp.Name+" "+sha)
		}
		first := s.Cfg.Repos[0]
		workdir, image = "/work/"+first.Name, first.Image
		reads = append(reads, "/work/.swarm")
		for _, rp := range s.Cfg.Repos[1:] {
			reads = append(reads, "/work/"+rp.Name)
		}
	} else {
		from := branch
		if newBranch {
			from = s.Cfg.Base
		}
		sha, err := clone(repo, from, work)
		if err != nil {
			return false, err
		}
		if newBranch {
			if err := s.git().Branch(work, branch); err != nil {
				fs.RemoveAll(r.Dir)
				return false, err
			}
		}
		startLines = append(startLines, "start "+sha)
		if j.Role == "worker" {
			startLines = append(startLines, "branch "+branch+" in "+repo.Name)
		}
	}
	if err := fs.MkdirAll(filepath.Join(work, ".swarm"), 0o755); err != nil {
		return false, err
	}

	o, rp, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	mounts := []runner.Mount{{Src: work, Dst: "/work"}, {Src: ioDir, Dst: "/work/.swarm"}}
	for i, res := range ResourceMounts(s.Cfg.Resources) {
		src, err := fs.EvalSymlinks(filepath.Join(s.WS, s.Cfg.Resources[i]))
		if err != nil {
			return false, err
		}
		mounts = append(mounts, runner.Mount{Src: src, Dst: res, ReadOnly: true})
		reads = append(reads, res)
	}
	spec := runner.Spec{Name: s.containerName(j.N, j.Role, j.K), Label: fmt.Sprintf("%s/%s#%d/%s", o, rp, j.N, j.Role), User: s.User,
		Mounts: mounts, Workdir: workdir, Image: image,
		Argv: provider.Providers[persona.Provider].Adapter.Command(persona.Model, persona.Effort, workdir, "/work/.swarm/system.md", "/work/.swarm/prompt.md", outside(workdir, reads))}

	key := s.Cfg.Keys[persona.Provider]
	var credential string
	var secrets []string
	var slot *runner.Slot
	if key.Env != nil {
		spec.EnvVar, spec.EnvValue = *key.Env, s.Secret(*key.Env)
		secrets = []string{spec.EnvValue}
		credential = "credential " + persona.Provider + " env " + *key.Env
	} else {
		holder := fmt.Sprintf("%s#%d/%s run %d since %s", s.Cfg.Forge.Repo, j.N, j.Role, j.K, now())
		var err error
		slot, err = runner.TakeSlot(fs, s.Runner, s.slotDir(persona.Provider), persona.Provider, *key.Slots, holder, running)
		var refused *runner.SlotRefused
		switch {
		case errors.Is(err, runner.ErrNoSlot):
			return false, fs.RemoveAll(r.Dir)
		case errors.As(err, &refused):
			return failed(append(startLines, "launch-failed "+now(), "fault: "+refused.Error())...)
		case err != nil:
			fs.RemoveAll(r.Dir)
			return false, err
		}
		secrets = slot.Secrets
		spec.Credential, spec.Slot = slot.Credential, slot.Dir
		credential = fmt.Sprintf("credential %s slot %d", persona.Provider, slot.J)
	}
	if err := fs.WriteFile(filepath.Join(r.Dir, "argv"), []byte(strings.Join(append([]string{"docker"}, spec.Args()...), "\n")+"\n"), 0o644); err != nil {
		if slot != nil {
			slot.Release()
		}
		return false, err
	}
	lines := append(startLines, labelLines...)
	if j.Batch != nil {
		lines = append(lines, "propose "+strconv.Itoa(j.Batch.Propose))
	}
	if err := r.Append(fs, append(lines, "started "+now(), credential)...); err != nil {
		if slot != nil {
			slot.Release()
		}
		return false, err
	}
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	c, err := s.Runner.Start(spec, timeout)
	if err != nil {
		if slot != nil {
			slot.Release()
		}
		return false, errors.Join(r.Append(fs, "launch-failed "+now(), "fault: launch: "+err.Error()), s.collect(r))
	}
	s.given[r.Dir] = secrets
	s.watch(r.Dir, c, slot, false)
	fmt.Fprintf(s.Out, "%slaunched\n", runHead(r))
	return true, nil
}

// runHead is the run's prefix on stdout: `#<n> <role> run <k>: `, the
// batch run's `owner run <k>: `.
func runHead(r Run) string {
	if r.N == 0 {
		return fmt.Sprintf("owner run %d: ", r.K)
	}
	return fmt.Sprintf("#%d %s run %d: ", r.N, r.Role, r.K)
}

func (s *Swarm) slotDir(p string) string { return filepath.Join(s.Local, "swarm", "credentials", p) }

// outside is the mounts a member started in workdir must be given leave
// to reach: those not under it.
func outside(workdir string, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if d != workdir && !strings.HasPrefix(d, workdir+"/") {
			out = append(out, d)
		}
	}
	return out
}

// watch waits for a started container in the background: once `docker
// run` has returned, a slot's secrets are read again and its lock
// released (sections 7.1 and 7.6), and done is closed.
func (s *Swarm) watch(dir string, c runner.Container, slot *runner.Slot, check bool) {
	l := &live{check: check, done: make(chan struct{})}
	s.live[dir] = l
	go func() {
		defer close(l.done)
		l.exit, l.err = c.Wait()
		if slot != nil {
			l.again, _ = runner.SlotSecrets(s.O.FS(), slot.Dir)
			slot.Release()
		}
	}()
}

// settle records how an ended container of this process ended (sections
// 2 and 7.7): a member's streams, end line, cost and failure or fault; a
// check's output and end line.
func (s *Swarm) settle(r Run) error {
	l := s.live[r.Dir]
	delete(s.live, r.Dir)
	fs := s.O.FS()
	now := stamp(s.O.Now())
	if l.check {
		if l.err != nil {
			return r.Append(fs, "fault: check: wait: "+l.err.Error())
		}
		if err := fs.WriteFile(filepath.Join(r.Dir, "check.out"), append(l.exit.Stdout, l.exit.Stderr...), 0o644); err != nil {
			return err
		}
		said := fmt.Sprintf("check exited %d", l.exit.Code)
		lines := []string{said + " " + now}
		switch {
		case l.exit.TimedOut:
			said, lines = "check timeout", []string{"check timeout " + now}
		case l.exit.Code >= 125 && l.exit.Code <= 127:
			lines = append(lines, fmt.Sprintf("fault: check: exit %d", l.exit.Code))
		}
		fmt.Fprintf(s.Out, "%s%s\n", runHead(r), said)
		return r.Append(fs, lines...)
	}
	s.given[r.Dir] = append(s.given[r.Dir], l.again...)
	if l.err != nil {
		return r.Append(fs, "exited -1 "+now, "fault: wait: "+l.err.Error())
	}
	exit := l.exit
	fs.WriteFile(filepath.Join(r.Dir, "stdout"), exit.Stdout, 0o644)
	fs.WriteFile(filepath.Join(r.Dir, "stderr"), exit.Stderr, 0o644)
	lines := []string{fmt.Sprintf("exited %d %s", exit.Code, now)}
	if exit.TimedOut {
		lines = []string{"timeout " + now}
	}
	rep := provider.Providers[s.Cfg.Personas[r.Role].Provider].Adapter.Report(exit.Stdout)
	if rep.Known {
		lines = append(lines, fmt.Sprintf("cost %s tokens %d/%d turns %d", strconv.FormatFloat(rep.CostUSD, 'f', -1, 64), rep.InTokens, rep.OutTokens, rep.Turns))
	}
	switch {
	case exit.TimedOut:
		lines = append(lines, "failed: timeout after "+s.Cfg.Timeout)
	case exit.Code >= 125 && exit.Code <= 127:
		lines = append(lines, fmt.Sprintf("fault: exit %d", exit.Code))
	case rep.Fault != "":
		lines = append(lines, "fault: provider: "+rep.Fault)
	case exit.Code != 0:
		lines = append(lines, fmt.Sprintf("failed: exit %d", exit.Code))
	}
	return r.Append(fs, lines...)
}

// runRef is the mirror's ref for a worker run's commits (section 7.6).
func runRef(r Run) string { return fmt.Sprintf("refs/swarm-runs/%d-worker-%d", r.N, r.K) }

// startCheck clones the worker run's fetched head from the mirror and
// starts the check container on it (sections 7.2 and 7.6, step 4); a
// check that cannot start is a swarm fault. A check whose apply died is
// started again.
func (s *Swarm) startCheck(r Run, lines []string) error {
	fs := s.O.FS()
	repo, _ := s.repo(branchRepo(lines))
	fetched, _ := Has(lines, "fetched")
	sha := strings.TrimPrefix(fetched, "fetched ")
	dir := filepath.Join(r.Dir, "check")
	if err := fs.RemoveAll(dir); err != nil {
		return err
	}
	if err := s.git().CheckClone(repo.Name, dir, runRef(r), sha); err != nil {
		return err
	}
	o, rp, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	spec := runner.Spec{Name: s.containerName(r.N, "check", r.K), Label: fmt.Sprintf("%s/%s#%d/check", o, rp, r.N), User: s.User, NoNetwork: true,
		Mounts: []runner.Mount{{Src: dir, Dst: "/work"}}, Workdir: "/work", Image: repo.Image, Argv: repo.Check}
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	c, err := s.Runner.Start(spec, timeout)
	if err != nil {
		return r.Append(fs, "fault: check: "+err.Error())
	}
	if err := r.Append(fs, "check started "+stamp(s.O.Now())); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "%scheck started\n", runHead(r))
	s.watch(r.Dir, c, nil, true)
	return nil
}

// branchRepo is the repository of a worker run's `branch <b> in <name>`
// line.
func branchRepo(lines []string) string {
	l, _ := Has(lines, "branch")
	_, name, _ := strings.Cut(l, " in ")
	return name
}

// runBranch is a worker run's branch.
func runBranch(lines []string) string {
	l, _ := Has(lines, "branch")
	b, _, _ := strings.Cut(strings.TrimPrefix(l, "branch "), " in ")
	return b
}
