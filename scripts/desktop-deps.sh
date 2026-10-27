#!/bin/bash
# Checks what the desktop build needs from the system (a C compiler for cgo; GTK and WebKitGTK on
# Linux) and, when something is missing, prints how to install it on this system.
# Usage: desktop-deps.sh [gtk4|gtk3]. Exit 0: all there, 1: something is missing.
# Tests set DESKTOP_DEPS_OS (uname -s) and DESKTOP_DEPS_OS_RELEASE (/etc/os-release);
# PKG_CONFIG and CC are the ones cgo uses.
set -euo pipefail

gtk=${1:-gtk4}
os=${DESKTOP_DEPS_OS:-$(uname -s)}
os_release=${DESKTOP_DEPS_OS_RELEASE:-/etc/os-release}
pkg_config=${PKG_CONFIG:-pkg-config}
compiler=${CC:-$(go env CC 2>/dev/null || echo cc)}
compiler=${compiler%% *}

have() { command -v "$1" >/dev/null 2>&1; }

if ! have "$compiler"; then
  case $os in
  Darwin)
    echo "desktop-deps: no C compiler ($compiler) for cgo; install the Xcode command line tools:"
    echo "  xcode-select --install"
    exit 1
    ;;
  MINGW* | MSYS* | CYGWIN* | Windows_NT)
    echo "desktop-deps: no C compiler ($compiler) for cgo; install MinGW-w64 GCC, for example from MSYS2"
    echo "(https://www.msys2.org/), and put its bin directory on PATH:"
    echo "  pacman -S mingw-w64-ucrt-x86_64-gcc"
    exit 1
    ;;
  esac
fi
case $os in Linux) ;; *) exit 0 ;; esac

# The pkg-config modules of Wails v3 (#cgo lines of github.com/wailsapp/wails/v3)
if [ "$gtk" = gtk3 ]; then
  modules="gtk+-3.0 webkit2gtk-4.1 gdk-3.0 libsoup-3.0 gio-unix-2.0 glib-2.0 gobject-2.0 x11"
else
  modules="gtk4 webkitgtk-6.0 libsoup-3.0 gio-unix-2.0 glib-2.0 gobject-2.0 x11"
fi

tools=()
have "$compiler" || tools+=(compiler)
missing=()
if have "$pkg_config"; then
  for module in $modules; do
    "$pkg_config" --exists "$module" || missing+=("$module")
  done
else
  # Without pkg-config nothing can be checked: the libraries go to the install command as well
  tools+=(pkg-config)
  read -ra missing <<<"$modules"
fi
[ ${#tools[@]} -eq 0 ] && [ ${#missing[@]} -eq 0 ] && exit 0

id='' id_like='' version_id='' pretty_name=''
if [ -r "$os_release" ]; then
  # shellcheck disable=SC1090
  {
    read -r id
    read -r id_like
    read -r version_id
    read -r pretty_name
  } < <(. "$os_release" && printf '%s\n' "${ID:-}" "${ID_LIKE:-}" "${VERSION_ID:-}" "${PRETTY_NAME:-}")
fi
family=''
for candidate in $id $id_like; do
  case $candidate in
  debian | ubuntu) family=debian ;;
  fedora | rhel | centos) family=fedora ;;
  arch) family=arch ;;
  opensuse* | suse | sles) family=suse ;;
  alpine) family=alpine ;;
  esac
  [ -n "$family" ] && break
done

# The package that provides a pkg-config module, a compiler or pkg-config itself
package() {
  case $family:$1 in
  debian:compiler) echo build-essential ;;
  debian:pkg-config) echo pkg-config ;;
  debian:gtk4) echo libgtk-4-dev ;;
  debian:webkitgtk-6.0) echo libwebkitgtk-6.0-dev ;;
  debian:gtk+-3.0 | debian:gdk-3.0) echo libgtk-3-dev ;;
  debian:webkit2gtk-4.1) echo libwebkit2gtk-4.1-dev ;;
  debian:libsoup-3.0) echo libsoup-3.0-dev ;;
  debian:gio-unix-2.0 | debian:glib-2.0 | debian:gobject-2.0) echo libglib2.0-dev ;;
  debian:x11) echo libx11-dev ;;
  arch:compiler) echo base-devel ;;
  arch:pkg-config) echo pkgconf ;;
  arch:gtk4) echo gtk4 ;;
  arch:webkitgtk-6.0) echo webkitgtk-6.0 ;;
  arch:gtk+-3.0 | arch:gdk-3.0) echo gtk3 ;;
  arch:webkit2gtk-4.1) echo webkit2gtk-4.1 ;;
  arch:libsoup-3.0) echo libsoup3 ;;
  arch:gio-unix-2.0 | arch:glib-2.0 | arch:gobject-2.0) echo glib2 ;;
  arch:x11) echo libx11 ;;
  alpine:compiler) echo build-base ;;
  alpine:pkg-config) echo pkgconf ;;
  alpine:gtk4) echo gtk4.0-dev ;;
  alpine:webkitgtk-6.0) echo webkit2gtk-6.0-dev ;;
  alpine:gtk+-3.0 | alpine:gdk-3.0) echo gtk+3.0-dev ;;
  alpine:webkit2gtk-4.1) echo webkit2gtk-4.1-dev ;;
  alpine:libsoup-3.0) echo libsoup3-dev ;;
  alpine:gio-unix-2.0 | alpine:glib-2.0 | alpine:gobject-2.0) echo glib-dev ;;
  alpine:x11) echo libx11-dev ;;
  # dnf and zypper install a package by the pkg-config module it provides
  fedora:compiler | suse:compiler) echo gcc ;;
  fedora:pkg-config) echo pkgconf-pkg-config ;;
  suse:pkg-config) echo pkg-config ;;
  *) echo "'pkgconfig($1)'" ;;
  esac
}
packages() {
  local item name list=()
  for item in "$@"; do
    name=$(package "$item")
    [[ " ${list[*]} " == *" $name "* ]] || list+=("$name")
  done
  echo "${list[*]}"
}
install_command() {
  case $family in
  debian) echo "sudo apt-get install -y $(packages "$@")" ;;
  fedora) echo "sudo dnf install $(packages "$@")" ;;
  arch) echo "sudo pacman -S --needed $(packages "$@")" ;;
  suse) echo "sudo zypper install $(packages "$@")" ;;
  alpine) echo "sudo apk add $(packages "$@")" ;;
  esac
}

echo "desktop-deps: the desktop build ($gtk) needs what this system does not have:"
[ ${#tools[@]} -gt 0 ] && echo "  tools: ${tools[*]/compiler/a C compiler ($compiler)}"
[ ${#missing[@]} -gt 0 ] && echo "  pkg-config modules: ${missing[*]}"
echo

old_release=false
if [ "$gtk" = gtk4 ] && [ -n "$version_id" ]; then
  case $id in
  ubuntu) [ "$(printf '%s\n' "$version_id" 24.04 | sort -V | head -1)" != 24.04 ] && old_release=true ;;
  debian) [ "$(printf '%s\n' "$version_id" 13 | sort -V | head -1)" != 13 ] && old_release=true ;;
  esac
fi

if [ "$old_release" = true ]; then
  echo "${pretty_name:-This release} has no WebKitGTK 6.0 (it comes with Ubuntu 24.04 and Debian 13)."
  echo "Build with GTK3 and WebKit2GTK 4.1 instead:"
  echo "  $(install_command "${tools[@]}" gtk+-3.0 webkit2gtk-4.1 libsoup-3.0 glib-2.0 x11)"
  echo "  make desktop-build LINUX_GTK=gtk3"
elif [ -n "$family" ]; then
  echo "Install them (${pretty_name:-$id}):"
  echo "  $(install_command "${tools[@]}" "${missing[@]}")"
else
  echo "Install the development packages that provide these pkg-config modules"
  echo "(${pretty_name:-an unknown distribution}); their names differ between distributions."
fi
exit 1
