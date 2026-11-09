package reconcile

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// gateNote posts the gate note on issue n, the flag step having just
// given it swarm:gated (section 5): which gate holds it and why, what the
// maintainer examines, drawn from the issue as it is now, and the
// decision's verbs. open is the open swarm issues as the flag step listed
// them, for the triage gate's overlap.
func (s *Swarm) gateNote(n int, open []forge.Issue) error {
	is, err := s.Forge.Issue(n)
	if err != nil {
		return err
	}
	text, err := s.gateText(is, open)
	if err != nil {
		return err
	}
	if err := s.Forge.Comment(forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: n}, gateMarker+"\n"+text); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "#%d gate note posted\n", n)
	return nil
}

// gateText is the gate note after its marker line: `**reconciler:** gated
// at <stage>: <why>.`, a blank line, the bullets, a blank line and the
// decision.
func (s *Swarm) gateText(is forge.Issue, open []forge.Issue) (string, error) {
	n := is.Number
	stage, _ := StageOf(is)
	verb := func(v string) string { return fmt.Sprintf("`daisy swarm %s %d`", v, n) }
	var why, decide string
	var items []string
	switch {
	case slices.Contains(is.Labels, forge.Blocked):
		why = "it is blocked, and only the maintainer unblocks it"
		items = s.blockedItems(is)
		decide = fmt.Sprintf("make the relabel the note names, if it names one, then `daisy swarm unblock %d <comment>`; or %s", n, verb("close"))
	case stage == forge.ProposalPosted:
		why = "the triage gate holds a proposal until the maintainer triages it"
		items = s.triageItems(is, open)
		decide = verb("triage") + ", " + verb("close") + ", or a comment here, which the owner answers"
	case stage == forge.PlanPosted:
		why = "the plan gate holds the plan until the maintainer accepts it"
		items = s.planItems(is)
		decide = verb("accept") + ", or a comment here, from which the architect revises the plan"
	default: // review-2-approved under the merge gate
		why = "both reviews approve, and the merge gate leaves the merge to the maintainer"
		var err error
		if items, err = s.mergeItems(is); err != nil {
			return "", err
		}
		decide = verb("merge") + ", " + verb("approve") + " first when the forge wants an approving review, or a comment"
	}
	return fmt.Sprintf("**reconciler:** gated at %s: %s.\n\n%s\n\nDecide: %s.", strings.TrimPrefix(stage, "swarm:"), why, strings.Join(items, "\n"), decide), nil
}

// issueRef is a reference to an issue of forge.repo in a text: `#<m>`
// not inside a word or a path (`<owner>/<repo>#<m>` is another's).
var issueRef = regexp.MustCompile(`(?:^|[^\w/#])#([1-9][0-9]*)\b`)

// triageItems: the brief and milestone the Brief: line names, the other
// open issues on that milestone, and the issues the proposal names with
// where each stands.
func (s *Swarm) triageItems(is forge.Issue, open []forge.Issue) []string {
	var items []string
	openIssue := func(m int) (forge.Issue, bool) {
		i := slices.IndexFunc(open, func(o forge.Issue) bool { return o.Number == m })
		if i < 0 {
			return forge.Issue{}, false
		}
		return open[i], true
	}
	if path, milestone := BriefOf(is, s.SwarmLogin); path == "" {
		items = append(items, "- Brief: none named; the body opens with no `Brief:` line.")
	} else {
		items = append(items, fmt.Sprintf("- Brief: %s, milestone %s.", path, milestone))
		var same []string
		for _, o := range open {
			if p, m := BriefOf(o, s.SwarmLogin); o.Number != is.Number && p == path && m == milestone {
				same = append(same, fmt.Sprintf("#%d %s", o.Number, o.Title))
			}
		}
		items = append(items, "- Open issues on the same milestone: "+listOrNone(same, "; ")+".")
	}
	body := is.Body
	if _, ok := Swarms(Entry{is.Author, is.Body, is.Created}, s.SwarmLogin); ok {
		body = swarmText(body)
	}
	var named []string
	seen := map[int]bool{is.Number: true}
	for _, m := range issueRef.FindAllStringSubmatch(body, -1) {
		num, _ := strconv.Atoi(m[1])
		if seen[num] {
			continue
		}
		seen[num] = true
		where := "no open swarm issue"
		if o, ok := openIssue(num); ok {
			where = "open"
			if st, ok := StageOf(o); ok {
				where += ", " + strings.TrimPrefix(st, "swarm:")
			}
		}
		named = append(named, fmt.Sprintf("#%d (%s)", num, where))
	}
	return append(items, "- Issues it names, its dependencies among them: "+listOrNone(named, ", ")+".")
}

// itemMark is a Markdown list item's or heading's mark.
var itemMark = regexp.MustCompile(`^(?:[-*+]|[0-9]+[.)]|#{1,6})[ \t]+`)

// planItems: the accepted plan's slice count and titles, its Open
// Questions as written and its Risks' first lines (each line of the
// section that is not indented: an item's, a heading's or a paragraph's).
func (s *Swarm) planItems(is forge.Issue) []string {
	plan, _, ok := (&Job{SwarmLogin: s.SwarmLogin, Issue: is}).Plan()
	if !ok {
		return []string{"- Plan: none; the thread holds no plan of the architect's."}
	}
	text := swarmText(plan.Body)
	var items []string
	if lines := sliceLines(text); len(lines) == 0 {
		items = append(items, "- Slices: 1; the plan numbers none under `## Steps`.")
	} else {
		items = append(items, fmt.Sprintf("- Slices: %d.", len(lines)))
		for _, l := range lines {
			items = append(items, "  "+cut(l))
		}
	}
	var questions []string
	for _, l := range section(text, "Open Questions") {
		questions = append(questions, l.text)
	}
	if q := strings.Trim(strings.Join(questions, "\n"), "\n"); q == "" {
		items = append(items, "- Open questions: none in the plan.")
	} else {
		items = append(items, "- Open questions, as the plan has them:", indent(q, "  "))
	}
	var risks []string
	for _, l := range section(text, "Risks") {
		if t := strings.TrimRight(l.text, " \t"); !l.fenced && t != "" && t == strings.TrimLeft(t, " \t") {
			risks = append(risks, "  - "+cut(itemMark.ReplaceAllString(t, "")))
		}
	}
	if len(risks) == 0 {
		return append(items, "- Risks: none in the plan.")
	}
	return append(append(items, "- Risks:"), risks...)
}

// mergeItems: the current pull request, its slice of the plan's, its
// review rounds, and the non-blocking findings of the reviews that
// approved its head.
func (s *Swarm) mergeItems(is forge.Issue) ([]string, error) {
	pulls, err := s.Forge.Pulls(is.Number)
	if err != nil {
		return nil, err
	}
	if len(pulls) == 0 {
		return []string{"- Pull request: none."}, nil
	}
	cur := pulls[len(pulls)-1]
	ref := forge.Ref{Repo: cur.Repo, N: cur.Number}
	reviews, err := s.Forge.Reviews(ref)
	if err != nil {
		return nil, err
	}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return nil, err
	}
	state := "open, head " + short(cur.HeadSHA)
	if cur.Merged {
		state = "merged already"
	}
	items := []string{
		fmt.Sprintf("- Pull request %s/%s#%d, %s (%s).", s.owner(), cur.Repo, cur.Number, cur.Title, state),
		fmt.Sprintf("- Slice %d of %d.", sliceOf(cur, is.Number), planSlices(is, s.SwarmLogin)),
		fmt.Sprintf("- Review rounds: %d.", Rounds(reviews, Answers(pullComments(comments), s.SwarmLogin), s.SwarmLogin, time.Time{}, "")),
	}
	head, _ := ApprovedHead(reviews, s.SwarmLogin)
	var findings []string
	for _, role := range []string{"adversary", "sympathetic"} {
		var last forge.Review
		for _, r := range reviews {
			if m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, s.SwarmLogin); ok && m.Role == role && r.Commit == head && !r.Submitted.Before(last.Submitted) {
				last = r
			}
		}
		for _, l := range strings.Split(swarmText(last.Body), "\n") {
			if l = strings.TrimRight(l, "\r "); strings.HasPrefix(l, "[non-blocking]") {
				findings = append(findings, "  - "+role+": "+cut(l))
			}
		}
	}
	if len(findings) == 0 {
		return append(items, "- Non-blocking findings in the approving reviews: none."), nil
	}
	return append(append(items, "- Non-blocking findings in the approving reviews:"), findings...), nil
}

// blockedItems: the reconciler's note that blocked the issue, by its time
// and first line.
func (s *Swarm) blockedItems(is forge.Issue) []string {
	e, ok := blockingNote(is, s.SwarmLogin)
	if !ok {
		return []string{"- The note that set it: none on the issue; swarm:blocked was set by hand."}
	}
	first, _, _ := strings.Cut(swarmText(e.Body), "\n")
	return []string{fmt.Sprintf("- The note that set it, at %s: %s", stamp(e.Created), strings.TrimRight(first, "\r "))}
}

// cut is a line of at most 120 characters, cut with `…`.
func cut(l string) string {
	if r := []rune(l); len(r) > 120 {
		return string(r[:119]) + "…"
	}
	return l
}

func indent(text, by string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = by + l
		}
	}
	return strings.Join(lines, "\n")
}

func listOrNone(items []string, sep string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, sep)
}
