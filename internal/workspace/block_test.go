package workspace

import "testing"

func isAB(line string) bool { return line == "a" || line == "b" || line == "c" }

// Adding twice equals adding once; removing what was added gives back the
// file byte for byte, user lines that happen to be entries included.
func TestBlockAddIsIdempotentAndRemoveUndoesIt(t *testing.T) {
	for _, original := range []string{"", "node_modules/\n", "b\nmine\n\n", "no final newline\n"} {
		once, changed := addBlock([]byte(original), "# H", []string{"a", "b"}, nil)
		if !changed && original != "b\nmine\n\n" {
			t.Errorf("%q: nothing added", original)
		}
		twice, changed := addBlock(once, "# H", []string{"a", "b"}, nil)
		if changed || string(twice) != string(once) {
			t.Errorf("%q: second add changed %q -> %q", original, once, twice)
		}
		back, _ := removeBlock(once, "# H", isAB)
		if string(back) != original {
			t.Errorf("%q: add then remove gave %q (added %q)", original, back, once)
		}
	}
}

// An entry missing from an existing block goes into the block, under the
// one header, not after whatever the user appended to the file.
func TestBlockAddFillsTheExistingBlock(t *testing.T) {
	in := "top\n\n# H\na\nb\n\nuser\n"
	got, _ := addBlock([]byte(in), "# H", []string{"a", "b", "c"}, nil)
	if string(got) != "top\n\n# H\na\nb\nc\n\nuser\n" {
		t.Errorf("got %q", got)
	}
	back, changed := removeBlock(got, "# H", isAB)
	if !changed || string(back) != "top\nuser\n" && string(back) != "top\n\nuser\n" {
		t.Errorf("remove gave %q", back)
	}
}
