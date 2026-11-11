package reconcile

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
)

// driftPaths are what the clone's working tree gives the swarm and its
// pushed head may not (section 11, Drift): the configuration.
var driftPaths = []string{"swarm.json", "prompts"}

// Drift is a swarm repository's clone's configuration compared with the
// swarm repository's default branch in its mirror (section 11, Drift):
// the branch and its commit, and each path that differs, in path order,
// with its line. Commit is "" when the mirror holds no commit (none
// fetched yet), and nothing is compared.
type Drift struct {
	Repo, Branch, Commit string
	Paths, Lines         []string
}

// Cleared is the line `apply` prints once no path drifts any more.
func (d Drift) Cleared() string {
	return fmt.Sprintf("✓ Drift cleared: swarm.json and prompts/ are %s's %s at %s", d.Repo, d.Branch, short(d.Commit))
}

// ConfigDrift compares the clone's swarm.json and every file under its
// prompts/ that git does not ignore with the same paths at the default
// branch in the swarm repository's mirror, by blob: `git hash-object` of
// the clone's file (a link by its target's text, as git stores one)
// against `git ls-tree` of the commit. It runs only git's reading
// commands, so status may run it under ReadOnly.
func ConfigDrift(g Git, repo string) (Drift, error) {
	d := Drift{Repo: repo}
	m := g.Mirror(repoName(repo))
	if d.Commit = g.Head(m, "HEAD"); d.Commit == "" {
		return d, nil
	}
	branch, err := g.run(m, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return d, err
	}
	d.Branch = branch

	pushed := map[string]TreeEntry{}
	for _, p := range driftPaths {
		entries, err := g.Tree(m, d.Commit, p)
		if err != nil {
			return d, err
		}
		for _, e := range entries {
			if e.Type != "tree" {
				pushed[e.Path] = e
			}
		}
	}

	out, errb, err := g.O.Exec(g.WS, "git", append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, driftPaths...)...)
	if err != nil {
		return d, fmt.Errorf("git ls-files: %v: %s", err, strings.TrimSpace(string(errb)))
	}
	fs := g.O.FS()
	local := map[string]bool{}
	var files []string
	links := map[string]string{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" || local[p] {
			continue
		}
		fi, err := fs.Lstat(filepath.Join(g.WS, p))
		switch {
		case err != nil || fi.IsDir():
			continue // deleted from the working tree, or a submodule
		case fi.Mode().IsRegular():
			files = append(files, p)
		default:
			target, err := fs.Readlink(filepath.Join(g.WS, p))
			if err != nil {
				continue
			}
			links[p] = target
		}
		local[p] = true
	}
	blobs := map[string]string{}
	if len(files) > 0 {
		h, err := g.run(g.WS, append([]string{"hash-object", "--"}, files...)...)
		if err != nil {
			return d, err
		}
		for i, sha := range strings.Split(h, "\n") {
			if i < len(files) {
				blobs[files[i]] = sha
			}
		}
	}

	at := fmt.Sprintf("%s's %s at %s; commit and push it", repo, branch, short(d.Commit))
	var paths []string
	for p := range local {
		paths = append(paths, p)
	}
	for p := range pushed {
		if !local[p] {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	for _, p := range paths {
		e, inCommit := pushed[p]
		line := ""
		switch {
		case !inCommit:
			line = p + " is not in " + at
		case !local[p]:
			line = p + " is only in " + at
		default:
			same := false
			if target, link := links[p]; link {
				if e.Mode == "120000" {
					b, err := g.Blob(m, e.Object)
					if err != nil {
						return d, err
					}
					same = string(b) == target
				}
			} else {
				same = e.Mode != "120000" && e.Type == "blob" && blobs[p] == e.Object
			}
			if !same {
				line = p + " differs from " + at
			}
		}
		if line != "" {
			d.Paths = append(d.Paths, p)
			d.Lines = append(d.Lines, "⚠ Drift: "+line)
		}
	}
	return d, nil
}

// drift is apply's Drift (section 11), at the end of each pass: the
// clone's configuration compared with the swarm repository's default
// branch as the pass fetched it (its mirror fetched now when the pass
// has not fetched), its lines on stderr at the first pass that compares
// and again only when the set of drifting paths changes, and once that
// set is empty again the cleared line on stdout. A fetch or a
// comparison git cannot make is one warning, and changes nothing.
func (s *Swarm) drift() {
	g := s.git()
	if !s.fetched {
		if err := g.FetchSwarm(repoName(s.Cfg.Forge.Repo)); err != nil {
			fmt.Fprintf(s.Err, "⚠ Drift: %v\n", err)
			return
		}
	}
	d, err := ConfigDrift(g, s.Cfg.Forge.Repo)
	if err != nil {
		fmt.Fprintf(s.Err, "⚠ Drift: %v\n", err)
		return
	}
	if d.Commit == "" {
		return
	}
	if s.driftSeen && slices.Equal(d.Paths, s.drifting) {
		return
	}
	switch {
	case len(d.Paths) > 0:
		printLines(s.Err, d.Lines)
	case s.driftSeen:
		fmt.Fprintln(s.Out, d.Cleared())
	}
	s.driftSeen, s.drifting = true, d.Paths
}

func printLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}
