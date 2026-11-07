// Package plan is what a plan's name and text are made of: the slug a
// description becomes, the file name it goes under, the template filled,
// the description read back from a spec's heading, and the project-file
// links an archive repoints. Pure functions; the commands do the I/O.
package plan

import (
	"errors"
	"fmt"
	"strings"
)

// SlugMax caps the slug, the only unbounded part of a plan's file name.
// 100 leaves room for the timestamp, a project and the suffix under the
// 255-byte limit; the timestamp carries uniqueness, so the cut costs
// readability, not correctness.
const SlugMax = 100

// NameMax is the file-name limit: a name of this many bytes or more is
// refused before anything is written.
const NameMax = 255

// Slug is the description with every run of bytes other than ASCII
// letters and digits made one hyphen (whatever the locale: the bash's sed
// kept the locale's letters, so a slug could be multibyte), the hyphens at either end trimmed,
// cut to SlugMax bytes and trimmed again.
func Slug(description string) string {
	var b strings.Builder
	hyphen := false
	for i := 0; i < len(description); i++ {
		ch := description[i]
		if 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || '0' <= ch && ch <= '9' {
			b.WriteByte(ch)
			hyphen = false
		} else if !hyphen {
			b.WriteByte('-')
			hyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > SlugMax {
		s = strings.TrimRight(s[:SlugMax], "-")
	}
	return s
}

// FileName is `<ts>.<project>.<slug>.plan.md`, or `<ts>.<slug>.plan.md`
// without a project.
func FileName(ts, project, slug string) string {
	if project == "" {
		return ts + "." + slug + ".plan.md"
	}
	return ts + "." + project + "." + slug + ".plan.md"
}

// CheckName refuses a name the file system cannot hold, in terms of what
// the caller can change: the slug is already capped, so only the project
// can have made it too long.
func CheckName(name string) error {
	if len(name) < NameMax {
		return nil
	}
	return fmt.Errorf("Error: plan filename would be %d bytes; the limit is %d\n  %s\nShorten the project tag (the description is already truncated to %d characters).", len(name), NameMax, name, SlugMax)
}

// CheckProject refuses a project tag that cannot be part of a file name.
func CheckProject(project string) error {
	if strings.ContainsAny(project, "/\x00") {
		return fmt.Errorf("Error: project tag %q cannot be part of a file name (it holds '/' or a NUL)", project)
	}
	return nil
}

// ErrEmptySlug: the description has no letter or digit to name a file by.
var ErrEmptySlug = errors.New("Error: description produced an empty slug\nDescriptions need at least one letter or digit.")

// Fill is templates/plan.md for one plan, line by line: the
// `{Description}{project_suffix}` line becomes the H1 (`# <description>`,
// with ` — +<project>` when there is one), the `**Project:** +{project}`
// line names the project or goes, the first `{YYYY-MM-DD HH:MM}` on its
// line becomes created, and on any other line every `$DAISY_ROOT` becomes
// root. Every line ends with a newline.
func Fill(template, description, project, created, root string) string {
	if template == "" {
		return ""
	}
	lines := strings.Split(template, "\n")
	if strings.HasSuffix(template, "\n") {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	for _, l := range lines {
		switch {
		case strings.Contains(l, "{Description}{project_suffix}"):
			l = "# " + description
			if project != "" {
				l += " — +" + project
			}
		case strings.Contains(l, "**Project:** +{project}"):
			if project == "" {
				continue
			}
			l = "**Project:** +" + project
		case strings.Contains(l, "{YYYY-MM-DD HH:MM}"):
			l = strings.Replace(l, "{YYYY-MM-DD HH:MM}", created, 1)
		default:
			l = strings.ReplaceAll(l, "$DAISY_ROOT", root)
		}
		b.WriteString(l + "\n")
	}
	return b.String()
}

// Header reads a spec's first `# ` line back into the description and
// project Fill wrote it from: `<description> — +<project>` splits at the
// last ` — +`. ok is false when there is no such line or it is empty.
func Header(content string) (description, project string, ok bool) {
	for _, l := range strings.Split(content, "\n") {
		if !strings.HasPrefix(l, "# ") {
			continue
		}
		h := strings.TrimPrefix(l, "# ")
		if h == "" {
			return "", "", false
		}
		if i := strings.LastIndex(h, " — +"); i >= 0 {
			return h[:i], h[i+len(" — +"):], true
		}
		return h, "", true
	}
	return "", "", false
}

// Repoint rewrites every `../plans/<name>` in a project file to
// `../plans/archive/<name>`; changed is false, and content returned as it
// was, when no link names the plan.
func Repoint(content, name string) (out string, changed bool) {
	old := "../plans/" + name
	if !strings.Contains(content, old) {
		return content, false
	}
	return strings.ReplaceAll(content, old, "../plans/archive/"+name), true
}
