# Test Plan #38: an unspecified --workflow groups entries by their
# **Workflow:** header and runs one cycle per group. Previously "no filter"
# and "the daisy workflow" resolved to the same target, so a mixed inbox was
# folded wholesale into prompts/daisy.md's ## Rules.

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

assert_eq "$RUN_EXIT" "0" "unscoped run with only below-floor groups succeeds"
assert_contains "$RUN_STDOUT" "[plan]" "plan reported as its own group"
assert_contains "$RUN_STDOUT" "[retrospective]" "retrospective reported as its own group"
assert_not_contains "$RUN_STDOUT" "prompts/daisy.md" "no group targets daisy.md"
