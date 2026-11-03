package feedback

import (
	"daisy/internal/ops"
	"daisy/internal/prompt"
	"daisy/internal/skill"
)

// RewriteThreshold is the most rules a `## Rules` section holds before its
// file is flagged for a whole-file rewrite, whose body absorbs what the
// rules have accreted (D12).
const RewriteThreshold = 10

// Group is one target's pending entries (indices into the Log), with the
// target's current rules. Tag is the first tag that named it; Resolved is
// false when it names no file, and then Target is empty.
type Group struct {
	Tag      string
	Target   Target
	Resolved bool
	Vendored bool
	Entries  []int
	Rules    int    // rules in the section
	HasRules bool   // the target has a `## Rules` section
	Section  []byte // the section, heading included
	Err      error  // the target could not be read; the rest is unknown
	Problem  error  // why apply --rules would refuse the section (prompt.RulesProblem)
}

// Flagged: the target's rules are past the threshold and it may be
// written.
func (g Group) Flagged() bool { return g.Resolved && !g.Vendored && g.Rules > RewriteThreshold }

// Flag is an active target with no pending entries and more rules than
// the threshold.
type Flag struct {
	Target Target
	Rules  int
}

// Briefing is what `daisy optimize` prints. Errors are the targets that
// could not be read, pending or active: the briefing is incomplete.
type Briefing struct {
	Pending []Group
	Flagged []Flag
	Errors  []error
}

// Brief groups l's entries by the target their tags resolve to — two tags
// naming one file are one target — in order of first appearance; a tag
// naming no file is a group of its own, unresolved. Flagged is every
// active target (the prompts include.txt names, in its order, then every
// skill source by name) that is not pending, not vendored and over the
// threshold; a pending target carries its flag on its Group. With a scope,
// only that target is briefed.
func Brief(fs ops.FS, root, home string, l Log, include []string, scope *Target) Briefing {
	var b Briefing
	byKey := map[string]int{}
	for i, e := range l.Entries {
		t, ok := Resolve(fs, root, home, e.Tag())
		key := "path:" + t.Path
		if !ok {
			key = "tag:" + e.Tag()
		}
		if scope != nil && (!ok || t.Path != scope.Path) {
			continue
		}
		g, seen := byKey[key]
		if !seen {
			g = len(b.Pending)
			byKey[key] = g
			ng := newGroup(fs, e.Tag(), t, ok)
			if ng.Err != nil {
				b.Errors = append(b.Errors, ng.Err)
			}
			b.Pending = append(b.Pending, ng)
		}
		b.Pending[g].Entries = append(b.Pending[g].Entries, i)
	}

	var active []Target
	for _, name := range include {
		if t, ok := promptTarget(fs, root, home, name); ok {
			active = append(active, t)
		}
	}
	for _, s := range skill.All(fs, skill.SourceDirs(root, home)) {
		if t, ok := skillTarget(fs, root, home, s.Name); ok {
			active = append(active, t)
		}
	}
	flagged := map[string]bool{}
	for _, t := range active {
		if _, pending := byKey["path:"+t.Path]; pending || flagged[t.Path] || (scope != nil && t.Path != scope.Path) {
			continue
		}
		g := newGroup(fs, t.Label, t, true)
		if g.Err != nil {
			b.Errors = append(b.Errors, g.Err)
		}
		if g.Flagged() {
			flagged[t.Path] = true
			b.Flagged = append(b.Flagged, Flag{Target: t, Rules: g.Rules})
		}
	}
	return b
}

func newGroup(fs ops.FS, tag string, t Target, resolved bool) Group {
	g := Group{Tag: tag, Target: t, Resolved: resolved}
	if !resolved {
		return g
	}
	g.Vendored = Vendored(fs, t)
	doc, err := t.Src.Text(fs) // an override as the agent reads it: merged
	if err != nil {
		g.Err = err
		return g
	}
	if start, end, ok := prompt.RulesSection(doc); ok {
		g.HasRules, g.Section, g.Rules = true, doc[start:end], prompt.CountRules(doc)
	}
	g.Problem = prompt.RulesProblem(doc)
	return g
}
