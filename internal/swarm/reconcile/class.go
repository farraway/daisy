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
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
)

// The two job classes (section 5): a routine job passes the triage and
// plan gates by itself; an architecture job keeps every gate.
const (
	Routine      = "routine"
	Architecture = "architecture"
)

// classRe is a class statement's line: the owner's after a proposal's
// Brief: line, or a maintainer's comment, which `swarm class` posts.
var classRe = regexp.MustCompile(`^Class: (routine|architecture)$`)

// classOfText is the class text states when it is a class statement alone,
// whitespace at either end aside; "" when it is not.
func classOfText(text string) string {
	if m := classRe.FindStringSubmatch(strings.TrimSpace(text)); m != nil {
		return m[1]
	}
	return ""
}

// ProposalClass is the class a proposal's text (after the marker lines)
// states on the line right after the `Brief:` line it opens with; "" when
// it opens with no Brief: line or that line is no class statement.
func ProposalClass(text string) string {
	lines := strings.SplitN(strings.TrimLeft(text, "\n"), "\n", 3)
	if len(lines) < 2 || !strings.HasPrefix(strings.TrimRight(lines[0], "\r "), "Brief: ") {
		return ""
	}
	return classOfText(lines[1])
}

// ClassOf is the job class of is, read with its comments (section 5):
// architecture when it carries swarm:class-architecture; otherwise its
// newest class statement's class, and architecture when it has none. The
// statements are the owner's, the line after its proposal's Brief: line
// (ProposalClass), and each comment by a login in trusted other than the
// swarm's that is `Class: <class>` alone.
func ClassOf(is forge.Issue, swarmLogin string, trusted []string) string {
	if slices.Contains(is.Labels, forge.ClassArchitecture) {
		return Architecture
	}
	class := ""
	if m, ok := Swarms(Entry{is.Author, is.Body, is.Created}, swarmLogin); ok && m.Role == "owner" {
		class = ProposalClass(swarmText(is.Body))
	}
	for _, c := range is.Comments {
		if c := maintainerClass(c, swarmLogin, trusted); c != "" {
			class = c
		}
	}
	if class == "" {
		return Architecture
	}
	return class
}

// maintainerClass is the class c states when it is a class statement by a
// login in trusted other than the swarm's; "" otherwise.
func maintainerClass(c forge.Comment, swarmLogin string, trusted []string) string {
	if strings.EqualFold(c.Author, swarmLogin) || !slices.ContainsFunc(trusted, func(l string) bool { return strings.EqualFold(l, c.Author) }) {
		return ""
	}
	return classOfText(c.Body)
}

// classGate is the gate that holds an issue with labels until the
// maintainer passes it, when its class is architecture (section 5): its
// one stage and `triage` at proposal-posted with gates.triage, `plan` at
// plan-0-posted with gates.plan; "" at any other stage, or with that gate
// off.
func (s *Swarm) classGate(labels []string) (stage, gate string) {
	stage, ok := StageOf(forge.Issue{Labels: labels})
	switch {
	case ok && stage == forge.ProposalPosted && s.Cfg.Gates.Triage:
		return stage, "triage"
	case ok && stage == forge.PlanPosted && s.Cfg.Gates.Plan:
		return stage, "plan"
	}
	return "", ""
}

// passedTo is the stage a gate passed moves its issue to (T3, T7).
var passedTo = map[string]string{forge.ProposalPosted: forge.ProposalAccepted, forge.PlanPosted: forge.PlanAccepted}

// autoPass is a routine job's gate passed by Daisy (section 5, T23 and
// T24): is, read with its comments, at stage under gate, moved to the
// next stage with the auto note posted, when the swarm is not paused, is
// is neither blocked nor paused, and the stage's role has nothing to
// answer (the owner's proposal, or the architect's plan, is the newest
// counted entry), so a comment of the maintainer's is answered first.
// It returns the labels is carries after it.
func (s *Swarm) autoPass(is forge.Issue, stage, gate string) ([]string, error) {
	if s.paused() != "" || Held(is.Labels) != "" || len(Launch(Turn{Label: stage, Thread: IssueThread(is)}, s.SwarmLogin, s.Cfg.Trusted)) > 0 {
		return is.Labels, nil
	}
	to := passedTo[stage]
	at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: is.Number}
	if err := s.Forge.Relabel(at, stage, to); err != nil {
		return is.Labels, err
	}
	s.moved(is.Number, to)
	fmt.Fprintf(s.Out, "#%d %s → %s (auto)\n", is.Number, stage, to)
	if err := s.recordAuto(is.Number, stage); err != nil {
		fmt.Fprintf(s.Err, "⚠ #%d: auto: %v\n", is.Number, err)
	}
	labels := slices.Clone(is.Labels)
	labels[slices.Index(labels, stage)] = to
	note := fmt.Sprintf("%s\n**reconciler:** auto at %s: a routine job passes the %s gate by itself, to %s.", autoMarker, strings.TrimPrefix(stage, "swarm:"), gate, strings.TrimPrefix(to, "swarm:"))
	if err := s.Forge.Comment(at, note); err != nil {
		return labels, fmt.Errorf("auto note: %w", err)
	}
	fmt.Fprintf(s.Out, "#%d auto note posted\n", is.Number)
	return labels, nil
}

// autoNote is the auto note's second line, naming the stage it passed.
var autoNote = regexp.MustCompile(`^\*\*reconciler:\*\* auto at (proposal-posted|plan-0-posted): `)

// recordAuto appends the gate Daisy passed for issue n at stage to
// <state>/auto, `<RFC 3339 time> #<n> <stage>`, the stage without
// `swarm:`: the count status keeps once the issue has left the forge's
// window (AutoRecorded).
func (s *Swarm) recordAuto(n int, stage string) error {
	fs, path := s.O.FS(), filepath.Join(s.state(), "auto")
	old, err := fs.ReadFile(path)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	line := fmt.Sprintf("%s #%d %s\n", s.O.Now().UTC().Format(time.RFC3339), n, strings.TrimPrefix(stage, "swarm:"))
	return fs.WriteFile(path, append(old, line...), 0o644)
}

// AutoRecorded is <state>/auto read (recordAuto): by issue, how many times
// Daisy passed its triage gate and its plan gate; none when the file is
// absent, and a line it cannot read counted nowhere.
func AutoRecorded(fs ops.FS, state string) (map[int][2]int, error) {
	b, err := fs.ReadFile(filepath.Join(state, "auto"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[int][2]int{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(f[1], "#"))
		if err != nil || !strings.HasPrefix(f[1], "#") {
			continue
		}
		c := out[n]
		switch "swarm:" + f[2] {
		case forge.ProposalPosted:
			c[0]++
		case forge.PlanPosted:
			c[1]++
		}
		out[n] = c
	}
	return out, nil
}

// AutoPasses is how many times Daisy passed is's triage gate and its plan
// gate for a routine job: the auto notes on it, by the stage each names.
func AutoPasses(is forge.Issue, swarmLogin string) (triage, plan int) {
	for _, c := range is.Comments {
		m, ok := Swarms(Entry{c.Author, c.Body, c.Created}, swarmLogin)
		if !ok || m.Kind != "auto" {
			continue
		}
		_, rest, _ := strings.Cut(c.Body, "\n")
		switch l := autoNote.FindStringSubmatch(rest); {
		case l == nil:
		case l[1] == "proposal-posted":
			triage++
		default:
			plan++
		}
	}
	return triage, plan
}
