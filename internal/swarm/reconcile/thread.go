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

// Mark is what a marker names: the role and the run's number.
type Mark struct {
	Role string
	Run  int
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
// the marker, whoever wrote it.
func Marked(body string) (Mark, bool) {
	first, _, _ := strings.Cut(body, "\n")
	m := markerRe.FindStringSubmatch(strings.TrimSuffix(first, "\r"))
	if m == nil {
		return Mark{}, false
	}
	run, _ := strconv.Atoi(m[2])
	return Mark{m[1], run}, true
}

// NewestCounted is the thread's newest counted entry: the swarm's, or one
// whose author is in trusted (section 6, Trust); ok is false when none is
// counted. Entries created at the same time keep the thread's order.
func NewestCounted(thread []Entry, swarmLogin string, trusted []string) (e Entry, ok bool) {
	sorted := slices.Clone(thread)
	slices.SortStableFunc(sorted, func(a, b Entry) int { return a.Created.Compare(b.Created) })
	for i := len(sorted) - 1; i >= 0; i-- {
		e := sorted[i]
		if _, swarm := Swarms(e, swarmLogin); swarm || slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, e.Author) }) {
			return e, true
		}
	}
	return Entry{}, false
}
