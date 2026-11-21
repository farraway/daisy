package declare

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ServicesFile is the product's test services file at its repository's
// root, read when repos[].services names none (section 3).
const ServicesFile = "compose.test.yaml"

// ServicesPath is the repository's test services file: its services
// field, else ServicesFile.
func (r Repo) ServicesPath() string {
	if r.Services != nil {
		return *r.Services
	}
	return ServicesFile
}

// TestServices is a product's test services file as Daisy runs it
// (section 3, repos[].services): its services in name order, and
// x-daisy-env's variables, `<name>=<value>` in name order, given to the
// check and the worker beside them.
type TestServices struct {
	Services []TestService
	Env      []string
}

// TestService is one service of the subset: its name (its address on
// the private network), its image, the paths mounted as tmpfs, its
// limits (`<name>=<soft>:<hard>`, by name), its environment
// (`<name>=<value>`, in the file's order for a list, by name for a
// mapping) and its command, none when absent.
type TestService struct {
	Name, Image                  string
	Tmpfs, Ulimits, Env, Command []string
}

// serviceKeys is the subset of a Compose service Daisy runs.
var serviceKeys = []string{"image", "tmpfs", "ulimits", "environment", "command"}

var (
	serviceShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	ulimitShape  = regexp.MustCompile(`^[a-z]+$`)
	digits       = regexp.MustCompile(`^[0-9]+$`)
)

// ParseServices reads a test services file (section 3): YAML of the
// subset Daisy reads (block mappings and lists, one-line flow
// collections, plain and quoted scalars, literal and folded block
// scalars, comments), a mapping of exactly
// services, one or more, each of the keys serviceKeys, and optional
// x-daisy-env. Anything else is refused, never run in part.
func ParseServices(data []byte) (TestServices, error) {
	root, err := readYAML(data)
	if err != nil {
		return TestServices{}, err
	}
	if root == nil {
		return TestServices{}, errors.New("services: missing")
	}
	if root.kind != yMap {
		return TestServices{}, errors.New("not a mapping of services and x-daisy-env")
	}
	for _, k := range root.keys {
		if k != "services" && k != "x-daisy-env" {
			return TestServices{}, fmt.Errorf("%s: not read by Daisy (a file holds services and x-daisy-env)", k)
		}
	}
	var out TestServices
	svcs, ok := root.items["services"]
	switch {
	case !ok:
		return out, errors.New("services: missing")
	case svcs.kind != yMap:
		return out, errors.New("services: not a mapping")
	case len(svcs.keys) == 0:
		return out, errors.New("services: none (at least one is required)")
	}
	for _, name := range slices.Sorted(slices.Values(svcs.keys)) {
		sv, err := service("services."+name, name, svcs.items[name])
		if err != nil {
			return TestServices{}, err
		}
		out.Services = append(out.Services, sv)
	}
	if env, ok := root.items["x-daisy-env"]; ok {
		if out.Env, err = envOf("x-daisy-env", env); err != nil {
			return TestServices{}, err
		}
	}
	return out, nil
}

// service is one service's entry, at the dotted path at.
func service(at, name string, n *yNode) (TestService, error) {
	sv := TestService{Name: name}
	if !serviceShape.MatchString(name) {
		return sv, fmt.Errorf("%s: %q is not a service name", at, name)
	}
	if n.kind != yMap {
		return sv, fmt.Errorf("%s: not a mapping", at)
	}
	for _, k := range n.keys {
		if !slices.Contains(serviceKeys, k) {
			return sv, fmt.Errorf("%s.%s: not in the subset Daisy runs (%s)", at, k, strings.Join(serviceKeys, ", "))
		}
	}
	image, ok := n.items["image"]
	switch {
	case !ok:
		return sv, fmt.Errorf("%s.image: missing", at)
	case image.kind != yScalar:
		return sv, fmt.Errorf("%s.image: not a value", at)
	case image.value == "":
		return sv, fmt.Errorf("%s.image: empty", at)
	case strings.HasPrefix(image.value, "-"):
		// docker run would read it as an option (--network=host).
		return sv, fmt.Errorf("%s.image: %q is not an image name", at, image.value)
	}
	sv.Image = image.value
	if t, ok := n.items["tmpfs"]; ok {
		paths, err := scalars(at+".tmpfs", t, true)
		if err != nil {
			return sv, err
		}
		for i, p := range paths {
			if !strings.HasPrefix(p, "/") {
				return sv, fmt.Errorf("%s.tmpfs[%d]: %q is not an absolute path", at, i, p)
			}
		}
		sv.Tmpfs = paths
	}
	if u, ok := n.items["ulimits"]; ok {
		var err error
		if sv.Ulimits, err = ulimitsOf(at+".ulimits", u); err != nil {
			return sv, err
		}
	}
	if e, ok := n.items["environment"]; ok {
		var err error
		if sv.Env, err = envOf(at+".environment", e); err != nil {
			return sv, err
		}
	}
	if c, ok := n.items["command"]; ok {
		if c.kind == yScalar && !c.null {
			var q []string
			for _, w := range strings.Fields(c.value) {
				q = append(q, strconv.Quote(w))
			}
			return sv, fmt.Errorf("%s.command: a string; write it as a list, [%s]", at, strings.Join(q, ", "))
		}
		var err error
		if sv.Command, err = scalars(at+".command", c, false); err != nil {
			return sv, err
		}
	}
	return sv, nil
}

// scalars is a list of values, or with one a single value.
func scalars(at string, n *yNode, one bool) ([]string, error) {
	if one && n.kind == yScalar && !n.null {
		return []string{n.value}, nil
	}
	if n.kind != yList {
		return nil, fmt.Errorf("%s: not a list", at)
	}
	var out []string
	for i, it := range n.list {
		if it.kind != yScalar || it.null {
			return nil, fmt.Errorf("%s[%d]: not a value", at, i)
		}
		out = append(out, it.value)
	}
	return out, nil
}

// ulimitsOf is limits as Compose's ulimits writes them, a mapping of a
// limit's name to one number (its soft and hard limit) or to exactly
// soft and hard, as docker run's --ulimit takes them:
// `<name>=<soft>:<hard>`, by name.
func ulimitsOf(at string, n *yNode) ([]string, error) {
	if n.kind != yMap {
		return nil, fmt.Errorf("%s: not a mapping", at)
	}
	var out []string
	for _, k := range slices.Sorted(slices.Values(n.keys)) {
		v, kat := n.items[k], at+"."+k
		if !ulimitShape.MatchString(k) {
			return nil, fmt.Errorf("%s: %q is not a limit's name", kat, k)
		}
		var soft, hard int64
		switch v.kind {
		case yScalar:
			var err error
			if soft, err = limitOf(kat, v); err != nil {
				return nil, err
			}
			hard = soft
		case yMap:
			for _, l := range v.keys {
				if l != "soft" && l != "hard" {
					return nil, fmt.Errorf("%s.%s: not soft or hard", kat, l)
				}
			}
			for _, l := range []struct {
				name string
				to   *int64
			}{{"soft", &soft}, {"hard", &hard}} {
				lv, ok := v.items[l.name]
				if !ok {
					return nil, fmt.Errorf("%s.%s: missing", kat, l.name)
				}
				var err error
				if *l.to, err = limitOf(kat+"."+l.name, lv); err != nil {
					return nil, err
				}
			}
			if soft > hard {
				// docker run refuses it, so the file is refused here.
				return nil, fmt.Errorf("%s: soft %d is above hard %d", kat, soft, hard)
			}
		default:
			return nil, fmt.Errorf("%s: not a number nor a mapping of soft and hard", kat)
		}
		out = append(out, fmt.Sprintf("%s=%d:%d", k, soft, hard))
	}
	return out, nil
}

// limitOf is a limit's number: digits, as docker run reads them, with no
// leading 0, which Compose's YAML reads as octal.
func limitOf(at string, v *yNode) (int64, error) {
	switch {
	case v.kind != yScalar:
		return 0, fmt.Errorf("%s: not a number", at)
	case v.null:
		return 0, fmt.Errorf("%s: no value", at)
	case !digits.MatchString(v.value):
		return 0, fmt.Errorf("%s: %q is not a whole number, 0 or more", at, v.value)
	case len(v.value) > 1 && v.value[0] == '0':
		return 0, fmt.Errorf("%s: %q begins with 0, which Compose reads as octal; write it without", at, v.value)
	}
	l, err := strconv.ParseInt(v.value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is above %d", at, v.value, int64(math.MaxInt64))
	}
	return l, nil
}

// envOf is variables as Compose's environment writes them: a mapping of
// name to value, by name, or a list of `<name>=<value>`, in order; each
// name a variable name and each given a value.
func envOf(at string, n *yNode) ([]string, error) {
	var out []string
	switch n.kind {
	case yMap:
		for _, k := range slices.Sorted(slices.Values(n.keys)) {
			v := n.items[k]
			switch {
			case !varShape.MatchString(k):
				return nil, fmt.Errorf("%s.%s: %q is not a variable name", at, k, k)
			case v.kind != yScalar:
				return nil, fmt.Errorf("%s.%s: not a value", at, k)
			case v.null:
				return nil, fmt.Errorf("%s.%s: no value", at, k)
			}
			out = append(out, k+"="+v.value)
		}
	case yList:
		if at == "x-daisy-env" {
			return nil, fmt.Errorf("%s: not a mapping", at)
		}
		for i, it := range n.list {
			k, _, ok := strings.Cut(it.value, "=")
			if it.kind != yScalar || !ok || !varShape.MatchString(k) {
				return nil, fmt.Errorf("%s[%d]: %q is not <name>=<value>", at, i, it.value)
			}
			out = append(out, it.value)
		}
	default:
		return nil, fmt.Errorf("%s: not a mapping", at)
	}
	return out, nil
}

// The YAML subset. A node is a scalar (its text; null for an absent
// value, ~ or null), a mapping (its keys in the file's order) or a list.
type yKind int

const (
	yScalar yKind = iota
	yMap
	yList
)

type yNode struct {
	kind  yKind
	value string
	null  bool
	keys  []string
	items map[string]*yNode
	list  []*yNode
}

// yLine is a line that holds YAML outside a block scalar: its number,
// its indentation, its text without its comment, and why it is refused,
// as the parser reads it (a block scalar's line is text).
type yLine struct {
	n, indent int
	text      string
	err       error
}

func lineErr(n int, format string, args ...any) error {
	return fmt.Errorf("line %d: %s", n, fmt.Sprintf(format, args...))
}

// readYAML is data's one document, nil when it holds none.
func readYAML(data []byte) (*yNode, error) {
	raw := strings.Split(string(data), "\n")
	eol := raw[len(raw)-1] == "" // the last line ends with a line break
	if eol {
		raw = raw[:len(raw)-1]
	}
	var lines []yLine
	for i, r := range raw {
		r = strings.TrimSuffix(r, "\r")
		raw[i] = r
		body := strings.TrimLeft(r, " ")
		text := strings.TrimRight(uncomment(body), " \t")
		if text == "" {
			continue
		}
		l := yLine{n: i + 1, indent: len(r) - len(body), text: text}
		switch {
		case strings.HasPrefix(body, "\t"):
			l.err = lineErr(l.n, "a tab in the indentation")
		case text == "---" && len(r) == len(body):
			if len(lines) == 0 {
				continue
			}
			l.err = lineErr(l.n, "a second document")
		case strings.HasPrefix(text, "%"):
			l.err = lineErr(l.n, "a directive (%%) is not read")
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return nil, nil
	}
	p := &yParser{raw: raw, eol: eol, lines: lines}
	root, err := p.block(lines[0].indent)
	if err != nil {
		return nil, err
	}
	if p.i < len(lines) {
		if err := lines[p.i].err; err != nil {
			return nil, err
		}
		return nil, lineErr(lines[p.i].n, "unexpected indentation")
	}
	return root, nil
}

// uncomment is a line without its comment: from a # at its start or
// after a space, outside a quoted value.
func uncomment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote != 0 && c == quote:
			if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			quote = 0
		case quote != 0:
		case (c == '"' || c == '\'') && (i == 0 || strings.IndexByte(" \t[{,:", s[i-1]) >= 0):
			quote = c
		case c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return s[:i]
		}
	}
	return s
}

// yParser reads lines from the i-th; a block scalar reads raw, the
// file's lines without their line breaks, eol whether the last has one.
type yParser struct {
	raw   []string
	eol   bool
	lines []yLine
	i     int
}

func isItem(t string) bool { return t == "-" || strings.HasPrefix(t, "- ") }

// block is the mapping or list whose lines start at the parser's line,
// at indent.
func (p *yParser) block(indent int) (*yNode, error) {
	if isItem(p.lines[p.i].text) {
		return p.items(indent)
	}
	return p.mapping(indent)
}

func (p *yParser) items(indent int) (*yNode, error) {
	node := &yNode{kind: yList}
	for p.i < len(p.lines) {
		l := p.lines[p.i]
		if l.indent < indent || l.indent == indent && !isItem(l.text) {
			break
		}
		if l.err != nil {
			return nil, l.err
		}
		if l.indent > indent {
			return nil, lineErr(l.n, "unexpected indentation")
		}
		rest := strings.TrimLeft(strings.TrimPrefix(l.text, "-"), " ")
		if rest == "" {
			return nil, lineErr(l.n, "an empty list item")
		}
		if !strings.ContainsAny(rest[:1], `[{"'`) && (strings.Contains(rest, ": ") || strings.HasSuffix(rest, ":")) {
			return nil, lineErr(l.n, "a mapping in a list")
		}
		p.i++
		v, err := p.value(rest, l.n, indent)
		if err != nil {
			return nil, err
		}
		node.list = append(node.list, v)
	}
	return node, nil
}

func (p *yParser) mapping(indent int) (*yNode, error) {
	node := &yNode{kind: yMap, items: map[string]*yNode{}}
	for p.i < len(p.lines) {
		l := p.lines[p.i]
		if l.indent < indent {
			break
		}
		if l.err != nil {
			return nil, l.err
		}
		if l.indent > indent || isItem(l.text) {
			return nil, lineErr(l.n, "unexpected indentation")
		}
		key, rest, err := splitKey(l.text, l.n)
		if err != nil {
			return nil, err
		}
		if _, dup := node.items[key]; dup {
			return nil, lineErr(l.n, "%s: a key given twice", key)
		}
		p.i++
		v := &yNode{kind: yScalar, null: true}
		switch {
		case rest != "":
			v, err = p.value(rest, l.n, indent)
		case p.i < len(p.lines) && p.lines[p.i].indent > indent:
			v, err = p.block(p.lines[p.i].indent)
		case p.i < len(p.lines) && p.lines[p.i].indent == indent && isItem(p.lines[p.i].text):
			v, err = p.items(indent) // a list at its key's indentation
		}
		if err != nil {
			return nil, err
		}
		node.keys, node.items[key] = append(node.keys, key), v
	}
	return node, nil
}

// value is the value rest on line n, after a key or an item's dash at
// indent: a block scalar, read from the lines after it, or a value on
// its line.
func (p *yParser) value(rest string, n, indent int) (*yNode, error) {
	if rest[0] == '|' || rest[0] == '>' {
		return p.blockScalar(rest, n, indent)
	}
	return inlineValue(rest, n)
}

// blockScalar is the literal (|) or folded (>) block scalar whose header
// head ends line n, after a key or a dash at parent: the lines after it
// that are blank or indented past parent, at the first such line's
// indentation, as YAML reads them, chomped as head says (- strips the
// final line breaks, + keeps them, neither keeps one). The parser moves
// past them.
func (p *yParser) blockScalar(head string, n, parent int) (*yNode, error) {
	if len(head) > 2 || len(head) == 2 && head[1] != '-' && head[1] != '+' {
		return nil, lineErr(n, "%q: a block scalar header Daisy does not read (| or >, either alone or with - or +)", head)
	}
	folded, chomp := head[0] == '>', head[len(head)-1]
	spaces := func(r string) int { return len(r) - len(strings.TrimLeft(r, " ")) }
	// The lines: from the first after the header, each "" when blank (no
	// more than indent spaces), else its text past indent; the first
	// that is neither ends it.
	var text []string
	k, indent, blank := n, 0, 0
	for ; k < len(p.raw); k++ {
		r := p.raw[k]
		s := spaces(r)
		if s == len(r) {
			text, blank = append(text, ""), max(blank, s)
			continue
		}
		if s <= parent && r[s] == '\t' {
			return nil, lineErr(k+1, "a tab in the indentation")
		}
		if s > parent {
			// libyaml's, so Compose's: a leading blank line deeper than
			// the first sets the indentation, and that line ends the block.
			indent = max(s, blank)
		}
		break
	}
	for ; k < len(p.raw) && indent > 0; k++ {
		r := p.raw[k]
		s := spaces(r)
		switch {
		case s == len(r) && s <= indent:
			text = append(text, "")
			continue
		case s < indent && r[s] == '\t':
			return nil, lineErr(k+1, "a tab in the indentation")
		case s < indent:
		default:
			text = append(text, r[indent:])
			continue
		}
		break
	}
	for p.i < len(p.lines) && p.lines[p.i].n <= k {
		p.i++
	}
	// Join them: a line's break kept, each blank line a break of its own;
	// with > between two lines that begin with no blank, the break is
	// folded: a space, or none before blank lines.
	plain := func(t string) bool { return t[0] != ' ' && t[0] != '\t' }
	var b strings.Builder
	breaks, ended, prev := 0, false, ""
	for j, t := range text {
		broken := n+j < len(p.raw)-1 || p.eol // its line ends with a break
		if t == "" {
			if broken {
				breaks++
			}
			continue
		}
		switch {
		case prev == "":
		case !folded || !plain(prev) || !plain(t):
			b.WriteByte('\n')
		case breaks == 0:
			b.WriteByte(' ')
		}
		b.WriteString(strings.Repeat("\n", breaks))
		b.WriteString(t)
		breaks, ended, prev = 0, broken, t
	}
	if chomp != '-' && ended {
		b.WriteByte('\n')
	}
	if chomp == '+' {
		b.WriteString(strings.Repeat("\n", breaks))
	}
	return &yNode{kind: yScalar, value: b.String()}, nil
}

// splitKey is a block mapping line's key and the value after it, "" when
// none.
func splitKey(text string, n int) (key, rest string, err error) {
	if text[0] == '"' || text[0] == '\'' {
		k, end, err := quoted(text, 0, n)
		if err != nil {
			return "", "", err
		}
		after := text[end:]
		if after != ":" && !strings.HasPrefix(after, ": ") {
			return "", "", lineErr(n, "not a key: value")
		}
		return k, strings.TrimLeft(after[1:], " "), nil
	}
	if i := strings.Index(text, ": "); i > 0 {
		return text[:i], strings.TrimLeft(text[i+2:], " "), nil
	}
	if strings.HasSuffix(text, ":") && len(text) > 1 {
		return text[:len(text)-1], "", nil
	}
	return "", "", lineErr(n, "not a key: value")
}

// inlineValue is a value on its line: a one-line flow collection, a quoted
// value or a plain one.
func inlineValue(s string, n int) (*yNode, error) {
	switch s[0] {
	case '&', '*', '!':
		return nil, lineErr(n, "an anchor, alias or tag (&, *, !) is not read")
	case '[', '{':
		f := &flow{s: s, n: n}
		v, err := f.value()
		if err != nil {
			return nil, err
		}
		if f.i < len(s) {
			return nil, lineErr(n, "text after a flow collection")
		}
		return v, nil
	case '"', '\'':
		v, end, err := quoted(s, 0, n)
		if err != nil {
			return nil, err
		}
		if end < len(s) {
			return nil, lineErr(n, "text after a quoted value")
		}
		return &yNode{kind: yScalar, value: v}, nil
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return nil, lineErr(n, "a plain value holding \": \"; quote it")
	}
	return plain(s), nil
}

func plain(s string) *yNode {
	switch s {
	case "", "~", "null", "Null", "NULL":
		return &yNode{kind: yScalar, null: true}
	}
	return &yNode{kind: yScalar, value: s}
}

// quoted is the quoted value starting at s[i] and the index after it.
func quoted(s string, i, n int) (string, int, error) {
	q := s[i]
	if q == '\'' {
		var b strings.Builder
		for j := i + 1; j < len(s); j++ {
			if s[j] != '\'' {
				b.WriteByte(s[j])
				continue
			}
			if j+1 < len(s) && s[j+1] == '\'' {
				b.WriteByte('\'')
				j++
				continue
			}
			return b.String(), j + 1, nil
		}
		return "", 0, lineErr(n, "a quoted value not closed on its line")
	}
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			v, err := strconv.Unquote(s[i : j+1])
			if err != nil {
				return "", 0, lineErr(n, "%s: an escape Daisy does not read", s[i:j+1])
			}
			return v, j + 1, nil
		}
	}
	return "", 0, lineErr(n, "a quoted value not closed on its line")
}

// flow reads a one-line flow collection.
type flow struct {
	s    string
	i, n int
}

func (f *flow) space() {
	for f.i < len(f.s) && f.s[f.i] == ' ' {
		f.i++
	}
}

func (f *flow) open() error { return lineErr(f.n, "a flow collection not closed on its line") }

func (f *flow) value() (*yNode, error) {
	f.space()
	if f.i >= len(f.s) {
		return nil, f.open()
	}
	switch c := f.s[f.i]; c {
	case '&', '*', '!':
		return nil, lineErr(f.n, "an anchor, alias or tag (&, *, !) is not read")
	case '"', '\'':
		v, end, err := quoted(f.s, f.i, f.n)
		if err != nil {
			return nil, err
		}
		f.i = end
		return &yNode{kind: yScalar, value: v}, nil
	case '[':
		f.i++
		node := &yNode{kind: yList}
		for {
			f.space()
			if f.i < len(f.s) && f.s[f.i] == ']' {
				f.i++
				return node, nil
			}
			v, err := f.value()
			if err != nil {
				return nil, err
			}
			node.list = append(node.list, v)
			if err := f.next(']'); err != nil {
				return nil, err
			}
		}
	case '{':
		f.i++
		node := &yNode{kind: yMap, items: map[string]*yNode{}}
		for {
			f.space()
			if f.i < len(f.s) && f.s[f.i] == '}' {
				f.i++
				return node, nil
			}
			k, err := f.key()
			if err != nil {
				return nil, err
			}
			if _, dup := node.items[k]; dup {
				return nil, lineErr(f.n, "%s: a key given twice", k)
			}
			v := &yNode{kind: yScalar, null: true}
			f.space()
			if f.i < len(f.s) && f.s[f.i] != ',' && f.s[f.i] != '}' {
				if v, err = f.value(); err != nil {
					return nil, err
				}
			}
			node.keys, node.items[k] = append(node.keys, k), v
			if err := f.next('}'); err != nil {
				return nil, err
			}
		}
	}
	start := f.i
	for f.i < len(f.s) && !strings.ContainsRune(",]}", rune(f.s[f.i])) {
		f.i++
	}
	return plain(strings.TrimRight(f.s[start:f.i], " ")), nil
}

// next steps past the comma after an entry, or stops at close.
func (f *flow) next(close byte) error {
	f.space()
	switch {
	case f.i >= len(f.s):
		return f.open()
	case f.s[f.i] == ',':
		f.i++
	case f.s[f.i] != close:
		return lineErr(f.n, "%q where , or %c was expected", f.s[f.i], close)
	}
	return nil
}

// key is a flow mapping's key and the colon after it.
func (f *flow) key() (string, error) {
	if f.i >= len(f.s) {
		return "", f.open()
	}
	var k string
	if c := f.s[f.i]; c == '"' || c == '\'' {
		v, end, err := quoted(f.s, f.i, f.n)
		if err != nil {
			return "", err
		}
		k, f.i = v, end
	} else {
		start := f.i
		for f.i < len(f.s) && f.s[f.i] != ':' && f.s[f.i] != ',' && f.s[f.i] != '}' {
			f.i++
		}
		k = strings.TrimRight(f.s[start:f.i], " ")
	}
	if f.i >= len(f.s) || f.s[f.i] != ':' {
		if f.i >= len(f.s) {
			return "", f.open()
		}
		return "", lineErr(f.n, "%s: a key without a value", k)
	}
	f.i++
	return k, nil
}
