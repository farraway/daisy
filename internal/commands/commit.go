package commands

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/env"
	"daisy/internal/gitx"
)

// Commit commits the home's repository once with the message, or with
// --root the root's, leaving out every registered home inside it (whose
// changes stay staged for their own commit). Nothing to commit is not an
// error. A message that looks like a flag is refused: `commit --home` and
// `--not-home` are gone, and must not become a message.
func Commit(c *Context) error {
	message := c.Args[0]
	if strings.HasPrefix(message, "-") {
		return UsageError("unknown flag: " + message)
	}
	if c.Flag("--root") {
		return commitRoot(c, message)
	}
	repo, err := homeRepo(c)
	if err != nil {
		return err
	}
	hash, committed, err := repo.Commit(c.Ops, message)
	if err != nil {
		return err
	}
	reportCommit(c, message, hash, committed)
	return nil
}

func commitRoot(c *Context, message string) error {
	if !gitx.IsRepoRoot(c.Ops, c.Env.Root) {
		return errors.New("Error: DAISY_ROOT is not a git repository")
	}
	repo := gitx.Repo{Dir: c.Env.Root, Exclude: env.RootExclusions(c.Ops.FS(), c.Env)}
	hash, committed, err := repo.Commit(c.Ops, message)
	if err != nil {
		return err
	}
	if !committed {
		label := "root"
		if len(repo.Exclude) > 0 {
			label = "root (excluding " + strings.Join(repo.Exclude, ", ") + ")"
		}
		fmt.Fprintf(c.Stderr, "No changes to commit in %s\n", label)
		return nil
	}
	fmt.Fprintf(c.Stdout, "📝 Committed: %s (%s)\n", message, hash)
	return nil
}
