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

// gateMarker is the gate note's first line (section 5): the reconciler's,
// naming no run, since the note reports on none.
const gateMarker = "<!-- daisy-swarm role=reconciler kind=gate -->"

// Mark is what a marker names: the role and the run's number, or for a
// gate note the reconciler and Gate, with no run.
type Mark struct {
	Role string
	Run  int
	Gate bool
}

// Entry is one entry of a thread: its author's login, its text and when it
// was created or submitted.
type Entry struct {
	Author, Body string
	Created      time.Time
}

// IssueThread is the issue body, then its comments, oldest first.
func IssueThread(is forge.Issue) []Entry {
	t := []Entry{{is.Author, is.Body, is.Created}}
	for _, c := range is.Comments {
		t = append(t, Entry{c.Author, c.Body, c.Created})
	}
	return t
}

// Swarms is e's mark when e is the swarm's: its first line, without a
// trailing `\r`, is the marker, and its author is the swarm's login.
func Swarms(e Entry, swarmLogin string) (Mark, bool) {
	if m, ok := Marked(e.Body); ok && strings.EqualFold(e.Author, swarmLogin) {
		return m, true
	}
	return Mark{}, false
}

// Marked is body's mark when its first line, without a trailing `\r`, is
// the marker, or the gate note's, whoever wrote it.
func Marked(body string) (Mark, bool) {
	first, _, _ := strings.Cut(body, "\n")
	first = strings.TrimSuffix(first, "\r")
	if first == gateMarker {
		return Mark{Role: "reconciler", Gate: true}, true
	}
	m := markerRe.FindStringSubmatch(first)
	if m == nil {
		return Mark{}, false
	}
	run, _ := strconv.Atoi(m[2])
	return Mark{Role: m[1], Run: run}, true
}

// counted is section 6's Trust: e is the swarm's, or its author is in
// trusted, and it is no gate note, which is the maintainer's to read and
// no member's to answer.
func counted(e Entry, swarmLogin string, trusted []string) bool {
	if m, swarm := Swarms(e, swarmLogin); swarm {
		return !m.Gate
	}
	return slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, e.Author) })
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

// sinceAnswer: a review submitted at t came after the newest of answers,
// or there is none, so it stands on its head as it is now.
func sinceAnswer(answers []time.Time, t time.Time) bool {
	return len(answers) == 0 || t.After(answers[len(answers)-1])
}

// pullComments is a pull request's comments as entries.
func pullComments(cs []forge.Comment) []Entry {
	out := make([]Entry, len(cs))
	for i, c := range cs {
		out[i] = Entry{c.Author, c.Body, c.Created}
	}
	return out
}
