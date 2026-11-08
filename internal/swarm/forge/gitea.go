package forge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
)

// pageLimit is the page size of every list call; a page shorter than it
// is the last.
const pageLimit = 50

// Gitea is the adapter for Gitea's API v1 (section 10, "Gitea (API v1)").
// The token travels only in the Authorization header; no error names it.
type Gitea struct {
	o           ops.Ops
	base, token string
	owner, repo string   // forge.repo
	names       []string // the declared repositories, in order
}

var _ Adapter = (*Gitea)(nil)

// NewGitea is the adapter for the API at base (forge.api), the issues
// repository repo (`<owner>/<repo>`) and the declared repositories' names.
func NewGitea(o ops.Ops, base, token, repo string, names []string) *Gitea {
	owner, r, _ := strings.Cut(repo, "/")
	return &Gitea{o: o, base: base, token: token, owner: owner, repo: r, names: names}
}

// apiError is a response other than the one expected: the method, the
// path, the status and the body's message.
type apiError struct {
	method, path string
	status       int
	message      string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("forge: %s %s: %d %s", e.method, e.path, e.status, e.message)
}

// retries is how many times a 429 or 5xx response is retried, and
// retryWait the wait when it carries no Retry-After (section 10).
const (
	retries   = 3
	retryWait = 10 * time.Second
)

// call sends one request and decodes a response of status want into out
// (out nil: the body is not read). A 429 or 5xx is sent again after its
// Retry-After, else retryWait, at most retries times; any other status,
// or the last of those, is an *apiError.
func (g *Gitea) call(method, path string, in any, want int, out any) error {
	var payload []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("forge: %s %s: %v", method, path, err)
		}
		payload = b
	}
	for attempt := 0; ; attempt++ {
		status, data, header, err := g.send(method, path, payload)
		if err != nil {
			return err
		}
		if (status == http.StatusTooManyRequests || status >= 500) && attempt < retries {
			<-g.o.After(retryAfter(header, g.o.Now()))
			continue
		}
		if status != want {
			return &apiError{method, path, status, message(status, data)}
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("forge: %s %s: %d with a body that is not the expected JSON: %v", method, path, status, err)
		}
		return nil
	}
}

// send makes one request and reads its whole response.
func (g *Gitea) send(method, path string, payload []byte) (int, []byte, http.Header, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, g.base+path, body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("forge: %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.o.HTTP().Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("forge: %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("forge: %s %s: reading the response: %v", method, path, err)
	}
	return resp.StatusCode, data, resp.Header, nil
}

// retryAfter is the wait a Retry-After header asks for, in seconds or as
// an HTTP date, else retryWait.
func retryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
		return 0
	}
	return retryWait
}

// message is a refusal's text: the body's `message` (Gitea's APIError),
// else the status text.
func message(status int, body []byte) string {
	var e struct{ Message string }
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	return http.StatusText(status)
}

// list reads every page of a list endpoint (path carries its own query,
// to which page and limit are added) into one slice.
func list[T any](g *Gitea, path string) ([]T, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var all []T
	for p := 1; ; p++ {
		var page []T
		if err := g.call("GET", fmt.Sprintf("%s%spage=%d&limit=%d", path, sep, p, pageLimit), nil, http.StatusOK, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageLimit {
			return all, nil
		}
	}
}

type giteaLabel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type giteaUser struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

// EnsureLabels works on the organization's labels, which every repository
// of the owner can carry: it lists them and creates each missing name. A
// refused create (401, 403) creates nothing more and names every label
// still missing; a 404 on the list is an owner that is no organization.
func (g *Gitea) EnsureLabels(names []string) (int, error) {
	path := "/orgs/" + url.PathEscape(g.owner) + "/labels"
	have, err := list[giteaLabel](g, path)
	var api *apiError
	if errors.As(err, &api) && api.status == http.StatusNotFound {
		return 0, fmt.Errorf("forge: EnsureLabels: %s is not an organization; kind gitea needs one (section 3)", g.owner)
	}
	if err != nil {
		return 0, err
	}
	var missing []string
	for _, n := range names {
		if !slices.ContainsFunc(have, func(l giteaLabel) bool { return l.Name == n }) && !slices.Contains(missing, n) {
			missing = append(missing, n)
		}
	}
	created := 0
	for i, n := range missing {
		err := g.call("POST", path, map[string]string{"name": n, "color": "#ededed", "description": ""}, http.StatusCreated, nil)
		if errors.As(err, &api) && (api.status == http.StatusUnauthorized || api.status == http.StatusForbidden) {
			return created, refusedLabels(g.owner, missing[i:])
		}
		if err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// repoPath is the API path of the repository name of the owner.
func (g *Gitea) repoPath(name string) string {
	return "/repos/" + url.PathEscape(g.owner) + "/" + url.PathEscape(name)
}

// Login reads the authenticated user.
func (g *Gitea) Login() (Account, error) {
	var u giteaUser
	if err := g.call("GET", "/user", nil, http.StatusOK, &u); err != nil {
		return Account{}, err
	}
	return Account{Login: u.Login, Email: u.Email}, nil
}

type giteaIssue struct {
	Number  int        `json:"number"`
	Title   string     `json:"title"`
	Body    string     `json:"body"`
	User    giteaUser  `json:"user"`
	Created time.Time  `json:"created_at"`
	Closed  *time.Time `json:"closed_at"`
	State   string     `json:"state"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *giteaMilestone `json:"milestone"`
	Pull      *struct{}       `json:"pull_request"`
}

func (r giteaIssue) issue() Issue {
	is := Issue{Number: r.Number, Title: r.Title, Body: r.Body, Author: r.User.Login, Created: r.Created, Open: r.State == "open", Pull: r.Pull != nil}
	if r.Closed != nil {
		is.Closed = *r.Closed
	}
	if r.Milestone != nil {
		is.Milestone = r.Milestone.ID
	}
	for _, l := range r.Labels {
		is.Labels = append(is.Labels, l.Name)
	}
	return is
}

// Issues is the issues of forge.repo in state carrying label, by number.
// Gitea drops a label filter naming no existing label and then answers
// every issue, so the label is checked on each issue as well.
func (g *Gitea) Issues(state State, label string) ([]Issue, error) {
	st := "open"
	if state == Closed {
		st = "closed"
	}
	path := fmt.Sprintf("%s/issues?state=%s&type=issues&labels=%s", g.repoPath(g.repo), st, url.QueryEscape(label))
	raw, err := list[giteaIssue](g, path)
	if err != nil {
		return nil, err
	}
	var out []Issue
	for _, r := range raw {
		if is := r.issue(); slices.Contains(is.Labels, label) {
			out = append(out, is)
		}
	}
	slices.SortFunc(out, func(a, b Issue) int { return a.Number - b.Number })
	return out, nil
}

type giteaPull struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Head   struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	State    string    `json:"state"`
	Merged   bool      `json:"merged"`
	MergeSHA *string   `json:"merge_commit_sha"`
	User     giteaUser `json:"user"`
	Labels   []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Created time.Time `json:"created_at"`
}

// Pulls reads every pull request, open and closed, of each declared
// repository and keeps the swarm's for issue n: head branch
// `swarm/<n>-<s>` and the Refs line for n. Sorted by slice.
func (g *Gitea) Pulls(n int) ([]Pull, error) {
	type bySlice struct {
		s int
		p Pull
	}
	var found []bySlice
	for _, name := range g.names {
		raw, err := list[giteaPull](g, g.repoPath(name)+"/pulls?state=all")
		if err != nil {
			return nil, err
		}
		for _, r := range raw {
			s, ok := swarmPull(r.Head.Ref, r.Body, g.owner, g.repo, name, n)
			if !ok {
				continue
			}
			p := Pull{Repo: name, Number: r.Number, Title: r.Title, Body: r.Body, Head: r.Head.Ref, HeadSHA: r.Head.SHA,
				Base: r.Base.Ref, Open: r.State == "open", Merged: r.Merged, Author: r.User.Login, Created: r.Created}
			if r.Merged && r.MergeSHA != nil {
				p.MergeSHA = *r.MergeSHA
			}
			for _, l := range r.Labels {
				p.Labels = append(p.Labels, l.Name)
			}
			found = append(found, bySlice{s, p})
		}
	}
	slices.SortStableFunc(found, func(a, b bySlice) int { return a.s - b.s })
	out := make([]Pull, len(found))
	for i, f := range found {
		out[i] = f.p
	}
	return out, nil
}

type giteaComment struct {
	Body    string    `json:"body"`
	User    giteaUser `json:"user"`
	Created time.Time `json:"created_at"`
}

// comments reads the comments of issue or pull request n in repository
// name, oldest first. Gitea's endpoint takes no page and answers them all.
func (g *Gitea) comments(name string, n int) ([]Comment, error) {
	var raw []giteaComment
	if err := g.call("GET", fmt.Sprintf("%s/issues/%d/comments", g.repoPath(name), n), nil, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	out := make([]Comment, len(raw))
	for i, r := range raw {
		out[i] = Comment{Author: r.User.Login, Body: r.Body, Created: r.Created}
	}
	return out, nil
}

// Issue reads issue n of forge.repo and its comments.
func (g *Gitea) Issue(n int) (Issue, error) {
	var r giteaIssue
	if err := g.call("GET", fmt.Sprintf("%s/issues/%d", g.repoPath(g.repo), n), nil, http.StatusOK, &r); err != nil {
		return Issue{}, err
	}
	is := r.issue()
	cs, err := g.comments(g.repo, n)
	if err != nil {
		return Issue{}, err
	}
	is.Comments = cs
	return is, nil
}

// labelIDs maps each label name to the id an issue or pull request of the
// repository name takes: the organization's labels, then the
// repository's own, which shadow an organization label of their name.
func (g *Gitea) labelIDs(name string) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, path := range []string{"/orgs/" + url.PathEscape(g.owner) + "/labels", g.repoPath(name) + "/labels"} {
		ls, err := list[giteaLabel](g, path)
		if err != nil {
			return nil, err
		}
		for _, l := range ls {
			ids[l.Name] = l.ID
		}
	}
	return ids, nil
}

// labelID is label's id for the repository name, from ids.
func (g *Gitea) labelID(method, name, label string, ids map[string]int64) (int64, error) {
	id, ok := ids[label]
	if !ok {
		return 0, noLabel(method, g.owner, name, label)
	}
	return id, nil
}

// CreateIssue opens an issue in forge.repo carrying labels.
func (g *Gitea) CreateIssue(title, body string, labels []string) (int, error) {
	ids, err := g.labelIDs(g.repo)
	if err != nil {
		return 0, err
	}
	in := struct {
		Title  string  `json:"title"`
		Body   string  `json:"body"`
		Labels []int64 `json:"labels"`
	}{title, body, []int64{}}
	for _, l := range labels {
		id, err := g.labelID("CreateIssue", g.repo, l, ids)
		if err != nil {
			return 0, err
		}
		in.Labels = append(in.Labels, id)
	}
	var out struct {
		Number int `json:"number"`
	}
	if err := g.call("POST", g.repoPath(g.repo)+"/issues", in, http.StatusCreated, &out); err != nil {
		return 0, err
	}
	return out.Number, nil
}

// Comment posts body on an issue or a pull request: Gitea keeps a pull
// request's comments as its issue's.
func (g *Gitea) Comment(at Ref, body string) error {
	return g.call("POST", fmt.Sprintf("%s/issues/%d/comments", g.repoPath(at.Repo), at.N), map[string]string{"body": body}, http.StatusCreated, nil)
}

// Relabel adds add, then removes remove ("" on either side: no call for
// it), on an issue or a pull request; no other label is touched.
func (g *Gitea) Relabel(at Ref, remove, add string) error {
	ids, err := g.labelIDs(at.Repo)
	if err != nil {
		return err
	}
	var addID, removeID int64
	if add != "" {
		if addID, err = g.labelID("Relabel", at.Repo, add, ids); err != nil {
			return err
		}
	}
	if remove != "" {
		if removeID, err = g.labelID("Relabel", at.Repo, remove, ids); err != nil {
			return err
		}
	}
	path := fmt.Sprintf("%s/issues/%d/labels", g.repoPath(at.Repo), at.N)
	if add != "" {
		if err := g.call("POST", path, map[string][]int64{"labels": {addID}}, http.StatusOK, nil); err != nil {
			return err
		}
	}
	if remove == "" {
		return nil
	}
	return g.call("DELETE", fmt.Sprintf("%s/%d", path, removeID), nil, http.StatusNoContent, nil)
}

// CloseIssue sets issue n's state to closed.
func (g *Gitea) CloseIssue(n int) error {
	return g.call("PATCH", fmt.Sprintf("%s/issues/%d", g.repoPath(g.repo), n), map[string]string{"state": "closed"}, http.StatusCreated, nil)
}

// CreatePull opens a pull request from head into base in the declared
// repository repo.
func (g *Gitea) CreatePull(repo, head, base, title, body string) (int, error) {
	if !slices.Contains(g.names, repo) {
		return 0, fmt.Errorf("forge: CreatePull: %s is not a declared repository (%s)", repo, strings.Join(g.names, ", "))
	}
	in := map[string]string{"head": head, "base": base, "title": title, "body": body}
	var out struct {
		Number int `json:"number"`
	}
	if err := g.call("POST", g.repoPath(repo)+"/pulls", in, http.StatusCreated, &out); err != nil {
		return 0, err
	}
	return out.Number, nil
}

// PullComments reads a pull request's comments, oldest first.
func (g *Gitea) PullComments(pr Ref) ([]Comment, error) { return g.comments(pr.Repo, pr.N) }

type giteaReview struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Commit    string    `json:"commit_id"`
	User      giteaUser `json:"user"`
	Submitted time.Time `json:"submitted_at"`
}

// reviewStates are the submitted reviews' states; Gitea lists a review
// request (REQUEST_REVIEW) and a pending review (PENDING) among them too,
// neither of which is a review anyone gave, so Reviews leaves them out.
var reviewStates = map[string]Verdict{"APPROVED": Approve, "REQUEST_CHANGES": RequestChanges, "COMMENT": Commented}

// Reviews reads a pull request's submitted reviews, oldest first.
func (g *Gitea) Reviews(pr Ref) ([]Review, error) {
	raw, err := list[giteaReview](g, fmt.Sprintf("%s/pulls/%d/reviews", g.repoPath(pr.Repo), pr.N))
	if err != nil {
		return nil, err
	}
	var out []Review
	for _, r := range raw {
		if v, ok := reviewStates[r.State]; ok {
			out = append(out, Review{ID: r.ID, Author: r.User.Login, Body: r.Body, State: v, Commit: r.Commit, Submitted: r.Submitted})
		}
	}
	slices.SortStableFunc(out, func(a, b Review) int { return a.Submitted.Compare(b.Submitted) })
	return out, nil
}

type giteaReviewComment struct {
	Body     string    `json:"body"`
	Path     string    `json:"path"`
	Position int       `json:"position"`
	Commit   string    `json:"commit_id"`
	User     giteaUser `json:"user"`
	Created  time.Time `json:"created_at"`
}

// ReviewComments reads the line comments of each review Reviews returns,
// oldest first. Gitea's endpoint takes no page and answers them all.
func (g *Gitea) ReviewComments(pr Ref) ([]ReviewComment, error) {
	reviews, err := g.Reviews(pr)
	if err != nil {
		return nil, err
	}
	var out []ReviewComment
	for _, r := range reviews {
		var raw []giteaReviewComment
		if err := g.call("GET", fmt.Sprintf("%s/pulls/%d/reviews/%d/comments", g.repoPath(pr.Repo), pr.N, r.ID), nil, http.StatusOK, &raw); err != nil {
			return nil, err
		}
		for _, c := range raw {
			out = append(out, ReviewComment{Author: c.User.Login, Body: c.Body, Path: c.Path, Commit: c.Commit, Line: c.Position, Created: c.Created})
		}
	}
	slices.SortStableFunc(out, func(a, b ReviewComment) int { return a.Created.Compare(b.Created) })
	return out, nil
}

// reviewEvents are the formal events Review posts first.
var reviewEvents = map[Verdict]string{Approve: "APPROVED", RequestChanges: "REQUEST_CHANGES"}

// Review posts the formal review of verdict and, when Gitea refuses that
// event (422: its own pull request, for one), the same body as a
// comment-type review. The verdict is read from the body, never from the
// review's state (section 10).
func (g *Gitea) Review(pr Ref, verdict Verdict, body string) error {
	path := fmt.Sprintf("%s/pulls/%d/reviews", g.repoPath(pr.Repo), pr.N)
	if event, ok := reviewEvents[verdict]; ok {
		err := g.call("POST", path, map[string]string{"event": event, "body": body}, http.StatusOK, nil)
		var api *apiError
		if !errors.As(err, &api) || api.status != http.StatusUnprocessableEntity {
			return err
		}
	}
	return g.call("POST", path, map[string]string{"event": "COMMENT", "body": body}, http.StatusOK, nil)
}

// Approve posts a formal approving review of the commit head, with no
// body; Gitea's refusal (422 for an approval of one's own pull request) is
// the error, never a comment-type review in its place.
func (g *Gitea) Approve(pr Ref, head string) error {
	return g.call("POST", fmt.Sprintf("%s/pulls/%d/reviews", g.repoPath(pr.Repo), pr.N), map[string]string{"event": "APPROVED", "body": "", "commit_id": head}, http.StatusOK, nil)
}

// Merge merges a pull request with a merge commit, only while its head is
// head; Gitea's refusal (405 not mergeable, 409 the head moved) is
// ErrNotMergeable with the forge's message.
func (g *Gitea) Merge(pr Ref, head string) error {
	err := g.call("POST", fmt.Sprintf("%s/pulls/%d/merge", g.repoPath(pr.Repo), pr.N), map[string]string{"do": "merge", "head_commit_id": head}, http.StatusOK, nil)
	var api *apiError
	if errors.As(err, &api) && (api.status == http.StatusMethodNotAllowed || api.status == http.StatusConflict) {
		return fmt.Errorf("%w: %d %s", ErrNotMergeable, api.status, api.message)
	}
	return err
}

type giteaMilestone struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Open   int    `json:"open_issues"`
	Closed int    `json:"closed_issues"`
}

// Milestones lists forge.repo's milestones in every state: without
// state=all, Gitea lists the open ones alone.
func (g *Gitea) Milestones() ([]Milestone, error) {
	raw, err := list[giteaMilestone](g, g.repoPath(g.repo)+"/milestones?state=all")
	if err != nil {
		return nil, err
	}
	out := make([]Milestone, len(raw))
	for i, r := range raw {
		out[i] = Milestone{ID: r.ID, Title: r.Title, Open: r.Open, Closed: r.Closed}
	}
	return out, nil
}

// EnsureMilestone reads the milestones first, since Gitea creates a
// second milestone of a title it already has; the first of the title is
// the one taken.
func (g *Gitea) EnsureMilestone(title string) (int64, error) {
	ms, err := g.Milestones()
	if err != nil {
		return 0, err
	}
	for _, m := range ms {
		if m.Title == title {
			return m.ID, nil
		}
	}
	var out giteaMilestone
	if err := g.call("POST", g.repoPath(g.repo)+"/milestones", map[string]string{"title": title}, http.StatusCreated, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// SetMilestone edits issue n's milestone and reads it back from the
// answer: Gitea answers 201 without the change to an account that may
// edit the issue, as its poster, but may not write the repository's
// issues.
func (g *Gitea) SetMilestone(n int, id int64) error {
	path := fmt.Sprintf("%s/issues/%d", g.repoPath(g.repo), n)
	var out giteaIssue
	if err := g.call("PATCH", path, map[string]int64{"milestone": id}, http.StatusCreated, &out); err != nil {
		return err
	}
	if got := out.issue().Milestone; got != id {
		return fmt.Errorf("forge: PATCH %s: 201 with milestone %d, not %d; the token's account needs write access to the issues of %s/%s", path, got, id, g.owner, g.repo)
	}
	return nil
}
