// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package github

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrGitHub is the error of each method of [Client] for a request that GitHub refuses or that
	// fails, and for a response that does not decode. An error of the embedded forge.Client of a
	// method of Client wraps it too. It classifies as [errs.Transient], as the failure of a
	// dependency.
	ErrGitHub = errs.WithClass(errors.New("github: the request failed"), errs.Transient)

	// ErrArchive is the error of [Client.Archive] for an archive that is no gzip archive of tar,
	// that ends early, that is larger than the limit, or that has an entry outside its directory.
	// It classifies as [errs.Integrity].
	ErrArchive = errs.WithClass(errors.New("github: invalid archive"), errs.Integrity)

	// ErrNoRelease is the error of [Client.LatestRelease] for a repository without a stable
	// release, and for a release whose tag the repository lacks. It classifies as [errs.NotFound].
	ErrNoRelease = errs.WithClass(errors.New("github: no stable release"), errs.NotFound)

	// ErrConfig is the error of [New] for an anonymous client whose http.Client has no Transport.
	// It classifies as [errs.Invalid].
	ErrConfig = errs.WithClass(errors.New("github: invalid configuration"), errs.Invalid)
)
