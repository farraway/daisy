# Test Plan #89: `daisy prompt build` with a layered override: the merged
# prompt — the root's with the home's rules appended to its Rules,
# renumbered after them — is written to the workspace's .daisy/prompts/,
# and the lazy stub in AGENTS.md points there. A root prompt with no
# override is as before; nothing is written to the home or the root.

home="$(fixture_home_dir)"
printf '~fixlayer\n~fixroot\n' > "$home/include.txt"
printf '## Trigger\n\nRead the full `$DAISY_ROOT/prompts/fixlayer.md` when layering.\n\n## Rules\n\n1. Root rule.\n\n## Body\n\nRoot body.\n' > "$FIXTURE_ROOT/prompts/fixlayer.md"
printf '## Trigger\n\nRead the full `$DAISY_ROOT/prompts/fixroot.md` when rooted.\n' > "$FIXTURE_ROOT/prompts/fixroot.md"
mkdir -p "$home/prompts"
printf '<!-- daisy: upstream body -->\n\n## Rules\n\n1. Home rule.\n' > "$home/prompts/fixlayer.md"
git -C "$home" add -A && git -C "$home" commit -q -m "fixture: layered override"
git -C "$FIXTURE_ROOT" add -A && git -C "$FIXTURE_ROOT" commit -q -m "fixture: prompts"

fixture_run_cli prompt build
assert_eq "$RUN_EXIT" "0" "the build succeeds"
assert_contains "$RUN_STDOUT" "Lazy: fixlayer fixroot" "both lazy"
assert_text_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/prompts/fixlayer.md")" \
    "$(printf '## Trigger\n\nRead the full `$DAISY_ROOT/prompts/fixlayer.md` when layering.\n\n## Rules\n\n1. Root rule.\n\n2. Home rule.\n\n## Body\n\nRoot body.')" \
    "the materialised prompt is the merge"
built=$(cat "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")
assert_contains "$built" "Read the full \`$FIXTURE_WORKSPACE/.daisy/prompts/fixlayer.md\` when layering." \
    "the layered stub points at the materialised prompt"
assert_contains "$built" "Read the full \`$FIXTURE_ROOT/prompts/fixroot.md\` when rooted." \
    "a root prompt's stub is as before"
assert_eq "$(ls "$FIXTURE_WORKSPACE/.daisy/prompts")" "fixlayer.md" "only the layered prompt is materialised"
assert_eq "$(git -C "$home" status --porcelain)" "" "the home is untouched"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "the root is untouched"
