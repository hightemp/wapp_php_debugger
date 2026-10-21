#!/bin/bash
# Refuses to release a stable tag (vX.Y.Z) with a pre-release library (Debugger::VERSION 2.0.0-dev):
# the library of a release writes its version into every log.
# Usage: release-check.sh TAG [path/to/Debugger.php]
set -euo pipefail

tag=${1:-}
source=${2:-client-php/src/Debugger.php}

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "release-check: $tag is not a version tag (vX.Y.Z)" >&2
  exit 1
fi
library=$(sed -n "s/.*public const VERSION = '\([^']*\)';.*/\1/p" "$source")
if [ -z "$library" ]; then
  echo "release-check: no Debugger::VERSION in $source" >&2
  exit 1
fi
if [[ "$tag" != *-* && "$library" == *-* ]]; then
  echo "release-check: $tag is stable but the library is $library; set Debugger::VERSION" >&2
  exit 1
fi
echo "release-check: $tag with library $library"
