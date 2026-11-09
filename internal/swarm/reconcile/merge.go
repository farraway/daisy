package reconcile

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/swarm/forge"
)

// mergeStep is pass step (6), with the merge gate off (section 12): for
// each open approved issue neither blocked nor paused, its current pull
// request merged at the head both its newest swarm reviews approve (or
// found merged), then T14 when the accepted plan declares a slice after
// it and T15 otherwise, the closed issue labelled swarm:done. A refused
// merge of a pull request the forge reports not mergeable is T25, back to
// the worker with the direction to merge base; of one whose mergeability
// is unknown, or read after this step merged into its repository (the
// forge recomputing it), nothing until the next pass; of any other, T16,
// which blocks it.
func (s *Swarm) mergeStep() error {
	issues, err := s.Forge.Issues(forge.Open, forge.ReviewApproved)
	if err != nil {
		return err
	}
	merged := map[string]bool{} // the repositories whose base this step moved
	for _, listed := range issues {
		n := listed.Number
		is, err := s.Forge.Issue(n)
		if err != nil {
			return err
		}
		if stage, ok := StageOf(is); !is.Open || !ok || stage != forge.ReviewApproved || Held(is.Labels) != "" {
			continue // two labels (warned in step 5), moved or closed since the list was read, or held
		}
		pulls, err := s.Forge.Pulls(n)
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
				if err := s.backToWorker(is, cur, head, k); err != nil {
					return err
				}
				continue
			}
			if msg != "" {
				fmt.Fprintf(s.Out, "#%d merge of pull %s#%d refused (%s)\n", n, cur.Repo, cur.Number, msg)
				note := marker("reconciler", max(k, 1)) + fmt.Sprintf("merge of pull request %s/%s#%d at %s refused: %s (at %s). Resolve it, then remove %s.", s.owner(), cur.Repo, cur.Number, short(head), msg, forge.ReviewApproved, forge.Blocked)
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

// backToWorker is T25 by the merge step: the direction to merge base
// posted on the pull request as the reconciler's note (unless it is the
// thread's last word already: a retry after a failed relabel), then the
// issue relabelled review-3-changes-requested, whose worker answers it.
func (s *Swarm) backToWorker(is forge.Issue, p forge.Pull, head string, k int) error {
	fmt.Fprintf(s.Out, "#%d not mergeable: back to the worker\n", is.Number)
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	note := marker("reconciler", max(k, 1)) + Direction(s.owner(), s.Cfg.Base, is.Number, p, head)
	last := len(comments) - 1
	if last < 0 || comments[last].Body != note || !strings.EqualFold(comments[last].Author, s.SwarmLogin) {
		if err := s.Forge.Comment(ref, note); err != nil {
			return err
		}
	}
	return s.mergeRelabel(is.Number, forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}, forge.ChangesRequested)
}

// Direction is the comment that sends an approved pull request p of issue
// n, which the forge reports not mergeable into base, back to the worker
// (T25): merge base into the branch and resolve. The forge names no
// conflicting file, so it names the branch and the base; the merge step
// posts it as the reconciler's note, `swarm merge` as the maintainer.
func Direction(owner, base string, n int, p forge.Pull, head string) string {
	branch := fmt.Sprintf("swarm/%d-%d", n, sliceOf(p, n))
	return fmt.Sprintf("Pull request %s/%s#%d was approved at %s, but the forge reports it not mergeable into %s, which has moved since %s last took it in. Merge %s into %s with a merge commit (\"git merge --no-edit %s\"), resolve each conflict, keeping what both sides meant, and commit; the reviewers then review the new head.",
		owner, p.Repo, p.Number, short(head), base, branch, base, branch, base)
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
// "close" when it was the last (T19).
func AfterApproval(is forge.Issue, pulls []forge.Pull, swarmLogin string) string {
	if len(pulls) == 0 || !pulls[len(pulls)-1].Merged {
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
