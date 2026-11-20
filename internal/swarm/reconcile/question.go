package reconcile

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
)

// question begins a member's question (section 7.5): a worker's or a
// reviewer's, or on swarm:question the architect's or the owner's that
// passes it on; answer begins the architect's or the owner's answer to
// one, which Daisy posts so marked.
const (
	question = "QUESTION:"
	answer   = "ANSWER:"
)

// asks: role's output, whose first line is first, is a question to pass
// up (section 6, The question): any worker's or reviewer's, and the
// architect's or the owner's on a question run.
func asks(role, first string, questionRun bool) bool {
	if !strings.HasPrefix(strings.TrimRight(first, "\r "), question) {
		return false
	}
	return role == "worker" || role == "adversary" || role == "sympathetic" || questionRun && (role == "architect" || role == "owner")
}

// onThePlan: an architect's text is its word on the plan, the plan or a
// STANDS: confirmation, not a BLOCKED:, a question or an answer.
func onThePlan(text string) bool {
	return !strings.HasPrefix(text, "BLOCKED:") && !strings.HasPrefix(text, question) && !strings.HasPrefix(text, answer)
}

// Answerer is the role that answers the question of an issue carrying
// swarm:question (section 6, The question), derived from its thread: the
// owner when the newest of the swarm's questions on it is the
// architect's, the architect otherwise.
func Answerer(is forge.Issue, swarmLogin string) string {
	if e, ok := newestQuestion(is, swarmLogin); ok {
		if m, _ := Swarms(e, swarmLogin); m.Role == "architect" {
			return "owner"
		}
	}
	return "architect"
}

// newestQuestion is the newest of the swarm's issue comments whose text
// begins QUESTION:.
func newestQuestion(is forge.Issue, swarmLogin string) (Entry, bool) {
	var newest Entry
	found := false
	for _, c := range is.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		if _, ok := Swarms(e, swarmLogin); ok && strings.HasPrefix(swarmText(c.Body), question) && (!found || !c.Created.Before(newest.Created)) {
			newest, found = e, true
		}
	}
	return newest, found
}

// questionNote is the reconciler's note on the owner's question passed to
// the maintainer, from its start (toMaintainer).
var questionNote = regexp.MustCompile(`^owner run [0-9]+ passed a question to the maintainer \(at \S+\): `)

// AtAQuestion: the newest of the reconciler's notes that blocked is
// (blockingNote) passed a question to the maintainer, which `status`
// lists as a question (section 11).
func AtAQuestion(is forge.Issue, swarmLogin string) bool {
	e, ok := blockingNote(is, swarmLogin)
	return ok && questionNote.MatchString(swarmText(e.Body))
}

// questions is the worker's and the reviewers' # Questions (section
// 7.3): from the first of the swarm's questions in the issue thread on,
// the swarm's questions and answers and the maintainer's comments; ""
// when the thread holds no question.
func (j *Job) questions() string {
	var out []string
	asked := false
	for _, c := range j.Issue.Comments {
		e := Entry{c.Author, c.Body, c.Created}
		m, swarm := Swarms(e, j.SwarmLogin)
		text := swarmText(c.Body)
		qa := swarm && m.Kind == "" && (strings.HasPrefix(text, question) || strings.HasPrefix(text, answer))
		asked = asked || qa && strings.HasPrefix(text, question)
		if asked && (qa || !swarm && counted(e, j.SwarmLogin, j.Cfg.Trusted)) {
			out = append(out, trim(j.entry(2, e, "")))
		}
	}
	if len(out) == 0 {
		return ""
	}
	return "# Questions\n\n" + strings.Join(out, "\n\n")
}

// ask is a question posted (section 6, The question): on the issue as
// the role's text, where the architect and the owner read it, a worker's
// commits kept, the issue taken home (T22), then swarm:question set
// beside the stage (T28; the architect's or the owner's run on it finds it
// set).
func (c *collection) ask(text string) error {
	if err := c.comment(c.r.Role, text); err != nil {
		return err
	}
	if _, err := c.keptNote(); err != nil {
		return err
	}
	if err := c.restore(); err != nil {
		return err
	}
	if err := c.flag(forge.Question, true); err != nil {
		return err
	}
	return c.end("question")
}

// answer posts the architect's or the owner's answer to the issue's
// question, marked ANSWER: when its text does not begin so, then clears
// swarm:question (T29): the asker's stage resumes, its role reading the
// answer as the newest counted entry.
func (c *collection) answer(out string) error {
	if err := c.comment(c.r.Role, answerText(out)); err != nil {
		return err
	}
	is, err := c.s.Forge.Issue(c.r.N)
	if err != nil {
		return err
	}
	if is.Open {
		if err := Unflag(c.s.Forge, c.at, is.Labels, forge.Question, c.s.Out); err != nil {
			return err
		}
	}
	return c.end("posted")
}

// answerText is an answer as Daisy posts it: marked ANSWER: when its
// text does not begin so.
func answerText(out string) string {
	if !strings.HasPrefix(out, answer) {
		return answer + " " + out
	}
	return out
}

// toMaintainer is the owner's question passed to the maintainer (T30): its
// text posted, then the reconciler's note naming the question and T17,
// which clears swarm:question beside swarm:blocked (block).
func (c *collection) toMaintainer(text, first string) error {
	if err := c.comment(c.r.Role, text); err != nil {
		return err
	}
	q := strings.TrimSpace(strings.TrimPrefix(strings.TrimRight(first, "\r "), question))
	return c.note(fmt.Sprintf("%s run %d passed a question to the maintainer (at %s): %s", c.r.Role, c.r.K, c.home(), q), "Answer, then remove "+forge.Blocked+" to resume.", "", "blocked")
}

// unquestion clears swarm:question from an issue just blocked (T17, T30):
// a blocked question is the maintainer's, whose unblock resumes the
// asker's stage.
func (c *collection) unquestion() error {
	if !slices.Contains(c.is.Labels, forge.Blocked) {
		return nil
	}
	return Unflag(c.s.Forge, c.at, c.is.Labels, forge.Question, c.s.Out)
}
