# Test Plan #53: the ## Rules section ends at the next heading of level 1 or
# 2 — a markdown section closes at the next heading of its own level or
# shallower; a `###` is a subsection inside it. The bash extractor and
# replacer once both terminated on "^## " alone, so a "# " heading between
# the rules and the next section was briefed as if it were a rule and then
# deleted on write. It ate "# Daily and Weekly Retrospective" from
# prompts/retrospective.md on the 2026-08-17 run. One boundary serves the
# briefing and `apply --rules`.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.

### Exceptions

2. Cut before nailing.


# Document Title

## Next Section

Body text.
PR
cp "$target" "$FIXTURE_WORKSPACE/original.md"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong.
FB

fixture_run_cli optimize
assert_contains "$RUN_STDOUT" "### Exceptions" "an H3 is a subsection of the rules, not a boundary"
assert_contains "$RUN_STDOUT" "2. Cut before nailing." "rules past an H3 subsection are briefed"
assert_contains "$RUN_STDOUT" "Rules: 2" "and counted"
assert_not_contains "$RUN_STDOUT" "Document Title" "an H1 after the rules is not part of the rules"
assert_not_contains "$RUN_STDOUT" "Body text" "the following section is not part of the rules"

# Replacing the rules with themselves is an identity, byte for byte. The
# bash replacer, fed through command substitution, lost the blank lines
# before the next heading on every run; the gap is the file's, whatever
# the proposal ends with.
same=$(fixture_write_proposal same.md <<'RU'
## Rules

1. Measure before cutting.

### Exceptions

2. Cut before nailing.
RU
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rules "$same" --yes
assert_eq "$RUN_EXIT" "0" "applying the rules as they are succeeds"
cmp -s "$target" "$FIXTURE_WORKSPACE/original.md"
assert_eq "$?" "0" "replacing the rules with themselves changes nothing"

new=$(fixture_write_proposal new.md <<'RU'
## Rules

1. Replacement rule.
RU
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rules "$new" --yes
written=$(cat "$target")
assert_contains "$written" "1. Replacement rule." "the new rules are written"
assert_not_contains "$written" "Measure before cutting" "the old rules are gone"
assert_not_contains "$written" "### Exceptions" "the H3 inside the rules went with them"
assert_contains "$written" "## Trigger" "the section above the rules survives"
assert_text_eq "$(sed -n '/^1. Replacement rule./,$p' "$target")" "1. Replacement rule.


# Document Title

## Next Section

Body text." "the H1 and the section after it survive, the gap as it was"
