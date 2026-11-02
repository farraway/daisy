#!/usr/bin/env bash
# Invocation: sourced by other daisy scripts — not executed directly, no `daisy` command.
# Shared functions for daisy scripts
# Source this at the top of each script:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/common.sh"

# Bash >= 5.2 defaults 'patsub_replacement' to on, which makes '&' in the
# replacement side of ${var//pattern/replacement} mean "the matched text"
# (sed-style). Every script that builds today.md by substituting task text
# into a template via this construct silently corrupts any '&' in a task
# description unless this is off. No-op (with the harmless error suppressed)
# on bash < 5.2, which doesn't have the option.
shopt -u patsub_replacement 2>/dev/null || true

# Every git call here is `git -C <dir>` and means that directory's repository
# and that repository's configured author. Inherited from a git hook's
# environment, these would point every call at the hook's repository, or sign
# a Daisy commit with the hook's author, instead.
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
    GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_AUTHOR_DATE \
    GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_COMMITTER_DATE \
    GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT "${!GIT_CONFIG_KEY_@}" "${!GIT_CONFIG_VALUE_@}"

# --- Homes registry ---
# A home is the data: journal, tasks, projects, plans, feedback, and the
# prompts and skills that shadow the root's. It lives wherever its own
# repository is; the root knows homes only by name, through the registry at
# $DAISY_CONFIG/homes (default ~/.config/daisy/homes), one `name=/abs/path`
# per line. Nothing derives a home's path from $DAISY_ROOT.
#
# Two per-machine directories: $DAISY_CONFIG (~/.config/daisy) holds
# configuration, today only the registry; $DAISY_LOCAL (~/.local/daisy) holds
# data — the default location `home new` offers is $DAISY_LOCAL/homes/<name>.

daisy_config_dir() {
    echo "${DAISY_CONFIG:-$HOME/.config/daisy}"
}

daisy_homes_file() {
    echo "$(daisy_config_dir)/homes"
}

# Registered home names, one per line, in registry order. Names and paths
# are trimmed of surrounding whitespace; comments and blank lines are skipped.
daisy_homes() {
    local file
    file=$(daisy_homes_file)
    [ -f "$file" ] || return 0
    awk -F= 'NF >= 2 && $1 !~ /^[[:space:]]*(#|$)/ {
        path = $0; sub(/^[^=]*=/, "", path)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", path)
        sub(/\/+$/, "", path)
        if (path == "") next
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1); print $1
    }' "$file"
}

# Echoes the registered path of a home, trimmed of surrounding whitespace
# and trailing slashes; returns 1 if the name is unregistered.
daisy_home_path() {
    local file path
    file=$(daisy_homes_file)
    [ -f "$file" ] || return 1
    path=$(awk -F= -v name="$1" '
        NF >= 2 { key = $1; gsub(/^[[:space:]]+|[[:space:]]+$/, "", key) }
        NF >= 2 && key == name {
            sub(/^[^=]*=/, "")
            gsub(/^[[:space:]]+|[[:space:]]+$/, "")
            sub(/\/+$/, "")
            if ($0 == "") next
            print; exit
        }' "$file")
    [ -n "$path" ] || return 1
    echo "$path"
}

# Echoes why a path cannot be a home, or nothing when it can. The one
# validator for every reader of the registry and for `home add`: the path
# must be absolute (the registry is read from any directory), present, and
# neither the root nor a directory holding the root — a home commit in
# either would commit the root under a Daisy message. A home inside the
# root is allowed: that is the layout until the split.
daisy_home_path_problem() {
    local path="$1" real root
    case "$path" in /*) ;; *) echo "is not an absolute path"; return 0 ;; esac
    real=$(daisy_realdir "$path") || { echo "does not exist"; return 0; }
    root=$(daisy_realdir "$DAISY_ROOT")
    if [ "$real" = "$root" ]; then
        echo "is the Daisy root; the root cannot be a home"
        return 0
    fi
    if [ "${root#"${real%/}/"}" != "$root" ]; then
        echo "contains the Daisy root; the root cannot be inside a home"
    fi
    return 0
}

# Echoes the directory of a registered home that can be used as one. On
# failure prints the error and the command that fixes it, and returns 1.
daisy_home_dir() {
    local name="$1" path problem
    path=$(daisy_home_path "$name") || {
        echo "Error: Home '$name' is not registered in $(daisy_homes_file)" >&2
        echo "  Run 'daisy home list' to see registered homes, or 'daisy home add $name /path/to/home'" >&2
        return 1
    }
    problem=$(daisy_home_path_problem "$path")
    if [ -n "$problem" ]; then
        echo "Error: Home '$name' is registered at $path, which $problem" >&2
        echo "  Run 'daisy home remove $name', then 'daisy home add $name /path/to/home' with its location" >&2
        return 1
    fi
    echo "$path"
}

# Registered homes whose directory lies inside $DAISY_ROOT, one per line as
# `name<TAB>path-relative-to-root`. Until the split this is every real home;
# afterwards it is empty. It is the one definition of what a root commit
# leaves out and what the health check reports as not yet moved. Paths are
# compared resolved, so a symlinked root or home still matches. An entry
# that cannot be a home (daisy_home_path_problem) is not listed.
daisy_homes_under_root() {
    local root name path
    root=$(daisy_realdir "$DAISY_ROOT") || return 0
    while IFS=$'\t' read -r name path; do
        path=$(daisy_realdir "$path")
        case "$path/" in
            "$root"/*) printf '%s\t%s\n' "$name" "${path#"$root"/}" ;;
        esac
    done < <(daisy_usable_homes)
}

# Registered homes that can be used as homes (daisy_home_path_problem), one
# `name<TAB>path` per line in registry order, each name once (the line
# daisy_home_path reads). The one walk for every reader that needs a home's
# files rather than only its name.
daisy_usable_homes() {
    local name path
    local -A seen=()
    while IFS= read -r name; do
        [ -z "${seen[$name]+x}" ] || continue
        seen[$name]=1
        path=$(daisy_home_path "$name") || continue
        [ -z "$(daisy_home_path_problem "$path")" ] || continue
        printf '%s\t%s\n' "$name" "$path"
    done < <(daisy_homes)
    return 0
}

# Echoes the resolved absolute path of an existing directory; returns 1 when
# it cannot be entered. The one spelling of a directory for every comparison
# here: `pwd -P` keeps a POSIX-special leading `//`, under which the same
# directory compares unequal to itself.
daisy_realdir() {
    local real
    [ -n "$1" ] || return 1
    real=$(cd "$1" 2>/dev/null && pwd -P) || return 1
    while [ "${real#//}" != "$real" ]; do real="${real#/}"; done
    echo "$real"
}

# Echoes the resolved top level of the repository containing a directory;
# returns 1 when there is none. A worktree or a .git gitfile counts, which a
# `-d .git` test would miss.
daisy_repo_toplevel() {
    git -C "$1" rev-parse --show-toplevel 2>/dev/null
}

# True when the directory is the top level of its own repository — the one
# test for "this is a repository", not "this is inside one".
daisy_is_repo_root() {
    local real
    real=$(daisy_realdir "$1") || return 1
    [ "$(daisy_repo_toplevel "$1")" = "$real" ]
}

# A home's commits may land in its own repository or, until the split, the
# root's — never in whichever larger repository happens to enclose it (a
# dotfiles-managed $HOME would silently receive every journal entry). The
# root's repository counts only when the root is one: a root that is itself
# a subdirectory of a foreign repository is that foreign repository.
daisy_home_repo_ok() {
    local home="$1"
    daisy_is_repo_root "$home" && return 0
    daisy_is_repo_root "$DAISY_ROOT" || return 1
    [ "$(daisy_repo_toplevel "$home")" = "$(daisy_repo_toplevel "$DAISY_ROOT")" ]
}

# Root-relative paths that are not the root's own content: every registered
# home inside it, and the top-level home/ that held the homes before the
# split while it still exists (it may hold directories nothing registers).
# A root commit leaves these out and the publication scan does not read
# them: one list, so the two cannot disagree.
daisy_root_exclusions() {
    local name rel
    while IFS=$'\t' read -r name rel; do
        echo "$rel"
    done < <(daisy_homes_under_root)
    [ -d "$DAISY_ROOT/home" ] && echo "home"
    return 0
}

# --- Identities ---
# An identity is a git author: the `user.name`, `user.email` and, through
# `core.sshCommand`, the SSH key a workspace repository commits and pushes
# with. It is a file of `key=value` lines — `name=`, `email=`, optional
# `sshkey=` — in a home: `<home>/gitconfig` is the home's default identity,
# named by the home's own name; `<home>/identities/<id>` is a further
# identity of that home, named `<id>`. A workspace records a non-default
# identity in `.daisy/identity` beside `.daisy/home`.
#
# The same three keys are set from a file and unset against one (the
# binary's internal/identity, for `init` and `clean`). `init` records what it applied in the
# workspace's `.daisy/gitconfig`, in the same format, and `clean` and a
# later `init` unset against that record: what was applied, not what the
# home's file says now, which may have changed or gone.

# Echoes the file holding a home's identity: gitconfig for the home's own
# name (or no name), identities/<id> otherwise. Returns 1 for a name that is
# not a name (daisy_name_ok): a path in `.daisy/identity` must not resolve
# to a file outside the home.
daisy_identity_file() {
    local home_dir="$1" home_name="$2" identity="$3"
    if [ -z "$identity" ] || [ "$identity" = "$home_name" ]; then
        echo "$home_dir/gitconfig"
    else
        daisy_name_ok "$identity" || return 1
        echo "$home_dir/identities/$identity"
    fi
}

# Reads an identity file. Sets IDENTITY_NAME, IDENTITY_EMAIL, IDENTITY_SSHKEY
# (empty when absent) and IDENTITY_SSH_COMMAND (the core.sshCommand value,
# empty when there is no key). Prints why and returns 1 when the file is
# missing or lacks name or email; both are required for a commit.
daisy_identity_read() {
    local file="$1"
    IDENTITY_NAME="" IDENTITY_EMAIL="" IDENTITY_SSHKEY="" IDENTITY_SSH_COMMAND=""
    if [ ! -f "$file" ]; then
        echo "Error: identity file $file does not exist" >&2
        return 1
    fi
    IDENTITY_NAME=$(grep '^name=' "$file" | head -1 | cut -d= -f2- | tr -d '\r')
    IDENTITY_EMAIL=$(grep '^email=' "$file" | head -1 | cut -d= -f2- | tr -d '\r')
    IDENTITY_SSHKEY=$(grep '^sshkey=' "$file" | head -1 | cut -d= -f2- | tr -d '\r')
    if [ -z "$IDENTITY_NAME" ] || [ -z "$IDENTITY_EMAIL" ]; then
        echo "Error: identity file $file is incomplete: it needs both 'name=' and 'email=' lines" >&2
        return 1
    fi
    # Verbatim, not quoted: git runs the command through the shell, which
    # expands a `~/.ssh/...` path so the same file serves every machine. So
    # the key must be one plain shell word: whitespace, a comment left on
    # the line, or a metacharacter would make it a different command.
    if [ -n "$IDENTITY_SSHKEY" ]; then
        local LC_ALL=C  # the same ASCII class daisy_identity_ssh_is_daisys recognises
        if ! [[ "$IDENTITY_SSHKEY" =~ ^[A-Za-z0-9._/~@+-]+$ ]]; then
            echo "Error: identity file $file: sshkey '$IDENTITY_SSHKEY' must be a plain path (letters, digits, '.', '_', '/', '~', '@', '+', '-')" >&2
            return 1
        fi
        IDENTITY_SSH_COMMAND="ssh -i $IDENTITY_SSHKEY -o IdentitiesOnly=yes"
    fi
    return 0
}

# Echoes the identity a workspace is bound to: the name in .daisy/identity
# when the file has one, else the home's own name. The one reader of that
# file, so an empty one means the default everywhere.
daisy_workspace_identity() {
    local daisy_dir="$1" home_name="$2" identity=""
    [ -s "$daisy_dir/identity" ] && identity=$(tr -d '\r' < "$daisy_dir/identity" | head -1 | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
    printf '%s\n' "${identity:-$home_name}"
}

# True when a core.sshCommand value is exactly one Daisy writes — `ssh -i`,
# one plain key path (the sshkey= rule), `-o IdentitiesOnly=yes` — so that,
# whatever record survived, it was applied by an init and is Daisy's to
# unset. A user's command with other options is the user's.
daisy_identity_ssh_is_daisys() {
    local LC_ALL=C
    [[ "$1" =~ ^ssh\ -i\ [A-Za-z0-9._/~@+-]+\ -o\ IdentitiesOnly=yes$ ]]
}

# True when a repository's local config holds a key and every value of it is
# the identity's: equal to the claim, or — for core.sshCommand, whatever the
# claim — in Daisy's own form (only an init writes one; the claim may have
# moved on since). The one judgement behind `unset` (what to take) and
# `status` (what to report), read NUL-separated so an empty value counts.
daisy_identity_key_is_ours() {
    local repo="$1" key="$2" claim="$3" v any=false
    while IFS= read -r -d '' v; do
        any=true
        if [ -n "$claim" ] && [ "$v" = "$claim" ]; then
            continue
        elif [ "$key" = core.sshCommand ] && daisy_identity_ssh_is_daisys "$v"; then
            continue
        fi
        return 1
    done < <(git -C "$repo" config --local --get-all -z "$key" 2>/dev/null)
    [ "$any" = true ]
}

# The three git keys an identity owns, and the value the identity last read
# (daisy_identity_read) gives each; empty when the identity does not set it.
DAISY_IDENTITY_KEYS=(user.name user.email core.sshCommand)
daisy_identity_value() {
    # printf, not echo: a value is never an option, '-n' included.
    case "$1" in
        user.name)       printf '%s\n' "$IDENTITY_NAME" ;;
        user.email)      printf '%s\n' "$IDENTITY_EMAIL" ;;
        core.sshCommand) printf '%s\n' "$IDENTITY_SSH_COMMAND" ;;
    esac
}

# What an interrupted `daisy init` swap can leave beside .daisy/: the previous
# .daisy/ moved aside (.daisy.previous.<pid>) and the staging directory
# (.daisy.stage.<6 chars>), directories only. One per line. Only these exact
# shapes are init's; a user's look-alike (.daisy.previous.bak, a symlink) is
# not listed and never touched.
daisy_init_leftovers() {
    local d name LC_ALL=C
    for d in "$1"/.daisy.previous.* "$1"/.daisy.stage.*; do
        [ -d "$d" ] && [ ! -L "$d" ] || continue
        name="${d##*/}"
        [[ "$name" =~ ^\.daisy\.previous\.[0-9]+$ || "$name" =~ ^\.daisy\.stage\.[A-Za-z0-9]{6}$ ]] || continue
        echo "$d"
    done
    return 0
}

# True when a directory carries a workspace binding: a .daisy/ directory
# that is not the Daisy root itself. The root is a directory named .daisy
# under the user's home, so a walk up from beside it must step past it,
# never read it as a binding, and never offer to clean it.
daisy_is_workspace_dir() {
    [ -d "$1/.daisy" ] || return 1
    [ "$(daisy_realdir "$1/.daisy")" != "$(daisy_realdir "$DAISY_ROOT")" ]
}

# True, having printed the error and the repair, when a directory is a
# workspace that cannot be read as one and must not be read as part of an
# enclosing workspace either: its .daisy is not a directory, or it has no
# .daisy but holds what an interrupted init swap leaves (its binding is in
# a leftover, not missing). The one judgement both walkers
# (find_workspace_root, resolve_home) make at every level.
daisy_workspace_problem() {
    local dir="$1" leftovers
    # The root, by whatever path, is no binding and no problem: the walk
    # steps past it (daisy_is_workspace_dir).
    [ -d "$dir/.daisy" ] && ! daisy_is_workspace_dir "$dir" && return 1
    if [ -L "$dir/.daisy" ]; then
        echo "Error: $dir/.daisy is a symlink; a binding is a directory of its own — move it aside, then run 'daisy init <identity>' there" >&2
        return 0
    fi
    if [ -e "$dir/.daisy" ] && [ ! -d "$dir/.daisy" ]; then
        echo "Error: $dir/.daisy exists and is not a directory; move it aside, then run 'daisy init <identity>' there" >&2
        return 0
    fi
    [ -e "$dir/.daisy" ] && return 1
    leftovers=$(daisy_init_leftovers "$dir")
    [ -n "$leftovers" ] || return 1
    echo "Error: an earlier 'daisy init' in $dir was interrupted mid-swap; what it left is $(echo "$leftovers" | paste -sd' ')" >&2
    echo "  Run 'daisy init <identity>' there to reclaim the binding" >&2
    return 0
}

# The six files of a workspace binding, `link<TAB>path-in-home` per line: the
# one list init writes and status checks.
daisy_workspace_links() {
    printf '%s\t%s\n' tasks tasks today.md journal/today.md journal.md journal/journal.md \
        projects projects plans plans feedback.md feedback/feedback.md
}

# Says, one line each, where a workspace has drifted from what init wrote:
# a repository config value that is no longer the record's (.daisy/gitconfig;
# init failed part way, or a hand edit of a key the record claims), a bound
# identity file whose values are not the record's (edited since init, or init
# did not finish writing the binding), or a binding link that is missing or
# points elsewhere. Nothing when all agree. For `status` and `files`: no
# order of writes in init can make every failure impossible, so the split is
# reported where it would otherwise hide.
daisy_workspace_drift() {
    local repo="$1" daisy_dir="$2" bound_file="$3" home_dir="$4"
    local record="$daisy_dir/gitconfig" against rec_name rec_email rec_ssh key value held link target
    # What the repository's config is checked against: the record; without
    # a readable one (a workspace bound before records, or a damaged file),
    # the bound identity file itself.
    if daisy_identity_read "$record" 2>/dev/null; then
        against="the record"
    elif [ -e "$record" ]; then
        echo "the identity record $record exists but cannot be read"
        against=""
    elif daisy_identity_read "$bound_file" 2>/dev/null; then
        against="the bound identity"
    else
        against=""
    fi
    if [ -n "$against" ]; then
        rec_name="$IDENTITY_NAME" rec_email="$IDENTITY_EMAIL" rec_ssh="$IDENTITY_SSH_COMMAND"
        # Config is compared only where init applies it: the workspace's own
        # repository. Elsewhere there is nothing applied to drift.
        if daisy_is_repo_root "$repo"; then for key in "${DAISY_IDENTITY_KEYS[@]}"; do
            value=$(daisy_identity_value "$key")
            held=$(git -C "$repo" config --local --get-all "$key" 2>/dev/null | sort -u | paste -sd,)
            if [ -n "$value" ]; then
                [ "$held" = "$value" ] || echo "git $key is '${held:-unset}' but $against says '$value'; the applied identity differs"
            elif [ "$key" = core.sshCommand ] && [ -n "$held" ]; then
                # A key the identity does not claim is the user's, except one in
                # Daisy's own form, which 'daisy clean' and the next init take.
                daisy_identity_key_is_ours "$repo" "$key" "" &&
                    echo "git core.sshCommand is '$held', in Daisy's form but not claimed by $against; 'daisy clean' or a re-init removes it"
            fi
        done; fi
        if [ "$against" = "the record" ]; then
            if daisy_identity_read "$bound_file" 2>/dev/null; then
                [ "$IDENTITY_NAME" = "$rec_name" ] && [ "$IDENTITY_EMAIL" = "$rec_email" ] && [ "$IDENTITY_SSH_COMMAND" = "$rec_ssh" ] ||
                    echo "the bound identity file $bound_file does not match what init recorded (edited since, or init did not finish)"
            else
                echo "the bound identity file $bound_file is missing or not a complete identity"
            fi
        fi
    elif [ ! -e "$record" ]; then
        echo "the bound identity file $bound_file is missing or not a complete identity; nothing claims the repository's git identity"
        if daisy_is_repo_root "$repo"; then for key in "${DAISY_IDENTITY_KEYS[@]}"; do
            held=$(git -C "$repo" config --local --get-all "$key" 2>/dev/null | paste -sd,)
            [ -z "$held" ] || echo "git $key is '$held', and nothing on record claims it"
        done; fi
    fi
    while IFS=$'\t' read -r link target; do
        [ "$(readlink -f "$daisy_dir/$link" 2>/dev/null)" = "$(readlink -f "$home_dir/$target")" ] ||
            echo "the binding link .daisy/$link is missing or does not point into the home"
    done < <(daisy_workspace_links)
    return 0
}

# Prints daisy_workspace_drift's lines as warnings on stderr naming the fix.
daisy_report_drift() {
    [ -n "$DAISY_WORKSPACE" ] || return 0  # the DAISY_DEFAULT_HOME fallback binds nothing to check
    daisy_workspace_drift "$DAISY_WORKSPACE" "$DAISY_WORKSPACE/.daisy" "$DAISY_IDENTITY_FILE" "$DAISY_HOME" |
        sed "s/^/⚠️  /; s/\$/; re-run 'daisy init $DAISY_IDENTITY_NAME'/" >&2
}

# A home or identity name is a registry key and a file name: letters,
# digits, '.', '_', '-', starting with a letter or digit. Refuses '=' (the
# registry delimiter), '/', whitespace and a leading dot, so a name can
# never be a path.
daisy_name_ok() {
    local LC_ALL=C  # ASCII letters and digits, whatever the locale
    [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]
}

# Resolve DAISY_HOME for the current workspace.
# 1. Walk up from $PWD looking for .daisy/home (authoritative)
# 2. Fall back to $DAISY_DEFAULT_HOME (convenience default, warns)
# 3. Error if neither exists
# Either way the name is looked up in the registry (daisy_home_dir).
#
# Sets: DAISY_HOME, DAISY_HOME_NAME, DAISY_WORKSPACE, DAISY_IDENTITY_NAME
# (from .daisy/identity, else the home's name), DAISY_IDENTITY_FILE
resolve_home() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        daisy_workspace_problem "$dir" && return 1
        if daisy_is_workspace_dir "$dir"; then
            # This directory is the workspace, complete or not: a .daisy/
            # without its home file is a binding to repair here, never a
            # level to walk past into an enclosing workspace's home.
            if [ ! -f "$dir/.daisy/home" ]; then
                echo "Error: $dir/.daisy has no home file; the binding is incomplete" >&2
                echo "  Run 'daisy init <identity>' there to rebind it, or 'daisy clean' to remove it" >&2
                return 1
            fi
            DAISY_HOME_NAME=$(cat "$dir/.daisy/home")
            DAISY_HOME=$(daisy_home_dir "$DAISY_HOME_NAME") || return 1
            DAISY_WORKSPACE="$dir"
            DAISY_IDENTITY_NAME=$(daisy_workspace_identity "$dir/.daisy" "$DAISY_HOME_NAME")
            DAISY_IDENTITY_FILE=$(daisy_identity_file "$DAISY_HOME" "$DAISY_HOME_NAME" "$DAISY_IDENTITY_NAME") || {
                echo "Error: $dir/.daisy/identity holds '$DAISY_IDENTITY_NAME', which is not an identity name" >&2
                echo "  Re-run 'daisy init <identity>' in the workspace to rebind it" >&2
                return 1
            }
            export DAISY_HOME DAISY_HOME_NAME DAISY_WORKSPACE DAISY_IDENTITY_NAME DAISY_IDENTITY_FILE
            return 0
        fi
        dir=$(dirname "$dir")
    done

    # Warn on the fallback so a silent wrong-home bug stays visible.
    if [ -n "$DAISY_DEFAULT_HOME" ]; then
        DAISY_HOME_NAME="$DAISY_DEFAULT_HOME"
        DAISY_HOME=$(daisy_home_dir "$DAISY_HOME_NAME") || return 1
        echo "⚠️  No .daisy/home found; falling back to \$DAISY_DEFAULT_HOME ($DAISY_HOME_NAME)" >&2
        DAISY_WORKSPACE=""  # no binding here, whatever the environment says
        DAISY_IDENTITY_NAME="$DAISY_HOME_NAME"
        DAISY_IDENTITY_FILE=$(daisy_identity_file "$DAISY_HOME" "$DAISY_HOME_NAME" "")
        export DAISY_HOME DAISY_HOME_NAME DAISY_WORKSPACE DAISY_IDENTITY_NAME DAISY_IDENTITY_FILE
        return 0
    fi

    echo "Error: Cannot resolve home. No .daisy/home found and DAISY_DEFAULT_HOME not set." >&2
    echo "  Run 'daisy init <identity>' in your workspace, or set DAISY_DEFAULT_HOME=<name> in ~/.zshenv" >&2
    return 1
}

# Resolve the projects this workspace is bound to. Call after require_env.
#
# A workspace binds to a *home* via .daisy/home; its project association was
# only ever prose in AGENTS.md and project files. The convention is already in
# the filesystem — workspaces are named {project}.{date} — so the directory
# name supplies the default and .daisy/project overrides it outright.
# Replacing rather than extending is what lets one rule cover both a rename
# (daisy-dev -> +daisy) and an addition (chessgrid.260807 -> +chessgrid
# +jobsearch): the file, when present, is the whole answer.
#
# Sets: DAISY_PROJECTS (array), DAISY_PROJECTS_SOURCE (explicit|convention|none)
resolve_projects() {
    DAISY_PROJECTS=()
    DAISY_PROJECTS_SOURCE="none"

    [ -n "$DAISY_WORKSPACE" ] || return 0

    local binding="$DAISY_WORKSPACE/.daisy/project"
    if [ -f "$binding" ]; then
        local name
        while IFS= read -r name || [ -n "$name" ]; do
            name="${name%%#*}"
            name=$(echo "$name" | xargs)
            [ -z "$name" ] && continue
            # A declared project that does not exist is a typo, not a
            # preference — say so, but never fail a read-only command over it.
            if [ ! -f "$DAISY_HOME/projects/$name.md" ]; then
                echo "⚠️  .daisy/project names '$name', but $DAISY_HOME/projects/$name.md does not exist" >&2
            fi
            DAISY_PROJECTS+=("$name")
        done < "$binding"
        [ ${#DAISY_PROJECTS[@]} -gt 0 ] && DAISY_PROJECTS_SOURCE="explicit"
        return 0
    fi

    # Convention: basename up to the first dot. A miss is silent — plenty of
    # workspaces legitimately have no project.
    local candidate
    candidate=$(basename "$DAISY_WORKSPACE")
    candidate="${candidate%%.*}"
    if [ -n "$candidate" ] && [ -f "$DAISY_HOME/projects/$candidate.md" ]; then
        DAISY_PROJECTS=("$candidate")
        DAISY_PROJECTS_SOURCE="convention"
    fi
    return 0
}

# One-line rendering of resolve_projects' result, for `daisy files`/`status`.
daisy_projects_summary() {
    if [ ${#DAISY_PROJECTS[@]} -eq 0 ]; then
        echo "(none)"
        return
    fi
    local out="" p
    for p in "${DAISY_PROJECTS[@]}"; do
        out+="+$p "
    done
    out="${out% }"
    case "$DAISY_PROJECTS_SOURCE" in
        explicit)   echo "$out (from .daisy/project)" ;;
        convention) echo "$out (from directory name)" ;;
        *)          echo "$out" ;;
    esac
}

# --- Skills ---
# Sources are what Daisy owns and edits; installed copies (what an agent
# loads, which `daisy list` and `daisy help <skill>` read) are the binary's
# concern. `daisy install`/`daisy init` copy the former to the latter.

# Skill sources, lowest precedence first (a home skill shadows a root one).
daisy_skill_source_dirs() {
    echo "$DAISY_ROOT/skills"
    [ -n "$DAISY_HOME" ] && echo "$DAISY_HOME/skills"
}

# Echoes the path to a skill's source SKILL.md, or nothing if there is none.
resolve_skill_source() {
    local name="$1" dir found=""
    while IFS= read -r dir; do
        [ -f "$dir/$name/SKILL.md" ] && found="$dir/$name/SKILL.md"
    done < <(daisy_skill_source_dirs)
    [ -n "$found" ] && echo "$found"
}

# --- Workflow tags ---
# A feedback entry's **Workflow:** header names the file its guidance belongs
# in. One definition serves both ends of the loop: `daisy feedback` asks "is
# this a real target?" before writing the tag, and `daisy optimize` asks
# "which file?" when reading it. They used to disagree — nothing validated
# the tag at all, so the resolver had to guess afterwards from a string
# nothing had constrained, and `daisy-write`, `daisy-scripts` and the literal
# `new-day, new-week` all reached the archive that way.
#
# `<layer>/<name>` says which layer, and is the form to use. A bare name is
# the legacy form: it tries both layers, then an alias table of workflow
# names that own no file of their own.

# Echoes the file a workflow tag names; nothing, and returns 1, if it names
# no file.
resolve_workflow_target() {
    local wf="$1" name skill

    case "$wf" in
        prompts/*)
            name="${wf#prompts/}"
            if [ -f "$DAISY_HOME/prompts/$name.md" ]; then
                echo "$DAISY_HOME/prompts/$name.md"; return 0
            fi
            if [ -f "$DAISY_ROOT/prompts/$name.md" ]; then
                echo "$DAISY_ROOT/prompts/$name.md"; return 0
            fi
            return 1
            ;;
        skills/*)
            skill=$(resolve_skill_source "${wf#skills/}")
            if [ -n "$skill" ]; then echo "$skill"; return 0; fi
            return 1
            ;;
    esac

    # Bare name: the layers in the order a prompt shadows a skill, then the
    # aliases. Checking the layers *first* is what stops an alias shadowing a
    # real file — `git` used to be swallowed by a `github|git)` arm and never
    # reached skills/git at all.
    if [ -f "$DAISY_HOME/prompts/$wf.md" ]; then
        echo "$DAISY_HOME/prompts/$wf.md"; return 0
    fi
    if [ -f "$DAISY_ROOT/prompts/$wf.md" ]; then
        echo "$DAISY_ROOT/prompts/$wf.md"; return 0
    fi
    skill=$(resolve_skill_source "$wf")
    if [ -n "$skill" ]; then echo "$skill"; return 0; fi

    # Workflows daisy.md owns because they have no prompt of their own.
    case "$wf" in
        new-day|new-week|done|log|add|priority|cancel|status|sync|project|projects|home)
            echo "$DAISY_ROOT/prompts/daisy.md"; return 0
            ;;
    esac
    return 1
}

# Every workflow tag that resolves, in qualified form, one per line. The
# discovery surface for `daisy feedback` — a hardcoded list drifts, and the
# one in feedback.sh's usage text had already drifted to offering `jira`
# (no prompt, no skill) while omitting every skill.
list_workflow_targets() {
    local f d name
    {
        for f in "$DAISY_ROOT"/prompts/*.md "$DAISY_HOME"/prompts/*.md; do
            [ -f "$f" ] || continue
            name=$(basename "$f" .md)
            echo "prompts/$name"
        done
        while IFS= read -r d; do
            [ -d "$d" ] || continue
            for f in "$d"/*/SKILL.md; do
                [ -f "$f" ] || continue
                echo "skills/$(basename "$(dirname "$f")")"
            done
        done < <(daisy_skill_source_dirs)
    } | sort -u
}

# A skill carrying LICENSE or NOTICE.md tracks an upstream source. Rewriting
# Daisy's copy from local feedback would fork it and turn every future sync
# into a reconciliation. Track them; never write them.
skill_is_vendored() {
    local dir="$1"
    [ -f "$dir/LICENSE" ] || [ -f "$dir/NOTICE.md" ]
}

# The `## Rules` section of a prompt or SKILL.md — the one span `daisy
# optimize` rewrites. Extraction and replacement have to agree on where it
# ends, so the boundary is defined once, here, rather than as a regex
# repeated at each site.
#
# A markdown section closes at the next heading of its own level or
# shallower: "# " or "## " end the rules, "### " is a subsection within
# them. Terminating on "## " alone (as both sites once did) swallowed any
# intervening "# " heading into the extract and deleted it on write.

# Echoes the ## Rules section of a file, heading included, or nothing when
# the file has no such section.
extract_rules_section() {
    awk '
        /^## Rules/ { found = 1; print; next }
        found && (/^# / || /^## /) { exit }
        found { print }
    ' "$1"
}

# Echoes the file with its ## Rules section replaced by new_rules (which
# must itself begin with the "## Rules" heading). Content outside the
# section is passed through untouched.
#
# The blank line before the next heading is re-emitted rather than carried
# through from new_rules: callers build new_rules via command substitution,
# which strips trailing newlines, so without this the rules weld onto the
# following heading and one blank line is lost on every run.
replace_rules_section() {
    local file="$1" new_rules="$2"
    awk -v new_rules="$new_rules" '
        /^## Rules/ { print new_rules; skip = 1; next }
        skip && (/^# / || /^## /) { print ""; skip = 0 }
        !skip { print }
    ' "$file"
}

# The daisy binary, for a script that needs a command the binary has taken
# over: this root's when it is built, else `daisy` on PATH (the suite's
# fixture root has no bin/; the binary on PATH is the one under test).
daisy_bin() {
    if [ -x "$DAISY_ROOT/bin/daisy" ]; then
        echo "$DAISY_ROOT/bin/daisy"
    else
        echo daisy
    fi
}

# Require DAISY_ROOT to be set. Call early in every script.
require_root() {
    if [ -z "$DAISY_ROOT" ]; then
        echo "Error: DAISY_ROOT not set" >&2
        echo "  Add to ~/.zshenv: export DAISY_ROOT=\"/path/to/daisy\"" >&2
        return 1
    fi
    if [ ! -d "$DAISY_ROOT" ]; then
        echo "Error: DAISY_ROOT directory does not exist: $DAISY_ROOT" >&2
        return 1
    fi
}

# Require both DAISY_ROOT and DAISY_HOME. Resolves home via .daisy/home or env var.
require_env() {
    require_root || return 1
    resolve_home || return 1
}
