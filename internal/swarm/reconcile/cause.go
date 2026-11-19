package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"strings"
)

// causes are the words a member's stop names its cause by (section 7.5,
// The stop's cause), the 2026-11-19 roadmap's fixed vocabulary in its
// order, each with the gloss the Task text gives it.
var causes = []struct{ Word, Gloss string }{
	{"plan", "the plan, or the issue where there is none, is wrong, unclear or cannot be followed"},
	{"prerequisite", "work the job depends on is not merged"},
	{"main-red", "the base branch is broken: its check fails without your change"},
	{"environment", "a tool, the image, a service or the network the run needs fails or is missing"},
	{"forge-fact", "a fact on the forge you need cannot be read"},
	{"kept-work", "work Daisy was to keep or give you is missing: a plan cut short, a kept commit not given back"},
	{"ruling", "only the maintainer's ruling settles it"},
	{"scope", "whether you may change what the plan or the job leaves out"},
	{"capacity", "a provider's limit, quota or credit stopped the work"},
}

// causeSentence ends the closing paragraph of every Task text (section
// 7.4): the Cause: line asked for under a stop's line, and the words.
var causeSentence = func() string {
	glossed := make([]string, len(causes))
	for i, c := range causes {
		glossed[i] = fmt.Sprintf("`%s` (%s)", c.Word, c.Gloss)
	}
	glossed[len(glossed)-1] = "or " + glossed[len(glossed)-1]
	return "Write one line `Cause: <word>` under any line beginning `BLOCKED:` or `QUESTION:` that you write, even one this text calls the only line, the word naming why you stopped: " +
		strings.Join(glossed, ", ") + "."
}()

// causeWords is the vocabulary as a warning names it.
func causeWords() string {
	words := make([]string, len(causes))
	for i, c := range causes {
		words[i] = c.Word
	}
	return strings.Join(words, ", ")
}

// isCause: word is one of causes.
func isCause(word string) bool {
	for _, c := range causes {
		if c.Word == word {
			return true
		}
	}
	return false
}

// stopCause is the word of the Cause: line under a stop's line (section
// 7.5, The stop's cause): the stop's line the output's first that is
// neither blank nor a Met: line (the batch's report, before it), and
// beginning BLOCKED: or QUESTION:; the Cause: line the next line after
// it that is not blank. found is false when there is no such line.
func stopCause(out string) (word string, found bool) {
	lines := strings.Split(out, "\n")
	i := 0
	for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || metLine.MatchString(strings.TrimRight(lines[i], " "))) {
		i++
	}
	if i == len(lines) || !strings.HasPrefix(lines[i], "BLOCKED:") && !strings.HasPrefix(lines[i], question) {
		return "", false
	}
	for i++; i < len(lines) && strings.TrimSpace(lines[i]) == ""; i++ {
	}
	if i == len(lines) {
		return "", false
	}
	rest, ok := strings.CutPrefix(lines[i], "Cause:")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// cause is the row's cause (section 7.6, The run's record): for a
// member's stop (outcome blocked or question), the word of its out.md's
// Cause: line; "" for any other run, for a stop without the line, and,
// with a warning, for a word not among causes or an out.md that cannot
// be read.
func (c *collection) cause(outcome string) string {
	if outcome != "blocked" && outcome != "question" {
		return ""
	}
	out, err := c.r.Output(c.s.O.FS())
	if err != nil {
		if !errors.Is(err, iofs.ErrNotExist) {
			fmt.Fprintf(c.s.Err, "⚠ %scause left empty: %v\n", c.head, err)
		}
		return ""
	}
	word, found := stopCause(string(out))
	if !found {
		return ""
	}
	if !isCause(word) {
		fmt.Fprintf(c.s.Err, "⚠ %scause left empty: %q is not one of %s\n", c.head, word, causeWords())
		return ""
	}
	return word
}
