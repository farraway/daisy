# Test Plan #40: --workflow X matches only entries declaring X. It used to
# also sweep in entries carrying no **Workflow:** header, which meant a
# scoped run and an unscoped run routed the same entry to different prompts.

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** plan

Plan workflow did the wrong thing.

## 2026-08-02 1100

An entry with no workflow header at all.
EOF

fixture_run_without_api_key optimize --workflow plan --dry-run

assert_eq "$RUN_EXIT" "0" "scoped run with a below-floor group succeeds"
assert_contains "$RUN_STDOUT" "1 entry" "only the declared entry is in scope"
assert_contains "$RUN_STDOUT" "Skipped" "one entry is below the floor"
