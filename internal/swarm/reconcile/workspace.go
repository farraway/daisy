package reconcile

import (
	"bytes"
	"errors"
	iofs "io/fs"
	"path/filepath"
	"strconv"
	"time"

	"daisy/internal/swarm/forge"
)

// The job's workspace, <state>/jobs/<n> (section 7.8), written by the
// collection of a run on issue n alone. Of its files this build writes
// texts/, plan.md, builder.md and slices/<s>/slice.md, and nothing reads
// them yet.

// workspace is issue n's workspace.
func (s *Swarm) workspace(n int) string { return filepath.Join(s.state(), "jobs", strconv.Itoa(n)) }

// writeWorkspace writes data to the file rel of issue n's workspace,
// whole under a temporary name in its directory and then renamed into
// place (ops.FS's WriteFile), unless the file holds data already, so a
// collection retried writes it once (section 7.6).
func (s *Swarm) writeWorkspace(n int, rel string, data []byte) error {
	fs := s.O.FS()
	p := filepath.Join(s.workspace(n), rel)
	old, err := fs.ReadFile(p)
	switch {
	case err == nil && bytes.Equal(old, data):
		return nil
	case err != nil && !errors.Is(err, iofs.ErrNotExist):
		return err
	}
	if err := fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return fs.WriteFile(p, data, 0o644)
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
// pruned or collected on another machine, is not written.
func (s *Swarm) writeReviews(n int, p forge.Pull, reviews []forge.Review, answers []time.Time) error {
	for _, rd := range roundsOf(reviews, answers, s.SwarmLogin) {
		roles := map[string]bool{}
		for _, rv := range rd.reviews {
			m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
			roles[m.Role] = true
		}
		complete := roles["adversary"] && roles["sympathetic"]
		if !complete && p.Open && rd.commit == p.HeadSHA && rd.answers == len(answers) {
			continue // its round may still complete
		}
		for _, rv := range rd.reviews {
			m, _ := Swarms(Entry{rv.Author, rv.Body, rv.Submitted}, s.SwarmLogin)
			run := outputRun(n, m)
			text, ok, err := s.outputs().Text(run)
			if err != nil {
				return err
			}
			if !ok {
				continue
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
		if err := c.s.writeReviews(c.r.N, p, reviews, answers); err != nil {
			return err
		}
	}
	return nil
}
