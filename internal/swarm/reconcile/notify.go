package reconcile

import (
	"bytes"
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

// Maintainer is the login the reconciler's notes mention (section 5, The
// maintainer's mention): the first of trusted that is not the swarm's;
// "" when there is none.
func Maintainer(trusted []string, swarmLogin string) string {
	for _, l := range trusted {
		if !strings.EqualFold(l, swarmLogin) {
			return l
		}
	}
	return ""
}

// mention ends a reconciler note that puts its issue in Waiting for you:
// a blank line and `cc @<maintainer>`, so the forge notifies the
// maintainer; "" when trusted names no maintainer. It follows the note's
// text, whose first line and start every reader of a note reads.
func (s *Swarm) mention() string {
	if m := Maintainer(s.Cfg.Trusted, s.SwarmLogin); m != "" {
		return "\n\ncc @" + m
	}
	return ""
}

// Waiting is what gated issue is, read with its comments, waits on the
// maintainer for, as `status`'s Waiting for you names it (section 11):
// `question` (a question passed to the maintainer) or `blocked` at any
// stage, else `triage`, `plan`, or at review-2-approved what its current
// pull request calls for (AfterApproval, of pulls, its pull requests);
// "" for an issue with no stage, or two. The plan is read from the job's
// workspace through o (planSlices).
func Waiting(is forge.Issue, pulls []forge.Pull, swarmLogin string, o Outputs) (string, error) {
	stage, ok := StageOf(is)
	switch {
	case !ok:
		return "", nil
	case slices.Contains(is.Labels, forge.Blocked) && AtAQuestion(is, swarmLogin):
		return "question", nil
	case slices.Contains(is.Labels, forge.Blocked):
		return "blocked", nil
	case stage == forge.ReviewApproved:
		return AfterApproval(is, pulls, swarmLogin, o)
	}
	return map[string]string{forge.ProposalPosted: "triage", forge.PlanPosted: "plan"}[stage], nil
}

// NoCheck is the declared repositories, by name in repos' order, whose
// pull requests the merge step merges with no check run on them
// (declare's Unchecked), as status's Waiting for you lists them under
// `no check` and notify tells them.
func NoCheck(cfg declare.Config) []string {
	var names []string
	for _, i := range cfg.Unchecked() {
		names = append(names, cfg.Repos[i].Name)
	}
	return names
}

// waitItem is one thing waiting for the maintainer, as notify tells it:
// key, its line in <state>/notified, and message, notify's stdin.
type waitItem struct{ key, message string }

// notify is the last of pass step (8) (section 5, Notify): with notify
// declared, each item waiting for the maintainer (waiting) that
// <state>/notified does not hold is told, notify run once with its line
// on stdin, then the record written as the items waiting now, so an item
// that left is told again when it comes back. Every failure is a
// warning; a read that fails tells nothing and keeps the record.
func (s *Swarm) notify() {
	if len(s.Cfg.Notify) == 0 {
		return
	}
	warn := func(format string, a ...any) { fmt.Fprintf(s.Err, "⚠ "+format+"\n", a...) }
	items, err := s.waiting()
	if err != nil {
		warn("notify: %v; nothing told this pass", err)
		return
	}
	fs := s.O.FS()
	path := filepath.Join(s.state(), "notified")
	b, err := fs.ReadFile(path)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		warn("notify: %v; nothing told this pass", err)
		return
	}
	told := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	var keep []string
	for _, it := range items {
		keep = append(keep, it.key)
		if slices.Contains(told, it.key) {
			continue
		}
		if err := s.tell(it.message); err != nil {
			warn("notify: %s: %v", it.message, err)
			continue
		}
		fmt.Fprintf(s.Out, "Notified: %s\n", it.message)
	}
	record := strings.Join(keep, "\n")
	if record != "" {
		record += "\n"
	}
	if record == string(b) {
		return
	}
	if err := fs.WriteFile(path, []byte(record), 0o644); err != nil {
		warn("notify: %v", err)
	}
}

// waiting is what waits for the maintainer after the pass's flags, as
// notify tells it: a pause the swarm took on two faults (its
// <state>/paused line, which `status` shows); each line of
// <state>/capacity, by the line (section 7.10); each open issue carrying
// swarm:gated, by number, its item Waiting's and its key naming the
// reconciler's note that put it there (waitNote), so a new block or a
// new gating event is a new item; each notes branch `status` lists; each
// repository it lists under `no check` (NoCheck), and under `ci differs`
// (as warnCI read them this pass), by its name.
func (s *Swarm) waiting() ([]waitItem, error) {
	repo := s.Cfg.Forge.Repo
	var out []waitItem
	if l := s.paused(); strings.Contains(l, ": swarm fault: ") {
		out = append(out, waitItem{l, repo + " waits for you: " + l})
	}
	capacity, err := ReadCapacity(s.O.FS(), s.state())
	if err != nil {
		return nil, err
	}
	for _, u := range capacity { // known by the line (section 7.10, The note)
		out = append(out, waitItem{u.String(), fmt.Sprintf("%s capacity: %s %s unavailable until %s: %s", repo, u.Credential, u.Model, u.Until, u.Reason)})
	}
	gated, err := s.Forge.Issues(forge.Open, forge.Gated)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(gated, func(a, b forge.Issue) int { return a.Number - b.Number })
	for _, listed := range gated {
		n := listed.Number
		is, err := s.Forge.Issue(n)
		if err != nil {
			return nil, err
		}
		if !is.Open || !slices.Contains(is.Labels, forge.Gated) {
			continue
		}
		var pulls []forge.Pull
		if stage, _ := StageOf(is); stage == forge.ReviewApproved && !slices.Contains(is.Labels, forge.Blocked) {
			if pulls, err = s.pulls(n); err != nil {
				return nil, err
			}
		}
		item, err := Waiting(is, pulls, s.SwarmLogin, s.outputs())
		if err != nil {
			return nil, err
		}
		if item == "" {
			continue
		}
		out = append(out, waitItem{fmt.Sprintf("#%d %s %d", n, item, waitNote(is, item, s.SwarmLogin)),
			fmt.Sprintf("%s#%d waits for you: %s: %s", repo, n, item, oneLine.Replace(is.Title))})
	}
	branches, err := NotesBranches(s.git(), repo)
	if err != nil {
		return nil, err
	}
	for _, b := range branches {
		out = append(out, waitItem{"notes " + b, repo + " waits for you: notes " + b})
	}
	for _, name := range NoCheck(s.Cfg) {
		out = append(out, waitItem{"no check " + name, repo + " waits for you: no check " + name})
	}
	if s.ciErr != nil {
		return nil, s.ciErr
	}
	for _, name := range CIDifferNames(s.ci) {
		out = append(out, waitItem{"ci differs " + name, repo + " waits for you: ci differs " + name})
	}
	return out, nil
}

// waitNote is the id of the reconciler's newest note on is that put it
// in Waiting for you as item: the note that blocked it (blocks) for
// `blocked` and `question`, the gate note for the gates; 0 when none is
// on it (the flag set by hand, or the note refused).
func waitNote(is forge.Issue, item, swarmLogin string) int64 {
	var id int64
	var at time.Time
	for _, c := range is.Comments {
		m, ok := Swarms(Entry{c.Author, c.Body, c.Created}, swarmLogin)
		if !ok || c.Created.Before(at) {
			continue
		}
		if item == "blocked" || item == "question" {
			ok = blocks(m, c.Body)
		} else {
			ok = m.Kind == "gate"
		}
		if ok {
			id, at = c.ID, c.Created
		}
	}
	return id
}

// tell runs notify on the host, in the swarm's clone, with message as
// its one line on stdin and apply's environment, stopped once it has
// run for the swarm's timeout; its output is shown only when it fails.
func (s *Swarm) tell(message string) error {
	var out bytes.Buffer
	p, err := s.O.Start(s.Place.Root, s.Cfg.Notify[0], strings.NewReader(message+"\n"), &out, &out, s.Cfg.Notify[1:]...)
	if err != nil {
		return err
	}
	type exit struct {
		code int
		err  error
	}
	done := make(chan exit, 1)
	go func() { code, err := p.Wait(); done <- exit{code, err} }()
	timeout, _ := time.ParseDuration(s.Cfg.Timeout) // validated
	select {
	case e := <-done:
		said := oneLine.Replace(strings.TrimSpace(out.String()))
		switch {
		case e.err != nil:
			err = e.err
		case e.code != 0:
			err = fmt.Errorf("exit status %d", e.code)
		default:
			return nil
		}
		if said != "" {
			return fmt.Errorf("%w: %s", err, said)
		}
		return err
	case <-s.O.After(timeout):
		// Its output is left to the waiting goroutine: a child it
		// started may hold the pipe open past the kill.
		if err := p.Kill(); err != nil {
			return fmt.Errorf("not stopped after %s: %w", s.Cfg.Timeout, err)
		}
		return fmt.Errorf("stopped after %s", s.Cfg.Timeout)
	}
}
