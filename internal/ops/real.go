package ops

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Real returns the Ops backed by the operating system.
func Real() Ops { return real{} }

type real struct{}

func (real) FS() FS { return realFS{} }

func (real) Exec(dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = childEnv()
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

func (real) Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = childEnv()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return realProcess{cmd}, nil
}

// childEnv is the environment a child gets: ours, less the variables with
// which a git hook points git at its own repository or author. Every git
// call Daisy makes means that directory's repository and that repository's
// configured author; inherited from a hook, these would redirect it.
func childEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if hookVars[key] || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

var hookVars = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true, "GIT_INDEX_FILE": true, "GIT_OBJECT_DIRECTORY": true,
	"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true, "GIT_AUTHOR_DATE": true,
	"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true, "GIT_COMMITTER_DATE": true,
	"GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true,
}

type realProcess struct{ cmd *exec.Cmd }

func (p realProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 0 {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

func (p realProcess) Kill() error { return p.cmd.Process.Kill() }

func (real) Now() time.Time                         { return time.Now() }
func (real) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (real) Getenv(key string) string               { return os.Getenv(key) }
func (real) Getwd() (string, error)                 { return os.Getwd() }
func (real) Executable() (string, error)            { return os.Executable() }
func (real) HTTP() Doer                             { return http.DefaultClient }

func (real) Prompt(r io.Reader, w io.Writer, question string) (string, error) {
	if _, err := io.WriteString(w, question); err != nil {
		return "", err
	}
	// One byte at a time: a buffered reader would swallow input meant for
	// whatever reads r next.
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n == 1 {
			if buf[0] == '\n' {
				return string(bytes.TrimSuffix(line, []byte{'\r'})), nil
			}
			line = append(line, buf[0])
		}
		if err == io.EOF {
			if len(line) == 0 {
				return "", io.EOF
			}
			return string(line), nil
		}
		if err != nil {
			return "", err
		}
	}
}

type realFS struct{}

func (realFS) ReadFile(name string) ([]byte, error)         { return os.ReadFile(name) }
func (realFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (realFS) Mkdir(path string, perm os.FileMode) error    { return os.Mkdir(path, perm) }
func (realFS) Chmod(name string, mode os.FileMode) error    { return os.Chmod(name, mode) }
func (realFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (realFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (realFS) Lstat(name string) (os.FileInfo, error)       { return os.Lstat(name) }
func (realFS) Symlink(oldname, newname string) error        { return os.Symlink(oldname, newname) }
func (realFS) Readlink(name string) (string, error)         { return os.Readlink(name) }
func (realFS) EvalSymlinks(path string) (string, error)     { return filepath.EvalSymlinks(path) }
func (realFS) Remove(name string) error                     { return os.Remove(name) }
func (realFS) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (realFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }

// WriteFile writes to a sibling temp file and renames it over name, so a
// reader never sees a half-written file. One rule for the mode: a file that
// exists keeps its mode exactly (as a shell's `>` or `>>` keeps it); a new
// file gets perm under the umask (as `>` or `cp` makes one).
func (realFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	keep := false
	if fi, err := os.Stat(name); err == nil && fi.Mode().IsRegular() {
		perm, keep = fi.Mode().Perm(), true
	}
	tmp, err := createSibling(name, perm)
	if err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if keep {
		if err := tmp.Chmod(perm); err != nil {
			return cleanup(err)
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// createSibling creates `.<base>.<random>` beside name, exclusively, with
// perm under the umask.
func createSibling(name string, perm os.FileMode) (*os.File, error) {
	dir, base := filepath.Dir(name), filepath.Base(name)
	for range 100 {
		p := filepath.Join(dir, "."+base+"."+strconv.FormatUint(rand.Uint64(), 36))
		f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if !errors.Is(err, os.ErrExist) {
			return f, err
		}
	}
	return nil, errors.New("no free temporary name")
}
