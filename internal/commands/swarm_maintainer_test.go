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
	"daisy/internal/swarm/reconcile"
)

// maintainerWS is a swarm initialised with --defaults, jane a trusted
// maintainer, and .env.sh holding the swarm's token and jane's as
// DAISY_MAINTAINER_TOKEN; the Fake answers the swarm's token as swarm-bot
// and jane's as jane (as).
type maintainerWS struct {
	*statusWS
	as     map[string]*forge.Fake          // the forge under each token
	opened []string                        // every token the verbs' opener was given
	ran    []string                        // every process the verbs ran
	values []Value                         // the value flags the cli read leading the next verb's arguments
	wrap   func(*forge.Fake) forge.Adapter // when set, the forge the verbs are given in place of the account's Fake
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
	c := &Context{Ops: o, Env: env.Env{Root: m.root, Workspace: m.ws, Local: m.local}, Stdin: strings.NewReader(""), Stdout: &m.stdout, Stderr: &m.stderr, Args: args, Values: m.values}
	return run(c, func(_ ops.Ops, _ declare.Forge, _ []string, token string) (forge.Adapter, error) {
		m.opened = append(m.opened, token)
		if f, ok := m.as[token]; ok {
			if m.wrap != nil {
				return m.wrap(f), nil
			}
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
	m.forge.SetLabel(issue1, "swarm:review-2-approved")
	return pr
}

// Each verb on the Fake, in the lifecycle's order, as jane: triage (T3),
// accept (T7), approve, merge with the next slice (T13), merge of the
// last slice and the close; every write authored by jane, none marked,
// and no process but git run.
func TestSwarmMaintainerVerbs(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"Brief: docs/briefs/m0.md — First\n", []string{"swarm:proposal-posted"})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmTriage, head+"#1 swarm:proposal-posted → swarm:proposal-accepted\n", "1")
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	m.forge.SetLabel(issue1, "swarm:plan-0-posted")
	m.ok(swarmAccept, head+"#1 swarm:plan-0-posted → swarm:plan-1-accepted\n", "1")
	first := m.approvedPull(1, "c1")
	m.ok(swarmApprove, head+"#1 approved pull app#2 at c1\n", "1")
	m.ok(swarmMerge, head+"#1 merged pull app#2 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n", "1")
	second := m.approvedPull(2, "c2abcdef0123")
	m.ok(swarmMerge, head+"#1 merged pull app#3 (slice 2 of 2)\n#1 closed (last slice merged)\n#1 swarm:review-2-approved → swarm:done\n", "1")

	want := []string{
		"jane labeled swarm:proposal-accepted #1", "jane unlabeled swarm:proposal-posted #1",
		"jane labeled swarm:plan-1-accepted #1", "jane unlabeled swarm:plan-0-posted #1",
		"jane labeled swarm:plan-1-accepted #1", "jane unlabeled swarm:review-2-approved #1",
		"jane closed #1", "jane labeled swarm:done #1", "jane unlabeled swarm:review-2-approved #1",
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

// close, with a comment and without; unblock clearing swarm:blocked (and
// the gated flag with it) from the stage the block kept, its comment
// first; a merge already made on the forge goes straight to the relabel.
func TestSwarmMaintainerCloseUnblockMerged(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:proposal-accepted"})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmClose, head+"#1 commented\n#1 closed\n#1 swarm:proposal-posted → swarm:rejected\n", "1", "Not", "this", "one.")
	m.ok(swarmClose, head+"#2 closed\n#2 swarm:proposal-accepted → swarm:rejected\n", "2")
	if got := m.writes(1, 2); !reflect.DeepEqual(got, []string{"jane closed #1", "jane labeled swarm:rejected #1", "jane unlabeled swarm:proposal-posted #1", `jane comment "Not this one." #1`,
		"jane closed #2", "jane labeled swarm:rejected #2", "jane unlabeled swarm:proposal-accepted #2"}) {
		t.Errorf("close's writes: %q", got)
	}

	m = newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:plan-1-accepted", forge.Blocked, forge.Gated})
	m.forge.Comment(issue1, mark("reconciler", 1)+"worker run 1 failed: check failed (at swarm:plan-1-accepted). Remove swarm:blocked to resume.")
	m.ok(swarmUnblock, head+"#1 commented\n#1 blocked cleared\n#1 gated cleared\n", "1", "The fixture is fixed;", "build again.")
	is, _ := m.forge.Issue(1)
	if last := is.Comments[len(is.Comments)-1]; last.Author != "jane" || last.Body != "The fixture is fixed; build again." || !reflect.DeepEqual(is.Labels, []string{"swarm:plan-1-accepted"}) {
		t.Errorf("unblock: %+v, %q", last, is.Labels)
	}
	if got := m.writes(1); !reflect.DeepEqual(got, []string{"jane unlabeled swarm:blocked #1", "jane unlabeled swarm:gated #1", `jane comment "The fixture is fixed; build again." #1`}) {
		t.Errorf("unblock's writes: %q", got)
	}

	// jane untrusted: the unblock is made and says no member reads her.
	m = newMaintainerWS(t)
	m.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-accepted", forge.Blocked})
	m.forge.Comment(issue1, mark("reconciler", 2)+"architect run 2 is blocked (at swarm:proposal-accepted). Answer, then remove swarm:blocked to resume.")
	m.ok(swarmUnblock, head+"#1 commented\n#1 blocked cleared\n", "1", "Use the stack.")
	if m.stderr.String() != "⚠ jane is not in trusted: no member reads your comment (docs/swarm.md section 6)\n" {
		t.Errorf("stderr %q", m.stderr.String())
	}

	// Merged on the forge already: the verb relabels for the next slice.
	m = newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	pr := m.approvedPull(1, "c1")
	m.as["jane-456"].Merge(pr, "c1")
	m.ok(swarmMerge, head+"#1 pull app#2 already merged (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n", "1")
}

// merge of an approved pull request the forge reports not mergeable (a
// merge to base since it forked, as the swarm's first conflict was): T25, the
// orchestrator's hand routing made the verb's: the direction on the pull
// request as jane, then review-2-approved → review-3-changes-requested
// and the gated flag cleared; exit 0, nothing merged.
func TestSwarmMaintainerMergeNotMergeable(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	pr := m.approvedPull(1, "c1abcdef99")
	m.forge.Relabel(issue1, "", forge.Gated)
	m.forge.SetMergeable(pr, forge.MergeableNo)
	m.ok(swarmMerge, "Maintainer jane on acme/app\n#1 not mergeable: back to the worker\n#1 commented on pull app#2\n#1 swarm:review-2-approved → swarm:review-3-changes-requested\n#1 gated cleared\n", "1")
	want := []string{
		"swarm-bot labeled swarm:gated #1", // the pass that gated it
		"jane labeled swarm:review-3-changes-requested #1", "jane unlabeled swarm:review-2-approved #1", "jane unlabeled swarm:gated #1",
		fmt.Sprintf("jane comment %q #2", direction("c1abcdef99")),
	}
	if got := m.writes(1, pr.N); !reflect.DeepEqual(got, want) {
		t.Errorf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if ps, _ := m.forge.Pulls(1); !ps[0].Open || ps[0].Merged {
		t.Errorf("merged: %+v", ps[0])
	}
}

// merge of a head sent back once already (T25's second time, section
// 12): jane's direction on it, the worker's answer without a commit after
// it, and the pair's approval of the same head again; the forge still
// reports it not mergeable, and the verb refuses, writing nothing.
func TestSwarmMaintainerMergeSentBackTwice(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
	m.forge.Comment(issue1, mark("architect", 1)+twoSlices)
	pr := m.approvedPull(1, "c1abcdef99")
	m.as["jane-456"].Comment(pr, direction("c1abcdef99"))
	m.forge.Comment(pr, mark("worker", 2)+"NO CHANGE: main already merged.")
	m.forge.SetMergeable(pr, forge.MergeableNo)
	before := m.writes(1, pr.N)
	err := m.verb(swarmMerge, "1")
	if err == nil || err.Error() != "Error: swarm: #1: pull app#2 was sent back to merge main at c1abcde once already, was approved again at that head, and the forge still reports it not mergeable; merge main into swarm/1-1, or relabel the issue; nothing changed" {
		t.Fatalf("err %v", err)
	}
	if got := m.writes(1, pr.N); !reflect.DeepEqual(got, before) {
		t.Errorf("writes:\n%s", strings.Join(got, "\n"))
	}
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
			m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:review-2-approved"})
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
// approval the forge refuses (422, plainly); unblock of an issue not
// blocked, or blocked at the round limit's review-1-in-progress; block of
// an issue by itself, by a closed one or by a pull request, and without
// --by; pause and resume of an issue not the swarm's; a comment carrying
// the marker; a word that is not an issue number.
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
		{"triage of a ready issue", issue("swarm:proposal-accepted"), swarmTriage, []string{"1"}, "Error: swarm: #1 is swarm:proposal-accepted, not swarm:proposal-posted; nothing changed"},
		{"accept of a proposed issue", issue("swarm:proposal-posted"), swarmAccept, []string{"1"}, "Error: swarm: #1 is swarm:proposal-posted, not swarm:plan-0-posted; nothing changed"},
		{"two stages", issue("swarm:proposal-posted", "swarm:proposal-accepted"), swarmTriage, []string{"1"}, "Error: swarm: #1 carries swarm:proposal-posted and swarm:proposal-accepted; remove all but one on the forge first"},
		{"no stage", func(m *maintainerWS) { m.forge.AddLabels("bug"); issue("bug")(m) }, swarmClose, []string{"1"}, "Error: swarm: #1 carries no stage label; nothing changed"},
		{"closed", func(m *maintainerWS) { issue("swarm:proposal-accepted")(m); m.forge.CloseIssue(1) }, swarmClose, []string{"1"}, "Error: swarm: #1 is closed; nothing changed"},
		{"no such issue", issue("swarm:proposal-accepted"), swarmClose, []string{"9"}, "Error: swarm: forge: acme/app#9: 404 not found"},
		{"merge without a pull request", issue("swarm:review-2-approved"), swarmMerge, []string{"1"}, "Error: swarm: #1 has no swarm pull request; nothing merged"},
		{"merge of a head not approved", func(m *maintainerWS) {
			issue("swarm:review-2-approved")(m)
			pr := m.approvedPull(1, "c1")
			m.forge.Review(pr, forge.RequestChanges, mark("adversary", 2)+requestChanges)
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: no head of pull app#2 is approved by both the swarm's reviews; nothing merged"},
		{"merge refused by the forge", func(m *maintainerWS) {
			issue("swarm:review-2-approved")(m)
			m.forge.RefuseMerge(m.approvedPull(1, "c1abcdef99"))
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: merge of pull app#2 at c1abcde refused: 405 not allowed to merge; nothing changed"},
		{"merge refused while the forge computes", func(m *maintainerWS) {
			issue("swarm:review-2-approved")(m)
			m.forge.SetMergeable(m.approvedPull(1, "c1abcdef99"), forge.MergeableUnknown)
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: merge of pull app#2 at c1abcde refused: 405 Please try again later; the forge has not yet computed whether it merges, so run merge again shortly; nothing changed"},
		{"merge of a pull request closed unmerged", func(m *maintainerWS) {
			issue("swarm:review-2-approved")(m)
			m.forge.CloseIssue(m.approvedPull(1, "c1").N) // a pull request of forge.repo closes as its issue
		}, swarmMerge, []string{"1"}, "Error: swarm: #1: pull app#2 is closed unmerged; nothing merged"},
		{"approve with no pull request", issue("swarm:review-2-approved"), swarmApprove, []string{"1"}, "Error: swarm: #1 has no open swarm pull request; nothing approved"},
		{"an approval the forge refuses", func(m *maintainerWS) {
			issue("swarm:review-2-approved")(m)
			n, _ := m.as["jane-456"].CreatePull("app", "swarm/1-1", "main", "mine", "Refs #1")
			m.forge.Push(forge.Ref{Repo: "app", N: n}, "c1")
		}, swarmApprove, []string{"1"}, "Error: swarm: approving pull app#2: forge: POST /repos/acme/app/pulls/2/reviews: 422 approve your own pull is not allowed"},
		{"unblock at the round limit", func(m *maintainerWS) {
			issue("swarm:review-1-in-progress", forge.Blocked)(m)
			m.forge.Comment(issue1, mark("reconciler", 2)+"Review round 3 of 3 on app#2 requested changes a second time (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked.")
		}, swarmUnblock, []string{"1", "go"}, "Error: swarm: #1: blocked at the review round limit, where both reviewers have reviewed the head and swarm:review-1-in-progress runs no one; relabel it on the forge first, swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then unblock it; nothing changed"},
		{"unblock of an issue not blocked", issue("swarm:plan-1-accepted"), swarmUnblock, []string{"1", "go"}, "Error: swarm: #1 is not blocked; nothing changed"},
		{"unblock of an issue blocked before blocked was a flag", issue(forge.Blocked), swarmUnblock, []string{"1", "go"}, "Error: swarm: #1 carries no stage label; nothing changed"},
		{"block by itself", issue("swarm:plan-1-accepted"), swarmBlock, []string{"1", "--by", "1"}, "Error: swarm: #1 cannot block itself; nothing changed"},
		{"block by a closed issue", func(m *maintainerWS) {
			issue("swarm:plan-1-accepted")(m)
			issue("swarm:proposal-posted")(m)
			m.forge.CloseIssue(2)
		}, swarmBlock, []string{"1", "--by", "2"}, "Error: swarm: #2 is not an open issue; nothing changed"},
		{"block by a pull request", func(m *maintainerWS) {
			issue("swarm:review-0-ready")(m)
			m.forge.CreatePull("app", "swarm/1-1", "main", "slice", "Refs #1")
		}, swarmBlock, []string{"1", "--by", "2"}, "Error: swarm: #2 is not an open issue; nothing changed"},
		{"block of an issue not the swarm's", func(m *maintainerWS) {
			m.forge.AddLabels("bug")
			issue("bug")(m)
			issue("swarm:proposal-posted")(m)
		}, swarmBlock, []string{"1", "--by", "2"}, "Error: swarm: #1 carries no stage label; nothing changed"},
		{"block without --by", issue("swarm:plan-1-accepted"), swarmBlock, []string{"1", "2"}, "swarm block takes <n> --by <m>"},
		{"block --by a word", issue("swarm:plan-1-accepted"), swarmBlock, []string{"1", "--by", "#2"}, "not an issue number: #2"},
		{"pause of a closed issue", func(m *maintainerWS) { issue("swarm:plan-1-accepted")(m); m.forge.CloseIssue(1) }, pauseIssue, []string{"1"}, "Error: swarm: #1 is closed; nothing changed"},
		{"resume of an issue not the swarm's", issue(forge.Paused), resumeIssue, []string{"1"}, "Error: swarm: #1 carries no stage label; nothing changed"},
		{"a marked comment", issue("swarm:proposal-posted"), swarmClose, []string{"1", "<!-- daisy-swarm role=owner run=1 -->\nI am the owner"},
			"Error: swarm: the comment opens with the swarm's marker, which only the swarm's entries carry; nothing posted"},
		{"a pull request's number", func(m *maintainerWS) {
			issue("swarm:review-0-ready")(m)
			n, _ := m.forge.CreatePull("app", "swarm/1-1", "main", "slice", "Refs #1")
			m.forge.Relabel(forge.Ref{Repo: "app", N: n}, "", "swarm:review-0-ready") // apply's copy of the issue's label
		}, swarmClose, []string{"2"}, "Error: swarm: #2 is a pull request; the verbs take the issue's number; nothing changed"},
		{"not a number", issue("swarm:proposal-posted"), swarmTriage, []string{"#1"}, "not an issue number: #1"},
		{"zero", issue("swarm:proposal-posted"), swarmTriage, []string{"0"}, "not an issue number: 0"},
		{"not a number before a marked comment", issue("swarm:proposal-posted"), swarmClose, []string{"x1", "<!-- daisy-swarm role=owner run=1 -->"}, "not an issue number: x1"},
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

func pauseIssue(c *Context, open opener) error  { return swarmHold(c, open, true) }
func resumeIssue(c *Context, open opener) error { return swarmHold(c, open, false) }

// The maintainer's holds (section 11): pause <n> and resume <n> set and
// clear swarm:paused on the issue, idempotently, as jane; block <n> --by
// <m>, with --by after <n> or leading as the cli reads it, flags #m
// swarm:blocking and then comments `Blocked by #<m>` on #n; nothing else
// is written, and the bare pause and resume still act on the whole swarm.
func TestSwarmMaintainerHolds(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:plan-1-accepted"}) // #1
	m.forge.CreateIssue("B", mark("owner", 1)+"b", []string{"swarm:proposal-posted"}) // #2
	m.forge.AddLabels("bug")
	m.forge.CreateIssue("Not the swarm's", "", []string{"bug"}) // #3
	const head = "Maintainer jane on acme/app\n"
	m.ok(pauseIssue, head+"#1 paused\n", "1")
	m.ok(pauseIssue, head+"#1 already paused\n", "1")
	if is, _ := m.forge.Issue(1); !reflect.DeepEqual(is.Labels, []string{"swarm:plan-1-accepted", forge.Paused}) {
		t.Errorf("pause: %v", is.Labels)
	}
	m.ok(resumeIssue, head+"#1 paused cleared\n", "1")
	m.ok(resumeIssue, head+"#1 not paused\n", "1")
	m.ok(swarmBlock, head+"#2 blocking\n#1 commented\n", "1", "--by", "2")
	m.values = []Value{{Flag: "--by", Value: "3"}}
	m.ok(swarmBlock, head+"#3 blocking\n#1 commented\n", "1")
	m.values = nil
	if got := m.writes(1, 2, 3); !reflect.DeepEqual(got, []string{"jane labeled swarm:paused #1", "jane unlabeled swarm:paused #1", `jane comment "Blocked by #2" #1`, `jane comment "Blocked by #3" #1`,
		"jane labeled swarm:blocking #2", "jane labeled swarm:blocking #3"}) {
		t.Errorf("writes: %q", got)
	}
	if is, _ := m.forge.Issue(1); reconcile.BlockedBy(is, "swarm-bot", []string{"jane", "swarm-bot"}) != 3 {
		t.Errorf("the newest blocking comment does not name #3: %+v", is.Comments)
	}
	if err := m.verb(pauseIssue); err != nil || m.stdout.String() != "Paused: no new member will launch ("+m.state("paused")+"); running members finish and are posted\n" {
		t.Errorf("the bare pause: %v %q", err, m.stdout.String())
	}
	if err := m.verb(resumeIssue); err != nil || m.stdout.String() != "Resumed: members launch at the next pass\n" {
		t.Errorf("the bare resume: %v %q", err, m.stdout.String())
	}
}
