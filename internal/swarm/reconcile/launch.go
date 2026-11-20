package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/provider"
	"daisy/internal/swarm/runner"
)

// launch prepares and starts the run of j (section 7.1): for the worker,
// its slice's repository resolved first; its directory and prompt files,
// read back; the clones; the credential; the argv and the result's first
// lines. It reports whether a container started, and once it has written
// the in-progress stage j.Label is that stage. A run that cannot launch is
// recorded and collected at once; none is made when no slot is free.
func (s *Swarm) launch(j *Job, running []runner.Running) (bool, error) {
	fs := s.O.FS()
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-%s-%d", j.N, j.Role, j.K)), N: j.N, K: j.K, Role: j.Role}
	ioDir := filepath.Join(r.Dir, "io")
	if err := fs.MkdirAll(ioDir, 0o755); err != nil {
		return false, err
	}
	now := func() string { return stamp(s.O.Now()) }
	ended := func() string { return endStamp(s.O.Now()) }
	var labelLines []string
	if j.Label != "" {
		labelLines = []string{"label " + j.Label}
	}
	if j.Question {
		labelLines = append(labelLines, "question") // the collection's: an answer, not a plan (section 6, The question)
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
	var round []string // a reviewer's round, which its record reads (section 7.6)
	branch, newBranch := s.Cfg.Base, false
	switch j.Role {
	case "worker":
		sl, name, bad := j.WorkerSlice()
		rp, declared := s.repo(name)
		if len(bad) == 0 && !declared {
			bad = []int{sl}
		}
		if len(bad) > 0 {
			return failed("launch-failed "+ended(), "failed: bad slices: "+numbers(bad))
		}
		repo, branch = rp, fmt.Sprintf("swarm/%d-%d", j.N, sl)
		_, newBranch = j.current()
		newBranch = !newBranch
	case "adversary", "sympathetic":
		cur, _ := j.current()
		repo, _ = s.repo(cur.Repo)
		branch = cur.Head
		rd, _ := j.round(cur)
		round = []string{"round " + strconv.Itoa(rd)}
	}

	// The run's copy of the notes, indexed for the prompt.
	notesDir, notesSHA := filepath.Join(r.Dir, "notes"), ""
	if s.Cfg.Notes {
		w, err := s.notesRepo()
		if err == nil {
			notesSHA, err = w.Export(notesDir)
		}
		if err == nil {
			j.Notes, err = ReadNotes(fs, notesDir)
		}
		if err != nil {
			fs.RemoveAll(r.Dir)
			return false, err
		}
		s.staleness(j.Notes)
	}

	// The swarm's notes about each repository the run works in, at the
	// pass's commit (section 4).
	var err error
	if j.Repos, err = s.repoNotes(s.noteRepos(j, repo)); err != nil {
		fs.RemoveAll(r.Dir)
		return false, err
	}

	// The resources as system.md lists them (section 7.3).
	if j.Resources, err = s.listing(); err != nil {
		fs.RemoveAll(r.Dir)
		return false, err
	}

	// Praxis and the testing skill from the installed root (section 7.3);
	// one that cannot be read is the swarm fault prompt refused (7.7).
	if readsPraxis(j.Role) {
		if j.Praxis, j.Testing, err = s.praxis(); err != nil {
			return failed("launch-failed "+ended(), "fault: prompt: system.md: "+err.Error())
		}
	}

	system, prompt := Render(j)
	for _, f := range []struct{ name, text string }{{"system.md", system}, {"prompt.md", prompt}} {
		p := filepath.Join(ioDir, f.name)
		if err := fs.WriteFile(p, []byte(f.text), 0o644); err != nil {
			return false, err
		}
		back, err := fs.ReadFile(p)
		if err != nil || !bytes.Equal(back, []byte(f.text)) {
			return failed("launch-failed "+ended(), "fault: prompt: "+f.name+": differs")
		}
	}
	if emptyField(prompt) {
		return failed("launch-failed "+ended(), "fault: prompt: prompt.md: empty field")
	}
	// The whole output of each text the prompt shows cut, where its note
	// points (section 7.3).
	if err := copyOutputs(fs, s.state(), ioDir, j.Cut()); err != nil {
		fs.RemoveAll(r.Dir)
		return false, err
	}
	if j.Role == "adversary" || j.Role == "sympathetic" {
		// The screenshots # Artifacts lists, where the reviewer opens
		// them (section 7.3).
		if err := copyArtifacts(fs, ioDir, j.Artifacts); err != nil {
			fs.RemoveAll(r.Dir)
			return false, err
		}
	}

	// The open branches, from the mirrors as the pass fetched them
	// (section 7.8): the architect has no branch of its own.
	if readsBranches(j.Role) {
		ownRepo, ownBranch := repo.Name, branch
		if j.Role == "architect" {
			ownRepo, ownBranch = "", ""
		}
		if err := s.writeBranches(ioDir, ownRepo, ownBranch); err != nil {
			fs.RemoveAll(r.Dir)
			return false, err
		}
	}

	// The resources, checked out of the mirrors at the pass's commits
	// (section 7.1, step 3).
	resDir := filepath.Join(r.Dir, "resources")
	resourceLines, refused, err := s.checkout(resDir)
	if err != nil {
		fs.RemoveAll(r.Dir)
		return false, err
	}
	if refused != "" {
		return failed("launch-failed "+ended(), "fault: "+refused)
	}

	persona := s.Cfg.Personas[j.Role]
	work := filepath.Join(r.Dir, "work")
	var startLines []string
	var svc *runner.Services // the worker's test services (section 7.6)
	var svcEnv []string
	image, workRepo := repo.Image, repo.Name
	// A clone that fails, or a step of its preparation, is a swarm fault
	// recorded as a launch's (section 7.7), collected at once, so the
	// pair's second reviewer's issue goes home.
	cloneFailed := func(err error) (bool, error) {
		return failed("launch-failed "+ended(), "fault: clone: "+err.Error())
	}
	clone := func(rp declare.Repo, branch, dir string, also ...string) (string, error) {
		return s.git().Clone(rp, branch, s.Cfg.Base, dir, j.Role, s.Cfg.Forge.CommitEmail, persona.CoAuthor, also...)
	}
	if clonesEvery(j.Role) {
		// The architect revising with the issue's pull request open sees
		// its head, a branch of that repository's clone (section 7.1).
		cur, open := j.current()
		for _, rp := range s.Cfg.Repos {
			var also []string
			if j.Role == "architect" && open && cur.Repo == rp.Name {
				also = []string{cur.Head}
			}
			sha, err := clone(rp, s.Cfg.Base, filepath.Join(work, rp.Name), also...)
			if err != nil {
				return cloneFailed(err)
			}
			startLines = append(startLines, "start "+rp.Name+" "+sha)
		}
		image, workRepo = s.Cfg.Repos[0].Image, s.Cfg.Repos[0].Name
	} else {
		from := branch
		if newBranch {
			from = s.Cfg.Base
		}
		sha, err := clone(repo, from, filepath.Join(work, repo.Name))
		if err != nil {
			return cloneFailed(err)
		}
		if newBranch {
			if err := s.git().Branch(filepath.Join(work, repo.Name), branch); err != nil {
				return cloneFailed(err)
			}
		}
		startLines = append(startLines, "start "+sha)
		if j.Role == "worker" {
			startLines = append(startLines, "branch "+branch+" in "+repo.Name)
			// The services at the commit the worker starts from; one the
			// file refuses runs beside none, its check reporting the
			// refusal if the head still holds it.
			ts, ok, refused, err := s.testServices(repo, sha)
			switch {
			case err != nil:
				fs.RemoveAll(r.Dir)
				return false, err
			case refused != "":
				fmt.Fprintf(s.Err, "⚠ %stest services refused (%s): launched without them\n", runHead(r), refused)
				startLines = append(startLines, "services worker refused: "+refused)
			case ok:
				v := s.services(r, "worker", ts)
				svc, svcEnv = &v, ts.Env
			}
		}
	}

	// The run's folder, <run>/job at /job, and its children (sections 7.2
	// and 7.8): their mount points made in it as the host user's, so
	// Docker makes none, and none in a checkout.
	jobDir := filepath.Join(r.Dir, "job")
	mounts := []runner.Mount{{Src: jobDir, Dst: JobMount}, {Src: work, Dst: WorkMount}, {Src: ioDir, Dst: IOMount}}
	if s.Cfg.Notes {
		mounts = append(mounts, runner.Mount{Src: notesDir, Dst: NotesMount, ReadOnly: !writesNotes(j.Role)})
		startLines = append(startLines, "notes-base "+notesSHA)
	}
	if len(resourceLines) > 0 {
		mounts = append(mounts, runner.Mount{Src: resDir, Dst: ResourcesMount, ReadOnly: true})
		startLines = append(startLines, resourceLines...)
	}
	if j.Batch == nil {
		// The job's workspace, read-only, for a run on an issue (section
		// 7.8, The mount); a batch run has no job.
		mounts = append(mounts, runner.Mount{Src: s.workspace(j.N), Dst: MemoryMount, ReadOnly: true})
	}
	for _, m := range mounts[1:] {
		if err := fs.MkdirAll(filepath.Join(jobDir, path.Base(m.Dst)), 0o755); err != nil {
			fs.RemoveAll(r.Dir)
			return false, err
		}
	}
	// A CLI that is not confined starts in its checkout and is given
	// /job, which holds all else (section 9's D).
	workdir := Workdir(s.Cfg, j.Role, workRepo)
	var reads []string
	if workdir != JobMount {
		reads = []string{JobMount}
	}
	o, rp, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	spec := runner.Spec{Name: s.containerName(j.N, j.Role, j.K), Label: fmt.Sprintf("%s/%s#%d/%s", o, rp, j.N, j.Role), User: s.User,
		Mounts: mounts, Workdir: workdir, Image: image,
		Argv: exitMarked(provider.Providers[persona.Provider].Adapter.Command(persona.Model, persona.Effort, workdir, IOMount+"/system.md", IOMount+"/prompt.md", reads))}
	if svc != nil {
		// Its own network, not internal: the CLI reaches its API through
		// it as through Docker's default bridge (section 7.2).
		spec.Network, spec.Env = svc.Network, svcEnv
	}

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
		prov := provider.Providers[persona.Provider]
		slot, err = runner.TakeSlot(fs, s.Runner, s.slotDir(persona.Provider), persona.Provider, *key.Slots, prov.LoginHint, holder, running)
		var refused *runner.SlotRefused
		switch {
		case errors.Is(err, runner.ErrNoSlot):
			return false, fs.RemoveAll(r.Dir)
		case errors.As(err, &refused):
			return failed(append(startLines, "launch-failed "+ended(), "fault: "+refused.Error())...)
		case err != nil:
			fs.RemoveAll(r.Dir)
			return false, err
		}
		secrets = slot.Secrets
		spec.Credential, spec.Slot, spec.Home = slot.Credential, slot.Dir, prov.Home
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
	// The persona the run is launched with, which its record names
	// whatever swarm.json says by its collection.
	lines = append(append(lines, "started "+now(), credential, "persona "+persona.Provider+" "+persona.Model+" "+persona.Effort), round...)
	if err := r.Append(fs, lines...); err != nil {
		if slot != nil {
			slot.Release()
		}
		return false, err
	}
	if svc != nil {
		fault, err := s.servicesUp(r, "worker", *svc)
		if err != nil || fault != "" {
			if slot != nil {
				slot.Release()
			}
			if err != nil {
				return false, err
			}
			return false, errors.Join(r.Append(fs, "launch-failed "+ended(), fault), s.collect(r))
		}
	}
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	c, err := s.Runner.Start(spec, timeout)
	if err != nil {
		if slot != nil {
			slot.Release()
		}
		return false, errors.Join(r.Append(fs, "launch-failed "+ended(), "fault: launch: "+err.Error()), s.collect(r))
	}
	s.given[r.Dir] = secrets
	s.watch(r.Dir, c, slot, "")
	fmt.Fprintf(s.Out, "%slaunched\n", runHead(r))
	return true, s.inProgress(j, r)
}

// inProgress writes the in-progress stage of run r of j once its container
// has started (T20, T21): a refused launch never moves the issue, and the
// collection moves it on from either stage, or back (section 5). j.Label
// is then the issue's stage, which the pair's second is launched on.
func (s *Swarm) inProgress(j *Job, r Run) error {
	to := InProgress(j.Role, j.Label)
	if to == "" {
		return nil
	}
	if err := s.Forge.Relabel(forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: j.N}, j.Label, to); err != nil {
		return err
	}
	s.moved(j.N, to)
	fmt.Fprintf(s.Out, "%s%s → %s\n", runHead(r), j.Label, to)
	from := j.Label
	j.Label = to
	return r.Append(s.O.FS(), "relabelled "+from+" "+to)
}

// exitScript runs the provider's argv under sh and, once the CLI has
// exited, writes its exit status to /job/io/exit (<run>/io/exit):
// the mark that the member finished, which a later apply reads when the
// apply that started the run died (sections 7.2 and 7.7, ended). A
// container killed outright writes none. It is the image's own sh, so no
// image is rebuilt for it.
const exitScript = `"$@"; s=$?; echo "$s" >` + IOMount + `/exit; exit "$s"`

// exitMarked is argv run under exitScript.
func exitMarked(argv []string) []string {
	return append([]string{"sh", "-c", exitScript, "sh"}, argv...)
}

// runHead is the run's prefix on stdout: `#<n> <role> run <k>: `, a
// batch run's `<role> run <k>: ` (the owner's, the QA's).
func runHead(r Run) string {
	if r.N == 0 {
		return fmt.Sprintf("%s run %d: ", r.Role, r.K)
	}
	return fmt.Sprintf("#%d %s run %d: ", r.N, r.Role, r.K)
}

func (s *Swarm) slotDir(p string) string { return filepath.Join(s.Local, "swarm", "credentials", p) }

// watch waits for a started container in the background, a member or
// a worker run's step ("deps" or "check"): once `docker run` has
// returned, a slot's secrets are read again and its lock released
// (sections 7.1 and 7.6), and done is closed.
func (s *Swarm) watch(dir string, c runner.Container, slot *runner.Slot, step string) {
	l := &live{step: step, done: make(chan struct{})}
	s.live[dir] = l
	s.started++
	go func() {
		defer close(l.done)
		l.exit, l.err = c.Wait()
		l.at = s.O.Now()
		if slot != nil {
			l.again, _ = runner.SlotSecrets(s.O.FS(), slot.Dir)
			slot.Release()
		}
	}()
}

// settle records how an ended container of this process ended (sections
// 2 and 7.7): a member's streams, end line, cost and failure or fault; a
// step's output, <step>.out, and end line.
func (s *Swarm) settle(r Run) error {
	l := s.live[r.Dir]
	delete(s.live, r.Dir)
	fs := s.O.FS()
	now := endStamp(l.at)
	if step := l.step; step != "" {
		if l.err != nil {
			return r.Append(fs, "fault: "+step+": wait: "+l.err.Error())
		}
		if err := fs.WriteFile(filepath.Join(r.Dir, step+".out"), append(l.exit.Stdout, l.exit.Stderr...), 0o644); err != nil {
			return err
		}
		said := fmt.Sprintf("%s exited %d", step, l.exit.Code)
		lines := []string{said + " " + now}
		switch {
		case l.exit.TimedOut:
			said, lines = step+" timeout", []string{step + " timeout " + now}
		case l.exit.Code >= 125 && l.exit.Code <= 127:
			lines = append(lines, fmt.Sprintf("fault: %s: exit %d", step, l.exit.Code))
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
		// A cost the CLI did not report is `-`, never 0 (section 3).
		cost := "-"
		if rep.CostKnown {
			cost = strconv.FormatFloat(rep.CostUSD, 'f', -1, 64)
		}
		line := fmt.Sprintf("cost %s tokens %d/%d turns %d", cost, rep.InTokens, rep.OutTokens, rep.Turns)
		if rep.CachedKnown {
			line += fmt.Sprintf(" cached %d", rep.CachedTokens)
		}
		lines = append(lines, line)
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

// startStep clones the worker run's fetched head from the mirror into
// <run>/check and starts step on it (sections 7.2 and 7.6, step 4): the
// dependency step ("deps") on Docker's default network, the clone
// read-only and the swarm's cache for the repository read-write; or the
// check, with no network and, when the repository declares deps, that
// cache read-only; both given deps' env. With the head's test services
// file, the check runs beside its services on their internal network,
// given x-daisy-env too; a file refused runs no check, its refusal the
// check's result. One that cannot start, services included, is a swarm
// fault; one whose apply died is started again, its services taken down
// first.
func (s *Swarm) startStep(r Run, lines []string, step string) error {
	fs := s.O.FS()
	repo, _ := s.repo(branchRepo(lines))
	if step == "deps" && repo.Deps == nil { // swarm.json changed since the step started
		return r.Append(fs, "fault: deps: "+repo.Name+" declares no deps")
	}
	lines, err := s.servicesDown(r, lines)
	if err != nil {
		return err
	}
	fetched, _ := Has(lines, "fetched")
	sha := strings.TrimPrefix(fetched, "fetched ")
	dir := filepath.Join(r.Dir, "check")
	if err := fs.RemoveAll(dir); err != nil {
		return err
	}
	if err := s.git().CheckClone(repo.Name, dir, runRef(r), sha); err != nil {
		return err
	}
	var ts declare.TestServices
	var svc bool
	if step == "check" {
		var refused string
		if ts, svc, refused, err = s.testServices(repo, sha); err != nil {
			return err
		}
		if refused != "" {
			fmt.Fprintf(s.Out, "%stest services refused (%s): the check did not run\n", runHead(r), refused)
			return r.Append(fs, "services check refused: "+refused)
		}
	}
	o, rp, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	spec := runner.Spec{Name: s.containerName(r.N, step, r.K), Label: fmt.Sprintf("%s/%s#%d/%s", o, rp, r.N, step), User: s.User, Network: "none",
		Mounts: []runner.Mount{{Src: dir, Dst: "/work"}}, Workdir: "/work", Image: repo.Image, Argv: repo.Check}
	if d := repo.Deps; d != nil {
		cache := s.depsCache(repo.Name)
		if err := fs.MkdirAll(cache, 0o755); err != nil {
			return err
		}
		for _, k := range slices.Sorted(maps.Keys(d.Env)) {
			spec.Env = append(spec.Env, k+"="+d.Env[k])
		}
		spec.Mounts = append(spec.Mounts, runner.Mount{Src: cache, Dst: declare.DepsMount, ReadOnly: step == "check"})
		if step == "deps" {
			spec.Network, spec.Mounts[0].ReadOnly, spec.Argv = "", true, d.Run
		}
	}
	if svc {
		sv := s.services(r, step, ts)
		spec.Network, spec.Env = sv.Network, append(spec.Env, ts.Env...)
		fault, err := s.servicesUp(r, step, sv)
		if err != nil {
			return err
		}
		if fault != "" {
			return r.Append(fs, fault)
		}
	}
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	c, err := s.Runner.Start(spec, timeout)
	if err != nil {
		return r.Append(fs, "fault: "+step+": "+err.Error())
	}
	if err := r.Append(fs, step+" started "+stamp(s.O.Now())); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "%s%s started\n", runHead(r), step)
	s.watch(r.Dir, c, nil, step)
	return nil
}

// depsCache is the swarm's dependency cache for the declared repository
// name, <state>/deps/<name>, which each of its dependency steps writes
// and each of its checks reads (section 7.2).
func (s *Swarm) depsCache(name string) string { return filepath.Join(s.state(), "deps", name) }

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
