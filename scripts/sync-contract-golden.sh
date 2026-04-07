#!/usr/bin/env bash
set -euo pipefail

# Sync canonical D-Mail contract golden files from phonewave to sibling tools.
# phonewave is the D-Mail courier and protocol owner — its golden files are
# the single source of truth for the cross-tool D-Mail contract.
#
# Usage: ./scripts/sync-contract-golden.sh
# Requires: rsync, sibling tool directories at the same level as phonewave.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PARENT_DIR="$(cd "$REPO_ROOT/.." && pwd)"

CANONICAL="$REPO_ROOT/tests/contract/testdata/golden"

if [ ! -d "$CANONICAL" ]; then
  echo "ERROR: canonical golden dir not found: $CANONICAL" >&2
  exit 1
fi

declare -A TARGETS=(
  [amadeus]="tests/contract/testdata/golden"
  [sightjack]="tests/contract/testdata/golden"
  [paintress]="tests/contract/testdata/golden"
)

synced=0
for tool in "${!TARGETS[@]}"; do
  dest="$PARENT_DIR/$tool/${TARGETS[$tool]}"
  if [ ! -d "$PARENT_DIR/$tool" ]; then
    echo "SKIP: $tool not found at $PARENT_DIR/$tool" >&2
    continue
  fi
  mkdir -p "$dest"
  rsync -a --delete "$CANONICAL/" "$dest/"
  echo "OK: $tool ($(ls "$dest" | wc -l | tr -d ' ') files)"
  synced=$((synced + 1))
done

echo "Synced $synced tools from $CANONICAL"
