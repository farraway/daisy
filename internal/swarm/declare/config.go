// Package declare is `swarm/swarm.json` (docs/swarm.md section 3): its
// types, the strict parse, the layout `swarm init` writes, validation with
// errors naming their field, the wizard that asks for it, and the default
// persona prompts (section 4).
package declare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// Roles are the five personas, in the order the file and the wizard list them.
var Roles = []string{"owner", "architect", "worker", "adversary", "sympathetic"}

// MaintainerTokenEnv is the variable holding the maintainer's own token,
// which only the maintainer's verbs read (section 8, rule 5; section 11).
const MaintainerTokenEnv = "DAISY_MAINTAINER_TOKEN"

// Config is swarm.json. Its field order is section 3's, which is the
// order Format writes; a field without omitempty is required.
type Config struct {
	Forge      Forge              `json:"forge"`
	Repos      []Repo             `json:"repos"`
	Base       string             `json:"base"`
	MaxMembers int                `json:"max_members"`
	MaxRounds  int                `json:"max_rounds"`
	Timeout    string             `json:"timeout"`
	Interval   string             `json:"interval"`
	Owner      Owner              `json:"owner"`
	Resources  []string           `json:"resources"`
	Trusted    []string           `json:"trusted"`
	Gates      Gates              `json:"gates"`
	Budget     *Budget            `json:"budget,omitempty"`
	Keys       map[string]Key     `json:"keys"`
	Personas   map[string]Persona `json:"personas"`
}

// SwarmLogin is the swarm's own login as swarm.json names it: trusted's
// last, which `init` writes after the maintainer's logins (section 3); ""
// when trusted is empty, which validation refuses.
func (c Config) SwarmLogin() string {
	if len(c.Trusted) == 0 {
		return ""
	}
	return c.Trusted[len(c.Trusted)-1]
}

type Forge struct {
	Kind        string  `json:"kind"`
	API         string  `json:"api"`
	Repo        string  `json:"repo"`
	TokenEnv    string  `json:"token_env"`
	CommitEmail string  `json:"commit_email"`
	Board       *string `json:"board,omitempty"`
}

// Repo is one declared repository: the forge repository <owner>/<Name>,
// whose clone is at Path in the workspace.
type Repo struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Check     []string `json:"check"`
	Image     string   `json:"image"`
	Protected []string `json:"protected"`
}

type Owner struct {
	MaxOpenProposals int `json:"max_open_proposals"`
}

type Gates struct {
	Triage bool `json:"triage"`
	Plan   bool `json:"plan"`
	Merge  bool `json:"merge"`
}

type Budget struct {
	PerDay float64 `json:"per_day"`
	Total  float64 `json:"total"`
}

// Key is a provider's credential: exactly one of an env variable's name
// and a count of login slots (section 8).
type Key struct {
	Env   *string `json:"env,omitempty"`
	Slots *int    `json:"slots,omitempty"`
}

// Persona is one role's provider, model, effort and the Co-Authored-By
// trailer value its commits carry.
type Persona struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	CoAuthor string `json:"co_author"`
}

// Error is a swarm.json problem: the dotted path of the field and what is
// wrong with it. Its text is the message after `Error: `.
type Error struct{ Field, Problem string }

func (e *Error) Error() string {
	if e.Field == "" {
		return "swarm.json: " + e.Problem
	}
	return "swarm.json: " + e.Field + ": " + e.Problem
}

func fieldErr(field, format string, args ...any) *Error {
	return &Error{field, fmt.Sprintf(format, args...)}
}

// File is swarm.json's path in the workspace ws.
func File(ws string) string { return filepath.Join(ws, "swarm", "swarm.json") }

// Load reads and parses the workspace's swarm.json; found is false when
// there is none, err then saying to run init.
func Load(fs ops.FS, ws string) (c Config, found bool, err error) {
	b, err := fs.ReadFile(File(ws))
	if errors.Is(err, iofs.ErrNotExist) {
		return Config{}, false, &Error{"", fmt.Sprintf("not found at %s; run 'daisy swarm init'", File(ws))}
	}
	if err != nil {
		return Config{}, true, &Error{"", err.Error()}
	}
	c, err = Parse(b)
	return c, true, err
}

// Parse decodes data strictly: an unknown field, a required field absent
// and a value of the wrong type are each an *Error naming the field.
func Parse(data []byte) (Config, error) {
	// Syntax only: a value json.Unmarshal into any cannot hold (1e400)
	// is the field's to report, by decode, with its name.
	var probe any
	if err := json.Unmarshal(data, &probe); err != nil && !json.Valid(data) {
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			return Config{}, &Error{"", fmt.Sprintf("%s at byte %d", syn.Error(), syn.Offset)}
		}
		return Config{}, &Error{"", strings.TrimPrefix(err.Error(), "json: ")}
	}
	var c Config
	if err := decode("", data, reflect.ValueOf(&c).Elem()); err != nil {
		return Config{}, err
	}
	return c, nil
}

// decode fills v from raw, naming path in any error. A JSON null is
// absent for an optional (pointer) field and the wrong type for any other:
// encoding/json would leave the zero value, a gate silently off.
func decode(path string, raw json.RawMessage, v reflect.Value) error {
	if v.Kind() != reflect.Pointer && string(bytes.TrimSpace(raw)) == "null" {
		return fieldErr(path, "null is not %s", kindName(v.Kind()))
	}
	switch v.Kind() {
	case reflect.Pointer:
		if string(bytes.TrimSpace(raw)) == "null" {
			return nil
		}
		v.Set(reflect.New(v.Type().Elem()))
		return decode(path, raw, v.Elem())
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || obj == nil {
			return fieldErr(path, "not an object")
		}
		t := v.Type()
		var names []string
		for i := range t.NumField() {
			names = append(names, jsonName(t.Field(i)))
		}
		var unknown []string
		for k := range obj {
			if !slices.Contains(names, k) {
				unknown = append(unknown, k)
			}
		}
		if len(unknown) > 0 {
			slices.Sort(unknown)
			return fieldErr(join(path, unknown[0]), "unknown field")
		}
		for i := range t.NumField() {
			f := t.Field(i)
			r, ok := obj[names[i]]
			if !ok {
				if strings.Contains(f.Tag.Get("json"), "omitempty") {
					continue
				}
				return fieldErr(join(path, names[i]), "missing")
			}
			if err := decode(join(path, names[i]), r, v.Field(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) != nil {
			return fieldErr(path, "not an array")
		}
		s := reflect.MakeSlice(v.Type(), len(items), len(items))
		for i, r := range items {
			if err := decode(fmt.Sprintf("%s[%d]", path, i), r, s.Index(i)); err != nil {
				return err
			}
		}
		v.Set(s)
		return nil
	case reflect.Map:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || obj == nil {
			return fieldErr(path, "not an object")
		}
		m := reflect.MakeMapWithSize(v.Type(), len(obj))
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			e := reflect.New(v.Type().Elem()).Elem()
			if err := decode(join(path, k), obj[k], e); err != nil {
				return err
			}
			m.SetMapIndex(reflect.ValueOf(k), e)
		}
		v.Set(m)
		return nil
	}
	if err := json.Unmarshal(raw, v.Addr().Interface()); err != nil {
		return fieldErr(path, "%s", strings.TrimPrefix(err.Error(), "json: "))
	}
	return nil
}

// kindName is a value kind as section 3 names it.
func kindName(k reflect.Kind) string {
	switch k {
	case reflect.Bool:
		return "a boolean"
	case reflect.Slice:
		return "an array"
	case reflect.Struct, reflect.Map:
		return "an object"
	case reflect.String:
		return "a string"
	}
	return "a number"
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// Format is c as `swarm init` writes it (section 3): two-space indented,
// fields in section 3's order, `forge`, `repos` and `personas` expanded
// with each repository and each persona on one line (the personas'
// fields aligned), the other objects and arrays on one line each, and a
// trailing newline.
func Format(c Config) []byte {
	var b strings.Builder
	b.WriteString("{\n  \"forge\": {\n")
	forge := []string{
		`"kind": ` + inline(c.Forge.Kind),
		`"api": ` + inline(c.Forge.API),
		`"repo": ` + inline(c.Forge.Repo),
		`"token_env": ` + inline(c.Forge.TokenEnv),
		`"commit_email": ` + inline(c.Forge.CommitEmail),
	}
	if c.Forge.Board != nil {
		forge = append(forge, `"board": `+inline(*c.Forge.Board))
	}
	b.WriteString("    " + strings.Join(forge, ",\n    ") + "\n  },\n  \"repos\": [\n")
	var repos []string
	for _, r := range c.Repos {
		r.Check, r.Protected = nonNil(r.Check), nonNil(r.Protected)
		repos = append(repos, inline(r))
	}
	b.WriteString("    " + strings.Join(repos, ",\n    ") + "\n  ],\n")
	top := []string{
		`"base": ` + inline(c.Base),
		`"max_members": ` + inline(c.MaxMembers),
		`"max_rounds": ` + inline(c.MaxRounds),
		`"timeout": ` + inline(c.Timeout),
		`"interval": ` + inline(c.Interval),
		`"owner": ` + inline(c.Owner),
		`"resources": ` + inline(nonNil(c.Resources)),
		`"trusted": ` + inline(nonNil(c.Trusted)),
		`"gates": ` + inline(c.Gates),
	}
	if c.Budget != nil {
		top = append(top, `"budget": `+inline(*c.Budget))
	}
	top = append(top, `"keys": `+inline(c.Keys))
	for _, line := range top {
		b.WriteString("  " + line + ",\n")
	}
	b.WriteString("  \"personas\": {\n")
	b.WriteString(personaLines(c.Personas))
	b.WriteString("  }\n}\n")
	return []byte(b.String())
}

// personaLines is one line per role, its key and each field padded so the
// next field starts in the same column on every line.
func personaLines(personas map[string]Persona) string {
	var roles []string
	for _, r := range Roles {
		if _, ok := personas[r]; ok {
			roles = append(roles, r)
		}
	}
	cells := make([][]string, len(roles))
	width := make([]int, 5)
	for i, r := range roles {
		p := personas[r]
		cells[i] = []string{inline(r) + ":", `"provider": ` + inline(p.Provider) + ",", `"model": ` + inline(p.Model) + ",", `"effort": ` + inline(p.Effort) + ",", `"co_author": ` + inline(p.CoAuthor)}
		for j, c := range cells[i] {
			width[j] = max(width[j], len([]rune(c)))
		}
	}
	var b strings.Builder
	for i, row := range cells {
		b.WriteString("    ")
		for j, c := range row {
			switch j {
			case 0:
				b.WriteString(pad(c, width[0]) + " {")
			case len(row) - 1:
				b.WriteString(c + "}")
			default:
				b.WriteString(pad(c, width[j]) + " ")
			}
		}
		if i < len(cells)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pad(s string, w int) string { return s + strings.Repeat(" ", w-len([]rune(s))) }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// inline is v as one line of JSON with a space after each `:` and `,`
// outside strings, HTML characters unescaped.
func inline(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err) // every value Format passes is plain data
	}
	compact := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	var b strings.Builder
	inString, escaped := false, false
	for _, c := range compact {
		b.WriteByte(c)
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case !inString && (c == ':' || c == ','):
			b.WriteByte(' ')
		}
	}
	return b.String()
}
