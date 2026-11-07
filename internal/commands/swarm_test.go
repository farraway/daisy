package commands

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// swarmWS is a temp workspace holding the product at app and its library
// at lib, of the organization acme on forge.example.net, and the token in
// .env.sh.
type swarmWS struct {
	t                 *testing.T
	ws, root          string
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

func newSwarmWS(t *testing.T) *swarmWS {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	s := &swarmWS{t: t, ws: t.TempDir(), root: t.TempDir(), vars: map[string]string{}}
	s.forge = forge.NewFake(fixedClock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot", Email: "swarm@example.net"})
	for name, url := range map[string]string{"app": "git@forge.example.net:acme/app.git", "lib": "git@forge.example.net:acme/lib.git"} {
		dir := filepath.Join(s.ws, name)
		os.MkdirAll(dir, 0o755)
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", url}} {
			if _, errb, err := ops.Real().Exec(dir, "git", args...); err != nil {
				t.Fatalf("git %v: %v %s", args, err, errb)
			}
		}
	}
	s.write(".env.sh", "export GITEA_TOKEN=\"tok-123\"\n")
	return s
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

func (f failingReads) Pulls(int) ([]forge.Pull, error) {
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
	c := &Context{Ops: o, Env: env.Env{Root: s.root, Workspace: s.ws}, Stdin: strings.NewReader(""), Stdout: &s.stdout, Stderr: &s.stderr, Flags: flags}
	return swarmInit(c, func(_ ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
		s.opened = []string{f.Kind, f.API, f.Repo, strings.Join(names, ","), token}
		return failingReads{s.forge, s.readFails, s.declaredReadFails, s.loginFails}, nil
	})
}

const units = `
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: one apply pass for the workspace %f

[Service]
Type=oneshot
WorkingDirectory=%f
Environment=DAISY_ROOT=<root>
ExecStart=/opt/daisy/bin/daisy swarm apply --once

# ~/.config/systemd/user/daisy-swarm@.timer
[Unit]
Description=Daisy swarm: apply every 5min for the workspace %f

[Timer]
OnActiveSec=1min
OnUnitInactiveSec=5min

[Install]
WantedBy=timers.target

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
	wantOut += "✓ Labels: 8 present (8 created)\n" + s.units()
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
	if err := declare.Validate(ops.Real(), s.ws, c, 0); err != nil {
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
	wantOut += "✓ Labels: 8 present (0 created)\n" + s.units()
	if s.stdout.String() != wantOut || s.read("swarm/swarm.json") != before || s.read("swarm/prompts/worker.md") != "# My worker\n" {
		t.Errorf("second run:\n%s", s.stdout.String())
	}
	if len(s.forge.Labels()) != 8 {
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
	s.answers = append(s.answers, "jane", "")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	f := s.read("swarm/swarm.json")
	f = strings.Replace(f, `"check": []`, `"check": ["make", "check"]`, 2)
	s.write("swarm/swarm.json", f)
	s.answers = make([]string, 35)
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
// written; once the owner creates them, a re-run finds all eight.
func TestSwarmInitRefusedLabels(t *testing.T) {
	s := newSwarmWS(t)
	s.forge.AddLabels("swarm:proposed", "swarm:ready")
	s.forge.RefuseLabels()
	err := s.run("--defaults")
	want := "Error: forge: EnsureLabels: acme's labels missing: swarm:planning, swarm:planned, swarm:in-review, swarm:reviewed, swarm:approved, swarm:blocked; an owner of acme creates them once"
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
	if err := s.run("--defaults"); err != nil || !strings.Contains(s.stdout.String(), "✓ Labels: 8 present (0 created)\n") {
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

// Question 33 refuses the swarm's login and asks again; EOF cancels and
// writes nothing.
func TestSwarmInitMaintainerQuestion(t *testing.T) {
	s := newSwarmWS(t)
	s.answers = append(make([]string, 33), "swarm-bot", "jane", "")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.stderr.String(), `swarm.json: trusted: "swarm-bot" is the token's own login; the swarm needs its own forge account (section 8)`+"\nMaintainer logins") {
		t.Errorf("stderr:\n%s", s.stderr.String())
	}
	if !strings.Contains(s.read("swarm/swarm.json"), `"trusted": ["jane", "swarm-bot"]`) {
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
	for in, want := range map[string]string{"/home/jane/ws": "daisy-swarm@home-jane-ws.timer", "/srv/my-ws": `'daisy-swarm@srv-my\x2dws.timer'`} {
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
