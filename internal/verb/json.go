package verb

import (
	"bytes"
	"encoding/json"
	"strings"
)

// JSONFlag is the flag a read-only command takes to print its report as
// one JSON document (docs/cli.md, Common behaviour, --json).
const JSONFlag = "--json"

// Doc is a --json report, one embedding Warned; its warnings are the
// lines the run would have written on stderr.
type Doc interface{ warn(lines []string) }

// Warned is embedded in every --json report.
type Warned struct {
	Warnings []string `json:"warnings"`
}

func (w *Warned) warn(lines []string) { w.Warnings = lines }

// PrintReport prints the report read reads: with --json as one JSON
// document (WriteJSON), else as text, printed once every read is made.
// Both forms come from the one value, so they cannot drift.
func PrintReport[R Doc](c *Context, read func() (R, error), text func(R)) error {
	if c.Flag(JSONFlag) {
		return WriteJSON(c, func() (Doc, error) { return read() })
	}
	r, err := read()
	if err != nil {
		return err
	}
	text(r)
	return nil
}

// WriteJSON runs report with c.Stderr held. On success the document it
// returns, the held lines as its warnings, is written to stdout and
// nothing to stderr. On a failure stdout stays empty and the held lines
// go to stderr, ahead of the error the caller prints, as without --json.
func WriteJSON(c *Context, report func() (Doc, error)) error {
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
