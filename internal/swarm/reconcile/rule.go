package reconcile

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/swarm/forge"
)

// stageRoles is each stage's roles in the order they are taken (section
// 5); plan-2-in-progress (a worker runs) and review-2-approved run nobody.
var stageRoles = map[string][]string{
	forge.ProposalPosted:   {"owner"},
	forge.ProposalAccepted: {"architect"},
	forge.PlanPosted:       {"architect"},
	forge.PlanAccepted:     {"worker"},
	forge.ReviewReady:      {"adversary", "sympathetic"},
	forge.ReviewInProgress: {"adversary", "sympathetic"},
	forge.ChangesRequested: {"worker"},
}

// reviewing: the stage's roles are the reviewers, who read the current
// pull request's reviews of its head.
func reviewing(stage string) bool {
	return stage == forge.ReviewReady || stage == forge.ReviewInProgress
}

// onPull: the stage's role reads the current pull request's thread.
func onPull(stage string) bool { return reviewing(stage) || stage == forge.ChangesRequested }

// InProgress is the stage a run of role launched on stage writes once its
// container has started (section 5): plan-2-in-progress for every worker
// run, review-1-in-progress for a reviewer launched on review-0-ready;
// "" when the launch writes none.
func InProgress(role, stage string) string {
	switch {
	case role == "worker" && (stage == forge.PlanAccepted || stage == forge.ChangesRequested):
		return forge.PlanInProgress
	case (role == "adversary" || role == "sympathetic") && stage == forge.ReviewReady:
		return forge.ReviewInProgress
	}
	return ""
}

// Turn is what the trigger rule reads for one open issue carrying exactly
// one stage label: the thread its roles read (the issue's, or in the
// review stages and review-3-changes-requested the current pull
// request's), in the review stages that pull request's reviews and head,
// and the roles that hold the issue (rule 2): the role of each container
// running on it (`check` for a worker run's check) and of each run on it
// ended and not collected.
type Turn struct {
	Label   string
	Thread  []Entry
	Reviews []forge.Review
	Head    string
	Running []string
}

// Launch is the roles the trigger rule launches (section 6, rules 1–3):
// the first of the label's roles that has something to answer, and in a
// review stage each reviewer that has (the pair launches in one pass,
// blind to each other on the head); none when nothing is to answer. A
// reviewer has something to answer when it has no review of the head
// since the worker's newest answer without a commit (Answers) in the
// thread. Anything running on the issue holds every role, but in a review
// stage the other reviewer's run, which holds only itself. Rule 1's holds
// (Held) and rule 4 (paused, max_members, a free slot) are the caller's.
func Launch(t Turn, swarmLogin string, trusted []string) []string {
	if !reviewing(t.Label) {
		if len(t.Running) > 0 {
			return nil
		}
		e, ok := NewestCounted(t.Thread, swarmLogin, trusted)
		for _, role := range stageRoles[t.Label] {
			if m, swarm := Swarms(e, swarmLogin); !ok || !swarm || m.Role != role {
				return []string{role}
			}
		}
		return nil
	}
	answers := Answers(t.Thread, swarmLogin)
	var out []string
	for _, role := range stageRoles[t.Label] {
		if slices.ContainsFunc(t.Running, func(r string) bool { return r == role || !slices.Contains(stageRoles[t.Label], r) }) {
			continue
		}
		reviewed := false
		for _, r := range t.Reviews {
			m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
			reviewed = reviewed || (ok && m.Role == role && r.Commit == t.Head && sinceAnswer(answers, r.Submitted))
		}
		if !reviewed {
			out = append(out, role)
		}
	}
	return out
}

// Held is the flag that holds an issue whatever its stage (section 6,
// rule 1): swarm:blocked, else swarm:paused; "" when it carries neither.
func Held(labels []string) string {
	for _, f := range []string{forge.Blocked, forge.Paused} {
		if slices.Contains(labels, f) {
			return f
		}
	}
	return ""
}

// blockedByRe is the maintainer's comment `swarm block` posts on the
// blocked issue (section 11).
var blockedByRe = regexp.MustCompile(`^Blocked by #([1-9][0-9]*)$`)

// BlockedBy is the issue the newest comment on is by a login in trusted
// other than the swarm's, a class statement aside, names as its blocker,
// `Blocked by #<m>` (section 6, rule 1); 0 when that comment names none.
// Whether #m is open is the caller's to read.
func BlockedBy(is forge.Issue, swarmLogin string, trusted []string) int {
	var newest *forge.Comment
	for i, c := range is.Comments {
		if strings.EqualFold(c.Author, swarmLogin) || !slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, c.Author) }) || classOfText(c.Body) != "" {
			continue
		}
		if newest == nil || !c.Created.Before(newest.Created) {
			newest = &is.Comments[i]
		}
	}
	if newest == nil {
		return 0
	}
	m := blockedByRe.FindStringSubmatch(strings.TrimSpace(newest.Body))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Blocker is whether issue n's `Blocked by #<m>` holds it (section 6,
// rule 1): #m is open. A #m the forge does not hold (forge.ErrNotFound)
// holds nothing, with a warning on warn, so one mistyped number does not
// stop the pass.
func Blocker(f forge.Adapter, n, m int, warn io.Writer) (bool, error) {
	blocker, err := f.Issue(m)
	if errors.Is(err, forge.ErrNotFound) {
		fmt.Fprintf(warn, "⚠ #%d: Blocked by #%d names no issue; not held\n", n, m)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return blocker.Open, nil
}

// marker is the two lines every swarm entry begins with (section 6).
func marker(role string, k int) string {
	return "<!-- daisy-swarm role=" + role + " run=" + itoa(k) + " -->\n**" + role + ":**\n\n"
}

func itoa(n int) string { return strconv.Itoa(n) }
