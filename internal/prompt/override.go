package prompt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"daisy/internal/ops"
)

// Homes consume the root. A home overrides a root prompt (or skill) by a
// file at the same path in the home, in one of two forms:
//
//   - layered: the file is Marker and one `## Rules` section, the home's
//     rules only. The prompt is the root's file with those rules appended to
//     its `## Rules` section, renumbered after the root's (a root without the
//     section gets one where apply --rules would put it). Upstream changes
//     flow through; a home rule may refine a root rule, never delete one.
//   - full: the file holds the whole prompt; the root's is not read.
//
// One rule (Merge) builds the layered text for every reader: prompt build,
// init's skill copy, help and the optimize briefing.

// Marker is the line a layered override's body is: the root's file stands
// in for it.
const Marker = "<!-- daisy: upstream body -->"

// Form is what a prompt or skill path is across the root and the home.
type Form int

const (
	Absent   Form = iota // neither has it
	RootOnly             // the root's file, no home file
	HomeOwn              // the home's file, the root has none
	Layered              // the home's file is the marker and its rules
	Full                 // the home's file replaces the root's
)

func (f Form) String() string {
	return [...]string{"absent", "root", "home", "layered", "full"}[f]
}

// Source is one path (Rel: `prompts/<name>.md` or `skills/<name>/SKILL.md`)
// in the root and the home, and its form.
type Source struct {
	Rel, Root, Home string
	Form            Form
}

// Locate is rel in home and root. A home file holding the marker line
// (outside a fence) is layered, whatever else it holds and whether or not
// the root has the file: Text then refuses what is not exactly the marker
// and the rules, or a root with nothing to layer on. A home file that
// cannot be read counts as full (or the home's own); Text reports it.
func Locate(fs ops.FS, home, root, rel string) Source {
	s := Source{Rel: rel, Root: root + "/" + rel, Home: home + "/" + rel}
	inRoot := regular(fs, s.Root)
	if regular(fs, s.Home) {
		s.Form = HomeOwn
		if inRoot {
			s.Form = Full
		}
		if doc, err := fs.ReadFile(s.Home); err == nil && IsLayered(doc) {
			s.Form = Layered
		}
		return s
	}
	if inRoot {
		s.Form = RootOnly
	}
	return s
}

// Overridden: the home's file stands over the root's.
func (s Source) Overridden() bool { return s.Form == Layered || s.Form == Full }

// Path is the file that holds the text as written: the root's for a root
// prompt, else the home's.
func (s Source) Path() string {
	if s.Form == RootOnly || s.Form == Absent {
		return s.Root
	}
	return s.Home
}

// Text is the prompt as an agent reads it: a layered one merged. The
// error names the file at fault.
func (s Source) Text(fs ops.FS) ([]byte, error) {
	doc, err := fs.ReadFile(s.Path())
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", s.Path(), err)
	}
	if s.Form != Layered {
		return doc, nil
	}
	root, err := fs.ReadFile(s.Root)
	if err != nil {
		if !regular(fs, s.Root) {
			return nil, fmt.Errorf("%s is a layered override (it holds the line %s), but the root has no %s to layer it on; write the whole prompt there instead, or remove it", s.Home, Marker, s.Rel)
		}
		return nil, fmt.Errorf("cannot read %s: %v", s.Root, err)
	}
	merged, err := Merge(root, doc)
	if _, ok := err.(TargetError); ok {
		return nil, fmt.Errorf("%s (the root's, under the layered %s): %v", s.Root, s.Home, err)
	} else if err != nil {
		return nil, fmt.Errorf("%s: %v", s.Home, err)
	}
	return merged, nil
}

// IsLayered: doc holds the marker line outside a fence.
func IsLayered(doc []byte) bool {
	for _, l := range scan(doc) {
		if !l.fenced && strings.TrimSpace(l.text) == Marker {
			return true
		}
	}
	return false
}

// Merge is root with the layered file's rules appended to its `## Rules`
// section, numbered after the root's rules; without a section, one is
// inserted as apply --rules inserts it. A layered file that is not the
// marker and one `## Rules` section is refused with the line; a root
// section apply --rules could not write is refused as the root's
// (TargetError).
func Merge(root, layered []byte) ([]byte, error) {
	section, err := layerSection(layered)
	if err != nil {
		return nil, err
	}
	if err := RulesProblem(root); err != nil {
		return nil, err
	}
	body := sectionBody(section)
	if len(body) == 0 {
		return root, nil
	}
	start, end, ok := RulesSection(root)
	if !ok {
		heading := section[:bytes.IndexByte(append(section, '\n'), '\n')]
		return insertRules(root, append(append(append([]byte{}, heading...), "\n\n"...), renumber(body, 1)...)), nil
	}
	kept := trimBlankLines(root[start:end])
	gap := root[start+len(kept) : end]
	out := append([]byte{}, root[:start+len(kept)]...)
	out = append(out, "\n\n"...)
	out = append(out, renumber(body, CountRules(root)+1)...)
	out = append(append(out, gap...), root[end:]...)
	return out, nil
}

// layerSection is the layered file's `## Rules` section, trimmed; nil when
// the file is the marker alone. Anything else is an error naming its line.
func layerSection(doc []byte) ([]byte, error) {
	ls := scan(doc)
	i := 0
	blank := func() {
		for i < len(ls) && strings.TrimSpace(ls[i].text) == "" {
			i++
		}
	}
	refuse := func(l line, why string) error {
		return fmt.Errorf("line %d ('%s') %s; a layered override is the line %s and then one ## Rules section holding the home's rules (use --rewrite, a full override, for anything else)", l.n, l.text, why, Marker)
	}
	blank()
	if i == len(ls) || strings.TrimSpace(ls[i].text) != Marker {
		if i == len(ls) {
			return nil, fmt.Errorf("is empty where the line %s should be", Marker)
		}
		return nil, refuse(ls[i], "stands before the marker")
	}
	i++
	blank()
	if i == len(ls) {
		return nil, nil
	}
	if !ls[i].rulesHeading() {
		return nil, refuse(ls[i], "is not the ## Rules heading")
	}
	head := ls[i]
	for _, l := range ls[i+1:] {
		if l.level >= 1 && l.level <= 2 {
			return nil, refuse(l, "is a heading outside the rules")
		}
		if !l.fenced && strings.TrimSpace(l.text) == Marker {
			return nil, refuse(l, "is the marker again")
		}
	}
	rest := doc[head.off:]
	if n := openFence(rest); n > 0 {
		return nil, refuse(ls[i+n-1], "opens a code fence that never closes")
	}
	if n := setextUnderline(rest); n > 0 {
		return nil, refuse(ls[i+n-1], "may be a setext heading underline")
	}
	return trimBlankLines(rest), nil
}

// sectionBody is a section without its heading line and the blank lines
// around what follows it.
func sectionBody(section []byte) []byte {
	nl := bytes.IndexByte(section, '\n')
	if nl < 0 {
		return nil
	}
	rest := section[nl+1:]
	for len(rest) > 0 {
		e := bytes.IndexByte(rest, '\n')
		if e < 0 || strings.TrimSpace(string(rest[:e])) != "" {
			break
		}
		rest = rest[e+1:]
	}
	return trimBlankLines(rest)
}

var numbered = regexp.MustCompile(`^[0-9]+\. `)

// renumber numbers body's numbered rules (at the left margin, outside a
// fence) from next on; bullets and everything else stay as they are.
func renumber(body []byte, next int) []byte {
	var out []byte
	for _, l := range scan(body) {
		text := l.text
		if !l.fenced && numbered.MatchString(text) {
			text = strconv.Itoa(next) + text[strings.IndexByte(text, '.'):]
			next++
		}
		out = append(out, text...)
		out = append(out, body[l.off+len(l.text):min(len(body), lineEnd(body, l.off))]...)
	}
	return out
}

// lineEnd is the offset just past the newline of the line at off.
func lineEnd(b []byte, off int) int {
	if nl := bytes.IndexByte(b[off:], '\n'); nl >= 0 {
		return off + nl + 1
	}
	return len(b)
}

// LayeredFrom is the layered file an apply --rules proposal makes over
// root: the proposal is the whole section as the agent saw it merged, so
// the root's section must stand first in it, unchanged, and the rules
// after it are the home's, renumbered from 1. A root rule changed or
// dropped is refused with the proposal's line: that is a full override's
// work.
func LayeredFrom(root, proposal []byte) ([]byte, error) {
	p, err := rulesProposal(proposal)
	if err != nil {
		return nil, err
	}
	if err := RulesProblem(root); err != nil {
		return nil, err
	}
	var rest []byte
	if start, end, ok := RulesSection(root); ok {
		kept := trimBlankLines(root[start:end])
		if !bytes.HasPrefix(p, kept) || (len(p) > len(kept) && p[len(kept)] != '\n' && p[len(kept)] != '\r') {
			n := firstDiff(p, kept) + firstLine(proposal)
			return nil, rootChanged(n)
		}
		rest = p[len(kept):]
		// A line that continues the root's last rule changes that rule
		// too: an indented one (a sub-item, a second paragraph), or one
		// right after it that starts no block of its own (a lazy
		// continuation; a list item, heading, fence, quote or break starts
		// one).
		at := len(p) - len(bytes.TrimLeft(rest, "\r\n"))
		adjacent := bytes.Count(p[len(kept):at], []byte("\n")) == 1
		if at < len(p) && (p[at] == ' ' || p[at] == '\t' || adjacent && !blockStart.Match(bytes.TrimRight(p[at:lineEnd(p, at)], "\r\n"))) {
			n := bytes.Count(p[:at], []byte("\n")) + firstLine(proposal)
			return nil, rootChanged(n)
		}
	} else {
		rest = sectionBody(p)
	}
	out := []byte(Marker + "\n\n## Rules\n")
	if body := trimBlankLines(bytes.TrimLeft(rest, "\r\n \t")); len(body) > 0 {
		out = append(append(append(out, '\n'), renumber(body, 1)...), '\n')
	}
	return out, nil
}

// blockStart is a line that ends the paragraph before it in CommonMark,
// not continues it: a list item, an ATX heading, a fence, a block quote or
// a thematic break. An HTML line is not told apart (inline HTML continues
// the paragraph; only some block tags end it), so it counts as a
// continuation and the proposal is refused: the safe side.
var blockStart = regexp.MustCompile("^(([0-9]+[.)]|[-*+])([ \t]|$)|#{1,6}([ \t]|$)|```|~~~|>|(\\*[ \t]*){3,}$|(_[ \t]*){3,}$|(-[ \t]*){3,}$)")

// rootChanged refuses a proposal that changes the root's rules at line n.
func rootChanged(n int) error {
	return fmt.Errorf("changes the root's rules at line %d; a layered override only adds rules after the root's, which stand first and unchanged (use --rewrite, a full override, to change or drop one)", n)
}

// firstDiff is the 0-based line of a where it first differs from b.
func firstDiff(a, b []byte) int {
	n := 0
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		if a[i] == '\n' {
			n++
		}
	}
	return n
}

// firstLine is the 1-based line of proposal's first non-blank line.
func firstLine(proposal []byte) int {
	for _, l := range scan(proposal) {
		if strings.TrimSpace(l.text) != "" {
			return l.n
		}
	}
	return 1
}

// Hash is the content hash a full override records of the root's file.
func Hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// regular: p is a regular file, or a link to one (as env.Lookup reads).
func regular(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// LedgerRel is the home file recording, for each full override, the root
// file's hash when the override was made: one `<rel> <hash>` line each.
const LedgerRel = "feedback/.overrides"

// ReadLedger is the home's ledger, rel → hash; no file is an empty one.
func ReadLedger(fs ops.FS, home string) (map[string]string, error) {
	data, err := fs.ReadFile(home + "/" + LedgerRel)
	if err != nil {
		if _, serr := fs.Lstat(home + "/" + LedgerRel); serr != nil {
			return map[string]string{}, nil
		}
		return nil, err
	}
	ledger := map[string]string{}
	for _, l := range lines(data) {
		if rel, hash, ok := cutLast(l); ok {
			ledger[rel] = hash
		}
	}
	return ledger, nil
}

// SetLedger is data with rel's line recording hash, replaced in place or
// appended.
func SetLedger(data []byte, rel, hash string) []byte {
	var out []string
	found := false
	for _, l := range lines(data) {
		if r, _, _ := cutLast(l); r == rel {
			l, found = rel+" "+hash, true
		}
		out = append(out, l)
	}
	if !found {
		out = append(out, rel+" "+hash)
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// cutLast splits a ledger line at its last space: a path may hold
// spaces, a hash never does.
func cutLast(l string) (rel, hash string, ok bool) {
	i := strings.LastIndexByte(l, ' ')
	if i < 0 {
		return "", "", false
	}
	return l[:i], l[i+1:], true
}

// Behind: s is a full override whose ledger hash is not the root file's
// now (the root changed since the copy, or cannot be read). A full
// override with no ledger line is not known to be behind; one whose root
// file went is no override at all (Locate makes it the home's own).
func (s Source) Behind(fs ops.FS, ledger map[string]string) bool {
	recorded, ok := ledger[s.Rel]
	if s.Form != Full || !ok {
		return false
	}
	root, err := fs.ReadFile(s.Root)
	return err != nil || Hash(root) != recorded
}

// SkillFiles are the files init installs for a skill the home overrides
// (s a SKILL.md source, layered or full), by path in the skill directory:
// the root's skill directory, the home's other files over it, and the
// SKILL.md as Text reads it.
func (s Source) SkillFiles(fs ops.FS) (map[string][]byte, error) {
	text, err := s.Text(fs)
	if err != nil {
		return nil, err
	}
	out := Files(fs, filepath.Dir(s.Root))
	for rel, b := range Files(fs, filepath.Dir(s.Home)) {
		out[rel] = b
	}
	out["SKILL.md"] = text
	return out, nil
}

// Files are the regular files under dir and what they hold, by path
// under it; links and what cannot be read are passed over.
func Files(fs ops.FS, dir string) map[string][]byte {
	out := map[string][]byte{}
	var walk func(rel string)
	walk = func(rel string) {
		entries, _ := fs.ReadDir(filepath.Join(dir, rel))
		for _, en := range entries {
			p := filepath.Join(rel, en.Name())
			if fi, err := fs.Lstat(filepath.Join(dir, p)); err == nil && fi.IsDir() {
				walk(p)
			} else if err == nil && fi.Mode().IsRegular() {
				if b, err := fs.ReadFile(filepath.Join(dir, p)); err == nil {
					out[p] = b
				}
			}
		}
	}
	walk("")
	return out
}
