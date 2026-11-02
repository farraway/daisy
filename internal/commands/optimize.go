package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/feedback"
	"daisy/internal/ops"
	"daisy/internal/prompt"
)

// Optimize prints the briefing the agent proposes from: each target with
// pending entries — its path, the entries, its current `## Rules` and their
// count — and every active prompt or skill whose rules are past the
// rewrite threshold. Read-only; the agent is the model, `optimize apply`
// writes what it proposes.
var Optimize = readOnly(optimize)

func optimize(c *Context) error {
	fsys := c.Ops.FS()
	var scope *feedback.Target
	if wf, ok := c.Value("--workflow", "-w"); ok {
		t, ok := feedback.Resolve(fsys, c.Env.Root, c.Env.Home, wf)
		if !ok {
			return unknownWorkflow(c, wf)
		}
		scope = &t
	}
	doc, err := readFeedback(c)
	if err != nil {
		return err
	}
	log := feedback.Parse(doc)
	include, err := c.includedPrompts()
	if err != nil {
		return err
	}
	b := feedback.Brief(fsys, c.Env.Root, c.Env.Home, log, include, scope)
	writeBriefing(c, log, b, scope)
	if len(b.Errors) > 0 {
		var msgs []string
		for _, err := range b.Errors {
			msgs = append(msgs, "Error: "+err.Error())
		}
		return fmt.Errorf("%s\nThe briefing above is incomplete.", strings.Join(msgs, "\n"))
	}
	return nil
}

// readFeedback is the home's feedback.md, which must exist.
func readFeedback(c *Context) ([]byte, error) {
	file := c.feedbackFile()
	doc, err := c.Ops.FS().ReadFile(file)
	if err != nil {
		if !lexists(c.Ops.FS(), file) {
			return nil, fmt.Errorf("Error: No feedback file found at %s\n  Run: mkdir -p \"%s\" && touch \"%s\"", file, filepath.Dir(file), file)
		}
		return nil, fmt.Errorf("Error: cannot read %s: %v", file, err)
	}
	return doc, nil
}

// includedPrompts are the prompt names include.txt lists: the active ones.
// A home without include.txt has none.
func (c *Context) includedPrompts() ([]string, error) {
	path := filepath.Join(c.Env.Home, "include.txt")
	data, err := c.Ops.FS().ReadFile(path)
	if err != nil {
		if !lexists(c.Ops.FS(), path) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	entries, err := prompt.ParseInclude(data)
	if err != nil {
		return nil, fmt.Errorf("Error: %s: %v", path, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func writeBriefing(c *Context, log feedback.Log, b feedback.Briefing, scope *feedback.Target) {
	w := c.Stdout
	entries := 0
	for _, g := range b.Pending {
		entries += len(g.Entries)
	}
	switch {
	case entries == 0 && scope != nil:
		fmt.Fprintf(w, "Nothing pending for %s in %s\n", scope.Label, c.feedbackFile())
	case entries == 0:
		fmt.Fprintf(w, "Nothing pending in %s\n", c.feedbackFile())
	default:
		fmt.Fprintf(w, "Pending feedback: %s for %s (%s)\n", plural(entries, "entry", "entries"), plural(len(b.Pending), "target", "targets"), c.feedbackFile())
	}
	for _, g := range b.Pending {
		label := g.Target.Label
		if !g.Resolved {
			label = g.Tag
		}
		fmt.Fprintf(w, "\n[%s] %s\n", label, plural(len(g.Entries), "entry", "entries"))
		switch {
		case !g.Resolved:
			fmt.Fprintf(w, "Target: none — '%s' names no prompt or skill. Correct the entry's **Workflow:** line (daisy feedback lists the targets).\n", g.Tag)
		case g.Err != nil:
			fmt.Fprintf(w, "Target: %s — %v\n", g.Target.Path, g.Err)
		case g.Vendored:
			fmt.Fprintf(w, "Target: %s\nNot offered — %s is a vendored skill (carries LICENSE/NOTICE.md); rewriting Daisy's copy would fork it from upstream. Its entries stay pending.\n", g.Target.Path, g.Target.Label)
		default:
			fmt.Fprintf(w, "Target: %s\n", g.Target.Path)
			if g.Problem != nil {
				fmt.Fprintf(w, "--rules refused: %v\n", g.Problem)
			}
			switch {
			case !g.HasRules:
				fmt.Fprintln(w, "Rules: none (no ## Rules section; apply --rules adds one)")
			case g.Flagged():
				fmt.Fprintf(w, "Rules: %d — more than %d: rewrite flagged\n", g.Rules, feedback.RewriteThreshold)
			default:
				fmt.Fprintf(w, "Rules: %d\n", g.Rules)
			}
		}
		fmt.Fprintln(w, "--- entries ---")
		for i, n := range g.Entries {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, log.Entries[n].Text)
		}
		if g.Resolved && !g.Vendored && g.Err == nil && g.HasRules {
			fmt.Fprintf(w, "--- current ## Rules ---\n%s\n", strings.TrimRight(string(g.Section), "\n"))
		}
		fmt.Fprintln(w, "--- end ---")
	}
	if len(b.Flagged) > 0 {
		fmt.Fprintf(w, "\nRewrite flagged (more than %d rules, nothing pending):\n", feedback.RewriteThreshold)
		for _, f := range b.Flagged {
			fmt.Fprintf(w, "[%s] rewrite flagged: %d rules\nTarget: %s\n", f.Target.Label, f.Rules, f.Target.Path)
		}
	}
	if entries == 0 && len(b.Flagged) == 0 {
		return
	}
	fmt.Fprintln(w)
	if entries > 0 {
		example := "<target>"
		for _, g := range b.Pending {
			if g.Resolved && !g.Vendored && g.Err == nil {
				example = g.Target.Label
				break
			}
		}
		fmt.Fprintf(w, "Propose a ## Rules section per target folding its entries in; once approved, apply it with:\n  daisy optimize apply --workflow %s --rules <file>\n", example)
	}
	flagged := len(b.Flagged) > 0
	for _, g := range b.Pending {
		flagged = flagged || g.Flagged()
	}
	if flagged {
		fmt.Fprintln(w, "For a flagged target, offer a rewrite whose body absorbs its rules, applied with:\n  daisy optimize apply --workflow <target> --rewrite <file>")
	}
}

// OptimizeApply writes the agent's proposal for one target: with --rules
// the target's `## Rules` section is replaced, with --rewrite the whole
// file. It shows the diff and asks (unless --yes, for text the user approved
// in the conversation); on approval it writes the target, moves the target's
// pending entries to the archive with their provenance, stamps
// feedback/.last-optimized, rebuilds AGENTS.md for a prompt and commits
// the home once. A target in the root is left for the root commit, which
// runs the publication gate. Every refusal comes before the first write;
// a failed write takes back the ones before it.
func OptimizeApply(c *Context) error {
	workflow, ok := c.Value("--workflow", "-w")
	if !ok {
		return UsageError("missing --workflow")
	}
	rules, isRules := c.Value("--rules")
	rewrite, isRewrite := c.Value("--rewrite")
	mode, file := "rules", rules
	switch {
	case isRules && isRewrite:
		return UsageError("--rules and --rewrite: give one")
	case isRewrite:
		mode, file = "rewrite", rewrite
	case !isRules:
		return UsageError("missing --rules <file> or --rewrite <file>")
	}

	fsys := c.Ops.FS()
	t, ok := feedback.Resolve(fsys, c.Env.Root, c.Env.Home, workflow)
	if !ok {
		return unknownWorkflow(c, workflow)
	}
	if feedback.Vendored(fsys, t) {
		return fmt.Errorf("Error: %s is a vendored skill (carries LICENSE/NOTICE.md); rewriting Daisy's copy would fork it from upstream", t.Label)
	}
	path, err := absolute(c, file)
	if err != nil {
		return err
	}
	proposal, err := fsys.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", file, err)
	}
	before, err := fsys.ReadFile(t.Path)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", t.Path, err)
	}
	after := proposal
	if mode == "rules" {
		if after, err = prompt.ReplaceRules(before, proposal); err != nil {
			var te prompt.TargetError
			if errors.As(err, &te) {
				return fmt.Errorf("Error: %s: %v", t.Path, err)
			}
			return fmt.Errorf("Error: %s %v", file, err)
		}
	} else if strings.TrimSpace(string(proposal)) == "" {
		return fmt.Errorf("Error: %s is empty; a rewrite replaces the whole of %s", file, t.Label)
	}
	feedbackDoc, err := readFeedback(c)
	if err != nil {
		return err
	}
	log := feedback.Parse(feedbackDoc)
	var pending []int
	for i, e := range log.Entries {
		if et, ok := feedback.Resolve(fsys, c.Env.Root, c.Env.Home, e.Tag()); ok && et.Path == t.Path {
			pending = append(pending, i)
		}
	}
	if string(after) == string(before) && len(pending) == 0 {
		fmt.Fprintf(c.Stdout, "Nothing to apply: %s already holds this text and no entries are pending for it.\n", t.Path)
		return nil
	}
	repo, err := homeRepo(c)
	if err != nil {
		return err
	}
	// The archive is read, and must be startable from the template, only
	// when entries will move to it.
	archivePath := c.archiveFile()
	var archive []byte
	if len(pending) > 0 {
		if archive, err = readCurrent(c, archivePath); err != nil {
			return err
		}
	}

	if d := feedback.Diff("a/"+t.Label, "b/"+t.Label, before, after); d != "" {
		fmt.Fprint(c.Stdout, d)
	} else {
		fmt.Fprintf(c.Stdout, "(no change to %s)\n", t.Label)
	}
	fmt.Fprintf(c.Stdout, "Pending entries archived on apply: %d\n", len(pending))
	if !c.Flag("--yes") {
		answer, err := c.Ops.Prompt(c.Stdin, c.Stderr, "Apply these changes to "+t.Label+"? [y/N] ")
		switch {
		case errors.Is(err, io.EOF):
			fmt.Fprintf(c.Stdout, "Cancelled — no answer on stdin (EOF); nothing written.\n")
			return nil
		case err != nil:
			return fmt.Errorf("Error: cannot read the answer: %v\nNothing was written.", err)
		case !yes(answer):
			fmt.Fprintf(c.Stdout, "Cancelled — %s left unchanged.\n", t.Label)
			return nil
		}
	}

	// The writes were computed from what was read before; a file changed
	// since (an entry recorded while the question waited) would be lost, so
	// the apply is refused instead — with --yes too, where the window is
	// short but not empty.
	for _, f := range []struct {
		path string
		read []byte
	}{{t.Path, before}, {c.feedbackFile(), feedbackDoc}, {archivePath, archive}} {
		if f.path == archivePath && len(pending) == 0 {
			continue
		}
		if got, err := readCurrent(c, f.path); err != nil || string(got) != string(f.read) {
			return fmt.Errorf("Error: %s changed while waiting for the answer; nothing was written. Run the apply again.", f.path)
		}
	}

	var entries []feedback.Entry
	archived := map[int]bool{}
	for _, i := range pending {
		entries, archived[i] = append(entries, log.Entries[i]), true
	}
	now := c.Ops.Now()
	writes := []fileWrite{{t.Path, after}}
	if len(pending) > 0 {
		provenance := fmt.Sprintf("**Addressed by:** `daisy optimize` %s — `%s` (%s)", now.Format("2006-01-02"), provenancePath(c.Env, t.Path), mode)
		writes = append(writes,
			fileWrite{archivePath, feedback.Archive(archive, entries, provenance)},
			fileWrite{c.feedbackFile(), log.Keep(func(i int) bool { return !archived[i] })})
	}
	writes = append(writes, fileWrite{filepath.Join(c.Env.Home, "feedback", ".last-optimized"), []byte(now.Format("2006-01-02 1504") + " " + t.Label + "\n")})
	var undo undoStack
	if err := writeAll(fsys, &undo, writes); err != nil {
		return err
	}
	if !t.Skill {
		// AGENTS.md is built from the prompts as now written, and written
		// under the same undo: a build that fails takes the apply back.
		agents, err := agentsWrites(c)
		if err != nil {
			return undo.fail(err, "applied")
		}
		if err := writeAll(fsys, &undo, agents); err != nil {
			return err
		}
	}
	if string(after) == string(before) {
		fmt.Fprintf(c.Stdout, "  ✓ %s unchanged\n", t.Path)
	} else {
		fmt.Fprintf(c.Stdout, "  ✓ Updated %s\n", t.Path)
	}
	if len(pending) > 0 {
		fmt.Fprintf(c.Stdout, "  ✓ Archived %s to %s\n", plural(len(pending), "entry", "entries"), archivePath)
	}
	if t.Skill {
		fmt.Fprintf(c.Stdout, "A skill source changed. The installed copies are still the old version — propagate with:\n  daisy install --update      # user-level skills (~/.claude/skills)\n  daisy init %s  # this workspace's skills\n", c.Env.Identity)
	} else {
		fmt.Fprintln(c.Stdout, "  ✓ AGENTS.md rebuilt")
	}

	message := fmt.Sprintf("Optimize %s (%s)", t.Label, mode)
	if inRoot(c.Env, t.Path) && string(after) != string(before) {
		fmt.Fprintf(c.Stdout, "  %s is in the root repository, which is published as it is pushed: review it, run 'daisy healthcheck --force', then commit it with:\n    daisy commit --not-home \"%s\"\n", t.Path, message)
	}
	hash, committed, err := repo.Commit(c.Ops, message)
	if err != nil {
		return fmt.Errorf("%v\nThe files are written but NOT committed; run: daisy commit --home \"%s\"", err, message)
	}
	reportCommit(c, message, hash, committed)
	return nil
}

func yes(answer string) bool {
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "yes"
}

// readCurrent is what apply reads of path; the home's archive.md, when it
// has none yet, reads as the template's (shipped in the root), which the
// apply then writes.
func readCurrent(c *Context, path string) ([]byte, error) {
	fsys := c.Ops.FS()
	b, err := fsys.ReadFile(path)
	if err == nil || path != c.archiveFile() || lexists(fsys, path) {
		if err != nil {
			return nil, fmt.Errorf("Error: cannot read %s: %v", path, err)
		}
		return b, nil
	}
	tmpl := filepath.Join(c.Env.Root, "templates", "home", "feedback", "archive.md")
	if b, err = fsys.ReadFile(tmpl); err != nil {
		return nil, fmt.Errorf("Error: %s does not exist and cannot be started from %s: %v", path, tmpl, err)
	}
	return b, nil
}

// fileWrite is one file apply writes and what it writes there.
type fileWrite struct {
	path string
	data []byte
}

// writeAll makes the writes in order, each through a link as ops.Rewrite
// does, pushing on undo what takes it back; the first that fails takes
// back every write before it, this call's and the ones undo already holds.
func writeAll(fsys ops.FS, undo *undoStack, writes []fileWrite) error {
	for _, w := range writes {
		old, err := fsys.ReadFile(w.path)
		existed := err == nil
		if !existed && lexists(fsys, w.path) {
			return undo.fail(fmt.Errorf("Error: cannot read %s: %v", w.path, err), "applied")
		}
		if err := ops.Rewrite(fsys, w.path, w.data, 0o644); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot write %s: %v", w.path, err), "applied")
		}
		p := w.path
		if existed {
			undo.push(func() error { return ops.Rewrite(fsys, p, old, 0o644) })
		} else {
			undo.push(func() error { return fsys.Remove(p) })
		}
	}
	return nil
}

// agentsWrites are the home's AGENTS.md and the workspace's (the one the
// agent in this session loads), built from the prompts as they are now.
func agentsWrites(c *Context) ([]fileWrite, error) {
	e := c.Env
	var writes []fileWrite
	for _, out := range []struct{ path, rebuild string }{
		{filepath.Join(e.Home, "AGENTS.md"), "daisy prompt build " + e.HomeName},
		{filepath.Join(e.Workspace, ".daisy", "AGENTS.md"), "daisy prompt build"},
	} {
		b, err := buildAgents(c, e, out.path, out.rebuild)
		if err != nil {
			return nil, fmt.Errorf("Error: AGENTS.md cannot be rebuilt: %v", err)
		}
		writes = append(writes, fileWrite{out.path, b.Text})
	}
	return writes, nil
}

// inRoot: path lies in the root and not in the home.
func inRoot(e env.Env, path string) bool {
	return !strings.HasPrefix(path, e.Home+"/") && strings.HasPrefix(path, e.Root+"/")
}

// provenancePath names a target in the archive so the name survives the
// home or the root moving: a home file relative to the home (the archive's
// own repository), a root file as `$DAISY_ROOT/…`.
func provenancePath(e env.Env, path string) string {
	if rel, ok := strings.CutPrefix(path, e.Home+"/"); ok {
		return rel
	}
	if rel, ok := strings.CutPrefix(path, e.Root+"/"); ok {
		return "$DAISY_ROOT/" + rel
	}
	return path
}
