#!/bin/sh
# Installs the wapp-debugger server from GitHub Releases:
#   curl -fsSL https://github.com/hightemp/wapp_php_debugger/releases/latest/download/install.sh | sh
#
# Picks the archive for this system, checks it against checksums.txt and puts the binary into
# /usr/local/bin when it is writable, ~/.local/bin otherwise. Settings (environment):
#   WAPP_DEBUGGER_VERSION      a tag such as v0.2.0 instead of the latest release
#   WAPP_DEBUGGER_INSTALL_DIR  where to put the binary
set -eu

releases="${WAPP_DEBUGGER_RELEASES:-https://github.com/hightemp/wapp_php_debugger/releases}"
version="${WAPP_DEBUGGER_VERSION:-}"
install_dir="${WAPP_DEBUGGER_INSTALL_DIR:-}"

fail() {
	echo "install.sh: $*" >&2
	exit 1
}

for tool in curl tar; do
	command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) fail "no build for $(uname -s) here; download one from $releases" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "no build for $(uname -m); download one from $releases" ;;
esac

if [ -z "$version" ]; then
	# releases/latest redirects to releases/tag/<latest tag>
	latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$releases/latest" 2>/dev/null) || latest=""
	version=${latest##*/}
	case "$version" in
	v[0-9]*) ;;
	*) fail "no published release found at $releases" ;;
	esac
fi

archive="wapp-debugger_${version#v}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

curl -fsSL -o "$tmp/$archive" "$releases/download/$version/$archive" || fail "cannot download $archive of $version"
curl -fsSL -o "$tmp/checksums.txt" "$releases/download/$version/checksums.txt" || fail "cannot download checksums.txt of $version"

expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
else
	actual=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $archive: the download is damaged or was tampered with"

tar -xzf "$tmp/$archive" -C "$tmp" wapp-debugger || fail "$archive has no wapp-debugger binary"

if [ -z "$install_dir" ]; then
	if [ -w /usr/local/bin ]; then install_dir=/usr/local/bin; else install_dir="$HOME/.local/bin"; fi
fi
mkdir -p "$install_dir"
cp "$tmp/wapp-debugger" "$install_dir/wapp-debugger.new"
chmod 755 "$install_dir/wapp-debugger.new"
mv "$install_dir/wapp-debugger.new" "$install_dir/wapp-debugger"

echo "wapp-debugger $version installed to $install_dir/wapp-debugger"
case ":$PATH:" in
*":$install_dir:"*) ;;
*) echo "add $install_dir to PATH to run it as wapp-debugger" ;;
esac
