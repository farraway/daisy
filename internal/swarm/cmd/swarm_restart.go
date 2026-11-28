package cmd

import (
	"crypto/sha256"
	"fmt"
	iofs "io/fs"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/reconcile"
)

// SwarmRestart is `daisy swarm restart` (docs/swarm.md section 11, The
// restart): <state>/restart written, so that the running apply loop
// re-executes itself at the end of its next pass, its members running on
// (section 7.10, rule 2).
func SwarmRestart(c *Context) error {
	return inSwarm(c, func() error {
		return typed(commentVerb{name: "restart", run: func(c *Context, _ opener) error { return restartSwarm(c) }})(c, openForge)
	})
}

func restartSwarm(c *Context) error {
	state, err := swarmStateOnly(c)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	path := reconcile.RestartFile(state)
	if b, err := fs.ReadFile(path); err == nil {
		fmt.Fprintf(c.Stdout, "Restart already requested at %s\n", strings.TrimPrefix(reconcile.FirstLine(b), "restart "))
		return nil
	}
	if err := fs.MkdirAll(state, 0o755); err != nil {
		return fmt.Errorf("Error: swarm: cannot create %s: %v", state, err)
	}
	if err := fs.WriteFile(path, []byte("restart "+c.Ops.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	fmt.Fprintf(c.Stdout, "Restart requested (%s): the running apply re-executes itself at the end of its next pass\n", path)
	return nil
}

// restarter is the loop's restart (section 11, The restart): the path of
// the binary apply runs from, the content it started from, and the file
// at that path as the last pass found it.
type restarter struct {
	path    string
	started [sha256.Size]byte
	seen    fileStamp
	sum     [sha256.Size]byte // the content of the file the last pass found
}

// fileStamp is what a pass compares a file by before reading it: its
// size, its modification time and its change time, which a copy that
// keeps the times still changes (section 7.10, rule 2; triage item L345).
type fileStamp struct{ size, mod, change int64 }

func stampOf(fi iofs.FileInfo) fileStamp {
	return fileStamp{fi.Size(), fi.ModTime().UnixNano(), ops.ChangeTime(fi)}
}

// newRestarter reads the running binary at the loop's start: nil, with a
// warning, when it cannot be read, and that loop never restarts.
func newRestarter(c *Context) *restarter {
	fs := c.Ops.FS()
	path, err := c.Ops.Executable()
	var fi iofs.FileInfo
	var b []byte
	if err == nil {
		fi, err = fs.Stat(path)
	}
	if err == nil {
		b, err = fs.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: the running binary cannot be read (%v); this apply restarts neither onto a new build nor at daisy swarm restart\n", err)
		return nil
	}
	sum := sha256.Sum256(b)
	return &restarter{path: path, started: sum, seen: stampOf(fi), sum: sum}
}

// newBuild: the file at the binary's path holds other content than the
// build apply started from, and has not changed since the last pass. A
// file whose size, modification time or change time changed is read and
// waits a pass,
// so a build still being written is never run, and the same build
// written again is no new one.
func (r *restarter) newBuild(fs ops.FS) (bool, error) {
	fi, err := fs.Stat(r.path)
	if err != nil {
		return false, err
	}
	if st := stampOf(fi); st != r.seen {
		b, err := fs.ReadFile(r.path)
		if err != nil {
			return false, err
		}
		r.seen, r.sum = st, sha256.Sum256(b)
		return false, nil
	}
	return r.sum != r.started, nil
}

// restart is the loop's step after a pass: with a request
// (<state>/restart) or a new build, the request removed, each container
// whose wait returned recorded (Record), the journal line queued and the
// process replaced by the binary at its path with apply's arguments, the
// members running on, for the process it starts to hold by their records
// (section 7.10, rule 2). The lock is released by the exec itself, its
// descriptor being close-on-exec, so a failed exec keeps it. True when
// the process was replaced (only a Fake's Reexec returns then); a failure
// is a warning and the loop goes on.
func (r *restarter) restart(c *Context, s *reconcile.Swarm) bool {
	fs := c.Ops.FS()
	flag := reconcile.RestartFile(s.Cfg.Forge.State(c.Env.Local))
	var why, said []string
	_, err := fs.Lstat(flag)
	requested := err == nil
	if requested {
		b, _ := fs.ReadFile(flag)
		at := "daisy swarm restart at " + strings.TrimPrefix(reconcile.FirstLine(b), "restart ")
		why, said = append(why, at), append(said, at)
	}
	built, err := r.newBuild(fs)
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: %v; the build is checked again at the next pass\n", err)
	}
	if built {
		why, said = append(why, "a new build at "+r.path), append(said, "a new build")
	}
	if len(why) == 0 {
		return false
	}
	held, err := s.Holding()
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: %v; not restarted at this pass\n", err)
		return false
	}
	if s.Stopping() {
		return false // a signal since the pass: the loop ends, not a new process
	}
	if requested {
		if err := fs.Remove(flag); err != nil {
			fmt.Fprintf(c.Stderr, "⚠ restart: %v; not restarted, so that the process it starts does not take the request again\n", err)
			return false
		}
	}
	if err := s.Record(); err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: %v\n", err)
	}
	key := "0-restart-" + c.Ops.Now().UTC().Format("20060102T150405Z")
	if err := s.QueueJournal(key, fmt.Sprintf("apply re-executed (%s), %d running", strings.Join(said, "; "), held)); err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: journal: %v\n", err)
	}
	fmt.Fprintf(c.Stdout, "Restart (%s): %d running; re-executing %s swarm apply\n", strings.Join(why, "; "), held, r.path)
	err = c.Ops.Reexec(r.path, "swarm", "apply")
	if err == nil {
		return true
	}
	fmt.Fprintf(c.Stderr, "⚠ restart: exec %s: %v; apply goes on from the build it started with\n", r.path, err)
	if err := s.QueueJournal(key+"-failed", fmt.Sprintf("exec failed: %v; apply goes on from the build it started with", err)); err != nil {
		fmt.Fprintf(c.Stderr, "⚠ restart: journal: %v\n", err)
	}
	r.started = r.sum // the build refused is tried again only once another replaces it
	return false
}
