// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

// Package keystore persists the account passphrase for the logged-in cloud
// session in the OS secret store — so "Sync now" is one tap and survives a
// restart. On Linux that is the freedesktop Secret Service (GNOME Keyring /
// KWallet, the libsecret backend); go-keyring talks to it over D-Bus. A single
// item (one account, per the "sync is sync" rule). This is the Linux half of the
// cross-platform contract (docs/cloud-sync.md §4) — the macOS Keychain
// counterpart is caravel-mac/app/Caravel/Keychain.swift.
package keystore

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	service = "org.pharosvpn.caravel"
	account = "account-passphrase"
)

// Store saves (or replaces) the passphrase.
func Store(secret string) error {
	return keyring.Set(service, account, secret)
}

// Read returns the stored passphrase, or ("", false) if none / the store is
// unavailable (e.g. no Secret Service running in a headless session).
func Read() (string, bool) {
	s, err := keyring.Get(service, account)
	if err != nil {
		return "", false
	}
	return s, true
}

// Delete clears the stored passphrase (log out). Removing a missing item is not
// an error.
func Delete() error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// HasCredential reports whether a passphrase is stored (the "logged in" flag).
func HasCredential() bool {
	_, ok := Read()
	return ok
}
