package todo_test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"daisy/internal/todo"
)

// shapes are every line the two format documents show and every todo.txt
// or done.txt line the acceptance suite writes into a fixture.
func shapes(t *testing.T) []string {
	t.Helper()
	var lines []string
	for _, doc := range []string{"../../docs/todotxt.md", "../../docs/task-format.md"} {
		lines = append(lines, fencedLines(t, doc)...)
	}
	files, _ := filepath.Glob("../../tests/cases/*.sh")
	files = append(files, "../../tests/lib/fixture.sh")
	for _, f := range files {
		lines = append(lines, heredocLines(t, f)...)
	}
	if len(lines) < 60 {
		t.Fatalf("only %d shapes found; the sweep is not reading the documents", len(lines))
	}
	return lines
}

// fencedLines are the lines inside ``` fences that look like tasks: the
// documents' examples, less the regex grammars and prose.
func fencedLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	in := false
	for _, l := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(l, "```"):
			in = !in
		case in && l != "" && !strings.HasPrefix(l, "^") && !strings.HasPrefix(l, "Where") && !strings.HasPrefix(l, " ") && !strings.Contains(l, "→") && !strings.HasPrefix(l, "Before:") && !strings.HasPrefix(l, "After:"):
			out = append(out, l)
		}
	}
	return out
}

var heredocStart = regexp.MustCompile(`^\s*fixture_write_(todo|done) <<'EOF'$`)

func heredocLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case heredocStart.MatchString(l):
			in = true
		case in && l == "EOF":
			in = false
		case in:
			out = append(out, l)
		}
	}
	return out
}

func TestFormatParseRoundTripsEveryShape(t *testing.T) {
	seen := 0
	for _, l := range shapes(t) {
		seen++
		if got := todo.Parse(l).Format(); got != l {
			t.Errorf("Format(Parse(%q)) = %q", l, got)
		}
	}
	t.Logf("%d shapes", seen)
	// and the odd ones the documents describe in prose
	for _, l := range []string{"", " ", "x", "x ", "z ", "(A)", "(A) ", "(A)  2011-03-02 two spaces", "2011-03-02 ", "x 2011-03-03 2011-03-01 2011-02-01 three dates", "\tindented", "x 2011-03-03 (A) priority after x", "trailing space ", "(a) lower", "(A)no space", "x2011-03-03 glued"} {
		if got := todo.Parse(l).Format(); got != l {
			t.Errorf("Format(Parse(%q)) = %q", l, got)
		}
	}
}

func TestParseFields(t *testing.T) {
	for _, tc := range []struct {
		line string
		want todo.Task
	}{
		{"(A) Call Mom", todo.Task{Priority: 'A', Text: "Call Mom"}},
		{"(A) 2011-03-02 Call Mom", todo.Task{Priority: 'A', Created: "2011-03-02", Text: "Call Mom"}},
		{"2011-03-02 Document +TodoTxt task format", todo.Task{Created: "2011-03-02", Text: "Document +TodoTxt task format"}},
		{"(A) Call Mom 2011-03-02", todo.Task{Priority: 'A', Text: "Call Mom 2011-03-02"}},
		{"Really gotta call Mom (A) @phone @someday", todo.Task{Text: "Really gotta call Mom (A) @phone @someday"}},
		{"(b) Get back to the boss", todo.Task{Text: "(b) Get back to the boss"}},
		{"(B)->Submit TPS report", todo.Task{Text: "(B)->Submit TPS report"}},
		{"x 2011-03-03 Call Mom", todo.Task{State: todo.Done, Closed: "2011-03-03", Text: "Call Mom"}},
		{"x 2011-03-02 2011-03-01 Review Tim's pull request +TodoTxtTouch @github", todo.Task{State: todo.Done, Closed: "2011-03-02", Created: "2011-03-01", Text: "Review Tim's pull request +TodoTxtTouch @github"}},
		{"z 2026-01-15 2026-01-14 Old task @context", todo.Task{State: todo.Cancelled, Closed: "2026-01-15", Created: "2026-01-14", Text: "Old task @context"}},
		{"xylophone lesson", todo.Task{Text: "xylophone lesson"}},
		{"X 2012-01-01 Make resolutions", todo.Task{Text: "X 2012-01-01 Make resolutions"}},
		{"(A) x Find ticket prices", todo.Task{Priority: 'A', Text: "x Find ticket prices"}},
		{"x Call Mom", todo.Task{State: todo.Done, Text: "Call Mom"}},
		{"", todo.Task{}},
	} {
		if got := todo.Parse(tc.line); got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.line, got, tc.want)
		}
	}
}

func TestComplete(t *testing.T) {
	for _, tc := range []struct{ line, date, want string }{
		{"(B) 2026-01-13 Fix bug @jira +PROJ-1234", "2026-01-15", "x 2026-01-15 2026-01-13 Fix bug @jira +PROJ-1234"},
		{"2026-01-15 Triage this task +INBOX", "2026-01-16", "x 2026-01-16 2026-01-15 Triage this task +INBOX"},
		// no creation date: one space, not two, and the priority still goes
		{"(A) Undated task", "2026-01-16", "x 2026-01-16 Undated task"},
	} {
		if got := todo.Parse(tc.line).Complete(tc.date).Format(); got != tc.want {
			t.Errorf("Complete(%q, %s) = %q, want %q", tc.line, tc.date, got, tc.want)
		}
	}
}

func TestProjects(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"(A) Call Mom +Family +PeaceLoveAndHappiness @iphone @phone", []string{"Family", "PeaceLoveAndHappiness"}},
		{"Learn how to add 2+2", nil},
		{"Email SoAndSo at soandso@example.com", nil},
		{"+lead then +trail", []string{"lead", "trail"}},
		{"a lone + sign", nil},
	} {
		got := todo.Parse(tc.line).Projects()
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("Projects(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
	tk := todo.Parse("(B) 2026-07-27 Task one +Proj")
	if !tk.HasProject("proj") || tk.HasProject("pro") || tk.HasProject("proj-dev") {
		t.Error("HasProject: case-insensitive whole tag")
	}
	if !todo.Parse("x 2026-07-01 2026-06-01 Old +daisy").HasProject("daisy") {
		t.Error("HasProject on a completed line")
	}
}

func TestLinesAndContent(t *testing.T) {
	for _, tc := range []struct {
		content string
		want    []string
	}{
		{"", nil},
		{"a\n", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
	} {
		got := todo.Lines(tc.content)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") || len(got) != len(tc.want) {
			t.Errorf("Lines(%q) = %q, want %q", tc.content, got, tc.want)
		}
	}
	if got := todo.Content([]string{"a", "", "b"}); got != "a\n\nb\n" {
		t.Errorf("Content = %q", got)
	}
	if got := todo.Content(nil); got != "" {
		t.Errorf("Content(nil) = %q", got)
	}
}

func TestContainsFold(t *testing.T) {
	if !todo.ContainsFold("Release version 1.0 +daisy", "VERSION 1.0") || todo.ContainsFold("Release version 1X0", "version 1.0") || !todo.ContainsFold("-1 point deduction", "-1 point") {
		t.Error("ContainsFold: case-insensitive literal substring")
	}
}
