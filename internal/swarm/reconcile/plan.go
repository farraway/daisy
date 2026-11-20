package reconcile

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/swarm/fence"
)

var (
	sliceLine = regexp.MustCompile(`^[0-9]+\. `)
	sliceRepo = regexp.MustCompile(`^[0-9]+\. ([A-Za-z0-9_][A-Za-z0-9_.-]*): `)
)

// Slices is the repository each slice of plan names, in order (section
// 7.5): the lines matching `^[0-9]+\. ` under its `## Steps` heading,
// outside a fenced block; "" for a line naming none. A plan without slice
// lines has none.
func Slices(plan string) []string {
	var out []string
	for _, line := range sliceLines(plan) {
		name := ""
		if m := sliceRepo.FindStringSubmatch(line); m != nil {
			name = m[1]
		}
		out = append(out, name)
	}
	return out
}

// sliceLines is the plan's slice lines (Slices), each as written.
func sliceLines(plan string) []string {
	var out []string
	for _, line := range section(plan, "Steps") {
		if !line.fenced && sliceLine.MatchString(line.text) {
			out = append(out, line.text)
		}
	}
	return out
}

// sliceText is slices/<s>/slice.md (section 7.8): slice s's item under
// plan's ## Steps, its numbered line and the lines under it to the next
// slice line or the section's end, then, after a blank line, its section
// of builder (builderSection); the item alone when builder has none. A
// plan without slice lines has one slice, whose item is its whole ##
// Steps section. Empty lines at either end of each part are dropped.
func sliceText(plan, builder string, s int) string {
	steps := section(plan, "Steps")
	var starts []int
	for i, line := range steps {
		if !line.fenced && sliceLine.MatchString(line.text) {
			starts = append(starts, i)
		}
	}
	var item []planLine
	switch {
	case len(starts) == 0 && s == 1:
		item = steps
	case s >= 1 && s <= len(starts):
		end := len(steps)
		if s < len(starts) {
			end = starts[s]
		}
		item = steps[starts[s-1]:end]
	}
	parts := []string{}
	for _, part := range [][]string{lineTexts(item), builderSection(builder, s)} {
		if t := strings.Trim(strings.Join(part, "\n"), "\n"); t != "" {
			parts = append(parts, t+"\n")
		}
	}
	return strings.Join(parts, "\n")
}

// builderSection is slice s's section of a builder plan (section 7.8):
// its lines from the first heading `## Slice <s>`, alone or followed by a
// character that is not a digit, to the next `## ` line, each outside a
// fenced block, without a trailing `\r`; none when it has no such
// heading.
func builderSection(builder string, s int) []string {
	heading := "## Slice " + strconv.Itoa(s)
	var out []string
	in, fences := false, fence.Scan{}
	for _, line := range strings.Split(builder, "\n") {
		line = strings.TrimRight(line, "\r")
		if !fences.Line(line) && strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			rest, ok := strings.CutPrefix(line, heading)
			in = ok && (rest == "" || rest[0] < '0' || rest[0] > '9')
		}
		if in {
			out = append(out, line)
		}
	}
	return out
}

func lineTexts(lines []planLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}

// planLine is a line of a plan section, and whether it is inside a fenced
// block (its fence lines included).
type planLine struct {
	text   string
	fenced bool
}

// section is the lines between plan's `## <heading>` line and the next
// `## ` line outside a fenced block, without a trailing `\r`; none when
// the plan has no such heading.
func section(plan, heading string) []planLine {
	var out []planLine
	in, fences := false, fence.Scan{}
	for _, line := range strings.Split(plan, "\n") {
		line = strings.TrimRight(line, "\r")
		fenced := fences.Line(line)
		if !fenced && strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == "## "+heading
			continue
		}
		if in {
			out = append(out, planLine{line, fenced})
		}
	}
	return out
}

// SliceRepos is each slice's declared repository, from 1, for a plan read
// against the declared names; bad is the slice numbers that are not each
// in one declared repository (section 7.5: with one repository declared, a
// line naming none, a line whose prefix names no repository the swarm
// knows (`1. Parser: …`, the slice's own text), or a plan without slice
// lines, is in it). known is the names the swarm holds a mirror of, so a
// repository removed from repos since the plan still names its slices,
// which no longer resolve.
func SliceRepos(plan string, names, known []string) (repos []string, bad []int) {
	lines := Slices(plan)
	if len(lines) == 0 {
		if len(names) == 1 {
			return []string{names[0]}, nil
		}
		return nil, []int{1}
	}
	for i, name := range lines {
		switch {
		case len(names) == 1 && name != names[0] && !slices.Contains(known, name):
			name = names[0]
		case !slices.Contains(names, name):
			bad = append(bad, i+1)
		}
		repos = append(repos, name)
	}
	return repos, bad
}

func numbers(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

// Shift gives each Markdown heading line of text, outside a fenced block,
// by more `#`, at most six in all, and closes a fenced block text leaves
// open, so that no fence runs on over the prompt's own lines (section 7.3).
func Shift(text string, by int) string {
	lines := strings.Split(text, "\n")
	var fences fence.Scan
	for i, line := range lines {
		if fences.Line(line) {
			continue
		}
		// A heading may stand up to three spaces in (CommonMark), as a
		// fence may, and an empty one ends a CRLF line at its `\r`.
		t := strings.TrimLeft(line, " ")
		in := len(line) - len(t)
		h := len(t) - len(strings.TrimLeft(t, "#"))
		if in <= 3 && h >= 1 && h <= 6 && (len(t) == h || t[h] == ' ' || t[h] == '\t' || t[h:] == "\r") {
			lines[i] = line[:in] + strings.Repeat("#", min(6, h+by)-h) + t
		}
	}
	if f := fences.Open(); f != "" {
		lines = append(lines, f)
	}
	return strings.Join(lines, "\n")
}
