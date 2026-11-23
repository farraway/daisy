package reconcile

import "testing"

// The intervention types are the catalogue's I1 to I17, written as
// strconv writes the number (section 11, The interventions' record):
// I15 to I17 since Step SF-5 of the plan implementing the 2026-11-19
// swarm roadmap.
func TestIsInterventionType(t *testing.T) {
	for word, want := range map[string]bool{
		"I1": true, "I7": true, "I14": true, "I15": true, "I16": true, "I17": true,
		"I0": false, "I18": false, "I99": false, "I07": false, "I+7": false, "I-1": false,
		"i7": false, "7": false, "I": false, "X": false, "": false, " I7": false, "I7 ": false,
	} {
		if got := IsInterventionType(word); got != want {
			t.Errorf("IsInterventionType(%q) = %v, want %v", word, got, want)
		}
	}
}
