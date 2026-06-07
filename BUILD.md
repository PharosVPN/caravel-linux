<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->

# Building caravel-linux

The PharosVPN Linux client is a [Wails v2](https://wails.io) app: a Go backend
(consuming the shared core `caravel/go`) and a Svelte/Vite frontend, shipped as
an **AppImage**. A separate small Go binary — **`pharos-helper`** — does the
privileged tunnel work (TUN + routes) under a systemd service the GUI installs
once via `pkexec`. This mirrors the macOS client's GUI-talks-to-a-root-daemon
model.

```
caravel-linux/
  main.go, app.go            the Wails GUI backend (≈ TunnelController.swift)
  cmd/pharos-helper/         the privileged helper + store/sync CLI (≈ caravel-mac)
  internal/worker/           tunnel (TUN+routes), daemon, systemd install, store, sync
  internal/uimodel/          offline profile→display + map (pins/arcs) model
  internal/keystore/         account passphrase in the Secret Service (libsecret)
  frontend/                  Svelte + Vite UI (reuses the coxswain design tokens)
  build/                     icon, .desktop, polkit policy, appimage.sh
```

## What compiles where

| Piece | Builds on macOS (cross)? | Needs Linux? |
|---|---|---|
| `pharos-helper` | **yes** (`GOOS=linux go build ./cmd/pharos-helper`) — static, CGO-free | runs on Linux (TUN) |
| The Wails GUI Go code | **yes** to type-check (`GOOS=linux go build .`) | the **TUN + GTK runtime** are Linux-only |
| The frontend | needs **Node** (any OS) | — |
| The AppImage | — | **yes** (GTK/WebKit2GTK + appimagetool) |

The helper and the whole Go GUI already cross-compile cleanly for Linux from a
Mac (verified). The **frontend build, the GUI runtime, and AppImage packaging
need a Linux box** — use the ProxMox Ubuntu VM.

## Prerequisites (the Linux build box — Ubuntu/Debian)

```sh
# Go 1.26+
#   https://go.dev/dl/

# GTK + WebKit2GTK (the Wails webview) and TUN tooling
sudo apt update
sudo apt install -y build-essential pkg-config \
  libgtk-3-dev libwebkit2gtk-4.1-dev \
  iproute2 policykit-1                  # ip(8) + pkexec at runtime

# Node 18+ (the frontend) — e.g. via nodesource or your distro
sudo apt install -y nodejs npm

# Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
wails doctor          # confirms the GTK/WebKit deps are found

# appimagetool (packaging)
wget -O /usr/local/bin/appimagetool \
  https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x /usr/local/bin/appimagetool
```

> **WebKit2GTK version:** older distros ship `libwebkit2gtk-4.0` (API 4.0); newer
> ones ship `4.1`. Wails defaults to 4.0; for 4.1 build with the `webkit2_41`
> tag: `wails build -tags webkit2_41` (or `WAILS_TAGS=webkit2_41 ./build/appimage.sh`).
> Fedora: `gtk3-devel webkit2gtk4.1-devel`; Arch: `gtk3 webkit2gtk-4.1`.

## Quick checks you can run anywhere (incl. this Mac)

```sh
make test                       # the offline display/geometry unit tests
make vet                        # go vet for the Linux build
GOOS=linux go build ./...       # type-check the whole module for Linux
make helper                     # cross-build the static helper → bin/pharos-helper
```

## Build + run on the Linux box

```sh
# Dev (hot reload):
wails dev                       # or: make dev

# Release GUI + helper:
wails build                     # → build/bin/caravel-linux  (the GUI)
make helper                     # → bin/pharos-helper         (the privileged worker)

# One-shot AppImage (builds frontend + GUI + helper, stages, packages):
make appimage                   # → dist/PharosVPN-x86_64.AppImage
#   WAILS_TAGS=webkit2_41 make appimage     # on WebKit2GTK 4.1 distros
```

## Installing / running (end user)

The AppImage is self-contained. On first **Connect** the GUI runs
`pkexec pharos-helper install`, which (one auth prompt) copies the helper to
`/usr/local/lib/pharosvpn/` and starts the `pharosvpn-helper.service` systemd
unit holding `CAP_NET_ADMIN`. After that, connect/disconnect go over the
helper's control socket (`/run/pharosvpn/control.sock`) with **no further
prompt** — the same "authorize once" model as the macOS LaunchDaemon.

For a smoother prompt, install the polkit policy system-wide (optional):

```sh
sudo cp build/linux/org.pharosvpn.caravel.policy /usr/share/polkit-1/actions/
```

### Testing the tunnel from the CLI (no GUI)

```sh
# import / sync / inspect (unprivileged)
pharos-helper import myprofile.pharos
pharos-helper sync mydevice.pharosid            # prompts for the passphrase
pharos-helper list
pharos-helper profiles myprofile

# bring a tunnel up directly (root), Ctrl-C to stop
sudo pharos-helper connect --profile myprofile --name home
pharos-helper status

# or via the installed daemon (no per-connect prompt)
sudo pharos-helper install
pharos-helper ctl connect "$HOME/.config/PharosVPN/profiles/myprofile.pharos" --name home
pharos-helper ctl status
pharos-helper ctl disconnect
```

## The app icon

`build/appicon.png` (1024×1024, transparent) ships a programmatically-drawn
beacon mark in the brand maroon/cream. For pixel parity with the other clients,
rasterize the canonical SVG **with an alpha-preserving renderer** (NOT
`qlmanage`, which flattens transparency onto white):

```sh
rsvg-convert -w 1024 -h 1024 .assets/icon.svg -o build/appicon.png
#   or: resvg .assets/icon.svg build/appicon.png
#   or: inkscape -w 1024 -h 1024 .assets/icon.svg -o build/appicon.png
```

(None of those rasterizers were available on the dev Mac, so the shipped PNG is
the Go-drawn beacon; swap it on the Linux box if you want the exact SVG.)

## Notes / gotchas

- **`frontend/dist/`** holds a committed placeholder `index.html` so
  `//go:embed all:frontend/dist` compiles before a build; `wails build` /
  `npm run build` overwrite it. Don't delete the directory.
- **`frontend/wailsjs/`** holds hand-kept stubs of the Go/runtime bindings so the
  frontend builds standalone; `wails build` regenerates identical files from the
  bound methods in `app.go`. Keep them in sync if you change `app.go`'s exported
  methods.
- The helper needs `iproute2` (`ip`) at runtime for addressing/routing, and
  `/dev/net/tun` present. The systemd unit grants `CAP_NET_ADMIN`/`CAP_NET_RAW`.
- No systemd? `pharos-helper install` falls back to launching the daemon
  detached for the session (won't survive a reboot without an init unit).
