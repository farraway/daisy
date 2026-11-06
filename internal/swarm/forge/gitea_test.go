package forge

import (
	"encoding/json"
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
// member (to confirm on the instance).

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
	o := &ops.Fake{HTTPFn: func() ops.Doer { return s }}
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
		  {"id":31,"number":12,"title":"Second","body":"b","user":{"login":"swarm-bot"},"state":"open","created_at":"2026-11-01T10:00:00Z","closed_at":null,"labels":[{"id":1,"name":"swarm:proposed"}]},
		  {"id":30,"number":9,"title":"First","body":"a","user":{"login":"jane"},"state":"open","created_at":"2026-10-30T09:00:00Z","closed_at":null,"labels":[{"id":5,"name":"bug"},{"id":1,"name":"swarm:proposed"}]},
		  {"id":32,"number":14,"title":"Unlabelled","body":"","user":{"login":"jane"},"state":"open","created_at":"2026-11-02T09:00:00Z","closed_at":null,"labels":[]}
		]`})
	got, err := g.Issues(Open, "swarm:proposed")
	want := []Issue{
		{Number: 9, Title: "First", Body: "a", Author: "jane", Created: time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC), Open: true, Labels: []string{"bug", "swarm:proposed"}},
		{Number: 12, Title: "Second", Body: "b", Author: "swarm-bot", Created: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC), Open: true, Labels: []string{"swarm:proposed"}},
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

	g, s = gitea(t, nil, pair{"GET", "/user", "", 500, `<html>oops</html>`})
	if _, err := g.Login(); errText(err) != "forge: GET /user: 500 Internal Server Error" {
		t.Errorf("Login error %v", err)
	}
	s.done()
}

func TestNotBuiltMethodsNameTheStep(t *testing.T) {
	g, _ := gitea(t, nil)
	if err := g.Comment(Ref{"app", 1}, "x"); errText(err) != "forge: Comment is not built until Step 4" {
		t.Errorf("Comment: %v", err)
	}
}
