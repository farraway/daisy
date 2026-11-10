package reconcile

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
)

// mergeStep is pass step (6), with the merge gate off (section 12): for
// each open approved issue neither blocked nor paused, its current pull
// request merged at the head both its newest swarm reviews approve (or
// found merged), then T14 when the accepted plan declares a slice after
// it and T15 otherwise, the closed issue labelled swarm:done. A refused
// merge of a pull request the forge reports not mergeable is read on the
// mirror (`git merge-tree`, Git.Conflicts): a conflict is T25, back to the
// worker with the direction to merge base naming the files, or, when this
// head was sent back once already, the issue blocked; a clean merge, the
// forge recomputing, waits for the next pass. A refused merge of one
// whose mergeability is unknown, or read after this step merged into its
// repository, waits too; of any other, T16, which blocks it.
func (s *Swarm) mergeStep() error {
	issues, err := s.Forge.Issues(forge.Open, forge.ReviewApproved)
	if err != nil {
		return err
	}
	merged := map[string]bool{}   // the repositories whose base this step moved
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
		S, sl := planSlices(is, s.SwarmLogin), sliceOf(cur, n)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: n}
		if !cur.Merged {
			reviews, err := s.Forge.Reviews(ref)
			if err != nil {
				return err
			}
			head, k := ApprovedHead(reviews, s.SwarmLogin)
			msg := "no head both reviews approve"
			if head != "" {
				msg = ""
				err := s.Forge.Merge(ref, head)
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
				note := marker("reconciler", max(k, 1)) + fmt.Sprintf("merge of pull request %s/%s#%d%s refused: %s (at %s). Resolve it, then remove %s.", s.owner(), cur.Repo, cur.Number, onHead, msg, forge.ReviewApproved, forge.Blocked)
				// Posted already only when it is the issue's last word and
				// the swarm's: a retry after a failed relabel, not another
				// login's copy or an earlier block's same note.
				last := len(is.Comments) - 1
				if last < 0 || is.Comments[last].Body != note || !strings.EqualFold(is.Comments[last].Author, s.SwarmLogin) {
					if err := s.Forge.Comment(at, note); err != nil {
						return err
					}
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
		note := marker("reconciler", max(k, 1)) + SentBackAgain(s.owner(), s.Cfg.Base, is.Number, p, head, files)
		last := len(is.Comments) - 1
		if last < 0 || is.Comments[last].Body != note || !strings.EqualFold(is.Comments[last].Author, s.SwarmLogin) {
			if err := s.Forge.Comment(at, note); err != nil {
				return err
			}
		}
		return s.mergeBlock(is.Number, at)
	}
	fmt.Fprintf(s.Out, "#%d not mergeable: back to the worker\n", is.Number)
	last := len(comments) - 1
	if last < 0 || !strings.EqualFold(comments[last].Author, s.SwarmLogin) || !strings.HasPrefix(swarmText(comments[last].Body), directionHead(s.owner(), p, head)) {
		note := marker("reconciler", max(k, 1)) + Direction(s.owner(), s.Cfg.Base, is.Number, p, head, files)
		if err := s.Forge.Comment(ref, note); err != nil {
			return err
		}
	}
	return s.mergeRelabel(is.Number, at, forge.ChangesRequested)
}

// Direction is the comment that sends an approved pull request p of issue
// n, which does not merge into base, back to the worker (T25): merge base
// into the branch and resolve. The merge step names the files that
// conflict on the mirror; with none (`swarm merge`, which reads the forge
// alone, whose report names no file), it names the branch and the base.
// The merge step posts it as the reconciler's note, `swarm merge` as the
// maintainer.
func Direction(owner, base string, n int, p forge.Pull, head string, files []string) string {
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(p, n))
	why := fmt.Sprintf("the forge reports it not mergeable into %s, which has moved since %s last took it in.", base, branch)
	if len(files) > 0 {
		why = fmt.Sprintf("it does not merge into %s, which has moved since %s last took it in: they conflict in %s.", base, branch, quoted(files))
	}
	return fmt.Sprintf("%s%s Merge %s into %s with a merge commit (\"git merge --no-edit %s\"), resolve each conflict, keeping what both sides meant, and commit; the reviewers then review the new head.",
		directionHead(owner, p, head), why, base, branch, base)
}

// directionHead is how every direction on p at head begins, which tells
// it from one on another head.
func directionHead(owner string, p forge.Pull, head string) string {
	return fmt.Sprintf("Pull request %s/%s#%d was approved at %s, but ", owner, p.Repo, p.Number, short(head))
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
// commit. The thread's last comment is not counted, a direction there
// being the one a retry after a refused relabel finds.
func SentBack(comments []forge.Comment, owner string, p forge.Pull, head, swarmLogin string, trusted []string) bool {
	prefix := directionHead(owner, p, head)
	for _, c := range comments[:max(len(comments)-1, 0)] {
		text, ok := c.Body, false
		if _, swarm := Swarms(Entry{c.Author, c.Body, c.Created}, swarmLogin); swarm {
			text, ok = swarmText(c.Body), true
		} else {
			ok = slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, c.Author) })
		}
		if ok && strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// SentBackAgain is the merge step's note on the issue when a head sent
// back once already (SentBack) does not merge a second time, which blocks
// it at review-2-approved.
func SentBackAgain(owner, base string, n int, p forge.Pull, head string, files []string) string {
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(p, n))
	conflict := ""
	if len(files) > 0 {
		conflict = " (they conflict in " + quoted(files) + ")"
	}
	return fmt.Sprintf("Pull request %s/%s#%d was sent back to merge %s at %s once already, was approved again at that head, and still does not merge into %s%s (at %s). Merge %s into %s, or relabel the issue, then remove %s.",
		owner, p.Repo, p.Number, base, short(head), base, conflict, forge.ReviewApproved, base, branch, forge.Blocked)
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

// planSlices is the number of slices is's accepted plan declares; a plan
// without slice lines, or none, is one slice (section 5).
func planSlices(is forge.Issue, swarmLogin string) int {
	if plan, _, ok := (&Job{SwarmLogin: swarmLogin, Issue: is}).Plan(); ok {
		return max(1, len(Slices(swarmText(plan.Body))))
	}
	return 1
}

// Slice is the slice of is's pull request p and the number of slices
// its accepted plan declares, as the merge step counts them.
func Slice(is forge.Issue, p forge.Pull, swarmLogin string) (s, S int) {
	return sliceOf(p, is.Number), planSlices(is, swarmLogin)
}

// AfterApproval is what an approved issue waits for with the merge gate
// on (section 12): "merge" until its current pull request is merged, then
// "next slice" when the accepted plan declares a slice after it (T13) or
// "close" when it was the last (T19); "pull closed" when it was closed
// unmerged, which neither `swarm merge` nor the merge step merges.
func AfterApproval(is forge.Issue, pulls []forge.Pull, swarmLogin string) string {
	switch {
	case len(pulls) == 0:
		return "merge"
	case !pulls[len(pulls)-1].Merged && !pulls[len(pulls)-1].Open:
		return "pull closed"
	case !pulls[len(pulls)-1].Merged:
		return "merge"
	}
	if sliceOf(pulls[len(pulls)-1], is.Number) < planSlices(is, swarmLogin) {
		return "next slice"
	}
	return "close"
}

// ApprovedHead is the commit both the newest adversary review and the
// newest sympathetic review approve, with the run of the later one (the
// run that completed the approving pair); "" when they do not.
func ApprovedHead(reviews []forge.Review, swarmLogin string) (string, int) {
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
	if a.k == 0 || b.k == 0 || a.r.Commit != b.r.Commit || verdictOf(swarmText(a.r.Body)) != "approve" || verdictOf(swarmText(b.r.Body)) != "approve" {
		return "", max(a.k, b.k)
	}
	if a.r.Submitted.After(b.r.Submitted) {
		return a.r.Commit, a.k
	}
	return b.r.Commit, b.k
}
