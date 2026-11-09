package reconcile

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
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
	in, fence := false, ""
	for _, line := range strings.Split(plan, "\n") {
		line = strings.TrimRight(line, "\r")
		f := fenceOf(line)
		if f == "" && fence == "" && strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == "## "+heading
			continue
		}
		fenced := fence != "" || f != ""
		if f != "" && (fence == "" || strings.HasPrefix(f, fence)) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
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
// line naming none, or a plan without slice lines, is in it).
func SliceRepos(plan string, names []string) (repos []string, bad []int) {
	lines := Slices(plan)
	if len(lines) == 0 {
		if len(names) == 1 {
			return []string{names[0]}, nil
		}
		return nil, []int{1}
	}
	for i, name := range lines {
		switch {
		case name == "" && len(names) == 1:
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

// fenceOf is a fence line's marker (``` or ~~~ and any more of the same),
// "" when the line opens or closes no fenced block.
func fenceOf(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return ""
	}
	for _, c := range []string{"`", "~"} {
		n := len(t) - len(strings.TrimLeft(t, c))
		if n >= 3 {
			return strings.Repeat(c, n)
		}
	}
	return ""
}

// Shift gives each Markdown heading line of text, outside a fenced block,
// by more `#`, at most six in all, and closes a fenced block text leaves
// open, so that no fence runs on over the prompt's own lines (section 7.3).
func Shift(text string, by int) string {
	lines := strings.Split(text, "\n")
	fence := ""
	for i, line := range lines {
		if f := fenceOf(line); f != "" && (fence == "" || strings.HasPrefix(f, fence)) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		// A heading may stand up to three spaces in (CommonMark), as a
		// fence may (fenceOf).
		t := strings.TrimLeft(line, " ")
		in := len(line) - len(t)
		h := len(t) - len(strings.TrimLeft(t, "#"))
		if in <= 3 && h >= 1 && h <= 6 && (len(t) == h || t[h] == ' ' || t[h] == '\t') {
			lines[i] = line[:in] + strings.Repeat("#", min(6, h+by)-h) + t
		}
	}
	if fence != "" {
		lines = append(lines, fence)
	}
	return strings.Join(lines, "\n")
}
