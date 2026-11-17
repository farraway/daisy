package reconcile

import (
	"io"
	"maps"
	"slices"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// Briefing is what `daisy swarm connect` reads on the forge (docs/swarm.md
// section 11): status's report, the gate note of each issue waiting for
// the maintainer, by issue (CurrentGateNote; an issue without one is not
// a key), and the open questions the swarm answers, by issue.
type Briefing struct {
	Status    *Status
	GateNotes map[int]string
	Questions []Question
}

// Question is an open issue carrying swarm:question (section 6, The
// question): the role that answers it (Answerer) and the newest of the
// swarm's questions on it, nil when none is (the flag set by hand).
type Question struct {
	Issue    int    `json:"issue"`
	Answerer string `json:"answerer"`
	Asked    *Asked `json:"asked"`
}

// Asked is a member's question: its role, its run, when it was posted
// (RFC 3339) and its text after the marker and role lines.
type Asked struct {
	Role string `json:"role"`
	Run  int    `json:"run"`
	At   string `json:"at"`
	Text string `json:"text"`
}

// ReadBriefing makes the reads ReadStatus makes and returns the briefing
// from them: no forge call of its own.
func ReadBriefing(o ops.Ops, at declare.Place, cfg declare.Config, local string, f forge.Adapter, warn io.Writer) (*Briefing, error) {
	s, read, err := readStatus(o, at, cfg, local, f, warn)
	if err != nil {
		return nil, err
	}
	b := &Briefing{Status: s, GateNotes: map[int]string{}, Questions: []Question{}}
	for _, w := range s.Waiting.Issues {
		if text, ok := CurrentGateNote(read.issues[w.Issue].Issue, read.login); ok {
			b.GateNotes[w.Issue] = text
		}
	}
	for _, n := range slices.Sorted(maps.Keys(read.issues)) {
		is := read.issues[n].Issue
		if !is.Open || !slices.Contains(is.Labels, forge.Question) {
			continue
		}
		q := Question{Issue: n, Answerer: Answerer(is, read.login)}
		if e, ok := newestQuestion(is, read.login); ok {
			m, _ := Swarms(e, read.login)
			q.Asked = &Asked{m.Role, m.Run, e.Created.UTC().Format(time.RFC3339), swarmText(e.Body)}
		}
		b.Questions = append(b.Questions, q)
	}
	return b, nil
}
