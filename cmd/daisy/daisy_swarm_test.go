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
// account swarm-bot, and issue 1 of acme/swarm-app, the swarm repository,
// for the maintainer's verbs.
type giteaServer struct {
	mux      *http.ServeMux
	labels   []string
	accounts map[string]string // each Authorization header the forge takes, and its login
	issue1   []string          // issue 1's labels
	requests []string          // method and path of each request, in order
	auths    []string          // the Authorization header of each request, in order
	bases    []string          // the scheme and host of each request, in order
	repo     map[string]any    // acme/swarm-app once created
	// each collaborator added to acme/swarm-app and its permission
	collaborators []string
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
	g.mux.HandleFunc("GET /api/v1/repos/acme/swarm-app/issues/1", func(w http.ResponseWriter, r *http.Request) {
		var labels []map[string]any
		for _, l := range g.issue1 {
			labels = append(labels, map[string]any{"name": l})
		}
		reply(w, 200, map[string]any{"number": 1, "title": "A job", "body": "", "user": map[string]any{"login": "swarm-bot"},
			"created_at": "2026-11-07T09:00:00Z", "state": "open", "labels": labels})
	})
	g.mux.HandleFunc("GET /api/v1/repos/acme/swarm-app/issues/1/comments", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("GET /api/v1/repos/acme/swarm-app/labels", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, []any{}) })
	g.mux.HandleFunc("POST /api/v1/repos/acme/swarm-app/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Labels []int }
		json.NewDecoder(r.Body).Decode(&in)
		for _, id := range in.Labels {
			g.issue1 = append(g.issue1, g.labels[id-1])
		}
		reply(w, 200, []any{})
	})
	g.mux.HandleFunc("DELETE /api/v1/repos/acme/swarm-app/issues/1/labels/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	g.mux.HandleFunc("GET /api/v1/repos/acme/swarm-app", func(w http.ResponseWriter, r *http.Request) {
		if g.repo == nil {
			reply(w, 404, map[string]any{"message": "The target couldn't be found."})
			return
		}
		reply(w, 200, g.repo)
	})
	g.mux.HandleFunc("POST /api/v1/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name    string
			Private bool
		}
		json.NewDecoder(r.Body).Decode(&in)
		g.repo = map[string]any{"name": in.Name, "full_name": "acme/" + in.Name, "private": in.Private, "empty": true,
			"ssh_url": "git@forge.example.net:acme/" + in.Name + ".git", "default_branch": "main"}
		reply(w, 201, g.repo)
	})
	g.mux.HandleFunc("PUT /api/v1/repos/acme/swarm-app/collaborators/{login}", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Permission string }
		json.NewDecoder(r.Body).Decode(&in)
		g.collaborators = append(g.collaborators, r.PathValue("login")+" "+in.Permission)
		w.WriteHeader(204)
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
	g.bases = append(g.bases, req.URL.Scheme+"://"+req.URL.Host)
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

// swarmFolder is a folder holding the product's clone, acme/app, outside
// any workspace, the swarm's token and the maintainer's, jane's, in the
// environment, and the maintainer's git identity; git reaches acme/app's
// and acme/swarm-app's upstreams, local bare repositories (swarmUp empty),
// in place of the forge's URLs.
func (m *machine) swarmFolder(t *testing.T) (dir, swarmUp string) {
	t.Helper()
	dir = t.TempDir()
	app, appUp := filepath.Join(dir, "app"), filepath.Join(t.TempDir(), "app.git")
	swarmUp = filepath.Join(t.TempDir(), "swarm-app.git")
	global := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, global, "[user]\n\tname = Jane\n\temail = jane@example.net\n"+
		"[url \""+appUp+"\"]\n\tinsteadOf = git@forge.example.net:acme/app.git\n[url \""+swarmUp+"\"]\n\tinsteadOf = git@forge.example.net:acme/swarm-app.git\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	m.gitIn(t, "", "init", "-q", "--bare", "-b", "main", swarmUp)
	m.gitIn(t, "", "init", "-q", "-b", "main", app)
	m.gitIn(t, app, "commit", "-q", "--allow-empty", "-m", "first")
	m.gitIn(t, "", "clone", "-q", "--bare", app, appUp)
	m.gitIn(t, app, "remote", "add", "origin", "git@forge.example.net:acme/app.git")
	m.vars["GITEA_TOKEN"], m.vars["DAISY_MAINTAINER_TOKEN"] = "tok-123", "jane-456"
	return dir, swarmUp
}

// swarm is the swarm repository's clone `swarm init --defaults` makes in
// swarmFolder's folder, through the cli and the gitea adapter over g.
func (m *machine) swarm(t *testing.T, g *giteaServer) string {
	t.Helper()
	g.accounts["token jane-456"] = "jane"
	dir, _ := m.swarmFolder(t)
	if code, out, errs := runWith(m.swarmOps(dir, g), "swarm", "init", "--defaults"); code != 0 {
		t.Fatalf("init: exit %d\n%s\n%s", code, out, errs)
	}
	return filepath.Join(dir, "swarm-app")
}

// `swarm init --defaults` through the cli and the gitea adapter over an
// in-memory forge: the labels created once, the second run, from the
// clone, creating none; an organization of Step 14's, its eleven labels
// under the old names: the seven old stage names renamed in place, by
// id, issue 1 reading its new stage, and the four new labels created.
func TestSwarmInitLabelsThroughGitea(t *testing.T) {
	m := newMachine(t)
	g := newGitea(t)
	clone := m.swarm(t, g)
	if len(g.labels) != 16 {
		t.Errorf("labels %q", g.labels)
	}
	code, out, _ := runWith(m.swarmOps(clone, g), "swarm", "init", "--defaults")
	if code != 0 || !strings.Contains(out, "  Unchanged swarm.json\n") || !strings.Contains(out, "✓ Token $GITEA_TOKEN: labels 16 present (0 created, 0 renamed)\n") || len(g.labels) != 16 {
		t.Errorf("second run: exit %d\n%s", code, out)
	}

	old := newGitea(t)
	old.accounts, old.repo = g.accounts, g.repo
	old.labels = []string{"swarm:proposed", "swarm:ready", "swarm:planning", "swarm:planned", "swarm:in-review", "swarm:reviewed", "swarm:approved", "swarm:blocked", "swarm:gated", "swarm:done", "swarm:rejected"}
	old.issue1 = []string{"swarm:in-review"}
	code, out, errs := runWith(m.swarmOps(clone, old), "swarm", "init", "--defaults")
	var patched int
	for _, r := range old.requests {
		if strings.HasPrefix(r, "PATCH /api/v1/orgs/acme/labels/") {
			patched++
		}
	}
	if code != 0 || !strings.Contains(out, "✓ Token $GITEA_TOKEN: labels 16 present (5 created, 7 renamed)\n") || patched != 7 || len(old.labels) != 16 ||
		old.labels[4] != "swarm:review-0-ready" || !slices.Equal(old.issue1, []string{"swarm:review-0-ready"}) {
		t.Errorf("the migration: exit %d, %d renames, labels %q, issue 1 %q\n%s%s", code, patched, old.labels, old.issue1, out, errs)
	}
}

// `swarm init --defaults` in revision 6 through the cli and the gitea
// adapter (section 13, revision 6's Step 6): in a folder holding a
// product's clone and no workspace, the maintainer's token creates
// acme/swarm-app private and gives swarm-bot write access, the swarm's
// token reads and makes the labels, each request under the token its line
// names; the clone is made beside the product's and its commit pushed; a
// second run, from inside the clone, adopts the repository and writes
// nothing.
func TestSwarmInitCreatesThroughGitea(t *testing.T) {
	m := newMachine(t)
	g := newGitea(t)
	g.accounts["token jane-456"] = "jane"
	dir, swarmUp := m.swarmFolder(t)

	// The forge's API at the address --api names (revision 6's plan, Step
	// 6b), where the products' SSH origins would guess https without the
	// port.
	code, out, errs := runWith(m.swarmOps(dir, g), "swarm", "init", "--defaults", "--api", "http://forge.example.net:3000/api/v1")
	clone := filepath.Join(dir, "swarm-app")
	head := "✓ Maintainer jane: acme/swarm-app created\n✓ Maintainer jane: swarm-bot may write to acme/swarm-app\n✓ Cloned acme/swarm-app into " + clone + "\n✓ Token $GITEA_TOKEN reads acme/swarm-app as swarm-bot\n✓ Wrote swarm.json\n"
	if code != 0 || !strings.HasPrefix(out, head) || !strings.Contains(out, "\n✓ Token $GITEA_TOKEN: labels 16 present (16 created, 0 renamed)\n") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if g.repo["private"] != true || !slices.Equal(g.collaborators, []string{"swarm-bot write"}) || len(g.labels) != 16 {
		t.Errorf("forge: %v %q %d labels", g.repo, g.collaborators, len(g.labels))
	}
	// The swarm's login, then the maintainer's four, then the swarm's
	// token alone.
	jane := []string{"GET /api/v1/user", "GET /api/v1/repos/acme/swarm-app", "POST /api/v1/orgs/acme/repos", "PUT /api/v1/repos/acme/swarm-app/collaborators/swarm-bot"}
	if len(g.requests) < 5 || g.requests[0] != "GET /api/v1/user" || !slices.Equal(g.requests[1:5], jane) {
		t.Fatalf("requests %q", g.requests)
	}
	for i, r := range g.requests {
		if want := map[bool]string{true: "token jane-456", false: "token tok-123"}[i >= 1 && i < 5]; g.auths[i] != want {
			t.Errorf("%s under %s, want %s", r, g.auths[i], want)
		}
	}
	if got := m.gitIn(t, swarmUp, "log", "--format=%an %s", "main"); got != "Jane swarm: written by init" {
		t.Errorf("pushed: %q", got)
	}
	if slices.ContainsFunc(g.bases, func(b string) bool { return b != "http://forge.example.net:3000" }) {
		t.Errorf("requests at %q", g.bases)
	}

	g.requests, g.auths = nil, nil
	code, out, errs = runWith(m.swarmOps(filepath.Join(clone, "prompts"), g), "swarm", "init", "--defaults")
	if code != 0 || !strings.HasPrefix(out, "  Maintainer jane: acme/swarm-app exists\n✓ Maintainer jane: swarm-bot may write to acme/swarm-app\n✓ Token $GITEA_TOKEN reads acme/swarm-app as swarm-bot\n  Unchanged swarm.json\n") ||
		!strings.Contains(out, "\n  Nothing to commit\n") || slices.Contains(g.requests, "POST /api/v1/orgs/acme/repos") {
		t.Errorf("second run: exit %d\n%s\n%s\n%q", code, out, errs, g.requests)
	}
}

func TestSwarmInitRefusals(t *testing.T) {
	m := newMachine(t)
	g := newGitea(t)
	// In a folder holding a github product's clone: no adapter, nothing
	// written.
	dir := t.TempDir()
	m.gitIn(t, "", "init", "-q", filepath.Join(dir, "app"))
	m.gitIn(t, filepath.Join(dir, "app"), "remote", "add", "origin", "git@github.com:acme/app.git")
	m.vars["GITHUB_TOKEN"], m.vars["DAISY_MAINTAINER_TOKEN"] = "tok-123", "jane-456"
	code, _, errs := runWith(m.swarmOps(dir, g), "swarm", "init", "--defaults")
	if code != 1 || errs != "Error: swarm: forge: kind github has no adapter yet; it is built when a run needs it (docs/swarm.md section 10)\n" {
		t.Errorf("github: exit %d %q", code, errs)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("github: %d entries written", len(entries)-1)
	}
	// In a folder holding no clone, outside any workspace: no product.
	empty := t.TempDir()
	code, _, errs = runWith(m.swarmOps(empty, g), "swarm", "init", "--defaults")
	if entries, _ := os.ReadDir(empty); code != 1 || errs != "Error: swarm.json: repos: no repository (at least one is required)\n" || len(entries) != 0 {
		t.Errorf("an empty folder: exit %d %q", code, errs)
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults", "extra")
	if code != 2 || !strings.Contains(errs, "Usage: daisy swarm init [--defaults] [--api <url>]") {
		t.Errorf("usage: exit %d %q", code, errs)
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "init", "--defaults", "--api")
	if code != 2 || !strings.Contains(errs, "--api needs a value") {
		t.Errorf("--api without a value: exit %d %q", code, errs)
	}
}

// `swarm status` through the cli, under its ReadOnly, and the gitea adapter
// over an in-memory forge: GETs only, docker ps the one command beside
// git's, and the swarm's own lines.
func TestSwarmStatusThroughGitea(t *testing.T) {
	m := newMachine(t)
	g := newGitea(t)
	clone := m.swarm(t, g)
	g.requests = nil
	o := m.swarmOps(clone, g)
	var ran []string
	o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
		ran = append(ran, name+" "+args[0])
		if name == "docker" {
			return []byte("daisy-acme-swarm-app-4-worker-1 acme/swarm-app#4/worker openai/1\n"), nil, nil
		}
		return ops.Real().Exec(dir, name, args...)
	}
	code, out, errs := runWith(o, "swarm", "status")
	if code != 0 || errs != "" || !strings.HasPrefix(out, "Swarm: acme/swarm-app on forge.example.net (gitea), repositories app\n") ||
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
	code, _, errs = runWith(m.swarmOps(clone, g), "swarm", "status", "extra")
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
	g := newGitea(t)
	clone := m.swarm(t, g)
	g.issue1 = []string{"swarm:proposal-posted"}
	env := filepath.Join(m.local, "swarm", "forge.example.net", "acme", "swarm-app", "env")
	delete(m.vars, "DAISY_MAINTAINER_TOKEN") // <state>/env's alone
	if err := os.WriteFile(env, []byte("export GITEA_TOKEN=tok-123\nexport DAISY_MAINTAINER_TOKEN=jane-456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g.requests, g.auths = nil, nil
	code, out, errs := runWith(m.swarmOps(clone, g), "swarm", "triage", "1")
	if code != 0 || errs != "" || out != "Maintainer jane on acme/swarm-app\n#1 swarm:proposal-posted → swarm:proposal-accepted\n" || !slices.Equal(g.issue1, []string{"swarm:proposal-accepted"}) {
		t.Fatalf("exit %d\n%s\n%s\nlabels %q", code, out, errs, g.issue1)
	}
	if slices.ContainsFunc(g.auths, func(a string) bool { return a != "token jane-456" }) || !slices.Contains(g.requests, "DELETE /api/v1/repos/acme/swarm-app/issues/1/labels/1") {
		t.Errorf("requests %q under %q", g.requests, g.auths)
	}

	if err := os.WriteFile(env, []byte("export GITEA_TOKEN=tok-123\nexport DAISY_MAINTAINER_TOKEN=tok-123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g.requests = nil
	code, _, errs = runWith(m.swarmOps(clone, g), "swarm", "triage", "1")
	if code != 1 || !strings.HasPrefix(errs, "Error: swarm: $DAISY_MAINTAINER_TOKEN holds the swarm's token ($GITEA_TOKEN)") || len(g.requests) != 0 {
		t.Errorf("the swarm's token: exit %d %q; requests %q", code, errs, g.requests)
	}
	code, _, errs = runWith(m.swarmOps(clone, g), "swarm", "triage", "one")
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
// the cli, in a swarm repository's clone outside any workspace, pause and
// resume act on its <state>, and check-secrets reports the swarm's
// variables from <state>/env, refusing one others may read. A workspace
// holding the old layout's swarm.json in its swarm/ holds no swarm
// (revision 6's plan, Step 7): check-secrets reports the workspace's
// .env.sh alone, and a swarm verb there finds none.
func TestSwarmVerbsWithoutAWorkspace(t *testing.T) {
	m := newMachine(t)
	g := newGitea(t)
	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export GITEA_TOKEN=tok-123\n")
	writeFile(t, filepath.Join(m.ws, "swarm", "swarm.json"), "{}\n")
	code, out, errs := runWith(m.swarmOps(m.ws, g), "check-secrets")
	wantWS := "Checking Daisy environment configuration...\n\nCore paths:\n✅ DAISY_ROOT: " + string([]rune(m.root)[:8]) + "...\n⚠️  DAISY_HOME: Not configured\n\nAPI tokens:\n⚠️  Webex API: Not configured\n⚠️  Backstage: Not configured\n⚠️  JIRA API: Not configured\n⚠️  GitHub: Not configured\n" +
		"\nNote: Only showing first 8 characters of tokens for security\n"
	if code != 0 || out != wantWS || errs != "" {
		t.Errorf("check-secrets in the workspace: exit %d %q\n%s\nwant\n%s", code, errs, out, wantWS)
	}
	code, _, errs = runWith(m.swarmOps(m.ws, g), "swarm", "status")
	if code != 1 || errs != "Error: swarm.json: not found in "+m.ws+" or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'\n" {
		t.Errorf("status in the old layout's workspace: exit %d %q", code, errs)
	}

	clone := m.swarm(t, g)
	inside := filepath.Join(clone, "prompts")
	state := filepath.Join(m.local, "swarm", "forge.example.net", "acme", "swarm-app")
	code, out, errs = runWith(m.swarmOps(inside, g), "swarm", "pause")
	if code != 0 || out != "Paused: no new member will launch ("+filepath.Join(state, "paused")+"); running members finish and are posted\n" || errs != "" {
		t.Errorf("pause: exit %d %q %q", code, out, errs)
	}
	if code, out, errs = runWith(m.swarmOps(inside, g), "swarm", "resume"); code != 0 || out != "Resumed: members launch at the next pass\n" {
		t.Errorf("resume: exit %d %q %q", code, out, errs)
	}

	env := filepath.Join(state, "env")
	// A variable init found unset is its commented line, `# NAME=`: no
	// assignment, so the row is the environment's value, else not
	// configured (revision 6's plan, Step 6a).
	if err := os.WriteFile(env, []byte("export GITEA_TOKEN=\"tok-456\"\n# CLAUDE_CODE_OAUTH_TOKEN=\nexport DAISY_MAINTAINER_TOKEN=\"jane-456\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runWith(m.swarmOps(inside, g), "check-secrets")
	want := "Checking Daisy environment configuration...\n\nSwarm acme/swarm-app (" + env + "):\n✅ GITEA_TOKEN: tok-456...\n⚠️  CLAUDE_CODE_OAUTH_TOKEN: Not configured\n✅ DAISY_MAINTAINER_TOKEN: jane-456...\n\nNote: Only showing first 8 characters of tokens for security\n"
	if code != 0 || out != want || errs != "" {
		t.Errorf("check-secrets in the clone: exit %d %q\n%s\nwant\n%s", code, errs, out, want)
	}
	m.vars["CLAUDE_CODE_OAUTH_TOKEN"] = "oauth-env-123"
	code, out, _ = runWith(m.swarmOps(inside, g), "check-secrets")
	if code != 0 || !strings.Contains(out, "\n✅ CLAUDE_CODE_OAUTH_TOKEN: oauth-en...\n") {
		t.Errorf("check-secrets, the commented variable in the environment: exit %d\n%s", code, out)
	}
	delete(m.vars, "CLAUDE_CODE_OAUTH_TOKEN")
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
