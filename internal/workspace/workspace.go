// Package workspace binds a directory to an identity and its home (init)
// and takes the binding out again (clean). Everything init writes is
// decided before its first write; the binding itself — .daisy/ and the
// repository's identity keys — is prepared beside .daisy/, applied, and
// swapped in by rename, or rolled back whole. clean undoes what init did:
// the keys init applied go back to what they held before it, the ignore
// blocks and allow rules it added come out, the files it wrote are removed.
package workspace

import (
	"path/filepath"
	"sort"
	"strings"

	"daisy/internal/identity"
	"daisy/internal/ops"
	"daisy/internal/skill"
)

// The files init writes into a workspace, outside .daisy/.
var (
	cursorRules = []struct{ Name, Template string }{
		{"daisy.mdc", "cursor-rule.mdc"},
		{"daisy-logging.mdc", "cursor-rule-logging.mdc"},
		{"daisy-plan.mdc", "cursor-rule-plan.mdc"},
	}
	// obsoleteRules are names init once wrote and now sweeps.
	obsoleteRules = []string{"daisy.md", "daisy-logging.md"}
	claudeCommand = ".claude/commands/daisy.md"
	settingsFile  = ".claude/settings.local.json"
	allowRules    = []string{"Read(.daisy/**)", "Edit(.daisy/**)", "Bash(daisy:*)"}

	gitignoreHeader    = "# Daisy workspace config (per-machine)"
	gitignoreEntries   = []string{".daisy/", ".daisy.stage.*", ".daisy.previous.*", ".cursor/rules/daisy.mdc", ".cursor/rules/daisy-logging.mdc", ".cursor/rules/daisy-plan.mdc", ".claude/commands/daisy.md", ".claude/settings.local.json", "PLAN.md", ".cursorignore"}
	cursorignoreHeader = "# Allow Cursor to index daisy paths (gitignored but needed for agent context)"
	cursorignoreLines  = []string{"!.daisy/", "!.cursor/rules/daisy.mdc", "!.cursor/rules/daisy-logging.mdc", "!PLAN.md"}
)

// The files of .daisy/ that init owns and rewrites on every run: the
// binding links (env.Links) and these.
const (
	homeFile     = "home"
	identityFile = "identity"
	recordFile   = "gitconfig"        // the identity init applied
	beforeFile   = "gitconfig.before" // what the keys held before it was applied
	agentsFile   = "AGENTS.md"
)

// legacyRules are allow rules older inits wrote, removed by clean.
func legacyRules(root string) []string {
	return []string{"Write(.daisy/**)", "Bash(" + root + "/daisy/scripts/*)", "Read(" + root + "/**)", "Edit(" + root + "/**)", "Write(" + root + "/**)"}
}

func sortedNames(m map[string]string) []string {
	var names []string
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// skillEntries are the ignore lines for a skill's copy and its stub.
func skillEntries(name string) []string {
	return []string{".claude/skills/" + name + "/", ".cursor/rules/" + name + ".mdc"}
}

// isGitignoreEntry: a line init may have written into the .gitignore block.
func isGitignoreEntry(line string) bool {
	if hasLine(gitignoreEntries, line) {
		return true
	}
	if n, ok := cutAround(line, ".claude/skills/", "/"); ok && n != "" {
		return true
	}
	n, ok := cutAround(line, ".cursor/rules/", ".mdc")
	return ok && n != ""
}

func cutAround(s, prefix, suffix string) (string, bool) {
	if len(s) < len(prefix)+len(suffix) || s[:len(prefix)] != prefix || s[len(s)-len(suffix):] != suffix {
		return "", false
	}
	return s[len(prefix) : len(s)-len(suffix)], true
}

// stub is the Cursor rule that points at a skill's copy, from its SKILL.md.
func stub(name string, doc []byte) []byte {
	desc, _ := skill.Field(doc, "description")
	if len(desc) > 0 && desc[len(desc)-1] == '"' {
		desc = desc[:len(desc)-1]
	}
	if len(desc) > 0 && desc[0] == '"' {
		desc = desc[1:]
	}
	return stubText(name, desc)
}

func stubText(name, desc string) []byte {
	return []byte("---\ndescription: " + desc + "\nalwaysApply: false\n---\n\n# " + name + "\n\nRead `.claude/skills/" + name + "/SKILL.md` when: " + desc + "\n")
}

// stubs are the skills whose .cursor/rules/<name>.mdc is a stub in exactly
// the form init writes: the one record of which skills init installed.
func stubs(fsys ops.FS, ws string) []string {
	dir := filepath.Join(ws, ".cursor", "rules")
	entries, _ := fsys.ReadDir(dir)
	var names []string
	for _, e := range entries {
		n, ok := strings.CutSuffix(e.Name(), ".mdc")
		if !ok || n == "" || n == "." || n == ".." || isCoreRule(e.Name()) {
			continue
		}
		// A regular file only: a link's target is not this workspace's.
		p := filepath.Join(dir, e.Name())
		if fi, err := fsys.Lstat(p); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		b, err := fsys.ReadFile(p)
		if err != nil {
			continue
		}
		if desc, ok := skill.Field(b, "description"); ok && string(b) == string(stubText(n, desc)) {
			names = append(names, n)
		}
	}
	return names
}

// claimOf is what a binding says init applied: the record when it reads;
// else the identity file the binding names, when that resolves (a binding
// from before records, or a damaged record). file is where it was read,
// "" with ok false when neither reads. before is what the keys held before
// the identity was applied, when init kept it.
type claim struct {
	id     identity.Identity
	file   string
	ok     bool
	before identity.Values
}

func readBefore(fsys ops.FS, daisyDir string) identity.Values {
	b, err := fsys.ReadFile(filepath.Join(daisyDir, beforeFile))
	if err != nil {
		return nil
	}
	v, err := identity.ParseValues(b)
	if err != nil {
		return nil
	}
	return v
}

func isFile(fsys ops.FS, p string) bool {
	fi, err := fsys.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func isDir(fsys ops.FS, p string) bool {
	fi, err := fsys.Stat(p)
	return err == nil && fi.IsDir()
}

func exists(fsys ops.FS, p string) bool {
	_, err := fsys.Lstat(p)
	return err == nil
}

// removeEmptyDirs removes each directory that is empty, deepest first as given.
func removeEmptyDirs(fsys ops.FS, dirs ...string) {
	for _, d := range dirs {
		if entries, err := fsys.ReadDir(d); err == nil && len(entries) == 0 {
			fsys.Remove(d)
		}
	}
}
