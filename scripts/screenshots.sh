#!/usr/bin/env bash
set -euo pipefail

OUTDIR="docs/images"
CONFIG="docs/freeze.json"
BIN=$(mktemp)

trap 'rm -f "$BIN"' EXIT

if ! command -v freeze &>/dev/null; then
    echo "error: freeze is not installed"
    echo "  go install github.com/charmbracelet/freeze@latest"
    echo "  or: brew install freeze"
    exit 1
fi

echo "Building vibeshots..."
go build -o "$BIN" ./cmd/vibeshots

mkdir -p "$OUTDIR"

scenes=$("$BIN" --list)

for scene in $scenes; do
    echo "→ ${scene}"
    "$BIN" "$scene" | freeze \
        --language ansi \
        --config "$CONFIG" \
        --output "${OUTDIR}/${scene}.svg"
done

echo ""
echo "Done: ${OUTDIR}/"
ls -lh "${OUTDIR}"/*.svg
