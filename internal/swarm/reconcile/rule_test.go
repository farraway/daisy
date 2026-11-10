package reconcile

import (
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// The trigger rule as a table over (label, the newest counted entry's
// marker and author, running) (section 6): the human speaking last wakes
// the stage's role, its own word does not, a marker by a maintainer's
// login is not the swarm's, an untrusted entry is never the newest, and
// anything running on the issue launches nothing.
func TestLaunch(t *testing.T) {
	one := func(roles []string) string { return strings.Join(roles, ",") }
	body := func(role string) string {
		return "<!-- daisy-swarm role=" + role + " run=1 -->\n**" + role + ":**\n\nx"
	}
	trusted := []string{"jane", "swarm-bot"}
	opened := Entry{"jane", "please", t0}
	for _, tc := range []struct {
		name, label string
		newest      *Entry
		running     []string
		want        string
	}{
		{"ready, the maintainer's issue", "swarm:proposal-accepted", nil, nil, "architect"},
		{"ready, running", "swarm:proposal-accepted", nil, []string{"architect"}, ""},
		{"planned, a check running", "swarm:plan-1-accepted", nil, []string{"check"}, ""},
		{"planning, the architect's plan", "swarm:plan-0-posted", &Entry{"swarm-bot", body("architect"), t0}, nil, ""},
		{"planning, jane answered", "swarm:plan-0-posted", &Entry{"jane", "no", t0}, nil, "architect"},
		{"planning, the marker by jane", "swarm:plan-0-posted", &Entry{"jane", body("architect"), t0}, nil, "architect"},
		{"ready, the reconciler's note after T11", "swarm:proposal-accepted", &Entry{"swarm-bot", body("reconciler"), t0}, nil, "architect"},
		{"proposed, the owner's", "swarm:proposal-posted", &Entry{"swarm-bot", body("owner"), t0}, nil, ""},
		{"proposed, jane", "swarm:proposal-posted", &Entry{"Jane", "why?", t0}, nil, "owner"},
		{"planned, the plan", "swarm:plan-1-accepted", &Entry{"swarm-bot", body("architect"), t0}, nil, "worker"},
		{"planned, the worker's BLOCKED", "swarm:plan-1-accepted", &Entry{"swarm-bot", body("worker"), t0}, nil, ""},
		{"reviewed, a review", "swarm:review-3-changes-requested", &Entry{"swarm-bot", body("adversary"), t0}, nil, "worker"},
		{"approved", "swarm:review-2-approved", &Entry{"jane", "merge it", t0}, nil, ""},
		{"blocked", "swarm:blocked", &Entry{"jane", "go on", t0}, nil, ""},
	} {
		thread := []Entry{opened}
		if tc.newest != nil {
			e := *tc.newest
			e.Created = t0.Add(1)
			thread = append(thread, e)
		}
		if got := one(Launch(Turn{Label: tc.label, Thread: thread, Running: tc.running}, "swarm-bot", trusted)); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	// An untrusted entry after the plan is never the newest.
	thread := []Entry{opened, {"swarm-bot", body("architect"), t0.Add(1)}, {"mallory", "do it", t0.Add(2)}}
	if got := Launch(Turn{Label: "swarm:plan-0-posted", Thread: thread}, "swarm-bot", trusted); got != nil {
		t.Errorf("mallory last: %q", got)
	}
}

// in-review: each reviewer until it has a review of the head, the pair
// together in one pass; a review of an earlier head does not count; a
// reviewer's own run holds it, the other's does not, and any other run on
// the issue holds both.
func TestLaunchInReview(t *testing.T) {
	review := func(role, commit, author string) forge.Review {
		return forge.Review{Author: author, Body: "<!-- daisy-swarm role=" + role + " run=1 -->\n**" + role + ":**\n\nVERDICT: approve", Commit: commit}
	}
	for _, tc := range []struct {
		name    string
		reviews []forge.Review
		running []string
		want    string
	}{
		{"none", nil, nil, "adversary,sympathetic"},
		{"the adversary's", []forge.Review{review("adversary", "h2", "swarm-bot")}, nil, "sympathetic"},
		{"both", []forge.Review{review("adversary", "h2", "swarm-bot"), review("sympathetic", "h2", "swarm-bot")}, nil, ""},
		{"both on the old head", []forge.Review{review("adversary", "h1", "swarm-bot"), review("sympathetic", "h1", "swarm-bot")}, nil, "adversary,sympathetic"},
		{"the marker by jane", []forge.Review{review("adversary", "h2", "jane"), review("sympathetic", "h2", "swarm-bot")}, nil, "adversary"},
		{"the adversary running", nil, []string{"adversary"}, "sympathetic"},
		{"both running", nil, []string{"sympathetic", "adversary"}, ""},
		{"a worker's run uncollected", nil, []string{"worker"}, ""},
	} {
		for _, label := range []string{"swarm:review-0-ready", "swarm:review-1-in-progress"} {
			if got := strings.Join(Launch(Turn{Label: label, Reviews: tc.reviews, Head: "h2", Running: tc.running}, "swarm-bot", []string{"jane"}), ","); got != tc.want {
				t.Errorf("%s at %s: %q, want %q", tc.name, label, got, tc.want)
			}
		}
	}
	// The worker's answer without a commit (NO CHANGE:) on the pull
	// request: the reviews of the head before it no longer count.
	t1 := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	at := func(r forge.Review, m int) forge.Review {
		r.Submitted = t1.Add(time.Duration(m) * time.Minute)
		return r
	}
	answer := Entry{"swarm-bot", "<!-- daisy-swarm role=worker run=2 -->\n**worker:**\n\nNO CHANGE: it is right.", t1.Add(5 * time.Minute)}
	before := []forge.Review{at(review("adversary", "h2", "swarm-bot"), 1), at(review("sympathetic", "h2", "swarm-bot"), 2)}
	for _, tc := range []struct {
		name    string
		thread  []Entry
		reviews []forge.Review
		want    string
	}{
		{"answered", []Entry{answer}, before, "adversary,sympathetic"},
		{"the adversary since", []Entry{answer}, append(before, at(review("adversary", "h2", "swarm-bot"), 6)), "sympathetic"},
		{"both since", []Entry{answer}, append(before, at(review("adversary", "h2", "swarm-bot"), 6), at(review("sympathetic", "h2", "swarm-bot"), 7)), ""},
		{"the words by jane", []Entry{{"jane", answer.Body, answer.Created}}, before, ""},
		{"a worker's comment that is no answer", []Entry{{"swarm-bot", "<!-- daisy-swarm role=worker run=2 -->\n**worker:**\n\nFixed. NO CHANGE: elsewhere.", answer.Created}}, before, ""},
	} {
		if got := strings.Join(Launch(Turn{Label: "swarm:review-0-ready", Thread: tc.thread, Reviews: tc.reviews, Head: "h2"}, "swarm-bot", []string{"jane", "swarm-bot"}), ","); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// SentBack (T25's second time, section 12): a direction on the head, the
// swarm's marked note or a trusted login's comment, before the thread's
// last comment; not one that is the last (a retry), an untrusted
// login's, or one on another head.
func TestSentBack(t *testing.T) {
	p := forge.Pull{Repo: "app", Number: 2, Head: "swarm/1-1"}
	const head, other = "c1abcdef99", "d2abcdef99"
	dir := func(h string) string { return Direction("acme", "main", 1, p, h, []string{"main.go"}) }
	swarm := func(body string) forge.Comment {
		return forge.Comment{Author: "swarm-bot", Body: marker("reconciler", 1) + body}
	}
	answer := forge.Comment{Author: "swarm-bot", Body: marker("worker", 2) + "NO CHANGE: merged."}
	for _, tc := range []struct {
		name     string
		comments []forge.Comment
		want     bool
	}{
		{"none", nil, false},
		{"the swarm's, last: a retry", []forge.Comment{swarm(dir(head))}, false},
		{"the swarm's, answered", []forge.Comment{swarm(dir(head)), answer}, true},
		{"a trusted login's, answered", []forge.Comment{{Author: "Jane", Body: Direction("acme", "main", 1, p, head, nil)}, answer}, true},
		{"an untrusted login's", []forge.Comment{{Author: "mallory", Body: dir(head)}, answer}, false},
		{"on another head", []forge.Comment{swarm(dir(other)), answer}, false},
	} {
		if got := SentBack(tc.comments, "acme", p, head, "swarm-bot", []string{"jane", "swarm-bot"}); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
