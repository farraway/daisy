package commands

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/swarm/runner"
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

// SwarmStatus is `daisy swarm status` (docs/swarm.md section 11): the
// swarm's stages, gates, whose turn it is, the running set, the personas,
// the throughput from runs/ and the active briefs, under ReadOnly (the
// forge's GETs and `docker ps` let through: section 14, item 31).
var SwarmStatus = readOnly(func(c *Context) error { return swarmStatus(c, openForge) })

func swarmStatus(c *Context, open opener) error {
	o, ws := c.Ops, c.Env.Workspace
	cfg, _, err := declare.Load(o.FS(), ws)
	if err == nil {
		err = declare.Validate(o, ws, cfg, 0)
	}
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	token := secret(c, ws, cfg.Forge.TokenEnv)
	if token == "" {
		return fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", cfg.Forge.TokenEnv, filepath.Join(ws, ".env.sh"))
	}
	var names []string
	for _, r := range cfg.Repos {
		names = append(names, r.Name)
	}
	f, err := open(o, cfg.Forge, names, token)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	acct, err := f.Login()
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	stages := map[string][]int{}
	for _, l := range forge.Labels {
		issues, err := f.Issues(forge.Open, l)
		if err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		for _, is := range issues {
			stages[l] = append(stages[l], is.Number)
		}
	}
	var planned []int // planning issues whose newest counted entry is the architect's
	if cfg.Gates.Plan {
		for _, n := range stages["swarm:planning"] {
			is, err := f.Issue(n)
			if err != nil {
				return fmt.Errorf("Error: swarm: %v", err)
			}
			e, ok := reconcile.NewestCounted(reconcile.IssueThread(is), acct.Login, cfg.Trusted)
			if m, swarm := reconcile.Swarms(e, acct.Login); ok && swarm && m.Role == "architect" {
				planned = append(planned, n)
			}
		}
	}
	briefs, err := brief.Find(o.FS(), ws, cfg.Resources)
	if err != nil {
		return fmt.Errorf("Error: swarm: reading the briefs: %v", err)
	}

	out := c.Stdout
	state := cfg.Forge.State(c.Env.Local)
	fmt.Fprintf(out, "Swarm: %s on %s (%s), repositories %s\n", cfg.Forge.Repo, cfg.Forge.ForgeHost(), cfg.Forge.Kind, strings.Join(names, ", "))
	fmt.Fprintf(out, "State: %s%s\n", state, stateNotes(c.Ops.FS(), state))
	fmt.Fprintf(out, "Gates: triage %s, plan %s, merge %s\n", onOff(cfg.Gates.Triage), onOff(cfg.Gates.Plan), onOff(cfg.Gates.Merge))
	for _, l := range forge.Labels {
		fmt.Fprintf(out, "  %-12s %d", strings.TrimPrefix(l, "swarm:"), len(stages[l]))
		if len(stages[l]) > 0 {
			fmt.Fprintf(out, "  %s", issueList(stages[l]))
		}
		fmt.Fprintln(out)
	}
	var waiting []string
	for _, w := range []struct {
		what string
		on   bool
		ns   []int
	}{{"triage", cfg.Gates.Triage, stages["swarm:proposed"]}, {"plan", true, planned}, {"merge", cfg.Gates.Merge, stages["swarm:approved"]}, {"blocked", true, stages["swarm:blocked"]}} {
		if w.on && len(w.ns) > 0 {
			waiting = append(waiting, w.what+" "+issueList(w.ns))
		}
	}
	if len(waiting) == 0 {
		waiting = []string{"nothing"}
	}
	fmt.Fprintf(out, "Waiting for you: %s\n", strings.Join(waiting, "; "))
	fmt.Fprintf(out, "Running: %s\n", running(o, ws, cfg.Forge.Repo))
	fmt.Fprintln(out, "Personas:")
	for _, r := range declare.Roles {
		p := cfg.Personas[r]
		fmt.Fprintf(out, "  %-12s %-10s %s %s\n", r, p.Provider, p.Model, p.Effort)
	}
	if err := throughput(c, state, cfg.Budget); err != nil {
		return fmt.Errorf("Error: swarm: reading the runs: %v", err)
	}
	var active []string
	for _, b := range briefs {
		if b.Status == "active" {
			active = append(active, "  "+b.Path+"\n")
		}
	}
	if len(active) == 0 {
		fmt.Fprintln(out, "Briefs: none active")
	} else {
		fmt.Fprint(out, "Briefs:\n"+strings.Join(active, ""))
	}
	return nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func issueList(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(s, " ")
}

// stateNotes is the State line's tail: since when and why the swarm is
// paused, and why the owner's batch is held (section 2's two files).
func stateNotes(fs ops.FS, state string) string {
	var notes string
	if b, err := fs.ReadFile(filepath.Join(state, "paused")); err == nil {
		notes += " — paused since " + strings.TrimPrefix(firstLine(b), "paused ")
	}
	if b, err := fs.ReadFile(filepath.Join(state, "owner-held")); err == nil {
		_, reason, found := strings.Cut(strings.TrimPrefix(firstLine(b), "held "), ": ")
		if !found {
			reason = firstLine(b)
		}
		notes += " — owner batch held: " + reason
	}
	return notes
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

// running is the Running line's value: this swarm's containers from
// `docker ps` as `apply` lists them, `#<n> <role>` by issue, or unknown
// with docker's message.
func running(o ops.Ops, ws, repo string) string {
	stdout, stderr, err := o.Exec(ws, "docker", "ps", "--filter", "label=daisy.swarm", "--format", `{{.Names}} {{.Label "daisy.swarm"}} {{.Label "daisy.credential"}}`)
	if err != nil {
		msg := firstLine(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return "unknown (docker: " + msg + ")"
	}
	type member struct {
		n    int
		role string
	}
	var ms []member
	for _, line := range strings.Split(string(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rest, ok := strings.CutPrefix(fields[1], repo+"#")
		num, role, slash := strings.Cut(rest, "/")
		n, err := strconv.Atoi(num)
		if ok && slash && err == nil {
			ms = append(ms, member{n, role})
		}
	}
	if len(ms) == 0 {
		return "none"
	}
	slices.SortStableFunc(ms, func(a, b member) int { return a.n - b.n })
	s := make([]string, len(ms))
	for i, m := range ms {
		s[i] = fmt.Sprintf("#%d %s", m.n, m.role)
	}
	return strings.Join(s, ", ")
}

// throughput prints section 11's Throughput block from <state>/runs/ only;
// no runs/ is no run, and a run without a result is being prepared.
func throughput(c *Context, state string, budget *declare.Budget) error {
	var runs, posted, blocked, failed, faults, uncosted, approved, atLimit, blockedThere int
	var today, all float64
	midnight := c.Ops.Now().UTC().Truncate(24 * time.Hour)
	entries, err := c.Ops.FS().ReadDir(filepath.Join(state, "runs"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := c.Ops.FS().ReadFile(filepath.Join(state, "runs", e.Name(), "result"))
		if errors.Is(err, iofs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		runs++
		var outcome string
		var started time.Time
		cost, costed := 0.0, false
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			f := strings.Fields(line)
			switch {
			case len(f) == 2 && f[0] == "outcome":
				outcome = f[1]
			case len(f) >= 2 && f[0] == "started":
				started, _ = time.Parse(time.RFC3339, f[1])
			case len(f) >= 2 && f[0] == "cost":
				if v, err := strconv.ParseFloat(f[1], 64); err == nil {
					cost, costed = v, true
				}
			}
		}
		switch outcome {
		case "posted":
			posted++
		case "blocked":
			blocked++
		case "failed":
			failed++
		case "fault":
			faults++
		}
		if !costed {
			uncosted++
		}
		all += cost
		if !started.Before(midnight) {
			today += cost
		}
		if slices.Contains(lines, "relabelled swarm:in-review swarm:approved") {
			approved++
		}
		if outcome == "posted" {
			for _, to := range []string{"planning", "ready", "blocked"} {
				if slices.Contains(lines, "relabelled swarm:in-review swarm:"+to) {
					atLimit++
					if to == "blocked" {
						blockedThere++
					}
				}
			}
		}
	}
	out := c.Stdout
	fmt.Fprintln(out, "Throughput:")
	fmt.Fprintf(out, "  %-12s %d (%d posted, %d blocked, %d failed, %d faults, %d other)\n", "runs", runs, posted, blocked, failed, faults, runs-posted-blocked-failed-faults)
	fmt.Fprintf(out, "  %-12s $%.2f today, $%.2f in all (%d runs without a cost)\n", "cost", today, all, uncosted)
	if budget == nil {
		fmt.Fprintf(out, "  %-12s none\n", "budget")
	} else {
		fmt.Fprintf(out, "  %-12s $%.2f per day, $%.2f in all\n", "budget", budget.PerDay, budget.Total)
	}
	fmt.Fprintf(out, "  %-12s %d pull requests; at the round limit %d, blocked there %d\n", "approved", approved, atLimit, blockedThere)
	return nil
}

// SwarmApply is `daisy swarm apply --once` (docs/swarm.md section 11):
// the start (validation, the credentials, the account, the host's uid and
// gid, Docker, <state> and its lock, the labels), then one pass, its runs
// waited for and collected.
func SwarmApply(c *Context) error { return swarmApply(c, openForge, openRunner) }

// runnerOpener builds the runner; tests pass one returning a runner.Fake.
type runnerOpener func(o ops.Ops, ws string) runner.Runner

func openRunner(o ops.Ops, ws string) runner.Runner { return runner.Docker{O: o, Dir: ws} }

func swarmApply(c *Context, open opener, openRun runnerOpener) error {
	if !c.Flag("--once") {
		return errors.New("Error: swarm apply: only --once is built (the loop joins with the lifecycle); run 'daisy swarm apply --once'")
	}
	o, ws := c.Ops, c.Env.Workspace
	cfg, err := loadSwarm(o, ws)
	if err != nil {
		return err
	}
	unset := func(v string) error {
		return fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", v, filepath.Join(ws, ".env.sh"))
	}
	token := secret(c, ws, cfg.Forge.TokenEnv)
	if token == "" {
		return unset(cfg.Forge.TokenEnv)
	}
	used := map[string]bool{}
	for _, p := range cfg.Personas {
		used[p.Provider] = true
	}
	for _, p := range slices.Sorted(maps.Keys(used)) {
		if key := cfg.Keys[p]; key.Env != nil && secret(c, ws, *key.Env) == "" {
			return unset(*key.Env)
		}
	}
	var names []string
	for _, r := range cfg.Repos {
		names = append(names, r.Name)
	}
	f, err := open(o, cfg.Forge, names, token)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	acct, err := f.Login()
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	var ids []string
	for _, flag := range []string{"-u", "-g"} {
		out, _, err := o.Exec(ws, "id", flag)
		if err != nil {
			return fmt.Errorf("Error: swarm: id %s: %v", flag, err)
		}
		ids = append(ids, strings.TrimSpace(string(out)))
	}
	run := openRun(o, ws)
	if _, err := run.Running(); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	state := cfg.Forge.State(c.Env.Local)
	if err := o.FS().MkdirAll(state, 0o755); err != nil {
		return fmt.Errorf("Error: swarm: cannot create %s: %v", state, err)
	}
	lock := filepath.Join(state, "lock")
	release, err := o.FS().Lock(lock, "apply --once since "+o.Now().UTC().Format(time.RFC3339))
	var held *ops.LockedError
	if errors.As(err, &held) {
		return fmt.Errorf("Error: swarm apply is already running for %s (%s); lock: %s", cfg.Forge.Repo, held.Holder, lock)
	}
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	defer release()
	if _, err := f.EnsureLabels(forge.Labels); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	s := &reconcile.Swarm{O: o, WS: ws, Local: c.Env.Local, Cfg: cfg, Forge: f, Runner: run, SwarmLogin: acct.Login,
		User: ids[0] + ":" + ids[1], Secret: func(v string) string { return secret(c, ws, v) }, Out: c.Stdout, Err: c.Stderr}
	if err := s.Pass(true); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	return nil
}

// loadSwarm reads and validates swarm.json as apply's start does.
func loadSwarm(o ops.Ops, ws string) (declare.Config, error) {
	cfg, _, err := declare.Load(o.FS(), ws)
	if err == nil {
		err = declare.Validate(o, ws, cfg, 0)
	}
	if err != nil {
		return cfg, fmt.Errorf("Error: %v", err)
	}
	return cfg, nil
}
