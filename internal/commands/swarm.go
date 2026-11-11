package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

// SwarmInit is `daisy swarm init [--defaults] [--api <url>]`
// (docs/swarm.md section 11): the wizard, `--api` question 3's default,
// the token's read check and account, swarm.json and the prompts, the
// labels, the readiness warnings and the unit files.
func SwarmInit(c *Context) error {
	if err := locate(c, true); err != nil {
		return err
	}
	return swarmInit(c, openForge)
}

// locate is every swarm verb's start (section 11, Finding the swarm): the
// root and the per-machine directories, as every command reads them, and
// the swarm the working folder is in. No Daisy workspace, home or
// identity is read.
func locate(c *Context, init bool) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	c.Env = m
	return find(c, init)
}

// inSwarm runs a verb other than init once locate has found its swarm.
func inSwarm(c *Context, run func() error) error {
	if err := locate(c, false); err != nil {
		return err
	}
	return run()
}

// find is c.Swarm, the swarm the working folder is in (declare.Find).
func find(c *Context, init bool) error {
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	p, err := declare.Find(c.Ops.FS(), cwd, init)
	var d *declare.Error
	if errors.As(err, &d) {
		return fmt.Errorf("Error: %v", d)
	}
	if err != nil {
		return err
	}
	c.Swarm = p
	return nil
}

// opener builds the forge adapter for a declaration and its token; tests
// pass one returning a forge.Fake.
type opener func(o ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error)

func openForge(o ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
	return forge.New(o, f.Kind, f.API, token, f.Repo, names)
}

func swarmInit(c *Context, open opener) error {
	o, p := c.Ops, c.Swarm
	var file *declare.Config
	if !p.Outside {
		existing, found, err := declare.Load(o.FS(), p.Root)
		if found && err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if found {
			file = &existing
		}
	}
	api, _ := c.Value("--api")
	w := &declare.Wizard{Ops: o, In: c.Stdin, Err: c.Stderr, Place: p, Defaults: c.Flag("--defaults"), File: file, API: api}
	cfg, err := w.Ask()
	if err == nil {
		err = declare.Validate(o, p, cfg, declare.SkipPrompts|declare.SkipAccount)
	}
	if err != nil {
		return cancelledOr(c, err)
	}
	return initRepository(c, w, cfg, open)
}

// repoNames is the declared repositories' names, in order.
func repoNames(cfg declare.Config) []string {
	var names []string
	for _, r := range cfg.Repos {
		names = append(names, r.Name)
	}
	return names
}

// readCheck is init's check that the swarm's token reads the swarm
// repository's issues and every declared repository's pull requests.
func readCheck(f forge.Adapter, cfg declare.Config) error {
	if _, err := f.Issues(forge.Open, forge.ProposalPosted); err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read %s: %v", cfg.Forge.TokenEnv, cfg.Forge.Repo, err)
	}
	if _, err := f.Pulls(0, nil); err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read the declared repositories: %v", cfg.Forge.TokenEnv, err)
	}
	return nil
}

// labelsLine is apply's report of EnsureLabels when it made a label.
func labelsLine(created, renamed int) string {
	return fmt.Sprintf("✓ Labels: %d present (%d created, %d renamed)", len(forge.Labels), created, renamed)
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

// secretsFile is where the swarm's variables are assigned (section 3,
// Secret values): <state>/env, outside every repository.
func secretsFile(c *Context, cfg declare.Config) string {
	return filepath.Join(cfg.Forge.State(c.Env.Local), "env")
}

// unsetErr is a needed variable found empty.
func unsetErr(c *Context, cfg declare.Config, name string) error {
	return fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", name, secretsFile(c, cfg))
}

// readSecrets resolves each of names as `daisy check-secrets` resolves a
// variable: its assignment in the secrets file (read, never run, once,
// its problems reported to warn), else the environment. Only names are
// kept: every other assignment, the maintainer's token among them when it
// is not asked for, is dropped with the parse. <state>/env that a group
// or other may read is refused before it is read.
func readSecrets(c *Context, cfg declare.Config, warn io.Writer, names ...string) (map[string]string, error) {
	vars, err := parseSecrets(c, secretsFile(c, cfg), warn)
	if err != nil {
		return nil, err
	}
	kept := make(map[string]string, len(names))
	for _, name := range names {
		v, ok := vars[name]
		if !ok {
			v = c.Ops.Getenv(name)
		}
		kept[name] = v
	}
	return kept, nil
}

// parseSecrets is the assignments of the secrets file at path, the
// swarm's <state>/env, nil when it is absent, refused when a group or
// other may read it. A line it cannot read past is named on warn by its
// path.
func parseSecrets(c *Context, path string, warn io.Writer) (map[string]string, error) {
	fs := c.Ops.FS()
	fi, err := fs.Stat(path)
	if err != nil {
		return nil, nil
	}
	if fi.Mode().Perm()&0o044 != 0 {
		return nil, fmt.Errorf("Error: swarm: %s is readable by others (mode %04o); run chmod 600 on it", path, fi.Mode().Perm())
	}
	b, err := fs.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	vars, problems := env.ParseShellEnv(b, c.Ops.Getenv)
	for _, p := range problems {
		fmt.Fprintf(warn, "⚠️  %s %s\n", path, p)
	}
	return vars, nil
}

// warnCloneEnv is apply's and status's warning of a .env.sh in the swarm
// repository's clone, which nothing reads (section 3, Secret values).
func warnCloneEnv(c *Context, cfg declare.Config) {
	p := filepath.Join(c.Swarm.Root, ".env.sh")
	if _, err := c.Ops.FS().Lstat(p); err == nil {
		fmt.Fprintf(c.Stderr, "⚠ %s: the swarm reads its secrets from %s, not the clone; move them there and delete it\n", p, secretsFile(c, cfg))
	}
}

// warnDrift is status's Drift (section 11): each path of the clone's
// swarm.json and prompts/ that differs from the swarm repository's
// default branch as the last fetch left its mirror, a line on stderr
// each time; nothing before a fetch. It changes no exit code.
func warnDrift(c *Context, cfg declare.Config) {
	d, err := reconcile.ConfigDrift(reconcile.Git{O: c.Ops, WS: c.Swarm.Root, State: cfg.Forge.State(c.Env.Local)}, cfg.Forge.Repo)
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ Drift: %v\n", err)
		return
	}
	for _, l := range d.Lines {
		fmt.Fprintln(c.Stderr, l)
	}
}

// writeDeclaration writes swarm.json when the answers differ from its
// values (its layout aside: a hand-formatted file with the same values is
// left as it is) and each persona prompt that is absent, each named
// relative to the swarm's root; written is what it wrote, so named.
func writeDeclaration(c *Context, at declare.Place, cfg declare.Config) (written []string, err error) {
	fs := c.Ops.FS()
	if err := fs.MkdirAll(filepath.Join(at.Root, "prompts"), 0o755); err != nil {
		return nil, fmt.Errorf("Error: cannot create %s: %v", filepath.Join(at.Root, "prompts"), err)
	}
	data := declare.Format(cfg)
	if old, found, err := declare.Load(fs, at.Root); found && err == nil && reflect.DeepEqual(old, cfg) {
		fmt.Fprintln(c.Stdout, "  Unchanged swarm.json")
	} else {
		if err := fs.WriteFile(declare.File(at.Root), data, 0o644); err != nil {
			return nil, fmt.Errorf("Error: cannot write %s: %v", declare.File(at.Root), err)
		}
		fmt.Fprintln(c.Stdout, "✓ Wrote swarm.json")
		written = append(written, "swarm.json")
	}
	for _, role := range declare.Roles {
		rel := filepath.Join("prompts", role+".md")
		w, err := writeAbsent(c, at.Root, rel, declare.DefaultPrompts[role])
		if err != nil {
			return nil, err
		}
		written = append(written, w...)
	}
	return written, nil
}

// writeAbsent writes content at rel under root when nothing is there,
// `✓ Wrote <rel>`, else names it kept; written is rel when it wrote it.
// An existing file is never overwritten.
func writeAbsent(c *Context, root, rel, content string) (written []string, err error) {
	fs := c.Ops.FS()
	p := filepath.Join(root, rel)
	if _, err := fs.Lstat(p); err == nil {
		fmt.Fprintf(c.Stdout, "  Kept %s (exists)\n", rel)
		return nil, nil
	}
	if err := fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, fmt.Errorf("Error: cannot create %s: %v", filepath.Dir(p), err)
	}
	if err := fs.WriteFile(p, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("Error: cannot write %s: %v", p, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Wrote %s\n", rel)
	return []string{rel}, nil
}

// writeRepoNotes writes, in the swarm repository's clone, repos/<name>.md
// for each declared repository where it is absent (section 4): its seed
// from the readiness facts, its instruction files read in its mirror at
// base. An existing file is never overwritten.
func writeRepoNotes(c *Context, at declare.Place, g reconcile.Git, cfg declare.Config) (written []string, err error) {
	for _, r := range cfg.Repos {
		var instructions []string
		for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
			if g.HasFile(r.Name, cfg.Base, name) {
				instructions = append(instructions, name)
			}
		}
		w, err := writeAbsent(c, at.Root, reconcile.RepoNotesFile(r.Name), reconcile.RepoNotesSeed(cfg, r, instructions))
		if err != nil {
			return nil, err
		}
		written = append(written, w...)
	}
	return written, nil
}

// readiness names, on stderr, what a product lacks for a swarm to improve
// it; never a refusal. Its instructions are read in its mirror at base.
func readiness(c *Context, g reconcile.Git, cfg declare.Config, swarmLogin string) {
	for _, r := range cfg.Repos {
		if !g.HasFile(r.Name, cfg.Base, "AGENTS.md") && !g.HasFile(r.Name, cfg.Base, "CLAUDE.md") {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n", r.Name)
		}
		if len(r.Check) == 0 {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s: check is empty; no member's work is executed (set it in swarm.json)\n", r.Name)
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

// printUnits prints the systemd user service that runs `swarm apply`'s
// loop in the swarm's root, its %f, and how to install it: restarted one
// interval after a failure, and a stop waiting one timeout, as long as a
// run may last, for the drain before systemd kills what is left.
func printUnits(c *Context, at declare.Place, interval, timeout string) error {
	daisy, err := c.Ops.Executable()
	if err != nil {
		return fmt.Errorf("Error: cannot find the running binary: %v", err)
	}
	d, _ := time.ParseDuration(interval) // validated
	t, _ := time.ParseDuration(timeout)  // validated
	fmt.Fprintf(c.Stdout, `
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: apply every %s for the swarm repository %%f

[Service]
WorkingDirectory=%%f
Environment=DAISY_ROOT=%s
ExecStart=%s swarm apply
Restart=on-failure
RestartSec=%s
KillMode=mixed
TimeoutStopSec=%s

[Install]
WantedBy=default.target

Install (your step):
  systemctl --user daemon-reload && systemctl --user enable --now %s
`, systemdSpan(d), c.Env.Root, daisy, systemdSpan(d), systemdSpan(t), installedUnit(at.Root))
	return nil
}

// installedUnit is the service's name on the install line, single-quoted
// when the escape holds a `\`, which the shell would eat.
func installedUnit(ws string) string {
	unit := "daisy-swarm@" + systemdEscapePath(ws) + ".service"
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
var SwarmStatus = readOnly(func(c *Context) error {
	if err := locate(c, false); err != nil {
		return err
	}
	return swarmStatus(c, openForge)
})

func swarmStatus(c *Context, open opener) error {
	o, ws := c.Ops, c.Swarm.Root
	cfg, err := loadSwarm(o, c.Swarm)
	if err != nil {
		return err
	}
	vars, err := readSecrets(c, cfg, c.Stderr, cfg.Forge.TokenEnv)
	if err != nil {
		return err
	}
	token := vars[cfg.Forge.TokenEnv]
	if token == "" {
		return unsetErr(c, cfg, cfg.Forge.TokenEnv)
	}
	warnCloneEnv(c, cfg)
	warnDrift(c, cfg)
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
	var opened, closed []forge.Issue
	for _, state := range []forge.State{forge.Open, forge.Closed} {
		for _, l := range forge.SwarmLabels(state) {
			issues, err := f.Issues(state, l)
			if err != nil {
				return fmt.Errorf("Error: swarm: %v", err)
			}
			for _, is := range issues {
				listed := func(in []forge.Issue) bool {
					return slices.ContainsFunc(in, func(c forge.Issue) bool { return c.Number == is.Number })
				}
				if state == forge.Closed {
					if !listed(closed) {
						closed = append(closed, is)
					}
					continue
				}
				stages[l] = append(stages[l], is.Number)
				if !listed(opened) { // an issue with two labels is listed under each
					opened = append(opened, is)
				}
			}
		}
	}
	recent, _ := reconcile.Recent(closed, o.Now())
	issues, err := readIssues(f, append(slices.Clone(opened), recent...), acct.Login, cfg.Trusted)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	// What waits for the maintainer: the open issues reconcile.Gated holds
	// for, the predicate apply flags them by, by item: a blocked one under
	// blocked, an approved one under what its pull request calls for
	// (section 12). What the maintainer holds: the paused ones, and those
	// whose newest maintainer comment names an open blocker.
	waits, classes := map[string][]int{}, map[int]string{}
	var paused, blockedBy []string
	for _, l := range forge.Stages {
		for _, n := range stages[l] {
			is := issues[n].Issue
			if stage, ok := reconcile.StageOf(is); !ok || stage != l {
				continue
			}
			if slices.Contains(is.Labels, forge.Paused) {
				paused = append(paused, "#"+strconv.Itoa(n))
			}
			if m := reconcile.BlockedBy(is, acct.Login, cfg.Trusted); m != 0 {
				held, err := reconcile.Blocker(f, n, m, c.Stderr)
				if err != nil {
					return fmt.Errorf("Error: swarm: %v", err)
				}
				if held {
					blockedBy = append(blockedBy, fmt.Sprintf("#%d blocked by #%d", n, m))
				}
			}
			class := reconcile.ClassOf(is, acct.Login, cfg.Trusted)
			if !reconcile.Gated(is.Labels, cfg.Gates, class) {
				continue
			}
			classes[n] = class
			item := map[string]string{forge.ProposalPosted: "triage", forge.PlanPosted: "plan"}[l]
			switch {
			case slices.Contains(is.Labels, forge.Blocked):
				item = "blocked"
			case l == forge.ReviewApproved:
				item = reconcile.AfterApproval(is, issues[n].Pulls, acct.Login)
			}
			waits[item] = append(waits[item], n)
		}
	}
	briefs, err := reconcile.Briefs(o, c.Swarm, cfg, cfg.Forge.State(c.Env.Local), nil)
	if err != nil {
		return fmt.Errorf("Error: swarm: reading the briefs: %v", err)
	}

	// The report is written whole at the end: a read failing on the way
	// leaves stdout empty, not a partial report before the error.
	var report bytes.Buffer
	out := &report
	state := cfg.Forge.State(c.Env.Local)
	fmt.Fprintf(out, "Swarm: %s on %s (%s), repositories %s\n", cfg.Forge.Repo, cfg.Forge.ForgeHost(), cfg.Forge.Kind, strings.Join(names, ", "))
	fmt.Fprintf(out, "State: %s%s\n", state, stateNotes(c.Ops.FS(), state, reconcile.NoActiveBrief(briefs)))
	fmt.Fprintf(out, "Gates: triage %s, plan %s, merge %s\n", onOff(cfg.Gates.Triage), onOff(cfg.Gates.Plan), onOff(cfg.Gates.Merge))
	for _, l := range forge.Stages {
		fmt.Fprintf(out, "  %-26s %d", strings.TrimPrefix(l, "swarm:"), len(stages[l]))
		if len(stages[l]) > 0 {
			fmt.Fprintf(out, "  %s", issueList(stages[l]))
		}
		fmt.Fprintln(out)
	}
	var waiting []string
	for _, item := range []string{"triage", "plan", "merge", "next slice", "close", "pull closed", "blocked"} {
		if len(waits[item]) > 0 {
			var ns []string
			for _, n := range waits[item] {
				// Only an architecture job waits at the triage and plan
				// gates: the class is named where it varies.
				if item == "triage" || item == "plan" {
					ns = append(ns, fmt.Sprintf("#%d", n))
				} else {
					ns = append(ns, fmt.Sprintf("#%d (%s)", n, classes[n]))
				}
			}
			waiting = append(waiting, item+" "+strings.Join(ns, " "))
		}
	}
	if len(waiting) == 0 {
		waiting = []string{"nothing"}
	}
	fmt.Fprintf(out, "Waiting for you: %s\n", strings.Join(waiting, "; "))
	held := blockedBy
	if len(paused) > 0 {
		held = append([]string{"paused " + strings.Join(paused, " ")}, blockedBy...)
	}
	if len(held) == 0 {
		held = []string{"nothing"}
	}
	fmt.Fprintf(out, "Held: %s\n", strings.Join(held, "; "))
	fmt.Fprintf(out, "Running: %s\n", running(o, ws, cfg.Forge.Repo))
	fmt.Fprintln(out, "Personas:")
	for _, r := range declare.Roles {
		p := cfg.Personas[r]
		fmt.Fprintf(out, "  %-12s %-10s %s %s\n", r, p.Provider, p.Model, p.Effort)
	}
	// The gates Daisy passed, per issue the larger of the forge's auto
	// notes (on the issues read) and <state>/auto's lines, so the count
	// keeps an issue gone from the forge's window.
	auto, err := reconcile.AutoRecorded(o.FS(), state)
	if err != nil {
		return fmt.Errorf("Error: swarm: reading %s: %v", filepath.Join(state, "auto"), err)
	}
	if auto == nil {
		auto = map[int][2]int{}
	}
	for n, si := range issues {
		t, p := reconcile.AutoPasses(si.Issue, acct.Login)
		auto[n] = [2]int{max(auto[n][0], t), max(auto[n][1], p)}
	}
	var autoTriage, autoPlan int
	for _, a := range auto {
		autoTriage, autoPlan = autoTriage+a[0], autoPlan+a[1]
	}
	if err := throughput(c, out, state, cfg.Budget, autoTriage, autoPlan); err != nil {
		return fmt.Errorf("Error: swarm: reading the runs: %v", err)
	}
	for _, n := range slices.Sorted(maps.Keys(issues)) {
		if line := countsLine(issues[n].Counts); line != "" {
			fmt.Fprintf(out, "  %-12s %s\n", "#"+strconv.Itoa(n), line)
		}
	}
	metRun, met, err := lastMet(o.FS(), state)
	if err != nil {
		return fmt.Errorf("Error: swarm: reading the runs: %v", err)
	}
	var active []string
	var milestones []forge.Milestone
	slugs := map[string]string{} // a slug → the active brief that has it, as apply mirrors them
	for _, b := range briefs {
		if b.Status != "active" {
			continue
		}
		if other, taken := slugs[b.Slug()]; taken {
			line := fmt.Sprintf("%s: shares the name %s with %s; mirrored to no milestone", b.Path, b.Slug(), other)
			if slices.Contains(met, b.Path) {
				line += fmt.Sprintf("; met (owner run %d)", metRun)
			}
			active = append(active, "  "+line+"\n")
			continue
		}
		slugs[b.Slug()] = b.Path
		if milestones == nil {
			if milestones, err = f.Milestones(); err != nil {
				return fmt.Errorf("Error: swarm: %v", err)
			}
		}
		active = append(active, "  "+briefLine(b, milestones, metRun, met)+"\n")
	}
	if len(active) == 0 {
		fmt.Fprintln(out, "Briefs: none active")
	} else {
		fmt.Fprint(out, "Briefs:\n"+strings.Join(active, ""))
	}
	_, err = c.Stdout.Write(report.Bytes())
	return err
}

// statusIssue is one issue as status reads it: the issue with its
// comments, its swarm pull requests and their correction counts.
type statusIssue struct {
	Issue  forge.Issue
	Pulls  []forge.Pull
	Counts reconcile.Counts
}

// readIssues reads each issue listed, its pull requests and their
// threads, by number.
func readIssues(f forge.Adapter, listed []forge.Issue, swarmLogin string, trusted []string) (map[int]statusIssue, error) {
	out := map[int]statusIssue{}
	for _, l := range listed {
		if _, done := out[l.Number]; done {
			continue
		}
		is, err := f.Issue(l.Number)
		if err != nil {
			return nil, err
		}
		pulls, err := f.Pulls(l.Number, reconcile.Openers(swarmLogin, trusted))
		if err != nil {
			return nil, err
		}
		threads := make([]reconcile.PullThread, len(pulls))
		for i, p := range pulls {
			ref := forge.Ref{Repo: p.Repo, N: p.Number}
			if threads[i].Comments, err = f.PullComments(ref); err == nil {
				if threads[i].Reviews, err = f.Reviews(ref); err == nil {
					threads[i].Lines, err = f.ReviewComments(ref)
				}
			}
			if err != nil {
				return nil, err
			}
		}
		out[l.Number] = statusIssue{is, pulls, reconcile.Count(is, pulls, threads, swarmLogin, trusted)}
	}
	return out, nil
}

// countsLine is an issue's line under Throughput, "" for an issue with no
// plan, no pull request and no block.
func countsLine(c reconcile.Counts) string {
	if c.Plans == 0 && c.Blocks == 0 && len(c.Pulls) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("plan revisions %d, blocks %d", c.Revisions(), c.Blocks)}
	for _, p := range c.Pulls {
		parts = append(parts, fmt.Sprintf("%s#%d rounds %d, maintainer comments %d", p.Repo, p.Number, p.Rounds, p.Maintainer))
	}
	return strings.Join(parts, "; ")
}

// briefLine is an active brief's line under Briefs: its path; each
// milestone's open and closed issues, as the forge counts them in the
// milestone that mirrors it (section 10), in the brief's heading order;
// how many of its milestones have no issue, and how many no milestone on
// the forge yet; and whether the newest batch run with an output reports
// it met.
func briefLine(b brief.Brief, milestones []forge.Milestone, metRun int, met []string) string {
	headings := b.Milestones()
	var parts []string
	none, absent := 0, 0
	for _, m := range headings {
		i := slices.IndexFunc(milestones, func(fm forge.Milestone) bool { return fm.Title == b.MilestoneTitle(m) })
		switch {
		case i < 0:
			absent++
		case milestones[i].Open+milestones[i].Closed == 0:
			none++
		default:
			parts = append(parts, fmt.Sprintf("%s (%d open, %d closed)", m, milestones[i].Open, milestones[i].Closed))
		}
	}
	if none > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d milestones with no issue", none, len(headings)))
	}
	if absent > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d milestones not on the forge", absent, len(headings)))
	}
	if slices.Contains(met, b.Path) {
		parts = append(parts, fmt.Sprintf("met (owner run %d)", metRun))
	}
	if len(parts) == 0 {
		return b.Path
	}
	return b.Path + ": " + strings.Join(parts, "; ")
}

// lastMet is the newest batch run whose out.md is not empty, by its
// number, and the briefs its Met: lines report met; 0 and none without
// such a run.
func lastMet(fs ops.FS, state string) (int, []string, error) {
	runs, err := reconcile.Runs(fs, state)
	if err != nil {
		return 0, nil, err
	}
	slices.SortFunc(runs, func(a, b reconcile.Run) int { return b.K - a.K })
	for _, r := range runs {
		if r.N != 0 {
			continue
		}
		out, err := r.Output(fs)
		if errors.Is(err, iofs.ErrNotExist) || err == nil && strings.TrimSpace(string(out)) == "" {
			continue
		}
		if err != nil {
			return 0, nil, err
		}
		met, _, _, _ := reconcile.Proposals(string(out))
		return r.K, met, nil
	}
	return 0, nil, nil
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
// paused, and why the owner's batch is held (section 2's two files, and
// noBrief, no brief under resources being active: section 6).
func stateNotes(fs ops.FS, state string, noBrief bool) string {
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
	if noBrief {
		notes += " — owner batch held: no brief is active"
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

// throughput writes section 11's Throughput block to out from
// <state>/runs/, and the gates Daisy passed for routine jobs as status
// counted them; no runs/ is no run, and a run without a result is being
// prepared.
func throughput(c *Context, out io.Writer, state string, budget *declare.Budget, autoTriage, autoPlan int) error {
	var runs, posted, blocked, failed, faults, uncosted, approved, atLimit, blockedThere int
	var today, all float64
	times := map[string][]time.Duration{} // run times by the stage launched on, "batch" and "check"
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
		var started, ended, checkStarted, checkEnded time.Time
		stage := "batch"
		cost, costed := 0.0, false
		var moves [][2]string // the run's relabels, by the stages' current names
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			f := strings.Fields(line)
			at := func(i int) time.Time { t, _ := time.Parse(time.RFC3339, f[i]); return t }
			switch {
			case len(f) == 2 && f[0] == "outcome":
				outcome = f[1]
			case len(f) == 2 && f[0] == "label":
				stage = strings.TrimPrefix(forge.Current(f[1]), "swarm:") // a run recorded before the rename by its old name
			case len(f) == 3 && f[0] == "relabelled":
				moves = append(moves, [2]string{forge.Current(f[1]), forge.Current(f[2])})
			case len(f) >= 2 && f[0] == "started":
				started = at(1)
			case len(f) == 3 && f[0] == "exited", len(f) == 2 && f[0] == "timeout":
				ended = at(len(f) - 1)
			case len(f) == 3 && f[0] == "check" && f[1] == "started":
				checkStarted = at(2)
			case len(f) == 4 && f[0] == "check" && f[1] == "exited":
				checkEnded = at(3)
			case len(f) >= 2 && f[0] == "cost":
				if v, err := strconv.ParseFloat(f[1], 64); err == nil {
					cost, costed = v, true
				}
			}
		}
		for _, span := range []struct {
			stage    string
			from, to time.Time
		}{{stage, started, ended}, {"check", checkStarted, checkEnded}} {
			if !span.from.IsZero() && !span.to.IsZero() && !span.to.Before(span.from) {
				times[span.stage] = append(times[span.stage], span.to.Sub(span.from))
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
		// A reviewer's moves: from review-0-ready (as a run recorded
		// before the in-progress stages names in-review) or
		// review-1-in-progress; the round limit's second time is its flag,
		// or, recorded before blocked was a flag, a move to it.
		for _, m := range moves {
			if m[0] != forge.ReviewReady && m[0] != forge.ReviewInProgress {
				continue
			}
			switch {
			case m[1] == forge.ReviewApproved:
				approved++
			case outcome == "posted" && (m[1] == forge.PlanPosted || m[1] == forge.ProposalAccepted):
				atLimit++
			case outcome == "posted" && m[1] == forge.Blocked:
				atLimit++
				blockedThere++
			}
		}
		if outcome == "posted" && slices.Contains(lines, "flagged "+forge.Blocked) {
			atLimit++
			blockedThere++
		}
	}
	fmt.Fprintln(out, "Throughput:")
	fmt.Fprintf(out, "  %-12s %d (%d posted, %d blocked, %d failed, %d faults, %d other)\n", "runs", runs, posted, blocked, failed, faults, runs-posted-blocked-failed-faults)
	fmt.Fprintf(out, "  %-12s $%.2f today, $%.2f in all (%d runs without a cost)\n", "cost", today, all, uncosted)
	if budget == nil {
		fmt.Fprintf(out, "  %-12s none\n", "budget")
	} else {
		fmt.Fprintf(out, "  %-12s $%.2f per day, $%.2f in all\n", "budget", budget.PerDay, budget.Total)
	}
	fmt.Fprintf(out, "  %-12s %d pull requests; at the round limit %d, blocked there %d\n", "approved", approved, atLimit, blockedThere)
	fmt.Fprintf(out, "  %-12s %d gates passed for routine jobs (triage %d, plan %d)\n", "auto", autoTriage+autoPlan, autoTriage, autoPlan)
	var medians []string
	for _, stage := range append(append([]string{"batch"}, forgeStages()...), "check") {
		if d := times[stage]; len(d) > 0 {
			medians = append(medians, fmt.Sprintf("%s %s (%d)", stage, median(d), len(d)))
		}
	}
	if len(medians) == 0 {
		medians = []string{"none"}
	}
	fmt.Fprintf(out, "  %-12s %s\n", "stage time", strings.Join(medians, ", "))
	return nil
}

// forgeStages is the nine stages without `swarm:`, in lifecycle order.
func forgeStages() []string {
	out := make([]string, len(forge.Stages))
	for i, l := range forge.Stages {
		out[i] = strings.TrimPrefix(l, "swarm:")
	}
	return out
}

// median is the middle of ds, or the mean of the two middle ones, to the
// second.
func median(ds []time.Duration) time.Duration {
	ds = slices.Clone(ds)
	slices.Sort(ds)
	m := ds[len(ds)/2]
	if len(ds)%2 == 0 {
		m = (ds[len(ds)/2-1] + m) / 2
	}
	return m.Round(time.Second)
}

// SwarmApply is `daisy swarm apply [--once]` (docs/swarm.md section 11):
// the start (validation, the credentials, the account, the host's uid and
// gid, Docker, <state> and its lock, the labels), then one pass with
// --once, or a pass every interval until SIGINT or SIGTERM.
func SwarmApply(c *Context) error {
	if err := locate(c, false); err != nil {
		return err
	}
	return swarmApply(c, openForge, openRunner)
}

// runnerOpener builds the runner, its docker run in dir, the swarm's
// root; tests pass one returning a runner.Fake.
type runnerOpener func(o ops.Ops, dir string) runner.Runner

func openRunner(o ops.Ops, dir string) runner.Runner { return runner.Docker{O: o, Dir: dir} }

// ErrInterrupted: a second signal ended `apply`'s drain (exit 130).
var ErrInterrupted = errors.New("interrupted")

func swarmApply(c *Context, open opener, openRun runnerOpener) error {
	o, ws := c.Ops, c.Swarm.Root
	once := c.Flag("--once")
	cfg, err := loadSwarm(o, c.Swarm)
	if err != nil {
		return err
	}
	unset := func(v string) error { return unsetErr(c, cfg, v) }
	used := map[string]bool{}
	for _, p := range cfg.Personas {
		used[p.Provider] = true
	}
	providers := slices.Sorted(maps.Keys(used))
	names := []string{cfg.Forge.TokenEnv}
	for _, p := range providers {
		if key := cfg.Keys[p]; key.Env != nil {
			names = append(names, *key.Env)
		}
	}
	vars, err := readSecrets(c, cfg, c.Stderr, names...) // its problems reported here, once
	if err != nil {
		return err
	}
	token := vars[cfg.Forge.TokenEnv]
	if token == "" {
		return unset(cfg.Forge.TokenEnv)
	}
	for _, p := range providers {
		if key := cfg.Keys[p]; key.Env != nil && vars[*key.Env] == "" {
			return unset(*key.Env)
		}
	}
	warnCloneEnv(c, cfg)
	// The lock before anything is read from the forge or Docker: a second
	// apply refuses having read nothing.
	state := cfg.Forge.State(c.Env.Local)
	if err := o.FS().MkdirAll(state, 0o755); err != nil {
		return fmt.Errorf("Error: swarm: cannot create %s: %v", state, err)
	}
	lock := filepath.Join(state, "lock")
	holder := "apply since "
	if once {
		holder = "apply --once since "
	}
	release, err := o.FS().Lock(lock, holder+o.Now().UTC().Format(time.RFC3339))
	var held *ops.LockedError
	if errors.As(err, &held) {
		return fmt.Errorf("Error: swarm apply is already running for %s (%s); lock: %s", cfg.Forge.Repo, held.Holder, lock)
	}
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	defer release()
	var repos []string
	for _, r := range cfg.Repos {
		repos = append(repos, r.Name)
	}
	f, err := open(o, cfg.Forge, repos, token)
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
	created, renamed, err := f.EnsureLabels(forge.Labels)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	if created+renamed > 0 {
		fmt.Fprintln(c.Stdout, labelsLine(created, renamed))
	}
	secret := func(v string) string {
		vars, _ := readSecrets(c, cfg, io.Discard, v) // re-read at each use, warned once above; refused, empty
		return vars[v]
	}
	s := &reconcile.Swarm{O: o, Place: c.Swarm, WS: ws, Local: c.Env.Local, Cfg: cfg, Forge: f, Runner: run, SwarmLogin: acct.Login,
		User: ids[0] + ":" + ids[1], Secret: secret, Out: c.Stdout, Err: c.Stderr}
	if once {
		if err := s.Pass(true); err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		return nil
	}
	return applyLoop(c, s)
}

// loadSwarm reads and validates swarm.json as apply's start and each pass
// do.
func loadSwarm(o ops.Ops, at declare.Place) (declare.Config, error) {
	cfg, _, err := declare.Load(o.FS(), at.Root)
	if err == nil {
		err = declare.Validate(o, at, cfg, 0)
	}
	if err != nil {
		return cfg, fmt.Errorf("Error: %v", err)
	}
	return cfg, nil
}

// applyLoop is `apply` without --once (section 11): a pass every
// interval, start to start, swarm.json re-read before each (a failure is
// printed and the pass skipped) and a pass's error printed, until SIGINT
// or SIGTERM; then no launch, the running members and checks collected as
// they exit, and exit 0, or at once on a second signal (130).
func applyLoop(c *Context, s *reconcile.Swarm) error {
	o := c.Ops
	signals := o.Signals()
	signalled := false
	// A signal during a pass ends its launching. The pass asks on this
	// goroutine, so a signal already in the channel is always seen.
	s.Stopping = func() bool {
		if !signalled {
			select {
			case <-signals:
				signalled = true
			default:
			}
		}
		return signalled
	}
	for !s.Stopping() {
		begin := o.Now()
		if cfg, err := loadSwarm(o, c.Swarm); err != nil {
			fmt.Fprintln(c.Stderr, err)
		} else if f, was := cfg.Forge, s.Cfg.Forge; f.Kind != was.Kind || f.API != was.API || f.Repo != was.Repo || f.TokenEnv != was.TokenEnv {
			// The lock, the adapter and the token are the start's: a pass
			// on another forge or repository would run on a <state> whose
			// lock it does not hold.
			fmt.Fprintf(c.Stderr, "Error: swarm: swarm.json's forge (kind, api, repo or token_env) changed since apply started; pass skipped: restart apply to act on it\n")
		} else {
			s.Cfg = cfg
			if err := s.Pass(false); err != nil {
				fmt.Fprintf(c.Stderr, "Error: swarm: %v\n", err)
			}
		}
		if signalled {
			break // signalled during the pass: drain now, not after the interval
		}
		interval, _ := time.ParseDuration(s.Cfg.Interval) // validated
		select {
		case <-signals:
			signalled = true
		case <-o.After(interval - o.Now().Sub(begin)):
		}
	}
	// A second signal ends the drain at once: one that came during the
	// pass already, before any wait, and one that comes while it waits.
	select {
	case <-signals:
		return ErrInterrupted
	default:
	}
	second, done, gone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(gone)
		select {
		case <-signals:
			close(second)
		case <-done:
		}
	}()
	err := s.Drain(second)
	close(done)
	<-gone // the watcher ends with the drain, taking no later signal
	if errors.Is(err, reconcile.ErrInterrupted) {
		return ErrInterrupted
	}
	if err != nil {
		fmt.Fprintf(c.Stderr, "Error: swarm: %v\n", err)
	}
	return nil
}

// swarmStateOnly is the <state> pause and resume act on: swarm.json read
// for its forge fields only (section 11).
func swarmStateOnly(c *Context) (string, error) {
	cfg, _, err := declare.Load(c.Ops.FS(), c.Swarm.Root)
	if err == nil {
		err = declare.ValidateForge(cfg.Forge)
	}
	if err != nil {
		return "", fmt.Errorf("Error: %v", err)
	}
	return cfg.Forge.State(c.Env.Local), nil
}

// pauseSwarm is `daisy swarm pause` (docs/swarm.md section 11):
// <state>/paused written, so the next pass of every apply launches nothing
// and merges nothing while collection goes on.
func pauseSwarm(c *Context) error {
	state, err := swarmStateOnly(c)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	path := filepath.Join(state, "paused")
	if b, err := fs.ReadFile(path); err == nil {
		fmt.Fprintf(c.Stdout, "Already paused since %s\n", strings.TrimPrefix(firstLine(b), "paused "))
		return nil
	}
	if err := fs.MkdirAll(state, 0o755); err != nil {
		return fmt.Errorf("Error: swarm: cannot create %s: %v", state, err)
	}
	if err := fs.WriteFile(path, []byte("paused "+c.Ops.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(c.Stdout, "Paused: no new member will launch (%s); running members finish and are posted\n", path)
	return nil
}

// resumeSwarm is `daisy swarm resume` (docs/swarm.md section 11):
// <state>/paused removed, whoever wrote it, and <state>/owner-held.
func resumeSwarm(c *Context) error {
	state, err := swarmStateOnly(c)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	removed := map[string]bool{}
	for _, name := range []string{"paused", "owner-held"} {
		path := filepath.Join(state, name)
		if _, err := fs.Lstat(path); err != nil {
			continue
		}
		if err := fs.Remove(path); err != nil {
			return fmt.Errorf("Error: swarm: %v", err)
		}
		removed[name] = true
	}
	if removed["paused"] {
		fmt.Fprintln(c.Stdout, "Resumed: members launch at the next pass")
	}
	if removed["owner-held"] {
		fmt.Fprintln(c.Stdout, "Owner batch released")
	}
	if len(removed) == 0 {
		fmt.Fprintln(c.Stdout, "Not paused")
	}
	return nil
}
