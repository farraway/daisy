package ops

import (
	"io"
	"time"
)

// Fake is an Ops whose every operation can be replaced by a function; a nil
// field falls back to Real(). Tests use it to pin the clock, script a git
// answer, set the environment or the working directory, or capture what a
// command starts. The file system is never faked: tests use real temp
// directories.
type Fake struct {
	ExecFn       func(dir, name string, args ...string) (stdout, stderr []byte, err error)
	StartFn      func(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error)
	NowFn        func() time.Time
	GetenvFn     func(key string) string
	GetwdFn      func() (string, error)
	ExecutableFn func() (string, error)
	HTTPFn       func() Doer
	PromptFn     func(r io.Reader, w io.Writer, question string) (string, error)
}

func (f *Fake) FS() FS { return Real().FS() }

func (f *Fake) Exec(dir, name string, args ...string) ([]byte, []byte, error) {
	if f.ExecFn != nil {
		return f.ExecFn(dir, name, args...)
	}
	return Real().Exec(dir, name, args...)
}

func (f *Fake) Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error) {
	if f.StartFn != nil {
		return f.StartFn(dir, name, stdin, stdout, stderr, args...)
	}
	return Real().Start(dir, name, stdin, stdout, stderr, args...)
}

func (f *Fake) Now() time.Time {
	if f.NowFn != nil {
		return f.NowFn()
	}
	return Real().Now()
}

func (f *Fake) Getenv(key string) string {
	if f.GetenvFn != nil {
		return f.GetenvFn(key)
	}
	return Real().Getenv(key)
}

func (f *Fake) Getwd() (string, error) {
	if f.GetwdFn != nil {
		return f.GetwdFn()
	}
	return Real().Getwd()
}

func (f *Fake) Executable() (string, error) {
	if f.ExecutableFn != nil {
		return f.ExecutableFn()
	}
	return Real().Executable()
}

func (f *Fake) HTTP() Doer {
	if f.HTTPFn != nil {
		return f.HTTPFn()
	}
	return Real().HTTP()
}

func (f *Fake) Prompt(r io.Reader, w io.Writer, question string) (string, error) {
	if f.PromptFn != nil {
		return f.PromptFn(r, w, question)
	}
	return Real().Prompt(r, w, question)
}
