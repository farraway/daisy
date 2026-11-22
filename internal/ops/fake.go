package ops

import (
	"errors"
	"io"
	"os"
	"runtime/debug"
	"time"
)

// Fake is an Ops whose every operation can be replaced by a function; a nil
// field falls back to Real(), but StartFn, ReexecFn and SignalsFn: a Fake
// starts no process, replaces none and takes no signal it was not given.
// Tests use it to pin the clock, script a git answer, set the environment
// or the working directory, or capture what a command starts. The file system is never faked: tests use real temp
// directories.
type Fake struct {
	ExecFn       func(dir, name string, args ...string) (stdout, stderr []byte, err error)
	StartFn      func(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error)
	NowFn        func() time.Time
	AfterFn      func(d time.Duration) <-chan time.Time
	SignalsFn    func() <-chan os.Signal
	GetenvFn     func(key string) string
	GetwdFn      func() (string, error)
	ExecutableFn func() (string, error)
	BuildInfoFn  func() (*debug.BuildInfo, bool)
	ReexecFn     func(path string, args ...string) error
	HTTPFn       func() Doer
	PromptFn     func(r io.Reader, w io.Writer, question string) (string, error)
	// IsTerminalFn stands a terminal in for a test's buffer.
	IsTerminalFn func(stream any) bool
}

func (f *Fake) FS() FS { return Real().FS() }

func (f *Fake) Exec(dir, name string, args ...string) ([]byte, []byte, error) {
	if f.ExecFn != nil {
		return f.ExecFn(dir, name, args...)
	}
	return Real().Exec(dir, name, args...)
}

// Start is StartFn's process; none set, an error, never Real's: a test
// starts only the processes it names (praxis rule 14), a real one by
// giving Real().Start.
func (f *Fake) Start(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (Process, error) {
	if f.StartFn != nil {
		return f.StartFn(dir, name, stdin, stdout, stderr, args...)
	}
	return nil, errors.New("ops: Fake.Start " + name + " " + joinArgs(args) + ": no StartFn set, so no process started")
}

func (f *Fake) Now() time.Time {
	if f.NowFn != nil {
		return f.NowFn()
	}
	return Real().Now()
}

// Signals is SignalsFn's channel; none set, a channel that never
// receives, not Real's: a test never takes the process's own signals.
func (f *Fake) Signals() <-chan os.Signal {
	if f.SignalsFn != nil {
		return f.SignalsFn()
	}
	return nil
}

func (f *Fake) After(d time.Duration) <-chan time.Time {
	if f.AfterFn != nil {
		return f.AfterFn(d)
	}
	return Real().After(d)
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

func (f *Fake) BuildInfo() (*debug.BuildInfo, bool) {
	if f.BuildInfoFn != nil {
		return f.BuildInfoFn()
	}
	return Real().BuildInfo()
}

// Reexec is ReexecFn's; none set, an error, never Real's: a test never
// replaces its own process. A ReexecFn returning nil stands for an exec
// that succeeded, after which the caller returns as if replaced.
func (f *Fake) Reexec(path string, args ...string) error {
	if f.ReexecFn != nil {
		return f.ReexecFn(path, args...)
	}
	return errors.New("ops: Fake.Reexec without a ReexecFn: a test never replaces its process")
}

func (f *Fake) HTTP() Doer {
	if f.HTTPFn != nil {
		return f.HTTPFn()
	}
	return Real().HTTP()
}

func (f *Fake) IsTerminal(stream any) bool {
	if f.IsTerminalFn != nil {
		return f.IsTerminalFn(stream)
	}
	return Real().IsTerminal(stream)
}

func (f *Fake) Prompt(r io.Reader, w io.Writer, question string) (string, error) {
	if f.PromptFn != nil {
		return f.PromptFn(r, w, question)
	}
	return Real().Prompt(r, w, question)
}
