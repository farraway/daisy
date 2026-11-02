# Test Plan #55: `plan new` bounds the filename slug. It built the slug from the
# whole description with no cap, so an ordinary-length description overran the
# 255-byte filename limit and the script died with a raw "File name too long"
# from the shell — after validating its inputs, and with no actionable message
# (praxis rule 7). Hit while creating plan 260817235209 itself.

long_desc="fix four defects surfaced by the previous session namely that plan archive leaves dead project links and a nonexistent daisy add command appears in the rules and the optimize floor strands entries at small n and dry run previews a different sample than the apply does"

fixture_run_cli plan new "$long_desc" "+testproj"

assert_eq "$RUN_EXIT" "0" "a long description does not fail the run"
assert_not_contains "$RUN_STDERR" "File name too long" "no raw filesystem error"

created=$(basename "$(readlink "$FIXTURE_WORKSPACE/PLAN.md")")
assert_eq "$(printf '%s' "$created" | wc -c | tr -d ' ' | awk '$1 < 255 {print "under"} $1 >= 255 {print "over"}')" "under" \
    "the generated filename is under the 255-byte limit"
assert_contains "$created" ".testproj." "the project tag survives truncation"
assert_contains "$created" ".plan.md" "the suffix survives truncation"
assert_contains "$created" "fix-four-defects" "the slug stays readable from its start"

# Spec mode builds ${SLUG}_PLAN.md from the same slug, so it inherits the cap.
rm -f "$FIXTURE_WORKSPACE/PLAN.md"
fixture_run_cli plan new --spec "$long_desc"
assert_eq "$RUN_EXIT" "0" "spec mode also survives a long description"
assert_not_contains "$RUN_STDERR" "File name too long" "spec mode raises no raw filesystem error"

# Capping the slug does not bound the whole filename: a long +project can
# still overrun. That case must report the limit and what to shorten, not die
# on the filesystem after every input has been validated (praxis rule 7).
rm -f "$FIXTURE_WORKSPACE"/*_PLAN.md
long_project=$(printf 'p%.0s' $(seq 1 250))
fixture_run_cli plan new "a short description" "+$long_project"

assert_eq "$RUN_EXIT" "1" "an unrepresentable filename is refused"
assert_contains "$RUN_STDERR" "the limit is 255" "the message names the limit"
assert_contains "$RUN_STDERR" "Shorten the project tag" "the message names what to change"
assert_not_contains "$RUN_STDERR" "File name too long" "the shell's error is pre-empted, not surfaced"
