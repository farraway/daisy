package reconcile

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// Counts is one issue's correction counts in `swarm status`'s Throughput
// (section 11), read from the forge: the plans, the blocks and,
// per swarm pull request, its review rounds and the maintainer's entries.
type Counts struct {
	Plans  int // the architect's plans in the issue thread; the revisions are those after the first
	Blocks int // the reconciler's notes that blocked the issue
	Pulls  []PullCounts
}

// PullCounts is one pull request's review rounds and maintainer entries.
type PullCounts struct {
	Repo       string
	Number     int
	Rounds     int // Rounds over all its reviews
	Maintainer int // comments, reviews and line comments by a trusted login not the swarm's
}

// Revisions is the plans after the first.
func (c Counts) Revisions() int { return max(0, c.Plans-1) }

// Count is is's Counts (is with its comments), pulls being Pulls(n) and
// threads each pull request's thread, in the same order.
func Count(is forge.Issue, pulls []forge.Pull, threads []PullThread, swarmLogin string, trusted []string) Counts {
	var c Counts
	for _, cm := range is.Comments {
		m, ok := Swarms(Entry{cm.Author, cm.Body, cm.Created}, swarmLogin)
		switch {
		case !ok:
		case m.Role == "architect" && !strings.HasPrefix(swarmText(cm.Body), "BLOCKED:"):
			c.Plans++
		case blocks(m, cm.Body):
			c.Blocks++
		}
	}
	maintainer := func(login string) bool {
		return !strings.EqualFold(login, swarmLogin) && slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, login) })
	}
	for i, p := range pulls {
		t := threads[i]
		pc := PullCounts{Repo: p.Repo, Number: p.Number, Rounds: Rounds(t.Reviews, Answers(pullComments(t.Comments), swarmLogin), swarmLogin, time.Time{}, "")}
		for _, cm := range t.Comments {
			if maintainer(cm.Author) {
				pc.Maintainer++
			}
		}
		for _, r := range t.Reviews {
			if maintainer(r.Author) {
				pc.Maintainer++
			}
		}
		for _, l := range t.Lines {
			if maintainer(l.Author) {
				pc.Maintainer++
			}
		}
		c.Pulls = append(c.Pulls, pc)
	}
	return c
}

// roundLimitNote is the round limit's second note, from its start: the
// words alone could be a member's path quoted in a failure's note.
var roundLimitNote = regexp.MustCompile(`^Review round [0-9]+ of [0-9]+ on \S+ ` + roundLimitAgain + ` \(at ` + forge.ReviewInProgress + `\)\.`)

// blocks: the swarm's entry marked m, with body, is a reconciler note that
// blocked its issue: every note but the round limit's first, which
// returns the issue to the architect, the gate notes, which say why the
// issue waits, and the auto notes, which say why it did not.
func blocks(m Mark, body string) bool {
	return m.Role == "reconciler" && m.Kind == "" && !strings.HasSuffix(swarmText(body), architectRevises)
}

// blockingNote is the newest of the reconciler's notes on is that blocked
// it (blocks); ok is false when there is none.
func blockingNote(is forge.Issue, swarmLogin string) (e Entry, ok bool) {
	thread := IssueThread(is)
	slices.SortStableFunc(thread, func(a, b Entry) int { return a.Created.Compare(b.Created) })
	for i := len(thread) - 1; i >= 0; i-- {
		if m, swarm := Swarms(thread[i], swarmLogin); swarm && blocks(m, thread[i].Body) {
			return thread[i], true
		}
	}
	return Entry{}, false
}

// AtTheRoundLimit: the newest of the reconciler's notes that blocked is
// (blockingNote) is the round limit's second, which leaves the issue at
// review-1-in-progress, where no role runs, both reviewers having
// reviewed the head (section 6).
func AtTheRoundLimit(is forge.Issue, swarmLogin string) bool {
	e, ok := blockingNote(is, swarmLogin)
	return ok && roundLimitNote.MatchString(swarmText(e.Body))
}
