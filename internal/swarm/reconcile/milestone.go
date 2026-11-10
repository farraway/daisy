package reconcile

import (
	"fmt"
	"slices"

	"daisy/internal/swarm/forge"
)

// milestones is pass step (8), the forge milestones that mirror the briefs
// (section 11, apply): for each milestone of each active brief, a milestone
// of forge.repo titled `<slug>: <milestone>` ensured; then each swarm
// issue, open or closed, that has no milestone and whose Brief: line names
// one of an active brief put in it. A milestone the line names that its
// brief lacks, or a brief that is not there, is a warning for an open
// issue; every failure is a warning, and the pass goes on. The open
// issues are open, the flag step's list (read again when nil); the closed
// ones are read only when this step created a milestone, or at this
// process's first reading of them (section 14, item 90): a closed issue
// is otherwise one an earlier step has already seen.
func (s *Swarm) milestones(open []forge.Issue) {
	warn := func(format string, a ...any) { fmt.Fprintf(s.Err, "⚠ "+format+"\n", a...) }
	briefs, err := Briefs(s.O, s.Place, s.Cfg, s.state(), nil)
	if err != nil {
		warn("milestones: reading the briefs: %v", err)
		return
	}
	known := map[string]bool{}               // every brief's path
	titles := map[string]map[string]string{} // an active brief's path → its milestone → the forge milestone's title
	slugs := map[string]string{}             // a slug → the active brief that has it
	var order []string                       // the titles, by brief and heading
	for _, b := range briefs {
		known[b.Path] = true
		if b.Status != "active" {
			continue
		}
		if other, taken := slugs[b.Slug()]; taken {
			warn("milestones: %s and %s share the name %s; %s is mirrored to no milestone", other, b.Path, b.Slug(), b.Path)
			continue
		}
		slugs[b.Slug()] = b.Path
		titles[b.Path] = map[string]string{}
		for _, m := range b.Milestones() {
			t := b.MilestoneTitle(m)
			titles[b.Path][m] = t
			order = append(order, t)
		}
	}
	if len(order) == 0 {
		return
	}
	have, err := s.Forge.Milestones()
	if err != nil {
		warn("milestones: %v", err)
		return
	}
	ids := map[string]int64{}
	for _, m := range have {
		if _, ok := ids[m.Title]; !ok {
			ids[m.Title] = m.ID
		}
	}
	created := false
	for _, t := range order {
		if _, ok := ids[t]; ok {
			continue
		}
		id, err := s.Forge.EnsureMilestone(t)
		if err != nil {
			warn("milestone %s: %v", t, err)
			continue
		}
		ids[t], created = id, true
		fmt.Fprintf(s.Out, "milestone created: %s\n", t)
	}
	if open == nil {
		var err error
		if open, err = s.issues(forge.Open); err != nil {
			warn("milestones: %v", err)
			return
		}
	}
	issues := slices.Clone(open)
	if created || !s.closedRead {
		closed, err := s.issues(forge.Closed)
		if err != nil {
			warn("milestones: %v", err)
			return
		}
		issues, s.closedRead = append(issues, closed...), true
	}
	for _, is := range issues {
		if is.Milestone != 0 {
			continue
		}
		path, m := s.briefOf(is)
		if path == "" {
			continue
		}
		ms, active := titles[path]
		t, named := ms[m]
		switch {
		case !known[path] && is.Open:
			warn("#%d: its Brief: line names %s, which is no brief under resources; no milestone set", is.Number, path)
			continue
		case active && !named && is.Open:
			warn("#%d: its Brief: line names %q, which is no milestone of %s; no milestone set", is.Number, m, path)
			continue
		}
		id, ok := ids[t]
		if !active || !named || !ok {
			continue // a brief not active, a closed issue's line, or a milestone not created (warned above)
		}
		if err := s.Forge.SetMilestone(is.Number, id); err != nil {
			warn("#%d: milestone %s: %v", is.Number, t, err)
			continue
		}
		fmt.Fprintf(s.Out, "#%d milestone: %s\n", is.Number, t)
	}
}
