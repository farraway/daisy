package cmd

import (
	"testing"

	"daisy/internal/testenv"
)

// TestMain gives the package's git, once for every test, the maintainer's
// identity as its global config: the identity that commits in the swarm
// repository's clone (newSwarmWS). Set once, not per test, so that the
// tests can run in parallel.
func TestMain(m *testing.M) {
	testenv.MainWithGitConfig(m, "[user]\n\tname = Jane\n\temail = jane@example.net\n")
}
