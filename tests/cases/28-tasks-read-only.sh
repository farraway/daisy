# Test Plan #28: read-only guarantee — todo.txt/done.txt byte-identical
# after; no new commit.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF
seed_done_basic

todo_before=$(fixture_read_todo)
done_before=$(fixture_read_done)
commits_before=$(fixture_commit_count)

fixture_run tasks --all +proj

assert_eq "$(fixture_read_todo)" "$todo_before" "todo.txt byte-identical after a tasks query"
assert_eq "$(fixture_read_done)" "$done_before" "done.txt byte-identical after a tasks query"
assert_eq "$(fixture_commit_count)" "$commits_before" "no commit is produced by a read-only query"
