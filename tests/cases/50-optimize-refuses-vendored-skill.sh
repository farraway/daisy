# Test Plan #50: coverage is per-skill, not blanket. A skill carrying
# LICENSE/NOTICE.md tracks an upstream source; rewriting Daisy's copy from
# local feedback would fork it and make every future sync a reconciliation.
# The briefing says so and goes on to the other targets; `apply` refuses it
# and writes nothing.

skill_dir="$(fixture_home_dir)/skills/borrowed"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'SK'
---
name: borrowed
description: Vendored from upstream.
---

## Rules

1. Upstream's rule.
SK
printf 'Upstream license\n' > "$skill_dir/LICENSE"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** borrowed

Vendored skill misbehaved.

## 2026-08-02 1100
**Workflow:** plan

Plan workflow did the wrong thing.
FB

fixture_run optimize

assert_eq "$RUN_EXIT" "0" "a vendored target does not fail the briefing"
assert_contains "$RUN_STDOUT" "[skills/borrowed] 1 entry" "the vendored target is listed"
assert_contains "$RUN_STDOUT" "vendored" "with the reason it is not offered"
assert_contains "$RUN_STDOUT" "[prompts/plan] 1 entry" "the other target is still briefed"

skill_before=$(cat "$skill_dir/SKILL.md")
feedback_before=$(fixture_read_feedback)
commits_before=$(fixture_commit_count)
proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Our own rule.
RU
)
fixture_run_cli optimize apply --workflow skills/borrowed --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "1" "apply refuses a vendored skill"
assert_contains "$RUN_STDERR" "vendored" "and says why"
assert_text_eq "$(cat "$skill_dir/SKILL.md")" "$skill_before" "the vendored SKILL.md is untouched"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "its entry stays pending"
assert_eq "$(fixture_commit_count)" "$commits_before" "nothing is committed"
