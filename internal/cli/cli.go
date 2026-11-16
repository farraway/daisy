// Package cli is the only place that knows argument syntax: the command
// tree, each command's flags and arity, --help, the --healthcheck hook,
// usage errors (exit 2, usage on stderr), and the table rendered as the
// help and as AGENTS.md's Command Reference.
package cli

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"daisy/internal/commands"
	"daisy/internal/env"
	"daisy/internal/ops"
	swarmcmd "daisy/internal/swarm/cmd"
	"daisy/internal/verb"
)

// Exit codes.
const (
	OK          = 0
	Failure     = 1
	Usage       = 2
	Interrupted = 130 // a second signal ended `swarm apply` at once
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
	if args[0] == "--version" {
		args = append([]string{"version"}, args[1:]...)
	}

	cmd, rest, ok := match(args)
	if !ok {
		if n, group := phrase(args); n > 0 {
			return nounOnly(stdout, stderr, args[:n], args[n:], group)
		}
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		writeTree(stderr)
		return Usage
	}
	if len(rest) == 1 && isHelpFlag(rest[0]) {
		writeUsage(stdout, cmd)
		return OK
	}
	c := newContext(o, stdin, stdout, stderr)
	if len(rest) == 1 && rest[0] == "--healthcheck" && cmd.Health != nil {
		return invoke(c, cmd, cmd.Health)
	}

	if err := parse(c, cmd, rest); err != nil {
		return usageError(stderr, cmd, err.Error())
	}
	return invoke(c, cmd, cmd.Run)
}

// parse checks the arguments after the command's name against its flags
// and arity and gives them to c; the error is the usage error's message.
func parse(c *verb.Context, cmd Command, rest []string) error {
	flags, values, positional, empty, err := splitArgs(cmd, rest)
	if err != nil {
		return err
	}
	if len(positional) > 0 && longFlag.MatchString(positional[0]) {
		return errors.New("unknown flag: " + positional[0])
	}
	c.Flags, c.Values, c.Args, c.EmptyWords = flags, values, positional, empty
	for _, v := range values {
		if v.Flag == commands.CoAuthorFlag && !commands.IsCoAuthor(v.Value) {
			return fmt.Errorf("%s %q is not one line 'Name <email>'", v.Flag, v.Value)
		}
	}
	if emptyBefore(cmd, rest) {
		return errors.New("empty argument")
	}
	if len(c.Args) < cmd.Min {
		return errors.New("missing argument")
	}
	if cmd.Max != Unbounded && len(c.Args) > cmd.Max {
		return fmt.Errorf("unexpected argument: %s", c.Args[cmd.Max])
	}
	return nil
}

// newContext is a command's Context before its arguments: the streams,
// and the table's hooks and Command Reference for the health check.
func newContext(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer) *verb.Context {
	c := &verb.Context{Ops: o, Stdin: stdin, Stdout: stdout, Stderr: stderr, Reference: Reference()}
	for _, cmd := range table {
		if cmd.Health != nil {
			c.Components = append(c.Components, verb.Component{Name: cmd.Name, Check: cmd.Health})
		}
	}
	return c
}

// help answers `daisy help <name>`. A command's help is what its own
// --help prints, followed by a note when a skill has the same name; a
// noun phrase's (`swarm`, `swarm prompt`) is the usages of the commands
// under it; `--markdown` is the Command Reference; any other single name
// is a skill's, which needs the workspace. The words name a command as
// the dispatcher reads them: the longest name, else the longest phrase.
func help(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	_, _, words, _, _ := splitArgs(Command{}, args)
	helpCmd, _, _ := match([]string{"help"})
	if emptyBefore(helpCmd, args) {
		return usageError(stderr, helpCmd, "empty argument")
	}
	c := newContext(o, stdin, stdout, stderr)
	c.Args = words
	cmd, rest, ok := match(words)
	n, group := phrase(words)
	switch {
	case len(words) == 0:
		writeTree(stdout)
		return OK
	case len(words) == 1 && isHelpFlag(words[0]):
		writeUsage(stdout, helpCmd)
		return OK
	case len(words) == 1 && words[0] == "--markdown":
		io.WriteString(stdout, Reference())
		return OK
	case ok && len(rest) > 0:
		return usageError(stderr, helpCmd, "unexpected argument: "+rest[0])
	case ok:
		writeUsage(stdout, cmd)
		commands.ShadowNote(c, cmd.Name)
		return OK
	case n > 0 && n < len(words):
		return usageError(stderr, helpCmd, "unexpected argument: "+words[n])
	case n > 0:
		for _, cmd := range group {
			writeUsage(stdout, cmd)
		}
		if n == 1 {
			commands.ShadowNote(c, words[0])
		}
		return OK
	case len(words) > 1:
		return usageError(stderr, helpCmd, "unexpected argument: "+words[1])
	}
	skill := helpCmd
	skill.Workspace = true
	return invoke(c, skill, commands.SkillHelp)
}

// nounOnly answers a noun phrase without a verb it has: the usages of the
// commands under it, on stdout for --help, else on stderr as a usage
// error.
func nounOnly(stdout, stderr io.Writer, noun, rest []string, group []Command) int {
	w, code := stderr, Usage
	switch {
	case len(rest) == 1 && isHelpFlag(rest[0]):
		w, code = stdout, OK
	case len(rest) == 0:
		fmt.Fprintf(stderr, "Error: missing command: daisy %s <command>\n", strings.Join(noun, " "))
	default:
		fmt.Fprintf(stderr, "Error: unknown command: %s %s\n", strings.Join(noun, " "), rest[0])
	}
	for _, cmd := range group {
		writeUsage(w, cmd)
	}
	return code
}

// splitArgs: the command's flags are the known words leading its
// arguments, a value flag taking the word after it as its value; the first
// other word starts the positionals (`tasks --todo "-1 point"` searches for
// `-1 point`), except that Run refuses one shaped like a long flag as an
// unknown flag (`plan new --specc …` is a mistyped flag, not a description). An empty word is no argument: `done ""`
// is a missing pattern, not one that matches every task; empty says one
// was dropped. A value flag without a value, or with an empty one, is a
// usage error.
func splitArgs(cmd Command, args []string) (flags []string, values []verb.Value, positional []string, empty bool, err error) {
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

// longFlag is a word shaped like a long flag: one before the positionals
// that is not the command's is a mistyped flag, not an argument.
var longFlag = regexp.MustCompile(`^--[A-Za-z0-9][A-Za-z0-9-]*$`)

// leading takes the flags and value flags off the front of args.
func leading(cmd Command, args []string) (rest, flags []string, values []verb.Value, err error) {
	for len(args) > 0 {
		switch {
		case slices.Contains(cmd.Flags, args[0]):
			flags, args = append(flags, args[0]), args[1:]
		case slices.Contains(cmd.values(), args[0]):
			// The value is the next word, unless that is one of the
			// command's own flags: `--rules --yes` is a missing value,
			// not a file named --yes.
			if len(args) < 2 || args[1] == "" || slices.Contains(cmd.Flags, args[1]) || slices.Contains(cmd.values(), args[1]) {
				return nil, nil, nil, fmt.Errorf("%s needs a value", args[0])
			}
			values, args = append(values, verb.Value{Flag: args[0], Value: args[1]}), args[2:]
		default:
			return args, flags, values, nil
		}
	}
	return nil, flags, values, nil
}

// emptyBefore: an empty word comes before a given one. A command whose
// arguments have places would read the later word in the empty one's
// (`plan new "" +proj` as the description `+proj`), so that is a usage
// error; for a command taking any number of words the places do not
// matter, but for its Placed leading ones (`swarm close "" 7` is not #7).
func emptyBefore(cmd Command, args []string) bool {
	args, _, _, _ = leading(cmd, args)
	if cmd.Max == Unbounded {
		return slices.Contains(args[:min(cmd.Placed, len(args))], "")
	}
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

// invoke resolves the workspace when the command needs one and runs fn,
// under ReadOnly when the table marks the command so; the error it
// returns is the message for stderr.
func invoke(c *verb.Context, cmd Command, fn func(*verb.Context) error) int {
	if cmd.Workspace {
		e, err := env.Resolve(c.Ops)
		if err != nil {
			fmt.Fprintln(c.Stderr, err)
			return Failure
		}
		c.Env = e
	}
	if cmd.ReadOnly {
		c.Ops = ops.ReadOnly(c.Ops)
	}
	if err := fn(c); err != nil {
		if errors.Is(err, verb.ErrReported) {
			return Failure
		}
		if errors.Is(err, swarmcmd.ErrInterrupted) {
			return Interrupted
		}
		var u verb.UsageError
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

// writeUsage prints the command's usage, its description and an example
// (a command's own help: the tree has no room for one per row).
func writeUsage(w io.Writer, cmd Command) {
	fmt.Fprintf(w, "Usage: daisy %s\n\n%s\n\nExample: daisy %s\n", cmd.Usage, cmd.description(), cmd.Example)
	if cmd.Commits {
		fmt.Fprintf(w, "\nIt commits: %s \"Name <email>\", leading its arguments, ends the commit with that Co-Authored-By trailer; without the flag, DAISY_CO_AUTHOR's.\n", commands.CoAuthorFlag)
	}
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
				fmt.Fprintf(&b, "  %s\n  %-*s%s\n", c.Usage, helpColumn, "", c.description())
			} else {
				fmt.Fprintf(&b, "  %-*s%s\n", helpColumn, c.Usage, c.description())
			}
		}
	}
	b.WriteString("\nEvery command also accepts --help/-h (as the sole argument) to print its usage.\n")
	b.WriteString("Every command that commits (its usage says so) also takes --co-author \"Name <email>\".\n")
	b.WriteString("\nEnvironment:\n  DAISY_ROOT    Path to the daisy repository (required)\n")
	io.WriteString(w, b.String())
}
