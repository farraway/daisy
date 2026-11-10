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
	g.mux.HandleFunc("PATCH /api/v1/orgs/acme/labels/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		var in struct{ Name string }
		json.NewDecoder(r.Body).Decode(&in)
		for i, l := range g.issue1 { // the id kept: issue 1 reads the new name
			if l == g.labels[id-1] {
				g.issue1[i] = in.Name
			}
		}
		g.labels[id-1] = in.Name
		reply(w, 200, map[string]any{"id": id, "name": in.Name})
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
		!strings.Contains(out, "✓ Labels: 16 present (16 created, 0 renamed)\n") || !strings.Contains(errs, "⚠ Readiness: app has no AGENTS.md") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if len(g.labels) != 16 {
		t.Errorf("labels %q", g.labels)
	}
	code, out, _ = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults")
	if code != 0 || !strings.Contains(out, "  Unchanged swarm/swarm.json\n") || !strings.Contains(out, "✓ Labels: 16 present (0 created, 0 renamed)\n") || len(g.labels) != 16 {
		t.Errorf("second run: exit %d\n%s", code, out)
	}

	// The organization of Step 14, its eleven labels under the old names:
	// the seven old stage names renamed in place, by id, issue 1 reading
	// its new stage, and the four new labels created.
	g = newGitea(t)
	g.labels = []string{"swarm:proposed", "swarm:ready", "swarm:planning", "swarm:planned", "swarm:in-review", "swarm:reviewed", "swarm:approved", "swarm:blocked", "swarm:gated", "swarm:done", "swarm:rejected"}
	g.issue1 = []string{"swarm:in-review"}
	code, out, errs = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults")
	var patched int
	for _, r := range g.requests {
		if strings.HasPrefix(r, "PATCH /api/v1/orgs/acme/labels/") {
			patched++
		}
	}
	if code != 0 || !strings.Contains(out, "✓ Labels: 16 present (5 created, 7 renamed)\n") || patched != 7 || len(g.labels) != 16 ||
		g.labels[4] != "swarm:review-0-ready" || !slices.Equal(g.issue1, []string{"swarm:review-0-ready"}) {
		t.Errorf("the migration: exit %d, %d renames, labels %q, issue 1 %q\n%s%s", code, patched, g.labels, g.issue1, out, errs)
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
		!strings.Contains(out, "\n  proposal-posted            0\n") || !strings.Contains(out, "\nWaiting for you: nothing\nHeld: nothing\nRunning: #4 worker\nPersonas:\n") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	for _, r := range g.requests {
		if !strings.HasPrefix(r, "GET ") {
			t.Errorf("status sent %s", r)
		}
	}
	if len(g.requests) != 21 {
		t.Errorf("requests %q: Login, then Issues open for each of the nine stages and closed for each of them and the two terminals", g.requests)
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
	g.issue1 = []string{"swarm:proposal-posted"}
	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export GITEA_TOKEN=tok-123\nexport DAISY_MAINTAINER_TOKEN=jane-456\n")
	g.requests, g.auths = nil, nil
	code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "triage", "1")
	if code != 0 || errs != "" || out != "Maintainer jane on acme/app\n#1 swarm:proposal-posted → swarm:proposal-accepted\n" || !slices.Equal(g.issue1, []string{"swarm:proposal-accepted"}) {
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

// An empty word in <n>'s place is a usage error, not the comment's first
// word shifted into it (`swarm close "" 7 dup` is not #7); an empty word in
// the comment is no argument.
func TestSwarmVerbsEmptyIssueWord(t *testing.T) {
	m := newMachine(t)
	for _, args := range [][]string{{"swarm", "close", "", "7", "dup"}, {"swarm", "unblock", "", "7", "dup"}, {"help", "", "status"}} {
		if code, out, errs := runWith(m.planOps(m.ws), args...); code != 2 || out != "" || !strings.HasPrefix(errs, "Error: empty argument\n") {
			t.Errorf("%q: exit %d out %q err %q", args, code, out, errs)
		}
	}
}

// The swarm verbs need no Daisy workspace (revision 6, Step 2): through
// the cli, in a swarm repository's clone outside any workspace (its
// swarm.json and prompts/ copied from a workspace's, as a maintainer moves
// a swarm by hand), pause and resume act on its <state>, and check-secrets
// reports the swarm's variables from <state>/env, refusing one others may
// read; in the workspace, check-secrets adds the old layout's swarm, from
// .env.sh, after the workspace's report.
func TestSwarmVerbsWithoutAWorkspace(t *testing.T) {
	m := newMachine(t)
	m.product(t, "git@forge.example.net:acme/app.git")
	g := newGitea(t)
	if code, out, errs := runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults"); code != 0 {
		t.Fatalf("init: exit %d\n%s\n%s", code, out, errs)
	}
	code, out, errs := runWith(m.swarmOps(m.ws, g), "check-secrets")
	wantWS := "Checking Daisy environment configuration...\n\nCore paths:\n✅ DAISY_ROOT: " + string([]rune(m.root)[:8]) + "...\n⚠️  DAISY_HOME: Not configured\n\nAPI tokens:\n⚠️  Webex API: Not configured\n⚠️  Backstage: Not configured\n⚠️  JIRA API: Not configured\n⚠️  GitHub: Not configured\n\n" +
		"Swarm acme/app (" + filepath.Join(m.ws, ".env.sh") + "):\n✅ GITEA_TOKEN: tok-123...\n⚠️  CLAUDE_CODE_OAUTH_TOKEN: Not configured\n⚠️  DAISY_MAINTAINER_TOKEN: Not configured\n\nNote: Only showing first 8 characters of tokens for security\n"
	if code != 0 || out != wantWS || errs != "" {
		t.Errorf("check-secrets in the workspace: exit %d %q\n%s\nwant\n%s", code, errs, out, wantWS)
	}

	clone := filepath.Join(t.TempDir(), "swarm-app")
	m.gitIn(t, "", "init", "-q", clone)
	b, _ := os.ReadFile(filepath.Join(m.ws, "swarm", "swarm.json"))
	writeFile(t, filepath.Join(clone, "swarm.json"), string(b))
	for _, r := range []string{"owner", "architect", "worker", "adversary", "sympathetic"} {
		b, _ := os.ReadFile(filepath.Join(m.ws, "swarm", "prompts", r+".md"))
		writeFile(t, filepath.Join(clone, "prompts", r+".md"), string(b))
	}
	inside := filepath.Join(clone, "prompts")
	state := filepath.Join(m.local, "swarm", "forge.example.net", "acme", "app")
	code, out, errs = runWith(m.swarmOps(inside, g), "swarm", "pause")
	if code != 0 || out != "Paused: no new member will launch ("+filepath.Join(state, "paused")+"); running members finish and are posted\n" || errs != "" {
		t.Errorf("pause: exit %d %q %q", code, out, errs)
	}
	if code, out, errs = runWith(m.swarmOps(inside, g), "swarm", "resume"); code != 0 || out != "Resumed: members launch at the next pass\n" {
		t.Errorf("resume: exit %d %q %q", code, out, errs)
	}

	env := filepath.Join(state, "env")
	if err := os.WriteFile(env, []byte("export GITEA_TOKEN=\"tok-456\"\nexport DAISY_MAINTAINER_TOKEN=\"jane-456\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runWith(m.swarmOps(inside, g), "check-secrets")
	want := "Checking Daisy environment configuration...\n\nSwarm acme/app (" + env + "):\n✅ GITEA_TOKEN: tok-456...\n⚠️  CLAUDE_CODE_OAUTH_TOKEN: Not configured\n✅ DAISY_MAINTAINER_TOKEN: jane-456...\n\nNote: Only showing first 8 characters of tokens for security\n"
	if code != 0 || out != want || errs != "" {
		t.Errorf("check-secrets in the clone: exit %d %q\n%s\nwant\n%s", code, errs, out, want)
	}
	os.Chmod(env, 0o640)
	code, out, _ = runWith(m.swarmOps(inside, g), "check-secrets")
	if code != 1 || !strings.Contains(out, "❌ "+env+" is readable by others (mode 0640); run chmod 600 on it\n") || strings.Contains(out, "tok-456") {
		t.Errorf("check-secrets refused: exit %d\n%s", code, out)
	}
	code, _, errs = runWith(m.swarmOps(t.TempDir(), g), "swarm", "pause")
	if code != 1 || !strings.HasPrefix(errs, "Error: swarm.json: not found in ") {
		t.Errorf("no swarm: exit %d %q", code, errs)
	}
}
