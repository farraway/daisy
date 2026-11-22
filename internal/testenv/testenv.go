// Package testenv pins, for a package's tests, what the processes those
// tests start read from the machine (praxis rule 14): the tests pin every
// variable the program reads through Ops, but git and other children
// inherit the test process's own environment. Imported by tests only.
package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Main runs the package's tests with HOME a temporary directory, USER a
// name in no file, no git variable from outside and no global or system git
// config, and none of Daisy's own variables that would change a child's
// behaviour from outside the test.
func Main(m *testing.M) { MainWithGitConfig(m, "") }

// MainWithGitConfig is Main with the global git config a file in HOME
// holding config when config is not empty, set once for the whole
// package: a test that set it itself (t.Setenv) could not run in
// parallel (t.Parallel), and no test may change it.
func MainWithGitConfig(m *testing.M, config string) {
	home, err := os.MkdirTemp("", "daisy-test-home-")
	if err != nil {
		panic(err)
	}
	global := os.DevNull
	if config != "" {
		global = filepath.Join(home, ".gitconfig")
		if err := os.WriteFile(global, []byte(config), 0o444); err != nil {
			panic(err)
		}
	}
	// A GIT_* variable from outside (GIT_DIR inside a hook, GIT_CONFIG_COUNT
	// and its pairs, GIT_INDEX_FILE) would point a test's git elsewhere.
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "GIT_") {
			os.Unsetenv(k)
		}
	}
	for k, v := range map[string]string{"HOME": home, "USER": "fixtureuser", "GIT_CONFIG_GLOBAL": global, "GIT_CONFIG_NOSYSTEM": "1"} {
		os.Setenv(k, v)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "DAISY_HEALTHCHECK_TTL", "DAISY_HEALTHCHECK_PASSED"} {
		os.Unsetenv(k)
	}
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// SkipAsRoot skips a test that takes a file's permissions away to see
// Daisy fail to read, write or remove it: run as root, every one of those
// succeeds whatever the mode, and the test would go red for the wrong
// reason. TestChmodTestsSkipAsRoot (cmd/daisy/rules_test.go) holds every
// test that sets mode 0 to call it.
func SkipAsRoot(t testing.TB) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("run as root, who reads, writes and removes a file whatever its mode")
	}
}
