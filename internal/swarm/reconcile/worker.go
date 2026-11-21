package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
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

// workerClone is the worker run's clone, which step 1 reads (section
// 7.6): <run>/work/<name>, <name> its repository; a run launched before
// the run's folder, which has no <run>/job, has it at <run>/work.
func (c *collection) workerClone() string {
	work := filepath.Join(c.r.Dir, "work")
	if _, err := c.s.O.FS().Lstat(filepath.Join(c.r.Dir, "job")); err != nil {
		return work
	}
	return filepath.Join(work, branchRepo(c.lines))
}

// fetchRef is fetch but for its failure: a clone that reaches outside
// itself is ErrForeignClone, nothing fetched or recorded.
func (c *collection) fetchRef() error {
	if _, ok := Has(c.lines, "fetched"); ok {
		return nil
	}
	sha, err := c.s.git().FetchRun(branchRepo(c.lines), c.workerClone(), runBranch(c.lines), runRef(c.r))
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

// toPush: the run has commits to push, its own past its start or, from
// a start at kept commits not yet on the slice's branch (`unpushed`,
// recorded at the launch), those commits fetched unchanged, which are
// not the failure no commits (section 7.9, The classification; 7.6,
// step 1).
func (c *collection) toPush() bool {
	_, unpushed := Has(c.lines, "unpushed "+c.fetched())
	return c.commits() || c.fetched() != "" && unpushed
}

// keep is step 2: a worker run that does not push keeps its commits as
// the run's ref, `kept <ref>`, and names them in its slice's kept unless
// they are refused (section 7.8); one with none drops the ref.
func (c *collection) keep() error {
	if _, ok := Has(c.lines, "fetched"); !ok {
		return nil
	}
	if !c.commits() {
		return c.s.git().DropRef(branchRepo(c.lines), runRef(c.r))
	}
	if _, ok := Has(c.lines, "kept"); !ok {
		if err := c.step("kept " + runRef(c.r)); err != nil {
			return err
		}
	}
	if reason, err := c.refused(); err != nil || reason != "" {
		return err
	}
	return c.writeKept()
}

// refused is why no run may build on the worker run's commits (section
// 7.6, step 3): "secret in output: commits" when an object the push
// would send holds a secret the member was given, then "protected path
// changed: <paths>" when they change a protected path (its declared
// dockerfile among them); "" when neither.
// The secret check comes first: the protected-path note names paths, and
// a path is the member's text too.
func (c *collection) refused() (string, error) {
	s, name, sha := c.s, branchRepo(c.lines), c.fetched()
	objects, err := s.git().Objects(name, s.Cfg.Base, sha)
	if err != nil {
		return "", err
	}
	if hit(objects, s.secretsOf(c.r, c.lines)) {
		return "secret in output: commits", nil
	}
	changed, err := s.git().Changed(name, s.Cfg.Base, sha)
	if err != nil {
		return "", err
	}
	repo, _ := s.repo(name)
	if hits := ProtectedIn(changed, repo); len(hits) > 0 {
		return "protected path changed: " + strings.Join(hits, ", "), nil
	}
	return "", nil
}

// worker is a worker run's steps after its output was read (section 7.6):
// no commits, which on review-3-changes-requested with a NO CHANGE: answer
// is that answer posted on the pull request and T12; the secret check of
// every object the push would send, then protected paths (refused); the
// artifacts read; the check, started and collected when it ends, its
// posted tail checked for secrets, or its whole output when it failed,
// which is then written to checks/; the push, and the slice's kept; then
// the pull request opened or commented on, the artifacts attached to it,
// and T8 or T12.
func (c *collection) worker(out string, pull *forge.Pull) error {
	s := c.s
	if !c.toPush() {
		if c.label != forge.ChangesRequested || !strings.HasPrefix(out, noChange) {
			return c.fail("no commits")
		}
		if err := c.keep(); err != nil { // the run's ref dropped
			return err
		}
		if err := c.onPull(*pull, c.runText("worker", out, "")); err != nil {
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
		if reason, err := c.refused(); err != nil {
			return err
		} else if reason != "" {
			return c.fail(reason)
		}
	}
	arts, clean, err := c.artifacts(secrets)
	if err != nil || !clean {
		return err
	}
	block := noCheckBlock
	if len(repo.Check) > 0 {
		if step := nextStep(repo, c.lines); step != "" {
			return c.startCheck(step)
		}
		result, out, file, _ := checkResult(s.O.FS(), c.r, c.lines, s.Cfg.Timeout)
		if result != "exit 0" && !pushed {
			if held, err := c.classify(repo, out); err != nil || held {
				return err
			}
		}
		tail := strings.Join(lastLines(out), "\n")
		// A check that failed is written whole to the workspace (section
		// 7.8, checks/), so its whole output is checked; else the tail
		// posted.
		failed, checked := result != "exit 0", tail
		if failed {
			checked = out
		}
		if hit([]byte(checked), secrets) {
			return c.fail("secret in output: " + file)
		}
		if failed {
			if err := c.writeCheck(sha, checkLine(repo.Check, sha, result), out); err != nil {
				return err
			}
		}
		block = checkBlock(repo.Check, sha, result, tail)
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
	// A run that pushed only the kept commits it started from leaves kept
	// naming the kept run: its fetched commit is not past its start
	// (section 7.8, slices/<s>/kept), as the replay reads it.
	if c.commits() {
		if err := c.writeKept(); err != nil {
			return err
		}
	}
	text, tail := report(out, block)
	l := c.runText("worker", text, tail)
	var pr forge.Ref
	if c.label == forge.ChangesRequested || pull != nil && pull.Open {
		if err := c.onPull(*pull, l); err != nil {
			return err
		}
		pr = forge.Ref{Repo: pull.Repo, N: pull.Number}
	} else {
		posted, more, err := c.prepare(l)
		if err != nil {
			return err
		}
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
			if p.Open && p.Head == branch && strings.HasPrefix(p.Body, mark) && strings.EqualFold(p.Author, s.SwarmLogin) {
				created, pr = true, forge.Ref{Repo: p.Repo, N: p.Number}
			}
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
			pr = forge.Ref{Repo: name, N: m}
		}
		if len(more) > 0 {
			existing, err := s.Forge.PullComments(pr)
			if err != nil {
				return err
			}
			if err := c.continued(pr, existing, l, more); err != nil {
				return err
			}
		}
	}
	if err := c.attach(pr, arts); err != nil {
		return err
	}
	if err := c.relabel(forge.ReviewReady); err != nil {
		return err
	}
	return c.end("posted")
}

// noCheckBlock is the check block of a repository that declares no check.
const noCheckBlock = "**Check:** none declared."

// checkBlock is the worker's report's check block (section 7.6): the
// check run on sha, its result and the tail of its output.
func checkBlock(check []string, sha, result, tail string) string {
	return fmt.Sprintf("**Check:** `%s` on %s: %s\n\n```\n%s\n```", declare.ShellLine(check), short(sha), result, tail)
}

// report is the worker's report as its collection posts it, its text out
// and, after one blank line, the check block, which is never cut
// (section 7.5).
func report(out, block string) (text, tail string) {
	return strings.TrimRight(out, "\r\n"), "\n\n" + block
}

// startCheck starts step, the dependency step or the check, on the
// worker run's head, a classified failure's first output kept first
// (keepFirst); the run is collected again at once when the step could
// not start, a fault.
func (c *collection) startCheck(step string) error {
	s := c.s
	if err := c.keepFirst(); err != nil {
		return err
	}
	if err := s.startStep(c.r, c.lines, step); err != nil {
		return err
	}
	if s.live[c.r.Dir] == nil {
		return s.collect(c.r) // the step could not start: a fault
	}
	return nil
}

// classify is section 7.9's classification of the worker run's failed
// check, or dependency step, whose whole output is out: a failure not
// classified (a test services file refused among them) is the change's,
// and the collection goes on (held false); one classified is run again
// once (`check rerun: <reason>`, the stdout line, then the step started
// again), or, classified on its rerun, is the failure check
// environment, nothing pushed or posted on the pull request and no
// checks/ file written. The change and the head's Go facts are read
// from the mirror only for an output Classify could classify.
func (c *collection) classify(repo declare.Repo, out string) (held bool, err error) {
	s, lines := c.s, c.checkLines()
	if servicesRefused(lines) != "" || !MayClassify([]byte(out)) {
		return false, nil
	}
	name, sha := branchRepo(c.lines), c.fetched()
	changed, err := s.git().Changed(name, s.Cfg.Base, sha)
	if err != nil {
		return false, err
	}
	facts, err := s.git().GoFacts(name, sha)
	if err != nil {
		return false, err
	}
	_, checkTimedOut := Has(lines, "check timeout")
	_, depsTimedOut := Has(lines, "deps timeout")
	cl := Classify([]byte(out), changed, facts, checkTimedOut || depsTimedOut)
	if !cl.Classified() {
		return false, nil
	}
	reason := cl.Reason()
	if hit([]byte(strings.Join(cl.Untouched, "\n")), s.secretsOf(c.r, c.lines)) {
		reason = "untouched packages withheld: one holds a secret the member was given"
		if cl.Signature != "" {
			reason += "; " + Classification{Signature: cl.Signature}.Reason()
		}
	}
	if _, again := Has(c.lines, rerun); again {
		return true, c.fail(checkEnvironment + reason)
	}
	if err := c.step(rerun + ": " + reason); err != nil {
		return true, err
	}
	fmt.Fprintf(s.Out, "%scheck failed in the environment (%s): run again\n", c.head, reason)
	return true, c.startCheck(nextStep(repo, c.lines))
}

// rerun begins the line that records a classified failure's one rerun,
// and checkEnvironment the reason of a failure classified again (section
// 7.9).
const (
	rerun            = "check rerun"
	checkEnvironment = "check environment: "
)

// checkLines is the worker run's lines its check is read from: those
// after its rerun (sinceRerun).
func (c *collection) checkLines() []string { l, _ := sinceRerun(c.lines); return l }

// sinceRerun is the lines a worker run's check is read from (section
// 7.9): those after its `check rerun`, every line when it has none; and
// whether that rerun is the check's alone, its dependency step having
// ended 0 before it, so that the step is not run again.
func sinceRerun(lines []string) (after []string, checkAlone bool) {
	for i, l := range lines {
		if strings.HasPrefix(l, rerun+": ") {
			before := lines[:i]
			_, depsTimedOut := Has(before, "deps timeout")
			return lines[i+1:], depsFailed(before) == "" && !depsTimedOut
		}
	}
	return lines, false
}

// keepFirst keeps a classified failure's first output before its rerun
// starts (section 7.9): <run>/check.out renamed check-1.out, or, for the
// dependency step's failure, deps.out deps-1.out; once, so a rerun
// started again after its apply died keeps the first output.
func (c *collection) keepFirst() error {
	i := slices.IndexFunc(c.lines, func(l string) bool { return strings.HasPrefix(l, rerun+": ") })
	if i < 0 {
		return nil
	}
	step := "check"
	if _, checkAlone := sinceRerun(c.lines); !checkAlone {
		step = "deps"
	}
	fs := c.s.O.FS()
	first := filepath.Join(c.r.Dir, step+"-1.out")
	if _, err := fs.Lstat(first); err == nil || !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return fs.Rename(filepath.Join(c.r.Dir, step+".out"), first)
}

// nextStep is the container a worker run's collection starts next
// (section 7.6, step 4), read from the lines after its rerun (7.9): the
// dependency step, when the repository declares one and it has not
// started (nor run before a rerun of the check alone); else the check,
// unless it has started, the dependency step failed or the head's test
// services file was refused; "" when there is none to start.
func nextStep(repo declare.Repo, lines []string) string {
	lines, checkAlone := sinceRerun(lines)
	_, depsStarted := Has(lines, "deps started")
	if repo.Deps != nil && !depsStarted && !checkAlone {
		return "deps"
	}
	_, checkStarted := Has(lines, "check started")
	_, depsTimedOut := Has(lines, "deps timeout")
	if checkStarted || depsTimedOut || depsFailed(lines) != "" || servicesRefused(lines) != "" {
		return ""
	}
	return "check"
}

// onPull posts the worker's text as a comment on its open pull request,
// unless it is the one this run opened, whose description holds it: then
// only the continuations not yet posted (section 7.5).
func (c *collection) onPull(p forge.Pull, l long) error {
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	existing, err := c.s.Forge.PullComments(ref)
	if err != nil {
		return err
	}
	if strings.HasPrefix(p.Body, marker("worker", c.r.K)) && strings.EqualFold(p.Author, c.s.SwarmLogin) {
		_, more, err := c.prepare(l)
		if err != nil {
			return err
		}
		return c.continued(ref, existing, l, more)
	}
	return c.sayLong(ref, existing, "worker", l)
}

// review is a reviewer run's steps (section 7.6): its verdict read and
// checked against its findings' severity (7.5), the review posted, and
// once the pair on the head is complete, T9, T10 or T11, from the stage
// the run is at (review-1-in-progress, or the stage it was launched on).
func (c *collection) review(out string, pull forge.Pull, pulls []forge.Pull) error {
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
	answers := Reopenings(pullComments(comments), s.SwarmLogin, s.Cfg.Trusted)
	posted := false
	for _, rv := range reviews {
		m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		posted = posted || ok && m.Role == c.r.Role && m.Run == c.r.K
	}
	l := c.runText(c.r.Role, out, "")
	l.review = true
	text, more, err := c.prepare(l)
	if err != nil {
		return err
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
	// A review's continuations are the pull request's comments: no review
	// but the one carries its verdict.
	if err := c.continued(ref, comments, l, more); err != nil {
		return err
	}
	// The round this review completes is written with it (section 7.8).
	if err := s.writeReviews(c.r.N, pull, reviews, answers, false); err != nil {
		return err
	}
	if err := c.writeRound(pull, pulls, reviews, comments, answers); err != nil {
		return err
	}
	pair := map[string]string{}
	at := map[string]time.Time{}
	for _, rv := range reviews {
		m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		if ok && rv.Commit == pull.HeadSHA && standsOnHead(answers, rv.Submitted) && (m.Role == "adversary" || m.Role == "sympathetic") && !rv.Submitted.Before(at[m.Role]) {
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
	on := pullLink(s.owner(), pull)
	unlinked := fmt.Sprintf("%s#%d", pull.Repo, pull.Number) // a note's form before Step 7 of the plan for the inbox requests F27-F35
	first := fmt.Sprintf("Review round %d of %d on %s requested changes (was %s). "+architectRevises, rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress)
	second := fmt.Sprintf("Review round %d of %d on %s "+roundLimitAgain+" (at %s). Relabel the issue %s for one more fix round or %s to re-plan, then remove %s.",
		rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress, forge.ChangesRequested, forge.PlanPosted, forge.Blocked) + s.mention()
	own := []string{marker("reconciler", c.r.K) + first, marker("reconciler", c.r.K) + second}
	again := false
	for _, cm := range c.is.Comments {
		m, ok := Swarms(Entry{cm.Author, cm.Body, cm.Created}, s.SwarmLogin)
		t := swarmText(cm.Body)
		again = again || ok && m.Role == "reconciler" && strings.HasPrefix(t, "Review round ") &&
			(strings.Contains(t, " on "+on+" requested changes") || strings.Contains(t, " on "+unlinked+" requested changes")) && cm.Body != own[0] && cm.Body != own[1]
	}
	if again {
		// The second time and each after, the stage stays and the issue
		// is blocked: the maintainer's (G4); unless the maintainer moved
		// it during the run.
		if now, moved, err := c.movedAway(false); err != nil {
			return err
		} else if moved {
			second = fmt.Sprintf("Review round %d of %d on %s "+roundLimitAgain+" (at %s). %s", rounds, s.Cfg.MaxRounds, on, forge.ReviewInProgress, notBlocked(now))
		}
		if err := c.comment("reconciler", second); err != nil {
			return err
		}
		if err := c.flag(forge.Blocked, false); err != nil {
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

// roundLimitAgain is the words of the round limit's note each time after
// the first, which counts no time: the reviewers have both reviewed the
// head, so review-1-in-progress runs no one (section 6).
const roundLimitAgain = "requested changes again"

// Rounds is a pull request's review rounds (section 6): the heads with a
// review marked role=adversary and one marked role=sympathetic, a head
// counted once more for each reopening without a commit (Reopenings,
// answers: the worker's answers and the maintainer's comments) after
// which both reviewed it again; counting only reviews submitted after
// since (all of them when since is zero), and not except's since the
// newest reopening, the head as it stands.
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
// reviewed and how many reopenings without a commit (Reopenings) came
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
func blockingLines(text string) []string { return linesBeginning(text, "[blocking]") }

// linesBeginning is text's lines that begin with one of prefixes, in
// order, each without the spaces and `\r` at its end.
func linesBeginning(text string, prefixes ...string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r ")
		if slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(l, p) }) {
			out = append(out, l)
		}
	}
	return out
}
