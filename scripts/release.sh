#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Cut a PharosVPN Linux release: build both-arch AppImages (amd64 + arm64) and
# publish a GitHub release tagged vX.Y.Z with both attached.
#
#   scripts/release.sh                 # release the current VERSION
#   BRANCH=main scripts/release.sh     # build from a specific pushed branch
#
# Requires Docker (the dual-arch build) and gh. The build clones from GitHub, so
# commit + push first. Bump the version beforehand with scripts/bump-version.sh.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="$(tr -d '[:space:]' < VERSION)"
TAG="v$VERSION"

if gh release view "$TAG" >/dev/null 2>&1; then
  echo "release $TAG already exists — bump the version first (scripts/bump-version.sh)" >&2
  exit 1
fi

echo "==> Building both-arch AppImages for ${TAG}…"
BRANCH="${BRANCH:-main}" ARCHES="amd64 arm64" ./build/build-appimages.sh

amd64="dist/PharosVPN-x86_64.AppImage"
arm64="dist/PharosVPN-aarch64.AppImage"
[ -f "$amd64" ] && [ -f "$arm64" ] || { echo "!! missing AppImage(s) in dist/ — aborting release" >&2; exit 1; }

echo "==> Publishing GitHub release ${TAG}…"
gh release create "$TAG" "$amd64" "$arm64" \
  --title "PharosVPN Linux $TAG" \
  --notes "First public build (pre-alpha). PharosVPN desktop client for Linux (Wails GUI). Dual-arch AppImages: x86_64 + aarch64. Dual-protocol AmneziaWG + XRay-REALITY with cloud profile sync and the signature live map. Bundles caravel core v$VERSION. To run: \`chmod +x PharosVPN-*.AppImage\` and launch it; the privileged tunnel helper installs on first connect via pkexec."
echo "==> Released $TAG"
