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

	"daisy/internal/swarm/forge"
)

// The job's workspace, <state>/jobs/<n> (section 7.8), written by the
// collection of a run on issue n and by the replay that builds it once
// for a job in flight (replay.go). Of its files this build writes
// texts/, plan.md, builder.md, slices/<s>/slice.md, slices/<s>/kept,
// slices/<s>/rounds/<r>.md, checks/<sha>.log and built, and of them
// only kept is read yet, by the worker's launch (start.go).

// workspace is issue n's workspace.
func (s *Swarm) workspace(n int) string { return filepath.Join(s.state(), "jobs", strconv.Itoa(n)) }

// writeWorkspace writes data to the file rel of issue n's workspace,
// whole under a temporary name in its directory and then renamed into
// place (ops.FS's WriteFile), unless the file holds data already, so a
// collection retried writes it once (section 7.6), or is a file written
// once (writtenOnce) that is there.
func (s *Swarm) writeWorkspace(n int, rel string, data []byte) error {
	fs := s.O.FS()
	p := filepath.Join(s.workspace(n), rel)
	old, err := fs.ReadFile(p)
	switch {
	case err == nil && (bytes.Equal(old, data) || writtenOnce(rel)):
		return nil
	case err != nil && !errors.Is(err, iofs.ErrNotExist):
		return workspaceUnusable(n, filepath.ToSlash(rel), 0, err)
	}
	if err := fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return fs.WriteFile(p, data, 0o644)
}

// workspaceUnusable is err, the failed read of the file rel of issue n's
// workspace, of run k's (0 for none), as the job's own (unusableError);
// nil and a file not there (iofs.ErrNotExist) as they are.
func workspaceUnusable(n int, rel string, k int, err error) error {
	if err == nil || errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return &unusableError{file: path.Join("jobs", strconv.Itoa(n), rel), k: k, err: err}
}

// writtenOnce: the workspace's file rel, a member's text, a round's or
// a failed check's (texts/, slices/<s>/rounds/, checks/), is never
// rewritten: one there is what the collection that wrote it read, which
// a replay that reads less (a text's out.md gone) leaves as it is
// (section 7.8, Rollout).
func writtenOnce(rel string) bool {
	dir := filepath.ToSlash(filepath.Dir(rel))
	return dir == "texts" || dir == "checks" || path.Base(dir) == "rounds"
}

// writePlan writes an architect's plan, collected on issue n, to
// plan.md whole (no amendment is written yet), its builder plan to
// builder.md, or, with none, removes builder.md, so that no builder plan
// outlives its plan; then each slice the plan declares its slice.md
// (sliceText). A slice the plan no longer declares keeps its directory.
func (s *Swarm) writePlan(n int, plan string, builder []byte) error {
	if err := s.writeWorkspace(n, "plan.md", []byte(plan)); err != nil {
		return err
	}
	if builder != nil {
		if err := s.writeWorkspace(n, "builder.md", builder); err != nil {
			return err
		}
	} else if err := s.O.FS().Remove(filepath.Join(s.workspace(n), "builder.md")); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	for i := 1; i <= max(1, len(Slices(plan))); i++ {
		if err := s.writeWorkspace(n, filepath.Join("slices", strconv.Itoa(i), "slice.md"), []byte(sliceText(plan, string(builder), i))); err != nil {
			return err
		}
	}
	return nil
}

// writeText writes run's member text, whole as Daisy would post it
// uncut, to texts/<run>.md.
func (s *Swarm) writeText(n int, run, text string) error {
	return s.writeWorkspace(n, filepath.Join("texts", run+".md"), []byte(text))
}

// writeReviews writes to texts/ each swarm review on pull request p of
// issue n held for its round (section 7.8): those of a round that
// completed, the pair's two reviews on one head (section 6's rounds,
// roundsOf), and those of a round that can no longer complete, p closed
// or merged, its head moved past the round's or reopened since (answers,
// Reopenings: none read for a closed p). Each is read from its run's
// out.md as its collection posted it; one whose out.md is gone, its run
// pruned or collected on another machine, is not written, unless posted,
// for a replay, which then takes it from its post (postedText).
func (s *Swarm) writeReviews(n int, p forge.Pull, reviews []forge.Review, answers []time.Time, posted bool) error {
	for _, rd := range roundsOf(reviews, answers, s.SwarmLogin) {
		if _, complete := rd.completed(s.SwarmLogin); !complete && p.Open && rd.commit == p.HeadSHA && rd.answers == len(answers) {
			continue // its round may still complete
		}
		for _, rv := range rd.reviews {
			e := Entry{rv.Author, rv.Body, rv.Submitted}
			m, _ := Swarms(e, s.SwarmLogin)
			run := outputRun(n, m)
			text, ok, err := s.outputs().Text(run)
			switch {
			case err != nil:
				return err
			case !ok && !posted:
				continue
			case !ok:
				text, _ = postedText(n, m, post{Entry: e, review: true})
			}
			if err := s.writeText(n, run, text); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeHeld is writeReviews over each of the issue's pull requests, at
// the collection of a run on the open issue: a review whose round can no
// longer complete is written at the first such collection after that.
func (c *collection) writeHeld(pulls []forge.Pull) error {
	for _, p := range pulls {
		ref := forge.Ref{Repo: p.Repo, N: p.Number}
		reviews, err := c.s.Forge.Reviews(ref)
		if err != nil {
			return err
		}
		var answers []time.Time
		if p.Open {
			comments, err := c.s.Forge.PullComments(ref)
			if err != nil {
				return err
			}
			answers = Reopenings(pullComments(comments), c.s.SwarmLogin, c.s.Cfg.Trusted)
		}
		if err := c.s.writeReviews(c.r.N, p, reviews, answers, false); err != nil {
			return err
		}
	}
	return nil
}

// writeKept names the worker run and its fetched commit in its slice's
// kept (section 7.8): the slice's newest worker run whose commits a run
// may build on.
func (c *collection) writeKept() error {
	sl := strings.TrimPrefix(runBranch(c.lines), fmt.Sprintf("swarm/%d-", c.r.N))
	return c.s.writeWorkspace(c.r.N, filepath.Join("slices", sl, "kept"), []byte(fmt.Sprintf("%d-worker-%d %s\n", c.r.N, c.r.K, c.fetched())))
}

// writeCheck writes a check that failed on head to checks/<head>.log
// (section 7.8): its `# Check` line, then its whole output.
func (c *collection) writeCheck(head, line, out string) error {
	text := line + "\n"
	if out != "" {
		text += "\n" + out
	}
	return c.s.writeWorkspace(c.r.N, filepath.Join("checks", head+".log"), []byte(text))
}

// completed is when the round completed, its first review that gives it
// a review of each of the pair's roles (section 6's rounds), and whether
// it has.
func (rd round) completed(swarmLogin string) (time.Time, bool) {
	roles := map[string]bool{}
	for _, rv := range rd.reviews {
		m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, swarmLogin)
		if roles[m.Role] = true; roles["adversary"] && roles["sympathetic"] {
			return rv.Submitted, true
		}
	}
	return time.Time{}, false
}

// writeRound writes the round the run's review completes on pull request
// p, the issue's current one, to its slice's rounds/<r>.md (section 7.8,
// A round's file), r counting the slice's completed rounds, over the
// life of its pull requests, in the order they completed; nothing when
// the review is in no completed round. Each review is in one round, so
// no other run's collection writes the file. reviews, comments and
// answers are p's.
func (c *collection) writeRound(p forge.Pull, pulls []forge.Pull, reviews []forge.Review, comments []forge.Comment, answers []time.Time) error {
	s, sl := c.s, sliceOf(p, c.r.N)
	type done struct {
		round
		pull int
		at   time.Time
	}
	var all []done
	for _, q := range pulls {
		if q.Repo != p.Repo || sliceOf(q, c.r.N) != sl {
			continue
		}
		rs, as := reviews, answers
		if q.Number != p.Number {
			ref := forge.Ref{Repo: q.Repo, N: q.Number}
			var err error
			if rs, err = s.Forge.Reviews(ref); err != nil {
				return err
			}
			cs, err := s.Forge.PullComments(ref)
			if err != nil {
				return err
			}
			as = Reopenings(pullComments(cs), s.SwarmLogin, s.Cfg.Trusted)
		}
		for _, rd := range roundsOf(rs, as, s.SwarmLogin) {
			if at, ok := rd.completed(s.SwarmLogin); ok {
				all = append(all, done{rd, q.Number, at})
			}
		}
	}
	slices.SortStableFunc(all, func(a, b done) int { return a.at.Compare(b.at) })
	r := slices.IndexFunc(all, func(d done) bool {
		return d.pull == p.Number && slices.ContainsFunc(d.reviews, func(rv forge.Review) bool {
			m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
			return m.Role == c.r.Role && m.Run == c.r.K
		})
	}) + 1
	if r == 0 {
		return nil
	}
	cur := all[r-1]
	var b strings.Builder
	fmt.Fprintf(&b, "# Round %d on %s\n\nSlice %d, pull request %s#%d. The round before: ", r, cur.commit, sl, p.Repo, p.Number)
	if r == 1 {
		b.WriteString("none.\n")
	} else {
		fmt.Fprintf(&b, "%d on %s.\n", r-1, all[r-2].commit)
	}
	reviewsOf := slices.Clone(cur.reviews)
	adversary := func(rv forge.Review) bool {
		m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		return m.Role == "adversary"
	}
	slices.SortStableFunc(reviewsOf, func(x, y forge.Review) int { // the adversary's first
		switch {
		case adversary(x) && !adversary(y):
			return -1
		case adversary(y) && !adversary(x):
			return 1
		}
		return 0
	})
	for _, rv := range reviewsOf {
		m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
		run := outputRun(c.r.N, m)
		text, ok, err := s.outputs().Text(run)
		if err != nil {
			return err
		}
		if !ok {
			text = swarmText(rv.Body)
		}
		fmt.Fprintf(&b, "\n## %s, run %d: %s\n\nWhole: /job/memory/texts/%s.md\n", m.Role, m.Run, verdictOf(text), run)
		if f := linesBeginning(text, "[blocking]", "[non-blocking]"); len(f) > 0 {
			b.WriteString("\n" + strings.Join(f, "\n") + "\n")
		}
	}
	if r > 1 {
		prev := all[r-2]
		fmt.Fprintf(&b, "\n## Changes since round %d\n\n", r-1)
		if prev.commit == cur.commit {
			b.WriteString(c.noCommitSince(r-1, prev.at, cur.reviews[0].Submitted, comments))
		} else {
			diff, err := s.git().RoundDiff(p.Repo, s.Cfg.Base, prev.commit, cur.commit)
			var gone NotInMirror
			switch {
			case errors.As(err, &gone): // the round's transition must not wait on a commit no pass brings back
				fmt.Fprintf(&b, "The changes since round %d cannot be shown: %s is not in the mirror.\n", r-1, gone.Commit)
			case err != nil:
				return err
			case diff == "":
				fmt.Fprintf(&b, "No change since round %d in the files the pull request changes.\n", r-1)
			default:
				f := fenceFor(diff)
				fmt.Fprintf(&b, "%sdiff\n%s%s\n", f, diff, f)
			}
		}
	}
	return s.writeWorkspace(c.r.N, filepath.Join("slices", strconv.Itoa(sl), "rounds", strconv.Itoa(r)+".md"), []byte(b.String()))
}

// noCommitSince is the last section of a round's file on the same head as
// round prev, which completed at from, the round's first review submitted
// at to: the newest answer of the worker's without a commit between them
// on the pull request's thread, comments; else the maintainer's comment,
// the other reopening without a commit (section 6).
func (c *collection) noCommitSince(prev int, from, to time.Time, comments []forge.Comment) string {
	run, at := "", from
	for _, e := range pullComments(comments) {
		if m, ok := Swarms(e, c.s.SwarmLogin); ok && m.Role == "worker" && strings.HasPrefix(swarmText(e.Body), noChange) && e.Created.After(at) && e.Created.Before(to) {
			run, at = outputRun(c.r.N, m), e.Created
		}
	}
	if run == "" {
		return fmt.Sprintf("No commit since round %d: the maintainer commented in the thread.\n", prev)
	}
	return fmt.Sprintf("No commit since round %d: the worker answered without a commit, /job/memory/texts/%s.md.\n", prev, run)
}

// fenceFor is a run of backticks that opens and closes a fenced block
// around text: one longer than any run in it, and at least three.
func fenceFor(text string) string {
	n, run := 3, 0
	for _, ch := range text {
		if ch != '`' {
			run = 0
			continue
		}
		run++
		n = max(n, run+1)
	}
	return strings.Repeat("`", n)
}
