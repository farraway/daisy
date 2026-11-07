// Package identity reads the git author a workspace is bound to: a file of
// `name=`, `email=` and optional `sshkey=` lines in a home, and the record
// init keeps of what it applied; resolving an identity name to its home;
// applying one to a repository and taking it back; and the drift status
// and files report between the record, the bound file and the repository.
package identity

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Identity is one git author.
type Identity struct{ Name, Email, SSHKey string }

// Keys are the three git keys an identity owns.
var Keys = []string{"user.name", "user.email", "core.sshCommand"}

// keyShape: the key must be one plain shell word, since git runs
// core.sshCommand through the shell (which is what expands `~`).
var keyShape = regexp.MustCompile(`^[A-Za-z0-9._/~@+-]+$`)

// sshIsDaisys matches exactly the core.sshCommand an init writes.
var sshIsDaisys = regexp.MustCompile(`^ssh -i [A-Za-z0-9._/~@+-]+ -o IdentitiesOnly=yes$`)

// SSHCommand is the core.sshCommand value for the key, "" without one.
func (id Identity) SSHCommand() string {
	if id.SSHKey == "" {
		return ""
	}
	return "ssh -i " + id.SSHKey + " -o IdentitiesOnly=yes"
}

// Value is what the identity gives a key; "" when it does not set it.
func (id Identity) Value(key string) string {
	switch key {
	case "user.name":
		return id.Name
	case "user.email":
		return id.Email
	case "core.sshCommand":
		return id.SSHCommand()
	}
	return ""
}

// Read parses an identity file. Name and email are required; the error
// says what is missing and names the file.
func Read(fs ops.FS, file string) (Identity, error) {
	b, err := fs.ReadFile(file)
	if err != nil {
		return Identity{}, fmt.Errorf("Error: identity file %s does not exist", file)
	}
	var id Identity
	first := func(key string) string {
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(line, key+"="); ok {
				return strings.ReplaceAll(v, "\r", "")
			}
		}
		return ""
	}
	id.Name, id.Email, id.SSHKey = first("name"), first("email"), first("sshkey")
	if id.SSHKey != "" && !keyShape.MatchString(id.SSHKey) {
		return Identity{}, &MalformedKeyError{File: file, Key: id.SSHKey}
	}
	if id.Name == "" || id.Email == "" {
		return Identity{}, fmt.Errorf("Error: identity file %s is incomplete: it needs both 'name=' and 'email=' lines", file)
	}
	return id, nil
}

// MalformedKeyError: the file's sshkey is not one plain shell word. A file
// with one is refused even where an incomplete one is not (a home's
// default identity at init).
type MalformedKeyError struct{ File, Key string }

func (e *MalformedKeyError) Error() string {
	return fmt.Sprintf("Error: identity file %s: sshkey '%s' must be a plain path (letters, digits, '.', '_', '/', '~', '@', '+', '-')", e.File, e.Key)
}

// configValues are every value a repository's local config holds for a key,
// in config order; nil when unset.
func configValues(o ops.Ops, repo, key string) []string {
	out, _, err := o.Exec(repo, "git", "config", "--local", "--get-all", "-z", key)
	if err != nil {
		return nil
	}
	return splitValues(out)
}

// splitValues splits `--get-all -z` output: each value ends with a NUL.
func splitValues(out []byte) []string {
	if len(out) == 0 {
		return nil
	}
	var values []string
	for _, v := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		values = append(values, string(v))
	}
	return values
}

// keyIsOurs: the repository holds the key and every value is the claim's,
// or, for core.sshCommand, in Daisy's own form.
func keyIsOurs(o ops.Ops, repo, key, claim string) bool {
	values := configValues(o, repo, key)
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if claim != "" && v == claim {
			continue
		}
		if key == "core.sshCommand" && sshIsDaisys.MatchString(v) {
			continue
		}
		return false
	}
	return true
}

// isRepoRoot: the directory is the top level of its own repository, where
// init applies an identity; elsewhere there is nothing applied to drift.
func isRepoRoot(o ops.Ops, dir string) bool {
	out, _, err := o.Exec(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top := strings.TrimRight(string(out), "\n")
	return top != "" && top == env.RealDir(o.FS(), dir)
}

// Drift says, one line each, where the workspace repository has drifted
// from what init wrote: a config value that is no longer the record's
// (.daisy/gitconfig; or, without a readable record, the bound identity
// file's), or a bound identity file whose values are not the record's.
// Nothing when all agree.
func Drift(o ops.Ops, repo, daisyDir, boundFile string) []string {
	fs := o.FS()
	record := filepath.Join(daisyDir, "gitconfig")
	var lines []string
	var claim Identity
	against := ""
	if id, err := Read(fs, record); err == nil {
		claim, against = id, "the record"
	} else if _, err := fs.Stat(record); err == nil {
		lines = append(lines, fmt.Sprintf("the identity record %s exists but cannot be read", record))
	} else if id, err := Read(fs, boundFile); err == nil {
		claim, against = id, "the bound identity"
	}

	switch {
	case against != "":
		if isRepoRoot(o, repo) {
			for _, key := range Keys {
				value := claim.Value(key)
				values := configValues(o, repo, key)
				sorted := append([]string(nil), values...)
				sort.Strings(sorted)
				held := strings.Join(uniq(sorted), ",")
				if value != "" {
					if held != value {
						if held == "" {
							held = "unset"
						}
						lines = append(lines, fmt.Sprintf("git %s is '%s' but %s says '%s'; the applied identity differs", key, held, against, value))
					}
				} else if key == "core.sshCommand" && held != "" && keyIsOurs(o, repo, key, "") {
					lines = append(lines, fmt.Sprintf("git core.sshCommand is '%s', in Daisy's form but not claimed by %s; 'daisy clean' or a re-init removes it", held, against))
				}
			}
		}
		if against == "the record" {
			if bound, err := Read(fs, boundFile); err == nil {
				if bound != claim {
					lines = append(lines, fmt.Sprintf("the bound identity file %s does not match what init recorded (edited since, or init did not finish)", boundFile))
				}
			} else {
				lines = append(lines, fmt.Sprintf("the bound identity file %s is missing or not a complete identity", boundFile))
			}
		}
	default:
		if _, err := fs.Stat(record); err != nil {
			lines = append(lines, fmt.Sprintf("the bound identity file %s is missing or not a complete identity; nothing claims the repository's git identity", boundFile))
			if isRepoRoot(o, repo) {
				for _, key := range Keys {
					if held := strings.Join(configValues(o, repo, key), ","); held != "" {
						lines = append(lines, fmt.Sprintf("git %s is '%s', and nothing on record claims it", key, held))
					}
				}
			}
		}
	}
	return lines
}

func uniq(sorted []string) []string {
	var out []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
