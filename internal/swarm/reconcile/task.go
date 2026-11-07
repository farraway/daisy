package reconcile

import (
	"fmt"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// closing ends every Task text (section 7.4).
const closing = "Write your output to /work/.swarm/out.md; that file is the only thing read back, and nothing you write elsewhere outside /work survives. If you cannot do this without an answer from the maintainer, write `BLOCKED: <your question>` as the first line of /work/.swarm/out.md instead."

const planHeadings = `"## Research" (what exists: the files and seams the job touches, what connects to what, and what the change affects, each line naming a path under /work), "## Goal", "## Non-goals", "## Constraints", "## Steps" (the slices: a numbered list, "1. " and so on, one item per pull request in the order they are built, each beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s, then the files it changes and the new or changed types and signatures, its claim, and the test that proves the claim, its lines after the first indented so that under "## Steps" only a slice begins a line with a number and a dot; one item when the job is one pull request; a slice that moves a pin to another slice's change comes after that slice), "## Risks", "## Test Plan" (the plan's acceptance: numbered claims, each with the check that proves it and where its test lives), "## Decisions" (each design decision the resources do not record, with its alternative), "## Deferred" and "## Open Questions".`

// task is the run's Task text: section 7.4's row for its role and label,
// its additions, and the closing paragraph.
func (j *Job) task() string {
	var t []string
	add := func(format string, args ...any) { t = append(t, fmt.Sprintf(format, args...)) }
	cur, open := j.current()
	names := strings.Join(j.names(), ", ")
	switch {
	case j.Batch != nil && len(j.Batch.active()) > 0:
		add(`Propose up to %d new jobs for %s, most important first, toward the first milestone of the highest-priority brief under "# Briefs" that the closed issues do not yet meet, never proposing what an open issue already covers; a brief is a goal with milestones, each with a persona, an exit criterion and what it forces, and no steps: you cut it into jobs. Start each with a line "## <title>", then a line "Brief: <path> — <milestone>" naming the brief's path as "# Briefs" gives it and the milestone's heading, then the job's body. When every milestone of a brief is met by closed issues, write a line "Met: <path>" before the first "## " line; write nothing else before it. If there is nothing worth proposing, write "BLOCKED: <why>" as the only line after any "Met:" lines.`, j.Batch.Propose, j.Cfg.Forge.Repo)
	case j.Batch != nil:
		add(`Propose up to %d new jobs for %s, most important first, from the resources and the code. Start each with a line "## <title>" and follow it with the job's body. Write nothing before the first "## " line. If there is nothing worth proposing, write "BLOCKED: <why>" as the only line.`, j.Batch.Propose, j.Cfg.Forge.Repo)
	case j.Role == "owner":
		if j.commented() {
			add(`Answer the latest comment in the thread of issue #%d.`, j.N)
		} else {
			add(`Issue #%d has no comment yet: say whether its body is a job the swarm can do and is worth doing next, and what is unclear in it.`, j.N)
		}
		if !j.Cfg.Gates.Triage {
			add(`Then restate the whole job as it now stands: the architect plans from your answer.`)
		}
	case j.Role == "architect":
		if _, _, planned := j.Plan(); j.Label == "swarm:planning" && planned {
			add(`Revise your plan for issue #%d in answer to the latest comment. Write the whole revised plan under the same "## " headings in the same order, not the changes to it, each slice beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s, a slice that moves a pin to another slice's change after that slice, a diagram where a slice touches more than one component.`, j.N, names)
		} else {
			add(`Write the plan for issue #%d as a Daisy plan, under these "## " headings in this order: `+planHeadings+` When a slice touches more than one component, add a Mermaid diagram of it (a flowchart or a sequence diagram) in a fenced "mermaid" block under that step.`, j.N, names)
			if j.Cfg.Gates.Plan {
				add(`The maintainer reads "## Research" first, then the plan, before any code is written.`)
			} else {
				add(`The plan is accepted as you write it: there is no revision round.`)
			}
		}
		if open {
			add(`Pull request %s#%d, slice %d, is open: it reached the review limit, or the maintainer returned the issue to you, and its reviews under "# Pull request" are the input. Revise the plan so that the slice can pass review: slice %d keeps its number, its repository and its branch, and the worker continues on them; what it does, how it is tested and the slices after it may change.`, cur.Repo, cur.Number, sliceOf(cur, j.N), sliceOf(cur, j.N))
		}
	case j.Role == "worker":
		s, repo, _ := j.WorkerSlice()
		_, _, planned := j.Plan()
		switch {
		case j.Label == "swarm:reviewed":
			add(`The newest entries of pull request %s#%d's thread need an answer: the blocking findings of its latest reviews and the maintainer's comments. Address each on the branch checked out in /work (swarm/%d-%d), and write what you changed, point by point.`, cur.Repo, cur.Number, j.N, s)
		case planned:
			e, _, _ := j.Plan()
			add(`Implement slice %d of the accepted plan for issue #%d (it declares %d) in %s/%s, on the branch checked out in /work (swarm/%d-%d). Do not change the plan's Test Plan; in the pull request's description, or in your comment when you continue an open pull request, report each Test Plan claim's status: done, not in this slice, or failing.`, s, j.N, max(1, len(Slices(swarmText(e.Body)))), owner(j.Cfg), repo, j.N, s)
		default:
			add(`There is no plan: the issue body and the maintainer's comments under Direction are the job of issue #%d. Keep the change small; if it needs a plan, write "BLOCKED: needs a plan". Work on the branch checked out in /work (swarm/%d-%d) and write the pull request's description.`, j.N, j.N, s)
		}
		if !j.Cfg.Gates.Merge {
			add(`Before anything else, run "git merge --no-edit %s"; if it conflicts, write "BLOCKED: %s conflicts" instead.`, j.Cfg.Base, j.Cfg.Base)
		}
		add(`Commit with git; do not push. Never amend, rebase or reset a commit that is already on the branch; add commits.`)
		if check := j.repo(repo).Check; len(check) > 0 {
			add(`Run "%s" before you finish and quote its last result.`, shellQuote(check))
		} else {
			add(`No check command is declared; say how you tested.`)
		}
		add(`End what you write with a line "Not established:" and list what you did not run or could not verify (live services, other platforms, behaviour that needs a network), or "none".`)
		if planned && s > 1 {
			add(`A pin to an earlier slice's change names its merged commit, listed under Earlier pull requests, never a branch or an unmerged head; if this slice needs an earlier slice that is not merged, write "BLOCKED: slice <i> is not merged".`)
		}
	default: // the reviewers
		s := sliceOf(cur, j.N)
		add(`Review pull request %s#%d, slice %d of issue #%d. The change is "git diff %s...HEAD" in /work, a throwaway clone you may build and test in; nothing you change there is kept. Check each claim of the plan's Test Plan against the diff and the check's result, and the worker's "Not established:" lists against what the change claims: a claim neither tested nor listed is a finding. The first line of your output must be exactly "VERDICT: approve" or "VERDICT: request-changes"; then the review, each finding a line beginning "[blocking]" or "[non-blocking]" with its file and line. Request changes only when a finding is blocking; approve otherwise.`, cur.Repo, cur.Number, s, j.N, j.Cfg.Base)
		if j.Role == "adversary" {
			add(`For each test that slice %d's item under the plan's Steps names, and for each Test Plan claim the worker's newest report (the description, or its comment when it continued the pull request) reports done, check that its test exists on this head and that it fails against %s's code: in /work, a throwaway clone, put everything the change touches except its tests back as %s has it (check out %s's version of each path, deleting those %s lacks), run the test as the head has it (a build failure counts as failing), then restore the head ("git checkout HEAD -- ." and remove what it adds back). A test that is missing, or that passes against %s's code, is a "[blocking]" finding; a claim reported not in this slice is not checked here.`, s, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base)
		}
		if r, prev := j.round(cur); r > 1 {
			add(`This is round %d. First say, for each blocking finding of your review of %s, whether it is resolved; raise new blocking findings only on code changed since ("git diff %s..HEAD").`, r, prev, prev)
		}
	}
	if (j.Role == "architect" || j.Role == "worker") && j.Batch == nil {
		if p := j.protected(); p != "" {
			add(`These paths are protected: %s. No change to them is pushed; the maintainer makes it. Leave such a change out and say so.`, p)
		}
	}
	return strings.Join(t, " ") + "\n\n" + closing
}

// commented: the issue has a counted comment (section 6).
func (j *Job) commented() bool {
	for _, c := range j.Issue.Comments {
		if j.counted(Entry{c.Author, c.Body, c.Created}) {
			return true
		}
	}
	return false
}

// protected is the Task text's list: the run's repository's patterns for
// the worker, each declared repository's as `<name>: <patterns>` for the
// architect.
func (j *Job) protected() string {
	if j.Role == "worker" {
		return strings.Join(j.repo(j.RunRepo()).Protected, ", ")
	}
	var parts []string
	for _, r := range j.Cfg.Repos {
		if len(r.Protected) > 0 {
			parts = append(parts, r.Name+": "+strings.Join(r.Protected, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

// round is the reviewer's round on the current head (section 6: the
// distinct commits both reviewers reviewed since the accepted plan, plus
// this one) and the commit of the role's newest review on the pull request.
func (j *Job) round(p forge.Pull) (int, string) {
	if j.Pull == nil {
		return 1, ""
	}
	plan, _, hasPlan := j.Plan()
	by := map[string]map[string]bool{}
	prev, prevAt := "", time.Time{}
	for _, r := range j.Pull.Reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, j.SwarmLogin)
		if !ok || (hasPlan && !r.Submitted.After(plan.Created)) {
			continue
		}
		if by[r.Commit] == nil {
			by[r.Commit] = map[string]bool{}
		}
		by[r.Commit][m.Role] = true
		if m.Role == j.Role && r.Commit != p.HeadSHA && !r.Submitted.Before(prevAt) {
			prev, prevAt = r.Commit, r.Submitted
		}
	}
	rounds := 0
	for c, roles := range by {
		if roles["adversary"] && roles["sympathetic"] && c != p.HeadSHA {
			rounds++
		}
	}
	return rounds + 1, prev
}
