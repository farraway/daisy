package reconcile

import (
	"fmt"
	"io"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// The pulls block (section 5, The pull requests on the issue): Daisy's
// list of an issue's pull requests, kept at the top of its body between
// these two lines, a view for the maintainer that no rule reads and that
// every reader of the body leaves out (withoutPulls).
const (
	pullsOpen  = "<!-- daisy-swarm kind=pulls -->"
	pullsClose = "<!-- /daisy-swarm kind=pulls -->"
)

// pullsSpan is the block in lines, its opening line and its closing
// line: the first closing line, whitespace at either end aside, and the
// nearest opening line before it, so the rests of a damaged block never
// take in the text between them; ok is false when there is none.
func pullsSpan(lines []string) (from, to int, ok bool) {
	from = -1
	for i, l := range lines {
		switch strings.TrimSpace(l) {
		case pullsOpen:
			from = i
		case pullsClose:
			if from >= 0 {
				return from, i, true
			}
		}
	}
	return 0, 0, false
}

// withoutPulls is body without its block and the empty line after it:
// the body as every reader reads it, the owner's marker, Brief: and
// Class: lines and what a member is given (sections 5 to 7).
func withoutPulls(body string) string {
	lines := strings.Split(body, "\n")
	from, to, ok := pullsSpan(lines)
	if !ok {
		return body
	}
	if to+1 < len(lines) && strings.TrimSpace(lines[to+1]) == "" {
		to++
	}
	return strings.Join(append(lines[:from:from], lines[to+1:]...), "\n")
}

// withPulls is body carrying block: its block replaced in place, the
// rest of the body as it was; with none, block and an empty line put
// after the marker lines (the marker, its `**<role>:**` line and the
// empty line after them) when the body's first line is the marker, so
// that it stays first, and at the top otherwise. block "" takes the
// block out.
func withPulls(body, block string) string {
	lines := strings.Split(body, "\n")
	from, to, ok := pullsSpan(lines)
	switch {
	case ok && block == "":
		return withoutPulls(body)
	case ok:
		return strings.Join(append(append(lines[:from:from], strings.Split(block, "\n")...), lines[to+1:]...), "\n")
	case block == "":
		return body
	}
	k := 0
	if _, marked := Marked(body); marked {
		k = 1
		if k < len(lines) && strings.HasPrefix(lines[k], "**") {
			k++
		}
		if k < len(lines) && strings.TrimSpace(lines[k]) == "" {
			k++
		}
	}
	return strings.Join(append(append(lines[:k:k], strings.Split(block, "\n")...), append([]string{""}, lines[k:]...)...), "\n")
}

// pullLink is pull request p named in a note or the block: its
// `<owner>/<name>#<m>`, which reads the same in a terminal, linked to its
// page, as the pull requests live in other repositories than the issue;
// the name alone when the forge reports no page.
func pullLink(owner string, p forge.Pull) string {
	name := fmt.Sprintf("%s/%s#%d", owner, p.Repo, p.Number)
	if p.URL == "" {
		return name
	}
	return "[" + name + "](" + p.URL + ")"
}

// pullState is how the block names pull request p's state: `merged`,
// `closed` (unmerged), or, open, `open, approved` when both newest
// reviews approve its head, else `open, review round <r>`, the round it
// is in: one more than its rounds (section 6, over all its reviews,
// comments holding the worker's answers without a commit).
func pullState(p forge.Pull, reviews []forge.Review, comments []forge.Comment, swarmLogin string) string {
	switch {
	case p.Merged:
		return "merged"
	case !p.Open:
		return "closed"
	}
	if head, _ := ApprovedHead(reviews, swarmLogin); head != "" && head == p.HeadSHA {
		return "open, approved"
	}
	return fmt.Sprintf("open, review round %d", Rounds(reviews, Answers(pullComments(comments), swarmLogin), swarmLogin, time.Time{}, "")+1)
}

// ListPulls keeps issue is's block (is read with its body) as its pull
// requests stand now (section 5): Pulls(n) with openers, and of each open
// one its reviews and comments for its state; a slice planned and not
// yet opened is not listed. When the body would change, the issue is
// read again right before the write, as a relabel reads the labels, and
// its body edited, the block alone replaced, with `#<n> pull requests
// listed` on out; a body with nothing to change is not written.
func ListPulls(f forge.Adapter, owner string, is forge.Issue, openers []string, swarmLogin string, out io.Writer) error {
	pulls, err := f.Pulls(is.Number, openers)
	if err != nil {
		return err
	}
	var items []string
	for _, p := range pulls {
		var reviews []forge.Review
		var comments []forge.Comment
		if p.Open {
			ref := forge.Ref{Repo: p.Repo, N: p.Number}
			if reviews, err = f.Reviews(ref); err != nil {
				return err
			}
			if comments, err = f.PullComments(ref); err != nil {
				return err
			}
		}
		items = append(items, fmt.Sprintf("slice %d %s %s", sliceOf(p, is.Number), pullLink(owner, p), pullState(p, reviews, comments, swarmLogin)))
	}
	block := ""
	if len(items) > 0 {
		block = pullsOpen + "\n**Pull requests:** " + strings.Join(items, " · ") + "\n" + pullsClose
	}
	if withPulls(is.Body, block) == is.Body {
		return nil
	}
	now, err := f.Issue(is.Number)
	if err != nil {
		return err
	}
	body := withPulls(now.Body, block)
	if body == now.Body {
		return nil
	}
	if err := f.EditIssue(is.Number, body); err != nil {
		return err
	}
	fmt.Fprintf(out, "#%d pull requests listed\n", is.Number)
	return nil
}

// listPulls is pass step (8)'s block, and the merge step's on an issue it
// closed: ListPulls under the swarm's token, a failure a warning, the
// block being a view.
func (s *Swarm) listPulls(is forge.Issue) {
	if err := ListPulls(s.Forge, s.owner(), is, Openers(s.SwarmLogin, s.Cfg.Trusted), s.SwarmLogin, s.Out); err != nil {
		fmt.Fprintf(s.Err, "⚠ #%d: pull requests: %v\n", is.Number, err)
	}
}
