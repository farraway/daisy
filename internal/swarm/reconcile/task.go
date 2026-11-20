package reconcile

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// closing ends every Task text (section 7.4), workerClosing the
// worker's, whose screenshots are read back too, and planClosing the
// architect's that ask for a plan, whose builder plan is.
var (
	closing       = closingOf("that file is the only thing read back")
	workerClosing = closingOf("that file and the screenshots in /work/.swarm/artifacts/ are the only things read back")
	planClosing   = closingOf("that file and the builder plan in /work/.swarm/builder.md are the only things read back")
)

func closingOf(readBack string) string {
	return "Write your output to /work/.swarm/out.md; " + readBack + ", and nothing you write elsewhere outside /work survives. If you cannot do this without an answer from the maintainer, write `BLOCKED: <your question>` as the first line of /work/.swarm/out.md instead. " + causeSentence
}

// builderSentence follows the plan's text in each architect Task text
// that asks for a plan or a revised plan (section 7.4; 7.8, The builder
// plan).
const builderSentence = `Then write the builder plan to /work/.swarm/builder.md, a document of its own, not posted: the plan consolidated for whoever builds it, one section "## Slice <s>" per slice under "## Steps", in their order, each holding all a builder needs for that slice (its files, types and signatures, claim and test, and what of the research, constraints and decisions bears on it), so that it reads without the plan. Write it whole each time you write the plan.`

const planHeadings = `"## Research" (what exists: the files and seams the job touches, what connects to what, and what the change affects, each line naming a path under /work), "## Goal", "## Non-goals", "## Constraints", "## Steps" (the slices: a numbered list, "1. " and so on, one item per pull request in the order they are built, each beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s, then the files it changes and the new or changed types and signatures, its claim, and the test that proves the claim, its lines after the first indented so that under "## Steps" only a slice begins a line with a number and a dot; one item when the job is one pull request; a slice that moves a pin to another slice's change comes after that slice), "## Risks", "## Test Plan" (the plan's acceptance: numbered claims, each with the check that proves it and where its test lives), "## Decisions" (each design decision the resources do not record, with its alternative), "## Deferred" and "## Open Questions".`

// task is the run's Task text: section 7.4's row for its role and label,
// its additions, and the closing paragraph.
func (j *Job) task() string {
	var t []string
	add := func(format string, args ...any) { t = append(t, fmt.Sprintf(format, args...)) }
	end := closing
	if j.Role == "worker" {
		end = workerClosing
	}
	// A text that asks for a plan asks for its builder plan with it.
	builderPlan := func() { add("%s", builderSentence); end = planClosing }
	cur, open := j.current()
	names := strings.Join(j.names(), ", ")
	switch {
	case j.Batch != nil && j.Role == declare.QARole:
		product := j.Cfg.Repos[0].Name
		add(`Use %s as its users do and report what fails. In its clone /work/%s, at %s, start the app and its browser as "# Repository notes: %s" says, and drive each route and control of the page map those notes keep. Where the product differs from the page map because an issue under "# Recently closed" changed it on purpose, follow the product: that is no failure. Report each failure as a job for %s, at most %d, the most serious first; failures beyond these are found again at the next exploration. Never report one an open issue under "# Open swarm issues" already covers; a failure that an issue under "# Recently closed" was to fix and that still reproduces is a new job, naming that issue in its body. Start each job with a line "## <title>" naming what fails in at most 200 characters, then a line "Class: routine" or "Class: architecture", then the job's body: the route, the steps that reproduce the failure from a fresh start (the app restarted, a new browser session), what you expected, what happened, and the error text or log line it left. Every line beginning "## " starts a new job: inside a body use "### " or plain labels, and write no summary after the last job. A job is routine when its fix takes at most two slices (pull requests) and changes no public API and no protected path; it is architecture otherwise, and always when it changes a package's structure or public API. Class a failure routine only when you have found the code that causes it and its fix fits that rule; otherwise it is architecture. Daisy triages a routine job and accepts its plan by itself, so the maintainer first reads it at the merge; a job without the Class line is architecture. If the app does not start because of its own code (an error in its build or its start-up), that is a failure: report it as a job. If nothing fails, write "NO FAILURE: <the routes you drove>" as the only line. Name each route you could not drive, and why, at the end of that line ("; not driven: <route>: <why>"), or, when you report failures, in a last line of the last job's body. If you cannot start or drive the app for any other reason (the notes keep no page map or no way to start it, the image has no browser or lacks what the app needs), or you need the maintainer's answer to go on, write "BLOCKED: <why, or your question>" as the only line: Daisy then holds the QA until the maintainer resumes the swarm.`,
			product, product, j.Cfg.Base, product, j.Cfg.Forge.Repo, j.Batch.Propose)
		if p := j.protected(); p != "" {
			add(`The protected paths are %s.`, p)
		}
	case j.Batch != nil: // the trigger holds while no brief is active (section 6)
		add(`Propose up to %d new jobs for %s, most important first, toward the first milestone of the highest-priority brief under "# Briefs" that the closed issues do not yet meet, never proposing what an open issue already covers; a brief is a goal with milestones, each with a persona, an exit criterion and what it forces, and no steps: you cut it into jobs. Start each with a line "## <title>", then a line "Brief: <path> — <milestone>" naming the brief's path as "# Briefs" gives it and the milestone's heading, then a line "Class: routine" or "Class: architecture", then the job's body. A job is routine when it is cut from that milestone, takes at most two slices (pull requests) and changes no public API and no protected path; it is architecture otherwise, and always when it changes a package's structure or public API. Daisy triages a routine job and accepts its plan by itself, so the maintainer first reads it at the merge; a job without the Class line is architecture. When every milestone of a brief is met by closed issues, write a line "Met: <path>" before the first "## " line; write nothing else before it. If there is nothing worth proposing, write "BLOCKED: <why>" as the only line after any "Met:" lines; when what is left waits on open issues, write it "BLOCKED: waiting on #<n>, #<m>: <why>", naming each of them, and Daisy runs you again once they are all closed.`, j.Batch.Propose, j.Cfg.Forge.Repo)
		if p := j.protected(); p != "" {
			add(`The protected paths are %s.`, p)
		}
	case j.Question && j.Role == "architect":
		asker := "A member"
		if e, ok := newestQuestion(j.Issue, j.SwarmLogin); ok {
			m, _ := Swarms(e, j.SwarmLogin)
			asker = fmt.Sprintf("%s run %d", strings.ToUpper(m.Role[:1])+m.Role[1:], m.Run)
		}
		add(`%s asks a question on issue #%d, its comment beginning "QUESTION:" under "# Thread". Answer it from the plan, the thread and the resources: write "ANSWER: <your answer>" as the first line of your output, then what the asker needs with it; do not revise the plan here. If they do not settle it, write "QUESTION: <the question>" as the first line instead, restated for the owner, who answers it from the briefs or passes it to the maintainer.`, asker, j.N)
	case j.Question && j.Role == "owner":
		add(`The architect passed you a question on issue #%d, its comment beginning "QUESTION:" under "# Thread", which the plan and the resources do not settle. Answer it from the briefs, the thread and the resources: write "ANSWER: <your answer>" as the first line of your output, then what the asker needs with it. If only the maintainer can settle it, write "QUESTION: <the question>" as the first line instead: Daisy blocks the issue with it for the maintainer.`, j.N)
	case j.Role == "owner":
		if j.commented() {
			add(`Answer the latest comment in the thread of issue #%d.`, j.N)
		} else {
			add(`Issue #%d has no comment yet: say whether its body is a job the swarm can do and is worth doing next, and what is unclear in it.`, j.N)
		}
		if !j.Cfg.Gates.Triage {
			add(`Then restate the whole job as it now stands: the architect plans from your answer.`)
		}
	case j.Role == "architect" && j.Label == forge.PlanAccepted:
		why := `for each changed ruling, why they agree with it`
		if len(j.Rulings) > 0 {
			add(`Rulings changed since your plan for issue #%d was written or last confirmed: %s; revise the remaining slices or confirm they stand.`, j.N, strings.Join(j.Rulings, ", "))
		}
		if e := j.Replan; e != nil {
			add(`The maintainer asks you to re-plan issue #%d: the comment of %s at %s, beginning "Re-plan" under "# Thread", is the direction. Revise the remaining slices as it directs, or confirm they stand.`, j.N, e.Author, stamp(e.Created))
			if len(j.Rulings) > 0 {
				why += `, and why they answer the maintainer's comment`
			} else {
				why = `why they answer the maintainer's comment`
			}
		}
		switch cur, _ := j.current(); sliceOf(cur, j.N) {
		case 0:
		case 1:
			add(`Slice 1 has a pull request, under "# Earlier pull requests": keep its line as it is, with its number and repository.`)
		default:
			add(`Slices 1 to %d have pull requests, under "# Earlier pull requests": keep their lines as they are, with their numbers and repositories.`, sliceOf(cur, j.N))
		}
		if len(j.Pulls) > 0 && !cur.Open && !cur.Merged {
			add(`Pull request %s#%d, slice %d, was closed unmerged: keep its line, its number being taken, and put what it was to do, if it is still wanted, in a later slice.`, cur.Repo, cur.Number, sliceOf(cur, j.N))
		}
		add(`To revise, write the whole revised plan under the same "## " headings in the same order, not the changes to it, each slice beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s. If the remaining slices stand as they are, write "STANDS: <why>" as the first line of your output instead, then %s.`, names, why)
		builderPlan()
		if j.Cfg.Gates.Plan {
			add(`A revised plan goes to the maintainer as a plan does; one that stands goes on to the next slice.`)
		} else {
			add(`A revised plan is accepted as you write it.`)
		}
		if len(j.Cfg.Repos) > 1 {
			add(`A worker sees only its slice's repository: quote in the plan any text the worker must copy from another repository, since a plan that says to copy it from there cannot be followed.`)
		}
	case j.Role == "architect":
		if _, _, planned := j.Plan(); j.Label == forge.PlanPosted && planned {
			add(`Revise your plan for issue #%d in answer to the latest comment. Write the whole revised plan under the same "## " headings in the same order, not the changes to it, each slice beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s, a slice that moves a pin to another slice's change after that slice, a diagram where a slice touches more than one component.`, j.N, names)
			builderPlan()
		} else {
			add(`Write the plan for issue #%d as a Daisy plan, under these "## " headings in this order: `+planHeadings+` When a slice touches more than one component, add a Mermaid diagram of it (a flowchart or a sequence diagram) in a fenced "mermaid" block under that step.`, j.N, names)
			builderPlan()
			if j.Cfg.Gates.Plan {
				add(`The maintainer reads "## Research" first, then the plan, before any code is written.`)
			} else {
				add(`The plan is accepted as you write it: there is no revision round.`)
			}
		}
		if len(j.Cfg.Repos) > 1 {
			add(`A worker sees only its slice's repository: quote in the plan any text the worker must copy from another repository, since a plan that says to copy it from there cannot be followed.`)
		}
		if open {
			add(`Pull request %s#%d, slice %d, is open: it reached the review limit, or the maintainer returned the issue to you, and its reviews under "# Pull request" are the input. Its head is the branch %s in /work/%s, beside %s: "git diff %s...%s" there is the change they judged. Revise the plan so that the slice can pass review: slice %d keeps its number, its repository and its branch, and the worker continues on them; what it does, how it is tested and the slices after it may change.`, cur.Repo, cur.Number, sliceOf(cur, j.N), cur.Head, cur.Repo, j.Cfg.Base, j.Cfg.Base, cur.Head, sliceOf(cur, j.N))
		}
	case j.Role == "worker":
		s, repo, _ := j.WorkerSlice()
		_, _, planned := j.Plan()
		switch {
		case j.Label == forge.ChangesRequested:
			add(`The newest entries of pull request %s#%d's thread need an answer: the blocking findings of its latest reviews, the maintainer's comments and, when the forge found the approved head not mergeable, Daisy's direction to merge %s. Address each on the branch checked out in /work (swarm/%d-%d), or say why you did not, and write what you changed, point by point. If none needs a change to the code, make no commit and write "NO CHANGE: <why>" as the first line of your output, then your answer point by point: the reviewers judge it against the unchanged head.`, cur.Repo, cur.Number, j.Cfg.Base, j.N, s)
		case planned:
			plan, _ := j.PlanText()
			add(`Implement slice %d of the accepted plan for issue #%d (it declares %d) in %s/%s, on the branch checked out in /work (swarm/%d-%d). Do not change the plan's Test Plan; in the pull request's description, or in your comment when you continue an open pull request, report each Test Plan claim's status: done, not in this slice, or failing.`, s, j.N, max(1, len(Slices(plan))), owner(j.Cfg), repo, j.N, s)
		default:
			add(`There is no plan: the issue body and the maintainer's comments under Direction are the job of issue #%d. Keep the change small; if it needs a plan, write "BLOCKED: needs a plan". Work on the branch checked out in /work (swarm/%d-%d) and write the pull request's description.`, j.N, j.N, s)
		}
		add(`Before anything else, run "git merge --no-edit %s"; if it conflicts, resolve each conflict, keeping what both sides meant, and commit the merge, or, if you cannot, write "BLOCKED: %s conflicts in <files>" instead, naming the conflicting files.`, j.Cfg.Base, j.Cfg.Base)
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
		add(`If the change on your branch alters what a page of the product shows, render each page it alters once your last commit is made, the way the repository notes give, and save a screenshot of each directly in /work/.swarm/artifacts/ as a .png, .jpg, .jpeg or .webp file in the format its extension names, whose name is letters, digits, ".", "-" and "_", beginning with a letter or a digit, and says which page it shows: Daisy attaches them to the pull request and shows them to the reviewers, and skips any other file there. List under "Not established:" each page you could not render, and why, a page the repository notes give no way to render among them.`)
	default: // the reviewers
		s := sliceOf(cur, j.N)
		add(`Review pull request %s#%d, slice %d of issue #%d. The change is "git diff %s...HEAD" in /work, a throwaway clone you may build and test in; nothing you change there is kept. Check each claim of the plan's Test Plan against the diff and the check's result, and the worker's "Not established:" lists against what the change claims: a claim neither tested nor listed is a finding. The first line of your output must be exactly "VERDICT: approve" or "VERDICT: request-changes"; then the review, each finding a line beginning "[blocking]" or "[non-blocking]" with its file and line. Request changes only when a finding is blocking; approve otherwise.`, cur.Repo, cur.Number, s, j.N, j.Cfg.Base)
		if j.Role == "adversary" {
			add(`For each test that slice %d's item under the plan's Steps names, and for each Test Plan claim the worker's newest report (the description, or its comment when it continued the pull request) reports done, check that its test exists on this head and that it fails against %s's code: in /work, a throwaway clone, put everything the change touches except its tests back as %s has it (check out %s's version of each path, deleting those %s lacks), run the test as the head has it (a build failure counts as failing), then restore the head ("git checkout HEAD -- ." and remove what it adds back). A test that is missing, or that passes against %s's code, is a "[blocking]" finding. Three cases that rollback cannot judge are checked another way, and are findings only when the test does not exercise its claim: a test whose subject the change keeps in test files (the rollback leaves them in place), and a claim met by a test the change does not touch (it passed against %s already), both checked by reading; and a test of behaviour the change keeps, where the plan or the report says the change removes code, restructures it or changes its cost but not its results (such a test passes against %s by design), checked by breaking the live path it exercises in the throwaway clone and seeing it fail, or by the worker's own report of that probe. A claim reported not in this slice is not checked here.`, s, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base)
		}
		if len(j.Artifacts) > 0 {
			add(`The worker saved screenshots of pages the change alters, listed under "# Artifacts": open each from /work/.swarm/artifacts/ (the URL beside it is the pull request's copy, and "not attached" means the forge refused the file) and judge what the page shows against what the plan and the slice claim, as you judge the diff. A page that does not show what is claimed is a finding, naming the screenshot and the file and line that render what it shows. If you cannot open an image, say so in the review and judge that page from the diff alone.`)
		}
		switch r, prev := j.round(cur); {
		case r > 1 && prev == cur.HeadSHA && !j.answeredSince(prev):
			add(`This is round %d, on the head you reviewed before (%s): the maintainer commented in the thread, with no commit since. First answer the maintainer's comment; then say, for each blocking finding of your previous review, whether it still stands in its light, and raise a new blocking finding only where the comment shows one.`, r, prev)
		case r > 1 && prev == cur.HeadSHA:
			add(`This is round %d, on the head you reviewed before (%s): the worker answered in the thread with "NO CHANGE:" and no commit. First say, for each blocking finding of your previous review, whether the answer resolves it with a reason you accept; raise no new blocking finding, since the code has not changed.`, r, prev)
		case r > 1:
			add(`This is round %d. First say, for each blocking finding of your review of %s, whether it is resolved, or answered with a reason you accept; raise new blocking findings only on code changed since %s in the files the pull request changes ("git diff --name-only -z %s...HEAD | xargs -0 -r git diff %s..HEAD --"): what a merge of %s brought in is not this pull request's change.`, r, prev, prev, j.Cfg.Base, prev, j.Cfg.Base)
		}
	}
	if texts := j.cutTexts(); texts != "" {
		add(`Daisy cuts a text longer than 60,000 bytes when it posts it on the forge, and these texts in your prompt were cut: %s. In each, a line "[truncated by Daisy; the whole output is in <file>]" marks the cut and names the file that holds the member's whole output; read the file before you act on the text, since what was cut off may be what you must act on.`, texts)
		if slices.ContainsFunc(j.cut, func(run string) bool { return strings.Contains(run, "-worker-") }) {
			add(`A worker's text keeps Daisy's "**Check:**" result after that line, which the file does not hold.`)
		}
	}
	if j.Role == "worker" || j.Role == "adversary" || j.Role == "sympathetic" {
		add(`When the plan, the issue and its thread leave open something your work depends on, do not guess: write "QUESTION: <one question>" as the first line of your output instead; the architect answers it, and you run again with the answer under "# Questions".`)
	}
	if j.Cfg.Notes && writesNotes(j.Role) {
		add("%s", j.notesTask())
	}
	if (j.Role == "architect" || j.Role == "worker") && j.Batch == nil {
		if p := j.protected(); p != "" {
			add(`These paths are protected: %s. No change to them is pushed; the maintainer makes it. Leave such a change out and say so.`, p)
		}
	}
	return strings.Join(t, " ") + "\n\n" + end
}

// cutTexts is the Task text's list of the texts the prompt shows cut
// (Cut), each with the file that holds it whole: the plan named so, any
// other by its role and run; "" when none is cut.
func (j *Job) cutTexts() string {
	_, pm, planned := j.Plan()
	var out []string
	for _, run := range j.cut {
		f := strings.Split(run, "-")
		what := fmt.Sprintf("%s run %s's text", f[1], f[2])
		if planned && run == outputRun(j.N, pm) {
			what = "the plan"
		}
		out = append(out, what+", whole in "+OutputsMount+"/"+run+".md")
	}
	return strings.Join(out, "; ")
}

// commented: the issue has a counted comment (section 6).
func (j *Job) commented() bool {
	for _, c := range j.Issue.Comments {
		if counted(Entry{c.Author, c.Body, c.Created}, j.SwarmLogin, j.Cfg.Trusted) {
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

// answeredSince: the worker answered without a commit (Answers) after
// the role's newest review of head, so what reopened it is the worker's
// answer, not the maintainer's comment alone (section 6).
func (j *Job) answeredSince(head string) bool {
	var at time.Time
	for _, r := range j.Pull.Reviews {
		if m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, j.SwarmLogin); ok && m.Role == j.Role && r.Commit == head && r.Submitted.After(at) {
			at = r.Submitted
		}
	}
	answers := Answers(pullComments(j.Pull.Comments), j.SwarmLogin)
	return len(answers) > 0 && answers[len(answers)-1].After(at)
}

// round is the reviewer's round on the current head (section 6: the
// rounds before this head as it stands, since the accepted plan, plus
// this one) and the commit of the role's newest review on the pull
// request before it: an earlier head's, or this head's before its
// newest reopening (Reopenings).
func (j *Job) round(p forge.Pull) (int, string) {
	if j.Pull == nil {
		return 1, ""
	}
	var since time.Time
	if plan, _, ok := j.Plan(); ok {
		since = plan.Created
	}
	answers := Reopenings(pullComments(j.Pull.Comments), j.SwarmLogin, j.Cfg.Trusted)
	prev, prevAt := "", time.Time{}
	for _, r := range j.Pull.Reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, j.SwarmLogin)
		standing := r.Commit == p.HeadSHA && standsOnHead(answers, r.Submitted)
		if ok && m.Role == j.Role && !standing && (since.IsZero() || r.Submitted.After(since)) && !r.Submitted.Before(prevAt) {
			prev, prevAt = r.Commit, r.Submitted
		}
	}
	return Rounds(j.Pull.Reviews, answers, j.SwarmLogin, since, p.HeadSHA) + 1, prev
}
