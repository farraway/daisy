package cmd

import (
	"bytes"
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
func SwarmTriage(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("triage")(c, openForge) })
}

// SwarmAccept is `daisy swarm accept <n>`: G2, T7 (plan-0-posted →
// plan-1-accepted).
func SwarmAccept(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("accept")(c, openForge) })
}

// SwarmClass is `daisy swarm class <n> routine|architecture`: the job's
// class (section 5), swarm:class-architecture set or cleared, then the
// statement `Class: <class>` posted as the maintainer.
func SwarmClass(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("class")(c, openForge) })
}

// SwarmApprove is `daisy swarm approve <n>`: a formal approving review of
// the issue's current open pull request at its head (section 12).
func SwarmApprove(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("approve")(c, openForge) })
}

// SwarmMerge is `daisy swarm merge <n>`: G3, the approved head merged,
// then T13 or the last slice's close (section 12).
func SwarmMerge(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("merge")(c, openForge) })
}

// SwarmUnblock is `daisy swarm unblock <n> [--cause <word>] [--start
// base] <comment...>`: G4, T18, the comment then swarm:blocked cleared;
// with --cause, the released stop's metrics row superseded with that
// cause first; with --start base, the comment opens with the line
// `Start: base` (section 7.8, The start).
func SwarmUnblock(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("unblock")(c, openForge) })
}

// SwarmReplan is `daisy swarm replan <n> [direction...]`: the
// maintainer's re-plan request posted, `Re-plan: <direction>`, so the
// architect revises the plan, or writes it, at the next pass (section 6,
// The re-plan).
func SwarmReplan(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("replan")(c, openForge) })
}

// SwarmBlock is `daisy swarm block <n> --by <m>`: #m flagged
// swarm:blocking, then `Blocked by #<m>` on #n, which holds #n's job while
// #m is open (section 6, rule 1).
func SwarmBlock(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("block")(c, openForge) })
}

// SwarmClose is `daisy swarm close <n> [comment...]`: T19.
func SwarmClose(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("close")(c, openForge) })
}

// commentVerbs is the maintainer's verbs a comment command runs (section
// 6, The comment commands), in section 11's order: each on the issue the
// comment is on, or whose pull request it is on, so no command takes
// <n>; pause and resume hold that issue alone, never the whole swarm,
// whose <state>/paused is the host's. prompt build writes the clone, so
// no comment runs it.
var commentVerbs = []commentVerb{
	{"triage", "", none, nil, swarmTriage},
	{"accept", "", none, nil, swarmAccept},
	{"class", " routine|architecture", func(n int) bool { return n == 1 }, nil, swarmClass},
	{"approve", "", none, nil, swarmApprove},
	{"merge", "", none, nil, swarmMerge},
	{"unblock", " [--cause <word>] [--start base] <comment...>", func(n int) bool { return n > 0 }, []string{causeFlag, startFlag}, swarmUnblock},
	{"block", " --by <m>", func(n int) bool { return n == 2 }, []string{"--by"}, swarmBlock},
	{"pause", "", none, nil, func(c *Context, open opener) error { return swarmHold(c, open, true) }},
	{"resume", "", none, nil, func(c *Context, open opener) error { return swarmHold(c, open, false) }},
	{"replan", " [direction...]", func(int) bool { return true }, nil, swarmReplan},
	{"close", " [comment...]", func(int) bool { return true }, nil, swarmClose},
}

// commentVerb is one of commentVerbs: its name, its arguments as its
// usage writes them (each also takes --type <type>, typed), whether it
// takes n of them, the value flags it reads after <n>, and the verb.
type commentVerb struct {
	name, args string
	takes      func(n int) bool
	flags      []string
	run        func(*Context, opener) error
}

// maintainerVerb is the verb of commentVerbs named name with its --type
// (typed), as the terminal and a comment run it.
func maintainerVerb(name string) func(*Context, opener) error {
	v := commentVerbs[slices.IndexFunc(commentVerbs, func(v commentVerb) bool { return v.name == name })]
	return typed(v)
}

func none(n int) bool { return n == 0 }

// commentRuns names the comment commands, as an answer lists them.
func commentRuns() string {
	var names []string
	for _, v := range commentVerbs {
		names = append(names, v.name)
	}
	return "a comment runs " + strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// commentCommand is apply's Command (section 6, The comment commands):
// the verb words name run on issue n as `daisy swarm <verb> <n> <args>`
// runs it, under $DAISY_MAINTAINER_TOKEN from <state>/env or the
// environment, its stderr apply's; what it printed and its refusal are
// returned with the host's paths named as the spec names them (<state>,
// <swarm>, $DAISY_LOCAL, ~), since the answer is posted on the forge.
func commentCommand(c *Context, open opener, state string) func(n int, words []string) (string, error) {
	var paths []string
	for _, p := range [][2]string{{state, "<state>"}, {c.Swarm.Root, "<swarm>"}, {c.Env.Local, "$DAISY_LOCAL"}, {c.Ops.Getenv("HOME"), "~"}} {
		if filepath.IsAbs(p[0]) && filepath.Clean(p[0]) != "/" {
			paths = append(paths, filepath.Clean(p[0]), p[1])
		}
	}
	hide := strings.NewReplacer(paths...)
	return func(n int, words []string) (string, error) {
		var out bytes.Buffer
		err := runComment(c, open, n, words, &out)
		if err != nil {
			msg := err.Error()
			if u := UsageError(""); errors.As(err, &u) {
				msg = "Error: swarm: " + msg
			}
			err = errors.New(hide.Replace(msg))
		}
		return hide.Replace(out.String()), err
	}
}

// runComment runs the comment verb words name on issue n, printing to
// out.
func runComment(c *Context, open opener, n int, words []string, out *bytes.Buffer) error {
	if len(words) == 0 {
		return errors.New("Error: swarm: " + commentRuns())
	}
	i := slices.IndexFunc(commentVerbs, func(v commentVerb) bool { return v.name == words[0] })
	if i < 0 {
		return fmt.Errorf("Error: swarm: %s is no comment command; %s", words[0], commentRuns())
	}
	v := commentVerbs[i]
	usage := fmt.Errorf("Error: swarm: usage: /swarm %s [--type <type>]%s (on the issue or its pull request)", v.name, v.args)
	if _, _, rest, err := typeAfter(words[1:], v.flags); err != nil || !v.takes(len(rest)) {
		return usage
	}
	sub := &Context{Ops: c.Ops, Env: c.Env, Swarm: c.Swarm, Stdin: strings.NewReader(""), Stdout: out, Stderr: c.Stderr,
		Args: append([]string{strconv.Itoa(n)}, words[1:]...)}
	return maintainerVerb(v.name)(sub, open)
}

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
func issueArg(c *Context) (int, error) { return parseIssue(c.Args[0]) }

// parseIssue is word as an issue number, 1 or more, else usage.
func parseIssue(word string) (int, error) {
	n, err := strconv.Atoi(word)
	if err != nil || n < 1 || strconv.Itoa(n) != word {
		return 0, UsageError("not an issue number: " + word)
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
	o := c.Ops
	cfg, err := loadSwarm(o, c.Swarm)
	if err != nil {
		return nil, err
	}
	vars, err := readSecrets(c, cfg, c.Stderr, MaintainerTokenEnv, cfg.Forge.TokenEnv)
	if err != nil {
		return nil, err
	}
	token := vars[MaintainerTokenEnv]
	if token == "" {
		return nil, unsetErr(c, cfg, MaintainerTokenEnv)
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
// maintainer's entries carries it, and so is one holding a command line,
// which would make it a command that no member reads (section 6).
func comment(c *Context) (string, error) {
	text := strings.Join(c.Args[1:], " ")
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	if _, marked := reconcile.Marked(text); marked {
		return "", fmt.Errorf("Error: swarm: the comment opens with the swarm's marker, which only the swarm's entries carry; nothing posted")
	}
	if reconcile.IsCommand(text) {
		lines := strings.Split(text, "\n")
		line := lines[slices.IndexFunc(lines, reconcile.IsCommand)]
		return "", fmt.Errorf("Error: swarm: the comment holds a command line (%s), which would make it a command that no member reads; nothing posted", strings.TrimSpace(line))
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
// the comment posted and goes on (section 11). Either way it says which
// thread, the issue's or the pull request's.
func (m *maintainer) sayOn(at forge.Ref, existing []forge.Comment, text string) error {
	on := " on the issue"
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
	pulls, _, _, err := m.current()
	if err != nil {
		return err
	}
	if text != "" {
		if err := m.say(text); err != nil {
			return err
		}
	}
	stage, _ := reconcile.StageOf(is)
	if err := m.close(stage, forge.Rejected, ""); err != nil {
		return err
	}
	for _, p := range pulls {
		if p.Open {
			fmt.Fprintf(c.Stdout, "#%d pull %s#%d left open (%s): close it on the forge\n", m.n, p.Repo, p.Number, p.URL)
		}
	}
	return nil
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
// maintainer is there to read why. The head's checks are read first: one
// failed sends the issue back to the worker as the merge step does
// (T26); one pending, or none reported while base requires them, is
// refused, nothing changed: the maintainer runs merge again later.
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
	outputs := reconcile.Outputs{FS: c.Ops.FS(), State: m.cfg.Forge.State(c.Env.Local)}
	// The plan read from the job's workspace, or built in memory for a
	// job without one (docs/swarm.md section 7.8, Daisy's own reads); one
	// whose plan the replay would take cut from its post would count its
	// slices short (section 7.8, Rollout): refused before any write.
	s, S, run, err := reconcile.Slice(is, cur, m.swarm, outputs)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v; nothing merged", err)
	} else if run != "" {
		return fmt.Errorf("Error: swarm: #%d: the plan of %s is cut on the forge and runs/%s/io/out.md is gone, so its slices cannot be counted (docs/swarm.md section 7.8, Rollout); nothing changed", m.n, run, run)
	}
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
		comments, err := m.f.PullComments(ref)
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		head, _ := reconcile.ApprovedHead(reviews, comments, m.swarm)
		if head == "" {
			return fmt.Errorf("Error: swarm: #%d: no head of pull %s#%d is approved by both the swarm's reviews; nothing merged", m.n, cur.Repo, cur.Number)
		}
		if err := m.mergeChecked(cur, head, outputs.State); err != nil {
			return err
		}
		checks, err := reconcile.HeadChecks(m.f, cur.Repo, m.cfg.Base, head)
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		switch {
		case len(checks.Failed) > 0:
			fmt.Fprintf(c.Stdout, "#%d checks failed at %s: back to the worker\n", m.n, short(head))
			owner, _, _ := strings.Cut(m.cfg.Forge.Repo, "/")
			if err := m.sayOnPull(cur, reconcile.ChecksFailed(owner, m.n, cur, head, checks.Failed)); err != nil {
				return err
			}
			return m.relabel(forge.ReviewApproved, forge.ChangesRequested)
		case checks.Wait():
			return fmt.Errorf("Error: swarm: #%d: merge of pull %s#%d waits: %s; run merge again when they have passed; nothing changed", m.n, cur.Repo, cur.Number, checks.Why(head, m.cfg.Base))
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
		err = m.relabel(forge.ReviewApproved, forge.PlanAccepted)
	} else {
		err = m.close(forge.ReviewApproved, forge.Done, " (last slice merged)")
	}
	if err != nil {
		return err
	}
	m.listPulls(outputs)
	return nil
}

// mergeChecked refuses, nothing changed, a head of a repository with a
// check that has no passed merge check on base as its mirror's last
// fetch left it (docs/swarm.md section 12.1, rule 3): the maintainer's
// merge is a person's act, made again once the queue has checked it.
func (m *maintainer) mergeChecked(cur forge.Pull, head, state string) error {
	if i := slices.IndexFunc(m.cfg.Repos, func(r declare.Repo) bool { return r.Name == cur.Repo }); i < 0 || len(m.cfg.Repos[i].Check) == 0 {
		return nil
	}
	g := reconcile.Git{O: m.c.Ops, State: state}
	base := g.Head(g.Mirror(cur.Repo), "refs/heads/"+m.cfg.Base)
	st, err := reconcile.MergeChecked(m.c.Ops.FS(), state, m.n, head, base)
	switch {
	case err != nil:
		return fmt.Errorf("Error: swarm: %v", err)
	case st == "passed":
		return nil
	case st != "running":
		st = "queued"
	}
	return fmt.Errorf("Error: swarm: #%d: merge of pull %s#%d waits: its merge check on %s at %s has not passed (%s); run merge again when it has; nothing changed", m.n, cur.Repo, cur.Number, m.cfg.Base, short(base), st)
}

// listPulls is the issue's pull requests listed in its body after the
// verb's merge (reconcile.ListPulls), as a pass lists them, the merged
// one's follow-up filed first (docs/swarm.md section 7.11, Follow-ups),
// under the maintainer's token, since no pass reads the issue the verb
// closes; a failure is a warning, the merge being made.
func (m *maintainer) listPulls(outputs reconcile.Outputs) {
	owner, _, _ := strings.Cut(m.cfg.Forge.Repo, "/")
	if err := reconcile.ListPulls(m.f, owner, m.is, reconcile.Openers(m.swarm, m.cfg.Trusted), m.swarm, &reconcile.FollowUps{Outputs: outputs}, m.c.Stdout); err != nil {
		fmt.Fprintf(m.c.Stderr, "⚠ #%d: pull requests: %v\n", m.n, err)
	}
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
// reads the comment when it runs. With --cause, the metrics row of the
// run whose stop set the block is superseded with that cause first
// (section 7.5, The stop's cause), so a run again after a refused forge
// write finds it written. With --start base the comment opens with the
// line `Start: base` and a blank line, so the slice's next worker run
// starts where it would with no kept run (section 7.8, The start): on
// its first line, no fence the comment opens can hold it.
func swarmUnblock(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	cause, base, args, err := unblockArgs(c)
	if err != nil {
		return err
	}
	rest := *c
	rest.Args = args
	text, err := comment(&rest)
	if err != nil {
		return err
	}
	if text == "" {
		return UsageError("swarm unblock takes a comment after <n>, saying what changed")
	}
	if base {
		text = reconcile.StartBase + "\n\n" + text
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
		return fmt.Errorf("Error: swarm: #%d: blocked at the review round limit, where the pair has completed on the head and %s runs no one; relabel it on the forge first, %s for one more fix round or %s to re-plan, then unblock it; nothing changed", m.n, stage, forge.ChangesRequested, forge.PlanPosted)
	}
	if cause != "" {
		if err := m.retag(cause); err != nil {
			return err
		}
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
	if err := m.sayAndRehold(is, text); err != nil {
		return err
	}
	return m.lift()
}

// sayAndRehold posts unblock's text on issue n, then each `Blocked by
// #<b>` again (rehold), each once however often the verb is run: the
// thread it finds ending with the text, the holds are read as they stood
// before it, and ending with the text and the holds again, all are
// posted already.
func (m *maintainer) sayAndRehold(is forge.Issue, text string) error {
	cs := is.Comments
	mine := func(i int, body string) bool {
		return i >= 0 && cs[i].Body == body && strings.EqualFold(cs[i].Author, m.login)
	}
	if bs := reconcile.BlockedBy(is, m.swarm, m.cfg.Trusted); len(bs) > 0 && mine(len(cs)-len(bs)-1, text) {
		reheld := true
		for i, b := range bs {
			reheld = reheld && mine(len(cs)-len(bs)+i, fmt.Sprintf("Blocked by #%d", b))
		}
		if reheld {
			return m.sayOn(m.at, cs[:len(cs)-len(bs)], text)
		}
	}
	if err := m.say(text); err != nil {
		return err
	}
	if mine(len(cs)-1, text) {
		is.Comments = cs[:len(cs)-1]
	}
	return m.rehold(is)
}

// rehold posts each `Blocked by #<b>` again on issue n after unblock's
// comment there when, as is was read before it, the maintainer's newest
// comments held n by an open #b (section 6, rule 1; section 12.1, rule
// 1): the comment would otherwise end those holds, which `block` placed
// after the unblock keeps.
func (m *maintainer) rehold(is forge.Issue) error {
	for _, b := range reconcile.BlockedBy(is, m.swarm, m.cfg.Trusted) {
		held, err := reconcile.Blocker(m.f, m.n, b, m.c.Stderr)
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		if !held {
			continue
		}
		if err := m.f.Comment(m.at, fmt.Sprintf("Blocked by #%d", b)); err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		fmt.Fprintf(m.c.Stdout, "#%d held by #%d: Blocked by #%d posted again\n", m.n, b, b)
	}
	return nil
}

// causeFlag is unblock's flag naming the released stop's cause, and
// startFlag the one that starts the slice's next worker run from base.
const (
	causeFlag = "--cause"
	startFlag = "--start"
)

// unblockArgs is unblock's --cause <word> and --start base, each leading
// as the cli reads a value flag or right after <n>, in either order, as
// the usage writes them, and the arguments without them: cause "" when
// it is not given, base whether --start is. A flag given twice, one
// without its word, a cause outside the vocabulary (section 7.5, The
// stop's cause) and any word for --start but base are usage.
func unblockArgs(c *Context) (cause string, base bool, args []string, err error) {
	word := map[string]string{}
	given := map[string]bool{}
	for _, f := range []string{causeFlag, startFlag} {
		word[f], given[f] = c.Value(f)
	}
	args = c.Args
	for len(args) > 1 && (args[1] == causeFlag || args[1] == startFlag) {
		f := args[1]
		if given[f] {
			return "", false, nil, UsageError(f + " is given twice")
		}
		if len(args) == 2 {
			return "", false, nil, UsageError(takes(f))
		}
		word[f], given[f], args = args[2], true, append([]string{args[0]}, args[3:]...)
	}
	if given[causeFlag] && !reconcile.IsCause(word[causeFlag]) {
		return "", false, nil, UsageError(fmt.Sprintf("--cause takes one of %s, not %q", reconcile.CauseWords(), word[causeFlag]))
	}
	if given[startFlag] && word[startFlag] != "base" {
		return "", false, nil, UsageError(fmt.Sprintf("%s, not %q", takes(startFlag), word[startFlag]))
	}
	return word[causeFlag], given[startFlag], args, nil
}

// takes is unblock's usage for flag f given without its word.
func takes(f string) string {
	if f == startFlag {
		return "--start takes base"
	}
	return "--cause takes a word, one of " + reconcile.CauseWords()
}

// retag supersedes the metrics row of the run whose stop set the block
// with cause (reconcile.Retag), a refusal before any write.
func (m *maintainer) retag(cause string) error {
	c := m.c
	s := &reconcile.Swarm{O: c.Ops, Place: c.Swarm, WS: c.Swarm.Root, Root: c.Env.Root, Config: c.Env.Config, Local: c.Env.Local, Cfg: m.cfg, Out: c.Stdout, Err: c.Stderr}
	t, err := s.Retag(m.n, cause)
	if err != nil {
		return fmt.Errorf("Error: swarm: #%d: --cause: %v; nothing changed", m.n, err)
	}
	if t.From.Cause == cause {
		fmt.Fprintf(c.Stdout, "#%d %s cause already %s\n", m.n, t.To.Unit, cause)
		return nil
	}
	if _, err := s.WriteRetag(t); err != nil {
		return fmt.Errorf("Error: swarm: #%d: --cause: %v; nothing changed", m.n, err)
	}
	from := t.From.Cause
	if from == "" {
		from = "—"
	}
	fmt.Fprintf(c.Stdout, "#%d %s cause %s → %s\n", m.n, t.To.Unit, from, cause)
	return nil
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

// swarmReplan posts the maintainer's re-plan request on the issue
// (reconcile.ReplanText), a counted entry the architect answers (section
// 6, rule 3), and moves the job to the stage where the architect reads
// it: proposal-accepted and plan-0-posted are kept (the architect plans,
// or revises, from the newest comment); plan-1-accepted with a plan is
// kept, the request running the architect in the worker's place (Step
// 9's revision: merged slices kept, the remaining revised or STANDS:);
// plan-1-accepted with no plan goes to proposal-accepted; a review stage
// whose current pull request is no longer open goes to plan-1-accepted,
// or proposal-accepted with no plan. Nothing in flight is interrupted:
// a proposal not triaged, a worker's run, a run on the issue not yet
// collected, an open pull request (but at the two stages where the
// architect reads it, after the round limit) and a question waiting for
// its answer are refused, nothing changed.
func swarmReplan(c *Context, open opener) error {
	if _, err := issueArg(c); err != nil {
		return err
	}
	direction, err := comment(c)
	if err != nil {
		return err
	}
	text := reconcile.ReplanText(direction)
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue("")
	if err != nil {
		return err
	}
	if !m.trusted() {
		return fmt.Errorf("Error: swarm: %s is not in trusted, so no member would read your direction; add the login to trusted in swarm.json (docs/swarm.md section 3); nothing changed", m.login)
	}
	stage, _ := reconcile.StageOf(is)
	switch {
	case slices.Contains(is.Labels, forge.Question):
		return fmt.Errorf("Error: swarm: #%d waits for the answer to a member's question (%s); replan once it is answered; nothing changed", m.n, forge.Question)
	case stage == forge.ProposalPosted:
		return fmt.Errorf("Error: swarm: #%d is %s, with no plan to re-plan: triage it (swarm triage %d), and the architect plans it; nothing changed", m.n, stage, m.n)
	case stage == forge.PlanInProgress:
		return fmt.Errorf("Error: swarm: #%d: a worker runs on it (%s); replan once its run is collected; nothing changed", m.n, stage)
	}
	runs, err := reconcile.InFlight(c.Ops.FS(), m.cfg.Forge.State(c.Env.Local), m.n)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	if len(runs) > 0 {
		return fmt.Errorf("Error: swarm: #%d: %s run %d is not yet collected; replan once it is; nothing changed", m.n, runs[0].Role, runs[0].K)
	}
	_, cur, ok, err := m.current()
	if err != nil {
		return err
	}
	architect := stage == forge.ProposalAccepted || stage == forge.PlanPosted
	if ok && cur.Open && !architect {
		owner, _, _ := strings.Cut(m.cfg.Forge.Repo, "/")
		return fmt.Errorf("Error: swarm: #%d: pull request %s/%s#%d (%s) is open (%s); close or merge it first, then replan; nothing changed", m.n, owner, cur.Repo, cur.Number, cur.URL, stage)
	}
	to := ""
	switch planned := reconcile.Planned(is, m.swarm); {
	case architect:
	case !planned:
		to = forge.ProposalAccepted
	case stage != forge.PlanAccepted:
		to = forge.PlanAccepted
	}
	if err := m.say(text); err != nil {
		return err
	}
	if to == "" {
		return nil
	}
	return m.relabel(stage, to)
}

// SwarmPause is `daisy swarm pause [<n>]`: with <n>, the maintainer's
// hold on issue n, swarm:paused set (section 6, rule 1); without, the
// whole swarm paused (pauseSwarm).
func SwarmPause(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("pause")(c, openForge) })
}

// SwarmResume is `daisy swarm resume [<n>]`: with <n>, swarm:paused
// cleared from issue n; without, the whole swarm resumed (resumeSwarm).
func SwarmResume(c *Context) error {
	return inSwarm(c, func() error { return maintainerVerb("resume")(c, openForge) })
}

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
// comment is among its newest by a maintainer and #m is open (section
// 12.1, rule 1: block run twice holds by both).
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
	b, err := parseIssue(by)
	if err != nil {
		return err
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
