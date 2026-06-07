// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

// Command caravel-linux is the PharosVPN desktop client for Linux — a Wails
// (Go + web) app matching the macOS client. It imports `.pharos` profiles, syncs
// an account from the controller, and connects an AmneziaWG or XRay/REALITY
// tunnel, drawing the signature live map. The privileged tunnel work runs in a
// separate root helper (pharos-helper) the GUI drives over a control socket.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "PharosVPN",
		Width:     1320,
		Height:    860,
		MinWidth:  1040,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Maroon window chrome, matching the brand (#5A1F2B).
		BackgroundColour: &options.RGBA{R: 0x12, G: 0x16, B: 0x18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Linux: &linux.Options{
			Icon:                icon,
			ProgramName:         "PharosVPN",
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: false,
		},
	})
	if err != nil {
		println("error:", err.Error())
	}
}
