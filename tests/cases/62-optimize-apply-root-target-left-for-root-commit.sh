# Test Plan #62: a target under $DAISY_ROOT/prompts/ or skills/ is a change
# to the public root repository. `apply` writes it, leaves it uncommitted
# for the root commit (which runs the publication-hygiene gate), and says
# so; the home's half — feedback.md and the archive — is committed once.

target="$FIXTURE_ROOT/prompts/retrospective.md"
fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/retrospective

Retrospective guidance was wrong.
FB
commits_before=$(fixture_commit_count)
root_commits_before=$(fixture_root_commit_count)

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Report a measurement, not a score.
RU
)
fixture_run_cli optimize apply --workflow retrospective --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "a root target applies"
assert_contains "$(cat "$target")" "1. Report a measurement, not a score." "the root file is written"
assert_eq "$(fixture_root_commit_count)" "$root_commits_before" "the root is not committed"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" " M prompts/retrospective.md" \
    "the change is left in the root's tree, and nothing else"
assert_contains "$RUN_STDOUT" "daisy commit --not-home" "the run says how to commit the root"
assert_contains "$RUN_STDOUT" "daisy healthcheck --force" "and to run the hygiene gate first"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the home is committed once"
assert_contains "$(fixture_read_archive)" "— \`\$DAISY_ROOT/prompts/retrospective.md\` (rules)" \
    "the archive names the root file as \$DAISY_ROOT/…"
