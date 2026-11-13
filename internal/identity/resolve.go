package identity

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Pair is one identity and the home it belongs to. Shadowed: the identity
// is an identities/ file named like a registered home, which HomeOf takes
// first, so the file is never resolved.
type Pair struct {
	Identity, Home string
	Shadowed       bool
}

// List is every identity on this machine: each usable registered home's
// own name, then its identities/ files, in registry and name order.
func List(fs ops.FS, m env.Env) []Pair {
	usable, _ := env.UsableHomes(fs, m.Root, m.HomesFile())
	registered := map[string]bool{}
	for _, h := range env.Homes(fs, m.HomesFile()) {
		registered[h.Name] = true
	}
	var pairs []Pair
	for _, h := range usable {
		pairs = append(pairs, Pair{Identity: h.Name, Home: h.Name})
		entries, _ := fs.ReadDir(filepath.Join(h.Path, "identities"))
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") && isFile(fs, filepath.Join(h.Path, "identities", e.Name())) {
				pairs = append(pairs, Pair{Identity: e.Name(), Home: h.Name, Shadowed: registered[e.Name()]})
			}
		}
	}
	return pairs
}

// FormatList renders pairs as `daisy identity list` and init's errors do.
func FormatList(pairs []Pair, indent string) string {
	var b strings.Builder
	for _, p := range pairs {
		fmt.Fprintf(&b, "%s%-14s %s", indent, p.Identity, p.Home)
		if p.Shadowed {
			fmt.Fprintf(&b, " (shadowed: the home %s takes this name)", p.Identity)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// HomeOf is the home an identity belongs to: the registered home of that
// name (its default identity), else the one home defining
// identities/<name>. Two homes defining it, none, a name that is not a
// name, or a registered home that cannot be checked are errors.
func HomeOf(o ops.Ops, m env.Env, name string) (string, error) {
	fs := o.FS()
	if !env.NameOK(name) {
		return "", fmt.Errorf("Error: '%s' is not a valid identity name (letters, digits, '.', '_', '-'; starting with a letter or digit)", name)
	}
	if _, ok := env.HomePath(fs, m.HomesFile(), name); ok {
		return name, nil
	}
	usable, problems := env.UsableHomes(fs, m.Root, m.HomesFile())
	if len(problems) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "Error: cannot resolve identity '%s' while a registered home cannot be checked for it:\n", name)
		for _, p := range problems {
			fmt.Fprintf(&b, "  home '%s' is registered at %s, which %s\n", p.Name, p.Path, p.Problem)
		}
		b.WriteString("  Fix the registration ('daisy home remove <name>', then 'daisy home add <name> /path/to/home') or restore the path, then re-run")
		return "", fmt.Errorf("%s", b.String())
	}
	var homes []string
	for _, h := range usable {
		if isFile(fs, filepath.Join(h.Path, "identities", name)) {
			homes = append(homes, h.Name)
		}
	}
	switch len(homes) {
	case 1:
		return homes[0], nil
	case 0:
		return "", fmt.Errorf("Error: '%s' is neither a registered home nor an identity of one\n  Identities on this machine (identity, home):\n%s  To add an identity: write name=, email= and optionally sshkey= to <home>/identities/%s\n  To create a new home: daisy home new %s",
			name, FormatList(List(fs, m), "    "), name, name)
	}
	return "", fmt.Errorf("Error: identity '%s' is defined in more than one home: %s\n  Remove or rename identities/%s in all but one of them", name, strings.Join(homes, ", "), name)
}

func isFile(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
