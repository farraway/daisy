package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
)

// The rollout (section 7.8, Rollout): a job open when a build with the
// workspace arrives has a partial workspace or none. It is built once,
// at the first pass that finds the issue at a stage without its built
// marker, by replaying the writes of its runs' collections in the order
// they ended, through the functions that write at collection; the marker,
// written last, says it is built. A new job is replayed over no run at
// its first pass, before its first launch.

// builtFile is the workspace's marker, an empty file the replay writes
// last.
const builtFile = "built"

// notAvailable ends a text a replay takes from a post Daisy cut, whose
// whole is gone, in place of the cut's note (section 7.8, Rollout).
const notAvailable = truncated + "the rest is not available to you]"

// Built: issue n's workspace under state holds its built marker; a
// marker that cannot be read, its directory closed, is the job's own
// failure (unusableError).
func Built(fs ops.FS, state string, n int) (bool, error) {
	_, err := fs.Lstat(filepath.Join(state, "jobs", strconv.Itoa(n), builtFile))
	if errors.Is(err, iofs.ErrNotExist) {
		return false, nil
	}
	return err == nil, workspaceUnusable(n, builtFile, 0, err)
}

// post is a swarm entry as the forge holds it: a comment, a review
// (review) or a pull request's description (desc).
type post struct {
	Entry
	review, desc bool
}

// issuePosts is the comments of issue is as posts; its body, a batch
// run's proposal, is no run's of the job.
func issuePosts(is forge.Issue) []post {
	out := make([]post, len(is.Comments))
	for i, c := range is.Comments {
		out[i] = post{Entry: Entry{c.Author, c.Body, c.Created}}
	}
	return out
}

// postedText is the text of run m on issue n as its post p holds it
// (section 7.8, Rollout): without its marker and, for a description, the
// Refs line Daisy ends it with; a text Daisy cut (cutRun) with its note
// replaced by notAvailable, cut saying so.
func postedText(n int, m Mark, p post) (text string, cut bool) {
	text = swarmText(p.Body)
	if i := strings.LastIndex(text, "\n\nRefs "); p.desc && i >= 0 && !strings.Contains(text[i+2:], "\n") {
		text = text[:i]
	}
	if _, at, end := cutRun(n, m, text); at >= 0 {
		return text[:at] + notAvailable + text[end:], true
	}
	return text, false
}

// jobRun is a run a replay reads: one collected here, with its result's
// lines, or one known by its post alone (lines nil), collected on
// another machine or its <run> pruned; post is the first entry its mark
// marks among the threads read, nil when there is none.
type jobRun struct {
	Run
	lines []string
	post  *post
	at    time.Time
}

func (jr jobRun) name() string { return filepath.Base(jr.Dir) }

func (jr jobRun) outcome() string {
	l, _ := Has(jr.lines, "outcome")
	return strings.TrimPrefix(l, "outcome ")
}

// jobRuns is issue n's runs under o in the order they ended: each run
// collected here, by its end line, and each run of a post among posts
// that has no directory here, by its post's time. A run here not yet
// collected is left out: its own collection writes what it posts.
func (o Outputs) jobRuns(n int, swarmLogin string, posts []post) ([]jobRun, error) {
	runs, err := Runs(o.FS, o.State)
	if err != nil {
		return nil, err
	}
	var out []jobRun
	here := map[string]int{} // a run's name → its index in out, -1 when not yet collected
	for _, r := range runs {
		if r.N != n {
			continue
		}
		lines, err := r.Result(o.FS)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(r.Dir)
		if _, done := Has(lines, "collected"); !done {
			here[name] = -1
			continue
		}
		at, _ := endTime(lines)
		here[name] = len(out)
		out = append(out, jobRun{Run: r, lines: lines, at: at})
	}
	for i := range posts {
		p := &posts[i]
		m, ok := Swarms(p.Entry, swarmLogin)
		if !ok || m.Kind != "" || m.Role == "reconciler" {
			continue
		}
		name := outputRun(n, m)
		switch j, ok := here[name]; {
		case !ok:
			here[name] = len(out)
			out = append(out, jobRun{Run: Run{Dir: filepath.Join(o.State, "runs", name), N: n, K: m.Run, Role: m.Role}, post: p, at: p.Created})
		case j >= 0 && out[j].post == nil:
			out[j].post = p
		}
	}
	slices.SortStableFunc(out, func(a, b jobRun) int {
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return strings.Compare(a.name(), b.name())
	})
	return out, nil
}

// planOf is the plan run jr's collection wrote to plan.md and its
// builder plan to builder.md (nil for none, which removes it), and
// whether the plan is taken cut from its post, its whole gone; plan is
// false for a run whose text is no plan (isPlan), or that posted none.
// The text is read whole as a collection wrote it to texts/, else from
// its out.md, else from its post (postedText); the builder plan from the
// run's io, which a run known by its post alone has none of.
func (o Outputs) planOf(n int, jr jobRun) (text string, builder []byte, plan, cut bool, err error) {
	questionRun := false
	if jr.lines != nil {
		if jr.outcome() != "posted" {
			return "", nil, false, false, nil
		}
		_, questionRun = Has(jr.lines, "question")
	}
	if jr.Role != "architect" || questionRun {
		return "", nil, false, false, nil
	}
	b, err := o.FS.ReadFile(filepath.Join(o.State, "jobs", strconv.Itoa(n), "texts", jr.name()+".md"))
	err = workspaceUnusable(n, path.Join("texts", jr.name()+".md"), jr.K, err)
	if errors.Is(err, iofs.ErrNotExist) {
		b, err = jr.Output(o.FS)
	}
	switch {
	case err == nil:
		text = string(b)
	case !errors.Is(err, iofs.ErrNotExist):
		return "", nil, false, false, err
	case jr.post == nil:
		return "", nil, false, false, nil
	default:
		text, cut = postedText(n, Mark{Role: jr.Role, Run: jr.K}, *jr.post)
	}
	if !isPlan(jr.Role, text, false) {
		return "", nil, false, false, nil
	}
	if builder, err = jr.Builder(o.FS); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return "", nil, false, false, err
	}
	if len(bytes.TrimSpace(builder)) == 0 {
		builder = nil
	}
	return text, builder, true, cut, nil
}

// JobPlan is issue is's accepted plan as Daisy reads it past its first
// lines, its slices and its plan gate's note (section 7.8, Daisy's own
// reads): plan.md of its workspace, when the workspace is built and the
// file is there; else, for a job without built or a plan.md not there,
// the plan the replay would leave there (planOf over jobRuns), built in
// memory and written nowhere, with cut the run whose plan is then taken
// cut from its post, its out.md gone ("" otherwise), which would count
// its slices short. ok is false for no plan. A file of the job's that is
// there and cannot be read is the error, its own (unusableError).
func (o Outputs) JobPlan(is forge.Issue, swarmLogin string) (text string, ok bool, cut string, err error) {
	p, err := o.jobPlan(is, swarmLogin)
	return p.text, p.ok, p.cut, err
}

// A jobPlan is the plan JobPlan reads and, read with it, its builder plan
// (section 7.8, The builder plan): the file, from <state>, and its bytes;
// builder is "" for none.
type jobPlan struct {
	text, cut string
	ok        bool
	builder   string
	size      int64
}

// jobPlan is JobPlan's plan with its builder plan, read from where the
// plan is: with plan.md read, the workspace's builder.md, a file there
// being the one its collection wrote (Job.size); with the plan built in
// memory, the plan run's io/builder.md, as planOf reads it (a file not
// regular, or holding only whitespace, being none).
func (o Outputs) jobPlan(is forge.Issue, swarmLogin string) (jobPlan, error) {
	n := is.Number
	built, err := Built(o.FS, o.State, n)
	if err != nil {
		return jobPlan{}, err
	}
	if built {
		b, err := o.FS.ReadFile(filepath.Join(o.State, "jobs", strconv.Itoa(n), "plan.md"))
		if err == nil {
			p := jobPlan{text: string(b), ok: true}
			rel := path.Join("jobs", strconv.Itoa(n), "builder.md")
			fi, err := o.FS.Lstat(filepath.Join(o.State, rel))
			switch {
			case err == nil:
				p.builder, p.size = rel, fi.Size()
			case !errors.Is(err, iofs.ErrNotExist):
				return jobPlan{}, workspaceUnusable(n, "builder.md", 0, err)
			}
			return p, nil
		}
		if err = workspaceUnusable(n, "plan.md", 0, err); !errors.Is(err, iofs.ErrNotExist) {
			return jobPlan{}, err
		}
	}
	runs, err := o.jobRuns(n, swarmLogin, issuePosts(is))
	if err != nil {
		return jobPlan{}, err
	}
	var p jobPlan
	for _, jr := range runs {
		t, builder, plan, c, err := o.planOf(n, jr)
		if err != nil {
			return jobPlan{}, fmt.Errorf("output %s: %w", jr.name(), err)
		}
		if plan {
			p = jobPlan{text: t, ok: true}
			if c {
				p.cut = jr.name()
			}
			if builder != nil {
				p.builder, p.size = path.Join("runs", jr.name(), "io", "builder.md"), int64(len(builder))
			}
		}
	}
	return p, nil
}

// replay builds the workspace of issue is, open at a stage with its pull
// requests pulls, unless its built marker is there (section 7.8,
// Rollout): for each of its runs in the order they ended (jobRuns), its
// plan (planOf), its text (replayedText) and, for a run collected here,
// a worker's failed check and kept (replayedWork) and the round a
// review completed (writeRound); then each review held for its round
// (writeReviews, a review whose out.md is gone taken from its post);
// then, when the plan it leaves is taken cut from its post, the block
// (blockCut); last the marker. blocked says it blocked the issue.
func (s *Swarm) replay(is forge.Issue, pulls []forge.Pull) (blocked bool, err error) {
	n, o := is.Number, s.outputs()
	if built, err := Built(o.FS, o.State, n); err != nil || built {
		return false, err
	}
	type thread struct {
		reviews  []forge.Review
		comments []forge.Comment
		answers  []time.Time
	}
	threads := make([]thread, len(pulls))
	posts := issuePosts(is)
	for i, p := range pulls {
		ref := forge.Ref{Repo: p.Repo, N: p.Number}
		reviews, err := s.Forge.Reviews(ref)
		if err != nil {
			return false, err
		}
		comments, err := s.Forge.PullComments(ref)
		if err != nil {
			return false, err
		}
		threads[i] = thread{reviews, comments, Reopenings(pullComments(comments), s.SwarmLogin, s.Cfg.Trusted)}
		posts = append(posts, post{Entry: Entry{p.Author, p.Body, p.Created}, desc: true})
		for _, c := range comments {
			posts = append(posts, post{Entry: Entry{c.Author, c.Body, c.Created}})
		}
		for _, r := range reviews {
			posts = append(posts, post{Entry: Entry{r.Author, r.Body, r.Submitted}, review: true})
		}
	}
	runs, err := o.jobRuns(n, s.SwarmLogin, posts)
	if err != nil {
		return false, err
	}
	var cut *jobRun // the run whose plan plan.md holds, taken cut from its post
	for _, jr := range runs {
		text, builder, plan, c, err := o.planOf(n, jr)
		if err != nil {
			return false, fmt.Errorf("output %s: %w", jr.name(), err)
		}
		if plan {
			if err := s.writePlan(n, text, builder); err != nil {
				return false, err
			}
			cut = nil
			if c {
				cut = &jr
			}
		}
		col := &collection{s: s, r: jr.Run, lines: jr.lines}
		if text, ok, err := col.replayedText(jr); err != nil {
			return false, fmt.Errorf("output %s: %w", jr.name(), err)
		} else if ok {
			if err := s.writeText(n, jr.name(), text); err != nil {
				return false, err
			}
		}
		if jr.lines == nil {
			continue
		}
		if err := col.replayedWork(); err != nil {
			return false, err
		}
		if (jr.Role == "adversary" || jr.Role == "sympathetic") && jr.outcome() == "posted" {
			for i, p := range pulls {
				if slices.ContainsFunc(threads[i].reviews, func(rv forge.Review) bool {
					m, ok := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
					return ok && m.Role == jr.Role && m.Run == jr.K
				}) {
					if err := col.writeRound(p, pulls, threads[i].reviews, threads[i].comments, threads[i].answers); err != nil {
						return false, err
					}
				}
			}
		}
	}
	for i, p := range pulls {
		if err := s.writeReviews(n, p, threads[i].reviews, threads[i].answers, true); err != nil {
			return false, err
		}
	}
	if cut != nil {
		if err := s.blockCut(is, cut.name()); err != nil {
			return false, err
		}
	}
	if len(runs) > 0 {
		fmt.Fprintf(s.Out, "#%d workspace built from %d runs\n", n, len(runs))
	}
	return cut != nil, s.writeWorkspace(n, builtFile, nil)
}

// replayedText is the member text run jr's collection wrote to texts/
// (section 7.8): for a run collected here that posted one (its outcome
// posted, question or blocked), as the collection made it from its
// out.md (textKind: an answer marked, a worker's report with its check
// block); else, its out.md gone, or for a run known by its post alone,
// as its post holds it (postedText). ok is false for a run that posted
// no text, and for a review, which writeReviews writes with its round.
func (c *collection) replayedText(jr jobRun) (text string, ok bool, err error) {
	if jr.lines != nil {
		switch jr.outcome() {
		case "posted", "question", "blocked":
		default:
			return "", false, nil
		}
		out, err := jr.Output(c.s.O.FS())
		switch {
		case err == nil:
			text, ok := c.textOf(string(out))
			return text, ok, nil
		case !errors.Is(err, iofs.ErrNotExist):
			return "", false, err
		}
	}
	if jr.post == nil || jr.post.review {
		return "", false, nil
	}
	text, _ = postedText(jr.N, Mark{Role: jr.Role, Run: jr.K}, *jr.post)
	return text, true, nil
}

// textOf is the text the collection of the run, collected here, posted
// for its output out (collect's switch on textKind); ok is false for a
// review.
func (c *collection) textOf(out string) (string, bool) {
	_, questionRun := Has(c.lines, "question")
	switch textKind(c.r.Role, out, questionRun) {
	case textAnswer:
		return answerText(out), true
	case textOwn:
	default:
		return out, true
	}
	switch c.r.Role {
	case "adversary", "sympathetic":
		return "", false
	case "worker":
	default:
		return out, true
	}
	if !c.commits() {
		return out, true // NO CHANGE:, posted as it is
	}
	repo, _ := c.s.repo(branchRepo(c.lines))
	block := noCheckBlock
	if len(repo.Check) > 0 {
		// A run that records no check result was collected while its
		// repository declared none, and posted the no-check block: it is
		// replayed with it, whatever the repository declares since
		// (section 7.8, Rollout; the maintainer's answer of 2026-11-20).
		if result, checked, _, ok := checkResult(c.s.O.FS(), c.r, c.lines, c.s.Cfg.Timeout); ok {
			block = checkBlock(repo.Check, c.fetched(), result, strings.Join(lastLines(checked), "\n"))
		}
	}
	text, tail := report(out, block)
	return text + tail, true
}

// replayedWork is a worker run's failed check's log and its slice's
// kept as its collection wrote them (section 7.8): the check that ended
// and failed, unless the run faulted, whose collection ends before step
// 4, or its whole output holds a secret the member was given; then, for a run whose commits are past its start and that was
// not discarded, its kept, unless the mirror no longer holds its commit
// or, not pushed, they fail the secret check or change a protected path
// (refused).
func (c *collection) replayedWork() error {
	if c.r.Role != "worker" || Ended(c.lines) == "launch-failed" || c.fetched() == "" {
		return nil
	}
	s, name, sha := c.s, branchRepo(c.lines), c.fetched()
	repo, _ := s.repo(name)
	_, fault := Has(c.lines, "fault") // its collection ended at the fault, before step 4
	if result, out, _, ok := checkResult(s.O.FS(), c.r, c.lines, s.Cfg.Timeout); ok && !fault && result != "exit 0" && !hit([]byte(out), s.secretsOf(c.r, c.lines)) {
		if err := c.writeCheck(sha, checkLine(repo.Check, sha, result), out); err != nil {
			return err
		}
	}
	if l, _ := Has(c.lines, "outcome"); !c.commits() || l == "outcome discarded" {
		return nil
	}
	if held, err := s.git().Holds(name, sha); err != nil || !held {
		return err
	}
	if _, pushed := Has(c.lines, "pushed"); !pushed {
		if reason, err := c.refused(); err != nil || reason != "" {
			return err
		}
	}
	return c.writeKept()
}

// blockCut is T17 for a job whose plan, replayed or built in memory
// (JobPlan), is taken cut from its post, run's out.md gone (section 7.8,
// Rollout), rather than have its slices counted short: the reconciler's
// note naming the missing file.
func (s *Swarm) blockCut(is forge.Issue, run string) error {
	r, _ := runOf(run)
	return s.block(is, r.K, fmt.Sprintf("The plan of %s is cut on the forge and runs/%s/io/out.md is gone, so its slices cannot be counted (docs/swarm.md section 7.8, Rollout). Restore the file and remove <state>/jobs/%d, or relabel the issue %s for a new plan; then remove %s to resume.",
		run, run, is.Number, forge.ProposalAccepted, forge.Blocked))
}

// blockUnusable is T17 for a job one of whose files Daisy cannot use
// (unusableError), met by its replay or by a read of a run's text whole
// (section 7.7, a job's file Daisy cannot use; the maintainer's rule of
// 2026-11-18), rather than stop the pass for every issue: the
// reconciler's note naming the file and the cause. blocked is false, with
// err as it is, for any other error, which is the pass's.
func (s *Swarm) blockUnusable(is forge.Issue, err error) (blocked bool, _ error) {
	var u *unusableError
	if !errors.As(err, &u) {
		return false, err
	}
	cause := u.err
	if pe := (*iofs.PathError)(nil); errors.As(cause, &pe) {
		cause = pe.Err // the path is the state's, which the note names from <state>
	}
	return true, s.block(is, max(u.k, 1), fmt.Sprintf("Daisy cannot use %s: %v (docs/swarm.md section 7.8, Rollout). Make the file readable, or remove it if it is an out.md, its run's text then taken from its post, or under jobs/%d/, which the replay writes again; then remove %s to resume.",
		u.file, cause, is.Number, forge.Blocked))
}

// block is T17 by a pass, no run's (section 7.8, Rollout): the
// reconciler's note, text marked with run k and the maintainer's
// mention, unless the issue holds it, then swarm:blocked beside the
// stage, and swarm:question cleared.
func (s *Swarm) block(is forge.Issue, k int, text string) error {
	at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
	body := marker("reconciler", k) + text + s.mention()
	if !slices.ContainsFunc(is.Comments, func(c forge.Comment) bool { return c.Body == body && strings.EqualFold(c.Author, s.SwarmLogin) }) {
		if err := s.Forge.Comment(at, body); err != nil {
			return err
		}
		fmt.Fprintf(s.Out, "#%d posted comment\n", is.Number)
	}
	if err := Flag(s.Forge, at, is.Labels, forge.Blocked, s.Out); err != nil {
		return err
	}
	return Unflag(s.Forge, at, is.Labels, forge.Question, s.Out)
}
