package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/fence"
)

// The start (section 7.8, The start): the maintainer's ruling of
// 2026-11-19 is that the next worker run of a slice starts from the last
// run's kept commit unless the architect's answer or the maintainer's
// unblock says start from base, and nothing discards kept work
// automatically; at the stop of Step 9, a revised plan's word resets it
// as an answer's does.

// Start is where a worker run on a slice starts and why.
type Start struct {
	// From is "base" (a slice not yet pushed) or "head" (the open pull
	// request's head as the pass fetched it), where it would start with
	// no kept run; "kept", the kept commit; or "moved", the branch's
	// head, moved past the kept run by a push not the run's.
	From string
	// SHA is the commit the clone's branch is made or moved to for
	// "kept" and "moved", fetched from the mirror's Ref; "" for the
	// others, which the clone gives.
	SHA, Ref string
	// Kept is the kept run, `<n>-worker-<k>`, and KeptSHA its commit;
	// "" when there is none, or the mirror no longer holds it.
	Kept, KeptSHA string
	// By is who reset the start with a `Start: base` line, a login or
	// `architect run <k>`, and At when; "" when no line did.
	By string
	At time.Time
}

// keptRe is a kept file's line (section 7.8, slices/<s>/kept).
var keptRe = regexp.MustCompile(`^([1-9][0-9]*)-worker-([1-9][0-9]*) ([0-9a-f]{40,64})$`)

// kept is slice s of issue n's kept run and its commit; ok is false when
// the slice has none. A kept file that cannot be read or is not Daisy's
// line is the job's own failure (unusableError).
func (s *Swarm) kept(n, sl int) (run Run, sha string, ok bool, err error) {
	rel := filepath.Join("slices", strconv.Itoa(sl), "kept")
	b, err := s.O.FS().ReadFile(filepath.Join(s.workspace(n), rel))
	if errors.Is(err, iofs.ErrNotExist) {
		return Run{}, "", false, nil
	}
	if err != nil {
		return Run{}, "", false, workspaceUnusable(n, filepath.ToSlash(rel), 0, err)
	}
	m := keptRe.FindStringSubmatch(strings.TrimSuffix(string(b), "\n"))
	if m == nil || m[1] != strconv.Itoa(n) {
		return Run{}, "", false, workspaceUnusable(n, filepath.ToSlash(rel), 0, errors.New("not one line `<n>-worker-<k> <sha>`"))
	}
	k, _ := strconv.Atoi(m[2])
	name := fmt.Sprintf("%d-worker-%d", n, k)
	return Run{Dir: filepath.Join(s.state(), "runs", name), N: n, K: k, Role: "worker"}, m[3], true, nil
}

// start is where worker run j on slice sl of repository repo, on branch,
// starts (section 7.8, The start): open is whether the slice's pull
// request is open. The mirrors are the pass's, fetched.
func (s *Swarm) start(j *Job, sl int, repo declare.Repo, branch string, open bool) (Start, error) {
	none := Start{From: "base"}
	if open {
		none.From = "head"
	}
	run, sha, ok, err := s.kept(j.N, sl)
	if err != nil || !ok {
		return none, err
	}
	// The kept commit is held while the kept run's ref names it, or the
	// slice's branch holds it (pushed); an object no ref reaches, its run's
	// ref pruned by the maintainer, is none to start from, as the replay
	// writes no kept for it.
	g := s.git()
	m := g.Mirror(repo.Name)
	ref, head := "refs/swarm-runs/"+filepath.Base(run.Dir), g.Head(m, "refs/heads/"+branch)
	if g.Head(m, ref) != sha {
		ref = ""
		if held, err := g.Holds(repo.Name, sha); err != nil {
			return none, err
		} else if held && head != "" {
			if in, err := g.IsAncestor(repo.Name, sha, head); err != nil {
				return none, err
			} else if in {
				ref = "refs/heads/" + branch
			}
		}
	}
	if ref == "" {
		fmt.Fprintf(s.Err, "⚠ #%d worker run %d: kept commit %s of %s is not in %s's mirror; started as with no kept run\n", j.N, j.K, short(sha), filepath.Base(run.Dir), repo.Name)
		return none, nil
	}
	none.Kept, none.KeptSHA = filepath.Base(run.Dir), sha
	lines, err := run.Result(s.O.FS())
	if err != nil {
		return none, err
	}
	if ended, known := endTime(lines); known {
		if by, at, reset := j.startBase(ended); reset {
			none.By, none.At = by, at
			return none, nil
		}
	}
	if head != "" {
		if under, err := g.IsAncestor(repo.Name, head, sha); err != nil {
			return none, err
		} else if !under {
			return Start{From: "moved", SHA: head, Ref: "refs/heads/" + branch, Kept: none.Kept, KeptSHA: sha}, nil
		}
	}
	return Start{From: "kept", SHA: sha, Ref: ref, Kept: none.Kept, KeptSHA: sha}, nil
}

// startBase is the newest `Start: base` line written since the kept run
// ended (section 7.8, The start, 1): in the architect's answer (its text
// beginning ANSWER:) or revised plan (its text a plan) on the issue, read
// whole, or in the maintainer's counted comment (maintainerWord) on the
// issue or its current pull request, whose thread j.Pull holds when it
// is open; by is its author's login, or `architect run <k>`. A forge
// time has whole seconds, so an entry in the second the run ended counts.
func (j *Job) startBase(ended time.Time) (by string, at time.Time, ok bool) {
	since := ended.Truncate(time.Second)
	entries := IssueThread(j.Issue)[1:]
	if j.Pull != nil {
		entries = append(entries, pullComments(j.Pull.Comments)...)
	}
	for _, e := range entries {
		if e.Created.Before(since) || ok && e.Created.Before(at) {
			continue
		}
		who, text := "", ""
		if m, swarm := Swarms(e, j.SwarmLogin); swarm {
			t := j.whole(m, swarmText(e.Body))
			if m.Role == "architect" && m.Kind == "" && (strings.HasPrefix(t, answer) || onThePlan(t) && !strings.HasPrefix(t, stands)) {
				who, text = "architect run "+strconv.Itoa(m.Run), t
			}
		} else if maintainerWord(e, j.SwarmLogin, j.Cfg.Trusted) {
			who, text = e.Author, e.Body
		}
		if who != "" && startBaseLine(text) {
			by, at, ok = who, e.Created, true
		}
	}
	return by, at, ok
}

// startBaseLine: text holds the line `Start: base` alone, whitespace at
// either end aside, outside a fenced block (section 7.8, The start).
func startBaseLine(text string) bool {
	var fences fence.Scan
	for _, line := range strings.Split(text, "\n") {
		if !fences.Line(line) && strings.TrimSpace(line) == "Start: base" {
			return true
		}
	}
	return false
}
