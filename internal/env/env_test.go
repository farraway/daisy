package env_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// fx is one machine: a root, a registry, a home registered as testhome and a
// workspace bound to it, each in its own temp directory.
type fx struct {
	t                                 *testing.T
	root, config, local, home, ws, hd string
	vars                              map[string]string
}

func newFx(t *testing.T) *fx {
	t.Helper()
	f := &fx{t: t, vars: map[string]string{}}
	f.root = t.TempDir()
	f.config = t.TempDir()
	f.local = t.TempDir()
	f.hd = t.TempDir()
	f.home = filepath.Join(t.TempDir(), "testhome")
	f.ws = t.TempDir()
	for _, d := range []string{"tasks", "journal", "projects", "plans", "feedback", "identities"} {
		f.mkdir(filepath.Join(f.home, d))
	}
	f.write(filepath.Join(f.home, "gitconfig"), "name=Fixture Author\nemail=fixture@example.invalid\n")
	f.write(filepath.Join(f.config, "homes"), "testhome="+f.home+"\n")
	f.bind(f.ws, "testhome")
	f.vars["DAISY_ROOT"] = f.root
	f.vars["DAISY_CONFIG"] = f.config
	f.vars["DAISY_LOCAL"] = f.local
	f.vars["HOME"] = f.hd
	return f
}

func (f *fx) mkdir(p string) {
	if err := os.MkdirAll(p, 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) write(p, s string) {
	f.mkdir(filepath.Dir(p))
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) link(target, name string) {
	f.mkdir(filepath.Dir(name))
	if err := os.Symlink(target, name); err != nil {
		f.t.Fatal(err)
	}
}

// bind writes what `daisy init` leaves: .daisy/home and the six links.
func (f *fx) bind(ws, home string) {
	f.write(filepath.Join(ws, ".daisy", "home"), home+"\n")
	for _, l := range env.Links {
		f.link(filepath.Join(f.home, l.Target), filepath.Join(ws, ".daisy", l.Link))
	}
}

func (f *fx) ops(cwd string) ops.Ops {
	return &ops.Fake{
		GetenvFn: func(k string) string { return f.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(f *fx) (cwd string)
		want    func(t *testing.T, f *fx, e env.Env)
		wantErr func(f *fx) []string
	}{
		{
			name:  "bound workspace resolves home, identity and workspace",
			setup: func(f *fx) string { return f.ws },
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Root != f.root || e.Config != f.config || e.Local != f.local {
					t.Errorf("dirs: %+v", e)
				}
				if e.Workspace != f.ws || e.HomeName != "testhome" || e.Home != f.home {
					t.Errorf("binding: %+v", e)
				}
				if e.Identity != "testhome" || e.IdentityFile != filepath.Join(f.home, "gitconfig") {
					t.Errorf("identity: %q %q", e.Identity, e.IdentityFile)
				}
				if len(e.Projects) != 0 || e.ProjectsSource != env.NoProjects || len(e.Warnings) != 0 {
					t.Errorf("projects: %+v", e)
				}
			},
		},
		{
			name: "a subdirectory walks up to the workspace",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "a", "b")
				f.mkdir(d)
				return d
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Workspace != f.ws {
					t.Errorf("workspace %q", e.Workspace)
				}
			},
		},
		{
			name:    "no .daisy above the directory",
			setup:   func(f *fx) string { return f.t.TempDir() },
			wantErr: func(f *fx) []string { return []string{"Not in a Daisy workspace", "daisy init <identity>"} },
		},
		{
			name: "the Daisy root is not a binding",
			setup: func(f *fx) string {
				h := f.t.TempDir()
				f.vars["DAISY_ROOT"] = filepath.Join(h, ".daisy")
				f.mkdir(filepath.Join(h, ".daisy"))
				f.mkdir(filepath.Join(h, "bin"))
				return filepath.Join(h, "bin")
			},
			wantErr: func(f *fx) []string { return []string{"Not in a Daisy workspace"} },
		},
		{
			name: "a root reached through a symlink is still the root",
			setup: func(f *fx) string {
				h := f.t.TempDir()
				f.link(f.root, filepath.Join(h, ".daisy"))
				f.vars["DAISY_ROOT"] = filepath.Join(h, ".daisy")
				f.mkdir(filepath.Join(h, "bin"))
				return filepath.Join(h, "bin")
			},
			wantErr: func(f *fx) []string { return []string{"Not in a Daisy workspace"} },
		},
		{
			name: "a dangling .daisy symlink is refused, not walked past",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "child")
				f.link("/nonexistent", filepath.Join(d, ".daisy"))
				return d
			},
			wantErr: func(f *fx) []string { return []string{"is a symlink", "daisy init <identity>"} },
		},
		{
			name: "a .daisy that is a file is refused",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "child")
				f.write(filepath.Join(d, ".daisy"), "junk")
				return d
			},
			wantErr: func(f *fx) []string { return []string{"not a directory"} },
		},
		{
			name: "an interrupted init's leftovers are refused and named",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "child")
				f.mkdir(filepath.Join(d, ".daisy.previous.4242"))
				f.mkdir(filepath.Join(d, ".daisy.stage.abc123"))
				f.mkdir(filepath.Join(d, ".daisy.previous.bak")) // not init's shape
				return d
			},
			wantErr: func(f *fx) []string {
				return []string{".daisy.previous.4242 ", ".daisy.stage.abc123", "interrupted", "daisy init <identity>"}
			},
		},
		{
			name: "a look-alike leftover is not init's",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "child")
				f.mkdir(filepath.Join(d, ".daisy.previous.bak"))
				return d
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Workspace != f.ws {
					t.Errorf("workspace %q", e.Workspace)
				}
			},
		},
		{
			name: "a .daisy without a home file is an incomplete binding here",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "child")
				f.mkdir(filepath.Join(d, ".daisy"))
				return d
			},
			wantErr: func(f *fx) []string { return []string{"has no home file", "incomplete", "daisy init <identity>"} },
		},
		{
			name: "an unregistered home names the fix",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "home"), "second\n")
				return f.ws
			},
			wantErr: func(f *fx) []string {
				return []string{"Home 'second' is not registered in " + filepath.Join(f.config, "homes"), "daisy home add second"}
			},
		},
		{
			name: "a home registered at the root is refused",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), "testhome="+f.root+"\n")
				return f.ws
			},
			wantErr: func(f *fx) []string {
				return []string{"registered at " + f.root + ", which is the Daisy root; the root cannot be a home", "daisy home remove testhome"}
			},
		},
		{
			name: "a home registered at a relative path is refused",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), "testhome=relative/path\n")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"which is not an absolute path"} },
		},
		{
			name: "a home registered at a missing path is refused",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), "testhome="+filepath.Join(f.local, "nowhere")+"\n")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"which does not exist"} },
		},
		{
			name: "a home holding the root is refused",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), "testhome="+filepath.Dir(f.root)+"\n")
				return f.ws
			},
			wantErr: func(f *fx) []string {
				return []string{"which contains the Daisy root; the root cannot be inside a home"}
			},
		},
		{
			name: "registry lines are trimmed, commented, first-match and name-exact",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), strings.Join([]string{
					"# a comment",
					"",
					"a testhome=/elsewhere",
					"  testhome =  " + f.home + "// ",
					"testhome=/second-entry-ignored",
				}, "\n")+"\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Home != f.home {
					t.Errorf("home %q", e.Home)
				}
			},
		},
		{
			name: "a named identity resolves to identities/<name>",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "identity"), "pelicanmoor\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Identity != "pelicanmoor" || e.IdentityFile != filepath.Join(f.home, "identities", "pelicanmoor") {
					t.Errorf("identity: %q %q", e.Identity, e.IdentityFile)
				}
			},
		},
		{
			name: "an empty or blank .daisy/identity is the default identity",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "identity"), "  \r\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Identity != "testhome" {
					t.Errorf("identity %q", e.Identity)
				}
			},
		},
		{
			name: "an identity value is trimmed of whitespace and CR",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "identity"), "  pelicanmoor \r\nsecond line\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Identity != "pelicanmoor" {
					t.Errorf("identity %q", e.Identity)
				}
			},
		},
		{
			name: "an identity that is not a name is refused",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "identity"), "../escape\n")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"holds '../escape', which is not an identity name"} },
		},
		{
			name: "an explicit .daisy/project replaces the convention",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "chessgrid.260807")
				f.bind(d, "testhome")
				f.write(filepath.Join(f.home, "projects", "chessgrid.md"), "# chessgrid\n")
				f.write(filepath.Join(f.home, "projects", "jobsearch.md"), "# jobsearch\n")
				f.write(filepath.Join(f.home, "projects", "other.md"), "# other\n")
				f.write(filepath.Join(d, ".daisy", "project"), "jobsearch\n# a comment\n\n  other  # trailing\n")
				return d
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if strings.Join(e.Projects, ",") != "jobsearch,other" || e.ProjectsSource != env.ExplicitProjects {
					t.Errorf("projects %v %v", e.Projects, e.ProjectsSource)
				}
				if len(e.Warnings) != 0 {
					t.Errorf("warnings %v", e.Warnings)
				}
			},
		},
		{
			name: "a declared project with no file is kept and warned about",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.ws, ".daisy", "project"), "typoed-name\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if strings.Join(e.Projects, ",") != "typoed-name" || e.ProjectsSource != env.ExplicitProjects {
					t.Errorf("projects %v %v", e.Projects, e.ProjectsSource)
				}
				if len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "'typoed-name'") ||
					!strings.Contains(e.Warnings[0], filepath.Join(f.home, "projects", "typoed-name.md")+" does not exist") {
					t.Errorf("warnings %v", e.Warnings)
				}
			},
		},
		{
			name: "the directory name before the first dot is the project by convention",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "chessgrid.260807")
				f.bind(d, "testhome")
				f.write(filepath.Join(f.home, "projects", "chessgrid.md"), "# chessgrid\n")
				return d
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if strings.Join(e.Projects, ",") != "chessgrid" || e.ProjectsSource != env.ConventionProjects {
					t.Errorf("projects %v %v", e.Projects, e.ProjectsSource)
				}
			},
		},
		{
			name: "a convention miss is silent",
			setup: func(f *fx) string {
				d := filepath.Join(f.ws, "nosuchthing.260101")
				f.bind(d, "testhome")
				return d
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if len(e.Projects) != 0 || e.ProjectsSource != env.NoProjects || len(e.Warnings) != 0 {
					t.Errorf("projects %+v", e)
				}
			},
		},
		{
			name: "DAISY_CONFIG and DAISY_LOCAL default under HOME",
			setup: func(f *fx) string {
				delete(f.vars, "DAISY_CONFIG")
				delete(f.vars, "DAISY_LOCAL")
				f.write(filepath.Join(f.hd, ".config", "daisy", "homes"), "testhome="+f.home+"\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Config != filepath.Join(f.hd, ".config", "daisy") || e.Local != filepath.Join(f.hd, ".local", "daisy") {
					t.Errorf("dirs %q %q", e.Config, e.Local)
				}
			},
		},
		{
			name: "HOME unset: the defaults are under /, not the cwd",
			setup: func(f *fx) string {
				delete(f.vars, "DAISY_CONFIG")
				delete(f.vars, "HOME")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"not registered in /.config/daisy/homes"} },
		},
		{
			name: "a symlink before .. cannot smuggle the root in as the home",
			setup: func(f *fx) string {
				s := f.t.TempDir()
				f.mkdir(filepath.Join(s, "y", "z"))
				f.mkdir(filepath.Join(s, "y", "root"))
				f.link(filepath.Join(s, "y", "z"), filepath.Join(s, "x", "link"))
				f.link(f.root, filepath.Join(s, "x", "root"))
				f.write(filepath.Join(f.config, "homes"), "testhome="+filepath.Join(s, "x", "link")+"/../root\n")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"/x/link/../root, which is the Daisy root"} },
		},
		{
			name: "a registered path is cleaned",
			setup: func(f *fx) string {
				f.write(filepath.Join(f.config, "homes"), "testhome="+f.home+"/./\n")
				return f.ws
			},
			want: func(t *testing.T, f *fx, e env.Env) {
				if e.Home != f.home || e.IdentityFile != filepath.Join(f.home, "gitconfig") {
					t.Errorf("home %q identity file %q", e.Home, e.IdentityFile)
				}
			},
		},
		{
			name: "DAISY_ROOT unset",
			setup: func(f *fx) string {
				delete(f.vars, "DAISY_ROOT")
				return f.ws
			},
			wantErr: func(f *fx) []string { return []string{"DAISY_ROOT not set"} },
		},
		{
			name: "DAISY_ROOT missing on disk",
			setup: func(f *fx) string {
				f.vars["DAISY_ROOT"] = filepath.Join(f.root, "gone")
				return f.ws
			},
			wantErr: func(f *fx) []string {
				return []string{"DAISY_ROOT directory does not exist: " + filepath.Join(f.root, "gone")}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFx(t)
			cwd := tc.setup(f)
			e, err := env.Resolve(f.ops(cwd))
			if tc.wantErr != nil {
				want := tc.wantErr(f)
				if err == nil {
					t.Fatalf("want error containing %q, got none: %+v", want, e)
				}
				for _, s := range want {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error %q\n  does not contain %q", err, s)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.want(t, f, e)
		})
	}
}

func TestLinkDrift(t *testing.T) {
	f := newFx(t)
	e, err := env.Resolve(f.ops(f.ws))
	if err != nil {
		t.Fatal(err)
	}
	if d := e.LinkDrift(ops.Real().FS()); len(d) != 0 {
		t.Errorf("clean binding reports drift: %v", d)
	}
	os.Remove(filepath.Join(f.ws, ".daisy", "today.md"))
	os.Remove(filepath.Join(f.ws, ".daisy", "tasks"))
	f.link(f.t.TempDir(), filepath.Join(f.ws, ".daisy", "tasks"))
	d := e.LinkDrift(ops.Real().FS())
	if len(d) != 2 || !strings.Contains(d[0], ".daisy/tasks") || !strings.Contains(d[1], ".daisy/today.md") {
		t.Errorf("drift %v", d)
	}
}

func TestRealPath(t *testing.T) {
	dir := t.TempDir()
	real, _ := filepath.EvalSymlinks(dir)
	os.Symlink(dir, filepath.Join(dir, "link"))
	fs := ops.Real().FS()
	if got := env.RealPath(fs, filepath.Join(dir, "link", "missing.md")); got != filepath.Join(real, "missing.md") {
		t.Errorf("missing last component: %q", got)
	}
	if got := env.RealPath(fs, filepath.Join(dir, "link")); got != real {
		t.Errorf("link to dir: %q", got)
	}
}
