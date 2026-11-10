// Package forge is the swarm's one seam to the forge (docs/swarm.md
// section 10): the Adapter interface, the gitea adapter over Ops.HTTP, and
// Fake, an in-memory forge for tests. Every forge call Daisy makes is an
// Adapter method; the only other traffic with the forge is host-side git.
package forge

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
)

// The nine stage labels (section 5), grouped by what they are about and
// numbered by how far the stage has got: the state machine, one on each
// open swarm issue. PlanInProgress and ReviewInProgress are written at a
// member's launch and taken back when its run does not succeed.
const (
	ProposalPosted   = "swarm:proposal-posted"
	ProposalAccepted = "swarm:proposal-accepted"
	PlanPosted       = "swarm:plan-0-posted"
	PlanAccepted     = "swarm:plan-1-accepted"
	PlanInProgress   = "swarm:plan-2-in-progress"
	ReviewReady      = "swarm:review-0-ready"
	ReviewInProgress = "swarm:review-1-in-progress"
	ReviewApproved   = "swarm:review-2-approved"
	ChangesRequested = "swarm:review-3-changes-requested"
)

// Stages are the stage labels in lifecycle order.
var Stages = []string{
	ProposalPosted, ProposalAccepted, PlanPosted, PlanAccepted, PlanInProgress,
	ReviewReady, ReviewInProgress, ReviewApproved, ChangesRequested,
}

// The flags sit beside the stage, never one (section 5): Gated, the
// stage waits for the maintainer; Blocked, progress cannot be made until
// the maintainer unblocks it; Paused, the maintainer holds the issue;
// Blocking, the issue blocks another; ClassArchitecture, the job is of
// the architecture class, whose triage and plan gates the maintainer
// keeps. Done and Rejected are the terminal
// labels, which replace the stage label when the swarm or the
// maintainer's verbs close an issue.
const (
	Gated    = "swarm:gated"
	Blocked  = "swarm:blocked"
	Paused   = "swarm:paused"
	Blocking = "swarm:blocking"
	Done     = "swarm:done"
	Rejected = "swarm:rejected"

	ClassArchitecture = "swarm:class-architecture"
)

// Flags are the five flags.
var Flags = []string{Gated, Blocked, Paused, Blocking, ClassArchitecture}

// Labels are the sixteen labels `swarm init` and `swarm apply` ensure:
// the stages, the flags and the two terminals.
var Labels = append(append(slices.Clone(Stages), Flags...), Done, Rejected)

// Renamed is each stage label before Step 15 of the swarm build by the
// name that replaced it (section 14, item 74): EnsureLabels renames a
// label found under its old name, and a run recorded before the rename
// is read under the new one. swarm:blocked keeps its name, as a flag.
var Renamed = map[string]string{
	"swarm:proposed":  ProposalPosted,
	"swarm:ready":     ProposalAccepted,
	"swarm:planning":  PlanPosted,
	"swarm:planned":   PlanAccepted,
	"swarm:in-review": ReviewReady,
	"swarm:approved":  ReviewApproved,
	"swarm:reviewed":  ChangesRequested,
}

// Current is label under its current name: a stage's old name (Renamed)
// is its new one, any other label itself.
func Current(label string) string {
	if n, ok := Renamed[label]; ok {
		return n
	}
	return label
}

// IsStage: l counts as a stage label wherever Daisy reads an issue's one
// stage: a `swarm:*` label other than the flags and the terminals, so an
// unknown `swarm:*` name beside a stage still makes two (section 5).
func IsStage(l string) bool {
	return strings.HasPrefix(l, "swarm:") && !slices.Contains(Flags, l) && l != Done && l != Rejected
}

// SwarmLabels are the labels whose issues in state are swarm issues, each
// listed by them: an open one carries a stage; a closed one its terminal,
// or the stage it was closed at before the terminals or by hand.
func SwarmLabels(state State) []string {
	if state == Open {
		return Stages
	}
	return append(slices.Clone(Stages), Done, Rejected)
}

// Adapter is every call the swarm makes to the forge, less Board, which is
// specified (section 10) and joins the interface with the step that builds
// the board. An adapter is built for forge.repo and the declared
// repositories' names.
type Adapter interface {
	// EnsureLabels gives the label set every name of names: a name absent
	// whose old name (Renamed) is present is that label renamed, keeping
	// its issues, and any other absent name is created; it reports how
	// many it created and renamed, and never deletes a label.
	EnsureLabels(names []string) (created, renamed int, err error)
	Login() (Account, error) // the token's own account
	Issues(state State, label string) ([]Issue, error)
	Issue(n int) (Issue, error) // with comments, oldest first
	CreateIssue(title, body string, labels []string) (int, error)
	Comment(at Ref, body string) error
	// Relabel adds add, then removes remove; "" on either side is no
	// change there, so one call sets or clears the flag alone.
	Relabel(at Ref, remove, add string) error
	CloseIssue(n int) error
	// Pulls is the swarm's pull requests for issue n in every declared
	// repository opened by one of openers (any case: the swarm's login
	// and trusted's), by slice; a pull request any other login opened is
	// never one (section 10).
	Pulls(n int, openers []string) ([]Pull, error)
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
	// Repository is forge.repo as the forge holds it, ErrNotFound when it
	// holds none (section 11, init).
	Repository() (Repository, error)
	// CreateRepository creates forge.repo, private, in its owner's
	// organization, as the forge then holds it.
	CreateRepository() (Repository, error)
	// AddCollaborator gives login write access to forge.repo; given
	// again, it changes nothing.
	AddCollaborator(login string) error
}

// Repository is the swarm repository as the forge reports it: the URL
// git reaches it at over SSH and its default branch.
type Repository struct{ SSHURL, DefaultBranch string }

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
	Pull      bool      // a pull request: Issue(n) reads both, as the forges number them together
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
	Mergeable                        Mergeable // whether it merges into Base, as the forge last computed it
	Labels                           []string
	Author                           string // the login that opened it
	Created                          time.Time
}

// Mergeable is whether an open pull request merges into its base, as the
// forge reports it (section 10): unknown while the forge has not computed
// it (GitHub's null), no when the forge says it does not (a conflict
// with the base among the causes).
type Mergeable int

const (
	MergeableUnknown Mergeable = iota
	MergeableYes
	MergeableNo
)

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

// ErrNotFound matches the forge's 404: Issue(n) of a number it does not
// hold.
var ErrNotFound = errors.New("forge: not found")

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
	return fmt.Errorf("forge: %s: no label %s for %s/%s; run 'daisy swarm init', which ensures the sixteen", method, label, owner, name)
}

// renames is each name of names absent from have whose old name (Renamed)
// have holds, as {old, new}, in names' order.
func renames(names []string, have func(string) bool) [][2]string {
	var out [][2]string
	for old, n := range Renamed {
		if slices.Contains(names, n) && !have(n) && have(old) {
			out = append(out, [2]string{old, n})
		}
	}
	slices.SortFunc(out, func(a, b [2]string) int { return slices.Index(names, a[1]) - slices.Index(names, b[1]) })
	return out
}

// absent is each name of names, once, that have lacks and no rename of
// rename gives.
func absent(names []string, have func(string) bool, rename [][2]string) []string {
	var out []string
	for _, n := range names {
		if !have(n) && !slices.Contains(out, n) && !slices.ContainsFunc(rename, func(r [2]string) bool { return r[1] == n }) {
			out = append(out, n)
		}
	}
	return out
}

// refusedLabels is EnsureLabels' failure when the forge refuses a rename
// or a create: what is still to rename and to create, for an owner.
func refusedLabels(owner string, rename [][2]string, missing []string) error {
	var todo []string
	if len(rename) > 0 {
		var rs []string
		for _, r := range rename {
			rs = append(rs, r[0]+" as "+r[1])
		}
		todo = append(todo, "to rename (keeping their issues): "+strings.Join(rs, ", "))
	}
	if len(missing) > 0 {
		todo = append(todo, "missing: "+strings.Join(missing, ", "))
	}
	return fmt.Errorf("forge: EnsureLabels: %s's labels %s; an owner of %s makes them once", owner, strings.Join(todo, "; "), owner)
}

// swarmPull: a pull request in the declared repository name, on branch
// head with title and body, opened by author, is the swarm's for issue n
// of owner/repo (section 10, Pulls), and its slice number: author is one
// of openers (any case), and the slice is read from the branch
// `swarm/<n>-<s>`, or, when the forge has deleted a merged branch and
// reports a head such as `refs/pull/<m>/head`, from the title the worker's
// collection wrote, `<issue> slice <s>: <title>`.
func swarmPull(head, title, body, author, owner, repo, name string, n int, openers []string) (int, bool) {
	if !slices.ContainsFunc(openers, func(l string) bool { return strings.EqualFold(l, author) }) {
		return 0, false
	}
	if !refersTo(body, owner, repo, name, n) {
		return 0, false
	}
	if s, ok := sliceOf(head, n); ok {
		return s, true
	}
	return sliceOfTitle(title, owner, repo, name, n)
}

// issueRef is how a pull request in repository name refers to issue n of
// owner/repo: `#<n>` in the issues repository itself, `<owner>/<repo>#<n>`
// in another.
func issueRef(owner, repo, name string, n int) string {
	if name != repo {
		return fmt.Sprintf("%s/%s#%d", owner, repo, n)
	}
	return fmt.Sprintf("#%d", n)
}

// sliceOfTitle is the slice number a swarm pull request's title names,
// `<issue> slice <s>: …`, and whether title is one.
func sliceOfTitle(title, owner, repo, name string, n int) (int, bool) {
	rest, ok := strings.CutPrefix(title, issueRef(owner, repo, name, n)+" slice ")
	if !ok {
		return 0, false
	}
	digits, _, ok := strings.Cut(rest, ":")
	s, err := strconv.Atoi(digits)
	if !ok || err != nil || s < 1 || strconv.Itoa(s) != digits {
		return 0, false
	}
	return s, true
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
	want := "Refs " + issueRef(owner, repo, name, n)
	for _, line := range strings.Split(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), want) {
			return true
		}
	}
	return false
}
