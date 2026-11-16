package declare

import (
	"fmt"
	"io"
	"path/filepath"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// SecretsFile is where the swarm's variables are assigned (section 3,
// Secret values): <state>/env, outside every repository.
func (f Forge) SecretsFile(local string) string {
	return filepath.Join(f.State(local), "env")
}

// ReadSecrets resolves each of names as `daisy check-secrets` resolves a
// variable: its assignment in the secrets file at path (read, never run,
// once, its problems reported to warn), else the environment. Only names
// are kept: every other assignment, the maintainer's token among them
// when it is not asked for, is dropped with the parse. A secrets file
// that a group or other may read is refused before it is read.
func ReadSecrets(o ops.Ops, path string, warn io.Writer, names ...string) (map[string]string, error) {
	vars, err := parseSecrets(o, path, warn)
	if err != nil {
		return nil, err
	}
	kept := make(map[string]string, len(names))
	for _, name := range names {
		v, ok := vars[name]
		if !ok {
			v = o.Getenv(name)
		}
		kept[name] = v
	}
	return kept, nil
}

// parseSecrets is the assignments of the secrets file at path, the
// swarm's <state>/env, nil when it is absent, refused when a group or
// other may read it. A line it cannot read past is named on warn by its
// path.
func parseSecrets(o ops.Ops, path string, warn io.Writer) (map[string]string, error) {
	fs := o.FS()
	fi, err := fs.Stat(path)
	if err != nil {
		return nil, nil
	}
	if fi.Mode().Perm()&0o044 != 0 {
		return nil, fmt.Errorf("%s is readable by others (mode %04o); run chmod 600 on it", path, fi.Mode().Perm())
	}
	b, err := fs.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	vars, problems := env.ParseShellEnv(b, o.Getenv)
	for _, p := range problems {
		fmt.Fprintf(warn, "⚠️  %s %s\n", path, p)
	}
	return vars, nil
}
