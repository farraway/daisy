package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/swarm/declare"
)

// CheckSecrets reports which paths and API tokens the workspace's .env.sh
// configures, and, in a swarm (docs/swarm.md section 11, Finding the
// swarm), the swarm's variables as its verbs resolve them, eight
// characters of each at most. The files are parsed, never run
// (env.ParseShellEnv); a variable a file does not assign is the
// environment's, as after `source`. A line it cannot read is named on
// stderr. Read-only.
var CheckSecrets = readOnly(checkSecrets)

var secretRows = []struct{ label, name string }{
	{"DAISY_ROOT", "DAISY_ROOT"},
	{"DAISY_HOME", "DAISY_HOME"},
	{"Webex API", "DAISY_SECRET_WEBEX_API_TOKEN"},
	{"Backstage", "DAISY_SECRET_BACKSTAGE_TOKEN"},
	{"JIRA API", "DAISY_SECRET_JIRA_API_TOKEN"},
	{"GitHub", "DAISY_SECRET_GITHUB_TOKEN"},
}

func checkSecrets(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	ws, wsErr := env.FindWorkspace(c.Ops.FS(), m.Root, cwd)
	at, swarmErr := declare.Find(c.Ops.FS(), cwd, false)
	if wsErr != nil && swarmErr != nil {
		return wsErr
	}
	out := c.Stdout
	fmt.Fprint(out, "Checking Daisy environment configuration...\n\n")
	failed := false
	if wsErr == nil {
		if !workspaceSecrets(c, ws) {
			if swarmErr != nil {
				return ErrReported
			}
			failed = true
		}
	}
	if swarmErr == nil {
		if wsErr == nil {
			fmt.Fprintln(out)
		}
		c.Env, c.Swarm = m, at
		failed = !swarmSecrets(c) || failed
	}
	fmt.Fprint(out, "\nNote: Only showing first 8 characters of tokens for security\n")
	if failed {
		return ErrReported
	}
	return nil
}

// workspaceSecrets is the report of <ws>/.env.sh; false when it is not a
// file, which it says on stdout.
func workspaceSecrets(c *Context, ws string) bool {
	out := c.Stdout
	path := filepath.Join(ws, ".env.sh")
	if fi, err := c.Ops.FS().Stat(path); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprint(out, "❌ .env.sh not found in workspace\n   Copy template: cp $DAISY_ROOT/templates/env.sh.template .env.sh\n")
		return false
	}
	b, err := c.Ops.FS().ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Error: cannot read %s: %v\n", path, err)
		return false
	}
	vars, problems := env.ParseShellEnv(b, c.Ops.Getenv)
	for _, p := range problems {
		fmt.Fprintf(c.Stderr, "⚠️  .env.sh %s\n", p)
	}
	for i, row := range secretRows {
		switch i {
		case 0:
			fmt.Fprintln(out, "Core paths:")
		case 2:
			fmt.Fprint(out, "\nAPI tokens:\n")
		}
		v, ok := vars[row.name]
		if !ok {
			v = c.Ops.Getenv(row.name)
		}
		secretRow(out, row.label, v)
	}
	return true
}

// swarmSecrets is the report of the swarm's variables (docs/swarm.md
// section 3, Secret values): forge.token_env's, each keys env's and
// DAISY_MAINTAINER_TOKEN, resolved as the swarm's verbs resolve them,
// from <state>/env, else the environment. swarm.json is read for
// its forge and keys alone; one that cannot be read is a warning, not a
// failure. False when <state>/env is refused, which it says on stdout.
func swarmSecrets(c *Context) bool {
	out := c.Stdout
	cfg, _, err := declare.Load(c.Ops.FS(), c.Swarm.Root)
	if err == nil {
		err = declare.ValidateForge(cfg.Forge)
	}
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ %v; the swarm's variables are not checked\n", err)
		return true
	}
	names := cfg.SecretNames()
	fmt.Fprintf(out, "Swarm %s (%s):\n", cfg.Forge.Repo, secretsFile(c, cfg))
	vars, err := readSecrets(c, cfg, c.Stderr, names...)
	if err != nil {
		fmt.Fprintf(out, "❌ %s\n", strings.TrimPrefix(err.Error(), "Error: swarm: "))
		return false
	}
	for _, name := range names {
		secretRow(out, name, vars[name])
	}
	return true
}

// secretRow is one line: the first eight characters of v, or that it is
// not configured.
func secretRow(out io.Writer, label, v string) {
	if v == "" {
		fmt.Fprintf(out, "⚠️  %s: Not configured\n", label)
		return
	}
	fmt.Fprintf(out, "✅ %s: %s...\n", label, string([]rune(v)[:min(8, len([]rune(v)))]))
}
