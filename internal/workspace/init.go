package workspace

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"daisy/internal/env"
	"daisy/internal/identity"
	"daisy/internal/ops"
	"daisy/internal/prompt"
	"daisy/internal/settings"
	"daisy/internal/skill"
)

// Init binds target (absolute) to the identity name: the identity's home,
// the binding links into it, AGENTS.md, the agent files, the ignore
// entries, and the identity applied to the target's repository. Progress
// goes to out, warnings to warn; the error is the message for stderr.
func Init(o ops.Ops, m env.Env, name, target string, out, warn io.Writer) error {
	p, err := planInit(o, m, name, target)
	if err != nil {
		return err
	}
	return p.apply(o, out, warn)
}

// initPlan is everything init will write, decided before the first write.
type initPlan struct {
	m                  env.Env
	homeName, home     string
	name               string // the identity
	id                 identity.Identity
	idOK               bool
	idFile             string
	target, daisyDir   string
	gitInit            bool
	reclaim            string   // a leftover to move back to .daisy/
	dropPrevious       []string // leftovers to remove, .daisy/ being there
	stages             []string // staging leftovers of interrupted inits
	bound              bool     // a binding (.daisy/ or the reclaimed one) exists
	from               string   // where that binding is read from
	oldHome, oldID     string
	agents             []byte
	agentsWarnings     []string
	rules              map[string][]byte // cursor rule name → content
	command            []byte
	skills             map[string]string // name → source dir
	staleStubs         []string
	settings           []byte
	gitignore          []byte
	gitignoreChanged   bool
	gitignoreNew       bool
	cursorignore       []byte
	cursorignoreChange bool
}

func planInit(o ops.Ops, m env.Env, name, target string) (*initPlan, error) {
	fs := o.FS()
	p := &initPlan{m: m, name: name, target: target, daisyDir: filepath.Join(target, ".daisy")}
	var err error
	if p.homeName, err = identity.HomeOf(o, m, name); err != nil {
		return nil, err
	}
	he, err := env.ForHome(o, p.homeName)
	if err != nil {
		return nil, err
	}
	p.home = he.Home
	// The default identity is optional (a home may not have one yet); a
	// named identity is what was asked for, so it must be whole.
	p.idFile, _ = env.IdentityFile(p.home, p.homeName, name)
	p.id, err = identity.Read(fs, p.idFile)
	p.idOK = err == nil
	if err != nil && name != p.homeName {
		return nil, err
	}

	if !isDir(fs, target) {
		return nil, fmt.Errorf("Error: %s is not a directory", target)
	}
	if identity.IsRepoRoot(o, target) && identity.IsLinkedWorktree(o, target) {
		return nil, fmt.Errorf("Error: %s is a linked worktree; its git config belongs to the repository at %s\n  Run 'daisy init %s' in that repository's main checkout instead", target, identity.CommonDir(o, target), name)
	}
	if isDir(fs, p.daisyDir) && !env.IsWorkspaceDir(fs, m.Root, target) {
		return nil, fmt.Errorf("Error: %s is the Daisy root, not a workspace binding; to bind the root itself, run 'daisy init %s' inside it", p.daisyDir, name)
	}
	if fi, err := fs.Lstat(p.daisyDir); err == nil && fi.Mode()&iofs.ModeSymlink != 0 {
		return nil, fmt.Errorf("Error: %s is a symlink; init binds a directory of its own — move it aside first", p.daisyDir)
	} else if err == nil && !fi.IsDir() {
		return nil, fmt.Errorf("Error: %s exists and is not a directory; move it aside first", p.daisyDir)
	}
	if err := p.planLeftovers(fs); err != nil {
		return nil, err
	}
	if err := p.planBinding(o); err != nil {
		return nil, err
	}
	if err := p.planFiles(o); err != nil {
		return nil, err
	}
	p.gitInit = !identity.IsRepoRoot(o, target)
	return p, nil
}

// planLeftovers: what an interrupted swap left — .daisy/ moved aside, or a
// staging directory. With no .daisy/, one aside is the binding; two cannot
// be told apart.
func (p *initPlan) planLeftovers(fs ops.FS) error {
	var previous []string
	for _, l := range env.InitLeftovers(fs, p.target) {
		if strings.Contains(filepath.Base(l), ".previous.") {
			previous = append(previous, l)
		} else {
			p.stages = append(p.stages, l)
		}
	}
	p.from = p.daisyDir
	switch {
	case exists(fs, p.daisyDir):
		p.dropPrevious = previous
	case len(previous) > 1:
		return fmt.Errorf("Error: no .daisy/ here but %d directories an earlier init moved aside: %s\n  Move the one holding the binding back to .daisy/ and remove the others, then re-run", len(previous), strings.Join(previous, " "))
	case len(previous) == 1:
		p.reclaim, p.from = previous[0], previous[0]
	}
	p.bound = isDir(fs, p.from)
	return nil
}

// planBinding reads the binding there is and refuses what init could not
// replace: a directory where it writes a file of its own.
func (p *initPlan) planBinding(o ops.Ops) error {
	fs := o.FS()
	if !p.bound {
		return nil
	}
	owned := []string{homeFile, identityFile, recordFile, beforeFile, agentsFile}
	for _, l := range env.Links {
		owned = append(owned, l.Link)
	}
	for _, f := range owned {
		if fi, err := fs.Lstat(filepath.Join(p.from, f)); err == nil && fi.IsDir() {
			return fmt.Errorf("Error: %s is a directory where init writes a file of its own; move it aside, then re-run", filepath.Join(p.daisyDir, f))
		}
	}
	if isFile(fs, filepath.Join(p.from, homeFile)) {
		b, err := fs.ReadFile(filepath.Join(p.from, homeFile))
		if err != nil {
			return fmt.Errorf("Error: cannot read %s: %v", filepath.Join(p.daisyDir, homeFile), err)
		}
		p.oldHome = strings.TrimRight(string(b), "\n")
		p.oldID = env.WorkspaceIdentity(fs, p.from, p.oldHome)
	}
	return nil
}

// planFiles builds every file init writes outside the binding's swap.
func (p *initPlan) planFiles(o ops.Ops) error {
	fs := o.FS()
	built, err := prompt.Build(fs, prompt.Spec{
		Root: p.m.Root, HomeName: p.homeName, Home: p.home,
		Output: filepath.Join(p.daisyDir, agentsFile), Rebuild: "daisy prompt build", Built: o.Now(),
	})
	if err != nil {
		return err
	}
	p.agents, p.agentsWarnings = built.Text, built.Warnings

	read := func(name string) ([]byte, error) {
		b, err := fs.ReadFile(filepath.Join(p.m.Root, "templates", name))
		if err != nil {
			return nil, fmt.Errorf("Error: cannot read the template %s: %v", filepath.Join(p.m.Root, "templates", name), err)
		}
		return b, nil
	}
	p.rules = map[string][]byte{}
	for _, r := range cursorRules {
		if p.rules[r.Name], err = read(r.Template); err != nil {
			return err
		}
	}
	if p.command, err = read("claude-command.md"); err != nil {
		return err
	}

	p.skills = skill.Discover(fs, filepath.Join(p.home, "skills"))
	for n, dir := range p.skills {
		// A skill's stub sits beside the core rules and its name is a line
		// of .gitignore: it can be neither a core rule nor two lines.
		if isCoreRule(n+".mdc") || strings.ContainsAny(n, "\r\n") {
			return fmt.Errorf("Error: the home's skill %s cannot be installed under the name %q (a core Cursor rule's name, or not one line); rename its directory", dir, n)
		}
		if _, err := fs.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil {
			return fmt.Errorf("Error: cannot read %s: %v", filepath.Join(dir, "SKILL.md"), err)
		}
	}
	// A stub of a skill the home no longer has is swept with its copy; a
	// rule not in init's stub form is the user's and stays.
	for _, n := range stubs(fs, p.target) {
		if p.skills[n] == "" {
			p.staleStubs = append(p.staleStubs, n)
		}
	}

	path := filepath.Join(p.target, settingsFile)
	existing, err := fs.ReadFile(path)
	if err != nil && exists(fs, path) {
		return fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	if p.settings, err = settings.Allow(existing, allowRules, []string{"Bash(" + p.m.Root + "/daisy/scripts/*)"}); err != nil {
		return fmt.Errorf("Error: %s cannot be read as settings: %v\n  Fix or remove it, then re-run 'daisy init %s'", path, err, p.name)
	}

	if err := p.checkWritable(fs); err != nil {
		return err
	}

	ignore := append([]string{}, gitignoreEntries...)
	for _, n := range sortedNames(p.skills) {
		ignore = append(ignore, skillEntries(n)...)
	}
	if p.gitignore, p.gitignoreChanged, p.gitignoreNew, err = planBlock(fs, filepath.Join(p.target, ".gitignore"), gitignoreHeader, ignore); err != nil {
		return err
	}
	p.cursorignore, p.cursorignoreChange, _, err = planBlock(fs, filepath.Join(p.target, ".cursorignore"), cursorignoreHeader, cursorignoreLines)
	return err
}

// checkWritable refuses, before anything is written, a path init must
// write that cannot become what init writes there: a file where it needs
// a directory on the way, or a directory where it writes a file.
func (p *initPlan) checkWritable(fs ops.FS) error {
	files := []string{claudeCommand, settingsFile, ".gitignore", ".cursorignore", ".claude/skills/.probe"}
	for _, r := range cursorRules {
		files = append(files, ".cursor/rules/"+r.Name)
	}
	for _, n := range sortedNames(p.skills) {
		files = append(files, ".cursor/rules/"+n+".mdc", ".claude/skills/"+n+"/SKILL.md")
	}
	for _, rel := range files {
		dir := p.target
		parts := strings.Split(rel, "/")
		for _, part := range parts[:len(parts)-1] {
			dir = filepath.Join(dir, part)
			if exists(fs, dir) && !isDir(fs, dir) {
				return fmt.Errorf("Error: %s is not a directory, and init writes %s; move it aside, then re-run", dir, filepath.Join(p.target, rel))
			}
		}
		path := filepath.Join(p.target, rel)
		if strings.HasPrefix(rel, ".claude/skills/") {
			continue // a skill's copy is replaced whole
		}
		// Under .cursor/rules/ a link is replaced (Lstat); elsewhere a file
		// is written through a link to it (Rewrite), so the target counts.
		stat := fs.Stat
		if strings.HasPrefix(rel, ".cursor/rules/") {
			stat = fs.Lstat
		}
		if fi, err := stat(path); err == nil && fi.IsDir() {
			return fmt.Errorf("Error: %s is a directory where init writes a file; move it aside, then re-run", path)
		}
	}
	return nil
}

func planBlock(fs ops.FS, path, header string, entries []string) (content []byte, changed, created bool, err error) {
	existing, err := fs.ReadFile(path)
	if err != nil {
		if exists(fs, path) {
			return nil, false, false, fmt.Errorf("Error: cannot read %s: %v", path, err)
		}
		created = true
	}
	content, changed = addBlock(existing, header, entries)
	return content, changed || created, created, nil
}

func isCoreRule(name string) bool {
	for _, r := range cursorRules {
		if r.Name == name {
			return true
		}
	}
	return false
}

func (p *initPlan) apply(o ops.Ops, out, warn io.Writer) error {
	fs := o.FS()
	if p.reclaim != "" {
		if err := fs.Rename(p.reclaim, p.daisyDir); err != nil {
			return fmt.Errorf("Error: cannot move %s back to %s: %v", p.reclaim, p.daisyDir, err)
		}
		fmt.Fprintf(warn, "  ⚠ Reclaimed %s as .daisy/ (an earlier init was interrupted mid-swap)\n", p.reclaim)
	}
	for _, l := range p.dropPrevious {
		if err := fs.RemoveAll(l); err != nil {
			return fmt.Errorf("Error: could not remove %s; remove it by hand: %v", l, err)
		}
		fmt.Fprintf(warn, "  ⚠ Removed %s (what .daisy/ held before an earlier init's swap)\n", l)
	}

	fmt.Fprintf(out, "Initializing Daisy in: %s\n  Home: %s\n  Identity: %s\n", p.target, p.homeName, p.name)
	if p.gitInit {
		if _, errb, err := o.Exec(p.target, "git", "init", "-q"); err != nil {
			return fmt.Errorf("Error: git init in %s failed: %s", p.target, strings.TrimSpace(string(errb)))
		}
		fmt.Fprintln(out, "  ✓ Initialized git repository")
	}
	switch {
	case p.oldHome == "":
	case p.oldHome == p.homeName && p.oldID == p.name:
		fmt.Fprintf(out, "  ✓ Already initialized with identity: %s\n", p.name)
	case p.oldHome == p.homeName:
		fmt.Fprintf(out, "  ↻ Switching identity: %s → %s\n", p.oldID, p.name)
	default:
		fmt.Fprintf(out, "  ↻ Switching home: %s → %s\n", p.oldHome, p.homeName)
	}
	for _, w := range p.agentsWarnings {
		fmt.Fprintln(warn, w)
	}
	if err := p.writeFiles(fs, out); err != nil {
		return err
	}
	return p.bind(o, out, warn)
}

func (p *initPlan) writeFiles(fs ops.FS, out io.Writer) error {
	at := func(rel ...string) string { return filepath.Join(append([]string{p.target}, rel...)...) }
	write := func(path string, b []byte) error {
		if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := ops.Rewrite(fs, path, b, 0o644); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		return nil
	}
	// The Cursor rules and stubs are files of the workspace's own (a stub
	// is the record of what init installed): a link standing there is
	// replaced, never written through.
	own := func(path string) error {
		if fi, err := fs.Lstat(path); err == nil && fi.Mode()&iofs.ModeSymlink != 0 {
			if err := fs.Remove(path); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
		}
		return nil
	}
	for _, r := range cursorRules {
		if err := own(at(".cursor", "rules", r.Name)); err != nil {
			return err
		}
		if err := write(at(".cursor", "rules", r.Name), p.rules[r.Name]); err != nil {
			return err
		}
		fmt.Fprintf(out, "  ✓ Installed Cursor rule: %s\n", r.Name)
	}
	for _, n := range obsoleteRules {
		if isFile(fs, at(".cursor", "rules", n)) {
			if err := fs.Remove(at(".cursor", "rules", n)); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			fmt.Fprintf(out, "  ✓ Removed obsolete Cursor rule: %s\n", n)
		}
	}
	if err := write(at(claudeCommand), p.command); err != nil {
		return err
	}
	fmt.Fprintln(out, "  ✓ Installed Claude command: daisy")

	if err := fs.MkdirAll(at(".claude", "skills"), 0o755); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	for _, n := range sortedNames(p.skills) {
		dst := at(".claude", "skills", n)
		if err := fs.RemoveAll(dst); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := ops.CopyTree(fs, p.skills[n], dst); err != nil {
			return fmt.Errorf("Error: copying the skill %s: %v", n, err)
		}
		doc, err := fs.ReadFile(filepath.Join(dst, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := own(at(".cursor", "rules", n+".mdc")); err != nil {
			return err
		}
		if err := write(at(".cursor", "rules", n+".mdc"), stub(n, doc)); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "  ✓ Installed %d skill(s) into .claude/skills/ and .cursor/rules/\n", len(p.skills))
	for _, n := range p.staleStubs {
		if err := fs.Remove(at(".cursor", "rules", n+".mdc")); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := fs.RemoveAll(at(".claude", "skills", n)); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	if len(p.staleStubs) > 0 {
		fmt.Fprintf(out, "  ✓ Swept %d stale skill stub(s) from .claude/skills/ and .cursor/rules/\n", len(p.staleStubs))
	}

	if err := write(at(settingsFile), p.settings); err != nil {
		return err
	}
	fmt.Fprintln(out, "  ✓ Configured .claude/settings.local.json with Daisy permissions")
	if p.gitignoreChanged {
		if err := write(at(".gitignore"), p.gitignore); err != nil {
			return err
		}
		if p.gitignoreNew {
			fmt.Fprintln(out, "  ✓ Created .gitignore")
		}
		fmt.Fprintln(out, "  ✓ Added daisy paths to .gitignore")
	}
	if p.cursorignoreChange {
		if err := write(at(".cursorignore"), p.cursorignore); err != nil {
			return err
		}
		fmt.Fprintln(out, "  ✓ Configured .cursorignore to index daisy paths")
	}
	return nil
}

// bind writes the binding and applies the identity, as a whole or not at
// all. The new .daisy/ — whatever the old one held, with init's own files
// rewritten — is prepared in a staging directory beside it; the identity
// an earlier init applied (its record; for a binding from before records,
// the identity file it names) is taken back, and so is any an interrupted
// init's stage records; the new one is recorded in the stage and applied;
// then the stage is swapped in by rename. Any failure before the swap
// completes restores the three keys as they were, a hand-set value
// included, puts .daisy/ back and removes the stage.
func (p *initPlan) bind(o ops.Ops, out, warn io.Writer) (err error) {
	fs := o.FS()
	var stage, aside string
	var snapshot identity.Values
	touched, bound := false, false
	defer func() {
		if bound || err == nil {
			return
		}
		var b strings.Builder
		b.WriteString(err.Error())
		b.WriteString("\nError: init did not finish; the binding and git identity are as they were before this run's final step")
		if aside != "" && isDir(fs, aside) {
			if exists(fs, p.daisyDir) {
				fs.RemoveAll(p.daisyDir)
			}
			if e := fs.Rename(aside, p.daisyDir); e != nil {
				fmt.Fprintf(&b, "\n  the previous .daisy/ is at %s and could not be moved back", aside)
			}
		}
		if touched {
			if e := identity.Restore(o, p.target, snapshot); e != nil {
				b.WriteString("\n  the previous git identity could not be put back; see 'daisy status'")
			}
		}
		if stage != "" {
			if e := fs.RemoveAll(stage); e != nil {
				fmt.Fprintf(&b, "\n  the staging directory %s could not be removed; remove it by hand", stage)
			}
		}
		err = errors.New(b.String())
	}()

	// 1. The new binding, beside .daisy/.
	if stage, err = makeStage(o, p.target); err != nil {
		return err
	}
	if err = p.fillStage(fs, stage); err != nil {
		return err
	}

	// 2. The identity: previous ones taken back, the new one recorded, then
	//    applied — so whatever is applied is claimed by a record (the
	//    stage's, until the swap) at every instant.
	if snapshot, err = identity.Snapshot(o, p.target); err != nil {
		return err
	}
	touched = true
	for _, s := range p.stages {
		c := readClaim(fs, s)
		cleared, err := identity.Unset(o, p.target, c.id, c.before)
		if err != nil {
			return err
		}
		if len(cleared) > 0 {
			fmt.Fprintf(out, "  ✓ Cleared the git identity an interrupted init's record names (%s)\n", strings.Join(cleared, ", "))
		}
	}
	old := claim{}
	if p.bound {
		old = p.oldClaim(o, warn)
	}
	cleared, err := identity.Unset(o, p.target, old.id, old.before)
	if err != nil {
		return err
	}
	if len(cleared) > 0 {
		if p.oldHome != "" {
			fmt.Fprintf(out, "  ✓ Cleared previous git identity (%s)\n", strings.Join(cleared, ", "))
		} else {
			fmt.Fprintf(out, "  ✓ Cleared a git identity an earlier init left here (%s)\n", strings.Join(cleared, ", "))
		}
	}
	if p.idOK {
		before, err := identity.Snapshot(o, p.target)
		if err != nil {
			return err
		}
		if err := fs.WriteFile(filepath.Join(stage, recordFile), p.id.Record(), 0o644); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if len(before) > 0 {
			if err := fs.WriteFile(filepath.Join(stage, beforeFile), before.Format(), 0o644); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
		}
		if err := identity.Apply(o, p.target, p.id); err != nil {
			return err
		}
		p.reportIdentity(o, out, warn)
	} else {
		fmt.Fprintf(warn, "  ⚠ Home '%s' has no complete default identity at %s (git identity not configured)\n    Write name= and email= lines there, then re-run 'daisy init %s'\n", p.homeName, p.idFile, p.homeName)
	}

	// 3. The swap.
	if isDir(fs, p.daisyDir) {
		if aside, err = asideName(o, p.target); err != nil {
			return err
		}
		if err = fs.Rename(p.daisyDir, aside); err != nil {
			aside = ""
			return fmt.Errorf("Error: %v", err)
		}
	}
	if err = fs.Rename(stage, p.daisyDir); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	bound = true
	fmt.Fprintf(out, "  ✓ Generated .daisy/AGENTS.md\n  ✓ Bound .daisy/ to home: %s\n", p.homeName)
	// The binding is in place; what cannot be tidied is said, not failed on.
	leftovers := append([]string{}, p.stages...)
	if aside != "" {
		leftovers = append(leftovers, aside)
	}
	for _, l := range leftovers {
		if fs.RemoveAll(l) != nil {
			fmt.Fprintf(warn, "  ⚠ %s could not be removed; remove it by hand (the next init or clean will try again)\n", l)
		}
	}
	fmt.Fprint(out, "\nDone. Daisy is ready in this workspace.\nStart a Cursor session and say: \"Daisy, start a new day\"\nOr use /daisy in Claude Code: \"/daisy start a new day\"\n")
	return nil
}

// fillStage copies the binding there is into the stage and writes init's
// own files over it, each removed first: a symlink standing in its place
// is replaced, never written through.
func (p *initPlan) fillStage(fs ops.FS, stage string) error {
	mode := iofs.FileMode(0o755)
	if isDir(fs, p.daisyDir) {
		entries, err := fs.ReadDir(p.daisyDir)
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		for _, e := range entries {
			if err := ops.CopyTree(fs, filepath.Join(p.daisyDir, e.Name()), filepath.Join(stage, e.Name())); err != nil {
				return fmt.Errorf("Error: copying .daisy/ to %s: %v", stage, err)
			}
		}
		fi, _ := fs.Stat(p.daisyDir)
		mode = fi.Mode().Perm()
	}
	if err := fs.Chmod(stage, mode); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	for _, f := range []string{homeFile, identityFile, recordFile, beforeFile, agentsFile} {
		if err := removeFile(fs, filepath.Join(stage, f)); err != nil {
			return err
		}
	}
	if err := fs.WriteFile(filepath.Join(stage, homeFile), []byte(p.homeName+"\n"), 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	// The default identity is the absence of .daisy/identity.
	if p.name != p.homeName {
		if err := fs.WriteFile(filepath.Join(stage, identityFile), []byte(p.name+"\n"), 0o644); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	for _, l := range env.Links {
		if err := removeFile(fs, filepath.Join(stage, l.Link)); err != nil {
			return err
		}
		if err := fs.Symlink(filepath.Join(p.home, l.Target), filepath.Join(stage, l.Link)); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	if err := fs.WriteFile(filepath.Join(stage, agentsFile), p.agents, 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	return nil
}

func removeFile(fs ops.FS, p string) error {
	if fi, err := fs.Lstat(p); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("Error: %s is a directory where init writes a file of its own", p)
		}
		if err := fs.Remove(p); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	return nil
}

// oldClaim is what the binding being replaced applied; it warns when only
// the identity file as it is now, or nothing, can stand in for the record.
func (p *initPlan) oldClaim(o ops.Ops, warn io.Writer) claim {
	c := readClaim(o.FS(), p.daisyDir)
	if c.ok {
		return c
	}
	if f, ok := p.boundFile(o); ok {
		if id, err := identity.Read(o.FS(), f); err == nil {
			fmt.Fprintf(warn, "  ⚠ No readable .daisy/gitconfig record of the previous identity; unsetting against %s as it is now\n", f)
			c.id, c.file, c.ok = id, f, true
			return c
		}
	}
	if p.oldHome != "" || exists(o.FS(), filepath.Join(p.daisyDir, recordFile)) {
		fmt.Fprintf(warn, "  ⚠ No readable .daisy/gitconfig record of the previous identity, and no identity file resolves for home '%s' identity '%s'; only a key of Daisy's form can be recognised as its\n", p.oldHome, p.oldID)
	}
	return c
}

// boundFile is the identity file the old binding names, when its home resolves.
func (p *initPlan) boundFile(o ops.Ops) (string, bool) {
	return boundIdentityFile(o, p.m, p.oldHome, p.oldID)
}

func boundIdentityFile(o ops.Ops, m env.Env, homeName, id string) (string, bool) {
	if homeName == "" {
		return "", false
	}
	m.HomeName = homeName
	home, err := env.HomeDir(o, m)
	if err != nil {
		return "", false
	}
	f, err := env.IdentityFile(filepath.Clean(home), homeName, id)
	return f, err == nil
}

// readClaim is the record in a binding (or a stage) and what it says the
// keys held before; ok is false when there is no readable record.
func readClaim(fs ops.FS, dir string) claim {
	c := claim{before: readBefore(fs, dir)}
	if id, err := identity.Read(fs, filepath.Join(dir, recordFile)); err == nil {
		c.id, c.file, c.ok = id, filepath.Join(dir, recordFile), true
	}
	return c
}

func (p *initPlan) reportIdentity(o ops.Ops, out, warn io.Writer) {
	if p.id.SSHKey == "" {
		fmt.Fprintf(out, "  ✓ Set local git identity: %s <%s>\n", p.id.Name, p.id.Email)
		return
	}
	fmt.Fprintf(out, "  ✓ Set local git identity: %s <%s>, pushing with key %s\n", p.id.Name, p.id.Email, p.id.SSHKey)
	// The key may be made later; until then every push fails, and this is
	// where the reason is on record. Only `~/...` and absolute paths are checked.
	key := ""
	switch {
	case strings.HasPrefix(p.id.SSHKey, "~/"):
		key = filepath.Join("/"+o.Getenv("HOME"), p.id.SSHKey[2:])
	case filepath.IsAbs(p.id.SSHKey):
		key = p.id.SSHKey
	}
	if key != "" && !exists(o.FS(), key) {
		fmt.Fprintf(warn, "  ⚠ SSH key %s not found; pushes from this workspace will fail until it exists\n", p.id.SSHKey)
	}
}

const stageChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// makeStage creates .daisy.stage.<6 letters or digits>, the shape the
// leftover rule recognises, exclusively.
func makeStage(o ops.Ops, dir string) (string, error) {
	seed := uint64(o.Now().UnixNano())
	for try := uint64(0); try < 100; try++ {
		n, name := seed+try*7919, make([]byte, 6)
		for i := range name {
			name[i] = stageChars[n%uint64(len(stageChars))]
			n /= uint64(len(stageChars))
		}
		stage := filepath.Join(dir, ".daisy.stage."+string(name))
		if err := o.FS().Mkdir(stage, 0o700); err == nil {
			return stage, nil
		} else if !exists(o.FS(), stage) {
			return "", fmt.Errorf("Error: cannot create a staging directory in %s: %v", dir, err)
		}
	}
	return "", fmt.Errorf("Error: cannot create a staging directory in %s", dir)
}

// asideName is a free .daisy.previous.<digits>.
func asideName(o ops.Ops, dir string) (string, error) {
	n := o.Now().UnixNano() % 1_000_000_000
	for try := int64(0); try < 100; try++ {
		p := filepath.Join(dir, ".daisy.previous."+strconv.FormatInt(n+try, 10))
		if !exists(o.FS(), p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("Error: no free .daisy.previous.* name in %s", dir)
}
