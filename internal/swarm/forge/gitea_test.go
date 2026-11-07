package forge

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
)

// The recorded pairs below are written from Gitea's API v1 Swagger
// (gitea.com's swagger.v1.json, version 29.0.0+dev, read 2026-11-06),
// one per call, by operationId: orgListLabels and orgCreateLabel
// (`/orgs/{org}/labels`; Label: id, name, color, description, exclusive,
// is_archived, url; CreateLabelOption requires name and color), userGetCurrent
// (`/user`; User: id, login, email, …), issueListIssues
// (`/repos/{owner}/{repo}/issues`, query state, labels as comma-separated
// names, type, page, limit) and repoListPullRequests
// (`/repos/{owner}/{repo}/pulls`, query state, page, limit; PullRequest:
// number, title, body, head.ref, head.sha, base.ref, state, merged,
// merge_commit_sha, labels, created_at). Error bodies are APIError
// (`{"message", "url"}`). A 403 on orgCreateLabel is not in the Swagger:
// the route requires the organization's ownership, which answers 403 to a
// member (confirmed on the instance at Step 1's smoke).
//
// Step 4's, from the same Swagger: issueGetIssue (`GET
// /repos/{owner}/{repo}/issues/{index}`, 200 Issue); issueGetComments
// (`GET …/issues/{index}/comments`, query since and before only, no page:
// 200 Comment list; Comment: id, body, user, created_at, …);
// repoListLabels (`GET /repos/{owner}/{repo}/labels`, page, limit);
// issueCreateIssue (`POST …/issues`, CreateIssueOption: title required,
// body, labels "list of label ids"; 201 Issue); issueCreateComment (`POST
// …/issues/{index}/comments`, CreateIssueCommentOption: body; 201);
// issueAddLabel (`POST …/issues/{index}/labels`, IssueLabelsOption: labels;
// 200 Label list); issueRemoveLabel (`DELETE …/issues/{index}/labels/{id}`,
// 204); issueEditIssue (`PATCH …/issues/{index}`, EditIssueOption: state;
// 201 Issue); repoCreatePullRequest (`POST …/pulls`,
// CreatePullRequestOption: head, base, title, body; 201 PullRequest);
// repoListPullReviews (`GET …/pulls/{index}/reviews`, page, limit;
// PullReview: id, body, state of APPROVED, PENDING, COMMENT,
// REQUEST_CHANGES, REQUEST_REVIEW, commit_id, user, submitted_at);
// repoGetPullReviewComments (`GET …/pulls/{index}/reviews/{id}/comments`,
// no page; PullReviewComment: body, path, position, commit_id, user,
// created_at); repoCreatePullReview (`POST …/pulls/{index}/reviews`,
// CreatePullReviewOptions: event, body; 200, 422); repoMergePullRequest
// (`POST …/pulls/{index}/merge`, MergePullRequestOption: do required,
// head_commit_id; 200, 405, 409).
//
// Step 9's, from the same Swagger (version 29.0.0+dev-160, read
// 2026-11-07): issueGetMilestonesList (`GET
// /repos/{owner}/{repo}/milestones`, query state "open, closed and all.
// Defaults to open", name, page, limit; 200 Milestone list; Milestone: id,
// title, description, state, open_issues, closed_issues, created_at,
// updated_at, closed_at, due_on); issueCreateMilestone (`POST …/milestones`,
// CreateMilestoneOption: title, description, due_on, state; 201
// Milestone); issueEditIssue again (EditIssueOption: milestone, an id;
// 201 Issue, whose milestone is a Milestone or null). issueGetMilestone
// (`GET …/milestones/{id}`) is not used: the list carries the counts.

// pair is one recorded request and its response.
type pair struct {
	method, path, body string // path relative to forge.api, with its query; body JSON or ""
	status             int
	resp               string
}

const (
	testAPI   = "https://forge.example.net/api/v1"
	testToken = "tok-0123456789abcdef"
)

// script is the Fake's HTTP: it answers each request with the next pair
// after checking the request against it.
type script struct {
	t     *testing.T
	pairs []pair
	next  int
	waits []time.Duration // what each retry waited, through Ops.After
}

func (s *script) Do(req *http.Request) (*http.Response, error) {
	s.t.Helper()
	if s.next >= len(s.pairs) {
		s.t.Fatalf("unexpected request %s %s", req.Method, req.URL)
	}
	p := s.pairs[s.next]
	s.next++
	if got := req.Method + " " + strings.TrimPrefix(req.URL.String(), testAPI); got != p.method+" "+p.path {
		s.t.Errorf("request %d: %s, want %s %s", s.next, got, p.method, p.path)
	}
	if got := req.Header.Get("Authorization"); got != "token "+testToken {
		s.t.Errorf("request %d: Authorization %q", s.next, got)
	}
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	if !sameJSON(string(body), p.body) {
		s.t.Errorf("request %d body: %s, want %s", s.next, body, p.body)
	}
	return &http.Response{StatusCode: p.status, Body: io.NopCloser(strings.NewReader(p.resp)), Header: http.Header{}}, nil
}

func (s *script) done() {
	s.t.Helper()
	if s.next != len(s.pairs) {
		s.t.Errorf("%d of %d recorded requests made", s.next, len(s.pairs))
	}
}

func sameJSON(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	var x, y any
	return json.Unmarshal([]byte(a), &x) == nil && json.Unmarshal([]byte(b), &y) == nil && reflect.DeepEqual(x, y)
}

func gitea(t *testing.T, names []string, pairs ...pair) (*Gitea, *script) {
	s := &script{t: t, pairs: pairs}
	o := &ops.Fake{HTTPFn: func() ops.Doer { return s }, AfterFn: func(d time.Duration) <-chan time.Time {
		s.waits = append(s.waits, d)
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}}
	return NewGitea(o, testAPI, testToken, "acme/app", names), s
}

func label(id int, name string) string {
	return `{"id":` + itoa(id) + `,"name":"` + name + `","exclusive":false,"is_archived":false,"color":"ededed","description":"","url":"` + testAPI + `/orgs/acme/labels/` + itoa(id) + `"}`
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func labelList(names ...string) string {
	var items []string
	for i, n := range names {
		items = append(items, label(i+1, n))
	}
	return "[" + strings.Join(items, ",") + "]"
}

const (
	listLabels  = "/orgs/acme/labels?page=1&limit=50"
	orgLabels   = "/orgs/acme/labels"
	createdBody = `{"id":99,"name":"x","color":"ededed","description":""}`
)

func createLabel(name string, status int, resp string) pair {
	return pair{"POST", orgLabels, `{"name":"` + name + `","color":"#ededed","description":""}`, status, resp}
}

func TestGiteaLogin(t *testing.T) {
	g, s := gitea(t, nil, pair{"GET", "/user", "", 200,
		`{"id":7,"login":"swarm-bot","login_name":"","source_id":0,"full_name":"","email":"swarm@example.net","avatar_url":"","html_url":"","language":"","is_admin":false,"active":true,"prohibit_login":false,"visibility":"public","type":"bot"}`})
	acct, err := g.Login()
	if err != nil || acct != (Account{Login: "swarm-bot", Email: "swarm@example.net"}) {
		t.Errorf("Login() = %+v, %v", acct, err)
	}
	s.done()
}

func TestGiteaEnsureLabels(t *testing.T) {
	refused := `{"message":"Must be an organization owner","url":"` + testAPI + `/swagger"}`
	all := Labels
	fifty := make([]string, 50)
	for i := range fifty {
		fifty[i] = "other-" + itoa(i)
	}
	for _, tc := range []struct {
		name    string
		pairs   []pair
		created int
		err     string
	}{
		{"all present: nothing created", []pair{{"GET", listLabels, "", 200, labelList(append([]string{"bug"}, all...)...)}}, 0, ""},
		{"the missing created", []pair{
			{"GET", listLabels, "", 200, labelList(all[:6]...)},
			createLabel("swarm:approved", 201, createdBody),
			createLabel("swarm:blocked", 201, createdBody),
		}, 2, ""},
		{"every page read", []pair{
			{"GET", listLabels, "", 200, labelList(fifty...)},
			{"GET", "/orgs/acme/labels?page=2&limit=50", "", 200, labelList(all[1:]...)},
			createLabel("swarm:proposed", 201, createdBody),
		}, 1, ""},
		{"a refused create (403) creates nothing more and names every missing label", []pair{
			{"GET", listLabels, "", 200, labelList(all[:5]...)},
			createLabel("swarm:reviewed", 403, refused),
		}, 0, "forge: EnsureLabels: acme's labels missing: swarm:reviewed, swarm:approved, swarm:blocked; an owner of acme creates them once"},
		{"a refusal (401) after one create names the rest", []pair{
			{"GET", listLabels, "", 200, labelList(all[:5]...)},
			createLabel("swarm:reviewed", 201, createdBody),
			createLabel("swarm:approved", 401, `{"message":"token is required"}`),
		}, 1, "forge: EnsureLabels: acme's labels missing: swarm:approved, swarm:blocked; an owner of acme creates them once"},
		{"a 404 on the list: not an organization", []pair{{"GET", listLabels, "", 404, `{"errors":null,"message":"not found","url":""}`}},
			0, "forge: EnsureLabels: acme is not an organization; kind gitea needs one (section 3)"},
		{"another failure is the forge's error", []pair{
			{"GET", listLabels, "", 200, "[]"},
			createLabel("swarm:proposed", 422, `{"message":"invalid color"}`),
		}, 0, "forge: POST /orgs/acme/labels: 422 invalid color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, s := gitea(t, nil, tc.pairs...)
			created, err := g.EnsureLabels(all)
			if created != tc.created || errText(err) != tc.err {
				t.Errorf("EnsureLabels = %d, %q; want %d, %q", created, errText(err), tc.created, tc.err)
			}
			s.done()
		})
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestGiteaIssues(t *testing.T) {
	g, s := gitea(t, nil,
		pair{"GET", "/repos/acme/app/issues?state=open&type=issues&labels=swarm%3Aproposed&page=1&limit=50", "", 200, `[
		  {"id":31,"number":12,"title":"Second","body":"b","user":{"login":"swarm-bot"},"state":"open","created_at":"2026-11-01T10:00:00Z","closed_at":null,"labels":[{"id":1,"name":"swarm:proposed"}],"milestone":` + milestone(7, "m0: First", 1, 0) + `},
		  {"id":30,"number":9,"title":"First","body":"a","user":{"login":"jane"},"state":"open","created_at":"2026-10-30T09:00:00Z","closed_at":null,"labels":[{"id":5,"name":"bug"},{"id":1,"name":"swarm:proposed"}]},
		  {"id":32,"number":14,"title":"Unlabelled","body":"","user":{"login":"jane"},"state":"open","created_at":"2026-11-02T09:00:00Z","closed_at":null,"labels":[]}
		]`})
	got, err := g.Issues(Open, "swarm:proposed")
	want := []Issue{
		{Number: 9, Title: "First", Body: "a", Author: "jane", Created: time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC), Open: true, Labels: []string{"bug", "swarm:proposed"}},
		{Number: 12, Title: "Second", Body: "b", Author: "swarm-bot", Created: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC), Open: true, Labels: []string{"swarm:proposed"}, Milestone: 7},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Issues = %+v, %v", got, err)
	}
	s.done()

	g, s = gitea(t, nil, pair{"GET", "/repos/acme/app/issues?state=closed&type=issues&labels=swarm%3Aready&page=1&limit=50", "", 200,
		`[{"number":3,"title":"Done","body":"","user":{"login":"jane"},"state":"closed","created_at":"2026-10-01T09:00:00Z","closed_at":"2026-10-02T09:00:00Z","labels":[{"name":"swarm:ready"}]}]`})
	got, err = g.Issues(Closed, "swarm:ready")
	if err != nil || len(got) != 1 || got[0].Open || !got[0].Closed.Equal(time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Issues(Closed) = %+v, %v", got, err)
	}
	s.done()
}

func TestGiteaPulls(t *testing.T) {
	pull := func(number int, ref, body, state string, merged bool, sha string) string {
		m, _ := json.Marshal(map[string]any{
			"id": 1000 + number, "number": number, "title": "T" + itoa(number), "body": body, "state": state, "merged": merged,
			"merge_commit_sha": sha, "head": map[string]string{"ref": ref, "sha": "h" + itoa(number)}, "base": map[string]string{"ref": "main"},
			"labels": []map[string]any{{"id": 4, "name": "swarm:in-review"}}, "created_at": "2026-11-03T08:00:00Z",
		})
		return string(m)
	}
	g, s := gitea(t, []string{"app", "lib"},
		pair{"GET", "/repos/acme/app/pulls?state=all&page=1&limit=50", "", 200, "[" + strings.Join([]string{
			pull(20, "swarm/7-2", "Slice two.\n\nrefs #7", "open", false, ""),
			pull(19, "swarm/7-3", "Refs acme/app#7", "open", false, ""),    // the long form is another repository's
			pull(18, "swarm/17-1", "Refs #17", "open", false, ""),          // another issue
			pull(16, "feature/x", "Refs #7", "open", false, ""),            // not a swarm branch
			pull(15, "swarm/7-01", "Refs #7", "open", false, ""),           // not a slice number
			pull(14, "swarm/7-4", "Refs #7 and more", "closed", false, ""), // no Refs line
		}, ",") + "]"},
		pair{"GET", "/repos/acme/lib/pulls?state=all&page=1&limit=50", "", 200, "[" + strings.Join([]string{
			pull(5, "swarm/7-1", "Lib slice.\nRefs acme/app#7\n", "closed", true, "m5"),
			pull(4, "swarm/7-5", "Refs #7", "open", false, ""), // the short form is the issues repository's
		}, ",") + "]"},
	)
	got, err := g.Pulls(7)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	want := []Pull{
		{Repo: "lib", Number: 5, Title: "T5", Body: "Lib slice.\nRefs acme/app#7\n", Head: "swarm/7-1", HeadSHA: "h5", Base: "main", MergeSHA: "m5", Merged: true, Labels: []string{"swarm:in-review"}, Created: created},
		{Repo: "app", Number: 20, Title: "T20", Body: "Slice two.\n\nrefs #7", Head: "swarm/7-2", HeadSHA: "h20", Base: "main", Open: true, Labels: []string{"swarm:in-review"}, Created: created},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Pulls(7) =\n%+v\nwant\n%+v", got, want)
	}
	s.done()
}

func TestGiteaErrorsNameTheEndpointNotTheToken(t *testing.T) {
	g, s := gitea(t, nil, pair{"GET", "/user", "", 401, `{"message":"invalid username, password or token","url":""}`})
	_, err := g.Login()
	if errText(err) != "forge: GET /user: 401 invalid username, password or token" || strings.Contains(errText(err), testToken) {
		t.Errorf("Login error %v", err)
	}
	s.done()

	oops := pair{"GET", "/user", "", 500, `<html>oops</html>`}
	g, s = gitea(t, nil, oops, oops, oops, oops)
	if _, err := g.Login(); errText(err) != "forge: GET /user: 500 Internal Server Error" {
		t.Errorf("Login error %v", err)
	}
	s.done()
}

// A 429 or 5xx is sent again after its Retry-After, else 10 s, at most
// three times (section 10); the body goes again with each.
func TestGiteaRetries(t *testing.T) {
	body := `{"body":"hello"}`
	busy := pair{"POST", "/repos/acme/app/issues/7/comments", body, 503, `{"message":"busy"}`}
	g, s := gitea(t, nil, busy,
		pair{"POST", "/repos/acme/app/issues/7/comments", body, 429, `{"message":"slow down"}`},
		pair{"POST", "/repos/acme/app/issues/7/comments", body, 201, `{"id":1}`})
	retryAfter := &headerDoer{inner: s, status: 429, header: "Retry-After", value: "2"}
	g.o.(*ops.Fake).HTTPFn = func() ops.Doer { return retryAfter }
	if err := g.Comment(Ref{"app", 7}, "hello"); err != nil {
		t.Fatal(err)
	}
	s.done()
	if !reflect.DeepEqual(s.waits, []time.Duration{10 * time.Second, 2 * time.Second}) {
		t.Errorf("waits %v", s.waits)
	}

	g, s = gitea(t, nil, busy, busy, busy, busy)
	if err := g.Comment(Ref{"app", 7}, "hello"); errText(err) != "forge: POST /repos/acme/app/issues/7/comments: 503 busy" {
		t.Errorf("after three retries: %v", err)
	}
	s.done()
	if len(s.waits) != 3 {
		t.Errorf("waits %v", s.waits)
	}

	g, s = gitea(t, nil, pair{"POST", "/repos/acme/app/issues/7/comments", body, 404, `{"message":"no"}`})
	g.Comment(Ref{"app", 7}, "hello")
	s.done()
	if len(s.waits) != 0 {
		t.Errorf("a 404 retried: %v", s.waits)
	}
}

// headerDoer adds a header to every response of status from inner.
type headerDoer struct {
	inner         *script
	status        int
	header, value string
}

func (d *headerDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.inner.Do(req)
	if err == nil && resp.StatusCode == d.status {
		resp.Header.Set(d.header, d.value)
	}
	return resp, err
}

func at(y, mo, d, h, mi int) time.Time { return time.Date(y, time.Month(mo), d, h, mi, 0, 0, time.UTC) }

func TestGiteaIssue(t *testing.T) {
	g, s := gitea(t, nil,
		pair{"GET", "/repos/acme/app/issues/7", "", 200,
			`{"id":40,"number":7,"title":"Drift","body":"Brief: docs/briefs/b.md — M0","user":{"login":"swarm-bot"},"state":"open","created_at":"2026-11-01T10:00:00Z","closed_at":null,"labels":[{"id":3,"name":"swarm:planning"}],"milestone":null,"pull_request":null}`},
		pair{"GET", "/repos/acme/app/issues/7/comments", "", 200, `[
		  {"id":501,"html_url":"","issue_url":"","pull_request_url":"","body":"plan","user":{"login":"swarm-bot"},"created_at":"2026-11-01T11:00:00Z","updated_at":"2026-11-01T11:00:00Z"},
		  {"id":502,"html_url":"","issue_url":"","pull_request_url":"","body":"ok","user":{"login":"jane"},"created_at":"2026-11-01T12:00:00Z","updated_at":"2026-11-01T12:00:00Z"}
		]`})
	got, err := g.Issue(7)
	want := Issue{Number: 7, Title: "Drift", Body: "Brief: docs/briefs/b.md — M0", Author: "swarm-bot", Created: at(2026, 11, 1, 10, 0), Open: true,
		Labels: []string{"swarm:planning"}, Comments: []Comment{{"swarm-bot", "plan", at(2026, 11, 1, 11, 0)}, {"jane", "ok", at(2026, 11, 1, 12, 0)}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Issue(7) = %+v, %v", got, err)
	}
	s.done()
}

// idPairs are the two label lists the id map reads for repository name:
// the organization's eight, and the repository's own swarm:ready (id 40),
// which shadows the organization's (id 2).
func idPairs(name string) []pair {
	return []pair{
		{"GET", listLabels, "", 200, labelList(Labels...)},
		{"GET", "/repos/acme/" + name + "/labels?page=1&limit=50", "", 200, `[` + label(40, "swarm:ready") + `,` + label(41, "bug") + `]`},
	}
}

func TestGiteaCreateIssue(t *testing.T) {
	g, s := gitea(t, nil, append(idPairs("app"),
		pair{"POST", "/repos/acme/app/issues", `{"title":"Add X","body":"Brief: b — M1","labels":[1]}`, 201,
			`{"id":77,"number":23,"title":"Add X","body":"Brief: b — M1","user":{"login":"swarm-bot"},"state":"open","labels":[{"id":1,"name":"swarm:proposed"}]}`})...)
	n, err := g.CreateIssue("Add X", "Brief: b — M1", []string{"swarm:proposed"})
	if n != 23 || err != nil {
		t.Errorf("CreateIssue = %d, %v", n, err)
	}
	s.done()

	g, s = gitea(t, nil, append(idPairs("app"),
		pair{"POST", "/repos/acme/app/issues", `{"title":"Y","body":"","labels":[40]}`, 201, `{"number":24}`})...)
	if n, err := g.CreateIssue("Y", "", []string{"swarm:ready"}); n != 24 || err != nil {
		t.Errorf("CreateIssue with the repository's label = %d, %v", n, err)
	}
	s.done()

	g, s = gitea(t, nil, idPairs("app")...)
	if _, err := g.CreateIssue("Z", "", []string{"swarm:nonesuch"}); errText(err) != "forge: CreateIssue: no label swarm:nonesuch for acme/app; run 'daisy swarm init', which ensures the eight" {
		t.Errorf("an unknown label: %v", err)
	}
	s.done()
}

func TestGiteaComment(t *testing.T) {
	g, s := gitea(t, []string{"app", "lib"},
		pair{"POST", "/repos/acme/lib/issues/5/comments", `{"body":"hello"}`, 201, `{"id":9,"body":"hello","user":{"login":"swarm-bot"}}`})
	if err := g.Comment(Ref{"lib", 5}, "hello"); err != nil {
		t.Error(err)
	}
	s.done()
}

func TestGiteaRelabel(t *testing.T) {
	// On a pull request in lib: add, then remove, each by its id; the
	// repository's own swarm:ready shadows the organization's.
	g, s := gitea(t, []string{"app", "lib"}, append(idPairs("lib"),
		pair{"POST", "/repos/acme/lib/issues/5/labels", `{"labels":[5]}`, 200, `[` + label(5, "swarm:in-review") + `]`},
		pair{"DELETE", "/repos/acme/lib/issues/5/labels/40", "", 204, ""})...)
	if err := g.Relabel(Ref{"lib", 5}, "swarm:ready", "swarm:in-review"); err != nil {
		t.Error(err)
	}
	s.done()

	g, s = gitea(t, nil, append(idPairs("app"),
		pair{"POST", "/repos/acme/app/issues/7/labels", `{"labels":[7]}`, 200, `[]`})...)
	if err := g.Relabel(Ref{"app", 7}, "", "swarm:approved"); err != nil {
		t.Errorf("remove \"\" only adds: %v", err)
	}
	s.done()

	g, s = gitea(t, nil, idPairs("app")...)
	if err := g.Relabel(Ref{"app", 7}, "swarm:nonesuch", "swarm:approved"); errText(err) != "forge: Relabel: no label swarm:nonesuch for acme/app; run 'daisy swarm init', which ensures the eight" {
		t.Errorf("an unknown label writes nothing: %v", err)
	}
	s.done()
}

func TestGiteaCloseIssue(t *testing.T) {
	g, s := gitea(t, nil, pair{"PATCH", "/repos/acme/app/issues/7", `{"state":"closed"}`, 201, `{"number":7,"state":"closed"}`})
	if err := g.CloseIssue(7); err != nil {
		t.Error(err)
	}
	s.done()
}

func TestGiteaCreatePull(t *testing.T) {
	g, s := gitea(t, []string{"app", "lib"},
		pair{"POST", "/repos/acme/lib/pulls", `{"head":"swarm/7-1","base":"main","title":"Slice 1","body":"Refs acme/app#7"}`, 201,
			`{"id":1005,"number":5,"title":"Slice 1","state":"open","head":{"ref":"swarm/7-1"},"base":{"ref":"main"}}`})
	if n, err := g.CreatePull("lib", "swarm/7-1", "main", "Slice 1", "Refs acme/app#7"); n != 5 || err != nil {
		t.Errorf("CreatePull = %d, %v", n, err)
	}
	s.done()

	g, s = gitea(t, []string{"app", "lib"})
	if _, err := g.CreatePull("other", "swarm/7-1", "main", "x", ""); errText(err) != "forge: CreatePull: other is not a declared repository (app, lib)" {
		t.Errorf("an undeclared repository: %v", err)
	}
	s.done()
}

func TestGiteaPullComments(t *testing.T) {
	g, s := gitea(t, []string{"app", "lib"}, pair{"GET", "/repos/acme/lib/issues/5/comments", "", 200,
		`[{"id":1,"body":"looks off","user":{"login":"jane"},"created_at":"2026-11-02T09:00:00Z"}]`})
	got, err := g.PullComments(Ref{"lib", 5})
	if err != nil || !reflect.DeepEqual(got, []Comment{{"jane", "looks off", at(2026, 11, 2, 9, 0)}}) {
		t.Errorf("PullComments = %+v, %v", got, err)
	}
	s.done()
}

const reviewList = `[
  {"id":11,"body":"VERDICT: request-changes","state":"REQUEST_CHANGES","commit_id":"c1","user":{"login":"jane"},"submitted_at":"2026-11-02T10:00:00Z","official":true,"stale":false,"dismissed":false,"comments_count":1},
  {"id":10,"body":"VERDICT: approve","state":"COMMENT","commit_id":"c1","user":{"login":"swarm-bot"},"submitted_at":"2026-11-02T09:00:00Z","comments_count":0},
  {"id":12,"body":"","state":"REQUEST_REVIEW","commit_id":"","user":{"login":"jane"},"submitted_at":"2026-11-02T11:00:00Z"},
  {"id":13,"body":"fine","state":"APPROVED","commit_id":"c2","user":{"login":"jane"},"submitted_at":"2026-11-02T12:00:00Z","comments_count":1}
]`

func TestGiteaReviews(t *testing.T) {
	g, s := gitea(t, nil, pair{"GET", "/repos/acme/app/pulls/20/reviews?page=1&limit=50", "", 200, reviewList})
	got, err := g.Reviews(Ref{"app", 20})
	want := []Review{
		{ID: 10, Author: "swarm-bot", Body: "VERDICT: approve", State: Commented, Commit: "c1", Submitted: at(2026, 11, 2, 9, 0)},
		{ID: 11, Author: "jane", Body: "VERDICT: request-changes", State: RequestChanges, Commit: "c1", Submitted: at(2026, 11, 2, 10, 0)},
		{ID: 13, Author: "jane", Body: "fine", State: Approve, Commit: "c2", Submitted: at(2026, 11, 2, 12, 0)},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Reviews =\n%+v\nwant\n%+v", got, want)
	}
	s.done()
}

func TestGiteaReviewComments(t *testing.T) {
	g, s := gitea(t, nil,
		pair{"GET", "/repos/acme/app/pulls/20/reviews?page=1&limit=50", "", 200, reviewList},
		pair{"GET", "/repos/acme/app/pulls/20/reviews/10/comments", "", 200, `[]`},
		pair{"GET", "/repos/acme/app/pulls/20/reviews/11/comments", "", 200,
			`[{"id":91,"body":"off by one","path":"a.go","position":12,"original_position":12,"commit_id":"c1","original_commit_id":"c1","user":{"login":"jane"},"created_at":"2026-11-02T10:00:00Z","pull_request_review_id":11}]`},
		pair{"GET", "/repos/acme/app/pulls/20/reviews/13/comments", "", 200,
			`[{"id":92,"body":"nice","path":"b.go","position":3,"commit_id":"c2","user":{"login":"jane"},"created_at":"2026-11-02T09:30:00Z","pull_request_review_id":13}]`},
	)
	got, err := g.ReviewComments(Ref{"app", 20})
	want := []ReviewComment{
		{Author: "jane", Body: "nice", Path: "b.go", Commit: "c2", Line: 3, Created: at(2026, 11, 2, 9, 30)},
		{Author: "jane", Body: "off by one", Path: "a.go", Commit: "c1", Line: 12, Created: at(2026, 11, 2, 10, 0)},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ReviewComments =\n%+v\nwant\n%+v", got, want)
	}
	s.done()
}

func TestGiteaReview(t *testing.T) {
	const path = "/repos/acme/app/pulls/20/reviews"
	accepted := `{"id":14,"state":"APPROVED"}`
	refused := `{"message":"approve your own pull is not allowed","url":""}`
	for _, tc := range []struct {
		name    string
		verdict Verdict
		pairs   []pair
		err     string
	}{
		{"the formal event accepted", Approve, []pair{{"POST", path, `{"event":"APPROVED","body":"b"}`, 200, accepted}}, ""},
		{"a 422 falls back to a comment-type review", RequestChanges, []pair{
			{"POST", path, `{"event":"REQUEST_CHANGES","body":"b"}`, 422, refused},
			{"POST", path, `{"event":"COMMENT","body":"b"}`, 200, `{"id":15,"state":"COMMENT"}`},
		}, ""},
		{"Commented posts the comment-type review alone", Commented, []pair{{"POST", path, `{"event":"COMMENT","body":"b"}`, 200, `{"id":16}`}}, ""},
		{"another refusal is the error, without the fallback", Approve, []pair{{"POST", path, `{"event":"APPROVED","body":"b"}`, 404, `{"message":"not found"}`}},
			"forge: POST /repos/acme/app/pulls/20/reviews: 404 not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, s := gitea(t, nil, tc.pairs...)
			if err := g.Review(Ref{"app", 20}, tc.verdict, "b"); errText(err) != tc.err {
				t.Errorf("Review: %v, want %q", err, tc.err)
			}
			s.done()
		})
	}
}

func TestGiteaMerge(t *testing.T) {
	const path = "/repos/acme/app/pulls/20/merge"
	body := `{"do":"merge","head_commit_id":"c2"}`
	for _, tc := range []struct {
		name         string
		status       int
		resp, err    string
		notMergeable bool
	}{
		{"merged", 200, "", "", false},
		{"405: not mergeable", 405, `{"message":"Please try again later"}`, "forge: not mergeable: 405 Please try again later", true},
		{"409: the head moved", 409, `{"message":"head out of date"}`, "forge: not mergeable: 409 head out of date", true},
		{"403 is the forge's error", 403, `{"message":"forbidden"}`, "forge: POST /repos/acme/app/pulls/20/merge: 403 forbidden", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, s := gitea(t, nil, pair{"POST", path, body, tc.status, tc.resp})
			err := g.Merge(Ref{"app", 20}, "c2")
			if errText(err) != tc.err || errors.Is(err, ErrNotMergeable) != tc.notMergeable {
				t.Errorf("Merge: %v", err)
			}
			s.done()
		})
	}
}

func milestone(id int, title string, open, closed int) string {
	return `{"id":` + itoa(id) + `,"title":"` + title + `","description":"","state":"open","open_issues":` + itoa(open) + `,"closed_issues":` + itoa(closed) +
		`,"created_at":"2026-11-07T09:00:00Z","updated_at":"2026-11-07T09:00:00Z","closed_at":null,"due_on":null}`
}

const listMilestones = "/repos/acme/app/milestones?state=all&page=1&limit=50"

// Milestones reads every page in every state, with the forge's counts.
func TestGiteaMilestones(t *testing.T) {
	fifty := make([]string, 50)
	for i := range fifty {
		fifty[i] = milestone(100+i, "old "+itoa(i), 0, 1)
	}
	g, s := gitea(t, []string{"app", "lib"},
		pair{"GET", listMilestones, "", 200, "[" + strings.Join(fifty, ",") + "]"},
		pair{"GET", "/repos/acme/app/milestones?state=all&page=2&limit=50", "", 200, "[" + milestone(7, "m0: First", 2, 3) + "]"})
	got, err := g.Milestones()
	if err != nil || len(got) != 51 || got[50] != (Milestone{ID: 7, Title: "m0: First", Open: 2, Closed: 3}) || got[0] != (Milestone{ID: 100, Title: "old 0", Closed: 1}) {
		t.Errorf("Milestones = %+v, %v", got, err)
	}
	s.done()
}

func TestGiteaEnsureMilestone(t *testing.T) {
	closed := strings.Replace(milestone(8, "m0: Second", 0, 2), `"state":"open"`, `"state":"closed"`, 1)
	have := "[" + milestone(7, "m0: First", 1, 0) + "," + closed + "," + milestone(9, "m0: First", 0, 0) + "]"
	for _, tc := range []struct {
		name, title string
		pairs       []pair
		id          int64
		err         string
	}{
		{"found: nothing created, the first of the title", "m0: First", []pair{{"GET", listMilestones, "", 200, have}}, 7, ""},
		{"a closed milestone is found", "m0: Second", []pair{{"GET", listMilestones, "", 200, have}}, 8, ""},
		{"missing: created", "m0: Third", []pair{
			{"GET", listMilestones, "", 200, have},
			{"POST", "/repos/acme/app/milestones", `{"title":"m0: Third"}`, 201, milestone(12, "m0: Third", 0, 0)},
		}, 12, ""},
		{"a refused create is the forge's error", "m0: Third", []pair{
			{"GET", listMilestones, "", 200, "[]"},
			{"POST", "/repos/acme/app/milestones", `{"title":"m0: Third"}`, 403, `{"message":"token does not have at least one of required scope(s): [write:issue]"}`},
		}, 0, "forge: POST /repos/acme/app/milestones: 403 token does not have at least one of required scope(s): [write:issue]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, s := gitea(t, nil, tc.pairs...)
			id, err := g.EnsureMilestone(tc.title)
			if id != tc.id || errText(err) != tc.err {
				t.Errorf("EnsureMilestone = %d, %q; want %d, %q", id, errText(err), tc.id, tc.err)
			}
			s.done()
		})
	}
}

func TestGiteaSetMilestone(t *testing.T) {
	const path = "/repos/acme/app/issues/7"
	edited := func(m string) string {
		return `{"id":40,"number":7,"title":"Drift","body":"","user":{"login":"swarm-bot"},"state":"open","labels":[],"milestone":` + m + `}`
	}
	for _, tc := range []struct {
		name   string
		status int
		resp   string
		err    string
	}{
		{"set", 201, edited(milestone(12, "m0: Third", 1, 0)), ""},
		{"201 without the change: no write access", 201, edited("null"),
			"forge: PATCH /repos/acme/app/issues/7: 201 with milestone 0, not 12; the token's account needs write access to the issues of acme/app"},
		{"403 is the forge's error", 403, `{"message":"forbidden"}`, "forge: PATCH /repos/acme/app/issues/7: 403 forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, s := gitea(t, []string{"app", "lib"}, pair{"PATCH", path, `{"milestone":12}`, tc.status, tc.resp})
			if err := g.SetMilestone(7, 12); errText(err) != tc.err {
				t.Errorf("SetMilestone: %v, want %q", err, tc.err)
			}
			s.done()
		})
	}
}
