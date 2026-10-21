// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package remote

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrRepository is the error of [Open] for a repository that is not valid, for a repository
	// without its default branch, and for a repository without the configuration file at the head
	// of that branch. It classifies as [errs.Invalid].
	ErrRepository = errs.WithClass(errors.New("remote: the repository is no implementation"), errs.Invalid)

	// ErrTrack is the error of [Resolve] for an entry whose definition lacks the branch that the
	// entry follows. It classifies as [errs.NotFound].
	ErrTrack = errs.WithClass(errors.New("remote: the definition has no branch of the track"), errs.NotFound)
)
