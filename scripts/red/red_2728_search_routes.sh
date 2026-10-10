#!/bin/bash
# RED evidence for STOMPY-2728 (CLI search routes + the command->route oracle).
#
# Copies the new tests onto a pristine `git archive origin/main` tree and runs
# them. They drive the real command tree, so on main they COMPILE and FAIL BY
# NAME. Requires `go test` to exit EXACTLY 1 and every named test to be red.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${RED_BASE:-origin/main}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

git -C "$ROOT" archive "$BASE" | tar -x -C "$WORK"
cp "$ROOT/cmd/search_routes_test.go" "$ROOT/cmd/contract_routes_test.go" "$WORK/cmd/"
mkdir -p "$WORK/internal/contract"
cp "$ROOT/internal/contract/command_routes.json" "$WORK/internal/contract/"
# The oracle's loader is production code (go:embed); on main it does not exist,
# so ship the loader too or the test would fail by build, not by behaviour.
cp "$ROOT/internal/contract/contract.go" "$WORK/internal/contract/"
cd "$WORK" || exit 2
go test ./cmd -count=1 -run 'TestSearchRoutes|TestCommandRoutesOracle' -v >"$WORK/out.txt" 2>&1
code=$?
if [ "$code" -ne 1 ]; then
  echo "NOT RED: go test exited $code (want exactly 1)"; tail -20 "$WORK/out.txt"; exit 1
fi
if grep -q "build failed\|\[build failed\]" "$WORK/out.txt"; then
  echo "RED by build failure, not by behaviour — the tests must compile on $BASE"; exit 1
fi
want=(
  TestSearchRoutes_RequestPathAndParams
  TestSearchRoutes_RendersWhatMCPReturnsPerHit
  TestSearchRoutes_JSONIsTheAPIObject
  TestSearchRoutes_EmptyIsExplicitAndNamesTheMode
  "TestCommandRoutesOracle/context_search"
)
status=0
for name in "${want[@]}"; do
  if grep -q -- "--- FAIL: $name" "$WORK/out.txt"; then
    echo "RED  $name"
  else
    echo "NOT RED  $name"; status=1
  fi
done
echo "go test exit code: $code (RED on $BASE)"
exit $status
