package reconcile

import (
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
	"daisy/internal/swarm/runner"
)

// Swarm is one `swarm apply`'s view of its swarm: the declaration, the
// forge, Docker and <state> (section 11, apply), and the containers this
// process started and has not yet collected. The caller sets Cfg before
// each pass, re-read and validated (pass step 1).
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

	live      map[string]*live    // run directory → its container in flight
	given     map[string][]string // run directory → the secrets its member was given
	Stopping  func() bool         // the loop was signalled: launch nothing more (nil: never)
	newFault  bool                // a run this process collected ended in a fault
	collected int                 // runs collected in this pass
}

// live is a container this process started, a member or a worker run's
// check: done is closed once `docker run` has returned, with how it ended
// and, for a slot, the slot's secrets read again before its lock was
// released.
type live struct {
	check bool
	done  chan struct{}
	exit  runner.Exit
	err   error
	again []string
}

func (s *Swarm) stopping() bool { return s.Stopping != nil && s.Stopping() }

func (s *Swarm) state() string { return s.Cfg.Forge.State(s.Local) }
func (s *Swarm) owner() string { return owner(s.Cfg) }

func (s *Swarm) git() Git { return Git{O: s.O, WS: s.WS, State: s.state()} }

func (s *Swarm) repo(name string) (declare.Repo, bool) {
	for _, r := range s.Cfg.Repos {
		if r.Name == name {
			return r, true
		}
	}
	return declare.Repo{}, false
}

// Pass is one pass of `apply` (section 11): (2) the containers this
// process started that have ended, and every run left uncollected,
// collected; (3) the abandoned and the orphans over time; (4) paused;
// (5) each stage's issues, their pull requests given the issue's label,
// and the role rule's launches up to max_members; (6) the merge step with
// the merge gate off; (7) the owner's batch trigger; (8) the summary
// line. With wait (`--once`), every container launched is waited for and
// collected, a worker run's check included, before the summary. A forge,
// git or docker error is returned after the pass's other work.
func (s *Swarm) Pass(wait bool) error {
	if s.live == nil {
		s.live, s.given = map[string]*live{}, map[string][]string{}
	}
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
	s.collected = 0
	errs := []error{s.collectAll(running)}
	launched := 0
	if paused := s.paused(); paused != "" {
		fmt.Fprintf(s.Out, "Paused since %s (%s): launching nothing\n", strings.TrimPrefix(paused, "paused "), filepath.Join(s.state(), "paused"))
		errs = append(errs, s.stages(running, nil))
	} else {
		fetched := false
		launch := func(j *Job) error {
			if mine+launched >= s.Cfg.MaxMembers || s.paused() != "" || s.stopping() {
				return nil // a second fault in a row, or a signal, this pass launches nothing more
			}
			if !fetched {
				for _, r := range s.Cfg.Repos {
					if err := s.git().Fetch(r); err != nil {
						return err
					}
				}
				fetched = true
			}
			ok, err := s.launch(j, running)
			if ok {
				launched++
			}
			s.faultPause()
			return err
		}
		if err := s.stages(running, launch); err != nil {
			errs = append(errs, err)
		} else {
			if !s.Cfg.Gates.Merge && s.paused() == "" && !s.stopping() {
				errs = append(errs, s.mergeStep())
			}
			errs = append(errs, s.batch(running, launch))
		}
	}
	if wait {
		errs = append(errs, s.Drain(nil))
	}
	fmt.Fprintf(s.Out, "Pass %s: launched %d, collected %d, running %d\n", s.O.Now().Format("15:04:05"), launched, s.collected, mine)
	return errors.Join(errs...)
}

// ErrInterrupted: a second signal stopped the drain (section 11, apply
// without --once).
var ErrInterrupted = errors.New("interrupted")

// Drain waits for every container this process started, collecting each
// run as it ends (a worker run's check started and waited for in turn). A
// signal on stop ends it at once with ErrInterrupted, the containers left
// running.
func (s *Swarm) Drain(stop <-chan struct{}) error {
	var errs []error
	for len(s.live) > 0 {
		dirs := slices.Sorted(func(yield func(string) bool) {
			for d := range s.live {
				if !yield(d) {
					return
				}
			}
		})
		for _, dir := range dirs {
			select {
			case <-s.live[dir].done:
			case <-stop:
				return ErrInterrupted
			}
			r, _ := runOf(dir)
			if err := s.settle(r); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := s.collect(r); err != nil {
				errs = append(errs, err) // retried at the next pass
			}
		}
	}
	s.faultPause()
	return errors.Join(errs...)
}

func runOf(dir string) (Run, bool) {
	m := runName.FindStringSubmatch(filepath.Base(dir))
	if m == nil {
		return Run{}, false
	}
	n, _ := strconv.Atoi(m[1])
	k, _ := strconv.Atoi(m[3])
	return Run{Dir: dir, N: n, K: k, Role: m[2]}, true
}

// collectAll is pass steps (2) and (3): each container of this process
// that has ended recorded, each run left uncollected collected, each
// check whose apply died started again, and the runs another apply
// started marked abandoned, or stopped when they have outlived timeout.
func (s *Swarm) collectAll(running []runner.Running) error {
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	slices.SortFunc(runs, func(a, b Run) int { return strings.Compare(a.Dir, b.Dir) })
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	over := func(at string) bool {
		t, err := time.Parse(time.RFC3339, at)
		return err == nil && s.O.Now().Sub(t) > timeout
	}
	isRunning := func(name string) bool {
		return slices.ContainsFunc(running, func(c runner.Running) bool { return c.Name == name })
	}
	var errs []error
	for _, r := range runs {
		if l := s.live[r.Dir]; l != nil {
			select {
			case <-l.done:
				if err := s.settle(r); err != nil {
					errs = append(errs, err)
					continue
				}
			default:
				continue
			}
		}
		lines, err := r.Result(fs)
		if err != nil {
			return err
		}
		if _, done := Has(lines, "collected"); done {
			continue
		}
		now := stamp(s.O.Now())
		if Ended(lines) == "" {
			started, ok := Has(lines, "started")
			if !ok {
				continue
			}
			name := s.containerName(r.N, r.Role, r.K)
			if isRunning(name) {
				if !over(strings.TrimPrefix(started, "started ")) {
					continue
				}
				if err := s.Runner.Stop(name); err != nil {
					errs = append(errs, err)
					continue
				}
			}
			if err := r.Append(fs, "abandoned "+now, "outcome abandoned", "collected"); err != nil {
				return err
			}
			fs.RemoveAll(filepath.Join(r.Dir, "work"))
			continue
		}
		if at, pending := checkPending(lines); pending {
			name := s.containerName(r.N, "check", r.K)
			switch {
			case isRunning(name) && over(at):
				if err := s.Runner.Stop(name); err != nil {
					errs = append(errs, err)
					continue
				}
				if err := r.Append(fs, "check timeout "+now); err != nil {
					return err
				}
			case isRunning(name):
				continue
			default:
				if err := s.startCheck(r, lines); err != nil {
					errs = append(errs, err)
				}
				continue
			}
		}
		if err := s.collect(r); err != nil {
			errs = append(errs, err)
		}
	}
	s.faultPause()
	return errors.Join(errs...)
}

// checkPending: the run's check started and has no end line yet; at is
// when it started.
func checkPending(lines []string) (at string, pending bool) {
	l, started := "", false
	for _, line := range lines {
		if strings.HasPrefix(line, "check started ") {
			l, started = line, true // the last: a check started again
		}
	}
	if !started {
		return "", false
	}
	_, exited := Has(lines, "check exited")
	_, timedOut := Has(lines, "check timeout")
	_, fault := Has(lines, "fault")
	return strings.TrimPrefix(l, "check started "), !exited && !timedOut && !fault
}

// paused is <state>/paused's line, "" when the swarm is not paused.
func (s *Swarm) paused() string {
	b, err := s.O.FS().ReadFile(filepath.Join(s.state(), "paused"))
	if err != nil {
		return ""
	}
	l, _, _ := strings.Cut(strings.TrimRight(string(b), "\n"), "\n")
	if l == "" {
		l = "paused"
	}
	return l
}

// faultPause writes <state>/paused when the two most recently ended runs
// both ended in a swarm fault (section 7.7), and the newest was collected
// by this process: a pause the maintainer resumed is not taken again from
// the same two runs.
func (s *Swarm) faultPause() {
	if !s.newFault || s.paused() != "" {
		return
	}
	s.newFault = false
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return
	}
	type ended struct {
		at     string
		dir    string
		reason string
	}
	var es []ended
	for _, r := range runs {
		lines, _ := r.Result(fs)
		if _, done := Has(lines, "collected"); !done {
			continue
		}
		e := ended{dir: r.Dir}
		for _, l := range lines {
			f := strings.Fields(l)
			if len(f) > 1 && slices.Contains(endWords, f[0]) {
				e.at = f[len(f)-1]
			}
		}
		if l, ok := Has(lines, "fault"); ok {
			e.reason = strings.TrimPrefix(l, "fault: ")
		}
		es = append(es, e)
	}
	slices.SortFunc(es, func(a, b ended) int {
		if c := strings.Compare(a.at, b.at); c != 0 {
			return c
		}
		return strings.Compare(a.dir, b.dir)
	})
	if len(es) < 2 || es[len(es)-1].reason == "" || es[len(es)-2].reason == "" {
		return
	}
	line := fmt.Sprintf("paused %s: swarm fault: %s\n", stamp(s.O.Now()), es[len(es)-1].reason)
	if err := fs.WriteFile(filepath.Join(s.state(), "paused"), []byte(line), 0o644); err != nil {
		fmt.Fprintf(s.Err, "Error: swarm: %v\n", err)
	}
}

func (s *Swarm) containerName(n int, role string, k int) string {
	o, repo, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	return fmt.Sprintf("daisy-swarm.%s.%s.%d.%s.%d", o, repo, n, role, k)
}

// stageOrder is section 5's labels in order: every stage is read, its
// current pull request given its label, and the stages with a role
// launched.
var stageOrder = forge.Labels

// stages is pass step (5): for each label in order, each open issue's
// thread and pull requests read, its current pull request given the
// issue's label, and launch called with the job of every issue whose role
// the trigger rule says runs (section 6), by stage then number; launch nil
// launches nothing (paused).
func (s *Swarm) stages(running []runner.Running, launch func(*Job) error) error {
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	for _, label := range stageOrder {
		issues, err := s.Forge.Issues(forge.Open, label)
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
				if stages[0] == label {
					fmt.Fprintf(s.Err, "⚠ #%d: carries %s and %s; skipped\n", n, stages[0], stages[1])
				}
				continue
			}
			pulls, err := s.Forge.Pulls(n)
			if err != nil {
				return err
			}
			j := &Job{Cfg: s.Cfg, Host: s.Cfg.Forge.ForgeHost(), SwarmLogin: s.SwarmLogin, N: n, Label: label, Time: s.O.Now(), Issue: is, Pulls: pulls}
			cur, open := j.current()
			if open {
				if err := s.copyLabel(n, cur, label); err != nil {
					return err
				}
			}
			roles := stageRoles[label]
			if len(roles) == 0 || launch == nil {
				continue
			}
			review := label == "swarm:in-review" || label == "swarm:reviewed"
			if review && !open {
				fmt.Fprintf(s.Err, "⚠ #%d: %s but its current pull request is not open; skipped\n", n, label)
				continue
			}
			marks := marksOf(IssueThread(is), s.SwarmLogin)
			if open {
				if j.Pull, err = s.pullThread(cur); err != nil {
					return err
				}
				j.Check = CheckSection(fs, runs, n, cur.HeadSHA, s.repoCheck(cur.Repo), s.Cfg.Timeout)
				marks = append(marks, marksOf(j.pullEntries(cur), s.SwarmLogin)...)
			}
			turn := Turn{Label: label, Thread: IssueThread(is), Running: s.busy(running, runs, n)}
			if review {
				turn.Thread, turn.Reviews, turn.Head = j.pullEntries(cur), j.Pull.Reviews, cur.HeadSHA
			}
			role := Launch(turn, s.SwarmLogin, s.Cfg.Trusted)
			if role == "" {
				continue
			}
			persona, err := fs.ReadFile(declare.PromptFile(s.WS, role))
			if err != nil {
				return err
			}
			j.Role, j.Persona = role, string(persona)
			j.K = NextK(runs, n, role, marks)
			if err := launch(j); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Swarm) repoCheck(name string) []string {
	r, _ := s.repo(name)
	return r.Check
}

// busy is rule 2: a container of this swarm runs on issue n, or a run on
// n has an end line and no `collected`.
func (s *Swarm) busy(running []runner.Running, runs []Run, n int) bool {
	prefix := fmt.Sprintf("%s#%d/", s.Cfg.Forge.Repo, n)
	if slices.ContainsFunc(running, func(c runner.Running) bool { return strings.HasPrefix(c.Swarm, prefix) }) {
		return true
	}
	for _, r := range runs {
		if r.N != n {
			continue
		}
		if s.live[r.Dir] != nil {
			return true
		}
		lines, _ := r.Result(s.O.FS())
		if _, done := Has(lines, "collected"); Ended(lines) != "" && !done {
			return true
		}
	}
	return false
}

// copyLabel gives the issue's open current pull request the issue's stage
// label (section 5): one of its own swarm:* labels other than the issue's
// replaced, or the label added; any other label kept.
func (s *Swarm) copyLabel(n int, p forge.Pull, label string) error {
	var own []string
	for _, l := range p.Labels {
		if strings.HasPrefix(l, "swarm:") {
			own = append(own, l)
		}
	}
	if len(own) == 1 && own[0] == label {
		return nil
	}
	remove := ""
	for _, l := range own {
		if l != label {
			remove = l
			break
		}
	}
	if err := s.Forge.Relabel(forge.Ref{Repo: p.Repo, N: p.Number}, remove, label); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "#%d pull %s#%d: %s\n", n, p.Repo, p.Number, label)
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
		result, tail, ok := checkResult(fs, r, lines, timeout)
		if !ok {
			break
		}
		for i := range tail {
			tail[i] = "    " + tail[i]
		}
		return shellQuote(check) + " on " + head + ": " + result + "\n\n" + strings.Join(tail, "\n")
	}
	return "No check result for this head."
}

// checkResult is a worker run's check as it ended, `exit <code>` or
// `timeout after <timeout>`, and the last 20 lines of its check.out; ok
// is false when it has no end.
func checkResult(fs ops.FS, r Run, lines []string, timeout string) (result string, tail []string, ok bool) {
	if l, ok := Has(lines, "check exited"); ok {
		result = "exit " + strings.Fields(l)[2]
	} else if _, ok := Has(lines, "check timeout"); ok {
		result = "timeout after " + timeout
	} else {
		return "", nil, false
	}
	out, _ := fs.ReadFile(filepath.Join(r.Dir, "check.out"))
	tail = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(tail) > 20 {
		tail = tail[len(tail)-20:]
	}
	return result, tail, true
}

var (
	issuesLine = regexp.MustCompile(`^- Issues: [^/\s]+/[^/\s]+ on \S+$`)
	cloneLine  = regexp.MustCompile(`^- Clone /\S*: [^/\s]+/[^,\s]+, branch \S+, base \S+, `)
	issueHead  = regexp.MustCompile(`^# Issue #[0-9]+: \S`)
	pullHead   = regexp.MustCompile(`^# Pull request [^#\s]+#[0-9]+: \S`)
)

// emptyField: a field of `# Repository` or of an issue or pull request
// heading line in the prompt is empty (section 7.1, step 2). Only
// Daisy's own lines are read: the `- Issues:` and `- Clone ` lines of the
// `# Repository` block, and heading lines outside a fenced block (a
// quoted text's headings are shifted below `# `, section 7.3).
func emptyField(prompt string) bool {
	repository, fence := false, ""
	for _, l := range strings.Split(prompt, "\n") {
		if f := fenceOf(l); f != "" && (fence == "" || strings.HasPrefix(f, fence)) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(l, "# ") {
			repository = l == "# Repository"
		}
		switch {
		case repository && strings.HasPrefix(l, "- Issues:") && !issuesLine.MatchString(l),
			repository && strings.HasPrefix(l, "- Clone ") && !cloneLine.MatchString(l),
			strings.HasPrefix(l, "# Issue ") && !issueHead.MatchString(l),
			strings.HasPrefix(l, "# Pull request ") && !pullHead.MatchString(l):
			return true
		}
	}
	return false
}
