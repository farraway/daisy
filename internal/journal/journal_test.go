package journal_test

import (
	"os"
	"strings"
	"testing"

	"daisy/internal/journal"
)

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// On the day and week templates the entry goes after the section's last
// entry, before the blank line that precedes the next heading: one line
// added, no blank line added or removed, everything else byte-identical.
func TestLogOnTemplates(t *testing.T) {
	for _, tpl := range []string{"../../templates/journal-day.md", "../../templates/journal-week.md"} {
		before := read(t, tpl)
		after, err := journal.Log(before, "1423", "Shipped the fix")
		if err != nil {
			t.Fatalf("%s: %v", tpl, err)
		}
		want := strings.Replace(before, "- {TIME} New day started\n", "- {TIME} New day started\n- 1423 Shipped the fix\n", 1)
		if after != want {
			t.Errorf("%s:\n%s", tpl, after)
		}
	}
}

func TestLogPlacement(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"entries abut the next heading",
			"#### Log\n\n- 0900 a\n- 0910 b\n#### Retrospective\n\n* x\n",
			"#### Log\n\n- 0900 a\n- 0910 b\n- 1000 m\n#### Retrospective\n\n* x\n"},
		{"blank lines inside the section are kept, the entry goes after the last one",
			"#### Log\n\n- 0900 a\n\n- 0910 b\n\n\n#### Retrospective\n",
			"#### Log\n\n- 0900 a\n\n- 0910 b\n- 1000 m\n\n\n#### Retrospective\n"},
		{"empty section: the template's shape, which the health check requires",
			"#### Log\n\n#### Retrospective\n",
			"#### Log\n\n- 1000 m\n\n#### Retrospective\n"},
		{"empty section with no blank line",
			"#### Log\n#### Retrospective\n",
			"#### Log\n\n- 1000 m\n\n#### Retrospective\n"},
		{"empty section with several blank lines",
			"#### Log\n\n\n\n#### Retrospective\n",
			"#### Log\n\n- 1000 m\n\n#### Retrospective\n"},
		{"empty section at end of file",
			"#### Tasks\n\n#### Log\n",
			"#### Tasks\n\n#### Log\n\n- 1000 m\n"},
		{"section at end of file",
			"#### Tasks\n\n#### Log\n\n- 0900 a\n",
			"#### Tasks\n\n#### Log\n\n- 0900 a\n- 1000 m\n"},
		{"end of file without a newline",
			"#### Log\n- 0900 a",
			"#### Log\n- 0900 a\n- 1000 m\n"},
		{"any heading level ends the section",
			"#### Log\n- 0900 a\n### Next day\n\n- stuff\n",
			"#### Log\n- 0900 a\n- 1000 m\n### Next day\n\n- stuff\n"},
		{"a later Log section is not the first",
			"#### Log\n- 0900 a\n\n#### Notes\n#### Log\n- old\n",
			"#### Log\n- 0900 a\n- 1000 m\n\n#### Notes\n#### Log\n- old\n"},
	} {
		got, err := journal.Log(tc.in, "1000", "m")
		if err != nil || got != tc.want {
			t.Errorf("%s: err %v\n got %q\nwant %q", tc.name, err, got, tc.want)
		}
	}
	if _, err := journal.Log("#### Tasks\n- [ ] a\n", "1000", "m"); err != journal.ErrNoLogSection {
		t.Errorf("no Log section: %v", err)
	}
	if _, err := journal.Log("", "1000", "m"); err != journal.ErrNoLogSection {
		t.Errorf("empty file: %v", err)
	}
}

func TestTick(t *testing.T) {
	in := "**Now:**\n- [ ] Alpha task +daisy\n- [ ] Alpha task +daisy\n- [ ] Alpha task +daisy more\n\n#### Log\n- 0900 Alpha task +daisy\n"
	got, ok := journal.Tick(in, "Alpha task +daisy")
	want := "**Now:**\n- [x] Alpha task +daisy\n- [ ] Alpha task +daisy\n- [ ] Alpha task +daisy more\n\n#### Log\n- 0900 Alpha task +daisy\n"
	if !ok || got != want {
		t.Errorf("ok %v\n got %q\nwant %q", ok, got, want)
	}
	if got, ok := journal.Tick(in, "Beta"); ok || got != in {
		t.Errorf("absent: ok %v changed %v", ok, got != in)
	}
	if got, ok := journal.Tick("- [ ] a", "a"); !ok || got != "- [x] a\n" {
		t.Errorf("no trailing newline: %q", got)
	}
}
