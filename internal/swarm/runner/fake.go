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
// stopped it, which are the files its sh writes (section 7.10, rule 2):
// Stdout, Stderr and, unless the wall clock stopped it, Code in
// io/stdout, io/stderr and io/exit. A check is scripted the same way
// under the role "check", its Stdout then Stderr its output, written with
// its Code to step/out and step/exit, its Work given its clone, mounted
// at /job/work/<name>.
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
	// Now and After are the clock a container Watch holds is stopped by
	// at its deadline; with After nil, none is.
	Now      func() time.Time
	After    func(time.Duration) <-chan time.Time
	canned   map[string][]Canned
	stops    map[string]chan struct{} // a held container's, closed by its Stop
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
			fs := f.fs()
			if c.Out != "" {
				if err := fs.WriteFile(filepath.Join(m.Src, "out.md"), []byte(c.Out), 0o644); err != nil {
					return nil, err
				}
			}
			if err := c.write(fs, m.Src, map[string]string{"stdout": c.Stdout, "stderr": c.Stderr}); err != nil {
				return nil, err
			}
			if c.IO != nil {
				if err := c.IO(m.Src); err != nil {
					return nil, err
				}
			}
		case m.Dst == "/job/step":
			if err := c.write(f.fs(), m.Src, map[string]string{"out": c.Stdout + c.Stderr}); err != nil {
				return nil, err
			}
		case (m.Dst == "/job/work" || strings.HasPrefix(m.Dst, "/job/work/")) && c.Work != nil:
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

func (f *Fake) fs() ops.FS {
	if f.FS == nil {
		return ops.Real().FS()
	}
	return f.FS
}

// write writes files into dir, and the exit status unless the wall clock
// stopped the container.
func (c Canned) write(fs ops.FS, dir string, files map[string]string) error {
	if !c.TimedOut {
		files["exit"] = strconv.Itoa(c.Code) + "\n"
	}
	for name, b := range files {
		if err := fs.WriteFile(filepath.Join(dir, name), []byte(b), 0o644); err != nil {
			return err
		}
	}
	return nil
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
	f.running = slices.DeleteFunc(f.running, func(r Running) bool { return r.Name == name })
	if c := f.stops[name]; c != nil {
		close(c)
		delete(f.stops, name)
	}
	return nil
}

// Stops is a copy of the names Stop was given.
func (f *Fake) Stops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.Stopped)
}

// Watch holds a running container this Fake did not start (one of
// Others): its Wait returns once Stop is given it, or at deadline by
// After, when it is stopped and a timeout.
func (f *Fake) Watch(name string, deadline time.Time) Container {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stops == nil {
		f.stops = map[string]chan struct{}{}
	}
	stop := make(chan struct{})
	f.stops[name] = stop
	var at <-chan time.Time
	if f.After != nil {
		at = f.After(deadline.Sub(f.Now()))
	}
	return heldRun{f, name, stop, at}
}

type heldRun struct {
	f    *Fake
	name string
	stop chan struct{}
	at   <-chan time.Time
}

func (r heldRun) Wait() (Exit, error) {
	select {
	case <-r.stop:
		return Exit{}, nil
	case <-r.at:
		r.f.Stop(r.name)
		return Exit{TimedOut: true}, nil
	}
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
	return Exit{TimedOut: r.c.TimedOut}, nil
}
