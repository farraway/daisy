package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"daisy/internal/ops"
	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// batch is pass step (7), the owner's batch trigger (section 6, G0): the
// owner runs as issue 0 when nothing runs or waits to be collected on
// issue 0, <state>/owner-held is absent, a brief under resources is
// `active` (Held, below) and the pacing condition holds:
// with the triage gate on, no open issue is proposal-posted and the
// routine jobs at proposal-accepted, which Daisy triaged, are fewer than
// owner.max_open_proposals, and the run proposes the difference; off, the
// open proposal-posted and proposal-accepted issues are fewer than that,
// and it proposes the difference.
func (s *Swarm) batch(running []runner.Running, launch func(*Job) error) error {
	fs := s.O.FS()
	if _, err := fs.Stat(heldFile(s.state(), "owner")); err == nil {
		return nil
	}
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	if len(s.busy(running, runs, 0)) > 0 {
		return nil
	}
	briefs, err := s.briefs()
	if err != nil {
		return err
	}
	if NoActiveBrief(briefs) {
		return nil
	}
	open, err := s.issues(forge.Open)
	if err != nil {
		return err
	}
	waiting := 0
	for _, is := range open {
		switch {
		case slices.Contains(is.Labels, forge.ProposalPosted):
			if s.Cfg.Gates.Triage {
				return nil
			}
			waiting++
		case slices.Contains(is.Labels, forge.ProposalAccepted):
			if s.Cfg.Gates.Triage {
				if slices.Contains(is.Labels, forge.ClassArchitecture) {
					continue // triaged by the maintainer
				}
				full, err := s.Forge.Issue(is.Number)
				if err != nil {
					return err
				}
				if ClassOf(full, s.SwarmLogin, s.Cfg.Trusted) != Routine {
					continue
				}
			}
			waiting++
		}
	}
	p := s.Cfg.Owner.MaxOpenProposals - waiting
	if p <= 0 {
		return nil
	}
	b := &Batch{Open: open, Propose: p, Briefs: briefs}
	if b.Closed, err = s.recentlyClosed(b.active()); err != nil {
		return err
	}
	persona, err := fs.ReadFile(declare.PromptFile(s.Place.Root, "owner"))
	if err != nil {
		return err
	}
	return launch(&Job{Cfg: s.Cfg, Host: s.Cfg.Forge.ForgeHost(), SwarmLogin: s.SwarmLogin, Role: "owner", K: NextK(runs, 0, "owner", nil),
		Time: s.O.Now(), Persona: string(persona), Batch: b})
}

// heldFile is <state>/<role>-held, the hold on a batch run's trigger
// (section 2): the owner's owner-held, the QA's qa-held.
func heldFile(state, role string) string { return filepath.Join(state, role+"-held") }

// qa is pass step (7)'s second trigger, the QA's (section 6,
// The QA trigger): with swarm.json declaring it, the QA
// runs as issue 0 when nothing runs or waits to be collected on issue 0,
// <state>/qa-held is absent and its trigger holds on the first
// repository's base at the pass's commit (qaDue); it reports at
// most owner.max_open_proposals failures, and reads the open and the
// recently closed swarm issues so as to report none twice.
func (s *Swarm) qa(running []runner.Running, launch func(*Job) error) error {
	if s.Cfg.QA == nil {
		return nil
	}
	fs := s.O.FS()
	if _, err := fs.Stat(heldFile(s.state(), declare.QARole)); err == nil {
		return nil
	}
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	if len(s.busy(running, runs, 0)) > 0 {
		return nil
	}
	if err := s.fetch(); err != nil {
		return err
	}
	if due, err := s.qaDue(runs); err != nil || !due {
		return err
	}
	b := &Batch{Propose: s.Cfg.Owner.MaxOpenProposals}
	if b.Open, err = s.issues(forge.Open); err != nil {
		return err
	}
	if b.Closed, err = s.recentlyClosed(nil); err != nil {
		return err
	}
	persona, err := fs.ReadFile(declare.PromptFile(s.Place.Root, declare.QARole))
	if err != nil {
		return err
	}
	return launch(&Job{Cfg: s.Cfg, Host: s.Cfg.Forge.ForgeHost(), SwarmLogin: s.SwarmLogin, Role: declare.QARole, K: NextK(runs, 0, declare.QARole, nil),
		Time: s.O.Now(), Persona: string(persona), Batch: b})
}

// qaDue: the QA trigger holds (section 6, The QA
// trigger): on the first repository's base at the pass's commit, since
// the commit the newest completed exploration explored (LastQA; with
// none, since the branch began), qa.merges commits or more on the
// first-parent line, or a tag matching qa.tag.
func (s *Swarm) qaDue(runs []Run) (bool, error) {
	product := s.Cfg.Repos[0].Name
	head := s.commits[product]
	if head == "" {
		return false, nil
	}
	from, err := LastQA(s.O.FS(), runs, product)
	if err != nil {
		return false, err
	}
	e, g := s.Cfg.QA, s.git()
	if e.Merges != nil {
		n, err := g.Merges(product, head, from)
		if err != nil || n >= *e.Merges {
			return err == nil, err
		}
	}
	if e.Tag != nil {
		tags, err := g.TagsSince(product, *e.Tag, head, from)
		return len(tags) > 0, err
	}
	return false, nil
}

// LastQA is the commit of the first repository, product, that the
// newest completed exploration explored: the `start <product> <sha>`
// line of the newest qa run whose outcome is posted (it reported
// its failures, or none); "" when no run has.
func LastQA(fs ops.FS, runs []Run, product string) (string, error) {
	runs = slices.Clone(runs)
	slices.SortFunc(runs, func(a, b Run) int { return b.K - a.K })
	for _, r := range runs {
		if r.N != 0 || r.Role != declare.QARole {
			continue
		}
		lines, err := r.Result(fs)
		if err != nil {
			return "", err
		}
		if !slices.Contains(lines, "outcome posted") {
			continue
		}
		if l, ok := Has(lines, "start "+product); ok {
			return strings.TrimPrefix(l, "start "+product+" "), nil
		}
	}
	return "", nil
}

// briefs is the briefs the batch reads (section 7.3): at the pass's
// commits, the mirrors fetched first (section 7.1, step 1), so that the
// trigger and the prompt read what is pushed.
func (s *Swarm) briefs() ([]brief.Brief, error) {
	if err := s.fetch(); err != nil {
		return nil, err
	}
	return Briefs(s.O, s.Place, s.Cfg, s.state(), s.commits)
}

// NoActiveBrief is the hold on the owner's batch trigger while no brief
// under resources is `active` (section 6, G0): the owner proposes the
// next jobs of an active brief, and with none it would propose from the
// whole tree; status names the hold on its State line.
func NoActiveBrief(briefs []brief.Brief) bool {
	return !slices.ContainsFunc(briefs, func(b brief.Brief) bool { return b.Status == "active" })
}

// issues is every swarm issue in state, listed by forge.SwarmLabels, by
// number.
func (s *Swarm) issues(state forge.State) ([]forge.Issue, error) {
	var out []forge.Issue
	for _, label := range forge.SwarmLabels(state) {
		is, err := s.Forge.Issues(state, label)
		if err != nil {
			return nil, err
		}
		for _, i := range is {
			if !slices.ContainsFunc(out, func(o forge.Issue) bool { return o.Number == i.Number }) {
				out = append(out, i)
			}
		}
	}
	slices.SortFunc(out, func(a, b forge.Issue) int { return a.Number - b.Number })
	return out, nil
}

// recentlyClosed is the batch prompt's closed list (section 7.3): the
// swarm issues closed in the last 30 days, newest first, at most 20, then
// every other closed issue whose Brief: line names an active brief,
// oldest first; each with its merged pull requests and its newest counted
// comment that is not the swarm's.
func (s *Swarm) recentlyClosed(active []brief.Brief) ([]Closed, error) {
	closed, err := s.issues(forge.Closed)
	if err != nil {
		return nil, err
	}
	recent, older := Recent(closed, s.O.Now())
	var rest []forge.Issue
	for _, is := range older {
		path, _ := BriefOf(is, s.SwarmLogin)
		if slices.ContainsFunc(active, func(b brief.Brief) bool { return b.Path == path }) {
			rest = append(rest, is)
		}
	}
	slices.Reverse(rest)
	var out []Closed
	for _, is := range append(recent, rest...) {
		pulls, err := s.pulls(is.Number)
		if err != nil {
			return nil, err
		}
		c := Closed{Issue: is}
		for _, p := range pulls {
			if p.Merged {
				c.Merged++
			}
		}
		full, err := s.Forge.Issue(is.Number)
		if err != nil {
			return nil, err
		}
		for _, cm := range full.Comments {
			e := Entry{cm.Author, cm.Body, cm.Created}
			if _, swarm := Swarms(e, s.SwarmLogin); !swarm && slices.ContainsFunc(s.Cfg.Trusted, func(l string) bool { return strings.EqualFold(l, cm.Author) }) {
				c.Last = cm.Body
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// Recent is closed split as the batch prompt's closed list reads it
// (section 7.3): the issues closed in the last 30 days, newest first, at
// most 20, and the others, newest first.
func Recent(closed []forge.Issue, now time.Time) (recent, older []forge.Issue) {
	closed = slices.Clone(closed)
	slices.SortStableFunc(closed, func(a, b forge.Issue) int { return b.Closed.Compare(a.Closed) })
	for _, is := range closed {
		if now.Sub(is.Closed) <= 30*24*time.Hour && len(recent) < 20 {
			recent = append(recent, is)
		} else {
			older = append(older, is)
		}
	}
	return recent, older
}

var (
	proposalLine = regexp.MustCompile(`^## (.+)$`)
	metLine      = regexp.MustCompile(`^Met: (.+)$`)
)

// noFailure begins the QA's output when every route and control it
// drove worked (section 7.5).
const noFailure = "NO FAILURE:"

// Proposal is one job a batch run proposes (section 7.5).
type Proposal struct{ Title, Body string }

// Proposals reads a batch run's out.md (section 7.5): the `Met:` lines
// before the first proposal, then the first line after them when it is a
// BLOCKED: line (isBlocked, blocked the rest of it, maybe empty), and the
// proposals, each a `## <title>` line (a title of 1 to 200 characters) and
// its body up to the next, blank lines trimmed.
func Proposals(out string) (met []string, blocked string, isBlocked bool, ps []Proposal) {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	i := 0
	for ; i < len(lines); i++ {
		l := strings.TrimRight(lines[i], " ")
		if m := metLine.FindStringSubmatch(l); m != nil {
			met = append(met, strings.TrimSpace(m[1]))
			continue
		}
		if l != "" {
			break
		}
	}
	if i < len(lines) && strings.HasPrefix(strings.TrimRight(lines[i], " "), "BLOCKED:") {
		return met, strings.TrimSpace(strings.TrimPrefix(lines[i], "BLOCKED:")), true, nil
	}
	var cur *Proposal
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.Trim(strings.Join(body, "\n"), "\n")
			ps = append(ps, *cur)
		}
	}
	for ; i < len(lines); i++ {
		if m := proposalLine.FindStringSubmatch(lines[i]); m != nil {
			if t := strings.TrimSpace(m[1]); utf8.RuneCountInString(t) >= 1 && utf8.RuneCountInString(t) <= 200 {
				flush()
				cur, body = &Proposal{Title: t}, nil
				continue
			}
		}
		body = append(body, lines[i])
	}
	flush()
	return met, "", false, ps
}

var waitingOn = regexp.MustCompile(`(?i)^waiting on (#\d+(?:\s*,\s*#\d+)*)\s*:`)

// WaitingOn is the issues a batch run's BLOCKED: text names in the form
// its Task text asks for when what is left waits on open issues (section
// 7.4), `waiting on #<n>, #<m>: <why>`; none for any other text, a
// number in passing included.
func WaitingOn(blocked string) []int {
	m := waitingOn.FindStringSubmatch(blocked)
	if m == nil {
		return nil
	}
	var ns []int
	for _, f := range strings.Split(m[1], ",") {
		n, _ := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(f), "#"))
		if !slices.Contains(ns, n) {
			ns = append(ns, n)
		}
	}
	return ns
}

// stillOpen is the issues of ns the forge reads open, the ones the hold
// waits on (section 6, G0): one closed already, or that names no issue,
// is not waited on, so a hold whose every issue is closed is the
// maintainer's to release, never released at the next pass into the
// same BLOCKED:.
func (s *Swarm) stillOpen(ns []int) ([]int, error) {
	var open []int
	for _, n := range ns {
		is, err := s.Forge.Issue(n)
		if errors.Is(err, forge.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if is.Open {
			open = append(open, n)
		}
	}
	return open, nil
}

var waitsLine = regexp.MustCompile(`^0-owner-(\d+) waits (#\d+(?: #\d+)*)$`)

// HoldWaits is what owner-held's second line records (section 2): the
// batch run that wrote the hold and the issues it waits on; none when the
// hold waits on no issue, so only `swarm resume` releases it.
func HoldWaits(held []byte) (k int, waits []int) {
	lines := strings.Split(string(held), "\n")
	if len(lines) < 2 {
		return 0, nil
	}
	m := waitsLine.FindStringSubmatch(strings.TrimSpace(lines[1]))
	if m == nil {
		return 0, nil
	}
	k, _ = strconv.Atoi(m[1])
	for _, f := range strings.Fields(m[2]) {
		n, _ := strconv.Atoi(strings.TrimPrefix(f, "#"))
		waits = append(waits, n)
	}
	return k, waits
}

// release is the owner's hold released by its issues (section 6, G0), at
// each pass after the collections and before the pause: owner-held
// removed once the forge reads every issue it waits on closed, with a
// stdout line and the journal line `0-release-<k>`, k the batch run that
// held it. A hold waiting on no issue (the maintainer's to release) is
// left; an issue that names none no longer is a warning, the hold kept.
func (s *Swarm) release() error {
	fs := s.O.FS()
	path := heldFile(s.state(), "owner")
	held, err := fs.ReadFile(path)
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	k, waits := HoldWaits(held)
	if len(waits) == 0 {
		return nil
	}
	for _, n := range waits {
		is, err := s.Forge.Issue(n)
		if errors.Is(err, forge.ErrNotFound) {
			fmt.Fprintf(s.Err, "⚠ owner batch: #%d, which its hold waits on, names no issue; held (%s)\n", n, path)
			return nil
		}
		if err != nil {
			return err
		}
		if is.Open {
			return nil
		}
	}
	if err := fs.Remove(path); err != nil {
		return err
	}
	what := "owner batch released: " + issueList(waits) + " closed"
	fmt.Fprintln(s.Out, strings.ToUpper(what[:1])+what[1:])
	now := s.O.Now().UTC()
	return s.queue(now.Format("2006-01-02"), fmt.Sprintf("- %s 0-release-%d: %s", now.Format("1504"), k, what))
}

// AreClosed is `#<n> is closed` or `#<n> #<m> are closed`, the waits of
// a hold as stdout and status name them.
func AreClosed(ns []int) string {
	if len(ns) == 1 {
		return issueList(ns) + " is closed"
	}
	return issueList(ns) + " are closed"
}

func issueList(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(s, " ")
}

// batch collects a batch run, the owner's or the QA's (sections
// 6, 7.5 and 7.6): each proposal created in order, marked as its role's,
// with swarm:class-architecture unless its Class: line says routine,
// `creating <i>` recorded before its CreateIssue so none is created
// twice; the QA's NO FAILURE: creates none and completes the run;
// a failure, BLOCKED: or nothing proposed writes <state>/<role>-held and
// creates nothing, the owner's BLOCKED: waiting on open issues recording
// them (WaitingOn, stillOpen); a swarm fault writes no hold.
func (c *collection) batch() error {
	s, fs := c.s, c.s.O.FS()
	qa := c.r.Role == declare.QARole
	if l, ok := Has(c.lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		return c.end("fault")
	}
	hold := func(reason, outcome string, waits []int) error {
		path := heldFile(s.state(), c.r.Role)
		body, until := "held "+stamp(s.O.Now())+": "+reason+"\n", ""
		if len(waits) > 0 {
			body += fmt.Sprintf("0-owner-%d waits %s\n", c.r.K, issueList(waits))
			until = " until " + AreClosed(waits)
		}
		if err := fs.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		what := "batch"
		if qa {
			what = "qa"
		}
		fmt.Fprintf(s.Out, "%s%s; %s held%s (%s)\n", c.head, reason, what, until, path)
		return c.end(outcome)
	}
	failed := func(reason string) error {
		if _, ok := Has(c.lines, "failed"); !ok {
			if err := c.step("failed: " + reason); err != nil {
				return err
			}
		}
		return hold(reason, "failed", nil)
	}
	if l, ok := Has(c.lines, "failed"); ok {
		return failed(strings.TrimPrefix(l, "failed: "))
	}
	out, err := c.r.Output(fs)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if strings.TrimSpace(string(out)) == "" {
		return failed("no output")
	}
	if hit(out, s.secretsOf(c.r, c.lines)) {
		return failed("secret in output: out.md")
	}
	if first, _, _ := strings.Cut(strings.TrimLeft(string(out), "\n"), "\n"); qa && strings.HasPrefix(first, noFailure) {
		fmt.Fprintf(s.Out, "%sno failure\n", c.head)
		return c.end("posted")
	}
	_, blocked, isBlocked, ps := Proposals(string(out))
	if isBlocked && qa {
		return hold("blocked: "+blocked, "blocked", nil)
	}
	if isBlocked {
		waits, err := s.stillOpen(WaitingOn(blocked))
		if err != nil {
			return err
		}
		return hold("blocked: "+blocked, "blocked", waits)
	}
	l, _ := Has(c.lines, "propose")
	p, _ := strconv.Atoi(strings.TrimPrefix(l, "propose "))
	if len(ps) > p {
		ps = ps[:p]
	}
	if len(ps) == 0 {
		return failed("proposed nothing")
	}
	label := forge.ProposalPosted
	if !s.Cfg.Gates.Triage {
		label = forge.ProposalAccepted
	}
	for i, pr := range ps {
		creating := fmt.Sprintf("creating %d", i+1)
		if slices.Contains(c.lines, creating) {
			continue
		}
		if err := c.step(creating); err != nil {
			return err
		}
		labels := []string{label}
		if ProposalClass(c.r.Role, pr.Body) != Routine {
			labels = append(labels, forge.ClassArchitecture) // unclassed is architecture
		}
		n, err := s.Forge.CreateIssue(pr.Title, marker(c.r.Role, c.r.K)+pr.Body, labels)
		if err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "%sproposed #%d %s\n", c.head, n, pr.Title)
	}
	return c.end("posted")
}
