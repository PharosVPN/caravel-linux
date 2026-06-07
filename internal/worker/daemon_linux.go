// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build linux

package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// CtlRequest / CtlResponse are the newline-delimited JSON control protocol the
// GUI and CLI speak to the root daemon. Mirrors the mac daemon's protocol.
type CtlRequest struct {
	Op       string `json:"op"`                 // connect | disconnect | status
	Profile  string `json:"profile,omitempty"`  // absolute .pharos bundle path (connect)
	Name     string `json:"name,omitempty"`     // named profile within the bundle (connect)
	Password string `json:"password,omitempty"` // password-mode profiles (connect)
	Proto    string `json:"proto,omitempty"`    // auto|amneziawg|xray when no name (connect)
	Full     *bool  `json:"full,omitempty"`     // full-tunnel (default true)
}

type CtlResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Status   string `json:"status"` // connected | disconnected
	Profile  string `json:"profile,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Iface    string `json:"iface,omitempty"`
}

// daemon holds the one active tunnel and serves the control socket.
type daemon struct {
	mu       sync.Mutex
	tn       *Tunnel
	label    string
	proto    string
	endpoint string
	iface    string
	since    time.Time
}

// RunDaemon runs the root helper: it listens on the control socket and manages
// the tunnel. Launched via pkexec/polkit (or a systemd unit / sudo). Mirrors the
// mac cmdDaemon.
func RunDaemon() error {
	if os.Geteuid() != 0 {
		return errors.New("daemon must run as root (CAP_NET_ADMIN) — launch via pkexec or systemd")
	}
	if err := os.MkdirAll(filepath.Dir(ControlSocket), 0o755); err != nil {
		return err
	}
	_ = os.Remove(ControlSocket)
	ln, err := net.Listen("unix", ControlSocket)
	if err != nil {
		return fmt.Errorf("listen %s: %w", ControlSocket, err)
	}
	_ = os.Chmod(ControlSocket, 0o666)

	d := &daemon{}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sigc
		d.disconnect()
		_ = ln.Close()
		_ = os.Remove(ControlSocket)
		os.Exit(0)
	}()

	go d.statsLoop()
	fmt.Println("pharos-helper daemon: ready on", ControlSocket)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil
		}
		go d.handle(conn)
	}
}

func (d *daemon) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	var req CtlRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(CtlResponse{Error: "bad request"})
		return
	}
	var resp CtlResponse
	switch req.Op {
	case "connect":
		full := true
		if req.Full != nil {
			full = *req.Full
		}
		if err := d.connect(req.Profile, req.Name, req.Password, req.Proto, full); err != nil {
			resp = CtlResponse{Error: err.Error(), Status: "disconnected"}
		} else {
			resp = d.statusResp()
		}
	case "disconnect":
		d.disconnect()
		resp = d.statusResp()
	case "status", "":
		resp = d.statusResp()
	default:
		resp = CtlResponse{Error: "unknown op " + req.Op}
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func (d *daemon) connect(profilePath, name, password, proto string, full bool) error {
	if profilePath == "" {
		return errors.New("profile path is required")
	}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("read profile: %w", err)
	}
	spec, err := ResolveProfileSpec(data, name, "", password, proto)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tn != nil { // switch: tear the old tunnel down first
		d.tn.Close()
		d.tn = nil
		ClearState()
	}
	tn, err := Connect(spec, full)
	if err != nil {
		return err
	}
	d.tn, d.label, d.proto, d.endpoint, d.iface, d.since = tn, spec.Label, spec.Proto, spec.Endpoint, tn.Iface(), time.Now()
	rx, tx := tn.Stats()
	_ = WriteState(State{Profile: spec.Label, Proto: spec.Proto, Iface: tn.Iface(), Endpoint: spec.Endpoint,
		PID: os.Getpid(), Since: d.since, RX: rx, TX: tx})
	return nil
}

// statsLoop refreshes RX/TX in the state file while a tunnel is up.
func (d *daemon) statsLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		d.mu.Lock()
		if d.tn != nil {
			rx, tx := d.tn.Stats()
			_ = WriteState(State{Profile: d.label, Proto: d.proto, Iface: d.iface, Endpoint: d.endpoint,
				PID: os.Getpid(), Since: d.since, RX: rx, TX: tx})
		}
		d.mu.Unlock()
	}
}

func (d *daemon) disconnect() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tn != nil {
		d.tn.Close()
		d.tn = nil
		ClearState()
	}
	d.label, d.proto, d.endpoint, d.iface = "", "", "", ""
}

func (d *daemon) statusResp() CtlResponse {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tn == nil {
		return CtlResponse{OK: true, Status: "disconnected"}
	}
	return CtlResponse{OK: true, Status: "connected", Profile: d.label, Endpoint: d.endpoint, Iface: d.iface}
}

// SendCtl sends one request to the daemon and returns its response. Used by the
// CLI and the GUI (which speaks the same protocol).
func SendCtl(req CtlRequest) (CtlResponse, error) {
	conn, err := net.DialTimeout("unix", ControlSocket, 3*time.Second)
	if err != nil {
		return CtlResponse{}, fmt.Errorf("daemon not reachable (install it: `pharos-helper install`): %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return CtlResponse{}, err
	}
	var resp CtlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return CtlResponse{}, err
	}
	return resp, nil
}
