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
# runs as a subprocess (the `daisy` dispatcher, the day commands' gate), and a
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
# homes (daisy_root_exclusions: the same list a root commit leaves out).
# This is what a push of the root repository publishes, so git's ignore
# rules are the single declaration of what publishes. Per-machine workspace
# state (.daisy/, .claude/settings.local.json) and secrets (.env.sh) are
# ignored and therefore out of scope.
#
# templates/home/ is published: exclusions are root-relative paths,
# not every dir named "home". Symlinks to directories are skipped (the -f
# test); git publishes a link's text, which this scan does not read. Check 3
# has already required the root to be a repository, so the ignore rules are
# always available here.
publishable_files() {
    local f ex excluded=()
    mapfile -t excluded < <(daisy_root_exclusions)
    while IFS= read -r -d '' f; do
        for ex in "${excluded[@]}"; do
            case "$f" in "$ex"|"$ex"/*) continue 2 ;; esac
        done
        [ -f "$DAISY_ROOT/$f" ] && printf '%s\0' "$f"
    done < <(git -C "$DAISY_ROOT" ls-files --cached --others \
        --exclude-standard -z 2>/dev/null)
}

# Function to check publication hygiene — the publishable surface (see
# publishable_files) is published verbatim by every push of the root
# repository, so no file in it may contain identifying terms. Terms are
# derived from this machine at runtime (a hardcoded list would itself leak):
# real home names, $USER, the git identity of the root and of every
# registered home's repository, and literal $HOME paths. Generic placeholder
# home names are exempt — they aren't identifying.
# See AGENTS.md "Publication Hygiene".

# Adds to the caller's `terms` when non-empty and not already listed. The
# scan is case-insensitive, so the comparison is too: a term shared by two
# sources (the root's author and a home's; a home name and its author's
# login) is scanned, and reported, once.
add_term() {
    [ -n "$1" ] || return 0
    local t
    for t in "${terms[@]}"; do [ "${t,,}" = "${1,,}" ] && return 0; done
    terms+=("$1")
}

git_identity_terms() {
    add_term "$(git -C "$1" config user.name 2>/dev/null || true)"
    add_term "$(git -C "$1" config user.email 2>/dev/null || true)"
}

check_publication_hygiene() {
    local terms=() term name path matches failed=0
    local placeholders=" work personal example testhome "

    while IFS= read -r name; do
        case "$placeholders" in
            *" $name "*) ;;
            *) add_term "$name" ;;
        esac
        path=$(daisy_home_path "$name") && [ -z "$(daisy_home_path_problem "$path")" ] && git_identity_terms "$path"
    done < <(daisy_homes)
    add_term "${USER:-}"
    git_identity_terms "$DAISY_ROOT"
    [ -n "${HOME:-}" ] && [ "$HOME" != "/" ] && add_term "$HOME/"

    for term in "${terms[@]}"; do
        matches=$(publishable_files \
            | (cd "$DAISY_ROOT" && xargs -0 -r grep -liwF -- "$term") 2>/dev/null || true)
        [ -z "$matches" ] && continue
        error "Publication hygiene: '$term' appears in publishable file(s) — the publishable surface publishes verbatim (see AGENTS.md: Publication Hygiene)"
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

# Check 2: DAISY_HOME (via .daisy/home or DAISY_DEFAULT_HOME, then the
# registry). resolve_home's own message names the cause and the fix.
if ! resolve_home; then
    error "Cannot resolve home for this workspace"
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

# Check 3: Git repositories — root code commits in the root's, every data
# mutation (done, log, new-day, plans) in the home's. Without the home's
# repository each of those would write its files and then fail at the commit.
if ! daisy_is_repo_root "$DAISY_ROOT"; then
    error "DAISY_ROOT is not a git repository"
    exit 1
fi
if ! daisy_home_repo_ok "$DAISY_HOME"; then
    error "Home '$DAISY_HOME_NAME' at $DAISY_HOME is not in its own git repository (or the root's)"
    echo "  Run: git -C $(printf '%q' "$DAISY_HOME") init && git -C $(printf '%q' "$DAISY_HOME") add -A && git -C $(printf '%q' "$DAISY_HOME") commit -m 'New home: $DAISY_HOME_NAME'" >&2
    exit 1
fi

ok "Git repository: $(cd "$DAISY_ROOT" && git rev-parse --short HEAD)"

# Check 3b: a registered home inside the root repository is the layout
# before the split. It works — commits are scoped by path — but it is not
# the intended state, so it is reported until the home moves out. A home
# under the root's path that is its own repository is not inside it.
while IFS=$'\t' read -r name rel; do
    daisy_is_repo_root "$DAISY_ROOT/$rel" && continue
    warn "Home '$name' lies inside the root repository ($rel) — move it to its own repository and re-register it: daisy home remove $name && daisy home add $name /new/path"
done < <(daisy_homes_under_root)

# Check 4: Required directories
for dir in "daisy/scripts" "prompts" "templates" "skills"; do
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
# one-level path — the same resolution `daisy init` uses to install them.
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
HEALTHCHECK_SCRIPTS=(feedback.sh optimize.sh eval.sh files.sh test.sh)
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

# Commands the Go binary has taken over answer `--healthcheck` themselves.
DAISY_BIN=$(daisy_bin)
# An entry may be a noun and its verb, split into words for the call.
HEALTHCHECK_COMMANDS=(done log tasks projects new-day new-week rotate list "home new" "plan new" "plan pickup" "plan archive")
for cmd in "${HEALTHCHECK_COMMANDS[@]}"; do
    # shellcheck disable=SC2086  # the words are meant to split
    if "$DAISY_BIN" $cmd --healthcheck >/dev/null 2>&1; then
        ok "Component: $cmd"
    else
        error "Component: $cmd failed health check"
        # shellcheck disable=SC2086
        "$DAISY_BIN" $cmd --healthcheck 2>&1 | sed 's/^/  /' >&2
        ERRORS=$((ERRORS + 1))
    fi
done

# Check 6c: Publication hygiene — identifying terms in the publishable tree
if check_publication_hygiene; then
    ok "Publication hygiene: no identifying terms outside the homes"
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
