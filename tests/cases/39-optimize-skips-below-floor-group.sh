# Test Plan #39: a group below the 2-entry floor is named and skipped, not
# silently dropped — and reaching that verdict costs no API call, so the
# absent key must not be an error.

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** plan

Plan workflow did the wrong thing.

## 2026-08-02 1100
**Workflow:** retrospective

Retrospective workflow did the wrong thing.
EOF

fixture_run_without_api_key optimize --dry-run

assert_eq "$RUN_EXIT" "0" "no key needed when nothing reaches the LLM"
assert_not_contains "$RUN_STDERR" "ANTHROPIC_API_KEY" "key checked lazily, not up front"
assert_contains "$RUN_STDOUT" "Skipped" "below-floor group reports why it was skipped"
assert_contains "$RUN_STDOUT" "2-entry floor" "the reason names the floor"

# Skipping must not archive: an entry that never trained a rewrite stays put.
assert_contains "$(fixture_read_feedback)" "Plan workflow did the wrong thing" \
    "skipped group's entry remains in feedback.md"
