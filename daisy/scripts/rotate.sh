#!/usr/bin/env bash
# Invocation: run as `daisy rotate` — do not execute this file directly.
# Archive journal.md day-blocks into rolling/closed window files, logrotate-style.
#
# journal.md always retains only its last 5 day-blocks; everything older is
# distributed into last-14 / month-to-date / year-to-date rolling files,
# which close out into permanent journal-YYYYMM.md / journal-YYYY.md files
# at each calendar boundary. There is no state marker: every day-block
# beyond the retained last 5 is the work queue, so a first-run backfill and
# an ordinary single-entry day both walk the same code path.
#
# Usage: rotate.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

RETAIN=5
LAST14_MAX=14

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    if [ ! -f "$DAISY_HOME/journal/journal.md" ]; then
        echo "Error: journal.md not found" >&2
        exit 1
    fi
    exit 0
fi

require_env || exit 1

JOURNAL="$DAISY_HOME/journal/journal.md"
JDIR="$DAISY_HOME/journal"

[ -s "$JOURNAL" ] || exit 0   # missing or empty: nothing to rotate

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# --- split a day-block file into individual blocks, one per file ---
# A block runs from one "### YYYY-MM-DD" heading up to (not including) the
# next one, or EOF. This is deliberately NOT based on "---" lines: content
# such as new-week.sh's "#### Weekly Retrospective" preamble (written just
# before that day's heading, separated by its own "---") has no date of its
# own and belongs with the day it precedes in the source — splitting on the
# heading itself, rather than on "---", attaches it there automatically with
# no special-casing, and survives being re-split later (e.g. during last-14
# pruning) the same way every time. Writes block_1, block_2, ... into $2.
split_blocks() {
    local src="$1" outdir="$2"
    mkdir -p "$outdir"
    awk -v out="$outdir/block_" '
        BEGIN { n = 0 }
        /^### [0-9]{4}-[0-9]{2}-[0-9]{2}/ {
            if (n > 0) close(out n)
            n++
        }
        n > 0 { print > (out n) }
    ' "$src"

    # Anything before the first heading is content with nowhere safe to
    # attach it — refuse rather than silently discard it.
    local leading
    leading=$(awk '/^### [0-9]{4}-[0-9]{2}-[0-9]{2}/ { exit } { print }' "$src" | grep -c '[^[:space:]]' || true)
    if [ "${leading:-0}" -gt 0 ]; then
        echo "Error: rotate.sh: journal.md has non-blank content before its first '### YYYY-MM-DD' heading — aborting, nothing written" >&2
        exit 1
    fi

    # Trim a trailing separator ("---" plus surrounding blank lines) that
    # each block (except the last) carries from the boundary before the
    # next heading — append_block_raw adds its own separator when rejoining
    # blocks, so this avoids doubling it up.
    local f
    for f in "$outdir"/block_*; do
        [ -f "$f" ] || continue
        sed -i -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}' "$f"
        if [ "$(tail -1 "$f")" = "---" ]; then
            sed -i '$d' "$f"
            sed -i -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}' "$f"
        fi
    done
}

# --- extract the YYYY-MM-DD heading date of a block file ---
# Every block produced by split_blocks starts with its date heading, so
# this only needs to check line 1. Empty output means the file wasn't
# produced by split_blocks (defensive; callers should not see this).
block_date() {
    local f="$1"
    head -1 "$f" | grep -oE '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' || true
}

# --- append raw block content to a window file (creates it if absent) ---
append_block_raw() {
    local file="$1" blockfile="$2"
    if [ -s "$file" ]; then
        printf '\n---\n\n' >> "$file"
    fi
    cat "$blockfile" >> "$file"
}

# --- regenerate TOC + Summary front matter on a window file ---
# Extracts everything from the first "### YYYY-MM-DD" line onward (this
# discards any existing front matter, since front matter never contains a
# "### " line) and rebuilds the header fresh — safe to call repeatedly.
regenerate_front_matter() {
    local file="$1" label="$2"
    local blocks="$WORK/fm_blocks_$$"
    awk '/^### [0-9]{4}-[0-9]{2}-[0-9]{2}/ { found=1 } found' "$file" > "$blocks"

    local dates count first last
    dates=$(grep -E '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' "$blocks" | sed -E 's/^### ([0-9]{4}-[0-9]{2}-[0-9]{2}).*/\1/')
    count=$(printf '%s\n' "$dates" | grep -c .)
    first=$(printf '%s\n' "$dates" | head -1)
    last=$(printf '%s\n' "$dates" | tail -1)

    {
        echo "# $label"
        echo
        echo "## Table of Contents"
        printf '%s\n' "$dates" | sed 's/^/- /'
        echo
        echo "## Summary"
        echo
            echo "$count entries, $first to $last."
        echo
        echo "---"
        echo
        cat "$blocks"
    } > "$file.new"
    mv "$file.new" "$file"
    rm -f "$blocks"
}

# --- drop the oldest block from a window file, return its new first date ---
drop_oldest_block() {
    local file="$1"
    local blocks="$WORK/fm_blocks_drop"
    awk '/^### [0-9]{4}-[0-9]{2}-[0-9]{2}/ { found=1 } found' "$file" > "$blocks"

    local splitdir="$WORK/drop_split"
    rm -rf "$splitdir"
    split_blocks "$blocks" "$splitdir"

    : > "$file"
    local first=true fnum newest_first=""
    for fnum in $(ls "$splitdir" | sed 's/block_//' | sort -n); do
        if [ "$first" = true ]; then
            first=false
            continue   # drop the oldest
        fi
        append_block_raw "$file" "$splitdir/block_$fnum"
    done
    rm -f "$blocks"
}

# ============================================================
# 1. Split journal.md into day-blocks; validate structure.
# ============================================================

split_blocks "$JOURNAL" "$WORK/journal_blocks"

BLOCK_FILES=()
for f in $(ls "$WORK/journal_blocks" 2>/dev/null | sed 's/block_//' | sort -n); do
    BLOCK_FILES+=("$WORK/journal_blocks/block_$f")
done

TOTAL=${#BLOCK_FILES[@]}

DATES=()
POSITION=0
for f in "${BLOCK_FILES[@]}"; do
    POSITION=$((POSITION + 1))
    d=$(block_date "$f")
    if [ -z "$d" ]; then
        FIRST_LINE=$(head -1 "$f")
        echo "Error: rotate.sh: block $POSITION of $TOTAL in journal.md does not start with '### YYYY-MM-DD' — found: \"$FIRST_LINE\". Fix journal.md's day-block heading at that position, then re-run. Aborting, nothing written." >&2
        exit 1
    fi
    DATES+=("$d")
done

if [ "$TOTAL" -le "$RETAIN" ]; then
    exit 0   # nothing beyond the retained last 5 — no-op
fi

OVERFLOW=$((TOTAL - RETAIN))

# ============================================================
# 2. Process each overflowing block, oldest to newest.
# ============================================================

for ((i = 0; i < OVERFLOW; i++)); do
    BLOCKFILE="${BLOCK_FILES[$i]}"
    DATE="${DATES[$i]}"
    YYYYMM="${DATE:0:7}"          # YYYY-MM
    YYYYMM="${YYYYMM/-/}"          # YYYYMM
    YYYY="${DATE:0:4}"
    COMPACT="${DATE//-/}"          # YYYYMMDD

    # --- last-14 ---
    LAST14_FILE=$(ls "$JDIR"/journal-*-last14.md 2>/dev/null | head -1 || true)
    if [ -z "$LAST14_FILE" ]; then
        LAST14_FILE="$JDIR/journal-${COMPACT}-last14.md"
        : > "$LAST14_FILE"
    fi
    append_block_raw "$LAST14_FILE" "$BLOCKFILE"
    regenerate_front_matter "$LAST14_FILE" "Journal Archive: Last 14 Entries"

    COUNT14=$(grep -cE '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' "$LAST14_FILE")
    if [ "$COUNT14" -gt "$LAST14_MAX" ]; then
        drop_oldest_block "$LAST14_FILE"
        regenerate_front_matter "$LAST14_FILE" "Journal Archive: Last 14 Entries"
        NEW_FIRST=$(grep -m1 -E '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' "$LAST14_FILE" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}')
        NEW_COMPACT="${NEW_FIRST//-/}"
        NEW_LAST14_NAME="$JDIR/journal-${NEW_COMPACT}-last14.md"
        if [ "$LAST14_FILE" != "$NEW_LAST14_NAME" ]; then
            mv "$LAST14_FILE" "$NEW_LAST14_NAME"
        fi
    fi

    # --- month-to-date ---
    MTD_FILE=$(ls "$JDIR"/journal-*-mtd.md 2>/dev/null | head -1 || true)
    MTD_MONTH=""
    if [ -n "$MTD_FILE" ]; then
        MTD_FIRST=$(grep -m1 -E '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' "$MTD_FILE" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}')
        MTD_MONTH="${MTD_FIRST:0:7}"
    fi
    if [ -n "$MTD_FILE" ] && [ "$MTD_MONTH" = "${DATE:0:7}" ]; then
        append_block_raw "$MTD_FILE" "$BLOCKFILE"
        regenerate_front_matter "$MTD_FILE" "Journal Archive: $MTD_MONTH"
    else
        if [ -n "$MTD_FILE" ]; then
            regenerate_front_matter "$MTD_FILE" "Journal Archive: $MTD_MONTH"
            mv "$MTD_FILE" "$JDIR/journal-${MTD_MONTH/-/}.md"
        fi
        NEW_MTD="$JDIR/journal-${COMPACT}-mtd.md"
        : > "$NEW_MTD"
        append_block_raw "$NEW_MTD" "$BLOCKFILE"
        regenerate_front_matter "$NEW_MTD" "Journal Archive: ${DATE:0:7}"
    fi

    # --- year-to-date ---
    YTD_FILE=$(ls "$JDIR"/journal-*-ytd.md 2>/dev/null | head -1 || true)
    YTD_YEAR=""
    if [ -n "$YTD_FILE" ]; then
        YTD_FIRST=$(grep -m1 -E '^### [0-9]{4}-[0-9]{2}-[0-9]{2}' "$YTD_FILE" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}')
        YTD_YEAR="${YTD_FIRST:0:4}"
    fi
    if [ -n "$YTD_FILE" ] && [ "$YTD_YEAR" = "$YYYY" ]; then
        append_block_raw "$YTD_FILE" "$BLOCKFILE"
        regenerate_front_matter "$YTD_FILE" "Journal Archive: $YTD_YEAR"
    else
        if [ -n "$YTD_FILE" ]; then
            regenerate_front_matter "$YTD_FILE" "Journal Archive: $YTD_YEAR"
            mv "$YTD_FILE" "$JDIR/journal-${YTD_YEAR}.md"
        fi
        NEW_YTD="$JDIR/journal-${COMPACT}-ytd.md"
        : > "$NEW_YTD"
        append_block_raw "$NEW_YTD" "$BLOCKFILE"
        regenerate_front_matter "$NEW_YTD" "Journal Archive: $YYYY"
    fi
done

# ============================================================
# 3. Rebuild journal.md from the retained last RETAIN blocks.
# ============================================================

{
    for ((i = OVERFLOW; i < TOTAL; i++)); do
        if [ "$i" -gt "$OVERFLOW" ]; then
            printf '\n---\n\n'
        fi
        cat "${BLOCK_FILES[$i]}"
    done
} > "$JOURNAL.new"
mv "$JOURNAL.new" "$JOURNAL"

echo "✅ Rotated $OVERFLOW day-block(s); journal.md retains its last $RETAIN"

exit 0
