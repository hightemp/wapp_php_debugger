#!/bin/sh
# Builds the desktop app and its installers for the current OS into dist/release-desktop (unsigned,
# plan 10.28): Linux — tar.gz, .deb, AppImage; Windows — zip and an NSIS setup.exe; macOS — a
# universal .app in a .dmg. Every installer registers wapp-debugger:// links.
# Usage: scripts/desktop-package.sh <version> <wails3 binary>   (make desktop-package)
set -eu

version=$1
wails3=$2
root=$(cd "$(dirname "$0")/.." && pwd)
dist="$root/dist"
out="$dist/release-desktop"
goos=$(go env GOOS)
goarch=$(go env GOARCH)
plain=${version#v} # 0.4.0, as packages want it
name="wapp-debugger-desktop_${plain}_${goos}"

rm -rf "$out"
mkdir -p "$out"
cd "$root"

case "$goos" in
linux)
	make desktop-build
	tar -czf "$out/${name}_${goarch}.tar.gz" -C "$dist" wapp-debugger-desktop -C "$root" README.md
	mkdir -p "$dist/deb"
	GOARCH=$goarch VERSION=$plain "$wails3" tool package -name wapp-debugger-desktop -format deb \
		-config desktop/packaging/linux/nfpm.yaml -out "$dist/deb"
	mv "$dist"/deb/*.deb "$out/${name}_${goarch}.deb"
	rm -rf "$dist/deb"
	# linuxdeploy wants the icon named as in the .desktop file
	rm -rf "$dist/appimage" && mkdir -p "$dist/appimage/icon" "$dist/appimage/out"
	cp desktop/icons/icon-256.png "$dist/appimage/icon/wapp-debugger.png"
	"$wails3" generate appimage -binary "$dist/wapp-debugger-desktop" -icon "$dist/appimage/icon/wapp-debugger.png" \
		-desktopfile desktop/packaging/linux/wapp-debugger.desktop -outputdir "$dist/appimage/out" -builddir "$dist/appimage/build"
	mv "$dist"/appimage/out/*.AppImage "$out/${name}_${goarch}.AppImage"
	rm -rf "$dist/appimage"
	;;
windows)
	# The icon and the manifest go into the .exe as a resource before it is built
	syso="server/cmd/wapp-debugger-desktop/rsrc_windows_${goarch}.syso"
	"$wails3" generate syso -arch "$goarch" -icon desktop/icons/icon.ico \
		-manifest desktop/packaging/windows/wapp-debugger.exe.manifest -out "$syso"
	make desktop-build
	rm -f "$syso"
	powershell -NoProfile -Command "Compress-Archive -Path '$dist/wapp-debugger-desktop.exe','README.md' -DestinationPath '$out/${name}_${goarch}.zip'"
	nsis_version=$(echo "$plain" | sed 's/[^0-9.].*//')
	makensis -V2 -DVERSION="${nsis_version:-0.0.0}" -DBINARY="$(cygpath -w "$dist/wapp-debugger-desktop.exe")" \
		-DICON="$(cygpath -w "$root/desktop/icons/icon.ico")" -DOUTFILE="$(cygpath -w "$out/${name}_${goarch}_setup.exe")" \
		desktop/packaging/windows/installer.nsi
	;;
darwin)
	# One app for Apple silicon and Intel
	GOARCH=arm64 CGO_ENABLED=1 make desktop-build DESKTOP_BIN="$dist/wapp-debugger-desktop-arm64"
	GOARCH=amd64 CGO_ENABLED=1 CC="clang -arch x86_64" make desktop-build DESKTOP_BIN="$dist/wapp-debugger-desktop-amd64"
	app="$dist/dmg/wapp-debugger.app"
	rm -rf "$dist/dmg" && mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	lipo -create -output "$app/Contents/MacOS/wapp-debugger-desktop" \
		"$dist/wapp-debugger-desktop-arm64" "$dist/wapp-debugger-desktop-amd64"
	sed "s/@VERSION@/$plain/g" desktop/packaging/darwin/Info.plist >"$app/Contents/Info.plist"
	cp desktop/icons/icon.icns "$app/Contents/Resources/icon.icns"
	# Not signed with a Developer ID (plan 10.28), but Apple silicon runs only signed code: ad-hoc
	codesign --force --deep --sign - "$app"
	tar -czf "$out/${name}_universal.tar.gz" -C "$dist/dmg" wapp-debugger.app -C "$root" README.md
	ln -s /Applications "$dist/dmg/Applications"
	hdiutil create -volname wapp-debugger -srcfolder "$dist/dmg" -ov -format UDZO "$out/${name}_universal.dmg"
	rm -rf "$dist/dmg" "$dist/wapp-debugger-desktop-arm64" "$dist/wapp-debugger-desktop-amd64"
	;;
*)
	echo "no desktop packages for $goos" >&2
	exit 1
	;;
esac
ls -l "$out"
