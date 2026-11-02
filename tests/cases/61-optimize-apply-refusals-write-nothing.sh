# Test Plan #61: `optimize apply` refuses, before writing anything, what it
# cannot apply: missing or conflicting flags are usage errors (exit 2); a
# workflow naming no file, an unreadable proposal, and a --rules proposal
# that is not one ## Rules section are failures (exit 1). The part that
# matters is the absence of wreckage: no half-written prompt, no archived
# entry whose guidance was never incorporated, no commit.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Rules

1. Measure before cutting.

## Next Section
PR

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.
FB
before=$(cat "$target")
feedback_before=$(fixture_read_feedback)
archive_before=$(fixture_read_archive)
commits_before=$(fixture_commit_count)

good=$(fixture_write_proposal good.md <<'RU'
## Rules

1. Measure twice.
RU
)
untitled=$(fixture_write_proposal untitled.md <<'RU'
1. Measure twice.
RU
)
spills=$(fixture_write_proposal spills.md <<'RU'
## Rules

1. Measure twice.

## Another Section
RU
)

usage() {
    fixture_run_cli optimize apply "$@" --yes
    assert_eq "$RUN_EXIT" "2" "usage error: $*"
    assert_contains "$RUN_STDERR" "Usage: daisy optimize apply" "the usage is printed: $*"
}
fixture_run_cli optimize apply </dev/null
assert_eq "$RUN_EXIT" "2" "bare optimize apply is a usage error"
usage --rules "$good"
usage --workflow prompts/widgetflow
usage --workflow prompts/widgetflow --rules "$good" --rewrite "$good"
usage --workflow prompts/widgetflow --rules
usage --workflow prompts/widgetflow --rules "$good" stray

refused() {
    local why="$1"; shift
    fixture_run_cli optimize apply "$@" --yes
    assert_eq "$RUN_EXIT" "1" "refused: $*"
    assert_contains "$RUN_STDERR" "$why" "the refusal says why: $*"
}
refused "names no prompt or skill" --workflow daisy-write --rules "$good"
refused "cannot read" --workflow prompts/widgetflow --rules "$FIXTURE_WORKSPACE/absent.md"
refused "must begin with" --workflow prompts/widgetflow --rules "$untitled"
refused "## Another Section" --workflow prompts/widgetflow --rules "$spills"

assert_text_eq "$(cat "$target")" "$before" "the target is untouched"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "feedback.md is untouched"
assert_text_eq "$(fixture_read_archive)" "$archive_before" "nothing is archived"
assert_eq "$(fixture_commit_count)" "$commits_before" "nothing is committed"
