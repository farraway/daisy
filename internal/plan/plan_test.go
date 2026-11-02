package plan

import (
	"strings"
	"testing"
)

// The slug seam (case 55): the bash's sed pipeline, then the cap.
func TestSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Fix four defects", "Fix-four-defects"},
		{"  --a  b--  ", "a-b"},
		{"a/b.c_d", "a-b-c-d"},
		{"café au lait", "caf-au-lait"},
		{"!!!", ""},
		{"", ""},
		{"two\nlines", "two-lines"},
		{"x" + strings.Repeat("y", 120), "x" + strings.Repeat("y", 99)},
		// Cut at 100 lands on a hyphen: it is trimmed, not kept.
		{strings.Repeat("a", 99) + " bcd", strings.Repeat("a", 99)},
	} {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Every slug is bounded, and a filename made of one is under the limit
// whatever the description; only the project can overrun it.
func TestSlugBoundsTheFilename(t *testing.T) {
	long := "fix four defects surfaced by the previous session namely that plan archive leaves dead project links and a nonexistent daisy add command appears in the rules and the optimize floor strands entries at small n and dry run previews a different sample than the apply does"
	s := Slug(long)
	if len(s) > SlugMax || !strings.HasPrefix(s, "fix-four-defects") || strings.HasSuffix(s, "-") {
		t.Fatalf("Slug(long) = %q (%d bytes)", s, len(s))
	}
	name := FileName("260101010101", "testproj", s)
	if err := CheckName(name); err != nil {
		t.Fatalf("CheckName(%q): %v", name, err)
	}
	if !strings.Contains(name, ".testproj.") || !strings.HasSuffix(name, ".plan.md") {
		t.Fatalf("FileName = %q", name)
	}
	over := FileName("260101010101", strings.Repeat("p", 250), "a-short-description")
	err := CheckName(over)
	if err == nil || !strings.Contains(err.Error(), "the limit is 255") || !strings.Contains(err.Error(), "Shorten the project tag") {
		t.Fatalf("CheckName(%d bytes) = %v", len(over), err)
	}
	if got := FileName("260101010101", "", "s"); got != "260101010101.s.plan.md" {
		t.Fatalf("FileName without a project = %q", got)
	}
}

func TestFill(t *testing.T) {
	tmpl := "# {Description}{project_suffix}\n\n**Project:** +{project}\n**Created:** {YYYY-MM-DD HH:MM}\n\nRe-read `$DAISY_ROOT/prompts/praxis.md`, `$DAISY_ROOT`.\n"
	got := Fill(tmpl, `a \t & b`, "proj", "2026-11-01 09:30", "/r&oot")
	want := "# a \\t & b — +proj\n\n**Project:** +proj\n**Created:** 2026-11-01 09:30\n\nRe-read `/r&oot/prompts/praxis.md`, `/r&oot`.\n"
	if got != want {
		t.Fatalf("Fill with a project:\n%q\nwant\n%q", got, want)
	}
	got = Fill(tmpl, "d", "", "C", "/r")
	want = "# d\n\n**Created:** C\n\nRe-read `/r/prompts/praxis.md`, `/r`.\n"
	if got != want {
		t.Fatalf("Fill without a project:\n%q\nwant\n%q", got, want)
	}
	if got := Fill("no newline", "d", "", "C", "/r"); got != "no newline\n" {
		t.Fatalf("Fill ends every line: %q", got)
	}
}

func TestHeader(t *testing.T) {
	for _, tc := range []struct{ in, desc, proj string }{
		{"intro\n# A plan — +proj\n# later\n", "A plan", "proj"},
		{"# A plan\n", "A plan", ""},
		{"#not\n# x — +a — +b\n", "x — +a", "b"},
	} {
		d, p, ok := Header(tc.in)
		if !ok || d != tc.desc || p != tc.proj {
			t.Errorf("Header(%q) = %q, %q, %v", tc.in, d, p, ok)
		}
	}
	for _, in := range []string{"", "#x\n", "# \n", "text\n"} {
		if _, _, ok := Header(in); ok {
			t.Errorf("Header(%q) found a header", in)
		}
	}
}

// The repoint seam (case 54): every link naming the plan moves to the
// archive; a file without one is not changed by a byte, dots in the name
// are literal, and an already-archived link is not repointed again.
func TestRepoint(t *testing.T) {
	name := "260101010101.testproj.a-plan.plan.md"
	in := "- [../plans/" + name + "](../plans/" + name + ") — status: built\n- [x](../../plans/" + name + ")"
	got, ok := Repoint(in, name)
	want := "- [../plans/archive/" + name + "](../plans/archive/" + name + ") — status: built\n- [x](../../plans/archive/" + name + ")"
	if !ok || got != want {
		t.Fatalf("Repoint:\n%q, %v\nwant\n%q", got, ok, want)
	}
	if again, ok := Repoint(got, name); ok || again != got {
		t.Fatalf("Repoint twice changed %q to %q", got, again)
	}
	other := "- [../plans/260101010101Xtestproj.a-plan.plan.md](../plans/260202020202.other.unrelated.plan.md)\n"
	if got, ok := Repoint(other, name); ok || got != other {
		t.Fatalf("Repoint touched an unrelated file: %q", got)
	}
	// The name is a whole path component's prefix only as written: a longer
	// plan whose name starts with this one is the bash's match too.
	if _, ok := Repoint("../plans/"+name, name); !ok {
		t.Fatal("Repoint missed a bare link")
	}
}

// An empty template is an empty plan, as awk made it (pass 1, F5).
func TestFillEmpty(t *testing.T) {
	if got := Fill("", "d", "p", "C", "/r"); got != "" {
		t.Fatalf("Fill(\"\") = %q", got)
	}
}
