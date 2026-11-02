# Test Plan #58: `optimize apply --rules` shows the diff, asks, and on `y`
# replaces the target's ## Rules section, moves the target's pending
# entries to the archive and commits the home exactly once. This is the
# loop's whole purpose.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.

# Document Title

## Next Section

Body text.
PR
git -C "$(fixture_home_dir)" add -A && git -C "$(fixture_home_dir)" commit -qm "fixture: widgetflow"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.

## 2026-08-02 1100
**Workflow:** widgetflow

Widget guidance was wrong again.

## 2026-08-03 1200
**Workflow:** plan

Plan guidance, not this target's.
FB
git -C "$(fixture_home_dir)" add -A && git -C "$(fixture_home_dir)" commit -qm "fixture: feedback"
commits_before=$(fixture_commit_count)
root_commits_before=$(fixture_root_commit_count)

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
2. Cut once.
RU
)
fixture_run_cli_with_input "y" optimize apply --workflow prompts/widgetflow --rules "$proposal"

assert_eq "$RUN_EXIT" "0" "an approved apply succeeds"
assert_contains "$RUN_STDOUT" "-1. Measure before cutting." "the diff shows the old rule"
assert_contains "$RUN_STDOUT" "+2. Cut once." "and the new ones"
assert_contains "$RUN_STDERR" "Apply these changes to prompts/widgetflow? [y/N]" "approval is asked on stderr"
assert_contains "$RUN_STDOUT" "Updated $target" "the run reports the file it wrote"

written=$(cat "$target")
assert_contains "$written" "1. Measure twice before cutting." "the proposed rules are written"
assert_contains "$written" "2. Cut once." "all of them"
assert_not_contains "$written" "Measure before cutting." "the old rules are replaced"
assert_contains "$written" "## Trigger" "the section above the rules survives"
assert_contains "$written" "# Document Title" "the H1 after the rules survives"
assert_contains "$written" "Body text." "the section below the rules survives"

remaining=$(fixture_read_feedback)
assert_not_contains "$remaining" "Widget guidance was wrong once" "the target's entry leaves feedback.md"
assert_not_contains "$remaining" "Widget guidance was wrong again" "every tag naming the target does"
assert_contains "$remaining" "Plan guidance, not this target's." "another target's entry stays"
assert_contains "$remaining" "# Feedback Log" "the file keeps its header"
archived=$(fixture_read_archive)
assert_contains "$archived" "Widget guidance was wrong once" "the entry lands in the archive"
assert_contains "$archived" "Widget guidance was wrong again" "both of them do"

assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the home is committed exactly once"
assert_eq "$(git -C "$(fixture_home_dir)" status --porcelain)" "" "and nothing is left uncommitted"
assert_contains "$(git -C "$(fixture_home_dir)" show --stat HEAD)" "prompts/widgetflow.md" "the commit holds the target"
assert_contains "$RUN_STDOUT" "📝 Committed:" "the commit is reported"
assert_eq "$(fixture_root_commit_count)" "$root_commits_before" "the root is not committed"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "and not written"
