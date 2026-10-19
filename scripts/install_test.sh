#!/bin/bash
# Tests scripts/install.sh against a fake GitHub Releases server: make install-test
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
SERVER_PID=""
cleanup() { [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null; rm -rf "$WORK"; }
trap cleanup EXIT

case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; esac
case "$(uname -m)" in x86_64 | amd64) arch=amd64 ;; aarch64 | arm64) arch=arm64 ;; esac

# Releases v9.9.8 and v9.9.9 (latest); the "binary" prints its version
for version in 9.9.8 9.9.9; do
  dir="$WORK/releases/download/v$version"
  mkdir -p "$dir" "$WORK/build"
  printf '#!/bin/sh\necho "wapp-debugger %s"\n' "$version" > "$WORK/build/wapp-debugger"
  chmod +x "$WORK/build/wapp-debugger"
  echo readme > "$WORK/build/README.md"
  archive="wapp-debugger_${version}_${os}_${arch}.tar.gz"
  tar -czf "$dir/$archive" -C "$WORK/build" wapp-debugger README.md
  (cd "$dir" && { sha256sum "$archive" 2>/dev/null || shasum -a 256 "$archive"; } > checksums.txt)
done

port=$((20000 + RANDOM % 20000))
cat > "$WORK/server.py" <<'PY'
import http.server, os, sys
root, latest = sys.argv[2], os.environ.get("LATEST", "v9.9.9")
class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=root, **kwargs)
    def redirect(self):
        if self.path.rstrip("/").endswith("/releases/latest"):
            if latest == "none":
                self.send_error(404)
            else:
                self.send_response(302)
                self.send_header("Location", "/releases/tag/" + latest)
                self.end_headers()
            return True
        if "/releases/tag/" in self.path:
            self.send_response(200)
            self.end_headers()
            return True
        return False
    def do_HEAD(self):
        if not self.redirect():
            super().do_HEAD()
    def do_GET(self):
        if not self.redirect():
            super().do_GET()
    def log_message(self, *args):
        pass
http.server.ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PY

start_server() { # start_server <latest tag or "none">
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  LATEST="$1" python3 "$WORK/server.py" "$port" "$WORK" &
  SERVER_PID=$!
  for _ in $(seq 50); do curl --noproxy "*" -fsS -o /dev/null "http://127.0.0.1:$port/releases/download/v9.9.9/checksums.txt" 2>/dev/null && return; sleep 0.1; done
  echo "fake server did not start" >&2; exit 1
}

run_install() { # run_install <install dir> [env...]
  local dir=$1; shift
  env NO_PROXY='*' no_proxy='*' WAPP_DEBUGGER_RELEASES="http://127.0.0.1:$port/releases" WAPP_DEBUGGER_INSTALL_DIR="$dir" "$@" \
    sh "$ROOT/scripts/install.sh"
}

failures=0
check() { # check <name> <condition...>
  local name=$1; shift
  if "$@"; then echo "ok   $name"; else echo "FAIL $name"; failures=$((failures + 1)); fi
}

start_server v9.9.9
run_install "$WORK/bin-latest" > "$WORK/out-latest" 2>&1 || true
check "installs the latest release" test "$("$WORK/bin-latest/wapp-debugger" 2>/dev/null)" = "wapp-debugger 9.9.9"
check "says where it went" grep -q "v9.9.9 installed to $WORK/bin-latest/wapp-debugger" "$WORK/out-latest"

run_install "$WORK/bin-pinned" WAPP_DEBUGGER_VERSION=v9.9.8 > /dev/null 2>&1 || true
check "installs a pinned version" test "$("$WORK/bin-pinned/wapp-debugger" 2>/dev/null)" = "wapp-debugger 9.9.8"

echo corrupted >> "$WORK/releases/download/v9.9.9/wapp-debugger_9.9.9_${os}_${arch}.tar.gz"
if run_install "$WORK/bin-corrupted" > "$WORK/out-corrupted" 2>&1; then status=0; else status=$?; fi
check "refuses a corrupted archive" test "$status" -ne 0
check "explains the checksum" grep -q "checksum mismatch" "$WORK/out-corrupted"
check "installs nothing then" test ! -e "$WORK/bin-corrupted/wapp-debugger"

start_server none
if run_install "$WORK/bin-none" > "$WORK/out-none" 2>&1; then status=0; else status=$?; fi
check "fails without a published release" test "$status" -ne 0
check "explains it" grep -q "no published release" "$WORK/out-none"

[ "$failures" -eq 0 ] || { echo "$failures check(s) failed"; exit 1; }
echo "install.sh: all checks passed"
