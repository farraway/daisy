package reconcile

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/swarm/forge"
)

// mergeStep is pass step (6), with the merge gate off (section 12): for
// each open approved issue, its current pull request merged at the head
// both its newest swarm reviews approve (or found merged), then T14 when
// the accepted plan declares a slice after it and T15 otherwise, the
// closed issue labelled swarm:done; a refused merge is T16.
func (s *Swarm) mergeStep() error {
	issues, err := s.Forge.Issues(forge.Open, "swarm:approved")
	if err != nil {
		return err
	}
	for _, listed := range issues {
		n := listed.Number
		is, err := s.Forge.Issue(n)
		if err != nil {
			return err
		}
		if stage, ok := StageOf(is); !is.Open || !ok || stage != "swarm:approved" {
			continue // two labels (warned in step 5), or moved or closed since the list was read
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
			err = errors.New("no head both reviews approve")
			if head != "" {
				err = s.Forge.Merge(ref, head)
			}
			if err != nil && head != "" && !errors.Is(err, forge.ErrNotMergeable) {
				return err
			}
			if err != nil {
				msg := strings.TrimPrefix(err.Error(), forge.ErrNotMergeable.Error()+": ")
				fmt.Fprintf(s.Out, "#%d merge of pull %s#%d refused (%s)\n", n, cur.Repo, cur.Number, msg)
				note := marker("reconciler", max(k, 1)) + fmt.Sprintf("merge of pull request %s/%s#%d at %s refused: %s (was swarm:approved). Resolve it and relabel the issue.", s.owner(), cur.Repo, cur.Number, short(head), msg)
				// Posted already only when it is the issue's last word and
				// the swarm's: a retry after a failed relabel, not another
				// login's copy or an earlier block's same note.
				last := len(is.Comments) - 1
				if last < 0 || is.Comments[last].Body != note || !strings.EqualFold(is.Comments[last].Author, s.SwarmLogin) {
					if err := s.Forge.Comment(at, note); err != nil {
						return err
					}
				}
				if err := s.mergeRelabel(n, at, "swarm:blocked"); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintf(s.Out, "#%d merged pull %s#%d (slice %d of %d)\n", n, cur.Repo, cur.Number, sl, S)
		}
		if sl < S {
			if err := s.mergeRelabel(n, at, "swarm:planned"); err != nil {
				return err
			}
			continue
		}
		if err := s.Forge.CloseIssue(n); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "#%d closed (last slice merged)\n", n)
		if err := Retire(s.Forge, at, "swarm:approved", is.Labels, forge.Done, s.Out); err != nil {
			return err
		}
	}
	return nil
}

// mergeRelabel is T14 or T16: approved to `to`, the labels read again right
// before (the merge or its refusal took a while, and the maintainer may
// have moved the issue meanwhile: kept, with a warning, section 5).
func (s *Swarm) mergeRelabel(n int, at forge.Ref, to string) error {
	is, err := s.Forge.Issue(n)
	if err != nil {
		return err
	}
	if stage, ok := StageOf(is); !ok || stage != "swarm:approved" {
		fmt.Fprintf(s.Err, "⚠ #%d: relabelled to %s during the merge step; not relabelled\n", n, stage)
		return nil
	}
	if err := s.Forge.Relabel(at, "swarm:approved", to); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "#%d swarm:approved → %s\n", n, to)
	return nil
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
