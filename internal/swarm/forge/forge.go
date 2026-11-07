// Package forge is the swarm's one seam to the forge (docs/swarm.md
// section 10): the Adapter interface, the gitea adapter over Ops.HTTP, and
// Fake, an in-memory forge for tests. Every forge call Daisy makes is an
// Adapter method; the only other traffic with the forge is host-side git.
package forge

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
)

// Labels are the eight stage labels (section 5), in lifecycle order: what
// `swarm init` and `swarm apply` ensure.
var Labels = []string{
	"swarm:proposed", "swarm:ready", "swarm:planning", "swarm:planned",
	"swarm:in-review", "swarm:reviewed", "swarm:approved", "swarm:blocked",
}

// Adapter is every call the swarm makes to the forge, less Board, which is
// specified (section 10) and joins the interface with the step that builds
// the board. An adapter is built for forge.repo and the declared
// repositories' names.
type Adapter interface {
	// EnsureLabels creates each name the label set lacks and reports how
	// many it created; it never edits or deletes a label.
	EnsureLabels(names []string) (created int, err error)
	Login() (Account, error) // the token's own account
	Issues(state State, label string) ([]Issue, error)
	Issue(n int) (Issue, error) // with comments, oldest first
	CreateIssue(title, body string, labels []string) (int, error)
	Comment(at Ref, body string) error
	Relabel(at Ref, remove, add string) error
	CloseIssue(n int) error
	// Pulls is the swarm's pull requests for issue n in every declared
	// repository, by slice.
	Pulls(n int) ([]Pull, error)
	CreatePull(repo, head, base, title, body string) (int, error)
	PullComments(pr Ref) ([]Comment, error)
	Reviews(pr Ref) ([]Review, error)
	ReviewComments(pr Ref) ([]ReviewComment, error)
	Review(pr Ref, verdict Verdict, body string) error
	// Approve is a formal approving review of pr at the commit head: the
	// maintainer's (section 12), never falling back to a comment-type
	// review, so a refusal is its error.
	Approve(pr Ref, head string) error
	Merge(pr Ref, head string) error
	// Milestones is forge.repo's milestones, open and closed, each with
	// the issues the forge counts in it.
	Milestones() ([]Milestone, error)
	// EnsureMilestone is the id of forge.repo's milestone titled title,
	// open or closed, created open when there is none; it never edits one.
	EnsureMilestone(title string) (id int64, err error)
	// SetMilestone puts issue n of forge.repo in the milestone id.
	SetMilestone(n int, id int64) error
}

// Account is the token's own forge account as the forge reports it; Email
// is "" when the forge reports none.
type Account struct{ Login, Email string }

// State is an issue's state as Issues selects it.
type State int

const (
	Open State = iota
	Closed
)

// Ref is a number in a repository of the owner: an issue, or a pull
// request, in the repository named Repo.
type Ref struct {
	Repo string
	N    int
}

type Issue struct {
	Number    int
	Title     string
	Body      string
	Author    string
	Created   time.Time
	Closed    time.Time // zero while open
	Open      bool
	Labels    []string
	Milestone int64     // its milestone's id, 0 for none
	Comments  []Comment // filled by Issue(n) only
}

// Milestone is a milestone of forge.repo and its issues, open and closed,
// as the forge counts them.
type Milestone struct {
	ID           int64
	Title        string
	Open, Closed int
}

type Comment struct {
	Author, Body string
	Created      time.Time
}

type Pull struct {
	Repo                             string // the declared repository's name
	Number                           int
	Title, Body, Head, HeadSHA, Base string
	MergeSHA                         string // "" unless merged
	Open, Merged                     bool
	Labels                           []string
	Author                           string // the login that opened it
	Created                          time.Time
}

type Review struct {
	ID        int64
	Author    string
	Body      string
	State     Verdict
	Commit    string
	Submitted time.Time
}

type ReviewComment struct {
	Author, Body, Path, Commit string
	Line                       int
	Created                    time.Time
}

type Verdict int

const (
	Approve Verdict = iota
	RequestChanges
	Commented
)

// ErrNotMergeable is a merge the forge refused.
var ErrNotMergeable = errors.New("forge: not mergeable")

// New is the adapter for kind over o's HTTP: gitea is built; github is
// specified (section 10) and built when a run needs it.
func New(o ops.Ops, kind, api, token, repo string, names []string) (Adapter, error) {
	switch kind {
	case "gitea":
		return NewGitea(o, api, token, repo, names), nil
	case "github":
		return nil, errors.New("forge: kind github has no adapter yet; it is built when a run needs it (docs/swarm.md section 10)")
	}
	return nil, fmt.Errorf("forge: kind %q has no adapter", kind)
}

// noLabel is CreateIssue's or Relabel's failure on a label the forge
// does not hold for the repository name of owner.
func noLabel(method, owner, name, label string) error {
	return fmt.Errorf("forge: %s: no label %s for %s/%s; run 'daisy swarm init', which ensures the eight", method, label, owner, name)
}

// refusedLabels is EnsureLabels' failure when the forge refuses a create.
func refusedLabels(owner string, missing []string) error {
	return fmt.Errorf("forge: EnsureLabels: %s's labels missing: %s; an owner of %s creates them once", owner, strings.Join(missing, ", "), owner)
}

// swarmPull: a pull request in the declared repository name, on branch
// head with body, is the swarm's for issue n of owner/repo (section 10,
// Pulls), and its slice number.
func swarmPull(head, body, owner, repo, name string, n int) (int, bool) {
	s, ok := sliceOf(head, n)
	return s, ok && refersTo(body, owner, repo, name, n)
}

// sliceOf is the slice number of a swarm branch `swarm/<n>-<s>` for issue
// n, and whether branch is one.
func sliceOf(branch string, n int) (int, bool) {
	rest, ok := strings.CutPrefix(branch, fmt.Sprintf("swarm/%d-", n))
	s, err := strconv.Atoi(rest)
	if !ok || err != nil || s < 1 || strconv.Itoa(s) != rest {
		return 0, false
	}
	return s, true
}

// refersTo: body has a line equal, in any case, to the Refs line a pull
// request in repository name carries for issue n of owner/repo: `Refs #<n>`
// in the issues repository itself, `Refs <owner>/<repo>#<n>` in another.
func refersTo(body, owner, repo, name string, n int) bool {
	want := fmt.Sprintf("Refs #%d", n)
	if name != repo {
		want = fmt.Sprintf("Refs %s/%s#%d", owner, repo, n)
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), want) {
			return true
		}
	}
	return false
}
