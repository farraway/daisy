package reconcile

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/ops"
)

// PullReverts is a swarm's merged pull requests and how many of them a
// revert on base took back, read from its mirrors as their last fetch
// left them (docs/swarm.md section 11, status's Throughput): no fetch is
// made, so a revert pushed since is not counted until the next pass
// fetches. Fetched is the oldest of the read mirrors' last fetch, zero
// when none was read.
type PullReverts struct {
	Merged, Reverted int
	Fetched          time.Time
}

// ReadPullReverts reads the swarm's PullReverts under state, the merges
// made at or after since (all of them when it is zero). A swarm pull
// request's merge is a merge commit on base's first-parent line in the
// mirror of the repository a worker run pushed to (its `branch … in
// <name>` line), whose second parent is a head that run pushed (its
// `pushed <sha>` line): the forge merges with a merge commit at the head
// both reviews approved, the run's push (Gitea.Merge). It is reverted
// when a commit on base says `This reverts commit <its sha>`
// (metrics.Reverted). A repository with no mirror, or whose mirror holds
// no base, has none.
func ReadPullReverts(o ops.Ops, state, base string, since time.Time) (PullReverts, error) {
	var p PullReverts
	runs, err := Runs(o.FS(), state)
	if err != nil {
		return p, err
	}
	heads := map[string]map[string]bool{} // a repository's pushed heads
	for _, r := range runs {
		if r.Role != "worker" {
			continue
		}
		lines, err := r.Result(o.FS())
		if err != nil {
			return p, err
		}
		pushed, ok := Has(lines, "pushed")
		repo := branchRepo(lines)
		if !ok || repo == "" {
			continue
		}
		if heads[repo] == nil {
			heads[repo] = map[string]bool{}
		}
		heads[repo][strings.TrimPrefix(pushed, "pushed ")] = true
	}
	g := Git{O: o, State: state}
	ref := "refs/heads/" + base
	for _, repo := range slices.Sorted(maps.Keys(heads)) {
		m := g.Mirror(repo)
		if g.Head(m, ref) == "" {
			continue
		}
		out, err := g.run(m, "log", "--first-parent", "--merges", "--format=%H %ct %P", ref, "--")
		if err != nil {
			return p, err
		}
		named, err := metrics.Reverted(o, m, ref)
		if err != nil {
			return p, err
		}
		for _, l := range strings.Split(out, "\n") {
			f := strings.Fields(l) // the merge, its time, its parents
			if len(f) < 4 || !heads[repo][f[3]] {
				continue
			}
			if at, err := strconv.ParseInt(f[1], 10, 64); err == nil && time.Unix(at, 0).Before(since) {
				continue
			}
			p.Merged++
			if named.Has(f[0]) {
				p.Reverted++
			}
		}
		fi, err := o.FS().Stat(filepath.Join(m, "FETCH_HEAD"))
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return p, err
		}
		if err == nil && (p.Fetched.IsZero() || fi.ModTime().Before(p.Fetched)) {
			p.Fetched = fi.ModTime().UTC()
		}
	}
	return p, nil
}

// MirroredBase is the base swarm.json names on the swarm repository's
// default branch, in its mirror under state as the last fetch left it:
// what `daisy metrics`, which reads no clone, counts a swarm's merges
// on. Only `base` is read, so a declaration another Daisy wrote reads
// too.
func MirroredBase(o ops.Ops, state, repo string) (string, error) {
	m := Git{O: o, State: state}.Mirror(repoName(repo))
	out, err := Git{O: o}.run(m, "show", "HEAD:swarm.json")
	if err != nil {
		return "", fmt.Errorf("no swarm.json in the mirror %s: %w", m, err)
	}
	var c struct {
		Base string `json:"base"`
	}
	if err := json.Unmarshal([]byte(out), &c); err != nil || c.Base == "" {
		return "", fmt.Errorf("the swarm.json in the mirror %s names no base", m)
	}
	return c.Base, nil
}

// States is each <state> of the swarm repo (`<owner>/<repo>`) under
// local ($DAISY_LOCAL), one per forge host that holds it, in path order.
func States(fs ops.FS, local, repo string) ([]string, error) {
	owner, name, _ := strings.Cut(repo, "/")
	hosts, err := fs.ReadDir(filepath.Join(local, "swarm"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var states []string
	for _, h := range hosts {
		s := filepath.Join(local, "swarm", h.Name(), owner, name)
		if fi, err := fs.Stat(s); h.IsDir() && err == nil && fi.IsDir() {
			states = append(states, s)
		}
	}
	return states, nil
}
