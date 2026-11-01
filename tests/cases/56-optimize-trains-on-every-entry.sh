# Test Plan #56: every in-scope entry trains. An 80/20 "train/test" split
# withheld 20% of a group from the LLM call and left them in feedback.md,
# but nothing ever scored against the holdout — test_count was computed,
# printed and never read, and there was no test_idx. At N=2 that stranded the
# remainder below MIN_ENTRIES indefinitely rather than deferring it, which is
# what happened to the 2026-08-17 1335 entry.
#
# The entry-count banner prints before ensure_llm_deps, so this is observable
# with no API key. The run then exits non-zero at the dependency check, which
# is expected — the assertions are on stdout, not on the exit code.

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** retrospective

Retrospective did the wrong thing once.

## 2026-08-02 1100
**Workflow:** retrospective

Retrospective did the wrong thing again.
EOF

fixture_run_without_api_key optimize --workflow retrospective

assert_contains "$RUN_STDOUT" "Entries: 2" "the group reports its entry count"
assert_not_contains "$RUN_STDOUT" "Test:" "no holdout is reported"
assert_not_contains "$RUN_STDOUT" "Train:" "no train/test split is reported"
assert_not_contains "$RUN_STDOUT" "Skipped" "two entries clear the floor"
