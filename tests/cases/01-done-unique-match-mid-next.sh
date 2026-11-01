# Test Plan #1: unique match mid-**Next:** — only that checkbox flips, all
# other lines byte-identical.

seed_basic
before=$(fixture_read_today)

fixture_run done "Delta target"

after=$(fixture_read_today | strip_log_section)
expected=$(printf '%s\n' "$before" | strip_log_section | sed 's/^- \[ \] Delta target task +daisy$/- [x] Delta target task +daisy/')

assert_eq "$RUN_EXIT" "0" "done exits 0 on a unique match"
assert_text_eq "$after" "$expected" "only the Delta target checkbox flips in today.md (Tasks section, ignoring the new Log entry)"
