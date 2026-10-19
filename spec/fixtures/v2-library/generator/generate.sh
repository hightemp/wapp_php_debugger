#!/bin/bash
# Writes spec/fixtures/v2-library with the real PHP library 2.x on PHP 7.4 (no Composer: src/autoload.php).
# Run from the repository root: make fixtures-v2-library
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
OUT="$ROOT/spec/fixtures/v2-library"
GEN="$OUT/generator"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cp -r "$GEN/app" "$WORK/app"
mkdir -p "$WORK/app/tmp/logs"
rm -f "$OUT"/*.jsonl "$OUT/manifest.json"

docker run --rm --init --hostname fixture -u "$(id -u):$(id -g)" -e TZ=UTC \
  -v "$WORK/app:/var/www/app" -v "$ROOT/client-php/src:/var/www/lib:ro" -v "$GEN:/generator:ro" -v "$OUT:/out" \
  -w /var/www/app php:7.4-cli bash -c '
    php -d display_errors=0 -S 127.0.0.1:8080 /var/www/app/public/index.php >/dev/null 2>&1 & sleep 1
    php /generator/driver.php /var/www/app/tmp/logs /out'
echo "fixtures written to $OUT"
