#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy init` equips Codex as it does Claude Code: .agents/skills/daisy/
# SKILL.md, Codex's way into Daisy, and every home skill copied into
# .agents/skills/<name>/ byte for byte as into .claude/skills/<name>/;
# re-init sweeps a stale copy from .agents/skills as from .claude/skills.

home="$(fixture_home_dir)"
mkdir -p "$home/skills/fixture-skill"
printf -- '---\nname: fixture-skill\ndescription: Use when testing\n---\nbody\n' > "$home/skills/fixture-skill/SKILL.md"
echo "ref" > "$home/skills/fixture-skill/ref.md"

fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "daisy init exits 0"
skill="$FIXTURE_WORKSPACE/.agents/skills/daisy/SKILL.md"
assert_eq "$(sed -n 1p "$skill")" "---" "the daisy skill opens with front matter"
assert_eq "$(sed -n '2,/^---$/p' "$skill" | grep -c '^name: daisy$')" "1" "named daisy"
assert_eq "$(sed -n '2,/^---$/p' "$skill" | grep -c '^description: .')" "1" "with a one-line description"
assert_contains "$(cat "$skill")" 'Read `.daisy/AGENTS.md`' "its body sends the request through .daisy/AGENTS.md"
diff -r "$FIXTURE_WORKSPACE/.claude/skills/fixture-skill" "$FIXTURE_WORKSPACE/.agents/skills/fixture-skill" >/dev/null && same=yes || same=no
assert_eq "$same" "yes" "the skill's copy in .agents/skills is its .claude/skills twin"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" status --porcelain --untracked-files=all -- .agents)" "" "init's .agents files are ignored as the .claude copies are"

rm -rf "$home/skills/fixture-skill"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "re-init exits 0"
[ -e "$FIXTURE_WORKSPACE/.agents/skills/fixture-skill" ] && left=yes || left=no
assert_eq "$left" "no" "re-init sweeps the stale .agents/skills copy"
[ -e "$FIXTURE_WORKSPACE/.claude/skills/fixture-skill" ] && left=yes || left=no
assert_eq "$left" "no" "and its .claude/skills twin"
[ -f "$skill" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "the daisy skill stays"
