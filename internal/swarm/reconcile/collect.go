package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// maxPost is the longest text posted (section 7.5).
const maxPost = 60000

// collect posts what an ended run produced and moves the label (section
// 7.6), each step first checked on the forge and skipped when done there,
// each step done appended to the result, so a collection interrupted by an
// error is retried whole at the next pass without posting twice. st is
// the run as this process started it (its secrets read at the launch), nil
// for a run found uncollected.
func (s *Swarm) collect(r Run, st *started) error {
	fs := s.O.FS()
	lines, err := r.Result(fs)
	if err != nil {
		return err
	}
	if _, done := Has(lines, "collected"); done || Ended(lines) == "" {
		return nil
	}
	head := fmt.Sprintf("#%d %s run %d: ", r.N, r.Role, r.K)
	end := func(outcome string) error {
		if err := r.Append(fs, "outcome "+outcome, "collected"); err != nil {
			return err
		}
		return fs.RemoveAll(filepath.Join(r.Dir, "work"))
	}
	if l, ok := Has(lines, "fault"); ok {
		fmt.Fprintf(s.Out, "%sfault (%s)\n", head, strings.TrimPrefix(l, "fault: "))
		return end("fault")
	}
	labelLine, _ := Has(lines, "label")
	label := strings.TrimPrefix(labelLine, "label ")
	is, err := s.Forge.Issue(r.N)
	if err != nil {
		return err
	}
	if !is.Open {
		fmt.Fprintf(s.Err, "⚠ #%d closed during the run; nothing posted\n", r.N)
		return end("discarded")
	}
	at := forge.Ref{Repo: repoName(s.Cfg.Forge.Repo), N: r.N}
	step := func(line string) error {
		if err := r.Append(fs, line); err != nil {
			return err
		}
		lines = append(lines, line)
		return nil
	}
	// comment posts body marked for role k unless the issue holds it.
	comment := func(role, text string) error {
		body := marker(role, r.K) + text
		for _, c := range is.Comments {
			m, ok := Swarms(Entry{c.Author, c.Body, c.Created}, s.SwarmLogin)
			if ok && m.Role == role && m.Run == r.K && (role != "reconciler" || c.Body == body) {
				return nil
			}
		}
		if err := s.Forge.Comment(at, body); err != nil {
			return err
		}
		fmt.Fprintln(s.Out, head+"posted comment")
		return step("posted comment")
	}
	relabel := func(from, to string) error {
		switch {
		case slices.Contains(is.Labels, from):
			if err := s.Forge.Relabel(at, from, to); err != nil {
				return err
			}
			fmt.Fprintf(s.Out, "%s%s → %s\n", head, from, to)
			return step("relabelled " + from + " " + to)
		case slices.Contains(is.Labels, to):
			return nil
		}
		now := ""
		for _, l := range is.Labels {
			if strings.HasPrefix(l, "swarm:") {
				now = l
			}
		}
		fmt.Fprintf(s.Err, "⚠ #%d: relabelled to %s during the run; not relabelled\n", r.N, now)
		return nil
	}
	fail := func(reason string) error {
		if _, ok := Has(lines, "failed"); !ok {
			if err := step("failed: " + reason); err != nil {
				return err
			}
		}
		fmt.Fprintf(s.Out, "%sfailed (%s)\n", head, reason)
		note := fmt.Sprintf("%s run %d failed: %s (was %s). Relabel the issue to resume.", r.Role, r.K, reason, label)
		if err := comment("reconciler", note); err != nil {
			return err
		}
		if err := relabel(label, "swarm:blocked"); err != nil {
			return err
		}
		return end("failed")
	}
	if l, ok := Has(lines, "failed"); ok {
		return fail(strings.TrimPrefix(l, "failed: "))
	}
	out, err := fs.ReadFile(filepath.Join(r.Dir, "io", "out.md"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return fail("no output")
	}
	secrets, err := s.secrets(lines, st)
	if err != nil {
		return err
	}
	for _, sec := range secrets {
		if sec != "" && bytes.Contains(out, []byte(sec)) {
			return fail("secret in output: out.md")
		}
	}
	text := postable(string(out), r)
	first, _, _ := strings.Cut(string(out), "\n")
	if strings.HasPrefix(strings.TrimRight(first, "\r "), "BLOCKED:") {
		if err := comment(r.Role, text); err != nil {
			return err
		}
		note := fmt.Sprintf("%s run %d is blocked (was %s). Answer and relabel the issue to resume.", r.Role, r.K, label)
		if err := comment("reconciler", note); err != nil {
			return err
		}
		if err := relabel(label, "swarm:blocked"); err != nil {
			return err
		}
		return end("blocked")
	}
	if r.Role == "architect" {
		if _, bad := SliceRepos(string(out), repoNames(s)); len(bad) > 0 {
			return fail("bad slices: " + numbers(bad))
		}
	}
	if err := comment(r.Role, text); err != nil {
		return err
	}
	if r.Role == "architect" && label == "swarm:ready" {
		to := "swarm:planning"
		if !s.Cfg.Gates.Plan {
			to = "swarm:planned"
		}
		if err := relabel(label, to); err != nil {
			return err
		}
	}
	return end("posted")
}

func repoName(repo string) string { _, r, _ := strings.Cut(repo, "/"); return r }

func repoNames(s *Swarm) []string {
	var out []string
	for _, r := range s.Cfg.Repos {
		out = append(out, r.Name)
	}
	return out
}

// postable is the output as posted: cut at the last line break before
// maxPost bytes, with the note naming the run (section 7.5).
func postable(out string, r Run) string {
	if len(out) <= maxPost {
		return out
	}
	cut := strings.LastIndex(out[:maxPost], "\n")
	if cut < 0 {
		cut = maxPost
	}
	return out[:cut] + "\n[truncated by Daisy; the whole output is in runs/" + filepath.Base(r.Dir) + "]"
}

// secrets is what the member was given (section 7.6): the env value, or
// the slot's long strings read at the launch and the return; a run found
// uncollected reads them as they are now.
func (s *Swarm) secrets(lines []string, st *started) ([]string, error) {
	if st != nil {
		return st.secrets, nil
	}
	l, _ := Has(lines, "credential")
	f := strings.Fields(l)
	switch {
	case len(f) == 4 && f[2] == "env":
		return []string{s.Secret(f[3])}, nil
	case len(f) == 4 && f[2] == "slot":
		sec, err := runner.SlotSecrets(s.O.FS(), filepath.Join(s.slotDir(f[1]), f[3]))
		if err != nil {
			return nil, nil
		}
		return sec, nil
	}
	return nil, nil
}
