package reconcile

import (
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
)

// The re-plan (section 6): the maintainer's comment `Re-plan.` or
// `Re-plan: <direction>`, which `swarm replan` posts, asks the architect
// to revise an accepted plan's remaining slices between slices, as
// rulings changed since the plan do (Step 9's revision): the architect
// runs on plan-1-accepted in the worker's place until its next word on
// the plan, a revised plan or STANDS:.

// ReplanText is the comment `swarm replan` posts: `Re-plan: <direction>`,
// or `Re-plan.` with none.
func ReplanText(direction string) string {
	if direction == "" {
		return "Re-plan."
	}
	return "Re-plan: " + direction
}

// isReplan: text is a re-plan request (ReplanText's forms), whitespace at
// its start aside.
func isReplan(text string) bool {
	t := strings.TrimLeft(text, " \t\r\n")
	return strings.TrimRight(t, " \t\r\n") == "Re-plan." || strings.HasPrefix(t, "Re-plan:")
}

// asked is the maintainer's newest re-plan request on j's issue after
// the basis (the architect's newest word on the accepted plan) in the
// thread's order, which a request posted in the basis's second keeps;
// ok is false when there is none, or no plan.
func (j *Job) asked() (e Entry, ok bool) {
	basis, _, planned := j.basis()
	if !planned {
		return Entry{}, false
	}
	thread := IssueThread(j.Issue)
	after := thread[slices.Index(thread, basis)+1:]
	for _, c := range after {
		if maintainerWord(c, j.SwarmLogin, j.Cfg.Trusted) && isReplan(c.Body) {
			e, ok = c, true
		}
	}
	return e, ok
}

// Planned: is's thread holds a plan (Job.Plan), so a re-plan revises it
// rather than writing the first.
func Planned(is forge.Issue, swarmLogin string) bool {
	_, _, ok := (&Job{SwarmLogin: swarmLogin, Issue: is}).Plan()
	return ok
}
