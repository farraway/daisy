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
		text, ok := Entry(l)
		if !ok {
			continue
		}
		switch t := todo.Parse(l); {
		case strings.Contains(l, "@git"):
			c.GitHub = append(c.GitHub, text)
		case t.Priority == 'A':
			c.Now = append(c.Now, text)
		case t.Priority == 'B':
			c.Next = append(c.Next, text)
		default:
			c.Inbox = append(c.Inbox, text)
		}
	}
	return c
}

// Entry is the text a new day writes for a todo.txt line, after `- [ ] `,
// and whether it carries the line at all (Carry's rules): `done` ticks the
// line by it.
func Entry(line string) (string, bool) {
	t := todo.Parse(line)
	if t.State != todo.Open || line == "" {
		return "", false
	}
	dated := t.Created != ""
	switch {
	case strings.Contains(line, "@git"):
		if dated && (t.Priority == 0 || t.Priority >= 'A' && t.Priority <= 'D') {
			return t.Text, true
		}
		return line, true
	case t.Priority == 'A' || t.Priority == 'B':
		if dated {
			return t.Text, true
		}
		return line, true
	case t.Priority == 0 && dated:
		return t.Text, true
	}
	return "", false
}

// Fill makes today.md from a day or week template: {DATE}, {DAY} and
// {TIME} from now, and each task placeholder with one `- [ ] ` line per
// task, in one pass over the template; the result ends in exactly one
// newline. A task's text is written as it is, a placeholder's name in it
// included (the bash's pattern substitution turned an `&` in it into the
// placeholder's name).
func Fill(template string, now time.Time, c Carried) string {
	return strings.TrimRight(strings.NewReplacer(
		"{DATE}", now.Format(time.DateOnly),
		"{DAY}", now.Format("Monday"),
		"{TIME}", now.Format("1504"),
		"{HIGH_PRIORITY_TASKS}", checklist(c.Now),
		"{NEXT_PRIORITY_TASKS}", checklist(c.Next),
		"{INBOX_TASKS}", checklist(c.Inbox),
		"{GITHUB_TASKS}", checklist(c.GitHub),
	).Replace(template), "\n") + "\n"
}

func checklist(tasks []string) string {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString("- [ ] " + t + "\n")
	}
	return b.String()
}
