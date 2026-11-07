package reconcile

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"daisy/internal/swarm/forge"
)

// fetch is a worker run's step 1 (section 7.6): its branch fetched from
// its clone into the mirror as the run's ref, then each new commit
// without the role's trailer reported (The trailer): on stderr and as
// `unstamped <sha>`, never a failure.
func (c *collection) fetch() error {
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
	if !c.commits() {
		return nil
	}
	coAuthor := c.s.Cfg.Personas["worker"].CoAuthor
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
// no commits; protected paths and the secret check of every object the
// push would send; the check, started and collected when it ends, its
// posted tail checked for secrets; the push; then the pull request opened
// or commented on, and T8 or T12.
func (c *collection) worker(text string, pull *forge.Pull) error {
	s := c.s
	if !c.commits() {
		return c.fail("no commits")
	}
	name, branch, sha := branchRepo(c.lines), runBranch(c.lines), c.fetched()
	repo, _ := s.repo(name)
	secrets := s.secretsOf(c.r, c.lines)
	_, pushed := Has(c.lines, "pushed")
	if !pushed {
		changed, err := s.git().Changed(name, s.Cfg.Base, sha)
		if err != nil {
			return err
		}
		if hits := Protected(changed, repo.Protected); len(hits) > 0 {
			return c.fail("protected path changed: " + strings.Join(hits, ", "))
		}
		objects, err := s.git().Objects(name, s.Cfg.Base, sha)
		if err != nil {
			return err
		}
		if hit(objects, secrets) {
			return c.fail("secret in output: commits")
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
	posted := text + "\n\n" + block
	if c.label == "swarm:reviewed" || pull != nil && pull.Open {
		if err := c.onPull(*pull, posted); err != nil {
			return err
		}
	} else {
		issue := fmt.Sprintf("#%d", c.r.N)
		if name != repoName(s.Cfg.Forge.Repo) {
			issue = s.Cfg.Forge.Repo + issue
		}
		pulls, err := s.Forge.Pulls(c.r.N)
		if err != nil {
			return err
		}
		mark := marker("worker", c.r.K)
		created := false
		for _, p := range pulls {
			created = created || p.Open && p.Head == branch && strings.HasPrefix(p.Body, mark)
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
	if err := c.relabel(c.label, "swarm:in-review"); err != nil {
		return err
	}
	return c.end("posted")
}

// onPull posts the worker's text as a comment on its open pull request,
// unless it is the one this run opened.
func (c *collection) onPull(p forge.Pull, posted string) error {
	if strings.HasPrefix(p.Body, marker("worker", c.r.K)) {
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
// once the pair on the head is complete, T9, T10 or T11.
func (c *collection) review(out, text string, pull forge.Pull) error {
	s := c.s
	verdict := verdictOf(out)
	blocking := false
	for _, l := range strings.Split(out, "\n") {
		blocking = blocking || strings.HasPrefix(l, "[blocking]")
	}
	if verdict == "comment" || (verdict == "request-changes") != blocking {
		return c.fail("bad verdict")
	}
	ref := forge.Ref{Repo: pull.Repo, N: pull.Number}
	reviews, err := s.Forge.Reviews(ref)
	if err != nil {
		return err
	}
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
		if ok && rv.Commit == pull.HeadSHA && (m.Role == "adversary" || m.Role == "sympathetic") && !rv.Submitted.Before(at[m.Role]) {
			pair[m.Role], at[m.Role] = verdictOf(swarmText(rv.Body)), rv.Submitted
		}
	}
	if len(pair) < 2 {
		return c.end("posted")
	}
	if pair["adversary"] == "approve" && pair["sympathetic"] == "approve" {
		if err := c.relabel(c.label, "swarm:approved"); err != nil {
			return err
		}
		return c.end("posted")
	}
	plan, _, hasPlan := (&Job{SwarmLogin: s.SwarmLogin, Issue: c.is}).Plan()
	var since time.Time
	if hasPlan {
		since = plan.Created
	}
	rounds := Rounds(reviews, s.SwarmLogin, since, "")
	if rounds < s.Cfg.MaxRounds {
		if err := c.relabel(c.label, "swarm:reviewed"); err != nil {
			return err
		}
		return c.end("posted")
	}
	on := fmt.Sprintf("%s#%d", pull.Repo, pull.Number)
	first := fmt.Sprintf("Review round %d of %d on %s requested changes (was swarm:in-review). The architect revises the plan from the reviews.", rounds, s.Cfg.MaxRounds, on)
	second := fmt.Sprintf("Review round %d of %d on %s requested changes a second time (was swarm:in-review). Relabel the issue to resume.", rounds, s.Cfg.MaxRounds, on)
	own := []string{marker("reconciler", c.r.K) + first, marker("reconciler", c.r.K) + second}
	again := false
	for _, cm := range c.is.Comments {
		m, ok := Swarms(Entry{cm.Author, cm.Body, cm.Created}, s.SwarmLogin)
		t := swarmText(cm.Body)
		again = again || ok && m.Role == "reconciler" && strings.HasPrefix(t, "Review round ") &&
			strings.Contains(t, " on "+on+" requested changes") && cm.Body != own[0] && cm.Body != own[1]
	}
	note, to := first, "swarm:planning"
	switch {
	case again:
		note, to = second, "swarm:blocked"
	case !s.Cfg.Gates.Plan:
		to = "swarm:ready"
	}
	if err := c.comment("reconciler", note); err != nil {
		return err
	}
	if err := c.relabel(c.label, to); err != nil {
		return err
	}
	return c.end("posted")
}

// Rounds is a pull request's review rounds (section 6): the distinct
// commits, other than except, with a review marked role=adversary and one
// marked role=sympathetic, counting only reviews submitted after since
// (all of them when since is zero).
func Rounds(reviews []forge.Review, swarmLogin string, since time.Time, except string) int {
	by := map[string]map[string]bool{}
	for _, r := range reviews {
		m, ok := Swarms(Entry{r.Author, r.Body, r.Submitted}, swarmLogin)
		if !ok || !since.IsZero() && !r.Submitted.After(since) || r.Commit == except {
			continue
		}
		if by[r.Commit] == nil {
			by[r.Commit] = map[string]bool{}
		}
		by[r.Commit][m.Role] = true
	}
	n := 0
	for _, roles := range by {
		if roles["adversary"] && roles["sympathetic"] {
			n++
		}
	}
	return n
}
