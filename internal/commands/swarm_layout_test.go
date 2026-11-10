package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
)

// Every swarm verb from a swarm repository's clone (section 13, Step 2's
// rows; revision 6's plan, Step 2's claim): no .daisy/ in it or above it,
// swarm.json and prompts/ at its top, the verbs finding it from a folder
// inside it and from its top; init names its files relative to the clone
// and prints the unit for the clone, %f the clone; apply's docker and id
// run in the clone; status, pause and resume run there. The maintainer's
// verbs and a launching pass run in both layouts (bothLayouts).
func TestSwarmVerbsFromAClone(t *testing.T) {
	a := newApplyWSIn(t, true)
	for dir := a.ws; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".daisy")); err == nil {
			t.Fatalf("%s holds .daisy", dir)
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	if _, err := os.Stat(filepath.Join(a.ws, "swarm")); err == nil {
		t.Error("swarm/ written in the clone")
	}
	if a.cwd != filepath.Join(a.ws, "prompts") {
		t.Fatalf("cwd %s", a.cwd)
	}

	if err := a.run("--defaults"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	wantOut := "✓ Token $GITEA_TOKEN reads acme/app as swarm-bot\n  Unchanged swarm.json\n"
	for _, r := range declare.Roles {
		wantOut += "  Kept prompts/" + r + ".md (exists)\n"
	}
	wantOut += "✓ Labels: 16 present (0 created, 0 renamed)\n" + strings.Replace(a.units(), "for the workspace %f", "for the swarm repository %f", 1)
	if a.stdout.String() != wantOut || !strings.Contains(wantOut, "daisy-swarm@"+systemdEscapePath(a.ws)+".service") {
		t.Errorf("init stdout:\n%s\nwant:\n%s", a.stdout.String(), wantOut)
	}
	if !strings.Contains(a.stderr.String(), "check is empty; no member's work is executed (set it in swarm.json)\n") {
		t.Errorf("init stderr:\n%s", a.stderr.String())
	}

	for _, cwd := range []string{a.cwd, a.ws} {
		a.cwd = cwd
		if err := a.status(a.forge); err != nil || !strings.HasPrefix(a.stdout.String(), "Swarm: acme/app on forge.example.net (gitea), repositories app, lib\n") {
			t.Errorf("status from %s: %v\n%s", cwd, err, a.stdout.String())
		}
	}

	a.idDirs = nil
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if a.runDir != a.ws || strings.Join(a.idDirs, ",") != a.ws+","+a.ws {
		t.Errorf("docker in %q, id in %q; want %s", a.runDir, a.idDirs, a.ws)
	}

	if err := a.verb(SwarmPause); err != nil || a.stdout.String() != "Paused: no new member will launch ("+a.state("paused")+"); running members finish and are posted\n" {
		t.Errorf("pause: %v %q", err, a.stdout.String())
	}
	if err := a.verb(SwarmResume); err != nil || !strings.HasPrefix(a.stdout.String(), "Resumed: members launch at the next pass\n") {
		t.Errorf("resume: %v %q", err, a.stdout.String())
	}
	// The cli's start for a verb, from a folder holding no swarm.
	a.cwd = t.TempDir()
	if err := a.verb(SwarmStatus); errText(err) != "Error: swarm.json: not found in "+a.cwd+" or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'" {
		t.Errorf("no swarm: %v", err)
	}
}

// The secrets (section 3, Secret values; section 13, Step 2's second
// row): in a clone, <state>/env, read and never run, the environment the
// fallback, refused at mode 0640 or 0604 before it is read, a .env.sh in
// the clone read by nothing and warned of by status and apply; in the old
// layout, the workspace's .env.sh and never <state>/env.
func TestSwarmSecretsFile(t *testing.T) {
	a := newApplyWSIn(t, true)
	env := a.secretsFile()
	if env != a.state("env") {
		t.Fatalf("secrets at %s", env)
	}

	for _, mode := range []os.FileMode{0o640, 0o604} {
		os.Chmod(env, mode)
		want := "Error: swarm: " + env + " is readable by others (mode " + map[os.FileMode]string{0o640: "0640", 0o604: "0604"}[mode] + "); run chmod 600 on it"
		if err := a.status(a.forge); errText(err) != want {
			t.Errorf("status at %o: %v", mode, err)
		}
		if err := a.apply(a.forge, "--once"); errText(err) != want {
			t.Errorf("apply at %o: %v", mode, err)
		}
		if _, err := os.Stat(a.state("lock")); err == nil {
			t.Errorf("apply at %o took the lock", mode)
		}
	}
	os.Chmod(env, 0o600)

	// Read, never run: a substitution stops the reading, named by path.
	a.secrets("export A=$(touch " + filepath.Join(a.ws, "ran") + ")\nexport GITEA_TOKEN=tok-123\n")
	err := a.status(a.forge)
	if errText(err) != "Error: swarm: $GITEA_TOKEN is not set (in "+env+" or the environment)" || !strings.HasPrefix(a.stderr.String(), "⚠️  "+env+" line 1: ") {
		t.Errorf("a line not read: %v %q", err, a.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(a.ws, "ran")); err == nil {
		t.Error("<state>/env was run")
	}

	// The environment's value when the file does not assign it, and when
	// there is no file.
	a.vars["GITEA_TOKEN"] = "tok-123"
	a.secrets("")
	if err := a.status(a.forge); err != nil {
		t.Errorf("from the environment: %v", err)
	}
	os.Remove(env)
	a.tokens = nil
	if err := a.status(a.forge); err != nil || strings.Join(a.tokens, ",") != "tok-123" {
		t.Errorf("no file: %v %q", err, a.tokens)
	}
	delete(a.vars, "GITEA_TOKEN")

	// A .env.sh in the clone: read by nothing, warned of.
	a.secrets("export GITEA_TOKEN=tok-123\n")
	a.write(".env.sh", "export GITEA_TOKEN=\"from-the-clone\"\n")
	warning := "⚠ " + filepath.Join(a.ws, ".env.sh") + ": the swarm reads its secrets from " + env + ", not the clone; move them there and delete it\n"
	a.tokens = nil
	if err := a.status(a.forge); err != nil || a.stderr.String() != warning {
		t.Errorf("status: %v %q", err, a.stderr.String())
	}
	if err := a.apply(a.forge, "--once"); err != nil || a.stderr.String() != warning {
		t.Errorf("apply: %v %q", err, a.stderr.String())
	}
	if strings.Join(a.tokens, ",") != "tok-123,tok-123" {
		t.Errorf("tokens %q", a.tokens)
	}

	// The old layout: <ws>/.env.sh, and <state>/env never read.
	o := newStatusWS(t)
	o.write(".env.sh", "")
	os.MkdirAll(o.state(""), 0o755)
	os.WriteFile(o.state("env"), []byte("export GITEA_TOKEN=tok-123\n"), 0o644)
	if err := o.status(o.forge); errText(err) != "Error: swarm: $GITEA_TOKEN is not set (in "+filepath.Join(o.ws, ".env.sh")+" or the environment)" || o.stderr.Len() != 0 {
		t.Errorf("old layout: %v %q", err, o.stderr.String())
	}
}
