package commands

import (
	"bytes"
	"encoding/json"
	"strings"
)

// JSONFlag is the flag a read-only command takes to print its report as
// one JSON document (docs/cli.md, Common behaviour, --json).
const JSONFlag = "--json"

// jsonDoc is a --json report; its warnings are the lines the run would
// have written on stderr.
type jsonDoc interface{ warn(lines []string) }

// warned is embedded in every --json report.
type warned struct {
	Warnings []string `json:"warnings"`
}

func (w *warned) warn(lines []string) { w.Warnings = lines }

// printReport prints the report read reads: with --json as one JSON
// document (writeJSON), else as text, printed once every read is made.
// Both forms come from the one value, so they cannot drift.
func printReport[R jsonDoc](c *Context, read func() (R, error), text func(R)) error {
	if c.Flag(JSONFlag) {
		return writeJSON(c, func() (jsonDoc, error) { return read() })
	}
	r, err := read()
	if err != nil {
		return err
	}
	text(r)
	return nil
}

// writeJSON runs report with c.Stderr held. On success the document it
// returns, the held lines as its warnings, is written to stdout and
// nothing to stderr. On a failure stdout stays empty and the held lines
// go to stderr, ahead of the error the caller prints, as without --json.
func writeJSON(c *Context, report func() (jsonDoc, error)) error {
	stderr := c.Stderr
	var held bytes.Buffer
	c.Stderr = &held
	doc, err := report()
	c.Stderr = stderr
	if err != nil {
		stderr.Write(held.Bytes())
		return err
	}
	lines := []string{}
	for l := range strings.Lines(held.String()) {
		lines = append(lines, strings.TrimSuffix(l, "\n"))
	}
	doc.warn(lines)
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err = c.Stdout.Write(out.Bytes())
	return err
}
