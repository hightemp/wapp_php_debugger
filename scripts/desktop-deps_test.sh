#!/bin/bash
# Tests scripts/desktop-deps.sh against fake pkg-config and compilers: make desktop-deps-test
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# pkg-config that knows only the modules in $FAKE_PC_MODULES
cat > "$WORK/pkg-config" <<'EOF'
#!/bin/bash
[ "$1" = --exists ] && shift
for module in "$@"; do
  [[ " $FAKE_PC_MODULES " == *" $module "* ]] || exit 1
done
EOF
printf '#!/bin/sh\n' > "$WORK/cc"
chmod +x "$WORK/pkg-config" "$WORK/cc"

GTK4="gtk4 webkitgtk-6.0 libsoup-3.0 gio-unix-2.0 glib-2.0 gobject-2.0 x11"
GTK3="gtk+-3.0 webkit2gtk-4.1 gdk-3.0 libsoup-3.0 gio-unix-2.0 glib-2.0 gobject-2.0 x11"

failures=0
indent() { while IFS= read -r line; do echo "    $line"; done <<<"$1"; }
release() { printf 'ID=%s\nID_LIKE="%s"\nVERSION_ID="%s"\nPRETTY_NAME="%s"\n' "$1" "$2" "$3" "$4" > "$WORK/os-release"; }
# expect NAME WANT(ok|missing) OS GTK MODULES [TEXT…]: the exit code and every TEXT in the output
expect() {
  local name=$1 want=$2 os=$3 gtk=$4 modules=$5 got=0 out text
  shift 5
  out=$(DESKTOP_DEPS_OS=$os DESKTOP_DEPS_OS_RELEASE="$WORK/os-release" FAKE_PC_MODULES=$modules \
    PKG_CONFIG=${FAKE_PKG_CONFIG:-$WORK/pkg-config} CC=${FAKE_CC:-$WORK/cc} \
    "$ROOT/scripts/desktop-deps.sh" "$gtk" 2>&1) || got=$?
  if { [ "$want" = ok ] && [ "$got" -ne 0 ]; } || { [ "$want" = missing ] && [ "$got" -eq 0 ]; }; then
    echo "FAIL $name: want $want, exit $got"
    indent "$out"
    failures=$((failures + 1))
    return
  fi
  for text in "$@"; do
    if ! grep -qF -- "$text" <<<"$out"; then
      echo "FAIL $name: no \"$text\" in"
      indent "$out"
      failures=$((failures + 1))
    fi
  done
}

release ubuntu debian 24.04 "Ubuntu 24.04.5 LTS"
expect "everything installed" ok Linux gtk4 "$GTK4"
expect "GTK3 installed" ok Linux gtk3 "$GTK3"
expect "Ubuntu without GTK4 and WebKitGTK" missing Linux gtk4 "glib-2.0 gobject-2.0 gio-unix-2.0 libsoup-3.0 x11" \
  "gtk4 webkitgtk-6.0" "Ubuntu 24.04.5 LTS" "sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev"
expect "Ubuntu with GTK3" missing Linux gtk3 "glib-2.0 gobject-2.0 gio-unix-2.0 x11" \
  "sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev libsoup-3.0-dev"
FAKE_PKG_CONFIG="$WORK/no-pkg-config" expect "no pkg-config" missing Linux gtk4 "" \
  "pkg-config" "sudo apt-get install -y pkg-config"
FAKE_CC="$WORK/no-cc" expect "no C compiler" missing Linux gtk4 "$GTK4" "build-essential"

release ubuntu debian 22.04 "Ubuntu 22.04.4 LTS"
expect "Ubuntu 22.04 has no WebKitGTK 6.0" missing Linux gtk4 "" "LINUX_GTK=gtk3"

release debian "" 13 "Debian GNU/Linux 13 (trixie)"
expect "Debian" missing Linux gtk4 "" "sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev"

release debian "" "" "Debian GNU/Linux forky/sid"
expect "Debian testing has no version" missing Linux gtk4 "" "sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev"

release fedora "" 42 "Fedora Linux 42"
expect "Fedora" missing Linux gtk4 "" "sudo dnf install 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'"

release linuxmint "ubuntu debian" 22 "Linux Mint 22"
expect "a derivative of Ubuntu" missing Linux gtk4 "" "sudo apt-get install -y libgtk-4-dev"

release arch "" "" "Arch Linux"
expect "Arch" missing Linux gtk4 "" "sudo pacman -S --needed gtk4 webkitgtk-6.0"

release opensuse-tumbleweed "opensuse suse" 20260101 "openSUSE Tumbleweed"
expect "openSUSE" missing Linux gtk4 "" "sudo zypper install 'pkgconfig(gtk4)'"

release alpine "" 3.22 "Alpine Linux v3.22"
expect "Alpine" missing Linux gtk4 "" "sudo apk add gtk4.0-dev webkit2gtk-6.0-dev"

release gentoo "" 2.17 "Gentoo Linux"
expect "an unknown distribution" missing Linux gtk4 "" "pkg-config modules" "gtk4 webkitgtk-6.0"

expect "macOS with a compiler" ok Darwin gtk4 ""
FAKE_CC="$WORK/no-cc" expect "macOS without a compiler" missing Darwin gtk4 "" "xcode-select --install"
expect "Windows with a compiler" ok MINGW64_NT-10.0-26100 gtk4 ""
FAKE_CC="$WORK/no-cc" expect "Windows without a compiler" missing MINGW64_NT-10.0-26100 gtk4 "" "MSYS2"

[ "$failures" -eq 0 ] && echo "desktop-deps: ok"
exit "$failures"
