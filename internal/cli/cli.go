// Package cli is the only place that knows argument syntax: the command
// tree, each command's flags and arity, --help, the --healthcheck hook,
// usage errors (exit 2, usage on stderr) and the fall-through to daisy.sh
// for commands the binary has not taken over.
package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/commands"
	"daisy/internal/env"
	"daisy/internal/ops"
)

// Exit codes.
const (
	OK      = 0
	Failure = 1
	Usage   = 2
)

// Run dispatches args. Errors a command returns are printed as they are:
// each is already the message for the user, fix included.
func Run(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 || isHelpWord(args[0]) && len(args) == 1 {
		writeTree(stdout)
		return OK
	}
	if isHelpWord(args[0]) {
		return help(o, stdin, stdout, stderr, args[1:])
	}

	cmd, rest, ok := match(args)
	if !ok {
		if group := noun(args[0]); group != nil {
			return nounOnly(stdout, stderr, args, group)
		}
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		writeTree(stderr)
		return Usage
	}
	if cmd.bash() {
		return fallThrough(o, stdin, stdout, stderr, args)
	}
	if len(rest) == 1 && isHelpFlag(rest[0]) {
		writeUsage(stdout, cmd)
		return OK
	}
	c := &commands.Context{Ops: o, Stdin: stdin, Stdout: stdout, Stderr: stderr}
	if len(rest) == 1 && rest[0] == "--healthcheck" && cmd.Health != nil {
		return invoke(c, cmd, cmd.Health)
	}

	flags, values, positional, empty, err := splitArgs(cmd, rest)
	if err != nil {
		return usageError(stderr, cmd, err.Error())
	}
	c.Flags, c.Values, c.Args, c.EmptyWords = flags, values, positional, empty
	if cmd.Max != Unbounded && emptyBefore(cmd, rest) {
		return usageError(stderr, cmd, "empty argument")
	}
	if len(c.Args) < cmd.Min {
		return usageError(stderr, cmd, "missing argument")
	}
	if cmd.Max != Unbounded && len(c.Args) > cmd.Max {
		return usageError(stderr, cmd, fmt.Sprintf("unexpected argument: %s", c.Args[cmd.Max]))
	}
	return invoke(c, cmd, cmd.Run)
}

// help answers `daisy help <name>`. A command's help is what its own
// --help prints (the bash's, for a command still in bash), followed by a
// note when a skill has the same name; a noun's is its commands' usages;
// any other single name is a skill's, which needs the workspace.
func help(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	_, _, words, _, _ := splitArgs(Command{}, args)
	helpCmd, _, _ := match([]string{"help"})
	c := &commands.Context{Ops: o, Stdin: stdin, Stdout: stdout, Stderr: stderr, Args: words}
	switch cmd, rest, ok := match(words); {
	case len(words) == 0:
		writeTree(stdout)
		return OK
	case len(words) == 1 && isHelpFlag(words[0]):
		writeUsage(stdout, helpCmd)
		return OK
	case ok && len(rest) > 0:
		return usageError(stderr, helpCmd, "unexpected argument: "+rest[0])
	case ok && cmd.bash():
		code := fallThrough(o, stdin, stdout, stderr, append(strings.Fields(cmd.Name), "--help"))
		commands.ShadowNote(c, cmd.Name)
		return code
	case ok:
		writeUsage(stdout, cmd)
		commands.ShadowNote(c, cmd.Name)
		return OK
	case len(words) > 1:
		return usageError(stderr, helpCmd, "unexpected argument: "+words[1])
	case noun(words[0]) != nil:
		for _, cmd := range noun(words[0]) {
			writeUsage(stdout, cmd)
		}
		commands.ShadowNote(c, words[0])
		return OK
	}
	return invoke(c, Command{Workspace: true}, commands.SkillHelp)
}

// nounOnly answers a noun without a verb it has: its commands' usages, on
// stdout for --help, else on stderr as a usage error.
func nounOnly(stdout, stderr io.Writer, args []string, group []Command) int {
	w, code := stderr, Usage
	switch {
	case len(args) == 2 && isHelpFlag(args[1]):
		w, code = stdout, OK
	case len(args) == 1:
		fmt.Fprintf(stderr, "Error: missing command: daisy %s <command>\n", args[0])
	default:
		fmt.Fprintf(stderr, "Error: unknown command: %s %s\n", args[0], args[1])
	}
	for _, cmd := range group {
		writeUsage(w, cmd)
	}
	return code
}

// splitArgs: the command's flags are the known words leading its
// arguments, a value flag taking the word after it as its value; the first
// other word, dashes or not, starts the positionals (`tasks --todo "-1
// point"` searches for `-1 point`). An empty word is no argument: `done ""`
// is a missing pattern, not one that matches every task; empty says one
// was dropped. A value flag without a value, or with an empty one, is a
// usage error.
func splitArgs(cmd Command, args []string) (flags []string, values []commands.Value, positional []string, empty bool, err error) {
	args, flags, values, err = leading(cmd, args)
	for _, a := range args {
		if a != "" {
			positional = append(positional, a)
		} else {
			empty = true
		}
	}
	return flags, values, positional, empty, err
}

// leading takes the flags and value flags off the front of args.
func leading(cmd Command, args []string) (rest, flags []string, values []commands.Value, err error) {
	for len(args) > 0 {
		switch {
		case slices.Contains(cmd.Flags, args[0]):
			flags, args = append(flags, args[0]), args[1:]
		case slices.Contains(cmd.Values, args[0]):
			// The value is the next word, unless that is one of the
			// command's own flags: `--rules --yes` is a missing value,
			// not a file named --yes.
			if len(args) < 2 || args[1] == "" || slices.Contains(cmd.Flags, args[1]) || slices.Contains(cmd.Values, args[1]) {
				return nil, nil, nil, fmt.Errorf("%s needs a value", args[0])
			}
			values, args = append(values, commands.Value{Flag: args[0], Value: args[1]}), args[2:]
		default:
			return args, flags, values, nil
		}
	}
	return nil, flags, values, nil
}

// emptyBefore: an empty word comes before a given one. A command whose
// arguments have places would read the later word in the empty one's
// (`plan new "" +proj` as the description `+proj`), so that is a usage
// error; for a command taking any number of words the places do not matter.
func emptyBefore(cmd Command, args []string) bool {
	args, _, _, _ = leading(cmd, args)
	empty := false
	for _, a := range args {
		if a == "" {
			empty = true
		} else if empty {
			return true
		}
	}
	return false
}

// invoke resolves the workspace when the command needs one and runs fn;
// the error it returns is the message for stderr.
func invoke(c *commands.Context, cmd Command, fn func(*commands.Context) error) int {
	if cmd.Workspace {
		e, err := env.Resolve(c.Ops)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return Failure
		}
		c.Env = e
	}
	if err := fn(c); err != nil {
		var u commands.UsageError
		if errors.As(err, &u) {
			return usageError(c.Stderr, cmd, string(u))
		}
		fmt.Fprintln(c.Stderr, err)
		return Failure
	}
	return OK
}

func isHelpWord(s string) bool { return s == "help" || isHelpFlag(s) }
func isHelpFlag(s string) bool { return s == "--help" || s == "-h" }

func usageError(stderr io.Writer, cmd Command, msg string) int {
	fmt.Fprintf(stderr, "Error: %s\n", msg)
	writeUsage(stderr, cmd)
	return Usage
}

func writeUsage(w io.Writer, cmd Command) {
	fmt.Fprintf(w, "Usage: daisy %s\n\n%s\n", cmd.Usage, cmd.Help)
}

// helpColumn is where a description starts; a usage that reaches it gets
// its description on the next line.
const helpColumn = 34

// writeTree prints the help: the table, grouped, in table order.
func writeTree(w io.Writer) {
	var b strings.Builder
	b.WriteString("Usage: daisy <command> [args...]\n")
	for _, g := range groups {
		b.WriteString("\n" + g + "\n")
		for _, c := range table {
			if c.Group != g {
				continue
			}
			if len(c.Usage) >= helpColumn {
				fmt.Fprintf(&b, "  %s\n  %-*s%s\n", c.Usage, helpColumn, "", c.Help)
			} else {
				fmt.Fprintf(&b, "  %-*s%s\n", helpColumn, c.Usage, c.Help)
			}
		}
	}
	b.WriteString("\nEvery command also accepts --help/-h (as the sole argument) to print its usage.\n")
	b.WriteString("\nEnvironment:\n  DAISY_ROOT    Path to the daisy repository (required)\n")
	io.WriteString(w, b.String())
}

// fallThrough runs $DAISY_ROOT/daisy.sh with the same arguments and streams
// and returns its exit code: every command works at every commit of the
// port, whichever side of it the command is on.
func fallThrough(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	root, err := env.Root(o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return Failure
	}
	// A cwd that cannot be read (deleted under the shell) is left to the
	// child, which inherits the descriptor as the bash did.
	cwd, _ := o.Getwd()
	script := filepath.Join(root, "daisy.sh")
	p, err := o.Start(cwd, script, stdin, stdout, stderr, args...)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot run %s: %v\n", script, err)
		return Failure
	}
	code, err := p.Wait()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s did not exit normally: %v\n", script, err)
		return Failure
	}
	return code
}
