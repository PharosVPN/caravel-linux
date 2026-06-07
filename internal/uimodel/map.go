// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package uimodel

import (
	"math"
	"time"
)

// PinKind classifies a map pin (mirrors macOS PinKind).
type PinKind string

const (
	PinClient     PinKind = "client"
	PinNode       PinKind = "node"
	PinController PinKind = "controller"
)

// ArcStyle follows the platform convention (DESIGN §3): data plane dashed,
// control plane solid.
type ArcStyle string

const (
	ArcData    ArcStyle = "data"
	ArcControl ArcStyle = "control"
)

// MapPin is one point on the map.
type MapPin struct {
	Coord  Coord   `json:"coord"`
	Label  string  `json:"label"`
	Sub    string  `json:"sub,omitempty"`
	Active bool    `json:"active"`
	Kind   PinKind `json:"kind"`
}

// MapArc is one route line: a list of great-circle points + a plane style.
type MapArc struct {
	Points []Coord  `json:"points"`
	Style  ArcStyle `json:"style"`
}

// MapModel is the full map render input the frontend draws.
type MapModel struct {
	Pins []MapPin `json:"pins"`
	Arcs []MapArc `json:"arcs"`
}

// ClientCoord is an offline, no-permission approximation of "you": longitude
// from the host timezone offset, fixed mid-latitude. Mirrors macOS clientCoord.
func ClientCoord() Coord {
	_, offset := time.Now().Zone()
	lon := float64(offset) / 3600.0 * 15.0
	return Coord{Lat: 30, Lon: math.Max(-179, math.Min(179, lon))}
}

// BuildMap computes the pins + arcs for a selected profile and connection state,
// mirroring TunnelController.mapPins / mapArcs. controllerReachable colours the
// controller pin; connected drives the live styling on the frontend.
func BuildMap(p *ProfileInfo, controllerReachable, connected bool) MapModel {
	if p == nil {
		return MapModel{}
	}
	you := ClientCoord()

	// Node / hop coords (data plane).
	var nodePins []MapPin
	var dataCoords []Coord
	if p.Path != nil {
		for _, h := range p.Path.Hops {
			if h.Coord == nil {
				continue
			}
			label := h.City
			if label == "" {
				label = h.Name
			}
			nodePins = append(nodePins, MapPin{Coord: *h.Coord, Label: label, Sub: title(h.Role),
				Active: h.Role == "exit", Kind: PinNode})
			dataCoords = append(dataCoords, *h.Coord)
		}
	} else {
		for _, n := range p.Nodes {
			if n.Coord == nil {
				continue
			}
			label := n.City
			if label == "" {
				label = n.Name
			}
			nodePins = append(nodePins, MapPin{Coord: *n.Coord, Label: label, Sub: n.ActiveIP,
				Active: n.ActiveIP != "", Kind: PinNode})
			dataCoords = append(dataCoords, *n.Coord)
		}
	}

	// Controller (control plane).
	var ctlPins []MapPin
	var ctlCoord *Coord
	if p.Control != nil {
		c := p.Control.AsCoord()
		label := p.Control.City
		if label == "" {
			label = p.Control.Label
		}
		ctlPins = append(ctlPins, MapPin{Coord: c, Label: label, Sub: "Controller",
			Active: controllerReachable, Kind: PinController})
		ctlCoord = &c
	}

	if len(nodePins) == 0 && len(ctlPins) == 0 {
		return MapModel{}
	}

	pins := []MapPin{{Coord: you, Label: "You", Active: connected, Kind: PinClient}}
	pins = append(pins, ctlPins...)
	pins = append(pins, nodePins...)

	// Arcs: data plane (dashed) You → hop chain; control plane (solid) You → controller.
	var arcs []MapArc
	if len(dataCoords) > 0 {
		chain := append([]Coord{you}, dataCoords...)
		for i := 0; i < len(chain)-1; i++ {
			arcs = append(arcs, MapArc{Points: GreatCircle(chain[i], chain[i+1], 64), Style: ArcData})
		}
	}
	if ctlCoord != nil {
		arcs = append(arcs, MapArc{Points: GreatCircle(you, *ctlCoord, 64), Style: ArcControl})
	}
	return MapModel{Pins: pins, Arcs: arcs}
}

// GreatCircle interpolates the shortest path on the sphere (lon/lat in degrees).
// Ported from the macOS LandMap greatCircle so arcs read identically.
func GreatCircle(a, b Coord, steps int) []Coord {
	const d2r = math.Pi / 180
	lat1, lon1 := a.Lat*d2r, a.Lon*d2r
	lat2, lon2 := b.Lat*d2r, b.Lon*d2r
	x1, y1, z1 := math.Cos(lat1)*math.Cos(lon1), math.Cos(lat1)*math.Sin(lon1), math.Sin(lat1)
	x2, y2, z2 := math.Cos(lat2)*math.Cos(lon2), math.Cos(lat2)*math.Sin(lon2), math.Sin(lat2)
	dot := math.Max(-1, math.Min(1, x1*x2+y1*y2+z1*z2))
	omega := math.Acos(dot)
	if omega < 1e-6 {
		return []Coord{a, b}
	}
	sinO := math.Sin(omega)
	out := make([]Coord, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		s1 := math.Sin((1-t)*omega) / sinO
		s2 := math.Sin(t*omega) / sinO
		x, y, z := s1*x1+s2*x2, s1*y1+s2*y2, s1*z1+s2*z2
		out = append(out, Coord{
			Lat: math.Atan2(z, math.Sqrt(x*x+y*y)) / d2r,
			Lon: math.Atan2(y, x) / d2r,
		})
	}
	return out
}

// title capitalizes the first letter of a lowercase ASCII role (entry/mid/exit).
func title(s string) string {
	if s == "" {
		return ""
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}
