#!/bin/bash
# Tests scripts/release-check.sh: make release-check-test
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failures=0
library() { mkdir -p "$WORK/src" && printf "<?php\nfinal class Debugger\n{\n    public const VERSION = '%s';\n}\n" "$1" > "$WORK/src/Debugger.php"; }
expect() {
  local name=$1 want=$2 tag=$3 got=0
  "$ROOT/scripts/release-check.sh" "$tag" "$WORK/src/Debugger.php" >/dev/null 2>&1 || got=$?
  if { [ "$want" = ok ] && [ "$got" -ne 0 ]; } || { [ "$want" = refused ] && [ "$got" -eq 0 ]; }; then
    echo "FAIL $name: want $want, exit $got"
    failures=$((failures + 1))
  fi
}

library 2.0.0
expect "a stable tag with a stable library" ok v1.0.1
expect "a pre-release tag" ok v1.1.0-rc.1
library 2.0.0-dev
expect "a stable tag with a dev library" refused v1.0.1
expect "a pre-release may carry a dev library" ok v1.1.0-rc.1
expect "not a version tag" refused main
library ''
expect "no version in the library" refused v1.0.1

[ "$failures" -eq 0 ] && echo "release-check: ok"
exit "$failures"
