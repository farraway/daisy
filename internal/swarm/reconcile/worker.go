package reconcile

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// fetch is a worker run's step 1 (section 7.6): its branch fetched from
// its clone into the mirror as the run's ref, then each new commit
// without the role's trailer reported (The trailer): on stderr and as
// `unstamped <sha>`, never a failure; an empty co_author asks for none.
// A fetch that fails is recorded (`fetch failed: <error>`) and is the
// pass's error, retried at the next pass; the second in a row is a
// failure of the work (section 7.7), `failed: fetch: <error>`, so the
// issue is blocked with the note naming it and the run is collected.
func (c *collection) fetch() error {
	err := c.fetchRef()
	if errors.Is(err, ErrForeignClone) {
		// A work failure, read by the collection as any other: nothing
		// is fetched from the clone, so nothing of it is pushed.
		if _, failed := Has(c.lines, "failed"); failed {
			return nil
		}
		return c.step("failed: " + err.Error())
	}
	if _, fetched := Has(c.lines, "fetched"); err == nil || fetched {
		return err // fetched, or an error past the fetch (the trailer's log)
	}
	reason := c.fetchReason(err)
	if err := c.step("fetch failed: " + reason); err != nil {
		return err
	}
	if len(fetchFailures(c.lines)) < 2 {
		return err
	}
	if _, failed := Has(c.lines, "failed"); failed {
		return nil // the run's own failure; its note names the fetch's too (keptNote)
	}
	return c.step("failed: fetch: " + reason)
}

// fetchFailures is the reasons of the run's failed fetches, oldest first.
func fetchFailures(lines []string) []string {
	var out []string
	for _, l := range lines {
		if r, ok := strings.CutPrefix(l, "fetch failed: "); ok {
			out = append(out, r)
		}
	}
	return out
}

// fetchReason is a failed fetch's error as the result and the note hold
// it: on one line, the swarm's state directory written <state> (the
// thread is read by members, which never see the host's paths, section
// 8), and withheld when it holds a secret the member was given (git's
// message may quote the clone's own text, which no secret check read).
func (c *collection) fetchReason(err error) string {
	reason := oneLine.Replace(strings.ReplaceAll(err.Error(), c.s.state(), "<state>"))
	if hit([]byte(err.Error()), c.s.secretsOf(c.r, c.lines)) {
		return "git's message withheld: it holds a secret the member was given"
	}
	return reason
}

// fetchRef is fetch but for its failure: a clone that reaches outside
// itself is ErrForeignClone, nothing fetched or recorded.
func (c *collection) fetchRef() error {
	if _, ok := Has(c.lines, "fetched"); ok {
		return nil
	}
	sha, err := c.s.git().FetchRun(branchRepo(c.lines), filepath.Join(c.r.Dir, "work"), runBranch(c.lines), runRef(c.r))
	if err != nil {
		return err
	}
	if err := c.step("fetched " + sha); err != nil {
		return err
	}
	coAuthor := c.s.Cfg.Personas["worker"].CoAuthor
	if !c.commits() || coAuthor == "" {
		return nil
	}
	bad, err := c.s.git().Unstamped(branchRepo(c.lines), c.s.Cfg.Base, sha, coAuthor)
	if err != nil {
		return err
	}
	for _, h := range bad {
		fmt.Fprintf(c.s.Err, "⚠ #%d worker run %d: commit %s has no Co-Authored-By: %s\n", c.r.N, c.r.K, short(h), coAuthor)
		if err := c.step("unstamped " + h); err != nil {
			return err
		}
	}
	return nil
}

func short(sha string) string { return sha[:min(7, len(sha))] }

// fetched is the worker run's fetched commit.
func (c *collection) fetched() string {
	l, _ := Has(c.lines, "fetched")
	return strings.TrimPrefix(l, "fetched ")
}

// commits: the fetched commit is past the run's start.
func (c *collection) commits() bool {
	start, _ := Has(c.lines, "start")
	return c.fetched() != "" && "start "+c.fetched() != start
}

// keep is step 2: a worker run that does not push keeps its commits as
// the run's ref, `kept <ref>`; one with none drops the ref.
func (c *collection) keep() error {
	if _, ok := Has(c.lines, "fetched"); !ok {
		return nil
	}
	if !c.commits() {
		return c.s.git().DropRef(branchRepo(c.lines), runRef(c.r))
	}
	if _, ok := Has(c.lines, "kept"); ok {
		return nil
	}
	return c.step("kept " + runRef(c.r))
}

// worker is a worker run's steps after its output was read (section 7.6):
// no commits, which on review-3-changes-requested with a NO CHANGE: answer
// is that answer posted on the pull request and T12; the secret check of
// every object the push would send, then protected paths; the check,
// started and collected when it ends, its posted tail checked for
// secrets; the push; then the pull request opened or commented on, and T8
// or T12.
func (c *collection) worker(out, text string, pull *forge.Pull) error {
	s := c.s
	if !c.commits() {
		if c.label != forge.ChangesRequested || !strings.HasPrefix(text, noChange) {
			return c.fail("no commits")
		}
		if err := c.keep(); err != nil { // the run's ref dropped
			return err
		}
		if err := c.onPull(*pull, text); err != nil {
			return err
		}
		if err := c.relabel(forge.ReviewReady); err != nil {
			return err
		}
		return c.end("posted")
	}
	name, branch, sha := branchRepo(c.lines), runBranch(c.lines), c.fetched()
	repo, _ := s.repo(name)
	secrets := s.secretsOf(c.r, c.lines)
	_, pushed := Has(c.lines, "pushed")
	if !pushed {
		// The secret check first: the protected-path note names paths,
		// and a path is the member's text too.
		objects, err := s.git().Objects(name, s.Cfg.Base, sha)
		if err != nil {
			return err
		}
		if hit(objects, secrets) {
			return c.fail("secret in output: commits")
		}
		changed, err := s.git().Changed(name, s.Cfg.Base, sha)
		if err != nil {
			return err
		}
		if hits := Protected(changed, repo.Protected); len(hits) > 0 {
			return c.fail("protected path changed: " + strings.Join(hits, ", "))
		}
	}
	block := "**Check:** none declared."
	if len(repo.Check) > 0 {
		if _, started := Has(c.lines, "check started"); !started {
			if err := s.startCheck(c.r, c.lines); err != nil {
				return err
			}
			if s.live[c.r.Dir] == nil {
				return s.collect(c.r) // the check could not start: a fault
			}
			return nil
		}
		result, tail, _ := checkResult(s.O.FS(), c.r, c.lines, s.Cfg.Timeout)
		if hit([]byte(strings.Join(tail, "\n")), secrets) {
			return c.fail("secret in output: check.out")
		}
		block = fmt.Sprintf("**Check:** `%s` on %s: %s\n\n```\n%s\n```", shellQuote(repo.Check), short(sha), result, strings.Join(tail, "\n"))
	}
	if !pushed {
		err := s.git().Push(repo, runRef(c.r), branch, sha)
		if errors.Is(err, ErrPushRejected) {
			return c.fail("push rejected")
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "%spushed %s %s\n", c.head, branch, short(sha))
		if err := c.step("pushed " + sha); err != nil {
			return err
		}
	}
	// One blank line between the text and the block, and the text cut
	// so that the two together hold to maxPost (section 7.5).
	sep := "\n\n"
	posted := strings.TrimRight(postable(out, c.r, len(sep)+len(block)), "\n") + sep + block
	if c.label == forge.ChangesRequested || pull != nil && pull.Open {
		if err := c.onPull(*pull, posted); err != nil {
			return err
		}
	} else {
		issue := fmt.Sprintf("#%d", c.r.N)
		if name != repoName(s.Cfg.Forge.Repo) {
			issue = s.Cfg.Forge.Repo + issue
		}
		pulls, err := s.pulls(c.r.N)
		if err != nil {
			return err
		}
		mark := marker("worker", c.r.K)
		created := false
		for _, p := range pulls {
			created = created || p.Open && p.Head == branch && strings.HasPrefix(p.Body, mark) && strings.EqualFold(p.Author, s.SwarmLogin)
		}
		if !created {
			sl := strings.TrimPrefix(branch, fmt.Sprintf("swarm/%d-", c.r.N))
			m, err := s.Forge.CreatePull(name, branch, s.Cfg.Base, fmt.Sprintf("%s slice %s: %s", issue, sl, c.is.Title), mark+posted+"\n\nRefs "+issue)
			if err != nil {
				return err
			}
			fmt.Fprintf(s.Out, "%screated pull %s#%d\n", c.head, name, m)
			if err := c.step(fmt.Sprintf("created pull %d", m)); err != nil {
				return err
			}
		}
	}
	if err := c.relabel(forge.ReviewReady); err != nil {
		return err
	}
	return c.end("posted")
}

// onPull posts the worker's text as a comment on its open pull request,
// unless it is the one this run opened.
func (c *collection) onPull(p forge.Pull, posted string) error {
	if strings.HasPrefix(p.Body, marker("worker", c.r.K)) && strings.EqualFold(p.Author, c.s.SwarmLogin) {
		return nil
	}
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	existing, err := c.s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	return c.say(ref, existing, "worker", posted)
}

// review is a reviewer run's steps (section 7.6): its verdict read and
// checked against its findings' severity (7.5), the review posted, and
// once the pair on the head is complete, T9, T10 or T11, from the stage
// the run is at (review-1-in-progress, or the stage it was launched on).
func (c *collection) review(out, text string, pull forge.Pull) error {
	s := c.s
	verdict := verdictOf(out)
	blocking := len(blockingLines(out)) > 0
	if verdict == "comment" || (verdict == "request-changes") != blocking {
		return c.fail("bad verdict")
	}
	ref := forge.Ref{Repo: pull.Repo, N: pull.Number}
	reviews, err := s.Forge.Reviews(ref)
	if err != nil {
		return err
	}
	comments, err := s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	answers := Answers(pullComments(comments), s.SwarmLogin)
	posted := false
	for _, rv := range reviews {
		m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		posted = posted || ok && m.Role == c.r.Role && m.Run == c.r.K
	}
	if !posted {
		v := forge.Approve
		if verdict == "request-changes" {
			v = forge.RequestChanges
		}
		if err := s.Forge.Review(ref, v, marker(c.r.Role, c.r.K)+text); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "%sposted review (%s)\n", c.head, verdict)
		if err := c.step("posted review"); err != nil {
			return err
		}
		if reviews, err = s.Forge.Reviews(ref); err != nil {
			return err
		}
	}
	pair := map[string]string{}
	at := map[string]time.Time{}
	for _, rv := range reviews {
		m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		if ok && rv.Commit == pull.HeadSHA && sinceAnswer(answers, rv.Submitted) && (m.Role == "adversary" || m.Role == "sympathetic") && !rv.Submitted.Before(at[m.Role]) {
			pair[m.Role], at[m.Role] = verdictOf(swarmText(rv.Body)), rv.Submitted
		}
	}
	if len(pair) < 2 {
		return c.end("posted")
	}
	if pair["adversary"] == "approve" && pair["sympathetic"] == "approve" {
		if err := c.relabel(forge.ReviewApproved); err != nil {
			return err
		}
		return c.end("posted")
	}
	plan, _, hasPlan := (&Job{SwarmLogin: s.SwarmLogin, Issue: c.is}).Plan()
	var since time.Time
	if hasPlan {
		since = plan.Created
	}
	rounds := Rounds(reviews, answers, s.SwarmLogin, since, "")
	if rounds < s.Cfg.MaxRounds {
		if err := c.relabel(forge.ChangesRequested); err != nil {
			return err
		}
		return c.end("posted")
	}
	on := fmt.Sprintf("%s#%d", pull.Repo, pull.Number)
	first := fmt.Sprintf("Review round %d of %d on %s requested changes (was %s). "+architectRevises, rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress)
	second := fmt.Sprintf("Review round %d of %d on %s "+roundLimitAgain+" (at %s). Relabel the issue %s for one more fix round or %s to re-plan, then remove %s.",
		rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress, forge.ChangesRequested, forge.PlanPosted, forge.Blocked)
	own := []string{marker("reconciler", c.r.K) + first, marker("reconciler", c.r.K) + second}
	again := false
	for _, cm := range c.is.Comments {
		m, ok := Swarms(Entry{cm.Author, cm.Body, cm.Created}, s.SwarmLogin)
		t := swarmText(cm.Body)
		again = again || ok && m.Role == "reconciler" && strings.HasPrefix(t, "Review round ") &&
			strings.Contains(t, " on "+on+" requested changes") && cm.Body != own[0] && cm.Body != own[1]
	}
	if again {
		// The second time, the stage stays and the issue is blocked: the
		// maintainer's (G4); unless the maintainer moved it during the run.
		if now, moved, err := c.movedAway(false); err != nil {
			return err
		} else if moved {
			second = fmt.Sprintf("Review round %d of %d on %s "+roundLimitAgain+" (at %s). %s", rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress, notBlocked(now))
		}
		if err := c.comment("reconciler", second); err != nil {
			return err
		}
		if err := c.flag(false); err != nil {
			return err
		}
		return c.end("posted")
	}
	to := forge.PlanPosted
	if !s.Cfg.Gates.Plan {
		to = forge.ProposalAccepted
	}
	if err := c.comment("reconciler", first); err != nil {
		return err
	}
	if err := c.relabel(to); err != nil {
		return err
	}
	return c.end("posted")
}

// architectRevises ends the round limit's first note (T11), the one
// reconciler note that does not block the issue.
const architectRevises = "The architect revises the plan from the reviews."

// roundLimitAgain is the round limit's second note's words: the reviewers
// have both reviewed the head, so review-1-in-progress runs no one
// (section 6).
const roundLimitAgain = "requested changes a second time"

// Rounds is a pull request's review rounds (section 6): the heads with a
// review marked role=adversary and one marked role=sympathetic, a head
// counted once more for each of the worker's answers without a commit
// (Answers, answers) after which both reviewed it again; counting only
// reviews submitted after since (all of them when since is zero), and not
// except's since the newest answer, the head as it stands.
func Rounds(reviews []forge.Review, answers []time.Time, swarmLogin string, since time.Time, except string) int {
	by := map[roundKey]map[string]bool{}
	for _, r := range reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
		key := keyOf(r, answers)
		if !ok || !since.IsZero() && !r.Submitted.After(since) || r.Commit == except && key.answers == len(answers) {
			continue
		}
		if by[key] == nil {
			by[key] = map[string]bool{}
		}
		by[key][m.Role] = true
	}
	n := 0
	for _, roles := range by {
		if roles["adversary"] && roles["sympathetic"] {
			n++
		}
	}
	return n
}

// roundKey is the round a review belongs to (section 6): the commit it
// reviewed and how many of the worker's answers without a commit came
// before it.
type roundKey struct {
	commit  string
	answers int
}

func keyOf(r forge.Review, answers []time.Time) roundKey {
	n := 0
	for n < len(answers) && answers[n].Before(r.Submitted) {
		n++
	}
	return roundKey{r.Commit, n}
}

// round is one round of a pull request's thread: its key and the swarm
// reviewers' reviews in it, oldest first.
type round struct {
	roundKey
	reviews []forge.Review
}

// roundsOf is reviews' rounds (keyOf) with a review marked role=adversary
// or role=sympathetic, in the order of each round's first review; a head
// reviewed by one of the two alone is a round here too.
func roundsOf(reviews []forge.Review, answers []time.Time, swarmLogin string) []round {
	sorted := slices.Clone(reviews)
	slices.SortStableFunc(sorted, func(a, b forge.Review) int { return a.Submitted.Compare(b.Submitted) })
	var out []round
	for _, r := range sorted {
		if m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin); !ok || m.Role != "adversary" && m.Role != "sympathetic" {
			continue
		}
		k := keyOf(r, answers)
		i := slices.IndexFunc(out, func(rd round) bool { return rd.roundKey == k })
		if i < 0 {
			out, i = append(out, round{roundKey: k}), len(out)
		}
		out[i].reviews = append(out[i].reviews, r)
	}
	return out
}

// blockingLines is a review text's blocking findings: its lines that
// begin "[blocking]" (section 7.4).
func blockingLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimRight(l, "\r "); strings.HasPrefix(l, "[blocking]") {
			out = append(out, l)
		}
	}
	return out
}
