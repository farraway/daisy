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

// call sends one request and decodes a response of status want into out
// (out nil: the body is not read). Any other status is an *apiError.
func (g *Gitea) call(method, path string, in any, want int, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("forge: %s %s: %v", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, g.base+path, body)
	if err != nil {
		return fmt.Errorf("forge: %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.o.HTTP().Do(req)
	if err != nil {
		return fmt.Errorf("forge: %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("forge: %s %s: reading the response: %v", method, path, err)
	}
	if resp.StatusCode != want {
		return &apiError{method, path, resp.StatusCode, message(resp.StatusCode, data)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("forge: %s %s: %d with a body that is not the expected JSON: %v", method, path, resp.StatusCode, err)
	}
	return nil
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
}

// Issues is the issues of forge.repo in state carrying label, by number.
// Gitea drops a label filter naming no existing label and then answers
// every issue, so the label is checked on each issue as well.
func (g *Gitea) Issues(state State, label string) ([]Issue, error) {
	st := "open"
	if state == Closed {
		st = "closed"
	}
	path := fmt.Sprintf("/repos/%s/%s/issues?state=%s&type=issues&labels=%s", url.PathEscape(g.owner), url.PathEscape(g.repo), st, url.QueryEscape(label))
	raw, err := list[giteaIssue](g, path)
	if err != nil {
		return nil, err
	}
	var out []Issue
	for _, r := range raw {
		is := Issue{Number: r.Number, Title: r.Title, Body: r.Body, Author: r.User.Login, Created: r.Created, Open: r.State == "open"}
		if r.Closed != nil {
			is.Closed = *r.Closed
		}
		for _, l := range r.Labels {
			is.Labels = append(is.Labels, l.Name)
		}
		if slices.Contains(is.Labels, label) {
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
	State    string  `json:"state"`
	Merged   bool    `json:"merged"`
	MergeSHA *string `json:"merge_commit_sha"`
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
		raw, err := list[giteaPull](g, fmt.Sprintf("/repos/%s/%s/pulls?state=all", url.PathEscape(g.owner), url.PathEscape(name)))
		if err != nil {
			return nil, err
		}
		for _, r := range raw {
			s, ok := sliceOf(r.Head.Ref, n)
			if !ok || !refersTo(r.Body, g.owner, g.repo, name, n) {
				continue
			}
			p := Pull{Repo: name, Number: r.Number, Title: r.Title, Body: r.Body, Head: r.Head.Ref, HeadSHA: r.Head.SHA,
				Base: r.Base.Ref, Open: r.State == "open", Merged: r.Merged, Created: r.Created}
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

func (g *Gitea) Issue(n int) (Issue, error)                        { return Issue{}, notBuilt("Issue") }
func (g *Gitea) CreateIssue(string, string, []string) (int, error) { return 0, notBuilt("CreateIssue") }
func (g *Gitea) Comment(Ref, string) error                         { return notBuilt("Comment") }
func (g *Gitea) Relabel(Ref, string, string) error                 { return notBuilt("Relabel") }
func (g *Gitea) CloseIssue(int) error                              { return notBuilt("CloseIssue") }
func (g *Gitea) CreatePull(string, string, string, string, string) (int, error) {
	return 0, notBuilt("CreatePull")
}
func (g *Gitea) PullComments(Ref) ([]Comment, error)         { return nil, notBuilt("PullComments") }
func (g *Gitea) Reviews(Ref) ([]Review, error)               { return nil, notBuilt("Reviews") }
func (g *Gitea) ReviewComments(Ref) ([]ReviewComment, error) { return nil, notBuilt("ReviewComments") }
func (g *Gitea) Review(Ref, Verdict, string) error           { return notBuilt("Review") }
func (g *Gitea) Merge(Ref, string) error                     { return notBuilt("Merge") }
