#!/usr/bin/env bash
# Integration test for `osf build`.
#
# Every folder in cases/ holds an .osf project (project/) and the JSON it must
# build to (expected/). For each case this script runs `osf build -o json`
# into a temporary folder and compares the result with expected/, file by
# file. It also checks that the output passes `osf validate`.
#
# Usage: bash tests/build/run.sh [path to osf]     (default: ./osf)
#        UPDATE=1 bash tests/build/run.sh          rewrite expected/ from the output
set -u

osf="${1:-./osf}"
if [ ! -x "$osf" ]; then
  echo "osf binary not found at $osf; build it with:" >&2
  echo "  (cd src/cli && go build -o ../../osf ./cmd/osf)" >&2
  exit 2
fi
osf="$(cd "$(dirname "$osf")" && pwd)/$(basename "$osf")"
cases="$(cd "$(dirname "$0")" && pwd)/cases"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failed=0
for case in "$cases"/*/; do
  name="$(basename "$case")"
  out="$tmp/$name"
  if ! "$osf" build -o json -out "$out" "$case/project" >"$tmp/$name.log" 2>&1; then
    echo "FAIL $name: osf build failed"
    sed 's/^/     /' "$tmp/$name.log"
    failed=1
    continue
  fi
  rm -f "$out/.osf-build" # the list of written files, not part of the output
  if [ "${UPDATE:-}" = 1 ]; then
    rm -rf "$case/expected"
    cp -R "$out" "$case/expected"
    echo "UPDATED $name"
    continue
  fi
  if ! diff -r "$case/expected" "$out" >"$tmp/$name.diff"; then
    echo "FAIL $name: output differs from expected/ (< expected, > built)"
    sed 's/^/     /' "$tmp/$name.diff"
    failed=1
    continue
  fi
  if ! "$osf" validate "$out" >"$tmp/$name.log" 2>&1; then
    echo "FAIL $name: the built JSON does not pass osf validate"
    sed 's/^/     /' "$tmp/$name.log"
    failed=1
    continue
  fi
  echo "ok   $name"
done
exit $failed
