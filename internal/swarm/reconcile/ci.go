package reconcile

import (
	"fmt"
	"slices"
	"strings"

	"daisy/internal/swarm/declare"
)

// CIDiffer is a declared repository whose CI differs from its declared
// check environment (section 7.9, The comparison with CI): its name and
// the comparison's lines.
type CIDiffer struct {
	Name  string
	Lines []string
}

// CIDiffers is the comparison with CI of each declared repository, in
// repos' order, read from its mirror at base as the last fetch left it
// (section 7.9): the repositories with a line, none for one whose mirror
// holds no base yet. It runs git's read commands alone, in the mirrors,
// and so writes nothing; an error is git's, naming the repository.
func CIDiffers(g Git, cfg declare.Config) ([]CIDiffer, error) {
	var out []CIDiffer
	for _, r := range cfg.Repos {
		lines, err := g.compareCI(cfg.Forge.Kind, r, cfg.Base)
		if err != nil {
			return nil, fmt.Errorf("comparing %s's CI: %w", r.Name, err)
		}
		if len(lines) > 0 {
			out = append(out, CIDiffer{r.Name, lines})
		}
	}
	return out, nil
}

// CIDifferNames is the names of ds, as status's Waiting for you lists
// them under `ci differs` and notify tells them.
func CIDifferNames(ds []CIDiffer) []string {
	names := []string{}
	for _, d := range ds {
		names = append(names, d.Name)
	}
	return names
}

// compareCI is r's comparison with CI at base in its mirror: the
// workflow files the kind runs (declare.WorkflowFiles, of the regular
// files in its directories) and the test services file, at the one
// commit; a refused services file is the refusal's line, the comparison
// made without it (declare.CompareCIRefused).
func (g Git) compareCI(kind string, r declare.Repo, base string) ([]string, error) {
	m := g.Mirror(r.Name)
	commit := g.Head(m, "refs/heads/"+base)
	if commit == "" {
		return nil, nil
	}
	var paths []string
	objects := map[string]string{}
	for _, dir := range declare.WorkflowDirs(kind) {
		entries, err := g.Tree(m, commit, strings.TrimSuffix(dir, "/"))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type == "blob" && (e.Mode == "100644" || e.Mode == "100755") {
				paths = append(paths, e.Path)
				objects[e.Path] = e.Object
			}
		}
	}
	var files []declare.Workflow
	for _, p := range declare.WorkflowFiles(kind, paths) {
		data, err := g.Blob(m, objects[p])
		if err != nil {
			return nil, err
		}
		files = append(files, declare.Workflow{Path: p, Data: data})
	}
	ts, ok, refused, err := g.TestServices(r, commit)
	switch {
	case err != nil:
		return nil, err
	case refused != "":
		return declare.CompareCIRefused(kind, r, strings.TrimPrefix(refused, r.ServicesPath()+": "), files), nil
	case ok:
		return declare.CompareCI(kind, r, &ts, files), nil
	}
	return declare.CompareCI(kind, r, nil, files), nil
}

// warnCI is apply's telling of the comparison with CI (section 7.9), in
// step (8) before notify, read from the mirrors as the pass fetched them
// (as an earlier pass did, when this one fetched none): every line on
// stderr at the process's first pass and again only when the set of
// lines changes, as warnUnchecked's are. It keeps the repositories for
// notify; a read git refuses is one warning, the lines last told and the
// repositories kept.
func (s *Swarm) warnCI() {
	ds, err := CIDiffers(s.git(), s.Cfg)
	if err != nil {
		s.ciErr = err
		fmt.Fprintf(s.Err, "⚠ %v\n", err)
		return
	}
	s.ci, s.ciErr = ds, nil
	var lines []string
	for _, d := range ds {
		lines = append(lines, d.Lines...)
	}
	if slices.Equal(lines, s.ciTold) {
		return
	}
	s.ciTold = lines
	printLines(s.Err, lines)
}
