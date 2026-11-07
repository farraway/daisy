package commands

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// statusWS is a swarm initialised with --defaults, its $DAISY_LOCAL a
// temp directory and `docker ps` scripted.
type statusWS struct {
	*swarmWS
	local     string
	docker    string // docker ps's stdout
	dockerErr bool
}

func newStatusWS(t *testing.T) *statusWS {
	s := &statusWS{swarmWS: newSwarmWS(t), local: t.TempDir()}
	if err := s.run("--defaults"); err != nil {
		t.Fatal(err)
	}
	return s
}

// edit rewrites swarm.json with edit applied.
func (s *statusWS) edit(edit func(c *declare.Config)) {
	s.t.Helper()
	c, _, err := declare.Load(ops.Real().FS(), s.ws)
	if err != nil {
		s.t.Fatal(err)
	}
	edit(&c)
	s.write("swarm/swarm.json", string(declare.Format(c)))
}

func (s *statusWS) state(rel string) string {
	return filepath.Join(s.local, "swarm", "forge.example.net", "acme", "app", rel)
}

// status is `swarm status` under ReadOnly, as the cli runs it, over the
// Fake forge.
func (s *statusWS) status(f forge.Adapter) error {
	s.stdout.Reset()
	s.stderr.Reset()
	o := &ops.Fake{
		GetenvFn: func(k string) string { return s.vars[k] },
		NowFn:    fixedClock().Now,
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			if name != "docker" {
				return ops.Real().Exec(dir, name, args...)
			}
			if strings.Join(args, " ") != `ps --filter label=daisy.swarm --format {{.Names}} {{.Label "daisy.swarm"}} {{.Label "daisy.credential"}}` {
				s.t.Errorf("docker %q", args)
			}
			if s.dockerErr {
				return nil, []byte("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n"), errors.New("exit status 1")
			}
			return []byte(s.docker), nil, nil
		},
		PromptFn: func(io.Reader, io.Writer, string) (string, error) { s.t.Error("status prompted"); return "", io.EOF },
	}
	c := &Context{Ops: o, Env: env.Env{Root: s.root, Workspace: s.ws, Local: s.local}, Stdin: strings.NewReader(""), Stdout: &s.stdout, Stderr: &s.stderr}
	return readOnly(func(c *Context) error {
		return swarmStatus(c, func(ops.Ops, declare.Forge, []string, string) (forge.Adapter, error) { return f, nil })
	})(c)
}

const statusPersonas = `Personas:
  owner        anthropic  claude-opus-5-5 high
  architect    anthropic  claude-fable-5-1 high
  worker       anthropic  claude-sonnet-5-5 medium
  adversary    openai     gpt-6.1-sol high
  sympathetic  anthropic  claude-opus-5-5 high
`

// After init: zero issues per stage, the gates, the personas, nothing
// running, no runs, no brief.
func TestSwarmStatusAfterInit(t *testing.T) {
	s := newStatusWS(t)
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	want := `Swarm: acme/app on forge.example.net (gitea), repositories app, lib
State: ` + s.state("") + `
Gates: triage on, plan on, merge on
  proposed     0
  ready        0
  planning     0
  planned      0
  in-review    0
  reviewed     0
  approved     0
  blocked      0
Waiting for you: nothing
Running: none
` + statusPersonas + `Throughput:
  runs         0 (0 posted, 0 blocked, 0 failed, 0 faults, 0 other)
  cost         $0.00 today, $0.00 in all (0 runs without a cost)
  budget       none
  approved     0 pull requests; at the round limit 0, blocked there 0
Briefs: none active
`
	if s.stdout.String() != want || s.stderr.Len() != 0 {
		t.Errorf("stdout:\n%s\nwant:\n%s\nstderr: %s", s.stdout.String(), want, s.stderr.String())
	}
	if _, err := os.Stat(s.state("")); err == nil {
		t.Error("status created <state>")
	}
}

const (
	archMark   = "<!-- daisy-swarm role=architect run=1 -->\n**architect:**\n\n## Research\n"
	reconciler = "<!-- daisy-swarm role=reconciler run=1 -->\n**reconciler:**\n\nfailed"
)

// The stages, whose turn each planning issue is, the paused and held
// notes, the running set, the active briefs.
func TestSwarmStatusStages(t *testing.T) {
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) { c.Trusted = []string{"jane", "swarm-bot"}; c.Resources = []string{"docs/"} })
	s.write("docs/briefs/roadmap.md", "# Roadmap\n\nStatus: active\n\n## M0\n")
	s.write("docs/briefs/old.md", "# Old\n\nStatus: done\n\n## M0\n")
	s.write("docs/briefs/next.md", "Status: active\n## M1\n")
	f := s.forge
	issue := func(label string) int {
		n, err := f.CreateIssue("t", "", []string{label})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	issue("swarm:proposed")                                  // 1
	issue("swarm:proposed")                                  // 2
	p3 := forge.Ref{Repo: "app", N: issue("swarm:planning")} // 3: the plan, the maintainer's turn
	f.Comment(p3, archMark)
	f.Say(p3, "mallory", "untrusted, passed over")
	p4 := forge.Ref{Repo: "app", N: issue("swarm:planning")} // 4: the maintainer answered: the architect's turn
	f.Comment(p4, archMark)
	f.Say(p4, "Jane", "revise step 2")
	p5 := forge.Ref{Repo: "app", N: issue("swarm:planning")} // 5: a marker under the maintainer's login is not the swarm's
	f.Say(p5, "jane", archMark)
	issue("swarm:approved")                                // 6
	b := forge.Ref{Repo: "app", N: issue("swarm:blocked")} // 7
	f.Comment(b, reconciler)
	issue("swarm:in-review")                                 // 8
	f.CreatePull("app", "swarm/8-1", "main", "s", "Refs #8") // 9: a pull request is no issue
	f.Relabel(forge.Ref{Repo: "app", N: 9}, "", "swarm:in-review")
	closed := issue("swarm:ready")
	f.CloseIssue(closed)

	os.MkdirAll(s.state(""), 0o755)
	os.WriteFile(s.state("paused"), []byte("paused 2026-11-06T10:00:00Z: swarm fault: docker: no such image\n"), 0o644)
	os.WriteFile(s.state("owner-held"), []byte("held 2026-11-06T09:00:00Z: proposed nothing\n"), 0o644)
	s.docker = "daisy-acme-app-8-adversary-1 acme/app#8/adversary \n" +
		"daisy-acme-app-0-owner-2 acme/app#0/owner anthropic/1\n" +
		"daisy-other-x-2-worker-1 other/x#2/worker openai/1\n" +
		"daisy-acme-app-12-check acme/app#12/check \n"
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	want := `Swarm: acme/app on forge.example.net (gitea), repositories app, lib
State: ` + s.state("") + ` — paused since 2026-11-06T10:00:00Z: swarm fault: docker: no such image — owner batch held: proposed nothing
Gates: triage on, plan on, merge on
  proposed     2  #1 #2
  ready        0
  planning     3  #3 #4 #5
  planned      0
  in-review    1  #8
  reviewed     0
  approved     1  #6
  blocked      1  #7
Waiting for you: triage #1 #2; plan #3; merge #6; blocked #7
Running: #0 owner, #8 adversary, #12 check
` + statusPersonas
	if out := s.stdout.String(); !strings.HasPrefix(out, want) || !strings.HasSuffix(out, "Briefs:\n  docs/briefs/next.md\n  docs/briefs/roadmap.md\n") {
		t.Errorf("stdout:\n%s\nwant:\n%s…\nBriefs:\n  docs/briefs/next.md\n  docs/briefs/roadmap.md\n", out, want)
	}

	// The gates off: nothing waits for triage, a plan or a merge.
	s.edit(func(c *declare.Config) {
		c.Gates = declare.Gates{}
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	s.dockerErr = true
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"Gates: triage off, plan off, merge off\n", "Waiting for you: blocked #7\n",
		"Running: unknown (docker: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?)\n",
		"  budget       $10.00 per day, $100.00 in all\n"} {
		if !strings.Contains(s.stdout.String(), line) {
			t.Errorf("gates off: no %q in\n%s", line, s.stdout.String())
		}
	}
}

// Throughput, from runs/ only.
func TestSwarmStatusThroughput(t *testing.T) {
	s := newStatusWS(t)
	today, yesterday := "started 2026-11-06T08:00:00Z\n", "started 2026-11-05T23:59:59Z\n"
	for name, result := range map[string]string{
		"3-architect-1":   today + "exited 0 2026-11-06T08:10:00Z\ncost 0.50 tokens 10/20 turns 3\nposted comment\nrelabelled swarm:ready swarm:planning\noutcome posted\ncollected\n",
		"3-adversary-1":   yesterday + "cost 1.25 tokens 1/2 turns 1\nrelabelled swarm:in-review swarm:approved\noutcome posted\ncollected\n",
		"3-sympathetic-2": today + "relabelled swarm:in-review swarm:planning\noutcome posted\ncollected\n",
		"4-adversary-3":   today + "cost 0.25 tokens 1/2 turns 1\nrelabelled swarm:in-review swarm:blocked\noutcome posted\ncollected\n",
		"4-worker-1":      today + "cost 2 tokens 1/2 turns 1\nfailed: push rejected\noutcome failed\ncollected\n",
		"5-worker-1":      today + "fault: provider\noutcome fault\ncollected\n",
		"0-owner-1":       today + "outcome blocked\ncollected\n",
		"6-architect-1":   today,
		"7-sympathetic-1": today + "relabelled swarm:in-review swarm:ready\noutcome failed\ncollected\n",
	} {
		os.MkdirAll(s.state("runs/"+name), 0o755)
		os.WriteFile(s.state("runs/"+name+"/result"), []byte(result), 0o644)
	}
	os.MkdirAll(s.state("runs/8-worker-1"), 0o755) // being prepared: no result yet
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	want := `Throughput:
  runs         9 (4 posted, 1 blocked, 2 failed, 1 faults, 1 other)
  cost         $2.75 today, $4.00 in all (5 runs without a cost)
  budget       none
  approved     1 pull requests; at the round limit 2, blocked there 1
`
	if !strings.Contains(s.stdout.String(), want) {
		t.Errorf("stdout:\n%s\nwant:\n%s", s.stdout.String(), want)
	}
}

func TestSwarmStatusRefusals(t *testing.T) {
	s := newStatusWS(t)
	s.write(".env.sh", "")
	if err := s.status(s.forge); strings.ReplaceAll(errText(err), s.ws, "<ws>") != "Error: swarm: $GITEA_TOKEN is not set (in <ws>/.env.sh or the environment)" {
		t.Errorf("token unset: %v", err)
	}
	s = newStatusWS(t)
	if err := s.status(failingReads{Fake: s.forge, login: true}); errText(err) != "Error: swarm: forge: GET /user: 401 invalid username, password or token" {
		t.Errorf("a forge failure: %v", err)
	}
	if err := s.status(failingReads{Fake: s.forge, issues: true}); errText(err) != "Error: swarm: forge: GET /repos/acme/app/issues?…: 404 Not Found" {
		t.Errorf("a forge failure: %v", err)
	}
	s.edit(func(c *declare.Config) { c.MaxRounds = 0 })
	if err := s.status(s.forge); errText(err) != "Error: swarm.json: max_rounds: 0 is not between 1 and 10" {
		t.Errorf("invalid swarm.json: %v", err)
	}
	os.RemoveAll(filepath.Join(s.ws, "swarm"))
	if err := s.status(s.forge); strings.ReplaceAll(errText(err), s.ws, "<ws>") != "Error: swarm.json: not found at <ws>/swarm/swarm.json; run 'daisy swarm init'" {
		t.Errorf("no swarm.json: %v", err)
	}
}
