package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/metrics"
)

// csvRow is a metrics CSV row: the cells named, every other empty.
func csvRow(cells map[string]string) string {
	row := make([]string, len(metrics.Header))
	for i, col := range metrics.Header {
		row[i] = cells[col]
	}
	return strings.Join(row, ",") + "\n"
}

// Step 10's claim (the plan remediating the 2026-11-15 design review,
// P1b): each read-only command `--json` was added to, status, list,
// tasks, projects, metrics, swarm list and swarm journal, prints through
// the cli one JSON document on stdout, decoded here whole against a
// literal (every key, null for an absent value, [] for none, the lines
// the text writes on stderr under warnings, stderr empty); a failure is
// the text form's exit with no document; `swarm journal --json --mark`
// is a usage error, and a command without the flag refuses it.
func TestReadOnlyJSON(t *testing.T) {
	m := newMachine(t)
	real, err := filepath.EvalSymlinks(m.home)
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(m.ws, "chessgrid.260807")
	writeFile(t, filepath.Join(proj, ".daisy", "home"), "testhome\n")
	for _, l := range env.Links {
		os.Symlink(filepath.Join(m.home, l.Target), filepath.Join(proj, ".daisy", l.Link))
	}

	// The day: today's counts, the backlog, the tasks and the projects.
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), "### 2026-11-16 Monday\n\n#### Tasks\n\n"+
		"- [ ] Write the spec +chessgrid\n- [x] Water the plants +garden\n- [ ] Call the vet\n\n"+
		"#### Log\n\n- 0900 New day started\n- 1015 Spec drafted\n\n#### Retrospective\n- 1100 not a log entry\n")
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) 2026-11-01 Write the spec +chessgrid\n(B) 2026-11-02 Water the plants +garden\nx 2026-11-03 2026-11-01 Order pieces +chessgrid\n")
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "x 2026-10-01 2026-09-01 Ship v1 +chessgrid\n")
	writeFile(t, filepath.Join(m.home, "projects", "chessgrid.md"), "# chessgrid\n")
	writeFile(t, filepath.Join(m.home, "projects", "garden.md"), "# garden\n")
	writeFile(t, filepath.Join(m.home, "projects", "_archive", "old.md"), "# old\n")
	writeFile(t, filepath.Join(m.home, "plans", "p.plan.md"), "## The Plan\n\nbody\n")
	os.Symlink(filepath.Join(m.home, "plans", "p.plan.md"), filepath.Join(proj, "PLAN.md"))

	// The prompts and skills: always loaded, a full override the root has
	// moved past, a lazy prompt with no trigger, one missing.
	writeFile(t, filepath.Join(m.home, "include.txt"), "full\n~lazy\n~notrig\nmissing\n")
	writeFile(t, filepath.Join(m.root, "prompts", "full.md"), "# Full\n")
	writeFile(t, filepath.Join(m.root, "prompts", "lazy.md"), "## Trigger\nroot's\n")
	writeFile(t, filepath.Join(m.root, "prompts", "notrig.md"), "# none\n")
	writeFile(t, filepath.Join(m.home, "prompts", "lazy.md"), "# Lazy\n\n## Trigger\n\n- When a lazy thing happens.\n\n## Rules\n")
	writeFile(t, filepath.Join(m.home, "feedback", ".overrides"), "prompts/lazy.md 0000\n")
	writeFile(t, filepath.Join(m.vars["HOME"], ".claude", "skills", "rooty", "SKILL.md"), "---\ndescription: Long: root.\nshort_description: \"Root.\"\n---\n")
	writeFile(t, filepath.Join(m.vars["HOME"], ".claude", "skills", "both", "SKILL.md"), "---\ndescription: user copy\n---\n")
	writeFile(t, filepath.Join(proj, ".claude", "skills", "both", "SKILL.md"), "---\ndescription: workspace copy\n---\n")

	// The metrics: a row before --since, two plan steps (one measured)
	// and a swarm run with no model.
	csv := filepath.Join(m.home, "metrics", "metrics.csv")
	writeFile(t, csv, strings.Join(metrics.Header, ",")+"\n"+
		csvRow(map[string]string{"when": "2026-10-01T10:00:00Z", "kind": "plan-step", "plan_or_swarm": "p", "step_or_run": "0", "role": "step", "model": "claude-opus-5-5", "tokens_total": "999"})+
		csvRow(map[string]string{"when": "2026-11-10T10:00:00Z", "kind": "plan-step", "plan_or_swarm": "p", "step_or_run": "1", "role": "step", "model": "claude-opus-5-5", "tokens_total": "1200", "cost_reported": "1.50",
			"code_added": "10", "code_removed": "2", "tests_added": "5", "tests_removed": "0", "prose_added": "3", "prose_removed": "1", "binary_files": "0"})+
		csvRow(map[string]string{"when": "2026-11-11T10:00:00Z", "kind": "plan-step", "plan_or_swarm": "p", "step_or_run": "2", "role": "step", "model": "claude-opus-5-5"})+
		csvRow(map[string]string{"when": "2026-11-12T10:00:00Z", "kind": "swarm-run", "plan_or_swarm": "acme/swarm-app", "step_or_run": "1-architect-1", "role": "architect", "cost_estimated": "0.25"}))

	// The swarm: its clone, its state's home, pause and recorded clone,
	// two days of the journal (one line still queued) and the mark.
	clone := m.swarm(t, newGitea(t))
	state := filepath.Join(m.local, "swarm", "forge.example.net", "acme", "swarm-app")
	writeFile(t, filepath.Join(state, "home"), "testhome\n")
	writeFile(t, filepath.Join(state, "paused"), "paused 2026-11-06T15:00:00Z: swarm fault: docker gone\n")
	writeFile(t, filepath.Join(state, "clone"), clone+"\n")
	writeFile(t, filepath.Join(state, "notes", "journal", "2026-11-05.md"), "# 2026-11-05\n\n- 0900 1-architect-1: posted; the plan\n")
	writeFile(t, filepath.Join(state, "notes", "journal", "2026-11-06.md"), "# 2026-11-06\n\n- 1501 2-worker-1: posted; pull request #3\n")
	writeFile(t, filepath.Join(state, "journal"), "2026-11-06 - 1502 3-owner-1: posted; the batch\n")
	seen := filepath.Join(state, "connect-seen")
	writeFile(t, seen, "1-architect-1 2026-11-05T10:00:00Z\n")
	elsewhere := t.TempDir()

	paths := strings.NewReplacer("<real>", real, "<home>", m.home, "<root>", m.root, "<proj>", proj, "<ws>", m.ws,
		"<local>", m.local, "<clone>", clone, "<elsewhere>", elsewhere, "<seen>", seen)
	for _, c := range []struct {
		name  string
		cwd   string
		args  []string
		setup func()
		code  int
		want  string // the document; for a failure, stderr's start
	}{
		{name: "status", cwd: proj, args: []string{"status", "--json"}, want: `{
			"home": "testhome", "identity": "testhome", "workspace": "<proj>",
			"workspace_projects": {"names": ["chessgrid"], "source": "convention"},
			"today": {"pending": 2, "completed": 1, "log_entries": 2},
			"backlog": 2,
			"plan": {"title": "The Plan", "path": "<real>/plans/p.plan.md"},
			"warnings": []}`},
		{name: "tasks", cwd: proj, args: []string{"tasks", "--json"}, want: `{
			"tasks": ["(A) 2026-11-01 Write the spec +chessgrid", "x 2026-11-03 2026-11-01 Order pieces +chessgrid", "x 2026-10-01 2026-09-01 Ship v1 +chessgrid"],
			"warnings": []}`},
		{name: "tasks with no project", cwd: m.ws, args: []string{"tasks", "--todo", "--json"}, want: `{
			"tasks": ["(A) 2026-11-01 Write the spec +chessgrid", "(B) 2026-11-02 Water the plants +garden", "x 2026-11-03 2026-11-01 Order pieces +chessgrid"],
			"warnings": ["⚠️  This workspace resolves to no project; listing every project's tasks.", "  Bind it by creating .daisy/project, or pass --all to ask for this deliberately."]}`},
		{name: "tasks matching none", cwd: proj, args: []string{"tasks", "--json", "nomatch"}, code: 1, want: "No tasks match: nomatch\n"},
		{name: "projects", cwd: proj, args: []string{"projects", "--json"}, want: `{
			"archived": false,
			"projects": [{"name": "chessgrid", "path": "<real>/projects/chessgrid.md"}, {"name": "garden", "path": "<real>/projects/garden.md"}],
			"warnings": []}`},
		{name: "projects archived", cwd: proj, args: []string{"projects", "--archived", "--json"}, want: `{
			"archived": true, "projects": [{"name": "old", "path": "<real>/projects/_archive/old.md"}], "warnings": []}`},
		{name: "list", cwd: proj, args: []string{"list", "--json"}, want: `{
			"home": "testhome",
			"prompts": [
				{"name": "full", "lazy": false, "trigger": null, "missing": null, "override": null, "behind": false},
				{"name": "lazy", "lazy": true, "trigger": "When a lazy thing happens.", "missing": null, "override": "full", "behind": true},
				{"name": "notrig", "lazy": true, "trigger": null, "missing": null, "override": null, "behind": false},
				{"name": "missing", "lazy": false, "trigger": null, "missing": "<root>/prompts/missing.md", "override": null, "behind": false}],
			"skills": [
				{"name": "both", "description": "workspace copy", "override": null, "behind": false},
				{"name": "rooty", "description": "Root.", "override": null, "behind": false}],
			"warnings": []}`},
		{name: "metrics", cwd: proj, args: []string{"metrics", "--since", "2026-11-01T00:00:00Z", "--json"}, want: `{
			"file": "<home>/metrics/metrics.csv", "exists": true,
			"rows_taken": {"swarm": null, "since": "2026-11-01T00:00:00Z"},
			"rows": 3, "superseded": 0,
			"groups": [
				{"kind": "plan-step", "model": "claude-opus-5-5", "rows": 2, "tokens": 1200, "rows_with_tokens": 1, "cost": 1.5, "rows_with_cost": 1,
				 "estimated": null, "rows_with_estimated": 0, "measured": 1,
				 "code": {"added": 10, "removed": 2}, "tests": {"added": 5, "removed": 0}, "prose": {"added": 3, "removed": 1}},
				{"kind": "swarm-run", "model": null, "rows": 1, "tokens": null, "rows_with_tokens": 0, "cost": null, "rows_with_cost": 0,
				 "estimated": 0.25, "rows_with_estimated": 1, "measured": 0, "code": null, "tests": null, "prose": null}],
			"warnings": []}`},
		{name: "swarm list", cwd: clone, args: []string{"swarm", "list", "--json"}, want: `{
			"dir": "<local>/swarm",
			"swarms": [{"repository": "acme/swarm-app", "host": "forge.example.net", "clone": {"path": "<clone>", "current": true}, "home": "testhome",
				"paused": {"since": "2026-11-06T15:00:00Z", "reason": "swarm fault: docker gone"}, "here": "<clone>"}],
			"warnings": []}`},
		{name: "swarm journal", cwd: clone, args: []string{"swarm", "journal", "--json"}, want: `{
			"repository": "acme/swarm-app",
			"marked": {"run": "1-architect-1", "at": "2026-11-05T10:00:00Z"},
			"day": null,
			"lines": [
				{"day": "2026-11-06", "run": "2-worker-1", "line": "- 1501 2-worker-1: posted; pull request #3"},
				{"day": "2026-11-06", "run": "3-owner-1", "line": "- 1502 3-owner-1: posted; the batch"}],
			"warnings": []}`},
		{name: "swarm journal --mark", cwd: clone, args: []string{"swarm", "journal", "--json", "--mark", "3-owner-1"}, code: 2,
			want: "Error: --json prints the journal and moves no mark; give --mark alone\nUsage: daisy swarm journal [--json|--mark <run>]\n"},
		{name: "a command without --json", cwd: proj, args: []string{"home", "list", "--json"}, code: 2, want: "Error: unknown flag: --json\n"},

		// The absent values: no today.md, no plan, no project, drift; no
		// CSV; a lost mark; a clone moved, no home, not paused, not here.
		{name: "status with nothing", cwd: m.ws, args: []string{"status", "--json"}, setup: func() {
			os.Remove(filepath.Join(m.home, "journal", "today.md"))
			os.Remove(filepath.Join(m.ws, ".daisy", "today.md"))
		}, want: `{
			"home": "testhome", "identity": "testhome", "workspace": "<ws>",
			"workspace_projects": {"names": [], "source": "none"},
			"today": null, "backlog": 2, "plan": null,
			"warnings": ["⚠️  the binding link .daisy/today.md is missing or does not point into the home; re-run 'daisy init testhome'"]}`},
		{name: "metrics with no CSV", cwd: proj, args: []string{"metrics", "--json"}, setup: func() { os.Remove(csv) }, want: `{
			"file": "<home>/metrics/metrics.csv", "exists": false, "rows_taken": {"swarm": null, "since": null},
			"rows": 0, "superseded": 0, "groups": [], "warnings": []}`},
		{name: "swarm journal, the mark lost", cwd: clone, args: []string{"swarm", "journal", "--json"}, setup: func() { writeFile(t, seen, "9-gone-1 2026-11-05T10:00:00Z\n") }, want: `{
			"repository": "acme/swarm-app", "marked": null, "day": "2026-11-06",
			"lines": [
				{"day": "2026-11-06", "run": "2-worker-1", "line": "- 1501 2-worker-1: posted; pull request #3"},
				{"day": "2026-11-06", "run": "3-owner-1", "line": "- 1502 3-owner-1: posted; the batch"}],
			"warnings": ["⚠ <seen> names 9-gone-1, which the journal does not hold: showing its newest day"]}`},
		{name: "swarm list, the clone moved", cwd: elsewhere, args: []string{"swarm", "list", "--json"}, setup: func() {
			writeFile(t, filepath.Join(state, "clone"), elsewhere+"\n")
			os.Remove(filepath.Join(state, "home"))
			os.Remove(filepath.Join(state, "paused"))
		}, want: `{
			"dir": "<local>/swarm",
			"swarms": [{"repository": "acme/swarm-app", "host": "forge.example.net", "clone": {"path": "<elsewhere>", "current": false}, "home": null, "paused": null, "here": null}],
			"warnings": []}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.setup != nil {
				c.setup()
			}
			code, out, errs := m.run(t, c.cwd, c.args...)
			want := paths.Replace(c.want)
			if c.code != 0 {
				if code != c.code || out != "" || !strings.HasPrefix(errs, want) {
					t.Errorf("exit %d out %q err %q, want exit %d and stderr starting %q", code, out, errs, c.code, want)
				}
				return
			}
			var got, doc any
			if err := json.Unmarshal([]byte(want), &doc); err != nil {
				t.Fatalf("the expected document: %v", err)
			}
			if code != 0 || errs != "" || json.Unmarshal([]byte(out), &got) != nil || !reflect.DeepEqual(got, doc) {
				t.Errorf("exit %d err %q out:\n%s\nwant:\n%s", code, errs, out, want)
			}
		})
	}
}
