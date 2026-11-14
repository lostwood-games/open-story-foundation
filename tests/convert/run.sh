#!/usr/bin/env bash
# Integration test for `osf convert`.
#
# cases/to-<format>/<name>/ holds a project (input/) and the files
# `osf convert -o <format>` must produce from it (expected/). For each case
# this script converts input/ into a temporary folder and compares the result
# with expected/, file by file. It also checks that the output passes
# `osf validate`.
#
# Usage: bash tests/convert/run.sh [path to osf]     (default: ./osf)
#        UPDATE=1 bash tests/convert/run.sh          rewrite expected/ from the output
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
for case in "$cases"/to-*/*/; do
  case="${case%/}"
  format="$(basename "$(dirname "$case")")"
  format="${format#to-}"
  name="to-$format/$(basename "$case")"
  out="$tmp/$name"
  log="$tmp/log"
  if ! "$osf" convert -o "$format" -out "$out" "$case/input" >"$log" 2>&1; then
    echo "FAIL $name: osf convert failed"
    sed 's/^/     /' "$log"
    failed=1
    continue
  fi
  rm -f "$out/.osf-files" # the list of written files, not part of the output
  if [ "${UPDATE:-}" = 1 ]; then
    rm -rf "$case/expected"
    cp -R "$out" "$case/expected"
    echo "UPDATED $name"
    continue
  fi
  if ! diff -r "$case/expected" "$out" >"$log"; then
    echo "FAIL $name: output differs from expected/ (< expected, > converted)"
    sed 's/^/     /' "$log"
    failed=1
    continue
  fi
  if ! "$osf" validate "$out" >"$log" 2>&1; then
    echo "FAIL $name: the converted project does not pass osf validate"
    sed 's/^/     /' "$log"
    failed=1
    continue
  fi
  echo "ok   $name"
done
exit $failed
