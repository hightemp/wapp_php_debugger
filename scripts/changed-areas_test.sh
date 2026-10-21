#!/bin/bash
# Tests scripts/changed-areas.sh on a throwaway repository: make changed-areas-test
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"
git init -q
git config user.email test@example.com
git config user.name test
commit() { mkdir -p "$(dirname "$1")" && echo "$RANDOM" >> "$1" && git add -A && git commit -qm "$1" && git rev-parse HEAD; }

failures=0
expect() {
  local name=$1 want=$2 got
  shift 2
  got=$("$ROOT/scripts/changed-areas.sh" "$@" | sort | tr '\n' ' ')
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: want '$want', got '$got'"
    failures=$((failures + 1))
  fi
}

base=$(commit README.md)
ui=$(commit ui/src/App.svelte)
expect "the UI only" "desktop=true php=false " "$base" "$ui"
php=$(commit client-php/src/Debugger.php)
expect "the library only" "desktop=false php=true " "$ui" "$php"
docs=$(commit docs/notes.md)
expect "docs only" "desktop=false php=false " "$php" "$docs"
spec=$(commit spec/openapi.yaml)
expect "the contract: both" "desktop=true php=true " "$docs" "$spec"
ci=$(commit .github/workflows/ci.yml)
expect "the workflows: everything" "desktop=true php=true " "$spec" "$ci"
expect "a new branch: everything" "desktop=true php=true " "0000000000000000000000000000000000000000" "$ci"
expect "no base: everything" "desktop=true php=true " "" "$ci"
expect "an unknown base: everything" "desktop=true php=true " "1234567890123456789012345678901234567890" "$ci"

[ "$failures" -eq 0 ] && echo "changed-areas: ok"
exit "$failures"
