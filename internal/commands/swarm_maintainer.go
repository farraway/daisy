package commands

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
)

// MaintainerTokenEnv is the variable the maintainer's verbs take their
// token from (docs/swarm.md section 11): the maintainer's own, never the
// swarm's. `apply`, `status` and `init` never read it, and swarm.json's
// forge.token_env may not name it.
const MaintainerTokenEnv = declare.MaintainerTokenEnv

// SwarmTriage is `daisy swarm triage <n>`: G1, T3 (proposal-posted →
// proposal-accepted).
func SwarmTriage(c *Context) error { return swarmTriage(c, openForge) }

// SwarmAccept is `daisy swarm accept <n>`: G2, T7 (plan-0-posted →
// plan-1-accepted).
func SwarmAccept(c *Context) error { return swarmAccept(c, openForge) }

// SwarmClass is `daisy swarm class <n> routine|architecture`: the job's
// class (section 5), swarm:class-architecture set or cleared, then the
// statement `Class: <class>` posted as the maintainer.
func SwarmClass(c *Context) error { return swarmClass(c, openForge) }

// SwarmApprove is `daisy swarm approve <n>`: a formal approving review of
// the issue's current open pull request at its head (section 12).
func SwarmApprove(c *Context) error { return swarmApprove(c, openForge) }

// SwarmMerge is `daisy swarm merge <n>`: G3, the approved head merged,
// then T13 or the last slice's close (section 12).
func SwarmMerge(c *Context) error { return swarmMerge(c, openForge) }

// SwarmUnblock is `daisy swarm unblock <n> <comment...>`: G4, T18, the
// comment then swarm:blocked cleared.
func SwarmUnblock(c *Context) error { return swarmUnblock(c, openForge) }

// SwarmBlock is `daisy swarm block <n> --by <m>`: #m flagged
// swarm:blocking, then `Blocked by #<m>` on #n, which holds #n's job while
// #m is open (section 6, rule 1).
func SwarmBlock(c *Context) error { return swarmBlock(c, openForge) }

// SwarmClose is `daisy swarm close <n> [comment...]`: T19.
func SwarmClose(c *Context) error { return swarmClose(c, openForge) }

// maintainer is the forge as one of the maintainer's verbs reaches it:
// under the maintainer's token, with the maintainer's login and the
// swarm's, for issue n.
type maintainer struct {
	c      *Context
	cfg    declare.Config
	f      forge.Adapter
	login  string // the maintainer's, as the forge reports the token's
	swarm  string // the swarm's, trusted's last
	n      int
	at     forge.Ref
	is     forge.Issue // issue n with its comments, as issue read it
	labels []string    // issue n's, as issue read them and the verb's writes left them
}

// issueArg is the verb's <n>, a positive issue number, else usage: checked
// first, before the comment.
func issueArg(c *Context) (int, error) {
	n, err := strconv.Atoi(c.Args[0])
	if err != nil || n < 1 || strconv.Itoa(n) != c.Args[0] {
		return 0, UsageError("not an issue number: " + c.Args[0])
	}
	return n, nil
}

// asMaintainer is every maintainer's verb's start (section 11): the issue
// number, swarm.json validated, $DAISY_MAINTAINER_TOKEN set and not the
// swarm's token, then its account read and refused when it is the swarm's,
// all before any write. The swarm's token is read only to be compared.
func asMaintainer(c *Context, open opener) (*maintainer, error) {
	n, err := issueArg(c)
	if err != nil {
		return nil, err
	}
	o, ws := c.Ops, c.Env.Workspace
	cfg, err := loadSwarm(o, ws)
	if err != nil {
		return nil, err
	}
	vars := readSecrets(c, ws, c.Stderr, MaintainerTokenEnv, cfg.Forge.TokenEnv)
	token := vars[MaintainerTokenEnv]
	if token == "" {
		return nil, fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", MaintainerTokenEnv, filepath.Join(ws, ".env.sh"))
	}
	if token == vars[cfg.Forge.TokenEnv] {
		return nil, fmt.Errorf("Error: swarm: $%s holds the swarm's token ($%s); the maintainer's verbs act with your own (docs/swarm.md section 8)", MaintainerTokenEnv, cfg.Forge.TokenEnv)
	}
	var names []string
	for _, r := range cfg.Repos {
		names = append(names, r.Name)
	}
	f, err := open(o, cfg.Forge, names, token)
	if err != nil {
		return nil, fmt.Errorf("Error: swarm: %v", err)
	}
	acct, err := f.Login()
	if err != nil {
		return nil, fmt.Errorf("Error: swarm: $%s cannot read its own account: %v", MaintainerTokenEnv, err)
	}
	swarm := cfg.SwarmLogin()
	if strings.EqualFold(acct.Login, swarm) {
		return nil, fmt.Errorf("Error: swarm: $%s is %s's token, the swarm's account (trusted's last login); the maintainer's verbs act with your own (docs/swarm.md section 8)", MaintainerTokenEnv, acct.Login)
	}
	fmt.Fprintf(c.Stdout, "Maintainer %s on %s\n", acct.Login, cfg.Forge.Repo)
	_, repo, _ := strings.Cut(cfg.Forge.Repo, "/")
	return &maintainer{c: c, cfg: cfg, f: f, login: acct.Login, swarm: swarm, n: n, at: forge.Ref{Repo: repo, N: n}}, nil
}

// issue is issue n with its comments, open and carrying exactly one stage
// label (forge.IsStage: the flag and the terminals are none), that label
// being want unless want is "".
func (m *maintainer) issue(want string) (forge.Issue, error) {
	is, err := m.f.Issue(m.n)
	if err != nil {
		return is, fmt.Errorf("Error: swarm: %v", err)
	}
	if is.Pull {
		return is, fmt.Errorf("Error: swarm: #%d is a pull request; the verbs take the issue's number; nothing changed", m.n)
	}
	if !is.Open {
		return is, fmt.Errorf("Error: swarm: #%d is closed; nothing changed", m.n)
	}
	var stage []string
	for _, l := range is.Labels {
		if forge.IsStage(l) {
			stage = append(stage, l)
		}
	}
	switch {
	case len(stage) == 0:
		return is, fmt.Errorf("Error: swarm: #%d carries no stage label; nothing changed", m.n)
	case len(stage) > 1:
		return is, fmt.Errorf("Error: swarm: #%d carries %s; remove all but one on the forge first", m.n, strings.Join(stage, " and "))
	case want != "" && stage[0] != want:
		return is, fmt.Errorf("Error: swarm: #%d is %s, not %s; nothing changed", m.n, stage[0], want)
	}
	m.is, m.labels = is, slices.Clone(is.Labels)
	return is, nil
}

// current is issue n's current pull request (section 5): the swarm's with
// the largest slice; ok is false when the issue has none.
func (m *maintainer) current() (pulls []forge.Pull, cur forge.Pull, ok bool, err error) {
	pulls, err = m.f.Pulls(m.n, reconcile.Openers(m.swarm, m.cfg.Trusted))
	if err != nil {
		return nil, cur, false, fmt.Errorf("Error: swarm: %v", err)
	}
	if len(pulls) == 0 {
		return pulls, cur, false, nil
	}
	return pulls, pulls[len(pulls)-1], true, nil
}

// comment is the words after <n> as one comment, "" when they are blank;
// one that opens with the swarm's marker is refused, since none of the
// maintainer's entries carries it (section 6).
func comment(c *Context) (string, error) {
	text := strings.Join(c.Args[1:], " ")
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	if _, marked := reconcile.Marked(text); marked {
		return "", fmt.Errorf("Error: swarm: the comment opens with the swarm's marker, which only the swarm's entries carry; nothing posted")
	}
	return text, nil
}

// relabel is a verb's transition, then the maintainer's flag set as the
// new labels want it (regate).
func (m *maintainer) relabel(from, to string) error {
	if err := m.f.Relabel(m.at, from, to); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d %s → %s\n", m.n, from, to)
	m.labels = append(slices.DeleteFunc(m.labels, func(l string) bool { return l == from }), to)
	m.regate()
	return nil
}

// regate clears swarm:gated from issue n after a verb's write unless its
// labels now are gated (reconcile.Gated, the predicate the pass sets the
// flag by: section 5), so the forge and status agree before the next
// pass, which adds the flag, with its note, where it is missing. A removal
// the forge refuses is a warning: the verb's write is made, and the next
// pass clears the flag.
func (m *maintainer) regate() {
	if reconcile.Gated(m.labels, m.cfg.Gates, reconcile.ClassOf(m.is, m.swarm, m.cfg.Trusted)) {
		return
	}
	if err := reconcile.Unflag(m.f, m.at, m.labels, forge.Gated, m.c.Stdout); err != nil {
		fmt.Fprintf(m.c.Stderr, "⚠ #%d: %s not cleared (%v); the next pass of apply clears it\n", m.n, forge.Gated, err)
	}
}

// unflag clears flag from issue n when it carries it.
func (m *maintainer) unflag(flag string) error {
	if err := reconcile.Unflag(m.f, m.at, m.labels, flag, m.c.Stdout); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	return nil
}

// say posts text on issue n (sayOn).
func (m *maintainer) say(text string) error { return m.sayOn(m.at, m.is.Comments, text) }

// sayOn posts text as the maintainer on the thread at, whose comments are
// existing, unless its newest comment is the maintainer's with that text:
// a verb run again after a write that followed its comment failed finds
// the comment posted and goes on (section 11).
func (m *maintainer) sayOn(at forge.Ref, existing []forge.Comment, text string) error {
	on := ""
	if at != m.at {
		on = fmt.Sprintf(" on pull %s#%d", at.Repo, at.N)
	}
	if last := len(existing) - 1; last >= 0 && existing[last].Body == text && strings.EqualFold(existing[last].Author, m.login) {
		fmt.Fprintf(m.c.Stdout, "#%d comment already posted%s\n", m.n, on)
		return nil
	}
	if err := m.f.Comment(at, text); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d commented%s\n", m.n, on)
	return nil
}

// close closes issue n, then gives it terminal in place of its stage
// label, stage, and clears every flag (section 5), as reconcile.Retire
// does, each refusal after the close named for what it left undone.
func (m *maintainer) close(stage, terminal, how string) error {
	if err := m.f.CloseIssue(m.n); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d closed%s\n", m.n, how)
	if err := m.f.Relabel(m.at, stage, terminal); err != nil {
		return fmt.Errorf("Error: swarm: #%d is closed, but not relabelled %s: %v; relabel it on the forge", m.n, terminal, err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d %s → %s\n", m.n, stage, terminal)
	for _, flag := range forge.Flags {
		if err := reconcile.Unflag(m.f, m.at, m.labels, flag, m.c.Stdout); err != nil {
			return fmt.Errorf("Error: swarm: #%d is closed and relabelled %s, but %s is not cleared: %v; remove it on the forge", m.n, terminal, flag, err)
		}
	}
	return nil
}

func swarmTriage(c *Context, open opener) error {
	return swarmMove(c, open, forge.ProposalPosted, forge.ProposalAccepted)
}

func swarmAccept(c *Context, open opener) error {
	return swarmMove(c, open, forge.PlanPosted, forge.PlanAccepted)
}

// swarmMove is a gate's relabel, from the one stage it decides: triage's
// and accept's.
func swarmMove(c *Context, open opener, from, to string) error {
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	if _, err := m.issue(from); err != nil {
		return err
	}
	return m.relabel(from, to)
}

// swarmClass sets issue n's class: architecture sets
// swarm:class-architecture, routine clears it, and either then posts
// `Class: <class>`, which ClassOf reads as the newest statement and no
// member answers. An issue of that class already is left as it is; a
// routine statement by a login not in trusted, which ClassOf would not
// read, is refused before any write.
func swarmClass(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	class := c.Args[1]
	if class != reconcile.Routine && class != reconcile.Architecture {
		return UsageError("not a class: " + class + "; swarm class takes routine or architecture")
	}
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue("")
	if err != nil {
		return err
	}
	if reconcile.ClassOf(is, m.swarm, m.cfg.Trusted) == class {
		// An architecture job by default (no statement, or the
		// maintainer's issue) carries no flag: set it, so the forge
		// shows the class.
		if class == reconcile.Architecture && !slices.Contains(m.labels, forge.ClassArchitecture) {
			if err := reconcile.Flag(m.f, m.at, m.labels, forge.ClassArchitecture, c.Stdout); err != nil {
				return fmt.Errorf("Error: swarm: %v", err)
			}
			return nil
		}
		fmt.Fprintf(c.Stdout, "#%d already %s\n", m.n, class)
		return nil
	}
	if class == reconcile.Architecture {
		if err := reconcile.Flag(m.f, m.at, m.labels, forge.ClassArchitecture, c.Stdout); err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		return m.say("Class: " + class)
	}
	if !m.trusted() {
		return fmt.Errorf("Error: swarm: %s is not in trusted, so Daisy would not read your class statement; add the login to trusted in swarm.json (docs/swarm.md section 3); nothing changed", m.login)
	}
	if err := m.unflag(forge.ClassArchitecture); err != nil {
		return err
	}
	return m.say("Class: " + class)
}

func swarmClose(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	text, err := comment(c)
	if err != nil {
		return err
	}
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue("")
	if err != nil {
		return err
	}
	if text != "" {
		if err := m.say(text); err != nil {
			return err
		}
	}
	stage, _ := reconcile.StageOf(is)
	return m.close(stage, forge.Rejected, "")
}

func swarmApprove(c *Context, open opener) error {
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	if _, err := m.issue(""); err != nil {
		return err
	}
	_, cur, ok, err := m.current()
	if err != nil {
		return err
	}
	if !ok || !cur.Open || cur.HeadSHA == "" {
		return fmt.Errorf("Error: swarm: #%d has no open swarm pull request; nothing approved", m.n)
	}
	if err := m.f.Approve(forge.Ref{Repo: cur.Repo, N: cur.Number}, cur.HeadSHA); err != nil {
		return fmt.Errorf("Error: swarm: approving pull %s#%d: %v", cur.Repo, cur.Number, err)
	}
	fmt.Fprintf(c.Stdout, "#%d approved pull %s#%d at %s\n", m.n, cur.Repo, cur.Number, short(cur.HeadSHA))
	return nil
}

// swarmMerge merges the current pull request of an approved issue at the
// head both the swarm's newest reviews approve, with `do: merge` as the
// merge step does, or finds it merged; then, as the merge step does with
// the gate off, relabels the issue planned when the accepted plan declares
// a slice after it (T13) and closes it when it was the last. A merge the
// forge refuses of a pull request it reports not mergeable goes back to
// the worker, as the merge step sends it (T25), unless that head was sent
// back once already (reconcile.SentBack), which the verb refuses as the
// merge step blocks it; any other refused merge changes nothing: the
// maintainer is there to read why.
func swarmMerge(c *Context, open opener) error {
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue(forge.ReviewApproved)
	if err != nil {
		return err
	}
	_, cur, ok, err := m.current()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Error: swarm: #%d has no swarm pull request; nothing merged", m.n)
	}
	s, S := reconcile.Slice(is, cur, m.swarm)
	switch {
	case cur.Merged:
		fmt.Fprintf(c.Stdout, "#%d pull %s#%d already merged (slice %d of %d)\n", m.n, cur.Repo, cur.Number, s, S)
	case !cur.Open:
		return fmt.Errorf("Error: swarm: #%d: pull %s#%d is closed unmerged; nothing merged", m.n, cur.Repo, cur.Number)
	default:
		ref := forge.Ref{Repo: cur.Repo, N: cur.Number}
		reviews, err := m.f.Reviews(ref)
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		head, _ := reconcile.ApprovedHead(reviews, m.swarm)
		if head == "" {
			return fmt.Errorf("Error: swarm: #%d: no head of pull %s#%d is approved by both the swarm's reviews; nothing merged", m.n, cur.Repo, cur.Number)
		}
		err = m.f.Merge(ref, head)
		if errors.Is(err, forge.ErrNotMergeable) {
			msg := strings.TrimPrefix(err.Error(), forge.ErrNotMergeable.Error()+": ")
			switch cur.Mergeable {
			case forge.MergeableNo:
				return m.backToWorker(cur, head)
			case forge.MergeableUnknown:
				return fmt.Errorf("Error: swarm: #%d: merge of pull %s#%d at %s refused: %s; the forge has not yet computed whether it merges, so run merge again shortly; nothing changed", m.n, cur.Repo, cur.Number, short(head), msg)
			}
			return fmt.Errorf("Error: swarm: #%d: merge of pull %s#%d at %s refused: %s; nothing changed", m.n, cur.Repo, cur.Number, short(head), msg)
		}
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		fmt.Fprintf(c.Stdout, "#%d merged pull %s#%d (slice %d of %d)\n", m.n, cur.Repo, cur.Number, s, S)
	}
	if s < S {
		return m.relabel(forge.ReviewApproved, forge.PlanAccepted)
	}
	return m.close(forge.ReviewApproved, forge.Done, " (last slice merged)")
}

// backToWorker is T25 by the maintainer: the direction to merge base
// (reconcile.Direction) posted on the pull request, whose thread the
// worker reads, then the issue relabelled review-3-changes-requested.
func (m *maintainer) backToWorker(p forge.Pull, head string) error {
	owner, _, _ := strings.Cut(m.cfg.Forge.Repo, "/")
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	existing, err := m.f.PullComments(ref)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	if reconcile.SentBack(existing, owner, p, head, m.swarm, m.cfg.Trusted) {
		return fmt.Errorf("Error: swarm: #%d: pull %s#%d was sent back to merge %s at %s once already, was approved again at that head, and the forge still reports it not mergeable; merge %s into %s, or relabel the issue; nothing changed",
			m.n, p.Repo, p.Number, m.cfg.Base, short(head), m.cfg.Base, p.Head)
	}
	fmt.Fprintf(m.c.Stdout, "#%d not mergeable: back to the worker\n", m.n)
	if err := m.sayOn(ref, existing, reconcile.Direction(owner, m.cfg.Base, m.n, p, head, nil)); err != nil {
		return err
	}
	return m.relabel(forge.ReviewApproved, forge.ChangesRequested)
}

// sayOnPull posts text on pull request p's thread, as sayOn.
func (m *maintainer) sayOnPull(p forge.Pull, text string) error {
	ref := forge.Ref{Repo: p.Repo, N: p.Number}
	existing, err := m.f.PullComments(ref)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	return m.sayOn(ref, existing, text)
}

// swarmUnblock posts the maintainer's comment, then clears swarm:blocked
// from the issue, whose stage the block kept, so the role that stage runs
// reads the comment when it runs.
func swarmUnblock(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	text, err := comment(c)
	if err != nil {
		return err
	}
	if text == "" {
		return UsageError("swarm unblock takes a comment after <n>, saying what changed")
	}
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue("")
	if err != nil {
		return err
	}
	if !slices.Contains(is.Labels, forge.Blocked) {
		return fmt.Errorf("Error: swarm: #%d is not blocked; nothing changed", m.n)
	}
	// The round limit's second time leaves the issue at
	// review-1-in-progress, or at review-0-ready when T21's write was
	// refused: either way no role runs.
	stage, _ := reconcile.StageOf(is)
	if (stage == forge.ReviewInProgress || stage == forge.ReviewReady) && reconcile.AtTheRoundLimit(is, m.swarm) {
		return fmt.Errorf("Error: swarm: #%d: blocked at the review round limit, where both reviewers have reviewed the head and %s runs no one; relabel it on the forge first, %s for one more fix round or %s to re-plan, then unblock it; nothing changed", m.n, stage, forge.ChangesRequested, forge.PlanPosted)
	}
	if !m.trusted() {
		fmt.Fprintf(c.Stderr, "⚠ %s is not in trusted: no member reads your comment (docs/swarm.md section 6)\n", m.login)
	}
	// The comment goes where the stage's role reads: the worker in
	// review-3-changes-requested reads the current pull request's thread,
	// whose newest entry may be its own BLOCKED: (section 6, rule 3).
	if stage == forge.ChangesRequested {
		_, cur, ok, err := m.current()
		if err != nil {
			return err
		}
		if ok && cur.Open {
			if err := m.sayOnPull(cur, text); err != nil {
				return err
			}
			return m.lift()
		}
	}
	if err := m.say(text); err != nil {
		return err
	}
	return m.lift()
}

// lift clears swarm:blocked, then swarm:gated unless the stage the block
// kept is gated (regate).
func (m *maintainer) lift() error {
	if err := m.unflag(forge.Blocked); err != nil {
		return err
	}
	m.labels = slices.DeleteFunc(m.labels, func(l string) bool { return l == forge.Blocked })
	m.regate()
	return nil
}

// trusted: the maintainer's login is in trusted, so the role rule reads
// its comments (section 6).
func (m *maintainer) trusted() bool {
	return slices.ContainsFunc(m.cfg.Trusted, func(l string) bool { return strings.EqualFold(l, m.login) })
}

// SwarmPause is `daisy swarm pause [<n>]`: with <n>, the maintainer's
// hold on issue n, swarm:paused set (section 6, rule 1); without, the
// whole swarm paused (pauseSwarm).
func SwarmPause(c *Context) error { return swarmHold(c, openForge, true) }

// SwarmResume is `daisy swarm resume [<n>]`: with <n>, swarm:paused
// cleared from issue n; without, the whole swarm resumed (resumeSwarm).
func SwarmResume(c *Context) error { return swarmHold(c, openForge, false) }

func swarmHold(c *Context, open opener, pause bool) error {
	if len(c.Args) == 0 {
		if pause {
			return pauseSwarm(c)
		}
		return resumeSwarm(c)
	}
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	if _, err := m.issue(""); err != nil {
		return err
	}
	paused := slices.Contains(m.labels, forge.Paused)
	switch {
	case pause && paused:
		fmt.Fprintf(c.Stdout, "#%d already paused\n", m.n)
		return nil
	case !pause && !paused:
		fmt.Fprintf(c.Stdout, "#%d not paused\n", m.n)
		return nil
	case pause:
		if err := reconcile.Flag(m.f, m.at, m.labels, forge.Paused, c.Stdout); err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		return nil
	}
	return m.unflag(forge.Paused)
}

// swarmBlock flags #m swarm:blocking, then comments `Blocked by #<m>` on
// #n as the maintainer: the role rule launches nothing on #n while that
// comment is its newest by a maintainer and #m is open.
func swarmBlock(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	// --by after <n>, as the usage writes it, or leading, as the cli reads
	// a value flag.
	by, given := c.Value("--by")
	switch {
	case given && len(c.Args) == 1:
	case !given && len(c.Args) == 3 && c.Args[1] == "--by":
		by = c.Args[2]
	default:
		return UsageError("swarm block takes <n> --by <m>")
	}
	b, err := strconv.Atoi(by)
	if err != nil || b < 1 || strconv.Itoa(b) != by {
		return UsageError("not an issue number: " + by)
	}
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	if b == m.n {
		return fmt.Errorf("Error: swarm: #%d cannot block itself; nothing changed", m.n)
	}
	if _, err := m.issue(""); err != nil {
		return err
	}
	if !m.trusted() {
		fmt.Fprintf(c.Stderr, "⚠ %s is not in trusted: the role rule does not read your Blocked by #%d, so it holds nothing (docs/swarm.md section 6)\n", m.login, b)
	}
	blocker, err := m.f.Issue(b)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	if !blocker.Open || blocker.Pull {
		return fmt.Errorf("Error: swarm: #%d is not an open issue; nothing changed", b)
	}
	if err := reconcile.Flag(m.f, forge.Ref{Repo: m.at.Repo, N: b}, blocker.Labels, forge.Blocking, c.Stdout); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	return m.say(fmt.Sprintf("Blocked by #%d", b))
}

// short is a commit's first seven characters.
func short(sha string) string { return sha[:min(7, len(sha))] }
