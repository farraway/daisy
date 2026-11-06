package cli

import (
	"fmt"
	"slices"
	"strings"

	"daisy/internal/commands"
	"daisy/internal/health"
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
// there are Min to Max. Health, when
// set, answers `<command> --healthcheck` (hidden from the help): the
// command's own precondition check, run by `daisy healthcheck`.
type Command struct {
	Group     string
	Name      string
	Usage     string
	Help      string
	Workspace bool // needs a resolved workspace before it runs
	Flags     []string
	Values    []string
	Min, Max  int
	Run       func(*commands.Context) error
	Health    func(*commands.Context) error
}

// table is the one source of the help text and the Command Reference: the
// agreed groups and order, each command under the name it answers to, so
// both always say what this binary does, and nothing it does not.
var table = []Command{
	{Group: "Workspace", Name: "init", Usage: "init <identity> [path]", Help: "Bind a workspace to an identity and its home", Min: 1, Max: 2, Run: commands.Init},
	{Group: "Workspace", Name: "clean", Usage: "clean [-f]", Help: "Remove Daisy from this workspace", Flags: []string{"-f", "--force"}, Run: commands.Clean},
	{Group: "Workspace", Name: "status", Usage: "status", Help: "Home, identity, projects, today's counts, active plan", Workspace: true, Run: commands.Status},
	{Group: "Workspace", Name: "files", Usage: "files", Help: "Resolved real paths for every home file", Workspace: true, Run: commands.Files},
	{Group: "Workspace", Name: "list", Usage: "list", Help: "Active prompts and installed skills", Workspace: true, Flags: []string{"--full"}, Run: commands.List, Health: commands.ListHealth},
	{Group: "Workspace", Name: "help", Usage: "help [<command>|<skill>|--markdown]", Help: "This help, detail for one command or skill, or the Command Reference"},

	{Group: "Day", Name: "new-day", Usage: "new-day", Help: "Start a new day", Workspace: true, Run: commands.NewDay, Health: commands.NewDayHealth},
	{Group: "Day", Name: "new-week", Usage: "new-week", Help: "Start a new week", Workspace: true, Run: commands.NewWeek, Health: commands.NewWeekHealth},
	{Group: "Day", Name: "log", Usage: "log <message...>", Help: "Add a log entry to today.md", Workspace: true, Min: 1, Max: Unbounded, Run: commands.Log, Health: commands.LogHealth},
	{Group: "Day", Name: "done", Usage: "done <pattern>", Help: "Mark a task complete", Workspace: true, Min: 1, Max: 1, Run: commands.Done, Health: commands.DoneHealth},
	{Group: "Day", Name: "tasks", Usage: "tasks [--all] [--todo|--done] [pattern]", Help: "Search tasks (read-only)", Workspace: true, Flags: []string{"--all", "--todo", "--done"}, Max: 1, Run: commands.Tasks, Health: commands.TasksHealth},
	{Group: "Day", Name: "projects", Usage: "projects [--archived]", Help: "List active or archived projects", Workspace: true, Flags: []string{"--archived"}, Run: commands.Projects, Health: commands.ProjectsHealth},
	{Group: "Day", Name: "rotate", Usage: "rotate", Help: "Rotate journal.md into window files", Workspace: true, Run: commands.Rotate, Health: commands.RotateHealth},

	{Group: "Plans", Name: "plan new", Usage: "plan new [--spec] <description> [+project]", Help: "Create a plan (or an unregistered spec draft)", Workspace: true, Flags: []string{"--spec"}, Min: 1, Max: 2, Run: commands.PlanNew, Health: commands.PlanNewHealth},
	{Group: "Plans", Name: "plan pickup", Usage: "plan pickup <path>", Help: "Promote a spec draft into a tracked plan", Workspace: true, Min: 1, Max: 1, Run: commands.PlanPickup, Health: commands.PlanPickupHealth},
	{Group: "Plans", Name: "plan archive", Usage: "plan archive", Help: "Archive the active plan", Workspace: true, Run: commands.PlanArchive, Health: commands.PlanArchiveHealth},

	{Group: "Homes and identities", Name: "home new", Usage: "home new <name> [path]", Help: "Create a home; prompts for identity and location", Min: 1, Max: 2, Run: commands.HomeNew, Health: commands.HomeNewHealth},
	{Group: "Homes and identities", Name: "home add", Usage: "home add <name> <path>", Help: "Register an existing home", Min: 2, Max: 2, Run: commands.HomeAdd},
	{Group: "Homes and identities", Name: "home list", Usage: "home list", Help: "Registered homes and where they live", Run: commands.HomeList},
	{Group: "Homes and identities", Name: "home remove", Usage: "home remove <name>", Help: "Unregister a home; its files stay where they are", Min: 1, Max: 1, Run: commands.HomeRemove},
	{Group: "Homes and identities", Name: "identity list", Usage: "identity list", Help: "Identities across registered homes", Run: commands.IdentityList},

	{Group: "Prompts and skills", Name: "prompt build", Usage: "prompt build [<home>]", Help: "Rebuild AGENTS.md", Max: 1, Run: commands.PromptBuild},
	{Group: "Prompts and skills", Name: "install", Usage: "install [--update]", Help: "~/bin/daisy, root skills, permissions", Flags: []string{"--update"}, Run: commands.Install},
	{Group: "Prompts and skills", Name: "feedback", Usage: "feedback [--workflow <n>] <text>", Help: "Record a prompt failure for optimization", Workspace: true, Values: []string{"--workflow", "-w"}, Max: Unbounded, Run: commands.Feedback, Health: commands.FeedbackHealth},
	{Group: "Prompts and skills", Name: "optimize", Usage: "optimize [--workflow <n>]", Help: "Briefing: pending entries per target, current rules, rewrite flags", Workspace: true, Values: []string{"--workflow", "-w"}, Run: commands.Optimize, Health: commands.OptimizeHealth},
	{Group: "Prompts and skills", Name: "optimize apply", Usage: "optimize apply --workflow <n> (--rules|--rewrite) <file> [--yes]", Help: "Apply a proposal: diff, approve, archive, commit", Workspace: true, Flags: []string{"--yes"}, Values: []string{"--workflow", "-w", "--rules", "--rewrite"}, Run: commands.OptimizeApply},
	{Group: "Prompts and skills", Name: "eval", Usage: "eval [<case>] [--record]", Help: "List, display, or record an eval case result", Workspace: true, Flags: []string{"--record"}, Max: 3, Run: commands.Eval, Health: commands.EvalHealth},

	{Group: "Swarm", Name: "swarm init", Usage: "swarm init [--defaults]", Help: "Declare the swarm: swarm.json, prompts, labels", Workspace: true, Flags: []string{"--defaults"}, Run: commands.SwarmInit},

	{Group: "System", Name: "commit", Usage: "commit [--root] <message>", Help: "Commit the home repo (or the root repo)", Workspace: true, Flags: []string{"--root"}, Min: 1, Max: 1, Run: commands.Commit},
	{Group: "System", Name: "healthcheck", Usage: "healthcheck [--force]", Help: "Run the system health check", Flags: []string{"--force"}, Run: commands.Healthcheck},
	{Group: "System", Name: "check-secrets", Usage: "check-secrets", Help: "Check which secrets/tokens are configured", Run: commands.CheckSecrets},
	{Group: "System", Name: "test", Usage: "test [filter]", Help: "Run go test, then the hermetic acceptance suite", Max: 1, Run: commands.Test, Health: commands.TestHealth},
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

// noun is the commands named `<word> <verb>`; none when word is no noun.
func noun(word string) []Command {
	var cmds []Command
	for _, c := range table {
		if n, _, ok := strings.Cut(c.Name, " "); ok && n == word {
			cmds = append(cmds, c)
		}
	}
	return cmds
}

// Flags is what the command named name takes as flags (its Flags and
// Values) and whether it answers `--healthcheck`; ok is false for a name
// the table does not have. The docs test checks prose against it.
func Flags(name string) (flags []string, health, ok bool) {
	for _, c := range table {
		if c.Name == name {
			return append(slices.Clone(c.Flags), c.Values...), c.Health != nil, true
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
		fmt.Fprintf(&b, "| %s | `daisy %s` | %s |\n", c.Group, strings.ReplaceAll(c.Usage, "|", "\\|"), strings.ReplaceAll(c.Help, "|", "\\|"))
	}
	b.WriteString(health.ReferenceEnd + "\n")
	return b.String()
}
