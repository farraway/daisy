package identity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/identity"
	"daisy/internal/ops"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, content string
		want          identity.Identity
		wantSSH       string
		wantErr       string
	}{
		{"name and email", "name=Pelican Moorhen\nemail=pelican@example.invalid\n", identity.Identity{Name: "Pelican Moorhen", Email: "pelican@example.invalid"}, "", ""},
		{"sshkey gives the ssh command", "name=A\nemail=a@b\nsshkey=~/.ssh/id_x\n", identity.Identity{Name: "A", Email: "a@b", SSHKey: "~/.ssh/id_x"}, "ssh -i ~/.ssh/id_x -o IdentitiesOnly=yes", ""},
		{"CR stripped, first line wins", "name=A\r\nname=B\nemail=a@b\n", identity.Identity{Name: "A", Email: "a@b"}, "", ""},
		{"template with comments only", "# name=Your Name\n# email=you@example.com\n", identity.Identity{}, "", "is incomplete"},
		{"missing email", "name=A\n", identity.Identity{}, "", "needs both 'name=' and 'email=' lines"},
		{"sshkey with a space", "name=A\nemail=a@b\nsshkey=/k/a b\n", identity.Identity{}, "", "must be a plain path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "_"))
			write(t, f, tc.content)
			got, err := identity.Read(ops.Real().FS(), f)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), f) {
					t.Fatalf("err %v, want containing %q and the file", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want || got.SSHCommand() != tc.wantSSH {
				t.Errorf("got %+v ssh %q", got, got.SSHCommand())
			}
		})
	}
	if _, err := identity.Read(ops.Real().FS(), filepath.Join(dir, "absent")); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("absent file: %v", err)
	}
}

// Drift against a real repository: the record claims one identity, the
// repository holds another.
func TestDrift(t *testing.T) {
	o := ops.Real()
	repo := t.TempDir()
	if _, _, err := o.Exec(repo, "git", "init", "-q"); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	bound := filepath.Join(home, "gitconfig")
	write(t, bound, "name=Zebra Quillfeather\nemail=fixture@example.invalid\n")
	daisyDir := filepath.Join(repo, ".daisy")
	record := filepath.Join(daisyDir, "gitconfig")
	write(t, record, "name=Zebra Quillfeather\nemail=fixture@example.invalid\n")
	set := func(k, v string) {
		if _, _, err := o.Exec(repo, "git", "config", "--local", k, v); err != nil {
			t.Fatal(err)
		}
	}
	set("user.name", "Zebra Quillfeather")
	set("user.email", "fixture@example.invalid")

	if d := identity.Drift(o, repo, daisyDir, bound); len(d) != 0 {
		t.Errorf("clean: %v", d)
	}

	set("user.email", "other@example.invalid")
	d := identity.Drift(o, repo, daisyDir, bound)
	if len(d) != 1 || !strings.Contains(d[0], "git user.email is 'other@example.invalid' but the record says 'fixture@example.invalid'; the applied identity differs") {
		t.Errorf("email changed: %v", d)
	}
	set("user.email", "fixture@example.invalid")

	write(t, bound, "name=Zebra Quillfeather\nemail=changed@example.invalid\n")
	d = identity.Drift(o, repo, daisyDir, bound)
	if len(d) != 1 || !strings.Contains(d[0], "the bound identity file "+bound+" does not match what init recorded") {
		t.Errorf("file edited: %v", d)
	}
	write(t, bound, "name=Zebra Quillfeather\nemail=fixture@example.invalid\n")

	// A core.sshCommand nobody claims is the user's unless it is in Daisy's form.
	set("core.sshCommand", "ssh -v")
	if d := identity.Drift(o, repo, daisyDir, bound); len(d) != 0 {
		t.Errorf("user's ssh command: %v", d)
	}
	set("core.sshCommand", "ssh -i /k/a -o IdentitiesOnly=yes")
	d = identity.Drift(o, repo, daisyDir, bound)
	if len(d) != 1 || !strings.Contains(d[0], "in Daisy's form but not claimed by the record") {
		t.Errorf("Daisy's unclaimed key: %v", d)
	}
	if _, _, err := o.Exec(repo, "git", "config", "--local", "--unset-all", "core.sshCommand"); err != nil {
		t.Fatal(err)
	}

	// No record and no readable bound file: report what the repository holds.
	os.Remove(record)
	os.Remove(bound)
	d = identity.Drift(o, repo, daisyDir, bound)
	if len(d) != 3 || !strings.Contains(d[0], "nothing claims the repository's git identity") ||
		!strings.Contains(d[1], "git user.name is 'Zebra Quillfeather', and nothing on record claims it") {
		t.Errorf("nothing on record: %v", d)
	}

	// A record that is a dangling link is no record, as `[ -e ]` saw it.
	os.Symlink(filepath.Join(daisyDir, "nowhere"), record)
	write(t, bound, "name=Zebra Quillfeather\nemail=fixture@example.invalid\n")
	if d := identity.Drift(o, repo, daisyDir, bound); len(d) != 0 {
		t.Errorf("dangling record: %v", d)
	}
	os.Remove(record)
	os.Remove(bound)

	// Drift runs under ReadOnly: every git call it makes is a read.
	write(t, record, "name=Zebra Quillfeather\nemail=fixture@example.invalid\n")
	if d := identity.Drift(ops.ReadOnly(o), repo, daisyDir, bound); len(d) != 1 || !strings.Contains(d[0], "is missing or not a complete identity") {
		t.Errorf("under ReadOnly: %v", d)
	}
}
