package reconcile

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// Gated is the maintainer's flag's predicate (section 5): an issue with
// labels, of the job class class (ClassOf), waits for the maintainer when
// it carries one stage and that stage is a gate that is on and holds its
// class (proposal-posted with the triage gate and plan-0-posted with the
// plan gate for an architecture job, review-2-approved with the merge
// gate for both), or it carries swarm:blocked. `apply` sets the flag by
// it and `status`'s Waiting line lists by it, so the forge and the
// terminal agree.
func Gated(labels []string, g declare.Gates, class string) bool {
	stage, ok := StageOf(forge.Issue{Labels: labels})
	if !ok {
		return false
	}
	arch := class != Routine
	gate := map[string]bool{forge.ProposalPosted: g.Triage && arch, forge.PlanPosted: g.Plan && arch, forge.ReviewApproved: g.Merge}[stage]
	return gate || slices.Contains(labels, forge.Blocked)
}

// flags is pass step (8), the maintainer's flag (section 5): first, an
// open issue at the triage or plan gate (classGate) is read with its
// comments for its class, and a routine job's gate passed (autoPass);
// then every open issue is given forge.Gated when Gated holds for its
// labels and class, with the gate note posted on it once the flag is on,
// and every other open issue carrying the flag cleared of it, one with no
// stage or two included, so an open issue carries the flag exactly when
// status lists it as waiting. A note is posted when the flag is added,
// and on an issue not blocked whose flag stayed on while it was gated
// again (regated): one per gating event, and none for an event whose
// relabel failed until a later pass adds the flag. Every failure is a
// warning, and the pass goes on.
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
		labels, class := is.Labels, Architecture // the class decides only at a classGate
		var full *forge.Issue                    // is read with its comments, when it was
		if stage, gate := s.classGate(labels); gate != "" {
			read, err := s.Forge.Issue(is.Number)
			if err != nil {
				warn("#%d: class: %v", is.Number, err)
				continue
			}
			full = &read
			if class = ClassOf(read, s.SwarmLogin, s.Cfg.Trusted); class == Routine {
				if labels, err = s.autoPass(read, stage, gate); err != nil {
					warn("#%d: auto: %v", is.Number, err)
				}
			}
		}
		want := Gated(labels, s.Cfg.Gates, class)
		at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
		switch has := slices.Contains(labels, forge.Gated); {
		case want && !has:
			if err := s.Forge.Relabel(at, "", forge.Gated); err != nil {
				warn("#%d: gated: %v", is.Number, err)
				continue
			}
			fmt.Fprintf(s.Out, "#%d gated\n", is.Number)
			if err := s.gateNote(is.Number, open); err != nil {
				warn("#%d: gate note: %v", is.Number, err)
			}
		case want && has && !slices.Contains(labels, forge.Blocked):
			if full == nil {
				read, err := s.Forge.Issue(is.Number)
				if err != nil {
					warn("#%d: gate note: %v", is.Number, err)
					continue
				}
				full = &read
			}
			if !s.regated(*full, labels) {
				continue
			}
			if err := s.gateNote(is.Number, open); err != nil {
				warn("#%d: gate note: %v", is.Number, err)
			}
		case !want && has:
			if err := Unflag(s.Forge, at, labels, forge.Gated, s.Out); err != nil {
				warn("#%d: gated: %v", is.Number, err)
			}
		}
	}
}

// gateHead is a gate note's line after its marker, naming the stage it
// gated (gateText).
var gateHead = regexp.MustCompile(`^\*\*reconciler:\*\* gated at (\S+): `)

// regated: is, flagged and gated still at the stage its labels name, was
// gated again since its newest gate note without the flag being cleared
// (section 5, one note per gating event): that note names another stage
// (a stage moved within one pass, or by hand with the flag left on), or,
// at plan-0-posted, the architect posted a plan after it (a revision,
// after a comment or the round limit, T11). An issue with no gate note
// (its note refused, or the flag set by hand) is not.
func (s *Swarm) regated(is forge.Issue, labels []string) bool {
	stage, _ := StageOf(forge.Issue{Labels: labels})
	var note *forge.Comment
	for i, c := range is.Comments {
		if m, ok := Swarms(Entry{c.Author, c.Body, c.Created}, s.SwarmLogin); ok && m.Kind == "gate" && (note == nil || !c.Created.Before(note.Created)) {
			note = &is.Comments[i]
		}
	}
	if note == nil {
		return false
	}
	_, rest, _ := strings.Cut(note.Body, "\n")
	if h := gateHead.FindStringSubmatch(rest); h == nil || "swarm:"+h[1] != stage {
		return true
	}
	plan, _, ok := (&Job{SwarmLogin: s.SwarmLogin, Issue: is}).Plan()
	return stage == forge.PlanPosted && ok && plan.Created.After(note.Created)
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
