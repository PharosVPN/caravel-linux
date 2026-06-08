// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

// Command pharos-helper is the PharosVPN Linux worker: the privileged tunnel
// daemon, the unprivileged store/sync CLI, and the connectivity test harness.
// The Wails GUI (caravel-linux) shells out to it — exactly the model the macOS
// client uses with caravel-mac.
//
// Subcommands:
//
//	pharos-helper import <file.pharos> [--name NAME]
//	pharos-helper sync <file.pharosid> [--email E] [--password PW|--password-stdin] [--name NAME]
//	pharos-helper list
//	pharos-helper profiles <bundle> [--password PW]
//	pharos-helper controller-status <bundle>
//	pharos-helper logout
//	pharos-helper status
//	pharos-helper install | uninstall            # systemd helper (root)
//	pharos-helper daemon                         # the privileged daemon (root)
//	pharos-helper ctl {connect <bundle> [--name P] [--protocol P] [--password PW] | disconnect | status}
//	sudo pharos-helper connect --profile NAME --name PROFILE [--password PW]   # direct (testing)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/PharosVPN/caravel-linux/internal/worker"
	"github.com/PharosVPN/caravel/core/profile"
	"golang.org/x/term"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pharos-helper:", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("a subcommand is required")
	}
	switch args[0] {
	case "import":
		return cmdImport(args[1:])
	case "sync":
		return cmdSync(args[1:])
	case "enroll":
		return cmdEnroll(args[1:])
	case "list", "ls":
		return cmdList(args[1:])
	case "profiles":
		return cmdProfiles(args[1:])
	case "controller-status":
		return cmdControllerStatus(args[1:])
	case "logout":
		return cmdLogout(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "daemon":
		return runDaemon()
	case "ctl":
		return cmdCtl(args[1:])
	case "connect":
		return cmdConnect(args[1:])
	case "install":
		return worker.Install()
	case "uninstall":
		return worker.Uninstall()
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `pharos-helper — PharosVPN Linux worker

  pharos-helper import <file.pharos> [--name NAME]      store a bundle
  pharos-helper sync <file.pharosid> [--email E] [--password PW] [--name NAME]
                                                        fetch your bundle from the controller
  pharos-helper list                                    list stored bundles
  pharos-helper profiles <bundle> [--password PW]       list a bundle's named profiles
  pharos-helper controller-status <bundle>              cloud-session liveness (JSON)
  pharos-helper logout                                  remove all cloud-synced profiles
  pharos-helper status                                  show whether a tunnel is up
  pharos-helper install | uninstall                     (root) systemd tunnel helper
  pharos-helper ctl {connect <bundle> [--name P] [--protocol P] [--password PW] | disconnect | status}
  sudo pharos-helper connect --profile NAME --name PROFILE [--password PW]   direct (testing)
`)
}

// ───────── store / sync (unprivileged, platform-neutral) ─────────

func cmdImport(args []string) error {
	var src, name string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--name":
			if i+1 >= len(args) {
				return errors.New("--name needs a value")
			}
			name = args[i+1]
			i++
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case !strings.HasPrefix(a, "-") && src == "":
			src = a
		default:
			return fmt.Errorf("unexpected argument %q", a)
		}
	}
	if src == "" {
		return errors.New("usage: pharos-helper import <file.pharos> [--name NAME]")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(src), profile.Extension)
	}
	path, err := worker.ImportProfile(name, data)
	if err != nil {
		return err
	}
	fmt.Printf("imported profile %q → %s\n", name, path)
	return nil
}

func cmdSync(args []string) error {
	var src, name, email, password string
	havePW := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", a)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case a == "--name":
			name, err = val()
		case a == "--email":
			email, err = val()
		case a == "--password":
			password, err = val()
			havePW = true
		case a == "--password-stdin":
			pw, rerr := io.ReadAll(os.Stdin)
			if rerr != nil {
				return fmt.Errorf("read passphrase from stdin: %w", rerr)
			}
			password, havePW = strings.TrimRight(string(pw), "\r\n"), true
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case strings.HasPrefix(a, "--email="):
			email = strings.TrimPrefix(a, "--email=")
		case strings.HasPrefix(a, "--password="):
			password, havePW = strings.TrimPrefix(a, "--password="), true
		case !strings.HasPrefix(a, "-") && src == "":
			src = a
		default:
			return fmt.Errorf("unexpected argument %q", a)
		}
		if err != nil {
			return err
		}
	}
	if src == "" {
		return errors.New("usage: pharos-helper sync <file.pharosid> [--email E] [--password PW] [--name NAME]")
	}
	if !havePW {
		fmt.Fprint(os.Stderr, "account passphrase: ")
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read passphrase: %w", err)
		}
		password = string(pw)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	res, err := worker.Sync(ctx, src, email, password, name)
	if err != nil {
		return err
	}
	if res.Replaced > 0 {
		fmt.Printf("replaced %d previously-synced profile(s)\n", res.Replaced)
	}
	fmt.Printf("synced %q (rev %d, %d profile(s)) → %s\n", res.Name, res.Revision, len(res.Profiles), res.Path)
	for _, pr := range res.Profiles {
		fmt.Printf("  · %s (%s)\n", pr.Name, pr.Protocol)
	}
	return nil
}

// cmdEnroll redeems a `pharosvpn://enroll?...` join link into a stored, ready
// profile WITHOUT any passphrase — the device key is generated on-device and the
// profile is sealed to it. The GUI shells out to this (mirrors `sync`).
func cmdEnroll(args []string) error {
	var link, name, platform string
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", a)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case a == "--name":
			name, err = val()
		case a == "--platform":
			platform, err = val()
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case strings.HasPrefix(a, "--platform="):
			platform = strings.TrimPrefix(a, "--platform=")
		case !strings.HasPrefix(a, "-") && link == "":
			link = a
		default:
			return fmt.Errorf("unexpected argument %q", a)
		}
		if err != nil {
			return err
		}
	}
	if link == "" {
		return errors.New("usage: pharos-helper enroll <pharosvpn://enroll?...> [--name NAME] [--platform P]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	res, err := worker.Enroll(ctx, link, name, platform)
	if err != nil {
		return err
	}
	if res.Replaced > 0 {
		fmt.Printf("replaced %d previously-synced profile(s)\n", res.Replaced)
	}
	fmt.Printf("enrolled %q (rev %d, %d profile(s)) → %s\n", res.Name, res.Revision, len(res.Profiles), res.Path)
	for _, pr := range res.Profiles {
		fmt.Printf("  · %s (%s)\n", pr.Name, pr.Protocol)
	}
	return nil
}

func cmdList(_ []string) error {
	st, err := worker.OpenStore()
	if err != nil {
		return err
	}
	entries, err := st.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("no profiles in %s — import one with `pharos-helper import <file.pharos>`\n", st.Dir())
		return nil
	}
	fmt.Printf("profiles in %s:\n", st.Dir())
	for _, e := range entries {
		fmt.Printf("  %-24s  (%s)\n", e.Name, e.Enc)
	}
	return nil
}

func cmdProfiles(args []string) error {
	fs := flag.NewFlagSet("profiles", flag.ContinueOnError)
	password := fs.String("password", "", "password for a password-mode bundle")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: pharos-helper profiles <bundle-name|path> [--password PW]")
	}
	ref := fs.Arg(0)
	data, err := worker.LoadProfileBytes(ref)
	if err != nil {
		return err
	}
	p, err := profile.Parse(data, profile.Options{Password: *password})
	if errors.Is(err, profile.ErrPasswordNeeded) && *password == "" {
		fmt.Fprintf(os.Stderr, "password for bundle %q: ", ref)
		pw, perr := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if perr != nil {
			return perr
		}
		p, err = profile.Parse(data, profile.Options{Password: strings.TrimSpace(string(pw))})
	}
	if err != nil {
		return err
	}
	if len(p.Profiles) == 0 {
		fmt.Printf("bundle %q carries no profiles\n", ref)
		return nil
	}
	fmt.Printf("profiles in %q:\n", ref)
	for _, cp := range p.Profiles {
		egress := "direct"
		if cp.Path != nil {
			hops := make([]string, len(cp.Path.Hops))
			for i, h := range cp.Path.Hops {
				hops[i] = h.Name
			}
			egress = "cascade " + strings.Join(hops, " → ")
		} else if len(cp.Nodes) > 0 {
			egress = cp.Nodes[0].Name
		}
		fmt.Printf("  %-24s  %-13s  %s\n", cp.Name, cp.Protocol, egress)
	}
	return nil
}

func cmdControllerStatus(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: pharos-helper controller-status <bundle-name|path>")
	}
	out, err := worker.ControllerStatusFor(args[0])
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func cmdLogout(_ []string) error {
	n, err := worker.Logout()
	if err != nil {
		return err
	}
	fmt.Printf("logged out — removed %d cloud profile(s)\n", n)
	return nil
}

func cmdStatus(_ []string) error {
	s, ok := worker.ReadState()
	if !ok {
		fmt.Println("disconnected")
		return nil
	}
	via := ""
	if s.Proto != "" {
		via = " [" + s.Proto + "]"
	}
	fmt.Printf("connected — profile %q%s on %s → %s (since %s)  ↓%s ↑%s\n",
		s.Profile, via, s.Iface, s.Endpoint, s.Since.Format(time.Kitchen),
		worker.HumanBytes(s.RX), worker.HumanBytes(s.TX))
	return nil
}

// cmdCtl drives the daemon over its control socket — no privilege needed.
func cmdCtl(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pharos-helper ctl {connect <bundle> [--name P] [--protocol P] [--password PW] | disconnect | status}")
	}
	var req worker.CtlRequest
	switch args[0] {
	case "connect":
		if len(args) < 2 {
			return errors.New("usage: pharos-helper ctl connect <bundle-path> [--name P] [--protocol P] [--password PW]")
		}
		req.Op = "connect"
		req.Profile = args[1]
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--name":
				if i+1 < len(args) {
					req.Name = args[i+1]
					i++
				}
			case "--protocol":
				if i+1 < len(args) {
					req.Proto = args[i+1]
					i++
				}
			case "--password":
				if i+1 < len(args) {
					req.Password = args[i+1]
					i++
				}
			default:
				if req.Password == "" {
					req.Password = args[i]
				}
			}
		}
	case "disconnect":
		req.Op = "disconnect"
	case "status":
		req.Op = "status"
	default:
		return fmt.Errorf("unknown ctl op %q", args[0])
	}
	resp, err := worker.SendCtl(req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if resp.Status == "connected" {
		fmt.Printf("connected — %s → %s on %s\n", resp.Profile, resp.Endpoint, resp.Iface)
	} else {
		fmt.Println("disconnected")
	}
	return nil
}
