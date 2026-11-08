package reconcile

import (
	"fmt"
	"io"
	"slices"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// Gated is the maintainer's flag's predicate (section 5): stage waits
// for the maintainer when it is a gate that is on (proposed with the
// triage gate, planning with the plan gate, approved with the merge gate)
// or blocked. `apply` sets the flag by it and `status`'s Waiting line
// lists by it, so the forge and the terminal agree.
func Gated(stage string, g declare.Gates) bool {
	switch stage {
	case "swarm:proposed":
		return g.Triage
	case "swarm:planning":
		return g.Plan
	case "swarm:approved":
		return g.Merge
	case "swarm:blocked":
		return true
	}
	return false
}

// flags is pass step (8), the maintainer's flag (section 5): every open
// issue carrying one stage label given forge.Gated when Gated holds for
// its stage, and every other open issue carrying the flag cleared of it,
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
		stage, one := StageOf(is)
		want := one && Gated(stage, s.Cfg.Gates)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
		switch has := slices.Contains(is.Labels, forge.Gated); {
		case want && !has:
			if err := s.Forge.Relabel(at, "", forge.Gated); err != nil {
				warn("#%d: gated: %v", is.Number, err)
				continue
			}
			fmt.Fprintf(s.Out, "#%d gated\n", is.Number)
		case !want && has:
			if err := Unflag(s.Forge, at, is.Labels, s.Out); err != nil {
				warn("#%d: gated: %v", is.Number, err)
			}
		}
	}
}

// Unflag clears the maintainer's flag from the issue at, whose labels
// are labels, when it carries it: what a maintainer's verb does with its
// relabel, and the flag step with a stage that no longer waits.
func Unflag(f forge.Adapter, at forge.Ref, labels []string, out io.Writer) error {
	if !slices.Contains(labels, forge.Gated) {
		return nil
	}
	if err := f.Relabel(at, forge.Gated, ""); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d gated cleared\n", at.N)
	return nil
}

// Retire is a close's labels (section 5), made after the close: the
// closed issue's stage label replaced by terminal (forge.Done or
// forge.Rejected) and the flag cleared, labels being the issue's as read
// before the close, so a closed issue keeps no stage and no flag.
func Retire(f forge.Adapter, at forge.Ref, stage string, labels []string, terminal string, out io.Writer) error {
	if err := f.Relabel(at, stage, terminal); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d %s → %s\n", at.N, stage, terminal)
	return Unflag(f, at, labels, out)
}
