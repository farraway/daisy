// Package ops is the one place Daisy touches the outside world: the file
// system, child processes, the clock, the environment, the network and the
// terminal. Everything above it takes an Ops and never calls os, os/exec,
// time.Now or net/http itself, so a command's side effects are whatever its
// Ops allows: Real() allows them all, ReadOnly() refuses every mutation, and
// Fake substitutes any one of them in a test.
package ops

import (
	"io"
	"net/http"
	"os"
	"time"
)

// FS is the file system as Daisy uses it. WriteFile is atomic: the file
// either holds the new content or the old.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	// Mkdir creates one directory and fails when anything is there already.
	Mkdir(path string, perm os.FileMode) error
	Chmod(name string, mode os.FileMode) error
	ReadDir(name string) ([]os.DirEntry, error)
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	// EvalSymlinks is `readlink -f` for a path that exists: every link
	// resolved, the result absolute and clean.
	EvalSymlinks(path string) (string, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error
	// Lock takes an exclusive flock on name without waiting, creating the
	// file, and writes holder and ` pid <pid>` as its one line. Another
	// holder is a *LockedError naming the line it wrote. release drops the
	// lock and leaves the file; the kernel drops it too when the process
	// exits, however it exits.
	Lock(name, holder string) (release func(), err error)
}

// LockedError is a Lock already held: Holder is the line its holder wrote.
type LockedError struct{ Name, Holder string }

func (e *LockedError) Error() string { return e.Name + " is held: " + e.Holder }

// Process is a child started by Ops.Start. Wait returns the exit status
// once it has exited; err is set only when the status is unknown (the
// process was signalled, or could not be waited for).
type Process interface {
	Wait() (code int, err error)
	Kill() error
}

// Doer is the slice of http.Client the forge API needs.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Ops is every side effect in one value.
type Ops interface {
	FS() FS
	// Exec runs a command to completion in dir and returns what it wrote.
	Exec(dir, name string, args ...string) (stdout, stderr []byte, err error)
	// Start runs a command in dir with the given streams attached and
	// returns without waiting. An *os.File passed as a stream is handed to
	// the child as its own descriptor, so a terminal stays a terminal.
	Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error)
	Now() time.Time
	// After is time.After: the channel receives once d has passed.
	After(d time.Duration) <-chan time.Time
	Getenv(key string) string
	Getwd() (string, error)
	// Executable is the path of the running binary, links unresolved.
	Executable() (string, error)
	HTTP() Doer
	// Prompt writes question to w and reads one line from r, without the
	// trailing newline.
	Prompt(r io.Reader, w io.Writer, question string) (string, error)
}

// IsTerminal: r is a terminal, as `[ -t 0 ]` asks — a file the terminal
// driver answers for, not merely a character device (/dev/null is one
// and no terminal). A command asks before prompting on r or before
// waiting for piped input that will not come. A reader that is not a file
// is no terminal.
func IsTerminal(r io.Reader) bool {
	f, ok := r.(interface{ Fd() uintptr })
	return ok && isatty(f.Fd())
}
