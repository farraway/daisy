package prompt

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// The `## Rules` section of a prompt or SKILL.md is the span `daisy
// optimize` briefs and `optimize apply --rules` replaces, and the one `daisy
// help <skill>` prints. One boundary serves all three: the section runs
// from its `## Rules` heading to the next level-1 or level-2 heading, a
// `###` being a subsection inside it, and a line in a code fence being no
// heading at all.
//
// Daisy reads ATX headings (`#`, `##`) and fenced code, the markdown its
// prompts are written in, with LF or CRLF line ends. Where the boundary
// is not certain ReplaceRules refuses rather than guess (RulesProblem): a
// fence opened in the section and never closed (the section would run to
// the end of the file), a missing section's rules appended into a fence
// left open, and any line of only `=` or `-` straight after a non-blank line
// that is not a heading, outside a fence, which another reader may take
// for a setext heading's underline (a blank line above makes it a
// thematic break, unambiguously).

// RulesSection is where doc's `## Rules` section lies, as byte offsets:
// from the start of its heading line to the start of the next level-1 or
// level-2 heading, or the end of doc.
func RulesSection(doc []byte) (start, end int, ok bool) {
	found := false
	for _, l := range scan(doc) {
		switch {
		case !found && l.rulesHeading():
			start, found = l.off, true
		case found && l.level >= 1 && l.level <= 2:
			return start, l.off, true
		}
	}
	return start, len(doc), found
}

// Section is where doc's first level-2 section titled exactly title lies,
// as byte offsets, by the same boundary as RulesSection: from its heading
// line to the next level-1 or level-2 heading outside a fence, or the end
// of doc.
func Section(doc []byte, title string) (start, end int, ok bool) {
	found := false
	for _, l := range scan(doc) {
		switch {
		case !found && l.level == 2 && l.title() == title:
			start, found = l.off, true
		case found && l.level >= 1 && l.level <= 2:
			return start, l.off, true
		}
	}
	return start, len(doc), found
}

// Rules is the section's lines, heading included; nil when there is none.
func Rules(doc []byte) []string {
	start, end, ok := RulesSection(doc)
	if !ok {
		return nil
	}
	return lines(doc[start:end])
}

// listItem is a rule: a list item at the left margin, numbered or bulleted.
var listItem = regexp.MustCompile(`^([0-9]+\.|[-*]) `)

// CountRules is the number of rules in doc's `## Rules` section: its list
// items at the left margin, outside fences.
func CountRules(doc []byte) int {
	start, end, ok := RulesSection(doc)
	if !ok {
		return 0
	}
	n := 0
	for _, l := range scan(doc[start:end]) {
		if !l.fenced && listItem.MatchString(l.text) {
			n++
		}
	}
	return n
}

// ReplaceRules is doc with its `## Rules` section replaced by proposal,
// which must be one such section: its first non-blank line the heading,
// and no other level-1 or level-2 heading after it (that would end the
// section, and the next run would read back less than was written). The
// blank lines between the section and the heading after it are doc's,
// whatever the proposal ends with, so replacing the rules with themselves
// changes nothing. Without a section, the proposal goes in before the
// first `## ` heading other than the Trigger, one blank line apart, or at
// the end.
func ReplaceRules(doc, proposal []byte) ([]byte, error) {
	body, err := rulesProposal(proposal)
	if err != nil {
		return nil, err
	}
	if err := RulesProblem(doc); err != nil {
		return nil, err
	}
	var out []byte
	if start, end, ok := RulesSection(doc); ok {
		// The gap is everything after the section's last line of text,
		// that line's newline included.
		gap := doc[start+len(trimBlankLines(doc[start:end])) : end]
		out = append(append(append(append([]byte{}, doc[:start]...), body...), gap...), doc[end:]...)
	} else {
		out = insertRules(doc, body)
	}
	// The invariant the checks above serve: the section the next run reads
	// back is the one written, and nothing after it is swallowed.
	// The proposal and the target are both checked by now, so a failure
	// here is the file's shape, not the proposal's.
	if s, e, ok := RulesSection(out); !ok || string(trimBlankLines(out[s:e])) != string(body) {
		return nil, TargetError("the rules written would not read back as its ## Rules section (the rest of the file would run into it)")
	}
	return out, nil
}

// RulesProblem is why `apply --rules` cannot write doc's `## Rules`
// section, as a TargetError; nil when it can. With a section: a fence
// opened in it and never closed, or a possible setext underline in it.
// Without one: a fence left open at the end of the file, when the rules
// would be appended into it (with a `## ` heading to go before, they land
// outside any fence). The briefing shows it, so the agent is not handed a
// target that apply would refuse.
func RulesProblem(doc []byte) error {
	start, end, ok := RulesSection(doc)
	if !ok {
		if _, before := insertAt(doc); !before {
			if n := openFence(doc); n > 0 {
				return TargetError(fmt.Sprintf("the code fence opened at line %d of the file never closes, and the rules would be appended inside it; close the fence first", n))
			}
		}
		return nil
	}
	section := doc[start:end]
	if n := openFence(section); n > 0 {
		return TargetError(fmt.Sprintf("the code fence opened at line %d of the file (in its ## Rules section) never closes, so the section runs to the end of the file and replacing it would delete what follows; close the fence first", lineOf(doc, start)+n-1))
	}
	if n := setextUnderline(section); n > 0 {
		return TargetError(fmt.Sprintf("line %d of the file (in its ## Rules section) may be a setext heading underline, so where the section ends is ambiguous; write that heading as `#`/`##`, or put a blank line above the line, first", lineOf(doc, start)+n-1))
	}
	return nil
}

// openFence is the 1-based line of b where a fence opens that never
// closes, 0 when every fence closes.
func openFence(b []byte) int {
	open := 0
	for _, l := range scan(b) {
		switch {
		case l.fence && open == 0:
			open = l.n
		case l.fence:
			open = 0
		}
	}
	return open
}

// TargetError is a refusal that is the target's doing, not the proposal's.
type TargetError string

func (e TargetError) Error() string { return string(e) }

// rulesProposal is the proposal checked, without its leading and trailing
// blank lines and its last newline.
func rulesProposal(proposal []byte) ([]byte, error) {
	ls := scan(proposal)
	first := 0
	for first < len(ls) && strings.TrimSpace(ls[first].text) == "" {
		first++
	}
	if first == len(ls) || !ls[first].rulesHeading() {
		return nil, fmt.Errorf("must begin with a '## Rules' heading (the section it replaces); use --rewrite to replace the whole file")
	}
	for _, l := range ls[first+1:] {
		if l.level >= 1 && l.level <= 2 {
			return nil, fmt.Errorf("holds a heading outside the rules at line %d: '%s'; a --rules proposal is one ## Rules section (use --rewrite to change the rest of the file)", l.n, l.text)
		}
	}
	if open := openFence(proposal); open != 0 {
		return nil, fmt.Errorf("leaves the code fence opened at line %d unclosed, which would swallow the rest of the file into the rules", open)
	}
	if n := setextUnderline(proposal[ls[first].off:]); n > 0 {
		return nil, fmt.Errorf("holds a setext heading underline at line %d; a --rules proposal is one ## Rules section, its headings `###` or deeper", ls[first].n+n-1)
	}
	return trimBlankLines(proposal[ls[first].off:]), nil
}

// insertAt is where a missing section goes: the offset of the first `## `
// heading other than the Trigger (before is true), else the end of doc.
func insertAt(doc []byte) (off int, before bool) {
	for _, l := range scan(doc) {
		if l.level == 2 && l.title() != "Trigger" {
			return l.off, true
		}
	}
	return len(doc), false
}

func insertRules(doc, body []byte) []byte {
	if off, before := insertAt(doc); before {
		out := append([]byte{}, doc[:off]...)
		out = append(out, body...)
		out = append(out, "\n\n"...)
		return append(out, doc[off:]...)
	}
	out := append([]byte{}, doc...)
	if len(out) > 0 {
		if !bytes.HasSuffix(out, []byte("\n")) {
			out = append(out, '\n')
		}
		out = append(out, '\n')
	}
	return append(append(out, body...), '\n')
}

// trimBlankLines is b up to the end of its last line that is not white
// space, without that line's newline.
func trimBlankLines(b []byte) []byte {
	end := 0
	for _, l := range scan(b) {
		if strings.TrimSpace(l.text) != "" {
			end = l.off + len(l.text)
		}
	}
	return b[:end]
}

// line is one line of a markdown document: its 1-based number, its offset,
// its text without the newline, its heading level (0 for none), whether it
// lies in a code fence and whether it opens or closes one.
type line struct {
	n, off, level int
	text          string
	fenced, fence bool
}

// rulesHeading: a level-2 heading titled `Rules`, or `Rules` and more
// after a space (`## Rules of the loop`), as the bash's `/^## Rules/` read
// it, bounded at the word.
func (l line) rulesHeading() bool {
	t := l.title()
	return l.level == 2 && (t == "Rules" || strings.HasPrefix(t, "Rules ") || strings.HasPrefix(t, "Rules\t"))
}

// title is a heading's text: the `#`s, an optional closing run of `#`s and
// the white space around them dropped.
func (l line) title() string {
	t := strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(l.text, " "), "#"))
	if s := strings.TrimRight(t, "#"); s == "" || strings.HasSuffix(s, " ") || strings.HasSuffix(s, "\t") {
		t = strings.TrimSpace(s)
	}
	return t
}

// scan splits doc into lines and marks ATX headings and fences, the two
// block kinds Daisy reads (CommonMark's rules for each): a heading is up to
// three spaces, one to six `#`, then a space, a tab or the end of the line,
// outside a fence; a fence opens at up to three spaces and three or more
// backticks or tildes, and closes at a run of the same character at least
// as long.
func scan(doc []byte) []line {
	var out []line
	var fence string // the run that opened the fence we are in
	for off, n := 0, 1; off < len(doc); n++ {
		nl := bytes.IndexByte(doc[off:], '\n')
		text := string(doc[off:])
		next := len(doc)
		if nl >= 0 {
			text, next = string(doc[off:off+nl]), off+nl+1
		}
		text = strings.TrimSuffix(text, "\r") // a CRLF line ends at the CR
		l := line{n: n, off: off, text: text, fenced: fence != ""}
		run := fenceRun(text)
		switch {
		case fence == "" && run != "":
			fence, l.fenced, l.fence = run, true, true
		case fence != "" && run != "" && run[0] == fence[0] && len(run) >= len(fence) && strings.Trim(strings.TrimLeft(strings.TrimLeft(text, " "), run[:1]), " \t") == "":
			fence, l.fence = "", true
		case fence == "":
			l.level = headingLevel(text)
		}
		out = append(out, l)
		off = next
	}
	return out
}

var underline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// setextUnderline is the 1-based line in section of the first line of
// only `=` or `-` straight after a non-blank line other than a heading,
// outside a fence; 0 when there is none.
func setextUnderline(section []byte) int {
	ls := scan(section)
	for i := 1; i < len(ls); i++ {
		prev, l := ls[i-1], ls[i]
		if l.fenced || prev.fenced || prev.level > 0 || strings.Trim(prev.text, " \t") == "" {
			continue
		}
		if underline.MatchString(l.text) {
			return l.n
		}
	}
	return 0
}

// lineOf is the 1-based line of doc at byte offset off.
func lineOf(doc []byte, off int) int { return bytes.Count(doc[:off], []byte("\n")) + 1 }

// fenceRun is the run of backticks or tildes a fence line starts with
// (after up to three spaces), "" when the line is no fence.
func fenceRun(s string) string {
	t := strings.TrimLeft(s, " ")
	if len(s)-len(t) > 3 || t == "" || (t[0] != '`' && t[0] != '~') {
		return ""
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	// A backtick fence's info string holds no backtick: "```x``` …" is
	// inline code, not a fence.
	if n < 3 || (t[0] == '`' && strings.Contains(t[n:], "`")) {
		return ""
	}
	return t[:n]
}

func headingLevel(s string) int {
	t := strings.TrimLeft(s, " ")
	if len(s)-len(t) > 3 {
		return 0
	}
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(t) && t[n] != ' ' && t[n] != '\t') {
		return 0
	}
	return n
}
