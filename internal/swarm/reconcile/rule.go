package reconcile

import (
	"strconv"

	"daisy/internal/swarm/forge"
)

// stageRoles is each stage's roles in the order they are taken (section
// 5); approved and blocked run nobody.
var stageRoles = map[string][]string{
	"swarm:proposed":  {"owner"},
	"swarm:ready":     {"architect"},
	"swarm:planning":  {"architect"},
	"swarm:planned":   {"worker"},
	"swarm:in-review": {"adversary", "sympathetic"},
	"swarm:reviewed":  {"worker"},
}

// Turn is what the trigger rule reads for one open issue carrying exactly
// one stage label: the thread its roles read (the issue's, or in
// in-review and reviewed the current pull request's), in in-review that
// pull request's reviews and head, and whether anything runs on the issue
// (rule 2: a container, or a run ended and not collected).
type Turn struct {
	Label   string
	Thread  []Entry
	Reviews []forge.Review
	Head    string
	Running bool
}

// Launch is the role the trigger rule launches (section 6, rules 1–3):
// the first of the label's roles that has something to answer, "" for
// none. Rule 4 (paused, max_members, a free slot) is the caller's.
func Launch(t Turn, swarmLogin string, trusted []string) string {
	if t.Running {
		return ""
	}
	for _, role := range stageRoles[t.Label] {
		if t.Label == "swarm:in-review" {
			reviewed := false
			for _, r := range t.Reviews {
				m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
				reviewed = reviewed || (ok && m.Role == role && r.Commit == t.Head)
			}
			if !reviewed {
				return role
			}
			continue
		}
		e, ok := NewestCounted(t.Thread, swarmLogin, trusted)
		if m, swarm := Swarms(e, swarmLogin); !ok || !swarm || m.Role != role {
			return role
		}
	}
	return ""
}

// marker is the two lines every swarm entry begins with (section 6).
func marker(role string, k int) string {
	return "<!-- daisy-swarm role=" + role + " run=" + itoa(k) + " -->\n**" + role + ":**\n\n"
}

func itoa(n int) string { return strconv.Itoa(n) }
