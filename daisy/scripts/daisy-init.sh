#!/usr/bin/env bash
# Invocation: run as `daisy init` — do not execute this file directly.
# Initialize Daisy in a workspace with an identity and its home.
# Creates .daisy/ directory with symlinks to the home's data and applies the
# identity to the workspace repository's local git config.
#
# Usage:
#   daisy-init <identity>              # init in current directory
#   daisy-init <identity> /path/to/ws  # init in specified directory
#   daisy-init --new <home>            # create a new home (daisy home new), then init
#
# <identity> is a registered home's name (its default identity, from the
# home's gitconfig) or the name of a <home>/identities/<identity> file in
# exactly one registered home. Either way the workspace binds to that home;
# a non-default identity is recorded in .daisy/identity.
#
# Re-running with a different identity switches the workspace to it: the
# previous identity's git config is unset where it still matches, then the
# new one is applied.
#
# Called via `daisy init` (see $DAISY_ROOT/daisy.sh). Can also be invoked directly.

set -e

if [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set" >&2
    echo "Add to ~/.zshenv:  export DAISY_ROOT=\"/path/to/daisy\"" >&2
    exit 1
fi

source "$DAISY_ROOT/daisy/scripts/common.sh"

# Parse arguments
CREATE_NEW=false
if [ "$1" = "--new" ]; then
    CREATE_NEW=true
    shift
fi

IDENTITY="$1"
TARGET="${2:-.}"

if [ -z "$IDENTITY" ]; then
    echo "Usage: daisy-init [--new] <identity> [workspace-path]" >&2
    echo "" >&2
    echo "Options:" >&2
    echo "  --new    Create a new home of that name before initializing" >&2
    echo "" >&2
    echo "Identities (identity, home):" >&2
    daisy_identities | awk -F'\t' '{ printf "  %-14s %s\n", $1, $2 }' >&2
    exit 1
fi

# Create new home if requested
if [ "$CREATE_NEW" = true ]; then
    "$DAISY_ROOT/daisy/scripts/home.sh" new "$IDENTITY"
fi

# Resolve the identity to its home, and the home through the registry.
HOME_NAME=$(daisy_identity_home "$IDENTITY") || exit 1
HOME_DIR=$(daisy_home_dir "$HOME_NAME") || exit 1

# Read the identity before writing anything. The default identity is
# optional (a home may not have one yet: `home new` lets the prompt go
# unanswered); a named identity is what was asked for, so it must be whole.
IDENTITY_FILE=$(daisy_identity_file "$HOME_DIR" "$HOME_NAME" "$IDENTITY")
IDENTITY_OK=true
if [ "$IDENTITY" = "$HOME_NAME" ]; then
    daisy_identity_read "$IDENTITY_FILE" 2>/dev/null || IDENTITY_OK=false
else
    daisy_identity_read "$IDENTITY_FILE" || exit 1
fi

TARGET="$(cd "$TARGET" && pwd)"

# A linked worktree has no git config of its own: an identity set here would
# be the main checkout's, and `daisy clean` here would strip it from there.
if daisy_is_repo_root "$TARGET" && daisy_is_linked_worktree "$TARGET"; then
    echo "Error: $TARGET is a linked worktree; its git config belongs to the repository at $(daisy_git_common_dir "$TARGET")" >&2
    echo "  Run 'daisy init $IDENTITY' in that repository's main checkout instead" >&2
    exit 1
fi
# The root is a directory named .daisy under the user's home, by whatever
# path: the directory holding it is not a workspace, and the swap below
# would replace the root.
if [ -d "$TARGET/.daisy" ] && ! daisy_is_workspace_dir "$TARGET"; then
    echo "Error: $TARGET/.daisy is the Daisy root, not a workspace binding; to bind the root itself, run 'daisy init $IDENTITY' inside it" >&2
    exit 1
fi
# init owns .daisy/ as a directory of its own: a symlink there would be
# replaced by the swap while its target kept the old binding.
if [ -L "$TARGET/.daisy" ]; then
    echo "Error: $TARGET/.daisy is a symlink; init binds a directory of its own — move it aside first" >&2
    exit 1
fi
if [ -e "$TARGET/.daisy" ] && [ ! -d "$TARGET/.daisy" ]; then
    echo "Error: $TARGET/.daisy exists and is not a directory; move it aside first" >&2
    exit 1
fi
# What an interrupted swap (below) leaves: .daisy/ moved aside and no .daisy/,
# or a staging directory. Both are init's own (daisy_init_leftovers); reclaim
# them before anything else reads the binding. With no .daisy/ and more than
# one previous directory nothing can tell which binding is real: refuse.
PREVIOUS=() STALE_STAGES=()
while IFS= read -r LEFTOVER; do
    case "$LEFTOVER" in
        */.daisy.previous.*) PREVIOUS+=("$LEFTOVER") ;;
        *) STALE_STAGES+=("$LEFTOVER") ;;  # consulted and removed in the final section
    esac
done < <(daisy_init_leftovers "$TARGET")
if [ ! -e "$TARGET/.daisy" ] && [ ${#PREVIOUS[@]} -gt 1 ]; then
    echo "Error: no .daisy/ here but ${#PREVIOUS[@]} directories an earlier init moved aside: ${PREVIOUS[*]}" >&2
    echo "  Move the one holding the binding back to .daisy/ and remove the others, then re-run" >&2
    exit 1
fi
for LEFTOVER in "${PREVIOUS[@]}"; do
    if [ ! -e "$TARGET/.daisy" ]; then
        mv "$LEFTOVER" "$TARGET/.daisy"
        echo "  ⚠ Reclaimed $LEFTOVER as .daisy/ (an earlier init was interrupted mid-swap)" >&2
    else
        rm -rf "${LEFTOVER:?}"
        echo "  ⚠ Removed $LEFTOVER (what .daisy/ held before an earlier init's swap)" >&2
    fi
done

echo "Initializing Daisy in: $TARGET"
echo "  Home: $HOME_NAME"
echo "  Identity: $IDENTITY"

# --- git init ---

# The top level of a repository already, a worktree's gitfile included, is
# left alone; anything else (including a subdirectory of some other
# repository) becomes its own.
if ! daisy_is_repo_root "$TARGET"; then
    git init -q "$TARGET"
    echo "  ✓ Initialized git repository"
fi

# --- .daisy/ directory with home config and symlinks ---

DAISY_DIR="$TARGET/.daisy"

# If .daisy/ already exists, say whether the home or the identity changes.
# Nothing in .daisy/ or the repository's config is touched until the final
# section, so a failure before it leaves the workspace as it was.
OLD_HOME="" OLD_IDENTITY=""
if [ -d "$DAISY_DIR" ] && [ -f "$DAISY_DIR/home" ]; then
    OLD_HOME=$(cat "$DAISY_DIR/home")
    OLD_IDENTITY=$(daisy_workspace_identity "$DAISY_DIR" "$OLD_HOME")
    if [ "$OLD_HOME" = "$HOME_NAME" ] && [ "$OLD_IDENTITY" = "$IDENTITY" ]; then
        echo "  ✓ Already initialized with identity: $IDENTITY"
    elif [ "$OLD_HOME" = "$HOME_NAME" ]; then
        echo "  ↻ Switching identity: $OLD_IDENTITY → $IDENTITY"
    else
        echo "  ↻ Switching home: $OLD_HOME → $HOME_NAME"
    fi
fi

# Generate AGENTS.md outside the workspace; it moves into .daisy/ in the
# final section with the rest of the binding. Until then no .daisy/ exists
# where there was none: a bare one would make this directory a workspace
# that resolves to the enclosing workspace's home.
AGENTS_TMP=$(mktemp)
trap 'rm -f "$AGENTS_TMP"' EXIT
DAISY_HOME="$HOME_DIR" "$DAISY_ROOT/daisy/scripts/build-prompt.sh" \
    --output "$AGENTS_TMP" "$HOME_NAME"

# --- Cursor rules ---

RULES_DIR="$TARGET/.cursor/rules"
mkdir -p "$RULES_DIR"

for RULE_NAME in "${DAISY_CURSOR_RULE_FILES[@]}"; do
    TEMPLATE="cursor-rule${RULE_NAME#daisy}"
    RULE_FILE="$RULES_DIR/$RULE_NAME"
    rm -f "$RULE_FILE"
    cp "$DAISY_ROOT/daisy/templates/$TEMPLATE" "$RULE_FILE"
    echo "  ✓ Installed Cursor rule: $RULE_NAME"
done

# Sweep obsolete rule names left behind by a prior .md → .mdc rename or a
# name later dropped from DAISY_CURSOR_RULE_FILES — daisy-init.sh only ever
# writes the current names, so nothing else removes these.
OBSOLETE_SWEPT=0
for OBSOLETE_NAME in "${DAISY_CURSOR_RULE_OBSOLETE[@]}"; do
    OBSOLETE_FILE="$RULES_DIR/$OBSOLETE_NAME"
    if [ -f "$OBSOLETE_FILE" ]; then
        rm -f "$OBSOLETE_FILE"
        OBSOLETE_SWEPT=$((OBSOLETE_SWEPT + 1))
        echo "  ✓ Removed obsolete Cursor rule: $OBSOLETE_NAME"
    fi
done

# --- Claude command ---

CLAUDE_COMMAND_PATH="$TARGET/$DAISY_CLAUDE_COMMAND_FILE"
mkdir -p "$(dirname "$CLAUDE_COMMAND_PATH")"
cp "$DAISY_ROOT/daisy/templates/claude-command.md" "$CLAUDE_COMMAND_PATH"
echo "  ✓ Installed Claude command: daisy"

# --- Skills ---
# Home skills only — root skills install separately via `daisy install`
# (user-level, ~/.claude/skills/), not per-workspace. Copied (not symlinked)
# so a workspace's skill set is a point-in-time snapshot, refreshed only by
# re-running `daisy init`. Only the specific skill-named subdirectories/files
# below are touched — anything else already in .claude/skills/ or
# .cursor/rules/ (unrelated, user-managed) is left alone.

SKILLS_CLAUDE_DIR="$TARGET/.claude/skills"
SKILLS_CURSOR_DIR="$TARGET/.cursor/rules"
mkdir -p "$SKILLS_CLAUDE_DIR" "$SKILLS_CURSOR_DIR"

declare -A SKILL_SRC
# find, not a one-level glob — a home's skills could nest, matching root's
# discovery convention. The skill's name is always the immediate parent
# directory of its SKILL.md, regardless of how deep that directory sits.
while IFS= read -r -d '' skill_md; do
    dir="$(dirname "$skill_md")"
    SKILL_SRC["$(basename "$dir")"]="$dir/"
done < <(find "$HOME_DIR/skills" -name SKILL.md -print0 2>/dev/null)

SKILL_COUNT=0
for name in "${!SKILL_SRC[@]}"; do
    src="${SKILL_SRC[$name]}"

    # Claude Code: copy the full skill directory (SKILL.md plus any sibling
    # reference files) so nothing a skill discloses to gets left behind.
    rm -rf "${SKILLS_CLAUDE_DIR:?}/$name"
    cp -r "$src" "$SKILLS_CLAUDE_DIR/$name"

    # Cursor has no workspace-scoped skills directory, so deliver the same
    # skill as a pointer rule — same "Read X when Y" pattern cursor-rule.mdc
    # already uses to deliver .daisy/AGENTS.md.
    desc=$(awk -F': ' '/^description:/{ $1=""; sub(/^ /, ""); print; exit }' "$src/SKILL.md")
    desc="${desc%\"}"; desc="${desc#\"}"
    cat > "$SKILLS_CURSOR_DIR/$name.mdc" <<EOF
---
description: $desc
alwaysApply: false
---

# $name

Read \`.claude/skills/$name/SKILL.md\` when: $desc
EOF

    SKILL_COUNT=$((SKILL_COUNT + 1))
done
echo "  ✓ Installed $SKILL_COUNT skill(s) into .claude/skills/ and .cursor/rules/"

# Sweep skill .mdc stubs (and .claude/skills/ copies) whose home skill no
# longer exists in $HOME_DIR/skills — otherwise a deleted skill's pointer
# rule and copy linger in the workspace forever after the next `daisy init`.
SKILL_STUB_SWEPT=0
for MDC_FILE in "$SKILLS_CURSOR_DIR"/*.mdc; do
    [ -f "$MDC_FILE" ] || continue
    STUB_NAME="$(basename "$MDC_FILE" .mdc)"
    case " ${DAISY_CURSOR_RULE_FILES[*]} " in
        *" $STUB_NAME.mdc "*) continue ;;  # core rule, not a skill stub
    esac
    if [ -z "${SKILL_SRC[$STUB_NAME]+x}" ]; then
        rm -f "$MDC_FILE"
        rm -rf "${SKILLS_CLAUDE_DIR:?}/$STUB_NAME"
        SKILL_STUB_SWEPT=$((SKILL_STUB_SWEPT + 1))
    fi
done
if [ "$SKILL_STUB_SWEPT" -gt 0 ]; then
    echo "  ✓ Swept $SKILL_STUB_SWEPT stale skill stub(s) from .claude/skills/ and .cursor/rules/"
fi

# --- Claude Code permissions ---

CLAUDE_SETTINGS="$TARGET/.claude/settings.local.json"
DAISY_ALLOW=("${DAISY_ALLOW_BASE[@]}")

if command -v jq >/dev/null 2>&1; then
    mkdir -p "$(dirname "$CLAUDE_SETTINGS")"
    existing='{}'
    [ -f "$CLAUDE_SETTINGS" ] && existing=$(cat "$CLAUDE_SETTINGS")
    updated="$existing"
    # Heal existing workspaces: remove the legacy raw-scripts grant superseded by Bash(daisy:*)
    updated=$(echo "$updated" | jq --arg r "Bash($DAISY_ROOT/daisy/scripts/*)" '
        if .permissions.allow then
            .permissions.allow |= map(select(. != $r))
        else . end
    ')
    for rule in "${DAISY_ALLOW[@]}"; do
        updated=$(echo "$updated" | jq \
            --arg r "$rule" \
            '.permissions.allow //= [] | if (.permissions.allow | index($r)) == null then .permissions.allow += [$r] else . end')
    done
    echo "$updated" | jq '.' > "$CLAUDE_SETTINGS"
    echo "  ✓ Configured .claude/settings.local.json with Daisy permissions"
else
    echo "  ⚠ jq not available — skipping Claude Code permissions (add manually)"
fi

# --- .gitignore ---

GITIGNORE="$TARGET/.gitignore"
DAISY_IGNORE_ENTRIES=("${DAISY_GITIGNORE_BASE_ENTRIES[@]}")
for name in "${!SKILL_SRC[@]}"; do
    DAISY_IGNORE_ENTRIES+=(".claude/skills/$name/" ".cursor/rules/$name.mdc")
done
GITIGNORE_CHANGED=false

if [ ! -f "$GITIGNORE" ]; then
    touch "$GITIGNORE"
    echo "  ✓ Created .gitignore"
fi

NEED_HEADER=true
grep -qxF "# Daisy workspace config (per-machine)" "$GITIGNORE" 2>/dev/null && NEED_HEADER=false
for entry in "${DAISY_IGNORE_ENTRIES[@]}"; do
    if ! grep -qxF "$entry" "$GITIGNORE" 2>/dev/null; then
        if [ "$NEED_HEADER" = true ]; then
            echo "" >> "$GITIGNORE"
            echo "# Daisy workspace config (per-machine)" >> "$GITIGNORE"
            NEED_HEADER=false
        fi
        echo "$entry" >> "$GITIGNORE"
        GITIGNORE_CHANGED=true
    fi
done

if [ "$GITIGNORE_CHANGED" = true ]; then
    echo "  ✓ Added daisy paths to .gitignore"
fi

# --- .cursorignore ---

CURSORIGNORE="$TARGET/.cursorignore"
CURSOR_NEGATE_ENTRIES=("${DAISY_CURSORIGNORE_BASE_ENTRIES[@]}")
CURSORIGNORE_CHANGED=false

if [ ! -f "$CURSORIGNORE" ]; then
    touch "$CURSORIGNORE"
fi

NEED_HEADER=true
grep -qxF "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" "$CURSORIGNORE" 2>/dev/null && NEED_HEADER=false
for entry in "${CURSOR_NEGATE_ENTRIES[@]}"; do
    if ! grep -qxF "$entry" "$CURSORIGNORE" 2>/dev/null; then
        if [ "$NEED_HEADER" = true ]; then
            echo "" >> "$CURSORIGNORE"
            echo "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" >> "$CURSORIGNORE"
            NEED_HEADER=false
        fi
        echo "$entry" >> "$CURSORIGNORE"
        CURSORIGNORE_CHANGED=true
    fi
done

if [ "$CURSORIGNORE_CHANGED" = true ]; then
    echo "  ✓ Configured .cursorignore to index daisy paths"
fi

# --- Binding and git identity ---
# Last, and as a whole. The new binding — .daisy/home, .daisy/identity, the
# identity record .daisy/gitconfig, the symlinks, AGENTS.md, plus whatever
# else .daisy/ already holds — is prepared in a sibling directory; then the
# identity a previous init applied (its record; for a workspace bound before
# records, the home's own file) is unset and the new one applied; then the
# prepared directory is swapped in by rename. Up to the swap nothing
# in .daisy/ has changed, so the previous record is intact for the rollback
# (init_rollback) that any failure in here triggers: the identity applied is
# taken back, the one cleared is put back, the staging directory goes, and a
# .daisy/ moved aside for the swap is moved back. `status` and `files`
# report whatever a rollback could not undo.

OLD_RECORD="" CLEARED="" TOUCHED=false BOUND=false STAGE="" ASIDE="" SNAPSHOT=""
init_rollback() {
    set +e  # every step here runs, whatever an earlier one could not do
    rm -f "${AGENTS_TMP:?}"
    if [ "$BOUND" = true ]; then rm -f "${SNAPSHOT:-/nonexistent}"; return 0; fi
    echo "Error: init did not finish; the binding and git identity are as they were before this run's final step" >&2
    if [ -n "$ASIDE" ] && [ -d "$ASIDE" ]; then
        rm -rf "${DAISY_DIR:?}"
        mv "$ASIDE" "$DAISY_DIR" || echo "  the previous .daisy/ is at $ASIDE and could not be moved back" >&2
    fi
    # The three keys exactly as they were before the identity step, a
    # hand-set value included — not what any identity file says.
    if [ "$TOUCHED" = true ]; then
        if ! daisy_identity_restore "$TARGET" "$SNAPSHOT"; then
            echo "  the previous git identity could not be put back; see 'daisy status'" >&2
        fi
    fi
    rm -f "${SNAPSHOT:-/nonexistent}"
    if [ -n "$STAGE" ] && ! rm -rf "${STAGE:?}"; then
        echo "  the staging directory $STAGE could not be removed; remove it by hand" >&2
    fi
    return 0
}
trap init_rollback EXIT  # supersedes the AGENTS_TMP trap above; it removes the file too

# 1. Prepare the new binding beside .daisy/ (a name find_workspace_root
#    does not take for a workspace).
STAGE=$(mktemp -d "$TARGET/.daisy.stage.XXXXXX")
if [ -d "$DAISY_DIR" ]; then
    cp -a "$DAISY_DIR/." "$STAGE/"
else
    chmod 755 "$STAGE"  # mktemp's 0700 is not what a fresh .daisy/ should be
fi
# Every file init owns is removed before it is written: a symlink standing
# in its place (copied from .daisy/) must not be written through.
rm -f "${STAGE:?}/home" "${STAGE:?}/identity" "${STAGE:?}/gitconfig"
echo "$HOME_NAME" > "$STAGE/home"
# The default identity is the absence of .daisy/identity.
if [ "$IDENTITY" != "$HOME_NAME" ]; then
    echo "$IDENTITY" > "$STAGE/identity"
fi
while IFS=$'\t' read -r link target; do
    rm -f "${STAGE:?}/${link:?}"
    ln -s "$HOME_DIR/$target" "$STAGE/$link"
done < <(daisy_workspace_links)
rm -f "${STAGE:?}/AGENTS.md"  # may be symlink (old) or file (new)
mv "$AGENTS_TMP" "$STAGE/AGENTS.md"
chmod 644 "$STAGE/AGENTS.md"  # mktemp's 0600 is not what a file beside .daisy/home should be

# 2. The git identity: previous one unset, new one applied. Snapshot first,
#    so the rollback can put the keys back exactly. A staging directory an
#    earlier init left holds the record of what that init applied: it is
#    unset against here, under the same cover, and removed only once this
#    init has completed.
SNAPSHOT=$(mktemp)
daisy_identity_snapshot "$TARGET" "$SNAPSHOT"
TOUCHED=true
for STALE in "${STALE_STAGES[@]}"; do
    CLEARED=$(daisy_identity_unset "$TARGET" "$STALE/gitconfig")
    [ -z "$CLEARED" ] || echo "  ✓ Cleared the git identity an interrupted init's record names ($CLEARED)"
done
if [ -d "$DAISY_DIR" ]; then
    if OLD_RECORD=$(daisy_identity_record_file "$DAISY_DIR" "$OLD_HOME" "$OLD_IDENTITY"); then
        [ "$OLD_RECORD" = "$DAISY_DIR/gitconfig" ] ||
            echo "  ⚠ No readable .daisy/gitconfig record of the previous identity; unsetting against $OLD_RECORD as it is now" >&2
    elif [ -n "$OLD_HOME" ] || [ -e "$DAISY_DIR/gitconfig" ]; then
        echo "  ⚠ No readable .daisy/gitconfig record of the previous identity, and no identity file resolves for home '$OLD_HOME' identity '$OLD_IDENTITY'; only a key of Daisy's form can be recognised as its" >&2
    fi
fi
# With no record (a first init included) this still takes a key of Daisy's
# form that an init elsewhere, or one that left no binding, applied here.
CLEARED=$(daisy_identity_unset "$TARGET" "$OLD_RECORD")
if [ -n "$CLEARED" ]; then
    if [ -n "$OLD_HOME" ]; then
        echo "  ✓ Cleared previous git identity ($CLEARED)"
    else
        echo "  ✓ Cleared a git identity an earlier init left here ($CLEARED)"
    fi
fi
if [ "$IDENTITY_OK" = true ]; then
    # One read serves the record and the apply, so the two agree even if the
    # file is edited while init runs; the record first, so whatever is
    # applied is claimed by a record (the stage's, until the swap) at every
    # instant — a kill in between leaves nothing unclaimed.
    daisy_identity_read "$IDENTITY_FILE"
    daisy_identity_record "$STAGE/gitconfig"
    daisy_identity_apply "$TARGET"
    if [ -n "$IDENTITY_SSH_COMMAND" ]; then
        echo "  ✓ Set local git identity: $IDENTITY_NAME <$IDENTITY_EMAIL>, pushing with key $IDENTITY_SSHKEY"
        # The key may be made later; until then every push from here fails,
        # and this is where the reason is on record. Only paths this shell
        # can spell are checked: `~/...` and absolute.
        case "$IDENTITY_SSHKEY" in
            "~/"*) KEY_PATH="$HOME/${IDENTITY_SSHKEY#\~/}" ;;
            /*)    KEY_PATH="$IDENTITY_SSHKEY" ;;
            *)     KEY_PATH="" ;;
        esac
        [ -z "$KEY_PATH" ] || [ -e "$KEY_PATH" ] || echo "  ⚠ SSH key $IDENTITY_SSHKEY not found; pushes from this workspace will fail until it exists" >&2
    else
        echo "  ✓ Set local git identity: $IDENTITY_NAME <$IDENTITY_EMAIL>"
    fi
else
    echo "  ⚠ Home '$HOME_NAME' has no complete default identity at $IDENTITY_FILE (git identity not configured)" >&2
    echo "    Write name= and email= lines there, then re-run 'daisy init $HOME_NAME'" >&2
fi

# 3. The swap.
if [ -d "$DAISY_DIR" ]; then
    ASIDE="$TARGET/.daisy.previous.$$"
    mv "$DAISY_DIR" "$ASIDE"
fi
mv "$STAGE" "$DAISY_DIR"
BOUND=true
STAGE=""
echo "  ✓ Generated .daisy/AGENTS.md"
echo "  ✓ Bound .daisy/ to home: $HOME_NAME"
# The binding is in place; what could not be tidied is said, not failed on.
for STALE in "${STALE_STAGES[@]}" ${ASIDE:+"$ASIDE"}; do
    rm -rf "${STALE:?}" || echo "  ⚠ $STALE could not be removed; remove it by hand (the next init or clean will try again)" >&2
done

echo ""
echo "Done. Daisy is ready in this workspace."
echo "Start a Cursor session and say: \"Daisy, start a new day\""
echo "Or use /daisy in Claude Code: \"/daisy start a new day\""
