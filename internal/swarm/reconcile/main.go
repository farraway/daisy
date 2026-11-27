package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/fence"
	"daisy/internal/swarm/forge"
)

// The red main (section 12.1, rule 4; decision 169): a commit of base is
// green when its tree is the tree of a commit a check passed on, a merge
// check's or a main check's, so a merge the forge makes of a passed head
// is green with no run; a commit whose tree no check has passed or failed
// is checked by a main check, `<state>/runs/0-main-<k>`, one per
// repository at a time. A main check that fails makes base red until a
// commit of base is green again: no worker launches on the repository and
// its queue checks no head, but for the fix's. One that fails in the
// environment after its rerun leaves base unchecked, a line of
// <state>/main-unchecked, until the bare `swarm resume`.

// mainRef is the mirror's ref a main check clones: the base commit it
// checks, set as it starts, so that a rerun clones that commit whatever
// base has moved to since.
const mainRef = "refs/swarm-main"

// mainLine is a main check's first result line, `main <name> <sha> tree
// <tree>` (section 2).
type mainLine struct{ name, sha, tree string }

func (m mainLine) String() string { return fmt.Sprintf("main %s %s tree %s", m.name, m.sha, m.tree) }

// mainOf is the main check's first result line, ok false when lines hold
// none.
func mainOf(lines []string) (m mainLine, ok bool) {
	l, ok := Has(lines, "main")
	f := strings.Fields(l)
	if !ok || len(f) != 5 || f[3] != "tree" {
		return mainLine{}, false
	}
	return mainLine{f[1], f[2], f[4]}, true
}

// noMember: the run is one no member runs, a merge check or a main check
// (section 12.1): its steps alone, no metrics row, no cause.
func noMember(r Run) bool { return r.Role == "merge" || r.Role == "main" }

// Verdicts is what the checks under <state> found of each repository's
// trees (section 12.1, rule 4): the trees a merge check or a main check
// passed on, by `<name> <tree>` with the run that passed it; the trees a
// main check failed on; and the repositories whose main check is not yet
// collected.
type Verdicts struct {
	Passed  map[string]Run
	Failed  map[string]bool
	Running map[string]bool
}

// ReadVerdicts reads the Verdicts from the merge checks' and the main
// checks' results under state; the other runs are not read. A merge check
// recorded before its `tree` line was written names no tree.
func ReadVerdicts(fs ops.FS, state string) (Verdicts, error) {
	v := Verdicts{Passed: map[string]Run{}, Failed: map[string]bool{}, Running: map[string]bool{}}
	runs, err := Runs(fs, state)
	if err != nil {
		return v, err
	}
	for _, r := range runs {
		if !noMember(r) {
			continue
		}
		lines, err := r.Result(fs)
		if err != nil {
			return v, err
		}
		_, passed := Has(lines, "passed")
		var key string
		if r.Role == "merge" {
			m, ok := mergeOf(lines)
			t, _ := Has(lines, "tree")
			if !ok || t == "" {
				continue
			}
			key = m.name + " " + strings.TrimPrefix(t, "tree ")
		} else {
			m, ok := mainOf(lines)
			if !ok {
				continue
			}
			key = m.name + " " + m.tree
			if _, done := Has(lines, "collected"); !done {
				v.Running[m.name] = true
			}
			if l, _ := Has(lines, "failed"); strings.HasPrefix(l, "failed: main check: ") {
				v.Failed[key] = true
			}
		}
		if passed {
			if o, seen := v.Passed[key]; !seen || o.Dir > r.Dir {
				v.Passed[key] = r // the first by name, whichever pass reads it
			}
		}
	}
	return v, nil
}

// MainState is how a repository's base stands (section 12.1, rule 4):
// the commit the mirror holds and its tree, whether it is green, red
// (its tree failed, or, its tree with no verdict, the newest commit with
// one on its first-parent line failed), unchecked (a line of
// <state>/main-unchecked names the commit) or being checked; By is the
// run whose check passed its tree, for a green commit. A repository with
// no check, or whose mirror holds no base, has no state ("").
type MainState struct {
	Name, SHA, Tree string
	Green, Red      bool
	Unchecked       bool
	Running         bool
	By              Run
	// Breaking is, base red, the breaking merges: the merge commits on
	// its first-parent line after its newest green commit, newest first;
	// Job is the breaking job, the issue of the newest of them whose
	// second parent a worker run of the swarm pushed, 0 for none.
	Breaking []BreakingMerge
	Job      int
}

// BreakingMerge is a merge commit on a red base's first-parent line
// after its newest green commit (section 12.1, The red main): its
// second parent, and the issue and slice of the worker run that pushed
// it (swarm/<n>-<s>; 0 for a merge no worker run of the swarm pushed).
type BreakingMerge struct {
	SHA, Head string
	N, Slice  int
}

// State is the word `status` names it by: green, red, unchecked or
// checking (a main check running or due).
func (m MainState) State() string {
	switch {
	case m.Green:
		return "green"
	case m.Red:
		return "red"
	case m.Unchecked:
		return "unchecked"
	}
	return "checking"
}

// ReadMainState reads repository name's MainState from its mirror under
// state, as its last fetch left it, with the verdicts v.
func ReadMainState(o ops.Ops, state, base, name string, v Verdicts) (MainState, error) {
	g := Git{O: o, State: state}
	m := g.Mirror(name)
	sha := g.Head(m, "refs/heads/"+base)
	if sha == "" {
		return MainState{}, nil
	}
	tree, err := g.run(m, "rev-parse", sha+"^{tree}")
	if err != nil {
		return MainState{}, err
	}
	st := MainState{Name: name, SHA: sha, Tree: tree, Running: v.Running[name]}
	if r, ok := v.Passed[name+" "+tree]; ok {
		st.Green, st.By = true, r
		return st, nil
	}
	unchecked, err := readUnchecked(o.FS(), state)
	if err != nil {
		return st, err
	}
	st.Unchecked = slices.Contains(unchecked, name+" "+sha)
	// Red until a commit of base is green again: the newest commit with a
	// verdict on the first-parent line decides, the commit's own first.
	line, err := firstParents(g, name, sha, v)
	if err != nil {
		return st, err
	}
	for _, c := range line {
		if v.Failed[name+" "+c.tree] {
			st.Red = true
			break
		}
	}
	return st, st.breaking(o.FS(), state, line)
}

// breaking reads, base red, its breaking merges and the breaking job from
// line, its first-parent line after its newest green commit, and the
// heads the worker runs under state pushed to the repository (their
// `pushed <sha>` lines, the scorecard's reading, ReadMerges).
func (st *MainState) breaking(fs ops.FS, state string, line []parentCommit) error {
	if !st.Red {
		return nil
	}
	var merges []BreakingMerge
	for _, c := range line {
		if len(c.parents) > 1 {
			merges = append(merges, BreakingMerge{SHA: c.sha, Head: c.parents[1]})
		}
	}
	if len(merges) > 0 {
		runs, err := Runs(fs, state)
		if err != nil {
			return err
		}
		pushed := map[string]Run{}
		for _, r := range runs {
			if r.Role != "worker" {
				continue
			}
			lines, err := r.Result(fs)
			if err != nil {
				return err
			}
			if p, ok := Has(lines, "pushed"); ok && branchRepo(lines) == st.Name {
				pushed[strings.TrimPrefix(p, "pushed ")] = r
			}
		}
		for i, m := range merges {
			if r, ok := pushed[m.Head]; ok {
				lines, _ := r.Result(fs)
				merges[i].N, merges[i].Slice = r.N, sliceOf(forge.Pull{Head: runBranch(lines)}, r.N)
				if st.Job == 0 {
					st.Job = r.N
				}
			}
		}
	}
	st.Breaking = merges
	return nil
}

// parentCommit is a commit of base's first-parent line: its tree and its
// parents.
type parentCommit struct {
	sha, tree string
	parents   []string
}

// firstParents is base's first-parent line from sha back to the newest
// commit whose tree a check passed (green), that commit left out: the
// line whose merges are the breaking merges when base is red; the whole
// line when no commit of it is green.
func firstParents(g Git, name, sha string, v Verdicts) ([]parentCommit, error) {
	out, err := g.run(g.Mirror(name), "log", "--first-parent", "--format=%H %T %P", sha, "--")
	if err != nil {
		return nil, err
	}
	var line []parentCommit
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		if _, green := v.Passed[name+" "+f[1]]; green {
			break
		}
		line = append(line, parentCommit{f[0], f[1], f[2:]})
	}
	return line, nil
}

// uncheckedFile is <state>/main-unchecked (section 2).
func uncheckedFile(state string) string { return filepath.Join(state, "main-unchecked") }

// readUnchecked is <state>/main-unchecked's lines, `<name> <sha>`.
func readUnchecked(fs ops.FS, state string) ([]string, error) {
	b, err := fs.ReadFile(uncheckedFile(state))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

// mainState is repository name's MainState as the mirror holds it now,
// read once a pass until a check's collection or start changes it.
func (s *Swarm) mainState(name string) (MainState, error) {
	if r, ok := s.repo(name); !ok || len(r.Check) == 0 {
		return MainState{}, nil
	}
	if s.mains == nil {
		v, err := ReadVerdicts(s.O.FS(), s.state())
		if err != nil {
			return MainState{}, err
		}
		s.verdicts, s.mains = v, map[string]MainState{}
	}
	if st, ok := s.mains[name]; ok {
		return st, nil
	}
	st, err := ReadMainState(s.O, s.state(), s.Cfg.Base, name, s.verdicts)
	if err != nil {
		return st, err
	}
	s.mains[name] = st
	return st, nil
}

// mainsChanged forgets the pass's MainStates: a check started or was
// collected.
func (s *Swarm) mainsChanged() { s.mains = nil }

// mainChecks is rule 4's start of the main checks, after the merge step
// and before the merge queue (section 12.1): for each repository with a
// check that the merge step merged nothing into, whose base commit, the
// mirrors fetched, has a tree no check has passed or failed, is named by
// no line of <state>/main-unchecked and has no main check running, a
// main check on that commit. Nothing is read when no check can start (a
// member slot free, the swarm not paused). Then a green base whose
// passed merge check was started on a red one queues its journal line
// (greenAgain). It returns the checks started.
func (s *Swarm) mainChecks(merged map[string]bool) (int, error) {
	if s.launchesNothing() {
		return 0, nil
	}
	var names []string
	for _, r := range s.Cfg.Repos {
		if len(r.Check) > 0 && !merged[r.Name] {
			names = append(names, r.Name)
		}
	}
	if len(names) == 0 {
		return 0, nil
	}
	if err := s.fetch(); err != nil {
		return 0, err
	}
	started := 0
	for _, name := range names {
		st, err := s.mainState(name)
		if err != nil {
			return started, err
		}
		if st.Green {
			if err := s.greenAgain(st); err != nil {
				return started, err
			}
		}
		if st.SHA == "" || st.Green || st.Running || st.Unchecked || s.verdicts.Failed[name+" "+st.Tree] || s.launchesNothing() {
			continue
		}
		ok, err := s.startMain(st)
		if err != nil {
			return started, err
		}
		if ok {
			started++
		}
	}
	return started, nil
}

// greenAgain queues the journal line of a base made green by the merge
// of a head whose merge check passed on a red base (its result's `red`
// line), once: `<n>-merge-<k>: <name> green at <short sha>`, the run
// then recording `green <sha>`.
func (s *Swarm) greenAgain(st MainState) error {
	if st.By.Role != "merge" {
		return nil
	}
	fs := s.O.FS()
	lines, err := st.By.Result(fs)
	if err != nil {
		return err
	}
	_, red := Has(lines, "red")
	if _, told := Has(lines, "green"); !red || told {
		return nil
	}
	if err := st.By.Append(fs, "green "+st.SHA); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "%s of %s green at %s: its merge check passed\n", s.Cfg.Base, st.Name, short(st.SHA))
	return s.QueueJournal(filepath.Base(st.By.Dir), fmt.Sprintf("%s green at %s", st.Name, short(st.SHA)))
}

// startMain starts a main check on st's base commit (section 12.1, rule
// 4): the run `0-main-<k>` records the commit and its tree, the
// repository as a worker run's branch line names it, the commit fetched,
// and `red` when base is red; the commit is set at refs/swarm-main in the
// mirror, and the check's first step starts on it as a merge check's
// does. ok is whether a container started.
func (s *Swarm) startMain(st MainState) (bool, error) {
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return false, err
	}
	g := s.git()
	if _, err := g.run(g.Mirror(st.Name), "update-ref", mainRef, st.SHA); err != nil {
		return false, err
	}
	k := NextK(runs, 0, "main", nil)
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("0-main-%d", k)), N: 0, K: k, Role: "main"}
	if err := fs.MkdirAll(r.Dir, 0o755); err != nil {
		return false, err
	}
	lines := []string{mainLine{st.Name, st.SHA, st.Tree}.String(), "branch " + s.Cfg.Base + " in " + st.Name, "fetched " + st.SHA}
	if st.Red {
		lines = append(lines, "red")
	}
	if err := r.Append(fs, lines...); err != nil {
		return false, err
	}
	s.mainsChanged()
	fmt.Fprintf(s.Out, "main check of %s: %s at %s\n", st.Name, s.Cfg.Base, short(st.SHA))
	repo, _ := s.repo(st.Name)
	if err := s.startStep(r, lines, nextStep(repo, lines)); err != nil {
		return false, err
	}
	if s.live[r.Dir] == nil {
		return false, s.collect(r) // the step could not start: a fault
	}
	return true, nil
}

// collectMain collects a main check whose step has ended (section 12.1,
// rule 4), as collectMerge collects a merge check: its next step
// started; a pass recorded `passed`, with the journal line `0-main-<k>:
// <name> green at <short sha>` when base was red; a classified failure
// run again once (7.9) and, again classified, base left unchecked
// (mainEnvironment); any other failure `failed: main check: <result>`,
// the whole output kept as the run's check.out, the journal line
// `0-main-<k>: <name> red at <short sha>`, then the fix routed
// (routeFix). A fault is checked again at the next pass.
func (s *Swarm) collectMain(r Run, lines []string) error {
	if step, _ := pending(lines); step != "" {
		return nil
	}
	lines, err := s.servicesDown(r, lines)
	if err != nil {
		return err
	}
	m, _ := mainOf(lines)
	repo, _ := s.repo(m.name)
	c := &collection{s: s, r: r, lines: lines, head: runHead(r), at: forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: 0}}
	if l, ok := Has(lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		return c.mainEnd()
	}
	if l, failed := Has(lines, "failed"); failed { // recorded, the rest not done
		if reason, env := strings.CutPrefix(l, "failed: "+checkEnvironment); env {
			return c.mainEnvironment(reason)
		}
		return c.mainFailed(m)
	}
	if step := nextStep(repo, lines); step != "" {
		return c.startCheck(step)
	}
	result, out, _, _ := checkResult(s.O.FS(), r, lines, s.Cfg.Timeout)
	if result == "exit 0" {
		fmt.Fprintf(s.Out, "%smain check of %s passed (%s at %s)\n", c.head, m.name, s.Cfg.Base, short(m.sha))
		if err := c.step("passed"); err != nil {
			return err
		}
		if _, red := Has(lines, "red"); red {
			if err := s.QueueJournal(filepath.Base(r.Dir), fmt.Sprintf("%s green at %s", m.name, short(m.sha))); err != nil {
				return err
			}
		}
		return c.mainEnd()
	}
	if held, err := c.classify(repo, out); err != nil || held {
		return err
	}
	if err := c.step("failed: main check: " + result); err != nil {
		return err
	}
	return c.mainFailed(m)
}

// mainFailed is a red base (section 12.1, rule 4): the journal line
// queued, then the fix routed, then the run collected.
func (c *collection) mainFailed(m mainLine) error {
	s := c.s
	l, _ := Has(c.lines, "failed")
	fmt.Fprintf(s.Out, "%smain check of %s failed (%s): %s red at %s\n", c.head, m.name, strings.TrimPrefix(l, "failed: main check: "), s.Cfg.Base, short(m.sha))
	if err := s.QueueJournal(filepath.Base(c.r.Dir), fmt.Sprintf("%s red at %s", m.name, short(m.sha))); err != nil {
		return err
	}
	return c.mainEnd()
}

// mainEnvironment is a main check whose failure is classified again on
// its rerun (section 7.9): base left unchecked, the line `<name> <sha>`
// added to <state>/main-unchecked, no note and no block.
func (c *collection) mainEnvironment(reason string) error {
	s, fs := c.s, c.s.O.FS()
	if _, ok := Has(c.lines, "failed"); !ok {
		if err := c.step("failed: " + checkEnvironment + reason); err != nil {
			return err
		}
	}
	m, _ := mainOf(c.lines)
	fmt.Fprintf(s.Out, "%sfailed (%s%s): %s of %s unchecked at %s\n", c.head, checkEnvironment, reason, s.Cfg.Base, m.name, short(m.sha))
	lines, err := readUnchecked(fs, s.state())
	if err != nil {
		return err
	}
	if line := m.name + " " + m.sha; !slices.Contains(lines, line) {
		old, err := fs.ReadFile(uncheckedFile(s.state()))
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return err
		}
		if err := fs.WriteFile(uncheckedFile(s.state()), append(old, line+"\n"...), 0o644); err != nil {
			return err
		}
	}
	return c.mainEnd()
}

// mainEnd records the main check `collected` and removes its clone and
// its step's files, keeping its check.out (mergeEnd).
func (c *collection) mainEnd() error { return c.mergeEnd() }

// mainChanged is a main check's change, which 7.9's classification reads
// failing packages against (section 12.1, rule 4): the paths base's
// commits since its newest green commit change up to sha; every path of
// sha when no commit of base is green.
func (s *Swarm) mainChanged(name, sha string) ([]string, error) {
	v, err := ReadVerdicts(s.O.FS(), s.state())
	if err != nil {
		return nil, err
	}
	g := s.git()
	line, err := firstParents(g, name, sha, v)
	if err != nil {
		return nil, err
	}
	if n := len(line); n > 0 && len(line[n-1].parents) > 0 {
		return g.paths(name, line[n-1].parents[0], sha)
	}
	out, err := g.run(g.Mirror(name), "ls-tree", "-r", "-z", "--name-only", sha)
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == 0 }), nil
}

// fixes: issue is is the fix's of st's red base (section 12.1, The red
// main): the breaking job's issue; or, that job closed or none, an issue
// whose body the swarm created with the line `Fixes: <name>` (FixesLine),
// the breaking job's issue read once a pass, and only for such an issue.
func (s *Swarm) fixes(st MainState, is forge.Issue) (bool, error) {
	if st.Job != 0 && is.Number == st.Job {
		return true, nil
	}
	if !FixesLine(is, s.SwarmLogin, st.Name) {
		return false, nil
	}
	if st.Job == 0 {
		return true, nil
	}
	open, ok := s.fixOpen[st.Job]
	if !ok {
		b, err := s.Forge.Issue(st.Job)
		if err != nil {
			return false, err
		}
		open = b.Open
		if s.fixOpen != nil {
			s.fixOpen[st.Job] = open
		}
	}
	return !open, nil
}

// FixesLine: the body of issue is, a proposal the swarm created (marked
// role=owner), holds the line `Fixes: <name>` outside a fence (section
// 12.1, The red main).
func FixesLine(is forge.Issue, swarmLogin, name string) bool {
	body := bodyEntry(is)
	if m, ok := Swarms(body, swarmLogin); !ok || m.Role != "owner" {
		return false
	}
	var fences fence.Scan
	for _, l := range strings.Split(swarmText(body.Body), "\n") {
		if !fences.Line(l) && strings.TrimSpace(l) == "Fixes: "+name {
			return true
		}
	}
	return false
}

// redHolds: the worker of job j, on repository name, is held by its red
// base (section 12.1, rule 4; section 6, rule 1): base red, the mirror
// fetched as the launch would fetch it, and j not the fix's. A
// repository with no check reads nothing.
func (s *Swarm) redHolds(j *Job, name string) (bool, error) {
	if r, ok := s.repo(name); !ok || len(r.Check) == 0 {
		return false, nil
	}
	if err := s.fetch(); err != nil {
		return false, err
	}
	st, err := s.mainState(name)
	if err != nil || !st.Red {
		return false, err
	}
	fix, err := s.fixes(st, j.Issue)
	return !fix, err
}
