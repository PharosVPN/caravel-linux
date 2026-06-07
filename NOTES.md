<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->

# Notes — caravel-linux

Observations from building the Linux client against the shared core
(`caravel/go`). **The core was not modified** (per the project rule); anything
below that wants a core change is a TODO for a later, deliberate core pass.

## Core gaps / TODOs (no core edits made)

- **None blocking.** The Linux client wires to the same surface the macOS client
  uses — `profile.Parse`, `profile.Store`, `profile.WrapPlaintext`,
  `sync.Fetch`, `sync.Reachable`, `deviceid.Parse`, `vp.Up`, `vp.UpXRay`,
  `Tunnel.Stats/Close` — and needed nothing new from it. The helper and GUI
  cross-compile for Linux unchanged.

- **`profile.Parse` opaque-bundle peek (minor, cosmetic).** Like the macOS
  client, the GUI peeks a plaintext (`enc:none`) bundle's `profiles[]` by parsing
  the envelope JSON directly (`internal/uimodel`), because `profile.Parse`
  returns `ErrPasswordNeeded`/`ErrAccountKeyNeeded` for encrypted bundles and the
  list view has no secret yet. That's by design — details fill in once the worker
  connects. If a future core helper exposed the always-readable header fields
  (name/protocol/region) without secrets, both clients could drop their hand
  JSON peek. Not needed now.

## Linux-specific design choices (parity with mac, adapted)

- **Privilege model.** macOS uses a root LaunchDaemon + a control socket; Linux
  uses a **systemd service** holding `CAP_NET_ADMIN` + the same control-socket
  protocol, installed once via **pkexec/polkit**. `pharos-helper install`
  writes/loads the unit; the GUI shells out to `pkexec pharos-helper install`.

- **Routing.** macOS uses `ifconfig`/`route`; Linux uses `iproute2` (`ip addr`,
  `ip route`) with the same 0.0.0.0/1 + 128.0.0.0/1 default-override split and
  the endpoint pinned to the physical gateway. Teardown undoes each step in
  reverse. TUN device is `pharos0` via `/dev/net/tun`.

- **Secret store.** macOS Keychain → Linux **Secret Service** (libsecret / GNOME
  Keyring / KWallet) via `github.com/zalando/go-keyring`. One item, same
  `org.pharosvpn.caravel` / `account-passphrase` keys as the mac app.

- **Map.** The signature map is ported to Svelte using the **coxswain FleetMap**
  aesthetic (d3-geo `naturalEarth1`, `world-atlas` land, bowed arcs, flowing
  pulses) driven by a pins/arcs model the Go backend computes
  (`internal/uimodel`), so the geometry (great-circle, two planes) matches the
  macOS `LandMap` and is unit-tested in Go.

## Verified on the dev Mac (cross-compile + tests)

- `GOOS=linux go build ./...` — the helper **and** the full Wails GUI build clean.
- `go vet ./...` (Linux) — clean.
- `go test ./internal/...` — region lookup, great-circle, two-plane map build,
  and plaintext/opaque bundle peek all pass.

## Still needs the Linux VM (documented in BUILD.md)

- The frontend build (Node/Vite), the GUI **runtime** (GTK/WebKit2GTK), the
  AppImage packaging, and a live TUN connect — all Linux-only. Everything that
  can be built/checked on the Mac has been.
