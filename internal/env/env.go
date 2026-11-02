// Package env resolves where Daisy is running: the root, the per-machine
// directories, the registry of homes, and the workspace the working
// directory belongs to with the home, identity and projects it is bound to.
// Pure functions over Ops.FS, Getenv and Getwd; the messages are the ones
// the bash printed, since the acceptance suite asserts on them.
package env

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"daisy/internal/ops"
)

// ProjectsSource says where a workspace's projects came from.
type ProjectsSource string

const (
	NoProjects         ProjectsSource = "none"
	ExplicitProjects   ProjectsSource = "explicit"   // .daisy/project
	ConventionProjects ProjectsSource = "convention" // the directory name
)

// Env is a resolved workspace.
type Env struct {
	Root, Config, Local string
	Workspace           string // the directory holding .daisy/
	HomeName, Home      string // the registered name and its directory
	Identity            string // .daisy/identity, else HomeName
	IdentityFile        string // <home>/gitconfig or <home>/identities/<Identity>
	Projects            []string
	ProjectsSource      ProjectsSource
	Warnings            []string // for stderr; nothing fatal
}

// Link is one of the workspace binding's symlinks: .daisy/<Link> → <home>/<Target>.
type Link struct{ Link, Target string }

// Links is the one list of binding links init writes and status checks.
var Links = []Link{
	{"tasks", "tasks"},
	{"today.md", "journal/today.md"},
	{"journal.md", "journal/journal.md"},
	{"projects", "projects"},
	{"plans", "plans"},
	{"feedback.md", "feedback/feedback.md"},
}

// Root is $DAISY_ROOT, which must be set and exist.
func Root(o ops.Ops) (string, error) {
	root := o.Getenv("DAISY_ROOT")
	if root == "" {
		return "", errors.New("Error: DAISY_ROOT not set\n  Add to ~/.zshenv: export DAISY_ROOT=\"/path/to/daisy\"")
	}
	if fi, err := o.FS().Stat(root); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("Error: DAISY_ROOT directory does not exist: %s", root)
	}
	return root, nil
}

// Resolve finds the workspace the working directory is in and the home it
// is bound to. The error is the message for stderr, fix included.
func Resolve(o ops.Ops) (Env, error) {
	e, err := machine(o)
	if err != nil {
		return e, err
	}
	root, fs := e.Root, o.FS()

	cwd, err := o.Getwd()
	if err != nil {
		return e, fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	ws, err := findWorkspace(fs, root, cwd)
	if err != nil {
		return e, err
	}
	e.Workspace = ws

	daisyDir := filepath.Join(ws, ".daisy")
	if fi, err := fs.Stat(filepath.Join(daisyDir, "home")); err != nil || !fi.Mode().IsRegular() {
		return e, fmt.Errorf("Error: %s has no home file; the binding is incomplete\n  Run 'daisy init <identity>' there to rebind it, or 'daisy clean' to remove it", daisyDir)
	}
	b, err := fs.ReadFile(filepath.Join(daisyDir, "home"))
	if err != nil {
		return e, fmt.Errorf("Error: cannot read %s: %v", filepath.Join(daisyDir, "home"), err)
	}
	e.HomeName = strings.TrimRight(string(b), "\n")
	if e.Home, err = homeDir(o, e); err != nil {
		return e, err
	}

	e.Identity = e.HomeName
	if b, err := fs.ReadFile(filepath.Join(daisyDir, "identity")); err == nil {
		first, _, _ := strings.Cut(strings.ReplaceAll(string(b), "\r", ""), "\n")
		if id := strings.TrimSpace(first); id != "" {
			e.Identity = id
		}
	}
	e.IdentityFile, err = IdentityFile(e.Home, e.HomeName, e.Identity)
	if err != nil {
		return e, fmt.Errorf("Error: %s holds '%s', which is not an identity name\n  Re-run 'daisy init <identity>' in the workspace to rebind it", filepath.Join(daisyDir, "identity"), e.Identity)
	}

	resolveProjects(fs, &e)
	return e, nil
}

// ForHome resolves a home by its registered name, with no workspace: the
// root, the per-machine directories and the home. Identity and projects
// are a workspace's and stay unset.
func ForHome(o ops.Ops, name string) (Env, error) {
	e, err := machine(o)
	if err != nil {
		return e, err
	}
	e.HomeName = name
	e.Home, err = homeDir(o, e)
	return e, err
}

// machine is the root and the per-machine directories.
func machine(o ops.Ops) (Env, error) {
	root, err := Root(o)
	if err != nil {
		return Env{}, err
	}
	return Env{
		Root:   root,
		Config: orDefault(o, "DAISY_CONFIG", ".config/daisy"),
		Local:  orDefault(o, "DAISY_LOCAL", ".local/daisy"),
	}, nil
}

// homeDir is HomeDir cleaned: the registry line as written, less `/./`,
// `//` and `..`.
func homeDir(o ops.Ops, e Env) (string, error) {
	home, err := HomeDir(o, e)
	if err != nil {
		return "", err
	}
	return filepath.Clean(home), nil
}

// orDefault is `${KEY:-$HOME/<underHome>}`: with HOME unset the default is
// under `/`, as the shell expanded it, never relative to the cwd.
func orDefault(o ops.Ops, key, underHome string) string {
	if v := o.Getenv(key); v != "" {
		return v
	}
	return filepath.Join("/"+o.Getenv("HOME"), underHome)
}

// findWorkspace walks up from cwd to the nearest directory that carries a
// binding, refusing one that is broken rather than walking past it.
func findWorkspace(fs ops.FS, root, cwd string) (string, error) {
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if err := workspaceProblem(fs, root, dir); err != nil {
			return "", err
		}
		if isWorkspaceDir(fs, root, dir) {
			return dir, nil
		}
	}
	return "", errors.New("Error: Not in a Daisy workspace (no .daisy/ directory found)\n  Run 'daisy init <identity>' to initialize this workspace.")
}

// isWorkspaceDir: a .daisy/ directory that is not the Daisy root itself. The
// root is a directory named .daisy under the user's home, so a walk up from
// beside it steps past it and never reads it as a binding.
func isWorkspaceDir(fs ops.FS, root, dir string) bool {
	d := filepath.Join(dir, ".daisy")
	fi, err := fs.Stat(d)
	if err != nil || !fi.IsDir() {
		return false
	}
	return RealDir(fs, d) != RealDir(fs, root)
}

var leftoverShape = regexp.MustCompile(`^\.daisy\.(previous\.[0-9]+|stage\.[A-Za-z0-9]{6})$`)

// workspaceProblem is the error for a directory that is a workspace that
// cannot be read as one: its .daisy is a symlink or not a directory, or it
// has no .daisy but holds what an interrupted init swap leaves.
func workspaceProblem(fs ops.FS, root, dir string) error {
	d := filepath.Join(dir, ".daisy")
	if fi, err := fs.Stat(d); err == nil && fi.IsDir() {
		if isWorkspaceDir(fs, root, dir) && isSymlink(fs, d) {
			return fmt.Errorf("Error: %s is a symlink; a binding is a directory of its own — move it aside, then run 'daisy init <identity>' there", d)
		}
		return nil
	}
	if isSymlink(fs, d) {
		return fmt.Errorf("Error: %s is a symlink; a binding is a directory of its own — move it aside, then run 'daisy init <identity>' there", d)
	}
	if _, err := fs.Lstat(d); err == nil {
		return fmt.Errorf("Error: %s exists and is not a directory; move it aside, then run 'daisy init <identity>' there", d)
	}
	leftovers := initLeftovers(fs, dir)
	if len(leftovers) == 0 {
		return nil
	}
	return fmt.Errorf("Error: an earlier 'daisy init' in %s was interrupted mid-swap; what it left is %s\n  Run 'daisy init <identity>' there to reclaim the binding", dir, strings.Join(leftovers, " "))
}

func isSymlink(fs ops.FS, p string) bool {
	fi, err := fs.Lstat(p)
	return err == nil && fi.Mode()&fsModeSymlink != 0
}

// initLeftovers: .daisy.previous.<pid> then .daisy.stage.<6 chars>,
// directories only; a user's look-alike is never listed.
func initLeftovers(fs ops.FS, dir string) []string {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil
	}
	var previous, stage []string
	for _, en := range entries {
		if !en.IsDir() || !leftoverShape.MatchString(en.Name()) {
			continue
		}
		p := filepath.Join(dir, en.Name())
		if strings.HasPrefix(en.Name(), ".daisy.previous.") {
			previous = append(previous, p)
		} else {
			stage = append(stage, p)
		}
	}
	return append(previous, stage...)
}

var nameShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// NameOK: a home or identity name is a registry key and a file name, never a path.
func NameOK(s string) bool { return nameShape.MatchString(s) }

// IdentityFile is the file holding an identity of a home: gitconfig for the
// home's own name, identities/<identity> otherwise. Errors for a name that
// is not a name.
func IdentityFile(homeDir, homeName, identity string) (string, error) {
	if identity == "" || identity == homeName {
		return filepath.Join(homeDir, "gitconfig"), nil
	}
	if !NameOK(identity) {
		return "", fmt.Errorf("'%s' is not an identity name", identity)
	}
	return filepath.Join(homeDir, "identities", identity), nil
}

// resolveProjects: .daisy/project, when present, is the whole answer; else
// the directory name before the first dot, when a project file of that
// name exists. A declared project with no file is a warning, never fatal.
func resolveProjects(fs ops.FS, e *Env) {
	e.ProjectsSource = NoProjects
	binding := filepath.Join(e.Workspace, ".daisy", "project")
	if b, err := fs.ReadFile(binding); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			name, _, _ := strings.Cut(line, "#")
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !isFile(fs, filepath.Join(e.Home, "projects", name+".md")) {
				e.Warnings = append(e.Warnings, fmt.Sprintf("⚠️  .daisy/project names '%s', but %s does not exist", name, filepath.Join(e.Home, "projects", name+".md")))
			}
			e.Projects = append(e.Projects, name)
		}
		if len(e.Projects) > 0 {
			e.ProjectsSource = ExplicitProjects
		}
		return
	}
	candidate, _, _ := strings.Cut(filepath.Base(e.Workspace), ".")
	if candidate != "" && isFile(fs, filepath.Join(e.Home, "projects", candidate+".md")) {
		e.Projects = []string{candidate}
		e.ProjectsSource = ConventionProjects
	}
}

func isFile(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ProjectsSummary is the one-line rendering for `files` and `status`.
func (e Env) ProjectsSummary() string {
	if len(e.Projects) == 0 {
		return "(none)"
	}
	out := "+" + strings.Join(e.Projects, " +")
	switch e.ProjectsSource {
	case ExplicitProjects:
		return out + " (from .daisy/project)"
	case ConventionProjects:
		return out + " (from directory name)"
	}
	return out
}

// LinkDrift says, one line each, which binding links are missing or point
// elsewhere than into the home.
func (e Env) LinkDrift(fs ops.FS) []string {
	var drift []string
	for _, l := range Links {
		if RealPath(fs, filepath.Join(e.Workspace, ".daisy", l.Link)) != RealPath(fs, filepath.Join(e.Home, l.Target)) {
			drift = append(drift, fmt.Sprintf("the binding link .daisy/%s is missing or does not point into the home", l.Link))
		}
	}
	return drift
}
