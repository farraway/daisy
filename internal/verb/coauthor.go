package verb

import (
	"fmt"
	"regexp"
)

// CoAuthorFlag is the value flag every command that commits takes.
const CoAuthorFlag = "--co-author"

// CoAuthor is the Co-Authored-By trailer every commit Daisy makes ends
// with: the command's --co-author, which an agent running Daisy gives
// its model's line, `Name <email>` (the cli refuses another shape), or
// else DAISY_CO_AUTHOR; with neither, none (a person at the keyboard).
// An environment value of another shape is warned about and left out,
// and the commit goes on.
func CoAuthor(c *Context) string {
	if v, ok := c.Value(CoAuthorFlag); ok {
		return v
	}
	v := c.Ops.Getenv("DAISY_CO_AUTHOR")
	if v == "" || IsCoAuthor(v) {
		return v
	}
	fmt.Fprintf(c.Stderr, "⚠ DAISY_CO_AUTHOR %q is not one line 'Name <email>': committing without Co-Authored-By\n", v)
	return ""
}

// IsCoAuthor is whether v is one line `Name <email>`.
func IsCoAuthor(v string) bool { return coAuthorLine.MatchString(v) }

var coAuthorLine = regexp.MustCompile(`^[^<>\s][^<>\r\n]* <[^<>\s]+@[^<>\s]+>$`)
