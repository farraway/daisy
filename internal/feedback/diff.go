package feedback

import (
	"fmt"
	"strings"
)

// context is the unchanged lines shown around a change, as `diff -u`.
const context = 3

// Diff is the unified diff from a to b (empty when they are equal), headed
// by the two names, three lines of context, hunks always `-l,n +l,n`: a
// patch(1) applies it, though GNU `diff -u` may differ in which of two
// equal-length edits it shows. A file not ending in a newline says so
// after its last line.
func Diff(aName, bName string, a, b []byte) string {
	if string(a) == string(b) {
		return ""
	}
	as, bs := diffLines(a), diffLines(b)
	ops := script(as, bs)
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", aName, bName)
	for start := 0; start < len(ops); {
		// The next hunk: from the first change, through every change less
		// than 2*context unchanged lines after the one before.
		first := start
		for first < len(ops) && ops[first].kind == ' ' {
			first++
		}
		if first == len(ops) {
			break
		}
		last := first
		for i, run := first, 0; i < len(ops); i++ {
			if ops[i].kind == ' ' {
				run++
				if run > 2*context {
					break
				}
			} else {
				last, run = i, 0
			}
		}
		lo, hi := max(first-context, start), min(last+context+1, len(ops))
		hunk := ops[lo:hi]
		aStart, aLen, bStart, bLen := hunk[0].a, 0, hunk[0].b, 0
		for _, o := range hunk {
			if o.kind != '+' {
				aLen++
			}
			if o.kind != '-' {
				bLen++
			}
		}
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", span(aStart, aLen), span(bStart, bLen))
		for _, o := range hunk {
			out.WriteString(string(o.kind) + o.text + "\n")
			if o.noEOL {
				out.WriteString("\\ No newline at end of file\n")
			}
		}
		start = hi
	}
	return out.String()
}

func span(start, n int) string {
	if n == 0 {
		return fmt.Sprintf("%d,0", start)
	}
	return fmt.Sprintf("%d,%d", start+1, n)
}

type dline struct {
	text  string
	noEOL bool
}

func diffLines(b []byte) []dline {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	noEOL := !strings.HasSuffix(s, "\n")
	parts := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	out := make([]dline, len(parts))
	for i, p := range parts {
		out[i] = dline{text: p}
	}
	out[len(out)-1].noEOL = noEOL
	return out
}

// edit is one line of the script: ' ' kept, '-' only in a, '+' only in b;
// a and b are the 0-based line numbers it stands at in each.
type edit struct {
	kind  byte
	text  string
	noEOL bool
	a, b  int
}

// script is a shortest edit script from as to bs, by longest common
// subsequence; prompts are hundreds of lines, so the table is small.
func script(as, bs []dline) []edit {
	n, m := len(as), len(bs)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if as[i] == bs[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []edit
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && as[i] == bs[j]:
			out = append(out, edit{' ', as[i].text, as[i].noEOL, i, j})
			i, j = i+1, j+1
		case i < n && (j == m || lcs[i+1][j] >= lcs[i][j+1]):
			out = append(out, edit{'-', as[i].text, as[i].noEOL, i, j})
			i++
		default:
			out = append(out, edit{'+', bs[j].text, bs[j].noEOL, i, j})
			j++
		}
	}
	return out
}
