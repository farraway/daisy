package main_test

import (
	"os"
	"testing"

	"daisy/internal/testenv"
)

// The functional tests read the root's own files (templates/, AGENTS.md)
// by paths relative to the module root, where they lived before Step 25;
// go test runs them in this package's directory, so move there first. The
// git and other processes they start see a pinned environment (testenv).
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	testenv.Main(m)
}
