package reconcile

import "testing"

// The stop's cause (section 7.5): the Cause: line is the next line not
// blank under the stop's line, which is the first line neither blank nor
// Met:; a line read with Windows line ends as with Unix ones.
func TestStopCause(t *testing.T) {
	for _, tc := range []struct {
		out, word string
		found     bool
	}{
		{"BLOCKED: x\nCause: plan\n", "plan", true},
		{"BLOCKED: x\r\nCause: ruling \r\n", "ruling", true},
		{"\nMet: briefs/a.md\n\nBLOCKED: x\n\n\nCause: capacity\n", "capacity", true},
		{"QUESTION: x\nCause:\n", "", true},
		{"QUESTION: x\nwhy\nCause: plan\n", "", false},
		{"BLOCKED: x\n", "", false},
		{"Done.\nBLOCKED: x\nCause: plan\n", "", false},
		{"", "", false},
	} {
		if word, found := stopCause(tc.out); word != tc.word || found != tc.found {
			t.Errorf("stopCause(%q) = %q, %v, want %q, %v", tc.out, word, found, tc.word, tc.found)
		}
	}
}
