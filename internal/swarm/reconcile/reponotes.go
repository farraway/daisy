package reconcile

import (
	"fmt"
	"strings"

	"daisy/internal/swarm/declare"
)

// RepoNote is the swarm's notes about one declared repository,
// repos/<name>.md in the swarm repository at the pass's commit (section
// 4), which system.md carries under `# Repository notes: <name>`.
type RepoNote struct{ Name, Text string }

// RepoNotesFile is the path of the swarm's notes about the declared
// repository name, in the swarm repository.
func RepoNotesFile(name string) string { return "repos/" + name + ".md" }

// noteRepos is the declared repositories whose notes the run reads
// (section 4): the owner's and the architect's every one, in repos'
// order; the worker's and a reviewer's the one it works in.
func (s *Swarm) noteRepos(j *Job, repo declare.Repo) []string {
	if j.Role == "owner" || j.Role == "architect" {
		return repoNames(s)
	}
	if repo.Name == "" {
		return nil
	}
	return []string{repo.Name}
}

// repoNotes reads repos/<name>.md for each of names in the swarm
// repository's mirror at the pass's commit: a file absent, or no regular
// file there (a link, a directory), gives none, as does a swarm
// repository with no commit fetched.
func (s *Swarm) repoNotes(names []string) ([]RepoNote, error) {
	g := s.git()
	own := repoName(s.Cfg.Forge.Repo)
	commit := s.commits[own]
	if commit == "" {
		return nil, nil
	}
	var out []RepoNote
	for _, name := range names {
		p := RepoNotesFile(name)
		entries, err := g.Tree(g.Mirror(own), commit, p)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Path != p || e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755") {
				continue
			}
			data, err := g.Blob(g.Mirror(own), e.Object)
			if err != nil {
				return nil, err
			}
			out = append(out, RepoNote{Name: name, Text: string(data)})
		}
	}
	return out, nil
}

// RepoNotesSeed is the repos/<name>.md `init` writes when it is absent
// (section 4): the heading, what the file is for, and the readiness facts
// init found, instructions the product's own instruction files at the
// base branch (AGENTS.md, CLAUDE.md, those it has), and the repository's
// check and protected paths as swarm.json declares them.
func RepoNotesSeed(c declare.Config, r declare.Repo, instructions []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", r.Name)
	fmt.Fprintf(&b, "The swarm's notes about %s/%s. Daisy adds this file to the system prompt of every member working in %s, under `# Repository notes: %s`; the repository itself carries no swarm material. Replace what follows with what a member should know about it.\n\n",
		owner(c), r.Name, r.Name, r.Name)
	b.WriteString("What `daisy swarm init` found:\n\n")
	if len(instructions) == 0 {
		fmt.Fprintf(&b, "- Instructions: no AGENTS.md or CLAUDE.md on %s; members start without the product's instructions.\n", c.Base)
	} else {
		fmt.Fprintf(&b, "- Instructions: %s on %s.\n", strings.Join(instructions, " and "), c.Base)
	}
	if len(r.Check) == 0 {
		b.WriteString("- Check: none declared in swarm.json; no member's work is executed.\n")
	} else {
		fmt.Fprintf(&b, "- Check: `%s`, as swarm.json declares it.\n", shellQuote(r.Check))
	}
	if len(r.Protected) == 0 {
		b.WriteString("- Protected: none declared in swarm.json.\n")
	} else {
		fmt.Fprintf(&b, "- Protected: `%s`, as swarm.json declares them.\n", strings.Join(r.Protected, "`, `"))
	}
	return b.String()
}
