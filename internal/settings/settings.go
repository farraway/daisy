// Package settings edits the allow list of a Claude Code settings file
// (`permissions.allow`) as the bash's jq did: rules added at the end when
// missing, rules removed wherever they are, every other key and value kept
// in its place, the result indented two spaces with a final newline.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Allow removes the drop rules from the allow list, then appends each add
// rule it lacks. Empty input is an empty object.
func Allow(data []byte, add, drop []string) ([]byte, error) {
	top, err := parse(data)
	if err != nil {
		return nil, err
	}
	perms, allow, err := allowList(top)
	if err != nil {
		return nil, err
	}
	allow = without(allow, drop)
	for _, r := range add {
		if !contains(allow, r) {
			allow = append(allow, quote(r))
		}
	}
	perms.set("allow", array(allow))
	top.set("permissions", perms.compact())
	return top.indented()
}

// Disallow removes the rules; removed says whether any was there, and
// empty that nothing but an empty allow list is left, so the file can go.
func Disallow(data []byte, rules []string) (out []byte, removed, empty bool, err error) {
	top, err := parse(data)
	if err != nil {
		return nil, false, false, err
	}
	perms, allow, err := allowList(top)
	if err != nil {
		return nil, false, false, err
	}
	if _, ok := perms.vals["allow"]; ok {
		kept := without(allow, rules)
		removed, allow = len(kept) < len(allow), kept
		perms.set("allow", array(allow))
		top.set("permissions", perms.compact())
	}
	out, err = top.indented()
	onlyAllow := len(perms.keys) == 0 || len(perms.keys) == 1 && perms.keys[0] == "allow"
	onlyPerms := len(top.keys) == 0 || len(top.keys) == 1 && top.keys[0] == "permissions"
	return out, removed, len(allow) == 0 && onlyAllow && onlyPerms, err
}

// object is a JSON object that keeps its keys in order.
type object struct {
	keys []string
	vals map[string]json.RawMessage
}

func parse(data []byte) (*object, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	o, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func parseObject(data []byte) (*object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	o := &object{vals: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		o.set(tok.(string), v)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return o, nil
}

// set keeps a key's first position and its last value, as jq reads a
// duplicated key.
func (o *object) set(k string, v json.RawMessage) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *object) compact() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(quote(k))
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

func (o *object) indented() ([]byte, error) {
	var b bytes.Buffer
	if err := json.Indent(&b, o.compact(), "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// allowList is the permissions object (empty when absent) and its allow
// list's elements (none when absent).
func allowList(top *object) (*object, []json.RawMessage, error) {
	perms := &object{vals: map[string]json.RawMessage{}}
	if raw, ok := top.vals["permissions"]; ok {
		p, err := parseObject(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("permissions is %v", err)
		}
		perms = p
	}
	var allow []json.RawMessage
	if raw, ok := perms.vals["allow"]; ok {
		if err := json.Unmarshal(raw, &allow); err != nil {
			return nil, nil, errors.New("permissions.allow is not a list")
		}
	}
	return perms, allow, nil
}

func without(list []json.RawMessage, rules []string) []json.RawMessage {
	var kept []json.RawMessage
	for _, v := range list {
		if s, ok := str(v); ok && containsString(rules, s) {
			continue
		}
		kept = append(kept, v)
	}
	return kept
}

func contains(list []json.RawMessage, rule string) bool {
	for _, v := range list {
		if s, ok := str(v); ok && s == rule {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, r := range list {
		if r == s {
			return true
		}
	}
	return false
}

func str(v json.RawMessage) (string, bool) {
	var s string
	return s, json.Unmarshal(v, &s) == nil
}

func array(list []json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, v := range list {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(v)
	}
	b.WriteByte(']')
	return b.Bytes()
}

// quote is s as a JSON string, `<`, `>` and `&` left as they are (jq does).
func quote(s string) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}
