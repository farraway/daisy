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

// SwarmTriage is `daisy swarm triage <n>`: G1, T3 (proposed → ready).
func SwarmTriage(c *Context) error { return swarmTriage(c, openForge) }

// SwarmAccept is `daisy swarm accept <n>`: G2, T7 (planning → planned).
func SwarmAccept(c *Context) error { return swarmAccept(c, openForge) }

// SwarmApprove is `daisy swarm approve <n>`: a formal approving review of
// the issue's current open pull request at its head (section 12).
func SwarmApprove(c *Context) error { return swarmApprove(c, openForge) }

// SwarmMerge is `daisy swarm merge <n>`: G3, the approved head merged,
// then T13 or the last slice's close (section 12).
func SwarmMerge(c *Context) error { return swarmMerge(c, openForge) }

// SwarmUnblock is `daisy swarm unblock <n> <comment...>`: G4, T18 to the
// stage the reconciler's note names.
func SwarmUnblock(c *Context) error { return swarmUnblock(c, openForge) }

// SwarmClose is `daisy swarm close <n> [comment...]`: T19.
func SwarmClose(c *Context) error { return swarmClose(c, openForge) }

// maintainer is the forge as one of the maintainer's verbs reaches it:
// under the maintainer's token, with the maintainer's login and the
// swarm's, for issue n.
type maintainer struct {
	c     *Context
	cfg   declare.Config
	f     forge.Adapter
	login string // the maintainer's, as the forge reports the token's
	swarm string // the swarm's, trusted's last
	n     int
	at    forge.Ref
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
	vars := secrets(c, ws)
	token := vars(MaintainerTokenEnv)
	if token == "" {
		return nil, fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", MaintainerTokenEnv, filepath.Join(ws, ".env.sh"))
	}
	if token == vars(cfg.Forge.TokenEnv) {
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

// issue is issue n with its comments, open and carrying exactly one
// swarm:* label, that label being want unless want is "".
func (m *maintainer) issue(want string) (forge.Issue, error) {
	is, err := m.f.Issue(m.n)
	if err != nil {
		return is, fmt.Errorf("Error: swarm: %v", err)
	}
	if !is.Open {
		return is, fmt.Errorf("Error: swarm: #%d is closed; nothing changed", m.n)
	}
	var stage []string
	for _, l := range is.Labels {
		if strings.HasPrefix(l, "swarm:") {
			stage = append(stage, l)
		}
	}
	switch {
	case len(stage) == 0:
		return is, fmt.Errorf("Error: swarm: #%d carries no swarm:* label; it is not the swarm's", m.n)
	case len(stage) > 1:
		return is, fmt.Errorf("Error: swarm: #%d carries %s; remove all but one on the forge first", m.n, strings.Join(stage, " and "))
	case want != "" && stage[0] != want:
		return is, fmt.Errorf("Error: swarm: #%d is %s, not %s; nothing changed", m.n, stage[0], want)
	}
	return is, nil
}

// current is issue n's current pull request (section 5): the swarm's with
// the largest slice; ok is false when the issue has none.
func (m *maintainer) current() (pulls []forge.Pull, cur forge.Pull, ok bool, err error) {
	pulls, err = m.f.Pulls(m.n)
	if err != nil {
		return nil, cur, false, fmt.Errorf("Error: swarm: %v", err)
	}
	if len(pulls) == 0 {
		return pulls, cur, false, nil
	}
	return pulls, pulls[len(pulls)-1], true, nil
}

// comment is the words after <n> as one comment; one that opens with the
// swarm's marker is refused, since none of the maintainer's entries carries
// it (section 6).
func comment(c *Context) (string, error) {
	text := strings.Join(c.Args[1:], " ")
	if _, marked := reconcile.Marked(text); marked {
		return "", fmt.Errorf("Error: swarm: the comment opens with the swarm's marker, which only the swarm's entries carry; nothing posted")
	}
	return text, nil
}

func (m *maintainer) relabel(from, to string) error {
	if err := m.f.Relabel(m.at, from, to); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d %s → %s\n", m.n, from, to)
	return nil
}

func (m *maintainer) say(text string) error {
	if err := m.f.Comment(m.at, text); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d commented\n", m.n)
	return nil
}

func (m *maintainer) close(how string) error {
	if err := m.f.CloseIssue(m.n); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(m.c.Stdout, "#%d closed%s\n", m.n, how)
	return nil
}

func swarmTriage(c *Context, open opener) error {
	return swarmMove(c, open, "swarm:proposed", "swarm:ready")
}

func swarmAccept(c *Context, open opener) error {
	return swarmMove(c, open, "swarm:planning", "swarm:planned")
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
	if _, err := m.issue(""); err != nil {
		return err
	}
	if text != "" {
		if err := m.say(text); err != nil {
			return err
		}
	}
	return m.close("")
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
// a slice after it (T13) and closes it when it was the last. A refused
// merge changes nothing: the maintainer is there to read why.
func swarmMerge(c *Context, open opener) error {
	m, err := asMaintainer(c, open)
	if err != nil {
		return err
	}
	is, err := m.issue("swarm:approved")
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
			return fmt.Errorf("Error: swarm: #%d: merge of pull %s#%d at %s refused: %s; nothing changed", m.n, cur.Repo, cur.Number, short(head), msg)
		}
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		fmt.Fprintf(c.Stdout, "#%d merged pull %s#%d (slice %d of %d)\n", m.n, cur.Repo, cur.Number, s, S)
	}
	if s < S {
		return m.relabel("swarm:approved", "swarm:planned")
	}
	return m.close(" (last slice merged)")
}

// swarmUnblock posts the maintainer's comment, then relabels the blocked
// issue to the stage the newest reconciler note that blocked it names, so
// the role that stage runs reads the comment when it runs.
func swarmUnblock(c *Context, open opener) error {
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
	is, err := m.issue("swarm:blocked")
	if err != nil {
		return err
	}
	prior, ok := reconcile.BlockedFrom(is, m.swarm)
	if !ok {
		return fmt.Errorf("Error: swarm: #%d: no reconciler note names the stage it was blocked from; relabel it on the forge; nothing changed", m.n)
	}
	if !slices.ContainsFunc(m.cfg.Trusted, func(l string) bool { return strings.EqualFold(l, m.login) }) {
		fmt.Fprintf(c.Stderr, "⚠ %s is not in trusted: no member reads your comment (docs/swarm.md section 6)\n", m.login)
	}
	if err := m.say(text); err != nil {
		return err
	}
	return m.relabel("swarm:blocked", prior)
}

// short is a commit's first seven characters.
func short(sha string) string { return sha[:min(7, len(sha))] }
