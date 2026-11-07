package runner

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"daisy/internal/ops"
)

// Canned is one scripted run: what the member writes to out.md ("" writes
// nothing), its exit status, its standard output, and whether the wall
// clock stopped it.
type Canned struct {
	Out      string
	Code     int
	Stdout   string
	TimedOut bool
}

// Fake is a runner that records each launch's Spec and answers it with the
// next Canned scripted for its issue and role (none: no output, exit 0).
// Others is the running set of containers it did not start; a container it
// starts is running until Wait. FS writes the canned out.md (nil:
// ops.Real().FS()).
type Fake struct {
	FS       ops.FS
	mu       sync.Mutex
	Launched []Spec
	Others   []Running
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
		if m.Dst == "/work/.swarm" && c.Out != "" {
			fs := f.FS
			if fs == nil {
				fs = ops.Real().FS()
			}
			if err := fs.WriteFile(filepath.Join(m.Src, "out.md"), []byte(c.Out), 0o644); err != nil {
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

type fakeRun struct {
	f    *Fake
	name string
	c    Canned
}

func (r *fakeRun) Wait() (Exit, error) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	for i, c := range r.f.running {
		if c.Name == r.name {
			r.f.running = append(r.f.running[:i], r.f.running[i+1:]...)
			break
		}
	}
	return Exit{Code: r.c.Code, TimedOut: r.c.TimedOut, Stdout: []byte(r.c.Stdout)}, nil
}
