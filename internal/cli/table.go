package cli

import "daisy/internal/commands"

// Unbounded is a Max for a command that takes any number of arguments.
const Unbounded = -1

// Command is one line of `daisy help` and, when Run is set, the function
// behind it. A command without Run is still the bash's: the binary execs
// $DAISY_ROOT/daisy.sh for it. Usage is the line as printed, Help the
// description beside it. Flags are the words the command takes as flags
// when they lead its arguments; the first other word starts the
// positionals, dashes included, of which there are Min to Max. Health, when
// set, answers `<command> --healthcheck` (hidden from the help): the
// command's own precondition check, run by `daisy healthcheck`.
type Command struct {
	Group     string
	Name      string // the first word: what the user types
	Usage     string
	Help      string
	Workspace bool // needs a resolved workspace before it runs
	Flags     []string
	Min, Max  int
	Run       func(*commands.Context) error
	Health    func(*commands.Context) error
}

// table is the one source of the help text: the agreed groups and order,
// each command under the name this build answers to. A command keeps its
// legacy spelling until the step that ports it renames it; the tree
// therefore always says what this binary does, and nothing it does not.
var table = []Command{
	{Group: "Workspace", Name: "init", Usage: "init <identity> [path]", Help: "Bind a workspace to an identity and its home"},
	{Group: "Workspace", Name: "clean", Usage: "clean [-f]", Help: "Remove Daisy from this workspace"},
	{Group: "Workspace", Name: "status", Usage: "status", Help: "Home, identity, projects, today's counts, active plan", Workspace: true, Run: commands.Status},
	{Group: "Workspace", Name: "files", Usage: "files", Help: "Resolved real paths for every home file", Workspace: true, Run: commands.Files},
	{Group: "Workspace", Name: "list", Usage: "list", Help: "Active prompts and installed skills"},
	{Group: "Workspace", Name: "help", Usage: "help [<command>|<skill>]", Help: "This help, or detail for one command or skill"},

	{Group: "Day", Name: "new-day", Usage: "new-day", Help: "Start a new day"},
	{Group: "Day", Name: "new-week", Usage: "new-week", Help: "Start a new week"},
	{Group: "Day", Name: "log", Usage: "log <message...>", Help: "Add a log entry to today.md", Workspace: true, Min: 1, Max: Unbounded, Run: commands.Log, Health: commands.LogHealth},
	{Group: "Day", Name: "done", Usage: "done <pattern>", Help: "Mark a task complete", Workspace: true, Min: 1, Max: 1, Run: commands.Done, Health: commands.DoneHealth},
	{Group: "Day", Name: "tasks", Usage: "tasks [--all] [--todo|--done] [pattern]", Help: "Search tasks (read-only)", Workspace: true, Flags: []string{"--all", "--todo", "--done"}, Max: 1, Run: commands.Tasks, Health: commands.TasksHealth},
	{Group: "Day", Name: "projects", Usage: "projects [--archived]", Help: "List active or archived projects", Workspace: true, Flags: []string{"--archived"}, Run: commands.Projects, Health: commands.ProjectsHealth},
	{Group: "Day", Name: "rotate", Usage: "rotate", Help: "Rotate journal.md into window files"},

	{Group: "Plans", Name: "plan-new", Usage: "plan-new [--spec] <description> [+project]", Help: "Create a plan (or an unregistered spec draft)"},
	{Group: "Plans", Name: "plan-pickup", Usage: "plan-pickup <path>", Help: "Promote a spec draft into a tracked plan"},
	{Group: "Plans", Name: "plan-archive", Usage: "plan-archive", Help: "Archive the active plan"},

	{Group: "Homes and identities", Name: "home", Usage: "home [list|add <name> <path>|remove <name>|new <name> [path]]", Help: "The homes registry: which homes exist here and where"},

	{Group: "Prompts and skills", Name: "build", Usage: "build [<home>]", Help: "Rebuild AGENTS.md"},
	{Group: "Prompts and skills", Name: "install", Usage: "install [--update]", Help: "~/bin/daisy, root skills, permissions"},
	{Group: "Prompts and skills", Name: "feedback", Usage: "feedback [--workflow <n>] <text>", Help: "Record a prompt failure for optimization"},
	{Group: "Prompts and skills", Name: "optimize", Usage: "optimize [--workflow <n>] [--dry-run] [--force]", Help: "Run the prompt learning loop on collected feedback"},
	{Group: "Prompts and skills", Name: "eval", Usage: "eval [<case>] [--record]", Help: "List, display, or record an eval case result"},

	{Group: "System", Name: "commit", Usage: "commit --home|--not-home <message>", Help: "Commit the home repo (or everything else)"},
	{Group: "System", Name: "healthcheck", Usage: "healthcheck [--force]", Help: "Run the system health check"},
	{Group: "System", Name: "check-secrets", Usage: "check-secrets", Help: "Check which secrets/tokens are configured"},
	{Group: "System", Name: "test", Usage: "test [filter]", Help: "Run the hermetic acceptance suite"},
}

// groups in display order.
var groups = []string{"Workspace", "Day", "Plans", "Homes and identities", "Prompts and skills", "System"}

func lookup(name string) (Command, bool) {
	for _, c := range table {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}
