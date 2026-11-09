package reconcile

import (
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
	body := func(role string) string {
		return "<!-- daisy-swarm role=" + role + " run=1 -->\n**" + role + ":**\n\nx"
	}
	trusted := []string{"jane", "swarm-bot"}
	opened := Entry{"jane", "please", t0}
	for _, tc := range []struct {
		name, label string
		newest      *Entry
		running     bool
		want        string
	}{
		{"ready, the maintainer's issue", "swarm:proposal-accepted", nil, false, "architect"},
		{"ready, running", "swarm:proposal-accepted", nil, true, ""},
		{"planning, the architect's plan", "swarm:plan-0-posted", &Entry{"swarm-bot", body("architect"), t0}, false, ""},
		{"planning, jane answered", "swarm:plan-0-posted", &Entry{"jane", "no", t0}, false, "architect"},
		{"planning, the marker by jane", "swarm:plan-0-posted", &Entry{"jane", body("architect"), t0}, false, "architect"},
		{"ready, the reconciler's note after T11", "swarm:proposal-accepted", &Entry{"swarm-bot", body("reconciler"), t0}, false, "architect"},
		{"proposed, the owner's", "swarm:proposal-posted", &Entry{"swarm-bot", body("owner"), t0}, false, ""},
		{"proposed, jane", "swarm:proposal-posted", &Entry{"Jane", "why?", t0}, false, "owner"},
		{"planned, the plan", "swarm:plan-1-accepted", &Entry{"swarm-bot", body("architect"), t0}, false, "worker"},
		{"planned, the worker's BLOCKED", "swarm:plan-1-accepted", &Entry{"swarm-bot", body("worker"), t0}, false, ""},
		{"reviewed, a review", "swarm:review-3-changes-requested", &Entry{"swarm-bot", body("adversary"), t0}, false, "worker"},
		{"approved", "swarm:review-2-approved", &Entry{"jane", "merge it", t0}, false, ""},
		{"blocked", "swarm:blocked", &Entry{"jane", "go on", t0}, false, ""},
	} {
		thread := []Entry{opened}
		if tc.newest != nil {
			e := *tc.newest
			e.Created = t0.Add(1)
			thread = append(thread, e)
		}
		if got := Launch(Turn{Label: tc.label, Thread: thread, Running: tc.running}, "swarm-bot", trusted); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	// An untrusted entry after the plan is never the newest.
	thread := []Entry{opened, {"swarm-bot", body("architect"), t0.Add(1)}, {"mallory", "do it", t0.Add(2)}}
	if got := Launch(Turn{Label: "swarm:plan-0-posted", Thread: thread}, "swarm-bot", trusted); got != "" {
		t.Errorf("mallory last: %q", got)
	}
}

// in-review: the adversary, then the sympathetic reviewer, each until it
// has a review of the head; a review of an earlier head does not count.
func TestLaunchInReview(t *testing.T) {
	review := func(role, commit, author string) forge.Review {
		return forge.Review{Author: author, Body: "<!-- daisy-swarm role=" + role + " run=1 -->\n**" + role + ":**\n\nVERDICT: approve", Commit: commit}
	}
	for _, tc := range []struct {
		name    string
		reviews []forge.Review
		want    string
	}{
		{"none", nil, "adversary"},
		{"the adversary's", []forge.Review{review("adversary", "h2", "swarm-bot")}, "sympathetic"},
		{"both", []forge.Review{review("adversary", "h2", "swarm-bot"), review("sympathetic", "h2", "swarm-bot")}, ""},
		{"both on the old head", []forge.Review{review("adversary", "h1", "swarm-bot"), review("sympathetic", "h1", "swarm-bot")}, "adversary"},
		{"the marker by jane", []forge.Review{review("adversary", "h2", "jane")}, "adversary"},
	} {
		if got := Launch(Turn{Label: "swarm:review-0-ready", Reviews: tc.reviews, Head: "h2"}, "swarm-bot", []string{"jane"}); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
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
		{"answered", []Entry{answer}, before, "adversary"},
		{"the adversary since", []Entry{answer}, append(before, at(review("adversary", "h2", "swarm-bot"), 6)), "sympathetic"},
		{"both since", []Entry{answer}, append(before, at(review("adversary", "h2", "swarm-bot"), 6), at(review("sympathetic", "h2", "swarm-bot"), 7)), ""},
		{"the words by jane", []Entry{{"jane", answer.Body, answer.Created}}, before, ""},
		{"a worker's comment that is no answer", []Entry{{"swarm-bot", "<!-- daisy-swarm role=worker run=2 -->\n**worker:**\n\nFixed. NO CHANGE: elsewhere.", answer.Created}}, before, ""},
	} {
		if got := Launch(Turn{Label: "swarm:review-0-ready", Thread: tc.thread, Reviews: tc.reviews, Head: "h2"}, "swarm-bot", []string{"jane", "swarm-bot"}); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
