package reconcile

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/fence"
	"daisy/internal/swarm/forge"
)

// closing is the paragraph before every Task text's last line (section
// 7.4), workerClosing the worker's, whose screenshots are read back too,
// and planClosing the architect's that ask for a plan, whose builder plan
// is.
var (
	closing       = closingOf("that file is the only thing read back")
	workerClosing = closingOf("that file and the screenshots in /job/io/artifacts/ are the only things read back")
	planClosing   = closingOf("that file and the builder plan in /job/io/builder.md are the only things read back")
)

func closingOf(readBack string) string {
	return "Write your output to /job/io/out.md; " + readBack + ", and nothing you write elsewhere outside /job survives. If you cannot do this without an answer from the maintainer, write `BLOCKED: <your question>` as the first line of /job/io/out.md instead. " + causeSentence
}

// builderSentence follows the plan's text in each architect Task text
// that asks for a plan or a revised plan (section 7.4; 7.8, The builder
// plan).
const builderSentence = `Then write the builder plan to /job/io/builder.md, a document of its own, not posted: the plan consolidated for whoever builds it, one section "## Slice <s>" per slice under "## Steps", in their order, each holding all a builder needs for that slice (its files, types and signatures, claim and test, its tests' probes and its "Unsupported:" line, and what of the research, constraints and decisions bears on it), so that it reads without the plan. Daisy gives each worker its slice's step and section. Write it whole each time you write the plan.`

// probesSentence follows the plan's text, before the builder sentence, in
// each architect Task text that asks for a plan or a revised plan
// (section 7.11, rules 3 and 4): each test's probe and each slice's
// unsupported setups, which the worker and the reviewers read and Daisy
// does not.
const probesSentence = `The plan holds two lines the worker and the reviewers read:

- under "## Test Plan", for each claim's test, "Probe: <the change to the code under test that the test must fail on>": a change the code can make and its test can tell (not an equivalent one, nor a fixture no test reads), or, for a guard of kept behaviour, the break of the live path it exercises;
- in each slice's item under "## Steps", "Unsupported: <the setups the slice does not support>", or "Unsupported: none".`

// dependsSentence follows the probes in each architect Task text that
// asks for a plan or a revised plan (section 12.1, rules 1 and 2): the
// lines Daisy reads to order the job and its slices.
const dependsSentence = `The plan also holds the lines Daisy reads to order the work, each alone on its line and outside any fenced block:

- once, "Depends: none" or "Depends: #<a>, #<b>", naming the open issues the job waits on: Daisy starts nothing of the job and merges nothing of it while one of them is open, and the plan's line stands in place of the proposal's;
- in each slice's item under "## Steps", "Touches: <pattern>, <pattern>", the paths the slice changes, each written as a protected pattern is (a directory ending "/", a file name without "/" at any depth, or a path that may hold "*"): Daisy does not start a slice's first build while it overlaps an open slice of another issue;
- in a slice's item, "Safe with: #<m>, #<n>" only when the slice may be built beside those issues' open slices though their paths overlap.`

// defectSentence is the batch texts' (section 7.11, rule 7): the line that
// counts a job among a merged pull request's escapes.
const defectSentence = `When you find that a failure's cause is the change of a merged swarm pull request, name it in the job's body with a line "Defect in: <name>#<m>".`

const planHeadings = `"## Research" (what exists: the files and seams the job touches, what connects to what, and what the change affects, each line naming a path under /job/work), "## Goal", "## Non-goals", "## Constraints", "## Steps" (the slices: a numbered list, "1. " and so on, one item per pull request in the order they are built, each beginning with the name of the repository it changes and a colon ("1. <name>: "), one of %s, then the files it changes and the new or changed types and signatures, its claim, and the test that proves the claim, its lines after the first indented so that under "## Steps" only a slice begins a line with a number and a dot; one item when the job is one pull request; a slice that moves a pin to another slice's change comes after that slice), "## Risks", "## Test Plan" (the plan's acceptance: numbered claims, each with the check that proves it and where its test lives), "## Decisions" (each design decision the resources do not record, with its alternative), "## Deferred" and "## Open Questions".`

// outputFile begins every role's # Output block and outputReminder the last
// line of every Task text (section 7.4; decision 167 as amended): the file
// the member writes, which a member that met it only in the closing
// paragraph left unwritten, its final message the output.
const (
	outputFile     = "Write your output to the file `/job/io/out.md`: Daisy reads your output from it alone, never from your final message."
	outputReminder = `Write your output to /job/io/out.md, not in your final message.`
)

// roleOutputs is each role's # Output block after outputFile (sections 7.3
// and 7.4; decision 167 as amended by the verdict fix): the first lines
// its output may or must begin with, which Render writes right after
// # Persona. Its text is the role's alone, never the run's, so that it
// stays in the prefix every run of the role shares.
var roleOutputs = map[string]string{
	"owner": "Your output is what the Task asks for, and its first line is one of these only where the Task asks for it, alone on its line:\n\n" +
		"- `ANSWER: <your answer>`, or `QUESTION: <the question>` when only the maintainer can settle it: on a question passed to you, one of the two is the required first line;\n" +
		"- in a proposal run, any lines `Met: <path>` first, then each job from its line `## <title>`; or `BLOCKED: <why>` as the only line after any `Met:` lines;\n" +
		"- `BLOCKED: <your question>`, only when you cannot do the Task without an answer from the maintainer.",
	"architect": "Your output is what the Task asks for, and its first line is one of these only where the Task asks for it, alone on its line:\n\n" +
		"- `STANDS: <why>`, only when the Task asks you to revise an accepted plan or confirm it and its remaining slices stand;\n" +
		"- `ANSWER: <your answer>`, or `QUESTION: <the question>` restated for the owner when the plan, the thread and the resources do not settle it: on a member's question, one of the two is the required first line;\n" +
		"- `BLOCKED: <your question>`, only when you cannot do the Task without an answer from the maintainer.",
	"worker": "Your output is your report, which Daisy posts as the pull request's description or as your comment on it. Its first line is one of these only in the case the Task names for it, alone on its line, and none is required:\n\n" +
		"- `NO CHANGE: <why>`, only when you answer a pull request's thread and no point needs a change to the code;\n" +
		"- `QUESTION: <one question>`, only when the plan, the issue and its thread leave open something your work depends on;\n" +
		"- `BLOCKED: <why, or your question>`, only when you cannot go on without the maintainer, as the Task says.",
	declare.QARole: "Your output's first line is one of these three, as the Task says:\n\n" +
		"- each failure's job from its line `## <title>`;\n" +
		"- `NO FAILURE: <the routes you drove>` as the only line, when nothing fails;\n" +
		"- `BLOCKED: <why, or your question>` as the only line, when you cannot start or drive the app.",
}

// reviewerOutput is the reviewers': the verdict line verdictOf reads, the
// findings whose severity review checks against it, and an approval's
// account of what it checked (sections 7.4 and 7.5; decision 172).
const reviewerOutput = "The first line of your output is exactly `VERDICT: approve` or `VERDICT: request-changes`, alone on its line, before any heading or other text. Then the review: each finding a line beginning `[blocking]` or `[non-blocking]`, with its file and line. Request changes only for a blocking finding, of a kind the Task lists; approve otherwise. An approval accounts for what you checked, after its verdict line: one line `[checked] claim <n>: <how it was checked; the probe and what it showed>` for each claim of the plan's Test Plan, `<n>` its number there (for a claim not in this slice, say so and how you know), and, from round 2, one line `[closed] <the earlier blocking finding's file:line or kind>: <how its reproduction now passes>` for each blocking finding of the previous round, or of your last review, that you judge closed. Daisy reads the verdict from the first line alone, so a review whose first line is anything else (`## Review`, a title, a blank line) fails. For example:\n\n" +
	"```text\nVERDICT: request-changes\n[blocking] <kind> <file>:<line>: <what is wrong>; reproduced by <a command, a test or the steps>\n[non-blocking] <file>:<line>: <what could be better>\n```\n\n" +
	"or, with no blocking finding:\n\n" +
	"```text\nVERDICT: approve\n[checked] claim <n>: <how it was checked; the probe and what it showed>\n[closed] <the earlier blocking finding's file:line or kind>: <how its reproduction now passes>\n[non-blocking] <file>:<line>: <what could be better>\n```\n\n" +
	"Two other first lines replace the verdict, each only in the case the Task names for it: `QUESTION: <one question>`, when the plan, the issue and its thread leave open something your review depends on, and `BLOCKED: <your question>`, when you cannot review without an answer from the maintainer."

// Output is role's # Output block, "" for a role that has none.
func Output(role string) string {
	contract := roleOutputs[role]
	if role == "adversary" || role == "sympathetic" {
		contract = reviewerOutput
	}
	if contract == "" {
		return ""
	}
	return outputFile + "\n\n" + contract
}

// The reminders end the last line of the Task texts whose first line is
// required (section 7.4): the reviewers', and the architect's and the
// owner's on a question. A reviewer's goes on with the lines an approval
// holds (decision 172): checkedReminder in every round, closedReminder
// from round 2.
const (
	verdictReminder   = `Begin your output with the line "VERDICT: approve" or "VERDICT: request-changes".`
	checkedReminder   = `An approval then holds a line "[checked] claim <n>: <how you checked it>" for each claim of the plan's Test Plan.`
	closedReminder    = `From this round it also holds a line "[closed] <the finding's file:line or kind>: <how its reproduction now passes>" for each earlier blocking finding you judge closed.`
	architectReminder = `Begin your output with the line "ANSWER: <your answer>", or with "QUESTION: <the question>" when the plan, the thread and the resources do not settle it.`
	ownerReminder     = `Begin your output with the line "ANSWER: <your answer>", or with "QUESTION: <the question>" when only the maintainer can settle it.`
)

// blockingSentence is the list of what blocks a review (section 7.11,
// rule 1; decision 171), the same for both reviewers in every round of
// slice s but for the repository notes' trust boundaries: "realistic",
// which bounds every kind (rule 1 as built at Step R6-F, the
// maintainer's ruling A4), is read by them, or by the default where the
// notes declare none (rule 2). Daisy does not read whether the slice
// changes only documentation (ruling A3): the claim's item says how a
// claim blocks then, for the reviewer to apply.
func (j *Job) blockingSentence(s int) string {
	return fmt.Sprintf(`A finding is blocking only when it is one of these four kinds:

- claim: a claim of slice %d that the head does not show, a test that survives the probe the plan names for it among them; on a slice that changes only documentation, only a false statement the slice added or changed;
- test: a test the plan names for slice %d that is missing on this head or fails;
- defect: a defect in the code the change adds or changes;
- introduced: a fault this revision introduced, behaviour that worked on a head reviewed before and fails on this one.

A finding of any kind blocks only when a realistic user, input or repository state meets it, "realistic" as %s; for tooling that builds or tests the code, a case the repository has today, not one it could have. A test that survives the probe the plan names for it still blocks: the plan named that case.

Every other finding is non-blocking, however sound: hardening against what the notes trust, a fault met only in a setup the slice does not support, a probe of your own that a test survives (unless the change that survives shows the head itself is wrong), and wording. Write a blocking finding's kind after its marker (from round 2, "unclosed" for an earlier one not closed) and, on the same line, the reproduction that shows it: a command, a test or the steps. Request changes only for a blocking finding; Daisy files every non-blocking one for the owner when the pull request merges, so none is lost.`, s, s, j.realistic())
}

// realistic is what "realistic" is read as (section 7.11, rules 1, 2 and
// 4): the trust boundaries the repository notes declare, or the default
// where they declare none, and the slice's unsupported setups.
func (j *Job) realistic() string {
	if !j.trustBoundaries() {
		return `the slice's "Unsupported:" line defines it, and, since the repository notes declare no trust boundaries, trusting the product's own process environment, configuration, filesystem and umask and the toolchain's settings, and nothing a user or a network peer sends`
	}
	return `"Trust boundaries" in the repository notes and the slice's "Unsupported:" line define it`
}

// judgement is the architect's text at the round limit's first time on
// pull request p (section 7.11, rule 8; option (b) of decision 171): each
// blocking finding of its last round judged against rules 1 and 5, and
// OVERRULED: when none qualifies.
func (j *Job) judgement(p forge.Pull) string {
	s := sliceOf(p, j.N)
	return fmt.Sprintf(`This is the first time pull request %s#%d reached the review round limit: before you revise, judge each blocking finding of its last round, under "# Pull request". `+
		`A finding qualifies only when it is one of the kinds that block (a claim of slice %d that the head does not show, a test that survives the probe the plan names for it among them, on a slice that changes only documentation only a false statement the slice added or changed; a test the plan names for slice %d missing or failing; a defect in the code the change adds or changes; a fault the revision introduced), `+
		`a realistic user, input or repository state meets it, "realistic" as %s, `+
		`and, from round 2, it is an earlier blocking finding not closed, a fault the revision introduced or a claim of slice %d that the head does not show. `+
		`When none qualifies, write "%s <why, finding by finding>" as the first line of your output instead of a plan: Daisy posts it on the pull request, the sympathetic reviewer reviews the head as it would after the adversary's approval, and the findings you overrule are filed for the owner, marked "%s". When any qualifies, revise the plan as above.`,
		p.Repo, p.Number, s, s, j.realistic(), s, overruled, outstanding)
}

// trustBoundaries is whether the notes of the repository the run works in
// hold the heading `## Trust boundaries`, outside a fenced block (section
// 4; 7.11, rule 2).
func (j *Job) trustBoundaries() bool {
	for _, n := range j.Repos {
		var fences fence.Scan
		for _, l := range strings.Split(n.Text, "\n") {
			if !fences.Line(l) && strings.TrimRight(l, " \t\r") == "## Trust boundaries" {
				return true
			}
		}
	}
	return false
}

// task is the run's Task text: section 7.4's row for its role and label,
// its additions, the closing paragraph and the last line, the output's
// file and, where the Task's first line is required, the reminder
// restating it. What add adds runs on in one paragraph; what para adds is
// a paragraph of its own, for an instruction that is missed in the middle
// of one (decision 167).
func (j *Job) task() string {
	var t []string
	alone := true
	add := func(format string, args ...any) {
		text := fmt.Sprintf(format, args...)
		if alone {
			t, alone = append(t, text), false
			return
		}
		t[len(t)-1] += " " + text
	}
	para := func(text string) { t, alone = append(t, text), true }
	end, last := closing, outputReminder
	if j.Role == "worker" {
		end = workerClosing
	}
	// A text that asks for a plan asks for its probes and its builder plan
	// with it.
	builderPlan := func() { para(probesSentence); para(dependsSentence); add("%s", builderSentence); end = planClosing }
	cur, open := j.current()
	names := strings.Join(j.names(), ", ")
	switch {
	case j.Batch != nil && j.Role == declare.QARole:
		product := j.Cfg.Repos[0].Name
		add(`Use %s as its users do and report what fails. In its clone /job/work/%s, at %s, start the app and its browser as "# Repository notes: %s" says, and drive each route and control of the page map those notes keep. Where the product differs from the page map because an issue under "# Recently closed" changed it on purpose, follow the product: that is no failure. Report each failure as a job for %s, at most %d, the most serious first; failures beyond these are found again at the next exploration. Never report one an open issue under "# Open swarm issues" already covers; a failure that an issue under "# Recently closed" was to fix and that still reproduces is a new job, naming that issue in its body. Start each job with a line "## <title>" naming what fails in at most 200 characters, then a line "Class: routine" or "Class: architecture", then the job's body: the route, the steps that reproduce the failure from a fresh start (the app restarted, a new browser session), what you expected, what happened, and the error text or log line it left. Every line beginning "## " starts a new job: inside a body use "### " or plain labels, and write no summary after the last job. A job is routine when its fix takes at most two slices (pull requests) and changes no public API and no protected path; it is architecture otherwise, and always when it changes a package's structure or public API. Class a failure routine only when you have found the code that causes it and its fix fits that rule; otherwise it is architecture. Daisy triages a routine job and accepts its plan by itself, so the maintainer first reads it at the merge; a job without the Class line is architecture. If the app does not start because of its own code (an error in its build or its start-up), that is a failure: report it as a job. If nothing fails, write "NO FAILURE: <the routes you drove>" as the only line. Name each route you could not drive, and why, at the end of that line ("; not driven: <route>: <why>"), or, when you report failures, in a last line of the last job's body. If you cannot start or drive the app for any other reason (the notes keep no page map or no way to start it, the image has no browser or lacks what the app needs), or you need the maintainer's answer to go on, write "BLOCKED: <why, or your question>" as the only line: Daisy then holds the QA until the maintainer resumes the swarm.`,
			product, product, j.Cfg.Base, product, j.Cfg.Forge.Repo, j.Batch.Propose)
		para(defectSentence)
		if p := j.protected(); p != "" {
			add(`The protected paths are %s.`, p)
		}
	case j.Batch != nil: // the trigger holds while no brief is active (section 6)
		add(`Propose up to %d new jobs for %s, most important first, toward the first milestone of the highest-priority brief under "# Briefs" that the closed issues do not yet meet, never proposing what an open issue already covers; a brief is a goal with milestones, each with a persona, an exit criterion and what it forces, and no steps: you cut it into jobs. Start each with a line "## <title>", then a line "Brief: <path> — <milestone>" naming the brief's path as "# Briefs" gives it and the milestone's heading, then a line "Class: routine" or "Class: architecture", then a line "Depends: none", or "Depends: #<a>, #<b>" naming the open issues the job waits on, then a line "Touches: <pattern>, <pattern>" naming the paths you expect it to change, each written as a protected pattern is (a directory ending "/", a file name without "/" at any depth, or a path that may hold "*"), then the job's body. Daisy starts nothing of a job while an issue its Depends: line names is open, so work that waits on open issues is proposed all the same, its Depends: line naming them. A job is routine when it is cut from that milestone, takes at most two slices (pull requests) and changes no public API and no protected path; it is architecture otherwise, and always when it changes a package's structure or public API. Daisy triages a routine job and accepts its plan by itself, so the maintainer first reads it at the merge; a job without the Class line is architecture. When every milestone of a brief is met by closed issues, write a line "Met: <path>" before the first "## " line; write nothing else before it. Each issue under "# Follow-ups" holds review findings that did not block a merge. Check each item against %s under /job/work; a job that fixes items worth fixing (a defect a user can meet, a test that cannot fail, or a finding the reviews keep repeating) is proposed like any other, with a line "Follow-up: #<f> <i>, <j>" after its "Class:" line naming the issue and the items it fixes; findings of one cause are one job. When no item of a follow-up is left to propose (each proposed, fixed already, met only where the repository notes trust, in a setup the slice does not support, or not worth a job), write a line "Triaged: #<f>: <why>" among the "Met:" lines, before the first "## " line or your "BLOCKED:" line: those lines alone may come before it. Daisy closes the issue with your reason. For a follow-up you leave open, you may write a line "Retitled: #<f>: <title>" there too, its title naming the findings' cause; Daisy sets the issue's title to it. If there is nothing worth proposing even so, write "BLOCKED: <why>" as the only line after any "Met:" lines; when what is left waits on open issues, write it "BLOCKED: waiting on #<n>, #<m>: <why>", naming each of them, and Daisy runs you again once they are all closed.`, j.Batch.Propose, j.Cfg.Forge.Repo, j.Cfg.Base)
		if len(j.Batch.Main) > 0 {
			para(fmt.Sprintf(`"# Main" names each repository whose %s is red: its check fails there, and Daisy launches no worker on it but the fix's. Propose its fix first, one job for each red repository, before any other, with a line "Fixes: <name>" naming the repository after its "Class:" line; that job is the fix's.`, j.Cfg.Base))
		}
		para(defectSentence)
		if p := j.protected(); p != "" {
			add(`The protected paths are %s.`, p)
		}
	case j.Question && j.Role == "architect":
		asker := "A member"
		if e, ok := newestQuestion(j.Issue, j.SwarmLogin); ok {
			m, _ := Swarms(e, j.SwarmLogin)
			asker = fmt.Sprintf("%s run %d", strings.ToUpper(m.Role[:1])+m.Role[1:], m.Run)
		}
		add(`%s asks a question on issue #%d, its comment beginning "QUESTION:" under "# Thread". Answer it from the plan, the thread and the resources, then write what the asker needs with it; do not revise the plan here. If they do not settle it, restate the question for the owner, who answers it from the briefs or passes it to the maintainer.`, asker, j.N)
		last += " " + architectReminder
	case j.Question && j.Role == "owner":
		add(`The architect passed you a question on issue #%d, its comment beginning "QUESTION:" under "# Thread", which the plan and the resources do not settle. Answer it from the briefs, the thread and the resources, then write what the asker needs with it. If only the maintainer can settle it, restate the question: Daisy blocks the issue with it for the maintainer.`, j.N)
		last += " " + ownerReminder
	case j.Role == "owner":
		if j.commented() {
			add(`Answer the latest comment in the thread of issue #%d.`, j.N)
		} else {
			add(`Issue #%d has no comment yet: say whether its body is a job the swarm can do and is worth doing next, and what is unclear in it.`, j.N)
		}
		if !j.Cfg.Gates.Triage {
			add(`Then restate the whole job as it now stands: the architect plans from your answer.`)
		}
		if p := j.protected(); p != "" {
			add(`The protected paths are %s.`, p)
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
			add(`Pull request %s#%d, slice %d, is open: it reached the review limit, or the maintainer returned the issue to you, and its reviews under "# Pull request" are the input. Its head is the branch %s in /job/work/%s, beside %s: "git diff %s...%s" there is the change they judged. Revise the plan so that the slice can pass review: slice %d keeps its number, its repository and its branch, and the worker continues on them; what it does, how it is tested and the slices after it may change.`, cur.Repo, cur.Number, sliceOf(cur, j.N), cur.Head, cur.Repo, j.Cfg.Base, j.Cfg.Base, cur.Head, sliceOf(cur, j.N))
			if AtTheLimitsFirst(j.Issue, cur, owner(j.Cfg), j.SwarmLogin, j.Cfg.Trusted) {
				para(j.judgement(cur))
			}
		}
	case j.Role == "worker":
		s, repo, _ := j.WorkerSlice()
		_, _, planned := j.Plan()
		switch {
		case j.Label == forge.ChangesRequested:
			add(`The newest entries of pull request %s#%d's thread need an answer: the blocking findings of its latest reviews, the maintainer's comments and, when the forge found the approved head not mergeable, Daisy's direction to merge %s, or, when checks failed on it, Daisy's note naming them. Address each on the branch checked out in /job/work/%s (swarm/%d-%d), or say why you did not, and write what you changed, point by point. If none needs a change to the code, make no commit and write "NO CHANGE: <why>" as the first line of your output, then your answer point by point: the reviewers judge it against the unchanged head.`, cur.Repo, cur.Number, j.Cfg.Base, repo, j.N, s)
			para(`A blocking finding that is none of the kinds that block (a claim of the slice the head does not show, on a slice that changes only documentation only a false statement the slice added or changed; a test the plan names missing or failing; a defect in the changed code; a fault the revision introduced), or that no realistic user, input or repository state meets, is answered, not fixed: say why it does not block. A non-blocking finding needs no change; Daisy files it for the owner.`)
		case planned:
			plan, _ := j.PlanText()
			add(`Implement slice %d of the accepted plan for issue #%d (it declares %d) in %s/%s, on the branch checked out in /job/work/%s (swarm/%d-%d). Do not change the plan's Test Plan; in the pull request's description, or in your comment when you continue an open pull request, report each Test Plan claim's status: done, not in this slice, or failing.`, s, j.N, max(1, len(Slices(plan))), owner(j.Cfg), repo, repo, j.N, s)
			para(`For each test of this slice whose Test Plan claim names a Probe, apply the probe, run the test and restore the code: report, in the description or your comment, the test, the probe and the failure it showed, or that it survived.`)
		default:
			add(`There is no plan: the issue body and the maintainer's comments under Direction are the job of issue #%d. Keep the change small; if it needs a plan, write "BLOCKED: needs a plan". Work on the branch checked out in /job/work/%s (swarm/%d-%d) and write the pull request's description.`, j.N, repo, j.N, s)
		}
		if read := j.sliceSentence(); read != "" {
			add("%s", read)
		}
		add(`Before anything else, run "git merge --no-edit %s"; if it conflicts, resolve each conflict, keeping what both sides meant, and commit the merge, or, if you cannot, write "BLOCKED: %s conflicts in <files>" instead, naming the conflicting files.`, j.Cfg.Base, j.Cfg.Base)
		if r := j.Red; r != nil {
			add(`%s of %s is red at %s since this job's merge: its check fails there, its output in /job/memory/checks/%s.log. Make your branch's first commit the fix, so that the check passes on %s, then go on with the slice.`, j.Cfg.Base, r.Name, short(r.SHA), r.SHA, j.Cfg.Base)
		}
		add(`Commit with git; do not push. Never amend, rebase or reset a commit that is already on the branch; add commits.`)
		if check := j.repo(repo).Check; len(check) > 0 {
			add(`Run "%s" before you finish and quote its last result.`, declare.ShellLine(check))
			para(fmt.Sprintf(`Daisy runs "%s" on the head you push, in the repository's declared check environment with its test services, and the forge's required checks run before the merge: those gates are Daisy's, and a failure of theirs comes back to you. A part of them your container cannot run (a service or a network it lacks) goes under "Not established:"; it is no reason to write BLOCKED:.`, declare.ShellLine(check)))
		} else {
			add(`No check command is declared; say how you tested.`)
			para(`The forge's required checks run before the merge: they are Daisy's gates, and a failure of theirs comes back to you.`)
		}
		add(`End what you write with a line "Not established:" and list what you did not run or could not verify (live services, other platforms, behaviour that needs a network), or "none".`)
		if planned && s > 1 {
			add(`A pin to an earlier slice's change names its merged commit, listed under Earlier pull requests, never a branch or an unmerged head; if this slice needs an earlier slice that is not merged, write "BLOCKED: slice <i> is not merged".`)
		}
		add(`If the change on your branch alters what a page of the product shows, render each page it alters once your last commit is made, the way the repository notes give, and save a screenshot of each directly in /job/io/artifacts/ as a .png, .jpg, .jpeg or .webp file in the format its extension names, whose name is letters, digits, ".", "-" and "_", beginning with a letter or a digit, and says which page it shows: Daisy attaches them to the pull request and shows them to the reviewers, and skips any other file there. List under "Not established:" each page you could not render, and why, a page the repository notes give no way to render among them.`)
	default: // the reviewers
		s := sliceOf(cur, j.N)
		r, prev := j.round(cur)
		last += " " + verdictReminder + " " + checkedReminder
		if r > 1 {
			last += " " + closedReminder
		}
		// The sympathetic reviewer reads its clone and runs nothing (section
		// 7.11, The defaults).
		clone := "a throwaway clone you may build and test in"
		if j.Role == "sympathetic" {
			clone = "a throwaway clone"
		}
		delta := r > 1 && prev != "" && prev != cur.HeadSHA
		if delta {
			// A delta review (Step E2): the previous round's findings and
			// what changed since the head the reviewer last reviewed, its
			// own, the full change readable but not the task.
			add(`This is round %d of pull request %s#%d, slice %d of issue #%d: review what changed since %s, the head you last reviewed. The change to review is "git diff --name-only -z %s...HEAD | xargs -0 -r git diff %s..HEAD --" in /job/work/%s, %s; nothing you change there is kept, and what a merge of %s brought in is not this pull request's change. The whole change, "git diff %s...HEAD", is there to read where the revision needs it; reviewing it again is not the task. First, for each blocking finding of the previous round, %s, and of your review of %s, say whether it is closed, or answered with a reason you accept. Then review what changed: check each claim of the plan's Test Plan the revision bears on against it and the check's result, and the worker's "Not established:" lists against what the revision claims.`,
				r, cur.Repo, cur.Number, s, j.N, prev, j.Cfg.Base, prev, cur.Repo, clone, j.Cfg.Base, j.Cfg.Base, j.previousFindings(), prev)
		} else {
			add(`Review pull request %s#%d, slice %d of issue #%d. The change is "git diff %s...HEAD" in /job/work/%s, %s; nothing you change there is kept. Check each claim of the plan's Test Plan against the diff and the check's result, and the worker's "Not established:" lists against what the change claims: a claim neither tested nor listed is a finding.`, cur.Repo, cur.Number, s, j.N, j.Cfg.Base, cur.Repo, clone)
		}
		para(j.blockingSentence(s))
		switch { // a later round narrows the list (section 7.11, rule 5)
		case delta:
			para(fmt.Sprintf(`A blocking finding of the previous round or of your last review is closed when its own reproduction now passes on this head. From this round, only three kinds of finding block, each as the paragraphs above bound it: an earlier blocking finding not closed, a fault this revision introduced, and a claim of slice %d that the head does not show; any other new finding, a new variant of a closed one among them, is non-blocking. A non-blocking finding the previous round or your last review already raised stands as written there: do not raise it again.`, s))
		case r > 1 && prev == "": // the reviewer's first review, in a later round
			para(fmt.Sprintf(`This is round %d of the pull request: a new finding blocks only when it is a claim of slice %d that the head does not show, or a fault the latest revision introduced, each as the paragraphs above bound it; any other is non-blocking.`, r, s))
		}
		if read := j.sliceSentence(); read != "" {
			add("%s", read)
		}
		if j.Role == "adversary" {
			add(`For each test that slice %d's item under the plan's Steps names, and for each Test Plan claim the worker's newest report (the description, or its comment when it continued the pull request) reports done, check that its test exists on this head and that it fails against %s's code: in /job/work/%s, a throwaway clone, put everything the change touches except its tests back as %s has it (check out %s's version of each path, deleting those %s lacks), run the test as the head has it (a build failure counts as failing), then restore the head ("git checkout HEAD -- ." and remove what it adds back). A test that is missing, or that passes against %s's code, is a "[blocking]" finding. Three cases that rollback cannot judge are checked another way, and are findings only when the test does not exercise its claim: a test whose subject the change keeps in test files (the rollback leaves them in place), and a claim met by a test the change does not touch (it passed against %s already), both checked by reading; and a test of behaviour the change keeps, where the plan or the report says the change removes code, restructures it or changes its cost but not its results (such a test passes against %s by design), checked by breaking the live path it exercises in the throwaway clone and seeing it fail, or by the worker's own report of that probe. A claim reported not in this slice is not checked here.`, s, j.Cfg.Base, cur.Repo, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base, j.Cfg.Base)
			para(`Then apply, for each such test, the probe its Test Plan claim names, run the test and restore the head, comparing the worker's report of it: a test that survives its named probe is a "[blocking]" finding of kind "claim". A test that survives a probe of your own is a "[non-blocking]" finding, unless the change that survives shows that the head itself is wrong, which is then a defect, judged as any other.`)
		}
		if len(j.Artifacts) > 0 {
			add(`The worker saved screenshots of pages the change alters, listed under "# Artifacts": open each from /job/io/artifacts/ (the URL beside it is the pull request's copy, and "not attached" means the forge refused the file) and judge what the page shows against what the plan and the slice claim, as you judge the diff. A page that does not show what is claimed is a finding, naming the screenshot and the file and line that render what it shows. If you cannot open an image, say so in the review and judge that page from the diff alone.`)
		}
		switch {
		case r > 1 && prev == cur.HeadSHA && !j.answeredSince(prev):
			add(`This is round %d, on the head you reviewed before (%s): the maintainer commented in the thread, with no commit since. First answer the maintainer's comment; then say, for each blocking finding of your previous review, whether it still stands in its light, and raise a new blocking finding only where the comment shows one.`, r, prev)
		case r > 1 && prev == cur.HeadSHA:
			add(`This is round %d, on the head you reviewed before (%s): the worker answered in the thread with "NO CHANGE:" and no commit. First say, for each blocking finding of your previous review, whether the answer resolves it with a reason you accept; raise no new blocking finding, since the code has not changed.`, r, prev)
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
	if !j.Deadline.IsZero() {
		end += " " + fmt.Sprintf(deadlineSentence, stamp(j.Deadline), j.Cfg.Timeout)
	}
	return strings.Join(append(t, end, last), "\n\n")
}

// deadlineSentence ends every Task text's closing paragraph (section
// 7.10, rule 5): the run's deadline and its timeout, and the file Daisy
// writes at 80% of it.
const deadlineSentence = `Your run is stopped at %s (%s after its start). If /job/io/deadline appears, 80%% of that time is used: write your output now, or, if the work cannot finish in the time left, write what you have and propose how to split it.`

// commented: the issue has a counted comment (section 6).
func (j *Job) commented() bool {
	for _, c := range j.Issue.Comments {
		if counted(Entry{c.Author, c.Body, c.Created}, j.SwarmLogin, j.Cfg.Trusted) {
			return true
		}
	}
	return false
}

// protected is the Task text's list: the run's repository's protected
// paths for the worker, each declared repository's as `<name>: <paths>`
// for the architect.
func (j *Job) protected() string {
	if j.Role == "worker" {
		return strings.Join(protectedPaths(j.repo(j.RunRepo())), ", ")
	}
	var parts []string
	for _, r := range j.Cfg.Repos {
		if ps := protectedPaths(r); len(ps) > 0 {
			parts = append(parts, r.Name+": "+strings.Join(ps, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

// protectedPaths is a repository's protected paths as the Task texts name
// them (7.4): its protected patterns, then its files protected by
// default (defaultProtected), each unless a pattern already matches it.
func protectedPaths(r declare.Repo) []string {
	out := slices.Clone(r.Protected)
	for _, p := range defaultProtected(r) {
		if len(Protected([]string{p}, r.Protected)) == 0 && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// previousFindings is where a delta review's Task text sends the
// reviewer for the previous round's findings (section 7.4; Step E2): the
// slice's latest round's file in the job's workspace, or, with none
// there, the previous round's reviews the prompt shows.
func (j *Job) previousFindings() string {
	if j.mem.lastRound == 0 {
		return `under "# Pull request"`
	}
	return MemoryMount + "/" + path.Join("slices", itoa(j.mem.slice), "rounds", itoa(j.mem.lastRound)+".md")
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
