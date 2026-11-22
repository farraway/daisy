package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/provider"
	"daisy/internal/swarm/runner"
)

// Unavailable is a line of <state>/capacity (section 7.10, The
// fallback): a model unavailable on a credential, `<provider>/<credential>
// <model> until <time>: <reason>`, Credential `<provider>/env` or
// `<provider>/<j>` for slot j, Until the reset the fault's message stated
// (RFC 3339) or `resume` when it stated none, Reason the message.
type Unavailable struct {
	Credential, Model, Until, Reason string
}

func (u Unavailable) String() string {
	return fmt.Sprintf("%s %s until %s: %s", u.Credential, u.Model, u.Until, u.Reason)
}

// untilResume is a line's Until when its fault stated no reset: the bare
// `swarm resume` alone removes it (section 11).
const untilResume = "resume"

// CapacityFile is <state>/capacity, which the bare `swarm resume`
// removes.
func CapacityFile(state string) string { return filepath.Join(state, "capacity") }

// ReadCapacity is <state>/capacity's lines, none when it is absent; a
// line not of Unavailable's form is an error naming it.
func ReadCapacity(fs ops.FS, state string) ([]Unavailable, error) {
	b, err := fs.ReadFile(CapacityFile(state))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Unavailable
	for i, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		f := strings.SplitN(l, " ", 4)
		var until, reason string
		ok := len(f) == 4 && f[2] == "until" && strings.Contains(f[0], "/")
		if ok {
			until, reason, ok = strings.Cut(f[3], ": ")
		}
		if !ok {
			return nil, fmt.Errorf("%s: line %d is not `<provider>/<credential> <model> until <time>: <reason>`", CapacityFile(state), i+1)
		}
		out = append(out, Unavailable{f[0], f[1], until, reason})
	}
	return out, nil
}

// writeCapacity writes lines as <state>/capacity, removing it when none
// is left.
func (s *Swarm) writeCapacity(lines []Unavailable) error {
	fs := s.O.FS()
	if len(lines) == 0 {
		if err := fs.Remove(CapacityFile(s.state())); err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return err
		}
		return nil
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.String() + "\n")
	}
	return fs.WriteFile(CapacityFile(s.state()), []byte(b.String()), 0o644)
}

// unavailableOn: a line of lines names the credential with the model.
func unavailableOn(lines []Unavailable, credential, model string) bool {
	return slices.ContainsFunc(lines, func(u Unavailable) bool { return u.Credential == credential && u.Model == model })
}

// capacityFault is the message of a fault: line's reason when it is a
// provider's fault for a limit, a quota or the credit (section 7.5, The
// stop's cause: `capacity`).
func capacityFault(reason string) (string, bool) {
	msg, ok := strings.CutPrefix(reason, "provider: ")
	return msg, ok && provider.Capacity(msg)
}

// isCapacity: a fault: line's reason is a capacity fault.
func isCapacity(reason string) bool { _, ok := capacityFault(reason); return ok }

// markCapacity is a faulted run's collection when its fault is a capacity
// fault (section 7.10, The fallback): its line written to <state>/capacity,
// the credential its credential line names and the model its persona
// line names, until the reset its message states (the adapter's Reset,
// from the run's end) or `resume`, replacing any line for the same
// credential and model. A run without either line (launched by an older
// Daisy) writes none.
func (c *collection) markCapacity() error {
	l, _ := Has(c.lines, "fault")
	msg, ok := capacityFault(strings.TrimPrefix(l, "fault: "))
	if !ok {
		return nil
	}
	cred, okCred := Has(c.lines, "credential")
	persona, okPersona := Has(c.lines, "persona")
	cf, pf := strings.Fields(cred), strings.Fields(persona)
	if !okCred || !okPersona || len(cf) != 4 || len(pf) != 4 {
		return nil
	}
	credential := cf[1] + "/" + cf[3] // `credential <p> slot <j>`
	if cf[2] == "env" {
		credential = cf[1] + "/env"
	}
	u := Unavailable{Credential: credential, Model: pf[2], Until: untilResume, Reason: msg}
	if p := provider.Providers[cf[1]]; p.Adapter != nil {
		at, _ := endTime(c.lines)
		if t, ok := p.Adapter.Reset(msg, at); ok {
			u.Until = stamp(t)
		}
	}
	s := c.s
	lines, err := ReadCapacity(s.O.FS(), s.state())
	if err != nil {
		return err
	}
	lines = slices.DeleteFunc(lines, func(l Unavailable) bool { return l.Credential == u.Credential && l.Model == u.Model })
	return s.writeCapacity(append(lines, u))
}

// expireCapacity removes each line of <state>/capacity whose reset has
// passed, printing `Capacity: <credential> <model> available again`
// (section 7.10, The fallback); a line until `resume` stays.
func (s *Swarm) expireCapacity() error {
	lines, err := ReadCapacity(s.O.FS(), s.state())
	if err != nil || len(lines) == 0 {
		return err
	}
	now := s.O.Now()
	var keep, gone []Unavailable
	for _, l := range lines {
		if t, err := time.Parse(time.RFC3339, l.Until); err == nil && now.After(t) {
			gone = append(gone, l)
			continue
		}
		keep = append(keep, l)
	}
	if len(gone) == 0 {
		return nil
	}
	if err := s.writeCapacity(keep); err != nil {
		return err
	}
	for _, l := range gone {
		fmt.Fprintf(s.Out, "Capacity: %s %s available again\n", l.Credential, l.Model)
	}
	return nil
}

// freeSlots is the slots of e's provider, of count, that no line of
// capacity names with e's model, lowest first.
func freeSlots(e declare.Entry, count int, capacity []Unavailable) []int {
	var out []int
	for j := 1; j <= count; j++ {
		if !unavailableOn(capacity, e.Provider+"/"+strconv.Itoa(j), e.Model) {
			out = append(out, j)
		}
	}
	return out
}

// runsOn is the entry a run of role launches on (section 7.10, The
// fallback): the first of the persona and its fallback's, in order, with
// a credential it may take, an env credential no line of capacity names
// with the entry's model, or a slot of its provider no running container
// holds and no line names with the model; ok false when none has, and
// the role waits.
func (s *Swarm) runsOn(role string, capacity []Unavailable) (declare.Entry, bool, error) {
	var running []runner.Running
	read := false
	for _, e := range s.Cfg.Personas[role].Entries() {
		key := s.Cfg.Keys[e.Provider]
		if key.Env != nil {
			if !unavailableOn(capacity, e.Provider+"/env", e.Model) {
				return e, true, nil
			}
			continue
		}
		if key.Slots == nil {
			continue
		}
		if !read {
			var err error
			if running, err = s.Runner.Running(); err != nil {
				return declare.Entry{}, false, err
			}
			read = true
		}
		for _, j := range freeSlots(e, *key.Slots, capacity) {
			cred := e.Provider + "/" + strconv.Itoa(j)
			if !slices.ContainsFunc(running, func(c runner.Running) bool { return c.Credential == cred }) {
				return e, true, nil
			}
		}
	}
	return declare.Entry{}, false, nil
}

// launchedEntry is the entry run r was launched on, its lines being
// lines: the entry of the role's persona or fallback with the provider,
// model and effort it launched with (launched), for its co-author; the
// persona's when none is (swarm.json changed since).
func (s *Swarm) launchedEntry(r Run, lines []string) declare.Entry {
	prov, model, effort := s.launched(r, lines)
	entries := s.Cfg.Personas[r.Role].Entries()
	for _, e := range entries {
		if e.Provider == prov && e.Model == model && e.Effort == effort {
			return e
		}
	}
	return entries[0]
}
