package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"strconv"
	"strings"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/ops"
)

// lastIntervention is the highest of the intervention types (section 11,
// The interventions' record): the maintainer's catalogue's I1 to I17, a
// fixed range as the stops' causes are a fixed vocabulary.
const lastIntervention = 17

// InterventionTypes is the types as a usage names them.
func InterventionTypes() string { return "I1 to I" + strconv.Itoa(lastIntervention) }

// IsInterventionType: word is one of the types, `I` and a number from 1
// to lastIntervention written as strconv writes it.
func IsInterventionType(word string) bool {
	k, err := strconv.Atoi(strings.TrimPrefix(word, "I"))
	return strings.HasPrefix(word, "I") && err == nil && k >= 1 && k <= lastIntervention && "I"+strconv.Itoa(k) == word
}

// Intervention is one of the maintainer's interventions as recorded: its
// type, the issue it was on (0 for none), the verb that made it and the
// maintainer's note (intervene's).
type Intervention struct {
	Type, Verb, Note string
	N                int
}

// InterventionCSV is the metrics CSV an intervention's row goes to, the
// one of the home <state>/home names, an error when it names no usable
// home or the CSV cannot be read or is not this Daisy's: a verb checks it
// before its writes.
func (s *Swarm) InterventionCSV() (string, error) {
	file, b, err := s.metricsCSV()
	if err != nil {
		return file, err
	}
	if _, err := metrics.Keys(b); err != nil {
		return file, fmt.Errorf("cannot record in %s: %v", file, err)
	}
	return file, nil
}

// RecordIntervention records i (section 11, The interventions' record):
// its row in the metrics CSV, of kind intervention, run
// `<n>-intervention-<k>` (k one more than the highest of the issue's the
// CSV's rows and the journal name), under the metrics lock, then its
// journal line queued for the next pass's push. The run's name.
func (s *Swarm) RecordIntervention(i Intervention) (string, error) {
	file, err := s.InterventionCSV()
	if err != nil {
		return "", err
	}
	journal, err := ReadJournal(s.O.FS(), s.state())
	if err != nil {
		return "", fmt.Errorf("cannot read the journal: %v", err)
	}
	now := s.O.Now().UTC()
	var on []string
	if i.Note != "" {
		on = append(on, i.Note)
	}
	if i.N > 0 {
		on = append(on, fmt.Sprintf("#%d", i.N))
	}
	home, _ := s.metricsHome()
	row := metrics.Row{When: now.Format(time.RFC3339), Kind: metrics.Intervention, Source: s.Cfg.Forge.Repo, Role: "maintainer",
		Outcome: i.Verb, Cause: i.Type, Notes: strings.Join(on, ", ")}
	row.RootSHA, row.HomeSHA = s.headOf("intervention: ", s.Root, "root_sha"), s.headOf("intervention: ", home, "home_sha")
	prefix := fmt.Sprintf("%d-intervention-", i.N)
	var unit string
	_, err = s.locked(func(fs ops.FS, file string, rows ...metrics.Row) (int, error) {
		b, err := fs.ReadFile(file)
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return 0, err
		}
		latest, err := metrics.Latest(b)
		if err != nil {
			return 0, err
		}
		names := []string{}
		for _, r := range latest {
			if r.Kind == metrics.Intervention && r.Source == s.Cfg.Forge.Repo {
				names = append(names, r.Unit)
			}
		}
		for _, l := range journal {
			names = append(names, l.Run)
		}
		k := 0
		for _, name := range names {
			if n, err := strconv.Atoi(strings.TrimPrefix(name, prefix)); strings.HasPrefix(name, prefix) && err == nil {
				k = max(k, n)
			}
		}
		unit = prefix + strconv.Itoa(k+1)
		rows[0].Unit = unit
		return metrics.Append(fs, file, rows...)
	}, file, "swarm "+i.Verb, row)
	if err != nil {
		return "", fmt.Errorf("cannot record in %s: %v", file, err)
	}
	line := fmt.Sprintf("- %s %s: %s", now.Format("1504"), unit, strings.Join(append([]string{i.Type, i.Verb}, on...), "; "))
	if err := s.queue(now.Format("2006-01-02"), line); err != nil {
		return "", fmt.Errorf("its row is written in %s, but its journal line is not queued in %s: %v", file, s.journalQueue(), err)
	}
	return unit, nil
}
