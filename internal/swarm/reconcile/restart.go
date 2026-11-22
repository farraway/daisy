package reconcile

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// RestartFile is <state>/restart, `daisy swarm restart`'s request that the
// running apply re-execute itself at its first idle pass (section 11, The
// restart).
func RestartFile(state string) string { return filepath.Join(state, "restart") }

// Holding is how many of this swarm's containers a new read of the
// running set holds: the members a restart leaves running for the
// process it starts, which holds each by its run's record (section 7.10,
// rule 2).
func (s *Swarm) Holding() (int, error) {
	running, err := s.Runner.Running()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range running {
		if strings.HasPrefix(c.Swarm, s.Cfg.Forge.Repo+"#") {
			n++
		}
	}
	return n, nil
}

// Record writes the end line of each container this process holds whose
// wait has returned, before the process ends or is replaced: the next
// apply then collects it as it ended (a timeout its timer stopped among
// them) rather than abandoned, its record being all it holds of it.
func (s *Swarm) Record() error {
	var errs []error
	for _, dir := range slices.Sorted(maps.Keys(s.live)) {
		select {
		case <-s.live[dir].done:
			r, _ := runOf(dir)
			errs = append(errs, s.settle(r))
		default:
		}
	}
	return errors.Join(errs...)
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
