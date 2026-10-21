// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrUsage is the error of a run whose options exclude each other or select no target. It
	// classifies as [errs.Invalid].
	ErrUsage = errs.WithClass(errors.New("command: invalid usage"), errs.Invalid)

	// ErrStale is the error of the command manifest with --check for a manifest that differs from
	// the files of the definition. It classifies as [errs.Integrity].
	ErrStale = errs.WithClass(errors.New("command: the manifest differs from the definition"), errs.Integrity)

	// ErrLock is the error of the command sync with --locked for a copy without the manifest of a
	// commit of its definition. It classifies as [errs.Integrity].
	ErrLock = errs.WithClass(errors.New("command: the copy has no lock"), errs.Integrity)

	// ErrLocal is the error of the command check for a copy of a checkout. It classifies as
	// [errs.Integrity].
	ErrLocal = errs.WithClass(errors.New("command: the copy comes from a checkout"), errs.Integrity)

	// ErrOffTrack is the error of the command check with --remote for a copy of a commit that is not
	// on the track of its entry. It classifies as [errs.Integrity].
	ErrOffTrack = errs.WithClass(errors.New("command: the copy comes from a commit off its track"), errs.Integrity)

	// ErrFailed is the error of the command try for a checkout whose test failed. It classifies as
	// [errs.Integrity].
	ErrFailed = errs.WithClass(errors.New("command: the test of a checkout failed"), errs.Integrity)
)
