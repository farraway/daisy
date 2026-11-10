package commands

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// swarmWS is a swarm of the product app and its library lib, of the
// organization acme on forge.example.net, each with an upstream holding
// one commit on main that git reaches in place of the forge, and the
// token in its secrets file. In the old layout ws is a Daisy workspace
// (.daisy/) holding the products' clones at app and lib, the swarm in its
// swarm/ and the token in its .env.sh; in revision 6's (clone) ws is
// <dir>/swarm-app, a clone of the swarm repository acme/swarm-app with no
// .daisy/ in it or above it, swarm.json and prompts/ at its top, the
// token in <state>/env, and the products' clones beside it in <dir> until
// init has run, then removed: no product's clone is on the machine.
type swarmWS struct {
	t                 *testing.T
	ws, root, local   string
	dir               string // the folder holding the clone (clone)
	clone             bool
	upstreams         map[string]string // forge URL → local bare repository
	place             declare.Place     // where the swarm is
	cwd               string            // the folder the verbs find the swarm from (find); "" before a clone holds its swarm.json, the verbs then run at place
	vars              map[string]string
	forge             *forge.Fake
	opened            []string // what the last opener was given: kind, api, repo, names, token
	tokens            []string // every token status's and apply's openers were given
	asked             []string // every variable status and apply asked the environment for
	stdout, stderr    bytes.Buffer
	answers           []string
	readFails         bool
	declaredReadFails bool
	loginFails        bool
}

func newSwarmWS(t *testing.T) *swarmWS { return newSwarmWSIn(t, false) }

// newSwarmWSIn is newSwarmWS in the old layout, or with clone in a swarm
// repository's clone.
func newSwarmWSIn(t *testing.T, clone bool) *swarmWS {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	s := &swarmWS{t: t, ws: t.TempDir(), root: t.TempDir(), local: t.TempDir(), clone: clone, vars: map[string]string{}, upstreams: map[string]string{}}
	issues := "acme/app"
	if clone {
		s.dir, issues = s.ws, "acme/swarm-app"
		s.ws = filepath.Join(s.dir, "swarm-app")
	}
	s.forge = forge.NewFake(fixedClock(), issues, []string{"app", "lib"}, forge.Account{Login: "swarm-bot", Email: "swarm@example.net"})
	for name, url := range map[string]string{"app": "git@forge.example.net:acme/app.git", "lib": "git@forge.example.net:acme/lib.git"} {
		dir := filepath.Join(cmp.Or(s.dir, s.ws), name)
		os.MkdirAll(dir, 0o755)
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", url}} {
			if _, errb, err := ops.Real().Exec(dir, "git", args...); err != nil {
				t.Fatalf("git %v: %v %s", args, err, errb)
			}
		}
		up := filepath.Join(t.TempDir(), name+".git")
		src := t.TempDir()
		for _, args := range [][]string{{"init", "-q", "-b", "main", src}, {"-C", src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first"},
			{"clone", "-q", "--bare", src, up}} {
			if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
				t.Fatalf("git %v: %v %s", args, err, errb)
			}
		}
		s.upstreams[url] = up
	}
	if clone {
		// The swarm repository: an upstream holding one commit on main,
		// its default branch, and the clone made from it.
		up := filepath.Join(t.TempDir(), "swarm-app.git")
		src := t.TempDir()
		git(t, "", "init", "-q", "-b", "main", src)
		git(t, src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
		git(t, "", "clone", "-q", "--bare", src, up)
		s.upstreams[swarmURL] = up
		git(t, "", "clone", "-q", up, s.ws)
		git(t, s.ws, "remote", "set-url", "origin", swarmURL)
		s.place = declare.Clone(s.ws)
	} else {
		os.Mkdir(filepath.Join(s.ws, ".daisy"), 0o755)
		s.place, s.cwd = declare.Workspace(s.ws), s.ws
	}
	s.secrets("export GITEA_TOKEN=\"tok-123\"\n")
	return s
}

// publish commits everything in the swarm repository's clone (clone) and
// pushes it to the swarm repository's default branch, as the maintainer
// does: what the members read is what is pushed (section 2).
func (s *swarmWS) publish(msg string) {
	s.t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", msg}, {"push", "-q", "origin", "HEAD:main"}} {
		if _, errb, err := s.exec(s.ws, "git", args...); err != nil {
			s.t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
}

// swarmURL is the swarm repository's clone's origin (clone).
const swarmURL = "git@forge.example.net:acme/swarm-app.git"

// cloned is the clone once init has written swarm.json at its top and
// made the mirrors: the verbs finding it from prompts/, a folder inside
// it, and no product's clone left on the machine (revision 6's plan,
// Step 3's claim).
func (s *swarmWS) cloned() {
	if !s.clone {
		return
	}
	s.cwd = filepath.Join(s.ws, "prompts")
	for _, name := range []string{"app", "lib"} {
		os.RemoveAll(filepath.Join(s.dir, name))
	}
}

// exec runs name for real, git reaching each upstream in place of the
// forge URL it stands for (url.<up>.insteadOf), so that the URLs the
// mirrors' configs record are the forge's.
func (s *swarmWS) exec(dir, name string, args ...string) ([]byte, []byte, error) {
	if name == "git" {
		var pre []string
		for url, up := range s.upstreams {
			pre = append(pre, "-c", "url."+up+".insteadOf="+url)
		}
		args = append(pre, args...)
	}
	return ops.Real().Exec(dir, name, args...)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, errb, err := ops.Real().Exec(dir, "git", args...); err != nil {
		t.Fatalf("git %v: %v %s", args, err, errb)
	}
}

// secretsFile is where the swarm's secrets are: the workspace's .env.sh,
// or <state>/env.
func (s *swarmWS) secretsFile() string {
	if s.clone {
		return filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "env")
	}
	return filepath.Join(s.ws, ".env.sh")
}

// secrets writes the secrets file, <state>/env with mode 0600.
func (s *swarmWS) secrets(content string) {
	s.t.Helper()
	p := s.secretsFile()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		s.t.Fatal(err)
	}
}

// cfg is a file of the configuration folder, relative to the root:
// swarm/<rel> in the old layout, <rel> in a clone.
func (s *swarmWS) cfg(rel string) string { return s.place.Rel(rel) }

// cwdOps is o with its working folder at cwd.
type cwdOps struct {
	ops.Ops
	cwd string
}

func (o cwdOps) Getwd() (string, error) { return o.cwd, nil }

// at runs verb on c as the cli does once the swarm is found: from s.cwd
// through find, the one lookup, or at s.place before a clone holds its
// swarm.json.
func (s *swarmWS) at(c *Context, init bool, verb func(*Context) error) error {
	c.Env.Root, c.Env.Local = s.root, s.local
	if s.cwd == "" {
		c.Swarm = s.place
		return verb(c)
	}
	c.Ops = cwdOps{c.Ops, s.cwd}
	if err := find(c, init); err != nil {
		return err
	}
	return verb(c)
}

// fixedClock is the Ops whose clock reads 2026-11-06 15:00 UTC.
func fixedClock() ops.Ops {
	return &ops.Fake{NowFn: func() time.Time { return time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC) }}
}

func (s *swarmWS) write(rel, content string) {
	s.t.Helper()
	p := filepath.Join(s.ws, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func (s *swarmWS) read(rel string) string {
	b, _ := os.ReadFile(filepath.Join(s.ws, rel))
	return string(b)
}

// failingReads is the Fake with its reads refused, as a token without
// access to the repositories finds them.
type failingReads struct {
	*forge.Fake
	issues, pulls, login bool
}

func (f failingReads) Login() (forge.Account, error) {
	if f.login {
		return forge.Account{}, errors.New("forge: GET /user: 401 invalid username, password or token")
	}
	return f.Fake.Login()
}

func (f failingReads) Issues(forge.State, string) ([]forge.Issue, error) {
	if f.issues {
		return nil, errors.New("forge: GET /repos/acme/app/issues?…: 404 Not Found")
	}
	return nil, nil
}

func (f failingReads) Pulls(int, []string) ([]forge.Pull, error) {
	if f.pulls {
		return nil, errors.New("forge: GET /repos/acme/lib/pulls?…: 404 Not Found")
	}
	return nil, nil
}

// run is `swarm init` with flags, over the Fake forge.
func (s *swarmWS) run(flags ...string) error {
	s.stdout.Reset()
	s.stderr.Reset()
	answers := s.answers
	o := &ops.Fake{
		GetenvFn:     func(k string) string { return s.vars[k] },
		ExecFn:       s.exec,
		ExecutableFn: func() (string, error) { return "/opt/daisy/bin/daisy", nil },
		PromptFn: func(_ io.Reader, w io.Writer, q string) (string, error) {
			io.WriteString(w, q)
			if len(answers) == 0 {
				return "", io.EOF
			}
			a := answers[0]
			answers = answers[1:]
			return a, nil
		},
	}
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &s.stdout, Stderr: &s.stderr, Flags: flags}
	return s.at(c, true, func(c *Context) error {
		return swarmInit(c, func(_ ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
			s.opened = []string{f.Kind, f.API, f.Repo, strings.Join(names, ","), token}
			return failingReads{s.forge, s.readFails, s.declaredReadFails, s.loginFails}, nil
		})
	})
}

const units = `
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: apply every 5min for the workspace %f

[Service]
WorkingDirectory=%f
Environment=DAISY_ROOT=<root>
ExecStart=/opt/daisy/bin/daisy swarm apply
Restart=on-failure
RestartSec=5min
KillMode=mixed
TimeoutStopSec=45min

[Install]
WantedBy=default.target

Install (your step):
  systemctl --user daemon-reload && systemctl --user enable --now <unit>
`

func (s *swarmWS) units() string {
	return strings.NewReplacer("<root>", s.root, "<unit>", installedUnit(s.ws)).Replace(units)
}

// init --defaults end to end: the files, the labels, the four readiness
// warnings, the unit files; a second run changes nothing.
func TestSwarmInitDefaults(t *testing.T) {
	s := newSwarmWS(t)
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if s.opened[0] != "gitea" || s.opened[1] != "https://forge.example.net/api/v1" || s.opened[2] != "acme/app" || s.opened[3] != "app,lib" || s.opened[4] != "tok-123" {
		t.Errorf("opened %q", s.opened)
	}
	wantOut := "✓ Token $GITEA_TOKEN reads acme/app as swarm-bot\n✓ Wrote swarm/swarm.json\n"
	for _, r := range declare.Roles {
		wantOut += "✓ Wrote swarm/prompts/" + r + ".md\n"
	}
	wantOut += "✓ Labels: 16 present (16 created, 0 renamed)\n" + s.units()
	if s.stdout.String() != wantOut {
		t.Errorf("stdout:\n%s\nwant:\n%s", s.stdout.String(), wantOut)
	}
	wantErr := "⚠ Readiness: app has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: app: check is empty; no member's work is executed (set it in swarm/swarm.json)\n" +
		"⚠ Readiness: lib has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: lib: check is empty; no member's work is executed (set it in swarm/swarm.json)\n" +
		"⚠ Readiness: resources is empty; the owner proposes from the code alone\n" +
		"⚠ Readiness: trusted holds no maintainer login; no comment of yours reaches a member\n"
	if s.stderr.String() != wantErr {
		t.Errorf("stderr:\n%s\nwant:\n%s", s.stderr.String(), wantErr)
	}
	c, err := declare.Parse([]byte(s.read("swarm/swarm.json")))
	if err != nil || len(c.Repos) != 2 || c.Forge.CommitEmail != "swarm@example.net" || strings.Join(c.Trusted, ",") != "swarm-bot" {
		t.Errorf("swarm.json %v:\n%s", err, s.read("swarm/swarm.json"))
	}
	if err := declare.Validate(ops.Real(), s.place, c, 0); err != nil {
		t.Errorf("what init wrote does not validate: %v", err)
	}
	for _, r := range declare.Roles {
		if s.read("swarm/prompts/"+r+".md") != declare.DefaultPrompts[r] {
			t.Errorf("%s's prompt", r)
		}
	}
	if strings.Join(s.forge.Labels(), ",") != strings.Join(forge.Labels, ",") {
		t.Errorf("labels %q", s.forge.Labels())
	}

	// Twice: nothing written, nothing created.
	before := s.read("swarm/swarm.json")
	s.write("swarm/prompts/worker.md", "# My worker\n")
	if err := s.run("--defaults"); err != nil {
		t.Fatal(err)
	}
	wantOut = "✓ Token $GITEA_TOKEN reads acme/app as swarm-bot\n  Unchanged swarm/swarm.json\n"
	for _, r := range declare.Roles {
		wantOut += "  Kept swarm/prompts/" + r + ".md (exists)\n"
	}
	wantOut += "✓ Labels: 16 present (0 created, 0 renamed)\n" + s.units()
	if s.stdout.String() != wantOut || s.read("swarm/swarm.json") != before || s.read("swarm/prompts/worker.md") != "# My worker\n" {
		t.Errorf("second run:\n%s", s.stdout.String())
	}
	if len(s.forge.Labels()) != 16 {
		t.Errorf("labels %q", s.forge.Labels())
	}

	// The same values in the maintainer's own layout: left as they are.
	relaid := strings.Replace(before, `"max_members": 3,`, `"max_members":    3,`, 1)
	s.write("swarm/swarm.json", relaid)
	if err := s.run("--defaults"); err != nil || !strings.Contains(s.stdout.String(), "  Unchanged swarm/swarm.json\n") || s.read("swarm/swarm.json") != relaid {
		t.Errorf("a relaid file: %v\n%s", err, s.stdout.String())
	}
}

// With the product's instructions, a check, resources and a maintainer's
// login, init warns about nothing.
func TestSwarmInitReadyWarnsNothing(t *testing.T) {
	s := newSwarmWS(t)
	s.write("app/AGENTS.md", "# App\n")
	s.write("lib/CLAUDE.md", "# Lib\n")
	s.write("docs/roadmap.md", "# Roadmap\n")
	s.answers = make([]string, 33)
	s.answers = append(s.answers, "jane", "", "")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	f := s.read("swarm/swarm.json")
	f = strings.Replace(f, `"check": []`, `"check": ["make", "check"]`, 2)
	s.write("swarm/swarm.json", f)
	s.answers = make([]string, 36)
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.stderr.String(), "⚠") {
		t.Errorf("warned:\n%s", s.stderr.String())
	}
	if !strings.Contains(s.stdout.String(), "  Unchanged swarm/swarm.json\n") {
		t.Errorf("the file's values are not the defaults:\n%s", s.stdout.String())
	}
}

// A refused label create: exit 1 with the missing names, the files
// written; once the owner creates them, a re-run finds all sixteen.
func TestSwarmInitRefusedLabels(t *testing.T) {
	s := newSwarmWS(t)
	s.forge.AddLabels("swarm:proposal-posted", "swarm:proposal-accepted")
	s.forge.RefuseLabels()
	err := s.run("--defaults")
	want := "Error: forge: EnsureLabels: acme's labels missing: swarm:plan-0-posted, swarm:plan-1-accepted, swarm:plan-2-in-progress, swarm:review-0-ready, swarm:review-1-in-progress, swarm:review-2-approved, swarm:review-3-changes-requested, swarm:gated, swarm:blocked, swarm:paused, swarm:blocking, swarm:class-architecture, swarm:done, swarm:rejected; an owner of acme makes them once"
	if errText(err) != want {
		t.Errorf("got  %v\nwant %s", err, want)
	}
	if s.read("swarm/swarm.json") == "" || s.read("swarm/prompts/owner.md") == "" {
		t.Error("the files were not written")
	}
	if strings.Contains(s.stdout.String(), "Labels:") || strings.Contains(s.stdout.String(), "systemd") {
		t.Errorf("went on past the failure:\n%s", s.stdout.String())
	}
	s.forge.AddLabels(forge.Labels...)
	if err := s.run("--defaults"); err != nil || !strings.Contains(s.stdout.String(), "✓ Labels: 16 present (0 created, 0 renamed)\n") {
		t.Errorf("re-run: %v\n%s", err, s.stdout.String())
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Every failure before the files writes nothing.
func TestSwarmInitWritesNothingBeforeTheForgeAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(s *swarmWS)
		want string
	}{
		{"token unset", func(s *swarmWS) { s.write(".env.sh", "") }, "Error: swarm: $GITEA_TOKEN is not set (in <ws>/.env.sh or the environment)"},
		{"issues repository unreadable", func(s *swarmWS) { s.readFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read acme/app: forge: GET /repos/acme/app/issues?…: 404 Not Found"},
		{"a declared repository unreadable", func(s *swarmWS) { s.declaredReadFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read the declared repositories: forge: GET /repos/acme/lib/pulls?…: 404 Not Found"},
		{"its own login not a forge login", func(s *swarmWS) {
			s.forge = forge.NewFake(fixedClock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm bot", Email: "swarm@example.net"})
		},
			`Error: swarm.json: trusted[0]: "swarm bot" is not a forge login`},
		{"its own account unreadable", func(s *swarmWS) { s.loginFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read its own account: forge: GET /user: 401 invalid username, password or token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSwarmWS(t)
			tc.edit(s)
			err := s.run("--defaults")
			if got := strings.ReplaceAll(errText(err), s.ws, "<ws>"); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
			if _, err := os.Stat(filepath.Join(s.ws, "swarm")); err == nil {
				t.Error("swarm/ was written")
			}
			if len(s.forge.Labels()) != 0 {
				t.Errorf("labels created: %q", s.forge.Labels())
			}
		})
	}
	// A line .env.sh cannot be read past is named, not skipped silently.
	s := newSwarmWS(t)
	s.write(".env.sh", "export A=$(date)\nexport GITEA_TOKEN=tok\n")
	if err := s.run("--defaults"); !strings.HasPrefix(s.stderr.String(), "⚠️  .env.sh line 1: ") || !strings.Contains(errText(err), "$GITEA_TOKEN is not set") {
		t.Errorf(".env.sh problem: %v %q", err, s.stderr.String())
	}

	// An invalid value in the file is refused as a default: nothing more
	// is written.
	s = newSwarmWS(t)
	if err := s.run("--defaults"); err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(s.read("swarm/swarm.json"), `"max_members": 3`, `"max_members": 0`, 1)
	s.write("swarm/swarm.json", broken)
	os.Remove(filepath.Join(s.ws, "swarm", "prompts", "owner.md"))
	if err := s.run("--defaults"); errText(err) != "Error: swarm.json: max_members: 0 is not between 1 and 16" ||
		s.read("swarm/swarm.json") != broken || s.read("swarm/prompts/owner.md") != "" {
		t.Errorf("an invalid default: %v", err)
	}

	// The token from the environment when .env.sh does not assign it.
	s = newSwarmWS(t)
	s.write(".env.sh", "")
	s.vars["GITEA_TOKEN"] = "tok-env"
	if err := s.run("--defaults"); err != nil || s.opened[4] != "tok-env" {
		t.Errorf("token from the environment: %v %q", err, s.opened)
	}
	// .env.sh's assignment before the environment's.
	s = newSwarmWS(t)
	s.vars["GITEA_TOKEN"] = "tok-env"
	if err := s.run("--defaults"); err != nil || s.opened[4] != "tok-123" {
		t.Errorf(".env.sh first: %v %q", err, s.opened)
	}
}

// Question 33 refuses the swarm's login and asks again, and question 35's
// yes declares the wiki; EOF cancels and writes nothing.
func TestSwarmInitMaintainerQuestion(t *testing.T) {
	s := newSwarmWS(t)
	s.answers = append(make([]string, 33), "swarm-bot", "jane", "", "yes")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.stderr.String(), `swarm.json: trusted: "swarm-bot" is the token's own login; the swarm needs its own forge account (section 8)`+"\nMaintainer logins") {
		t.Errorf("stderr:\n%s", s.stderr.String())
	}
	if !strings.Contains(s.read("swarm/swarm.json"), `"wiki": true,`+"\n"+`  "trusted": ["jane", "swarm-bot"]`) {
		t.Errorf("swarm.json:\n%s", s.read("swarm/swarm.json"))
	}

	s = newSwarmWS(t)
	s.answers = make([]string, 33) // EOF at question 33
	if err := s.run(); err != nil || !strings.HasSuffix(s.stderr.String(), "Cancelled — no answer on stdin (EOF); nothing written.\n") {
		t.Errorf("EOF: %v %q", err, s.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(s.ws, "swarm")); err == nil || len(s.forge.Labels()) != 0 {
		t.Error("written after EOF")
	}
}

func TestSystemdNotation(t *testing.T) {
	for in, want := range map[string]string{"/home/jane/ws": "home-jane-ws", "/srv/my-ws/a.b": `srv-my\x2dws-a.b`, "/.hidden/x y": `\x2ehidden-x\x20y`, "/": "-"} {
		if got := systemdEscapePath(in); got != want {
			t.Errorf("systemdEscapePath(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"/home/jane/ws": "daisy-swarm@home-jane-ws.service", "/srv/my-ws": `'daisy-swarm@srv-my\x2dws.service'`} {
		if got := installedUnit(in); got != want {
			t.Errorf("installedUnit(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"5m": "5min", "1h30m": "1h30min", "90s": "1min30s", "24h": "24h"} {
		d, _ := time.ParseDuration(in)
		if got := systemdSpan(d); got != want {
			t.Errorf("systemdSpan(%s) = %q, want %q", in, got, want)
		}
	}
}

// bothLayouts runs test in the old layout and in a swarm repository's
// clone (revision 6), until Step 7 of its plan retires the old.
func bothLayouts(t *testing.T, test func(t *testing.T, clone bool)) {
	t.Run("workspace", func(t *testing.T) { test(t, false) })
	t.Run("clone", func(t *testing.T) { test(t, true) })
}
