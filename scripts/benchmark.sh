#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$ROOT_DIR/benchmarks"

mkdir -p "$OUT_DIR"

OUTFILE="$OUT_DIR/$(date +%Y%m%d_%H%M%S).txt"

echo "Running Go benchmarks..."
echo "Output: $OUTFILE"
echo ""

cd "$ROOT_DIR/solver_go"
go test -bench . -benchmem -count="${COUNT:-3}" -run='^$' -timeout 600s 2>&1 | tee "$OUTFILE"

echo ""
echo "Saved to: $OUTFILE"
echo ""
echo "To compare two runs:"
echo "  go install golang.org/x/perf/cmd/benchstat@latest"
echo "  benchstat old.txt new.txt"
