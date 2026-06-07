// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package worker

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/PharosVPN/caravel/core/profile"
)

// On Linux the user-facing files follow the XDG Base Directory spec, while the
// privileged daemon's control socket + running-state file live under a fixed
// system path (so a root daemon and an unprivileged GUI agree on them regardless
// of which user is logged in). None of the shared files hold secrets at rest.
const (
	// ControlSocket is where the root daemon listens; the GUI and CLI dial it to
	// bring tunnels up/down with no privilege prompt each time (the daemon already
	// holds CAP_NET_ADMIN). 0666 so any local user can drive it.
	ControlSocket = "/run/pharosvpn/control.sock"

	// SharedStateFile is the running-tunnel record the root worker writes and the
	// unprivileged GUI reads. World-readable; holds no secrets.
	SharedStateFile = "/run/pharosvpn/state.json"
)

// xdgConfigBase returns the user's XDG config base ($XDG_CONFIG_HOME, else
// ~/.config). When the daemon runs as root via pkexec/sudo it targets the
// *invoking* user's home (SUDO_USER / PKEXEC_UID) so the CLI finds the user's
// store rather than root's empty one. The GUI always passes absolute paths, so
// this only matters for the `sudo pharos-helper …` terminal case.
func xdgConfigBase() (string, error) {
	if home, ok := invokingUserHome(); ok {
		return filepath.Join(home, ".config"), nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// invokingUserHome resolves the home directory of the user who invoked a
// privileged run (sudo/pkexec), if any.
func invokingUserHome() (string, bool) {
	if os.Geteuid() != 0 {
		return "", false
	}
	if su := os.Getenv("SUDO_USER"); su != "" {
		if u, err := user.Lookup(su); err == nil && u.HomeDir != "" {
			return u.HomeDir, true
		}
	}
	if uid := os.Getenv("PKEXEC_UID"); uid != "" {
		if u, err := user.LookupId(uid); err == nil && u.HomeDir != "" {
			return u.HomeDir, true
		}
	}
	return "", false
}

// StoreDir is the on-disk profile store (~/.config/PharosVPN/profiles).
func StoreDir() (string, error) {
	base, err := xdgConfigBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "PharosVPN", "profiles"), nil
}

// OpenStore opens the on-disk profile store.
func OpenStore() (*profile.Store, error) {
	dir, err := StoreDir()
	if err != nil {
		return nil, err
	}
	return profile.NewStore(dir)
}
