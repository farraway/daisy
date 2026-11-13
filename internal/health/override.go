package health

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"

	"daisy/internal/prompt"
	"daisy/internal/skill"
)

// overrides is Check 8 (warnings): each full override the home's ledger
// records whose root file has changed since the copy. A layered override
// takes the root as it is and is never behind.
func (r *run) overrides() {
	ledger, err := prompt.ReadLedger(r.fs, r.e.Home)
	if err != nil {
		r.warn("Cannot read %s: %v", filepath.Join(r.e.Home, prompt.LedgerRel), err)
		return
	}
	var rels []string
	for rel := range ledger {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if prompt.Locate(r.fs, r.e.Home, r.root, rel).Behind(r.fs, ledger) {
			r.warn("Override of %s is behind the root (root changed since the copy) — fold the root's changes into %s, then record them with 'daisy optimize apply --workflow <target> --rewrite <file>'", rel, filepath.Join(r.e.Home, rel))
		}
	}
}

// materialised is Check 8b (warnings): the workspace's build must be what
// the home's overrides make now. Its AGENTS.md and .daisy/prompts/ are
// written at build, init and apply time, and go stale when the root moves
// on (a pull) or another workspace's apply changes an override: so,
// wherever an override is involved, on either side, the files are
// compared with a build now (the Built: line aside), and a skill the home
// overrides with the copy init would install now.
func (r *run) materialised() {
	ws := filepath.Join(r.e.Workspace, ".daisy")
	b, err := prompt.Build(r.fs, prompt.Spec{
		Root: r.root, HomeName: r.e.HomeName, Home: r.e.Home,
		Output: filepath.Join(ws, "AGENTS.md"), Rebuild: "daisy prompt build", Built: r.o.Now(),
		Prompts: filepath.Join(ws, "prompts"),
	})
	built, _ := r.fs.ReadFile(filepath.Join(ws, "AGENTS.md"))
	on := r.homeOverrides() || bytes.Contains(b.Text, overridesBlock) || bytes.Contains(built, overridesBlock) || len(prompt.MDFiles(r.fs, filepath.Join(ws, "prompts"))) > 0
	switch {
	case err != nil && on:
		r.warn("The workspace's AGENTS.md cannot be rebuilt: %v", err)
	case err != nil:
	case on && string(builtLine.ReplaceAll(built, nil)) != string(builtLine.ReplaceAll(b.Text, nil)):
		r.warn("%s is not what a build makes now (the root's prompts or the home's overrides changed since) — run 'daisy init'", filepath.Join(ws, "AGENTS.md"))
	case on:
		have := map[string]bool{}
		for _, rel := range prompt.MDFiles(r.fs, filepath.Join(ws, "prompts")) {
			have[filepath.Join(ws, "prompts", rel)] = true
		}
		stale := len(have) != len(b.Materialised)
		for p, want := range b.Materialised {
			got, err := r.fs.ReadFile(p)
			stale = stale || err != nil || string(got) != string(want)
		}
		if stale {
			r.warn("%s is not the merge of the root's prompts and the home's overrides as they are now — run 'daisy init'", filepath.Join(ws, "prompts"))
		}
	}
	r.skillOverrides()
}

// homeOverrides: a prompt in the home overrides the root's.
func (r *run) homeOverrides() bool {
	for _, name := range prompt.HomePrompts(r.fs, r.e.Home) {
		if prompt.Locate(r.fs, r.e.Home, r.root, "prompts/"+name).Overridden() {
			return true
		}
	}
	return false
}

var (
	overridesBlock = []byte("# Home overrides: ")
	builtLine      = regexp.MustCompile(`(?m)^# Built: .*\n`)
)

// skillOverrides warns for each home skill overriding the root's whose
// copy in the workspace is not the one init would install now.
func (r *run) skillOverrides() {
	for _, rel := range prompt.MDFiles(r.fs, filepath.Join(r.e.Home, "skills")) {
		if filepath.Base(rel) != "SKILL.md" {
			continue
		}
		src := prompt.Locate(r.fs, r.e.Home, r.root, "skills/"+rel)
		if !src.Overridden() {
			continue
		}
		want, err := src.SkillFiles(r.fs)
		if err != nil {
			r.warn("Skill override %s cannot be read as the workspace would install it: %v", src.Home, err)
			continue
		}
		for _, tree := range skill.WorkspaceTrees {
			installed := filepath.Join(r.e.Workspace, tree, filepath.Base(filepath.Dir(rel)))
			got := prompt.Files(r.fs, installed)
			stale := len(got) != len(want)
			for f, w := range want {
				g, ok := got[f]
				stale = stale || !ok || string(g) != string(w)
			}
			if stale {
				r.warn("%s is not the home's override of %s as it is now — run 'daisy init %s'", installed, filepath.Dir(src.Root), r.e.Identity)
			}
		}
	}
}
