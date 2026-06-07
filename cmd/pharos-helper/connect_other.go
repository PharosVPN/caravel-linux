// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build !linux

package main

import "errors"

// The tunnel + daemon are Linux-only; these stubs let the CLI/GUI compile-check
// on other platforms (e.g. cross-checking the Wails bindings on a Mac dev box).
func runDaemon() error            { return errors.New("the tunnel daemon runs on Linux only") }
func cmdConnect(_ []string) error { return errors.New("connect runs on Linux only") }
