package reconcile

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// The follow-up (section 7.11, Follow-ups; rule 6): the non-blocking
// findings of a merged pull request's reviews, filed once as an issue
// carrying swarm:follow-up alone, for the owner's batch to triage.

// FollowUps files the follow-up of each pull request the listing of its
// issue's pull requests (ListPulls) finds merged and not yet shown so in
// the block. The issues carrying swarm:follow-up, open and closed, are
// read once by a value (a pass's, or a verb's) and the ones it files
// added to them. Outputs is where a review Daisy cut is read whole.
type FollowUps struct {
	Outputs Outputs
	filed   []forge.Issue
	read    bool
}

// file files pull request p's follow-up, p merged on issue is: none when
// a follow-up of p is already filed (followUpOf), or when no finding is
// left; otherwise CreateIssue with the title (followUpTitle), the marker lines and the
// numbered findings, and `#<n> follow-up of <owner>/<name>#<m> filed as
// #<f>` on out. Reviews, PullComments and the lists are read under f.
func (fu *FollowUps) file(f forge.Adapter, owner string, is forge.Issue, p forge.Pull, openers []string, swarmLogin string, out io.Writer) error {
	if !fu.read {
		for _, state := range []forge.State{forge.Open, forge.Closed} {
			issues, err := f.Issues(state, forge.FollowUp)
			if err != nil {
				return err
			}
			fu.filed = append(fu.filed, issues...)
		}
		fu.read = true
	}
	for _, fi := range fu.filed {
		if followUpOf(fi.Body, owner, p) {
			return nil
		}
	}
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	reviews, err := f.Reviews(ref)
	if err != nil {
		return err
	}
	comments, err := f.PullComments(ref)
	if err != nil {
		return err
	}
	findings, err := followUpFindings(is.Number, fu.Outputs, reviews, Reopenings(pullComments(comments), swarmLogin, openers), swarmLogin)
	if err != nil || len(findings) == 0 {
		return err
	}
	s := sliceOf(p, is.Number)
	body := fmt.Sprintf("%s\n**reconciler:** follow-up of %s, merged at %s, slice %d of #%d: the reviews' findings that did not block it.\n", followUpMarker, pullLink(owner, p), short(p.MergeSHA), s, is.Number)
	for i, l := range findings {
		body += fmt.Sprintf("\n%d. %s", i+1, l)
	}
	k, err := f.CreateIssue(followUpTitle(len(findings), is), body, []string{forge.FollowUp})
	if err != nil {
		return err
	}
	fu.filed = append(fu.filed, forge.Issue{Number: k, Body: body, Open: true, Labels: []string{forge.FollowUp}})
	fmt.Fprintf(out, "#%d follow-up of %s/%s#%d filed as #%d\n", is.Number, owner, p.Repo, p.Number, k)
	return nil
}

// followUpTitle is the title of a follow-up of k findings left by a pull
// request of issue is (section 7.11, Follow-ups): `<k> findings left by
// #<n>: <the issue's title>`, "1 finding" when k is 1.
func followUpTitle(k int, is forge.Issue) string {
	findings := "findings"
	if k == 1 {
		findings = "finding"
	}
	return fmt.Sprintf("%d %s left by #%d: %s", k, findings, is.Number, is.Title)
}

// followUpItems is a follow-up's numbered items as its body holds them,
// the marker line and the `**reconciler:**` line left out (section 7.3).
func followUpItems(body string) string {
	lines := strings.SplitN(body, "\n", 3)
	if len(lines) < 3 {
		return ""
	}
	return strings.Trim(lines[2], "\n")
}

// followUpOf: body is a follow-up of pull request p, its first line the
// follow-up's marker and its second linking p.
func followUpOf(body, owner string, p forge.Pull) bool {
	lines := strings.SplitN(body, "\n", 3)
	if len(lines) < 2 || strings.TrimSuffix(lines[0], "\r") != followUpMarker {
		return false
	}
	name := fmt.Sprintf("%s/%s#%d", owner, p.Repo, p.Number)
	rest, ok := strings.CutPrefix(lines[1], "**reconciler:** follow-up of ")
	return ok && (strings.HasPrefix(rest, "["+name+"](") || strings.HasPrefix(rest, name+","))
}

// shownMerged: body's pulls block shows pull request p merged, its item
// naming p and ending `merged` (section 5, The pull requests on the issue).
func shownMerged(body, owner string, p forge.Pull) bool {
	lines := strings.Split(body, "\n")
	from, to, ok := pullsSpan(lines)
	if !ok {
		return false
	}
	name := fmt.Sprintf("%s/%s#%d", owner, p.Repo, p.Number)
	for _, l := range lines[from+1 : to] {
		for _, item := range strings.Split(strings.TrimPrefix(strings.TrimSpace(l), "**Pull requests:** "), " · ") {
			if strings.HasSuffix(item, " merged") && (strings.Contains(item, " ["+name+"](") || strings.Contains(item, " "+name+" ")) {
				return true
			}
		}
	}
	return false
}

// lineNumber is the line number of a finding's `<file>:<line>`
// reference, which the follow-up's deduplication drops.
var lineNumber = regexp.MustCompile(`([^\s:]):[0-9]+`)

// findingKey is how two findings are told the same (section 7.11,
// Follow-ups): the line with its reference's line number dropped,
// whitespace and case aside.
func findingKey(line string) string {
	return strings.ToLower(strings.Join(strings.Fields(lineNumber.ReplaceAllString(line, "$1")), ""))
}

// followUpFindings is what the follow-up of a pull request of issue n
// holds (section 7.11, Follow-ups): each line beginning `[non-blocking]`
// of every swarm review on it, every round's, the approving ones
// included, as `<role> run <k>, round <r>: <the line>`, by round and, in
// each, by review; a finding already listed (findingKey) is not listed
// again, the earliest round's kept. A review Daisy cut on the forge is
// read whole (Job.text; decision 149).
func followUpFindings(n int, outputs Outputs, reviews []forge.Review, answers []time.Time, swarmLogin string) ([]string, error) {
	j := &Job{N: n, Outputs: outputs}
	var out []string
	seen := map[string]bool{}
	for r, rd := range roundsOf(reviews, answers, swarmLogin) {
		for _, rv := range rd.reviews {
			m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, swarmLogin)
			text, err := j.text(m, swarmText(rv.Body))
			if err != nil {
				return nil, err
			}
			for _, l := range linesBeginning(text, "[non-blocking]") {
				if k := findingKey(l); !seen[k] {
					seen[k] = true
					out = append(out, fmt.Sprintf("%s run %d, round %d: %s", m.Role, m.Run, r+1, l))
				}
			}
		}
	}
	return out, nil
}
