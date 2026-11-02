package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/workspace"
)

// Init binds the workspace (the path argument, else the working
// directory) to an identity and its home.
func Init(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	target := "."
	if len(c.Args) == 2 {
		target = c.Args[1]
	}
	if target, err = absolute(c, target); err != nil {
		return err
	}
	return workspace.Init(c.Ops, m, c.Args[0], target, c.Stdout, c.Stderr)
}

// Clean removes Daisy from the workspace the working directory is in,
// after asking unless -f/--force.
func Clean(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	ws, err := env.FindWorkspace(c.Ops.FS(), m.Root, cwd)
	if err != nil {
		return err
	}
	b := workspace.ReadBinding(c.Ops, m, ws)
	if !c.Flag("-f") && !c.Flag("--force") {
		home := b.HomeName
		if home == "" {
			home = "unknown"
		}
		fmt.Fprintf(c.Stdout, "This will remove Daisy from: %s\n  Home: %s\n", ws, home)
		if b.Identity != "" {
			fmt.Fprintf(c.Stdout, "  Identity: %s\n", b.Identity)
		}
		fmt.Fprintln(c.Stdout)
		answer, _ := c.Ops.Prompt(c.Stdin, c.Stderr, "Continue? [y/N] ")
		if a := strings.ToLower(answer); a != "y" && a != "yes" {
			fmt.Fprintln(c.Stdout, "Cancelled.")
			return nil
		}
	}
	return workspace.Clean(c.Ops, m, b, c.Stdout)
}

// absolute is p against the working directory, cleaned.
func absolute(c *Context, p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	return filepath.Join(cwd, p), nil
}
