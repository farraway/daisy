package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// maxPost is the longest text posted (section 7.5).
const maxPost = 60000

// collection is one run's collection (section 7.6): each step first
// checked on the forge and skipped when done there, each step done
// appended to the result, so a collection interrupted by an error is
// retried whole at the next pass without posting twice.
type collection struct {
	s     *Swarm
	r     Run
	lines []string
	head  string // the run's prefix on stdout
	label string // the stage the run was launched on
	is    forge.Issue
	at    forge.Ref // the issue
}

// collect posts what an ended run produced and moves the label (section
// 7.6); a worker run whose check is due starts it and is collected once
// the check has ended.
func (s *Swarm) collect(r Run) error {
	lines, err := r.Result(s.O.FS())
	if err != nil {
		return err
	}
	if _, done := Has(lines, "collected"); done || Ended(lines) == "" {
		return nil
	}
	if _, pending := checkPending(lines); pending {
		return nil
	}
	labelLine, _ := Has(lines, "label")
	c := &collection{s: s, r: r, lines: lines, head: runHead(r), label: strings.TrimPrefix(labelLine, "label "),
		at: forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: r.N}}
	if r.N == 0 {
		return c.batch()
	}
	worker := r.Role == "worker" && Ended(lines) != "launch-failed"
	if worker {
		if err := c.fetch(); err != nil {
			return err
		}
	}
	if l, ok := Has(c.lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", c.head, strings.TrimPrefix(l, "fault: "))
		if worker {
			if err := c.keep(); err != nil {
				return err
			}
		}
		return c.end("fault")
	}
	if c.is, err = s.Forge.Issue(r.N); err != nil {
		return err
	}
	if !c.is.Open {
		fmt.Fprintf(s.Err, "⚠ #%d closed during the run; nothing posted\n", r.N)
		return c.end("discarded")
	}
	var pull *forge.Pull
	if r.Role == "worker" || r.Role == "adversary" || r.Role == "sympathetic" {
		pulls, err := s.Forge.Pulls(r.N)
		if err != nil {
			return err
		}
		pull = c.runPull(pulls)
		onPull := r.Role != "worker" || c.label == "swarm:reviewed"
		if pull != nil && !pull.Open || pull == nil && onPull && Ended(lines) != "launch-failed" {
			return c.end("discarded")
		}
		if start, _ := Has(lines, "start"); !worker && pull != nil && "start "+pull.HeadSHA != start {
			if err := c.step("stale " + pull.HeadSHA); err != nil {
				return err
			}
			return c.end("stale")
		}
	}
	if l, ok := Has(c.lines, "failed"); ok {
		return c.fail(strings.TrimPrefix(l, "failed: "))
	}
	out, err := s.O.FS().ReadFile(filepath.Join(r.Dir, "io", "out.md"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return c.fail("no output")
	}
	if hit(out, s.secretsOf(r, c.lines)) {
		return c.fail("secret in output: out.md")
	}
	text := postable(string(out), r)
	first, _, _ := strings.Cut(string(out), "\n")
	if strings.HasPrefix(strings.TrimRight(first, "\r "), "BLOCKED:") {
		return c.blocked(text, pull)
	}
	switch r.Role {
	case "owner":
		if err := c.comment(r.Role, text); err != nil {
			return err
		}
		if c.label == "swarm:proposed" && !s.Cfg.Gates.Triage {
			if err := c.relabel(c.label, "swarm:ready"); err != nil {
				return err
			}
		}
	case "architect":
		if _, bad := SliceRepos(string(out), repoNames(s)); len(bad) > 0 {
			return c.fail("bad slices: " + numbers(bad))
		}
		if err := c.comment(r.Role, text); err != nil {
			return err
		}
		if c.label == "swarm:ready" {
			to := "swarm:planning"
			if !s.Cfg.Gates.Plan {
				to = "swarm:planned"
			}
			if err := c.relabel(c.label, to); err != nil {
				return err
			}
		}
	case "worker":
		return c.worker(text, pull)
	default:
		return c.review(string(out), text, *pull)
	}
	return c.end("posted")
}

func (c *collection) step(line string) error {
	if err := c.r.Append(c.s.O.FS(), line); err != nil {
		return err
	}
	c.lines = append(c.lines, line)
	return nil
}

// end records the outcome and `collected`, and removes the run's clones.
func (c *collection) end(outcome string) error {
	fs := c.s.O.FS()
	if err := c.r.Append(fs, "outcome "+outcome, "collected"); err != nil {
		return err
	}
	c.s.collected++
	c.s.newFault = c.s.newFault || outcome == "fault"
	delete(c.s.given, c.r.Dir)
	return errors.Join(fs.RemoveAll(filepath.Join(c.r.Dir, "work")), fs.RemoveAll(filepath.Join(c.r.Dir, "check")))
}

// say posts text marked for role and this run at at, unless existing (its
// comments) holds it: a reconciler note is told by its whole body, since
// its marker carries the reported run's number.
func (c *collection) say(at forge.Ref, existing []forge.Comment, role, text string) error {
	body := marker(role, c.r.K) + text
	for _, e := range existing {
		m, ok := Swarms(Entry{e.Author, e.Body, e.Created}, c.s.SwarmLogin)
		if ok && m.Role == role && m.Run == c.r.K && (role != "reconciler" || e.Body == body) {
			return nil
		}
	}
	if err := c.s.Forge.Comment(at, body); err != nil {
		return err
	}
	fmt.Fprintln(c.s.Out, c.head+"posted comment")
	return c.step("posted comment")
}

// comment posts on the issue.
func (c *collection) comment(role, text string) error { return c.say(c.at, c.is.Comments, role, text) }

// relabel moves the issue from one stage to another; a `from` the issue no
// longer carries is skipped with a warning, and a move already made is
// skipped.
func (c *collection) relabel(from, to string) error {
	switch {
	case slices.Contains(c.is.Labels, from):
		if err := c.s.Forge.Relabel(c.at, from, to); err != nil {
			return err
		}
		fmt.Fprintf(c.s.Out, "%s%s → %s\n", c.head, from, to)
		return c.step("relabelled " + from + " " + to)
	case slices.Contains(c.is.Labels, to):
		return nil
	}
	now := ""
	for _, l := range c.is.Labels {
		if strings.HasPrefix(l, "swarm:") {
			now = l
		}
	}
	fmt.Fprintf(c.s.Err, "⚠ #%d: relabelled to %s during the run; not relabelled\n", c.r.N, now)
	return nil
}

// note posts the reconciler's note on the issue, then T17.
func (c *collection) note(text, outcome string) error {
	if err := c.comment("reconciler", text); err != nil {
		return err
	}
	if err := c.relabel(c.label, "swarm:blocked"); err != nil {
		return err
	}
	return c.end(outcome)
}

// fail is a failure of the work (section 7.7): the reason recorded, a
// worker's commits kept, the note and T17.
func (c *collection) fail(reason string) error {
	if _, ok := Has(c.lines, "failed"); !ok {
		if err := c.step("failed: " + reason); err != nil {
			return err
		}
	}
	fmt.Fprintf(c.s.Out, "%sfailed (%s)\n", c.head, reason)
	note := fmt.Sprintf("%s run %d failed: %s (was %s). Relabel the issue to resume.", c.r.Role, c.r.K, reason, c.label)
	kept, err := c.keptNote()
	if err != nil {
		return err
	}
	return c.note(note+kept, "failed")
}

// blocked posts the member's BLOCKED: text, on the pull request for the
// worker in reviewed and the reviewers, else on the issue; then the note
// and T17.
func (c *collection) blocked(text string, pull *forge.Pull) error {
	onPull := c.r.Role == "adversary" || c.r.Role == "sympathetic" || c.r.Role == "worker" && c.label == "swarm:reviewed"
	if onPull && pull != nil {
		ref := forge.Ref{Repo: pull.Repo, N: pull.Number}
		existing, err := c.s.Forge.PullComments(ref)
		if err != nil {
			return err
		}
		if err := c.say(ref, existing, c.r.Role, text); err != nil {
			return err
		}
	} else if err := c.comment(c.r.Role, text); err != nil {
		return err
	}
	kept, err := c.keptNote()
	if err != nil {
		return err
	}
	return c.note(fmt.Sprintf("%s run %d is blocked (was %s). Answer and relabel the issue to resume.", c.r.Role, c.r.K, c.label)+kept, "blocked")
}

// keptNote keeps a worker run's commits (section 7.6, step 2) and is the
// note's sentence naming their ref; "" when there are none.
func (c *collection) keptNote() (string, error) {
	if c.r.Role != "worker" || Ended(c.lines) == "launch-failed" {
		return "", nil
	}
	if err := c.keep(); err != nil {
		return "", err
	}
	if !c.commits() {
		return "", nil
	}
	return " Its commits are kept in the swarm's mirror as " + runRef(c.r) + ".", nil
}

// runPull is the pull request the run works on: the worker's branch's,
// a reviewer's the issue's current one; nil when there is none.
func (c *collection) runPull(pulls []forge.Pull) *forge.Pull {
	if c.r.Role != "worker" {
		if len(pulls) == 0 {
			return nil
		}
		return &pulls[len(pulls)-1]
	}
	branch := runBranch(c.lines)
	for i := len(pulls) - 1; i >= 0; i-- {
		if pulls[i].Head == branch {
			return &pulls[i]
		}
	}
	return nil
}

func hit(data []byte, secrets []string) bool {
	return slices.ContainsFunc(secrets, func(sec string) bool { return sec != "" && bytes.Contains(data, []byte(sec)) })
}

// secretsOf is what the run's member was given (section 7.6): the env
// value, or the slot's long strings read at the launch and the return; a
// run this process did not launch reads them as they are now.
func (s *Swarm) secretsOf(r Run, lines []string) []string {
	if given, ok := s.given[r.Dir]; ok {
		return given
	}
	l, _ := Has(lines, "credential")
	f := strings.Fields(l)
	switch {
	case len(f) == 4 && f[2] == "env":
		return []string{s.Secret(f[3])}
	case len(f) == 4 && f[2] == "slot":
		sec, _ := runner.SlotSecrets(s.O.FS(), filepath.Join(s.slotDir(f[1]), f[3]))
		return sec
	}
	return nil
}

func repoName(repo string) string { _, r, _ := strings.Cut(repo, "/"); return r }

func repoNames(s *Swarm) []string {
	var out []string
	for _, r := range s.Cfg.Repos {
		out = append(out, r.Name)
	}
	return out
}

// postable is the output as posted: cut at the last line break before
// maxPost bytes, with the note naming the run (section 7.5).
func postable(out string, r Run) string {
	if len(out) <= maxPost {
		return out
	}
	cut := strings.LastIndex(out[:maxPost], "\n")
	if cut < 0 {
		cut = maxPost
	}
	return out[:cut] + "\n[truncated by Daisy; the whole output is in runs/" + filepath.Base(r.Dir) + "]"
}
