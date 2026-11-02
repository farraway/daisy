package main_test

import (
	"os"
	"testing"
)

// The functional tests read the root's own files (templates/, AGENTS.md)
// by paths relative to the module root, where they lived before Step 25;
// go test runs them in this package's directory, so move there first.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
