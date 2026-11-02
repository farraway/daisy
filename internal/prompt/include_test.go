package prompt

import (
	"reflect"
	"strings"
	"testing"
)

// include.txt: one prompt per line, `~` for lazy, `#` comments and blank
// lines skipped, each line's words taken as xargs takes them (the bash
// trimmed through `echo | xargs`): blanks collapsed, quotes and
// backslashes honoured.
func TestParseInclude(t *testing.T) {
	in := "# header\n\n~daisy\npersonal\n  ~plan  \n\t# indented comment\n" +
		"two  words\n\"quoted  name\"\nback\\ slash\n~\n\v\n\vb\v c\nlast"
	got, err := ParseInclude([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Name: "daisy", Lazy: true},
		{Name: "personal"},
		{Name: "plan", Lazy: true},
		{Name: "two words"},
		{Name: "quoted  name"},
		{Name: "back slash"},
		{Name: "", Lazy: true},
		{Name: "b\v c"},
		{Name: "last"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseIncludeRefusesUnmatchedQuote(t *testing.T) {
	for _, in := range []string{"ok\nit's\n", "a\"b\n"} {
		_, err := ParseInclude([]byte(in))
		if err == nil || !strings.Contains(err.Error(), "line 2") && !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "unmatched") {
			t.Errorf("%q: err %v", in, err)
		}
	}
}
