# Test Plan #62: a target in the root is never written there; homes consume
# the root. `apply --rules` on a root prompt writes a layered override in
# the home — the marker line and only the rules the proposal adds after the
# root's — materialises the merged prompt beside the workspace's AGENTS.md
# (its stub points there) and commits the home once, the materialised file
# not in it. A second apply changes only the override; the root edited
# afterwards changes the next build with no change to the home.

root_prompt="$FIXTURE_ROOT/prompts/retrospective.md"
override="$(fixture_home_dir)/prompts/retrospective.md"
materialised="$FIXTURE_WORKSPACE/.daisy/prompts/retrospective.md"
root_section=$(awk '/^## Rules/ {on=1; print; next} on && /^#/ {exit} on' "$root_prompt")
fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/retrospective

Retrospective guidance was wrong.
FB
git -C "$(fixture_home_dir)" commit -q -am "fixture: pending entry"
commits_before=$(fixture_commit_count)
root_commits_before=$(fixture_root_commit_count)
root_before=$(cat "$root_prompt")

proposal=$(fixture_write_proposal rules.md <<RU
$root_section

12. Report a measurement, not a score.
RU
)
fixture_run_cli optimize apply --workflow retrospective --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "a root target applies"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "the root's tree is untouched"
assert_text_eq "$(cat "$root_prompt")" "$root_before" "the root's prompt is byte-identical"
assert_eq "$(fixture_root_commit_count)" "$root_commits_before" "the root is not committed"
assert_text_eq "$(cat "$override")" "$(printf '<!-- daisy: upstream body -->\n\n## Rules\n\n1. Report a measurement, not a score.')" \
    "the override is the marker and the new rule, renumbered from 1"
assert_text_eq "$(cat "$materialised")" \
    "$(awk '{print} /^11\. / {print ""; print "12. Report a measurement, not a score."}' "$root_prompt")" \
    "the materialised prompt is the root's with the rule appended to its Rules"
assert_contains "$(cat "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")" "Read the full \`$materialised\`" \
    "the stub points at the materialised prompt"
assert_not_contains "$RUN_STDOUT" "daisy commit --root" "no root commit is asked for"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the home is committed once"
assert_eq "$(git -C "$(fixture_home_dir)" show --name-only --format= HEAD | LC_ALL=C sort | tr '\n' ' ')" \
    "feedback/.last-optimized feedback/archive.md feedback/feedback.md prompts/retrospective.md " \
    "the commit holds the override and the feedback, not the materialised prompt"
assert_contains "$(fixture_read_archive)" "— \`prompts/retrospective.md\` (layered rules)" \
    "the archive names the home's override and its form"

proposal=$(fixture_write_proposal rules.md <<RU
$root_section

12. Report a measurement, not a score.
13. Name the measurement's unit.
RU
)
fixture_run_cli optimize apply --workflow retrospective --rules "$proposal" --yes
assert_eq "$RUN_EXIT" "0" "a second apply"
assert_eq "$(git -C "$(fixture_home_dir)" show --name-only --format= HEAD)" "prompts/retrospective.md" \
    "changes only the override"
assert_eq "$(fixture_commit_count)" "$((commits_before + 2))" "in one commit"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "the root still untouched"

printf '\nUpstream body line.\n' >> "$root_prompt"
home_head=$(git -C "$(fixture_home_dir)" rev-parse HEAD)
fixture_run_cli prompt build
assert_eq "$RUN_EXIT" "0" "the next build"
assert_contains "$(cat "$materialised")" "Upstream body line." "carries the root's change"
assert_contains "$(cat "$materialised")" "13. Name the measurement's unit." "and the home's rules"
assert_eq "$(git -C "$(fixture_home_dir)" rev-parse HEAD) $(git -C "$(fixture_home_dir)" status --porcelain)" "$home_head " \
    "with no change to the home"
