package reconcile

import (
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

func TestSwarmsReadsTheMarkerAndTheAuthor(t *testing.T) {
	for _, tc := range []struct {
		name, author, body string
		want               Mark
		ok                 bool
	}{
		{"the architect's", "swarm-bot", "<!-- daisy-swarm role=architect run=2 -->\n**architect:**\n\nplan", Mark{"architect", 2}, true},
		{"a CRLF first line", "Swarm-Bot", "<!-- daisy-swarm role=reconciler run=12 -->\r\n**reconciler:**", Mark{"reconciler", 12}, true},
		{"the marker by a maintainer's login", "jane", "<!-- daisy-swarm role=architect run=2 -->\nplan", Mark{}, false},
		{"run 0", "swarm-bot", "<!-- daisy-swarm role=owner run=0 -->", Mark{}, false},
		{"a leading zero", "swarm-bot", "<!-- daisy-swarm role=owner run=01 -->", Mark{}, false},
		{"an unknown role", "swarm-bot", "<!-- daisy-swarm role=maintainer run=1 -->", Mark{}, false},
		{"text before the marker", "swarm-bot", "x <!-- daisy-swarm role=owner run=1 -->", Mark{}, false},
		{"the marker on the second line", "swarm-bot", "hello\n<!-- daisy-swarm role=owner run=1 -->", Mark{}, false},
	} {
		got, ok := Swarms(Entry{tc.author, tc.body, time.Time{}}, "swarm-bot")
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: Swarms = %+v, %v", tc.name, got, ok)
		}
	}
}

func TestNewestCounted(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	is := forge.Issue{Author: "jane", Body: "please", Created: t0, Comments: []forge.Comment{
		{Author: "swarm-bot", Body: "<!-- daisy-swarm role=architect run=1 -->\nplan", Created: t0.Add(time.Hour)},
		{Author: "mallory", Body: "<!-- daisy-swarm role=owner run=1 -->\nnot trusted", Created: t0.Add(2 * time.Hour)},
		{Author: "swarm-bot", Body: "unmarked, by the swarm's login", Created: t0.Add(3 * time.Hour)},
	}}
	trusted := []string{"Jane", "swarm-bot"}
	e, ok := NewestCounted(IssueThread(is), "swarm-bot", trusted)
	if !ok || e.Body != "unmarked, by the swarm's login" {
		t.Errorf("newest = %+v, %v: swarm-bot is trusted", e, ok)
	}
	e, ok = NewestCounted(IssueThread(is), "swarm-bot", []string{"jane"})
	if m, swarm := Swarms(e, "swarm-bot"); !ok || !swarm || m.Role != "architect" {
		t.Errorf("newest = %+v: the untrusted entries are passed over", e)
	}
	// The same time: the thread's order decides.
	tie := []Entry{{"jane", "a", t0}, {"swarm-bot", "<!-- daisy-swarm role=architect run=1 -->", t0}}
	if e, _ := NewestCounted(tie, "swarm-bot", nil); e.Author != "swarm-bot" {
		t.Errorf("tie: %+v", e)
	}
	if _, ok := NewestCounted([]Entry{{"mallory", "x", t0}}, "swarm-bot", trusted); ok {
		t.Error("nothing counted, yet an entry")
	}
}
