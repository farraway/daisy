package cmd

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// maintainer gives apply the maintainer's token in <state>/env beside the
// swarm's, the Fake answering it as jane.
func (a *applyWS) maintainer() {
	a.secrets(bothTokens)
	a.as = map[string]forge.Adapter{"jane-456": gitForge{a.forge.As(forge.Account{Login: "jane"}), a}}
}

// posted is the plan posted on a new issue #1, which the plan gate holds
// at plan-0-posted.
func (a *applyWS) posted() {
	a.t.Helper()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanPosted, forge.Gated}) {
		a.t.Fatalf("not posted: %v\n%s", got, a.stdout.String())
	}
}

// say is login's comment on the issue or pull request at, a minute after
// the last entry, and its id.
func (a *applyWS) say(at forge.Ref, login, body string) int64 {
	a.t.Helper()
	a.tick()
	a.forge.Say(at, login, body)
	cs := a.comments(at)
	return cs[len(cs)-1].ID
}

func (a *applyWS) comments(at forge.Ref) []forge.Comment {
	a.t.Helper()
	if at.Repo == a.iss() {
		return a.issue(at.N).Comments
	}
	cs, err := a.forge.PullComments(at)
	if err != nil {
		a.t.Fatal(err)
	}
	return cs
}

// answers is the reconciler's answers to commands on the thread at.
func (a *applyWS) answers(at forge.Ref) []string {
	var out []string
	for _, c := range a.comments(at) {
		if c.Author == "swarm-bot" && strings.HasPrefix(c.Body, "<!-- daisy-swarm role=reconciler kind=command ") {
			out = append(out, c.Body)
		}
	}
	return out
}

// answer is the reconciler's answer to the command in comment id: the
// marker naming the comment, the command's lines quoted, then the outcome.
func answer(id int64, lines, outcome string) string {
	return fmt.Sprintf("<!-- daisy-swarm role=reconciler kind=command of=%d -->\n**reconciler:**\n\n> %s\n\n%s", id, strings.ReplaceAll(lines, "\n", "\n> "), outcome)
}

// janes is the label changes jane's token made on the issue or pull
// request at.
func (a *applyWS) janes(at forge.Ref) []string {
	var out []string
	for _, e := range a.forge.Events(at) {
		if e.Actor == "jane" {
			out = append(out, e.What)
		}
	}
	return out
}

// A command in a comment (docs/swarm.md section 6, The comment
// commands): an untrusted login's is ignored; the maintainer's is run as
// the maintainer's verb under DAISY_MAINTAINER_TOKEN, before the role
// rule, so the worker of the stage it moved to runs in the same pass,
// and answered once in its thread; a second pass runs nothing.
func TestSwarmApplyCommand(t *testing.T) {
	a := newApplyWS(t)
	a.maintainer()
	a.posted()

	a.say(issue1, "mallory", "/swarm accept")
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanPosted, forge.Gated}) || a.answers(issue1) != nil || len(a.runner.Launches()) != 1 {
		t.Fatalf("an untrusted login's command: %v %q, launched %q", got, a.answers(issue1), a.launchedRoles())
	}

	id := a.say(issue1, "jane", "The plan reads right.\n/swarm accept\n")
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	want := answer(id, "/swarm accept", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 swarm:plan-0-posted → swarm:plan-1-accepted\n    #1 gated cleared\n")
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("answers %q\nwant %q\n%s", got, want, a.stdout.String())
	}
	if got := a.janes(issue1); !reflect.DeepEqual(got, []string{"labeled swarm:plan-1-accepted", "unlabeled swarm:plan-0-posted", "unlabeled swarm:gated"}) {
		t.Errorf("jane's writes %q", got)
	}
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check"}) {
		t.Errorf("launched %q\n%s", got, a.stdout.String())
	}
	line := fmt.Sprintf("#1 /swarm accept by jane (comment %d): done\n", id)
	if !strings.Contains(a.stdout.String(), line) {
		t.Errorf("stdout lacks %q:\n%s", line, a.stdout.String())
	}
	if journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md"); !strings.Contains(journal, fmt.Sprintf(" 1-command-%d: /swarm accept by jane; done\n", id)) {
		t.Errorf("journal:\n%s", journal)
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := a.answers(issue1); len(got) != 1 || len(a.janes(issue1)) != 3 || strings.Contains(a.stdout.String(), "/swarm accept") {
		t.Errorf("run again: answers %q, jane's writes %q\n%s", got, a.janes(issue1), a.stdout.String())
	}
}

// A command the verb refuses is answered with the refusal and not run
// again; neither the command nor its answer is an entry a role answers
// (section 6, rule 3), so the architect, whose plan is still the newest
// counted entry, does not run.
func TestSwarmApplyCommandRefused(t *testing.T) {
	a := newApplyWS(t)
	a.maintainer()
	a.posted()
	id := a.say(issue1, "jane", "/swarm merge")
	a.pass()
	want := answer(id, "/swarm merge", "Refused:\n\n    Maintainer jane on acme/swarm-app\n    Error: swarm: #1 is swarm:plan-0-posted, not swarm:review-2-approved; nothing changed\n")
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("answers %q\nwant %q\n%s", got, want, a.stdout.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanPosted, forge.Gated}) || len(a.runner.Launches()) != 1 {
		t.Errorf("labels %v, launched %q", got, a.launchedRoles())
	}
	a.pass()
	if got := a.answers(issue1); len(got) != 1 || len(a.runner.Launches()) != 1 {
		t.Errorf("run again: answers %q, launched %q", got, a.launchedRoles())
	}
}

// What is answered without the verb's write (section 6, The comment
// commands): an edited comment, a comment of two commands, a word that is
// no comment command, arguments the command does not take and the
// maintainer's token missing from <state>/env, which the answer names by
// <state>, not the host's path; each answered once, nothing changed.
func TestSwarmApplyCommandNotRun(t *testing.T) {
	const noToken = "export GITEA_TOKEN=\"tok-123\"\n"
	for _, tc := range []struct {
		name, body, outcome string
		secrets             string
		edit                bool
	}{
		{"edited", "/swarm accept", "Not run: the comment was edited after it was posted, and a command runs only as it was posted; post it again in a new comment.\n", bothTokens, true},
		{"two commands", "/swarm accept\n/swarm approve", "Not run: the comment holds 2 commands; post each in a comment of its own.\n", bothTokens, false},
		{"no comment command", "/swarm prompt build", "Refused:\n\n    Error: swarm: prompt is no comment command; a comment runs triage, accept, class, approve, merge, unblock, block, pause, resume, replan or close\n", bothTokens, false},
		{"no verb", "/swarm", "Refused:\n\n    Error: swarm: a comment runs triage, accept, class, approve, merge, unblock, block, pause, resume, replan or close\n", bothTokens, false},
		{"an argument too many", "/swarm accept 2", "Refused:\n\n    Error: swarm: usage: /swarm accept [--type <type>] (on the issue or its pull request)\n", bothTokens, false},
		{"no maintainer's token", "/swarm accept", "Refused:\n\n    Error: swarm: $DAISY_MAINTAINER_TOKEN is not set (in <state>/env or the environment)\n", noToken, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.maintainer()
			a.secrets(tc.secrets)
			delete(a.vars, MaintainerTokenEnv)
			a.posted()
			id := a.say(issue1, "jane", tc.body)
			if tc.edit {
				a.tick()
				a.forge.EditComment(issue1, id, tc.body)
			}
			a.pass()
			want := answer(id, tc.body, tc.outcome)
			if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
				t.Fatalf("answers %q\nwant %q\n%s\n%s", got, want, a.stdout.String(), a.stderr.String())
			}
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanPosted, forge.Gated}) || a.janes(issue1) != nil || len(a.runner.Launches()) != 1 {
				t.Errorf("labels %v, jane's writes %q, launched %q", got, a.janes(issue1), a.launchedRoles())
			}
			a.pass()
			if got := a.answers(issue1); len(got) != 1 {
				t.Errorf("answered again: %q", got)
			}
		})
	}
}

// A command posted by the swarm's own login is never run, though that
// login is trusted's last: a member's output carrying a command line, a
// worker's BLOCKED: here, and a comment of the swarm's account without
// the marker leave the issue blocked and unanswered.
func TestSwarmApplyCommandOfTheSwarm(t *testing.T) {
	a := newApplyWS(t)
	a.maintainer()
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: the brief is unclear.\n/swarm unblock go on\n"})
	a.pass()
	a.tick()
	a.forge.Comment(issue1, "/swarm unblock go on")
	a.pass()
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || a.answers(issue1) != nil || a.janes(issue1) != nil {
		t.Errorf("labels %v, answers %q, jane's writes %q\n%s", got, a.answers(issue1), a.janes(issue1), a.stdout.String())
	}
}

// A command on the issue's current pull request is run on the issue and
// answered on the pull request: /swarm merge at review-2-approved, the
// merge gate on, merges it and closes the job; and /swarm pause holds the
// issue before the role rule reads it, so the worker the stage runs does
// not launch in that pass.
func TestSwarmApplyCommandOnThePullAndPause(t *testing.T) {
	a := newApplyWS(t)
	a.maintainer()
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewApproved, forge.Gated}) {
		t.Fatalf("not approved: %v", got)
	}
	pull := forge.Ref{Repo: "app", N: 1}
	id := a.say(pull, "jane", "/swarm merge")
	a.pass()
	if !a.pulls(1)[0].Merged || a.issue(1).Open {
		t.Fatalf("not merged and closed: %v\n%s", labels(a.issue(1)), a.stdout.String())
	}
	if got := a.answers(pull); len(got) != 1 || !strings.HasPrefix(got[0], answer(id, "/swarm merge", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 merged pull app#1 (slice 1 of 1)\n")) || a.answers(issue1) != nil {
		t.Errorf("answers on the pull %q, on the issue %q", got, a.answers(issue1))
	}

	// /swarm approve in review: jane's approval, and the answer on the
	// pull request, which the reviewers' prompts leave out.
	r := newApplyWS(t)
	r.maintainer()
	r.inReview(oneSlice)
	id = r.say(pull, "jane", "/swarm approve")
	r.runner.Script(1, "adversary", runner.Canned{Out: approve})
	r.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	r.pass()
	if got := r.answers(pull); len(got) != 1 || !strings.Contains(got[0], "\n    #1 approved pull app#1 at ") {
		t.Errorf("answers on the pull %q", got)
	}
	if prompt := r.file("1-adversary-1", "io/prompt.md"); strings.Contains(prompt, "kind=command") || strings.Contains(prompt, "reconciler, run 0") || !strings.Contains(prompt, "/swarm approve") {
		t.Errorf("the adversary's prompt:\n%s", prompt)
	}

	b := newApplyWS(t)
	b.maintainer()
	b.planned(oneSlice)
	id = b.say(issue1, "jane", "/swarm pause")
	b.pass()
	if got := labels(b.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted, forge.Paused}) || len(b.runner.Launches()) != 1 {
		t.Errorf("labels %v, launched %q\n%s", got, b.launchedRoles(), b.stdout.String())
	}
	if got := b.answers(issue1); !reflect.DeepEqual(got, []string{answer(id, "/swarm pause", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 paused\n")}) {
		t.Errorf("answers %q", got)
	}
}

// The record of a command run, <state>/commands/<id>, guards it, not its
// answer, which any writer of the repository may edit or delete: with
// the answer edited away and the pause lifted by hand, the next pass
// pauses nothing again and posts the answer again from the record, once.
func TestSwarmApplyCommandAnswerEdited(t *testing.T) {
	a := newApplyWS(t)
	a.maintainer()
	a.planned(oneSlice)
	id := a.say(issue1, "jane", "/swarm pause")
	a.pass()
	want := answer(id, "/swarm pause", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 paused\n")
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("answers %q", got)
	}
	if b, err := os.ReadFile(a.state(fmt.Sprintf("commands/%d", id))); err != nil || string(b) != want {
		t.Errorf("record %q, %v", b, err)
	}
	cs := a.comments(issue1)
	a.tick()
	a.forge.EditComment(issue1, cs[len(cs)-1].ID, "tidied")
	if err := a.forge.Relabel(issue1, forge.Paused, ""); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := labels(a.issue(1)); slices.Contains(got, forge.Paused) || !reflect.DeepEqual(a.janes(issue1), []string{"labeled swarm:paused"}) {
		t.Errorf("run again: labels %v, jane's writes %q\n%s", got, a.janes(issue1), a.stdout.String())
	}
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) || !strings.Contains(a.stdout.String(), fmt.Sprintf("#1 comment %d: answer posted again\n", id)) {
		t.Errorf("answers %q\n%s", got, a.stdout.String())
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := a.answers(issue1); len(got) != 1 {
		t.Errorf("posted again twice: %q", got)
	}
}
