// Package prompt assembles a home's AGENTS.md: include.txt names the
// prompts, each is looked up in the home's prompts/ and then the root's,
// and goes in whole or, lazy, as its `## Trigger` section only, with
// $DAISY_ROOT substituted. The output is the bash build script's, byte
// for byte, but for the header lines that name the build and its rebuild
// command (and the Built: time).
package prompt

import (
	"fmt"
	"strings"
	"unicode"
)

// Entry is one include.txt line: a prompt name, lazy when it was written
// with a leading `~`.
type Entry struct {
	Name string
	Lazy bool
}

// ParseInclude reads include.txt. Empty lines and lines whose first
// non-space (shellSpace) is `#` are skipped; every other line is taken as `echo "$line"
// | xargs` gave it to the bash: words split at spaces and tabs, quotes and
// backslashes honoured, the words joined by one space. A line the result
// is empty for is skipped too. An unmatched quote (where xargs failed and
// the build stopped) is an error naming the line.
func ParseInclude(data []byte) ([]Entry, error) {
	var entries []Entry
	for i, line := range lines(data) {
		if line == "" || strings.HasPrefix(strings.TrimLeftFunc(line, shellSpace), "#") {
			continue
		}
		name, err := xargsWords(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", i+1, err)
		}
		if name == "" {
			continue
		}
		e := Entry{Name: name}
		if rest, ok := strings.CutPrefix(name, "~"); ok {
			e = Entry{Name: rest, Lazy: true}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// shellSpace is `[[:space:]]` in bash under a UTF-8 glibc locale: Unicode
// white space less U+0085 and the no-break spaces.
func shellSpace(r rune) bool {
	return unicode.IsSpace(r) && r != 0x85 && r != 0xA0 && r != 0x2007 && r != 0x202F
}

// xargsWords is GNU xargs' reading of one line: words end at a space, a
// tab or the newline, and any white space before a word is skipped (\v,
// \f and \r too, which inside a word are kept); inside '…' or "…" every byte is literal up to the
// matching quote; outside, a backslash takes the next byte literally (the
// newline included). The words are joined with one space, and trailing
// newlines dropped, as the command substitution did.
func xargsWords(line string) (string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	in := line + "\n"
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case !inWord && strings.IndexByte(" \t\n\v\f\r", c) >= 0:
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				words, inWord = append(words, word.String()), false
				word.Reset()
			}
		case c == '\'' || c == '"':
			end := strings.IndexAny(in[i+1:], string(c)+"\n")
			if end < 0 || in[i+1+end] == '\n' {
				kind := "single"
				if c == '"' {
					kind = "double"
				}
				return "", fmt.Errorf("unmatched %s quote", kind)
			}
			word.WriteString(in[i+1 : i+1+end])
			i += 1 + end
			inWord = true
		case c == '\\' && i+1 < len(in):
			i++
			word.WriteByte(in[i])
			inWord = true
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return strings.TrimRight(strings.Join(words, " "), "\n"), nil
}

// lines splits data as awk and `read` do: at each newline, a last line
// without one still a line, no line after a final newline.
func lines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}
