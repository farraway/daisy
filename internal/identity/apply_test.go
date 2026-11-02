package identity_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/identity"
	"daisy/internal/ops"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func getAll(t *testing.T, dir, key string) []string {
	t.Helper()
	out, _ := exec.Command("git", "-C", dir, "config", "--local", "--get-all", "-z", key).Output()
	if len(out) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

var pelican = identity.Identity{Name: "Pelican Moorhen", Email: "pelican@example.invalid", SSHKey: "~/.ssh/id_p"}

// What clean undoes is what init did: the keys the identity still holds go,
// and what they held before init comes back; a key changed by hand since is
// the user's and is left whole.
func TestUnsetRestoresWhatWasThereBefore(t *testing.T) {
	o := ops.Real()
	repo := gitRepo(t)
	git(t, repo, "config", "--local", "user.name", "Hand Set")
	git(t, repo, "config", "--local", "--add", "user.name", "\tSecond\nline")
	before, err := identity.Snapshot(o, repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Apply(o, repo, pelican); err != nil {
		t.Fatal(err)
	}
	if got := getAll(t, repo, "user.name"); !reflect.DeepEqual(got, []string{"Pelican Moorhen"}) {
		t.Fatalf("apply replaced every value? %q", got)
	}
	cleared, err := identity.Unset(o, repo, pelican, before)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cleared, ", ") != "user.name, user.email, core.sshCommand" {
		t.Errorf("cleared %v", cleared)
	}
	if got := getAll(t, repo, "user.name"); !reflect.DeepEqual(got, []string{"Hand Set", "\tSecond\nline"}) {
		t.Errorf("user.name after unset %q", got)
	}
	for _, k := range []string{"user.email", "core.sshCommand"} {
		if got := getAll(t, repo, k); got != nil {
			t.Errorf("%s after unset %q", k, got)
		}
	}

	// A hand edit since apply: that key is left, and nothing is restored over it.
	identity.Apply(o, repo, pelican)
	git(t, repo, "config", "--local", "--add", "user.name", "Added")
	cleared, _ = identity.Unset(o, repo, pelican, before)
	if strings.Join(cleared, ", ") != "user.email, core.sshCommand" {
		t.Errorf("cleared %v", cleared)
	}
	if got := getAll(t, repo, "user.name"); !reflect.DeepEqual(got, []string{"Pelican Moorhen", "Added"}) {
		t.Errorf("hand-edited key %q", got)
	}
}

// A core.sshCommand in Daisy's exact form is Daisy's whatever the claim; a
// key holding any value of another form, the empty string included, is not.
func TestUnsetJudgesEveryValue(t *testing.T) {
	o := ops.Real()
	repo := gitRepo(t)
	git(t, repo, "config", "--local", "core.sshCommand", "ssh -i /k/orphan -o IdentitiesOnly=yes")
	git(t, repo, "config", "--local", "user.email", "pelican@example.invalid")
	git(t, repo, "config", "--local", "--add", "user.email", "")
	cleared, err := identity.Unset(o, repo, identity.Identity{Name: "x", Email: "pelican@example.invalid"}, nil)
	if err != nil || strings.Join(cleared, ",") != "core.sshCommand" {
		t.Errorf("cleared %v err %v", cleared, err)
	}
	if got := getAll(t, repo, "user.email"); len(got) != 2 {
		t.Errorf("mixed key touched: %q", got)
	}
}

// Restore puts the three keys back exactly, byte for byte, unset ones unset.
func TestSnapshotRestoreRoundTrip(t *testing.T) {
	o := ops.Real()
	repo := gitRepo(t)
	git(t, repo, "config", "--local", "user.email", "multi\nline@example.invalid")
	git(t, repo, "config", "--local", "--add", "user.name", "")
	git(t, repo, "config", "--local", "--add", "user.name", "\tTabbed")
	snap, _ := identity.Snapshot(o, repo)
	identity.Apply(o, repo, pelican)
	if err := identity.Restore(o, repo, snap); err != nil {
		t.Fatal(err)
	}
	if got := getAll(t, repo, "user.name"); !reflect.DeepEqual(got, []string{"", "\tTabbed"}) {
		t.Errorf("user.name %q", got)
	}
	if got := getAll(t, repo, "user.email"); !reflect.DeepEqual(got, []string{"multi\nline@example.invalid"}) {
		t.Errorf("user.email %q", got)
	}
	if got := getAll(t, repo, "core.sshCommand"); got != nil {
		t.Errorf("core.sshCommand %q", got)
	}
	// The record of them reads back as written.
	back, err := identity.ParseValues(snap.Format())
	if err != nil || !reflect.DeepEqual(back, snap) {
		t.Errorf("round trip %q -> %v %v", snap.Format(), back, err)
	}
}

// Resolution: a registered home's name is its default identity, whatever
// another home defines; another name is the one home defining it; two is an
// error naming both; a home that cannot be checked blocks a non-home name.
func TestHomeOf(t *testing.T) {
	root, config := t.TempDir(), t.TempDir()
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	write(t, filepath.Join(a, "identities", "shared"), "name=S\nemail=s@x\n")
	write(t, filepath.Join(b, "identities", "shared"), "name=S\nemail=s@x\n")
	write(t, filepath.Join(b, "identities", "alpha"), "name=I\nemail=i@x\n")
	write(t, filepath.Join(b, "identities", "only"), "name=O\nemail=o@x\n")
	reg := filepath.Join(config, "homes")
	write(t, reg, "alpha="+a+"\nbeta="+b+"\n")
	o := &ops.Fake{GetenvFn: func(k string) string { return map[string]string{"DAISY_ROOT": root, "DAISY_CONFIG": config}[k] }}
	m, err := env.Machine(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, home, err string }{
		{"alpha", "alpha", ""},
		{"only", "beta", ""},
		{"shared", "", "Error: identity 'shared' is defined in more than one home: alpha, beta"},
		{"nobody", "", "'nobody' is neither a registered home nor an identity of one"},
		{"../x", "", "'../x' is not a valid identity name"},
	} {
		home, err := identity.HomeOf(o, m, tc.name)
		if tc.err == "" && (err != nil || home != tc.home) || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("%s: home %q err %v", tc.name, home, err)
		}
	}
	_, err = identity.HomeOf(o, m, "nobody")
	if !strings.Contains(err.Error(), "    only           beta") {
		t.Errorf("the unknown-identity error lists the identities: %v", err)
	}
	os.WriteFile(reg, []byte("alpha="+a+"\nbeta="+b+"\naway=/no/such/dir\n"), 0o644)
	if _, err := identity.HomeOf(o, m, "only"); err == nil || !strings.Contains(err.Error(), "home 'away' is registered at /no/such/dir, which does not exist") {
		t.Errorf("problem home: %v", err)
	}
	if home, err := identity.HomeOf(o, m, "alpha"); err != nil || home != "alpha" {
		t.Errorf("a home name resolves despite a problem home: %q %v", home, err)
	}
	got := identity.List(o.FS(), m)
	var want []identity.Pair
	for _, p := range [][2]string{{"alpha", "alpha"}, {"shared", "alpha"}, {"beta", "beta"}, {"alpha", "beta"}, {"only", "beta"}, {"shared", "beta"}} {
		want = append(want, identity.Pair{Identity: p[0], Home: p[1]})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List %v", got)
	}
}
