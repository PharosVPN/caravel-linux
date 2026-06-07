// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

package worker

import "github.com/PharosVPN/caravel/core/profile"

// ErrPasswordNeeded re-exports the profile package's sentinel so callers (the CLI
// / GUI) can detect a password-mode bundle without importing the core directly.
var ErrPasswordNeeded = profile.ErrPasswordNeeded
