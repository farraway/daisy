package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/swarm/forge"
)

// causes are the words a member's stop names its cause by (section 7.5,
// The stop's cause), the 2026-11-19 roadmap's fixed vocabulary in its
// order, each with the gloss the Task text gives it.
var causes = []struct{ Word, Gloss string }{
	{"plan", "the plan, or the issue where there is none, is wrong, unclear or cannot be followed"},
	{"prerequisite", "work the job depends on is not merged"},
	{"main-red", "the base branch is broken: its check fails without your change"},
	{"environment", "a tool, the image, a service or the network the run needs fails or is missing"},
	{"forge-fact", "a fact on the forge you need cannot be read"},
	{"kept-work", "work Daisy was to keep or give you is missing: a plan cut short, a kept commit not given back"},
	{"ruling", "only the maintainer's ruling settles it"},
	{"scope", "whether you may change what the plan or the job leaves out"},
	{"capacity", "a provider's limit, quota or credit stopped the work"},
}

// causeSentence ends the closing paragraph of every Task text (section
// 7.4): the Cause: line asked for under a stop's line, and the words.
var causeSentence = func() string {
	glossed := make([]string, len(causes))
	for i, c := range causes {
		glossed[i] = fmt.Sprintf("`%s` (%s)", c.Word, c.Gloss)
	}
	glossed[len(glossed)-1] = "or " + glossed[len(glossed)-1]
	return "Write one line `Cause: <word>` under any line beginning `BLOCKED:` or `QUESTION:` that you write, even one this text calls the only line, the word naming why you stopped: " +
		strings.Join(glossed, ", ") + "."
}()

// CauseWords is the vocabulary as a warning or a usage names it.
func CauseWords() string {
	words := make([]string, len(causes))
	for i, c := range causes {
		words[i] = c.Word
	}
	return strings.Join(words, ", ")
}

// IsCause: word is one of causes.
func IsCause(word string) bool {
	for _, c := range causes {
		if c.Word == word {
			return true
		}
	}
	return false
}

// stopCause is the word of the Cause: line under a stop's line (section
// 7.5, The stop's cause): the stop's line the output's first that is
// neither blank nor a Met: or Triaged: line (the batch's report, before
// it), and
// beginning BLOCKED: or QUESTION:; the Cause: line the next line after
// it that is not blank. found is false when there is no such line.
func stopCause(out string) (word string, found bool) {
	lines := strings.Split(out, "\n")
	i := 0
	for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || reportLine(strings.TrimRight(lines[i], " "))) {
		i++
	}
	if i == len(lines) || !strings.HasPrefix(lines[i], "BLOCKED:") && !strings.HasPrefix(lines[i], question) {
		return "", false
	}
	for i++; i < len(lines) && strings.TrimSpace(lines[i]) == ""; i++ {
	}
	if i == len(lines) {
		return "", false
	}
	rest, ok := strings.CutPrefix(lines[i], "Cause:")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// cause is the row's cause (section 7.6, The run's record): for a
// member's stop (outcome blocked or question), the word of its out.md's
// Cause: line; for a stop of Daisy's own (outcome fault or failed), the
// word its reason names (ownCause); "" for any other run, for a stop
// without the line, and, with a warning, for a word not among causes or
// an out.md that cannot be read.
func (c *collection) cause(outcome string) string {
	switch outcome {
	case "fault", "failed":
		l, _ := Has(c.lines, outcome)
		return ownCause(outcome, strings.TrimPrefix(l, outcome+": "))
	case "blocked", "question":
	default:
		return ""
	}
	out, err := c.r.Output(c.s.O.FS())
	if err != nil {
		if !errors.Is(err, iofs.ErrNotExist) {
			fmt.Fprintf(c.s.Err, "⚠ %scause left empty: %v\n", c.head, err)
		}
		return ""
	}
	word, found := stopCause(string(out))
	if !found {
		return ""
	}
	if !IsCause(word) {
		fmt.Fprintf(c.s.Err, "⚠ %scause left empty: %q is not one of %s\n", c.head, word, CauseWords())
		return ""
	}
	return word
}

// failedCauses are the work failures of section 7.7 that name a cause
// (section 7.5, Daisy's own stops), by their reason's beginning; any
// other failure (a timeout, a non-zero exit, no output, no commits, a
// bad verdict, a secret in the output, a rejected push, a clone reaching
// outside itself, nothing proposed) has no word among causes.
var failedCauses = []struct{ reason, word string }{
	{"bad slices: ", "plan"},
	{"protected path changed: ", "scope"},
	{"fetch: ", "environment"},
	{checkEnvironment, "environment"},
}

// ownCause is the cause of a stop of Daisy's own, a run's fault: or
// failed: line's reason (section 7.5): a fault is the machine's,
// `environment`, but for a provider's limit, quota or credit,
// `capacity`; a failure's as failedCauses names it, else "".
func ownCause(outcome, reason string) string {
	if outcome == "fault" {
		if _, ok := capacityFault(reason); ok {
			return "capacity"
		}
		return "environment"
	}
	for _, f := range failedCauses {
		if strings.HasPrefix(reason, f.reason) {
			return f.word
		}
	}
	return ""
}

// Retag is the row `swarm unblock --cause` writes (section 11, unblock):
// in the metrics CSV File, the held row of the run whose stop the
// unblock releases, and the row superseding it.
type Retag struct {
	File string
	Change
}

// Retag makes, writing nothing, the row superseding the last row of the
// run whose stop set issue n's swarm:blocked (blockingRun) in the metrics
// CSV of the home <state>/home names: the held row with cause word and
// its notes ending `retagged`, every other cell carried over, as a
// reprice carries them; the held row itself when its cause is word
// already. An error when no run's stop set the block, when no usable
// home is named, or when the CSV holds no row of the run.
func (s *Swarm) Retag(n int, word string) (Retag, error) {
	r, err := s.blockingRun(n)
	if err != nil {
		return Retag{}, err
	}
	var t Retag
	var before []byte
	if t.File, before, err = s.metricsCSV(); err != nil {
		return t, err
	}
	latest, err := metrics.Latest(before)
	if err != nil {
		return t, fmt.Errorf("cannot record in %s: %v", t.File, err)
	}
	held, ok := latest[s.runKey(r).Key()]
	if !ok {
		return t, fmt.Errorf("%s holds no row of run %s; write it with swarm metrics --backfill first", t.File, filepath.Base(r.Dir))
	}
	t.From, t.To = held, held
	if held.Cause != word {
		t.To.Cause = word
		if !strings.HasSuffix(t.To.Notes, ", retagged") {
			t.To.Notes += ", retagged"
		}
	}
	return t, nil
}

// WriteRetag writes t's row under the metrics lock by the CSV's rule for
// a correction (metrics.Supersede): how many rows it wrote, none when
// the CSV's last row of the run is it already.
func (s *Swarm) WriteRetag(t Retag) (int, error) {
	return s.locked(metrics.Supersede, t.File, "swarm unblock", t.To)
}

// blockingRun is the run whose stop set issue n's swarm:blocked: of the
// issue's collected runs whose collection flagged it (`flagged
// swarm:blocked`), the one that ended last, unless a run of the issue
// started after it ended, the block it set having been lifted since (a
// block set again by the merge step or by hand, which no run's row
// records). An error, saying so, when there is none.
func (s *Swarm) blockingRun(n int) (Run, error) {
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return Run{}, err
	}
	var last Run
	var ended time.Time
	var starts []struct {
		r Run
		t time.Time
	}
	for _, r := range runs {
		if r.N != n {
			continue
		}
		lines, err := r.Result(fs)
		if err != nil {
			return Run{}, err
		}
		if t, ok := lineTime(lines, "started"); ok {
			starts = append(starts, struct {
				r Run
				t time.Time
			}{r, t})
		}
		if _, collected := Has(lines, "collected"); !collected || !slices.Contains(lines, "flagged "+forge.Blocked) {
			continue
		}
		if t, ok := endTime(lines); ok && (last.Dir == "" || t.After(ended)) {
			last, ended = r, t
		}
	}
	none := "no run's stop set this block (%s); unblock it without --cause"
	if last.Dir == "" {
		return Run{}, fmt.Errorf(none, "no run of the issue flagged it "+forge.Blocked)
	}
	for _, st := range starts {
		if st.t.After(ended) {
			return Run{}, fmt.Errorf(none, fmt.Sprintf("run %s, whose stop flagged it, ended before run %s started: the block was set again since, by the merge step or by hand", filepath.Base(last.Dir), filepath.Base(st.r.Dir)))
		}
	}
	return last, nil
}
