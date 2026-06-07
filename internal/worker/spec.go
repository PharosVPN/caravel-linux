// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package worker

import (
	"errors"
	"fmt"

	"github.com/PharosVPN/caravel/core/profile"
	"github.com/PharosVPN/caravel/core/vp"
)

// DialSpec is the unified tunnel input a profile resolves to — the same shape
// the mac worker uses, so both protocols share one connect path.
type DialSpec struct {
	Proto      string        // profile.ProtocolAmneziaWG or profile.ProtocolXRayReality
	Cfg        vp.Config     // AmneziaWG config (Proto == amneziawg)
	XRay       vp.XRayConfig // XRay/REALITY config (Proto == xray-reality)
	Endpoint   string        // server host:port pinned to the physical gateway
	AllowedIPs []string      // CIDRs routed into the tunnel
	Address    string        // bare tunnel IP
	MTU        int
	Label      string // for logs / state
}

// ResolveProfileSpec decrypts a .pharos bundle and resolves one of its named
// profiles to a DialSpec, without prompting (the daemon supplies the password).
// profileName selects which named profile; when empty, proto picks the first of
// that protocol, else the first profile. The chosen profile's own protocol
// drives the tunnel type. Mirrors caravel-mac resolveProfileSpec exactly.
func ResolveProfileSpec(data []byte, profileName, nodeID, password, proto string) (DialSpec, error) {
	p, err := profile.Parse(data, profile.Options{Password: password})
	if err != nil {
		return DialSpec{}, err
	}
	cp, err := chooseProfile(p, profileName, proto)
	if err != nil {
		return DialSpec{}, err
	}
	node, err := cp.Node(nodeID)
	if err != nil {
		return DialSpec{}, err
	}

	// The profile's protocol drives the tunnel. A "both" profile offers both on
	// the entry, so the client picks here: explicit "xray" → REALITY, otherwise
	// AmneziaWG (the auto/default). The cascade beyond the entry is always
	// AmneziaWG; protocol only describes the client↔entry hop.
	useXRay := cp.Protocol == profile.ProtocolXRayReality
	if cp.Protocol == profile.ProtocolBoth {
		useXRay = proto == "xray" || proto == profile.ProtocolXRayReality
	}

	if useXRay {
		xt, err := node.XRayTunnel()
		if err != nil {
			return DialSpec{}, err
		}
		return DialSpec{
			Proto: profile.ProtocolXRayReality,
			XRay: vp.XRayConfig{
				UUID:        xt.UUID,
				Flow:        xt.Flow,
				Endpoint:    xt.Endpoint,
				PublicKey:   xt.PublicKey,
				ServerName:  xt.ServerName,
				ShortID:     xt.ShortID,
				Fingerprint: xt.Fingerprint,
				AllowedIPs:  xt.AllowedIPs,
				MTU:         xt.MTU,
			},
			Endpoint:   xt.Endpoint,
			AllowedIPs: xt.AllowedIPs,
			Address:    xt.Address,
			MTU:        xt.MTU,
			Label:      fmt.Sprintf("%s/%s [xray]", cp.Name, xt.NodeName),
		}, nil
	}

	tun, err := node.Tunnel()
	if err != nil {
		return DialSpec{}, err
	}
	return DialSpec{
		Proto: profile.ProtocolAmneziaWG,
		Cfg: vp.Config{
			PrivateKey:      tun.PrivateKey,
			ServerPublicKey: tun.ServerPublicKey,
			PresharedKey:    tun.PresharedKey,
			Endpoint:        tun.Endpoint,
			AllowedIPs:      tun.AllowedIPs,
			Keepalive:       tun.Keepalive,
			Obfuscation:     toVPObfuscation(tun.Obfuscation),
		},
		Endpoint:   tun.Endpoint,
		AllowedIPs: tun.AllowedIPs,
		Address:    tun.Address,
		MTU:        tun.MTU,
		Label:      fmt.Sprintf("%s/%s", cp.Name, tun.NodeName),
	}, nil
}

// chooseProfile picks which named profile in the bundle to connect with: an
// explicit name wins; otherwise proto selects the first profile of that
// protocol; otherwise the first profile.
func chooseProfile(p *profile.Profile, name, proto string) (*profile.ClientProfile, error) {
	if name != "" {
		return p.Select(name)
	}
	switch proto {
	case "xray", profile.ProtocolXRayReality:
		if cp, err := p.SelectByProtocol(profile.ProtocolXRayReality); err == nil {
			return cp, nil
		}
	case "amneziawg", "awg":
		if cp, err := p.SelectByProtocol(profile.ProtocolAmneziaWG); err == nil {
			return cp, nil
		}
	}
	return p.Select("")
}

// toVPObfuscation maps a profile obfuscation set to the engine's.
func toVPObfuscation(o profile.Obfuscation) vp.Obfuscation {
	return vp.Obfuscation{
		Jc: o.Jc, Jmin: o.Jmin, Jmax: o.Jmax,
		S1: o.S1, S2: o.S2, S3: o.S3, S4: o.S4,
		H1: o.H1, H2: o.H2, H3: o.H3, H4: o.H4,
		I1: o.I1, I2: o.I2, I3: o.I3, I4: o.I4, I5: o.I5,
	}
}

// LoadProfileBytes resolves a profile reference: a readable file path, else a
// stored profile name.
func LoadProfileBytes(ref string) ([]byte, error) {
	if data, err := readFile(ref); err == nil {
		return data, nil
	}
	st, err := OpenStore()
	if err != nil {
		return nil, err
	}
	data, err := st.Raw(ref)
	if errors.Is(err, profile.ErrProfileNotFound) {
		return nil, fmt.Errorf("no profile %q (not a file path, not in %s)", ref, st.Dir())
	}
	return data, err
}
