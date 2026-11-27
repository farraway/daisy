package reconcile

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
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
// one stage label: the thread its roles read (the issue's; in
// review-3-changes-requested the current pull request's; in the review
// stages that pull request's comments, where its reopenings are: the
// worker's answers without a commit and the maintainer's comments), in
// the review stages its reviews and head,
// and the roles that hold the issue (rule 2): the role of each container
// running on it (`check` for a worker run's check) and of each run on it
// ended and not collected; on plan-1-accepted, whether rulings changed
// since the plan or the maintainer asked to re-plan (Replan), so the
// architect runs in the worker's place;
// and, on an issue carrying swarm:question, the role that answers it
// (Answerer, section 6, The question), which runs in place of the
// stage's, reading the issue thread.
type Turn struct {
	Label    string
	Thread   []Entry
	Reviews  []forge.Review
	Head     string
	Running  []string
	Replan   bool
	Answerer string
}

// Launch is the roles the trigger rule launches (section 6, rules 1–3):
// the first of the label's roles that has something to answer (on
// plan-1-accepted with Replan, the architect's; with Answerer, that
// role's, at any stage), and in a
// review stage each reviewer that has; none when nothing is to answer. A
// reviewer has something to answer when it has no review of the head
// since the head's newest reopening (Reopenings) in the thread, and the
// sympathetic reviewer only once the adversary's newest such review
// approves (decision
// [163](docs/swarm/decisions/0163-the-sympathetic-reviewer-after-the-adversary-approves.md)).
// Anything running on the issue holds every role, but in a review
// stage the other reviewer's run, which holds only itself. Rule 1's holds
// (Held) and rule 4 (paused, max_members, a free slot) are the caller's.
func Launch(t Turn, swarmLogin string, trusted []string) []string {
	if !reviewing(t.Label) || t.Answerer != "" {
		if len(t.Running) > 0 {
			return nil
		}
		roles := stageRoles[t.Label]
		switch {
		case t.Answerer != "":
			roles = []string{t.Answerer}
		case t.Replan && t.Label == forge.PlanAccepted:
			roles = []string{"architect"}
		}
		e, ok := NewestCounted(t.Thread, swarmLogin, trusted)
		for _, role := range roles {
			if m, swarm := Swarms(e, swarmLogin); !ok || !swarm || !answered(role, m.Role) {
				return []string{role}
			}
		}
		return nil
	}
	verdicts := headVerdicts(t.Reviews, t.Head, Reopenings(t.Thread, swarmLogin, trusted), overruledAt(t.Thread, swarmLogin), swarmLogin)
	var out []string
	for _, role := range stageRoles[t.Label] {
		if slices.ContainsFunc(t.Running, func(r string) bool { return r == role || !slices.Contains(stageRoles[t.Label], r) }) {
			continue
		}
		if _, reviewed := verdicts[role]; reviewed || role == "sympathetic" && verdicts["adversary"] != "approve" {
			continue
		}
		out = append(out, role)
	}
	return out
}

// headVerdicts is the verdict of each swarm reviewer's newest review of
// head standing since the head's newest reopening (answers), by role, an
// adversary's request the architect overruled at over standing as its
// approval would (standing).
func headVerdicts(reviews []forge.Review, head string, answers []time.Time, over time.Time, swarmLogin string) map[string]string {
	verdicts, at := map[string]string{}, map[string]time.Time{}
	for _, rv := range reviews {
		m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, swarmLogin)
		if ok && isReviewer(m.Role) && rv.Commit == head && standsOnHead(answers, rv.Submitted) && !rv.Submitted.Before(at[m.Role]) {
			verdicts[m.Role], at[m.Role] = standing(m.Role, verdictOf(swarmText(rv.Body)), rv.Submitted, over), rv.Submitted
		}
	}
	return verdicts
}

// overruledAt is when the architect overruled the blocking findings on a
// pull request at the round limit (section 7.11, rule 8; T33): its
// newest comment there, thread, beginning OVERRULED:; zero for none.
func overruledAt(thread []Entry, swarmLogin string) time.Time {
	var at time.Time
	for _, e := range thread {
		if m, ok := Swarms(e, swarmLogin); ok && m.Role == "architect" && strings.HasPrefix(swarmText(e.Body), overruled) && e.Created.After(at) {
			at = e.Created
		}
	}
	return at
}

// standing is a role's review verdict as it stands: an adversary's
// request for changes submitted at or before over, the architect's
// overruling (overruledAt), stands as its approval would (section 7.11,
// rule 8).
func standing(role, verdict string, submitted, over time.Time) string {
	if role == "adversary" && verdict == "request-changes" && !over.IsZero() && !submitted.After(over) {
		return "approve"
	}
	return verdict
}

// answered: an entry of the swarm's marked by role by leaves role nothing
// to answer (section 6, rule 3): role's own, or for the owner the
// QA's proposal, which stands at proposal-posted as the owner's
// would.
func answered(role, by string) bool {
	return by == role || role == "owner" && by == declare.QARole
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

// BlockedBy is the issues that the maintainer's newest comments on is
// name as its blockers, `Blocked by #<m>` (section 6, rule 1; section
// 12.1, rule 1): every such comment among the comments by a login in
// trusted other than the swarm's since the newest of them that is not
// one, a class statement, a command and a declaration (`Depends:`) aside,
// oldest first, each issue once; none when the newest is no `Blocked by`.
// So `swarm block` run twice in a row holds by both, and any other
// comment of the maintainer's lifts them all. Whether each is open is the
// caller's to read.
func BlockedBy(is forge.Issue, swarmLogin string, trusted []string) []int {
	var mine []forge.Comment
	for _, c := range is.Comments {
		if !trustedOther(c.Author, swarmLogin, trusted) || classOfText(c.Body) != "" || IsCommand(c.Body) || isDeclaration(c.Body) {
			continue
		}
		mine = append(mine, c)
	}
	slices.SortStableFunc(mine, func(a, b forge.Comment) int { return a.Created.Compare(b.Created) })
	var out []int
	for i := len(mine) - 1; i >= 0; i-- {
		m := blockedByRe.FindStringSubmatch(strings.TrimSpace(mine[i].Body))
		if m == nil {
			break
		}
		n, _ := strconv.Atoi(m[1])
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Reverse(out)
	return out
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

// Lifting is the maintainer's comment that lifts the block of issue is,
// carrying swarm:blocked (section 6, The maintainer's word): the newest
// of words, the threads the caller reads it in, that is the maintainer's
// word (maintainerWord) and was posted after the reconciler's note that
// blocked is (blockingNote); ok is false when none was, or when is holds
// no such note (the flag set by hand). The block's note and every other
// entry of the swarm's lift nothing.
func Lifting(is forge.Issue, words []Entry, swarmLogin string, trusted []string) (Entry, bool) {
	note, ok := blockingNote(is, swarmLogin)
	if !ok {
		return Entry{}, false
	}
	var newest *Entry
	for i, w := range words {
		if w.Created.After(note.Created) && maintainerWord(w, swarmLogin, trusted) && (newest == nil || !w.Created.Before(newest.Created)) {
			newest = &words[i]
		}
	}
	if newest == nil {
		return Entry{}, false
	}
	return *newest, true
}
