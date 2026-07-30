#!/usr/bin/env bash
# Invocation: run as `daisy healthcheck` — do not execute this file directly.
# Master health check for daisy system
# Validates global environment and runs all component health checks
# Exit 0 = healthy, Exit 1 = issues found
#
# Usage:
#   healthcheck.sh         - Run health check (cached for $DAISY_HEALTHCHECK_TTL seconds)
#   healthcheck.sh --force - Force re-run (ignore cache)
#
# Caching. A passing run stamps $DAISY_HOME/.healthcheck.ttl with the unix
# time the result expires; a run inside that window exits 0 without
# re-checking. The stamp is a file because it has to be: this script always
# runs as a subprocess (the `daisy` dispatcher, new-day.sh's gate), and a
# subprocess cannot write to its parent's environment. The previous
# mechanism exported DAISY_HEALTHCHECK_PASSED=1 one line before exiting,
# which set a variable in an environment that was discarded on the next
# statement -- so nothing was ever cached and every check ran every time.
#
# The cache fails toward re-running, deliberately. A cached *pass* is the
# dangerous direction: it can call a tree healthy on a result computed
# before the tree was edited, and the publication-hygiene scan is a leak
# check. So a failure clears the stamp, and a corrupt or out-of-range stamp
# is distrusted rather than honoured.
#
# DAISY_HEALTHCHECK_PASSED remains as an explicit external override for
# callers that have already validated (see tests/cases/17). It only works
# set from outside; --force outranks it.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

FORCE=0
if [ "$1" = "--force" ]; then
    FORCE=1
fi

# Seconds a passing result stays valid. 0 disables caching.
HEALTHCHECK_TTL="${DAISY_HEALTHCHECK_TTL:-300}"
case "$HEALTHCHECK_TTL" in
    ''|*[!0-9]*) HEALTHCHECK_TTL=300 ;;
esac

if [ "$FORCE" = "0" ] && [ "${DAISY_HEALTHCHECK_PASSED:-}" = "1" ]; then
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

# Function to check journal rotation state (warn-only, mirrors check_today())
check_journal_rotation() {
    local journal="$DAISY_HOME/journal/journal.md"
    [ -f "$journal" ] || return 0

    local count
    count=$(grep -c '^### [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' "$journal" 2>/dev/null) || count=0
    if [ "$count" -gt 5 ]; then
        warn "journal.md has $count day-blocks (expected ≤ 5) — run 'daisy rotate', or check that new-day/new-week are invoking it"
    fi

    local f
    for f in "$DAISY_HOME"/journal/journal-[0-9]*.md; do
        [ -f "$f" ] || continue
        if ! grep -q '^## Table of Contents' "$f" || ! grep -q '^## Summary' "$f"; then
            warn "$(basename "$f") is missing TOC/Summary front matter"
        fi
    done
}

# Enumerates the publishable surface as NUL-separated DAISY_ROOT-relative
# paths: git-tracked files plus untracked-but-not-ignored ones, minus the
# top-level home/. This is exactly what dz-meta's migrate.sh mirrors — it
# drives its rsync from the same enumeration — so git's ignore rules are the
# single declaration of what publishes. Per-machine workspace state
# (.daisy/, .claude/settings.local.json) and secrets (.env.sh) are ignored
# and therefore out of scope.
#
# daisy/templates/home/ is published, so only the top-level home/ is
# filtered, not every dir named "home". Symlinks to directories are skipped
# (the -f test): rsync copies the link, not the tree behind it.
#
# Without a git repo the ignore rules can't be consulted, so fall back to a
# full filesystem walk — over-reporting is the safe direction for a leak
# check.
publishable_files() {
    local f
    if [ -d "$DAISY_ROOT/.git" ]; then
        while IFS= read -r -d '' f; do
            case "$f" in home/*) continue ;; esac
            [ -f "$DAISY_ROOT/$f" ] && printf '%s\0' "$f"
        done < <(git -C "$DAISY_ROOT" ls-files --cached --others \
            --exclude-standard -z 2>/dev/null)
    else
        while IFS= read -r -d '' f; do
            f="${f#"$DAISY_ROOT"/}"
            case "$f" in home/*|.git/*) continue ;; esac
            printf '%s\0' "$f"
        done < <(find "$DAISY_ROOT" -type f -print0 2>/dev/null)
    fi
}

# Function to check publication hygiene — the publishable surface (see
# publishable_files) is mirrored verbatim by dz-meta's migrate.sh, so no file
# in it may contain identifying terms. Terms are derived from this machine at
# runtime (a hardcoded list would itself leak): real home names, $USER, the
# repo's git identity, and literal $HOME paths. Generic placeholder home
# names are exempt — they aren't identifying.
# See AGENTS.md "Publication Hygiene".
check_publication_hygiene() {
    local terms=() term name dir matches failed=0
    local placeholders=" work personal example testhome "

    for dir in "$DAISY_ROOT"/home/*/; do
        [ -d "$dir" ] || continue
        name=$(basename "$dir")
        case "$placeholders" in
            *" $name "*) ;;
            *) terms+=("$name") ;;
        esac
    done
    [ -n "${USER:-}" ] && terms+=("$USER")
    term=$(git -C "$DAISY_ROOT" config user.name 2>/dev/null || true)
    [ -n "$term" ] && terms+=("$term")
    term=$(git -C "$DAISY_ROOT" config user.email 2>/dev/null || true)
    [ -n "$term" ] && terms+=("$term")
    [ -n "${HOME:-}" ] && [ "$HOME" != "/" ] && terms+=("$HOME/")

    for term in "${terms[@]}"; do
        matches=$(publishable_files \
            | (cd "$DAISY_ROOT" && xargs -0 -r grep -liwF -- "$term") 2>/dev/null || true)
        [ -z "$matches" ] && continue
        error "Publication hygiene: '$term' appears in publishable file(s) — the publishable surface mirrors verbatim (see AGENTS.md: Publication Hygiene)"
        echo "$matches" | sed 's|^|    |' >&2
        failed=1
    done
    return $failed
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

# The stamp lives per-home and so can only be consulted once the home is
# resolved. Checks 1-2 above are a symlink read and an env lookup, and
# running them unconditionally means a cache can never mask a broken
# DAISY_ROOT or an unresolvable home.
HEALTHCHECK_STAMP="$DAISY_HOME/.healthcheck.ttl"

# True when the stamp records an unexpired pass. Anything unparseable, or
# dated further ahead than the TTL could legitimately produce (a corrupt
# write, a clock jump, a hand-edit), is treated as no cache at all.
healthcheck_cached() {
    [ "$FORCE" = "0" ] || return 1
    [ "$HEALTHCHECK_TTL" -gt 0 ] || return 1
    [ -f "$HEALTHCHECK_STAMP" ] || return 1

    local expiry now
    expiry=$(cat "$HEALTHCHECK_STAMP" 2>/dev/null || true)
    case "$expiry" in
        ''|*[!0-9]*) return 1 ;;
    esac

    now=$(date +%s)
    [ "$now" -lt "$expiry" ] || return 1
    [ "$expiry" -le "$(( now + HEALTHCHECK_TTL ))" ] || return 1
    return 0
}

if healthcheck_cached; then
    ok "Checks cached — valid for $(( $(cat "$HEALTHCHECK_STAMP") - $(date +%s) ))s (--force to re-run)"
    exit 0
fi

# Check 3: Git repository
if [ ! -d "$DAISY_ROOT/.git" ]; then
    error "DAISY_ROOT is not a git repository"
    exit 1
fi

ok "Git repository: $(cd "$DAISY_ROOT" && git rev-parse --short HEAD)"

# Check 4: Required directories
for dir in "home" "daisy/scripts" "prompts" "daisy/templates" "skills"; do
    if [ ! -d "$DAISY_ROOT/$dir" ]; then
        error "Missing directory: $dir"
        ERRORS=$((ERRORS + 1))
    fi
done

# Check 4b: Installed skill drift (warn-only) — a workspace's .claude/skills/*
# entry with no matching source anywhere under $DAISY_ROOT/skills/ or
# $DAISY_HOME/skills/ means the skill was removed/renamed upstream since the
# last `daisy init`. Sources may nest (e.g. skills/vendor/<source>/<name>/),
# so this matches by SKILL.md's containing-directory basename, not a fixed
# one-level path — same resolution `daisy-init.sh` uses to install them.
if [ -n "$DAISY_WORKSPACE" ] && [ -d "$DAISY_WORKSPACE/.claude/skills" ]; then
    KNOWN_SKILL_NAMES=" "
    while IFS= read -r -d '' skill_md; do
        KNOWN_SKILL_NAMES+="$(basename "$(dirname "$skill_md")") "
    done < <(find "$DAISY_ROOT/skills" "$DAISY_HOME/skills" -name SKILL.md -print0 2>/dev/null)

    for dir in "$DAISY_WORKSPACE"/.claude/skills/*/; do
        [ -d "$dir" ] || continue
        skill_name=$(basename "$dir")
        case "$KNOWN_SKILL_NAMES" in
            *" $skill_name "*) ;;
            *) warn "Installed skill '$skill_name' has no matching source in skills/ — stale from a removed/renamed skill; re-run 'daisy init' or remove .claude/skills/$skill_name manually" ;;
        esac
    done
fi

# Check 4c: Root-installed skill drift (warn-only) — `daisy install` copies
# $DAISY_ROOT/skills/* into $HOME/.claude/skills (machine-wide, not
# per-workspace). Resolved via $HOME, never a literal `~`, so the hermetic
# test fixture can override it. `daisy install --update` prunes this itself
# on every run; this check catches drift for installs that predate the prune
# or ran outside the CLI.
if [ -d "$HOME/.claude/skills" ]; then
    ROOT_SKILL_NAMES=" "
    while IFS= read -r -d '' skill_md; do
        ROOT_SKILL_NAMES+="$(basename "$(dirname "$skill_md")") "
    done < <(find "$DAISY_ROOT/skills" -name SKILL.md -print0 2>/dev/null)

    for dir in "$HOME/.claude/skills"/*/; do
        [ -d "$dir" ] || continue
        skill_name=$(basename "$dir")
        case "$ROOT_SKILL_NAMES" in
            *" $skill_name "*) ;;
            *) warn "Root-installed skill '$skill_name' has no matching source in \$DAISY_ROOT/skills — stale from a removed skill; re-run 'daisy install --update' or remove $HOME/.claude/skills/$skill_name manually" ;;
        esac
    done
fi

# Check 5: today.md format validation
if check_today; then
    ok "today.md format valid"
fi
# Continue even if check_today fails (errors already reported)

# Check 5b: journal rotation state (warn-only — never fails overall healthcheck)
check_journal_rotation

# Check 6: Run component health checks
HEALTHCHECK_SCRIPTS=(new-day.sh new-week.sh done.sh log.sh create-home.sh feedback.sh optimize.sh eval.sh files.sh list.sh plan-pickup.sh projects.sh rotate.sh test.sh tasks.sh)
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

# Check 6c: Publication hygiene — identifying terms in the publishable tree
if check_publication_hygiene; then
    ok "Publication hygiene: no identifying terms outside home/"
fi
# Continue even if it fails (errors already reported)

# Check 6b: AGENTS.md Script Reference table drift (warn-only)
for script_path in "$DAISY_ROOT"/daisy/scripts/*.sh; do
    script_name=$(basename "$script_path")
    if ! grep -q "\`$script_name\`" "$DAISY_ROOT/AGENTS.md"; then
        warn "$script_name is missing from AGENTS.md's Script Reference table"
    fi
done

# Check 7: Feedback stats
FEEDBACK_FILE="$DAISY_HOME/feedback/feedback.md"
if [ -f "$FEEDBACK_FILE" ]; then
    FEEDBACK_COUNT=$(grep -c "^## [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}" "$FEEDBACK_FILE" 2>/dev/null || true)
    LAST_OPT_FILE="$DAISY_HOME/feedback/.last-optimized"
    if [ -f "$LAST_OPT_FILE" ]; then
        LAST_OPT=$(cat "$LAST_OPT_FILE")
        ok "Feedback: $FEEDBACK_COUNT entries (last optimized: $LAST_OPT)"
    else
        ok "Feedback: $FEEDBACK_COUNT entries (never optimized)"
    fi
    if [ "${FEEDBACK_COUNT:-0}" -ge 20 ]; then
        warn "Feedback has $FEEDBACK_COUNT entries — consider running 'daisy optimize'"
    fi
fi

# Summary
echo ""
if [ $ERRORS -eq 0 ]; then
    ok "System healthy"
    # Stamp the pass. Best-effort: a read-only home is not a health failure,
    # it just means the next run re-checks.
    if [ "$HEALTHCHECK_TTL" -gt 0 ]; then
        echo "$(( $(date +%s) + HEALTHCHECK_TTL ))" > "$HEALTHCHECK_STAMP" 2>/dev/null || true
    fi
    exit 0
else
    error "$ERRORS issue(s) found"
    # Never let a stamp outlive the failure it precedes.
    rm -f "$HEALTHCHECK_STAMP" 2>/dev/null || true
    exit 1
fi
