#!/bin/bash
# RED evidence for STOMPY-2722 (CLI door of the context lifecycle).
#
# Copies cmd/context_lifecycle_test.go onto a pristine `git archive origin/main`
# tree and runs it. The tests drive the real command tree, so on main they
# COMPILE and FAIL BY NAME (unknown command), rather than failing to build.
# Requires `go test` to exit EXACTLY 1 and every named test below to be red.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${RED_BASE:-origin/main}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

git -C "$ROOT" archive "$BASE" | tar -x -C "$WORK"
cp "$ROOT/cmd/context_lifecycle_test.go" "$WORK/cmd/"
cd "$WORK" || exit 2
go test ./cmd -count=1 -run 'TestContextLifecycle' -v >"$WORK/out.txt" 2>&1
code=$?
if [ "$code" -ne 1 ]; then
  echo "NOT RED: go test exited $code (want exactly 1)"; tail -20 "$WORK/out.txt"; exit 1
fi
if grep -q "build failed" "$WORK/out.txt"; then
  echo "RED by build failure, not by behaviour — the tests must compile on $BASE"; exit 1
fi
want=(
  TestContextLifecycle_SendsTheRESTContract
  TestContextLifecycle_JSONIsTheAPIsObject
  TestContextLifecycle_HumanOutput
  TestContextLifecycle_RefusalsPassThroughVerbatim
  TestContextLifecycle_ClientSideUsageErrorsNeverReachTheServer
  TestContextLifecycle_IsRegisteredUnderContext
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
