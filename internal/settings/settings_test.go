package settings_test

import (
	"strings"
	"testing"

	"daisy/internal/settings"
)

var rules = []string{"Read(.daisy/**)", "Edit(.daisy/**)", "Bash(daisy:*)"}

// Allow adds what is missing, keeps everything else where it was (key order
// included), and applied twice equals once; Disallow is its inverse for the
// rules it was given.
func TestAllowIsIdempotentAndDisallowUndoesIt(t *testing.T) {
	in := `{"model": "x", "permissions": {"deny": ["Bash(rm:*)"], "allow": ["Bash(ls:*)", "Bash(old/*)", "Read(.daisy/**)"]}, "a": [1, {"b": "<&>"}]}`
	once, err := settings.Allow([]byte(in), rules, []string{"Bash(old/*)"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "model": "x",
  "permissions": {
    "deny": [
      "Bash(rm:*)"
    ],
    "allow": [
      "Bash(ls:*)",
      "Read(.daisy/**)",
      "Edit(.daisy/**)",
      "Bash(daisy:*)"
    ]
  },
  "a": [
    1,
    {
      "b": "<&>"
    }
  ]
}
`
	if string(once) != want {
		t.Errorf("Allow:\n%s", once)
	}
	twice, _ := settings.Allow(once, rules, []string{"Bash(old/*)"})
	if string(twice) != string(once) {
		t.Errorf("not idempotent:\n%s", twice)
	}
	back, removed, empty, err := settings.Disallow(once, rules)
	if err != nil || empty || !removed {
		t.Fatalf("Disallow: %v empty=%v", err, empty)
	}
	if !strings.Contains(string(back), `"allow": [
      "Bash(ls:*)"
    ]`) || strings.Contains(string(back), "daisy") {
		t.Errorf("Disallow:\n%s", back)
	}
}

func TestAllowFromNothingAndBackToNothing(t *testing.T) {
	for _, in := range []string{"", "  \n", "{}"} {
		out, err := settings.Allow([]byte(in), rules, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "{\n  \"permissions\": {\n    \"allow\": [\n      \"Read(.daisy/**)\",\n      \"Edit(.daisy/**)\",\n      \"Bash(daisy:*)\"\n    ]\n  }\n}\n"
		if string(out) != want {
			t.Errorf("from %q:\n%s", in, out)
		}
		if _, _, empty, _ := settings.Disallow(out, rules); !empty {
			t.Errorf("only Daisy's rules: not empty")
		}
	}
}

// Anything else in the file keeps it: a deny list, another key.
func TestDisallowKeepsAFileWithOtherContent(t *testing.T) {
	for _, in := range []string{`{"permissions": {"allow": ["Bash(daisy:*)"], "deny": []}}`, `{"x": 1, "permissions": {"allow": []}}`, `{"x": 1}`} {
		if _, _, empty, _ := settings.Disallow([]byte(in), rules); empty {
			t.Errorf("%s: called empty", in)
		}
	}
}

func TestDisallowSaysWhenNothingWasThere(t *testing.T) {
	if _, removed, _, _ := settings.Disallow([]byte(`{"permissions":{"allow":["Bash(ls:*)"]}}`), rules); removed {
		t.Error("removed nothing, said removed")
	}
}

func TestRefusesWhatIsNotASettingsObject(t *testing.T) {
	for _, in := range []string{"{ not json", "[]", `{"a":1} {}`, `{"permissions": "x"}`, `{"permissions": {"allow": "x"}}`} {
		if _, err := settings.Allow([]byte(in), rules, nil); err == nil {
			t.Errorf("Allow accepted %q", in)
		}
	}
}
