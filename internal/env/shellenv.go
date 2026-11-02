package env

import (
	"fmt"
	"regexp"
	"strings"
)

var assignment = regexp.MustCompile(`^(export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)=`)
var exportOnly = regexp.MustCompile(`^export([ \t]+[A-Za-z_][A-Za-z0-9_]*)+[ \t]*(#.*)?$`)

// ParseShellEnv reads a workspace's .env.sh without running it: each line
// `[export ]NAME=<word>` assigns, the word in the shell's quoting ('…'
// literal; "…" and bare words with `\` escapes and $NAME / ${NAME}
// expanded from the assignments above it, else lookup), optionally
// followed by a `# comment`. Blank lines, comments and a bare `export
// NAME` change nothing. The first other line — a command, a substitution,
// a quote or a continuation left open, a second word — is named in
// problems, and reading stops there: it and every line after it assign
// nothing, since what the shell would do with them depends on running it.
func ParseShellEnv(src []byte, lookup func(string) string) (vars map[string]string, problems []string) {
	vars = map[string]string{}
	get := func(k string) string {
		if v, ok := vars[k]; ok {
			return v
		}
		return lookup(k)
	}
	text := strings.TrimSuffix(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimLeft(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") || exportOnly.MatchString(line) {
			continue
		}
		why := "not an assignment"
		if m := assignment.FindStringSubmatch(line); m != nil {
			value, err := shellWord(line[len(m[0]):], get, m[1] != "")
			if err == nil {
				vars[m[2]] = value
				continue
			}
			why = err.Error()
		}
		// The shell would run this line, and what it runs may decide how
		// the lines after it read (a quote, a heredoc, an if): stop here.
		problems = append(problems, fmt.Sprintf("line %d: %s; it and every line after it are not read", i+1, why))
		break
	}
	return vars, problems
}

// shellWord reads one shell word and what may follow it: nothing, or
// blanks and a comment.
func shellWord(s string, get func(string) string, exported bool) (string, error) {
	if strings.HasPrefix(s, "~") {
		return "", fmt.Errorf("a leading '~' is not read: write $HOME")
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch ch := s[i]; ch {
		case ' ', '\t':
			if rest := strings.TrimLeft(s[i:], " \t"); rest != "" && !strings.HasPrefix(rest, "#") {
				return "", fmt.Errorf("more than one word after '='")
			}
			return b.String(), nil
		case '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return "", fmt.Errorf("unterminated quote")
			}
			b.WriteString(s[i+1 : i+1+end])
			i += end + 1
		case '"':
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				switch {
				case s[i] == '\\' && i+1 == len(s):
					return "", fmt.Errorf("a line continuation is not read")
				case s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\"\\$`", s[i+1]) >= 0:
					i++
					b.WriteByte(s[i])
				case s[i] == '$' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\''):
					b.WriteByte('$') // "a$" and "a$'x'" keep a literal dollar
				case s[i] == '$':
					n, err := expand(s[i:], get, &b)
					if err != nil {
						return "", err
					}
					i += n - 1
				case s[i] == '`':
					return "", fmt.Errorf("command substitution is not read")
				default:
					b.WriteByte(s[i])
				}
			}
			if i >= len(s) {
				return "", fmt.Errorf("unterminated quote")
			}
		case '\\':
			if i+1 >= len(s) {
				return "", fmt.Errorf("a line continuation is not read")
			}
			i++
			b.WriteByte(s[i])
		case '$':
			n, err := expand(s[i:], get, &b)
			if err != nil {
				return "", err
			}
			i += n - 1
		case '{':
			// `export` is a command: bash brace-expands its arguments.
			if exported {
				return "", fmt.Errorf("brace expansion in an export is not read")
			}
			b.WriteByte(ch)
		case '~':
			if i > 0 && s[i-1] == ':' {
				return "", fmt.Errorf("a '~' after ':' is not read: write $HOME")
			}
			b.WriteByte(ch)
		case '`', ';', '&', '|', '<', '>', '(', ')':
			return "", fmt.Errorf("'%c' is shell syntax, not a value", ch)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String(), nil
}

var (
	plainVar  = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)
	bracedVar = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// expand writes the value of the $NAME or ${NAME} at the start of s and
// returns its length; a lone `$` is itself.
func expand(s string, get func(string) string, b *strings.Builder) (int, error) {
	if m := bracedVar.FindStringSubmatch(s); m != nil {
		b.WriteString(get(m[1]))
		return len(m[0]), nil
	}
	if m := plainVar.FindStringSubmatch(s); m != nil {
		b.WriteString(get(m[1]))
		return len(m[0]), nil
	}
	if len(s) > 1 && strings.ContainsRune("({'\"0123456789$?!#@*-[", rune(s[1])) {
		return 0, fmt.Errorf("'%s' is not read: only $NAME and ${NAME} expand", s[:2])
	}
	b.WriteByte('$')
	return 1, nil
}
