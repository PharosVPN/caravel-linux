// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PharosVPN/caravel-linux/internal/keystore"
	"github.com/PharosVPN/caravel-linux/internal/uimodel"
	"github.com/PharosVPN/caravel-linux/internal/worker"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound GUI backend — the Linux counterpart of the macOS
// TunnelController. It lists stored profiles, polls the worker's state file for
// live status, drives connect/disconnect over the helper's control socket, and
// manages the cloud session (sync / logout / keystore). It is fully offline: no
// network, no geolocation. The privileged tunnel work happens in pharos-helper.
type App struct {
	ctx      context.Context
	stopPoll context.CancelFunc
	// connecting flags a connect/sync in flight (the UI busy state). Read by the
	// 2s state poller while a bound method sets it, so it is atomic.
	connecting atomic.Bool
}

// NewApp builds the GUI backend.
func NewApp() *App { return &App{} }

// Version returns the app version (baked from the repo-root VERSION file), for
// the About line + diagnostics. The bundled caravel core reports its own version
// separately (caravel/go: CoreVersion).
func (a *App) Version() string { return strings.TrimSpace(version) }

// startup wires the Wails context and starts the background pollers (live tunnel
// state every 2s; controller liveness every 30s — gentle, per the contract).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	pctx, cancel := context.WithCancel(ctx)
	a.stopPoll = cancel
	go a.pollStateLoop(pctx)
	go a.pollControllerLoop(pctx)
}

func (a *App) shutdown(context.Context) {
	if a.stopPoll != nil {
		a.stopPoll()
	}
}

// ───────── profiles ─────────

// Profile is the JSON-friendly profile row the frontend renders.
type Profile struct {
	ID          string               `json:"id"`
	Bundle      string               `json:"bundle"`
	ProfileName string               `json:"profileName"`
	Name        string               `json:"name"`
	Enc         string               `json:"enc"`
	Proto       string               `json:"proto"`
	ProtoBadge  string               `json:"protoBadge"`
	IsBoth      bool                 `json:"isBoth"`
	Readable    bool                 `json:"readable"`
	CloudSynced bool                 `json:"cloudSynced"`
	Disabled    bool                 `json:"disabled"`
	Nodes       []uimodel.NodeInfo   `json:"nodes"`
	Path        *uimodel.PathView    `json:"path"`
	Control     *uimodel.ControlInfo `json:"control"`
}

func toProfile(p uimodel.ProfileInfo) Profile {
	name := p.ProfileName
	if name == "" {
		name = p.Bundle
	}
	return Profile{
		ID: p.ID(), Bundle: p.Bundle, ProfileName: p.ProfileName, Name: name,
		Enc: p.Enc, Proto: p.Proto, ProtoBadge: protoBadge(p.Proto), IsBoth: p.Proto == "both",
		Readable: p.Enc == "none", CloudSynced: p.CloudSynced, Disabled: p.Disabled,
		Nodes: p.Nodes, Path: p.Path, Control: p.Control,
	}
}

func protoBadge(proto string) string {
	switch proto {
	case "amneziawg":
		return "AmneziaWG"
	case "xray-reality", "xray":
		return "XRay"
	case "both":
		return "Both"
	case "":
		return ""
	default:
		return proto
	}
}

func (a *App) storeDir() string {
	dir, err := worker.StoreDir()
	if err != nil {
		return ""
	}
	return dir
}

// ListProfiles returns every named profile in the store, cloud + imported.
func (a *App) ListProfiles() []Profile {
	infos := uimodel.List(a.storeDir())
	out := make([]Profile, 0, len(infos))
	for _, p := range infos {
		out = append(out, toProfile(p))
	}
	return out
}

func (a *App) findInfo(id string) *uimodel.ProfileInfo {
	for _, p := range uimodel.List(a.storeDir()) {
		if p.ID() == id {
			return &p
		}
	}
	return nil
}

// ───────── connection state ─────────

// TunnelState mirrors the worker's running-tunnel record, plus a UI status.
type TunnelState struct {
	Status    string `json:"status"` // disconnected | connecting | connected | disconnecting
	Profile   string `json:"profile,omitempty"`
	Proto     string `json:"proto,omitempty"`
	Iface     string `json:"iface,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	RX        int64  `json:"rx"`
	TX        int64  `json:"tx"`
	SinceUnix int64  `json:"sinceUnix,omitempty"`
}

// GetState returns the current tunnel status the frontend renders.
func (a *App) GetState() TunnelState {
	s, ok := worker.ReadState()
	if !ok {
		if a.connecting.Load() {
			return TunnelState{Status: "connecting"}
		}
		return TunnelState{Status: "disconnected"}
	}
	return TunnelState{
		Status: "connected", Profile: s.Profile, Proto: s.Proto, Iface: s.Iface,
		Endpoint: s.Endpoint, RX: s.RX, TX: s.TX, SinceUnix: s.Since.Unix(),
	}
}

// pollStateLoop emits the live tunnel state to the frontend every 2s.
func (a *App) pollStateLoop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			wruntime.EventsEmit(a.ctx, "state", a.GetState())
		}
	}
}

// ───────── connect / disconnect ─────────

// Connect brings the selected profile up: it installs the privileged helper once
// (one pkexec prompt) if needed, then drives the tunnel over the control socket
// — no prompt per connect after the first install. proto is the protocol picker
// value ("auto"|"amneziawg"|"xray") for a "both" profile.
func (a *App) Connect(id, proto, password string) error {
	info := a.findInfo(id)
	if info == nil {
		return errors.New("no profile selected")
	}
	a.connecting.Store(true)
	wruntime.EventsEmit(a.ctx, "state", TunnelState{Status: "connecting"})
	defer func() { a.connecting.Store(false) }()

	bundlePath := filepath.Join(a.storeDir(), info.Bundle+".pharos")

	// Ensure the helper is installed + its socket is up.
	if !worker.Installed() || a.helperStale() {
		if err := a.ensureHelper(); err != nil {
			return err
		}
	}

	args := []string{"ctl", "connect", bundlePath}
	if info.ProfileName == "" {
		args = append(args, "--protocol", proto)
	} else {
		args = append(args, "--name", info.ProfileName)
		if info.Proto == "both" {
			args = append(args, "--protocol", proto)
		}
	}
	if password != "" {
		args = append(args, "--password", password)
	}

	// A just-(re)installed daemon needs a moment to bind its socket; retry the
	// connect while it is not yet reachable (the mac had this exact race).
	if err := a.runHelperWaiting(args); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "state", a.GetState())
	return nil
}

// Disconnect tears the active tunnel down.
func (a *App) Disconnect() error {
	wruntime.EventsEmit(a.ctx, "state", TunnelState{Status: "disconnecting"})
	if _, err := a.runHelper("ctl", "disconnect"); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "state", a.GetState())
	return nil
}

// helperStale reports whether the installed helper differs from this app's
// bundled one (size proxy), so an upgraded app reinstalls on the next connect.
func (a *App) helperStale() bool {
	bundled := a.bundledHelperPath()
	bi, err1 := os.Stat(bundled)
	ii, err2 := os.Stat(worker.HelperBin)
	if err1 != nil || err2 != nil {
		return false // can't compare → don't force a reinstall
	}
	return bi.Size() != ii.Size()
}

// ensureHelper installs the privileged helper via pkexec (the one and only
// password prompt). On a system without pkexec it falls back to a clear error.
func (a *App) ensureHelper() error {
	helper := a.bundledHelperPath()
	if _, err := os.Stat(helper); err != nil {
		return fmt.Errorf("bundled helper not found at %s", helper)
	}
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return errors.New("pkexec not found — install polkit, or run `sudo pharos-helper install` once")
	}
	out, err := exec.Command(pkexec, helper, "install").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("authorize the helper: %s", msg)
	}
	return nil
}

// ───────── cloud sync ─────────

// CloudInfo is the cloud-session summary the controller card renders.
type CloudInfo struct {
	Bundle        string `json:"bundle"`
	LoggedIn      bool   `json:"loggedIn"`
	Reachable     bool   `json:"reachable"`
	LastSyncedAt  string `json:"lastSyncedAt,omitempty"`
	Relay         string `json:"relay,omitempty"`
	HasController bool   `json:"hasController"`
}

// cloudBundle returns the cloud-synced bundle to act on (the first one).
func (a *App) cloudBundle() string {
	for _, p := range uimodel.List(a.storeDir()) {
		if p.CloudSynced {
			return p.Bundle
		}
	}
	return ""
}

// GetCloudInfo returns the cloud session for the controller card (nil-ish when
// there is no cloud profile). Reachability is a network probe, so the frontend
// calls this on demand / on a gentle interval, not in a tight loop.
func (a *App) GetCloudInfo() *CloudInfo {
	bundle := a.cloudBundle()
	if bundle == "" {
		return nil
	}
	ci := &CloudInfo{Bundle: bundle, LoggedIn: keystore.HasCredential()}
	if st, err := worker.ControllerStatusFor(bundle); err == nil {
		ci.Reachable = st.Reachable
		ci.LastSyncedAt = st.LastSyncedAt
		ci.Relay = st.Relay
		ci.HasController = st.Controller != nil
	}
	return ci
}

// pollControllerLoop emits the cloud session to the frontend on a gentle 30s
// interval (a desktop poll, per docs/cloud-sync.md §7).
func (a *App) pollControllerLoop(ctx context.Context) {
	emit := func() {
		if ci := a.GetCloudInfo(); ci != nil {
			wruntime.EventsEmit(a.ctx, "cloud", ci)
		} else {
			wruntime.EventsEmit(a.ctx, "cloud", nil)
		}
	}
	emit()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			emit()
		}
	}
}

// PickDeviceFile opens a file dialog for a `.pharosid` device file. Empty string
// = cancelled.
func (a *App) PickDeviceFile() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose your .pharosid device file",
		Filters: []wruntime.FileFilter{
			{DisplayName: "PharosVPN device (*.pharosid)", Pattern: "*.pharosid"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

// SyncFromController fetches + decrypts the account bundle through the relay in
// the device file, stores it (replace-all), and on success stashes the
// passphrase in the keystore (logged in). Mirrors macOS syncFromController.
func (a *App) SyncFromController(deviceFile, email, password string) error {
	a.connecting.Store(true)
	wruntime.EventsEmit(a.ctx, "state", TunnelState{Status: "connecting"})
	defer func() { a.connecting.Store(false); wruntime.EventsEmit(a.ctx, "state", a.GetState()) }()

	name, err := a.runSync(deviceFile, email, password)
	if err != nil {
		return err
	}
	_ = keystore.Store(password) // one-tap re-sync from now on
	_ = name
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	wruntime.EventsEmit(a.ctx, "cloud", a.GetCloudInfo())
	return nil
}

// Enroll redeems a `pharosvpn://enroll?...` join link into a stored, ready
// profile WITHOUT any passphrase — the device key is generated on-device and the
// profile is sealed to it. Mirrors SyncFromController; there is no passphrase to
// stash (re-sync is cert-based off the device leaf).
func (a *App) Enroll(link, deviceName, platform string) error {
	a.connecting.Store(true)
	wruntime.EventsEmit(a.ctx, "state", TunnelState{Status: "connecting"})
	defer func() { a.connecting.Store(false); wruntime.EventsEmit(a.ctx, "state", a.GetState()) }()

	if _, err := a.runEnroll(link, deviceName, platform); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	wruntime.EventsEmit(a.ctx, "cloud", a.GetCloudInfo())
	return nil
}

// SyncNow re-fetches the cloud bundle with the stored passphrase (one tap). With
// none stored it returns ErrNeedsLogin so the frontend opens the login sheet.
var ErrNeedsLogin = errors.New("needs-login")

func (a *App) SyncNow() error {
	bundle := a.cloudBundle()
	if bundle == "" {
		return errors.New("no cloud profile to sync")
	}
	pass, ok := keystore.Read()
	if !ok {
		return ErrNeedsLogin
	}
	deviceFile := uimodel.DevicePath(a.storeDir(), bundle)
	a.connecting.Store(true)
	wruntime.EventsEmit(a.ctx, "state", TunnelState{Status: "connecting"})
	defer func() { a.connecting.Store(false); wruntime.EventsEmit(a.ctx, "state", a.GetState()) }()
	if _, err := a.runSync(deviceFile, "", pass); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	wruntime.EventsEmit(a.ctx, "cloud", a.GetCloudInfo())
	return nil
}

// Logout removes every cloud profile + the stored passphrase, disconnecting
// first if a tunnel is up.
func (a *App) Logout() error {
	if s, ok := worker.ReadState(); ok && s.PID > 0 {
		_ = a.Disconnect()
	}
	if _, err := a.runHelper("logout"); err != nil {
		return err
	}
	_ = keystore.Delete()
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	wruntime.EventsEmit(a.ctx, "cloud", nil)
	return nil
}

// ───────── import / manage ─────────

// ImportProfile opens a file dialog for a `.pharos` file and stores it.
func (a *App) ImportProfile() error {
	src, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Add a .pharos profile",
		Filters: []wruntime.FileFilter{
			{DisplayName: "PharosVPN profile (*.pharos)", Pattern: "*.pharos"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return err
	}
	if src == "" {
		return nil // cancelled
	}
	data, rerr := os.ReadFile(src)
	if rerr != nil {
		return rerr
	}
	name := strings.TrimSuffix(filepath.Base(src), ".pharos")
	if _, err := worker.ImportProfile(name, data); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	return nil
}

// SetDisabled toggles a cloud bundle on/off (the only client action allowed on a
// cloud-synced bundle).
func (a *App) SetDisabled(bundle string, disabled bool) error {
	if err := uimodel.SetDisabled(a.storeDir(), bundle, disabled); err != nil {
		return err
	}
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	return nil
}

// DeleteProfile removes a file-imported bundle (cloud bundles can't be deleted).
func (a *App) DeleteProfile(bundle string) error {
	uimodel.DeleteImported(a.storeDir(), bundle)
	wruntime.EventsEmit(a.ctx, "profiles", a.ListProfiles())
	return nil
}

// ───────── map ─────────

// GetMap returns the map model (pins + arcs) for a selected profile + status.
func (a *App) GetMap(id string, connected bool) uimodel.MapModel {
	info := a.findInfo(id)
	if info == nil {
		return uimodel.MapModel{}
	}
	reachable := false
	if ci := a.GetCloudInfo(); ci != nil {
		reachable = ci.Reachable
	}
	return uimodel.BuildMap(info, reachable, connected)
}

// ───────── helper subprocess plumbing ─────────

// bundledHelperPath finds the pharos-helper shipped beside the app (AppImage
// layout), or on PATH / common dev locations.
func (a *App) bundledHelperPath() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, c := range []string{
			filepath.Join(dir, "pharos-helper"),
			filepath.Join(dir, "..", "lib", "pharosvpn", "pharos-helper"),
		} {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return c
			}
		}
	}
	if v := os.Getenv("PHAROS_HELPER_BIN"); v != "" {
		return v
	}
	if p, err := exec.LookPath("pharos-helper"); err == nil {
		return p
	}
	return "pharos-helper"
}

// runHelper runs the bundled helper (unprivileged) and returns stdout. The helper
// already-installed daemon does the privileged work; these calls don't prompt.
func (a *App) runHelper(args ...string) (string, error) {
	cmd := exec.Command(a.bundledHelperPath(), args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), errors.New(msg)
	}
	return out.String(), nil
}

// runHelperWaiting retries a `ctl connect` while the daemon's socket is not yet
// up (a just-bootstrapped daemon needs a moment to bind). Retrying a connect that
// never reached the daemon is safe — no tunnel was brought up.
func (a *App) runHelperWaiting(args []string) error {
	_, err := a.runHelper(args...)
	for tries := 0; err != nil && tries < 30 && isUnreachable(err); tries++ {
		time.Sleep(400 * time.Millisecond)
		_, err = a.runHelper(args...)
	}
	return err
}

func isUnreachable(err error) bool {
	s := err.Error()
	return strings.Contains(s, "not reachable") || strings.Contains(s, "refused") || strings.Contains(s, "no such file")
}

// runSync runs `pharos-helper sync …`, piping the passphrase on stdin so it never
// lands in the process table. Returns the stored bundle name.
func (a *App) runSync(deviceFile, email, password string) (string, error) {
	args := []string{"sync", deviceFile}
	if email != "" {
		args = append(args, "--email", email)
	}
	args = append(args, "--password-stdin")
	cmd := exec.Command(a.bundledHelperPath(), args...)
	cmd.Stdin = strings.NewReader(password + "\n")
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New("sync failed: " + msg)
	}
	// Worker prints: synced "NAME" (rev N, …) — pull NAME out.
	s := out.String()
	if lo := strings.IndexByte(s, '"'); lo >= 0 {
		if hi := strings.IndexByte(s[lo+1:], '"'); hi >= 0 {
			return s[lo+1 : lo+1+hi], nil
		}
	}
	return "", nil
}

// runEnroll shells out to `pharos-helper enroll <link>`, returning the stored
// profile name. Enrollment needs no passphrase — the join link carries the
// one-time ticket and the device key is generated on-device.
func (a *App) runEnroll(link, deviceName, platform string) (string, error) {
	args := []string{"enroll", link}
	if deviceName != "" {
		args = append(args, "--name", deviceName)
	}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	cmd := exec.Command(a.bundledHelperPath(), args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New("enrollment failed: " + msg)
	}
	// Worker prints: enrolled "NAME" (rev N, …) — pull NAME out.
	s := out.String()
	if lo := strings.IndexByte(s, '"'); lo >= 0 {
		if hi := strings.IndexByte(s[lo+1:], '"'); hi >= 0 {
			return s[lo+1 : lo+1+hi], nil
		}
	}
	return "", nil
}
