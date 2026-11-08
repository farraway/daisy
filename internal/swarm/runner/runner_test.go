package runner

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
)

func member() Spec {
	return Spec{Name: "daisy-swarm.acme.app.3.architect.1", Label: "acme/app#3/architect", User: "1000:1000",
		Mounts: []Mount{{Src: "/s/runs/3-architect-1/work", Dst: "/work"}, {Src: "/s/runs/3-architect-1/io", Dst: "/work/.swarm"},
			{Src: "/ws/docs", Dst: "/resources/docs", ReadOnly: true}},
		Workdir: "/work/app", Image: "daisy-member-app:2", Argv: []string{"claude", "-p"}}
}

// Section 7.2's two argvs: the member with an env credential, with a
// slot, and the check, keyless and without a network.
func TestArgs(t *testing.T) {
	env := member()
	env.EnvVar, env.EnvValue = "CLAUDE_CODE_OAUTH_TOKEN", "sk-secret"
	head := []string{"run", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.app.3.architect.1", "--label", "daisy.swarm=acme/app#3/architect",
		"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30"}
	mounts := []string{"--mount", "type=bind,src=/s/runs/3-architect-1/work,dst=/work", "--mount", "type=bind,src=/s/runs/3-architect-1/io,dst=/work/.swarm",
		"--mount", "type=bind,src=/ws/docs,dst=/resources/docs,readonly", "--workdir", "/work/app", "daisy-member-app:2", "claude", "-p"}
	cat := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	if got, want := env.Args(), cat(head, []string{"--env-file", "/dev/stdin"}, mounts); !reflect.DeepEqual(got, want) {
		t.Errorf("env member:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(strings.Join(env.Args(), " "), "sk-secret") {
		t.Error("the env value is on the argv")
	}
	slot := member()
	slot.Credential, slot.Slot = "openai/2", "/l/swarm/credentials/openai/2"
	if got, want := slot.Args(), cat(head, []string{"--label", "daisy.credential=openai/2", "--mount", "type=bind,src=/l/swarm/credentials/openai/2,dst=/credential"}, mounts); !reflect.DeepEqual(got, want) {
		t.Errorf("slot member:\n got %q\nwant %q", got, want)
	}
	check := Spec{Name: "daisy-swarm.acme.app.3.check.1", Label: "acme/app#3/check", User: "1000:1000", NoNetwork: true,
		Mounts: []Mount{{Src: "/s/runs/3-worker-1/check", Dst: "/work"}}, Workdir: "/work", Image: "daisy-member-app:2", Argv: []string{"make", "check"}}
	want := []string{"run", "--rm", "--pull", "never", "--name", "daisy-swarm.acme.app.3.check.1", "--label", "daisy.swarm=acme/app#3/check",
		"--user", "1000:1000", "--security-opt", "no-new-privileges", "--stop-timeout", "30", "--network", "none",
		"--mount", "type=bind,src=/s/runs/3-worker-1/check,dst=/work", "--workdir", "/work", "daisy-member-app:2", "make", "check"}
	if got := check.Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("check:\n got %q\nwant %q", got, want)
	}
}

// blocked is a process that exits with code when released.
type blocked struct {
	release chan struct{}
	code    int
}

func (p *blocked) Wait() (int, error) { <-p.release; return p.code, nil }
func (p *blocked) Kill() error        { return nil }

// Docker.Start passes the env value on standard input only, and at the
// wall clock stops the container by name and reports a timeout.
func TestDockerStartAndTheWallClock(t *testing.T) {
	var stdin string
	var stopped []string
	proc := &blocked{release: make(chan struct{}), code: 137}
	fire := make(chan time.Time, 1)
	var asked time.Duration
	o := &ops.Fake{
		StartFn: func(dir, name string, in io.Reader, stdout, stderr io.Writer, args ...string) (ops.Process, error) {
			if in != nil {
				b, _ := io.ReadAll(in)
				stdin = string(b)
			}
			io.WriteString(stdout, "out")
			return proc, nil
		},
		AfterFn: func(d time.Duration) <-chan time.Time { asked = d; return fire },
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			stopped = append(stopped, name+" "+strings.Join(args, " "))
			close(proc.release)
			return nil, nil, nil
		},
	}
	s := member()
	s.EnvVar, s.EnvValue = "CLAUDE_CODE_OAUTH_TOKEN", "sk-secret"
	c, err := Docker{O: o, Dir: "/ws"}.Start(s, 45*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	fire <- time.Time{}
	exit, err := c.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if stdin != "CLAUDE_CODE_OAUTH_TOKEN=sk-secret\n" || asked != 45*time.Minute {
		t.Errorf("stdin %q, timer %v", stdin, asked)
	}
	if !exit.TimedOut || exit.Code != 137 || string(exit.Stdout) != "out" || !reflect.DeepEqual(stopped, []string{"docker stop --time 30 daisy-swarm.acme.app.3.architect.1"}) {
		t.Errorf("exit %+v, stopped %q", exit, stopped)
	}

	// Without a timeout: the exit as docker run returns it, nothing stopped.
	stopped, stdin = nil, "none"
	proc = &blocked{release: make(chan struct{}), code: 0}
	close(proc.release)
	c, _ = Docker{O: o, Dir: "/ws"}.Start(member(), time.Hour)
	exit, _ = c.Wait()
	if exit.TimedOut || exit.Code != 0 || stopped != nil || stdin != "none" {
		t.Errorf("exit %+v, stopped %q, stdin %q", exit, stopped, stdin)
	}
}

// A stop that fails at the wall clock (the container gone already, --rm)
// is still a timeout, reported once docker run has returned, never before:
// the slot's lock is released only then (section 7.1).
func TestDockerStopFailingStillWaits(t *testing.T) {
	proc := &blocked{release: make(chan struct{}), code: 137}
	fire := make(chan time.Time, 1)
	returned := make(chan struct{})
	o := &ops.Fake{
		StartFn: func(dir, name string, in io.Reader, stdout, stderr io.Writer, args ...string) (ops.Process, error) {
			return proc, nil
		},
		AfterFn: func(time.Duration) <-chan time.Time { return fire },
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			return nil, []byte("Error response from daemon: No such container"), errors.New("exit status 1")
		},
	}
	c, err := Docker{O: o, Dir: "/ws"}.Start(member(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	fire <- time.Time{}
	var exit Exit
	go func() { exit, err = c.Wait(); close(returned) }()
	select {
	case <-returned:
		t.Fatal("Wait returned before docker run did")
	case <-time.After(50 * time.Millisecond):
	}
	close(proc.release)
	<-returned
	if err != nil || !exit.TimedOut || exit.Code != 137 {
		t.Errorf("exit %+v, err %v", exit, err)
	}
}

func TestDockerRunning(t *testing.T) {
	o := &ops.Fake{ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
		if got := name + " " + strings.Join(args, " "); got != `docker ps --filter label=daisy.swarm --format {{.Names}} {{.Label "daisy.swarm"}} {{.Label "daisy.credential"}}` {
			t.Errorf("exec %s", got)
		}
		return []byte("daisy-swarm.acme.app.3.adversary.1 acme/app#3/adversary openai/1\ndaisy-swarm.acme.app.4.check.2 acme/app#4/check \n"), nil, nil
	}}
	got, err := Docker{O: o}.Running()
	want := []Running{{"daisy-swarm.acme.app.3.adversary.1", "acme/app#3/adversary", "openai/1"}, {"daisy-swarm.acme.app.4.check.2", "acme/app#4/check", ""}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("%+v %v", got, err)
	}
	o.ExecFn = func(string, string, ...string) ([]byte, []byte, error) {
		return nil, []byte("Cannot connect to the Docker daemon\n"), errors.New("exit status 1")
	}
	if _, err := (Docker{O: o}).Running(); err == nil || err.Error() != "docker: Cannot connect to the Docker daemon" {
		t.Errorf("%v", err)
	}
}

// slots is a credentials directory of n openai slots, each logged in.
func slots(t *testing.T, n int) string {
	dir := t.TempDir()
	for j := 1; j <= n; j++ {
		d := filepath.Join(dir, itoa(j))
		os.MkdirAll(filepath.Join(d, "sessions"), 0o700)
		os.WriteFile(filepath.Join(d, "auth.json"), []byte(`{"tokens":{"access_token":"`+strings.Repeat("a", 31)+itoa(j)+`","id":"short"}}`), 0o600)
		os.WriteFile(filepath.Join(d, "config.toml"), []byte("model = \"x\"\n"), 0o600)
	}
	return dir
}

func itoa(n int) string { return string(rune('0' + n)) }

// Section 7.1, step 4: the lowest free slot; the snapshot, a held lock and
// the running set read again under the lock each pass a slot over; none
// free; the slot cleared of all but auth.json, its secrets read.
func TestTakeSlot(t *testing.T) {
	fs := ops.Real().FS()
	dir := slots(t, 3)
	r := &Fake{}
	s, err := TakeSlot(fs, r, dir, "openai", 3, "acme/app#3/adversary run 1", nil)
	if err != nil || s.J != 1 || s.Credential != "openai/1" {
		t.Fatalf("%+v %v", s, err)
	}
	if entries, _ := os.ReadDir(s.Dir); len(entries) != 1 || entries[0].Name() != "auth.json" {
		t.Errorf("slot 1 holds %v", entries)
	}
	if !reflect.DeepEqual(s.Secrets, []string{strings.Repeat("a", 31) + "1"}) {
		t.Errorf("secrets %q", s.Secrets)
	}
	// Slot 1's lock is held; slot 2 is busy in the snapshot; slot 3 busy
	// only in the running set read again.
	r.Others = []Running{{Name: "x", Swarm: "other/repo#1/adversary", Credential: "openai/3"}}
	if _, err := TakeSlot(fs, r, dir, "openai", 3, "h", []Running{{Credential: "openai/2"}}); !errors.Is(err, ErrNoSlot) {
		t.Errorf("all busy: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "3.lock")); len(b) == 0 {
		t.Error("slot 3 was not locked before the re-read")
	}
	r.Others = nil
	s3, err := TakeSlot(fs, r, dir, "openai", 3, "h", []Running{{Credential: "openai/2"}})
	if err != nil || s3.J != 3 {
		t.Fatalf("%+v %v", s3, err)
	}
	s3.Release()
	s.Release()
	if s, err := TakeSlot(fs, r, dir, "openai", 3, "h", nil); err != nil || s.J != 1 {
		t.Errorf("after release: %+v %v", s, err)
	} else {
		s.Release()
	}
}

// An auth.json that is a link is *slot refused*; its target is untouched
// and the lock released.
func TestTakeSlotRefusesALink(t *testing.T) {
	fs := ops.Real().FS()
	dir := slots(t, 1)
	target := filepath.Join(t.TempDir(), "host-auth.json")
	os.WriteFile(target, []byte("{}"), 0o600)
	auth := filepath.Join(dir, "1", "auth.json")
	os.Remove(auth)
	os.Symlink(target, auth)
	_, err := TakeSlot(fs, &Fake{}, dir, "openai", 1, "h", nil)
	var refused *SlotRefused
	if !errors.As(err, &refused) || err.Error() != "slot openai/1: auth.json is not a regular file" {
		t.Fatalf("%v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "{}" {
		t.Error("the link's target changed")
	}
	release, err := fs.Lock(filepath.Join(dir, "1.lock"), "x")
	if err != nil {
		t.Fatalf("the lock was kept: %v", err)
	}
	release()
}

// No directory for the provider (no login was ever made) is *slot
// refused* with the login's hint, as a slot directory without auth.json
// is (section 7.1, step 4).
func TestTakeSlotWithoutTheProviderDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "openai")
	_, err := TakeSlot(ops.Real().FS(), &Fake{}, dir, "openai", 1, "h", nil)
	var refused *SlotRefused
	if !errors.As(err, &refused) || err.Error() != "slot openai/1: no login; run 'CODEX_HOME="+filepath.Join(dir, "1")+" codex login -c cli_auth_credentials_store=file'" {
		t.Fatalf("%v", err)
	}
}
