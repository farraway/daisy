// Package runner is the swarm's seam to Docker (docs/swarm.md sections
// 7.1, 7.2 and 13): one container per run, a member or a check, started
// with its argv and a wall clock, the running set read from `docker ps`,
// and the login slots a run takes. Docker is the real runner over
// Ops.Start and Ops.Exec; Fake records launches and answers with canned
// output.
package runner

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"daisy/internal/ops"
)

// Mount is one bind mount.
type Mount struct {
	Src, Dst string
	ReadOnly bool
}

// Spec is one container as section 7.2 writes it: a member, or with
// NoNetwork and no credential, a check.
type Spec struct {
	Name, Label, User string // --name, the daisy.swarm label, --user uid:gid
	// Credential is a slot's `<provider>/<j>` and Slot its directory,
	// mounted at /credential; EnvVar and EnvValue an env credential, read
	// by docker from its standard input. At most one of the two.
	Credential, Slot string
	EnvVar, EnvValue string
	NoNetwork        bool
	Mounts           []Mount
	Workdir, Image   string
	Argv             []string
}

// Args is the `docker` argv, section 7.2's lines in order; it holds no
// secret value.
func (s Spec) Args() []string {
	a := []string{"run", "--rm", "--pull", "never", "--name", s.Name, "--label", "daisy.swarm=" + s.Label, "--user", s.User,
		"--security-opt", "no-new-privileges", "--stop-timeout", "30"}
	if s.NoNetwork {
		a = append(a, "--network", "none")
	}
	if s.EnvVar != "" {
		a = append(a, "--env-file", "/dev/stdin")
	}
	if s.Credential != "" {
		a = append(a, "--label", "daisy.credential="+s.Credential, "--mount", "type=bind,src="+s.Slot+",dst=/credential")
	}
	for _, m := range s.Mounts {
		v := "type=bind,src=" + m.Src + ",dst=" + m.Dst
		if m.ReadOnly {
			v += ",readonly"
		}
		a = append(a, "--mount", v)
	}
	a = append(a, "--workdir", s.Workdir, s.Image)
	return append(a, s.Argv...)
}

// Exit is how a container ended: docker run's exit status, whether the
// wall clock stopped it, and its two streams.
type Exit struct {
	Code           int
	TimedOut       bool
	Stdout, Stderr []byte
}

// Container is a started container; Wait returns once `docker run` has.
type Container interface{ Wait() (Exit, error) }

// Running is one container of `docker ps`: its name and its two labels.
type Running struct{ Name, Swarm, Credential string }

// Runner starts containers, lists the running set and stops one.
type Runner interface {
	// Start runs s; timeout is its wall clock, at which it is stopped.
	Start(s Spec, timeout time.Duration) (Container, error)
	// Running is every container labelled daisy.swarm, of every swarm.
	Running() ([]Running, error)
	// Stop is `docker stop --time 30 <name>`: the wall clock's stop, and
	// an orphan's (section 7.7).
	Stop(name string) error
}

// Docker is the runner over the docker CLI.
type Docker struct {
	O   ops.Ops
	Dir string // where docker is run from
}

var _ Runner = Docker{}

// Start starts `docker run` with the env credential on its standard input
// as one line, and a timer of timeout: when it fires, `docker stop --time
// 30` (section 7.2, the wall clock).
func (d Docker) Start(s Spec, timeout time.Duration) (Container, error) {
	var stdin *strings.Reader
	if s.EnvVar != "" {
		stdin = strings.NewReader(s.EnvVar + "=" + s.EnvValue + "\n")
	}
	c := &dockerRun{d: d, name: s.Name, timeout: timeout}
	var err error
	if stdin != nil {
		c.p, err = d.O.Start(d.Dir, "docker", stdin, &c.stdout, &c.stderr, s.Args()...)
	} else {
		c.p, err = d.O.Start(d.Dir, "docker", nil, &c.stdout, &c.stderr, s.Args()...)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

type dockerRun struct {
	d              Docker
	name           string
	timeout        time.Duration
	p              ops.Process
	stdout, stderr bytes.Buffer
}

func (c *dockerRun) Wait() (Exit, error) {
	type result struct {
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() { code, err := c.p.Wait(); done <- result{code, err} }()
	var r result
	timedOut := false
	select {
	case r = <-done:
	case <-c.d.O.After(c.timeout):
		timedOut = true
		if err := c.d.Stop(c.name); err != nil {
			return Exit{}, err
		}
		r = <-done
	}
	if r.err != nil {
		return Exit{}, r.err
	}
	return Exit{Code: r.code, TimedOut: timedOut, Stdout: c.stdout.Bytes(), Stderr: c.stderr.Bytes()}, nil
}

// Stop sends the container its stop signal and, after 30 seconds, kills it.
func (d Docker) Stop(name string) error {
	if _, errb, err := d.O.Exec(d.Dir, "docker", "stop", "--time", "30", name); err != nil {
		return fmt.Errorf("docker stop %s: %v %s", name, err, strings.TrimSpace(string(errb)))
	}
	return nil
}

// psFormat is the running set's columns: name, daisy.swarm, daisy.credential.
const psFormat = `{{.Names}} {{.Label "daisy.swarm"}} {{.Label "daisy.credential"}}`

// Running is `docker ps --filter label=daisy.swarm` (section 11, apply's
// pass, step 3); a failure carries docker's first line of error.
func (d Docker) Running() ([]Running, error) {
	stdout, stderr, err := d.O.Exec(d.Dir, "docker", "ps", "--filter", "label=daisy.swarm", "--format", psFormat)
	if err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(string(stderr)), "\n")
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("docker: %s", msg)
	}
	var out []Running
	for _, line := range strings.Split(string(stdout), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		r := Running{Name: f[0], Swarm: f[1]}
		if len(f) > 2 {
			r.Credential = f[2]
		}
		out = append(out, r)
	}
	return out, nil
}
