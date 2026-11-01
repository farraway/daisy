package commands

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/journal"
)

// Log adds `- HHMM <message>` to today's Log section and commits the home.
// The message is the arguments joined with single spaces, so no quoting is
// needed on the command line.
func Log(c *Context) error {
	if err := preflight(c); err != nil {
		return err
	}
	message := strings.Join(c.Args, " ")
	now := c.Ops.Now().Format("1504")
	if err := appendLog(c, now, message); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "✅ Logged: %s %s\n", now, message)
	hash, committed, err := commitHome(c, "Log: "+now+" - "+summary(message))
	if err != nil {
		return err
	}
	reportCommit(c, "Log: "+now+" - "+summary(message), hash, committed)
	return nil
}

// appendLog writes the entry into today.md; the errors are the bash's.
func appendLog(c *Context, time, message string) error {
	fs := c.Ops.FS()
	b, err := fs.ReadFile(c.todayFile())
	if err != nil {
		return errors.New("Error: today.md not found")
	}
	out, err := journal.Log(string(b), time, message)
	if err != nil {
		return err
	}
	return fs.WriteFile(c.todayFile(), []byte(out), 0o644)
}

// summary is the first 60 characters of a message, for a commit subject.
func summary(message string) string {
	if r := []rune(message); len(r) > 60 {
		return string(r[:60])
	}
	return message
}

// reportCommit prints what `daisy commit` prints.
func reportCommit(c *Context, message, hash string, committed bool) {
	if committed {
		fmt.Fprintf(c.Stdout, "📝 Committed: %s (%s)\n", message, hash)
	} else {
		fmt.Fprintf(c.Stderr, "No changes to commit in home '%s'\n", c.Env.HomeName)
	}
}
