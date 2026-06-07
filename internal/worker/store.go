// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PharosVPN/caravel/core/deviceid"
	"github.com/PharosVPN/caravel/core/profile"
	csync "github.com/PharosVPN/caravel/core/sync"
)

// ImportProfile stores a .pharos bundle's bytes under name, returning the path.
func ImportProfile(name string, data []byte) (string, error) {
	st, err := OpenStore()
	if err != nil {
		return "", err
	}
	return st.Import(name, data)
}

// PurgeCloudProfiles removes every cloud-synced profile (a bundle with a .synced
// marker) and its sidecars, returning how many it removed. Called before storing
// a fresh sync so "sync is sync" — the synced set is exactly the latest sync.
// Imported profiles (no .synced marker) are never touched. Mirrors the mac
// purgeCloudProfiles (docs/cloud-sync.md §5).
func PurgeCloudProfiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".synced") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".synced")
		for _, suffix := range []string{profile.Extension, ".synced", ".disabled", deviceid.Extension} {
			_ = os.Remove(filepath.Join(dir, name+suffix))
		}
		removed++
	}
	return removed
}

// SyncResult summarizes a completed sync for the caller (CLI / GUI).
type SyncResult struct {
	Name     string `json:"name"`     // stored bundle name
	Path     string `json:"path"`     // on-disk path
	Revision int64  `json:"revision"` // bundle revision
	Replaced int    `json:"replaced"` // previously-synced profiles purged
	Profiles []struct {
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
	} `json:"profiles"`
}

// Sync fetches the account's e2e-encrypted bundle through the relay in the
// `.pharosid`, decrypts it on-device, replaces the prior cloud set, and stores
// it as a connectable, cloud-synced profile. The controller only served
// ciphertext. Mirrors caravel-mac cmdSync (docs/cloud-sync.md §3, §5, §8).
func Sync(ctx context.Context, deviceFile, email, password, name string) (*SyncResult, error) {
	data, err := os.ReadFile(deviceFile)
	if err != nil {
		return nil, err
	}
	bundle, err := deviceid.Parse(data)
	if err != nil {
		return nil, err
	}

	res, err := csync.Fetch(ctx, bundle, email, password)
	if errors.Is(err, csync.ErrNoProfile) {
		return nil, errors.New("signed in, but no profile has been issued for this account yet")
	}
	if err != nil {
		return nil, err
	}

	env, err := profile.WrapPlaintext(res.Plaintext)
	if err != nil {
		return nil, err
	}
	if name == "" {
		if bundle.Alias != "" {
			name = syncProfileName(bundle.Alias)
		} else {
			name = syncProfileName(email)
		}
	}
	st, err := OpenStore()
	if err != nil {
		return nil, err
	}
	// Replace-all: the cloud-synced set is the latest sync only.
	replaced := PurgeCloudProfiles(st.Dir())
	path, err := st.Import(name, env)
	if err != nil {
		return nil, err
	}
	// Mark cloud-synced and stash the device bundle next to it (re-sync needs no
	// re-import). `controller` ties it to this fleet.
	marker, _ := json.Marshal(map[string]any{
		"user": email, "revision": res.Revision,
		"relay": bundle.RelayAddr, "controller": bundle.CAFingerprint,
		"synced_at": time.Now().UTC().Format(time.RFC3339),
	})
	_ = os.WriteFile(filepath.Join(st.Dir(), name+".synced"), marker, 0o600)
	_ = os.WriteFile(filepath.Join(st.Dir(), name+deviceid.Extension), data, 0o600)

	out := &SyncResult{Name: name, Path: path, Revision: res.Revision, Replaced: replaced}
	_ = json.Unmarshal(res.Plaintext, &out)
	return out, nil
}

// syncProfileName derives a stable store name from an account email/alias.
func syncProfileName(email string) string {
	n := email
	if at := strings.IndexByte(n, '@'); at > 0 {
		n = n[:at]
	}
	n = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, n)
	if n == "" {
		n = "account"
	}
	return n
}

// ControllerStatus is the cloud-session liveness for a bundle (docs/cloud-sync.md
// §7): whether the controller's relay is reachable now, when/where it last
// synced, and the controller's map location.
type ControllerStatus struct {
	Reachable    bool                     `json:"reachable"`
	LastSyncedAt string                   `json:"last_synced_at,omitempty"`
	Relay        string                   `json:"relay,omitempty"`
	Controller   *profile.ControlEndpoint `json:"controller,omitempty"`
}

// ControllerStatusFor reports the cloud-sync state for a stored bundle (by name
// or path). It needs no passphrase: the stored bundle is plaintext on-device and
// reachability uses the device leaf in the stashed .pharosid.
func ControllerStatusFor(ref string) (*ControllerStatus, error) {
	st, err := OpenStore()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(ref), profile.Extension)
	out := &ControllerStatus{}

	// Controller map location, from the (plaintext, on-device) bundle.
	if data, derr := LoadProfileBytes(ref); derr == nil {
		if p, perr := profile.Parse(data, profile.Options{}); perr == nil {
			out.Controller = p.Control
		}
	}
	// Last sync + relay, from the .synced sidecar.
	if mb, merr := os.ReadFile(filepath.Join(st.Dir(), name+".synced")); merr == nil {
		var m struct {
			Relay    string `json:"relay"`
			SyncedAt string `json:"synced_at"`
		}
		_ = json.Unmarshal(mb, &m)
		out.Relay, out.LastSyncedAt = m.Relay, m.SyncedAt
	}
	// Live reachability, via the stashed device identity.
	if pid, perr := os.ReadFile(filepath.Join(st.Dir(), name+deviceid.Extension)); perr == nil {
		if b, berr := deviceid.Parse(pid); berr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			out.Reachable = csync.Reachable(ctx, b, 5*time.Second)
		}
	}
	return out, nil
}

// Logout removes every cloud-synced profile, returning the count. The GUI also
// clears the keystore passphrase separately.
func Logout() (int, error) {
	st, err := OpenStore()
	if err != nil {
		return 0, err
	}
	return PurgeCloudProfiles(st.Dir()), nil
}
