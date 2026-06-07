// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

// Package uimodel turns a stored `.pharos` bundle into the display model the GUI
// renders (named profiles, nodes, egress path, the controller endpoint) and the
// map model (pins + arcs). It is fully offline — no network, no IP geolocation —
// mirroring the macOS Profiles.swift / TunnelController map logic.
package uimodel

import "strings"

// Coord is a plain lat/lon (no MapKit, no network).
type Coord struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// region maps a node region code to a city centroid, entirely offline — the same
// table as the website geo.ts / macOS Regions.swift, so a node lands on its city
// without any IP lookup.
type region struct {
	Coord Coord
	City  string
}

var regionTable = map[string]region{
	// DigitalOcean regions (the platform's provider) + common cities.
	"nyc1": {Coord{40.71, -74.01}, "New York"},
	"nyc2": {Coord{40.71, -74.01}, "New York"},
	"nyc3": {Coord{40.71, -74.01}, "New York"},
	"sfo1": {Coord{37.77, -122.42}, "San Francisco"},
	"sfo2": {Coord{37.77, -122.42}, "San Francisco"},
	"sfo3": {Coord{37.77, -122.42}, "San Francisco"},
	"tor1": {Coord{43.65, -79.38}, "Toronto"},
	"ams2": {Coord{52.37, 4.90}, "Amsterdam"},
	"ams3": {Coord{52.37, 4.90}, "Amsterdam"},
	"lon1": {Coord{51.51, -0.13}, "London"},
	"fra1": {Coord{50.11, 8.68}, "Frankfurt"},
	"sgp1": {Coord{1.35, 103.82}, "Singapore"},
	"blr1": {Coord{12.97, 77.59}, "Bangalore"},
	"syd1": {Coord{-33.87, 151.21}, "Sydney"},
	// AWS (fallback coverage).
	"us-east-1":      {Coord{38.95, -77.45}, "N. Virginia"},
	"us-west-2":      {Coord{45.84, -119.7}, "Oregon"},
	"eu-west-1":      {Coord{53.41, -8.24}, "Ireland"},
	"eu-central-1":   {Coord{50.11, 8.68}, "Frankfurt"},
	"ap-southeast-1": {Coord{1.35, 103.82}, "Singapore"},
	// Bare country / city codes that may appear in a region field.
	"us": {Coord{39.0, -98.0}, "United States"},
	"eu": {Coord{50.0, 9.0}, "Europe"},
	"nl": {Coord{52.37, 4.90}, "Netherlands"},
	"de": {Coord{51.0, 9.0}, "Germany"},
	"gb": {Coord{51.51, -0.13}, "United Kingdom"},
	"uk": {Coord{51.51, -0.13}, "United Kingdom"},
	"sg": {Coord{1.35, 103.82}, "Singapore"},
	"in": {Coord{20.6, 78.96}, "India"},
	"au": {Coord{-33.87, 151.21}, "Australia"},
	"ca": {Coord{43.65, -79.38}, "Canada"},
}

// locate returns the coordinate + display city for a region code, or (zero,
// false) if unknown. A region like "eu-nl" tries the trailing then leading part.
func locate(r string) (Coord, string, bool) {
	r = strings.ToLower(strings.TrimSpace(r))
	if r == "" {
		return Coord{}, "", false
	}
	if hit, ok := regionTable[r]; ok {
		return hit.Coord, hit.City, true
	}
	parts := strings.Split(r, "-")
	for i := len(parts) - 1; i >= 0; i-- {
		if hit, ok := regionTable[parts[i]]; ok {
			return hit.Coord, hit.City, true
		}
	}
	return Coord{}, "", false
}
