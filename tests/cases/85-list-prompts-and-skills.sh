#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy list`: one line per include.txt entry (the trigger's first line for
# a lazy prompt, `(always loaded)`, `(missing: …)`), then every installed
# skill, user-level then workspace, the workspace's copy winning by name,
# its short_description else its description. Read-only.

home="$(fixture_home_dir)"
printf 'fixfull\n~fixlazy\nfixmissing\n' > "$home/include.txt"
printf '# Full\n' > "$FIXTURE_ROOT/prompts/fixfull.md"
printf '## Trigger\n\nRead the full prompt when lazy.\n- When a lazy thing happens.\n' > "$FIXTURE_ROOT/prompts/fixlazy.md"
fixture_setup_home_override
mkdir -p "$FIXTURE_HOME_OVERRIDE/.claude/skills/both" "$FIXTURE_HOME_OVERRIDE/.claude/skills/useronly" "$FIXTURE_WORKSPACE/.claude/skills/both"
printf -- '---\ndescription: the user copy\n---\n' > "$FIXTURE_HOME_OVERRIDE/.claude/skills/both/SKILL.md"
printf -- '---\ndescription: Long: and full.\nshort_description: Short.\n---\n' > "$FIXTURE_HOME_OVERRIDE/.claude/skills/useronly/SKILL.md"
printf -- '---\ndescription: the workspace copy\n---\n' > "$FIXTURE_WORKSPACE/.claude/skills/both/SKILL.md"
status_before=$(git -C "$home" status --porcelain)

fixture_run_cli list
assert_eq "$RUN_EXIT" "0" "list exits 0"
assert_contains "$RUN_STDOUT" "Prompts (testhome):" "names the home"
assert_contains "$RUN_STDOUT" "  fixfull              (always loaded)" "a full prompt"
assert_contains "$RUN_STDOUT" "  fixlazy              When a lazy thing happens." "a lazy prompt's trigger line"
assert_contains "$RUN_STDOUT" "  fixmissing           (missing: $FIXTURE_ROOT/prompts/fixmissing.md)" "a missing prompt"
assert_contains "$RUN_STDOUT" "  both                 the workspace copy" "the workspace copy wins"
assert_contains "$RUN_STDOUT" "  useronly             Short." "the short description"
fixture_run_cli list --full
assert_contains "$RUN_STDOUT" "  useronly             Long: and full." "--full: the description"
assert_eq "$(git -C "$home" status --porcelain)" "$status_before" "list writes nothing"
fixture_teardown_home_override
