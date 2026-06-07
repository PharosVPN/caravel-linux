// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// State is the running-tunnel record the root worker writes while connected, so
// the GUI (and `pharos-helper status`) can see what's up. It lives at a fixed
// system path (SharedStateFile) and holds no secrets.
type State struct {
	Profile  string    `json:"profile"`
	Proto    string    `json:"proto"` // resolved data-plane protocol (amneziawg | xray-reality)
	Iface    string    `json:"iface"`
	Endpoint string    `json:"endpoint"`
	PID      int       `json:"pid"`
	Since    time.Time `json:"since"`
	RX       int64     `json:"rx"`
	TX       int64     `json:"tx"`
}

// WriteState records the running tunnel at the shared path (world-readable so an
// unprivileged GUI can see it; it holds no secrets). The worker runs as root, so
// it can create the system directory.
func WriteState(s State) error {
	if err := os.MkdirAll(filepath.Dir(SharedStateFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SharedStateFile, data, 0o644)
}

// ClearState removes the running-tunnel record.
func ClearState() { _ = os.Remove(SharedStateFile) }

// ReadState returns the recorded tunnel state, or (zero, false) if none is
// recorded or the recorded worker is no longer alive (a stale record).
func ReadState() (State, bool) {
	data, err := os.ReadFile(SharedStateFile)
	if err != nil {
		return State{}, false
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, false
	}
	if s.PID > 0 && !ProcessAlive(s.PID) {
		return State{}, false
	}
	return s, true
}

// ProcessAlive reports whether a PID names a live process (signal 0 probe).
func ProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// HumanBytes formats a byte count compactly (e.g. 1.2 MB).
func HumanBytes(n int64) string {
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(u), 0
	for x := n / u; x >= u; x /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// readFile is a thin os.ReadFile wrapper kept package-local so spec.go can resolve
// a file-or-name reference without importing os everywhere.
func readFile(path string) ([]byte, error) { return os.ReadFile(path) }
