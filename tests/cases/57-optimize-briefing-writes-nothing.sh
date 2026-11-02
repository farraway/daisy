# Test Plan #57: the briefing runs read-only — it writes nothing in the home
# or the root, whatever is pending — and the API loop's flags are gone:
# `--dry-run`, `--force` and `--list` are usage errors (exit 2), not
# silently ignored options.

fixture_write_home_prompt widgetflow <<'PR'
## Rules

1. Measure before cutting.
PR

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.

## 2026-08-02 1100
**Workflow:** prompts/widgetflow

Widget guidance was wrong again.
FB

git -C "$(fixture_home_dir)" add -A && git -C "$(fixture_home_dir)" commit -qm "fixture: feedback"

snapshot() { (cd "$1" && find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 sha1sum); }
home_before=$(snapshot "$(fixture_home_dir)")
root_before=$(snapshot "$FIXTURE_ROOT")

fixture_run optimize
assert_eq "$RUN_EXIT" "0" "the briefing succeeds"
assert_contains "$RUN_STDOUT" "[prompts/widgetflow] 2 entries" "and briefs the target"
assert_text_eq "$(snapshot "$(fixture_home_dir)")" "$home_before" "the home is byte-identical"
assert_text_eq "$(snapshot "$FIXTURE_ROOT")" "$root_before" "the root is byte-identical"
assert_eq "$(git -C "$(fixture_home_dir)" status --porcelain)" "" "the home's tree is clean"

for flag in --dry-run --force --list; do
    fixture_run optimize "$flag"
    assert_eq "$RUN_EXIT" "2" "optimize $flag is a usage error"
    assert_contains "$RUN_STDERR" "Usage: daisy optimize" "optimize $flag prints the usage"
done
