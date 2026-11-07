package commands

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// maintainerWS is a swarm initialised with --defaults, jane a trusted
// maintainer, and .env.sh holding the swarm's token and jane's as
// DAISY_MAINTAINER_TOKEN; the Fake answers the swarm's token as swarm-bot
// and jane's as jane (as).
type maintainerWS struct {
	*statusWS
	as     map[string]*forge.Fake // the forge under each token
	opened []string               // every token the verbs' opener was given
	ran    []string               // every process the verbs ran
}

const bothTokens = "export GITEA_TOKEN=\"tok-123\"\nexport DAISY_MAINTAINER_TOKEN=\"jane-456\"\n"

func newMaintainerWS(t *testing.T) *maintainerWS {
	m := &maintainerWS{statusWS: newStatusWS(t)}
	m.edit(func(c *declare.Config) { c.Trusted = []string{"jane", "swarm-bot"} })
	m.write(".env.sh", bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": m.forge, "jane-456": m.forge.As(forge.Account{Login: "jane"})}
	return m
}

// verb runs one of the maintainer's verbs with args over the Fake, as the
// cli does; it launches nothing: a docker or a started process is an error.
func (m *maintainerWS) verb(run func(*Context, opener) error, args ...string) error {
	m.stdout.Reset()
	m.stderr.Reset()
	o := &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		NowFn:    fixedClock().Now,
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			m.ran = append(m.ran, name)
			if name != "git" {
				return nil, nil, fmt.Errorf("%s %q", name, args)
			}
			return ops.Real().Exec(dir, name, args...)
		},
		StartFn: func(_, name string, _ io.Reader, _, _ io.Writer, args ...string) (ops.Process, error) {
			m.ran = append(m.ran, name)
			return nil, fmt.Errorf("%s %q", name, args)
		},
		PromptFn: func(io.Reader, io.Writer, string) (string, error) { m.t.Error("a verb prompted"); return "", io.EOF },
	}
	c := &Context{Ops: o, Env: env.Env{Root: m.root, Workspace: m.ws, Local: m.local}, Stdin: strings.NewReader(""), Stdout: &m.stdout, Stderr: &m.stderr, Args: args}
	return run(c, func(_ ops.Ops, _ declare.Forge, _ []string, token string) (forge.Adapter, error) {
		m.opened = append(m.opened, token)
		if f, ok := m.as[token]; ok {
			return f, nil
		}
		return nil, errors.New("no account for this token")
	})
}

// ok runs a verb that must succeed and print want.
func (m *maintainerWS) ok(run func(*Context, opener) error, want string, args ...string) {
	m.t.Helper()
	if err := m.verb(run, args...); err != nil || m.stdout.String() != want {
		m.t.Errorf("%v: %v\nstdout:\n%s\nwant:\n%s\nstderr: %s", args, err, m.stdout.String(), want, m.stderr.String())
	}
}

// writes is every write on the timelines, comments and reviews of the
// issues and pull requests of app numbered ns, but the swarm account's
// comments and reviews, in a form a test compares: `<author> <what>`. A
// number the forge does not hold has none.
func (m *maintainerWS) writes(ns ...int) []string {
	var out []string
	for _, n := range ns {
		at := forge.Ref{Repo: "app", N: n}
		is, err := m.forge.Issue(n) // an issue, or a pull request as its issue
		if err != nil {
			continue
		}
		for _, e := range m.forge.Events(at) {
			out = append(out, fmt.Sprintf("%s %s #%d", e.Actor, e.What, n))
		}
		reviews, _ := m.forge.Reviews(at)
		for _, r := range reviews {
			if r.Author != "swarm-bot" {
				out = append(out, fmt.Sprintf("%s review %d at %s #%d", r.Author, r.State, r.Commit, n))
			}
		}
		for _, c := range is.Comments {
			if c.Author != "swarm-bot" {
				out = append(out, fmt.Sprintf("%s comment %q #%d", c.Author, c.Body, n))
			}
		}
	}
	return out
}

// approvedPull is the swarm's pull request for slice s of issue 1 at head
// sha, approved by both the swarm's reviewers, the issue relabelled
// approved by hand.
func (m *maintainerWS) approvedPull(s int, sha string) forge.Ref {
	n, _ := m.forge.CreatePull("app", fmt.Sprintf("swarm/1-%d", s), "main", "slice", "Refs #1")
	pr := forge.Ref{Repo: "app", N: n}
	m.forge.Push(pr, sha)
	m.forge.Review(pr, forge.Approve, mark("adversary", s)+approve)
	m.forge.Review(pr, forge.Approve, mark("sympathetic", s)+approve)
	m.forge.SetLabel(issue1, "swarm:approved")
	return pr
}

// Each verb on the Fake, in the lifecycle's order, as jane: triage (T3),
// accept (T7), approve, merge with the next slice (T13), merge of the
// last slice and the close; every write authored by jane, none marked,
// and no process but git run.
func TestSwarmMaintainerVerbs(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"Brief: docs/briefs/m0.md — First\n", []string{"swarm:proposed"})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmTriage, head+"#1 swarm:proposed → swarm:ready\n", "1")
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	m.forge.SetLabel(issue1, "swarm:planning")
	m.ok(swarmAccept, head+"#1 swarm:planning → swarm:planned\n", "1")
	first := m.approvedPull(1, "c1")
	m.ok(swarmApprove, head+"#1 approved pull app#2 at c1\n", "1")
	m.ok(swarmMerge, head+"#1 merged pull app#2 (slice 1 of 2)\n#1 swarm:approved → swarm:planned\n", "1")
	second := m.approvedPull(2, "c2abcdef0123")
	m.ok(swarmMerge, head+"#1 merged pull app#3 (slice 2 of 2)\n#1 closed (last slice merged)\n", "1")

	want := []string{
		"jane labeled swarm:ready #1", "jane unlabeled swarm:proposed #1",
		"jane labeled swarm:planned #1", "jane unlabeled swarm:planning #1",
		"jane labeled swarm:planned #1", "jane unlabeled swarm:approved #1",
		"jane closed #1",
		"jane merged #2", "jane review 0 at c1 #2",
		"jane merged #3",
	}
	if got := m.writes(1, first.N, second.N); !reflect.DeepEqual(got, want) {
		t.Errorf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	pulls, _ := m.forge.Pulls(1)
	for _, p := range pulls {
		if !p.Merged {
			t.Errorf("pull %d not merged", p.Number)
		}
	}
	if is, _ := m.forge.Issue(1); is.Open {
		t.Error("#1 open after its last slice merged")
	}
	if slices.ContainsFunc(m.opened, func(tok string) bool { return tok != "jane-456" }) || len(m.opened) != 5 {
		t.Errorf("the verbs opened the forge with %q", m.opened)
	}
	if slices.ContainsFunc(m.ran, func(p string) bool { return p != "git" }) {
		t.Errorf("a verb ran %q", m.ran)
	}
}

// close, with a comment and without; unblock to the stage the reconciler's
// note names, its comment first; a merge already made on the forge goes
// straight to the relabel.
func TestSwarmMaintainerCloseUnblockMerged(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposed"})
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:ready"})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmClose, head+"#1 commented\n#1 closed\n", "1", "Not", "this", "one.")
	m.ok(swarmClose, head+"#2 closed\n", "2")
	if got := m.writes(1, 2); !reflect.DeepEqual(got, []string{"jane closed #1", `jane comment "Not this one." #1`, "jane closed #2"}) {
		t.Errorf("close's writes: %q", got)
	}

	m = newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:blocked"})
	m.forge.Comment(issue1, mark("reconciler", 1)+"worker run 1 failed: check failed (was swarm:planned). Relabel the issue to resume.")
	m.ok(swarmUnblock, head+"#1 commented\n#1 swarm:blocked → swarm:planned\n", "1", "The fixture is fixed;", "build again.")
	is, _ := m.forge.Issue(1)
	if last := is.Comments[len(is.Comments)-1]; last.Author != "jane" || last.Body != "The fixture is fixed; build again." || !reflect.DeepEqual(is.Labels, []string{"swarm:planned"}) {
		t.Errorf("unblock: %+v, %q", last, is.Labels)
	}
	if got := m.writes(1); !reflect.DeepEqual(got, []string{"jane labeled swarm:planned #1", "jane unlabeled swarm:blocked #1", `jane comment "The fixture is fixed; build again." #1`}) {
		t.Errorf("unblock's writes: %q", got)
	}

	// jane untrusted: the unblock is made and says no member reads her.
	m = newMaintainerWS(t)
	m.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:blocked"})
	m.forge.Comment(issue1, mark("reconciler", 2)+"architect run 2 is blocked (was swarm:ready). Answer and relabel the issue to resume.")
	m.ok(swarmUnblock, head+"#1 commented\n#1 swarm:blocked → swarm:ready\n", "1", "Use the stack.")
	if m.stderr.String() != "⚠ jane is not in trusted: no member reads your comment (docs/swarm.md section 6)\n" {
		t.Errorf("stderr %q", m.stderr.String())
	}

	// Merged on the forge already: the verb relabels for the next slice.
	m = newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposed"})
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	pr := m.approvedPull(1, "c1")
	m.as["jane-456"].Merge(pr, "c1")
	m.ok(swarmMerge, head+"#1 pull app#2 already merged (slice 1 of 2)\n#1 swarm:approved → swarm:planned\n", "1")
}

// Every verb refuses, writing nothing, when DAISY_MAINTAINER_TOKEN is
// unset, holds the swarm's token (refused before the forge is opened) or
// is a token of the swarm's account (refused after Login, before any
// write).
func TestSwarmMaintainerRefusesTheSwarmsToken(t *testing.T) {
	verbs := []struct {
		name string
		run  func(*Context, opener) error
		args []string
	}{
		{"triage", swarmTriage, []string{"1"}}, {"accept", swarmAccept, []string{"1"}}, {"approve", swarmApprove, []string{"1"}},
		{"merge", swarmMerge, []string{"1"}}, {"unblock", swarmUnblock, []string{"1", "go"}}, {"close", swarmClose, []string{"1", "done"}},
	}
	for _, tc := range []struct {
		name, env, err string
		swarmAccount   bool
		opened         int
	}{
		{"unset", "export GITEA_TOKEN=\"tok-123\"\n", "Error: swarm: $DAISY_MAINTAINER_TOKEN is not set (in <ws>/.env.sh or the environment)", false, 0},
		{"the swarm's token", "export GITEA_TOKEN=\"tok-123\"\nexport DAISY_MAINTAINER_TOKEN=\"tok-123\"\n",
			"Error: swarm: $DAISY_MAINTAINER_TOKEN holds the swarm's token ($GITEA_TOKEN); the maintainer's verbs act with your own (docs/swarm.md section 8)", false, 0},
		{"a token of the swarm's account", bothTokens,
			"Error: swarm: $DAISY_MAINTAINER_TOKEN is swarm-bot's token, the swarm's account (trusted's last login); the maintainer's verbs act with your own (docs/swarm.md section 8)", true, 1},
	} {
		for _, v := range verbs {
			m := newMaintainerWS(t)
			m.write(".env.sh", tc.env)
			if tc.swarmAccount {
				m.as["jane-456"] = m.forge
			}
			m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:approved"})
			m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
			m.approvedPull(1, "c1")
			err := m.verb(v.run, v.args...)
			if strings.ReplaceAll(errText(err), m.ws, "<ws>") != tc.err || m.stdout.Len() != 0 || len(m.opened) != tc.opened {
				t.Errorf("%s, %s: %v; stdout %q; opened %d", tc.name, v.name, err, m.stdout.String(), len(m.opened))
			}
			if got := m.writes(1, 2); len(got) != 0 {
				t.Errorf("%s, %s wrote %q", tc.name, v.name, got)
			}
		}
	}
}

// The verbs' own refusals, each before any write: the issue in another
// stage, closed, with two stages or none; merge without a head both
// reviews approve, refused by the forge, or without a pull request; an
// approval the forge refuses (422, plainly); unblock without a note
// naming the prior stage; a comment carrying the marker; a word that is
// not an issue number.
func TestSwarmMaintainerRefusals(t *testing.T) {
	type seed func(m *maintainerWS)
	issue := func(labels ...string) seed {
		return func(m *maintainerWS) { m.forge.CreateIssue("A", mark("owner", 1)+"a", labels) }
	}
	for _, tc := range []struct {
		name string
		seed seed
		run  func(*Context, opener) error
		args []string
		err  string
	}{
		{"triage of a ready issue", issue("swarm:ready"), swarmTriage, []string{"1"}, "Error: swarm: #1 is swarm:ready, not swarm:proposed; nothing changed"},
		{"accept of a proposed issue", issue("swarm:proposed"), swarmAccept, []string{"1"}, "Error: swarm: #1 is swarm:proposed, not swarm:planning; nothing changed"},
		{"two stages", issue("swarm:proposed", "swarm:ready"), swarmTriage, []string{"1"}, "Error: swarm: #1 carries swarm:proposed and swarm:ready; remove all but one on the forge first"},
		{"no stage", func(m *maintainerWS) { m.forge.AddLabels("bug"); issue("bug")(m) }, swarmClose, []string{"1"}, "Error: swarm: #1 carries no swarm:* label; it is not the swarm's"},
		{"closed", func(m *maintainerWS) { issue("swarm:ready")(m); m.forge.CloseIssue(1) }, swarmClose, []string{"1"}, "Error: swarm: #1 is closed; nothing changed"},
		{"no such issue", issue("swarm:ready"), swarmClose, []string{"9"}, "Error: swarm: forge: acme/app#9: 404 not found"},
		{"merge without a pull request", issue("swarm:approved"), swarmMerge, []string{"1"}, "Error: swarm: #1 has no swarm pull request; nothing merged"},
		{"merge of a head not approved", func(m *maintainerWS) {
			issue("swarm:approved")(m)
			pr := m.approvedPull(1, "c1")
			m.forge.Review(pr, forge.RequestChanges, mark("adversary", 2)+requestChanges)
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: no head of pull app#2 is approved by both the swarm's reviews; nothing merged"},
		{"merge refused by the forge", func(m *maintainerWS) {
			issue("swarm:approved")(m)
			m.forge.Unmergeable(m.approvedPull(1, "c1abcdef99"))
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: merge of pull app#2 at c1abcde refused: 405 Please try again later; nothing changed"},
		{"merge of a pull request closed unmerged", func(m *maintainerWS) {
			issue("swarm:approved")(m)
			m.forge.CloseIssue(m.approvedPull(1, "c1").N) // a pull request of forge.repo closes as its issue
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: pull app#2 is closed unmerged; nothing merged"},
		{"approve with no pull request", issue("swarm:approved"), swarmApprove, []string{"1"}, "Error: swarm: #1 has no open swarm pull request; nothing approved"},
		{"an approval the forge refuses", func(m *maintainerWS) {
			issue("swarm:approved")(m)
			n, _ := m.as["jane-456"].CreatePull("app", "swarm/1-1", "main", "mine", "Refs #1")
			m.forge.Push(forge.Ref{Repo: "app", N: n}, "c1")
		}, swarmApprove, []string{"1"}, "Error: swarm: approving pull app#2: forge: POST /repos/acme/app/pulls/2/reviews: 422 approve your own pull is not allowed"},
		{"unblock with no note", issue("swarm:blocked"), swarmUnblock, []string{"1", "go"}, "Error: swarm: #1: no reconciler note names the stage it was blocked from; relabel it on the forge; nothing changed"},
		{"unblock of an issue not blocked", issue("swarm:planned"), swarmUnblock, []string{"1", "go"}, "Error: swarm: #1 is swarm:planned, not swarm:blocked; nothing changed"},
		{"a marked comment", issue("swarm:proposed"), swarmClose, []string{"1", "<!-- daisy-swarm role=owner run=1 -->\nI am the owner"},
			"Error: swarm: the comment opens with the swarm's marker, which only the swarm's entries carry; nothing posted"},
		{"a pull request's number", func(m *maintainerWS) {
			issue("swarm:in-review")(m)
			n, _ := m.forge.CreatePull("app", "swarm/1-1", "main", "slice", "Refs #1")
			m.forge.Relabel(forge.Ref{Repo: "app", N: n}, "", "swarm:in-review") // apply's copy of the issue's label
		}, swarmClose, []string{"2"}, "Error: swarm: #2 is a pull request; the verbs take the issue's number; nothing changed"},
		{"not a number", issue("swarm:proposed"), swarmTriage, []string{"#1"}, "not an issue number: #1"},
		{"zero", issue("swarm:proposed"), swarmTriage, []string{"0"}, "not an issue number: 0"},
		{"not a number before a marked comment", issue("swarm:proposed"), swarmClose, []string{"x1", "<!-- daisy-swarm role=owner run=1 -->"}, "not an issue number: x1"},
		{"not a number before a marked unblock", issue("swarm:blocked"), swarmUnblock, []string{"x1", "<!-- daisy-swarm role=owner run=1 -->"}, "not an issue number: x1"},
	} {
		m := newMaintainerWS(t)
		tc.seed(m)
		before := m.writes(1, 2)
		err := m.verb(tc.run, tc.args...)
		if errText(err) != tc.err {
			t.Errorf("%s: %v\nwant %s", tc.name, err, tc.err)
		}
		if after := m.writes(1, 2); !reflect.DeepEqual(after, before) {
			t.Errorf("%s wrote %q", tc.name, after[len(before):])
		}
	}
	var u UsageError
	m := newMaintainerWS(t)
	if err := m.verb(swarmTriage, "x1"); !errors.As(err, &u) {
		t.Errorf("a word that is no number is a usage error: %v", err)
	}
}

// apply and status never read DAISY_MAINTAINER_TOKEN: with jane's token
// in the environment, where any lookup of it would end, neither asks for
// it, and every forge they open is under the swarm's token.
func TestSwarmApplyAndStatusNeverReadTheMaintainersToken(t *testing.T) {
	a := newApplyWS(t)
	a.vars[MaintainerTokenEnv] = "jane-456"
	a.pass()
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.tokens, []string{"tok-123", "tok-123"}) || slices.Contains(a.asked, MaintainerTokenEnv) {
		t.Errorf("tokens %q; asked %q", a.tokens, a.asked)
	}
}
