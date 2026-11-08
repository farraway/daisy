package commands

import (
	"errors"
	"fmt"
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

// statusWS is a swarm initialised with --defaults, its $DAISY_LOCAL a
// temp directory and `docker ps` scripted.
type statusWS struct {
	*swarmWS
	now       time.Time // status's clock when set, else the Fake forge's
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
		GetenvFn: func(k string) string { s.asked = append(s.asked, k); return s.vars[k] },
		NowFn: func() time.Time {
			if !s.now.IsZero() {
				return s.now
			}
			return fixedClock().Now()
		},
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
		return swarmStatus(c, func(_ ops.Ops, _ declare.Forge, _ []string, token string) (forge.Adapter, error) {
			s.tokens = append(s.tokens, token)
			return f, nil
		})
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
  stage time   none
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
	inMilestone(t, f, "roadmap: M0") // next.md's M1 has none on the forge yet

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
	briefs := "Briefs:\n  docs/briefs/next.md: 1 of 1 milestones not on the forge\n  docs/briefs/roadmap.md: 1 of 1 milestones with no issue\n"
	if out := s.stdout.String(); !strings.HasPrefix(out, want) || !strings.HasSuffix(out, briefs) {
		t.Errorf("stdout:\n%s\nwant:\n%s…\n%s", out, want, briefs)
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

// runResult writes a run's result, and its out.md when out is not "".
func (s *statusWS) runResult(name, result, out string) {
	s.t.Helper()
	os.MkdirAll(s.state("runs/"+name+"/io"), 0o755)
	if err := os.WriteFile(s.state("runs/"+name+"/result"), []byte(result), 0o644); err != nil {
		s.t.Fatal(err)
	}
	if out != "" {
		os.WriteFile(s.state("runs/"+name+"/io/out.md"), []byte(out), 0o644)
	}
}

// ran is a run's result lines: its label (none for the batch), its start
// at the time of day start and its end length later, then rest.
func ran(label, start, length, rest string) string {
	from, end := span(start, length)
	head := ""
	if label != "" {
		head = "label swarm:" + label + "\n"
	}
	return head + "started " + from + "\nexited 0 " + end + "\n" + rest
}

// checked is a worker run's check lines, from start for length.
func checked(start, length string) string {
	from, end := span(start, length)
	return "check started " + from + "\ncheck exited 0 " + end + "\n"
}

func span(start, length string) (string, string) {
	from, _ := time.Parse(time.RFC3339, "2026-11-06T"+start+"Z")
	d, _ := time.ParseDuration(length)
	return from.Format(time.RFC3339), from.Add(d).Format(time.RFC3339)
}

// inMilestone is the milestone title on the forge, ensured as apply
// ensures it, with the issues ns put in it.
func inMilestone(t *testing.T, f *forge.Fake, title string, ns ...int) {
	t.Helper()
	id, err := f.EnsureMilestone(title)
	for _, n := range ns {
		if err == nil {
			err = f.SetMilestone(n, id)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

const m0 = "Brief: docs/briefs/m0.md — lib measured against upstream 2.0.0\n\nMeasure it.\n"

// Status on Step 7's swarm (the first real issue, docs/swarm.md section
// 13): batch 1's five proposals closed unanswered; batch 2's two
// proposals naming the brief; #6 planned once, built in two slices in
// lib, slice 1 reviewed in two rounds with one revision, slice 2 in one;
// the maintainer's only acts the relabels and the merges. Each count is
// the one the forge shows by hand, and the Waiting line follows the
// merges: merge, then next slice, then close.
func TestSwarmStatusStep7(t *testing.T) {
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) { c.Trusted = []string{"jane", "swarm-bot"}; c.Resources = []string{"docs/"} })
	s.write("docs/briefs/m0.md", "# M0\n\nStatus: active\n\n## lib measured against upstream 2.0.0\n\n## lib green against upstream 2.0.0\n\n## Drift checks in CI\n\n## A second port\n")
	f := s.forge
	for i := 1; i <= 5; i++ { // batch 1, before the brief: closed by the maintainer
		n, _ := f.CreateIssue("a guess", mark("owner", 1)+"A job.\n", []string{"swarm:proposed"})
		f.CloseIssue(n)
	}
	n, _ := f.CreateIssue("lib: pin to v2.0.0", mark("owner", 2)+m0, []string{"swarm:proposed"}) // 6
	f.CreateIssue("lib: CI", mark("owner", 2)+m0, []string{"swarm:proposed"})                    // 7
	is := forge.Ref{Repo: "app", N: n}
	f.SetLabel(is, "swarm:ready")
	f.Comment(is, mark("architect", 1)+"## Research\n\n## Steps\n\n1. lib: the mechanism.\n2. lib: the pin.\n")
	f.Relabel(is, "swarm:ready", "swarm:planning")
	f.SetLabel(is, "swarm:planned")
	slice := func(s int, heads ...string) forge.Ref {
		m, _ := f.CreatePull("lib", fmt.Sprintf("swarm/%d-%d", n, s), "main", "slice", mark("worker", 1)+"Refs acme/app#6\n")
		pr := forge.Ref{Repo: "lib", N: m}
		for i, h := range heads {
			f.Push(pr, h)
			if i > 0 {
				f.Comment(pr, mark("worker", 2)+"Left both findings, with reasons.\n")
			}
			adv := approve
			if i < len(heads)-1 {
				adv = requestChanges
			}
			f.Review(pr, forge.RequestChanges, mark("adversary", 1)+adv)
			f.Review(pr, forge.Approve, mark("sympathetic", 1)+approve)
		}
		return pr
	}
	s1 := slice(1, "aaa1", "aaa2")
	f.SetLabel(is, "swarm:approved")
	want := func(waiting string) {
		t.Helper()
		if err := s.status(f); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s.stdout.String(), "Waiting for you: "+waiting+"\n") {
			t.Errorf("want Waiting for you: %s in\n%s", waiting, s.stdout.String())
		}
	}
	want("triage #7; merge #6")
	f.Merge(s1, "aaa2")
	want("triage #7; next slice #6")
	f.SetLabel(is, "swarm:planned")
	s2 := slice(2, "bbb1")
	f.SetLabel(is, "swarm:approved")
	want("triage #7; merge #6")
	f.Merge(s2, "bbb1")
	want("triage #7; close #6")
	f.CloseIssue(n)
	// The milestones apply mirrors from the brief, #6 and #7 in the first.
	inMilestone(t, f, "m0: lib measured against upstream 2.0.0", 6, 7)
	inMilestone(t, f, "m0: lib green against upstream 2.0.0")
	inMilestone(t, f, "m0: Drift checks in CI")
	inMilestone(t, f, "m0: A second port")

	s.runResult("0-owner-1", ran("", "08:26:22", "2m32s", "propose 5\nposted\noutcome posted\ncollected\n"), "## a guess\n")
	s.runResult("0-owner-2", ran("", "09:16:00", "2m57s", "propose 5\noutcome posted\ncollected\n"), "## lib: pin to v2.0.0\n"+m0)
	s.runResult("6-architect-1", ran("ready", "10:06:00", "6m9s", "posted comment\nrelabelled swarm:ready swarm:planning\noutcome posted\ncollected\n"), "")
	s.runResult("6-worker-1", ran("planned", "10:24:00", "2m43s", checked("10:27:00", "19s")+"relabelled swarm:planned swarm:in-review\noutcome posted\ncollected\n"), "")
	s.runResult("6-adversary-1", ran("in-review", "10:33:00", "2m55s", "outcome posted\ncollected\n"), "")
	s.runResult("6-sympathetic-1", ran("in-review", "10:41:00", "1m44s", "relabelled swarm:in-review swarm:reviewed\noutcome posted\ncollected\n"), "")
	s.runResult("6-worker-2", ran("reviewed", "11:06:00", "2m14s", checked("11:09:00", "18s")+"relabelled swarm:reviewed swarm:in-review\noutcome posted\ncollected\n"), "")
	s.runResult("6-adversary-2", ran("in-review", "11:14:00", "2m43s", "outcome posted\ncollected\n"), "")
	s.runResult("6-sympathetic-2", ran("in-review", "11:22:00", "1m9s", "relabelled swarm:in-review swarm:approved\noutcome posted\ncollected\n"), "")
	s.runResult("6-worker-3", ran("planned", "11:41:00", "1m44s", checked("11:43:00", "18s")+"relabelled swarm:planned swarm:in-review\noutcome posted\ncollected\n"), "")
	s.runResult("6-adversary-3", ran("in-review", "11:49:00", "3m37s", "outcome posted\ncollected\n"), "")
	s.runResult("6-sympathetic-3", ran("in-review", "11:58:00", "1m24s", "relabelled swarm:in-review swarm:approved\noutcome posted\ncollected\n"), "")
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	wantTail := `Waiting for you: triage #7
Running: none
` + statusPersonas + `Throughput:
  runs         12 (12 posted, 0 blocked, 0 failed, 0 faults, 0 other)
  cost         $0.00 today, $0.00 in all (12 runs without a cost)
  budget       none
  approved     2 pull requests; at the round limit 0, blocked there 0
  stage time   batch 2m45s (2), ready 6m9s (1), planned 2m14s (2), in-review 2m14s (6), reviewed 2m14s (1), check 18s (3)
  #6           plan revisions 0, blocks 0; lib#1 rounds 2, maintainer comments 0; lib#2 rounds 1, maintainer comments 0
Briefs:
  docs/briefs/m0.md: lib measured against upstream 2.0.0 (1 open, 1 closed); 3 of 4 milestones with no issue
`
	if out := s.stdout.String(); !strings.HasSuffix(out, wantTail) {
		t.Errorf("stdout:\n%s\nwant it to end:\n%s", out, wantTail)
	}
}

// The counts' rules: a plan revision is an architect's plan after its
// first (BLOCKED: and another login's marker are none); a block is a
// reconciler note other than the round limit's first; a maintainer
// comment is a comment, review or line comment by a trusted login that is
// not the swarm's; an issue with none of a plan, a pull request or a
// block has no line, and a closed one only within the batch's window
// (30 days), while a brief's milestone counts every issue the forge has
// in it, closed at any time or not the swarm's, and none outside it. A
// batch run's Met: line is read from the newest batch run with an output;
// a run's time from its start to its exit or timeout.
func TestSwarmStatusCorrections(t *testing.T) {
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) { c.Trusted = []string{"jane", "swarm-bot"}; c.Resources = []string{"docs/"} })
	s.write("docs/briefs/m0.md", "Status: active\n\n## lib measured against upstream 2.0.0\n\n## Later\n")
	f := s.forge
	issue := func(label, body string) forge.Ref {
		n, _ := f.CreateIssue("t", body, []string{label})
		return forge.Ref{Repo: "app", N: n}
	}
	a := issue("swarm:planning", mark("owner", 1)+m0) // 1
	f.Comment(a, mark("architect", 1)+"## Research\n")
	f.Comment(a, mark("architect", 2)+"BLOCKED: which corpus?\n")
	f.Say(a, "jane", mark("architect", 3)+"not the swarm's")
	f.Comment(a, mark("architect", 4)+"## Research\nrevised\n")
	f.Comment(a, mark("reconciler", 1)+"architect run 2 is blocked (was swarm:planning). Answer and relabel the issue to resume.")
	f.Comment(a, mark("reconciler", 1)+"Review round 5 of 5 on app#9 requested changes (was swarm:in-review). The architect revises the plan from the reviews.")
	f.Comment(a, mark("reconciler", 2)+"Review round 5 of 5 on app#9 requested changes a second time (was swarm:in-review). Relabel the issue to resume.")
	m, _ := f.CreatePull("lib", "swarm/1-1", "main", "s", mark("worker", 1)+"Refs acme/app#1\n")
	pr := forge.Ref{Repo: "lib", N: m}
	f.Push(pr, "ccc1")
	f.Say(pr, "jane", "why?")
	f.Say(pr, "mallory", "not trusted")
	f.Comment(pr, mark("worker", 1)+"because\n")
	f.SayOnLine(pr, "Jane", "a.go", 3, "this line")
	f.ReviewAs(pr, "jane", forge.RequestChanges, "no")
	f.ReviewAs(pr, "SWARM-BOT", forge.Commented, "the swarm's own login, unmarked")
	b := issue("swarm:blocked", "") // 2: a block, no plan
	f.Comment(b, mark("reconciler", 1)+"owner run 1 failed: no output (was swarm:proposed). Relabel the issue to resume.")
	issue("swarm:proposed", mark("owner", 2)+m0) // 3: nothing to count
	old := issue("swarm:ready", mark("owner", 2)+"Brief: docs/briefs/m0.md — Later\n")
	f.Comment(old, mark("architect", 1)+"## Research\n")
	f.CloseIssue(old.N)                                                                // 4
	f.CreateIssue("t", mark("owner", 2)+m0, []string{"swarm:proposed", "swarm:ready"}) // 5: two labels, one issue
	f.AddLabels("bug")
	if n, err := f.CreateIssue("t", "", []string{"bug"}); n != 6 || err != nil { // 6: the maintainer's own, in a milestone by hand
		t.Fatal(n, err)
	}
	issue("swarm:proposed", mark("owner", 2)+m0) // 7: in no milestone yet, so not counted
	inMilestone(t, f, "m0: lib measured against upstream 2.0.0", 1, 3, 5, 6)
	inMilestone(t, f, "m0: Later", 4)

	s.runResult("0-owner-1", ran("", "08:00:00", "1m", "outcome posted\ncollected\n"), "Met: docs/briefs/other.md\n## x\n")
	s.runResult("0-owner-2", ran("", "09:00:00", "3m", "outcome posted\ncollected\n"), "Met: docs/briefs/m0.md\n\n## x\n")
	s.runResult("0-owner-3", ran("", "10:00:00", "2m", "failed: no output\noutcome failed\ncollected\n"), " \n")
	s.runResult("1-architect-4", "label swarm:planning\nstarted 2026-11-06T10:00:00Z\ntimeout 2026-11-06T10:45:00Z\noutcome failed\ncollected\n", "")
	s.runResult("1-architect-5", "label swarm:planning\nstarted 2026-11-06T11:00:00Z\nabandoned 2026-11-06T12:00:00Z\noutcome abandoned\ncollected\n", "")
	s.runResult("1-worker-1", ran("planned", "11:00:00", "4m", "check started 2026-11-06T11:04:00Z\n"), "")
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	wantTail := `  stage time   batch 2m0s (3), planning 45m0s (1), planned 4m0s (1)
  #1           plan revisions 1, blocks 2; lib#1 rounds 0, maintainer comments 3
  #2           plan revisions 0, blocks 1
  #4           plan revisions 0, blocks 0
Briefs:
  docs/briefs/m0.md: lib measured against upstream 2.0.0 (4 open, 0 closed); Later (0 open, 1 closed); met (owner run 2)
`
	if out := s.stdout.String(); !strings.HasSuffix(out, wantTail) {
		t.Errorf("stdout:\n%s\nwant it to end:\n%s", out, wantTail)
	}

	s.now = fixedClock().Now().Add(31 * 24 * time.Hour) // #4 closed 31 days ago: out of the window
	if err := s.status(f); err != nil {
		t.Fatal(err)
	}
	if out := s.stdout.String(); strings.Contains(out, "  #4 ") || !strings.Contains(out, "Later (0 open, 1 closed)") {
		t.Errorf("31 days on:\n%s", out)
	}
}

// Met: is read from the newest batch run with an output, never from a
// member's run, however new (section 11, status).
func TestSwarmStatusMetFromBatchRunsOnly(t *testing.T) {
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) { c.Resources = []string{"docs/"} })
	s.write("docs/briefs/m0.md", "Status: active\n\n## First\n")
	s.runResult("0-owner-2", ran("", "09:00:00", "3m", "outcome posted\ncollected\n"), "Met: docs/briefs/m0.md\n")
	s.runResult("1-architect-3", ran("planning", "10:00:00", "3m", "outcome posted\ncollected\n"), "Met: docs/briefs/other.md\n\n## Research\nplan\n")
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	if out := s.stdout.String(); !strings.Contains(out, "met (owner run 2)") {
		t.Errorf("stdout:\n%s", out)
	}
}
