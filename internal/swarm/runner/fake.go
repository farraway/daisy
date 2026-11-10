package runner

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"daisy/internal/ops"
)

// Canned is one scripted run: what the member writes to out.md ("" writes
// nothing), what it does in its clone (Work, given the directory mounted
// at /work: a worker's commits) and in its copy of the research notes
// (Notes, given the directory mounted at /work/.notes: an architect's
// notes) and what it reads of its resources (Resources, given the
// directory mounted at /work/.resources), its exit
// status, its standard output and error, and whether the wall clock
// stopped it. A check is scripted the same way under the role "check",
// its Stdout and Stderr its output.
type Canned struct {
	Out            string
	Work           func(work string) error
	Notes          func(notes string) error
	Resources      func(resources string) error
	Code           int
	Stdout, Stderr string
	TimedOut       bool
}

// Fake is a runner that records each launch's Spec and answers it with the
// next Canned scripted for its issue and role (none: no output, exit 0).
// Others is the running set of containers it did not start; a container it
// starts is running until Wait. Stopped is every name Stop was given. FS
// writes the canned out.md (nil: ops.Real().FS()).
type Fake struct {
	FS       ops.FS
	mu       sync.Mutex
	Launched []Spec
	Others   []Running
	Stopped  []string
	StartErr error
	canned   map[string][]Canned
	running  []Running
}

var _ Runner = (*Fake)(nil)

// Script queues runs for issue n's role, in launch order.
func (f *Fake) Script(n int, role string, runs ...Canned) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.canned == nil {
		f.canned = map[string][]Canned{}
	}
	key := strconv.Itoa(n) + "/" + role
	f.canned[key] = append(f.canned[key], runs...)
}

// Launches is a copy of the specs launched so far.
func (f *Fake) Launches() []Spec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.Launched)
}

func (f *Fake) Start(s Spec, _ time.Duration) (Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StartErr != nil {
		return nil, f.StartErr
	}
	f.Launched = append(f.Launched, s)
	_, rest, _ := strings.Cut(s.Label, "#")
	var c Canned
	if q := f.canned[rest]; len(q) > 0 {
		c, f.canned[rest] = q[0], q[1:]
	}
	for _, m := range s.Mounts {
		switch {
		case m.Dst == "/work/.swarm" && c.Out != "":
			fs := f.FS
			if fs == nil {
				fs = ops.Real().FS()
			}
			if err := fs.WriteFile(filepath.Join(m.Src, "out.md"), []byte(c.Out), 0o644); err != nil {
				return nil, err
			}
		case m.Dst == "/work" && c.Work != nil:
			if err := c.Work(m.Src); err != nil {
				return nil, err
			}
		case m.Dst == "/work/.notes" && c.Notes != nil:
			if err := c.Notes(m.Src); err != nil {
				return nil, err
			}
		case m.Dst == "/work/.resources" && c.Resources != nil:
			if err := c.Resources(m.Src); err != nil {
				return nil, err
			}
		}
	}
	f.running = append(f.running, Running{Name: s.Name, Swarm: s.Label, Credential: s.Credential})
	return &fakeRun{f, s.Name, c}, nil
}

func (f *Fake) Running() ([]Running, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(append([]Running(nil), f.Others...), f.running...), nil
}

// Stop records the name and takes the container out of the running set.
func (f *Fake) Stop(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Stopped = append(f.Stopped, name)
	f.Others = slices.DeleteFunc(f.Others, func(r Running) bool { return r.Name == name })
	return nil
}

type fakeRun struct {
	f    *Fake
	name string
	c    Canned
}

func (r *fakeRun) Wait() (Exit, error) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	r.f.running = slices.DeleteFunc(r.f.running, func(c Running) bool { return c.Name == r.name })
	return Exit{Code: r.c.Code, TimedOut: r.c.TimedOut, Stdout: []byte(r.c.Stdout), Stderr: []byte(r.c.Stderr)}, nil
}
