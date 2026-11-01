# Test Plan #41: entries with no **Workflow:** header form an explicit
# `daisy` group on an unscoped run, preserving the target they have always
# had — the conflation removed in #38 was `""`, not the daisy workflow.
#
# Kept to one unheadered entry deliberately: a second would clear the floor
# and reach the LLM, which no case in this suite may do.

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000

An entry with no workflow header at all.

## 2026-08-02 1100
**Workflow:** plan

Plan workflow did the wrong thing.
EOF

fixture_run_without_api_key optimize --dry-run

assert_eq "$RUN_EXIT" "0" "unscoped run succeeds"
assert_contains "$RUN_STDOUT" "[daisy]" "unheadered entry groups as daisy"
assert_contains "$RUN_STDOUT" "[plan]" "headered entry keeps its own group"
