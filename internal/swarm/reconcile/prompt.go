package reconcile

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// Job is what one run's two prompt files are rendered from (section 7.3):
// the declaration, the run, and the forge and <state> as read in this
// pass. The owner's batch run is issue 0 with Batch set.
type Job struct {
	Cfg        declare.Config
	Host       string // <forge-host>
	SwarmLogin string
	N, K       int
	Role       string
	Label      string // the stage the run is launched on
	Time       time.Time
	Persona    string // swarm/prompts/<role>.md as read

	Issue forge.Issue  // with its comments
	Pulls []forge.Pull // Pulls(n), by slice
	Pull  *PullThread  // the current pull request's thread, when it is open
	Check string       // the # Check section's text for the current head
	Batch *Batch       // the owner's batch
}

// PullThread is the current pull request's comments, reviews and line
// comments, as the forge returns them.
type PullThread struct {
	Comments []forge.Comment
	Reviews  []forge.Review
	Lines    []forge.ReviewComment
}

// Batch is the owner's batch prompt's material (section 7.3): the briefs
// under the resources (the active ones are shown), the open and the
// recently closed swarm issues, and how many jobs the pacing allows.
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

// Render is the run's system.md and prompt.md.
func Render(j *Job) (system, prompt string) {
	head := fmt.Sprintf("%s#%d %s run %d, %s -->", j.Cfg.Forge.Repo, j.N, j.Role, j.K, stamp(j.Time))
	system = doc("<!-- daisy-swarm system: "+head, "# Persona", Shift(trim(j.Persona), 1), "# Task", j.task())
	blocks := []string{"<!-- daisy-swarm prompt: " + head, j.repository()}
	if j.Batch != nil {
		return system, doc(append(blocks, j.Batch.sections(j)...)...)
	}
	blocks = append(blocks, j.issue())
	cur, open := j.current()
	reviewer := j.Role == "adversary" || j.Role == "sympathetic"
	switch {
	case j.Role == "owner" || j.Role == "architect":
		blocks = append(blocks, "# Thread", j.thread())
		if j.Role == "architect" && open {
			blocks = append(blocks, j.earlier(), j.pullRequest(cur), "# Check", j.Check)
		}
	default:
		blocks = append(blocks, "# Plan", j.plan())
		if j.Role == "worker" && j.Label == forge.PlanAccepted {
			blocks = append(blocks, "# Direction", j.direction())
		}
		blocks = append(blocks, j.earlier())
		if open || j.Label == forge.ChangesRequested || reviewer {
			blocks = append(blocks, j.pullRequest(cur), "# Check", j.Check)
		}
	}
	return system, doc(blocks...)
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
// comment marked role=architect whose text does not begin BLOCKED:.
func (j *Job) Plan() (Entry, Mark, bool) {
	var e Entry
	var m Mark
	found := false
	for _, c := range j.Issue.Comments {
		ce := Entry{c.Author, c.Body, c.Created}
		if mk, ok := Swarms(ce, j.SwarmLogin); ok && mk.Role == "architect" && !strings.HasPrefix(swarmText(c.Body), "BLOCKED:") {
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
		if e, _, ok := j.Plan(); ok {
			if repos, _ := SliceRepos(swarmText(e.Body), names); s <= len(repos) && repos[s-1] != cur.Repo {
				return s, cur.Repo, []int{s}
			}
		}
		return s, cur.Repo, nil
	} else if len(j.Pulls) > 0 {
		s = sliceOf(j.Pulls[len(j.Pulls)-1], j.N) + 1
	} else {
		s = 1
	}
	e, _, ok := j.Plan()
	if !ok {
		return s, names[0], nil
	}
	repos, bad := SliceRepos(swarmText(e.Body), names)
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

func (j *Job) repository() string {
	lines := []string{"# Repository", "", "- Issues: " + j.Cfg.Forge.Repo + " on " + j.Host}
	clone := func(path, name, branch string) {
		r := j.repo(name)
		check := "no check declared"
		if len(r.Check) > 0 {
			check = "check " + shellQuote(r.Check)
		}
		lines = append(lines, fmt.Sprintf("- Clone %s: %s/%s, branch %s, base %s, %s", path, owner(j.Cfg), name, branch, j.Cfg.Base, check))
	}
	switch j.Role {
	case "owner", "architect":
		for _, n := range j.names() {
			clone("/work/"+n, n, j.Cfg.Base)
		}
	case "worker":
		s, r, _ := j.WorkerSlice()
		clone("/work", r, fmt.Sprintf("swarm/%d-%d", j.N, s))
	default:
		cur, _ := j.current()
		clone("/work", cur.Repo, cur.Head)
	}
	res := "none"
	if len(j.Cfg.Resources) > 0 {
		res = strings.Join(ResourceMounts(j.Cfg.Resources), ", ")
	}
	return strings.Join(append(lines, "- Resources (read-only): "+res), "\n")
}

// ResourceMounts is where each resource is mounted: /resources/<path>.
func ResourceMounts(resources []string) []string {
	var out []string
	for _, r := range resources {
		out = append(out, "/resources/"+strings.Trim(strings.TrimPrefix(r, "./"), "/"))
	}
	return out
}

var safeWord = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func shellQuote(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		if safeWord.MatchString(a) {
			q[i] = a
		} else {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(q, " ")
}

func (j *Job) issue() string {
	is := j.Issue
	opened := fmt.Sprintf("Opened by %s at %s.", is.Author, stamp(is.Created))
	body := is.Body
	if m, ok := Swarms(Entry{is.Author, is.Body, is.Created}, j.SwarmLogin); ok {
		opened = fmt.Sprintf("Opened by %s, run %d, at %s.", m.Role, m.Run, stamp(is.Created))
		body = swarmText(body)
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
		return doc(fmt.Sprintf("%s %s, run %d, at %s%s", h, m.Role, m.Run, stamp(e.Created), suffix), Shift(trim(swarmText(e.Body)), level))
	}
	if !j.trusted(e.Author) {
		return fmt.Sprintf("%s %s at %s%s (not trusted; content omitted)", h, e.Author, stamp(e.Created), suffix)
	}
	return doc(fmt.Sprintf("%s %s at %s%s", h, e.Author, stamp(e.Created), suffix), Shift(trim(e.Body), level))
}

func (j *Job) trusted(login string) bool {
	return slices.ContainsFunc(j.Cfg.Trusted, func(l string) bool { return strings.EqualFold(l, login) })
}

func (j *Job) counted(e Entry) bool {
	_, swarm := Swarms(e, j.SwarmLogin)
	return swarm || j.trusted(e.Author)
}

func orNone(blocks []string) string {
	if len(blocks) == 0 {
		return "None."
	}
	return strings.Join(blocks, "\n\n")
}

func (j *Job) thread() string {
	var out []string
	for _, c := range j.Issue.Comments {
		out = append(out, trim(j.entry(2, Entry{c.Author, c.Body, c.Created}, "")))
	}
	return orNone(out)
}

func (j *Job) plan() string {
	e, m, ok := j.Plan()
	if !ok {
		return "None: there is no plan."
	}
	text := swarmText(e.Body)
	S := max(1, len(Slices(text)))
	s, repo := 0, ""
	if j.Role == "worker" {
		s, repo, _ = j.WorkerSlice()
	} else {
		cur, _ := j.current()
		s, repo = sliceOf(cur, j.N), cur.Repo
	}
	return doc(fmt.Sprintf("## architect, run %d, at %s: slice %d of %d, in %s", m.Run, stamp(e.Created), s, S, repo), Shift(trim(text), 2))
}

func (j *Job) direction() string {
	plan, _, hasPlan := j.Plan()
	var out []string
	for _, c := range j.Issue.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		if _, swarm := Swarms(e, j.SwarmLogin); swarm || !j.trusted(c.Author) || (hasPlan && !c.Created.After(plan.Created)) {
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

func (j *Job) pullRequest(p forge.Pull) string {
	type item struct {
		at   time.Time
		text string
	}
	var items []item
	if j.Pull != nil {
		for _, c := range j.Pull.Comments {
			items = append(items, item{c.Created, j.entry(3, Entry{c.Author, c.Body, c.Created}, "")})
		}
		for _, r := range j.Pull.Reviews {
			e := Entry{r.Author, r.Body, r.Submitted}
			m, swarm := Swarms(e, j.SwarmLogin)
			if swarm && r.Commit == p.HeadSHA && (j.Role == "adversary" || j.Role == "sympathetic") && m.Role != j.Role && (m.Role == "adversary" || m.Role == "sympathetic") {
				continue // the other reviewer's review of this head
			}
			verdict := stateName(r.State)
			if swarm {
				verdict = verdictOf(swarmText(r.Body))
			}
			items = append(items, item{r.Submitted, j.entry(3, e, fmt.Sprintf(", on %s: %s", r.Commit, verdict))})
		}
		for _, l := range j.Pull.Lines {
			items = append(items, item{l.Created, j.entry(3, Entry{l.Author, l.Body, l.Created}, fmt.Sprintf(", on %s:%d at %s", l.Path, l.Line, l.Commit))})
		}
	}
	slices.SortStableFunc(items, func(a, b item) int { return a.at.Compare(b.at) })
	var thread []string
	for _, it := range items {
		thread = append(thread, trim(it.text))
	}
	// The body is its opener's entry (section 6): the swarm's, marked;
	// a trusted login's as written; anyone else's omitted.
	body := p.Body
	if _, ok := Swarms(Entry{p.Author, p.Body, p.Created}, j.SwarmLogin); ok {
		body = swarmText(p.Body)
	} else if !j.trusted(p.Author) {
		body = fmt.Sprintf("Opened by %s, not trusted; content omitted.", p.Author)
	}
	return doc(fmt.Sprintf("# Pull request %s#%d: %s", p.Repo, p.Number, p.Title),
		fmt.Sprintf("Branch %s into %s; head %s.", p.Head, p.Base, p.HeadSHA), Shift(trim(body), 1), "## Thread", orNone(thread))
}

func (b *Batch) sections(j *Job) []string {
	var briefs []string
	for _, br := range b.active() {
		briefs = append(briefs, doc(fmt.Sprintf("## %s (Status: active; mounted at /resources/%s)", br.Path, br.Path), Shift(trim(br.Text), 2)))
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
	return []string{"# Briefs", orNone(briefs), "# Open swarm issues", list(open), "# Recently closed", list(closed)}
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
	text := is.Body
	if _, ok := Swarms(Entry{is.Author, is.Body, is.Created}, swarmLogin); ok {
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
