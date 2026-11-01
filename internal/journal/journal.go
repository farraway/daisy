// Package journal edits today.md: the log entry and the done tick. Both
// change exactly the lines they mean to and leave every other byte alone.
package journal

import (
	"errors"
	"regexp"
	"strings"
)

// ErrNoLogSection: today.md has no `#### Log` heading.
var ErrNoLogSection = errors.New("Error: Log section not found in today.md")

var heading = regexp.MustCompile(`^#{1,6} `)

// Log appends `- <time> <message>` to the Log section: the lines from the
// first `#### Log` heading to the next heading of any level or the end of
// the file. The entry goes after the section's last non-blank line, so
// blank lines before the next heading stay where they are. A section with
// no entries is written in the template's shape — heading, blank line,
// entry, blank line before what follows — which is what the health check
// requires of it.
func Log(content, time, message string) (string, error) {
	lines := split(content)
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "#### Log") {
			start = i
			break
		}
	}
	if start < 0 {
		return "", ErrNoLogSection
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if heading.MatchString(lines[i]) {
			end = i
			break
		}
	}
	entry := "- " + time + " " + message
	for i := end - 1; i > start; i-- {
		if lines[i] != "" {
			lines = append(lines[:i+1], append([]string{entry}, lines[i+1:]...)...)
			return join(lines), nil
		}
	}
	section := []string{"", entry}
	if end < len(lines) {
		section = append(section, "")
	}
	lines = append(lines[:start+1], append(section, lines[end:]...)...)
	return join(lines), nil
}

// Tick flips the first line that is exactly `- [ ] <text>` to `- [x] <text>`.
// ok is false, and the content unchanged, when there is none.
func Tick(content, text string) (out string, ok bool) {
	target := "- [ ] " + text
	lines := split(content)
	for i, l := range lines {
		if l == target {
			lines[i] = "- [x] " + text
			return join(lines), true
		}
	}
	return content, false
}

// split and join keep the file's lines; the result always ends in a
// newline, as a file should.
func split(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func join(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}
