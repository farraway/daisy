// Package runner is the swarm's seam to Docker (docs/swarm.md sections
// 7.1, 7.2 and 13): one container per run, a member or a check, started
// with its argv and a wall clock, the running set read from `docker ps`,
// and the login slots a run takes. Docker is the real runner over
// Ops.Start and Ops.Exec; Fake records launches and answers with canned
// output. A check or a worker run may have a product's test services
// beside it (Services): a private network and a container per service.
package runner

import (
	"bytes"
	"errors"
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

// Spec is one container as section 7.2 writes it: a member, or with no
// credential, a check or a dependency step.
type Spec struct {
	Name, Label, User string // --name, the daisy.swarm label, --user uid:gid
	// Credential is a slot's `<provider>/<j>` and Slot its directory,
	// mounted at /credential, Home the variable that names the mount as
	// the CLI's home (CODEX_HOME, GROK_HOME); EnvVar and EnvValue an env
	// credential, read by docker from its standard input. At most one of
	// the two.
	Credential, Slot, Home string
	EnvVar, EnvValue       string
	// Network is the container's network: "" Docker's default bridge,
	// "none" the check's without test services, or the network of the
	// run's or check's test services (Services).
	Network string
	// Env is the dependency step's variables and the test services'
	// x-daisy-env, `<name>=<value>` each, one --env each, in order.
	Env            []string
	Mounts         []Mount
	Workdir, Image string
	Argv           []string
}

// Args is the `docker` argv, section 7.2's lines in order; it holds no
// secret value.
func (s Spec) Args() []string {
	a := []string{"run", "--rm", "--pull", "never", "--name", s.Name, "--label", "daisy.swarm=" + s.Label, "--user", s.User,
		"--security-opt", "no-new-privileges", "--stop-timeout", "30"}
	if s.Network != "" {
		a = append(a, "--network", s.Network)
	}
	if s.EnvVar != "" {
		a = append(a, "--env-file", "/dev/stdin")
	}
	if s.Credential != "" {
		a = append(a, "--label", "daisy.credential="+s.Credential, "--mount", "type=bind,"+mountField("src", s.Slot)+","+mountField("dst", "/credential"))
		if s.Home != "" {
			a = append(a, "--env", s.Home+"=/credential")
		}
	}
	for _, e := range s.Env {
		a = append(a, "--env", e)
	}
	for _, m := range s.Mounts {
		v := "type=bind," + mountField("src", m.Src) + "," + mountField("dst", m.Dst)
		if m.ReadOnly {
			v += ",readonly"
		}
		a = append(a, "--mount", v)
	}
	a = append(a, "--workdir", s.Workdir, s.Image)
	return append(a, s.Argv...)
}

// mountField is one key=value of docker's --mount, which docker reads as a
// CSV record: a value holding a comma or a quote is quoted, so a path can
// add no option of its own (`d,src=/etc`).
func mountField(key, value string) string {
	f := key + "=" + value
	if strings.ContainsAny(f, ",\"\r\n") {
		return `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
	}
	return f
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

// Services is a step's private network and a product's test services on
// it (section 7.2): Network names the network, which is the name of the
// container it is made for, and prefixes each service's container,
// `<Network>.<service>`; Internal gives it no route out (a check's). The
// network and the containers carry the label daisy.services=<Network>,
// not daisy.swarm: they are their check's or run's, never counted apart.
type Services struct {
	Network  string
	Internal bool
	List     []Service
}

// Service is one test service: its name, which is its address on the
// network, its image, its tmpfs paths, its environment (`<name>=<value>`)
// and its command, none for the image's own.
type Service struct {
	Name, Image         string
	Tmpfs, Env, Command []string
}

// NetworkArgs is `docker network create`'s argv for s.
func (s Services) NetworkArgs() []string {
	a := []string{"network", "create"}
	if s.Internal {
		a = append(a, "--internal")
	}
	return append(a, "--label", "daisy.services="+s.Network, s.Network)
}

// ServiceArgs is the `docker run` argv of v on s's network: detached, on
// that network alone under v's name, from a local image, as its image's
// user, without new privileges, removed once stopped.
func (s Services) ServiceArgs(v Service) []string {
	a := []string{"run", "--detach", "--rm", "--pull", "never", "--name", s.Network + "." + v.Name, "--label", "daisy.services=" + s.Network,
		"--network", s.Network, "--network-alias", v.Name, "--security-opt", "no-new-privileges"}
	for _, t := range v.Tmpfs {
		a = append(a, "--tmpfs", t)
	}
	for _, e := range v.Env {
		a = append(a, "--env", e)
	}
	return append(append(a, v.Image), v.Command...)
}

// Runner starts containers, lists the running set and stops one.
type Runner interface {
	// Start runs s; timeout is its wall clock, at which it is stopped.
	Start(s Spec, timeout time.Duration) (Container, error)
	// Running is every container labelled daisy.swarm, of every swarm.
	Running() ([]Running, error)
	// Stop is `docker stop --time 30 <name>`: the wall clock's stop, and
	// an orphan's (section 7.7).
	Stop(name string) error
	// Up makes s's network and starts each of its services on it, in
	// order; on an error, what it made is left for Down.
	Up(s Services) error
	// Down removes the network's services and the network, whatever of
	// them exists (an apply's that died included); none is no error.
	Down(network string) error
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
		// A timeout whatever the run exits with (section 7.2), and only
		// once `docker run` has returned, a failed stop included (the
		// container gone already, --rm): the slot's lock is held until then.
		timedOut = true
		c.d.Stop(c.name)
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

// Up runs `docker network create`, then `docker run --detach` for each
// service; a failure names the network or the service, with docker's
// first line of error (a missing image: `No such image: <image>`).
func (d Docker) Up(s Services) error {
	if _, err := d.docker(s.NetworkArgs()...); err != nil {
		return fmt.Errorf("network %s: %v", s.Network, err)
	}
	for _, v := range s.List {
		if _, err := d.docker(s.ServiceArgs(v)...); err != nil {
			return fmt.Errorf("%s: %v", v.Name, err)
		}
	}
	return nil
}

// Down removes, by their label, the network's containers (`docker rm
// --force`) and then the network.
func (d Docker) Down(network string) error {
	label := "label=daisy.services=" + network
	out, err := d.docker("ps", "--all", "--quiet", "--filter", label)
	if err == nil && len(strings.Fields(out)) > 0 {
		_, err = d.docker(append([]string{"rm", "--force"}, strings.Fields(out)...)...)
	}
	if err == nil {
		out, err = d.docker("network", "ls", "--quiet", "--filter", label)
	}
	if err == nil && len(strings.Fields(out)) > 0 {
		_, err = d.docker("network", "rm", network)
	}
	if err != nil {
		return fmt.Errorf("network %s: %v", network, err)
	}
	return nil
}

// docker runs one docker command; its error is docker's first line of
// error, else the exit's.
func (d Docker) docker(args ...string) (string, error) {
	stdout, stderr, err := d.O.Exec(d.Dir, "docker", args...)
	if err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(string(stderr)), "\n")
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return string(stdout), nil
}

// psFormat is the running set's columns: name, daisy.swarm, daisy.credential.
const psFormat = `{{.Names}} {{.Label "daisy.swarm"}} {{.Label "daisy.credential"}}`

// Running is `docker ps --filter label=daisy.swarm` (section 11, apply's
// pass, step 3); a failure carries docker's first line of error.
func (d Docker) Running() ([]Running, error) {
	stdout, err := d.docker("ps", "--filter", "label=daisy.swarm", "--format", psFormat)
	if err != nil {
		return nil, fmt.Errorf("docker: %v", err)
	}
	var out []Running
	for _, line := range strings.Split(stdout, "\n") {
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
