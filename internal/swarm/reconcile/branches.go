package reconcile

import (
	"cmp"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// OpenBranch is a swarm branch of a declared repository not merged into
// its base, as its mirror holds it: issue N's slice S at Head, changing
// Files against the base, or Unrelated, sharing no history with it
// (section 7.8, The open branches).
type OpenBranch struct {
	Repo, Branch, Head string
	N, S               int
	Files              []string
	Unrelated          bool
}

var swarmBranch = regexp.MustCompile(`^refs/heads/(swarm/([0-9]+)-([0-9]+))$`)

// OpenBranches is each branch refs/heads/swarm/<n>-<s> of the
// repository's mirror that refs/heads/<base> does not hold (`git
// merge-base --is-ancestor` false), with the files `git diff --name-only
// -z refs/heads/<base>...<branch>` names, by issue then slice; a branch
// with no merge base is Unrelated, which no three-dot diff can compare.
// The branch skip names is left out before any git command reads it.
func (g Git) OpenBranches(name, base, skip string) ([]OpenBranch, error) {
	m := g.Mirror(name)
	refs, err := g.run(m, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/swarm/")
	if err != nil {
		return nil, err
	}
	var open []OpenBranch
	for _, l := range strings.Split(refs, "\n") {
		sha, ref, _ := strings.Cut(l, " ")
		sm := swarmBranch.FindStringSubmatch(ref)
		if sm == nil || sm[1] == skip {
			continue
		}
		n, err1 := strconv.Atoi(sm[2])
		s, err2 := strconv.Atoi(sm[3])
		if err1 != nil || err2 != nil {
			continue
		}
		// Exit 1 is the branch not held by base; 0 merged into it.
		_, errb, err := g.O.Exec(m, "git", "merge-base", "--is-ancestor", ref, "refs/heads/"+base)
		var exit *exec.ExitError
		switch {
		case err == nil:
			continue
		case !errors.As(err, &exit) || exit.ExitCode() != 1:
			return nil, fmt.Errorf("git merge-base --is-ancestor %s refs/heads/%s: %v: %s", ref, base, err, strings.TrimSpace(string(errb)))
		}
		_, errb, err = g.O.Exec(m, "git", "merge-base", "refs/heads/"+base, ref)
		switch {
		case errors.As(err, &exit) && exit.ExitCode() == 1:
			open = append(open, OpenBranch{Repo: name, Branch: sm[1], Head: sha, N: n, S: s, Unrelated: true})
			continue
		case err != nil:
			return nil, fmt.Errorf("git merge-base refs/heads/%s %s: %v: %s", base, ref, err, strings.TrimSpace(string(errb)))
		}
		out, errb, err := g.O.Exec(m, "git", "diff", "--name-only", "-z", "refs/heads/"+base+"..."+ref)
		if err != nil {
			return nil, fmt.Errorf("git diff --name-only -z refs/heads/%s...%s: %v: %s", base, ref, err, strings.TrimSpace(string(errb)))
		}
		var files []string
		for _, f := range strings.Split(string(out), "\x00") {
			if f != "" {
				files = append(files, f)
			}
		}
		open = append(open, OpenBranch{Repo: name, Branch: sm[1], Head: sha, N: n, S: s, Files: files})
	}
	slices.SortFunc(open, func(a, b OpenBranch) int { return cmp.Or(cmp.Compare(a.N, b.N), cmp.Compare(a.S, b.S)) })
	return open, nil
}

// writeBranches writes <run>/io/branches.md, /job/io/branches.md in the
// member's container, before the launch of a worker's, a reviewer's or
// the architect's run (section 7.8, The open branches): a line per open
// swarm branch of each declared repository, in their declared order, but
// the run's own (ownRepo's ownBranch; none for the architect), as the
// pass fetched them; "None." when there is none.
func (s *Swarm) writeBranches(ioDir, ownRepo, ownBranch string) error {
	var lines []string
	for _, rp := range s.Cfg.Repos {
		skip := ""
		if rp.Name == ownRepo {
			skip = ownBranch
		}
		open, err := s.git().OpenBranches(rp.Name, s.Cfg.Base, skip)
		if err != nil {
			return err
		}
		for _, b := range open {
			files := strings.Join(b.Files, ", ")
			if b.Unrelated {
				files = "no history in common with " + s.Cfg.Base
			}
			lines = append(lines, fmt.Sprintf("- %s %s (#%d, slice %d) at %s: %s", b.Repo, b.Branch, b.N, b.S, short(b.Head), files))
		}
	}
	if len(lines) == 0 {
		lines = []string{"None."}
	}
	return s.O.FS().WriteFile(filepath.Join(ioDir, "branches.md"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// readsBranches: role's runs are given branches.md (section 7.8): the
// worker, the reviewers and the architect.
func readsBranches(role string) bool {
	return role == "worker" || role == "adversary" || role == "sympathetic" || role == "architect"
}
