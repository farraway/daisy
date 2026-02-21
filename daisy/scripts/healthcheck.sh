#!/usr/bin/env bash
# Master health check for daisy system
# Validates global environment and runs all component health checks
# Exit 0 = healthy, Exit 1 = issues found
#
# Usage:
#   healthcheck.sh         - Run health check (cached)
#   healthcheck.sh --force - Force re-run (ignore cache)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Handle --force flag
if [ "$1" = "--force" ]; then
    unset DAISY_HEALTHCHECK_PASSED
fi

# Check if already validated in this session (cached)
if [ "$DAISY_HEALTHCHECK_PASSED" = "1" ]; then
    exit 0
fi

ERRORS=0

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

error() {
    echo -e "${RED}✗${NC} $1" >&2
    ERRORS=$((ERRORS + 1))
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1" >&2
}

ok() {
    echo -e "${GREEN}✓${NC} $1"
}

# Function to check today.md format
check_today() {
    local today_file="$DAISY_HOME/journal/today.md"
    
    if [ ! -f "$today_file" ] || [ ! -s "$today_file" ]; then
        return 0  # Skip if file doesn't exist yet or is empty (pre-first-run)
    fi
    
    local found_issues=0
    
    # Check for required H4 headings
    local required_headings=("Agenda" "Tasks" "Log" "Retrospective")
    for heading in "${required_headings[@]}"; do
        if ! grep -q "^#### $heading" "$today_file"; then
            error "today.md missing required heading: #### $heading"
            found_issues=1
        fi
    done
    
    # Check blank line after #### Log
    if ! awk '/^#### Log/ {getline; if ($0 != "") exit 1}' "$today_file"; then
        error "today.md missing blank line after #### Log"
        found_issues=1
    fi
    
    # Check blank line before #### Retrospective
    if ! awk '/^#### Retrospective/ {if (prev != "") exit 1} {prev=$0}' "$today_file"; then
        error "today.md missing blank line before #### Retrospective"
        found_issues=1
    fi
    
    # Check no blank lines within log entries (after content starts, before section ends)
    local blank_violations=$(awk '
      BEGIN { in_log=0; in_whitespace=0; entries=0; violations=0 }
      /^#### Log/ { in_log=1; in_whitespace=1; next }
      in_log && /^#### / && entries > 0 && in_whitespace { violations-- }
      in_log && /^#### / { exit }
      in_log && in_whitespace && /^$/ { next }
      in_log && /^$/ && entries > 0 { in_whitespace=1; violations++; next }
      in_log && /^$/    { in_whitespace=1; violations++; next }
      in_log && /^[^$]/ { in_whitespace=0; entries++; next }
      END { print violations }
    ' "$today_file")
    
    if [ "$blank_violations" -gt 0 ]; then
        error "today.md has $blank_violations whitespace chunk(s) within log entries"
        found_issues=1
    fi
    
    if [ $found_issues -eq 0 ]; then
        return 0
    else
        return 1
    fi
}

# Check 1: DAISY_ROOT environment variable
if ! require_root 2>/dev/null; then
    error "DAISY_ROOT not set or invalid"
    echo "  Add to ~/.zshenv: export DAISY_ROOT=/path/to/daisy" >&2
    exit 1
fi

ok "DAISY_ROOT: $DAISY_ROOT"

# Check 2: DAISY_HOME (via .daisy/home or env var)
if ! resolve_home 2>/dev/null; then
    error "Cannot resolve home. No .daisy/home found and DAISY_HOME not set."
    echo "  Run 'daisy init <home>' in your workspace, or set DAISY_HOME in ~/.zshenv" >&2
    exit 1
fi

ok "DAISY_HOME: $DAISY_HOME (home: $DAISY_HOME_NAME)"

# Check 3: Git repository
if [ ! -d "$DAISY_ROOT/.git" ]; then
    error "DAISY_ROOT is not a git repository"
    exit 1
fi

ok "Git repository: $(cd "$DAISY_ROOT" && git rev-parse --short HEAD)"

# Check 4: Required directories
for dir in "home" "daisy/scripts" "prompts" "daisy/templates"; do
    if [ ! -d "$DAISY_ROOT/$dir" ]; then
        error "Missing directory: $dir"
        ERRORS=$((ERRORS + 1))
    fi
done

# Check 5: today.md format validation
if check_today; then
    ok "today.md format valid"
fi
# Continue even if check_today fails (errors already reported)

# Check 6: Run component health checks
HEALTHCHECK_SCRIPTS=(new-day.sh new-week.sh done.sh log.sh create-home.sh)
for script_name in "${HEALTHCHECK_SCRIPTS[@]}"; do
    script="$DAISY_ROOT/daisy/scripts/$script_name"
    if [ -f "$script" ] && [ -x "$script" ]; then
        if "$script" --healthcheck >/dev/null 2>&1; then
            ok "Component: $script_name"
        else
            error "Component: $script_name failed health check"
            "$script" --healthcheck 2>&1 | sed 's/^/  /' >&2
            ERRORS=$((ERRORS + 1))
        fi
    fi
done

# Summary
echo ""
if [ $ERRORS -eq 0 ]; then
    ok "System healthy"
    # Cache success for this session
    export DAISY_HEALTHCHECK_PASSED=1
    exit 0
else
    error "$ERRORS issue(s) found"
    exit 1
fi
