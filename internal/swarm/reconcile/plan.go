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
// 7.5): the lines matching `^[0-9]+\. ` between its `## Steps` line and
// the next `## ` line, outside a fenced block; "" for a line naming none.
// A plan without slice lines has none.
func Slices(plan string) []string {
	var out []string
	in, fence := false, ""
	for _, line := range strings.Split(plan, "\n") {
		line = strings.TrimRight(line, "\r")
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
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == "## Steps"
			continue
		}
		if in && sliceLine.MatchString(line) {
			name := ""
			if m := sliceRepo.FindStringSubmatch(line); m != nil {
				name = m[1]
			}
			out = append(out, name)
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
// by more `#`, at most six in all (section 7.3).
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
		h := len(line) - len(strings.TrimLeft(line, "#"))
		if h >= 1 && h <= 6 && (len(line) == h || line[h] == ' ' || line[h] == '\t') {
			lines[i] = strings.Repeat("#", min(6, h+by)-h) + line
		}
	}
	return strings.Join(lines, "\n")
}
