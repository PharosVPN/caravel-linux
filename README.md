# caravel-linux

The **PharosVPN desktop client for Linux** — a [Wails](https://wails.io) app
(Go backend + Svelte web UI) matching the macOS client's features and polish,
shipped as an **AppImage** for Ubuntu / Fedora / Arch.

PharosVPN is a self-hostable, dual-protocol VPN. caravel is its client: log into
your account, sync your **profiles** from the controller (coxswain), and connect
over **AmneziaWG** or **XRay/REALITY** — direct or through a multi-hop cascade —
all drawn on a live world map.

## Features (parity with the macOS client)

- **Import** a `.pharos` profile, or **cloud-sync** with a `.pharosid` device
  file + your account passphrase (stored in the libsecret/Secret Service keyring;
  first sync = login). Sync is **replace-all**; imported profiles are untouched.
- **Named profiles** per bundle; connect by name. A `both` profile shows an
  **Auto / AmneziaWG / XRay** picker (auto prefers AmneziaWG).
- **Cascade**: a profile's egress chain (entry → mid → exit) is shown as a route.
- **The map** — a "You" pin, the node chain, and a **controller** pin; data plane
  drawn as **dashed** teal arcs, control plane as a **solid** violet line, with
  flowing traffic + pulsing pins while connected. Built on the coxswain web
  design system.
- **Controller card**: reachability dot, "last synced … via <relay>", Sync now,
  Log out. **Live protocol indicator** + throughput while connected.

## Architecture

- **`caravel-linux`** (the Wails GUI) — lists profiles, draws the map, drives
  connect/disconnect. The Linux counterpart of `TunnelController.swift`.
- **`pharos-helper`** (a small root binary) — opens `/dev/net/tun`, runs the
  caravel engine (`vp.Up`/`vp.UpXRay`), applies `iproute2` routing, and serves a
  control socket. Installed once as a **systemd** service via **pkexec** (the
  "authorize once" model the macOS LaunchDaemon uses), and doubles as the
  import/sync CLI. The Linux counterpart of `caravel-mac`.

Both consume the shared core `caravel/go` directly (`replace … => ../caravel/go`)
and are **not** modified copies of it.

## Build

See **[BUILD.md](./BUILD.md)**. TL;DR on a Linux box:

```sh
make test          # offline unit tests (run anywhere)
make appimage      # → dist/PharosVPN-x86_64.AppImage  (needs Linux + Wails + Node)
```

The helper and the whole Go GUI cross-compile for Linux from any OS
(`GOOS=linux go build ./...`); the frontend build, GUI runtime, and AppImage
packaging need Linux (GTK/WebKit2GTK).

See [docs/cloud-sync.md](https://github.com/PharosVPN/docs/blob/main/cloud-sync.md)
for the cloud-sync UX contract this client implements.
