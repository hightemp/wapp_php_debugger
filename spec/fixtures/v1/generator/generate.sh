#!/bin/bash
# Generates v1 format fixtures with library v1 (client-php/v1/Debugger.class.php) on PHP 7.4.
# Run from the repository root: make fixtures-v1
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
OUT="$ROOT/spec/fixtures/v1"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
IMAGE="php:7.4-cli"

mkdir -p "$WORK/app/logs" "$WORK/app/tmp"
cp "$ROOT/client-php/v1/Debugger.class.php" "$WORK/app/"
cat > "$WORK/app/dws_debugger.json" <<JSON
{"sProjectRootPath":"/var/www/app","sTempPath":"/var/www/app/tmp","sMessagesFilesPath":"/var/www/app/tmp/logs","bIsOnFlag":true}
JSON

# Library before the hotfix (commit ecf8962): a long header overwrites the first messages
git -C "$ROOT" show ecf8962:Debugger.class.php > "$WORK/app/Debugger.pre-hotfix.php"
sed -i "s|^define('DA_CONFIG_FILE_NAME'|define('DA_FLAG', true); define('DA_ROOT_DIR', __DIR__);\ndefine('DA_CONFIG_FILE_NAME'|" "$WORK/app/Debugger.pre-hotfix.php"
sed -i "s|define('DA_CONFIG_FILE_PATH', __DIR__.\"/\".DA_CONFIG_FILE_NAME);|define('DA_CONFIG_FILE_PATH', __DIR__.'/dws_debugger.json');|" "$WORK/app/Debugger.pre-hotfix.php"

IN_DOCKER=(docker run --rm --init --hostname fixture -u "$(id -u):$(id -g)" -e TZ=UTC
  -v "$WORK/app:/var/www/app" -v "$OUT:/out" -v "$OUT/generator:/generator:ro" -w /var/www/app)

# Web requests through the built-in server: SAPI cli-server, like a real web request
"${IN_DOCKER[@]}" "$IMAGE" bash -c '
  cp /generator/scenario.php /var/www/app/
  php -S 127.0.0.1:8080 -t /var/www/app >/dev/null 2>&1 & sleep 1
  php /generator/web-driver.php /var/www/app/tmp/logs /out'

cli() { # cli <name> [docker-args...] -- [script-args...]
  local name=$1; shift
  local docker_args=() script_args=()
  while [ $# -gt 0 ] && [ "$1" != "--" ]; do docker_args+=("$1"); shift; done
  [ "${1:-}" = "--" ] && shift
  script_args=("$@")
  "${IN_DOCKER[@]}" "${docker_args[@]}" "$IMAGE" bash -c 'cp /generator/scenario.php . && php scenario.php "$@"' _ "$name" "${script_args[@]}" >/dev/null 2>&1 || true
  local files=("$WORK"/app/tmp/logs/*)
  [ ${#files[@]} -eq 1 ] || { echo "$name: expected 1 log file, got ${#files[@]}" >&2; exit 1; }
  mv "${files[0]}" "$OUT/$name.log"
}

cli cli -- --queue=emails --verbose
cli killed
# 100 arguments of ~25 bytes each: the legacy library header grows far beyond 1000 bytes
cli pre-hotfix-long-header -e FIXTURE_LIB=/var/www/app/Debugger.pre-hotfix.php -- $(printf -- '--argument-number=%03d ' $(seq 1 100))

# The write is cut off in the middle of the last line
cp "$OUT/web-get.log" "$OUT/truncated-last-line.log"
size=$(stat -c %s "$OUT/truncated-last-line.log")
truncate -s $((size - 40)) "$OUT/truncated-last-line.log"

docker run --rm -u "$(id -u):$(id -g)" -v "$OUT:/fixtures" -v "$OUT/generator:/generator:ro" "$IMAGE" php /generator/manifest.php /fixtures > "$OUT/manifest.json"
echo "fixtures written to $OUT"
