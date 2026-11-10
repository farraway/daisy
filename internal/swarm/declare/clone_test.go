package declare

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// cloneFolder is a folder holding a swarm repository's clone, swarm-app,
// whose origin is acme/swarm-app on forge.example.net, with docs/ and the
// five prompts, beside the clones of the product app (its
// core.sshCommand set) and its library lib: the folder init runs in.
func cloneFolder(t *testing.T) (dir, clone string) {
	t.Helper()
	hermetic(t)
	dir = t.TempDir()
	clone = filepath.Join(dir, "swarm-app")
	gitRepo(t, clone, "git@forge.example.net:acme/swarm-app.git")
	gitRepo(t, filepath.Join(dir, "app"), "git@forge.example.net:acme/app.git")
	gitRepo(t, filepath.Join(dir, "lib"), "https://forge.example.net/acme/lib.git")
	if _, errb, err := ops.Real().Exec(filepath.Join(dir, "app"), "git", "config", "core.sshCommand", "ssh -i /keys/app"); err != nil {
		t.Fatalf("%v %s", err, errb)
	}
	write(t, filepath.Join(clone, "docs", "roadmap.md"), "# Roadmap\n")
	for _, r := range Roles {
		write(t, PromptFile(clone, r), DefaultPrompts[r])
	}
	return dir, clone
}

// validClone is valid() as a swarm repository's clone declares it.
func validClone() Config {
	c := valid()
	c.Forge.Repo = "acme/swarm-app"
	ssh := "ssh -i /keys/app"
	c.Repos[0].Path, c.Repos[0].Repo, c.Repos[0].SSHCommand = "", "acme/app", &ssh
	c.Repos[1].Path, c.Repos[1].Repo = "", "acme/lib"
	return c
}

// The rows section 3's error table adds for revision 6 (section 13,
// revision 6's Step 3, the second row): repos[].repo, ssh_command and the
// clone's origin, byte for byte; each layout refusing the other's fields.
func TestValidateAClone(t *testing.T) {
	_, clone := cloneFolder(t)
	p := Clone(clone)
	if err := Validate(ops.Real(), p, validClone(), 0); err != nil {
		t.Fatalf("the valid declaration: %v", err)
	}
	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: repos[1].name: "swarm-app" is the swarm repository`, func(c *Config) { c.Repos[1].Name, c.Repos[1].Repo = "swarm-app", "acme/swarm-app" }},
		{`swarm.json: repos[0].path: unknown field`, func(c *Config) { c.Repos[0].Path = "app" }},
		{`swarm.json: repos[0].repo: missing`, func(c *Config) { c.Repos[0].Repo = "" }},
		{`swarm.json: repos[1].repo: "acme/library" is not acme/lib`, func(c *Config) { c.Repos[1].Repo = "acme/library" }},
		{`swarm.json: repos[1].repo: "someone/lib" is not acme/lib`, func(c *Config) { c.Repos[1].Repo = "someone/lib" }},
		{`swarm.json: repos[1].repo: "lib" is not acme/lib`, func(c *Config) { c.Repos[1].Repo = "lib" }},
		{`swarm.json: repos[0].ssh_command: empty`, func(c *Config) { c.Repos[0].SSHCommand = ptr("") }},
		{`swarm.json: repos[0].ssh_command: contains a newline`, func(c *Config) { c.Repos[0].SSHCommand = ptr("ssh\nrm -rf /") }},
		{`swarm.json: forge.repo: the clone's origin is "git@forge.example.net:acme/swarm-app.git", not acme/swarm-x on forge.example.net`, func(c *Config) { c.Forge.Repo = "acme/swarm-x" }},
		{`swarm.json: forge.repo: the clone's origin is "git@forge.example.net:acme/swarm-app.git", not acme/swarm-app on other.example.net`, func(c *Config) { c.Forge.API = "https://other.example.net/api/v1" }},
	} {
		c := validClone()
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}
	for url, want := range map[string]string{
		"": `swarm.json: forge.repo: the clone ` + clone + ` has no origin remote`,
		"https://jane:tok@forge.example.net/acme/swarm-app.git": `swarm.json: forge.repo: the clone's origin URL carries user information; use SSH or a credential helper`,
		"/srv/git/swarm-app.git":                                `swarm.json: forge.repo: the clone's origin is "/srv/git/swarm-app.git", not <owner>/<repo> on a forge`,
	} {
		ops.Real().Exec(clone, "git", "remote", "remove", "origin")
		if url != "" {
			ops.Real().Exec(clone, "git", "remote", "add", "origin", url)
		}
		if got := errText(Validate(ops.Real(), p, validClone(), 0)); got != want {
			t.Errorf("origin %q:\ngot  %s\nwant %s", url, got, want)
		}
	}

	// The old layout refuses the clone's fields, and needs path.
	ws := workspace(t)
	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: repos[0].repo: unknown field`, func(c *Config) { c.Repos[0].Repo = "acme/app" }},
		{`swarm.json: repos[0].ssh_command: unknown field`, func(c *Config) { c.Repos[0].SSHCommand = ptr("ssh") }},
		{`swarm.json: repos[1].path: missing`, func(c *Config) { c.Repos[1].Path = "" }},
	} {
		c := valid()
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), Workspace(ws), c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}

	// Parsed and written: ssh_command after repo, absent when unset.
	got := string(Format(validClone()))
	if !strings.Contains(got, `{"name": "app", "repo": "acme/app", "ssh_command": "ssh -i /keys/app", "check": ["make", "check"], `) ||
		!strings.Contains(got, `{"name": "lib", "repo": "acme/lib", "check": [], `) {
		t.Errorf("Format:\n%s", got)
	}
	if c, err := Parse([]byte(got)); err != nil || !reflect.DeepEqual(c, validClone()) {
		t.Errorf("round trip: %v %+v", err, c)
	}
}

// Question 1 as forge names (section 13, revision 6's Step 3, the third
// row): its default and each ssh_command from the clones beside the swarm
// repository's, the swarm repository's own left out; question 4 the
// clone's origin, refusing a declared repository and asking again; the
// file's repositories the default once there is one.
func TestWizardInAClone(t *testing.T) {
	dir, clone := cloneFolder(t)
	gitRepo(t, filepath.Join(dir, "swarm-other"), "git@forge.example.net:acme/swarm-other.git")
	o, asked := scripted(append([]string{"", "", "", "acme/app", ""}, make([]string, 40)...)...)
	var errb bytes.Buffer
	w := &Wizard{Ops: o, Err: &errb, Place: Clone(clone)}
	c, err := w.Ask()
	if err == nil {
		err = w.AskAccount(&c, "swarm-bot", "swarm@example.net")
	}
	if err == nil {
		err = w.AskNotes(&c)
	}
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{
		"Product repositories (comma-separated <owner>/<name>, the product first) [acme/app, acme/lib]: ",
		"Forge kind (gitea, github) [gitea]: ",
		"Forge API [https://forge.example.net/api/v1]: ",
		"Swarm repository [acme/swarm-app]: ",
		"Swarm repository [acme/swarm-app]: ",
		"Token variable [GITEA_TOKEN]: ",
	} {
		if (*asked)[i] != want {
			t.Errorf("question %d: %q, want %q", i+1, (*asked)[i], want)
		}
	}
	// Question 32: swarm-repository paths, its default the directories
	// init seeds, whether the clone holds them yet or not.
	if !slices.Contains(*asked, "Resources (comma-separated: paths in the swarm repository, or <name>:<path> in a declared one; - for none) [briefs/, docs/]: ") || !reflect.DeepEqual(c.Resources, []string{"briefs/", "docs/"}) {
		t.Errorf("question 32: %q, resources %q", *asked, c.Resources)
	}
	if last := (*asked)[len(*asked)-1]; last != "Research notes, in acme/swarm-app's notes/ (yes, no) [no]: " {
		t.Errorf("the notes' question: %q", last)
	}
	if !strings.Contains(errb.String(), "swarm.json: forge.repo: acme/app is a declared repository, which holds product code; the swarm repository holds none\nSwarm repository") {
		t.Errorf("stderr:\n%s", errb.String())
	}
	ssh := "ssh -i /keys/app"
	if c.Forge.Repo != "acme/swarm-app" || !reflect.DeepEqual(c.Repos, []Repo{
		{Name: "app", Repo: "acme/app", SSHCommand: &ssh, Check: []string{}, Image: defaultImage, Protected: defaultProtected},
		{Name: "lib", Repo: "acme/lib", Check: []string{}, Image: defaultImage, Protected: defaultProtected},
	}) {
		t.Errorf("answers: %s %+v", c.Forge.Repo, c.Repos)
	}
	c.Trusted, c.Forge.CommitEmail = []string{"swarm-bot"}, "swarm@example.net"
	if err := Validate(ops.Real(), Clone(clone), c, 0); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}

	// A refused name, asked again; with a file, its repositories and
	// their ssh_command the defaults, an entry by path named after
	// forge.repo's owner.
	o, asked = scripted("app", "acme/lib", "")
	errb.Reset()
	file := validClone()
	file.Repos[1].Path, file.Repos[1].Repo = "lib", ""
	w = &Wizard{Ops: o, Err: &errb, Place: Clone(clone), File: &file}
	if _, err := w.Ask(); err == nil {
		t.Fatal("no EOF")
	}
	if (*asked)[0] != "Product repositories (comma-separated <owner>/<name>, the product first) [acme/app, acme/lib]: " ||
		!strings.HasPrefix(errb.String(), (*asked)[0]+`swarm.json: repos[0].repo: "app" is not <owner>/<name>`+"\n") {
		t.Errorf("asked %q\nstderr %s", *asked, errb.String())
	}
	o, _ = scripted()
	w = &Wizard{Ops: o, Err: &errb, Place: Clone(clone), File: &file, Defaults: true}
	c, err = w.Ask()
	if err != nil || c.Repos[0].SSHCommand == nil || *c.Repos[0].SSHCommand != ssh || c.Repos[1].Repo != "acme/lib" || c.Repos[1].Path != "" {
		t.Errorf("from the file: %v %+v", err, c.Repos)
	}

	// No origin on the clone: question 4 after the first product.
	os.RemoveAll(filepath.Join(dir, "swarm-other"))
	ops.Real().Exec(clone, "git", "remote", "remove", "origin")
	o, _ = scripted()
	w = &Wizard{Ops: o, Err: &errb, Place: Clone(clone), Defaults: true}
	if c, err = w.Ask(); err != nil || c.Forge.Repo != "acme/swarm-app" || c.Forge.API != "https://forge.example.net/api/v1" {
		t.Errorf("no origin: %v %+v", err, c.Forge)
	}
}

// init in a folder holding no swarm repository's clone (section 11, the
// wizard; section 13, revision 6's Step 6): detection reads the clones
// directly under the folder itself, never the folder's own origin;
// question 4's default the swarm- clone there, else <owner>/swarm-<name>
// after the first product; validation leaves out the clone's origin until
// the clone is made.
func TestWizardInAFolder(t *testing.T) {
	dir, clone := cloneFolder(t)
	gitRepo(t, dir, "git@forge.example.net:acme/not-this.git") // the folder a repository: never read
	o, asked := scripted()
	w := &Wizard{Ops: o, Err: &bytes.Buffer{}, Place: Folder(dir), Defaults: true}
	c, err := w.Ask()
	if err != nil || len(*asked) != 0 {
		t.Fatalf("%v %q", err, *asked)
	}
	ssh := "ssh -i /keys/app"
	if c.Forge.Repo != "acme/swarm-app" || c.Forge.API != "https://forge.example.net/api/v1" || !reflect.DeepEqual(c.Resources, []string{"briefs/", "docs/"}) || !reflect.DeepEqual(c.Repos, []Repo{
		{Name: "app", Repo: "acme/app", SSHCommand: &ssh, Check: []string{}, Image: defaultImage, Protected: defaultProtected},
		{Name: "lib", Repo: "acme/lib", Check: []string{}, Image: defaultImage, Protected: defaultProtected},
	}) {
		t.Errorf("defaults: %+v", c)
	}
	c.Trusted, c.Forge.CommitEmail = []string{"swarm-bot"}, "swarm@example.net"
	c.Forge.Repo = "acme/swarm-elsewhere"
	if err := Validate(ops.Real(), Folder(dir), c, SkipPrompts); err != nil {
		t.Errorf("the folder's answers: %v", err)
	}
	if got := errText(Validate(ops.Real(), Clone(clone), c, SkipPrompts)); got != `swarm.json: forge.repo: the clone's origin is "git@forge.example.net:acme/swarm-app.git", not acme/swarm-elsewhere on forge.example.net` {
		t.Errorf("the clone's origin once made: %s", got)
	}
	if !IsCloneOf(ops.Real(), clone, validClone().Forge) || IsCloneOf(ops.Real(), clone, c.Forge) || IsCloneOf(ops.Real(), filepath.Join(clone, "docs"), validClone().Forge) {
		t.Error("IsCloneOf")
	}

	// No swarm- clone in the folder: <owner>/swarm-<name>.
	os.RemoveAll(clone)
	o, _ = scripted()
	w = &Wizard{Ops: o, Err: &bytes.Buffer{}, Place: Folder(dir), Defaults: true}
	if c, err = w.Ask(); err != nil || c.Forge.Repo != "acme/swarm-app" {
		t.Errorf("no swarm clone: %v %s", err, c.Forge.Repo)
	}
}

// resources and references in a swarm repository's clone (section 3's
// rows; section 13, revision 6's Step 4): names, checked here, their
// content at the launch, so a path the clone does not hold is valid; the
// old layout has no references; references written one line each after
// resources, and read back.
func TestValidateResourcesAndReferences(t *testing.T) {
	_, clone := cloneFolder(t)
	p := Clone(clone)
	head := strings.Repeat("ab", 20)
	ok := func(c *Config) {
		c.Resources = []string{"briefs/", "docs/", "lib:guide/", "app:docs/x.md", "not/a:name"}
		c.References = []Reference{{Name: "spec", URL: "https://forge.example.org/std/spec.git", Pin: "v1.2", Paths: []string{"SPEC.md"}},
			{Name: "std", URL: "git@forge.example.org:std.git", Pin: head, Paths: []string{"lib/", "doc/a.md"}},
			{Name: "ssh", URL: "ssh://git@forge.example.org:2222/std/ssh", Pin: "main", Paths: []string{"x"}}}
	}
	c := validClone()
	ok(&c)
	if err := Validate(ops.Real(), p, c, 0); err != nil {
		t.Fatalf("the valid declaration: %v", err)
	}
	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: resources[1]: "." is the whole repository`, func(c *Config) { c.Resources = []string{"docs/", "."} }},
		{`swarm.json: resources[0]: "lib:./" is the whole repository`, func(c *Config) { c.Resources = []string{"lib:./"} }},
		{`swarm.json: resources[0]: "/etc" is not a relative path`, func(c *Config) { c.Resources = []string{"/etc"} }},
		{`swarm.json: resources[0]: "docs/../x" is not a relative path`, func(c *Config) { c.Resources = []string{"docs/../x"} }},
		{`swarm.json: resources[0]: "lib:" is not a relative path`, func(c *Config) { c.Resources = []string{"lib:"} }},
		{`swarm.json: resources[0]: "gjot:docs/" names no declared repository`, func(c *Config) { c.Resources = []string{"gjot:docs/"} }},
		{`swarm.json: resources[1]: "./docs" is listed twice`, func(c *Config) { c.Resources = []string{"docs/", "./docs"} }},
		{`swarm.json: resources[1]: "lib:guide" is listed twice`, func(c *Config) { c.Resources = []string{"lib:guide/", "lib:guide"} }},
		{`swarm.json: references[0].name: "a b" is not a repository name`, func(c *Config) { c.References[0].Name = "a b" }},
		{`swarm.json: references[1].name: "spec" is listed twice`, func(c *Config) { c.References[1].Name = "spec" }},
		{`swarm.json: references[0].name: "lib" is a declared repository`, func(c *Config) { c.References[0].Name = "lib" }},
		{`swarm.json: references[0].name: "swarm-app" is the swarm repository`, func(c *Config) { c.References[0].Name = "swarm-app" }},
		{`swarm.json: references[0].url: "/srv/spec" is not a git URL`, func(c *Config) { c.References[0].URL = "/srv/spec" }},
		{`swarm.json: references[0].url: "ftp://forge.example.org/spec" is not a git URL`, func(c *Config) { c.References[0].URL = "ftp://forge.example.org/spec" }},
		{`swarm.json: references[0].url: "https://jane:tok@forge.example.org/spec.git" carries user information`, func(c *Config) {
			c.References[0].URL = "https://jane:tok@forge.example.org/spec.git"
		}},
		{`swarm.json: references[2].url: "ssh://git:pw@forge.example.org/std/ssh" carries user information`, func(c *Config) { c.References[2].URL = "ssh://git:pw@forge.example.org/std/ssh" }},
		{`swarm.json: references[0].pin: "v1..2" is not a commit or a tag`, func(c *Config) { c.References[0].Pin = "v1..2" }},
		{`swarm.json: references[0].pin: "" is not a commit or a tag`, func(c *Config) { c.References[0].Pin = "" }},
		{`swarm.json: references[0].paths: no path (at least one is required)`, func(c *Config) { c.References[0].Paths = nil }},
		{`swarm.json: references[1].paths[1]: "lib" is listed twice`, func(c *Config) { c.References[1].Paths = []string{"lib/", "lib"} }},
		{`swarm.json: references[1].paths[0]: "." is the whole repository`, func(c *Config) { c.References[1].Paths = []string{"."} }},
		{`swarm.json: references[1].paths[0]: "../x" is not a relative path`, func(c *Config) { c.References[1].Paths = []string{"../x"} }},
	} {
		c := validClone()
		ok(&c)
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}

	// The old layout: no references.
	old := valid()
	old.References = []Reference{{Name: "spec", URL: "https://forge.example.org/spec.git", Pin: "v1", Paths: []string{"SPEC.md"}}}
	if got := errText(Validate(ops.Real(), Workspace(workspace(t)), old, 0)); got != `swarm.json: references: unknown field` {
		t.Errorf("old layout: %s", got)
	}

	// Written after resources, one line per reference; read back.
	got := string(Format(c))
	want := `  "resources": ["briefs/", "docs/", "lib:guide/", "app:docs/x.md", "not/a:name"],` + "\n" + `  "references": [` + "\n" +
		`    {"name": "spec", "url": "https://forge.example.org/std/spec.git", "pin": "v1.2", "paths": ["SPEC.md"]},` + "\n" +
		`    {"name": "std", "url": "git@forge.example.org:std.git", "pin": "` + head + `", "paths": ["lib/", "doc/a.md"]},` + "\n" +
		`    {"name": "ssh", "url": "ssh://git@forge.example.org:2222/std/ssh", "pin": "main", "paths": ["x"]}` + "\n  ],\n" + `  "trusted": `
	if !strings.Contains(got, want) {
		t.Errorf("Format:\n%s\nwant within:\n%s", got, want)
	}
	if back, err := Parse([]byte(got)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v %+v", err, back)
	}
	if strings.Contains(string(Format(validClone())), "references") {
		t.Error("references written when none")
	}
}
