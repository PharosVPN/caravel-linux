// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package uimodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PharosVPN/caravel/core/deviceid"
)

// NodeInfo is one node in a profile: its region (→ an offline map coordinate)
// and its endpoint IP pool, with the dialed one marked active.
type NodeInfo struct {
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	City     string   `json:"city,omitempty"`
	Coord    *Coord   `json:"coord,omitempty"`
	IPs      []string `json:"ips"`
	ActiveIP string   `json:"activeIP,omitempty"`
	Proto    string   `json:"proto,omitempty"`
}

// PathHop is one node in a device's egress chain (entry → [mid] → exit).
type PathHop struct {
	Name   string   `json:"name"`
	Region string   `json:"region"`
	City   string   `json:"city,omitempty"`
	Coord  *Coord   `json:"coord,omitempty"`
	Role   string   `json:"role"`
	IPs    []string `json:"ips"`
}

// PathView is the ordered egress chain a path-bound profile carries.
type PathView struct {
	Name string    `json:"name"`
	Hops []PathHop `json:"hops"`
}

// ControlInfo is the bundle's control-plane endpoint (the controller, via its
// relay) for the map — coordinates embedded by coxswain so it places offline.
type ControlInfo struct {
	Label string  `json:"label"`
	City  string  `json:"city,omitempty"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

// Coord returns the control endpoint as a map coordinate.
func (c ControlInfo) AsCoord() Coord { return Coord{Lat: c.Lat, Lon: c.Lon} }

// ProfileInfo is one named profile the UI can connect with — the rendered form
// of one entry in a bundle's profiles[]. `Bundle` is the store file (connect's
// --profile); `ProfileName` is the entry within it (--name).
type ProfileInfo struct {
	Bundle      string       `json:"bundle"`
	ProfileName string       `json:"profileName"`
	Enc         string       `json:"enc"`
	Proto       string       `json:"proto,omitempty"`
	Nodes       []NodeInfo   `json:"nodes"`
	Path        *PathView    `json:"path,omitempty"`
	Control     *ControlInfo `json:"control,omitempty"`
	CloudSynced bool         `json:"cloudSynced"`
	Disabled    bool         `json:"disabled"`
}

// ID is the stable per-profile identity (bundle + named profile).
func (p ProfileInfo) ID() string { return p.Bundle + "/" + p.ProfileName }

// List expands every `.pharos` bundle in dir into its named profiles, sorted by
// (bundle, name). Mirrors macOS Profiles.list().
func List(dir string) []ProfileInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ProfileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pharos") {
			continue
		}
		out = append(out, Peek(filepath.Join(dir, e.Name()))...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bundle != out[j].Bundle {
			return out[i].Bundle < out[j].Bundle
		}
		return out[i].name() < out[j].name()
	})
	return out
}

func (p ProfileInfo) name() string {
	if p.ProfileName == "" {
		return p.Bundle
	}
	return p.ProfileName
}

// Peek expands one stored bundle into its named profiles. A plaintext bundle
// yields one ProfileInfo per named profile; an opaque (password/account) or
// unreadable bundle yields a single placeholder. Mirrors macOS Profiles.peek().
func Peek(path string) []ProfileInfo {
	bundle := strings.TrimSuffix(filepath.Base(path), ".pharos")
	dir := filepath.Dir(path)
	synced := fileExists(filepath.Join(dir, bundle+".synced"))
	off := fileExists(filepath.Join(dir, bundle+".disabled"))
	opaque := func(enc string) []ProfileInfo {
		return []ProfileInfo{{Bundle: bundle, Enc: enc, CloudSynced: synced, Disabled: off}}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return opaque("?")
	}
	var env struct {
		Fmt     string          `json:"fmt"`
		Enc     string          `json:"enc"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(data, &env) != nil || env.Fmt != "pharos-profile" {
		return opaque("?")
	}
	if env.Enc != "none" {
		return opaque(env.Enc)
	}

	var payload struct {
		Profiles []rawProfile `json:"profiles"`
		Control  *rawControl  `json:"control"`
	}
	if json.Unmarshal(env.Payload, &payload) != nil || len(payload.Profiles) == 0 {
		return opaque(env.Enc)
	}
	control := parseControl(payload.Control)

	out := make([]ProfileInfo, 0, len(payload.Profiles))
	for _, pr := range payload.Profiles {
		nodes := make([]NodeInfo, 0, len(pr.Nodes))
		for _, n := range pr.Nodes {
			ni := NodeInfo{Name: orDefault(n.Name, "node"), Region: n.Region, IPs: endpointIPs(n), Proto: protoLabel(n)}
			if len(ni.IPs) > 0 {
				ni.ActiveIP = ni.IPs[0]
			}
			if c, city, ok := locate(n.Region); ok {
				cc := c
				ni.Coord, ni.City = &cc, city
			}
			nodes = append(nodes, ni)
		}
		out = append(out, ProfileInfo{
			Bundle:      bundle,
			ProfileName: orDefault(pr.Name, "profile"),
			Enc:         env.Enc,
			Proto:       pr.Protocol,
			Nodes:       nodes,
			Path:        parsePath(pr.Path),
			Control:     control,
			CloudSynced: synced,
			Disabled:    off,
		})
	}
	return out
}

// rawProfile / rawNode / rawControl mirror the on-disk JSON shape we peek into.
type rawProfile struct {
	Name     string          `json:"name"`
	Protocol string          `json:"protocol"`
	Nodes    []rawNode       `json:"nodes"`
	Path     json.RawMessage `json:"path"`
}
type rawNode struct {
	Name      string        `json:"name"`
	Region    string        `json:"region"`
	Endpoints []string      `json:"endpoints"`
	Protocols []rawProtocol `json:"protocols"`
}
type rawProtocol struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}
type rawControl struct {
	Label string  `json:"label"`
	City  string  `json:"city"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

func parseControl(c *rawControl) *ControlInfo {
	if c == nil || (c.Lat == 0 && c.Lon == 0) {
		return nil
	}
	label := c.Label
	if label == "" {
		label = "Controller"
	}
	return &ControlInfo{Label: label, City: c.City, Lat: c.Lat, Lon: c.Lon}
}

func parsePath(raw json.RawMessage) *PathView {
	if len(raw) == 0 {
		return nil
	}
	var pj struct {
		Name string `json:"name"`
		Hops []struct {
			Name   string   `json:"name"`
			Region string   `json:"region"`
			Role   string   `json:"role"`
			IPs    []string `json:"ips"`
		} `json:"hops"`
	}
	if json.Unmarshal(raw, &pj) != nil || len(pj.Hops) == 0 {
		return nil
	}
	pv := &PathView{Name: orDefault(pj.Name, "path")}
	for _, h := range pj.Hops {
		hop := PathHop{Name: orDefault(h.Name, "node"), Region: h.Region, Role: h.Role, IPs: h.IPs}
		if c, city, ok := locate(h.Region); ok {
			cc := c
			hop.Coord, hop.City = &cc, city
		}
		pv.Hops = append(pv.Hops, hop)
	}
	return pv
}

// protoLabel lists a node's protocol(s) for display.
func protoLabel(n rawNode) string {
	names := make([]string, 0, len(n.Protocols))
	for _, p := range n.Protocols {
		switch p.Type {
		case "amneziawg":
			names = append(names, "AmneziaWG")
		case "xray-reality", "xray":
			names = append(names, "XRay")
		default:
			if p.Type != "" {
				names = append(names, p.Type)
			}
		}
	}
	return strings.Join(names, ", ")
}

// endpointIPs returns a node's endpoint-pool IPs (decision 17), falling back to
// the node's flat endpoint list.
func endpointIPs(n rawNode) []string {
	for _, p := range n.Protocols {
		if p.Type != "amneziawg" {
			continue
		}
		var params struct {
			Endpoints []struct {
				IP string `json:"ip"`
			} `json:"endpoints"`
		}
		if json.Unmarshal(p.Params, &params) == nil {
			var ips []string
			for _, ep := range params.Endpoints {
				if ep.IP != "" {
					ips = append(ips, ep.IP)
				}
			}
			if len(ips) > 0 {
				return ips
			}
		}
	}
	return n.Endpoints
}

// IsCloudSynced / IsDisabled read the per-bundle sidecar markers.
func IsCloudSynced(dir, bundle string) bool { return fileExists(filepath.Join(dir, bundle+".synced")) }
func IsDisabled(dir, bundle string) bool    { return fileExists(filepath.Join(dir, bundle+".disabled")) }

// SetDisabled toggles a bundle's `.disabled` marker.
func SetDisabled(dir, bundle string, disabled bool) error {
	u := filepath.Join(dir, bundle+".disabled")
	if disabled {
		return os.WriteFile(u, nil, 0o600)
	}
	if err := os.Remove(u); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteImported removes a file-imported bundle + its disabled marker. A
// cloud-synced bundle is never deleted (it would re-sync) — disable it instead.
func DeleteImported(dir, bundle string) {
	if IsCloudSynced(dir, bundle) {
		return
	}
	_ = os.Remove(filepath.Join(dir, bundle+".pharos"))
	_ = os.Remove(filepath.Join(dir, bundle+".disabled"))
}

// DevicePath returns the stashed `.pharosid` path for a cloud bundle (for
// re-sync). Empty string is fine; the caller checks existence.
func DevicePath(dir, bundle string) string {
	return filepath.Join(dir, bundle+deviceid.Extension)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
