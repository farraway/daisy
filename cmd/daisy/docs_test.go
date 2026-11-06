package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/cli"
)

// docFiles are the documents whose commands and paths must exist: what an
// agent or a reader follows. Paths are relative to the module root, where
// TestMain runs the tests.
func docFiles(t *testing.T) []string {
	t.Helper()
	files := []string{"README.md", "AGENTS.md"}
	for _, g := range []string{"docs/*.md", "prompts/*.md"} {
		m, err := filepath.Glob(g)
		if err != nil || len(m) == 0 {
			t.Fatalf("glob %s: %v %v", g, m, err)
		}
		files = append(files, m...)
	}
	return files
}

// codeSpan is one piece of code in a document: a backtick span, or a line
// of a fenced block.
type codeSpan struct {
	line                     int
	text, rowPrefix, section string // rowPrefix: the line up to the span; section: the last `## ` heading
	fenced                   bool
}

// listItem is a line that starts a list item.
var listItem = regexp.MustCompile(`^ *([-*+]|\d+\.) `)

var spanRe = regexp.MustCompile("(`+)(.+?)`+")

func codeSpans(t *testing.T, file string) []codeSpan {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var spans []codeSpan
	fence, section := false, ""
	var para []string // the current paragraph's lines; a span may wrap
	start := 0        // its first line's index
	flush := func() {
		text := strings.Join(para, "\n")
		for i := 0; i < len(text); {
			if text[i] != '`' {
				i++
				continue
			}
			n := 1
			for i+n < len(text) && text[i+n] == '`' {
				n++
			}
			// The closing run is the next run of exactly n backticks.
			end := -1
			for j := i + n; j < len(text); {
				if text[j] != '`' {
					j++
					continue
				}
				k := j
				for k < len(text) && text[k] == '`' {
					k++
				}
				if k-j == n {
					end = j
					break
				}
				j = k
			}
			if end < 0 {
				i += n // no closer: literal backticks
				continue
			}
			line := start + strings.Count(text[:i], "\n")
			lineStart := strings.LastIndex(text[:i], "\n") + 1
			code := strings.ReplaceAll(text[i+n:end], "\n", " ")
			spans = append(spans, codeSpan{line + 1, code, text[lineStart:i], section, false})
			i = end + n
		}
		para = nil
	}
	for i, l := range strings.Split(string(b), "\n") {
		if !fence && strings.HasPrefix(l, "## ") {
			section = strings.TrimPrefix(l, "## ")
		}
		if strings.HasPrefix(strings.TrimLeft(l, " "), "```") {
			flush()
			fence = !fence
			continue
		}
		if fence {
			spans = append(spans, codeSpan{i + 1, l, "", section, true})
			continue
		}
		// A blank line, a heading or a table row ends a paragraph; a row
		// is a paragraph of its own.
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "|") {
			flush()
			if strings.HasPrefix(l, "|") {
				para, start = []string{l}, i
				flush()
			}
			continue
		}
		if listItem.MatchString(l) { // a list item starts a paragraph of its own
			flush()
		}
		if len(para) == 0 {
			start = i
		}
		para = append(para, l)
	}
	flush()
	return spans
}

// Every `daisy <command>` (or `bin/daisy <command>`) in the docs is a row of
// the cli table, and every dash word of the phrase is a flag the
// table gives it (`--help`/`-h`, or `--healthcheck` for a command with a
// hook, only alone), so a renamed or removed command
// or flag cannot survive in prose. The removed names a document records on
// purpose are docs/cli.md's history: its Legacy rows and the Legacy column
// of its Renames table.
func TestDocsNameOnlyTableCommands(t *testing.T) {
	rowRe := regexp.MustCompile("^\\| [^|]+ \\| `daisy ([^`]*)`")
	wordRe := regexp.MustCompile(`^[a-z][a-z-]*$`)
	usage := map[string]string{} // command name -> usage line
	first := map[string]bool{}
	for _, l := range strings.Split(cli.Reference(), "\n") {
		m := rowRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		u := strings.ReplaceAll(m[1], `\|`, "|")
		var name []string
		for _, w := range strings.Fields(u) {
			if !wordRe.MatchString(w) {
				break
			}
			name = append(name, w)
		}
		usage[strings.Join(name, " ")] = u
		first[name[0]] = true
	}
	if len(usage) < 25 {
		t.Fatalf("read %d commands from the table", len(usage))
	}
	// A program path ending bin/daisy counts where it starts the span
	// (`<root>/bin/daisy install`, `~/bin/daisy …`); elsewhere a bare daisy.
	useRe := regexp.MustCompile(`(?:^ *[^ ]*bin/|(?:^|[^/\w.-]))daisy((?: +[^ ]+)+)`)
	for _, f := range docFiles(t) {
		for _, s := range codeSpans(t, f) {
			if f == "docs/cli.md" && (strings.HasPrefix(s.rowPrefix, "| Legacy |") ||
				s.section == "Renames" && strings.Count(s.rowPrefix, "|") == 1) {
				continue // history: a Legacy row, or a Renames row's Legacy cell
			}
			if f == "docs/cli.md" && s.section == "Command tree" && s.fenced {
				continue // the help text itself: TestCliMdTreeIsHelp
			}
			var uses [][]string // each command of a `a && b; c` line on its own
			for _, part := range regexp.MustCompile(` *(&&|\|\||;) *`).Split(s.text, -1) {
				uses = append(uses, useRe.FindAllStringSubmatch(part, -1)...)
			}
			for _, m := range uses {
				var w []string // the words up to the first that ends the phrase
				for _, tok := range strings.Fields(m[1]) {
					clean := strings.TrimLeft(tok, "`'\"(")
					end := strings.TrimRight(clean, "`'\".,;:)")
					if end != "" {
						w = append(w, end)
					}
					if end != clean {
						break
					}
				}
				if len(w) == 0 {
					continue
				}
				if !regexp.MustCompile(`^[A-Za-z]`).MatchString(w[0]) && !strings.HasPrefix(w[0], "-") {
					continue // `daisy <command>`, `daisy "…"`: not a name
				}
				if strings.HasPrefix(w[0], "-") {
					w = append([]string{""}, w...) // `daisy --help`: the bare program
				}
				// The longest table name the words start with.
				name := ""
				for n := range usage {
					nw := strings.Fields(n)
					if len(nw) <= len(w) && len(nw) > len(strings.Fields(name)) && slices.Equal(nw, w[:len(nw)]) {
						name = n
					}
				}
				// docs/swarm.md specifies the Swarm group before it is built
				// (Steps 27–30): an agreed verb absent from the table is
				// checked against its agreed flags; once the verb is a row,
				// the table governs it.
				agreed, isAgreed := []string(nil), false
				if f == "docs/swarm.md" && name == "" && len(w) >= 2 {
					agreed, isAgreed = swarmAgreed[w[0]+" "+w[1]]
					if isAgreed {
						name = w[0] + " " + w[1]
					}
				}
				if w[0] != "" && name == "" {
					if first[w[0]] && len(w) > 1 && !wordRe.MatchString(w[1]) && !strings.HasPrefix(w[1], "-") {
						continue // `daisy plan …`, `daisy home <verb>`: a group named, no verb
					}
					if first[w[0]] && len(w) == 1 {
						continue
					}
					t.Errorf("%s:%d: `daisy %s` is not a command of the cli table", f, s.line, strings.Join(w, " "))
					continue
				}
				allowed, health, _ := cli.Flags(name)
				if isAgreed {
					allowed = agreed
				}
				if name == "help" { // help answers its flag itself; its usage shows it
					allowed = append(allowed, "--markdown")
				}
				rest := w[len(strings.Fields(name)):]
				if w[0] == "" {
					rest = w[1:] // the bare program takes only --help and -h
				}
				if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h" || rest[0] == "--healthcheck" && health) {
					continue
				}
				var words []string // `--todo|--done` is two
				for _, tok := range rest {
					words = append(words, strings.Split(strings.ReplaceAll(tok, `\|`, "|"), "|")...)
				}
				for _, tok := range words {
					// Every dash word of the phrase, values and brackets
					// between them included (`--workflow <n> --rules`).
					tok = strings.TrimLeft(tok, "[(")
					if !strings.HasPrefix(tok, "-") {
						continue
					}
					flag := regexp.MustCompile(`=.*$`).ReplaceAllString(strings.TrimRight(tok, "])|"), "")
					if flag != "" && flag != "-" && !slices.Contains(allowed, flag) {
						t.Errorf("%s:%d: `daisy %s` takes no %s here", f, s.line, strings.TrimSpace(name), flag)
					}
				}
			}
		}
	}
	built := 0
	for n := range swarmAgreed {
		if _, ok := usage[n]; ok {
			built++
		}
	}
	if built == len(swarmAgreed) {
		t.Error("every agreed swarm verb is a row of the cli table: drop swarmAgreed")
	}
}

// swarmAgreed is the Swarm group's verbs not yet built, as docs/cli.md
// agrees them (D30), and their flags, which docs/swarm.md names before
// the binary runs them; a built verb (`swarm init`) is a row of the table.
var swarmAgreed = map[string][]string{
	"swarm apply":  {"--once"},
	"swarm status": nil,
	"swarm pause":  nil,
	"swarm resume": nil,
}

// pathPlaceholders are the repository-shaped paths a document names that
// are not tracked files, by document, each with its reason.
var pathPlaceholders = map[string]string{
	"README.md bin/daisy":                  "make build writes it (gitignored)",
	"AGENTS.md bin/daisy":                  "make build writes it (gitignored)",
	"docs/cli.md bin/daisy":                "make build writes it (gitignored)",
	"docs/home-management.md bin/daisy":    "make build writes it (gitignored)",
	"docs/cli.md identities/pelicanmoor":   "case 77/79 fixture identity",
	"docs/cli.md home/stray/note.txt":      "case 73 fixture",
	"docs/cli.md prompts/daisy":            "a feedback target name, not a file",
	"docs/cli.md feedback/.last-optimized": "the home file optimize apply writes",
	"docs/cli.md feedback/.overrides":      "the home file a full override records the root's hash in, created on first use",
	"docs/cli.md y/N/e":                    "the bash optimize prompt's answers",
	"AGENTS.md os/exec":                    "a Go standard package",
	"docs/swarm.md swarm/swarm.json":       "the workspace file swarm init writes, not a root file",
}

// Every repository-relative path a document names, in backticks or as a
// link target, is a tracked file or directory: under the root, beside the
// document, or in a home (templates/home). Absolute, home-anchored and
// placeholder paths (`~/…`, `$DAISY_ROOT/…`, `<home>/…`, `{name}`), paths
// starting with a dot (workspace paths: `.daisy/…`, `.claude/…`) and names
// without a directory are not checked. Outside a git checkout (a tarball
// of the root) there is no tracked set, and the test is skipped.
func TestDocsNameExistingPaths(t *testing.T) {
	git := exec.Command("git", "ls-files", "-z")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			git.Env = append(git.Env, kv)
		}
	}
	git.Env = append(git.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if _, err := os.Stat(".git"); err != nil {
		t.Skip("not a git checkout: no tracked set to check against")
	}
	out, err := git.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	tracked := map[string]bool{}
	for _, f := range strings.Split(string(out), "\x00") {
		for ; f != "." && f != "" && f != "/"; f = filepath.Dir(f) {
			tracked[f] = true
		}
	}
	if !tracked["cmd/daisy/main.go"] {
		t.Fatalf("read %d tracked paths", len(tracked))
	}
	pathRe := regexp.MustCompile(`^(?:\./)?[\w-][\w./-]*/[\w./-]*$`)
	linkRe := regexp.MustCompile(`\]\(([^)#\s]+)(?:#[^)]*)?\)`)
	used := map[string]bool{}
	for _, f := range docFiles(t) {
		var names []codeSpan
		for _, s := range codeSpans(t, f) {
			if !s.fenced {
				names = append(names, s)
			}
		}
		b, _ := os.ReadFile(f)
		fence := false
		for i, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimLeft(l, " "), "```") {
				fence = !fence
			}
			for _, m := range linkRe.FindAllStringSubmatch(spanRe.ReplaceAllString(l, "``"), -1) { // a link inside code is code
				if !fence && !strings.Contains(m[1], ":") {
					names = append(names, codeSpan{line: i + 1, text: m[1], rowPrefix: "]("})
				}
			}
		}
		for _, s := range names {
			p := strings.TrimSpace(s.text)
			link := s.rowPrefix == "]("
			if !link && (!pathRe.MatchString(p) || !strings.Contains(strings.TrimRight(p, "/"), "/")) {
				continue
			}
			p = strings.TrimRight(strings.TrimPrefix(p, "./"), "/")
			found := false
			for _, base := range []string{"", filepath.Dir(f), "templates/home"} {
				if tracked[filepath.Clean(filepath.Join(base, p))] {
					found = true
				}
			}
			if _, ok := pathPlaceholders[f+" "+p]; ok && !found {
				used[f+" "+p] = true
				continue
			}
			if !found {
				t.Errorf("%s:%d: `%s` is not a tracked path", f, s.line, p)
			}
		}
	}
	for p := range pathPlaceholders {
		if !used[p] {
			t.Errorf("placeholder %q is no longer named, or is now tracked; drop it", p)
		}
	}
}

// docs/cli.md's command tree is `daisy help`, byte for byte.
func TestCliMdTreeIsHelp(t *testing.T) {
	b, err := os.ReadFile("docs/cli.md")
	if err != nil {
		t.Fatal(err)
	}
	_, tree, ok := strings.Cut(string(b), "\n```\nUsage: daisy <command>")
	tree, _, ok2 := strings.Cut(tree, "```\n")
	m := newMachine(t)
	code, out, _ := m.run(t, m.ws, "help")
	if !ok || !ok2 || code != 0 || "Usage: daisy <command>"+tree != out {
		t.Errorf("docs/cli.md's Command tree is not `daisy help` (exit %d); replace it with:\n%s", code, out)
	}
}
