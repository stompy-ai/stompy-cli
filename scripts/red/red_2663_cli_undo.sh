#!/bin/bash
# RED evidence for the STOMPY-2663 follow-up (the CLI prints its own undo).
#
# Copies cmd/conflict_undo_test.go onto a pristine `git archive origin/main`
# tree and runs it. It drives the real command tree, so on main it COMPILES and
# FAILS BY NAME. Requires `go test` to exit EXACTLY 1 and each subtest named
# below to be red (the two no-undo controls stay green).
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${RED_BASE:-origin/main}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

git -C "$ROOT" archive "$BASE" | tar -x -C "$WORK"
cp "$ROOT/cmd/conflict_undo_test.go" "$WORK/cmd/"
cd "$WORK" || exit 2
go test ./cmd -count=1 -run 'TestConflictResolve_' -v >"$WORK/out.txt" 2>&1
code=$?
if [ "$code" -ne 1 ]; then
  echo "NOT RED: go test exited $code (want exactly 1)"; tail -20 "$WORK/out.txt"; exit 1
fi
if grep -q "build failed" "$WORK/out.txt"; then
  echo "RED by build failure, not by behaviour"; exit 1
fi
status=0
for name in "keep_a,_server_sent_the_MCP_form" "keep_b,_server_sent_the_REST_form" "undo_carries_the_project_it_ran_in"; do
  if grep -q -- "--- FAIL: TestConflictResolve_PrintsTheCLIUndoNotTheServers/$name" "$WORK/out.txt"; then
    echo "RED  $name"
  else
    echo "NOT RED  $name"; status=1
  fi
done
exit $status
