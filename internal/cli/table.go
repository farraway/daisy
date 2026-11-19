package cli

import (
	"fmt"
	"slices"
	"strings"

	"daisy/internal/commands"
	"daisy/internal/health"
	swarmcmd "daisy/internal/swarm/cmd"
	"daisy/internal/verb"
)

// Unbounded is a Max for a command that takes any number of arguments.
const Unbounded = -1

// Command is one line of `daisy help` and of AGENTS.md's Command
// Reference, and Run the function behind it (help alone has none: the
// cli answers it itself). Name is what the user types, one word or a noun and its verb (`prompt
// build`). Usage is the line as printed, Help the description beside it.
// Flags are the words the command takes as flags when they lead its
// arguments, Values those of them that take the next word as a value; the
// first other word starts the positionals, dashes included, of which
// there are Min to Max. Commits marks a command that commits: it also
// takes --co-author as a value flag. ReadOnly runs the whole command
// under ops.ReadOnly, and its help says so: the description's
// ` (read-only)` is this mark, never written by hand. Example is a
// command line that uses it, after `daisy `, shown in its own help.
// Health, when set, answers `<command> --healthcheck` (hidden from the
// help): the command's own precondition check, run by `daisy
// healthcheck`.
type Command struct {
	Group     string
	Name      string
	Usage     string
	Help      string
	Example   string
	Workspace bool // needs a resolved workspace before it runs
	ReadOnly  bool
	Flags     []string
	Values    []string
	Min, Max  int
	Placed    int // of an Unbounded command, the leading arguments with places (`<n>`)
	Commits   bool
	Run       func(*verb.Context) error
	Health    func(*verb.Context) error
}

// table is the one source of the help text and the Command Reference: the
// agreed groups and order, each command under the name it answers to, so
// both always say what this binary does, and nothing it does not.
var table = []Command{
	{Group: "Workspace", Name: "init", Usage: "init [<identity> [path]]", Help: "Bind a workspace to an identity and its home (bare: re-initialize this one)", Example: "init jane", Max: 2, Run: commands.Init},
	{Group: "Workspace", Name: "clean", Usage: "clean [-f]", Help: "Remove Daisy from this workspace", Example: "clean", Flags: []string{"-f", "--force"}, Run: commands.Clean},
	{Group: "Workspace", Name: "status", Usage: "status [--json]", Help: "Home, identity, projects, today's counts, active plan", Example: "status --json", Workspace: true, ReadOnly: true, Flags: []string{verb.JSONFlag}, Run: commands.Status},
	{Group: "Workspace", Name: "files", Usage: "files [--json|<name>]", Help: "Resolved real paths for every home file, or the one path <name> names (plan, today, todo, …)", Example: "files plan", Workspace: true, ReadOnly: true, Flags: []string{verb.JSONFlag}, Max: 1, Run: commands.Files},
	{Group: "Workspace", Name: "list", Usage: "list [--json]", Help: "Active prompts and installed skills", Example: "list --json", Workspace: true, ReadOnly: true, Flags: []string{"--full", verb.JSONFlag}, Run: commands.List, Health: commands.ListHealth},
	{Group: "Workspace", Name: "help", Usage: "help [<command>|<skill>|--markdown]", Help: "This help, detail for one command or skill, or the Command Reference", Example: "help swarm prompt build", ReadOnly: true},

	{Group: "Day", Name: "new-day", Usage: "new-day [--quick]", Help: "Start a new day, unless today has started (--quick: a week if one turned)", Example: "new-day --quick", Workspace: true, Flags: []string{"--quick"}, Commits: true, Run: commands.NewDay, Health: commands.NewDayHealth},
	{Group: "Day", Name: "new-week", Usage: "new-week [--quick]", Help: "Start a new week, unless this week has started (--quick: as new-day --quick)", Example: "new-week --quick", Workspace: true, Flags: []string{"--quick"}, Commits: true, Run: commands.NewWeek, Health: commands.NewWeekHealth},
	{Group: "Day", Name: "log", Usage: "log <message...>", Help: "Add a log entry to today.md, quick-starting a stale day", Example: "log \"Reviewed the parser change\"", Workspace: true, Min: 1, Max: Unbounded, Commits: true, Run: commands.Log, Health: commands.LogHealth},
	{Group: "Day", Name: "add", Usage: "add <description> A|B|C|D|none", Help: "Add a task to todo.txt, and to today.md when (A) or (B)", Example: "add \"Write the parser tests +daisy\" B", Workspace: true, Min: 2, Max: 2, Commits: true, Run: commands.Add},
	{Group: "Day", Name: "done", Usage: "done <pattern>", Help: "Mark a task complete", Example: "done \"parser tests\"", Workspace: true, Min: 1, Max: 1, Commits: true, Run: commands.Done, Health: commands.DoneHealth},
	{Group: "Day", Name: "cancel", Usage: "cancel <pattern>", Help: "Cancel a task; the next new day deletes it", Example: "cancel \"parser tests\"", Workspace: true, Min: 1, Max: 1, Commits: true, Run: commands.Cancel},
	{Group: "Day", Name: "priority", Usage: "priority <pattern> A|B|C|D|none", Help: "Change a task's priority, and its section in today.md", Example: "priority \"parser tests\" A", Workspace: true, Min: 2, Max: 2, Commits: true, Run: commands.Priority},
	{Group: "Day", Name: "tasks", Usage: "tasks [--all] [--todo|--done] [--json] [pattern]", Help: "Search tasks", Example: "tasks --todo parser", Workspace: true, ReadOnly: true, Flags: []string{"--all", "--todo", "--done", verb.JSONFlag}, Max: 1, Run: commands.Tasks, Health: commands.TasksHealth},
	{Group: "Day", Name: "projects", Usage: "projects [--archived] [--json]", Help: "List active or archived projects", Example: "projects --archived", Workspace: true, ReadOnly: true, Flags: []string{"--archived", verb.JSONFlag}, Run: commands.Projects, Health: commands.ProjectsHealth},
	{Group: "Day", Name: "rotate", Usage: "rotate", Help: "Rotate journal.md into window files", Example: "rotate", Workspace: true, Run: commands.Rotate, Health: commands.RotateHealth},

	{Group: "Plans", Name: "plan new", Usage: "plan new [--spec] <description> [+project]", Help: "Create a plan (or an unregistered spec draft)", Example: "plan new \"Port the parser to Go\" +parser", Workspace: true, Flags: []string{"--spec"}, Min: 1, Max: 2, Run: commands.PlanNew, Health: commands.PlanNewHealth},
	{Group: "Plans", Name: "plan pickup", Usage: "plan pickup <path>", Help: "Promote a spec draft into a tracked plan", Example: "plan pickup parser_PLAN.md", Workspace: true, Min: 1, Max: 1, Commits: true, Run: commands.PlanPickup, Health: commands.PlanPickupHealth},
	{Group: "Plans", Name: "plan archive", Usage: "plan archive", Help: "Archive the active plan", Example: "plan archive", Workspace: true, Commits: true, Run: commands.PlanArchive, Health: commands.PlanArchiveHealth},
	{Group: "Plans", Name: "metrics", Usage: "metrics [--swarm <owner>/<repo>] [--since <time>] [--json]", Help: "Summary of the home's metrics CSV, by kind and model", Example: "metrics --since 2026-11-01T00:00:00Z --json", Workspace: true, ReadOnly: true, Flags: []string{verb.JSONFlag}, Values: []string{"--swarm", "--since"}, Run: commands.Metrics},
	{Group: "Plans", Name: "metrics add", Usage: "metrics add --step <s> [--plan <p>] [--range <r> [--repo <dir>]] [<flag> <value>...]", Help: "Record a plan step: a row in the home's metrics CSV and in the plan's ## Metrics table", Example: "metrics add --step 3 --range HEAD^! --outcome done", Workspace: true,
		Values: []string{"--step", "--plan", "--range", "--repo", "--role", "--provider", "--model", "--effort", "--root-sha", "--home-sha", "--prompt-hash", "--outcome", "--review-rounds", "--tokens-in", "--tokens-out", "--tokens", "--tokens-cached", "--review-effect", "--cost", "--cost-estimated", "--notes", "--when"}, Run: commands.MetricsAdd},

	{Group: "Homes and identities", Name: "home new", Usage: "home new <name> [path]", Help: "Create a home; prompts for identity and location", Example: "home new work", Min: 1, Max: 2, Commits: true, Run: commands.HomeNew, Health: commands.HomeNewHealth},
	{Group: "Homes and identities", Name: "home add", Usage: "home add <name> <path>", Help: "Register an existing home", Example: "home add work ~/daisy-work", Min: 2, Max: 2, Run: commands.HomeAdd},
	{Group: "Homes and identities", Name: "home list", Usage: "home list", Help: "Registered homes and where they live", Example: "home list", ReadOnly: true, Run: commands.HomeList},
	{Group: "Homes and identities", Name: "home remove", Usage: "home remove <name>", Help: "Unregister a home; its files stay where they are", Example: "home remove work", Min: 1, Max: 1, Run: commands.HomeRemove},
	{Group: "Homes and identities", Name: "identity list", Usage: "identity list", Help: "Identities across registered homes", Example: "identity list", ReadOnly: true, Run: commands.IdentityList},

	{Group: "Prompts and skills", Name: "prompt build", Usage: "prompt build [<home>]", Help: "Rebuild AGENTS.md", Example: "prompt build work", Max: 1, Run: commands.PromptBuild},
	{Group: "Prompts and skills", Name: "install", Usage: "install [--update]", Help: "~/bin/daisy and DAISY_ROOT; nothing under ~/.claude", Example: "install --update", Flags: []string{"--update"}, Run: commands.Install},
	{Group: "Prompts and skills", Name: "feedback", Usage: "feedback [--workflow <n>] <text>", Help: "Record a prompt failure for optimization", Example: "feedback --workflow plan \"Skipped the review gate\"", Workspace: true, Values: []string{"--workflow", "-w"}, Max: Unbounded, Run: commands.Feedback, Health: commands.FeedbackHealth},
	{Group: "Prompts and skills", Name: "optimize", Usage: "optimize [--workflow <n>]", Help: "Briefing: pending entries per target, current rules, rewrite flags", Example: "optimize --workflow plan", Workspace: true, ReadOnly: true, Values: []string{"--workflow", "-w"}, Run: commands.Optimize, Health: commands.OptimizeHealth},
	{Group: "Prompts and skills", Name: "optimize apply", Usage: "optimize apply --workflow <n> (--rules|--rewrite) <file> [--yes]", Help: "Apply a proposal: diff, approve, archive, commit", Example: "optimize apply --workflow plan --rules proposal.md", Workspace: true, Flags: []string{"--yes"}, Values: []string{"--workflow", "-w", "--rules", "--rewrite"}, Commits: true, Run: commands.OptimizeApply},
	{Group: "Prompts and skills", Name: "eval", Usage: "eval [<case>] [--record]", Help: "List, display, or record an eval case result", Example: "eval --record new-day pass", Workspace: true, Flags: []string{"--record"}, Max: 3, Run: commands.Eval, Health: commands.EvalHealth},

	{Group: "Swarm", Name: "swarm init", Usage: "swarm init [--defaults] [--api <url>]", Help: "Declare the swarm: swarm.json, prompts, labels", Example: "swarm init --defaults", Flags: []string{"--defaults"}, Values: []string{"--api"}, Run: swarmcmd.SwarmInit},
	{Group: "Swarm", Name: "swarm prompt build", Usage: "swarm prompt build [--force] [--dry-run]", Help: "Reseed prompts/ from this Daisy's defaults; an edited prompt is kept", Example: "swarm prompt build --dry-run", Flags: []string{"--force", "--dry-run"}, Run: swarmcmd.SwarmPromptBuild},
	{Group: "Swarm", Name: "swarm apply", Usage: "swarm apply [--once]", Help: "Reconcile the forge: launch members, post their output, move labels", Example: "swarm apply --once", Flags: []string{"--once"}, Run: swarmcmd.SwarmApply},
	{Group: "Swarm", Name: "swarm metrics", Usage: "swarm metrics --backfill | --reprice [--dry-run]", Help: "Write each collected run's missing metrics row (--backfill), or supersede a row its output now reads otherwise (--reprice)", Example: "swarm metrics --reprice --dry-run", Flags: []string{"--backfill", "--reprice", "--dry-run"}, Run: swarmcmd.SwarmMetrics},
	{Group: "Swarm", Name: "swarm status", Usage: "swarm status [--json] [--waiting]", Help: "Stages, gates, running members, what waits for you (--waiting: that alone)", Example: "swarm status --waiting", ReadOnly: true, Flags: []string{verb.JSONFlag, swarmcmd.WaitingFlag}, Run: swarmcmd.SwarmStatus},
	{Group: "Swarm", Name: "swarm list", Usage: "swarm list [--json]", Help: "The swarms this machine runs: clone, home, pause; from anywhere", Example: "swarm list --json", ReadOnly: true, Flags: []string{verb.JSONFlag}, Run: swarmcmd.SwarmList},
	{Group: "Swarm", Name: "swarm journal", Usage: "swarm journal [--json] [--issue <n>] | --mark <run>", Help: "The journal since the line marked seen (--issue: one issue's lines); --mark moves the mark, its one write", Example: "swarm journal --mark 3-architect-1", Flags: []string{verb.JSONFlag}, Values: []string{"--mark", "--issue"}, Run: swarmcmd.SwarmJournal},
	{Group: "Swarm", Name: "swarm connect", Usage: "swarm connect [--json]", Help: "The connect briefing: journal by issue, waits with gate notes, questions, AUTHORITIES.md, metrics", Example: "swarm connect --json", ReadOnly: true, Flags: []string{verb.JSONFlag}, Run: swarmcmd.SwarmConnect},
	{Group: "Swarm", Name: "swarm pause", Usage: "swarm pause [<n>]", Help: "Launch no new member until resumed (with <n>, as the maintainer, on that issue)", Example: "swarm pause 12", Max: 1, Run: swarmcmd.SwarmPause},
	{Group: "Swarm", Name: "swarm resume", Usage: "swarm resume [<n>]", Help: "Launch members again and release the owner's held batch (with <n>, that issue)", Example: "swarm resume 12", Max: 1, Run: swarmcmd.SwarmResume},
	{Group: "Swarm", Name: "swarm restart", Usage: "swarm restart", Help: "Have the running apply re-execute itself, onto the installed build, once no member runs", Example: "swarm restart", Run: swarmcmd.SwarmRestart},
	{Group: "Swarm", Name: "swarm triage", Usage: "swarm triage <n>", Help: "As the maintainer: triage a proposal, proposal-posted to proposal-accepted", Example: "swarm triage 12", Min: 1, Max: 1, Run: swarmcmd.SwarmTriage},
	{Group: "Swarm", Name: "swarm accept", Usage: "swarm accept <n>", Help: "As the maintainer: accept the plan, plan-0-posted to plan-1-accepted", Example: "swarm accept 12", Min: 1, Max: 1, Run: swarmcmd.SwarmAccept},
	{Group: "Swarm", Name: "swarm class", Usage: "swarm class <n> routine|architecture", Help: "As the maintainer: set the job's class; Daisy passes a routine job's triage and plan", Example: "swarm class 12 routine", Min: 2, Max: 2, Run: swarmcmd.SwarmClass},
	{Group: "Swarm", Name: "swarm approve", Usage: "swarm approve <n>", Help: "As the maintainer: approve the issue's open pull request", Example: "swarm approve 12", Min: 1, Max: 1, Run: swarmcmd.SwarmApprove},
	{Group: "Swarm", Name: "swarm merge", Usage: "swarm merge <n>", Help: "As the maintainer: merge the approved pull request, then next slice or close", Example: "swarm merge 12", Min: 1, Max: 1, Run: swarmcmd.SwarmMerge},
	{Group: "Swarm", Name: "swarm unblock", Usage: "swarm unblock <n> [--cause <word>] <comment...>", Help: "As the maintainer: comment, then clear swarm:blocked from the issue (--cause: retag the stop's cause)", Example: "swarm unblock 12 --cause environment \"The token is renewed\"", Values: []string{"--cause"}, Min: 2, Max: Unbounded, Placed: 1, Run: swarmcmd.SwarmUnblock},
	{Group: "Swarm", Name: "swarm block", Usage: "swarm block <n> --by <m>", Help: "As the maintainer: hold the issue's job until #m is closed", Example: "swarm block 12 --by 9", Values: []string{"--by"}, Min: 1, Max: 3, Run: swarmcmd.SwarmBlock},
	{Group: "Swarm", Name: "swarm replan", Usage: "swarm replan <n> [direction...]", Help: "As the maintainer: have the architect re-plan the job, from your direction", Example: "swarm replan 12 \"Split the parser change in two\"", Min: 1, Max: Unbounded, Placed: 1, Run: swarmcmd.SwarmReplan},
	{Group: "Swarm", Name: "swarm close", Usage: "swarm close <n> [comment...]", Help: "As the maintainer: close the issue, after the comment when given", Example: "swarm close 12 \"Superseded by #14\"", Min: 1, Max: Unbounded, Placed: 1, Run: swarmcmd.SwarmClose},

	{Group: "System", Name: "commit", Usage: "commit [--root] <message>", Help: "Commit the home repo (or the root repo)", Example: "commit --co-author \"Jane Doe <jane@example.net>\" \"Notes from the review\"", Workspace: true, Flags: []string{"--root"}, Min: 1, Max: 1, Commits: true, Run: commands.Commit},
	{Group: "System", Name: "healthcheck", Usage: "healthcheck [--force]", Help: "Run the system health check", Example: "healthcheck --force", Flags: []string{"--force"}, Run: commands.Healthcheck},
	{Group: "System", Name: "check-secrets", Usage: "check-secrets", Help: "Check which secrets/tokens are configured", Example: "check-secrets", ReadOnly: true, Run: commands.CheckSecrets},
	{Group: "System", Name: "test", Usage: "test [filter]", Help: "Run go test, then the hermetic acceptance suite", Example: "test done", Max: 1, Run: commands.Test, Health: commands.TestHealth},
	{Group: "System", Name: "version", Usage: "version", Help: "The commit this binary was built from; also --version", Example: "version", ReadOnly: true, Run: commands.Version},
}

// groups in display order.
var groups = []string{"Workspace", "Day", "Plans", "Homes and identities", "Prompts and skills", "Swarm", "System"}

// match is the command args name and the arguments after its name: the
// longest name that leads args (`optimize apply` before `optimize`).
func match(args []string) (cmd Command, rest []string, ok bool) {
	best := 0
	for _, c := range table {
		words := strings.Fields(c.Name)
		if len(words) > best && len(args) >= len(words) && slices.Equal(args[:len(words)], words) {
			cmd, rest, ok, best = c, args[len(words):], true, len(words)
		}
	}
	return cmd, rest, ok
}

// phrase is the longest noun phrase that leads args, its length in
// words, and the commands whose names go on past it (`swarm prompt` and
// `swarm prompt build`); n is 0 when args start no command's name.
func phrase(args []string) (n int, cmds []Command) {
	for _, c := range table {
		words := strings.Fields(c.Name)
		k := 0
		for k < len(words)-1 && k < len(args) && args[k] == words[k] {
			k++
		}
		switch {
		case k > n:
			n, cmds = k, []Command{c}
		case k == n && k > 0:
			cmds = append(cmds, c)
		}
	}
	return n, cmds
}

// description is the command's Help, then ` (read-only)` when the table
// runs it under ReadOnly: the help says what the cli enforces.
func (c Command) description() string {
	if c.ReadOnly {
		return c.Help + " (read-only)"
	}
	return c.Help
}

// values is the command's value flags, --co-author among them when it
// commits.
func (c Command) values() []string {
	if c.Commits {
		return append(slices.Clone(c.Values), commands.CoAuthorFlag)
	}
	return c.Values
}

// Flags is what the command named name takes as flags (its Flags and
// values) and whether it answers `--healthcheck`; ok is false for a name
// the table does not have. The docs test checks prose against it.
func Flags(name string) (flags []string, health, ok bool) {
	for _, c := range table {
		if c.Name == name {
			return append(slices.Clone(c.Flags), c.values()...), c.Health != nil, true
		}
	}
	return nil, false, false
}

// Reference is AGENTS.md's Command Reference, rendered from the table
// between the markers the health check finds it by: one row per command,
// in table order. `daisy help --markdown` prints it.
func Reference() string {
	var b strings.Builder
	b.WriteString(health.ReferenceBegin + "\n")
	b.WriteString("| Group | Command | What it does |\n|---|---|---|\n")
	for _, c := range table {
		fmt.Fprintf(&b, "| %s | `daisy %s` | %s |\n", c.Group, strings.ReplaceAll(c.Usage, "|", "\\|"), strings.ReplaceAll(c.description(), "|", "\\|"))
	}
	b.WriteString(health.ReferenceEnd + "\n")
	return b.String()
}
