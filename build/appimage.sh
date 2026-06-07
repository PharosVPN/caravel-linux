#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (C) 2026 The PharosVPN Authors
#
# Build the PharosVPN AppImage. Linux-only (needs the GTK/WebKit2GTK dev libs,
# Wails, Node, and appimagetool). See BUILD.md for prerequisites.
#
#   ./build/appimage.sh            # build dist/PharosVPN-x86_64.AppImage
#
# The AppImage bundles:
#   - the Wails GUI binary  (caravel-linux → PharosVPN)
#   - the privileged helper (pharos-helper) the GUI installs via pkexec
#   - the .desktop, icon, and the polkit policy
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ARCH="${ARCH:-x86_64}"
GOARCH="amd64"
[ "$ARCH" = "aarch64" ] && GOARCH="arm64"

DIST="$ROOT/dist"
APPDIR="$DIST/PharosVPN.AppDir"
rm -rf "$APPDIR"
mkdir -p "$DIST" "$APPDIR/usr/bin" "$APPDIR/usr/local/lib/pharosvpn" \
	"$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/512x512/apps" \
	"$APPDIR/usr/share/polkit-1/actions"

echo "==> Building the Wails GUI (caravel-linux)…"
# `wails build` builds the frontend (npm) + the Go app and regenerates the JS
# bindings. -tags webkit2_41 if your distro ships WebKit2GTK 4.1 (see BUILD.md).
if command -v wails >/dev/null 2>&1; then
	wails build -platform "linux/$GOARCH" -o caravel-linux ${WAILS_TAGS:+-tags "$WAILS_TAGS"}
	cp build/bin/caravel-linux "$APPDIR/usr/bin/PharosVPN"
else
	echo "!! wails not found — falling back to a manual frontend+go build" >&2
	( cd frontend && npm install && npm run build )
	CGO_ENABLED=1 GOARCH="$GOARCH" go build -tags "${WAILS_TAGS:-desktop,production}" -o "$APPDIR/usr/bin/PharosVPN" .
fi

echo "==> Building the privileged helper (pharos-helper)…"
CGO_ENABLED=0 GOARCH="$GOARCH" go build -trimpath -ldflags "-s -w" \
	-o "$APPDIR/usr/local/lib/pharosvpn/pharos-helper" ./cmd/pharos-helper
# Also drop the helper next to the GUI so the AppImage's bundled-helper lookup
# (App.bundledHelperPath) finds it at runtime before install.
cp "$APPDIR/usr/local/lib/pharosvpn/pharos-helper" "$APPDIR/usr/bin/pharos-helper"

echo "==> Staging desktop integration…"
cp build/linux/pharosvpn.desktop "$APPDIR/usr/share/applications/pharosvpn.desktop"
cp build/linux/pharosvpn.desktop "$APPDIR/pharosvpn.desktop"
cp build/linux/org.pharosvpn.caravel.policy "$APPDIR/usr/share/polkit-1/actions/"
cp build/appicon.png "$APPDIR/usr/share/icons/hicolor/512x512/apps/pharosvpn.png"
cp build/appicon.png "$APPDIR/pharosvpn.png"
ln -sf pharosvpn.png "$APPDIR/.DirIcon"

cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
export PATH="$HERE/usr/bin:$PATH"
# Point the GUI's bundled-helper lookup at the AppImage copy.
export PHAROS_HELPER_BIN="$HERE/usr/bin/pharos-helper"
exec "$HERE/usr/bin/PharosVPN" "$@"
EOF
chmod +x "$APPDIR/AppRun"

echo "==> Packaging the AppImage…"
OUT="$DIST/PharosVPN-$ARCH.AppImage"
if command -v appimagetool >/dev/null 2>&1; then
	ARCH="$ARCH" appimagetool "$APPDIR" "$OUT"
else
	echo "!! appimagetool not found — download it (BUILD.md) and re-run." >&2
	echo "   The staged AppDir is ready at: $APPDIR" >&2
	exit 1
fi

echo "==> Done: $OUT"
