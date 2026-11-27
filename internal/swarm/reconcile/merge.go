package reconcile

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
)

// warnUnchecked is apply's warning of each repository the merge step
// would merge with no check run (declare's Unchecked, section 3): on
// stderr at the process's first pass and again only when the set of
// warnings changes, as Drift's lines are. It holds nothing up.
func (s *Swarm) warnUnchecked() {
	var lines []string
	for _, i := range s.Cfg.Unchecked() {
		lines = append(lines, s.Cfg.UncheckedWarning(i))
	}
	if slices.Equal(lines, s.unchecked) {
		return
	}
	s.unchecked = lines
	for _, l := range lines {
		fmt.Fprintln(s.Err, "⚠ "+l)
	}
}

// mergeStep is pass step (6), with the merge gate off (section 12): for
// each open approved issue neither blocked nor paused, its current pull
// request merged at the head both its newest swarm reviews approve (or
// found merged), then T14 when the accepted plan declares a slice after
// it and T15 otherwise, the closed issue labelled swarm:done. The head's
// checks are read first (HeadChecks): one failed is T26, back to the
// worker with the contexts named; one pending, or none reported while
// base requires them, waits for the next pass. A refused
// merge of a pull request the forge reports not mergeable is read on the
// mirror (`git merge-tree`, Git.Conflicts): a conflict is T25, back to the
// worker with the direction to merge base naming the files, or, when this
// head was sent back once already, the issue blocked; a clean merge, the
// forge recomputing, waits for the next pass. A refused merge of one
// whose mergeability is unknown, or read after this step merged into its
// repository, waits too; of any other, T16, which blocks it.
func (s *Swarm) mergeStep(merged map[string]bool) error {
	issues, err := s.Forge.Issues(forge.Open, forge.ReviewApproved)
	if err != nil {
		return err
	}
	mirrored := map[string]bool{} // the mirrors this step fetched
	for _, listed := range issues {
		n := listed.Number
		is, err := s.Forge.Issue(n)
		if err != nil {
			return err
		}
		if stage, ok := StageOf(is); !is.Open || !ok || stage != forge.ReviewApproved || Held(is.Labels) != "" {
			continue // two labels (warned in step 5), moved or closed since the list was read, or held
		}
		pulls, err := s.pulls(n)
		if err != nil {
			return err
		}
		cur := forge.Pull{}
		if len(pulls) > 0 {
			cur = pulls[len(pulls)-1]
		}
		if !cur.Merged && !cur.Open {
			continue
		}
		ref := forge.Ref{Repo: cur.Repo, N: cur.Number}
		S, cut, err := planSlices(is, s.SwarmLogin, s.outputs())
		if err != nil {
			if _, err = s.blockUnusable(is, err); err != nil {
				return err
			}
			continue
		}
		if cut != "" {
			// Its plan taken cut from its post, the slices would be
			// counted short (section 7.8, Rollout).
			if err := s.blockCut(is, cut); err != nil {
				return err
			}
			continue
		}
		sl := sliceOf(cur, n)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: n}
		repo, _ := s.repo(cur.Repo)
		queue := len(repo.Check) > 0 // the merge queue's (section 12.1, rule 3)
		if !cur.Merged && queue && merged[cur.Repo] {
			continue // a merge ends its repository's merges for the pass: the others' checks are stale
		}
		if !cur.Merged {
			// Nothing of an issue with an open dependency is merged
			// (section 12.1, rule 1).
			j := &Job{SwarmLogin: s.SwarmLogin, N: n, Issue: is, Outputs: s.outputs()}
			if err := j.readPlan(); err != nil {
				return fmt.Errorf("#%d: %w", n, err)
			}
			if deps, err := OpenDependencies(s.Forge, n, j.dependencies(s.Cfg.Trusted), s.Err); err != nil {
				return err
			} else if len(deps) > 0 {
				continue
			}
			reviews, err := s.Forge.Reviews(ref)
			if err != nil {
				return err
			}
			comments, err := s.Forge.PullComments(ref)
			if err != nil {
				return err
			}
			head, k := ApprovedHead(reviews, comments, s.SwarmLogin)
			msg := "no head both reviews approve"
			if head != "" && queue {
				// No head is merged without a passed merge check on base
				// as the pass fetched it (section 12.1, rule 3).
				if err := s.fetch(); err != nil {
					return err
				}
				base := s.mergeBase(cur.Repo)
				st, err := MergeChecked(s.O.FS(), s.state(), n, head, base)
				if err != nil {
					return err
				}
				if st != "passed" {
					if st != "running" {
						s.awaiting = append(s.awaiting, queued{is, cur, head, k, pairCompleted(reviews, head, s.SwarmLogin)})
						st = "queued"
					}
					fmt.Fprintf(s.Out, "#%d merge of pull %s#%d waits: its merge check on %s at %s has not passed (%s); the next pass tries again\n", n, cur.Repo, cur.Number, s.Cfg.Base, short(base), st)
					continue
				}
			}
			if head != "" {
				checks, err := HeadChecks(s.Forge, cur.Repo, s.Cfg.Base, head)
				if err != nil {
					return err
				}
				switch {
				case len(checks.Failed) > 0:
					if err := s.checksFailed(is, cur, head, k, checks.Failed); err != nil {
						return err
					}
					continue
				case checks.Wait():
					fmt.Fprintf(s.Out, "#%d merge of pull %s#%d waits: %s; the next pass tries again\n", n, cur.Repo, cur.Number, checks.Why(head, s.Cfg.Base))
					continue
				}
				msg = ""
				err = s.Forge.Merge(ref, head)
				switch {
				case errors.Is(err, forge.ErrNotMergeable):
					msg = strings.TrimPrefix(err.Error(), forge.ErrNotMergeable.Error()+": ")
				case err != nil:
					return err
				}
			}
			// The forge's refusal read by the pull request's mergeability,
			// which a merge into its repository earlier in this step left
			// being recomputed.
			refused := head != "" && msg != ""
			switch {
			case refused && (cur.Mergeable == forge.MergeableUnknown || merged[cur.Repo]):
				fmt.Fprintf(s.Out, "#%d merge of pull %s#%d refused (%s); its mergeability is not settled, so the next pass tries again\n", n, cur.Repo, cur.Number, msg)
				continue
			case refused && cur.Mergeable == forge.MergeableNo:
				// The mirror brought up to date first: a pass that launched
				// nothing has not fetched it.
				if r, ok := s.repo(cur.Repo); ok && !mirrored[cur.Repo] {
					if err := s.git().Fetch(r); err != nil {
						return err
					}
					mirrored[cur.Repo] = true
				}
				files, clean, err := s.git().Conflicts(cur.Repo, s.Cfg.Base, head)
				switch {
				case err != nil:
					// The mirror cannot tell (the head not in it): the
					// forge's report stands, and the direction names no file.
					fmt.Fprintf(s.Err, "⚠ #%d: pull %s#%d's head %s not merged on the mirror (%s); the forge's report stands\n", n, cur.Repo, cur.Number, short(head), oneLine.Replace(err.Error()))
				case clean:
					fmt.Fprintf(s.Out, "#%d merge of pull %s#%d refused (%s); its head merges into %s on the mirror, so the forge is taken to be recomputing and the next pass tries again\n", n, cur.Repo, cur.Number, msg, s.Cfg.Base)
					continue
				}
				if err := s.backToWorker(is, cur, head, k, files); err != nil {
					return err
				}
				continue
			}
			if msg != "" {
				fmt.Fprintf(s.Out, "#%d merge of pull %s#%d refused (%s)\n", n, cur.Repo, cur.Number, msg)
				onHead := "" // no head both approve: none to name
				if head != "" {
					onHead = " at " + short(head)
				}
				mk := marker("reconciler", max(k, 1))
				l := s.mergeNote(n, fmt.Sprintf("merge of pull request %s%s refused: %s (at %s).", pullLink(s.owner(), cur), onHead, msg, forge.ReviewApproved),
					fmt.Sprintf(" Resolve it, then remove %s.", forge.Blocked)+s.mention())
				// Posted already only when it is the issue's last word and
				// the swarm's: a retry after a failed relabel, not another
				// login's copy or an earlier block's same note.
				if err := s.postNote(n, at, is.Comments, mk, l, s.lastNote(is.Comments, mk, l), ""); err != nil {
					return err
				}
				if err := s.mergeBlock(n, at); err != nil {
					return err
				}
				continue
			}
			merged[cur.Repo] = true
			fmt.Fprintf(s.Out, "#%d merged pull %s#%d (slice %d of %d)\n", n, cur.Repo, cur.Number, sl, S)
		}
		if sl < S {
			if err := s.mergeRelabel(n, at, forge.PlanAccepted); err != nil {
				return err
			}
			continue
		}
		if err := s.Forge.CloseIssue(n); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "#%d closed (last slice merged)\n", n)
		if err := Retire(s.Forge, at, forge.ReviewApproved, is.Labels, forge.Done, s.Out); err != nil {
			return err
		}
		s.listPulls(is) // step (8) lists the open issues alone
	}
	return nil
}

// backToWorker is T25 by the merge step: the direction to merge base,
// naming the files that conflict, posted on the pull request as the
// reconciler's note (unless the thread's last word is a direction on
// this head by the swarm already: a retry after a failed relabel), then
// the issue relabelled review-3-changes-requested, whose worker answers
// it. A head sent back once already (SentBack) is not sent back again:
// the issue is blocked at review-2-approved, the maintainer's, as the
// round limit's second time blocks it (T11).
func (s *Swarm) backToWorker(is forge.Issue, p forge.Pull, head string, k int, files []string) error {
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
	if SentBack(comments, s.owner(), p, head, s.SwarmLogin, s.Cfg.Trusted) {
		fmt.Fprintf(s.Out, "#%d not mergeable a second time at %s: blocked\n", is.Number, short(head))
		mk := marker("reconciler", max(k, 1))
		text, tail := sentBackAgainNote(s.owner(), s.Cfg.Base, is.Number, p, head, files)
		l := s.mergeNote(is.Number, text, tail+s.mention())
		if err := s.postNote(is.Number, at, is.Comments, mk, l, s.lastNote(is.Comments, mk, l), ""); err != nil {
			return err
		}
		return s.mergeBlock(is.Number, at)
	}
	fmt.Fprintf(s.Out, "#%d not mergeable: back to the worker\n", is.Number)
	var posted *forge.Comment // a direction on this head, whatever files it named
	if last := lastWord(comments, s.SwarmLogin); last >= 0 && strings.EqualFold(comments[last].Author, s.SwarmLogin) && isDirection(swarmText(comments[last].Body), s.owner(), p, head) {
		posted = &comments[last]
	}
	text, tail := directionNote(s.owner(), s.Cfg.Base, is.Number, p, head, files)
	if err := s.postNote(is.Number, ref, comments, marker("reconciler", max(k, 1)), s.mergeNote(is.Number, text, tail), posted, ""); err != nil {
		return err
	}
	return s.mergeRelabel(is.Number, at, forge.ChangesRequested)
}

// Checks is what the checks on an approved head say of its merge
// (section 12): Failed, those that failed or errored; Pending, those
// still running; Unreported, none has reported while the base requires
// checks; Missing, the contexts the base requires that have not reported
// (section 12, The checks, folding in L346). With none failed, none pending, not unreported
// and none missing, the merge is the forge's to make or refuse.
type Checks struct {
	Failed, Pending []forge.Check
	Unreported      bool
	Missing         []string
}

// Wait: the merge waits for the checks, none having failed.
func (c Checks) Wait() bool {
	return len(c.Failed) == 0 && (len(c.Pending) > 0 || c.Unreported || len(c.Missing) > 0)
}

// Why is what the merge of head into base waits for (Wait).
func (c Checks) Why(head, base string) string {
	if len(c.Pending) > 0 {
		names := make([]string, len(c.Pending))
		for i, k := range c.Pending {
			names[i] = k.Context
		}
		return fmt.Sprintf("its checks on %s are pending: %s", short(head), quoted(names))
	}
	if c.Unreported {
		return fmt.Sprintf("no check has reported on %s, and %s requires them", short(head), base)
	}
	if len(c.Missing) == 1 {
		return fmt.Sprintf("%s has not reported on %s, and %s requires it", quoted(c.Missing), short(head), base)
	}
	return fmt.Sprintf("%s have not reported on %s, and %s requires them", quoted(c.Missing), short(head), base)
}

// HeadChecks reads the checks on head in the declared repository repo
// and, when none has failed or is pending, whether base requires checks
// and which contexts (section 12, The checks, folding in L346): none reported
// while base requires them, or a required context with no check
// reported that it matches (a glob, as Gitea matches it), waits; a forge
// that reports no requirement, or a base without one, merges as before.
func HeadChecks(f forge.Adapter, repo, base, head string) (Checks, error) {
	checks, err := f.Status(repo, head)
	if err != nil {
		return Checks{}, err
	}
	var c Checks
	for _, k := range checks {
		switch k.State {
		case "failure", "error":
			c.Failed = append(c.Failed, k)
		case "pending":
			c.Pending = append(c.Pending, k)
		}
	}
	if len(c.Failed) > 0 || len(c.Pending) > 0 {
		return c, nil
	}
	required, contexts, err := f.RequiresChecks(repo, base)
	if err != nil || !required {
		return c, err
	}
	if len(checks) == 0 && len(contexts) == 0 {
		c.Unreported = true
	}
	for _, want := range contexts {
		if !slices.ContainsFunc(checks, func(k forge.Check) bool { return forge.ContextMatch(want, k.Context) }) {
			c.Missing = append(c.Missing, want)
		}
	}
	return c, nil
}

// ChecksFailed is the comment that sends an approved pull request p of
// issue n back to the worker when checks failed on its approved head
// (T26), naming each: the merge step posts it as the reconciler's note,
// `swarm merge` as the maintainer.
func ChecksFailed(owner string, n int, p forge.Pull, head string, failed []forge.Check) string {
	text, tail := checksFailedNote(owner, n, p, head, failed)
	return text + tail
}

// checksFailedNote is ChecksFailed's text and its last sentence, which says
// what to do and is never cut (section 7.5).
func checksFailedNote(owner string, n int, p forge.Pull, head string, failed []forge.Check) (text, tail string) {
	named := make([]string, len(failed))
	for i, k := range failed {
		named[i] = fmt.Sprintf("`%s` (%s)", k.Context, k.State)
	}
	return fmt.Sprintf("The checks on pull request %s failed at its approved head %s: %s.", pullLink(owner, p), short(head), strings.Join(named, ", ")),
		fmt.Sprintf(" Fix what they report on swarm/%d-%d and commit; the reviewers then review the new head.", n, sliceOf(p, n))
}

// mergeNote is a merge step's note on issue n (section 12), text then
// tail, by section 7.5's rule: attached, when it does not fit, as
// <n>-merge-<time>.md.
func (s *Swarm) mergeNote(n int, text, tail string) long {
	return long{text: text, tail: tail, file: s.noteFile(n, "merge"), what: "the merge step's note"}
}

// checksFailed is T26 by the merge step: ChecksFailed posted on the pull
// request as the reconciler's note (unless the thread's last word is
// that note by the swarm already: a retry after a failed relabel), then
// the issue relabelled review-3-changes-requested, whose worker answers
// it. Nothing blocks it: a worker that answers without a commit and a
// pair that approves the head again send it back again, each round
// counted towards the round limit (T11).
func (s *Swarm) checksFailed(is forge.Issue, p forge.Pull, head string, k int, failed []forge.Check) error {
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "#%d checks failed at %s: back to the worker\n", is.Number, short(head))
	mk := marker("reconciler", max(k, 1))
	text, tail := checksFailedNote(s.owner(), is.Number, p, head, failed)
	l := s.mergeNote(is.Number, text, tail)
	if err := s.postNote(is.Number, ref, comments, mk, l, s.lastNote(comments, mk, l), ""); err != nil {
		return err
	}
	return s.mergeRelabel(is.Number, forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}, forge.ChangesRequested)
}

// Direction is the comment that sends an approved pull request p of issue
// n, which does not merge into base, back to the worker (T25): merge base
// into the branch and resolve. The merge step names the files that
// conflict on the mirror; with none (`swarm merge`, which reads the forge
// alone, whose report names no file), it names the branch and the base.
// The merge step posts it as the reconciler's note, `swarm merge` as the
// maintainer.
func Direction(owner, base string, n int, p forge.Pull, head string, files []string) string {
	text, tail := directionNote(owner, base, n, p, head, files)
	return text + tail
}

// directionNote is Direction's text and its last sentence, which says what
// to do and is never cut (section 7.5).
func directionNote(owner, base string, n int, p forge.Pull, head string, files []string) (text, tail string) {
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(p, n))
	why := fmt.Sprintf("the forge reports it not mergeable into %s, which has moved since %s last took it in.", base, branch)
	if len(files) > 0 {
		why = fmt.Sprintf("it does not merge into %s, which has moved since %s last took it in: they conflict in %s.", base, branch, quoted(files))
	}
	return directionHead(owner, p, head) + why,
		fmt.Sprintf(" Merge %s into %s with a merge commit (\"git merge --no-edit %s\"), resolve each conflict, keeping what both sides meant, and commit; the reviewers then review the new head.", base, branch, base)
}

// directionHead is how every direction on p at head begins, which tells
// it from one on another head.
func directionHead(owner string, p forge.Pull, head string) string {
	return fmt.Sprintf("Pull request %s was approved at %s, but ", pullLink(owner, p), short(head))
}

// isDirection: text begins as a direction on p at head does, or as one
// did before the pull request was linked in it (Step 7 of the plan for
// the inbox requests F27-F35), so a head sent back before is told too.
func isDirection(text, owner string, p forge.Pull, head string) bool {
	unlinked := p
	unlinked.URL = ""
	return strings.HasPrefix(text, directionHead(owner, p, head)) || strings.HasPrefix(text, directionHead(owner, unlinked, head))
}

// quoted is paths in backticks, comma-separated.
func quoted(paths []string) string {
	q := make([]string, len(paths))
	for i, p := range paths {
		q[i] = "`" + p + "`"
	}
	return strings.Join(q, ", ")
}

// SentBack: pull request p's thread holds a direction on head (T25), the
// swarm's or a trusted login's, before its last comment: the head was
// sent back to the worker once already, and approved again without a
// commit. The thread's last word (lastWord, a cut direction's
// continuations passed by) is not counted, a direction there being the
// one a retry after a refused relabel finds.
func SentBack(comments []forge.Comment, owner string, p forge.Pull, head, swarmLogin string, trusted []string) bool {
	for _, c := range comments[:max(lastWord(comments, swarmLogin), 0)] {
		text, ok := c.Body, false
		if _, swarm := Swarms(Entry{c.Author, c.Body, c.Created}, swarmLogin); swarm {
			text, ok = swarmText(c.Body), true
		} else {
			ok = slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, c.Author) })
		}
		if ok && isDirection(text, owner, p, head) {
			return true
		}
	}
	return false
}

// SentBackAgain is the merge step's note on the issue when a head sent
// back once already (SentBack) does not merge a second time, which blocks
// it at review-2-approved.
func SentBackAgain(owner, base string, n int, p forge.Pull, head string, files []string) string {
	text, tail := sentBackAgainNote(owner, base, n, p, head, files)
	return text + tail
}

// sentBackAgainNote is SentBackAgain's text and its last sentence, which says
// what to do and is never cut (section 7.5).
func sentBackAgainNote(owner, base string, n int, p forge.Pull, head string, files []string) (text, tail string) {
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(p, n))
	conflict := ""
	if len(files) > 0 {
		conflict = " (they conflict in " + quoted(files) + ")"
	}
	return fmt.Sprintf("Pull request %s was sent back to merge %s at %s once already, was approved again at that head, and still does not merge into %s%s (at %s).",
			pullLink(owner, p), base, short(head), base, conflict, forge.ReviewApproved),
		fmt.Sprintf(" Merge %s into %s, or relabel the issue, then remove %s.", base, branch, forge.Blocked)
}

// mergeRelabel is T14: approved to `to`, the labels read again right
// before (the merge took a while, and the maintainer may have moved the
// issue meanwhile: kept, with a warning, section 5).
func (s *Swarm) mergeRelabel(n int, at forge.Ref, to string) error {
	if _, ok, err := s.stillApproved(n); err != nil || !ok {
		return err
	}
	if err := s.Forge.Relabel(at, forge.ReviewApproved, to); err != nil {
		return err
	}
	s.moved(n, to)
	fmt.Fprintf(s.Out, "#%d %s → %s\n", n, forge.ReviewApproved, to)
	return nil
}

// mergeBlock is T16: the approved issue flagged swarm:blocked, its stage
// kept, the labels read again right before as mergeRelabel reads them.
func (s *Swarm) mergeBlock(n int, at forge.Ref) error {
	labels, ok, err := s.stillApproved(n)
	if err != nil || !ok {
		return err
	}
	return Flag(s.Forge, at, labels, forge.Blocked, s.Out)
}

// stillApproved is issue n's labels and whether it is still at
// review-2-approved, warning when it was moved during the merge step.
func (s *Swarm) stillApproved(n int) ([]string, bool, error) {
	is, err := s.Forge.Issue(n)
	if err != nil {
		return nil, false, err
	}
	if stage, ok := StageOf(is); !ok || stage != forge.ReviewApproved {
		fmt.Fprintf(s.Err, "⚠ #%d: relabelled to %s during the merge step; not relabelled\n", n, stage)
		return nil, false, nil
	}
	return is.Labels, true, nil
}

// planSlices is the number of slices is's accepted plan declares, read
// from its workspace through o (JobPlan), with cut the run whose plan is
// taken cut from its post, its out.md gone, for a job whose plan is
// built in memory; a plan without slice lines, or none, is one slice
// (section 5).
func planSlices(is forge.Issue, swarmLogin string, o Outputs) (S int, cut string, err error) {
	plan, ok, cut, err := o.JobPlan(is, swarmLogin)
	if err != nil {
		return 0, "", fmt.Errorf("#%d: %w", is.Number, err)
	}
	if !ok {
		return 1, "", nil
	}
	return max(1, len(Slices(plan))), cut, nil
}

// Slice is the slice of is's pull request p and the number of slices
// its accepted plan declares, read through o as the merge step counts
// them (planSlices), with cut as planSlices gives it.
func Slice(is forge.Issue, p forge.Pull, swarmLogin string, o Outputs) (s, S int, cut string, err error) {
	S, cut, err = planSlices(is, swarmLogin, o)
	return sliceOf(p, is.Number), S, cut, err
}

// AfterApproval is what an approved issue waits for with the merge gate
// on (section 12): "merge" until its current pull request is merged, then
// "next slice" when the accepted plan declares a slice after it (T13) or
// "close" when it was the last (T19); "pull closed" when it was closed
// unmerged, which neither `swarm merge` nor the merge step merges. The
// plan is read from the job's workspace through o (planSlices).
func AfterApproval(is forge.Issue, pulls []forge.Pull, swarmLogin string, o Outputs) (string, error) {
	switch {
	case len(pulls) == 0:
		return "merge", nil
	case !pulls[len(pulls)-1].Merged && !pulls[len(pulls)-1].Open:
		return "pull closed", nil
	case !pulls[len(pulls)-1].Merged:
		return "merge", nil
	}
	S, _, err := planSlices(is, swarmLogin, o)
	if err != nil {
		return "", err
	}
	if sliceOf(pulls[len(pulls)-1], is.Number) < S {
		return "next slice", nil
	}
	return "close", nil
}

// ApprovedHead is the commit both the newest adversary review and the
// newest sympathetic review approve, with the run of the later one (the
// run that completed the approving pair); "" when they do not. An
// adversary's request the architect overruled on the pull request,
// comments being its comments, stands as its approval would (standing).
func ApprovedHead(reviews []forge.Review, comments []forge.Comment, swarmLogin string) (string, int) {
	over := overruledAt(pullComments(comments), swarmLogin)
	type newest struct {
		r forge.Review
		k int
	}
	by := map[string]newest{}
	for _, r := range reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
		if ok && (m.Role == "adversary" || m.Role == "sympathetic") && !r.Submitted.Before(by[m.Role].r.Submitted) {
			by[m.Role] = newest{r, m.Run}
		}
	}
	a, b := by["adversary"], by["sympathetic"]
	if a.k == 0 || b.k == 0 || a.r.Commit != b.r.Commit || standing("adversary", verdictOf(swarmText(a.r.Body)), a.r.Submitted, over) != "approve" || verdictOf(swarmText(b.r.Body)) != "approve" {
		return "", max(a.k, b.k)
	}
	if a.r.Submitted.After(b.r.Submitted) {
		return a.r.Commit, a.k
	}
	return b.r.Commit, b.k
}
