package declare

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
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
