package provider

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The real grok over the argv Command builds (section 9, the xai row; a
// check of section 14's item 87): a headless session is created from it,
// so no flag is refused, and the member reads a resource under its
// working directory, laid out as a member's /work is. Opt-in: it runs only
// with DAISY_TEST_GROK_HOME naming a scratch copy of a logged-in grok
// home (never a swarm's login slot), since it needs the login and the
// network; DAISY_TEST_GROK_MODEL names the model (grok-4.7, the wizard's
// default, when unset). The argv's closing line names
// /work/.swarm/out.md, which the host lacks: what is checked is the
// session and the reply, not the output file.
func TestRealGrokTakesTheArgv(t *testing.T) {
	home := os.Getenv("DAISY_TEST_GROK_HOME")
	if home == "" {
		t.Skip("DAISY_TEST_GROK_HOME unset: the real grok check is opt-in")
	}
	if abs, err := filepath.Abs(home); err != nil || strings.Contains(filepath.ToSlash(abs), "/swarm/credentials/") {
		t.Fatalf("DAISY_TEST_GROK_HOME %s: a scratch copy of a grok home, never a swarm's login slot", home)
	}
	model := os.Getenv("DAISY_TEST_GROK_MODEL")
	if model == "" {
		model = "grok-4.7"
	}
	w := t.TempDir()
	token := "daisy-resource-token-5f3a9c"
	for f, text := range map[string]string{
		".swarm/system.md":          "Reply with the one word on the first line of " + filepath.Join(w, ".resources/docs/README.md") + " and nothing else. Do not write any file.\n",
		".swarm/prompt.md":          "There is no material.\n",
		".resources/docs/README.md": token + "\n",
	} {
		p := filepath.Join(w, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	argv := Providers["xai"].Adapter.Command(model, "high", w, filepath.Join(w, ".swarm/system.md"), filepath.Join(w, ".swarm/prompt.md"),
		[]string{filepath.Join(w, ".resources/docs"), t.TempDir()})
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = w
	cmd.Env = append(os.Environ(), "GROK_HOME="+home)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil || strings.Contains(stderr.String(), "Couldn't create session") {
		t.Fatalf("grok %q: %v\nstdout:\n%s\nstderr:\n%s", argv[1:], err, stdout.String(), stderr.String())
	}
	if rep := (grok{}).Report(stdout.Bytes()); !rep.Known || rep.Fault != "" || !strings.Contains(stdout.String(), token) {
		t.Errorf("report %+v\nstdout:\n%s", rep, stdout.String())
	}
}
