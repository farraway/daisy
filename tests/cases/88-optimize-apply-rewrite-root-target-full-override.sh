# Test Plan #88: `apply --rewrite` on a root prompt copies it to the home
# and edits the copy — a full override — recording the root file's hash in
# feedback/.overrides; the root is untouched. `daisy healthcheck` is
# silent while the root is as copied and warns once it changes; `daisy
# list` marks the override `full`, then `full, behind`.

fixture_setup_home_override
root_prompt="$FIXTURE_ROOT/prompts/plan.md"
override="$(fixture_home_dir)/prompts/plan.md"
fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** plan

Plan guidance was wrong.
FB
git -C "$(fixture_home_dir)" commit -q -am "fixture: pending entry"
commits_before=$(fixture_commit_count)
root_hash=$(sha256sum "$root_prompt" | cut -d' ' -f1)

proposal=$(fixture_write_proposal rewrite.md <<'RW'
## Trigger

Read the full `$DAISY_ROOT/prompts/plan.md` when planning.

The home's own plan prompt.
RW
)
fixture_run_cli optimize apply --workflow plan --rewrite "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "a rewrite of a root target applies"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "the root's tree is untouched"
cmp -s "$override" "$proposal"
assert_eq "$?" "0" "the override is the proposal, byte for byte"
assert_eq "$(cat "$(fixture_home_dir)/feedback/.overrides")" "prompts/plan.md $root_hash" \
    "the root file's hash is recorded"
assert_contains "$(fixture_read_archive)" "— \`prompts/plan.md\` (full rewrite)" "the archive names the form"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the home is committed once"
assert_contains "$(cat "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")" "Read the full \`$override\` when planning." \
    "the stub points at the home's file"

fixture_run_cli healthcheck --force
assert_not_contains "$RUN_STDERR" "behind the root" "the root as copied: no warning"
fixture_run_cli list
assert_contains "$RUN_STDOUT" "[full]" "list marks the full override"

printf '\nAn upstream change.\n' >> "$root_prompt"
fixture_run_cli healthcheck --force
assert_contains "$RUN_STDERR" "Override of prompts/plan.md is behind the root (root changed since the copy)" \
    "the root changed: healthcheck warns"
fixture_run_cli list
assert_contains "$RUN_STDOUT" "[full, behind]" "and list says so"
fixture_teardown_home_override
