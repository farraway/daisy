package ops

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Real returns the Ops backed by the operating system.
func Real() Ops { return real{} }

type real struct{}

func (real) FS() FS { return realFS{} }

func (real) Exec(dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

func (real) Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return realProcess{cmd}, nil
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

func (real) Now() time.Time              { return time.Now() }
func (real) Getenv(key string) string    { return os.Getenv(key) }
func (real) Getwd() (string, error)      { return os.Getwd() }
func (real) Executable() (string, error) { return os.Executable() }
func (real) HTTP() Doer                  { return http.DefaultClient }

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
// reader never sees a half-written file.
func (realFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Chmod(perm); err != nil {
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
