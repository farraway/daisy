package reconcile

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// Gated is the maintainer's flag's predicate (section 5): an issue with
// labels waits for the maintainer when it carries one stage and that
// stage is a gate that is on (proposal-posted with the triage gate,
// plan-0-posted with the plan gate, review-2-approved with the merge
// gate), or it carries swarm:blocked. `apply` sets the flag by it and
// `status`'s Waiting line lists by it, so the forge and the terminal
// agree.
func Gated(labels []string, g declare.Gates) bool {
	stage, ok := StageOf(forge.Issue{Labels: labels})
	if !ok {
		return false
	}
	gate := map[string]bool{forge.ProposalPosted: g.Triage, forge.PlanPosted: g.Plan, forge.ReviewApproved: g.Merge}[stage]
	return gate || slices.Contains(labels, forge.Blocked)
}

// flags is pass step (8), the maintainer's flag (section 5): every open
// issue given forge.Gated when Gated holds for its labels, and every
// other open issue carrying the flag cleared of it,
// one with no stage or two included, so an open issue carries the flag
// exactly when status lists it as waiting. Every failure is a warning,
// and the pass goes on.
func (s *Swarm) flags() {
	warn := func(format string, a ...any) { fmt.Fprintf(s.Err, "⚠ "+format+"\n", a...) }
	open, err := s.issues(forge.Open)
	if err == nil {
		var flagged []forge.Issue
		if flagged, err = s.Forge.Issues(forge.Open, forge.Gated); err == nil {
			for _, is := range flagged {
				if !slices.ContainsFunc(open, func(o forge.Issue) bool { return o.Number == is.Number }) {
					open = append(open, is)
				}
			}
		}
	}
	if err != nil {
		warn("gated: %v", err)
		return
	}
	slices.SortFunc(open, func(a, b forge.Issue) int { return a.Number - b.Number })
	for _, is := range open {
		want := Gated(is.Labels, s.Cfg.Gates)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
		switch has := slices.Contains(is.Labels, forge.Gated); {
		case want && !has:
			if err := s.Forge.Relabel(at, "", forge.Gated); err != nil {
				warn("#%d: gated: %v", is.Number, err)
				continue
			}
			fmt.Fprintf(s.Out, "#%d gated\n", is.Number)
		case !want && has:
			if err := Unflag(s.Forge, at, is.Labels, forge.Gated, s.Out); err != nil {
				warn("#%d: gated: %v", is.Number, err)
			}
		}
	}
}

// Unflag clears flag from the issue at, whose labels are labels, when it
// carries it: what a maintainer's verb does with the gated flag beside its
// relabel and with the flag it lifts, and the flag step with a stage that
// no longer waits.
func Unflag(f forge.Adapter, at forge.Ref, labels []string, flag string, out io.Writer) error {
	if !slices.Contains(labels, flag) {
		return nil
	}
	if err := f.Relabel(at, flag, ""); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d %s cleared\n", at.N, strings.TrimPrefix(flag, "swarm:"))
	return nil
}

// Flag sets flag on the issue at, whose labels are labels, unless it
// carries it.
func Flag(f forge.Adapter, at forge.Ref, labels []string, flag string, out io.Writer) error {
	if slices.Contains(labels, flag) {
		return nil
	}
	if err := f.Relabel(at, "", flag); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d %s\n", at.N, strings.TrimPrefix(flag, "swarm:"))
	return nil
}

// Retire is a close's labels (section 5), made after the close: the
// closed issue's stage label replaced by terminal (forge.Done or
// forge.Rejected) and every flag cleared, labels being the issue's as
// read before the close, so a closed issue keeps no stage and no flag.
func Retire(f forge.Adapter, at forge.Ref, stage string, labels []string, terminal string, out io.Writer) error {
	if err := f.Relabel(at, stage, terminal); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d %s → %s\n", at.N, stage, terminal)
	for _, flag := range forge.Flags {
		if err := Unflag(f, at, labels, flag, out); err != nil {
			return err
		}
	}
	return nil
}
