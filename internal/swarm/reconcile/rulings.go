package reconcile

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/swarm/forge"
)

// stands begins the architect's confirmation, on plan-1-accepted, that
// the remaining slices stand against the rulings changed since the plan
// (section 7.5): no plan, but the plan's new basis.
const stands = "STANDS:"

// rulingsTail ends the reconciler's note that rulings changed since the
// plan (section 6), which blocks nothing (handsOn).
const rulingsTail = "The architect revises the remaining slices or confirms they stand."

// rulingsHead begins that note's text.
const rulingsHead = "Rulings changed since the plan"

// basis is the architect's newest word on the accepted plan: the plan
// itself, or a STANDS: confirmation after it. The plan stands against
// the declared repositories as that run cloned them (its `start <name>
// <sha>` lines, section 7.1).
func (j *Job) basis() (Entry, Mark, bool) {
	return j.newestArchitect(func(string) bool { return true })
}

// replan is section 6's rulings check for issue j.N at plan-1-accepted
// between slices: the paths of each repository's rulings (RulingPaths)
// that changed between the commit the basis's run cloned and the
// repository's <base> as the pass fetched it, each `<name>:<path>`; none
// when there is no plan, or no repository declares rulings. A
// repository the run did not clone (declared since) is not compared.
// When some changed and no note of it follows the basis, the
// reconciler's note naming them is posted and j.Issue read again, so the
// turn reads it. A basis whose run, or its commit, cannot be read is a
// warning, and nothing changed.
func (s *Swarm) replan(j *Job) ([]string, error) {
	basis, m, ok := j.basis()
	if !ok {
		return nil, nil
	}
	ruled := false
	for _, r := range s.Cfg.Repos {
		ruled = ruled || len(r.RulingPaths()) > 0
	}
	if !ruled {
		return nil, nil
	}
	run := Run{Dir: filepath.Join(s.state(), "runs", fmt.Sprintf("%d-architect-%d", j.N, m.Run))}
	lines, err := run.Result(s.O.FS())
	if err == nil && lines == nil {
		err = fmt.Errorf("%s has no result", run.Dir)
	}
	if err != nil {
		fmt.Fprintf(s.Err, "⚠ #%d: rulings not compared: %v\n", j.N, err)
		return nil, nil
	}
	if err := s.fetch(); err != nil {
		return nil, err
	}
	var files []string
	for _, r := range s.Cfg.Repos {
		l, ok := Has(lines, "start "+r.Name)
		if len(r.RulingPaths()) == 0 || !ok {
			continue
		}
		changed, err := s.git().Since(r.Name, strings.TrimPrefix(l, "start "+r.Name+" "), s.Cfg.Base)
		if err != nil {
			fmt.Fprintf(s.Err, "⚠ #%d: rulings not compared: %v\n", j.N, err)
			return nil, nil
		}
		for _, p := range Protected(changed, r.RulingPaths()) {
			files = append(files, r.Name+":"+p)
		}
	}
	if len(files) == 0 {
		return files, nil
	}
	noted := s.rulingsNoted(j.Issue, basis)
	l := long{text: fmt.Sprintf("%s (architect run %d): `%s`.", rulingsHead, m.Run, strings.Join(files, "`, `")), tail: " " + rulingsTail,
		file: s.noteFile(j.N, "rulings"), what: "the rulings note"}
	at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: j.N}
	if err := s.postNote(j.N, at, j.Issue.Comments, marker("reconciler", m.Run), l, noted, fmt.Sprintf("#%d rulings note posted", j.N)); err != nil || noted != nil {
		return files, err
	}
	if j.Issue, err = s.Forge.Issue(j.N); err != nil {
		return nil, err
	}
	return files, nil
}

// rulingsNoted is the reconciler's note that rulings changed following
// basis on is, nil when there is none, so the rulings it names are
// noted once (a cut note keeps its last sentence, section 7.5).
func (s *Swarm) rulingsNoted(is forge.Issue, basis Entry) *forge.Comment {
	for i, c := range is.Comments {
		m, ok := Swarms(Entry{c.Author, c.Body, c.Created}, s.SwarmLogin)
		if ok && m.Role == "reconciler" && c.Created.After(basis.Created) && strings.HasPrefix(swarmText(c.Body), rulingsHead) && strings.HasSuffix(swarmText(c.Body), rulingsTail) {
			return &is.Comments[i]
		}
	}
	return nil
}
