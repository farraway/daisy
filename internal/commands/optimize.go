package commands

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
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
			fmt.Fprintf(w, "Target: %s%s\n", g.Target.Path, overrideMark(c, g.Target))
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
			fmt.Fprintf(w, "[%s] rewrite flagged: %d rules\nTarget: %s%s\n", f.Target.Label, f.Rules, f.Target.Path, overrideMark(c, f.Target))
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

// overrideMark is what the briefing says of a target in the root or
// overriding it: apply never writes the root, so a root target's apply
// writes a home override; a full override says when the root has moved on
// since the copy, for the rewrite to fold in.
func overrideMark(c *Context, t feedback.Target) string {
	switch t.Src.Form {
	case prompt.RootOnly:
		return " (root — apply writes a home override)"
	case prompt.Layered:
		return " (home override, layered)"
	case prompt.Full:
		if ledger, err := prompt.ReadLedger(c.Ops.FS(), c.Env.Home); err == nil && t.Src.Behind(c.Ops.FS(), ledger) {
			return " (home override, full, behind the root: fold the root's changes into a rewrite)"
		}
		return " (home override, full)"
	}
	return ""
}

// OptimizeApply writes the agent's proposal for one target: with --rules
// the target's `## Rules` section is replaced, with --rewrite the whole
// file. A target in the root is never written: the home's override at
// its path is (planChange). It shows the diff and asks (unless --yes, for
// text the user approved in the conversation); on approval it writes the
// target, moves the target's pending entries to the archive with their
// provenance, stamps feedback/.last-optimized, rebuilds AGENTS.md and
// the materialised prompts for a prompt and commits the home once. Every
// refusal comes before the first write; a failed write takes back the
// ones before it.
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
	ch, err := planChange(fsys, t, mode, file, proposal)
	if err != nil {
		return err
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
	ledgerPath := filepath.Join(c.Env.Home, prompt.LedgerRel)
	ledger, ledgerExisted, err := readMaybe(fsys, ledgerPath)
	if err != nil {
		return err
	}
	newLedger := ledger
	if ch.hash != "" {
		newLedger = prompt.SetLedger(ledger, t.Src.Rel, ch.hash)
	}
	ledgerChanged := string(newLedger) != string(ledger)
	if !ch.writes() && !ledgerChanged && len(pending) == 0 {
		fmt.Fprintf(c.Stdout, "Nothing to apply: %s already holds this text and no entries are pending for it.\n", ch.path)
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
	var archiveRead read // as it was read: absent when the template stands in
	if len(pending) > 0 {
		if archive, err = readCurrent(c, archivePath); err != nil {
			return err
		}
		archiveRead = read{archivePath, nil, lexists(fsys, archivePath)}
		if archiveRead.existed {
			archiveRead.data = archive
		}
	}

	if d := feedback.Diff("a/"+t.Label, "b/"+t.Label, ch.before, ch.after); d != "" {
		fmt.Fprint(c.Stdout, d)
	} else {
		fmt.Fprintf(c.Stdout, "(no change to %s)\n", t.Label)
	}
	if ch.override != "" && ch.writes() {
		fmt.Fprintf(c.Stdout, "Writes the home's %s override %s; the root's %s is not written.\n", ch.override, ch.path, t.Src.Root)
		if t.Src.Form == prompt.Layered && ch.override == "full" {
			fmt.Fprintf(c.Stdout, "The layered override is replaced by a full override: the root's later changes reach it no more (daisy healthcheck says when the root moves on).\n")
		}
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
	reads := append(ch.reads, read{c.feedbackFile(), feedbackDoc, true}, read{ledgerPath, ledger, ledgerExisted})
	if len(pending) > 0 {
		reads = append(reads, archiveRead)
	}
	for _, r := range reads {
		if r.changed(fsys) {
			return fmt.Errorf("Error: %s changed while waiting for the answer; nothing was written. Run the apply again.", r.path)
		}
	}

	var entries []feedback.Entry
	archived := map[int]bool{}
	for _, i := range pending {
		entries, archived[i] = append(entries, log.Entries[i]), true
	}
	now := c.Ops.Now()
	var writes []fileWrite
	if ch.writes() {
		writes = append(writes, fileWrite{ch.path, ch.file})
	}
	if len(pending) > 0 {
		// Nothing written: the entries are addressed by the text as it stands.
		addressed, how := t.Path, mode
		if ch.writes() {
			addressed, how = ch.path, ch.mode
		}
		provenance := fmt.Sprintf("**Addressed by:** `daisy optimize` %s — `%s` (%s)", now.Format("2006-01-02"), provenancePath(c.Env, addressed), how)
		writes = append(writes,
			fileWrite{archivePath, feedback.Archive(archive, entries, provenance)},
			fileWrite{c.feedbackFile(), log.Keep(func(i int) bool { return !archived[i] })})
	}
	writes = append(writes, fileWrite{filepath.Join(c.Env.Home, "feedback", ".last-optimized"), []byte(now.Format("2006-01-02 1504") + " " + t.Label + "\n")})
	if ledgerChanged {
		writes = append(writes, fileWrite{ledgerPath, newLedger})
	}
	var undo undoStack
	if err := writeAll(fsys, c.Env, &undo, writes); err != nil {
		return err
	}
	if !t.Skill {
		// AGENTS.md and the materialised prompts are built from the prompts
		// as now written, and written under the same undo: a build that
		// fails takes the apply back.
		agents, err := agentsWrites(c)
		if err != nil {
			return undo.fail(err, "applied")
		}
		if err := writeAll(fsys, c.Env, &undo, agents); err != nil {
			return err
		}
	}
	if ch.writes() {
		fmt.Fprintf(c.Stdout, "  ✓ Updated %s\n", ch.path)
	} else {
		fmt.Fprintf(c.Stdout, "  ✓ %s unchanged\n", t.Path)
	}
	if len(pending) > 0 {
		fmt.Fprintf(c.Stdout, "  ✓ Archived %s to %s\n", plural(len(pending), "entry", "entries"), archivePath)
	}
	if t.Skill && ch.path != t.Src.Root && t.Src.Form != prompt.HomeOwn {
		fmt.Fprintf(c.Stdout, "A home override of a root skill changed. The workspace's installed copy is still the old version — propagate with:\n  daisy init %s\n", c.Env.Identity)
	} else if t.Skill {
		fmt.Fprintf(c.Stdout, "A skill source changed. The installed copies are still the old version — propagate with 'daisy init' in each workspace; in this one:\n  daisy init %s\n", c.Env.Identity)
	} else {
		fmt.Fprintln(c.Stdout, "  ✓ AGENTS.md rebuilt")
	}

	message := fmt.Sprintf("Optimize %s (%s)", t.Label, ch.mode)
	hash, committed, err := repo.Commit(c.Ops, message)
	if err != nil {
		return fmt.Errorf("%v\nThe files are written but NOT committed; run: daisy commit \"%s\"", err, message)
	}
	reportCommit(c, message, hash, committed)
	return nil
}

// change is what an apply writes to its target: the file it writes (the
// home's override for a target in the root, which is never written) and
// what it writes there; the prompt as the agent saw it and as it will
// read after, for the diff; the mode the archive names; the root's hash
// for the ledger, when a full override is written over the root; and the
// files read, which must not change while the question waits.
type change struct {
	path, mode    string
	file          []byte
	existed       bool
	current       []byte
	before, after []byte
	override      string // "layered" or "full" when the home overrides the root's file
	hash          string
	reads         []read
}

// writes: the target file changes. A layer adding nothing over a root
// target is no override at all.
func (ch change) writes() bool {
	if !ch.existed && ch.override == "layered" && string(ch.before) == string(ch.after) {
		return false
	}
	return !ch.existed || string(ch.file) != string(ch.current)
}

// planChange is the change mode's proposal makes to t. A target whose
// text is the root's (no home file, or a layered one) gets a home
// override: --rules a layered one holding the home's rules, --rewrite a
// full one, the root's hash recorded. A home prompt, or a full override,
// is written in place as before; a --rewrite of a full override records
// the root as it is now.
func planChange(fsys ops.FS, t feedback.Target, mode, file string, proposal []byte) (change, error) {
	src := t.Src
	before, err := src.Text(fsys)
	if err != nil {
		return change{}, fmt.Errorf("Error: %v", err)
	}
	ch := change{path: t.Path, mode: mode, before: before}
	var root []byte
	if src.Overridden() || src.Form == prompt.RootOnly {
		if root, err = fsys.ReadFile(src.Root); err != nil {
			return change{}, fmt.Errorf("Error: cannot read %s: %v", src.Root, err)
		}
		ch.reads = append(ch.reads, read{src.Root, root, true})
	}
	if src.Form == prompt.RootOnly || src.Form == prompt.Layered {
		ch.path = src.Home
		ch.override = "layered"
		if mode == "rewrite" {
			ch.override = "full"
		}
		ch.mode = ch.override + " " + mode
	}
	if ch.current, ch.existed, err = readMaybe(fsys, ch.path); err != nil {
		return change{}, err
	}
	ch.reads = append(ch.reads, read{ch.path, ch.current, ch.existed})

	switch {
	case mode == "rewrite" && strings.TrimSpace(string(proposal)) == "":
		return change{}, fmt.Errorf("Error: %s is empty; a rewrite replaces the whole of %s", file, t.Label)
	case mode == "rewrite":
		ch.file, ch.after = proposal, proposal
		if src.Form != prompt.HomeOwn {
			ch.hash = prompt.Hash(root)
		}
	case ch.override == "layered":
		ch.file, err = prompt.LayeredFrom(root, proposal)
		if err == nil {
			// The layer is read back as every reader reads it.
			if ch.after, err = prompt.Merge(root, ch.file); err != nil {
				return change{}, fmt.Errorf("Error: %s cannot be written as a layered override: %v", file, err)
			}
		}
	default:
		ch.after, err = prompt.ReplaceRules(before, proposal)
		ch.file = ch.after
	}
	if err != nil {
		var te prompt.TargetError
		if errors.As(err, &te) {
			return change{}, fmt.Errorf("Error: %s: %v", src.Path(), err)
		}
		return change{}, fmt.Errorf("Error: %s %v", file, err)
	}
	// The file written is read back by its form: it holds the marker line
	// exactly when it is meant to be a layered override.
	if prompt.IsLayered(ch.file) != (ch.override == "layered") {
		return change{}, fmt.Errorf("Error: %s holds the line %s, which makes a file a layered override; a %s cannot hold it outside a code fence (use --rules on the root's target for a layered override)", file, prompt.Marker, mode)
	}
	return ch, nil
}

// read is a file as apply read it: its bytes, and whether it existed.
type read struct {
	path    string
	data    []byte
	existed bool
}

// changed: the file is not as it was read.
func (r read) changed(fsys ops.FS) bool {
	now, existed, err := readMaybe(fsys, r.path)
	return err != nil || existed != r.existed || string(now) != string(r.data)
}

// readMaybe reads path; a file that is not there is no error.
func readMaybe(fsys ops.FS, path string) ([]byte, bool, error) {
	b, err := fsys.ReadFile(path)
	if err != nil {
		if !lexists(fsys, path) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	return b, true, nil
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
// does (a nil data removes the file), pushing on undo what takes it back;
// the first that fails takes back every write before it, this call's and
// the ones undo already holds. Daisy writes by path and never at a root
// path; env.RefuseRoot is a best-effort check against a user-made link
// leading a write into the root.
func writeAll(fsys ops.FS, e env.Env, undo *undoStack, writes []fileWrite) error {
	removed := map[string]bool{} // a removal acts on the entry, not what a link there leads to
	for _, w := range writes {
		entry := w.data == nil || removed[w.path]
		removed[w.path] = removed[w.path] || w.data == nil
		if err := env.RefuseRoot(fsys, e, w.path, entry); err != nil {
			return undo.fail(err, "applied")
		}
	}
	for _, w := range writes {
		// Removing a link removes the link, live or dangling; the undo
		// puts the link back.
		if fi, err := fsys.Lstat(w.path); w.data == nil && err == nil && fi.Mode()&iofs.ModeSymlink != 0 {
			p := w.path
			target, err := fsys.Readlink(p)
			if err == nil {
				err = fsys.Remove(p)
			}
			if err != nil {
				return undo.fail(fmt.Errorf("Error: cannot remove %s: %v", p, err), "applied")
			}
			undo.push(func() error { return fsys.Symlink(target, p) })
			continue
		}
		old, err := fsys.ReadFile(w.path)
		existed := err == nil
		if !existed && lexists(fsys, w.path) {
			return undo.fail(fmt.Errorf("Error: cannot read %s: %v", w.path, err), "applied")
		}
		p := w.path
		if w.data == nil {
			if !existed {
				continue
			}
			if err := fsys.Remove(p); err != nil {
				return undo.fail(fmt.Errorf("Error: cannot remove %s: %v", p, err), "applied")
			}
			undo.push(func() error { return ops.Rewrite(fsys, p, old, 0o644) })
			continue
		}
		if err := fsys.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot write %s: %v", p, err), "applied")
		}
		if err := ops.Rewrite(fsys, p, w.data, 0o644); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot write %s: %v", p, err), "applied")
		}
		if existed {
			undo.push(func() error { return ops.Rewrite(fsys, p, old, 0o644) })
		} else {
			undo.push(func() error { return fsys.Remove(p) })
		}
	}
	return nil
}

// agentsWrites are the home's AGENTS.md and the workspace's (the one the
// agent in this session loads) with the workspace's materialised
// prompts, built from the prompts as they are now.
func agentsWrites(c *Context) ([]fileWrite, error) {
	e := c.Env
	var writes []fileWrite
	for _, out := range []struct{ path, rebuild, prompts string }{
		{filepath.Join(e.Home, "AGENTS.md"), "daisy prompt build " + e.HomeName, ""},
		{filepath.Join(e.Workspace, ".daisy", "AGENTS.md"), "daisy prompt build", filepath.Join(e.Workspace, ".daisy", "prompts")},
	} {
		b, err := buildAgents(c, e, out.path, out.rebuild, out.prompts)
		if err != nil {
			return nil, fmt.Errorf("Error: AGENTS.md cannot be rebuilt: %v", err)
		}
		writes = append(writes, fileWrite{out.path, b.Text})
		if out.prompts != "" {
			writes = append(writes, promptWrites(c.Ops.FS(), out.prompts, b)...)
		}
	}
	return writes, nil
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
