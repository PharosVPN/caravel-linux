// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build !linux

// Non-Linux stubs so the worker package (and anything importing it — the Wails
// GUI bindings) compiles on a Mac/Windows dev box for cross-checking. The real
// tunnel, daemon, and systemd install live in the //go:build linux files and run
// only on Linux.

package worker

import "errors"

const (
	HelperBin   = "/usr/local/lib/pharosvpn/pharos-helper"
	ServiceName = "pharosvpn-helper.service"
	ServicePath = "/etc/systemd/system/pharosvpn-helper.service"
)

var errLinuxOnly = errors.New("the PharosVPN tunnel runs on Linux only")

// CtlRequest / CtlResponse mirror the daemon protocol so the GUI's ctl client
// type-checks off-Linux. The real implementation is in daemon_linux.go.
type CtlRequest struct {
	Op       string `json:"op"`
	Profile  string `json:"profile,omitempty"`
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
	Proto    string `json:"proto,omitempty"`
	Full     *bool  `json:"full,omitempty"`
}

type CtlResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Status   string `json:"status"`
	Profile  string `json:"profile,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Iface    string `json:"iface,omitempty"`
}

// Tunnel is a stub on non-Linux; Connect always errors there.
type Tunnel struct{}

func (t *Tunnel) Iface() string         { return "" }
func (t *Tunnel) Close() error          { return nil }
func (t *Tunnel) Stats() (int64, int64) { return 0, 0 }

func Connect(_ DialSpec, _ bool) (*Tunnel, error) { return nil, errLinuxOnly }
func RunDaemon() error                            { return errLinuxOnly }
func SendCtl(_ CtlRequest) (CtlResponse, error)   { return CtlResponse{}, errLinuxOnly }
func Install() error                              { return errLinuxOnly }
func Uninstall() error                            { return errLinuxOnly }
func Installed() bool                             { return false }
