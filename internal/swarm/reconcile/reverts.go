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
// when none was read or none of those read records one; Read is whether
// a mirror holding base was read.
type PullReverts struct {
	Merged, Reverted int
	Fetched          time.Time
	Read             bool
}

// ReadPullReverts reads the swarm's PullReverts under state, the merges
// made at or after since (all of them when it is zero), from ReadMerges.
func ReadPullReverts(o ops.Ops, state, base string, since time.Time) (PullReverts, error) {
	merges, fetched, read, err := ReadMerges(o, state, base)
	if err != nil {
		return PullReverts{}, err
	}
	p := PullReverts{Fetched: fetched, Read: read}
	for _, m := range merges {
		if m.At.Before(since) {
			continue
		}
		p.Merged++
		if m.Reverted {
			p.Reverted++
		}
	}
	return p, nil
}

// Merge is a swarm pull request's merge as a mirror holds it: the issue
// and slice its branch names (`swarm/<n>-<s>`), the repository, the
// merge commit and its committer time, the worker run whose push it
// merged, and whether a revert in effect on base takes it back.
type Merge struct {
	N, Slice  int
	Repo, SHA string
	At        time.Time
	Run       string
	Reverted  bool
}

// ReadMerges reads the swarm's merged pull requests under state, in the
// order of their times, the oldest of the read mirrors' last fetch
// (zero when none was read or none records one) and whether any mirror
// holding base was read, from its mirrors as their last fetch left
// them: no fetch is made. A swarm pull request's merge is a merge commit
// on base's first-parent line in the mirror of the repository a worker
// run pushed to (its `branch … in <name>` line), whose second parent is
// a head that run pushed (its `pushed <sha>` line): the forge merges
// with a merge commit at the head both reviews approved, the run's push
// (Gitea.Merge). It is reverted when a revert in effect on base names it
// (metrics.Reverted). A repository with no mirror, or whose mirror holds
// no base, has none.
func ReadMerges(o ops.Ops, state, base string) ([]Merge, time.Time, bool, error) {
	var fetched time.Time
	read := false
	runs, err := Runs(o.FS(), state)
	if err != nil {
		return nil, fetched, read, err
	}
	type push struct {
		run      string
		n, slice int
	}
	heads := map[string]map[string]push{} // a repository's pushed heads
	for _, r := range runs {
		if r.Role != "worker" {
			continue
		}
		lines, err := r.Result(o.FS())
		if err != nil {
			return nil, fetched, read, err
		}
		pushed, ok := Has(lines, "pushed")
		repo := branchRepo(lines)
		if !ok || repo == "" {
			continue
		}
		if heads[repo] == nil {
			heads[repo] = map[string]push{}
		}
		p := push{run: filepath.Base(r.Dir), n: r.N}
		if b, ok := Has(lines, "branch"); ok {
			_, rest, _ := strings.Cut(strings.Fields(b)[1], "-") // swarm/<n>-<s>
			p.slice, _ = strconv.Atoi(rest)
		}
		heads[repo][strings.TrimPrefix(pushed, "pushed ")] = p
	}
	g := Git{O: o, State: state}
	ref := "refs/heads/" + base
	var merges []Merge
	for _, repo := range slices.Sorted(maps.Keys(heads)) {
		m := g.Mirror(repo)
		if g.Head(m, ref) == "" {
			continue
		}
		read = true
		out, err := g.run(m, "log", "--first-parent", "--merges", "--format=%H %ct %P", ref, "--")
		if err != nil {
			return nil, fetched, read, err
		}
		named, err := metrics.Reverted(o, m, ref)
		if err != nil {
			return nil, fetched, read, err
		}
		for _, l := range strings.Split(out, "\n") {
			f := strings.Fields(l) // the merge, its time, its parents
			if len(f) < 4 {
				continue
			}
			p, ok := heads[repo][f[3]]
			if !ok {
				continue
			}
			at, err := strconv.ParseInt(f[1], 10, 64)
			if err != nil {
				return nil, fetched, read, fmt.Errorf("git log in %s: %q has no commit time", m, l)
			}
			merges = append(merges, Merge{N: p.n, Slice: p.slice, Repo: repo, SHA: f[0], At: time.Unix(at, 0).UTC(), Run: p.run, Reverted: named.Has(f[0])})
		}
		fi, err := o.FS().Stat(filepath.Join(m, "FETCH_HEAD"))
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return nil, fetched, read, err
		}
		if err == nil && (fetched.IsZero() || fi.ModTime().Before(fetched)) {
			fetched = fi.ModTime().UTC()
		}
	}
	slices.SortStableFunc(merges, func(a, b Merge) int { return a.At.Compare(b.At) })
	return merges, fetched, read, nil
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
