package cmd

import (
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
	"daisy/internal/feedback"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/swarm/runner"
	"daisy/internal/verb"
)

// SwarmInit is `daisy swarm init [--defaults] [--api <url>]`
// (docs/swarm.md section 11): the wizard, `--api` question 3's default,
// the token's read check and account, swarm.json and the prompts, the
// labels, the readiness warnings and the unit file.
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

// ForgeFileEnv names a file holding a fake forge (forge.OpenFile), the
// test seam through which a run of the binary reaches the forge's verbs
// with no forge: set, it is the forge every verb opens, in place of the
// one swarm.json declares, which never names it, once the file holds the
// forge.repo and repositories swarm.json declares (section 10, The fake
// forge's file).
const ForgeFileEnv = "DAISY_FORGE"

func openForge(o ops.Ops, f declare.Forge, names []string, token string) (forge.Adapter, error) {
	if file := o.Getenv(ForgeFileEnv); file != "" {
		return forge.OpenFile(o, file, token, f.Repo, names)
	}
	return forge.New(o, f.Kind, f.API, token, f.Repo, names)
}

func swarmInit(c *Context, open opener) error {
	o, p := c.Ops, c.Swarm
	var file *declare.Config
	var dropped map[string]int
	if !p.Outside {
		// A retired field (co_author) is dropped: init writes the file anew.
		existing, gone, found, err := declare.LoadDropping(o.FS(), p.Root)
		if found && err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if found {
			file, dropped = &existing, gone
		}
	}
	api, _ := c.Value("--api")
	w := &declare.Wizard{Ops: o, In: c.Stdin, Err: c.Stderr, Place: p, Defaults: c.Flag("--defaults"), File: file, Dropped: dropped, API: api}
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

// secretsFile is where the swarm's variables are assigned
// (declare.Forge.SecretsFile).
func secretsFile(c *Context, cfg declare.Config) string {
	return cfg.Forge.SecretsFile(c.Env.Local)
}

// unsetErr is a needed variable found empty.
func unsetErr(c *Context, cfg declare.Config, name string) error {
	return fmt.Errorf("Error: swarm: $%s is not set (in %s or the environment)", name, secretsFile(c, cfg))
}

// readSecrets is declare.ReadSecrets of the swarm's secrets file.
func readSecrets(c *Context, cfg declare.Config, warn io.Writer, names ...string) (map[string]string, error) {
	vars, err := declare.ReadSecrets(c.Ops, secretsFile(c, cfg), warn, names...)
	if err != nil {
		return nil, fmt.Errorf("Error: swarm: %w", err)
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
// Each retired field dropped from the file (dropped) gets its line
// before swarm.json's.
func writeDeclaration(c *Context, at declare.Place, cfg declare.Config, dropped map[string]int) (written []string, err error) {
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
		for _, name := range slices.Sorted(maps.Keys(dropped)) {
			fmt.Fprintln(c.Stdout, declare.DroppedLine(name, dropped[name]))
		}
		fmt.Fprintln(c.Stdout, "✓ Wrote swarm.json")
		written = append(written, "swarm.json")
	}
	for _, role := range cfg.DeclaredRoles() {
		rel := filepath.Join("prompts", role+".md")
		w, err := writeAbsent(c, at.Root, rel, declare.DefaultPrompts[role])
		if err != nil {
			return nil, err
		}
		if len(w) == 0 {
			warnUnusablePrompt(c, at.Root, rel)
		}
		written = append(written, w...)
	}
	return written, nil
}

// warnUnusablePrompt warns of a prompt init kept that validation refuses
// (section 3, missing prompt): empty, a link to nothing, or unreadable.
func warnUnusablePrompt(c *Context, root, rel string) {
	b, err := c.Ops.FS().ReadFile(filepath.Join(root, rel))
	problem := ""
	switch {
	case errors.Is(err, iofs.ErrNotExist):
		problem = "is a link to nothing"
	case err != nil:
		problem = fmt.Sprintf("cannot be read (%v)", err)
	case len(b) == 0:
		problem = "is empty"
	default:
		return
	}
	fmt.Fprintf(c.Stderr, "⚠ %s %s; apply refuses it: write the role's prompt, or remove it and run 'daisy swarm prompt build' for the default\n", rel, problem)
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
// it; never a refusal. Its instructions and the comparison with its CI
// (section 7.9) are read in its mirror at base.
func readiness(c *Context, g reconcile.Git, cfg declare.Config, swarmLogin string) {
	ci, err := reconcile.CIDiffers(g, cfg)
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ Readiness: %v\n", err)
	}
	for _, r := range cfg.Repos {
		if !g.HasFile(r.Name, cfg.Base, "AGENTS.md") && !g.HasFile(r.Name, cfg.Base, "CLAUDE.md") {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s has no AGENTS.md or CLAUDE.md; members start without the product's instructions\n", r.Name)
		}
		if len(r.Check) == 0 {
			fmt.Fprintf(c.Stderr, "⚠ Readiness: %s: check is empty; no member's work is executed (set it in swarm.json)\n", r.Name)
		}
		if i := slices.IndexFunc(ci, func(d reconcile.CIDiffer) bool { return d.Name == r.Name }); i >= 0 {
			for _, l := range ci[i].Lines {
				fmt.Fprintln(c.Stderr, "⚠ Readiness: "+strings.TrimPrefix(l, "⚠ "))
			}
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

// printUnit prints the systemd user service that runs `swarm apply`'s
// loop in the swarm's root, its %f, and how to install it: restarted one
// interval after a failure, and a stop waiting one timeout, as long as a
// run may last, for the drain before systemd kills what is left. The
// root and the binary are quoted words (unitWord), the binary's `$`
// doubled, which ExecStart= would read as a variable.
func printUnit(c *Context, at declare.Place, interval, timeout string) error {
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
Environment=%s
ExecStart=%s swarm apply
Restart=on-failure
RestartSec=%s
KillMode=mixed
TimeoutStopSec=%s

[Install]
WantedBy=default.target

Install (your step): save the unit above as ~/.config/systemd/user/daisy-swarm@.service, then run
  systemctl --user daemon-reload && systemctl --user enable --now %s
`, systemdSpan(d), unitWord("DAISY_ROOT="+c.Env.Root), strings.ReplaceAll(unitWord(daisy), "$", "$$"), systemdSpan(d), systemdSpan(t), installedUnit(at.Root))
	return nil
}

// unitWord is s as one quoted word of a unit file: a \ and a " escaped
// with a \, and a % doubled, which systemd would read as a specifier.
func unitWord(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(s) + `"`
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

// SwarmStatus is `daisy swarm status [--json] [--waiting]` (docs/swarm.md
// section 11): the swarm's stages, gates, the running set, the personas,
// the throughput from runs/, the active briefs and, last, whose turn it
// is, under ReadOnly (the forge's GETs and `docker ps` let through:
// docs/swarm/decisions/0031-status-runs-under-a-widened-readonly.md); with --waiting, what waits for the maintainer
// alone.
func SwarmStatus(c *Context) error {
	if err := locate(c, false); err != nil {
		return err
	}
	return swarmStatus(c, openForge)
}

// WaitingFlag narrows `swarm status` to what waits for the maintainer.
const WaitingFlag = "--waiting"

// swarmStatus prints the report once every read is made, so a read
// failing on the way leaves stdout empty: as text, or with --json as one
// JSON document (docs/cli.md, --json); with --waiting, the report's
// waiting alone, in either form.
func swarmStatus(c *Context, open opener) error {
	read := func() (*statusReport, error) { return readStatus(c, open) }
	if c.Flag(WaitingFlag) {
		return verb.PrintReport(c, func() (*waitingReport, error) {
			r, err := read()
			if err != nil {
				return nil, err
			}
			return &waitingReport{Waiting: r.Waiting}, nil
		}, func(w *waitingReport) { fmt.Fprintln(c.Stdout, waitingLine(w.Waiting)) })
	}
	return verb.PrintReport(c, read, func(r *statusReport) { writeStatus(c.Stdout, r) })
}

// waitingReport is `swarm status --waiting`'s report: the full report's
// waiting under its key, and nothing the text does not print, so no key
// reads as a fact it does not hold (an empty running set, no stage).
type waitingReport struct {
	Waiting reconcile.StatusWaiting `json:"waiting"`
	verb.Warned
}

// statusReport is status's report, reconcile.Status; its warnings, under
// --json, are the lines status writes on stderr.
type statusReport struct {
	reconcile.Status
	verb.Warned
}

// readStatus makes every read status makes and returns the report: the
// declaration, the token and the forge here (openStatus), the rest
// reconcile.ReadStatus.
func readStatus(c *Context, open opener) (*statusReport, error) {
	cfg, f, err := openStatus(c, open)
	if err != nil {
		return nil, err
	}
	s, err := reconcile.ReadStatus(c.Ops, c.Swarm, cfg, c.Env.Local, f, c.Stderr)
	if err != nil {
		return nil, fmt.Errorf("Error: swarm: %v", err)
	}
	return &statusReport{Status: *s}, nil
}

// openStatus is what status reads before the forge, and the forge: the
// declaration, the swarm's token, the warnings on the clone's .env.sh and
// its drift, and the forge opened with the token.
func openStatus(c *Context, open opener) (declare.Config, forge.Adapter, error) {
	o := c.Ops
	cfg, err := loadSwarm(o, c.Swarm)
	if err != nil {
		return cfg, nil, err
	}
	vars, err := readSecrets(c, cfg, c.Stderr, cfg.Forge.TokenEnv)
	if err != nil {
		return cfg, nil, err
	}
	token := vars[cfg.Forge.TokenEnv]
	if token == "" {
		return cfg, nil, unsetErr(c, cfg, cfg.Forge.TokenEnv)
	}
	warnCloneEnv(c, cfg)
	warnDrift(c, cfg)
	f, err := open(o, cfg.Forge, repoNames(cfg), token)
	if err != nil {
		return cfg, nil, fmt.Errorf("Error: swarm: %v", err)
	}
	return cfg, f, nil
}

// writeStatus writes the report as text (docs/swarm.md section 11).
func writeStatus(out io.Writer, r *statusReport) {
	fmt.Fprintf(out, "Swarm: %s on %s (%s), repositories %s\n", r.Repository, r.Forge.Host, r.Forge.Kind, strings.Join(r.Repositories, ", "))
	fmt.Fprintf(out, "State: %s%s\n", r.State.Path, stateNotes(r.State))
	fmt.Fprintf(out, "Gates: triage %s, plan %s, merge %s\n", onOff(r.Gates.Triage), onOff(r.Gates.Plan), onOff(r.Gates.Merge))
	for _, l := range forge.Stages {
		ns := r.Stages[l]
		fmt.Fprintf(out, "  %-26s %d", strings.TrimPrefix(l, "swarm:"), len(ns))
		if len(ns) > 0 {
			fmt.Fprintf(out, "  %s", issueList(ns))
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "Running: %s\n", runningLine(r.Running))
	for _, q := range r.Queue { // section 12.1, What `status` shows
		line := fmt.Sprintf("Queue %s: %s at %s", q.Repository, q.State, q.SHA[:min(7, len(q.SHA))])
		var heads []string
		for _, h := range q.Heads {
			heads = append(heads, fmt.Sprintf("#%d %s", h.Issue, h.State))
		}
		if len(heads) > 0 {
			line += "; " + strings.Join(heads, ", ")
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "Personas:")
	for _, role := range declare.AllRoles {
		p, ok := r.Personas[role]
		if !ok {
			continue // the QA, undeclared
		}
		fmt.Fprintf(out, "  %-12s %-10s %s %s\n", role, p.Provider, p.Model, p.Effort)
		for _, b := range p.ByLabel {
			fmt.Fprintf(out, "    on %s: %s %s %s\n", b.Label, b.Provider, b.Model, b.Effort)
		}
	}
	writeCapacity(out, r.Capacity)
	writeThroughput(out, r.Throughput)
	for _, k := range r.Corrections {
		parts := []string{fmt.Sprintf("plan revisions %d, blocks %d", k.PlanRevisions, k.Blocks)}
		for _, p := range k.Pulls {
			parts = append(parts, fmt.Sprintf("%s#%d rounds %d, maintainer comments %d", p.Repo, p.Number, p.Rounds, p.MaintainerComments))
		}
		fmt.Fprintf(out, "  %-12s %s\n", "#"+strconv.Itoa(k.Issue), strings.Join(parts, "; "))
	}
	if len(r.Briefs) == 0 {
		fmt.Fprintln(out, "Briefs: none active")
	} else {
		fmt.Fprintln(out, "Briefs:")
		for _, b := range r.Briefs {
			fmt.Fprintln(out, "  "+briefLine(b))
		}
	}
	// What waits for the maintainer and what they hold come last, nearest
	// the prompt after a long report.
	fmt.Fprintln(out, waitingLine(r.Waiting))
	fmt.Fprintln(out, heldLine(r.Held))
}

// waitingLine is the Waiting for you line: the gated issues by item, then
// the notes' branches, then the repositories with no check, then those
// whose CI differs, or nothing.
func waitingLine(w reconcile.StatusWaiting) string {
	var waiting []string
	for i, ws := 0, w.Issues; i < len(ws); {
		item := ws[i].Item
		var ns []string
		for ; i < len(ws) && ws[i].Item == item; i++ {
			// Only an architecture job waits at the triage and plan
			// gates: the class is named where it varies.
			if item == "triage" || item == "plan" {
				ns = append(ns, fmt.Sprintf("#%d", ws[i].Issue))
			} else {
				ns = append(ns, fmt.Sprintf("#%d (%s)", ws[i].Issue, ws[i].Class))
			}
		}
		waiting = append(waiting, item+" "+strings.Join(ns, " "))
	}
	if len(w.Notes) > 0 {
		waiting = append(waiting, "notes "+strings.Join(w.Notes, " "))
	}
	if len(w.NoCheck) > 0 {
		waiting = append(waiting, "no check "+strings.Join(w.NoCheck, " "))
	}
	if len(w.CIDiffers) > 0 {
		waiting = append(waiting, "ci differs "+strings.Join(w.CIDiffers, " "))
	}
	if len(w.MainRed) > 0 {
		waiting = append(waiting, "main red "+strings.Join(w.MainRed, " "))
	}
	if len(w.MainUnchecked) > 0 {
		waiting = append(waiting, "main unchecked "+strings.Join(w.MainUnchecked, " "))
	}
	if len(waiting) == 0 {
		waiting = []string{"nothing"}
	}
	return "Waiting for you: " + strings.Join(waiting, "; ")
}

// heldLine is the Held line: the paused issues, then each held by an
// open dependency, then each first build held by an overlap (section
// 12.1), or nothing.
func heldLine(h reconcile.StatusHeld) string {
	var held []string
	if len(h.Paused) > 0 {
		held = append(held, "paused "+issueList(h.Paused))
	}
	for _, b := range h.BlockedBy {
		held = append(held, fmt.Sprintf("#%d blocked by #%d", b.Issue, b.By))
	}
	for _, b := range h.Overlaps {
		held = append(held, fmt.Sprintf("#%d overlaps #%d", b.Issue, b.By))
	}
	if len(held) == 0 {
		held = []string{"nothing"}
	}
	return "Held: " + strings.Join(held, "; ")
}

// briefLine is an active brief's line under Briefs: its path; each
// milestone's open and closed issues, as the forge counts them in the
// milestone that mirrors it (section 10), in the brief's heading order;
// how many of its milestones have no issue, and how many no milestone on
// the forge yet; and whether the newest batch run with an output reports
// it met. A brief another active brief's name has is mirrored to no
// milestone.
func briefLine(b reconcile.StatusBrief) string {
	met := ""
	if b.MetRun != nil {
		met = fmt.Sprintf("met (owner run %d)", *b.MetRun)
	}
	if b.SharesNameWith != nil {
		line := fmt.Sprintf("%s: shares the name %s with %s; mirrored to no milestone", b.Path, b.Name, *b.SharesNameWith)
		if met != "" {
			line += "; " + met
		}
		return line
	}
	var parts []string
	none, absent := 0, 0
	for _, m := range b.Milestones {
		switch {
		case !m.OnForge:
			absent++
		case m.Open+m.Closed == 0:
			none++
		default:
			parts = append(parts, fmt.Sprintf("%s (%d open, %d closed)", m.Heading, m.Open, m.Closed))
		}
	}
	if none > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d milestones with no issue", none, len(b.Milestones)))
	}
	if absent > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d milestones not on the forge", absent, len(b.Milestones)))
	}
	if met != "" {
		parts = append(parts, met)
	}
	if len(parts) == 0 {
		return b.Path
	}
	return b.Path + ": " + strings.Join(parts, "; ")
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

// stateNotes is the State line's tail.
func stateNotes(s reconcile.StatusState) string {
	var notes string
	if s.Paused != nil {
		notes += " — paused since " + s.Paused.Since
		if s.Paused.Reason != "" {
			notes += ": " + s.Paused.Reason
		}
	}
	if s.RestartRequestedAt != nil {
		notes += " — restart requested at " + *s.RestartRequestedAt
	}
	if s.OwnerHeld != nil {
		notes += " — owner batch held: " + s.OwnerHeld.Reason
		if len(s.OwnerHeld.Waits) > 0 {
			notes += " (released once " + reconcile.AreClosed(s.OwnerHeld.Waits) + ")"
		}
	}
	if s.NoActiveBrief {
		notes += " — owner batch held: no brief is active"
	}
	if s.QAHeld != nil {
		notes += " — qa held: " + s.QAHeld.Reason
	}
	return notes
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

// runningLine is the Running line's value: `#<n> <role>` by issue, none,
// or unknown with docker's message.
func runningLine(r reconcile.StatusRunning) string {
	if r.Error != nil {
		return "unknown (" + *r.Error + ")"
	}
	if len(r.Members) == 0 {
		return "none"
	}
	s := make([]string, len(r.Members))
	for i, m := range r.Members {
		s[i] = fmt.Sprintf("#%d %s", m.Issue, m.Role)
	}
	return strings.Join(s, ", ")
}

// runsLine is Throughput's runs line after its label.
func runsLine(r reconcile.StatusRuns) string {
	return fmt.Sprintf("%d (%d posted, %d blocked, %d questions, %d failed, %d faults, %d other)", r.Total, r.Posted, r.Blocked, r.Questions, r.Failed, r.Faults, r.Other)
}

// writeCapacity writes section 11's Capacity block (section 7.10, rule
// 3).
func writeCapacity(out io.Writer, c reconcile.StatusCapacity) {
	fmt.Fprintln(out, "Capacity:")
	for _, r := range c.Roles {
		line := fmt.Sprintf("  %-12s queue %d, running %d", onLabel(r.Role, r.Label), len(r.Queue), r.Running)
		if r.Why != nil {
			line += ": " + *r.Why
		}
		fmt.Fprintln(out, line)
	}
	for _, u := range c.Unavailable {
		fmt.Fprintln(out, "  "+u)
	}
	for _, t := range c.Tokens {
		var parts []string
		if t.PerDay != nil {
			parts = append(parts, fmt.Sprintf("%d of %d per day", t.Day, *t.PerDay))
		}
		if t.PerWeek != nil {
			parts = append(parts, fmt.Sprintf("%d of %d per week", t.Week, *t.PerWeek))
		}
		fmt.Fprintf(out, "  %s tokens %s\n", t.Credential, strings.Join(parts, ", "))
	}
	if b := c.Bottleneck; b != nil {
		fmt.Fprintf(out, "  Bottleneck: %s (%s)\n", onLabel(b.Role, b.Label), b.Why)
	} else {
		fmt.Fprintln(out, "  Bottleneck: none")
	}
}

// onLabel is a Capacity line's name: the role, and ` on <label>` for a
// by_label entry's line (section 7.10, The model by label).
func onLabel(role string, label *string) string {
	if label == nil {
		return role
	}
	return role + " on " + *label
}

// writeThroughput writes section 11's Throughput block.
func writeThroughput(out io.Writer, t reconcile.StatusThroughput) {
	fmt.Fprintln(out, "Throughput:")
	fmt.Fprintf(out, "  %-12s %s\n", "runs", runsLine(t.Runs))
	fmt.Fprintf(out, "  %-12s $%.2f today, $%.2f in all (%d runs without a cost); estimated $%.2f in all at list price (%d runs without an estimate)\n",
		"cost", t.Cost.Today, t.Cost.All, t.Cost.Uncosted, t.Cost.Estimated, t.Cost.Unestimated)
	if t.Budget == nil {
		fmt.Fprintf(out, "  %-12s none\n", "budget")
	} else {
		fmt.Fprintf(out, "  %-12s $%.2f per day, $%.2f in all\n", "budget", t.Budget.PerDay, t.Budget.Total)
	}
	fmt.Fprintf(out, "  %-12s %d pull requests; at the round limit %d, blocked there %d\n", "approved", t.Approved.PullRequests, t.Approved.AtRoundLimit, t.Approved.BlockedThere)
	fetched := "no mirror fetched"
	switch at := t.Reverted.Fetched; {
	case at != nil:
		fetched = "as of the mirrors' last fetch, " + *at
	case t.Reverted.MirrorsRead:
		fetched = "fetch time unknown"
	}
	fmt.Fprintf(out, "  %-12s %d of %d merged pull requests (%s)\n", "reverted", t.Reverted.RevertedPulls, t.Reverted.MergedPulls, fetched)
	fmt.Fprintf(out, "  %-12s %d gates passed for routine jobs (triage %d, plan %d)\n", "auto", t.Auto.Triage+t.Auto.Plan, t.Auto.Triage, t.Auto.Plan)
	var medians []string
	for _, stage := range reconcile.TimedStages() {
		if st, ok := t.StageTime[stage]; ok {
			medians = append(medians, fmt.Sprintf("%s %s (%d)", strings.TrimPrefix(stage, "swarm:"), time.Duration(st.MedianSeconds)*time.Second, st.Runs))
		}
	}
	if len(medians) == 0 {
		medians = []string{"none"}
	}
	fmt.Fprintf(out, "  %-12s %s\n", "stage time", strings.Join(medians, ", "))
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
	recordClone(c, state, ws)
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
	s := &reconcile.Swarm{O: o, Place: c.Swarm, WS: ws, Root: c.Env.Root, Config: c.Env.Config, Local: c.Env.Local, Cfg: cfg, Forge: f, Runner: run, SwarmLogin: acct.Login,
		User: ids[0] + ":" + ids[1], Secret: secret, Command: commentCommand(c, open, state), Out: c.Stdout, Err: c.Stderr}
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
// printed and the pass skipped) and a pass's error printed, each followed
// by the restart (restarter.restart), until SIGINT or SIGTERM, which ends
// the loop at its pass's end without a drain: the running members and
// checks run on, each container whose wait returned recorded, and the
// next apply holds them by their records (section 7.10, rule 2); exit 0.
func applyLoop(c *Context, s *reconcile.Swarm) error {
	o := c.Ops
	restart := newRestarter(c)
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
		if !signalled && restart != nil && restart.restart(c, s) {
			return nil // replaced by the exec: only a Fake's returns
		}
		if signalled {
			break // signalled during the pass or the restart: end now, not after the interval
		}
		interval, _ := time.ParseDuration(s.Cfg.Interval) // validated
		select {
		case <-signals:
			signalled = true
		case <-o.After(interval - o.Now().Sub(begin)):
		}
	}
	if err := s.Record(); err != nil {
		fmt.Fprintf(c.Stderr, "Error: swarm: %v\n", err)
	}
	return nil
}

// swarmStateOnly is the <state> pause and resume act on: swarm.json read
// for its forge fields only (section 11).
func swarmStateOnly(c *Context) (string, error) {
	cfg, err := forgeOnly(c)
	if err != nil {
		return "", err
	}
	return cfg.Forge.State(c.Env.Local), nil
}

// forgeOnly is swarm.json with only its forge fields validated.
func forgeOnly(c *Context) (declare.Config, error) {
	cfg, _, err := declare.Load(c.Ops.FS(), c.Swarm.Root)
	if err == nil {
		err = declare.ValidateForge(cfg.Forge)
	}
	if err != nil {
		return cfg, fmt.Errorf("Error: %v", err)
	}
	return cfg, nil
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
// <state>/paused removed, whoever wrote it, <state>/owner-held,
// <state>/qa-held, <state>/capacity (section 7.10) and
// <state>/main-unchecked (section 12.1, The red main).
func resumeSwarm(c *Context) error {
	state, err := swarmStateOnly(c)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	removed := map[string]bool{}
	for _, name := range []string{"paused", "owner-held", "qa-held", filepath.Base(reconcile.CapacityFile(state)), filepath.Base(reconcile.UncheckedFile(state))} {
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
	if removed["qa-held"] {
		fmt.Fprintln(c.Stdout, "QA released")
	}
	if removed["capacity"] {
		fmt.Fprintln(c.Stdout, "Capacity released: every model is tried again at the next pass")
	}
	if removed["main-unchecked"] {
		fmt.Fprintln(c.Stdout, "Main checks released: each unchecked base is checked again at the next pass")
	}
	if len(removed) == 0 {
		fmt.Fprintln(c.Stdout, "Not paused")
	}
	return nil
}

// SwarmMetrics is `daisy swarm metrics --backfill | --reprice
// [--dry-run] | --scorecard [--since <time>] [--until <time>]`
// (docs/swarm.md section 11):
// in the metrics CSV of the home <state>/home names, a row for each
// collected run it holds none for, or a row superseding each run's whose
// output now reads otherwise; or the scorecard read from it, writing
// nothing.
func SwarmMetrics(c *Context) error {
	return inSwarm(c, func() error { return swarmMetrics(c, openForge) })
}

func swarmMetrics(c *Context, open opener) error {
	backfill, reprice, scorecard, dryRun := c.Flag("--backfill"), c.Flag("--reprice"), c.Flag("--scorecard"), c.Flag("--dry-run")
	_, since := c.Value("--since")
	_, until := c.Value("--until")
	actions := 0
	for _, a := range []bool{backfill, reprice, scorecard} {
		if a {
			actions++
		}
	}
	switch {
	case actions != 1:
		return UsageError("give --backfill, --reprice or --scorecard: each is one action of the verb")
	case dryRun && !reprice:
		return UsageError("--dry-run goes with --reprice")
	case since && !scorecard:
		return UsageError("--since goes with --scorecard")
	case until && !scorecard:
		return UsageError("--until goes with --scorecard")
	}
	if scorecard {
		return swarmScorecard(c, open)
	}
	cfg, err := forgeOnly(c)
	if err != nil {
		return err
	}
	s := &reconcile.Swarm{O: c.Ops, Place: c.Swarm, WS: c.Swarm.Root, Root: c.Env.Root, Config: c.Env.Config, Local: c.Env.Local, Cfg: cfg, Out: c.Stdout, Err: c.Stderr}
	if reprice {
		return swarmReprice(c, s, dryRun)
	}
	b, err := s.Backfill()
	if err != nil {
		return fmt.Errorf("Error: swarm metrics --backfill: %v", err)
	}
	superseded := ""
	if b.Superseded > 0 {
		superseded = fmt.Sprintf("; %d earlier reviews' rows superseded with their review_effect", b.Superseded)
	}
	fmt.Fprintf(c.Stdout, "✓ Backfilled %d rows into %s (%d runs collected, %d with its row already)%s\n", b.Written, b.File, b.Collected, b.Had, superseded)
	return nil
}

// swarmReprice is `swarm metrics --reprice [--dry-run]`: with --dry-run,
// each row it would write, its usage cells from the held row's to its
// own, and nothing written.
func swarmReprice(c *Context, s *reconcile.Swarm, dryRun bool) error {
	p, err := s.Reprice(dryRun)
	if err != nil {
		return fmt.Errorf("Error: swarm metrics --reprice: %v", err)
	}
	counts := fmt.Sprintf("(%d runs collected, %d with a row: %d current, %d with no usage to read)", p.Collected, p.Held, p.Current, p.Unread)
	if !dryRun {
		fmt.Fprintf(c.Stdout, "✓ Repriced %d rows into %s %s\n", p.Written, p.File, counts)
		return nil
	}
	cell := func(v string) string {
		if v == "" {
			return "—"
		}
		return v
	}
	for _, ch := range p.Changes {
		f, t := ch.From, ch.To
		effect := ""
		if t.Role == "adversary" || t.Role == "sympathetic" {
			effect = fmt.Sprintf(", review_effect %s → %s", cell(f.ReviewEffect), cell(t.ReviewEffect))
		}
		if f.ConfigHash != t.ConfigHash {
			effect += fmt.Sprintf(", config_hash %s → %s", cell(f.ConfigHash), cell(t.ConfigHash))
		}
		fmt.Fprintf(c.Stdout, "  %s %s %s: tokens_in %s → %s, tokens_cached %s → %s, tokens_total %s → %s, cost_estimated %s → %s%s\n",
			t.Unit, t.Provider, t.Model, cell(f.TokensIn), cell(t.TokensIn), cell(f.TokensCached), cell(t.TokensCached),
			cell(f.TokensTotal), cell(t.TokensTotal), cell(f.CostEstimated), cell(t.CostEstimated), effect)
	}
	fmt.Fprintf(c.Stdout, "Would reprice %d rows in %s %s; nothing written\n", len(p.Changes), p.File, counts)
	return nil
}

// swarmScorecard is `swarm metrics --scorecard [--since <time>] [--until
// <time>]`, run read-only: a line per slice merged in the window, then
// the summary.
func swarmScorecard(c *Context, open opener) error {
	repo, sc, err := readScorecard(c, open)
	if err != nil {
		return err
	}
	writeScorecard(c.Stdout, repo, sc)
	return nil
}

// readScorecard reads the scorecard of the window the flags give,
// read-only, and the swarm's `<owner>/<repo>`.
func readScorecard(c *Context, open opener) (string, reconcile.Scorecard, error) {
	c.Ops = ops.ReadOnly(c.Ops)
	since, err := timeValue(c, "--since")
	if err != nil {
		return "", reconcile.Scorecard{}, err
	}
	until, err := timeValue(c, "--until")
	if err != nil {
		return "", reconcile.Scorecard{}, err
	}
	if !since.IsZero() && !until.IsZero() && until.Before(since) {
		s, _ := c.Value("--since")
		u, _ := c.Value("--until")
		return "", reconcile.Scorecard{}, UsageError(fmt.Sprintf("--until %s is before --since %s: the window would hold nothing", u, s))
	}
	cfg, err := forgeOnly(c)
	if err != nil {
		return "", reconcile.Scorecard{}, err
	}
	vars, err := readSecrets(c, cfg, c.Stderr, cfg.Forge.TokenEnv)
	if err != nil {
		return "", reconcile.Scorecard{}, err
	}
	token := vars[cfg.Forge.TokenEnv]
	if token == "" {
		return "", reconcile.Scorecard{}, unsetErr(c, cfg, cfg.Forge.TokenEnv)
	}
	f, err := open(c.Ops, cfg.Forge, repoNames(cfg), token)
	if err != nil {
		return "", reconcile.Scorecard{}, fmt.Errorf("Error: swarm: %v", err)
	}
	s := &reconcile.Swarm{O: c.Ops, Place: c.Swarm, WS: c.Swarm.Root, Root: c.Env.Root, Config: c.Env.Config, Local: c.Env.Local, Cfg: cfg, Forge: f, Out: c.Stdout, Err: c.Stderr}
	sc, err := s.Scorecard(since, until)
	if err != nil {
		return "", sc, fmt.Errorf("Error: swarm metrics --scorecard: %v", err)
	}
	return cfg.Forge.Repo, sc, nil
}

// timeValue is the RFC 3339 time a value flag gives, in UTC; zero when
// the flag is not given.
func timeValue(c *Context, flag string) (time.Time, error) {
	v, ok := c.Value(flag)
	if !ok {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return t, fmt.Errorf("Error: %s must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: %s)", flag, v)
	}
	return t.UTC(), nil
}

// writeScorecard writes the scorecard (docs/swarm.md section 11, The
// scorecard).
func writeScorecard(out io.Writer, repo string, sc reconcile.Scorecard) {
	fetched := "no mirror fetched"
	if !sc.Fetched.IsZero() {
		fetched = "the mirrors as last fetched, " + sc.Fetched.Format(time.RFC3339)
	}
	fmt.Fprintf(out, "Scorecard of %s: runs and interventions from %s, merges from %s\n", repo, sc.File, fetched)
	if len(sc.Slices) == 0 {
		fmt.Fprintln(out, "  no slice merged")
	}
	for _, r := range sc.Slices {
		from := "the previous slice's merge"
		if r.FromOpening {
			from = "the issue's opening"
		}
		fmt.Fprintf(out, "  #%d slice %d in %s, merged %s, %s after %s: runs %s; tokens %d uncached, %d cached, %d out%s; estimated $%.2f at list price%s; review rounds %d; stops %s; interventions %s; escapes %s; follow-up %s\n",
			r.N, r.Slice, r.Repo, r.At.Format(time.RFC3339), r.At.Sub(r.From).Round(time.Second), from,
			roleCounts(r.Runs), r.Uncached, r.Cached, r.Output, without(r.NoUsage, "usage"), r.Cost, without(r.Unpriced, "an estimate"),
			r.Rounds, keyCounts(r.Stops), keyCounts(r.Interventions), escapeParts(r.Escapes), followUpItems(r))
	}
	var bounds []string
	if !sc.Since.IsZero() {
		bounds = append(bounds, "since "+sc.Since.Format(time.RFC3339))
	}
	if !sc.Until.IsZero() {
		bounds = append(bounds, "until "+sc.Until.Format(time.RFC3339))
	}
	window := "every record"
	if len(bounds) > 0 {
		window = strings.Join(bounds, " ")
	}
	sm := sc.Summary
	days := "no record"
	if !sc.Start.IsZero() {
		days = fmt.Sprintf("%.1f days", sc.Days())
	}
	perMerge := func(n int, format string) string {
		if sm.Merges == 0 {
			return "—"
		}
		return fmt.Sprintf(format, float64(n)/float64(sm.Merges))
	}
	stopRate := "—"
	if sm.WorkerRuns > 0 {
		stopRate = fmt.Sprintf("%.0f%%", 100*float64(sm.WorkerStops)/float64(sm.WorkerRuns))
	}
	perDay := "—"
	if d := sc.Days(); !sc.Start.IsZero() && d > 0 {
		perDay = fmt.Sprintf("%.1f", float64(sm.Interventions)/d)
	}
	fmt.Fprintf(out, "Summary, %s (%s): %d slices merged; %d runs, %s per merge; input %s tokens per merge; worker stop rate %s (%d of %d worker runs); %d interventions, %s a day; "+
		"escapes %d on %d merged slices (%d reverted, %d filed, %d fixed by hand); follow-up items %d, %d proved real; blocking findings by kind %s\n",
		window, days, sm.Merges, sm.Runs, perMerge(sm.Runs, "%.1f"), perMerge(sm.Input, "%.0f"), stopRate, sm.WorkerStops, sm.WorkerRuns, sm.Interventions, perDay,
		sm.Escapes.Count(), sm.Merges, sm.Escapes.Reverted, sm.Escapes.Filed, sm.Escapes.ByHand, sm.Items, sm.ProvedReal, blockingKinds(sm.Blocking))
	fmt.Fprintf(out, "By configuration, %s:\n", window)
	if len(sc.Configs) == 0 {
		fmt.Fprintln(out, "  no run")
	}
	dash := func(v string) string {
		if v == "" {
			return "—"
		}
		return v
	}
	for _, g := range sc.Configs {
		hash := "unknown"
		if g.ConfigHash != "" {
			hash = g.ConfigHash[:min(12, len(g.ConfigHash))]
		}
		effect := ""
		if g.Role == "adversary" || g.Role == "sympathetic" {
			effect = fmt.Sprintf("; review effect %d (%d of %d runs known); merged heads approved %d, escaped %d; follow-up items raised %d, %d proved real",
				g.Effect, g.Known, g.Runs, g.Approved, g.Escaped, g.Raised, g.RaisedReal)
		}
		fmt.Fprintf(out, "  %s %s %s %s, config %s: runs %d; tokens %d uncached, %d cached, %d out%s; estimated $%.2f at list price%s; stops %d%s\n",
			g.Role, dash(g.Provider), dash(g.Model), dash(g.Effort), hash, g.Runs, g.Uncached, g.Cached, g.Output, without(g.NoUsage, "usage"),
			g.Cost, without(g.Unpriced, "an estimate"), g.Stops, effect)
	}
}

// blockingKinds is the window's blocking findings, `<kind> <k>, …` in
// the kinds' order, `none` for none.
func blockingKinds(kinds map[string]int) string {
	var parts []string
	for _, k := range reconcile.BlockingKinds {
		if n := kinds[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, n))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// escapeParts is a slice's escapes, `<e>` and, when it has any, ` (<the
// parts it holds>)` of `reverted`, `<b> filed` and `<h> fixed by hand`.
func escapeParts(e reconcile.Escapes) string {
	var parts []string
	if e.Reverted {
		parts = append(parts, "reverted")
	}
	if e.Filed > 0 {
		parts = append(parts, fmt.Sprintf("%d filed", e.Filed))
	}
	if e.ByHand > 0 {
		parts = append(parts, fmt.Sprintf("%d fixed by hand", e.ByHand))
	}
	if len(parts) == 0 {
		return "0"
	}
	return fmt.Sprintf("%d (%s)", e.Count(), strings.Join(parts, ", "))
}

// followUpItems is a slice's follow-up, `<k> items, <p> proved real`,
// `none` when it has none.
func followUpItems(r reconcile.SliceScore) string {
	if r.FollowUp == 0 {
		return "none"
	}
	return fmt.Sprintf("%d items, %d proved real", len(r.Items), r.ProvedReal())
}

// roleCounts is a slice's runs, `<k> (<role> <k>, …)` in the roles'
// order, the QA's last.
func roleCounts(runs map[string]int) string {
	total, parts := 0, []string{}
	for _, role := range append(slices.Clone(declare.Roles), "qa") {
		if k := runs[role]; k > 0 {
			total += k
			parts = append(parts, fmt.Sprintf("%s %d", role, k))
		}
	}
	if total == 0 {
		return "0"
	}
	return fmt.Sprintf("%d (%s)", total, strings.Join(parts, ", "))
}

// keyCounts is `<key> <k>, …` in the keys' order, `none` for no key.
func keyCounts(m map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// without is ` (<k> runs without <what>)`, "" for none.
func without(k int, what string) string {
	switch k {
	case 0:
		return ""
	case 1:
		return " (1 run without " + what + ")"
	}
	return fmt.Sprintf(" (%d runs without %s)", k, what)
}

// SwarmPromptBuild is `daisy swarm prompt build [--force] [--dry-run]`
// (docs/swarm.md section 11): the clone's persona prompts reseeded from
// this Daisy's defaults, committed and pushed; an edited one is kept, its
// diff from the default printed, and refused unless --force.
func SwarmPromptBuild(c *Context) error {
	return inSwarm(c, func() error { return promptBuild(c) })
}

// promptAction is what prompt build does with one role's prompt.
type promptAction struct {
	role, rel string
	kind      declare.PromptKind
	absent    bool
	text      []byte // the prompt as the clone holds it
}

// promptBuild reads every prompt before it writes any: a role's prompt
// that is absent or an earlier default is written as the current
// default, an edited one only with force; the writes are committed and
// pushed, then the swarm repository's mirror fetched, as init does; the
// edited prompts kept are refused last. With --dry-run it runs read-only
// and says what it would do, --force included.
func promptBuild(c *Context) error {
	dry, force := c.Flag("--dry-run"), c.Flag("--force")
	if dry {
		c.Ops = ops.ReadOnly(c.Ops)
	}
	o, at := c.Ops, c.Swarm
	cfg, _, err := declare.Load(o.FS(), at.Root)
	if err == nil {
		err = declare.ValidateForge(cfg.Forge)
	}
	if err == nil {
		err = declare.CheckClone(o, at, cfg.Forge)
	}
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	// The defaults are the binary's, so the commit it was built from names
	// them, " (modified)" kept as `daisy version` prints it.
	build, err := env.Commit(o)
	if err != nil {
		return err
	}
	rev, modified, _ := strings.Cut(build, " ")
	built := rev[:min(7, len(rev))]
	if modified != "" {
		built += " " + modified
	}

	var actions []promptAction
	var roles []string // the roles whose prompt is written
	for _, role := range cfg.DeclaredRoles() {
		a := promptAction{role: role, rel: filepath.Join("prompts", role+".md")}
		b, err := o.FS().ReadFile(filepath.Join(at.Root, a.rel))
		switch {
		case errors.Is(err, iofs.ErrNotExist):
			a.absent = true
		case err != nil:
			return fmt.Errorf("Error: swarm: cannot read %s: %v", filepath.Join(at.Root, a.rel), err)
		default:
			a.text, a.kind = b, declare.ComparePrompt(role, b)
		}
		if a.absent || a.kind == declare.PromptEarlier || a.kind == declare.PromptEdited && force {
			roles = append(roles, role)
		}
		actions = append(actions, a)
	}
	branch := ""
	if len(roles) > 0 && !dry {
		if branch, err = defaultBranch(c, at); err != nil {
			return err
		}
	}

	var written, kept []string
	for _, a := range actions {
		def := declare.DefaultPrompts[a.role]
		writes := slices.Contains(roles, a.role)
		verb, would, what := "Rebuilt", "Would rebuild", a.rel
		switch {
		case a.absent:
			verb, would = "Wrote", "Would write"
		case a.kind == declare.PromptCurrent:
			verb = "Unchanged"
		case a.kind == declare.PromptEarlier:
			what += " (an earlier default)"
		case writes:
			what += " (edited, overwritten by --force), its diff from the default:"
		default:
			verb, what = "Kept", what+" (edited), its diff from the default:"
			kept = append(kept, a.rel)
		}
		switch {
		case writes && dry:
			fmt.Fprintf(c.Stdout, "  %s %s\n", would, what)
		case writes:
			p := filepath.Join(at.Root, a.rel)
			if err := o.FS().MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return fmt.Errorf("Error: cannot create %s: %v", filepath.Dir(p), err)
			}
			if err := o.FS().WriteFile(p, []byte(def), 0o644); err != nil {
				return fmt.Errorf("Error: cannot write %s: %v", p, err)
			}
			fmt.Fprintf(c.Stdout, "✓ %s %s\n", verb, what)
			written = append(written, a.rel)
		default:
			fmt.Fprintf(c.Stdout, "  %s %s\n", verb, what)
		}
		if a.kind == declare.PromptEdited && !a.absent {
			io.WriteString(c.Stdout, feedback.Diff(a.rel+" (default)", a.rel, []byte(def), a.text))
		}
	}

	message := "swarm: prompts rebuilt from the root at " + built + " (" + strings.Join(roles, ", ") + ")"
	switch {
	case dry && len(roles) > 0:
		fmt.Fprintf(c.Stdout, "  Dry run: nothing written; would commit %q and push it\n", message)
	case dry:
		fmt.Fprintln(c.Stdout, "  Dry run: nothing to write")
	case len(written) == 0:
		fmt.Fprintln(c.Stdout, "  Nothing to commit")
	default:
		if err := commitPaths(c, at, cfg.Forge.Repo, branch, message, written); err != nil {
			return err
		}
		// The swarm repository's mirror after the push, as init's, so
		// that status shows no drift for what was pushed.
		_, repo, _ := strings.Cut(cfg.Forge.Repo, "/")
		if err := (reconcile.Git{O: o, WS: at.Root, State: cfg.Forge.State(c.Env.Local)}).FetchSwarm(repo); err != nil {
			return fmt.Errorf("Error: swarm: mirror %s: %v", repo, err)
		}
		fmt.Fprintf(c.Stdout, "✓ Mirrors: %s fetched\n", repo)
	}
	if len(kept) > 0 {
		return fmt.Errorf("Error: swarm: edited, so kept: %s (each diff above); merge the default's changes in by hand, or run 'daisy swarm prompt build --force' to overwrite them", strings.Join(kept, ", "))
	}
	return nil
}

// defaultBranch is the swarm repository's default branch, as its
// origin's HEAD names it (`git ls-remote --symref origin HEAD` in the
// clone, with the clone's own credentials), which init reads from the
// forge.
func defaultBranch(c *Context, at declare.Place) (string, error) {
	out, errb, err := c.Ops.Exec(at.Root, "git", "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", fmt.Errorf("Error: swarm: cannot read the default branch of %s's origin: git ls-remote: %s", at.Root, gitErr(err, errb))
	}
	for _, l := range strings.Split(string(out), "\n") {
		if target, ok := strings.CutPrefix(l, "ref: refs/heads/"); ok {
			branch, _, _ := strings.Cut(target, "\t")
			return branch, nil
		}
	}
	return "", fmt.Errorf("Error: swarm: %s's origin names no default branch (git ls-remote --symref origin HEAD)", at.Root)
}
