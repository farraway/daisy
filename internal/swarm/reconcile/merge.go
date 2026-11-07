package reconcile

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
)

// mergeStep is pass step (6), with the merge gate off (section 12): for
// each open approved issue, its current pull request merged at the head
// both its newest swarm reviews approve (or found merged), then T14 when
// the accepted plan declares a slice after it and T15 otherwise; a
// refused merge is T16.
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
		pulls, err := s.Forge.Pulls(n)
		if err != nil {
			return err
		}
		j := &Job{Cfg: s.Cfg, SwarmLogin: s.SwarmLogin, N: n, Issue: is, Pulls: pulls}
		cur := forge.Pull{}
		if len(pulls) > 0 {
			cur = pulls[len(pulls)-1]
		}
		if !cur.Merged && !cur.Open {
			continue
		}
		ref := forge.Ref{Repo: cur.Repo, N: cur.Number}
		S := 1
		if plan, _, ok := j.Plan(); ok {
			S = max(1, len(Slices(swarmText(plan.Body))))
		}
		sl := sliceOf(cur, n)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: n}
		if !cur.Merged {
			reviews, err := s.Forge.Reviews(ref)
			if err != nil {
				return err
			}
			head, k := approvedHead(reviews, s.SwarmLogin)
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
				if !slices.ContainsFunc(is.Comments, func(c forge.Comment) bool { return c.Body == note }) {
					if err := s.Forge.Comment(at, note); err != nil {
						return err
					}
				}
				if err := s.Forge.Relabel(at, "swarm:approved", "swarm:blocked"); err != nil {
					return err
				}
				fmt.Fprintf(s.Out, "#%d swarm:approved → swarm:blocked\n", n)
				continue
			}
			fmt.Fprintf(s.Out, "#%d merged pull %s#%d (slice %d of %d)\n", n, cur.Repo, cur.Number, sl, S)
		}
		if sl < S {
			if err := s.Forge.Relabel(at, "swarm:approved", "swarm:planned"); err != nil {
				return err
			}
			fmt.Fprintf(s.Out, "#%d swarm:approved → swarm:planned\n", n)
			continue
		}
		if err := s.Forge.CloseIssue(n); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "#%d closed (last slice merged)\n", n)
	}
	return nil
}

// approvedHead is the commit both the newest adversary review and the
// newest sympathetic review approve, with the run of the later one (the
// run that completed the approving pair); "" when they do not.
func approvedHead(reviews []forge.Review, swarmLogin string) (string, int) {
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
