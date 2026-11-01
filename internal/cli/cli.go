// Package cli is the only place that knows argument syntax: the command
// tree, flag parsing, --help, usage errors (exit 2, usage on stderr) and the
// fall-through to daisy.sh for commands the binary has not taken over.
package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
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
		// `help <name>` is the bash's until the step that ports help: the
		// skill lookup, and the note when a skill shadows a command, live
		// there.
		return fallThrough(o, stdin, stdout, stderr, args)
	}

	cmd, ok := lookup(args[0])
	if !ok {
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		writeTree(stderr)
		return Usage
	}
	if cmd.Run == nil {
		return fallThrough(o, stdin, stdout, stderr, args)
	}
	rest := args[1:]
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		writeUsage(stdout, cmd)
		return OK
	}

	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(rest); err != nil {
		return usageError(stderr, cmd, err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, cmd, fmt.Sprintf("unexpected argument: %s", fs.Arg(0)))
	}

	c := &commands.Context{Ops: o, Stdin: stdin, Stdout: stdout, Stderr: stderr, Args: fs.Args()}
	if cmd.Workspace {
		e, err := env.Resolve(o)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return Failure
		}
		c.Env = e
	}
	if err := cmd.Run(c); err != nil {
		fmt.Fprintln(stderr, err)
		return Failure
	}
	return OK
}

func isHelpWord(s string) bool { return s == "help" || s == "--help" || s == "-h" }

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
// port, whichever side of it the command is on. Without DAISY_ROOT the
// daisy.sh beside this binary (bin/daisy) is run, so `daisy install` still
// bootstraps a shell that has nothing set; daisy.sh says what else needs it.
func fallThrough(o ops.Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	root, err := env.Root(o)
	if err != nil && o.Getenv("DAISY_ROOT") == "" {
		if exe, exeErr := o.Executable(); exeErr == nil {
			root, err = filepath.Dir(filepath.Dir(env.RealPath(o.FS(), exe))), nil
		}
	}
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
