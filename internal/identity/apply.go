package identity

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"daisy/internal/ops"
)

// Record is the identity in its file format: what init keeps, in
// .daisy/gitconfig, of the identity it applied.
func (id Identity) Record() []byte {
	s := "name=" + id.Name + "\nemail=" + id.Email + "\n"
	if id.SSHKey != "" {
		s += "sshkey=" + id.SSHKey + "\n"
	}
	return []byte(s)
}

// Values are what a repository's local config holds for each of the three
// keys, every value in config order; a key that is unset is absent.
type Values map[string][]string

// Format is one `key="value"` line per value (Go-quoted, so a newline or a
// tab in a value survives), the keys in Keys order.
func (v Values) Format() []byte {
	var b strings.Builder
	for _, k := range Keys {
		for _, value := range v[k] {
			b.WriteString(k + "=" + strconv.Quote(value) + "\n")
		}
	}
	return []byte(b.String())
}

// ParseValues reads Format's output.
func ParseValues(b []byte) (Values, error) {
	v := Values{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		k, q, ok := strings.Cut(line, "=")
		value, err := strconv.Unquote(q)
		if !ok || err != nil || !isKey(k) {
			return nil, fmt.Errorf("not a line of git values: %q", line)
		}
		v[k] = append(v[k], value)
	}
	return v, nil
}

func isKey(k string) bool {
	for _, key := range Keys {
		if k == key {
			return true
		}
	}
	return false
}

// Snapshot is what the repository's local config holds for the three keys.
func Snapshot(o ops.Ops, repo string) (Values, error) {
	v := Values{}
	for _, k := range Keys {
		values, err := readValues(o, repo, k)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			v[k] = values
		}
	}
	return v, nil
}

// Restore makes the three keys hold exactly what v holds.
func Restore(o ops.Ops, repo string, v Values) error {
	for _, k := range Keys {
		if err := setValues(o, repo, k, v[k]); err != nil {
			return err
		}
	}
	return nil
}

// Apply sets the identity's keys, replacing every value a key may hold (a
// plain set refuses a multi-valued key). core.sshCommand only with a key.
func Apply(o ops.Ops, repo string, id Identity) error {
	for _, k := range Keys {
		value := id.Value(k)
		if value == "" {
			continue
		}
		if _, errb, err := o.Exec(repo, "git", "config", "--local", "--replace-all", k, value); err != nil {
			return fmt.Errorf("Error: could not set %s in the repository at %s: %s", k, repo, gitMessage(errb, err))
		}
	}
	return nil
}

// Unset takes back each key that is still the identity's — every value it
// holds is the claim's, or, for core.sshCommand, in Daisy's own form (only
// an init writes one) — and puts back what the key held before the
// identity was applied. A key holding anything else is the user's and is
// left whole. It returns the keys it took, in Keys order.
func Unset(o ops.Ops, repo string, claim Identity, before Values) (cleared []string, err error) {
	for _, k := range Keys {
		if !keyIsOurs(o, repo, k, claim.Value(k)) {
			continue
		}
		if _, errb, err := o.Exec(repo, "git", "config", "--local", "--unset-all", k); err != nil {
			return cleared, fmt.Errorf("Error: could not unset %s in the repository at %s: %s", k, repo, gitMessage(errb, err))
		}
		cleared = append(cleared, k)
		for _, value := range before[k] {
			if _, errb, err := o.Exec(repo, "git", "config", "--local", "--add", k, value); err != nil {
				return cleared, fmt.Errorf("Error: could not restore %s in the repository at %s: %s", k, repo, gitMessage(errb, err))
			}
		}
	}
	return cleared, nil
}

// readValues are a key's values; an unset key has none, and only a config
// git cannot read is an error.
func readValues(o ops.Ops, repo, key string) ([]string, error) {
	out, errb, err := o.Exec(repo, "git", "config", "--local", "--get-all", "-z", key)
	if err != nil {
		if len(out) == 0 && len(strings.TrimSpace(string(errb))) == 0 {
			return nil, nil // exit 1: the key is not set
		}
		return nil, fmt.Errorf("Error: could not read %s in the repository at %s: %s", key, repo, gitMessage(errb, err))
	}
	return splitValues(out), nil
}

func setValues(o ops.Ops, repo, key string, values []string) error {
	if held, err := readValues(o, repo, key); err != nil {
		return err
	} else if len(held) > 0 {
		if _, errb, err := o.Exec(repo, "git", "config", "--local", "--unset-all", key); err != nil {
			return fmt.Errorf("Error: could not unset %s in the repository at %s: %s", key, repo, gitMessage(errb, err))
		}
	}
	for _, value := range values {
		if _, errb, err := o.Exec(repo, "git", "config", "--local", "--add", key, value); err != nil {
			return fmt.Errorf("Error: could not set %s in the repository at %s: %s", key, repo, gitMessage(errb, err))
		}
	}
	return nil
}

func gitMessage(errb []byte, err error) string {
	if msg := strings.TrimSpace(string(errb)); msg != "" {
		return msg
	}
	return err.Error()
}

// IsRepoRoot: the directory is the top level of its own repository.
func IsRepoRoot(o ops.Ops, dir string) bool { return isRepoRoot(o, dir) }

// CommonDir is the resolved git directory every worktree of dir's
// repository shares; "" outside a repository.
func CommonDir(o ops.Ops, dir string) string { return gitPath(o, dir, "--git-common-dir") }

// IsLinkedWorktree: `git config --local` in dir writes another checkout's
// config, so an identity cannot be bound to dir alone.
func IsLinkedWorktree(o ops.Ops, dir string) bool {
	gitDir, common := gitPath(o, dir, "--git-dir"), CommonDir(o, dir)
	return gitDir != "" && common != "" && gitDir != common
}

// RepoHere: init may have applied an identity to dir — it is the top level
// of its own repository and not a linked worktree.
func RepoHere(o ops.Ops, dir string) bool { return IsRepoRoot(o, dir) && !IsLinkedWorktree(o, dir) }

func gitPath(o ops.Ops, dir, flag string) string {
	out, _, err := o.Exec(dir, "git", "rev-parse", flag)
	if err != nil {
		return ""
	}
	p := strings.TrimRight(string(out), "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	real, err := o.FS().EvalSymlinks(p)
	if err != nil {
		return ""
	}
	return real
}
