#!/bin/bash
# RED evidence for STOMPY-2724 (CLI context recall/list show lifecycle state
# and review date; -o json is the API object).
#
# Copies the new test onto a pristine `git archive origin/main` tree and runs
# it. It drives the real command tree, so on main it COMPILES and FAILS BY
# NAME. Requires `go test` to exit EXACTLY 1 and every named test red.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${RED_BASE:-origin/main}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

git -C "$ROOT" archive "$BASE" | tar -x -C "$WORK"
cp "$ROOT/cmd/context_read_lifecycle_test.go" "$WORK/cmd/"
cd "$WORK" || exit 2
go test ./cmd -count=1 -run 'TestContextRecall_|TestContextList_' -v >"$WORK/out.txt" 2>&1
code=$?
if [ "$code" -ne 1 ]; then
  echo "NOT RED: go test exited $code (want exactly 1)"; tail -20 "$WORK/out.txt"; exit 1
fi
if grep -q "build failed\|\[build failed\]" "$WORK/out.txt"; then
  echo "RED by build failure, not by behaviour — the tests must compile on $BASE"; exit 1
fi
want=(
  "TestContextRecall_ShowsLifecycleState/parked_names_the_review_date"
  "TestContextRecall_ShowsLifecycleState/archived"
  "TestContextRecall_ShowsLifecycleState/delivered"
  TestContextRecall_JSONIsTheAPIObject
  TestContextList_ShowsLifecycleColumn
  TestContextList_JSONIsTheAPIObject
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
