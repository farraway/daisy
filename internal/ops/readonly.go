package ops

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"time"
)

// ReadOnly wraps inner so that every mutating FS call, every Exec other than
// a git read, every Start and every HTTP request returns an error naming the
// call. The commands whose usage promises "read-only" run under it, so the
// promise is checked rather than trusted.
func ReadOnly(inner Ops) Ops { return readOnly{inner} }

type readOnly struct{ inner Ops }

func refused(call string, args ...string) error {
	return fmt.Errorf("ops: %s %s refused: this command runs read-only", call, joinArgs(args))
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func (r readOnly) FS() FS { return readOnlyFS{r.inner.FS()} }

// gitReadVerbs are the git subcommands that change nothing in a repository.
var gitReadVerbs = []string{"status", "log", "rev-parse", "ls-files", "diff", "show", "cat-file"}

// gitConfigReads are the config flags that only read.
var gitConfigReads = []string{"--get", "--get-all", "--get-regexp", "--list", "-l"}

func gitReads(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if slices.Contains(gitReadVerbs, args[0]) {
		return true
	}
	if args[0] == "config" {
		return slices.ContainsFunc(args[1:], func(a string) bool { return slices.Contains(gitConfigReads, a) })
	}
	return false
}

func (r readOnly) Exec(dir, name string, args ...string) ([]byte, []byte, error) {
	if name != "git" || !gitReads(args) {
		return nil, nil, refused("Exec", append([]string{name}, args...)...)
	}
	return r.inner.Exec(dir, name, args...)
}

func (r readOnly) Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error) {
	return nil, refused("Start", append([]string{name}, args...)...)
}

func (r readOnly) Now() time.Time              { return r.inner.Now() }
func (r readOnly) Getenv(key string) string    { return r.inner.Getenv(key) }
func (r readOnly) Getwd() (string, error)      { return r.inner.Getwd() }
func (r readOnly) Executable() (string, error) { return r.inner.Executable() }
func (r readOnly) HTTP() Doer                  { return refusingDoer{} }

func (r readOnly) Prompt(in io.Reader, out io.Writer, question string) (string, error) {
	return r.inner.Prompt(in, out, question)
}

type refusingDoer struct{}

func (refusingDoer) Do(req *http.Request) (*http.Response, error) {
	url := ""
	if req != nil && req.URL != nil {
		url = req.URL.String()
	}
	return nil, refused("HTTP", url)
}

type readOnlyFS struct{ inner FS }

func (f readOnlyFS) ReadFile(name string) ([]byte, error)       { return f.inner.ReadFile(name) }
func (f readOnlyFS) ReadDir(name string) ([]os.DirEntry, error) { return f.inner.ReadDir(name) }
func (f readOnlyFS) Stat(name string) (os.FileInfo, error)      { return f.inner.Stat(name) }
func (f readOnlyFS) Lstat(name string) (os.FileInfo, error)     { return f.inner.Lstat(name) }
func (f readOnlyFS) Readlink(name string) (string, error)       { return f.inner.Readlink(name) }
func (f readOnlyFS) EvalSymlinks(path string) (string, error)   { return f.inner.EvalSymlinks(path) }

func (readOnlyFS) WriteFile(name string, _ []byte, _ os.FileMode) error {
	return refused("WriteFile", name)
}
func (readOnlyFS) MkdirAll(path string, _ os.FileMode) error { return refused("MkdirAll", path) }
func (readOnlyFS) Symlink(oldname, newname string) error     { return refused("Symlink", oldname, newname) }
func (readOnlyFS) Remove(name string) error                  { return refused("Remove", name) }
func (readOnlyFS) RemoveAll(path string) error               { return refused("RemoveAll", path) }
func (readOnlyFS) Rename(oldpath, newpath string) error      { return refused("Rename", oldpath, newpath) }
