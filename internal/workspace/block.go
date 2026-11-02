package workspace

import "strings"

// A Daisy block in a line-based ignore file (.gitignore, .cursorignore) is
// a header line and the entry lines that follow it, after one blank line.
// init adds the entries the file lacks anywhere, into the block; clean takes
// the block out and nothing else, so a user's own line that happens to be
// an entry, or a blank line of theirs, stays.

// addBlock returns content with every entry present, and whether it changed.
func addBlock(content []byte, header string, entries []string) ([]byte, bool) {
	lines := splitLines(string(content))
	var missing []string
	for _, e := range entries {
		if !hasLine(lines, e) && !hasLine(missing, e) {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return content, false
	}
	if h := indexOf(lines, header); h >= 0 {
		end := blockEnd(lines, h, func(l string) bool { return hasLine(entries, l) })
		out := append(append(append([]string{}, lines[:end]...), missing...), lines[end:]...)
		return joinLines(out), true
	}
	return joinLines(append(append(lines, "", header), missing...)), true
}

// removeBlock takes out the header, the entry lines contiguous with it and
// the blank line before it; nothing when there is no header.
func removeBlock(content []byte, header string, isEntry func(string) bool) ([]byte, bool) {
	lines := splitLines(string(content))
	h := indexOf(lines, header)
	if h < 0 {
		return content, false
	}
	start, end := h, blockEnd(lines, h, isEntry)
	if start > 0 && lines[start-1] == "" {
		start--
	}
	return joinLines(append(append([]string{}, lines[:start]...), lines[end:]...)), true
}

// blockEnd is the index after the last entry line following the header.
func blockEnd(lines []string, h int, isEntry func(string) bool) int {
	end := h + 1
	for end < len(lines) && isEntry(lines[end]) {
		end++
	}
	return end
}

// splitLines splits a file into lines; a last line without a newline is
// still a line (it gets one when the file is written back).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func joinLines(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func indexOf(lines []string, s string) int {
	for i, l := range lines {
		if l == s {
			return i
		}
	}
	return -1
}

func hasLine(lines []string, s string) bool { return indexOf(lines, s) >= 0 }
