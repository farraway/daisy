package commands

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// SwarmInit is `daisy swarm init [--defaults]` (docs/swarm.md section
// 11): the wizard, the token's read check and account, swarm/swarm.json
// and the prompts, the labels, the readiness warnings and the unit files.
func SwarmInit(c *Context) error { return swarmInit(c, openForge) }

// opener builds the forge adapter for a declaration and its token; tests
// pass one returning a forge.Fake.
type opener func(o ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error)

func openForge(o ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
	return forge.New(o, f.Kind, f.API, token, f.Repo, names)
}

func swarmInit(c *Context, open opener) error {
	o, ws := c.Ops, c.Env.Workspace
	var file *declare.Config
	existing, found, err := declare.Load(o.FS(), ws)
	if found && err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	if found {
		file = &existing
	}
	w := &declare.Wizard{Ops: o, In: c.Stdin, Err: c.Stderr, WS: ws, Defaults: c.Flag("--defaults"), File: file}
	cfg, err := w.Ask()
	if err == nil {
		err = declare.Validate(o, ws, cfg, declare.SkipPrompts|declare.SkipAccount)
	}
	if err != nil {
		return cancelledOr(c, err)
	}

	tokenVar := cfg.Forge.TokenEnv
	token := secret(c, ws, tokenVar)
	if token == "" {
		return fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", tokenVar, filepath.Join(ws, ".env.sh"))
	}
	var names []string
	for _, r := range cfg.Repos {
		names = append(names, r.Name)
	}
	f, err := open(o, cfg.Forge, names, token)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	if _, err := f.Issues(forge.Open, "swarm:proposed"); err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read %s: %v", tokenVar, cfg.Forge.Repo, err)
	}
	if _, err := f.Pulls(0); err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read the declared repositories: %v", tokenVar, err)
	}
	acct, err := f.Login()
	if err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read its own account: %v", tokenVar, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Token $%s reads %s as %s\n", tokenVar, cfg.Forge.Repo, acct.Login)
	err = w.AskAccount(&cfg, acct.Login, acct.Email)
	if err == nil {
		err = declare.Validate(o, ws, cfg, declare.SkipPrompts)
	}
	if err != nil {
		return cancelledOr(c, err)
	}

	if err := writeDeclaration(c, ws, cfg); err != nil {
		return err
	}
	created, err := f.EnsureLabels(forge.Labels)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	fmt.Fprintf(c.Stdout, "✓ Labels: %d present (%d created)\n", len(forge.Labels), created)
	readiness(c, ws, cfg, acct.Login)
	return printUnits(c, ws, cfg.Interval)
}

// cancelledOr: EOF at a question cancels (exit 0, nothing written);
// anything else is the error.
func cancelledOr(c *Context, err error) error {
	if errors.Is(err, declare.ErrCancelled) {
		fmt.Fprintln(c.Stderr, err)
		return nil
	}
	var d *declare.Error
	if errors.As(err, &d) {
		return fmt.Errorf("Error: %v", d)
	}
	return err
}

// secret is a variable as `daisy check-secrets` resolves it: its
// assignment in <ws>/.env.sh (read, never run), else the environment.
func secret(c *Context, ws, name string) string {
	if b, err := c.Ops.FS().ReadFile(filepath.Join(ws, ".env.sh")); err == nil {
		vars, problems := env.ParseShellEnv(b, c.Ops.Getenv)
		for _, p := range problems {
			fmt.Fprintf(c.Stderr, "⚠️  .env.sh %s\n", p)
		}
		if v, ok := vars[name]; ok {
			return v
		}
	}
	return c.Ops.Getenv(name)
}

// writeDeclaration writes swarm.json when the answers differ from its
// values (its layout aside: a hand-formatted file with the same values is
// left as it is) and each persona prompt that is absent.
func writeDeclaration(c *Context, ws string, cfg declare.Config) error {
	fs := c.Ops.FS()
	if err := fs.MkdirAll(filepath.Join(ws, "swarm", "prompts"), 0o755); err != nil {
		return fmt.Errorf("Error: cannot create %s: %v", filepath.Join(ws, "swarm", "prompts"), err)
	}
	data := declare.Format(cfg)
	if old, found, err := declare.Load(fs, ws); found && err == nil && reflect.DeepEqual(old, cfg) {
		fmt.Fprintln(c.Stdout, "  Unchanged swarm/swarm.json")
	} else {
		if err := fs.WriteFile(declare.File(ws), data, 0o644); err != nil {
			return fmt.Errorf("Error: cannot write %s: %v", declare.File(ws), err)
		}
		fmt.Fprintln(c.Stdout, "✓ Wrote swarm/swarm.json")
	}
	for _, role := range declare.Roles {
		p := declare.PromptFile(ws, role)
		if _, err := fs.Lstat(p); err == nil {
			fmt.Fprintf(c.Stdout, "  Kept swarm/prompts/%s.md (exists)\n", role)
			continue
		}
		if err := fs.WriteFile(p, []byte(declare.DefaultPrompts[role]), 0o644); err != nil {
			return fmt.Errorf("Error: cannot write %s: %v", p, err)
		}
		fmt.Fprintf(c.Stdout, "✓ Wrote swarm/prompts/%s.md\n", role)
	}
	return nil
}

// readiness names, on stderr, what a product lacks for a swarm to improve
// it; never a refusal.
func readiness(c *Context, ws string, cfg declare.Config, swarmLogin string) {
	fs := c.Ops.FS()
	exists := func(p string) bool { _, err := fs.Stat(p); return err == nil }
	for _, r := range cfg.Repos {
		dir := filepath.Join(ws, r.Path)
		if !exists(filepath.Join(dir, "AGENTS.md")) && !exists(filepath.Join(dir, "CLAUDE.md")) {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n", r.Name)
		}
		if len(r.Check) == 0 {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s: check is empty; no member's work is executed (set it in swarm/swarm.json)\n", r.Name)
		}
	}
	if len(cfg.Resources) == 0 {
		fmt.Fprintln(c.Stderr, "⚠ Readiness: resources is empty; the owner proposes from the code alone")
	}
	maintainer := false
	for _, l := range cfg.Trusted {
		maintainer = maintainer || !strings.EqualFold(l, swarmLogin)
	}
	if !maintainer {
		fmt.Fprintln(c.Stderr, "⚠ Readiness: trusted holds no maintainer login; no comment of yours reaches a member")
	}
}

// printUnits prints the systemd user units that run `swarm apply --once`
// every interval in this workspace, and how to install them.
func printUnits(c *Context, ws, interval string) error {
	daisy, err := c.Ops.Executable()
	if err != nil {
		return fmt.Errorf("Error: cannot find the running binary: %v", err)
	}
	d, _ := time.ParseDuration(interval) // validated
	unit := installedUnit(ws)
	fmt.Fprintf(c.Stdout, `
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: one apply pass for the workspace %%f

[Service]
Type=oneshot
WorkingDirectory=%%f
Environment=DAISY_ROOT=%s
ExecStart=%s swarm apply --once

# ~/.config/systemd/user/daisy-swarm@.timer
[Unit]
Description=Daisy swarm: apply every %s for the workspace %%f

[Timer]
OnActiveSec=1min
OnUnitInactiveSec=%s

[Install]
WantedBy=timers.target

Install (your step):
  systemctl --user daemon-reload && systemctl --user enable --now %s
`, c.Env.Root, daisy, systemdSpan(d), systemdSpan(d), unit)
	return nil
}

// installedUnit is the timer's name on the install line, single-quoted
// when the escape holds a `\`, which the shell would eat.
func installedUnit(ws string) string {
	unit := "daisy-swarm@" + systemdEscapePath(ws) + ".timer"
	if strings.Contains(unit, `\`) {
		return "'" + unit + "'"
	}
	return unit
}

// systemdSpan is d in systemd's time-span notation: `5min`, `1h30min`,
// `90s` reads `1min30s`.
func systemdSpan(d time.Duration) string {
	var b strings.Builder
	for _, u := range []struct {
		unit string
		d    time.Duration
	}{{"h", time.Hour}, {"min", time.Minute}, {"s", time.Second}, {"ms", time.Millisecond}} {
		if n := d / u.d; n > 0 {
			fmt.Fprintf(&b, "%d%s", n, u.unit)
			d -= n * u.d
		}
	}
	return b.String()
}

// systemdEscapePath is `systemd-escape --path p`: the path without its
// outer slashes, each `/` a `-`, and every byte but ASCII letters, digits,
// `:`, `_` and a `.` that does not lead written `\xNN`.
func systemdEscapePath(p string) string {
	p = strings.Trim(filepath.Clean(p), "/")
	if p == "" {
		return "-"
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		ch := p[i]
		switch {
		case ch == '/':
			b.WriteByte('-')
		case ch == '.' && i == 0,
			!(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == ':' || ch == '_' || ch == '.'):
			fmt.Fprintf(&b, `\x%02x`, ch)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}
