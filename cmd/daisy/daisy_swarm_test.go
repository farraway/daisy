package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// giteaServer is an in-memory Gitea answering the calls `swarm init`
// makes, for the organization acme, the token tok-123 and the swarm's
// account swarm-bot, and issue 1 of acme/app for the maintainer's verbs.
type giteaServer struct {
	mux      *http.ServeMux
	labels   []string
	accounts map[string]string // each Authorization header the forge takes, and its login
	issue1   []string          // issue 1's labels
	requests []string          // method and path of each request, in order
	auths    []string          // the Authorization header of each request, in order
}

func newGitea(t *testing.T) *giteaServer {
	g := &giteaServer{mux: http.NewServeMux(), accounts: map[string]string{"token tok-123": "swarm-bot"}}
	reply := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	g.mux.HandleFunc("GET /api/v1/user", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"id": 7, "login": g.accounts[r.Header.Get("Authorization")], "email": "swarm@example.net"})
	})
	g.mux.HandleFunc("GET /api/v1/repos/acme/app/issues/1", func(w http.ResponseWriter, r *http.Request) {
		var labels []map[string]any
		for _, l := range g.issue1 {
			labels = append(labels, map[string]any{"name": l})
		}
		reply(w, 200, map[string]any{"number": 1, "title": "A job", "body": "", "user": map[string]any{"login": "swarm-bot"},
			"created_at": "2026-11-07T09:00:00Z", "state": "open", "labels": labels})
	})
	g.mux.HandleFunc("GET /api/v1/repos/acme/app/issues/1/comments", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("GET /api/v1/repos/acme/app/labels", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("POST /api/v1/repos/acme/app/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Labels []int }
		json.NewDecoder(r.Body).Decode(&in)
		for _, id := range in.Labels {
			g.issue1 = append(g.issue1, g.labels[id-1])
		}
		reply(w, 200, []any{})
	})
	g.mux.HandleFunc("DELETE /api/v1/repos/acme/app/issues/1/labels/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		g.issue1 = slices.DeleteFunc(g.issue1, func(l string) bool { return l == g.labels[id-1] })
		w.WriteHeader(204)
	})
	g.mux.HandleFunc("GET /api/v1/repos/acme/{repo}/issues", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("GET /api/v1/repos/acme/{repo}/pulls", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("GET /api/v1/orgs/acme/labels", func(w http.ResponseWriter, r *http.Request) {
		var out []map[string]any
		for i, l := range g.labels {
			out = append(out, map[string]any{"id": i + 1, "name": l})
		}
		reply(w, 200, out)
	})
	g.mux.HandleFunc("POST /api/v1/orgs/acme/labels", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name, Color string }
		json.NewDecoder(r.Body).Decode(&in)
		g.labels = append(g.labels, in.Name)
		reply(w, 201, map[string]any{"id": len(g.labels), "name": in.Name, "color": in.Color})
	})
	return g
}

func (g *giteaServer) Do(req *http.Request) (*http.Response, error) {
	g.requests = append(g.requests, req.Method+" "+req.URL.Path)
	g.auths = append(g.auths, req.Header.Get("Authorization"))
	rec := httptest.NewRecorder()
	if _, ok := g.accounts[req.Header.Get("Authorization")]; !ok {
		rec.WriteHeader(401)
		return rec.Result(), nil
	}
	g.mux.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func (m *machine) swarmOps(cwd string, g *giteaServer) *ops.Fake {
	return &ops.Fake{
		GetenvFn:     func(k string) string { return m.vars[k] },
		GetwdFn:      func() (string, error) { return cwd, nil },
		ExecutableFn: func() (string, error) { return "/opt/daisy/bin/daisy", nil },
		HTTPFn:       func() ops.Doer { return g },
	}
}

func (m *machine) product(t *testing.T, origin string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	m.gitIn(t, m.ws, "init", "-q")
	m.gitIn(t, m.ws, "remote", "add", "origin", origin)
	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export GITEA_TOKEN=tok-123\nexport GITHUB_TOKEN=tok-123\n")
}

// `swarm init --defaults` through the cli and the gitea adapter over an
// in-memory forge: the labels created once, the second run creating none.
func TestSwarmInitThroughGitea(t *testing.T) {
	m := newMachine(t)
	m.product(t, "git@forge.example.net:acme/app.git")
	g := newGitea(t)
	code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults")
	if code != 0 || !strings.HasPrefix(out, "✓ Token $GITEA_TOKEN reads acme/app as swarm-bot\n✓ Wrote swarm/swarm.json\n") ||
		!strings.Contains(out, "✓ Labels: 8 present (8 created)\n") || !strings.Contains(errs, "⚠ Readiness: app has no AGENTS.md") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if len(g.labels) != 8 {
		t.Errorf("labels %q", g.labels)
	}
	code, out, _ = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults")
	if code != 0 || !strings.Contains(out, "  Unchanged swarm/swarm.json\n") || !strings.Contains(out, "✓ Labels: 8 present (0 created)\n") || len(g.labels) != 8 {
		t.Errorf("second run: exit %d\n%s", code, out)
	}
}

func TestSwarmInitRefusals(t *testing.T) {
	m := newMachine(t)
	m.product(t, "git@github.com:acme/app.git")
	g := newGitea(t)
	code, _, errs := runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults")
	if code != 1 || errs != "Error: swarm: forge: kind github has no adapter yet; it is built when a run needs it (docs/swarm.md section 10)\n" {
		t.Errorf("github: exit %d %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(m.ws, "swarm")); err == nil {
		t.Error("github: swarm/ written")
	}
	code, _, errs = runWith(m.swarmOps(t.TempDir(), g), "swarm", "init")
	if code != 1 || !strings.Contains(errs, "Not in a Daisy workspace") {
		t.Errorf("outside a workspace: exit %d %q", code, errs)
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults", "extra")
	if code != 2 || !strings.Contains(errs, "Usage: daisy swarm init [--defaults]") {
		t.Errorf("usage: exit %d %q", code, errs)
	}
}

// `swarm status` through the cli, under its ReadOnly, and the gitea adapter
// over an in-memory forge: GETs only, docker ps the one command beside
// git's, and the swarm's own lines.
func TestSwarmStatusThroughGitea(t *testing.T) {
	m := newMachine(t)
	m.product(t, "git@forge.example.net:acme/app.git")
	g := newGitea(t)
	if code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults"); code != 0 {
		t.Fatalf("init: exit %d\n%s\n%s", code, out, errs)
	}
	g.requests = nil
	o := m.swarmOps(m.ws, g)
	var ran []string
	o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
		ran = append(ran, name+" "+args[0])
		if name == "docker" {
			return []byte("daisy-acme-app-4-worker-1 acme/app#4/worker openai/1\n"), nil, nil
		}
		return ops.Real().Exec(dir, name, args...)
	}
	code, out, errs := runWith(o, "swarm", "status")
	if code != 0 || errs != "" || !strings.HasPrefix(out, "Swarm: acme/app on forge.example.net (gitea), repositories app\n") ||
		!strings.Contains(out, "\n  proposed     0\n") || !strings.Contains(out, "\nWaiting for you: nothing\nRunning: #4 worker\nPersonas:\n") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	for _, r := range g.requests {
		if !strings.HasPrefix(r, "GET ") {
			t.Errorf("status sent %s", r)
		}
	}
	if len(g.requests) != 17 {
		t.Errorf("requests %q: Login, then Issues open and closed for each of the eight labels", g.requests)
	}
	for _, r := range ran {
		if !strings.HasPrefix(r, "git ") && r != "docker ps" {
			t.Errorf("status ran %s", r)
		}
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "status", "extra")
	if code != 2 || !strings.Contains(errs, "Usage: daisy swarm status") {
		t.Errorf("usage: exit %d %q", code, errs)
	}
}

// `swarm triage` through the cli and the gitea adapter: every request
// carries the maintainer's token, none the swarm's, and the relabel is
// made; the swarm's token as DAISY_MAINTAINER_TOKEN is refused before
// any request; a word that is no issue number is usage.
func TestSwarmTriageThroughGitea(t *testing.T) {
	m := newMachine(t)
	m.product(t, "git@forge.example.net:acme/app.git")
	g := newGitea(t)
	if code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults"); code != 0 {
		t.Fatalf("init: exit %d\n%s\n%s", code, out, errs)
	}
	g.accounts["token jane-456"] = "jane"
	g.issue1 = []string{"swarm:proposed"}
	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export GITEA_TOKEN=tok-123\nexport DAISY_MAINTAINER_TOKEN=jane-456\n")
	g.requests, g.auths = nil, nil
	code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "triage", "1")
	if code != 0 || errs != "" || out != "Maintainer jane on acme/app\n#1 swarm:proposed → swarm:ready\n" || !slices.Equal(g.issue1, []string{"swarm:ready"}) {
		t.Fatalf("exit %d\n%s\n%s\nlabels %q", code, out, errs, g.issue1)
	}
	if slices.ContainsFunc(g.auths, func(a string) bool { return a != "token jane-456" }) || !slices.Contains(g.requests, "DELETE /api/v1/repos/acme/app/issues/1/labels/1") {
		t.Errorf("requests %q under %q", g.requests, g.auths)
	}

	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export GITEA_TOKEN=tok-123\nexport DAISY_MAINTAINER_TOKEN=tok-123\n")
	g.requests = nil
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "triage", "1")
	if code != 1 || !strings.HasPrefix(errs, "Error: swarm: $DAISY_MAINTAINER_TOKEN holds the swarm's token ($GITEA_TOKEN)") || len(g.requests) != 0 {
		t.Errorf("the swarm's token: exit %d %q; requests %q", code, errs, g.requests)
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "triage", "one")
	if code != 2 || !strings.Contains(errs, "Error: not an issue number: one\nUsage: daisy swarm triage <n>") {
		t.Errorf("usage: exit %d %q", code, errs)
	}
}
