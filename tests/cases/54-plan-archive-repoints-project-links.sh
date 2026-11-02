# Test Plan #54: archiving a plan repoints the project-file links that named
# it. `plan archive` is the only place that knows both the old and the new path
# at the moment they change, so it is the only place that can fix the links
# without scanning for them. Three links were repaired by hand before this
# was mechanised (260729134724, 260817140134, 260728121456).

home="$(fixture_home_dir)"
plan="260101010101.testproj.a-plan.plan.md"

mkdir -p "$home/plans/archive" "$home/projects"
printf '# a plan\n' > "$home/plans/$plan"

cat > "$home/projects/testproj.md" <<EOF
# testproj

## Plans

- [../plans/$plan](../plans/$plan) — status: built
EOF

# An unrelated project file must come through byte-identical: FILENAME is
# full of dots, and an unescaped sed pattern would match far too much.
cat > "$home/projects/other.md" <<'EOF'
# other

## Plans

- [../plans/260202020202.other.unrelated.plan.md](../plans/260202020202.other.unrelated.plan.md) — status: built
EOF
other_before=$(cat "$home/projects/other.md")

ln -s "$home/plans/$plan" "$FIXTURE_WORKSPACE/PLAN.md"

fixture_run_cli plan archive

assert_eq "$RUN_EXIT" "0" "archive succeeds"
assert_contains "$(cat "$home/projects/testproj.md")" "../plans/archive/$plan" "the link is repointed at the archive"
assert_not_contains "$(cat "$home/projects/testproj.md")" "(../plans/$plan)" "no link still names the pre-archive path"
assert_contains "$RUN_STDOUT" "project file" "the repointing is reported, not silent"
assert_text_eq "$(cat "$home/projects/other.md")" "$other_before" "an unrelated project file is untouched"
