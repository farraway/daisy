package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/provider"
	"daisy/internal/swarm/runner"
)

// Swarm is one `swarm apply`'s view of its swarm: the declaration, the
// forge, Docker and <state> (section 11, apply). As built at Step 5 of the
// swarm build, a pass runs the architect on `swarm:ready` and
// `swarm:planning` issues; the other roles join with the lifecycle.
type Swarm struct {
	O          ops.Ops
	WS, Local  string
	Cfg        declare.Config
	Forge      forge.Adapter
	Runner     runner.Runner
	SwarmLogin string
	User       string                   // the host's uid:gid
	Secret     func(name string) string // a variable as check-secrets resolves it
	Out, Err   io.Writer
}

// built are the stages a pass acts on, with the role each runs.
var built = []struct{ label, role string }{{"swarm:ready", "architect"}, {"swarm:planning", "architect"}}

func (s *Swarm) state() string { return s.Cfg.Forge.State(s.Local) }
func (s *Swarm) owner() string { return owner(s.Cfg) }

func (s *Swarm) git() Git { return Git{O: s.O, WS: s.WS, State: s.state()} }

// started is a run this process launched.
type started struct {
	run     Run
	job     *Job
	c       runner.Container
	slot    *runner.Slot
	secrets []string
}

// Once is one pass of `apply --once` (section 11): the runs left
// uncollected and abandoned settled, the role rule over the built stages,
// launches up to max_members, then every launched run waited for and
// collected, and the summary line. A forge, git or docker error is
// returned after the pass's other work.
func (s *Swarm) Once() error {
	fs := s.O.FS()
	running, err := s.Runner.Running()
	if err != nil {
		return err
	}
	mine := 0
	for _, c := range running {
		if strings.HasPrefix(c.Swarm, s.Cfg.Forge.Repo+"#") {
			mine++
		}
	}
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	collected := 0
	var errs []error
	for _, r := range runs {
		lines, err := r.Result(fs)
		if err != nil {
			return err
		}
		if _, done := Has(lines, "collected"); done {
			continue
		}
		if Ended(lines) == "" {
			if _, ok := Has(lines, "started"); !ok || slices.ContainsFunc(running, func(c runner.Running) bool { return c.Name == s.containerName(r) }) {
				continue
			}
			if err := r.Append(fs, "abandoned "+stamp(s.O.Now()), "outcome abandoned", "collected"); err != nil {
				return err
			}
			fs.RemoveAll(filepath.Join(r.Dir, "work"))
			continue
		}
		if err := s.collect(r, nil); err != nil {
			errs = append(errs, err)
		} else {
			collected++
		}
	}
	if runs, err = Runs(fs, s.state()); err != nil {
		return err
	}

	var launched []*started
	fetched := false
	err = s.candidates(running, runs, func(j *Job) error {
		if mine+len(launched) >= s.Cfg.MaxMembers {
			return nil
		}
		if !fetched {
			for _, r := range s.Cfg.Repos {
				if err := s.git().Fetch(r); err != nil {
					return err
				}
			}
			fetched = true
		}
		st, err := s.launch(j, running)
		if st != nil {
			launched = append(launched, st)
		}
		return err
	})
	if err != nil {
		errs = append(errs, err)
	}
	for _, st := range launched {
		if err := s.finish(st); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := s.collect(st.run, st); err != nil {
			errs = append(errs, err)
		} else {
			collected++
		}
	}
	fmt.Fprintf(s.Out, "Pass %s: launched %d, collected %d, running %d\n", s.O.Now().Format("15:04:05"), countStarted(launched), collected, mine)
	return errors.Join(errs...)
}

func countStarted(ls []*started) int {
	n := 0
	for _, l := range ls {
		if l.c != nil {
			n++
		}
	}
	return n
}

func (s *Swarm) containerName(r Run) string {
	o, repo, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	return fmt.Sprintf("daisy-swarm.%s.%s.%d.%s.%d", o, repo, r.N, r.Role, r.K)
}

// candidates reads each built stage's open issues and calls launch with
// the job of every issue whose role the trigger rule says runs (section
// 6), by stage then number.
func (s *Swarm) candidates(running []runner.Running, runs []Run, launch func(*Job) error) error {
	fs := s.O.FS()
	for _, st := range built {
		issues, err := s.Forge.Issues(forge.Open, st.label)
		if err != nil {
			return err
		}
		for _, listed := range issues {
			n := listed.Number
			is, err := s.Forge.Issue(n)
			if err != nil {
				return err
			}
			var stages []string
			for _, l := range is.Labels {
				if strings.HasPrefix(l, "swarm:") {
					stages = append(stages, l)
				}
			}
			if len(stages) > 1 {
				fmt.Fprintf(s.Err, "⚠ #%d: carries %s and %s; skipped\n", n, stages[0], stages[1])
				continue
			}
			pulls, err := s.Forge.Pulls(n)
			if err != nil {
				return err
			}
			busy := slices.ContainsFunc(running, func(c runner.Running) bool {
				return strings.HasPrefix(c.Swarm, fmt.Sprintf("%s#%d/", s.Cfg.Forge.Repo, n))
			})
			for _, r := range runs {
				lines, err := r.Result(fs)
				if err != nil {
					return err
				}
				_, done := Has(lines, "collected")
				busy = busy || (r.N == n && Ended(lines) != "" && !done)
			}
			role := Launch(Turn{Label: st.label, Thread: IssueThread(is), Running: busy}, s.SwarmLogin, s.Cfg.Trusted)
			if role != st.role {
				continue
			}
			persona, err := fs.ReadFile(declare.PromptFile(s.WS, role))
			if err != nil {
				return err
			}
			j := &Job{Cfg: s.Cfg, Host: s.Cfg.Forge.ForgeHost(), SwarmLogin: s.SwarmLogin, N: n, Role: role, Label: st.label,
				Time: s.O.Now(), Persona: string(persona), Issue: is, Pulls: pulls}
			marks := marksOf(IssueThread(is), s.SwarmLogin)
			if cur, open := j.current(); open {
				if j.Pull, err = s.pullThread(cur); err != nil {
					return err
				}
				j.Check = CheckSection(fs, runs, n, cur.HeadSHA, j.repo(cur.Repo).Check, s.Cfg.Timeout)
				for _, c := range j.Pull.Comments {
					marks = append(marks, marksOf([]Entry{{c.Author, c.Body, c.Created}}, s.SwarmLogin)...)
				}
				for _, r := range j.Pull.Reviews {
					marks = append(marks, marksOf([]Entry{{r.Author, r.Body, r.Submitted}}, s.SwarmLogin)...)
				}
			}
			j.K = NextK(runs, n, role, marks)
			if err := launch(j); err != nil {
				return err
			}
		}
	}
	return nil
}

func marksOf(thread []Entry, login string) []Mark {
	var out []Mark
	for _, e := range thread {
		if m, ok := Swarms(e, login); ok {
			out = append(out, m)
		}
	}
	return out
}

func (s *Swarm) pullThread(p forge.Pull) (*PullThread, error) {
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	cs, err := s.Forge.PullComments(ref)
	if err != nil {
		return nil, err
	}
	rs, err := s.Forge.Reviews(ref)
	if err != nil {
		return nil, err
	}
	ls, err := s.Forge.ReviewComments(ref)
	if err != nil {
		return nil, err
	}
	return &PullThread{cs, rs, ls}, nil
}

// CheckSection is the `# Check` section's text for head (section 7.3):
// the check of the worker run that pushed it, or why there is none.
func CheckSection(fs ops.FS, runs []Run, n int, head string, check []string, timeout string) string {
	if len(check) == 0 {
		return "No check is declared."
	}
	for _, r := range runs {
		if r.N != n || r.Role != "worker" {
			continue
		}
		lines, _ := r.Result(fs)
		if !slices.Contains(lines, "pushed "+head) {
			continue
		}
		result := ""
		if l, ok := Has(lines, "check exited"); ok {
			f := strings.Fields(l)
			result = "exit " + f[2]
		} else if _, ok := Has(lines, "check timeout"); ok {
			result = "timeout after " + timeout
		} else {
			break
		}
		out, _ := fs.ReadFile(filepath.Join(r.Dir, "check.out"))
		tail := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if len(tail) > 20 {
			tail = tail[len(tail)-20:]
		}
		for i := range tail {
			tail[i] = "    " + tail[i]
		}
		return shellQuote(check) + " on " + head + ": " + result + "\n\n" + strings.Join(tail, "\n")
	}
	return "No check result for this head."
}

var (
	issuesLine = regexp.MustCompile(`^- Issues: [^/\s]+/[^/\s]+ on \S+$`)
	cloneLine  = regexp.MustCompile(`^- Clone /\S*: [^/\s]+/[^,\s]+, branch \S+, base \S+, `)
	issueHead  = regexp.MustCompile(`^# Issue #[0-9]+: \S`)
	pullHead   = regexp.MustCompile(`^# Pull request [^#\s]+#[0-9]+: \S`)
)

// emptyField: a field of `# Repository` or of an issue or pull request
// heading line in the prompt is empty (section 7.1, step 2).
func emptyField(prompt string) bool {
	for _, l := range strings.Split(prompt, "\n") {
		switch {
		case strings.HasPrefix(l, "- Issues:") && !issuesLine.MatchString(l),
			strings.HasPrefix(l, "- Clone ") && !cloneLine.MatchString(l),
			strings.HasPrefix(l, "# Issue ") && !issueHead.MatchString(l),
			strings.HasPrefix(l, "# Pull request ") && !pullHead.MatchString(l):
			return true
		}
	}
	return false
}

// launch prepares and starts the run of j (section 7.1): its directory and
// prompt files, read back; the clones; the credential; the argv and the
// result's first lines. A run that cannot launch is recorded and returned
// with no container; nil when no slot is free.
func (s *Swarm) launch(j *Job, running []runner.Running) (*started, error) {
	fs := s.O.FS()
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-%s-%d", j.N, j.Role, j.K)), N: j.N, K: j.K, Role: j.Role}
	ioDir := filepath.Join(r.Dir, "io")
	if err := fs.MkdirAll(ioDir, 0o755); err != nil {
		return nil, err
	}
	st := &started{run: r, job: j}
	now := func() string { return stamp(s.O.Now()) }
	fault := func(reason string) (*started, error) {
		return st, r.Append(fs, "launch-failed "+now(), "fault: "+reason)
	}
	system, prompt := Render(j)
	for _, f := range []struct{ name, text string }{{"system.md", system}, {"prompt.md", prompt}} {
		p := filepath.Join(ioDir, f.name)
		if err := fs.WriteFile(p, []byte(f.text), 0o644); err != nil {
			return nil, err
		}
		back, err := fs.ReadFile(p)
		if err != nil || !bytes.Equal(back, []byte(f.text)) {
			return fault("prompt: " + f.name + ": differs")
		}
	}
	if emptyField(prompt) {
		return fault("prompt: prompt.md: empty field")
	}

	persona := s.Cfg.Personas[j.Role]
	work := filepath.Join(r.Dir, "work")
	var startLines []string
	for _, repo := range s.Cfg.Repos {
		sha, err := s.git().Clone(repo, s.Cfg.Base, s.Cfg.Base, filepath.Join(work, repo.Name), j.Role, s.Cfg.Forge.CommitEmail, persona.CoAuthor)
		if err != nil {
			fs.RemoveAll(r.Dir)
			return nil, err
		}
		startLines = append(startLines, "start "+repo.Name+" "+sha)
	}
	if err := fs.MkdirAll(filepath.Join(work, ".swarm"), 0o755); err != nil {
		return nil, err
	}

	o, repo, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	first := s.Cfg.Repos[0]
	workdir := "/work/" + first.Name
	reads := []string{"/work/.swarm"}
	for _, rp := range s.Cfg.Repos[1:] {
		reads = append(reads, "/work/"+rp.Name)
	}
	mounts := []runner.Mount{{Src: work, Dst: "/work"}, {Src: ioDir, Dst: "/work/.swarm"}}
	for i, res := range ResourceMounts(s.Cfg.Resources) {
		src, err := fs.EvalSymlinks(filepath.Join(s.WS, s.Cfg.Resources[i]))
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, runner.Mount{Src: src, Dst: res, ReadOnly: true})
		reads = append(reads, res)
	}
	spec := runner.Spec{Name: s.containerName(r), Label: fmt.Sprintf("%s/%s#%d/%s", o, repo, j.N, j.Role), User: s.User,
		Mounts: mounts, Workdir: workdir, Image: first.Image,
		Argv: provider.Providers[persona.Provider].Adapter.Command(persona.Model, persona.Effort, workdir, "/work/.swarm/system.md", "/work/.swarm/prompt.md", outside(workdir, reads))}

	key := s.Cfg.Keys[persona.Provider]
	credential := ""
	if key.Env != nil {
		spec.EnvVar, spec.EnvValue = *key.Env, s.Secret(*key.Env)
		st.secrets = []string{spec.EnvValue}
		credential = "credential " + persona.Provider + " env " + *key.Env
	} else {
		holder := fmt.Sprintf("%s#%d/%s run %d since %s", s.Cfg.Forge.Repo, j.N, j.Role, j.K, now())
		slot, err := runner.TakeSlot(fs, s.Runner, s.slotDir(persona.Provider), persona.Provider, *key.Slots, holder, running)
		var refused *runner.SlotRefused
		switch {
		case errors.Is(err, runner.ErrNoSlot):
			return nil, fs.RemoveAll(r.Dir)
		case errors.As(err, &refused):
			st.run = r
			return fault(refused.Error())
		case err != nil:
			fs.RemoveAll(r.Dir)
			return nil, err
		}
		st.slot, st.secrets = slot, slot.Secrets
		spec.Credential, spec.Slot = slot.Credential, slot.Dir
		credential = fmt.Sprintf("credential %s slot %d", persona.Provider, slot.J)
	}
	if err := fs.WriteFile(filepath.Join(r.Dir, "argv"), []byte(strings.Join(append([]string{"docker"}, spec.Args()...), "\n")+"\n"), 0o644); err != nil {
		return st, err
	}
	lines := append(startLines, "label "+j.Label, "started "+now(), credential)
	if err := r.Append(fs, lines...); err != nil {
		return st, err
	}
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	c, err := s.Runner.Start(spec, timeout)
	if err != nil {
		if st.slot != nil {
			st.slot.Release()
		}
		return st, r.Append(fs, "launch-failed "+now(), "fault: launch: "+err.Error())
	}
	st.c = c
	fmt.Fprintf(s.Out, "#%d %s run %d: launched\n", j.N, j.Role, j.K)
	return st, nil
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

// finish waits for a started container and records how it ended
// (sections 2 and 7.7): its streams, the end line, its cost, and a
// failure or a fault; the slot is released once docker run has returned.
func (s *Swarm) finish(st *started) error {
	if st.c == nil {
		return nil
	}
	fs := s.O.FS()
	exit, err := st.c.Wait()
	if st.slot != nil {
		if again, err := runner.SlotSecrets(fs, st.slot.Dir); err == nil {
			st.secrets = append(st.secrets, again...)
		}
		st.slot.Release()
	}
	r := st.run
	now := stamp(s.O.Now())
	if err != nil {
		return r.Append(fs, "exited -1 "+now, "fault: wait: "+err.Error())
	}
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
