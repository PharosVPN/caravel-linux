// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build linux

package worker

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/PharosVPN/caravel/core/profile"
	"github.com/PharosVPN/caravel/core/vp"
	"github.com/amnezia-vpn/amneziawg-go/device"
	"github.com/amnezia-vpn/amneziawg-go/tun"
)

// ifaceName is the TUN device the Linux client creates. A fixed name keeps the
// teardown deterministic and is easy to spot (`ip link show pharos0`).
const ifaceName = "pharos0"

// vpTunnel is the common surface of the AmneziaWG (*vp.Tunnel) and XRay/REALITY
// (*vp.XRayTunnel) engines, so the worker handles both uniformly.
type vpTunnel interface {
	Close() error
	Stats() (rx, tx int64, ok bool)
}

// Tunnel is a running tunnel plus the host network state to undo on close.
type Tunnel struct {
	vt    vpTunnel
	iface string
	undo  []func() // teardown steps, run in reverse order on Close
}

// Iface returns the tunnel's interface name.
func (t *Tunnel) Iface() string { return t.iface }

// Connect creates a TUN device, brings up the chosen engine over it, and applies
// addressing + routing. full=true routes all traffic through the tunnel (with
// the server endpoint pinned to the physical gateway); full=false routes only
// the profile's AllowedIPs.
func Connect(spec DialSpec, full bool) (*Tunnel, error) {
	mtu := spec.MTU
	if mtu == 0 {
		mtu = 1420
	}
	dev, err := tun.CreateTUN(ifaceName, mtu)
	if err != nil {
		return nil, fmt.Errorf("create tun %s: %w (is /dev/net/tun present? are you root/CAP_NET_ADMIN?)", ifaceName, err)
	}
	name, err := dev.Name()
	if err != nil {
		dev.Close()
		return nil, err
	}

	var vt vpTunnel
	if spec.Proto == profile.ProtocolXRayReality {
		vt, err = vp.UpXRay(spec.XRay, dev)
	} else {
		vt, err = vp.Up(spec.Cfg, dev, device.LogLevelError)
	}
	if err != nil {
		dev.Close() // vp.Up/UpXRay close the device on failure, but be safe
		return nil, err
	}

	t := &Tunnel{vt: vt, iface: name}
	if err := t.configureNetwork(spec, full); err != nil {
		t.Close()
		return nil, fmt.Errorf("configure network: %w", err)
	}
	return t, nil
}

// configureNetwork addresses the TUN device and installs routes. For a full
// tunnel it pins the server endpoint to the current physical gateway (so the
// encrypted packets to the server don't loop back into the tunnel) and overrides
// the default route with the 0.0.0.0/1 + 128.0.0.0/1 split.
func (t *Tunnel) configureNetwork(spec DialSpec, full bool) error {
	if spec.Address == "" {
		return errors.New("profile/config has no tunnel address")
	}
	// Address the interface and bring it up. /32 — the routes carry the reach.
	if err := ip("addr", "add", spec.Address+"/32", "dev", t.iface); err != nil {
		return err
	}
	t.undo = append(t.undo, func() { _ = ipQuiet("addr", "del", spec.Address+"/32", "dev", t.iface) })
	if err := ip("link", "set", "dev", t.iface, "up"); err != nil {
		return err
	}
	if spec.MTU > 0 {
		_ = ip("link", "set", "dev", t.iface, "mtu", itoa(spec.MTU))
	}

	if !full {
		for _, cidr := range spec.AllowedIPs {
			if cidr == "0.0.0.0/0" || cidr == "::/0" {
				continue // split-tunnel: skip default-equivalent routes
			}
			if err := ip("route", "add", cidr, "dev", t.iface); err != nil {
				return err
			}
			c := cidr
			t.undo = append(t.undo, func() { _ = ipQuiet("route", "del", c, "dev", t.iface) })
		}
		return nil
	}

	// Pin the server endpoint to the current physical gateway so the tunnel's own
	// encrypted packets (WireGuard UDP / REALITY TCP) reach it directly.
	host, _, err := net.SplitHostPort(spec.Endpoint)
	if err != nil {
		host = spec.Endpoint
	}
	ipAddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return fmt.Errorf("resolve endpoint %q: %w", host, err)
	}
	gw, dev0, err := defaultRoute()
	if err != nil {
		return err
	}
	if err := ip("route", "add", ipAddr.String()+"/32", "via", gw, "dev", dev0); err != nil {
		return err
	}
	endpointHost := ipAddr.String()
	t.undo = append(t.undo, func() { _ = ipQuiet("route", "del", endpointHost+"/32") })

	// Override the default route with two /1 halves pointed at the tunnel; this
	// out-specifies the system default without deleting it, so teardown is clean.
	for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := ip("route", "add", half, "dev", t.iface); err != nil {
			return err
		}
		h := half
		t.undo = append(t.undo, func() { _ = ipQuiet("route", "del", h, "dev", t.iface) })
	}
	return nil
}

// Close tears down routes/addresses (in reverse order) and the engine.
func (t *Tunnel) Close() error {
	for i := len(t.undo) - 1; i >= 0; i-- {
		t.undo[i]()
	}
	t.undo = nil
	if t.vt != nil {
		t.vt.Close()
		t.vt = nil
	}
	return nil
}

// Stats returns the tunnel's cumulative RX/TX bytes (0 if unavailable).
func (t *Tunnel) Stats() (rx, tx int64) {
	if t.vt == nil {
		return 0, 0
	}
	rx, tx, _ = t.vt.Stats()
	return rx, tx
}

// defaultRoute returns the current IPv4 default gateway and its interface.
func defaultRoute() (gw, dev string, err error) {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil {
		return "", "", fmt.Errorf("read default route: %w", err)
	}
	// e.g. "default via 192.168.0.1 dev eth0 proto dhcp metric 100"
	fields := strings.Fields(string(out))
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "via":
			gw = fields[i+1]
		case "dev":
			dev = fields[i+1]
		}
	}
	if gw == "" || dev == "" {
		return "", "", errors.New("no default gateway found (ip route show default)")
	}
	return gw, dev, nil
}

// ip runs an `ip` command, surfacing stderr on failure.
func ip(args ...string) error {
	cmd := exec.Command("ip", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ipQuiet runs an `ip` command, ignoring errors (teardown best-effort).
func ipQuiet(args ...string) error {
	_ = exec.Command("ip", args...).Run()
	return nil
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
