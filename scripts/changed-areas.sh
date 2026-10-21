#!/bin/bash
# Which heavy CI jobs a change needs: prints php=true|false and desktop=true|false (for $GITHUB_OUTPUT).
# Usage: changed-areas.sh BASE HEAD. Without a known base (a new branch, a tag, a manual run)
# everything runs.
set -euo pipefail

base=${1:-}
head=${2:-HEAD}

if [[ -z "$base" || "$base" =~ ^0+$ ]] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "php=true"
  echo "desktop=true"
  exit 0
fi

files=$(git diff --name-only "$base" "$head")
# The make targets and the workflows themselves concern every job
shared='^(Makefile|\.github/workflows/.*)$'
php="^(client-php/|spec/)"
desktop="^(server/|desktop/|ui/|spec/|scripts/desktop)"

match() { grep -Eq -e "$1" -e "$shared" <<<"$files" && echo true || echo false; }
echo "php=$(match "$php")"
echo "desktop=$(match "$desktop")"
