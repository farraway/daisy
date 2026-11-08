package journal

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Retain is how many day-blocks journal.md keeps; Last14 how many the
// last-14 window holds.
const (
	Retain = 5
	Last14 = 14
)

var dayHeading = regexp.MustCompile(`^### [0-9]{4}-[0-9]{2}-[0-9]{2}`)

const (
	last14Glob = "journal-*-last14.md"
	mtdGlob    = "journal-*-mtd.md"
	ytdGlob    = "journal-*-ytd.md"
)

// IsRollingWindow: name is one of the three files Rotate reads and
// rewrites — the last-14, month-to-date and year-to-date windows.
func IsRollingWindow(name string) bool {
	for _, g := range []string{last14Glob, mtdGlob, ytdGlob} {
		if ok, _ := path.Match(g, name); ok {
			return true
		}
	}
	return false
}

// Rotation is what rotating journal.md changes in the journal directory:
// journal.md's new content, the window files to write (by name) and the
// window files renamed away. Moved is how many day-blocks left journal.md;
// when it is 0 nothing changes.
type Rotation struct {
	Journal string
	Moved   int
	Write   map[string]string
	Remove  []string
}

// Rotate keeps journal.md's last Retain day-blocks and files every older
// one, oldest first, into all three rolling windows: the last-14 window
// (which then drops its oldest block beyond Last14), the month-to-date and
// the year-to-date window. A month or year window whose first block is of
// another month or year than the incoming block closes first, into
// journal-YYYYMM.md or journal-YYYY.md (replacing any file of that name),
// and a new one opens. Each rolling window is named by the date of its
// first block and carries a table of contents and a summary. So every
// rotated block is in exactly one month file and one year file, and in the
// last-14 window while it is among the fourteen most recently rotated.
//
// A day-block runs from a `### YYYY-MM-DD` heading to the next one, less
// its trailing blank lines and one trailing `---`; anything between a `---`
// and the next heading (new-week's Weekly Retrospective) stays with the
// block before it. windows holds the rolling files that exist, by name.
// This is the bash rotation, byte for byte; nothing here reads the clock.
func Rotate(journal string, windows map[string]string) (Rotation, error) {
	pre, blocks, err := splitBlocks(journal)
	if err != nil {
		return Rotation{}, fmt.Errorf("Error: rotate: journal.md has %v — aborting, nothing written", err)
	}
	if len(blocks) <= Retain {
		return Rotation{}, nil
	}
	moved := len(blocks) - Retain
	dir := maps.Clone(windows)
	if dir == nil {
		dir = map[string]string{}
	}
	for _, b := range blocks[:moved] {
		date := b[4:14]
		fileLast14(dir, b, date)
		if err := fileCalendar(dir, mtdGlob, b, date[:7], func(month string) string {
			return "journal-" + strings.Replace(month, "-", "", 1) + ".md"
		}); err != nil {
			return Rotation{}, err
		}
		if err := fileCalendar(dir, ytdGlob, b, date[:4], func(year string) string {
			return "journal-" + year + ".md"
		}); err != nil {
			return Rotation{}, err
		}
	}

	r := Rotation{Journal: pre + strings.Join(blocks[moved:], "\n---\n\n"), Moved: moved, Write: map[string]string{}}
	for name, content := range dir {
		if old, ok := windows[name]; !ok || old != content {
			r.Write[name] = content
		}
	}
	for name := range windows {
		if _, ok := dir[name]; !ok {
			r.Remove = append(r.Remove, name)
		}
	}
	slices.Sort(r.Remove)
	return r, nil
}

// fileLast14 appends the block to the last-14 window, opening one named by
// the block's date when there is none; past Last14 blocks it drops the
// oldest and takes the name of its new first block.
func fileLast14(dir map[string]string, block, date string) {
	const label = "Journal Archive: Last 14 Entries"
	name := first(dir, last14Glob)
	if name == "" {
		name = windowName(date, "last14")
	}
	content := frontMatter(appendBlock(dir[name], block), label)
	if len(dates(content)) > Last14 {
		content = frontMatter(dropOldest(content), label)
		if renamed := windowName(firstDate(content), "last14"); renamed != name {
			delete(dir, name)
			name = renamed
		}
	}
	dir[name] = content
}

// fileCalendar appends the block to the month (or year) window when the
// window's first block is of the same period; otherwise it closes the
// window into closed(period) and opens a new one with the block. period is
// the block's YYYY-MM (or YYYY).
func fileCalendar(dir map[string]string, glob, block, period string, closed func(string) string) error {
	name := first(dir, glob)
	current := ""
	if name != "" {
		d := firstDate(dir[name])
		if d == "" {
			return fmt.Errorf("Error: rotate: %s has no '### YYYY-MM-DD' day-block, so its month or year is unknown — fix or remove it, then re-run. Nothing written.", name)
		}
		current = d[:len(period)]
	}
	if name != "" && current == period {
		dir[name] = frontMatter(appendBlock(dir[name], block), "Journal Archive: "+current)
		return nil
	}
	if name != "" {
		content := frontMatter(dir[name], "Journal Archive: "+current)
		delete(dir, name)
		dir[closed(current)] = content
	}
	kind := strings.TrimSuffix(strings.TrimPrefix(glob, "journal-*-"), ".md")
	dir[windowName(block[4:14], kind)] = frontMatter(block, "Journal Archive: "+period)
	return nil
}

// windowName is journal-YYYYMMDD-<kind>.md for a block dated YYYY-MM-DD.
func windowName(date, kind string) string {
	return "journal-" + strings.ReplaceAll(date, "-", "") + "-" + kind + ".md"
}

// first is the first name matching glob, in `ls` order, or "".
func first(dir map[string]string, glob string) string {
	for _, name := range slices.Sorted(maps.Keys(dir)) {
		if ok, _ := path.Match(glob, name); ok {
			return name
		}
	}
	return ""
}

// splitBlocks cuts content into day-blocks, each its lines newline-
// terminated, trailing blank lines and one trailing `---` trimmed. A line
// with anything but white space before the first heading is refused: it
// has no block to belong to.
func splitBlocks(content string) (pre string, _ []string, _ error) {
	var blocks [][]string
	var head []string
	for _, l := range lines(content) {
		switch {
		case dayHeading.MatchString(l):
			blocks = append(blocks, []string{l})
		case len(blocks) > 0:
			blocks[len(blocks)-1] = append(blocks[len(blocks)-1], l)
		default:
			head = append(head, l)
		}
	}
	// What comes before the first heading is refused, but for a preamble
	// that ends in `---` (new-week's Weekly Retrospective on a home whose
	// journal was empty): with no block before it to stay with, it stays
	// at the top of journal.md, never rotated.
	if slices.ContainsFunc(head, func(l string) bool { return !blank(l) }) {
		b := trimBlank(head)
		if b[len(b)-1] != "---" || len(blocks) == 0 {
			return "", nil, fmt.Errorf("non-blank content before its first '### YYYY-MM-DD' heading")
		}
		pre = strings.Join(b, "\n") + "\n\n"
	}
	out := make([]string, len(blocks))
	for i, b := range blocks {
		b = trimBlank(b)
		if b[len(b)-1] == "---" {
			b = trimBlank(b[:len(b)-1])
		}
		out[i] = strings.Join(b, "\n") + "\n"
	}
	return pre, out, nil
}

// trimBlank drops trailing blank lines; the heading is never blank.
func trimBlank(b []string) []string {
	for len(b) > 1 && blank(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func blank(l string) bool { return strings.Trim(l, " \t\r\v\f") == "" }

// lines are a file's lines as awk reads them: a missing final newline
// still ends the last line, and there is no empty line after the last.
func lines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

// appendBlock adds a block to a window, after a `---` separator when the
// window is not empty.
func appendBlock(window, block string) string {
	if window == "" {
		return block
	}
	return window + "\n---\n\n" + block
}

// blocksPart is a window from its first heading on: its blocks, without
// the front matter.
func blocksPart(window string) string {
	ls := lines(window)
	for i, l := range ls {
		if dayHeading.MatchString(l) {
			return strings.Join(ls[i:], "\n") + "\n"
		}
	}
	return ""
}

// frontMatter rebuilds a window's header — title, table of contents (one
// date per block), summary — over its blocks; any header it had is
// replaced, so it is safe to apply again.
func frontMatter(window, label string) string {
	blocks := blocksPart(window)
	dates := dates(blocks)
	var b strings.Builder
	b.WriteString("# " + label + "\n\n## Table of Contents\n")
	for _, d := range dates {
		b.WriteString("- " + d + "\n")
	}
	fmt.Fprintf(&b, "\n## Summary\n\n%d entries, %s to %s.\n\n---\n\n", len(dates), dates[0], dates[len(dates)-1])
	b.WriteString(blocks)
	return b.String()
}

// dates are the window's day-block dates, in order.
func dates(window string) []string {
	var ds []string
	for _, l := range lines(window) {
		if dayHeading.MatchString(l) {
			ds = append(ds, l[4:14])
		}
	}
	return ds
}

// dropOldest removes a window's first block, re-joining the rest; the
// front matter goes with it, for frontMatter to rebuild.
func dropOldest(window string) string {
	_, blocks, _ := splitBlocks(blocksPart(window)) // starts at a heading: nothing to refuse
	return strings.Join(blocks[1:], "\n---\n\n")
}

// firstDate is the date of a window's first heading, "" when it has none.
func firstDate(window string) string {
	for _, l := range lines(window) {
		if dayHeading.MatchString(l) {
			return l[4:14]
		}
	}
	return ""
}
