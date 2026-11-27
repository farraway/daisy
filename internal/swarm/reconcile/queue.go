package reconcile

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// The merge queue (section 12.1, rule 3; decision 169): each
// repository's approved heads, in the order their pairs completed, are
// checked one at a time on the commit that merges the head into base, in
// a run `<state>/runs/<n>-merge-<k>` that no member runs, and neither the
// merge step nor `swarm merge` merges a head without a passed merge
// check on base as it is now.

// approved is an issue the pass's step (5) read at review-2-approved,
// kept for the queue so that it reads neither again.
type approved struct {
	is    forge.Issue
	pulls []forge.Pull
}

// queued is a head in a repository's merge queue: issue is's current
// pull request cur, approved at head by the pair whose later review, of
// run k, completed at at.
type queued struct {
	is   forge.Issue
	cur  forge.Pull
	head string
	k    int
	at   time.Time
}

// mergeLine is a merge check's first result line, `merge <name> <head>
// into <base sha> at <merge sha>` (section 2).
type mergeLine struct{ name, head, base, sha string }

func (m mergeLine) String() string {
	return fmt.Sprintf("merge %s %s into %s at %s", m.name, m.head, m.base, m.sha)
}

// mergeOf is the merge check's first result line, ok false when lines
// hold none.
func mergeOf(lines []string) (m mergeLine, ok bool) {
	l, ok := Has(lines, "merge")
	f := strings.Fields(l)
	if !ok || len(f) != 7 || f[3] != "into" || f[5] != "at" {
		return mergeLine{}, false
	}
	return mergeLine{f[1], f[2], f[4], f[6]}, true
}

// mergeRef is the mirror's ref of the merge check of branch
// swarm/<n>-<s>, refs/swarm-merge/<n>-<s>, which its check clones.
func mergeRef(branch string) string {
	return "refs/swarm-merge/" + strings.TrimPrefix(branch, "swarm/")
}

// MergeChecked is how the newest merge check of issue n's head on the
// base commit base stands under state: "passed", "failed", "running"
// (not yet collected), or "" when none was started.
func MergeChecked(fs ops.FS, state string, n int, head, base string) (string, error) {
	runs, err := Runs(fs, state)
	if err != nil {
		return "", err
	}
	got, newest := "", 0
	for _, r := range runs {
		if r.Role != "merge" || r.N != n || r.K < newest {
			continue
		}
		lines, err := r.Result(fs)
		if err != nil {
			return "", err
		}
		if m, ok := mergeOf(lines); !ok || m.head != head || m.base != base {
			continue
		}
		newest = r.K
		_, passed := Has(lines, "passed")
		_, failed := Has(lines, "failed")
		_, done := Has(lines, "collected")
		switch {
		case passed:
			got = "passed"
		case failed:
			got = "failed"
		case !done:
			got = "running"
		default:
			got = "" // a fault: checked again
		}
	}
	return got, nil
}

// mergeBase is the commit refs/heads/<base> is at in the repository's
// mirror as its last fetch left it, "" when it has none.
func (s *Swarm) mergeBase(name string) string {
	g := s.git()
	return g.Head(g.Mirror(name), "refs/heads/"+s.Cfg.Base)
}

// mergeChecking is the repositories whose merge check is not yet
// collected: one runs at a time in each (section 12.1, rule 3).
func (s *Swarm) mergeChecking(runs []Run) (map[string]bool, error) {
	busy := map[string]bool{}
	for _, r := range runs {
		if r.Role != "merge" {
			continue
		}
		lines, err := r.Result(s.O.FS())
		if err != nil {
			return nil, err
		}
		if _, done := Has(lines, "collected"); !done {
			if m, ok := mergeOf(lines); ok {
				busy[m.name] = true
			}
		}
	}
	return busy, nil
}

// pairCompleted is when the pair approving head completed: the newest
// swarm review of head by the adversary or the sympathetic reviewer.
func pairCompleted(reviews []forge.Review, head, swarmLogin string) time.Time {
	var at time.Time
	for _, r := range reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
		if ok && (m.Role == "adversary" || m.Role == "sympathetic") && r.Commit == head && r.Submitted.After(at) {
			at = r.Submitted
		}
	}
	return at
}

// mergeQueue is rule 3's start of the merge checks, after the merge step
// (section 11, apply, step 6): for each repository with a check that the
// merge step merged nothing into and with no merge check running, the
// first queued head without a passed merge check on base as it is now,
// checked on the merge commit. It reads the forge only when a check can
// start (a member slot free, the swarm not paused), and only for the
// heads its own records do not show checked or running: with the merge
// gate off the merge step's reads are its own (s.awaiting); with it on,
// the approved issues step (5) read, then each one's reviews and
// dependencies. It returns the checks started.
func (s *Swarm) mergeQueue(merged map[string]bool) (int, error) {
	if s.launchesNothing() || len(s.Cfg.Repos) == 0 {
		return 0, nil
	}
	checked := slices.ContainsFunc(s.Cfg.Repos, func(r declare.Repo) bool { return len(r.Check) > 0 && !merged[r.Name] })
	if !checked || len(s.approved)+len(s.awaiting) == 0 {
		return 0, nil
	}
	runs, err := Runs(s.O.FS(), s.state())
	if err != nil {
		return 0, err
	}
	busy, err := s.mergeChecking(runs)
	if err != nil {
		return 0, err
	}
	open := func(name string) bool {
		r, ok := s.repo(name)
		return ok && len(r.Check) > 0 && !merged[name] && !busy[name]
	}
	heads := s.awaiting
	if s.Cfg.Gates.Merge {
		if heads, err = s.queuedHeads(open); err != nil {
			return 0, err
		}
	}
	by := map[string][]queued{}
	for _, q := range heads {
		if open(q.cur.Repo) {
			by[q.cur.Repo] = append(by[q.cur.Repo], q)
		}
	}
	started := 0
	for _, name := range slices.Sorted(maps.Keys(by)) {
		if s.launchesNothing() {
			break
		}
		q := slices.MinFunc(by[name], func(a, b queued) int {
			return cmp.Or(a.at.Compare(b.at), a.is.Number-b.is.Number)
		})
		ok, err := s.startMerge(q, runs)
		if err != nil {
			return started, err
		}
		if ok {
			started++
		}
	}
	return started, nil
}

// queuedHeads is, with the merge gate on, the approved heads step (5)
// read that may be checked: open, neither blocked nor paused, the current
// pull request open in a repository open says may start a check, with no
// open dependency (rule 1), the head both reviews approve not checked or
// running on base as it is now. An issue whose current head the records
// show passed or running costs no read.
func (s *Swarm) queuedHeads(open func(string) bool) ([]queued, error) {
	fs := s.O.FS()
	var out []queued
	for _, a := range s.approved {
		is, n := a.is, a.is.Number
		if Held(is.Labels) != "" || len(a.pulls) == 0 {
			continue
		}
		cur := a.pulls[len(a.pulls)-1]
		if !cur.Open || !open(cur.Repo) {
			continue
		}
		// base as it is now: the mirrors fetched once a pass, as the pass
		// fetches them (the merge step, gate off, has fetched them).
		if err := s.fetch(); err != nil {
			return nil, err
		}
		base := s.mergeBase(cur.Repo)
		if base == "" {
			continue
		}
		skip := func(head string) (bool, error) {
			st, err := MergeChecked(fs, s.state(), n, head, base)
			return st == "passed" || st == "running", err
		}
		if done, err := skip(cur.HeadSHA); err != nil || done {
			if err != nil {
				return nil, err
			}
			continue
		}
		j := &Job{SwarmLogin: s.SwarmLogin, N: n, Issue: is, Outputs: s.outputs()}
		if err := j.readPlan(); err != nil {
			return nil, fmt.Errorf("#%d: %w", n, err)
		}
		if deps, err := OpenDependencies(s.Forge, n, j.dependencies(s.Cfg.Trusted), s.Err); err != nil {
			return nil, err
		} else if len(deps) > 0 {
			continue
		}
		ref := forge.Ref{Repo: cur.Repo, N: cur.Number}
		reviews, err := s.Forge.Reviews(ref)
		if err != nil {
			return nil, err
		}
		comments, err := s.Forge.PullComments(ref)
		if err != nil {
			return nil, err
		}
		head, k := ApprovedHead(reviews, comments, s.SwarmLogin)
		if head == "" {
			continue
		}
		if head != cur.HeadSHA {
			if done, err := skip(head); err != nil || done {
				if err != nil {
					return nil, err
				}
				continue
			}
		}
		out = append(out, queued{is, cur, head, k, pairCompleted(reviews, head, s.SwarmLogin)})
	}
	return out, nil
}

// startMerge starts q's merge check (section 12.1, rule 3): head merged
// into base on the mirror (MergeCommit); a conflict is T25 at once,
// whichever the merge gate (backToWorker); a head the mirror cannot read
// is blocked with a note, never merged unchecked (the maintainer's ruling
// 5 of 2026-11-21); else the run `<n>-merge-<k>` records the merge and
// its first step starts on the merge commit as a worker's check does.
// ok is whether a container started.
func (s *Swarm) startMerge(q queued, runs []Run) (bool, error) {
	n, name := q.is.Number, q.cur.Repo
	base := s.mergeBase(name)
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(q.cur, n))
	sha, files, err := s.git().MergeCommit(name, base, q.head, mergeRef(branch), s.Cfg.Forge.CommitEmail)
	var u UnreadableHead
	switch {
	case errors.As(err, &u):
		fmt.Fprintf(s.Err, "⚠ #%d: pull %s#%d's head %s not merged on the mirror (%s); blocked\n", n, name, q.cur.Number, short(q.head), oneLine.Replace(err.Error()))
		return false, s.block(q.is, max(q.k, 1), fmt.Sprintf("The merge check of pull request %s cannot run: the mirror of %s cannot merge its approved head %s into %s at %s (at %s), so it is not merged unchecked (docs/swarm.md section 12.1, The merge queue). Push the head again, or relabel the issue; then remove %s to resume.",
			pullLink(s.owner(), q.cur), name, short(q.head), s.Cfg.Base, short(base), forge.ReviewApproved, forge.Blocked))
	case err != nil:
		return false, err
	case sha == "":
		return false, s.backToWorker(q.is, q.cur, q.head, q.k, files)
	}
	k := NextK(runs, n, "merge", nil)
	r := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-merge-%d", n, k)), N: n, K: k, Role: "merge"}
	fs := s.O.FS()
	if err := fs.MkdirAll(r.Dir, 0o755); err != nil {
		return false, err
	}
	lines := []string{mergeLine{name, q.head, base, sha}.String(), "branch " + branch + " in " + name, "fetched " + sha}
	if err := r.Append(fs, lines...); err != nil {
		return false, err
	}
	fmt.Fprintf(s.Out, "#%d merge check of pull %s#%d: %s merged into %s at %s\n", n, name, q.cur.Number, short(q.head), s.Cfg.Base, short(base))
	repo, _ := s.repo(name)
	if err := s.startStep(r, lines, nextStep(repo, lines)); err != nil {
		return false, err
	}
	if s.live[r.Dir] == nil {
		return false, s.collect(r) // the step could not start: a fault
	}
	return true, nil
}

// collectMerge collects a merge check whose step has ended (section
// 12.1, rule 3): its next step started, as a worker's check is; a pass
// recorded `passed`; a classified failure run again once (7.9) and, again
// classified, the issue blocked, nothing posted on the pull request; any
// other failure T32, the whole output written to the job's workspace as
// checks/<merge sha>.log and the reconciler's note on the pull request,
// then the issue relabelled review-3-changes-requested, whichever the
// merge gate. No member ran: no metrics row, no cause.
func (s *Swarm) collectMerge(r Run, lines []string) error {
	if step, _ := pending(lines); step != "" {
		return nil
	}
	fs := s.O.FS()
	lines, err := s.servicesDown(r, lines)
	if err != nil {
		return err
	}
	m, _ := mergeOf(lines)
	repo, _ := s.repo(m.name)
	c := &collection{s: s, r: r, lines: lines, head: runHead(r), at: forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: r.N}}
	if l, ok := Has(lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		return c.mergeEnd()
	}
	if _, failed := Has(lines, "failed"); !failed {
		if step := nextStep(repo, lines); step != "" {
			return c.startCheck(step)
		}
		result, out, _, _ := checkResult(fs, r, lines, s.Cfg.Timeout)
		if result == "exit 0" {
			fmt.Fprintf(s.Out, "%smerge check passed (%s on %s at %s)\n", c.head, short(m.head), s.Cfg.Base, short(m.base))
			if err := c.step("passed"); err != nil {
				return err
			}
			return c.mergeEnd()
		}
		if held, err := c.classify(repo, out); err != nil || held {
			return err
		}
		if err := c.writeCheck(m.sha, fmt.Sprintf("%s (%s merged into %s at %s)", checkLine(repo.Check, m.sha, result), m.head, s.Cfg.Base, m.base), out); err != nil {
			return err
		}
		if err := c.step("failed: merge check: " + result); err != nil {
			return err
		}
	}
	return c.mergeFailed(m, repo.Check)
}

// mergeFailed is T32 (section 12.1, rule 3): the merge check's note on
// the pull request whose head it checked, unless the thread's last word
// is that note by the swarm already (a retry), then the issue relabelled
// review-3-changes-requested; nothing when the issue has left
// review-2-approved or the pull request is no longer open on that branch.
func (c *collection) mergeFailed(m mergeLine, check []string) error {
	s, n := c.s, c.r.N
	result := strings.TrimPrefix(func() string { l, _ := Has(c.lines, "failed"); return l }(), "failed: merge check: ")
	is, err := s.Forge.Issue(n)
	if err != nil {
		return err
	}
	pulls, err := s.pulls(n)
	if err != nil {
		return err
	}
	branch := runBranch(c.lines)
	if stage, ok := StageOf(is); !is.Open || !ok || stage != forge.ReviewApproved || len(pulls) == 0 || !pulls[len(pulls)-1].Open || pulls[len(pulls)-1].Head != branch {
		fmt.Fprintf(s.Out, "%smerge check failed (%s); the issue has moved since: nothing posted\n", c.head, result)
		return c.mergeEnd()
	}
	p := pulls[len(pulls)-1]
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "%smerge check failed (%s): back to the worker\n", c.head, result)
	mk := marker("reconciler", c.r.K)
	l := s.mergeNote(n, fmt.Sprintf("The merge check on pull request %s failed: its approved head %s merged into %s at %s fails `%s` (%s). The output is /job/memory/checks/%s.log.",
		pullLink(s.owner(), p), short(m.head), s.Cfg.Base, short(m.base), declare.ShellLine(check), result, m.sha),
		fmt.Sprintf(" Merge %s into %s, fix what fails and commit; the reviewers then review the new head.", s.Cfg.Base, branch))
	if err := s.postNote(n, ref, comments, mk, l, s.lastNote(comments, mk, l), ""); err != nil {
		return err
	}
	if err := s.mergeRelabel(n, c.at, forge.ChangesRequested); err != nil {
		return err
	}
	return c.mergeEnd()
}

// mergeEnvironment is a merge check whose failure is classified again on
// its rerun (section 7.9): the issue blocked as a worker's check
// environment blocks it (T17), the note naming the reason, nothing posted
// on the pull request.
func (c *collection) mergeEnvironment(reason string) error {
	s := c.s
	if _, ok := Has(c.lines, "failed"); !ok {
		if err := c.step("failed: " + checkEnvironment + reason); err != nil {
			return err
		}
	}
	fmt.Fprintf(s.Out, "%sfailed (%s%s)\n", c.head, checkEnvironment, reason)
	is, err := s.Forge.Issue(c.r.N)
	if err != nil {
		return err
	}
	m, _ := mergeOf(c.lines)
	if stage, ok := StageOf(is); is.Open && ok && stage == forge.ReviewApproved {
		if err := s.block(is, c.r.K, fmt.Sprintf("The merge check of %s merged into %s at %s failed in the check environment: %s (at %s). Fix the environment, then remove %s to check it again.",
			short(m.head), s.Cfg.Base, short(m.base), reason, forge.ReviewApproved, forge.Blocked)); err != nil {
			return err
		}
	}
	return c.mergeEnd()
}

// mergeEnd records the merge check `collected` and removes its clone and
// its step's files.
func (c *collection) mergeEnd() error {
	fs := c.s.O.FS()
	if err := c.r.Append(fs, "collected"); err != nil {
		return err
	}
	c.s.collected++
	return errors.Join(removeTree(fs, filepath.Join(c.r.Dir, "check")), removeTree(fs, filepath.Join(c.r.Dir, "step")))
}

// stepName is the name a run's step container goes by, in its name and
// its label: a worker run's "deps" or "check", a merge check's "merge"
// (section 12.1, rule 3).
func stepName(r Run, step string) string {
	if r.Role == "merge" {
		return "merge"
	}
	return step
}

// stepRef is the mirror's ref a run's step clones: a worker run's
// refs/swarm-runs/<n>-worker-<k>, a merge check's
// refs/swarm-merge/<n>-<s>.
func stepRef(r Run, lines []string) string {
	if r.Role == "merge" {
		return mergeRef(runBranch(lines))
	}
	return runRef(r)
}
