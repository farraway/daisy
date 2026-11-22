package reconcile

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/provider"
)

// Job is what one run's two prompt files are rendered from (section 7.3):
// the declaration, the run, and the forge and <state> as read in this
// pass. A batch run, the owner's or the QA's, is issue 0 with
// Batch set.
type Job struct {
	Cfg        declare.Config
	Host       string // <forge-host>
	SwarmLogin string
	N, K       int
	Role       string
	Label      string // the stage the run is launched on
	Time       time.Time
	Persona    string        // prompts/<role>.md as read
	RunsOn     declare.Entry // the persona or the fallback entry the run launches on (section 7.10); unset, the persona
	Parts      []PromptPart  // the prompt parts swarm.json adds to the role's prompt, in its order (section 7.3)

	Issue forge.Issue  // with its comments
	Pulls []forge.Pull // Pulls(n), by slice
	Known []string     // the repositories the swarm holds a mirror of (SliceRepos)
	Pull  *PullThread  // the current pull request's thread, when it is open
	Check string       // the # Check section's text for the current head
	Batch *Batch       // a batch run's, the owner's or the QA's
	Notes []Note       // the research notes in the run's copy, with `notes`
	Repos []RepoNote   // the swarm's notes about each repository the run works in (section 4), in a swarm repository's clone

	Praxis  string // PraxisFile in the installed root, as read, for the roles that readsPraxis
	Testing string // TestingFile there, as read

	Resources []Listed   // each resource and reference path, as # Resources lists them (section 7.3)
	Artifacts []Artifact // the screenshots of the worker run that pushed the current head, which a reviewer's prompt lists (section 7.3)
	Outputs   Outputs    // <state>, where the job's workspace and its runs are read
	planText  string     // the accepted plan as readPlan read it from the job's workspace (Outputs.JobPlan)
	planned   bool       // readPlan found a plan
	textErr   error      // the first read of a member text Render showed (shownText) that failed

	Rulings  []string // the architect's on plan-1-accepted: the rulings changed since the plan, each `<name>:<path>` (section 6)
	Replan   *Entry   // the architect's on plan-1-accepted: the maintainer's re-plan request since the plan (section 6, The re-plan)
	Question bool     // the architect's or the owner's run answers the issue's question (swarm:question, section 6)
	Start    Start    // a worker's: where its clone starts and why (section 7.8, The start)
	Branches int      // the lines of the run's branches.md, for the roles that readsBranches (section 7.8, The open branches)
	mem      memory   // what # Job points to in the job's workspace, as readMemory read it
	blocked  bool     // the launch blocked the issue on a file of the job's it cannot use, so the pair's second reviewer is not launched

}

// PromptPart is a prompt_parts file as read for a run: its path in the
// swarm repository's clone and its text.
type PromptPart struct{ Path, Text string }

// PullThread is the current pull request's comments, reviews and line
// comments, as the forge returns them.
type PullThread struct {
	Comments []forge.Comment
	Reviews  []forge.Review
	Lines    []forge.ReviewComment
}

// Batch is a batch prompt's material (section 7.3): the briefs under the
// resources (the active ones are shown; the QA's has none), the
// open and the recently closed swarm issues, and how many jobs the run
// may propose.
type Batch struct {
	Briefs  []brief.Brief
	Open    []forge.Issue
	Closed  []Closed
	Propose int
}

// Closed is a recently closed swarm issue with its merged pull requests
// and its newest counted comment that is not the swarm's ("" for none).
type Closed struct {
	Issue  forge.Issue
	Merged int
	Last   string
}

var owner = func(c declare.Config) string { o, _, _ := strings.Cut(c.Forge.Repo, "/"); return o }

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// endStamp is a result's end line's time (section 2), to the nanosecond,
// so that runs ended within one second are ordered as they ended
// (faultPause).
func endStamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// Render is the run's system.md and prompt.md. Each member text the
// prompt shows is read from the job's workspace (shownText); a read that
// failed is left in j for the launch (TextErr). system.md writes the
// blocks every run of the role shares first and the run's own header
// and # Task last, so that runs of one role share a prefix a provider's
// prompt cache keeps (section 7.3), the role's # Output contract right
// after # Persona, before the prompt parts (decision 167 as amended).
func Render(j *Job) (system, prompt string) {
	j.textErr = nil
	head := fmt.Sprintf("%s#%d %s run %d, %s -->", j.Cfg.Forge.Repo, j.N, j.Role, j.K, stamp(j.Time))
	prompt = j.prompt(head)
	sys := []string{"# Persona", Shift(trim(j.Persona), 1)}
	if out := Output(j.Role); out != "" {
		sys = append(sys, "# Output", out)
	}
	for _, p := range j.Parts {
		sys = append(sys, "# Prompt part: "+p.Path, Shift(trim(p.Text), 1))
	}
	if readsPraxis(j.Role) {
		for _, b := range [][2]string{{"# Praxis", j.Praxis}, {"# Testing", skillBody(j.Testing)}} {
			if text := trim(b[1]); strings.TrimSpace(text) != "" {
				sys = append(sys, b[0], Shift(text, 1))
			}
		}
	}
	for _, n := range j.Repos {
		sys = append(sys, "# Repository notes: "+n.Name, Shift(trim(n.Text), 1))
	}
	return doc(append(sys, "# Resources", j.resources(), "<!-- daisy-swarm system: "+head, "# Task", j.task())...), prompt
}

// prompt is the run's prompt.md.
func (j *Job) prompt(head string) string {
	blocks := []string{"<!-- daisy-swarm prompt: " + head, j.repository()}
	if j.Cfg.Notes && writesNotes(j.Role) {
		blocks = append(blocks, "# Research notes", j.notes())
	}
	if j.Batch != nil {
		return doc(append(blocks, j.Batch.sections(j)...)...)
	}
	blocks = append(blocks, j.issue(), j.jobSection())
	cur, open := j.current()
	reviewer := j.Role == "adversary" || j.Role == "sympathetic"
	switch {
	case j.Role == "owner" || j.Role == "architect":
		blocks = append(blocks, "# Thread", j.thread())
		switch {
		case j.Role == "architect" && open:
			blocks = append(blocks, j.earlier(), j.pullRequest(cur), "# Check", j.Check)
		case j.Role == "architect" && j.Label == forge.PlanAccepted:
			blocks = append(blocks, j.earlier())
		}
	default:
		if j.Role == "worker" && j.Label == forge.PlanAccepted {
			blocks = append(blocks, "# Direction", j.direction())
		}
		blocks = append(blocks, j.questions(), j.earlier())
		if open || j.Label == forge.ChangesRequested || reviewer {
			blocks = append(blocks, j.pullRequest(cur), "# Check", j.Check)
		}
		if reviewer && len(j.Artifacts) > 0 {
			blocks = append(blocks, "# Artifacts", j.artifactsSection())
		}
	}
	return doc(blocks...)
}

// PraxisFile and TestingFile are the installed root's praxis and testing
// skill, which the architect's and the worker's system.md carry (section
// 7.3).
const (
	PraxisFile  = "prompts/praxis.md"
	TestingFile = "skills/testing/SKILL.md"
)

// readsPraxis is whether role's system.md carries praxis and the testing
// skill: the roles that plan and build, the architect and the worker.
func readsPraxis(role string) bool { return role == "architect" || role == "worker" }

// praxis is PraxisFile and TestingFile read in the installed root.
func (s *Swarm) praxis() (praxis, testing string, err error) {
	var text [2]string
	for i, rel := range []string{PraxisFile, TestingFile} {
		b, err := s.O.FS().ReadFile(filepath.Join(s.Root, rel))
		if err != nil {
			return "", "", err
		}
		text[i] = string(b)
	}
	return text[0], text[1], nil
}

// persona is role's prompt, prompts/<role>.md, and the prompt parts
// swarm.json adds to it, in its order, each read in the swarm
// repository's clone (section 7.3).
func (s *Swarm) persona(role string) (string, []PromptPart, error) {
	fs := s.O.FS()
	b, err := fs.ReadFile(declare.PromptFile(s.Place.Root, role))
	if err != nil {
		return "", nil, err
	}
	var parts []PromptPart
	for _, p := range s.Cfg.PromptParts {
		if !slices.Contains(p.Roles, role) {
			continue
		}
		text, err := fs.ReadFile(filepath.Join(s.Place.Root, p.Path))
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, PromptPart{Path: p.Path, Text: string(text)})
	}
	return string(b), parts, nil
}

// skillBody is a SKILL.md without its frontmatter, the lines from a first
// line "---" through the next "---" line, and the blank lines after it;
// the text as it is when it has none.
func skillBody(text string) string {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return text
	}
	if _, body, ok := strings.Cut(rest, "\n---\n"); ok {
		return strings.TrimLeft(body, "\n")
	}
	return text
}

func doc(blocks ...string) string {
	var kept []string
	for _, b := range blocks {
		if b = trim(b); b != "" {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "\n\n") + "\n"
}

func trim(s string) string { return strings.Trim(s, "\n") }

// swarmText is a swarm entry's text without its two marker lines and the
// blank line after them.
func swarmText(body string) string {
	_, rest, _ := strings.Cut(body, "\n")
	if l, after, ok := strings.Cut(rest, "\n"); ok && strings.HasPrefix(l, "**") {
		rest = after
	} else if strings.HasPrefix(rest, "**") && !ok {
		rest = ""
	}
	return strings.TrimPrefix(strings.TrimPrefix(rest, "\r"), "\n")
}

// current is the issue's current pull request (the largest slice) and
// whether it is open.
func (j *Job) current() (forge.Pull, bool) {
	if len(j.Pulls) == 0 {
		return forge.Pull{}, false
	}
	p := j.Pulls[len(j.Pulls)-1]
	return p, p.Open
}

func sliceOf(p forge.Pull, n int) int {
	s, _ := strconv.Atoi(strings.TrimPrefix(p.Head, fmt.Sprintf("swarm/%d-", n)))
	return s
}

// Plan is the accepted plan (section 7.3): the newest counted issue
// comment marked role=architect whose text begins none of BLOCKED:,
// QUESTION:, ANSWER: (onThePlan) and STANDS: (a confirmation that the
// plan stands, section 7.5).
func (j *Job) Plan() (Entry, Mark, bool) {
	return j.newestArchitect(func(text string) bool { return !strings.HasPrefix(text, stands) })
}

// PlanText is the accepted plan's text as Daisy reads its slices, their
// repositories and their count (section 7.8, Daisy's own reads): the
// job's plan.md, as readPlan read it; ok is false for no plan.
func (j *Job) PlanText() (string, bool) {
	return j.planText, j.planned
}

// readPlan reads the accepted plan from the job's workspace through
// Outputs (JobPlan), for PlanText; a file of the job's that cannot be
// read is the error, its own (unusableError).
func (j *Job) readPlan() error {
	var err error
	j.planText, j.planned, _, err = j.Outputs.JobPlan(j.Issue, j.SwarmLogin)
	return err
}

// newestArchitect is the newest of the architect's issue comments that
// are its word on the plan (onThePlan) and that keep takes.
func (j *Job) newestArchitect(keep func(text string) bool) (Entry, Mark, bool) {
	var e Entry
	var m Mark
	found := false
	for _, c := range j.Issue.Comments {
		ce := Entry{c.Author, c.Body, c.Created}
		text := swarmText(c.Body)
		if mk, ok := Swarms(ce, j.SwarmLogin); ok && mk.Role == "architect" && onThePlan(text) && keep(text) {
			if !found || !c.Created.Before(e.Created) {
				e, m, found = ce, mk, true
			}
		}
	}
	return e, m, found
}

// WorkerSlice is the slice the worker builds and its repository (section
// 5): the open current pull request's, else the next slice, in the
// repository the accepted plan names for it (the first repository's with
// no plan); bad is the slices that are not each in one declared
// repository.
func (j *Job) WorkerSlice() (s int, repo string, bad []int) {
	names := j.names()
	if cur, open := j.current(); open {
		s = sliceOf(cur, j.N)
		if plan, ok := j.PlanText(); ok {
			if repos, _ := SliceRepos(plan, names, j.Known); s <= len(repos) && repos[s-1] != cur.Repo {
				return s, cur.Repo, []int{s}
			}
		}
		return s, cur.Repo, nil
	} else if len(j.Pulls) > 0 {
		s = sliceOf(j.Pulls[len(j.Pulls)-1], j.N) + 1
	} else {
		s = 1
	}
	plan, ok := j.PlanText()
	if !ok {
		return s, names[0], nil
	}
	repos, bad := SliceRepos(plan, names, j.Known)
	if len(bad) > 0 {
		return s, "", bad
	}
	if s > len(repos) {
		return s, "", []int{s}
	}
	return s, repos[s-1], nil
}

func (j *Job) names() []string {
	var out []string
	for _, r := range j.Cfg.Repos {
		out = append(out, r.Name)
	}
	return out
}

func (j *Job) repo(name string) declare.Repo {
	for _, r := range j.Cfg.Repos {
		if r.Name == name {
			return r
		}
	}
	return declare.Repo{Name: name}
}

// RunRepo is the repository a worker or reviewer run works in.
func (j *Job) RunRepo() string {
	if j.Role == "worker" {
		_, r, _ := j.WorkerSlice()
		return r
	}
	cur, _ := j.current()
	return cur.Repo
}

// workRepo is the repository whose checkout is the member's working
// directory (section 7.2's <W>): the run's, the first declared for a role
// that clonesEvery repository; "" when none is declared.
func (j *Job) workRepo() string {
	if !clonesEvery(j.Role) {
		return j.RunRepo()
	}
	if names := j.names(); len(names) > 0 {
		return names[0]
	}
	return ""
}

func (j *Job) repository() string {
	lines := []string{"# Repository", "", "- Issues: " + j.Cfg.Forge.Repo + " on " + j.Host}
	clone := func(path, name, branch string) {
		r := j.repo(name)
		check := "no check declared"
		if len(r.Check) > 0 {
			check = "check " + declare.ShellLine(r.Check)
		}
		lines = append(lines, fmt.Sprintf("- Clone %s: %s/%s, branch %s, base %s, %s", path, owner(j.Cfg), name, branch, j.Cfg.Base, check))
	}
	switch {
	case clonesEvery(j.Role):
		for _, n := range j.names() {
			clone(WorkMount+"/"+n, n, j.Cfg.Base)
		}
	case j.Role == "worker":
		s, r, _ := j.WorkerSlice()
		clone(WorkMount+"/"+r, r, fmt.Sprintf("swarm/%d-%d", j.N, s))
	default:
		cur, _ := j.current()
		clone(WorkMount+"/"+cur.Repo, cur.Repo, cur.Head)
	}
	if repo := j.workRepo(); repo != "" && Workdir(j.provider(), repo) == JobMount {
		lines = append(lines, "- Working directory: "+JobMount+", holding every clone under "+WorkMount+"; the repository is "+WorkMount+"/"+repo)
	}
	if j.Batch == nil {
		lines = append(lines, "- Job workspace (read-only): "+MemoryMount)
	}
	switch {
	case j.Cfg.Notes && writesNotes(j.Role):
		lines = append(lines, "- Research notes: "+NotesMount+", "+notesHome(j.Cfg)+", pushed by Daisy after the run")
	case j.Cfg.Notes:
		lines = append(lines, "- Research notes (read-only): "+NotesMount)
	}
	return strings.Join(lines, "\n")
}

// resources is system.md's # Resources (section 7.3): a line per
// resource, its mount, its index page and its note, or None.
func (j *Job) resources() string {
	var lines []string
	for _, l := range j.Resources {
		line := "- " + l.Mount
		if l.Index != "" {
			line += " (index: " + l.Index + ")"
		}
		if l.Note != "" {
			line += " — " + l.Note
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "None."
	}
	return "Mounted read-only:\n\n" + strings.Join(lines, "\n")
}

// The run's folder (sections 7.2 and 7.8): <run>/job mounted at JobMount,
// holding only the mount points of its children, every one outside each
// checkout, so that a CLI confined to /job reaches all the run is given
// (section 9): the checkouts at WorkMount/<name>, the run's io at IOMount,
// the notes at NotesMount, the resources at ResourcesMount and, on an
// issue, the job's workspace at MemoryMount.
const (
	JobMount  = "/job"
	WorkMount = JobMount + "/work"
	IOMount   = JobMount + "/io"
)

// ResourcesMount is the directory under which each resource is mounted.
const ResourcesMount = JobMount + "/resources"

// MemoryMount is where a run on an issue reads its job's workspace,
// <state>/jobs/<n>, read-only (section 7.8, The mount).
const MemoryMount = JobMount + "/memory"

// clonesEvery: role works over every declared repository, each cloned at
// base under /job/work/<name> (section 7.1, step 3): the owner, the
// architect and the QA.
func clonesEvery(role string) bool {
	return role == "owner" || role == "architect" || role == declare.QARole
}

// Workdir is the member's working directory, section 7.2's <W>: its
// checkout of repo, /job/work/<repo> (repo the run's repository, the
// first declared for a role that clonesEvery repository), unless its
// provider, prov, the entry's it launches on, is Confined to its working
// directory (section 9): then /job, which holds every checkout and all
// else the run is given.
func Workdir(prov, repo string) string {
	if provider.Providers[prov].Confined {
		return JobMount
	}
	return WorkMount + "/" + repo
}

// provider is the provider the run launches on: its RunsOn entry's, else
// the role's persona's.
func (j *Job) provider() string {
	if j.RunsOn.Provider != "" {
		return j.RunsOn.Provider
	}
	return j.Cfg.Personas[j.Role].Provider
}

func (j *Job) issue() string {
	is := j.Issue
	opened := fmt.Sprintf("Opened by %s at %s.", is.Author, stamp(is.Created))
	e := bodyEntry(is)
	body := e.Body
	if m, ok := Swarms(e, j.SwarmLogin); ok {
		opened = fmt.Sprintf("Opened by %s, run %d, at %s.", m.Role, m.Run, stamp(is.Created))
		body = j.proposal(m, swarmText(body))
	} else if !j.trusted(is.Author) {
		// The body is its opener's entry (section 6), and it can be edited
		// after the maintainer labelled the issue.
		opened = fmt.Sprintf("Opened by %s at %s (not trusted; content omitted).", is.Author, stamp(is.Created))
		body = ""
	}
	return doc(fmt.Sprintf("# Issue #%d: %s", is.Number, is.Title), opened+" Labels: "+strings.Join(is.Labels, ", ")+".", Shift(trim(body), 1))
}

// entry is one thread entry at heading level, with suffix after its time.
func (j *Job) entry(level int, e Entry, suffix string) string {
	h := strings.Repeat("#", level)
	if m, ok := Swarms(e, j.SwarmLogin); ok {
		return doc(fmt.Sprintf("%s %s, run %d, at %s%s", h, m.Role, m.Run, stamp(e.Created), suffix), Shift(trim(j.shownText(m, swarmText(e.Body))), level))
	}
	if !j.trusted(e.Author) {
		return fmt.Sprintf("%s %s at %s%s (not trusted; content omitted)", h, e.Author, stamp(e.Created), suffix)
	}
	return doc(fmt.Sprintf("%s %s at %s%s", h, e.Author, stamp(e.Created), suffix), Shift(trim(e.Body), level))
}

func (j *Job) trusted(login string) bool {
	return slices.ContainsFunc(j.Cfg.Trusted, func(l string) bool { return strings.EqualFold(l, login) })
}

func orNone(blocks []string) string {
	if len(blocks) == 0 {
		return "None."
	}
	return strings.Join(blocks, "\n\n")
}

// thread is the issue thread's comments but the gate notes and the auto
// notes, which are the maintainer's, compacted as the pull-request thread
// is (section 7.3): an entry not the swarm's whole whatever its age; of
// the swarm's, the accepted plan its heading and the pointer to the job's
// plan.md (section 7.8, The prompt), each role's newest other entry
// whole, and every older one its heading alone, so that a prompt stops
// growing with the plan's revisions.
func (j *Job) thread() string {
	newest := map[string]Entry{}
	runs := map[string]int{}
	for _, c := range j.Issue.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		if m, ok := Swarms(e, j.SwarmLogin); ok && m.Kind == "" && !e.Created.Before(newest[m.Role].Created) {
			newest[m.Role], runs[m.Role] = e, m.Run
		}
	}
	plan, _, planned := j.Plan()
	var out []string
	for _, c := range j.Issue.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		m, ok := Swarms(e, j.SwarmLogin)
		switch {
		case ok && m.Kind != "":
			continue
		case planned && same(e, plan) && !j.mem.noPlanFile:
			out = append(out, fmt.Sprintf("## %s, run %d, at %s (the accepted plan: whole in %s/plan.md)", m.Role, m.Run, stamp(e.Created), MemoryMount))
		case ok && !same(e, newest[m.Role]):
			out = append(out, fmt.Sprintf("## %s, run %d, at %s (superseded by run %d; content omitted)", m.Role, m.Run, stamp(e.Created), runs[m.Role]))
		default:
			out = append(out, trim(j.entry(2, e, "")))
		}
	}
	return orNone(out)
}

// outputRun is the run directory's name of the run whose entry mark is
// on issue n.
func outputRun(n int, m Mark) string { return fmt.Sprintf("%d-%s-%d", n, m.Role, m.Run) }

// cutRun is the run whose output the text of the swarm entry marked m on
// issue n holds cut (section 7.5), and where its note's line begins and
// ends: the last line of the text beginning the note Daisy ends a cut
// text with, naming m's own run `<n>-<role>-<k>`, which is that note
// whole (cutTail); "" and -1 for any other text, so that a note a text
// quotes, or one naming another run, is never read as the cut.
func cutRun(n int, m Mark, text string) (run string, at, end int) {
	if m.Kind != "" || n == 0 {
		return "", -1, -1
	}
	run = outputRun(n, m)
	if at, end = cutOf(run, text); at < 0 {
		return "", -1, -1
	}
	return run, at, end
}

// cutOf is where the note that ends text, cut by Daisy as run's output
// (section 7.5), begins and ends: its last line, that note whole; -1
// when text ends in no such note.
func cutOf(run, text string) (at, end int) {
	begin := "\n" + truncated + "the whole output is in runs/" + run
	at = strings.LastIndex(text, begin)
	if at < 0 {
		return -1, -1
	}
	line, _, _ := strings.Cut(text[at+1:], "\n")
	if !cutTail.MatchString(line[len(begin)-1:]) {
		return -1, -1
	}
	return at + 1, at + 1 + len(line)
}

// proposal is the issue body of the swarm's marked m, posted its text as
// posted: when Daisy cut it on the forge (section 7.5), the proposal
// whole, read from its batch run's out.md (Outputs.Text), the proposal
// there whose body begins as the post's text before the note does; else
// as posted. A read that fails is kept for the launch (TextErr), the
// body shown as posted.
func (j *Job) proposal(m Mark, posted string) string {
	run := outputRun(0, m)
	at, _ := cutOf(run, posted)
	if at < 0 || m.Kind != "" || j.Outputs.FS == nil {
		return posted
	}
	out, ok, err := j.Outputs.Text(run)
	if err != nil && j.textErr == nil {
		j.textErr = fmt.Errorf("output %s: %w", run, err)
	}
	if !ok {
		return posted
	}
	_, _, _, ps := Proposals(out)
	for _, p := range ps {
		if strings.HasPrefix(p.Body, posted[:at-1]) {
			return p.Body
		}
	}
	return posted
}

// cutTail is what follows the run in a cut's note to the end of its line
// (long.note): the file's link, the continuations' count, or, in a note
// posted before Step 38d of the plan remediating the 2026-11-15 design
// review, nothing.
var cutTail = regexp.MustCompile(`^(?: and attached as \[[^\]\n]*\]\([^)\s]*\)|; the rest follows in [1-9][0-9]* comments?)?\]$`)

// shownText is the text of the swarm entry marked m, posted its text as
// posted, as a prompt shows it (section 7.8, The prompt): its member's
// text whole, read from the job's workspace as Daisy reads a text past
// its first lines (Job.text), so that no cut text reaches a prompt; with
// no workspace to read (no Outputs), as posted. A read that fails is kept for the launch, which
// blocks the issue on it (TextErr), and the text shown as posted.
func (j *Job) shownText(m Mark, posted string) string {
	if j.Outputs.FS == nil {
		return posted
	}
	text, err := j.text(m, posted)
	if err != nil {
		if j.textErr == nil {
			j.textErr = err
		}
		return posted
	}
	return text
}

// TextErr is the first read of a member text the prompt Render made last
// shows that failed, nil when none did.
func (j *Job) TextErr() error { return j.textErr }

func same(a, b Entry) bool {
	return a.Author == b.Author && a.Body == b.Body && a.Created.Equal(b.Created)
}

// direction is the maintainer's comments since the plan's newest basis
// (section 7.8, The prompt): the accepted plan, or the architect's newest
// STANDS: after it (section 6, The re-plan), so that a re-plan request
// the architect answered STANDS: is no longer shown as direction; every
// one when there is no plan.
func (j *Job) direction() string {
	basis, _, hasPlan := j.Plan()
	if st, _, ok := j.newestArchitect(func(text string) bool { return strings.HasPrefix(text, stands) }); hasPlan && ok && st.Created.After(basis.Created) {
		basis = st
	}
	var out []string
	for _, c := range j.Issue.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		if _, swarm := Swarms(e, j.SwarmLogin); swarm || !j.trusted(c.Author) || (hasPlan && !c.Created.After(basis.Created)) {
			continue
		}
		out = append(out, trim(j.entry(2, e, "")))
	}
	return orNone(out)
}

// earlier is the issue's pull requests before the slice the run works
// on: every one but the current while it is open, every one when the
// worker starts the next slice.
func (j *Job) earlier() string {
	var out []string
	_, open := j.current()
	for i, p := range j.Pulls {
		if open && i == len(j.Pulls)-1 {
			break
		}
		state := "closed"
		switch {
		case p.Merged:
			state = "merged " + p.MergeSHA
		case p.Open:
			state = "open"
		}
		out = append(out, fmt.Sprintf("- %s#%d %s %s: %s", p.Repo, p.Number, p.Head, state, p.Title))
	}
	if len(out) == 0 {
		return "# Earlier pull requests\n\nNone."
	}
	return "# Earlier pull requests\n\n" + strings.Join(out, "\n")
}

// verdictOf is a swarm review's verdict, read from its text's first line.
func verdictOf(text string) string {
	first, _, _ := strings.Cut(text, "\n")
	switch strings.TrimRight(first, "\r ") {
	case "VERDICT: approve":
		return "approve"
	case "VERDICT: request-changes":
		return "request-changes"
	}
	return "comment"
}

func stateName(v forge.Verdict) string {
	switch v {
	case forge.Approve:
		return "approve"
	case forge.RequestChanges:
		return "request-changes"
	}
	return "comment"
}

// pullEntries is the pull request's thread as entries (section 6): its
// body, by the login that opened it, then its comments, reviews and line
// comments.
// roleThread is the thread the stage's role reads (section 6, rule 3's
// table), j.Pull read: in the review stages the current pull request p's
// comments alone, where its reopenings are (Reopenings), as the
// collection and the rounds read them; in review-3-changes-requested
// p's whole thread (pullEntries); at any other stage the issue thread.
func (j *Job) roleThread(p forge.Pull) []Entry {
	switch {
	case reviewing(j.Label):
		return pullComments(j.Pull.Comments)
	case onPull(j.Label):
		return j.pullEntries(p)
	}
	return IssueThread(j.Issue)
}

func (j *Job) pullEntries(p forge.Pull) []Entry {
	out := []Entry{{p.Author, p.Body, p.Created}}
	if j.Pull == nil {
		return out
	}
	for _, c := range j.Pull.Comments {
		out = append(out, Entry{c.Author, c.Body, c.Created})
	}
	for _, r := range j.Pull.Reviews {
		out = append(out, Entry{r.Author, r.Body, r.Submitted})
	}
	for _, l := range j.Pull.Lines {
		out = append(out, Entry{l.Author, l.Body, l.Created})
	}
	return out
}

// How a round's reviews are shown (section 7.3): left out, as a line under
// "## Earlier rounds", as their headings with their blocking findings, or
// in full.
const (
	roundHidden = iota
	roundLine
	roundFindings
	roundFull
)

// shown is how each of the pull request's rounds is shown to the run's
// role: a reviewer sees the previous round's findings and every earlier
// round as a line, never the current round (the other reviewer's review
// of the head as it stands); the worker the latest round's findings, the
// reviews whole in the job's workspace (section 7.8, The prompt), and the
// rest as lines; the architect every round's findings.
func (j *Job) shown(rounds []round, current roundKey) []int {
	modes := make([]int, len(rounds))
	switch j.Role {
	case "adversary", "sympathetic":
		prev := -1
		for i, rd := range rounds {
			if rd.roundKey == current {
				modes[i] = roundHidden
				continue
			}
			modes[i], prev = roundLine, i
		}
		if prev >= 0 {
			modes[prev] = roundFindings
		}
	case "worker":
		for i := range rounds {
			modes[i] = roundLine
		}
		if len(rounds) > 0 {
			modes[len(rounds)-1] = roundFindings
		}
	default:
		for i := range rounds {
			modes[i] = roundFindings
		}
	}
	return modes
}

// roundLines is the "## Earlier rounds" lines: each round shown as a
// line, named by the head it reviewed rather than numbered, since the
// Task text's round number counts other rounds (section 7.3), with its
// blocking findings counted and what followed it: a revision, the
// worker's answer without a commit (one of answers, Answers) or, with
// none between, the maintainer's comment. A review is counted whole, as
// shownText reads it.
func (j *Job) roundLines(rounds []round, modes []int, answers []time.Time) []string {
	var out []string
	for i, rd := range rounds {
		if modes[i] != roundLine {
			continue
		}
		b := 0
		for _, r := range rd.reviews {
			text := r.Body
			if m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, j.SwarmLogin); ok {
				text = j.shownText(m, swarmText(r.Body))
			}
			b += len(blockingLines(text))
		}
		// A round shown as a line is never the last (shown), so a round
		// follows it.
		next := "revised in " + rounds[i+1].commit
		if rounds[i+1].commit == rd.commit {
			last, first := rd.reviews[len(rd.reviews)-1].Submitted, rounds[i+1].reviews[0].Submitted
			next = "reviewed again after the maintainer's comment"
			if slices.ContainsFunc(answers, func(a time.Time) bool { return a.After(last) && a.Before(first) }) {
				next = "answered without a commit"
			}
		}
		out = append(out, fmt.Sprintf("- round on %s: %d blocking, %s", rd.commit, b, next))
	}
	return out
}

// pullRequest is the current pull request and its thread, compacted
// (section 7.3): the entries not the swarm's always, the swarm's from the
// worker's newest report on, and the swarm reviewers' reviews as shown
// says.
func (j *Job) pullRequest(p forge.Pull) string {
	type item struct {
		at   time.Time
		text string
	}
	var items []item
	var lines []string
	if j.Pull != nil {
		answers := Reopenings(pullComments(j.Pull.Comments), j.SwarmLogin, j.Cfg.Trusted)
		rounds := roundsOf(j.Pull.Reviews, answers, j.SwarmLogin)
		modes := j.shown(rounds, roundKey{p.HeadSHA, len(answers)})
		lines = j.roundLines(rounds, modes, Answers(pullComments(j.Pull.Comments), j.SwarmLogin))
		mode := map[roundKey]int{}
		for i, rd := range rounds {
			mode[rd.roundKey] = modes[i]
		}
		report, _, _ := j.newestReport(p)
		// A swarm comment older than the newest report is the history the
		// round lines summarise; a command's answer is the maintainer's
		// (section 6); the worker's reports are the worker's and the
		// reviewers' to read from the job's workspace, by # Job's Report
		// line (section 7.8, The prompt).
		omitted := func(e Entry) bool {
			m, swarm := Swarms(e, j.SwarmLogin)
			return swarm && (e.Created.Before(report) || m.Kind != "" || m.Role == "worker" && j.Role != "architect")
		}
		for _, c := range j.Pull.Comments {
			if e := (Entry{c.Author, c.Body, c.Created}); !omitted(e) {
				items = append(items, item{c.Created, j.entry(3, e, "")})
			}
		}
		for _, r := range j.Pull.Reviews {
			e := Entry{r.Author, r.Body, r.Submitted}
			m, swarm := Swarms(e, j.SwarmLogin)
			verdict := stateName(r.State)
			if swarm {
				verdict = verdictOf(swarmText(r.Body))
			}
			suffix := fmt.Sprintf(", on %s: %s", r.Commit, verdict)
			md := roundFull // a review not the swarm's reviewers' is in no round
			if swarm && (m.Role == "adversary" || m.Role == "sympathetic") {
				md = mode[keyOf(r, answers)]
			}
			switch md {
			case roundFindings:
				// A review is read whole (shownText), so the findings
				// past a cut on the forge are listed too.
				findings := blockingLines(j.shownText(m, swarmText(r.Body)))
				items = append(items, item{r.Submitted, doc(fmt.Sprintf("### %s, run %d, at %s%s", m.Role, m.Run, stamp(r.Submitted), suffix), strings.Join(findings, "\n"))})
			case roundFull:
				items = append(items, item{r.Submitted, j.entry(3, e, suffix)})
			}
		}
		for _, l := range j.Pull.Lines {
			if e := (Entry{l.Author, l.Body, l.Created}); !omitted(e) {
				items = append(items, item{l.Created, j.entry(3, e, fmt.Sprintf(", on %s:%d at %s", l.Path, l.Line, l.Commit))})
			}
		}
	}
	slices.SortStableFunc(items, func(a, b item) int { return a.at.Compare(b.at) })
	var thread []string
	for _, it := range items {
		thread = append(thread, trim(it.text))
	}
	// The body is its opener's entry (section 6): the swarm's, marked,
	// the worker's report, which the worker and the reviewers read from
	// the job's workspace (# Job's Report); a trusted login's as written;
	// anyone else's omitted.
	body := p.Body
	if m, ok := Swarms(Entry{p.Author, p.Body, p.Created}, j.SwarmLogin); ok && j.Role != "architect" {
		body = ""
	} else if ok {
		body = j.shownText(m, swarmText(p.Body))
	} else if !j.trusted(p.Author) {
		body = fmt.Sprintf("Opened by %s, not trusted; content omitted.", p.Author)
	}
	earlier := ""
	if len(lines) > 0 {
		earlier = "## Earlier rounds\n\n" + strings.Join(lines, "\n")
	}
	return doc(fmt.Sprintf("# Pull request %s#%d: %s", p.Repo, p.Number, p.Title),
		fmt.Sprintf("Branch %s into %s; head %s.", p.Head, p.Base, p.HeadSHA), Shift(trim(body), 1), earlier, "## Thread", orNone(thread))
}

// newestReport is the worker's newest report on the pull request and its
// mark: the newest of its entries that are the swarm's and marked
// role=worker, its body (the description) or a comment (a revision's
// report, an answer without a commit, a BLOCKED:); ok is false, the time
// zero, when there is none.
func (j *Job) newestReport(p forge.Pull) (at time.Time, m Mark, ok bool) {
	entries := []Entry{{p.Author, p.Body, p.Created}}
	if j.Pull != nil {
		entries = append(entries, pullComments(j.Pull.Comments)...)
	}
	for _, e := range entries {
		if mk, swarm := Swarms(e, j.SwarmLogin); swarm && mk.Role == "worker" && !e.Created.Before(at) {
			at, m, ok = e.Created, mk, true
		}
	}
	return at, m, ok
}

func (b *Batch) sections(j *Job) []string {
	var briefs []string
	for _, br := range b.active() {
		briefs = append(briefs, doc(fmt.Sprintf("## %s (Status: active; mounted at %s/%s)", br.Path, ResourcesMount, br.Mount), Shift(trim(br.Text), 2)))
	}
	// An issue's line names its stage, not the flag; a closed issue's
	// its terminal, or the stage it was closed at.
	line := func(is forge.Issue) string {
		stage := ""
		for _, l := range is.Labels {
			if forge.IsStage(l) || l == forge.Done || l == forge.Rejected {
				stage = strings.TrimPrefix(l, "swarm:")
			}
		}
		s := fmt.Sprintf("- #%d [%s] %s", is.Number, stage, is.Title)
		if bl := BriefLine(is, j.SwarmLogin); bl != "" {
			s += "; " + bl
		}
		return s
	}
	var open, closed []string
	for _, is := range b.Open {
		open = append(open, line(is))
	}
	for _, c := range b.Closed {
		s := fmt.Sprintf("%s: closed %s; %d merged pull requests", line(c.Issue), c.Issue.Closed.UTC().Format("2006-01-02"), c.Merged)
		if c.Last != "" {
			s += "\n\n  > " + strings.ReplaceAll(trim(c.Last), "\n", "\n  > ")
		}
		closed = append(closed, s)
	}
	list := func(items []string) string {
		if len(items) == 0 {
			return "None."
		}
		return strings.Join(items, "\n")
	}
	issues := []string{"# Open swarm issues", list(open), "# Recently closed", list(closed)}
	if j.Role == declare.QARole {
		return issues // it reports failures, toward no brief
	}
	return append([]string{"# Briefs", orNone(briefs)}, issues...)
}

func (b *Batch) active() []brief.Brief {
	var out []brief.Brief
	for _, br := range b.Briefs {
		if br.Status == "active" {
			out = append(out, br)
		}
	}
	return out
}

// BriefLine is the `Brief: <path> — <milestone>` line an issue's text,
// after the marker lines, opens with; "" when it opens with none.
func BriefLine(is forge.Issue, swarmLogin string) string {
	e := bodyEntry(is)
	text := e.Body
	if _, ok := Swarms(e, swarmLogin); ok {
		text = swarmText(text)
	}
	first, _, _ := strings.Cut(strings.TrimLeft(text, "\n"), "\n")
	if first = strings.TrimRight(first, "\r "); strings.HasPrefix(first, "Brief: ") {
		return first
	}
	return ""
}

// BriefOf is the brief's path and the milestone is's Brief: line names;
// both "" without the line.
func BriefOf(is forge.Issue, swarmLogin string) (path, milestone string) {
	path, milestone, _ = strings.Cut(strings.TrimPrefix(BriefLine(is, swarmLogin), "Brief: "), " — ")
	return path, milestone
}
