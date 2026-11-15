package reconcile

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/runner"
)

// RestartFile is <state>/restart, `daisy swarm restart`'s request that the
// running apply re-execute itself at its first idle pass (section 11, The
// restart).
func RestartFile(state string) string { return filepath.Join(state, "restart") }

// Idle: nothing of this swarm is in flight, so a re-exec loses nothing a
// pass would collect: no container this process started, none of this
// swarm's in the running set, and no run that started or ended without
// `collected` (rule 2's holders, on no issue).
func (s *Swarm) Idle() (bool, error) {
	if len(s.live) > 0 {
		return false, nil
	}
	running, err := s.Runner.Running()
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(running, func(c runner.Running) bool { return strings.HasPrefix(c.Swarm, s.Cfg.Forge.Repo+"#") }) {
		return false, nil
	}
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return false, err
	}
	for _, r := range runs {
		lines, err := r.Result(fs)
		if err != nil {
			return false, err
		}
		if inFlight(lines) {
			return false, nil
		}
	}
	return true, nil
}

// inFlight: a run's result says it started or ended, and it was not
// collected (rule 2's holders).
func inFlight(lines []string) bool {
	_, started := Has(lines, "started")
	_, done := Has(lines, "collected")
	return !done && (started || Ended(lines) != "")
}

// InFlight is the runs on issue n under state that are in flight
// (inFlight), so a maintainer's verb waits for them (section 11, swarm
// replan).
func InFlight(fs ops.FS, state string, n int) ([]Run, error) {
	runs, err := Runs(fs, state)
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, r := range runs {
		if r.N != n {
			continue
		}
		lines, err := r.Result(fs)
		if err != nil {
			return nil, err
		}
		if inFlight(lines) {
			out = append(out, r)
		}
	}
	return out, nil
}

// QueueJournal queues the journal line `- <HHMM> <key>: <what>` for the
// next pass's push (section 7.6, The run's record), the time UTC; key
// names the line, as a run's name does, and no other line takes it.
func (s *Swarm) QueueJournal(key, what string) error {
	now := s.O.Now().UTC()
	return s.queue(now.Format("2006-01-02"), fmt.Sprintf("- %s %s: %s", now.Format("1504"), key, what))
}
