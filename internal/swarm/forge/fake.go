package forge

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"daisy/internal/ops"
)

// Fake is an in-memory forge (section 10, "forge.Fake"): the issues
// repository and the declared repositories it is built with, their issues,
// pull requests, comments, reviews and line comments; one label set for
// all of them, as Gitea's organization labels are; numbers from one
// counter per repository, shared by its issues and pull requests; times
// from the Ops clock; and the account its token is. It applies the real
// adapters' semantics, and its helpers stand in for what other logins and
// git do on the forge. A helper given a number the Fake does not hold
// panics: that is the test's mistake. Tests assert on its state, never on
// which methods were called. It is safe for concurrent use, as two
// `apply` processes share one forge.
type Fake struct {
	mu          sync.Mutex
	o           ops.Ops
	owner, repo string   // forge.repo
	names       []string // the declared repositories
	account     Account
	labels      []string
	refuse      bool
	items       map[Ref]*item
	next        map[string]int
	reviewID    int64
}

// item is an issue, or a pull request when pull is set.
type item struct {
	title, body, author string
	created, closed     time.Time
	open                bool
	labels              []string
	comments            []Comment
	pull                *pullState
}

type pullState struct {
	head, headSHA, base, mergeSHA string
	merged, unmergeable           bool
	reviews                       []Review
	lines                         []ReviewComment
}

var _ Adapter = (*Fake)(nil)

// NewFake is the forge of the issues repository repo (`<owner>/<repo>`)
// and the declared repositories names, whose token is account's; o's
// clock dates every entry.
func NewFake(o ops.Ops, repo string, names []string, account Account) *Fake {
	owner, r, _ := strings.Cut(repo, "/")
	return &Fake{o: o, owner: owner, repo: r, names: slices.Clone(names), account: account, items: map[Ref]*item{}, next: map[string]int{}}
}

// RefuseLabels makes every later label create be refused: EnsureLabels
// fails as Gitea's refused create does, creating nothing more.
func (f *Fake) RefuseLabels() { f.mu.Lock(); defer f.mu.Unlock(); f.refuse = true }

// AddLabels creates labels as an owner of the organization does by hand,
// refused or not.
func (f *Fake) AddLabels(names ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addLabels(names...)
}

func (f *Fake) addLabels(names ...string) {
	for _, n := range names {
		if !slices.Contains(f.labels, n) {
			f.labels = append(f.labels, n)
		}
	}
}

// Labels is the label set, in creation order.
func (f *Fake) Labels() []string { f.mu.Lock(); defer f.mu.Unlock(); return slices.Clone(f.labels) }

func (f *Fake) EnsureLabels(names []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var missing []string
	for _, n := range names {
		if !slices.Contains(f.labels, n) && !slices.Contains(missing, n) {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 && f.refuse {
		return 0, refusedLabels(f.owner, missing)
	}
	f.addLabels(missing...)
	return len(missing), nil
}

func (f *Fake) Login() (Account, error) { f.mu.Lock(); defer f.mu.Unlock(); return f.account, nil }

// find is the item at, as the forge answers a number it does not hold.
func (f *Fake) find(at Ref) (*item, error) {
	if it, ok := f.items[at]; ok {
		return it, nil
	}
	return nil, fmt.Errorf("forge: %s/%s#%d: 404 not found", f.owner, at.Repo, at.N)
}

func (f *Fake) findPull(pr Ref) (*item, error) {
	it, err := f.find(pr)
	if err == nil && it.pull == nil {
		err = fmt.Errorf("forge: %s/%s#%d: 404 not a pull request", f.owner, pr.Repo, pr.N)
	}
	return it, err
}

// must is a helper's item: a number the Fake does not hold is the test's
// mistake.
func must(it *item, err error) *item {
	if err != nil {
		panic("forge.Fake: " + err.Error())
	}
	return it
}

func (f *Fake) add(repo string, it *item) int {
	f.next[repo]++
	n := f.next[repo]
	f.items[Ref{repo, n}] = it
	return n
}

func (it *item) issue(n int) Issue {
	return Issue{Number: n, Title: it.title, Body: it.body, Author: it.author, Created: it.created, Closed: it.closed, Open: it.open, Labels: slices.Clone(it.labels)}
}

// Issues: forge.repo's issues in state carrying label, by number; pull
// requests excluded.
func (f *Fake) Issues(state State, label string) ([]Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Issue
	for at, it := range f.items {
		if at.Repo == f.repo && it.pull == nil && it.open == (state == Open) && slices.Contains(it.labels, label) {
			out = append(out, it.issue(at.N))
		}
	}
	slices.SortFunc(out, func(a, b Issue) int { return a.Number - b.Number })
	return out, nil
}

func (f *Fake) Issue(n int) (Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.find(Ref{f.repo, n})
	if err != nil {
		return Issue{}, err
	}
	is := it.issue(n)
	is.Comments = slices.Clone(it.comments)
	return is, nil
}

// known: every label exists, as Gitea's id lookup requires.
func (f *Fake) known(method, repo string, labels ...string) error {
	for _, l := range labels {
		if !slices.Contains(f.labels, l) {
			return noLabel(method, f.owner, repo, l)
		}
	}
	return nil
}

func (f *Fake) CreateIssue(title, body string, labels []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.known("CreateIssue", f.repo, labels...); err != nil {
		return 0, err
	}
	var ls []string
	for _, l := range labels {
		if !slices.Contains(ls, l) {
			ls = append(ls, l)
		}
	}
	return f.add(f.repo, &item{title: title, body: body, author: f.account.Login, created: f.o.Now(), open: true, labels: ls}), nil
}

func (f *Fake) Comment(at Ref, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.find(at)
	if err != nil {
		return err
	}
	it.comments = append(it.comments, Comment{Author: f.account.Login, Body: body, Created: f.o.Now()})
	return nil
}

// Say is a comment by login on an issue or a pull request.
func (f *Fake) Say(at Ref, login, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it := must(f.find(at))
	it.comments = append(it.comments, Comment{Author: login, Body: body, Created: f.o.Now()})
}

// SayOnLine is a line comment by login on the pull request's current head.
func (f *Fake) SayOnLine(pr Ref, login, path string, line int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it := must(f.findPull(pr))
	it.pull.lines = append(it.pull.lines, ReviewComment{Author: login, Body: body, Path: path, Commit: it.pull.headSHA, Line: line, Created: f.o.Now()})
}

// Relabel adds add, then removes remove ("" removes nothing); both must
// exist in the label set, and no other label is touched.
func (f *Fake) Relabel(at Ref, remove, add string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.find(at)
	if err != nil {
		return err
	}
	check := []string{add}
	if remove != "" {
		check = append(check, remove)
	}
	if err := f.known("Relabel", at.Repo, check...); err != nil {
		return err
	}
	if !slices.Contains(it.labels, add) {
		it.labels = append(it.labels, add)
	}
	if remove != "" && remove != add {
		it.labels = slices.DeleteFunc(it.labels, func(l string) bool { return l == remove })
	}
	return nil
}

// SetLabel is the maintainer moving a stage by hand: every swarm:* label
// at carries replaced by label, which the label set must hold; the others
// kept.
func (f *Fake) SetLabel(at Ref, label string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it := must(f.find(at))
	if err := f.known("SetLabel", at.Repo, label); err != nil {
		panic("forge.Fake: " + err.Error())
	}
	it.labels = slices.DeleteFunc(it.labels, func(l string) bool { return strings.HasPrefix(l, "swarm:") })
	it.labels = append(it.labels, label)
}

func (f *Fake) CloseIssue(n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.find(Ref{f.repo, n})
	if err != nil {
		return err
	}
	if it.open {
		it.open, it.closed = false, f.o.Now()
	}
	return nil
}

// CreatePull opens a pull request in the declared repository repo; its
// head commit is "" until Push moves it, since the Fake holds no git.
// Like the forge, it refuses a second open pull request from head to base.
func (f *Fake) CreatePull(repo, head, base, title, body string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !slices.Contains(f.names, repo) {
		return 0, fmt.Errorf("forge: CreatePull: %s is not a declared repository (%s)", repo, strings.Join(f.names, ", "))
	}
	for at, it := range f.items {
		if at.Repo == repo && it.pull != nil && it.open && it.pull.head == head && it.pull.base == base {
			return 0, fmt.Errorf("forge: POST /repos/%s/%s/pulls: 409 pull request already exists for these targets", f.owner, repo)
		}
	}
	it := &item{title: title, body: body, author: f.account.Login, created: f.o.Now(), open: true, pull: &pullState{head: head, base: base}}
	return f.add(repo, it), nil
}

// Push moves a pull request's head to sha.
func (f *Fake) Push(pr Ref, sha string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	must(f.findPull(pr)).pull.headSHA = sha
}

// Unmergeable makes every later Merge of pr be refused.
func (f *Fake) Unmergeable(pr Ref) {
	f.mu.Lock()
	defer f.mu.Unlock()
	must(f.findPull(pr)).pull.unmergeable = true
}

// Pulls: the swarm's pull requests for issue n in every declared
// repository (section 10's filter), by slice.
func (f *Fake) Pulls(n int) ([]Pull, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	type bySlice struct {
		s int
		p Pull
	}
	var found []bySlice
	for _, name := range f.names {
		for m := 1; m <= f.next[name]; m++ {
			it := f.items[Ref{name, m}]
			if it == nil || it.pull == nil {
				continue
			}
			s, ok := swarmPull(it.pull.head, it.body, f.owner, f.repo, name, n)
			if !ok {
				continue
			}
			found = append(found, bySlice{s, Pull{Repo: name, Number: m, Title: it.title, Body: it.body, Head: it.pull.head, HeadSHA: it.pull.headSHA,
				Base: it.pull.base, MergeSHA: it.pull.mergeSHA, Open: it.open, Merged: it.pull.merged, Labels: slices.Clone(it.labels), Created: it.created}})
		}
	}
	slices.SortStableFunc(found, func(a, b bySlice) int { return a.s - b.s })
	out := make([]Pull, len(found))
	for i, f := range found {
		out[i] = f.p
	}
	return out, nil
}

func (f *Fake) PullComments(pr Ref) ([]Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.findPull(pr)
	if err != nil {
		return nil, err
	}
	return slices.Clone(it.comments), nil
}

func (f *Fake) Reviews(pr Ref) ([]Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.findPull(pr)
	if err != nil {
		return nil, err
	}
	return slices.Clone(it.pull.reviews), nil
}

func (f *Fake) ReviewComments(pr Ref) ([]ReviewComment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.findPull(pr)
	if err != nil {
		return nil, err
	}
	return slices.Clone(it.pull.lines), nil
}

// Review stores a comment-type review on the pull request's current head:
// the fallback, since the Fake opens every pull request as its own login.
func (f *Fake) Review(pr Ref, _ Verdict, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.findPull(pr)
	if err != nil {
		return err
	}
	f.reviewID++
	it.pull.reviews = append(it.pull.reviews, Review{ID: f.reviewID, Author: f.account.Login, Body: body, State: Commented, Commit: it.pull.headSHA, Submitted: f.o.Now()})
	return nil
}

// Merge merges an open pull request whose head is head and which is not
// marked unmergeable, recording a merge commit; it closes no issue.
func (f *Fake) Merge(pr Ref, head string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, err := f.findPull(pr)
	if err != nil {
		return err
	}
	switch {
	case !it.open || it.pull.unmergeable:
		return fmt.Errorf("%w: 405 Please try again later", ErrNotMergeable)
	case it.pull.headSHA != head:
		return fmt.Errorf("%w: 409 head out of date", ErrNotMergeable)
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("merge %s#%d %s", pr.Repo, pr.N, head)))
	it.pull.mergeSHA, it.pull.merged = hex.EncodeToString(sum[:]), true
	it.open, it.closed = false, f.o.Now()
	return nil
}
