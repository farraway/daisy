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
	Blocks int // the reconciler's notes that moved the issue to blocked
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
		case m.Role == "reconciler" && !strings.HasSuffix(swarmText(cm.Body), architectRevises):
			c.Blocks++
		}
	}
	maintainer := func(login string) bool {
		return !strings.EqualFold(login, swarmLogin) && slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, login) })
	}
	for i, p := range pulls {
		t := threads[i]
		pc := PullCounts{Repo: p.Repo, Number: p.Number, Rounds: Rounds(t.Reviews, swarmLogin, time.Time{}, "")}
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
var roundLimitNote = regexp.MustCompile(`^Review round [0-9]+ of [0-9]+ on \S+ ` + roundLimitAgain + ` \(was swarm:in-review\)\.`)

// blockedFromRe is the label a reconciler note names the issue left
// (section 7.6): `(was swarm:<label>)`, the last in the note, since every
// note writes it after any text it quotes (a failure's reason may hold a
// member's path, a refused merge the forge's message).
var blockedFromRe = regexp.MustCompile(`\(was (swarm:[a-z-]+)\)`)

// BlockedFrom is the stage is was blocked from, as the newest of the
// reconciler's notes that moved it to blocked names it (every note but
// the round limit's first, which returns the issue to the architect); ok
// is false when no such note names one of the eight labels but blocked.
func BlockedFrom(is forge.Issue, swarmLogin string) (label string, ok bool) {
	label, _, ok = blockedFrom(is, swarmLogin)
	return label, ok
}

// AtTheRoundLimit: the newest note that blocked the issue is the round
// limit's second, whose `(was swarm:in-review)` names a stage where no
// role runs, both reviewers having reviewed the head (section 6).
func AtTheRoundLimit(is forge.Issue, swarmLogin string) bool {
	_, again, ok := blockedFrom(is, swarmLogin)
	return ok && again
}

func blockedFrom(is forge.Issue, swarmLogin string) (label string, roundLimit, ok bool) {
	thread := IssueThread(is)
	slices.SortStableFunc(thread, func(a, b Entry) int { return a.Created.Compare(b.Created) })
	for i := len(thread) - 1; i >= 0; i-- {
		m, swarm := Swarms(thread[i], swarmLogin)
		text := swarmText(thread[i].Body)
		if !swarm || m.Role != "reconciler" || strings.HasSuffix(text, architectRevises) {
			continue
		}
		all := blockedFromRe.FindAllStringSubmatch(text, -1)
		if all == nil {
			return "", false, false
		}
		was := all[len(all)-1][1]
		if was == "swarm:blocked" || !slices.Contains(forge.Labels, was) {
			return "", false, false
		}
		return was, roundLimitNote.MatchString(text), true
	}
	return "", false, false
}
