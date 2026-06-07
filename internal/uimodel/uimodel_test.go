// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package uimodel

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLocate(t *testing.T) {
	cases := map[string]string{
		"nyc3":  "New York",
		"AMS3":  "Amsterdam",   // case-insensitive
		"eu-nl": "Netherlands", // compound → trailing part
		"fra1":  "Frankfurt",
	}
	for region, wantCity := range cases {
		_, city, ok := locate(region)
		if !ok || city != wantCity {
			t.Errorf("locate(%q) = (%q, %v), want %q", region, city, ok, wantCity)
		}
	}
	if _, _, ok := locate("mars-1"); ok {
		t.Errorf("locate(unknown) should not resolve")
	}
}

func TestGreatCircleEndpoints(t *testing.T) {
	a := Coord{Lat: 40.71, Lon: -74.01} // NYC
	b := Coord{Lat: 52.37, Lon: 4.90}   // AMS
	pts := GreatCircle(a, b, 64)
	if len(pts) != 65 {
		t.Fatalf("GreatCircle returned %d points, want 65", len(pts))
	}
	if !close(pts[0], a) || !close(pts[len(pts)-1], b) {
		t.Errorf("GreatCircle endpoints %v..%v should match %v..%v", pts[0], pts[len(pts)-1], a, b)
	}
}

func close(a, b Coord) bool {
	return math.Abs(a.Lat-b.Lat) < 1e-6 && math.Abs(a.Lon-b.Lon) < 1e-6
}

func TestBuildMapPlanes(t *testing.T) {
	exit := Coord{Lat: 52.37, Lon: 4.90}
	ctl := Coord{Lat: 40.71, Lon: -74.01}
	p := &ProfileInfo{
		Nodes:   []NodeInfo{{Name: "ams", City: "Amsterdam", Coord: &exit, ActiveIP: "1.2.3.4"}},
		Control: &ControlInfo{Label: "Controller · New York", Lat: ctl.Lat, Lon: ctl.Lon},
	}
	m := BuildMap(p, true, false)

	// You + node + controller pins.
	kinds := map[PinKind]int{}
	for _, pin := range m.Pins {
		kinds[pin.Kind]++
	}
	if kinds[PinClient] != 1 || kinds[PinNode] != 1 || kinds[PinController] != 1 {
		t.Errorf("pins = %v, want 1 client + 1 node + 1 controller", kinds)
	}

	// One dashed data arc (You→node) + one solid control arc (You→controller).
	var data, control int
	for _, arc := range m.Arcs {
		switch arc.Style {
		case ArcData:
			data++
		case ArcControl:
			control++
		}
	}
	if data != 1 || control != 1 {
		t.Errorf("arcs = %d data + %d control, want 1 + 1", data, control)
	}
}

func TestPeekPlaintextBundle(t *testing.T) {
	dir := t.TempDir()
	// A minimal enc:none bundle with one named profile + a control endpoint.
	bundle := `{"fmt":"pharos-profile","v":1,"enc":"none","payload":{
		"profiles":[{"name":"home","protocol":"both","nodes":[
			{"name":"ams","region":"ams3","endpoints":["1.2.3.4"],
			 "protocols":[{"type":"amneziawg","params":{"endpoints":[{"ip":"1.2.3.4"}]}}]}]}],
		"control":{"label":"Controller · NYC","city":"New York","lat":40.71,"lon":-74.01}}}`
	if err := os.WriteFile(filepath.Join(dir, "home.pharos"), []byte(bundle), 0o600); err != nil {
		t.Fatal(err)
	}
	got := List(dir)
	if len(got) != 1 {
		t.Fatalf("List = %d profiles, want 1", len(got))
	}
	p := got[0]
	if p.ProfileName != "home" || p.Proto != "both" {
		t.Errorf("profile = %q/%q, want home/both", p.ProfileName, p.Proto)
	}
	if len(p.Nodes) != 1 || p.Nodes[0].City != "Amsterdam" || p.Nodes[0].ActiveIP != "1.2.3.4" {
		t.Errorf("node = %+v, want Amsterdam/1.2.3.4", p.Nodes)
	}
	if p.Control == nil || p.Control.City != "New York" {
		t.Errorf("control = %+v, want New York", p.Control)
	}
}

func TestPeekOpaqueBundle(t *testing.T) {
	dir := t.TempDir()
	// A password-mode bundle is opaque until connect: one placeholder, no nodes.
	bundle := `{"fmt":"pharos-profile","v":1,"enc":"password","payload":"x"}`
	if err := os.WriteFile(filepath.Join(dir, "secret.pharos"), []byte(bundle), 0o600); err != nil {
		t.Fatal(err)
	}
	got := List(dir)
	if len(got) != 1 || got[0].Enc != "password" || len(got[0].Nodes) != 0 {
		t.Errorf("opaque bundle = %+v, want 1 placeholder enc=password", got)
	}
}
