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

	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// batch is pass step (7), the owner's batch trigger (section 6, G0): the
// owner runs as issue 0 when nothing runs or waits to be collected on
// issue 0, <state>/owner-held is absent and the pacing condition holds:
// with the triage gate on, no open issue is proposed, and the run
// proposes up to owner.max_open_proposals; off, the open proposed and
// ready issues are fewer than that, and it proposes the difference.
func (s *Swarm) batch(running []runner.Running, launch func(*Job) error) error {
	fs := s.O.FS()
	if _, err := fs.Stat(filepath.Join(s.state(), "owner-held")); err == nil {
		return nil
	}
	runs, err := Runs(fs, s.state())
	if err != nil {
		return err
	}
	if s.busy(running, runs, 0) {
		return nil
	}
	open, err := s.issues(forge.Open)
	if err != nil {
		return err
	}
	waiting := 0
	for _, is := range open {
		for _, l := range is.Labels {
			if l == "swarm:proposed" || l == "swarm:ready" && !s.Cfg.Gates.Triage {
				waiting++
				break
			}
		}
	}
	p := s.Cfg.Owner.MaxOpenProposals
	switch {
	case s.Cfg.Gates.Triage && waiting > 0:
		return nil
	case !s.Cfg.Gates.Triage:
		p -= waiting
	}
	if p <= 0 {
		return nil
	}
	b := &Batch{Open: open, Propose: p}
	if b.Briefs, err = brief.Find(fs, s.WS, s.Cfg.Resources); err != nil {
		return err
	}
	if b.Closed, err = s.recentlyClosed(b.active()); err != nil {
		return err
	}
	persona, err := fs.ReadFile(declare.PromptFile(s.WS, "owner"))
	if err != nil {
		return err
	}
	return launch(&Job{Cfg: s.Cfg, Host: s.Cfg.Forge.ForgeHost(), SwarmLogin: s.SwarmLogin, Role: "owner", K: NextK(runs, 0, "owner", nil),
		Time: s.O.Now(), Persona: string(persona), Batch: b})
}

// issues is every issue in state carrying a swarm:* label, by number.
func (s *Swarm) issues(state forge.State) ([]forge.Issue, error) {
	var out []forge.Issue
	for _, label := range forge.Labels {
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
		pulls, err := s.Forge.Pulls(is.Number)
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

// Proposal is one job a batch run proposes (section 7.5).
type Proposal struct{ Title, Body string }

// Proposals reads a batch run's out.md (section 7.5): the `Met:` lines
// before the first proposal, then the first line after them when it is a
// BLOCKED: line, and the proposals, each a `## <title>` line (a title of
// 1 to 200 characters) and its body up to the next, blank lines trimmed.
func Proposals(out string) (met []string, blocked string, ps []Proposal) {
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
		return met, strings.TrimSpace(strings.TrimPrefix(lines[i], "BLOCKED:")), nil
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
	return met, "", ps
}

// batch collects the owner's batch run (sections 6, 7.5 and 7.6): each
// proposal created in order, `creating <i>` recorded before its
// CreateIssue so none is created twice; a failure, BLOCKED: or nothing
// proposed writes <state>/owner-held and creates nothing; a swarm fault
// writes no owner-held.
func (c *collection) batch() error {
	s, fs := c.s, c.s.O.FS()
	if l, ok := Has(c.lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		return c.end("fault")
	}
	hold := func(reason, outcome string) error {
		path := filepath.Join(s.state(), "owner-held")
		if err := fs.WriteFile(path, []byte("held "+stamp(s.O.Now())+": "+reason+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "%s%s; batch held (%s)\n", c.head, reason, path)
		return c.end(outcome)
	}
	failed := func(reason string) error {
		if _, ok := Has(c.lines, "failed"); !ok {
			if err := c.step("failed: " + reason); err != nil {
				return err
			}
		}
		return hold(reason, "failed")
	}
	if l, ok := Has(c.lines, "failed"); ok {
		return failed(strings.TrimPrefix(l, "failed: "))
	}
	out, err := fs.ReadFile(filepath.Join(c.r.Dir, "io", "out.md"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if strings.TrimSpace(string(out)) == "" {
		return failed("no output")
	}
	if hit(out, s.secretsOf(c.r, c.lines)) {
		return failed("secret in output: out.md")
	}
	_, blocked, ps := Proposals(string(out))
	if blocked != "" {
		return hold("blocked: "+blocked, "blocked")
	}
	l, _ := Has(c.lines, "propose")
	p, _ := strconv.Atoi(strings.TrimPrefix(l, "propose "))
	if len(ps) > p {
		ps = ps[:p]
	}
	if len(ps) == 0 {
		return failed("proposed nothing")
	}
	label := "swarm:proposed"
	if !s.Cfg.Gates.Triage {
		label = "swarm:ready"
	}
	for i, pr := range ps {
		creating := fmt.Sprintf("creating %d", i+1)
		if slices.Contains(c.lines, creating) {
			continue
		}
		if err := c.step(creating); err != nil {
			return err
		}
		n, err := s.Forge.CreateIssue(pr.Title, marker("owner", c.r.K)+pr.Body, []string{label})
		if err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "%sproposed #%d %s\n", c.head, n, pr.Title)
	}
	return c.end("posted")
}
