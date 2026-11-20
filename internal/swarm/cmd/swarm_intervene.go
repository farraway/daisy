package cmd

import (
	"fmt"
	"slices"
	"strings"

	"daisy/internal/swarm/reconcile"
)

// typeFlag names the type of the maintainer's intervention a verb makes
// (docs/swarm.md section 11, The interventions' record).
const typeFlag = "--type"

// issueFlag names the issue an intervention no verb makes was on.
const issueFlag = "--issue"

// SwarmIntervene is `daisy swarm intervene --type <type> [--issue <n>]
// <note...>`: the record of an intervention no verb makes, a hand edit
// of swarm.json or a hand-published patch, on issue <n> or on none; it
// writes nothing else, and reads no forge, so <n> is not checked there.
func SwarmIntervene(c *Context) error {
	return inSwarm(c, func() error { return swarmIntervene(c) })
}

func swarmIntervene(c *Context) error {
	typ, given := c.Value(typeFlag)
	if !given {
		return UsageError("swarm intervene takes --type <type>, one of " + reconcile.InterventionTypes())
	}
	if err := checkType(typ); err != nil {
		return err
	}
	n := 0 // no issue
	if word, given := c.Value(issueFlag); given {
		if len(slices.DeleteFunc(slices.Clone(c.Values), func(v Value) bool { return v.Flag != issueFlag })) > 1 {
			return UsageError("--issue is given twice")
		}
		var err error
		if n, err = parseIssue(word); err != nil {
			return err
		}
	}
	note := strings.Join(strings.Fields(strings.Join(c.Args, " ")), " ")
	if note == "" {
		return UsageError("swarm intervene takes a note after --type <type>, saying what you did")
	}
	s, err := recorder(c)
	if err != nil {
		return err
	}
	return record(c, s, reconcile.Intervention{Type: typ, Verb: "intervene", Note: note, N: n})
}

// checkType refuses a type outside the catalogue's, as usage.
func checkType(typ string) error {
	if !reconcile.IsInterventionType(typ) {
		return UsageError(fmt.Sprintf("--type takes one of %s, not %q", reconcile.InterventionTypes(), typ))
	}
	return nil
}

// recorder is the swarm an intervention is recorded in, its record's
// metrics CSV checked usable, before any write.
func recorder(c *Context) (*reconcile.Swarm, error) {
	cfg, err := forgeOnly(c)
	if err != nil {
		return nil, err
	}
	s := &reconcile.Swarm{O: c.Ops, Place: c.Swarm, WS: c.Swarm.Root, Root: c.Env.Root, Config: c.Env.Config, Local: c.Env.Local, Cfg: cfg, Out: c.Stdout, Err: c.Stderr}
	if _, err := s.InterventionCSV(); err != nil {
		return nil, fmt.Errorf("Error: swarm: --type: %v; nothing changed", err)
	}
	return s, nil
}

// record records i in s and says so.
func record(c *Context, s *reconcile.Swarm, i reconcile.Intervention) error {
	run, err := s.RecordIntervention(i)
	if err != nil {
		return fmt.Errorf("Error: swarm: intervention not recorded: %v", err)
	}
	fmt.Fprintf(c.Stdout, "Intervention %s recorded: %s\n", run, i.Type)
	return nil
}

// typed is the maintainer's verb v with its --type <type>: the type read
// leading, as the cli reads a value flag, or among the value flags right
// after <n> (typeAfter; v.flags the verb's own), checked, the words left
// after <n> checked against what v takes (the table's arity, which counts
// --type among them), and the record's CSV checked usable, before the
// verb runs; once the verb returns with its writes made, the intervention
// is recorded, on issue <n> or, for the whole swarm's pause, resume and
// restart, on none. Without --type it is the verb as it is.
func typed(v commentVerb) func(*Context, opener) error {
	name, flags, verb := v.name, v.flags, v.run
	return func(c *Context, open opener) error {
		typ, lead := c.Value(typeFlag)
		args := c.Args
		if len(args) > 0 {
			after, given, rest, err := typeAfter(args[1:], flags)
			if err != nil {
				return err
			}
			if given && lead {
				return UsageError("--type is given twice")
			}
			if given {
				typ, lead = after, true
			}
			args = append([]string{args[0]}, rest...)
			words := len(rest)
			for _, f := range flags {
				if _, ok := c.Value(f); ok {
					words += 2 // a flag of the verb's, leading, and its value
				}
			}
			if v.takes == nil || !v.takes(words) {
				return UsageError(fmt.Sprintf("swarm %s takes <n> [--type <type>]%s", name, v.args))
			}
		}
		if !lead {
			return verb(c, open)
		}
		if err := checkType(typ); err != nil {
			return err
		}
		n := 0
		if len(args) > 0 {
			var err error
			if n, err = parseIssue(args[0]); err != nil {
				return err
			}
		}
		s, err := recorder(c)
		if err != nil {
			return err
		}
		sub := *c
		sub.Args = args
		sub.Values = slices.DeleteFunc(slices.Clone(c.Values), func(v Value) bool { return v.Flag == typeFlag })
		if err := verb(&sub, open); err != nil {
			return err
		}
		return record(c, s, reconcile.Intervention{Type: typ, Verb: name, N: n})
	}
}

// typeAfter reads --type <type> from the run of value flags that leads
// args (the words after <n>), the verb's own flags passed over with their
// values: the type, whether it was given, and args without it. --type
// with no word after it, or given twice, is usage.
func typeAfter(args, flags []string) (typ string, given bool, rest []string, err error) {
	i := 0
	for i < len(args) && (args[i] == typeFlag || slices.Contains(flags, args[i])) {
		if args[i] != typeFlag {
			rest = append(rest, args[i:min(i+2, len(args))]...)
			i += 2
			continue
		}
		if i+1 == len(args) {
			return "", false, nil, UsageError("--type takes a type, one of " + reconcile.InterventionTypes())
		}
		if given {
			return "", false, nil, UsageError("--type is given twice")
		}
		typ, given = args[i+1], true
		i += 2
	}
	return typ, given, append(rest, args[min(i, len(args)):]...), nil
}
