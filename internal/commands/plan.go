package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"daisy/internal/gitx"
	"daisy/internal/ops"
	"daisy/internal/plan"
)

// Paths the plan commands share.
func (c *Context) plansDir() string     { return filepath.Join(c.Env.Home, "plans") }
func (c *Context) planLink() string     { return filepath.Join(c.Env.Workspace, "PLAN.md") }
func (c *Context) planTemplate() string { return filepath.Join(c.Env.Root, "templates", "plan.md") }

// errPlanExists: the workspace already has a plan, or a link where one goes.
var errPlanExists = errors.New("Error: PLAN.md already exists in this workspace\nArchive it first: /daisy archive PLAN.md")

// PlanNew creates a plan from templates/plan.md in the home's plans/ and
// links it as the workspace's PLAN.md; with --spec it writes an unregistered
// draft `<slug>_PLAN.md` into the workspace instead. Every refusal is
// decided before the first write; a failed write takes back the ones before
// it. Nothing is committed: the next home commit sweeps the plan up.
func PlanNew(c *Context) error {
	fsys := c.Ops.FS()
	description := c.Args[0]
	project := ""
	if len(c.Args) > 1 {
		project = strings.TrimPrefix(c.Args[1], "+")
	}
	slug := plan.Slug(description)
	if slug == "" {
		return plan.ErrEmptySlug
	}
	if err := plan.CheckProject(project); err != nil {
		return err
	}
	tmpl, err := fsys.ReadFile(c.planTemplate())
	if err != nil {
		return fmt.Errorf("Error: Plan template not found at %s", c.planTemplate())
	}
	now := c.Ops.Now()
	spec := c.Flag("--spec")

	var name, dest, entry string
	if spec {
		name = slug + "_PLAN.md"
		dest, entry = filepath.Join(c.Env.Workspace, name), "*_PLAN.md"
		if lexists(fsys, dest) {
			return fmt.Errorf("Error: %s already exists in this workspace", name)
		}
	} else {
		if lexists(fsys, c.planLink()) {
			return errPlanExists
		}
		name = plan.FileName(now.Format("060102150405"), project, slug)
		dest, entry = filepath.Join(c.plansDir(), name), "PLAN.md"
		if err := plan.CheckName(name); err != nil {
			return err
		}
		if lexists(fsys, dest) {
			return fmt.Errorf("Error: %s already exists", dest)
		}
	}
	ignore, ignoreNote, err := ignoreWith(fsys, filepath.Join(c.Env.Workspace, ".gitignore"), entry)
	if err != nil {
		return err
	}

	content := plan.Fill(string(tmpl), description, project, now.Format("2006-01-02 15:04"), c.Env.Root)
	if err := fsys.WriteFile(dest, []byte(content), 0o644); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", dest, err)
	}
	var undo undoStack
	undo.push(func() error { return fsys.Remove(dest) })
	if !spec {
		if err := fsys.Symlink(filepath.Join(".daisy", "plans", name), c.planLink()); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot link %s: %v", c.planLink(), err), "created")
		}
		undo.push(func() error { return fsys.Remove(c.planLink()) })
	}
	if ignore != nil {
		if err := ops.Rewrite(fsys, filepath.Join(c.Env.Workspace, ".gitignore"), ignore, 0o644); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot write %s: %v", filepath.Join(c.Env.Workspace, ".gitignore"), err), "created")
		}
		fmt.Fprintln(c.Stdout, ignoreNote)
	}
	if spec {
		fmt.Fprintf(c.Stdout, "✅ Spec created: %s\n   Location: %s\n   Not registered with Daisy — promote later with: daisy plan pickup %s\n", name, dest, dest)
	} else {
		fmt.Fprintf(c.Stdout, "✅ Plan created: %s\n   Linked as: %s\n", name, c.planLink())
	}
	return nil
}

// ignoreWith is the workspace .gitignore with entry added as a line of its
// own, and the line saying so; nil content when entry is already a line of
// it. A file without a final newline gets one before the entry.
func ignoreWith(fsys ops.FS, path, entry string) (content []byte, note string, err error) {
	b, err := fsys.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !lexists(fsys, path):
		return []byte(entry + "\n"), "  ✓ Created .gitignore with " + entry, nil
	case err != nil:
		return nil, "", fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l == entry {
			return nil, "", nil
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return append(b, entry+"\n"...), "  ✓ Added " + entry + " to .gitignore", nil
}

// PlanPickup promotes a spec into a tracked plan: a copy in the home's
// plans/ named from the spec's H1, linked as PLAN.md, and one home commit.
// The original is never touched. A relative path is the working
// directory's.
func PlanPickup(c *Context) error {
	fsys := c.Ops.FS()
	path := c.Args[0]
	src := path
	if !filepath.IsAbs(src) {
		cwd, err := c.Ops.Getwd()
		if err != nil {
			return fmt.Errorf("Error: cannot read the working directory: %v", err)
		}
		src = filepath.Join(cwd, src)
	}
	fi, err := fsys.Stat(src)
	if err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("Error: File not found: %s", path)
	}
	if lexists(fsys, c.planLink()) {
		return errPlanExists
	}
	b, err := fsys.ReadFile(src)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	description, project, ok := plan.Header(string(b))
	if !ok {
		return fmt.Errorf("Error: Could not find a '# {Description}' header in %s", path)
	}
	slug := plan.Slug(description)
	if slug == "" {
		return plan.ErrEmptySlug
	}
	if err := plan.CheckProject(project); err != nil {
		return err
	}
	name := plan.FileName(c.Ops.Now().Format("060102150405"), project, slug)
	if err := plan.CheckName(name); err != nil {
		return err
	}
	dest := filepath.Join(c.plansDir(), name)
	if lexists(fsys, dest) {
		return fmt.Errorf("Error: %s already exists", dest)
	}
	repo, err := homeRepo(c)
	if err != nil {
		return err
	}

	// The copy takes the spec's mode under the umask, as `cp` did.
	if err := fsys.WriteFile(dest, b, fi.Mode().Perm()); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", dest, err)
	}
	if err := fsys.Symlink(filepath.Join(".daisy", "plans", name), c.planLink()); err != nil {
		undo := undoStack{func() error { return fsys.Remove(dest) }}
		return undo.fail(fmt.Errorf("Error: cannot link %s: %v", c.planLink(), err), "picked up")
	}
	fmt.Fprintf(c.Stdout, "✅ Picked up: %s → %s\n   Linked as: %s\n   Original left in place at: %s\n", filepath.Base(path), name, c.planLink(), path)
	return commitPlan(c, repo, "Pick up plan: "+name)
}

// PlanArchive moves the plan PLAN.md links to into plans/archive/, removes
// the link, repoints every project file's link to it and commits the home
// once. Every refusal is decided before the first write; a failed write
// takes back the ones before it.
func PlanArchive(c *Context) error {
	fsys := c.Ops.FS()
	link := c.planLink()
	fi, err := fsys.Lstat(link)
	if err != nil {
		return errors.New("Error: PLAN.md does not exist in this workspace")
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		return errors.New("Error: PLAN.md exists but is not a symlink (not a Daisy plan)")
	}
	target, err := fsys.Readlink(link)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", link, err)
	}
	name := filepath.Base(target)
	src := filepath.Join(c.plansDir(), name)
	dest := filepath.Join(c.plansDir(), "archive", name)
	if fi, err := fsys.Stat(src); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("Error: Plan file not found at %s\nThe symlink may be broken. Remove PLAN.md manually to proceed.", src)
	}
	if lexists(fsys, dest) {
		return fmt.Errorf("Error: %s already exists; nothing was archived", dest)
	}
	edits, err := repointEdits(fsys, c.Env.Home, name)
	if err != nil {
		return err
	}
	repo, err := homeRepo(c)
	if err != nil {
		return err
	}

	if err := fsys.Rename(src, dest); err != nil {
		return fmt.Errorf("Error: cannot move %s to %s: %v", src, dest, err)
	}
	undo := undoStack{func() error { return fsys.Rename(dest, src) }}
	if err := fsys.Remove(link); err != nil {
		return undo.fail(fmt.Errorf("Error: cannot remove %s: %v", link, err), "archived")
	}
	undo.push(func() error { return fsys.Symlink(target, link) })
	for _, e := range edits {
		if err := ops.Rewrite(fsys, e.path, []byte(e.after), 0o644); err != nil {
			return undo.fail(fmt.Errorf("Error: cannot repoint %s: %v", e.path, err), "archived")
		}
		undo.push(func() error { return ops.Rewrite(fsys, e.path, []byte(e.before), 0o644) })
	}
	fmt.Fprintf(c.Stdout, "✅ Archived: %s\n", name)
	if len(edits) > 0 {
		fmt.Fprintf(c.Stdout, "  ✓ Repointed links in %d project file(s)\n", len(edits))
	}
	return commitPlan(c, repo, "Archive plan: "+name)
}

// undoStack takes back a command's writes, newest first.
type undoStack []func() error

func (u *undoStack) push(f func() error) { *u = append(*u, f) }

// fail runs the undos and returns err with what they left: `Nothing was
// <what>.` when every one succeeded, else each that failed.
func (u undoStack) fail(err error, what string) error {
	var failed []string
	for i := len(u) - 1; i >= 0; i-- {
		if e := u[i](); e != nil {
			failed = append(failed, e.Error())
		}
	}
	if len(failed) == 0 {
		return fmt.Errorf("%v\nNothing was %s.", err, what)
	}
	return fmt.Errorf("%v\nThe rollback failed too, so this is only partly %s: %s", err, what, strings.Join(failed, "; "))
}

// edit is one project file's content before and after the repoint.
type edit struct{ path, before, after string }

// repointEdits is the project files (projects/*.md, then
// projects/_archive/*.md, each in name order) whose links name the plan.
func repointEdits(fsys ops.FS, home, name string) ([]edit, error) {
	var edits []edit
	for _, dir := range []string{filepath.Join(home, "projects"), filepath.Join(home, "projects", "_archive")} {
		entries, _ := fsys.ReadDir(dir) // no directory, no project files
		for _, en := range entries {
			p := filepath.Join(dir, en.Name())
			// `*.md`, as the bash's glob: a dotfile is not a project file.
			if !strings.HasSuffix(en.Name(), ".md") || strings.HasPrefix(en.Name(), ".") {
				continue
			}
			if fi, err := fsys.Stat(p); err != nil || !fi.Mode().IsRegular() {
				continue
			}
			b, err := fsys.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("Error: cannot read %s: %v\nNothing was archived.", p, err)
			}
			if after, ok := plan.Repoint(string(b), name); ok {
				edits = append(edits, edit{p, string(b), after})
			}
		}
	}
	return edits, nil
}

// commitPlan commits the home once and prints what `daisy commit` prints.
func commitPlan(c *Context, repo gitx.Repo, message string) error {
	hash, committed, err := repo.Commit(c.Ops, message)
	if err != nil {
		return fmt.Errorf("%v\nThe plan files are written but NOT committed; run: daisy commit --home \"%s\"", err, message)
	}
	reportCommit(c, message, hash, committed)
	return nil
}

func lexists(fsys ops.FS, p string) bool {
	_, err := fsys.Lstat(p)
	return err == nil
}

// The plan commands' --healthcheck hooks: the home's plans/ (and its
// archive/, for archive) and, for new, the template.
var (
	PlanNewHealth = readOnly(func(c *Context) error {
		if err := requireDir(c, c.plansDir()); err != nil {
			return err
		}
		if fi, err := c.Ops.FS().Stat(c.planTemplate()); err != nil || !fi.Mode().IsRegular() {
			return errors.New("Error: plan template not found at templates/plan.md")
		}
		return nil
	})
	PlanPickupHealth  = readOnly(func(c *Context) error { return requireDir(c, c.plansDir()) })
	PlanArchiveHealth = readOnly(func(c *Context) error { return requireDir(c, filepath.Join(c.plansDir(), "archive")) })
)
