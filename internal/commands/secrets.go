package commands

import (
	"fmt"
	"path/filepath"

	"daisy/internal/env"
)

// CheckSecrets reports which paths and API tokens the workspace's .env.sh
// configures, eight characters of each at most. The file is parsed, never
// run (env.ParseShellEnv); a variable it does not assign is the
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
	root, err := env.Root(c.Ops)
	if err != nil {
		return err
	}
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	ws, err := env.FindWorkspace(c.Ops.FS(), root, cwd)
	if err != nil {
		return err
	}
	out := c.Stdout
	fmt.Fprint(out, "Checking Daisy environment configuration...\n\n")
	path := filepath.Join(ws, ".env.sh")
	if fi, err := c.Ops.FS().Stat(path); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprint(out, "❌ .env.sh not found in workspace\n   Copy template: cp $DAISY_ROOT/templates/env.sh.template .env.sh\n")
		return ErrReported
	}
	b, err := c.Ops.FS().ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", path, err)
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
		if v == "" {
			fmt.Fprintf(out, "⚠️  %s: Not configured\n", row.label)
		} else {
			fmt.Fprintf(out, "✅ %s: %s...\n", row.label, string([]rune(v)[:min(8, len([]rune(v)))]))
		}
	}
	fmt.Fprint(out, "\nNote: Only showing first 8 characters of tokens for security\n")
	return nil
}
