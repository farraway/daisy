package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"daisy/internal/env"
	"daisy/internal/prompt"
)

// planRoot is the root's plan prompt in the override tests: its trigger
// names the file to read, as the shipped prompts' do.
const planRoot = "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/plan.md` when planning.\n\n## Rules\n\n1. a\n"

// overrideLoop is the feedback loop with the root a repository, its plan
// prompt naming itself, and one entry pending for it.
func overrideLoop(t *testing.T) *loop {
	t.Helper()
	l := newLoop(t)
	writeFile(t, filepath.Join(l.root, "prompts", "plan.md"), planRoot)
	writeFile(t, filepath.Join(l.home, "feedback", "feedback.md"), "# Feedback Log\n\n---\n\n## 2026-08-02 1100\n**Workflow:** plan\n\nPlan did the wrong thing.\n")
	l.git(t, "add", "-A")
	l.git(t, "commit", "-q", "-m", "fixture: plan entry")
	l.rootRepo(t)
	return l
}

func (l *loop) wsRead(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(l.ws, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// lastCommitFiles are the paths the home's last commit touched.
func (l *loop) lastCommitFiles(t *testing.T) string {
	files := strings.Fields(l.git(t, "show", "--name-only", "--format=", "HEAD"))
	sort.Strings(files)
	return strings.Join(files, " ")
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// apply --rules on a root target writes a layered override in the home and
// never the root; the merged prompt is materialised in the workspace and
// the stub points there; one home commit holds the override and not the
// materialised file; a second apply changes only the override; the root's
// prompt changed afterwards changes the next build, the home untouched.
func TestApplyRulesOnARootTargetLayers(t *testing.T) {
	l := overrideLoop(t)
	rootBefore := snapshot(t, l.root)
	code, out, errs := l.run("optimize", "apply", "--workflow", "plan", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. a\n2. Plan the right thing.\n"), "--yes")
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, out, errs)
	}
	sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
	if s := l.gitIn(t, l.root, "status", "--porcelain"); s != "" {
		t.Errorf("root status %q", s)
	}
	layer := prompt.Marker + "\n\n## Rules\n\n1. Plan the right thing.\n"
	if got := l.read(t, "prompts/plan.md"); got != layer {
		t.Errorf("override %q", got)
	}
	merged := "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/plan.md` when planning.\n\n## Rules\n\n1. a\n\n2. Plan the right thing.\n"
	if got := l.wsRead(t, ".daisy/prompts/plan.md"); got != merged {
		t.Errorf("materialised %q", got)
	}
	if got := l.wsRead(t, ".daisy/AGENTS.md"); !strings.Contains(got, "Read the full `"+filepath.Join(l.ws, ".daisy", "prompts", "plan.md")+"` when planning.\n") {
		t.Errorf("the stub does not name the materialised prompt:\n%s", got)
	}
	if got := l.read(t, "feedback/archive.md"); !strings.Contains(got, "— `prompts/plan.md` (layered rules)") {
		t.Errorf("provenance:\n%s", got)
	}
	if l.commits(t) != "3" || l.git(t, "status", "--porcelain") != "" {
		t.Errorf("commits %s status %q", l.commits(t), l.git(t, "status", "--porcelain"))
	}
	if got := l.lastCommitFiles(t); got != "feedback/.last-optimized feedback/archive.md feedback/feedback.md prompts/plan.md" {
		t.Errorf("the commit holds %s", got)
	}
	if strings.Contains(out, "commit --root") {
		t.Errorf("the run still points at a root commit:\n%s", out)
	}

	// Again, one more rule: only the override changes, in one commit.
	homeBefore := snapshot(t, l.home)
	code, out, errs = l.run("optimize", "apply", "--workflow", "plan", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. a\n2. Plan the right thing.\n3. Check it.\n"), "--yes")
	if code != 0 {
		t.Fatalf("second apply: exit %d: %s%s", code, out, errs)
	}
	homeAfter := snapshot(t, l.home)
	var changed []string
	for k, v := range homeAfter {
		if rel, _ := filepath.Rel(l.home, k); homeBefore[k] != v && rel != "AGENTS.md" {
			changed = append(changed, rel)
		}
	}
	if strings.Join(changed, " ") != "prompts/plan.md" || l.commits(t) != "4" {
		t.Errorf("second apply changed %v, commits %s", changed, l.commits(t))
	}
	sameFiles(t, "the root", rootBefore, snapshot(t, l.root))

	// Upstream flows through: the root edited, the next build has it.
	writeFile(t, filepath.Join(l.root, "prompts", "plan.md"), planRoot+"2. upstream\n")
	homeBefore = snapshot(t, l.home)
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatalf("build: %s", errs)
	}
	if got := l.wsRead(t, ".daisy/prompts/plan.md"); !strings.HasSuffix(got, "1. a\n2. upstream\n\n3. Plan the right thing.\n4. Check it.\n") {
		t.Errorf("after the root changed: %q", got)
	}
	sameFiles(t, "the home", homeBefore, snapshot(t, l.home))
	// A layered override is never behind, even with a ledger line left
	// from a full one.
	writeFile(t, filepath.Join(l.home, "feedback", ".overrides"), "prompts/plan.md "+sha("old")+"\n")
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "behind") {
		t.Errorf("a layered override warned: %s", errs)
	}
}

// apply --rewrite on a root target copies the root's file to the home and
// edits the copy: the diff is from the root's text, the root's hash is
// recorded, and healthcheck warns once the root moves on. Over a layered
// override, the full one replaces it, and says so.
func TestApplyRewriteOnARootTargetIsFull(t *testing.T) {
	l := overrideLoop(t)
	rootBefore := snapshot(t, l.root)
	full := "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/plan.md` when planning.\n\nMine.\n"
	code, out, errs := l.run("optimize", "apply", "--workflow", "plan", "--rewrite", l.proposal(t, "w.md", full), "--yes")
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, out, errs)
	}
	sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
	if !strings.Contains(out, "-## Rules\n-\n-1. a\n+Mine.\n") {
		t.Errorf("the diff is not from the root's text:\n%s", out)
	}
	if got := l.read(t, "prompts/plan.md"); got != full {
		t.Errorf("override %q", got)
	}
	if got := l.read(t, "feedback/.overrides"); got != "prompts/plan.md "+sha(planRoot)+"\n" {
		t.Errorf(".overrides %q", got)
	}
	if got := l.read(t, "feedback/archive.md"); !strings.Contains(got, "— `prompts/plan.md` (full rewrite)") {
		t.Errorf("provenance:\n%s", got)
	}
	if got := l.wsRead(t, ".daisy/AGENTS.md"); !strings.Contains(got, "Read the full `"+filepath.Join(l.home, "prompts", "plan.md")+"` when planning.\n") {
		t.Errorf("the stub does not name the home's file:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(l.ws, ".daisy", "prompts", "plan.md")); err == nil {
		t.Error("a full override was materialised")
	}
	if got := l.lastCommitFiles(t); got != "feedback/.last-optimized feedback/.overrides feedback/archive.md feedback/feedback.md prompts/plan.md" {
		t.Errorf("the commit holds %s", got)
	}

	// behind: silent while the root is as copied, a warning once it is not.
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "behind") {
		t.Errorf("warned with the root unchanged: %s", errs)
	}
	if _, out, _ := l.run("list"); !strings.Contains(out, "  plan                 Mine. [full]\n") {
		t.Errorf("list:\n%s", out)
	}
	writeFile(t, filepath.Join(l.root, "prompts", "plan.md"), planRoot+"2. upstream\n")
	if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, "Override of prompts/plan.md is behind the root (root changed since the copy)") {
		t.Errorf("no behind warning: %s", errs)
	}
	if _, out, _ := l.run("list"); !strings.Contains(out, "  plan                 Mine. [full, behind]\n") {
		t.Errorf("list:\n%s", out)
	}

	// Over a layered override.
	l = overrideLoop(t)
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. mine\n")
	code, out, errs = l.run("optimize", "apply", "--workflow", "plan", "--rewrite", l.proposal(t, "w.md", full), "--yes")
	if code != 0 || !strings.Contains(out, "replaced by a full override") || !strings.Contains(out, "+Mine.\n") || !strings.Contains(out, "-2. mine\n") {
		t.Errorf("exit %d: %s%s", code, out, errs)
	}
	if got := l.read(t, "feedback/.overrides"); got != "prompts/plan.md "+sha(planRoot)+"\n" {
		t.Errorf(".overrides %q", got)
	}
}

// prompt build: a root prompt is as it was; a layered one is materialised
// beside the workspace's AGENTS.md, byte-equal to the merge, and its stub
// points there; a full one's stub points at the home's file; a home
// prompt's trigger is its own. A home's own build materialises nothing in
// the home and takes the layered prompt in whole.
func TestPromptBuildMaterialisesLayered(t *testing.T) {
	m := newMachine(t)
	for _, n := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(m.root, "prompts", n+".md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/"+n+".md` when "+n+".\n\n## Rules\n\n1. root "+n+"\n")
	}
	writeFile(t, filepath.Join(m.home, "prompts", "b.md"), prompt.Marker+"\n\n## Rules\n\n1. home b\n")
	writeFile(t, filepath.Join(m.home, "prompts", "c.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/c.md` when c.\n\nFull c.\n")
	writeFile(t, filepath.Join(m.home, "prompts", "d.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/home/x/prompts/d.md` when d.\n")
	writeFile(t, filepath.Join(m.home, "include.txt"), "~a\n~b\n~c\n~d\n")
	writeFile(t, filepath.Join(m.ws, ".daisy", "prompts", "gone.md"), "stale\n")
	if code, _, errs := m.run(t, m.ws, "prompt", "build"); code != 0 {
		t.Fatalf("build: %s", errs)
	}
	agents, _ := os.ReadFile(filepath.Join(m.ws, ".daisy", "AGENTS.md"))
	for _, want := range []string{
		"Read the full `" + m.root + "/prompts/a.md` when a.\n",
		"Read the full `" + m.ws + "/.daisy/prompts/b.md` when b.\n",
		"Read the full `" + m.home + "/prompts/c.md` when c.\n",
		"Read the full `" + m.root + "/home/x/prompts/d.md` when d.\n",
	} {
		if !strings.Contains(string(agents), want) {
			t.Errorf("AGENTS.md lacks %q:\n%s", want, agents)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(m.ws, ".daisy", "prompts"))
	if len(entries) != 1 || entries[0].Name() != "b.md" {
		t.Errorf(".daisy/prompts holds %v", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(m.ws, ".daisy", "prompts", "b.md")); string(b) != "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/b.md` when b.\n\n## Rules\n\n1. root b\n\n2. home b\n" {
		t.Errorf("materialised %q", b)
	}

	homeBefore := snapshot(t, m.home)
	if code, out, errs := m.run(t, m.ws, "prompt", "build", "testhome"); code != 0 || !strings.Contains(out, "Full: b\n") {
		t.Fatalf("home build: %s%s", out, errs)
	}
	home, _ := os.ReadFile(filepath.Join(m.home, "AGENTS.md"))
	if !strings.Contains(string(home), "2. home b\n") {
		t.Errorf("the home's build lacks the layered rules:\n%s", home)
	}
	delete(homeBefore, filepath.Join(m.home, "AGENTS.md"))
	after := snapshot(t, m.home)
	delete(after, filepath.Join(m.home, "AGENTS.md"))
	sameFiles(t, "the home", homeBefore, after)

	// A malformed layer fails the build, naming the file and the line.
	writeFile(t, filepath.Join(m.home, "prompts", "b.md"), "intro\n"+prompt.Marker+"\n")
	if code, _, errs := m.run(t, m.ws, "prompt", "build"); code != 1 || !strings.Contains(errs, filepath.Join(m.home, "prompts", "b.md")) || !strings.Contains(errs, "line 1") {
		t.Errorf("malformed: exit %d %q", code, errs)
	}
}

// The briefing marks a root target and an override, and counts the
// merged rules.
func TestOptimizeBriefsOverrides(t *testing.T) {
	l := overrideLoop(t)
	_, out, _ := l.run("optimize", "-w", "plan")
	if !strings.Contains(out, "Target: "+filepath.Join(l.root, "prompts", "plan.md")+" (root — apply writes a home override)\nRules: 1\n") {
		t.Errorf("root target:\n%s", out)
	}
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. mine\n")
	_, out, _ = l.run("optimize", "-w", "plan")
	if !strings.Contains(out, "Target: "+filepath.Join(l.home, "prompts", "plan.md")+" (home override, layered)\nRules: 2\n") || !strings.Contains(out, "--- current ## Rules ---\n## Rules\n\n1. a\n\n2. mine\n--- end ---") {
		t.Errorf("layered:\n%s", out)
	}
	if _, out, _ := l.run("list"); !strings.Contains(out, "  plan                 (lazy, no trigger summary) [layered]\n") {
		t.Errorf("list:\n%s", out)
	}
}

// A skill merges the same way: init copies the root's skill with the
// home's rules in its SKILL.md; help shows the merged rules from the
// source; a vendored root skill stays refused, overridden or not.
func TestSkillOverrides(t *testing.T) {
	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\ndescription: A skill.\n---\n\n## Rules\n\n1. root rule\n")
	writeFile(t, filepath.Join(l.root, "skills", "sk", "ref.md"), "ref\n")
	writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. home rule\n")
	merged := "---\nname: sk\ndescription: A skill.\n---\n\n## Rules\n\n1. root rule\n\n2. home rule\n"
	if code, out, _ := l.run("help", "sk"); code != 0 || !strings.Contains(out, "1. root rule\n\n2. home rule\n") {
		t.Errorf("help: %s", out)
	}
	ws := scratch(t)
	if code, _, errs := runWith(l.ops(), "init", "testhome", ws); code != 0 {
		t.Fatalf("init: %s", errs)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".claude", "skills", "sk", "SKILL.md")); string(b) != merged {
		t.Errorf("installed %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".claude", "skills", "sk", "ref.md")); string(b) != "ref\n" {
		t.Errorf("ref %q", b)
	}
	twins(t, ws)

	// A full override stands on the root's directory too.
	writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), "---\nname: sk\ndescription: Mine.\n---\n")
	ws = scratch(t)
	if code, _, errs := runWith(l.ops(), "init", "testhome", ws); code != 0 {
		t.Fatalf("init: %s", errs)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".claude", "skills", "sk", "ref.md")); string(b) != "ref\n" {
		t.Errorf("full: ref %q", b)
	}
	twins(t, ws)

	writeFile(t, filepath.Join(l.root, "skills", "sk", "LICENSE"), "upstream\n")
	if code, _, errs := l.run("optimize", "apply", "--workflow", "skills/sk", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. root rule\n2. x\n"), "--yes"); code != 1 || !strings.Contains(errs, "vendored") {
		t.Errorf("vendored: exit %d %q", code, errs)
	}
}

// Every override is redirected wherever the build writes the root's path
// for it, included in AGENTS.md or only named by another prompt, lazy or
// whole; every layered one is materialised.
func TestBuildRedirectsEveryOverride(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.root, "prompts", "daisy.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/daisy.md` always.\nFor plans read `$DAISY_ROOT/prompts/praxis.md`.\n\n## Rules\n\n1. root\n")
	writeFile(t, filepath.Join(m.root, "prompts", "praxis.md"), "## Rules\n\n1. root praxis\n")
	writeFile(t, filepath.Join(m.home, "prompts", "daisy.md"), prompt.Marker+"\n\n## Rules\n\n1. home daisy\n")
	writeFile(t, filepath.Join(m.home, "prompts", "praxis.md"), prompt.Marker+"\n\n## Rules\n\n1. home praxis\n")
	writeFile(t, filepath.Join(m.home, "include.txt"), "daisy\n")
	if code, _, errs := m.run(t, m.ws, "prompt", "build"); code != 0 {
		t.Fatalf("build: %s", errs)
	}
	agents, _ := os.ReadFile(filepath.Join(m.ws, ".daisy", "AGENTS.md"))
	mat := filepath.Join(m.ws, ".daisy", "prompts")
	for _, want := range []string{
		"Read the full `" + mat + "/daisy.md` always.\nFor plans read `" + mat + "/praxis.md`.\n",
		"- `" + m.root + "/prompts/praxis.md`: read `" + mat + "/praxis.md` instead (a layered override)\n",
	} {
		if !strings.Contains(string(agents), want) {
			t.Errorf("AGENTS.md lacks %q:\n%s", want, agents)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(mat, "praxis.md")); string(b) != "## Rules\n\n1. root praxis\n\n2. home praxis\n" {
		t.Errorf("praxis not materialised: %q", b)
	}
}

// A layer with no root file to stand on is refused, never used raw; a
// rewrite or rules proposal carrying the marker line is refused.
func TestLayerRefusals(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.home, "prompts", "orphan.md"), prompt.Marker+"\n\n## Rules\n\n1. x\n")
	writeFile(t, filepath.Join(m.home, "include.txt"), "~orphan\n")
	if code, _, errs := m.run(t, m.ws, "prompt", "build"); code != 1 || !strings.Contains(errs, "the root has no prompts/orphan.md") {
		t.Errorf("orphan prompt: exit %d %q", code, errs)
	}
	writeFile(t, filepath.Join(m.home, "include.txt"), "\n")
	os.Remove(filepath.Join(m.home, "prompts", "orphan.md"))
	writeFile(t, filepath.Join(m.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. x\n")
	if code, _, errs := m.run(t, scratch(t), "init", "testhome", "."); code != 1 || !strings.Contains(errs, "the root has no skills/sk/SKILL.md") {
		t.Errorf("orphan skill: exit %d %q", code, errs)
	}

	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n")
	before := snapshot(t, l.home, l.root)
	for _, args := range [][]string{
		{"plan", "--rewrite", l.proposal(t, "w.md", "## Trigger\n\nT.\n\n"+prompt.Marker+"\n")},
		{"skills/sk", "--rewrite", l.proposal(t, "w.md", "---\nname: sk\n---\n"+prompt.Marker+"\n\n## Rules\n\n1. b\n")},
		{"plan", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. a\n2. x\n"+prompt.Marker+"\n")},
	} {
		if code, _, errs := l.run(append([]string{"optimize", "apply", "--workflow"}, append(args, "--yes")...)...); code != 1 || !strings.Contains(errs, prompt.Marker) {
			t.Errorf("%s %s: exit %d %q", args[0], args[1], code, errs)
		}
	}
	sameFiles(t, "refused", before, snapshot(t, l.home, l.root))
}

// A materialised prompt the root has moved past is said by healthcheck,
// and is silent once rebuilt.
func TestHealthcheckSaysAStaleMaterialisedPrompt(t *testing.T) {
	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. home\n")
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatal(errs)
	}
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "run 'daisy init'\n") {
		t.Errorf("warned when current: %s", errs)
	}
	writeFile(t, filepath.Join(l.root, "prompts", "plan.md"), planRoot+"2. upstream\n")
	stale := func(what string) {
		t.Helper()
		if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, "run 'daisy init'\n") {
			t.Errorf("%s: no stale warning: %s", what, errs)
		}
		if code, _, errs := l.run("prompt", "build"); code != 0 {
			t.Fatal(errs)
		}
		if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "run 'daisy init'\n") {
			t.Errorf("%s: warned once rebuilt: %s", what, errs)
		}
	}
	stale("the root moved on")
	// Another workspace's apply turned the layer into a full override.
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/plan.md` when planning.\n\nMine.\n")
	stale("layered to full")
	// The override reverted.
	os.Remove(filepath.Join(l.home, "prompts", "plan.md"))
	stale("removed")
	// A new full override.
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/plan.md` when planning.\n\nMine.\n")
	stale("a new full override")
	// A layered prompt in a subdirectory.
	writeFile(t, filepath.Join(l.root, "prompts", "team", "x.md"), "## Trigger\n\nRead the full `$DAISY_ROOT/prompts/team/x.md` when x.\n\n## Rules\n\n1. r\n")
	writeFile(t, filepath.Join(l.home, "prompts", "team", "x.md"), prompt.Marker+"\n\n## Rules\n\n1. home x\n")
	writeFile(t, filepath.Join(l.home, "include.txt"), "~team/x\n")
	stale("a layer in a subdirectory")
	agents, _ := os.ReadFile(filepath.Join(l.ws, ".daisy", "AGENTS.md"))
	if !strings.Contains(string(agents), "Read the full `"+filepath.Join(l.ws, ".daisy", "prompts", "team", "x.md")+"` when x.") {
		t.Errorf("subdirectory stub:\n%s", agents)
	}
	// An installed skill override the root's skill moved past.
	writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n")
	writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. b\n")
	if code, _, errs := runWith(l.ops(), "init", "testhome", l.ws); code != 0 {
		t.Fatal(errs)
	}
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "daisy init") {
		t.Errorf("skill warned when current: %s", errs)
	}
	writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n2. upstream\n")
	if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, "run 'daisy init testhome'") {
		t.Errorf("no skill warning: %s", errs)
	}
	// The root skill's other files are installed too, and go stale too.
	writeFile(t, filepath.Join(l.root, "skills", "sk", "ref.md"), "ref v1\n")
	if code, _, errs := runWith(l.ops(), "init", "testhome", l.ws); code != 0 {
		t.Fatal(errs)
	}
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "daisy init") {
		t.Errorf("skill warned when current: %s", errs)
	}
	writeFile(t, filepath.Join(l.root, "skills", "sk", "ref.md"), "ref v2\n")
	if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, "run 'daisy init testhome'") {
		t.Errorf("no warning for a stale ref.md: %s", errs)
	}
}

// A malformed layer is said by healthcheck the moment it appears, in a
// workspace that has never had an override.
func TestHealthcheckSaysAMalformedLayer(t *testing.T) {
	l := overrideLoop(t)
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatal(errs)
	}
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), "intro\n"+prompt.Marker+"\n\n## Rules\n\n1. x\n")
	if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, "line 1 ('intro') stands before the marker") {
		t.Errorf("silent: %s", errs)
	}
}

// A link where Daisy writes a merged file is replaced, never written
// through: the root's linked SKILL.md, a link in .daisy/prompts/.
func TestMergedFilesReplaceLinks(t *testing.T) {
	l := overrideLoop(t)
	shared := filepath.Join(l.root, "skills", "shared", "SKILL.md")
	writeFile(t, shared, "---\nname: shared\n---\n\n## Rules\n\n1. root rule\n")
	os.MkdirAll(filepath.Join(l.root, "skills", "sk"), 0o755)
	if err := os.Symlink(shared, filepath.Join(l.root, "skills", "sk", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. home rule\n")
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. home\n")
	os.MkdirAll(filepath.Join(l.ws, ".daisy", "prompts"), 0o755)
	if err := os.Symlink(filepath.Join(l.root, "prompts", "plan.md"), filepath.Join(l.ws, ".daisy", "prompts", "plan.md")); err != nil {
		t.Fatal(err)
	}
	rootBefore := snapshot(t, l.root)
	if code, _, errs := runWith(l.ops(), "init", "testhome", l.ws); code != 0 {
		t.Fatalf("init: %s", errs)
	}
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatalf("build: %s", errs)
	}
	sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
	for _, p := range []string{filepath.Join(l.ws, ".claude", "skills", "sk", "SKILL.md"), filepath.Join(l.ws, ".daisy", "prompts", "plan.md")} {
		if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
			t.Errorf("%s is not a regular file: %v", p, err)
		}
	}
	// init then prompt build over the link: the build's own file replaces it.
	os.Remove(filepath.Join(l.ws, ".daisy", "prompts", "plan.md"))
	os.Symlink(filepath.Join(l.root, "prompts", "plan.md"), filepath.Join(l.ws, ".daisy", "prompts", "plan.md"))
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatalf("build: %s", errs)
	}
	sameFiles(t, "the root after build", rootBefore, snapshot(t, l.root))
}

// A dangling link at a materialised prompt's path is replaced like a live
// one: the build's own file stands there after, nothing written through.
func TestMaterialisedPathOverADanglingLink(t *testing.T) {
	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. home\n")
	os.MkdirAll(filepath.Join(l.ws, ".daisy", "prompts"), 0o755)
	p := filepath.Join(l.ws, ".daisy", "prompts", "plan.md")
	gone := filepath.Join(l.ws, "gone.md")
	os.Symlink(gone, p)
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatalf("build: exit %d: %s", code, errs)
	}
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("%s is not a regular file: %v", p, err)
	}
	if _, err := os.Lstat(gone); err == nil {
		t.Error("the link's target was written")
	}
}

// No write lands in the root through a link on the way: .daisy/prompts a
// link into the root's prompts/, the home's prompt a link to the root's,
// the home's skill directory a link to the root's. Each is refused,
// nothing written.
func TestNoWriteReachesTheRootThroughALink(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(l *loop)
		args []string
	}{
		{"prompts dir link", func(l *loop) {
			writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. home\n")
			os.RemoveAll(filepath.Join(l.ws, ".daisy", "prompts"))
			os.Symlink(filepath.Join(l.root, "prompts"), filepath.Join(l.ws, ".daisy", "prompts"))
		}, []string{"prompt", "build"}},
		{"home prompt link", func(l *loop) {
			os.MkdirAll(filepath.Join(l.home, "prompts"), 0o755)
			os.Symlink(filepath.Join(l.root, "prompts", "plan.md"), filepath.Join(l.home, "prompts", "plan.md"))
		}, []string{"optimize", "apply", "--workflow", "plan", "--rules", "RULES", "--yes"}},
		{"home skill dir link", func(l *loop) {
			writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n")
			os.MkdirAll(filepath.Join(l.home, "skills"), 0o755)
			os.Symlink(filepath.Join(l.root, "skills", "sk"), filepath.Join(l.home, "skills", "sk"))
		}, []string{"optimize", "apply", "--workflow", "skills/sk", "--rules", "RULES", "--yes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := overrideLoop(t)
			tc.set(l)
			rules := l.proposal(t, "r.md", "## Rules\n\n1. a\n2. NEW\n")
			args := append([]string{}, tc.args...)
			for i, a := range args {
				if a == "RULES" {
					args[i] = rules
				}
			}
			rootBefore := snapshot(t, l.root)
			code, _, errs := l.run(args...)
			if code != 1 || !strings.Contains(errs, "leads into the root") {
				t.Errorf("exit %d %q", code, errs)
			}
			sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
		})
	}
}

// A registered home inside the root's directory is not the root (a root
// commit leaves it out): apply and the home's build write it as before.
func TestHomeInsideTheRootIsWritten(t *testing.T) {
	l := overrideLoop(t)
	inside := filepath.Join(l.root, "homes", "testhome")
	os.MkdirAll(filepath.Dir(inside), 0o755)
	if err := os.Rename(l.home, inside); err != nil {
		t.Fatal(err)
	}
	l.home = inside
	writeFile(t, filepath.Join(l.config, "homes"), "testhome="+inside+"\n")
	for _, ln := range env.Links {
		p := filepath.Join(l.ws, ".daisy", ln.Link)
		os.Remove(p)
		os.Symlink(filepath.Join(inside, ln.Target), p)
	}
	code, out, errs := l.run("optimize", "apply", "--workflow", "plan", "--rules", l.proposal(t, "r.md", "## Rules\n\n1. a\n2. x\n"), "--yes")
	if code != 0 || l.commits(t) != "3" {
		t.Errorf("exit %d commits %s: %s%s", code, l.commits(t), out, errs)
	}
	if code, _, errs := l.run("prompt", "build", "testhome"); code != 0 {
		t.Errorf("home build: %s", errs)
	}
}

// init refuses, before any write, a .claude/skills or .agents/skills
// linked into the root: it would remove and rewrite the root's skills.
func TestInitRefusesASkillsLinkIntoTheRoot(t *testing.T) {
	for _, tree := range []string{".claude", ".agents"} {
		l := overrideLoop(t)
		writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n")
		writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. b\n")
		ws := scratch(t)
		os.MkdirAll(filepath.Join(ws, tree), 0o755)
		os.Symlink(filepath.Join(l.root, "skills"), filepath.Join(ws, tree, "skills"))
		rootBefore := snapshot(t, l.root)
		if code, _, errs := runWith(l.ops(), "init", "testhome", ws); code != 1 || !strings.Contains(errs, "leads into the root") {
			t.Errorf("%s: exit %d %q", tree, code, errs)
		}
		sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
	}
}

// init writes these files through a link at the file itself (ops.Rewrite):
// a link there into the root is refused before any write, named.
func TestInitRefusesAFileLinkIntoTheRoot(t *testing.T) {
	for _, rel := range []string{".claude/settings.local.json", ".claude/commands/daisy.md", ".cursorignore", ".gitignore"} {
		t.Run(rel, func(t *testing.T) {
			l := overrideLoop(t)
			writeFile(t, filepath.Join(l.root, "victim.txt"), "{}\n") // as settings would read
			ws := scratch(t)
			link := filepath.Join(ws, rel)
			os.MkdirAll(filepath.Dir(link), 0o755)
			os.Remove(link)
			if err := os.Symlink(filepath.Join(l.root, "victim.txt"), link); err != nil {
				t.Fatal(err)
			}
			rootBefore := snapshot(t, l.root)
			if code, _, errs := runWith(l.ops(), "init", "testhome", ws); code != 1 || !strings.Contains(errs, link+" leads into the root") {
				t.Errorf("exit %d %q", code, errs)
			}
			sameFiles(t, "the root", rootBefore, snapshot(t, l.root))
		})
	}
}

// The root bound as a workspace: its own .daisy/ is the workspace's, not
// the root's, and the build writes AGENTS.md and the materialised prompts
// there.
func TestRootBoundAsAWorkspaceBuilds(t *testing.T) {
	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.home, "prompts", "plan.md"), prompt.Marker+"\n\n## Rules\n\n1. home\n")
	writeFile(t, filepath.Join(l.root, ".daisy", "home"), "testhome\n")
	for _, ln := range env.Links {
		os.Symlink(filepath.Join(l.home, ln.Target), filepath.Join(l.root, ".daisy", ln.Link))
	}
	l.ws = l.root
	if code, _, errs := l.run("prompt", "build"); code != 0 {
		t.Fatalf("build in the root's workspace: %s", errs)
	}
	if b, _ := os.ReadFile(filepath.Join(l.root, ".daisy", "prompts", "plan.md")); !strings.Contains(string(b), "2. home") {
		t.Errorf("materialised %q", b)
	}
}

// The health check compares each tree's copy of an overriding skill with
// what init would install now: one out of date in .agents/skills warns
// naming it, as one in .claude/skills does.
func TestHealthCheckComparesBothSkillCopies(t *testing.T) {
	l := overrideLoop(t)
	writeFile(t, filepath.Join(l.root, "skills", "sk", "SKILL.md"), "---\nname: sk\n---\n\n## Rules\n\n1. a\n")
	writeFile(t, filepath.Join(l.home, "skills", "sk", "SKILL.md"), prompt.Marker+"\n\n## Rules\n\n1. b\n")
	if code, _, errs := runWith(l.ops(), "init", "testhome", l.ws); code != 0 {
		t.Fatalf("init: %s", errs)
	}
	if _, _, errs := l.run("healthcheck", "--force"); strings.Contains(errs, "is not the home's override") {
		t.Fatalf("warns after init: %s", errs)
	}
	for _, tree := range []string{".agents", ".claude"} {
		writeFile(t, filepath.Join(l.ws, tree, "skills", "sk", "SKILL.md"), "old\n")
		if _, _, errs := l.run("healthcheck", "--force"); !strings.Contains(errs, filepath.Join(l.ws, tree, "skills", "sk")+" is not the home's override") {
			t.Errorf("%s: silent: %s", tree, errs)
		}
	}
}
