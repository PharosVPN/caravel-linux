// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build linux

package worker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// The privileged helper is installed as a systemd system service so it holds
// CAP_NET_ADMIN once and the unprivileged GUI drives it over the control socket
// thereafter — the Linux mirror of the mac LaunchDaemon. install/uninstall are
// the only steps that need root (run via pkexec from the GUI, or sudo from a
// terminal); connect/disconnect afterwards never prompt.
const (
	HelperBin   = "/usr/local/lib/pharosvpn/pharos-helper"
	ServiceName = "pharosvpn-helper.service"
	ServicePath = "/etc/systemd/system/pharosvpn-helper.service"
	socketDir   = "/run/pharosvpn"
)

// Install copies this binary to a stable location, writes the systemd unit, and
// starts the service. Needs root. Mirrors caravel-mac cmdInstallHelper.
func Install() error {
	if os.Geteuid() != 0 {
		return errors.New("install must run as root (the GUI authorizes this once via pkexec; or use sudo)")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(HelperBin), 0o755); err != nil {
		return err
	}
	if err := copyFile(self, HelperBin); err != nil {
		return fmt.Errorf("copy helper: %w", err)
	}
	if err := os.Chmod(HelperBin, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ServicePath, []byte(unitTemplate), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}

	// systemd path: enable + (re)start. daemon-reload picks up unit changes on an
	// upgrade; restart (not just start) re-launches a stale daemon.
	if hasSystemd() {
		_ = exec.Command("systemctl", "daemon-reload").Run()
		if out, err := exec.Command("systemctl", "enable", "--now", ServiceName).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl enable --now: %w (%s)", err, out)
		}
		_ = exec.Command("systemctl", "restart", ServiceName).Run()
	} else {
		// No systemd (rare on the desktop): launch the daemon detached so connect
		// still works this session. It won't survive a reboot without an init unit.
		cmd := exec.Command(HelperBin, "daemon")
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start daemon (no systemd): %w", err)
		}
		_ = cmd.Process.Release()
	}

	// Wait until the socket actually ACCEPTS — the daemon needs a moment to bind,
	// and the first connect would otherwise race (the mac had this exact bug).
	for i := 0; i < 60; i++ {
		if c, err := net.DialTimeout("unix", ControlSocket, 200*time.Millisecond); err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("helper installed but its control socket did not come up")
}

// Uninstall stops and removes the systemd service and helper files.
func Uninstall() error {
	if os.Geteuid() != 0 {
		return errors.New("uninstall must run as root")
	}
	if hasSystemd() {
		_ = exec.Command("systemctl", "disable", "--now", ServiceName).Run()
		_ = exec.Command("systemctl", "daemon-reload").Run()
	}
	_ = os.Remove(ServicePath)
	_ = os.Remove(HelperBin)
	_ = os.Remove(ControlSocket)
	return nil
}

// Installed reports whether the systemd unit is present.
func Installed() bool {
	_, err := os.Stat(ServicePath)
	return err == nil
}

// hasSystemd reports whether systemctl is usable (PID 1 is systemd).
func hasSystemd() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	// /run/systemd/system exists iff systemd is the init system.
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// unitTemplate is the systemd service that runs the privileged daemon. It keeps
// CAP_NET_ADMIN/CAP_NET_RAW (for the TUN + routes) and restarts on failure.
const unitTemplate = `[Unit]
Description=PharosVPN privileged tunnel helper
Documentation=https://github.com/PharosVPN/caravel-linux
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/lib/pharosvpn/pharos-helper daemon
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
RuntimeDirectory=pharosvpn
RuntimeDirectoryMode=0755
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
