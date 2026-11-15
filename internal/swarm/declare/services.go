package declare

import (
	"errors"
	"fmt"
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
// environment (`<name>=<value>`, in the file's order for a list, by name
// for a mapping) and its command, none when absent.
type TestService struct {
	Name, Image         string
	Tmpfs, Env, Command []string
}

// serviceKeys is the subset of a Compose service Daisy runs.
var serviceKeys = []string{"image", "tmpfs", "environment", "command"}

var serviceShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ParseServices reads a test services file (section 3): YAML of the
// subset Daisy reads (block mappings and lists, one-line flow
// collections, plain and quoted scalars, comments), a mapping of exactly
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

type yLine struct {
	n, indent int
	text      string
}

func lineErr(n int, format string, args ...any) error {
	return fmt.Errorf("line %d: %s", n, fmt.Sprintf(format, args...))
}

// readYAML is data's one document, nil when it holds none.
func readYAML(data []byte) (*yNode, error) {
	var lines []yLine
	for i, raw := range strings.Split(string(data), "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		body := strings.TrimLeft(raw, " ")
		text := strings.TrimRight(uncomment(body), " \t")
		if text == "" {
			continue
		}
		n := i + 1
		switch {
		case strings.HasPrefix(body, "\t"):
			return nil, lineErr(n, "a tab in the indentation")
		case text == "---" && len(raw) == len(body):
			if len(lines) > 0 {
				return nil, lineErr(n, "a second document")
			}
			continue
		case strings.HasPrefix(text, "%"):
			return nil, lineErr(n, "a directive (%%) is not read")
		}
		lines = append(lines, yLine{n, len(raw) - len(body), text})
	}
	if len(lines) == 0 {
		return nil, nil
	}
	p := &yParser{lines: lines}
	root, err := p.block(lines[0].indent)
	if err != nil {
		return nil, err
	}
	if p.i < len(lines) {
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

type yParser struct {
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
		v, err := inlineValue(rest, l.n)
		if err != nil {
			return nil, err
		}
		node.list = append(node.list, v)
		p.i++
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
			v, err = inlineValue(rest, l.n)
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
	case '|', '>':
		return nil, lineErr(n, "a block scalar (| or >) is not read; write the value on its line")
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
