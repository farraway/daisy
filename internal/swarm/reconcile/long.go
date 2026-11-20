package reconcile

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"daisy/internal/swarm/forge"
)

// maxPost is the longest text Daisy posts in one comment, review or pull
// request description (section 7.5).
const maxPost = 60000

// continuedMarker is a continuation's first line (section 7.5): the
// reconciler's, naming no run, so that no rule reads a continuation as a
// role's entry.
const continuedMarker = "<!-- daisy-swarm role=reconciler kind=continued -->"

// truncated begins the note that ends a text Daisy cut.
const truncated = "[truncated by Daisy; "

// long is a text Daisy posts by section 7.5's rule: the text, what Daisy
// writes after it and never cuts (tail: a worker's check block, a note's
// last sentences), the name it is attached as when it does not fit, the
// run whose output it is ("" for a note of Daisy's own), what its
// continuations call it, and whether it is a review, which the job's
// workspace holds until its round (writeReviews).
type long struct {
	text, tail string
	file       string
	run        string
	what       string
	review     bool
}

// fits: the text and its tail are posted as they are.
func (l long) fits() bool { return len(l.text)+len(l.tail) <= maxPost }

// note is the line that ends the text cut: for a run's output, the run it
// is whole in; then the file's link (url), or with none the count of the
// continuations, c.
func (l long) note(url string, c int) string {
	rest := "; the rest follows in " + plural(c, "comment")
	if url != "" {
		rest = " and attached as [" + l.file + "](" + url + ")"
	}
	switch {
	case l.run != "":
		return truncated + "the whole output is in runs/" + l.run + rest + "]"
	case url != "":
		return truncated + "the whole note is attached as [" + l.file + "](" + url + ")]"
	}
	return truncated + "the rest follows in " + plural(c, "comment") + "]"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

// posted is the post and the continuations that follow it (section 7.5):
// a text that fits whole with its tail; else the text cut at its last line
// break that leaves room for the note and the tail, the note on its own
// line, then the tail. With the file attached at url the note links it;
// with none the rest of the text follows in continuations, each cut the
// same way within maxPost, so that the post's text and theirs together are
// the text.
func (l long) posted(url string) (body string, more []string) {
	if l.fits() {
		return l.text + l.tail, nil
	}
	for c := 1; ; {
		note := l.note(url, c)
		head := cutAt(l.text, max(maxPost-len(l.tail)-len(note)-1, 0))
		if head != "" && !strings.HasSuffix(head, "\n") {
			note = "\n" + note
		}
		body = head + note + l.tail
		if url != "" {
			return body, nil
		}
		var pieces []string
		for rest := l.text[len(head):]; rest != ""; rest = rest[len(pieces[len(pieces)-1]):] {
			pieces = append(pieces, cutAt(rest, maxPost))
		}
		// The note's count only grows the note, and so the pieces: the
		// count settles.
		if len(pieces) == c {
			for i, p := range pieces {
				more = append(more, fmt.Sprintf("%s\nContinued from %s (part %d of %d):\n\n%s", continuedMarker, l.what, i+1, c, p))
			}
			return body, more
		}
		c = len(pieces)
	}
}

// cutAt is s's longest beginning of at most limit bytes that ends in a
// line break, or, where none of its first limit bytes is one (a line
// longer than the limit), that ends before a character; s whole when it
// fits.
func cutAt(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if i := strings.LastIndex(s[:limit], "\n"); i >= 0 {
		return s[:i+1]
	}
	for i := limit; i > 0 && i > limit-utf8.UTFMax; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return s[:limit] // no character begins near the limit: bytes, not text
}

// attachWhole attaches l's text whole to the issue at as l.file, and is
// the URL the forge serves it at; "" and the error when the forge refuses
// the file (ErrNotAttached) or the upload fails (any other error, a
// forge's 5xx or a network error among them): either way the rest of the
// text follows in continuations (section 7.5), so that a forge whose
// uploads fail neither keeps a run uncollected nor loses a note.
func (s *Swarm) attachWhole(at forge.Ref, l long) (url string, refused error) {
	url, err := s.Forge.Attach(at, l.file, []byte(l.text))
	if err != nil {
		return "", err
	}
	return url, nil
}

// noteFile is the file a note of the pass's own on issue n, of kind (a
// gate note's `gate`), is attached as: `<n>-<kind>-<time>.md`, its time
// the pass's in UTC to the second, so that each note, each gating's among
// them, has its own (section 7.5).
func (s *Swarm) noteFile(n int, kind string) string {
	return fmt.Sprintf("%d-%s-%s.md", n, kind, s.O.Now().UTC().Format("20060102T150405Z"))
}

// runText is the run's text as the collection posts it, by role: a
// member's output, whole in runs/<run>, attached as <run>.md; the
// reconciler's note on the run, Daisy's own, as <run>-note.md.
func (c *collection) runText(role, text, tail string) long {
	run := outputRun(c.r.N, Mark{Role: c.r.Role, Run: c.r.K})
	if role == "reconciler" {
		return long{text: text, tail: tail, file: run + "-note.md", what: fmt.Sprintf("the note on %s run %d", c.r.Role, c.r.K)}
	}
	return long{text: text, tail: tail, file: run + ".md", run: run, what: fmt.Sprintf("%s run %d's text", c.r.Role, c.r.K)}
}

// prepare is l's post and its continuations (posted): a member's text
// but a review first written whole to the job's workspace (section 7.8,
// texts/); a text that does not fit is first attached whole to the
// issue, once, recorded `attached <file> <url>`, or, the forge refusing
// it or the upload failing, warned of and recorded `not attached <file>:
// <error>`, and continued; a retried collection reads the line and does not attach it
// again.
func (c *collection) prepare(l long) (string, []string, error) {
	if l.run != "" && !l.review {
		if err := c.s.writeText(c.r.N, l.run, l.text+l.tail); err != nil {
			return "", nil, err
		}
	}
	if l.fits() {
		body, _ := l.posted("")
		return body, nil, nil
	}
	url, known := "", false
	for _, line := range c.lines {
		if u, ok := strings.CutPrefix(line, "attached "+l.file+" "); ok {
			url, known = u, true
		}
		known = known || strings.HasPrefix(line, "not attached "+l.file+": ")
	}
	if !known {
		u, refused := c.s.attachWhole(c.at, l)
		line := "attached " + l.file + " " + u
		if refused != nil {
			line = "not attached " + l.file + ": " + oneLine.Replace(refused.Error())
			fmt.Fprintf(c.s.Err, "⚠ #%d %s run %d: %s not attached: %s; the rest follows in comments\n", c.r.N, c.r.Role, c.r.K, l.file, oneLine.Replace(refused.Error()))
		} else {
			fmt.Fprintf(c.s.Out, "%sattached %s\n", c.head, l.file)
		}
		if err := c.step(line); err != nil {
			return "", nil, err
		}
		url = u
	}
	body, more := l.posted(url)
	return body, more, nil
}

// continued posts each of the text's continuations on the thread at, in
// order, recorded `continued <file> <i> of <c>`: one the result records,
// or that existing (the thread's comments) holds by its body, is not
// posted again.
func (c *collection) continued(at forge.Ref, existing []forge.Comment, l long, more []string) error {
	for i, body := range more {
		line := fmt.Sprintf("continued %s %d of %d", l.file, i+1, len(more))
		if slices.Contains(c.lines, line) {
			continue
		}
		if !slices.ContainsFunc(existing, func(e forge.Comment) bool {
			return e.Body == body && strings.EqualFold(e.Author, c.s.SwarmLogin)
		}) {
			if err := c.s.Forge.Comment(at, body); err != nil {
				return err
			}
			fmt.Fprintf(c.s.Out, "%sposted continuation %d of %d\n", c.head, i+1, len(more))
		}
		if err := c.step(line); err != nil {
			return err
		}
	}
	return nil
}
