// Package reconcile is the swarm's lifecycle over the forge (docs/swarm.md
// sections 5–7): the marker and the threads, the trigger rule, the prompt
// files and Task texts, host-side git (the mirrors, the clones and the
// secret check) and `swarm apply`'s pass (section 11): the five roles'
// launches and collections, the merge step (section 12), the owner's
// batch, the forge milestones, and the counts `status` reads.
package reconcile

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// markerRe is an entry's first line when Daisy posted it (section 6).
var markerRe = regexp.MustCompile(`^<!-- daisy-swarm role=(owner|architect|worker|adversary|sympathetic|reconciler) run=([1-9][0-9]*) -->$`)

// gateMarker and autoMarker are the first lines of the gate note and the
// auto note (section 5): the reconciler's, naming no run, since the notes
// report on none.
const (
	gateMarker = "<!-- daisy-swarm role=reconciler kind=gate -->"
	autoMarker = "<!-- daisy-swarm role=reconciler kind=auto -->"
)

// Mark is what a marker names: the role and the run's number, or for a
// gate note or an auto note the reconciler and Kind, `gate` or `auto`,
// with no run, and for a command's answer Kind `command` and Of, the id
// of the comment it answers.
type Mark struct {
	Role string
	Run  int
	Kind string
	Of   int64
}

// Entry is one entry of a thread: its author's login, its text and when it
// was created or submitted.
type Entry struct {
	Author, Body string
	Created      time.Time
}

// IssueThread is the issue body, then its comments, oldest first.
func IssueThread(is forge.Issue) []Entry {
	t := []Entry{bodyEntry(is)}
	for _, c := range is.Comments {
		t = append(t, Entry{c.Author, c.Body, c.Created})
	}
	return t
}

// bodyEntry is the issue body as an entry of its thread, as every reader
// reads it: without the pulls block (withoutPulls), which no rule reads,
// so it reads as its opener wrote it.
func bodyEntry(is forge.Issue) Entry { return Entry{is.Author, withoutPulls(is.Body), is.Created} }

// Swarms is e's mark when e is the swarm's: its first line, without a
// trailing `\r`, is the marker, and its author is the swarm's login.
func Swarms(e Entry, swarmLogin string) (Mark, bool) {
	if m, ok := Marked(e.Body); ok && strings.EqualFold(e.Author, swarmLogin) {
		return m, true
	}
	return Mark{}, false
}

// Marked is body's mark when its first line, without a trailing `\r`, is
// the marker, or the gate note's, the auto note's or a command's
// answer's, whoever wrote it; a run or a comment id past a 64-bit integer
// is no marker's (section 6).
func Marked(body string) (Mark, bool) {
	first, _, _ := strings.Cut(body, "\n")
	first = strings.TrimSuffix(first, "\r")
	switch first {
	case gateMarker:
		return Mark{Role: "reconciler", Kind: "gate"}, true
	case autoMarker:
		return Mark{Role: "reconciler", Kind: "auto"}, true
	}
	if m := answerRe.FindStringSubmatch(first); m != nil {
		if of, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return Mark{Role: "reconciler", Kind: "command", Of: of}, true
		}
		return Mark{}, false
	}
	if m := markerRe.FindStringSubmatch(first); m != nil {
		if run, err := strconv.Atoi(m[2]); err == nil {
			return Mark{Role: m[1], Run: run}, true
		}
	}
	return Mark{}, false
}

// counted is section 6's Trust: e is the swarm's, or its author is in
// trusted, and it is no gate note, auto note or command's answer, which
// are the maintainer's to read, no class statement (classComment), which
// is Daisy's, and no command (IsCommand), which is the maintainer's to
// Daisy: none is a member's to answer.
func counted(e Entry, swarmLogin string, trusted []string) bool {
	if m, swarm := Swarms(e, swarmLogin); swarm {
		return m.Kind == ""
	}
	return slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, e.Author) }) && classOfText(e.Body) == "" && !IsCommand(e.Body)
}

// NewestCounted is the thread's newest counted entry (counted); ok is
// false when none is counted. Entries created at the same time keep the
// thread's order.
func NewestCounted(thread []Entry, swarmLogin string, trusted []string) (e Entry, ok bool) {
	sorted := slices.Clone(thread)
	slices.SortStableFunc(sorted, func(a, b Entry) int { return a.Created.Compare(b.Created) })
	for i := len(sorted) - 1; i >= 0; i-- {
		if counted(sorted[i], swarmLogin, trusted) {
			return sorted[i], true
		}
	}
	return Entry{}, false
}

// noChange begins the worker's answer that needs no commit (section 7.5).
const noChange = "NO CHANGE:"

// Answers is when the worker answered a pull request without a commit,
// oldest first: thread's entries that are the swarm's, marked
// role=worker, whose text begins NO CHANGE: (section 6). A review counts
// for the head only when it was submitted after the newest.
func Answers(thread []Entry, swarmLogin string) []time.Time {
	var out []time.Time
	for _, e := range thread {
		if m, ok := Swarms(e, swarmLogin); ok && m.Role == "worker" && strings.HasPrefix(swarmText(e.Body), noChange) {
			out = append(out, e.Created)
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

// maintainerWord: e is the maintainer's word to a member (section 6, The
// maintainer's word): a counted entry by a login in trusted other than
// the swarm's (so no class statement and no command) that is no
// `Blocked by #<m>`, which keeps its meaning (rule 1).
func maintainerWord(e Entry, swarmLogin string, trusted []string) bool {
	return !strings.EqualFold(e.Author, swarmLogin) && counted(e, swarmLogin, trusted) && !blockedByRe.MatchString(strings.TrimSpace(e.Body))
}

// Reopenings is when a pull request's head was put back to the pair
// without a commit, oldest first: the worker's answers without a commit
// (Answers) and the maintainer's comments (maintainerWord), thread being
// its comments. A review stands on the head only when submitted after
// the newest (standsOnHead), so each reopening runs both reviewers again.
func Reopenings(thread []Entry, swarmLogin string, trusted []string) []time.Time {
	out := Answers(thread, swarmLogin)
	for _, e := range thread {
		if maintainerWord(e, swarmLogin, trusted) {
			out = append(out, e.Created)
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

// standsOnHead: a review submitted at t came after the newest of reopened
// (Reopenings), or there is none, so it stands on its head as it is now.
func standsOnHead(reopened []time.Time, t time.Time) bool {
	return len(reopened) == 0 || t.After(reopened[len(reopened)-1])
}

// pullComments is a pull request's comments as entries.
func pullComments(cs []forge.Comment) []Entry {
	out := make([]Entry, len(cs))
	for i, c := range cs {
		out[i] = Entry{c.Author, c.Body, c.Created}
	}
	return out
}
