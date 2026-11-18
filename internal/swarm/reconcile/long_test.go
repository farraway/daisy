package reconcile

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A text longer than the limit is posted cut, its note then its tail
// after it, the post within the limit; with no file the continuations
// hold the rest, each within the limit, so that the post's text and
// theirs together are the text, cut at line breaks, or inside a line
// longer than the limit before a character, never inside one; the note's
// count is theirs; a text that fits with its tail is posted as it is
// (section 7.5).
func TestLongPosted(t *testing.T) {
	lines := strings.Repeat("a line of text, long enough to count\n", 5000)
	wide := strings.Repeat("é", 100000) // one line of two-byte characters
	for _, c := range []struct {
		name, text, tail string
		parts            int
	}{
		{"fits", "short\n", "\n\n**Check:** ok", 0},
		{"lines", lines, "", 3},
		{"lines with a tail", lines, "\n\n**Check:** " + strings.Repeat("x", 500), 3},
		{"one wide line", wide, " Remove swarm:blocked to resume.", 3},
		{"bytes", strings.Repeat("\x80", 130000), "", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := long{text: c.text, tail: c.tail, file: "1-worker-1.md", run: "1-worker-1", what: "worker run 1's text"}
			body, more := l.posted("")
			if c.parts == 0 {
				if body != c.text+c.tail || more != nil {
					t.Fatalf("a text that fits: %q, %d more", body, len(more))
				}
				return
			}
			if len(body) > maxPost || !strings.HasSuffix(body, c.tail) || len(more) != c.parts {
				t.Fatalf("post of %d bytes, %d continuations", len(body), len(more))
			}
			note := l.note("", len(more))
			head, ok := strings.CutSuffix(strings.TrimSuffix(body, c.tail), note)
			if !ok {
				t.Fatalf("the post does not end in %q and its tail:\n%s", note, body[max(0, len(body)-200):])
			}
			if !strings.HasSuffix(head, "\n") {
				t.Fatalf("the note is not on its own line")
			}
			if !strings.HasSuffix(c.text[:len(head)], "\n") {
				head = strings.TrimSuffix(head, "\n") // the line break before the note, past a cut inside a line
			}
			got := head
			for i, m := range more {
				want := continuedMarker + "\nContinued from worker run 1's text (part " + itoa(i+1) + " of " + itoa(len(more)) + "):\n\n"
				piece, ok := strings.CutPrefix(m, want)
				if !ok || len(piece) > maxPost || piece == "" {
					t.Fatalf("continuation %d: %d bytes, begins %q", i+1, len(piece), m[:min(len(m), 120)])
				}
				if utf8.ValidString(c.text) && !utf8.ValidString(piece) {
					t.Errorf("continuation %d cuts a character", i+1)
				}
				got += piece
			}
			if got != c.text {
				t.Errorf("the comments hold %d bytes of %d", len(got), len(c.text))
			}
			if url := "https://forge.example.net/attachments/x"; true {
				body, more := l.posted(url)
				if len(body) > maxPost || more != nil || !strings.HasSuffix(body, l.note(url, 0)+c.tail) {
					t.Errorf("attached: %d bytes, %d continuations", len(body), len(more))
				}
			}
		})
	}
}

// The note's forms (section 7.5): a run's output names its run, then the
// file's link or the continuations' count; a note of Daisy's own names
// the file or the count alone.
func TestLongNote(t *testing.T) {
	run := long{file: "7-architect-2.md", run: "7-architect-2"}
	own := long{file: "7-gate.md"}
	for _, c := range []struct{ got, want string }{
		{run.note("https://f/a/1", 0), "[truncated by Daisy; the whole output is in runs/7-architect-2 and attached as [7-architect-2.md](https://f/a/1)]"},
		{run.note("", 1), "[truncated by Daisy; the whole output is in runs/7-architect-2; the rest follows in 1 comment]"},
		{run.note("", 12), "[truncated by Daisy; the whole output is in runs/7-architect-2; the rest follows in 12 comments]"},
		{own.note("https://f/a/1", 0), "[truncated by Daisy; the whole note is attached as [7-gate.md](https://f/a/1)]"},
		{own.note("", 2), "[truncated by Daisy; the rest follows in 2 comments]"},
	} {
		if c.got != c.want {
			t.Errorf("%q, want %q", c.got, c.want)
		}
	}
}

// cutRun reads the note's every form, the form before Step 38d among
// them, on the text's last such line, and nothing else: a note naming
// another run, one with a tail of another shape or text after it on its
// line, and an entry of a kind (a continuation's) are not the cut.
func TestCutRun(t *testing.T) {
	arch := Mark{Role: "architect", Run: 2}
	for _, c := range []struct {
		m    Mark
		text string
		cut  bool
	}{
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2]", true},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2 and attached as [7-architect-2.md](https://f/a/1)]", true},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2; the rest follows in 3 comments]\n\n**Check:** ok", true},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2; the rest follows in 1 comment]", true},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-20]", false},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2; see /etc/passwd]", false},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2] and more", false},
		{arch, "plan\n[truncated by Daisy; the whole output is in runs/7-architect-2 and attached as [x](https://f/a b)]", false},
		{Mark{Role: "reconciler", Kind: "continued"}, "plan\n[truncated by Daisy; the whole output is in runs/7-reconciler-0]", false},
	} {
		run, at, end := cutRun(7, c.m, c.text)
		if c.cut != (at >= 0) || c.cut && (run != "7-architect-2" || !strings.HasPrefix(c.text[at:end], "[truncated") || !strings.HasSuffix(c.text[at:end], "]")) {
			t.Errorf("cutRun(%q) = %q, %d, %d", c.text, run, at, end)
		}
	}
}

// A prompt shows a text cut on the forge once, in whichever form its note
// took there, the note naming the member's copy, and no continuation:
// the copy holds the rest (sections 7.3 and 7.5).
func TestRenderCutNoteForms(t *testing.T) {
	note := func(where string) string { return "[truncated by Daisy; the whole output is in " + where + "]" }
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan+"\n[truncated by Daisy; the whole output is in runs/1-architect-1 and attached as [1-architect-1.md](https://forge.example.net/attachments/a1)]")
	f.Comment(at, marker("owner", 3)+"Long.\n[truncated by Daisy; the whole output is in runs/1-owner-3; the rest follows in 1 comment]")
	f.Comment(at, continuedMarker+"\nContinued from owner run 3's text (part 1 of 1):\n\nQUESTION: the rest of the owner's text")
	j := job(f, at.N, "architect", "swarm:proposal-accepted")
	j.Outputs = outputsAt(t, map[string]string{"1-architect-1": "", "1-owner-3": ""})
	if err := j.readWhole(); err != nil {
		t.Fatal(err)
	}
	_, prompt := Render(j)
	has(t, "prompt.md", prompt, []string{
		"it works.\n\n" + note("/work/.swarm/outputs/1-architect-1.md") + "\n",
		"Long.\n" + note("/work/.swarm/outputs/1-owner-3.md") + "\n",
	}, "runs/1-", "attached as", "the rest follows", "Continued from", "QUESTION:", "reconciler, run")
	if got := j.Cut(); strings.Join(got, " ") != "1-architect-1 1-owner-3" {
		t.Errorf("Cut: %v", got)
	}
	if _, ok := newestQuestion(j.Issue, "swarm-bot"); ok {
		t.Errorf("a continuation beginning QUESTION: read as a question")
	}
}
