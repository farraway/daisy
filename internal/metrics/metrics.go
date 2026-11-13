// Package metrics is what a plan step or a swarm run costs and changes,
// one row each: the home's canonical CSV (`metrics/metrics.csv`, one
// header, the columns in a fixed order, rows only ever appended), the
// plan's `## Metrics` table that is a plan step's local view of it, the
// lines a commit range adds and removes per class (code, tests, prose)
// counted from `git diff --numstat`, and the summary `daisy metrics`
// prints. The writes are the caller's; Measure runs git through Ops.
package metrics

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/prompt"
)

// Kind is what a row measures.
type Kind string

const (
	PlanStep Kind = "plan-step"
	SwarmRun Kind = "swarm-run"
)

// Header is the CSV's columns, in their one order. A column is only ever
// added at the end, with every older row then one cell short.
var Header = []string{
	"when", "kind", "plan_or_swarm", "step_or_run", "role", "provider", "model", "effort",
	"root_sha", "home_sha", "prompt_hash", "outcome", "review_rounds",
	"tokens_in", "tokens_out", "tokens_total", "cost_reported", "cost_estimated",
	"code_added", "code_removed", "tests_added", "tests_removed", "prose_added", "prose_removed",
	"binary_files", "range", "notes",
}

// Row is one record. Every field is a cell as written; an empty cell is
// unknown, never zero (a run with no reported cost has an empty
// cost_reported). Lines is nil when no commit range was measured.
type Row struct {
	When                             string // RFC 3339
	Kind                             Kind
	Source                           string // the plan's or the swarm's name
	Unit                             string // the step or the run
	Role, Provider, Model, Effort    string
	RootSHA, HomeSHA                 string // the root and the home in use: every prompt and skill
	PromptHash, Outcome              string
	ReviewRounds                     string
	TokensIn, TokensOut, TokensTotal string
	CostReported, CostEstimated      string // in US dollars
	Lines                            *Lines
	Range                            string // the commit range Lines was measured over
	Notes                            string
}

// Class is a changed file's kind.
type Class int

const (
	Code Class = iota
	Tests
	Prose
)

func (c Class) String() string { return [...]string{"code", "tests", "prose"}[c] }

// Classify: a `_test.go` file, or one under a directory named `tests` or
// `test`, is tests; else a `.md` file, or one under a directory named
// `docs`, is prose; anything else is code. The directory may sit at any
// depth.
func Classify(path string) Class {
	parts := strings.Split(path, "/")
	dirs, base := parts[:len(parts)-1], parts[len(parts)-1]
	switch {
	case strings.HasSuffix(base, "_test.go") || slices.Contains(dirs, "tests") || slices.Contains(dirs, "test"):
		return Tests
	case strings.HasSuffix(base, ".md") || slices.Contains(dirs, "docs"):
		return Prose
	}
	return Code
}

// Lines is what a range adds and removes per class, and the binary files
// in it, which git counts no lines for and which are counted as none.
type Lines struct {
	CodeAdded, CodeRemoved   int
	TestsAdded, TestsRemoved int
	ProseAdded, ProseRemoved int
	Binary                   int
}

func (l *Lines) add(c Class, added, removed int) {
	switch c {
	case Tests:
		l.TestsAdded, l.TestsRemoved = l.TestsAdded+added, l.TestsRemoved+removed
	case Prose:
		l.ProseAdded, l.ProseRemoved = l.ProseAdded+added, l.ProseRemoved+removed
	default:
		l.CodeAdded, l.CodeRemoved = l.CodeAdded+added, l.CodeRemoved+removed
	}
}

func (l *Lines) plus(o Lines) {
	l.add(Code, o.CodeAdded, o.CodeRemoved)
	l.add(Tests, o.TestsAdded, o.TestsRemoved)
	l.add(Prose, o.ProseAdded, o.ProseRemoved)
	l.Binary += o.Binary
}

// Code, Tests and Prose are each class as `+added/-removed`.
func (l Lines) Code() string  { return pair(l.CodeAdded, l.CodeRemoved) }
func (l Lines) Tests() string { return pair(l.TestsAdded, l.TestsRemoved) }
func (l Lines) Prose() string { return pair(l.ProseAdded, l.ProseRemoved) }

func pair(added, removed int) string { return fmt.Sprintf("+%d/-%d", added, removed) }

// Measure is the lines git counts over rng in the repository at repo:
// `git diff --numstat -z -M <rng> --`, renames found, each file classed
// by the path it ends at. The range is `<a>..<b>` (or `<a>...<b>`) or
// `<commit>^!`; a lone commit would be diffed against the working tree,
// and a word starting with `-` would be an option, so both are refused.
func Measure(o ops.Ops, repo, rng string) (Lines, error) {
	if err := CheckRange(rng); err != nil {
		return Lines{}, err
	}
	out, stderr, err := o.Exec(repo, "git", "diff", "--numstat", "-z", "-M", rng, "--")
	if err != nil {
		return Lines{}, fmt.Errorf("git diff --numstat %s in %s failed: %s", rng, repo, firstLine(stderr, err))
	}
	return Count(out)
}

// CheckRange refuses what Measure would not read as a commit range.
func CheckRange(rng string) error {
	if strings.HasPrefix(rng, "-") || !(strings.Contains(rng, "..") || strings.HasSuffix(rng, "^!")) {
		return fmt.Errorf("a range is <a>..<b> or <commit>^! (got: %s)", rng)
	}
	return nil
}

func firstLine(stderr []byte, err error) string {
	if s, _, _ := strings.Cut(strings.TrimSpace(string(stderr)), "\n"); s != "" {
		return s
	}
	return err.Error()
}

// Count reads `git diff --numstat -z` output: `<added>\t<removed>\t<path>\0`
// per file, a rename's path empty and followed by `<from>\0<to>\0`, and a
// binary file's counts `-`.
func Count(out []byte) (Lines, error) {
	var l Lines
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		if fields[i] == "" && i == len(fields)-1 {
			break // the final NUL
		}
		counts := strings.SplitN(fields[i], "\t", 3)
		if len(counts) != 3 {
			return Lines{}, fmt.Errorf("unexpected git numstat entry %q", fields[i])
		}
		path := counts[2]
		if path == "" { // a rename or copy: from, then to
			if i+2 >= len(fields) || fields[i+2] == "" {
				return Lines{}, fmt.Errorf("unexpected git numstat rename %q", fields[i])
			}
			path, i = fields[i+2], i+2
		}
		if counts[0] == "-" && counts[1] == "-" {
			l.Binary++
			continue
		}
		added, err1 := strconv.Atoi(counts[0])
		removed, err2 := strconv.Atoi(counts[1])
		if err1 != nil || err2 != nil || added < 0 || removed < 0 {
			return Lines{}, fmt.Errorf("unexpected git numstat counts %q", fields[i])
		}
		l.add(Classify(path), added, removed)
	}
	return l, nil
}

var (
	wholeNumber = regexp.MustCompile(`^[0-9]+$`)
	amount      = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	sha         = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

// CheckCell is why value cannot stand in column, "" when it can: the
// counts are whole numbers, the costs amounts in dollars, the root's sha
// hex; an empty cell is always unknown, and so allowed.
func CheckCell(column, value string) string {
	if value == "" {
		return ""
	}
	switch column {
	case "review_rounds", "tokens_in", "tokens_out", "tokens_total", "code_added", "code_removed",
		"tests_added", "tests_removed", "prose_added", "prose_removed", "binary_files":
		if !wholeNumber.MatchString(value) {
			return "must be a whole number"
		}
	case "cost_reported", "cost_estimated":
		if !amount.MatchString(value) {
			return "must be a number of dollars, such as 1.25"
		}
	case "root_sha", "home_sha":
		if !sha.MatchString(value) {
			return "must be a commit's hex sha (7 to 64 of 0-9a-f)"
		}
	}
	return ""
}

func (r Row) cells() []string {
	c := []string{r.When, string(r.Kind), r.Source, r.Unit, r.Role, r.Provider, r.Model, r.Effort,
		r.RootSHA, r.HomeSHA, r.PromptHash, r.Outcome, r.ReviewRounds,
		r.TokensIn, r.TokensOut, r.TokensTotal, r.CostReported, r.CostEstimated}
	if l := r.Lines; l != nil {
		c = append(c, strconv.Itoa(l.CodeAdded), strconv.Itoa(l.CodeRemoved), strconv.Itoa(l.TestsAdded),
			strconv.Itoa(l.TestsRemoved), strconv.Itoa(l.ProseAdded), strconv.Itoa(l.ProseRemoved), strconv.Itoa(l.Binary))
	} else {
		c = append(c, "", "", "", "", "", "", "")
	}
	return append(c, r.Range, r.Notes)
}

// Check is why r cannot be written, nil when it can.
func (r Row) Check() error {
	if r.Kind != PlanStep && r.Kind != SwarmRun {
		return fmt.Errorf("kind must be %s or %s (got: %s)", PlanStep, SwarmRun, r.Kind)
	}
	if r.When == "" || r.Source == "" || r.Unit == "" {
		return errors.New("a row needs its when, its plan or swarm, and its step or run")
	}
	for i, v := range r.cells() {
		if why := CheckCell(Header[i], v); why != "" {
			return fmt.Errorf("%s %s (got: %s)", Header[i], why, v)
		}
	}
	return nil
}

// AppendCSV is the CSV file's content with r appended: the header first
// when the file is empty, else the file's own header, which must be
// Header, so a row never lands under other columns. The bytes before the
// row are the file's, a final newline added when it has none.
func AppendCSV(file []byte, r Row) ([]byte, error) {
	if err := r.Check(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if len(bytes.TrimSpace(file)) == 0 {
		w.Write(Header)
	} else {
		if err := checkHeader(file); err != nil {
			return nil, err
		}
		b.Write(file)
		if file[len(file)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	w.Write(r.cells())
	w.Flush()
	return b.Bytes(), w.Error()
}

func checkHeader(file []byte) error {
	header, err := csv.NewReader(bytes.NewReader(file)).Read()
	if err != nil || !slices.Equal(header, Header) {
		return fmt.Errorf("its header is not this Daisy's columns (%s); move the file aside and record again", strings.Join(Header, ","))
	}
	return nil
}

// TableHeader is the plan's `## Metrics` table's first two lines.
const TableHeader = "| Step | Role | Model | Effort | Root | Home | Outcome | Reviews | Tokens | Cost | Code | Tests | Prose | Range | Notes |\n" +
	"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n"

// tableRow is r as a line of the plan's table, its cells escaped (`|` as
// `\|`, a line end as a space).
func tableRow(r Row) string {
	var code, tests, prose string
	if r.Lines != nil {
		code, tests, prose = r.Lines.Code(), r.Lines.Tests(), r.Lines.Prose()
	}
	cells := []string{r.Unit, r.Role, r.Model, r.Effort, short(r.RootSHA), short(r.HomeSHA), r.Outcome, r.ReviewRounds, tokens(r), cost(r), code, tests, prose, r.Range, r.Notes}
	for i, c := range cells {
		c = strings.ReplaceAll(c, "|", `\|`)
		cells[i] = strings.Join(strings.Fields(strings.ReplaceAll(c, "\r", " ")), " ")
	}
	return "| " + strings.Join(cells, " | ") + " |\n"
}

// keyEnd is where a table row's second cell, the role, ends: the row up
// to it is the step and role, the key a later record replaces a row by.
func keyEnd(row string) int {
	cells := 0
	for i := 1; i < len(row); i++ {
		if row[i] == '|' && row[i-1] != '\\' {
			if cells++; cells == 2 {
				return i + 1
			}
		}
	}
	return len(row)
}

// short is a sha as the table shows it, its first 7 digits.
func short(sha string) string { return sha[:min(7, len(sha))] }

func tokens(r Row) string {
	switch {
	case r.TokensTotal != "":
		return r.TokensTotal
	case r.TokensIn != "" || r.TokensOut != "":
		return orUnknown(r.TokensIn) + " in, " + orUnknown(r.TokensOut) + " out"
	}
	return ""
}

func cost(r Row) string {
	switch {
	case r.CostReported != "":
		return "$" + r.CostReported
	case r.CostEstimated != "":
		return "~$" + r.CostEstimated
	}
	return ""
}

func orUnknown(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

// PlanRow is the plan with r as its step's row in the `## Metrics` table:
// the row of the same step and role replaced where the table has one,
// else appended. Without
// the table it is added at the end of the section; without the section,
// the section is added after `## Decisions`, or at the end of the plan,
// after a blank line (and the final newline a plan without one lacks).
// Every other byte outside the section is the plan's.
func PlanRow(doc []byte, r Row) ([]byte, error) {
	if err := r.Check(); err != nil {
		return nil, err
	}
	line := tableRow(r)
	start, end, ok := prompt.Section(doc, "Metrics")
	if !ok {
		at := len(doc)
		if _, decisionsEnd, ok := prompt.Section(doc, "Decisions"); ok {
			at = decisionsEnd
		}
		before := append([]byte{}, doc[:at]...)
		if len(before) > 0 && !bytes.HasSuffix(before, []byte("\n")) {
			before = append(before, '\n')
		}
		if len(before) > 0 && !bytes.HasSuffix(before, []byte("\n\n")) {
			before = append(before, '\n')
		}
		section := "## Metrics\n\n" + TableHeader + line
		if at < len(doc) {
			section += "\n"
		}
		return append(append(before, section...), doc[at:]...), nil
	}
	section := withRow(string(doc[start:end]), line)
	return append(append(append([]byte{}, doc[:start]...), section...), doc[end:]...), nil
}

// withRow is the section with line as its step and role's row of the
// table: the row whose first two cells are line's replaced, else line
// appended.
func withRow(section, line string) string {
	header, _, _ := strings.Cut(TableHeader, "\n")
	lines := strings.SplitAfter(section, "\n")
	for i, l := range lines {
		if strings.TrimRight(l, "\r\n") != header {
			continue
		}
		last := i + 1 // the table's last line: the separator, then each row
		prefix := line[:keyEnd(line)]
		for j := i + 2; j < len(lines) && strings.HasPrefix(lines[j], "|"); j++ {
			if strings.HasPrefix(lines[j], prefix) {
				lines[j] = line
				return strings.Join(lines, "")
			}
			last = j
		}
		if !strings.HasSuffix(lines[last], "\n") {
			lines[last] += "\n"
		}
		return strings.Join(slices.Insert(lines, last+1, line), "")
	}
	// The table goes after the section's last line of text, one blank
	// line apart; the blank lines after that text stay after the table.
	end, off := 0, 0
	for _, l := range lines {
		if t := strings.TrimRight(l, "\r\n"); strings.TrimSpace(t) != "" {
			end = off + len(t)
		}
		off += len(l)
	}
	gap := section[end:]
	if gap == "" {
		gap = "\n"
	}
	return section[:end] + "\n\n" + strings.TrimSuffix(TableHeader+line, "\n") + gap
}
