package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/swarm/fence"
	"daisy/internal/swarm/forge"
)

// The comment commands (section 6): a line `/swarm <verb> [<args>]` in a
// comment by a login in trusted other than the swarm's, on an open swarm
// issue or on its current pull request while that is open, is the
// maintainer's verb run on the issue (Swarm.Command), and answered once
// in its thread by the reconciler, the answer's marker naming the
// comment's id, so that a command answered is never run again.

// answerRe is a command's answer's first line, naming the comment it
// answers by its id.
var answerRe = regexp.MustCompile(`^<!-- daisy-swarm role=reconciler kind=command of=([1-9][0-9]*) -->$`)

// commandLines is body's command lines, each without the whitespace at
// its ends: a line outside a fenced block, indented at most three spaces,
// that is `/swarm` alone or followed by a space or a tab.
func commandLines(body string) []string {
	var out []string
	fences := fence.Scan{}
	for _, l := range strings.Split(body, "\n") {
		if fences.Line(l) {
			continue
		}
		t := strings.TrimLeft(l, " ")
		if len(l)-len(t) > 3 {
			continue
		}
		t = strings.TrimRight(t, " \t\r")
		if rest, ok := strings.CutPrefix(t, "/swarm"); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			out = append(out, t)
		}
	}
	return out
}

// IsCommand: body holds a command line (commandLines). A comment that
// does is the maintainer's word to Daisy, never an entry a role answers
// nor a `Blocked by #<m>` (section 6).
func IsCommand(body string) bool { return len(commandLines(body)) > 0 }

// command is a comment holding a command, on the thread at.
type command struct {
	at forge.Ref
	c  forge.Comment
}

// commands is pass step 5's first act on issue is, read with its
// comments, and pulls, its pull requests (section 11): each command on
// the issue thread, and on the current pull request's while it is open,
// by a login in trusted other than the swarm's, oldest first, run and
// answered once; ran says one was. The record of a command run,
// <state>/commands/<id>, is the guard: a command recorded is never run
// again, whatever became of its answer on the forge, which any writer
// of the repository may edit or delete, and an answer that names it no
// more is posted again from the record. An answer naming a command
// that has no record (a <state> lost) keeps it from running too. A
// forge error ends the step, the record kept.
func (s *Swarm) commands(is forge.Issue, pulls []forge.Pull) (ran bool, err error) {
	if s.Command == nil {
		return false, nil
	}
	threads := []command{{at: forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}}}
	comments := [][]forge.Comment{is.Comments}
	if len(pulls) > 0 && pulls[len(pulls)-1].Open {
		cur := pulls[len(pulls)-1]
		at := forge.Ref{Repo: cur.Repo, N: cur.Number}
		cs, err := s.Forge.PullComments(at)
		if err != nil {
			return false, err
		}
		threads, comments = append(threads, command{at: at}), append(comments, cs)
	}
	answered := map[int64]bool{}
	var todo []command
	for i, t := range threads {
		for _, c := range comments[i] {
			if m, ok := Swarms(commentEntry(c), s.SwarmLogin); ok {
				if m.Kind == "command" {
					answered[m.Of] = true
				}
				continue
			}
			if s.maintainer(c.Author) && IsCommand(c.Body) {
				todo = append(todo, command{t.at, c})
			}
		}
	}
	slices.SortStableFunc(todo, func(a, b command) int { return a.c.Created.Compare(b.c.Created) })
	fs := s.O.FS()
	for _, cmd := range todo {
		id := cmd.c.ID
		if id == 0 {
			fmt.Fprintf(s.Err, "⚠ #%d: a command by %s has no comment id on the forge, so its run could not be recorded; not run\n", is.Number, cmd.c.Author)
			continue
		}
		record, err := fs.ReadFile(s.commandRecord(id))
		switch {
		case err != nil && !errors.Is(err, iofs.ErrNotExist):
			return ran, err
		case err == nil && !answered[id]:
			// Run already: its answer was refused, edited or deleted.
			if err := s.Forge.Comment(cmd.at, string(record)); err != nil {
				return ran, err
			}
			fmt.Fprintf(s.Out, "#%d comment %d: answer posted again\n", is.Number, id)
			continue
		case err == nil || answered[id]:
			continue
		}
		if err := s.command(is.Number, cmd); err != nil {
			return ran, err
		}
		ran = true
	}
	return ran, nil
}

// commandRecord is the record of the command in comment id once run:
// <state>/commands/<id>, holding its answer.
func (s *Swarm) commandRecord(id int64) string {
	return filepath.Join(s.state(), "commands", strconv.FormatInt(id, 10))
}

// maintainer: login is in trusted and is not the swarm's.
func (s *Swarm) maintainer(login string) bool {
	return !strings.EqualFold(login, s.SwarmLogin) && slices.ContainsFunc(s.Cfg.Trusted, func(l string) bool { return strings.EqualFold(l, login) })
}

// command runs cmd on issue n, unless its comment holds more than one
// command or was edited after it was posted (an edit can be made by
// another login than its author), then records it, posts the answer on
// its thread, prints the outcome and queues its journal line. The
// record is written before the verb runs, holding an answer that says
// the outcome is not known, so an apply stopped during the verb never
// runs it a second time; and again with the answer once it returned.
func (s *Swarm) command(n int, cmd command) error {
	lines := commandLines(cmd.c.Body)
	answer := func(outcome string) []byte {
		return fmt.Appendf(nil, "<!-- daisy-swarm role=reconciler kind=command of=%d -->\n**reconciler:**\n\n> %s\n\n%s", cmd.c.ID, strings.Join(lines, "\n> "), outcome)
	}
	record := s.commandRecord(cmd.c.ID)
	fs := s.O.FS()
	if err := fs.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		return err
	}
	var outcome, word string
	switch {
	case len(lines) > 1:
		word = "not run (more than one command)"
		outcome = fmt.Sprintf("Not run: the comment holds %d commands; post each in a comment of its own.\n", len(lines))
	case cmd.c.Updated.After(cmd.c.Created):
		word = "not run (edited)"
		outcome = "Not run: the comment was edited after it was posted, and a command runs only as it was posted; post it again in a new comment.\n"
	default:
		if err := fs.WriteFile(record, answer(notKnown), 0o644); err != nil {
			return err
		}
		out, err := s.Command(n, strings.Fields(lines[0])[1:])
		head := "Done:"
		word = "done"
		if err != nil {
			first, _, _ := strings.Cut(err.Error(), "\n")
			word, head = "refused ("+first+")", "Refused:"
			out = strings.TrimRight(out, "\n") + "\n" + err.Error()
		}
		// A code block, which shows the verb's lines as it printed them.
		outcome = head + "\n\n" + indent(strings.Trim(out, "\n"), "    ") + "\n"
	}
	body := answer(outcome)
	if err := fs.WriteFile(record, body, 0o644); err != nil {
		return err
	}
	what := fmt.Sprintf("%s by %s", lines[0], cmd.c.Author)
	fmt.Fprintf(s.Out, "#%d %s (comment %d): %s\n", n, what, cmd.c.ID, word)
	now := s.O.Now().UTC()
	if err := s.queue(now.Format("2006-01-02"), fmt.Sprintf("- %s %d-command-%d: %s; %s", now.Format("1504"), n, cmd.c.ID, what, word)); err != nil {
		return err
	}
	return s.Forge.Comment(cmd.at, string(body))
}

// notKnown is the outcome a command's record holds while its verb runs:
// the answer posted when apply stopped before the verb returned.
const notKnown = "Not known: apply stopped while the verb ran, so its outcome was not recorded; read the issue's labels and timeline before posting the command again.\n"
