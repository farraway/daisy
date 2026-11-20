package metrics

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// A revert is git's: a commit whose message holds the line `This
// reverts commit <sha>` that `git revert` writes (with `, reversing
// changes made to <parent>.` after a merge's), <sha> naming the commit
// it takes back. A change taken back by hand, under another message, is
// not one. A revert that a later revert takes back (a reapply, git's
// `Reapply "…"`) no longer takes its commit back.
var revertLine = regexp.MustCompile(`(?m)^This reverts commit ([0-9a-f]{7,64})\b`)

// Named is the commits the reverts in effect in a history name, each as
// its message gives it, in full or abbreviated.
type Named []string

// Has is whether a revert names the commit sha (in full).
func (n Named) Has(sha string) bool {
	return slices.ContainsFunc(n, func(p string) bool { return strings.HasPrefix(sha, p) })
}

// Reverted is the commits the reverts in effect in rev's history name,
// in the repository at dir: `git log -E --grep` for the line, each
// revert read with its own sha. A revert is in effect unless a revert in
// effect names it, so a chain is followed: a commit reverted, reapplied
// and reverted again is reverted.
func Reverted(o ops.Ops, dir, rev string) (Named, error) {
	out, stderr, err := o.Exec(dir, "git", "log", "-E", "--grep=^This reverts commit [0-9a-f]{7,64}", "--topo-order", "--format=%x00%H%n%B", rev, "--")
	if err != nil {
		return nil, fmt.Errorf("git log %s in %s failed: %w: %s", rev, dir, err, strings.TrimSpace(string(stderr)))
	}
	names := map[string][]string{} // a revert's sha -> the commits it names
	var reverts []string           // newest first: git log --topo-order lists a commit before its parents
	for _, entry := range strings.Split(string(out), "\x00")[1:] {
		sha, body, _ := strings.Cut(entry, "\n")
		reverts = append(reverts, sha)
		for _, m := range revertLine.FindAllStringSubmatch(body, -1) {
			names[sha] = append(names[sha], m[1])
		}
	}
	// A revert names only commits older than itself, so walking from the
	// newest, every revert that could take one back is settled before it.
	cancelled := map[string]bool{}
	for _, r := range reverts {
		if cancelled[r] {
			continue
		}
		for _, p := range names[r] {
			for _, older := range reverts {
				if strings.HasPrefix(older, p) {
					cancelled[older] = true
				}
			}
		}
	}
	var n Named
	for _, r := range reverts {
		if !cancelled[r] {
			n = append(n, names[r]...)
		}
	}
	return n, nil
}

// StepReverts reads whether a plan step's commits were reverted: its
// range is looked up in Dirs in order, the first repository whose git
// reads it holding the step, and the step is reverted when a revert in
// that repository's HEAD names one of the range's commits.
type StepReverts struct {
	O     ops.Ops
	Dirs  []string
	named map[string]Named // each repository's, read once
}

// Check is the repository in Dirs that holds rng, "" when none does (or
// rng is no range CheckRange takes), and whether the step is reverted
// there.
func (s *StepReverts) Check(rng string) (dir string, reverted bool, err error) {
	if CheckRange(rng) != nil {
		return "", false, nil
	}
	for _, d := range s.Dirs {
		out, _, err := s.O.Exec(d, "git", "log", "--format=%H", rng, "--")
		if err != nil {
			continue // not this repository's
		}
		if s.named == nil {
			s.named = map[string]Named{}
		}
		named, ok := s.named[d]
		if !ok {
			if named, err = Reverted(s.O, d, "HEAD"); err != nil {
				return d, false, err
			}
			s.named[d] = named
		}
		return d, slices.ContainsFunc(strings.Fields(string(out)), named.Has), nil
	}
	return "", false, nil
}
