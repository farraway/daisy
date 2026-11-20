package runner

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"daisy/internal/ops"
)

// Canned is one scripted run: what the member writes to out.md ("" writes
// nothing), what else it writes or reads there (IO, given the directory
// mounted at /job/io: a worker's screenshots, a reviewer's copy of
// them), what it does in its checkouts (Work, given the directory mounted
// at /job/work, which holds each at <name>: a worker's commits) and in its
// copy of the research notes (Notes, given the directory mounted at
// /job/notes: an architect's notes) and what it reads of its resources
// (Resources, given the directory mounted at /job/resources), its exit
// status, its standard output and error, and whether the wall clock
// stopped it. A check is scripted the same way under the role "check",
// its Stdout and Stderr its output, its Work given its clone, mounted at
// /work.
type Canned struct {
	Out            string
	IO             func(io string) error
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
// writes the canned out.md (nil: ops.Real().FS()). Ups is every Services
// Up was given, and Networks those up and not yet down, by network, as
// Docker holds them: a container started on a network that is not up is
// refused, as is Down while a container it started on the network runs,
// and UpErr fails Up once its network is made.
type Fake struct {
	FS       ops.FS
	mu       sync.Mutex
	Launched []Spec
	Others   []Running
	Stopped  []string
	StartErr error
	Ups      []Services
	Networks map[string]Services
	UpErr    error
	canned   map[string][]Canned
	running  []Running
	attached map[string]string // a running container's network, by name
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
	if _, up := f.Networks[s.Network]; s.Network != "" && s.Network != "none" && !up {
		return nil, fmt.Errorf("network %s not found", s.Network)
	}
	f.Launched = append(f.Launched, s)
	_, rest, _ := strings.Cut(s.Label, "#")
	var c Canned
	if q := f.canned[rest]; len(q) > 0 {
		c, f.canned[rest] = q[0], q[1:]
	}
	for _, m := range s.Mounts {
		switch {
		case m.Dst == "/job/io":
			fs := f.FS
			if fs == nil {
				fs = ops.Real().FS()
			}
			if c.Out != "" {
				if err := fs.WriteFile(filepath.Join(m.Src, "out.md"), []byte(c.Out), 0o644); err != nil {
					return nil, err
				}
			}
			if c.IO != nil {
				if err := c.IO(m.Src); err != nil {
					return nil, err
				}
			}
		case (m.Dst == "/job/work" || m.Dst == "/work") && c.Work != nil:
			if err := c.Work(m.Src); err != nil {
				return nil, err
			}
		case m.Dst == "/job/notes" && c.Notes != nil:
			if err := c.Notes(m.Src); err != nil {
				return nil, err
			}
		case m.Dst == "/job/resources" && c.Resources != nil:
			if err := c.Resources(m.Src); err != nil {
				return nil, err
			}
		}
	}
	f.running = append(f.running, Running{Name: s.Name, Swarm: s.Label, Credential: s.Credential})
	if f.attached == nil {
		f.attached = map[string]string{}
	}
	f.attached[s.Name] = s.Network
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

// Up records s and makes its network, refusing one already up.
func (f *Fake) Up(s Services) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Ups = append(f.Ups, s)
	if _, up := f.Networks[s.Network]; up {
		return fmt.Errorf("network %s: network with name %s already exists", s.Network, s.Network)
	}
	if f.Networks == nil {
		f.Networks = map[string]Services{}
	}
	f.Networks[s.Network] = s
	return f.UpErr
}

// Down takes the network down, refusing while a container started on it
// runs (Docker's `network has active endpoints`).
func (f *Fake) Down(network string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.running {
		if f.attached[r.Name] == network {
			return fmt.Errorf("network %s: error while removing network: network %s has active endpoints", network, network)
		}
	}
	delete(f.Networks, network)
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
	delete(r.f.attached, r.name)
	return Exit{Code: r.c.Code, TimedOut: r.c.TimedOut, Stdout: []byte(r.c.Stdout), Stderr: []byte(r.c.Stderr)}, nil
}
