package journal

import (
	"strings"
	"time"

	"daisy/internal/todo"
)

// Carried is what a new day takes from todo.txt into today.md: one
// description per open task, by section.
type Carried struct {
	Now, Next, Inbox, GitHub []string
}

// Carry sorts the open lines of todo.txt into today's sections. A line
// mentioning @git (so @github too) goes to GitHub whatever its priority;
// otherwise (A) is Now, (B) is Next and a dated line without a priority is
// Inbox; anything else stays in todo.txt only. The description is the line
// less its priority and creation date — when it has a date; an undated
// (A) line, or a GitHub line whose priority is outside A–D, is carried
// whole.
func Carry(lines []string) Carried {
	var c Carried
	for _, l := range lines {
		t := todo.Parse(l)
		if t.State != todo.Open || l == "" {
			continue
		}
		dated := t.Created != ""
		switch {
		case strings.Contains(l, "@git"):
			if dated && (t.Priority == 0 || t.Priority >= 'A' && t.Priority <= 'D') {
				l = t.Text
			}
			c.GitHub = append(c.GitHub, l)
		case t.Priority == 'A':
			c.Now = append(c.Now, textIfDated(t, l))
		case t.Priority == 'B':
			c.Next = append(c.Next, textIfDated(t, l))
		case t.Priority == 0 && dated:
			c.Inbox = append(c.Inbox, t.Text)
		}
	}
	return c
}

func textIfDated(t todo.Task, line string) string {
	if t.Created != "" {
		return t.Text
	}
	return line
}

// Fill makes today.md from a day or week template: {DATE}, {DAY} and
// {TIME} from now, then each task placeholder with one `- [ ] ` line per
// task, in that order; the result ends in exactly one newline. A task's
// text is written as it is (the bash's pattern substitution turned an `&`
// in it into the placeholder's name).
func Fill(template string, now time.Time, c Carried) string {
	s := strings.TrimRight(template, "\n")
	for _, r := range []struct {
		placeholder, value string
	}{
		{"{DATE}", now.Format(time.DateOnly)},
		{"{DAY}", now.Format("Monday")},
		{"{TIME}", now.Format("1504")},
		{"{HIGH_PRIORITY_TASKS}", checklist(c.Now)},
		{"{NEXT_PRIORITY_TASKS}", checklist(c.Next)},
		{"{INBOX_TASKS}", checklist(c.Inbox)},
		{"{GITHUB_TASKS}", checklist(c.GitHub)},
	} {
		s = strings.ReplaceAll(s, r.placeholder, r.value)
	}
	return s + "\n"
}

func checklist(tasks []string) string {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString("- [ ] " + t + "\n")
	}
	return b.String()
}
