# Test Plan #50: coverage is per-skill, not blanket. A skill carrying
# LICENSE/NOTICE.md tracks an upstream source; rewriting Daisy's copy from
# local feedback would fork it and make every future sync a reconciliation.
# Refusing must not abort the run — other groups still get their cycle.

skill_dir="$(fixture_home_dir)/skills/borrowed"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'EOF'
---
name: borrowed
description: Vendored from upstream.
---

## Rules

1. Upstream's rule.
EOF
printf 'Upstream license\n' > "$skill_dir/LICENSE"

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** borrowed

Vendored skill misbehaved.

## 2026-08-02 1100
**Workflow:** plan

Plan workflow did the wrong thing.
EOF

fixture_run_without_api_key optimize --dry-run

assert_eq "$RUN_EXIT" "0" "refusing one group does not abort the run"
assert_contains "$RUN_STDOUT" "vendored" "the refusal says why"
assert_contains "$RUN_STDOUT" "[plan]" "the other group is still processed"
