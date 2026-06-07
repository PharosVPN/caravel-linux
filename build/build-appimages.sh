#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (C) 2026 The PharosVPN Authors
#
# Build PharosVPN Linux AppImages for BOTH amd64 and arm64 — PharosVPN ships
# both architectures, always. Uses Docker so the build is reproducible on any
# host (an Apple-Silicon Mac builds the arm64 image natively; an x86 host uses
# emulation for the foreign arch — install qemu-user-static + binfmt if needed).
#
#   ./build/build-appimages.sh            # both arches → dist/
#   ARCHES="arm64" ./build/build-appimages.sh   # just one
#
# Output: dist/PharosVPN-x86_64.AppImage and dist/PharosVPN-aarch64.AppImage.
#
# It builds the pushed branch from GitHub inside the container (CI-friendly).
# Point BRANCH/REPO elsewhere to build a fork or a different branch.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

REPO="${REPO:-https://github.com/PharosVPN/caravel-linux.git}"
BRANCH="${BRANCH:-feat/linux-client-scaffold}"
ARCHES="${ARCHES:-amd64 arm64}"

build_one() {
  local goarch="$1" gotar aitool aiarch platform
  case "$goarch" in
    amd64) platform=linux/amd64; gotar=go1.26.3.linux-amd64.tar.gz; aitool=appimagetool-x86_64.AppImage;  aiarch=x86_64 ;;
    arm64) platform=linux/arm64; gotar=go1.26.3.linux-arm64.tar.gz; aitool=appimagetool-aarch64.AppImage; aiarch=aarch64 ;;
    *) echo "unknown arch $goarch" >&2; return 1 ;;
  esac
  echo "==> $platform"
  docker run --rm --platform "$platform" -v "$OUT:/out" ubuntu:24.04 bash -euc "
    export DEBIAN_FRONTEND=noninteractive APPIMAGE_EXTRACT_AND_RUN=1
    apt-get update -qq
    apt-get install -y -qq curl git build-essential pkg-config libgtk-3-dev \
      libwebkit2gtk-4.1-dev libfuse2 file desktop-file-utils ca-certificates nodejs npm >/dev/null
    curl -sSL https://go.dev/dl/${gotar} | tar -C /usr/local -xz
    export PATH=\$PATH:/usr/local/go/bin:/root/go/bin:/root/bin
    mkdir -p /root/bin
    curl -sSL -o /root/bin/appimagetool \
      https://github.com/AppImage/appimagetool/releases/download/continuous/${aitool}
    chmod +x /root/bin/appimagetool
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    mkdir -p /work && cd /work
    git clone -q https://github.com/PharosVPN/caravel.git
    git clone -q -b '${BRANCH}' '${REPO}' caravel-linux
    cd caravel-linux
    # Tell appimage.sh which arch to build for. It otherwise defaults ARCH=x86_64,
    # which inside this native arm64 container would (cross-)build amd64 with the
    # aarch64 gcc and fail on x86 cgo assembly (gcc_amd64.S). Build native per arch.
    ARCH=${aiarch} WAILS_TAGS=webkit2_41 ./build/appimage.sh
    cp dist/*.AppImage /out/
  "
}

for a in $ARCHES; do build_one "$a"; done
echo "==> done:"
ls -lh "$OUT"/*.AppImage
