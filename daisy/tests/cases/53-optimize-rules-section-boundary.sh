# Test Plan #53: the ## Rules section ends at the next heading of level 1 or
# 2 — a markdown section closes at the next heading of its own level or
# shallower. Both the extractor and the replacer previously terminated on
# "^## " alone, so a "# " heading between the rules and the next section was
# swallowed into the LLM input as if it were a rule and then deleted on
# write. It ate "# Daily and Weekly Retrospective" from
# prompts/retrospective.md on the 2026-08-17 run.

prompt="$FIXTURE_ROOT/sample-prompt.md"
cat > "$prompt" <<'EOF'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.
2. Cut before nailing.

# Document Title

## Next Section

Body text.
EOF

extracted=$(source "$DAISY_ROOT/daisy/scripts/common.sh" && extract_rules_section "$prompt")
assert_contains "$extracted" "1. Measure before cutting." "the rules themselves are extracted"
assert_not_contains "$extracted" "Document Title" "an H1 after the rules is not part of the rules"
assert_not_contains "$extracted" "Body text" "the following section is not part of the rules"

replaced=$(source "$DAISY_ROOT/daisy/scripts/common.sh" && replace_rules_section "$prompt" "## Rules

1. Replacement rule.")
assert_contains "$replaced" "1. Replacement rule." "the new rules are written"
assert_not_contains "$replaced" "Measure before cutting" "the old rules are gone"
assert_contains "$replaced" "# Document Title" "the H1 between the sections survives the rewrite"
assert_contains "$replaced" "## Next Section" "the following section survives the rewrite"
assert_contains "$replaced" "Body text" "the following section's body survives the rewrite"

# A deeper heading is a *child* of the rules section, not its terminator —
# so it stays inside the span and is replaced along with the rules.
nested="$FIXTURE_ROOT/nested-prompt.md"
cat > "$nested" <<'EOF'
## Rules

1. First rule.

### Exceptions

2. Second rule.

## Next Section
EOF

nested_extract=$(source "$DAISY_ROOT/daisy/scripts/common.sh" && extract_rules_section "$nested")
assert_contains "$nested_extract" "### Exceptions" "an H3 is a subsection of the rules, not a boundary"
assert_contains "$nested_extract" "2. Second rule." "rules past an H3 subsection are still extracted"
assert_not_contains "$nested_extract" "Next Section" "the H2 that follows still bounds the section"

# Replacing a section with itself must be an identity. Command substitution
# strips the extract's trailing newlines, so without an explicit separator
# the replacer welds the rules onto the following heading — one blank line
# lost per run, on every file the loop touches.
roundtrip=$(source "$DAISY_ROOT/daisy/scripts/common.sh" \
    && replace_rules_section "$prompt" "$(extract_rules_section "$prompt")")
assert_text_eq "$roundtrip" "$(cat "$prompt")" "replacing the rules with themselves changes nothing"
