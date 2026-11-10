package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// maxPost is the longest text posted (section 7.5).
const maxPost = 60000

// collection is one run's collection (section 7.6): each step first
// checked on the forge and skipped when done there, each step done
// appended to the result, so a collection interrupted by an error is
// retried whole at the next pass without posting twice.
type collection struct {
	s     *Swarm
	r     Run
	lines []string
	head  string // the run's prefix on stdout
	label string // the stage the run was launched on, by its current name
	is    forge.Issue
	at    forge.Ref // the issue
}

// collect posts what an ended run produced and moves the label (section
// 7.6); a worker run whose check is due starts it and is collected once
// the check has ended.
func (s *Swarm) collect(r Run) error {
	lines, err := r.Result(s.O.FS())
	if err != nil {
		return err
	}
	if _, done := Has(lines, "collected"); done || Ended(lines) == "" {
		return nil
	}
	if _, pending := checkPending(lines); pending {
		return nil
	}
	labelLine, _ := Has(lines, "label")
	// A run recorded before the stages' rename names its stage by the old
	// name, which the forge no longer holds (forge.Renamed).
	c := &collection{s: s, r: r, lines: lines, head: runHead(r), label: forge.Current(strings.TrimPrefix(labelLine, "label ")),
		at: forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: r.N}}
	if Ended(lines) == "abandoned" {
		return c.abandoned()
	}
	if err := c.notes(); err != nil {
		return err
	}
	if r.N == 0 {
		return c.batch()
	}
	worker := r.Role == "worker" && Ended(lines) != "launch-failed"
	if worker {
		if err := c.fetch(); err != nil {
			return err
		}
	}
	// A worker's second failed fetch is a failure of the work even after
	// a fault: its issue is blocked, not relaunched on a clone that cannot
	// be read.
	if l, ok := Has(c.lines, "fault"); ok && !(worker && len(fetchFailures(c.lines)) >= 2) {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		if worker {
			if err := c.keep(); err != nil {
				return err
			}
		}
		if err := c.restore(); err != nil {
			return err
		}
		return c.end("fault")
	}
	if c.is, err = s.Forge.Issue(r.N); err != nil {
		return err
	}
	if !c.is.Open {
		fmt.Fprintf(s.Err, "⚠ #%d closed during the run; nothing posted\n", r.N)
		return c.end("discarded")
	}
	var pull *forge.Pull
	if r.Role == "worker" || r.Role == "adversary" || r.Role == "sympathetic" {
		pulls, err := s.pulls(r.N)
		if err != nil {
			return err
		}
		pull = c.runPull(pulls)
		onPull := r.Role != "worker" || c.label == forge.ChangesRequested
		if pull != nil && !pull.Open || pull == nil && onPull && Ended(lines) != "launch-failed" {
			if err := c.restore(); err != nil {
				return err
			}
			return c.end("discarded")
		}
		if start, _ := Has(lines, "start"); !worker && pull != nil && "start "+pull.HeadSHA != start {
			if _, ok := Has(c.lines, "stale"); !ok {
				if err := c.step("stale " + pull.HeadSHA); err != nil {
					return err
				}
			}
			if err := c.restore(); err != nil {
				return err
			}
			return c.end("stale")
		}
	}
	if l, ok := Has(c.lines, "failed"); ok {
		return c.fail(strings.TrimPrefix(l, "failed: "))
	}
	out, err := r.Output(s.O.FS())
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return c.fail("no output")
	}
	if hit(out, s.secretsOf(r, c.lines)) {
		return c.fail("secret in output: out.md")
	}
	text := postable(string(out), r, 0)
	first, _, _ := strings.Cut(string(out), "\n")
	if strings.HasPrefix(strings.TrimRight(first, "\r "), "BLOCKED:") {
		return c.blocked(text, pull)
	}
	switch r.Role {
	case "owner":
		if err := c.comment(r.Role, text); err != nil {
			return err
		}
		if c.label == forge.ProposalPosted && !s.Cfg.Gates.Triage {
			if err := c.relabel(forge.ProposalAccepted); err != nil {
				return err
			}
		}
	case "architect":
		if _, bad := SliceRepos(string(out), repoNames(s)); len(bad) > 0 {
			return c.fail("bad slices: " + numbers(bad))
		}
		if err := c.comment(r.Role, text); err != nil {
			return err
		}
		if c.label == forge.ProposalAccepted {
			to := forge.PlanPosted
			if !s.Cfg.Gates.Plan {
				to = forge.PlanAccepted
			}
			if err := c.relabel(to); err != nil {
				return err
			}
		}
	case "worker":
		return c.worker(string(out), text, pull)
	default:
		return c.review(string(out), text, *pull)
	}
	return c.end("posted")
}

func (c *collection) step(line string) error {
	if err := c.r.Append(c.s.O.FS(), line); err != nil {
		return err
	}
	c.lines = append(c.lines, line)
	return nil
}

// end records the outcome and `collected`, and removes the run's clones
// and its checkout of the resources.
func (c *collection) end(outcome string) error {
	fs := c.s.O.FS()
	if err := c.r.Append(fs, "outcome "+outcome, "collected"); err != nil {
		return err
	}
	c.s.collected++
	if outcome == "fault" {
		c.s.faulted = append(c.s.faulted, c.r.Dir)
	}
	delete(c.s.given, c.r.Dir)
	return errors.Join(removeTree(fs, filepath.Join(c.r.Dir, "work")), removeTree(fs, filepath.Join(c.r.Dir, "check")), removeTree(fs, filepath.Join(c.r.Dir, "resources")), c.dropNotes())
}

// removeTree removes a tree a member or a check wrote: when RemoveAll
// fails (a directory left unreadable, mode 000, cannot be listed), each
// directory in it, never through a link, is given the owner's rights,
// then the removal is made again (section 7.6).
func removeTree(fs ops.FS, dir string) error {
	if fs.RemoveAll(dir) == nil {
		return nil
	}
	var open func(string)
	open = func(d string) {
		fs.Chmod(d, 0o700)
		entries, _ := fs.ReadDir(d)
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if fi, err := fs.Lstat(p); err == nil && fi.IsDir() {
				open(p)
			}
		}
	}
	open(dir)
	return fs.RemoveAll(dir)
}

// dropNotes removes the run's copy of the notes, kept when they were not
// pushed (`notes kept`), for the maintainer.
func (c *collection) dropNotes() error {
	if _, kept := Has(c.lines, "notes kept"); kept {
		return nil
	}
	return c.s.O.FS().RemoveAll(filepath.Join(c.r.Dir, "notes"))
}

// say posts text marked for role and this run at at, unless existing (its
// comments) holds it: a reconciler note is told by its whole body, since
// its marker carries the reported run's number.
func (c *collection) say(at forge.Ref, existing []forge.Comment, role, text string) error {
	body := marker(role, c.r.K) + text
	for _, e := range existing {
		m, ok := Swarms(Entry{e.Author, e.Body, e.Created}, c.s.SwarmLogin)
		if ok && m.Role == role && m.Run == c.r.K && (role != "reconciler" || e.Body == body) {
			return nil
		}
	}
	if err := c.s.Forge.Comment(at, body); err != nil {
		return err
	}
	fmt.Fprintln(c.s.Out, c.head+"posted comment")
	return c.step("posted comment")
}

// comment posts on the issue.
func (c *collection) comment(role, text string) error { return c.say(c.at, c.is.Comments, role, text) }

// stage is the stage of the run the issue carries now: the in-progress
// stage its launch wrote (InProgress), else the stage it was launched on;
// "" when it carries neither, the maintainer (or, for the pair's second
// reviewer, the first's T22) having moved it. Its home is not its stage:
// the pair's second reviewer, launched on review-1-in-progress, finds the
// issue at review-0-ready only when it was moved there during the run.
func (c *collection) stage() string {
	for _, l := range []string{InProgress(c.r.Role, c.label), c.label} {
		if l != "" && slices.Contains(c.is.Labels, l) {
			return l
		}
	}
	return ""
}

// relabel moves the open issue from the run's stage (stage) to to; an
// issue the maintainer moved during the run is skipped with a warning, a
// move already made is skipped, and a closed issue is left as it is.
func (c *collection) relabel(to string) error {
	// The labels as they are now, read right before the write: the
	// maintainer may have moved the issue while the run was collected (the
	// forge has no compare-and-set, so this narrows the window to the two
	// calls, section 5).
	is, err := c.s.Forge.Issue(c.r.N)
	if err != nil {
		return err
	}
	c.is.Labels = is.Labels
	from := c.stage()
	switch {
	case !is.Open, from == to:
		return nil
	case from != "":
		if err := c.s.Forge.Relabel(c.at, from, to); err != nil {
			return err
		}
		c.s.moved(c.r.N, to)
		c.is.Labels = append(slices.DeleteFunc(slices.Clone(c.is.Labels), func(l string) bool { return l == from }), to)
		fmt.Fprintf(c.s.Out, "%s%s → %s\n", c.head, from, to)
		return c.step("relabelled " + from + " " + to)
	case slices.Contains(c.is.Labels, to):
		return nil
	}
	now := ""
	for _, l := range c.is.Labels {
		if forge.IsStage(l) {
			now = l
		}
	}
	fmt.Fprintf(c.s.Err, "⚠ #%d: relabelled to %s during the run; not relabelled\n", c.r.N, now)
	return nil
}

// movedAway: the open issue, read again, carries neither the run's stage
// nor, with home, the run's home, the maintainer having moved it during
// the run, so a flag set there (flag) is skipped; now is the stage it
// carries, "" for none.
func (c *collection) movedAway(home bool) (now string, moved bool, err error) {
	is, err := c.s.Forge.Issue(c.r.N)
	if err != nil {
		return "", false, err
	}
	c.is.Labels = is.Labels
	if !is.Open || c.stage() != "" || home && slices.Contains(is.Labels, c.home()) {
		return "", false, nil
	}
	now, _ = StageOf(is)
	return now, true, nil
}

// notBlocked ends a reconciler note that blocked nothing, the issue moved
// during the run to now (movedAway).
func notBlocked(now string) string {
	if now == "" {
		now = "no stage"
	}
	return "The issue was moved to " + now + " during the run" + notBlockedTail
}

// notBlockedTail is notBlocked's end, which blocks reads.
const notBlockedTail = ", so it is not blocked."

// home is where a run that does not move the issue on leaves it (section
// 5, T22): review-0-ready for a reviewer, whichever of the pair it was,
// and the stage it was launched on for every other role.
func (c *collection) home() string {
	if (c.r.Role == "adversary" || c.r.Role == "sympathetic") && reviewing(c.label) {
		return forge.ReviewReady
	}
	return c.label
}

// restore takes the issue back home (T22), the in-progress stage undone:
// a run that ends without moving the issue on, a fault, stale, discarded
// or abandoned, leaves it where the role rule finds it again.
func (c *collection) restore() error {
	if InProgress(c.r.Role, c.label) == "" && c.home() == c.label {
		return nil // the launch wrote none, and home is where it launched
	}
	return c.relabel(c.home())
}

// abandoned ends a run whose apply died before its container ended
// (section 7.7): a worker run's commits fetched and kept (7.6, steps 1
// and 2), the issue restored, the run collected, its clone removed.
func (c *collection) abandoned() error {
	if c.r.Role == "worker" {
		// A clone that cannot be fetched (it reaches outside itself, or
		// the member killed mid-run left no branch) keeps nothing, with
		// the warning, so the run is still collected.
		if err := c.fetchRef(); err != nil {
			fmt.Fprintf(c.s.Err, "⚠ #%d worker run %d: abandoned; commits not kept (%s)\n", c.r.N, c.r.K, oneLine.Replace(err.Error()))
		} else if err := c.keep(); err != nil {
			return err
		}
	}
	if c.r.N != 0 {
		if err := c.restore(); err != nil {
			return err
		}
	}
	fs := c.s.O.FS()
	if err := c.r.Append(fs, "outcome abandoned", "collected"); err != nil {
		return err
	}
	return errors.Join(removeTree(fs, filepath.Join(c.r.Dir, "work")), removeTree(fs, filepath.Join(c.r.Dir, "resources")), c.dropNotes())
}

// block is T17, swarm:blocked being a flag: the issue taken back home,
// then flagged beside it (section 5).
func (c *collection) block() error {
	if err := c.relabel(c.home()); err != nil {
		return err
	}
	return c.flag(true)
}

// flag sets swarm:blocked on the open issue at the run's stage, or with
// home at the run's home, where T22 has just taken it; the labels read
// again first: an issue the maintainer moved during the run is left as
// it is.
func (c *collection) flag(home bool) error {
	is, err := c.s.Forge.Issue(c.r.N)
	if err != nil {
		return err
	}
	c.is.Labels = is.Labels
	at := c.stage() != "" || home && slices.Contains(is.Labels, c.home())
	if !is.Open || !at || slices.Contains(is.Labels, forge.Blocked) {
		return nil
	}
	if err := c.s.Forge.Relabel(c.at, "", forge.Blocked); err != nil {
		return err
	}
	fmt.Fprintln(c.s.Out, c.head+"blocked")
	return c.step("flagged " + forge.Blocked)
}

// note posts the reconciler's note on the issue, head then resume and
// kept, then T17. An issue the maintainer moved during the run off both
// the run's stage and its home, which T17 leaves as it is, gets head and
// kept then notBlocked in place of resume, a note that blocked nothing
// (blocks).
func (c *collection) note(head, resume, kept, outcome string) error {
	text := head + " " + resume + kept
	if now, moved, err := c.movedAway(true); err != nil {
		return err
	} else if moved {
		text = head + kept + " " + notBlocked(now)
	}
	if err := c.comment("reconciler", text); err != nil {
		return err
	}
	if err := c.block(); err != nil {
		return err
	}
	return c.end(outcome)
}

// fail is a failure of the work (section 7.7): the reason recorded, a
// worker's commits kept, the note and T17.
func (c *collection) fail(reason string) error {
	reason = oneLine.Replace(reason) // as the result holds it, on stdout and in the note alike
	if _, ok := Has(c.lines, "failed"); !ok {
		if err := c.step("failed: " + reason); err != nil {
			return err
		}
	}
	fmt.Fprintf(c.s.Out, "%sfailed (%s)\n", c.head, reason)
	kept, err := c.keptNote()
	if err != nil {
		return err
	}
	return c.note(fmt.Sprintf("%s run %d failed: %s (at %s).", c.r.Role, c.r.K, reason, c.home()), "Remove "+forge.Blocked+" to resume.", kept, "failed")
}

// blocked posts the member's BLOCKED: text, on the pull request for the
// worker in reviewed and the reviewers, else on the issue; then the note
// and T17.
func (c *collection) blocked(text string, pull *forge.Pull) error {
	onPull := c.r.Role == "adversary" || c.r.Role == "sympathetic" || c.r.Role == "worker" && c.label == forge.ChangesRequested
	if onPull && pull != nil {
		ref := forge.Ref{Repo: pull.Repo, N: pull.Number}
		existing, err := c.s.Forge.PullComments(ref)
		if err != nil {
			return err
		}
		if err := c.say(ref, existing, c.r.Role, text); err != nil {
			return err
		}
	} else if err := c.comment(c.r.Role, text); err != nil {
		return err
	}
	kept, err := c.keptNote()
	if err != nil {
		return err
	}
	return c.note(fmt.Sprintf("%s run %d is blocked (at %s).", c.r.Role, c.r.K, c.home()), "Answer, then remove "+forge.Blocked+" to resume.", kept, "blocked")
}

// keptNote keeps a worker run's commits (section 7.6, step 2) and is the
// note's sentence naming their ref; "" when there are none.
func (c *collection) keptNote() (string, error) {
	if c.r.Role != "worker" || Ended(c.lines) == "launch-failed" {
		return "", nil
	}
	if err := c.keep(); err != nil {
		return "", err
	}
	if f := fetchFailures(c.lines); len(f) >= 2 {
		if l, _ := Has(c.lines, "failed"); strings.HasPrefix(l, "failed: fetch: ") {
			return "", nil // the failure is the fetch's, named already
		}
		return " Its commits could not be fetched from its clone: " + f[len(f)-1] + ".", nil
	}
	if !c.commits() {
		return "", nil
	}
	return " Its commits are kept in the swarm's mirror as " + runRef(c.r) + ".", nil
}

// runPull is the pull request the run works on: the worker's branch's,
// a reviewer's the issue's current one; nil when there is none.
func (c *collection) runPull(pulls []forge.Pull) *forge.Pull {
	if c.r.Role != "worker" {
		if len(pulls) == 0 {
			return nil
		}
		return &pulls[len(pulls)-1]
	}
	branch := runBranch(c.lines)
	for i := len(pulls) - 1; i >= 0; i-- {
		if pulls[i].Head == branch {
			return &pulls[i]
		}
	}
	return nil
}

func hit(data []byte, secrets []string) bool {
	return slices.ContainsFunc(secrets, func(sec string) bool { return sec != "" && bytes.Contains(data, []byte(sec)) })
}

// secretsOf is what the run's member was given (section 7.6): the env
// value, or the slot's long strings read at the launch and the return; a
// run this process did not launch reads them as they are now.
func (s *Swarm) secretsOf(r Run, lines []string) []string {
	if given, ok := s.given[r.Dir]; ok {
		return given
	}
	l, _ := Has(lines, "credential")
	f := strings.Fields(l)
	switch {
	case len(f) == 4 && f[2] == "env":
		return []string{s.Secret(f[3])}
	case len(f) == 4 && f[2] == "slot":
		sec, _ := runner.SlotSecrets(s.O.FS(), filepath.Join(s.slotDir(f[1]), f[3]))
		return sec
	}
	return nil
}

func repoName(repo string) string { _, r, _ := strings.Cut(repo, "/"); return r }

func repoNames(s *Swarm) []string {
	var out []string
	for _, r := range s.Cfg.Repos {
		out = append(out, r.Name)
	}
	return out
}

// postable is the output as posted, leaving room bytes for what follows
// it (a worker's check block): cut at the last line break before maxPost
// less room bytes, the note naming the run included (section 7.5).
func postable(out string, r Run, room int) string {
	limit := maxPost - room
	if len(out) <= limit {
		return out
	}
	note := "\n[truncated by Daisy; the whole output is in runs/" + filepath.Base(r.Dir) + "]"
	limit = max(limit-len(note), 0)
	cut := strings.LastIndex(out[:limit], "\n")
	if cut < 0 {
		cut = limit
	}
	return out[:cut] + note
}
