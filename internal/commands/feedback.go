package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/feedback"
	"daisy/internal/ops"
)

// Paths the feedback commands share.
func (c *Context) feedbackFile() string { return filepath.Join(c.Env.Home, "feedback", "feedback.md") }
func (c *Context) archiveFile() string  { return filepath.Join(c.Env.Home, "feedback", "archive.md") }
func (c *Context) evalsDir() string     { return filepath.Join(c.Env.Root, "prompts", "evals") }

// Feedback appends an entry to the home's feedback.md: the message from
// the arguments, else from piped stdin, tagged with --workflow when given.
// A tag naming no prompt or skill is refused before anything is written,
// with every tag that does. Nothing is committed: the next home commit
// sweeps the entry up.
func Feedback(c *Context) error {
	fsys := c.Ops.FS()
	workflow, _ := c.Value("--workflow", "-w")
	if workflow != "" {
		if _, ok := feedback.Resolve(fsys, c.Env.Root, c.Env.Home, workflow); !ok {
			return unknownWorkflow(c, workflow)
		}
	}
	var message string
	given := len(c.Args) > 0 || c.EmptyWords
	if given {
		message = strings.Join(c.Args, " ")
	} else if !ops.IsTerminal(c.Stdin) {
		b, err := io.ReadAll(c.Stdin)
		if err != nil {
			return fmt.Errorf("Error: cannot read stdin: %v", err)
		}
		message = strings.TrimRight(string(b), "\n")
	}
	// No word and nothing piped (a terminal, or an empty pipe) is no
	// message given: the usage and the tags. An empty word is a message.
	if !given && message == "" {
		return errors.New("Usage: daisy feedback [--workflow <name>] \"<description>\"\n" +
			"  or:  echo \"<description>\" | daisy feedback [--workflow <name>]\n\n" +
			"Workflows:\n" + indent(c.workflowLabels(), "  "))
	}
	if message == "" {
		return errors.New("Error: feedback message cannot be empty")
	}
	file := c.feedbackFile()
	if fi, err := fsys.Stat(file); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("Error: feedback file not found at %s\n  Run: mkdir -p \"%s\" && touch \"%s\"", file, filepath.Join(c.Env.Home, "feedback"), file)
	}
	doc, err := fsys.ReadFile(file)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", file, err)
	}
	now := c.Ops.Now()
	if err := ops.Rewrite(fsys, file, feedback.Record(doc, now.Format("2006-01-02 1504"), workflow, message), 0o644); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", file, err)
	}
	tag := ""
	if workflow != "" {
		tag = "workflow: " + workflow + ", "
	}
	fmt.Fprintf(c.Stdout, "  ✓ Feedback recorded (%s%s)\n  File: %s\n", tag, now.Format("2006-01-02 15:04"), file)
	return nil
}

// unknownWorkflow is the refusal of a tag naming nothing, with the tags
// that name something.
func unknownWorkflow(c *Context, workflow string) error {
	return fmt.Errorf("Error: workflow '%s' names no prompt or skill\n"+
		"  Qualify the target, e.g. --workflow prompts/daisy or --workflow skills/git.\n\n"+
		"  Available:\n%s", workflow, indent(c.workflowLabels(), "    "))
}

func (c *Context) workflowLabels() []string {
	return feedback.Labels(c.Ops.FS(), c.Env.Root, c.Env.Home)
}

// indent is the lines, each after prefix, without a final newline.
func indent(lines []string, prefix string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(prefix + l)
	}
	return b.String()
}

// Eval lists the eval cases in $DAISY_ROOT/prompts/evals, prints one, or
// with --record appends a result to the home's feedback/eval/results.md
// (asking for notes on a terminal when none are given). Nothing is
// committed.
func Eval(c *Context) error {
	fsys := c.Ops.FS()
	dir := c.evalsDir()
	if fi, err := fsys.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("Error: No evals directory found at %s", dir)
	}
	if c.Flag("--record") {
		return evalRecord(c)
	}
	switch len(c.Args) {
	case 0:
		return evalList(c)
	case 1:
		name := c.Args[0]
		b, err := evalCase(c, name)
		if err != nil {
			return err
		}
		c.Stdout.Write(b)
		fmt.Fprintf(c.Stdout, "\n  Record result: daisy eval --record %s pass|fail \"notes\"\n", name)
		return nil
	}
	return UsageError("unexpected argument: " + c.Args[1])
}

// evalCase reads the named case; a name holding a slash names none.
func evalCase(c *Context, name string) ([]byte, error) {
	path := filepath.Join(c.evalsDir(), name+".md")
	if fi, err := c.Ops.FS().Stat(path); strings.ContainsRune(name, '/') || err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("Error: Eval case not found: %s\n  Run 'daisy eval' to list available cases.", name)
	}
	b, err := c.Ops.FS().ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	return b, nil
}

func evalList(c *Context) error {
	fsys := c.Ops.FS()
	entries, err := fsys.ReadDir(c.evalsDir())
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", c.evalsDir(), err)
	}
	var lines []string
	for _, e := range entries { // ReadDir is in byte order
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok {
			continue
		}
		path := filepath.Join(c.evalsDir(), e.Name())
		if fi, err := fsys.Stat(path); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		b, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Error: cannot read %s: %v", path, err)
		}
		lines = append(lines, fmt.Sprintf("  %s  %s", pad(name, 20), title(b)))
	}
	if len(lines) == 0 {
		fmt.Fprintf(c.Stdout, "No eval cases found in %s\n", c.evalsDir())
		return nil
	}
	fmt.Fprintf(c.Stdout, "Available eval cases:\n\n%s\n\n", strings.Join(lines, "\n"))
	fmt.Fprintln(c.Stdout, "  Usage: daisy eval <case>                           # display criteria")
	fmt.Fprintln(c.Stdout, "         daisy eval --record <case> pass|fail        # record a result")
	return nil
}

// title is the first `# ` heading's text, "" when there is none.
func title(doc []byte) string {
	for _, l := range strings.Split(string(doc), "\n") {
		if t, ok := strings.CutPrefix(l, "# "); ok {
			return t
		}
	}
	return ""
}

const resultsHeader = "# Eval Results\n\nRecords of manual eval runs. Used to track eval accuracy and improve eval criteria.\nRun `daisy eval <case>` to view criteria; `daisy eval --record <case> pass|fail` to add an entry.\n\n---\n"

func evalRecord(c *Context) error {
	if len(c.Args) == 0 {
		return errors.New("Usage: daisy eval --record <case> pass|fail [\"notes\"]")
	}
	name, result, notes := c.Args[0], "", ""
	if len(c.Args) > 1 {
		result = c.Args[1]
	}
	if len(c.Args) > 2 {
		notes = c.Args[2]
	}
	if _, err := evalCase(c, name); err != nil {
		return err
	}
	if result != "pass" && result != "fail" {
		return fmt.Errorf("Error: result must be 'pass' or 'fail' (got: %s)", result)
	}
	if notes == "" && ops.IsTerminal(c.Stdin) {
		var err error
		if notes, err = c.Ops.Prompt(c.Stdin, c.Stderr, "Notes (optional): "); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("Error: cannot read the notes: %v\nNothing was recorded.", err)
		} // EOF: no notes
	}
	fsys := c.Ops.FS()
	dir := filepath.Join(c.Env.Home, "feedback", "eval")
	file := filepath.Join(dir, "results.md")
	if err := fsys.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("Error: cannot create %s: %v", dir, err)
	}
	doc, err := fsys.ReadFile(file)
	if err != nil {
		if lexists(fsys, file) {
			return fmt.Errorf("Error: cannot read %s: %v", file, err)
		}
		doc = []byte(resultsHeader)
	}
	now := c.Ops.Now()
	entry := fmt.Sprintf("\n## %s\n**Case:** %s\n**Result:** %s\n", now.Format("2006-01-02 1504"), name, result)
	if notes != "" {
		entry += "\n" + notes + "\n"
	}
	if err := ops.Rewrite(fsys, file, append(doc, entry...), 0o644); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", file, err)
	}
	fmt.Fprintf(c.Stdout, "  ✓ Recorded %s for eval '%s' (%s)\n  File: %s\n", result, name, now.Format("2006-01-02 15:04"), file)
	return nil
}

// The --healthcheck hooks: feedback and eval need the workspace and its
// home, which the cli has resolved; the briefing the home's feedback/.
var (
	FeedbackHealth = readOnly(func(*Context) error { return nil })
	EvalHealth     = readOnly(func(*Context) error { return nil })
	OptimizeHealth = readOnly(func(c *Context) error {
		if fi, err := c.Ops.FS().Stat(filepath.Join(c.Env.Home, "feedback")); err != nil || !fi.IsDir() {
			return errors.New("Error: feedback directory not found (needed for feedback.md and archive.md)")
		}
		return nil
	})
)
