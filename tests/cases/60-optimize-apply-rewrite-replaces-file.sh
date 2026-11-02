# Test Plan #60: `optimize apply --rewrite` replaces the whole target with
# the proposal, for a prompt flagged for rewrite; `--yes` applies without
# asking, for text the user has already approved in the conversation.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.
2. Measure again.

## Body

Old body.
PR

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.
FB
commits_before=$(fixture_commit_count)

proposal=$(fixture_write_proposal rewrite.md <<'RW'
## Trigger

Read this when relevant.

## Body

Measure twice, cut once: the body now says what the rules said.

## Rules

1. Keep the blade sharp.
RW
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rewrite "$proposal" --yes </dev/null

assert_eq "$RUN_EXIT" "0" "an approved rewrite succeeds"
cmp -s "$target" "$proposal"
assert_eq "$?" "0" "the target is the proposal, byte for byte"
assert_not_contains "$RUN_STDERR" "Apply these changes" "--yes does not ask"
assert_contains "$RUN_STDOUT" "+Measure twice, cut once" "the diff is still shown"
assert_contains "$(fixture_read_archive)" "— \`prompts/widgetflow.md\` (rewrite)" \
    "the archive records a rewrite"
assert_not_contains "$(fixture_read_feedback)" "Widget guidance was wrong once" "the entry is archived"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the home is committed once"
