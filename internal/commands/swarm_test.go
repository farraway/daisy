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
	"daisy/internal/swarm/reconcile"
)

// swarmWS is a swarm of the product app and its library lib, of the
// organization acme on forge.example.net, each with an upstream holding
// one commit on main that git reaches in place of the forge, and the
// token in its secrets file. init runs in <dir>, a folder holding the
// products' clones and no swarm repository's clone, with no .daisy/ in it or above it, the swarm's
// token and jane's, the maintainer's, in the environment; the forge holds
// no acme/swarm-app, whose upstream is empty, and init creates it and
// makes ws, <dir>/swarm-app, its clone, swarm.json and prompts/ at its
// top. Once init has run (cloned), the products' clones are removed: no
// product's clone is on the machine.
type swarmWS struct {
	t                 *testing.T
	ws, root, local   string
	dir               string            // the folder holding the clone
	upstreams         map[string]string // forge URL → local bare repository
	place             declare.Place     // where the swarm is
	cwd               string            // the folder the verbs find the swarm from (find): <dir> for the first init
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
	maintainer        forge.Account // the account jane-456 is, jane's when zero
	repoReadFails     bool          // the maintainer's read of the swarm repository refused
}

func newSwarmWS(t *testing.T) *swarmWS {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	s := &swarmWS{t: t, dir: t.TempDir(), root: t.TempDir(), local: t.TempDir(), vars: map[string]string{}, upstreams: map[string]string{}}
	s.ws = filepath.Join(s.dir, "swarm-app")
	// The installed root's praxis and testing skill, which the architect's
	// and the worker's system.md carry (section 7.3).
	for rel, text := range map[string]string{reconcile.PraxisFile: rootPraxis, reconcile.TestingFile: rootTesting} {
		os.MkdirAll(filepath.Dir(filepath.Join(s.root, rel)), 0o755)
		os.WriteFile(filepath.Join(s.root, rel), []byte(text), 0o644)
	}
	s.forge = forge.NewFake(fixedClock(), "acme/swarm-app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot", Email: "swarm@example.net"})
	for name, url := range map[string]string{"app": "git@forge.example.net:acme/app.git", "lib": "git@forge.example.net:acme/lib.git"} {
		dir := filepath.Join(s.dir, name)
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
	// The swarm repository: on the forge once init creates it, its
	// upstream empty, its default branch main. The maintainer's git
	// identity commits in its clone.
	up := filepath.Join(t.TempDir(), "swarm-app.git")
	git(t, "", "init", "-q", "--bare", "-b", "main", up)
	s.upstreams[swarmURL] = up
	s.forge.SetSSHURL(swarmURL)
	global := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(global, []byte("[user]\n\tname = Jane\n\temail = jane@example.net\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	s.place, s.cwd = declare.Clone(s.ws), s.dir
	s.vars["GITEA_TOKEN"], s.vars["DAISY_MAINTAINER_TOKEN"] = "tok-123", "jane-456"
	return s
}

// publish commits everything in the swarm repository's clone and
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

// fetchMirror brings the swarm repository's mirror up to date with its
// upstream, as a pass's fetch does.
func (s *swarmWS) fetchMirror() {
	s.t.Helper()
	m := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "git", "swarm-app")
	if _, errb, err := s.exec(m, "git", "fetch", "-q", "--prune", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		s.t.Fatalf("git fetch: %v %s", err, errb)
	}
}

// swarmURL is the swarm repository's clone's origin.
const swarmURL = "git@forge.example.net:acme/swarm-app.git"

// cloned is the swarm once init has written it: the verbs find it from
// prompts/, a folder inside it, with no product's clone left on the
// machine (revision 6's plan, Step 3's claim).
func (s *swarmWS) cloned() {
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

// secretsFile is where the swarm's secrets are, <state>/env.
func (s *swarmWS) secretsFile() string {
	return filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app", "env")
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

// cwdOps is o with its working folder at cwd.
type cwdOps struct {
	ops.Ops
	cwd string
}

func (o cwdOps) Getwd() (string, error) { return o.cwd, nil }

// at runs verb on c as the cli does once the swarm is found: from s.cwd
// through find, the one lookup.
func (s *swarmWS) at(c *Context, init bool, verb func(*Context) error) error {
	c.Env.Root, c.Env.Local = s.root, s.local
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
		return nil, errors.New("forge: GET /repos/acme/swarm-app/issues?…: 404 Not Found")
	}
	return nil, nil
}

func (f failingReads) Pulls(int, []string) ([]forge.Pull, error) {
	if f.pulls {
		return nil, errors.New("forge: GET /repos/acme/lib/pulls?…: 404 Not Found")
	}
	return nil, nil
}

// failingRepo is the Fake under the maintainer's token with its read of
// the swarm repository refused, as a token without access finds it.
type failingRepo struct {
	*forge.Fake
	fails bool
}

func (f failingRepo) Repository() (forge.Repository, error) {
	if f.fails {
		return forge.Repository{}, errors.New("forge: GET /repos/acme/swarm-app: 403 forbidden")
	}
	return f.Fake.Repository()
}

// run is `swarm init` with flags, over the Fake forge; `--api` takes the
// next word as its value, as the cli does.
func (s *swarmWS) run(args ...string) error {
	s.stdout.Reset()
	s.stderr.Reset()
	answers := s.answers
	var flags []string
	var values []Value
	for i := 0; i < len(args); i++ {
		if args[i] == "--api" && i+1 < len(args) {
			values = append(values, Value{Flag: args[i], Value: args[i+1]})
			i++
			continue
		}
		flags = append(flags, args[i])
	}
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
	c := &Context{Ops: o, Stdin: strings.NewReader(""), Stdout: &s.stdout, Stderr: &s.stderr, Flags: flags, Values: values}
	return s.at(c, true, func(c *Context) error {
		return swarmInit(c, func(_ ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
			if token == "jane-456" {
				return failingRepo{s.forge.As(cmp.Or(s.maintainer, forge.Account{Login: "jane", Email: "jane@example.net"})), s.repoReadFails}, nil
			}
			s.opened = []string{f.Kind, f.API, f.Repo, strings.Join(names, ","), token}
			return failingReads{s.forge, s.readFails, s.declaredReadFails, s.loginFails}, nil
		})
	})
}

const units = `
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: apply every 5min for the swarm repository %f

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

// init --defaults end to end in the clone (section 11, init): the four
// readiness warnings, what it wrote valid and the prompts section 4's; a
// second run, from the clone, keeps a prompt the maintainer changed and a
// swarm.json of the same values in the maintainer's own layout.
func TestSwarmInitDefaults(t *testing.T) {
	s := newSwarmWS(t)
	s.vars["CLAUDE_CODE_OAUTH_TOKEN"] = "oauth-token"
	if err := s.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, s.stderr.String())
	}
	if s.opened[0] != "gitea" || s.opened[1] != "https://forge.example.net/api/v1" || s.opened[2] != "acme/swarm-app" || s.opened[3] != "app,lib" || s.opened[4] != "tok-123" {
		t.Errorf("opened %q", s.opened)
	}
	wantErr := "⚠ Readiness: app has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: app: check is empty; no member's work is executed (set it in swarm.json)\n" +
		"⚠ Readiness: lib has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n" +
		"⚠ Readiness: lib: check is empty; no member's work is executed (set it in swarm.json)\n" +
		"⚠ Readiness: trusted holds no maintainer login; no comment of yours reaches a member\n"
	if s.stderr.String() != wantErr {
		t.Errorf("stderr:\n%s\nwant:\n%s", s.stderr.String(), wantErr)
	}
	c, err := declare.Parse([]byte(s.read("swarm.json")))
	if err != nil || len(c.Repos) != 2 || c.Forge.CommitEmail != "swarm@example.net" || strings.Join(c.Trusted, ",") != "swarm-bot" {
		t.Errorf("swarm.json %v:\n%s", err, s.read("swarm.json"))
	}
	if err := declare.Validate(ops.Real(), s.place, c, 0); err != nil {
		t.Errorf("what init wrote does not validate: %v", err)
	}
	for _, r := range declare.Roles {
		if s.read("prompts/"+r+".md") != declare.DefaultPrompts[r] {
			t.Errorf("%s's prompt", r)
		}
	}

	// Again, from the clone: the maintainer's prompt and layout kept.
	s.cwd = s.ws
	before := s.read("swarm.json")
	s.write("prompts/worker.md", "# My worker\n")
	relaid := strings.Replace(before, `"max_members": 3,`, `"max_members":    3,`, 1)
	s.write("swarm.json", relaid)
	if err := s.run("--defaults"); err != nil {
		t.Fatal(err)
	}
	if out := s.stdout.String(); !strings.Contains(out, "  Unchanged swarm.json\n") || !strings.Contains(out, "  Kept prompts/worker.md (exists)\n") ||
		s.read("swarm.json") != relaid || s.read("prompts/worker.md") != "# My worker\n" {
		t.Errorf("second run:\n%s", out)
	}
}

// With the products' instructions pushed, a check, the resources and a
// maintainer's login, init warns about nothing.
func TestSwarmInitReadyWarnsNothing(t *testing.T) {
	s := newSwarmWS(t)
	s.vars["CLAUDE_CODE_OAUTH_TOKEN"] = "oauth-token"
	editUp(t, s.upstreams["git@forge.example.net:acme/app.git"], "main", map[string]string{"AGENTS.md": "# App\n"})
	editUp(t, s.upstreams["git@forge.example.net:acme/lib.git"], "main", map[string]string{"CLAUDE.md": "# Lib\n"})
	s.answers = make([]string, 33)
	s.answers = append(s.answers, "jane", "", "")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	s.cwd = s.ws
	f := s.read("swarm.json")
	f = strings.Replace(f, `"check": []`, `"check": ["make", "check"]`, 2)
	s.write("swarm.json", f)
	s.answers = make([]string, 36)
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.stderr.String(), "⚠") {
		t.Errorf("warned:\n%s", s.stderr.String())
	}
	if !strings.Contains(s.stdout.String(), "  Unchanged swarm.json\n") {
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
	if s.read("swarm.json") == "" || s.read("prompts/owner.md") == "" {
		t.Error("the files were not written")
	}
	if strings.Contains(s.stdout.String(), "labels") || strings.Contains(s.stdout.String(), "systemd") {
		t.Errorf("went on past the failure:\n%s", s.stdout.String())
	}
	s.cwd = s.ws
	s.forge.AddLabels(forge.Labels...)
	if err := s.run("--defaults"); err != nil || !strings.Contains(s.stdout.String(), "✓ Token $GITEA_TOKEN: labels 16 present (0 created, 0 renamed)\n") {
		t.Errorf("re-run: %v\n%s", err, s.stdout.String())
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Every failure before the files writes none of them, and makes no label.
func TestSwarmInitWritesNothingBeforeTheForgeAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(s *swarmWS)
		want string
	}{
		{"token unset", func(s *swarmWS) { delete(s.vars, "GITEA_TOKEN") }, "Error: swarm: $GITEA_TOKEN is not set (in <state>/env or the environment)"},
		{"swarm repository unreadable", func(s *swarmWS) { s.readFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read acme/swarm-app: forge: GET /repos/acme/swarm-app/issues?…: 404 Not Found"},
		{"a declared repository unreadable", func(s *swarmWS) { s.declaredReadFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read the declared repositories: forge: GET /repos/acme/lib/pulls?…: 404 Not Found"},
		{"its own login not a forge login", func(s *swarmWS) {
			s.forge = forge.NewFake(fixedClock(), "acme/swarm-app", []string{"app", "lib"}, forge.Account{Login: "swarm bot", Email: "swarm@example.net"})
			s.forge.SetSSHURL(swarmURL)
		},
			`Error: swarm.json: trusted[0]: "swarm bot" is not a forge login`},
		{"its own account unreadable", func(s *swarmWS) { s.loginFails = true },
			"Error: swarm: $GITEA_TOKEN cannot read its own account: forge: GET /user: 401 invalid username, password or token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSwarmWS(t)
			tc.edit(s)
			err := s.run("--defaults")
			state := filepath.Join(s.local, "swarm", "forge.example.net", "acme", "swarm-app")
			if got := strings.ReplaceAll(errText(err), state, "<state>"); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
			if _, err := os.Stat(filepath.Join(s.ws, "swarm.json")); err == nil {
				t.Error("swarm.json was written")
			}
			if len(s.forge.Labels()) != 0 {
				t.Errorf("labels created: %q", s.forge.Labels())
			}
		})
	}
	// A line <state>/env cannot be read past is named, not skipped
	// silently.
	s := newSwarmWS(t)
	delete(s.vars, "GITEA_TOKEN")
	s.secrets("export A=$(date)\nexport GITEA_TOKEN=tok\n")
	if err := s.run("--defaults"); !strings.HasPrefix(s.stderr.String(), "⚠️  "+s.secretsFile()+" line 1: ") || !strings.Contains(errText(err), "$GITEA_TOKEN is not set") {
		t.Errorf("<state>/env problem: %v %q", err, s.stderr.String())
	}

	// An invalid value in the file is refused as a default: nothing more
	// is written.
	s = newSwarmWS(t)
	if err := s.run("--defaults"); err != nil {
		t.Fatal(err)
	}
	s.cwd = s.ws
	broken := strings.Replace(s.read("swarm.json"), `"max_members": 3`, `"max_members": 0`, 1)
	s.write("swarm.json", broken)
	os.Remove(filepath.Join(s.ws, "prompts", "owner.md"))
	if err := s.run("--defaults"); errText(err) != "Error: swarm.json: max_members: 0 is not between 1 and 16" ||
		s.read("swarm.json") != broken || s.read("prompts/owner.md") != "" {
		t.Errorf("an invalid default: %v", err)
	}

	// The token from the environment when <state>/env does not assign it;
	// <state>/env's assignment before the environment's.
	s = newSwarmWS(t)
	s.secrets("")
	s.vars["GITEA_TOKEN"] = "tok-env"
	if err := s.run("--defaults"); err != nil || s.opened[4] != "tok-env" {
		t.Errorf("token from the environment: %v %q", err, s.opened)
	}
	s = newSwarmWS(t)
	s.secrets("export GITEA_TOKEN=\"tok-123\"\n")
	s.vars["GITEA_TOKEN"] = "tok-env"
	if err := s.run("--defaults"); err != nil || s.opened[4] != "tok-123" {
		t.Errorf("<state>/env first: %v %q", err, s.opened)
	}
}

// Question 33 refuses the swarm's login and asks again, and question 35's
// yes declares the notes; EOF cancels and writes nothing.
func TestSwarmInitMaintainerQuestion(t *testing.T) {
	s := newSwarmWS(t)
	s.answers = append(make([]string, 33), "swarm-bot", "jane", "", "yes")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.stderr.String(), `swarm.json: trusted: "swarm-bot" is the token's own login; the swarm needs its own forge account (section 8)`+"\nMaintainer logins") {
		t.Errorf("stderr:\n%s", s.stderr.String())
	}
	if !strings.Contains(s.read("swarm.json"), `"notes": true,`+"\n"+`  "trusted": ["jane", "swarm-bot"]`) {
		t.Errorf("swarm.json:\n%s", s.read("swarm.json"))
	}

	s = newSwarmWS(t)
	s.answers = make([]string, 33) // EOF at question 33
	if err := s.run(); err != nil || !strings.HasSuffix(s.stderr.String(), "Cancelled — no answer on stdin (EOF); nothing written.\n") {
		t.Errorf("EOF: %v %q", err, s.stderr.String())
	}
	if _, err := os.Stat(s.ws); err == nil || len(s.forge.Labels()) != 0 {
		t.Error("written after EOF")
	}
	if _, created := s.forge.Held(); created {
		t.Error("the repository created after EOF")
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
