// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PharosVPN/caravel-linux/internal/worker"
	"golang.org/x/term"
)

func runDaemon() error { return worker.RunDaemon() }

// cmdConnect brings a tunnel up directly in this process (no daemon) — the
// foreground test path. Runs until Ctrl-C. Must be root.
func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	profileRef := fs.String("profile", "", "a stored bundle name, or a path to a .pharos file")
	name := fs.String("name", "", "which named profile in the bundle to connect with")
	password := fs.String("password", "", "password for a password-mode profile (prompted if omitted)")
	nodeID := fs.String("node", "", "which node in the profile to use (default: the first)")
	proto := fs.String("protocol", "auto", "data-plane protocol when no --name: auto|amneziawg|xray")
	fullTunnel := fs.Bool("full-tunnel", true, "route all traffic through the tunnel")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profileRef == "" {
		return errors.New("give --profile NAME|PATH")
	}

	data, err := worker.LoadProfileBytes(*profileRef)
	if err != nil {
		return err
	}
	spec, err := worker.ResolveProfileSpec(data, *name, *nodeID, *password, *proto)
	if errors.Is(err, worker.ErrPasswordNeeded) && *password == "" {
		fmt.Fprintf(os.Stderr, "password for profile %q: ", *profileRef)
		pw, perr := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if perr != nil {
			return perr
		}
		spec, err = worker.ResolveProfileSpec(data, *name, *nodeID, string(pw), *proto)
	}
	if err != nil {
		return err
	}

	if os.Geteuid() != 0 {
		return errors.New("must run as root (TUN + routes) — re-run with sudo")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tn, err := worker.Connect(spec, *fullTunnel)
	if err != nil {
		return err
	}
	defer tn.Close()

	since := time.Now()
	write := func() {
		rx, tx := tn.Stats()
		_ = worker.WriteState(worker.State{Profile: spec.Label, Proto: spec.Proto, Iface: tn.Iface(),
			Endpoint: spec.Endpoint, PID: os.Getpid(), Since: since, RX: rx, TX: tx})
	}
	write()
	defer worker.ClearState()
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				write()
			}
		}
	}()

	fmt.Printf("pharos-helper: tunnel up on %s → %s (%s, full-tunnel=%v). Ctrl-C to disconnect.\n",
		tn.Iface(), spec.Endpoint, spec.Label, *fullTunnel)
	<-ctx.Done()
	fmt.Println("\npharos-helper: disconnecting")
	return nil
}
