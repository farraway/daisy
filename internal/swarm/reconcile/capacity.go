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
	"daisy/internal/swarm/forge"
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

// mayTake is the slots of e's provider, of count, that e may take: its
// slots, in their order, else every slot, lowest first (section 7.10,
// rule 3).
func mayTake(e declare.Entry, count int) []int {
	if e.Slots != nil {
		return e.Slots
	}
	var out []int
	for j := 1; j <= count; j++ {
		out = append(out, j)
	}
	return out
}

// freeSlots is the slots e may take (mayTake) that no line of capacity
// names with e's model.
func freeSlots(e declare.Entry, count int, capacity []Unavailable) []int {
	var out []int
	for _, j := range mayTake(e, count) {
		if !unavailableOn(capacity, e.Provider+"/"+strconv.Itoa(j), e.Model) {
			out = append(out, j)
		}
	}
	return out
}

// runsOn is the entry a run of role launches on (section 7.10, The
// fallback): the first of the persona and its fallback's, in order, with
// a credential it may take (heldBy); ok false when none has, and the role
// waits. The running set is read once, for the first entry on slots, and
// the runs' tokens once, for the first whose provider declares a budget.
func (s *Swarm) runsOn(role string, capacity []Unavailable) (declare.Entry, bool, error) {
	at := credentials{cfg: s.Cfg, capacity: capacity, now: s.O.Now()}
	readRunning, haveSpent := false, false
	for _, e := range s.Cfg.Personas[role].Entries() {
		key := s.Cfg.Keys[e.Provider]
		var err error
		if key.Slots != nil && !readRunning {
			at.running, err = s.Runner.Running()
			readRunning = true
		}
		if key.Tokens != nil && !haveSpent && err == nil {
			at.spent, err = readSpent(s.O.FS(), s.state())
			haveSpent = true
		}
		if err != nil {
			return declare.Entry{}, false, err
		}
		if at.heldBy(e) == "" {
			return e, true, nil
		}
	}
	return declare.Entry{}, false, nil
}

// credentials is what tells whether a launch may take a credential: the
// lines of <state>/capacity, the running set, each credential's runs and
// their tokens (spent), at now.
type credentials struct {
	cfg      declare.Config
	capacity []Unavailable
	running  []runner.Running
	spent    map[string][]spent
	now      time.Time
}

// heldBy is what keeps entry e from every credential of its provider, ""
// when one is free (section 7.10, rules 1, 3 and 4): an env credential a
// line of capacity names with e's model, `<provider>/env <model>
// unavailable until <time>`, or at its token budget, `<provider>/env at
// its token budget until <time>`; of the slots e may take, each a line
// names with the model or at its budget, likewise, and those a container
// of running holds, `no free slot of <provider> <j>, …`; "; " between
// them.
func (c credentials) heldBy(e declare.Entry) string {
	key := c.cfg.Keys[e.Provider]
	var creds []string
	switch {
	case key.Env != nil:
		creds = []string{e.Provider + "/env"}
	case key.Slots != nil:
		for _, j := range mayTake(e, *key.Slots) {
			creds = append(creds, e.Provider+"/"+strconv.Itoa(j))
		}
	}
	var why, busy []string
	for _, cred := range creds {
		if i := slices.IndexFunc(c.capacity, func(u Unavailable) bool { return u.Credential == cred && u.Model == e.Model }); i >= 0 {
			why = append(why, fmt.Sprintf("%s %s unavailable until %s", cred, e.Model, c.capacity[i].Until))
			continue
		}
		if until, reached := c.budget(key.Tokens, cred).reached(); reached {
			why = append(why, fmt.Sprintf("%s at its token budget until %s", cred, stamp(until)))
			continue
		}
		if slices.ContainsFunc(c.running, func(r runner.Running) bool { return r.Credential == cred }) {
			busy = append(busy, strings.TrimPrefix(cred, e.Provider+"/"))
			continue
		}
		return ""
	}
	if len(busy) > 0 {
		why = append(why, fmt.Sprintf("no free slot of %s %s", e.Provider, strings.Join(busy, ", ")))
	}
	return strings.Join(why, "; ")
}

// spent is a run a credential was given: when it started and the tokens
// its cost line counts, its input (the cache's included) and its output.
type spent struct {
	started time.Time
	tokens  int
}

// readSpent is each credential's runs, `<provider>/env` or
// `<provider>/<j>` by its credential line, with their start and tokens
// (section 7.10, rule 4); a run without a cost line, in flight or
// reporting none, counts no token.
func readSpent(fs ops.FS, state string) (map[string][]spent, error) {
	runs, err := Runs(fs, state)
	if err != nil {
		return nil, err
	}
	out := map[string][]spent{}
	for _, r := range runs {
		lines, err := r.Result(fs)
		if err != nil {
			return nil, err
		}
		l, _ := Has(lines, "credential")
		started, ok := lineTime(lines, "started")
		f := strings.Fields(l)
		if !ok || len(f) != 4 {
			continue
		}
		cred := f[1] + "/" + f[3]
		if f[2] == "env" {
			cred = f[1] + "/env"
		}
		sp := spent{started: started}
		if cl, ok := Has(lines, "cost"); ok {
			if u, ok := readCost(cl); ok {
				in, _ := strconv.Atoi(u.in)
				out, _ := strconv.Atoi(u.out)
				sp.tokens = in + out
			}
		}
		out[cred] = append(out[cred], sp)
	}
	return out, nil
}

// window is a token budget over the runs a credential took in the span
// before now: their tokens and, when they have reached limit, the time
// enough of them leave it to bring the rest under limit.
type window struct {
	span   time.Duration
	limit  *int
	tokens int
	until  time.Time
}

// budget is cred's windows under tokens, the 24 hours and the 7 days
// before now, a window without its limit counting tokens alone; none
// without tokens.
type budget []window

func (c credentials) budget(tokens *declare.Tokens, cred string) budget {
	if tokens == nil {
		return nil
	}
	var b budget
	for _, w := range []window{{span: 24 * time.Hour, limit: tokens.PerDay}, {span: 7 * 24 * time.Hour, limit: tokens.PerWeek}} {
		var in []spent
		for _, sp := range c.spent[cred] {
			if sp.started.After(c.now.Add(-w.span)) && !sp.started.After(c.now) {
				in = append(in, sp)
				w.tokens += sp.tokens
			}
		}
		if w.limit != nil && w.tokens >= *w.limit {
			slices.SortFunc(in, func(a, b spent) int { return a.started.Compare(b.started) })
			left := w.tokens
			for _, sp := range in {
				left -= sp.tokens
				if left < *w.limit {
					w.until = sp.started.Add(w.span)
					break
				}
			}
		}
		b = append(b, w)
	}
	return b
}

// reached: a window of b is at its limit, and until is the latest time
// one of them frees.
func (b budget) reached() (until time.Time, reached bool) {
	for _, w := range b {
		if w.limit != nil && w.tokens >= *w.limit {
			reached = true
			if w.until.After(until) {
				until = w.until
			}
		}
	}
	return until, reached
}

// launchedEntry is the entry run r was launched on, its lines being
// lines: the entry of the role's persona or fallback with the provider,
// model and effort it launched with (launched), for its co-author; the
// persona's when none is (swarm.json changed since).
func (s *Swarm) launchedEntry(r Run, lines []string) declare.Entry {
	prov, model, effort := s.launched(r, lines)
	entries := s.Cfg.Personas[r.Role].Entries()
	for _, e := range entries {
		if e.Is(declare.Entry{Provider: prov, Model: model, Effort: effort}) {
			return e
		}
	}
	return entries[0]
}

// capacityMarker is the capacity note's first line (section 7.10, The
// note): the reconciler's, naming no run, and no counted entry.
const capacityMarker = "<!-- daisy-swarm role=reconciler kind=capacity -->"

// capacityNote posts the capacity note on run r's issue when j launched
// on another entry than the role's newest run there did, that run's
// persona line read, the persona standing for it when the issue has no
// such run (section 7.10, The note): on a fallback, `**reconciler:** the
// <role> runs on <provider> <model> (<effort>) in place of the persona's,
// and, after a colon, the lines of capacity that name an entry passed
// over, `; ` between them; back on the persona, `… again.`. A batch run
// has no issue, and gets none.
func (s *Swarm) capacityNote(j *Job, r Run, capacity []Unavailable) error {
	if j.N == 0 {
		return nil
	}
	entries := s.Cfg.Personas[j.Role].Entries()
	prev := entries[0]
	runs, err := Runs(s.O.FS(), s.state())
	if err != nil {
		return err
	}
	newest := 0
	for _, o := range runs {
		if o.N != j.N || o.Role != j.Role || o.K >= j.K || o.K < newest {
			continue
		}
		lines, err := o.Result(s.O.FS())
		if err != nil {
			return err
		}
		if l, ok := Has(lines, "persona"); ok {
			if f := strings.Fields(l); len(f) == 4 {
				prev, newest = declare.Entry{Provider: f[1], Model: f[2], Effort: f[3]}, o.K
			}
		}
	}
	on := j.RunsOn
	if on.Is(prev) {
		return nil
	}
	what := func(e declare.Entry) string { return fmt.Sprintf("%s %s (%s)", e.Provider, e.Model, e.Effort) }
	text := fmt.Sprintf("**reconciler:** the %s runs on %s again.", j.Role, what(on))
	if p := entries[0]; !on.Is(p) {
		var pairs []string
		for _, e := range entries {
			if e.Is(on) {
				break
			}
			for _, u := range capacity {
				if strings.HasPrefix(u.Credential, e.Provider+"/") && u.Model == e.Model {
					pairs = append(pairs, u.String())
				}
			}
		}
		text = fmt.Sprintf("**reconciler:** the %s runs on %s in place of %s", j.Role, what(on), what(p))
		if len(pairs) > 0 {
			text += ": " + strings.Join(pairs, "; ")
		}
		text += "."
	}
	if err := s.Forge.Comment(forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: j.N}, capacityMarker+"\n"+text); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "#%d capacity note posted\n", j.N)
	return nil
}
