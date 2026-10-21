// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package remote reads implementations and definitions on GitHub.
//
// [Open] returns an implementation repository at the head of its default branch, with the entries
// of its configuration file. [Resolve] returns the commit of the definition that an entry follows:
// the head of a branch, or the newest stable release. [Fetch] returns the copy of the definition at
// a commit, which it builds from the archive of the commit as spec sync builds a copy. The pull
// requests, the checks and the status of a sync read through this package, so each of them reads
// an implementation, follows a track and builds a copy in one way.
//
// # Errors
//
// A repository that is no implementation returns an error that wraps [ErrRepository]. A definition
// without the branch that an entry follows returns an error that wraps [ErrTrack].
//
// # Dependency position
//
// Imports context, errors and fmt from the standard library, go.stealthscale.io/spec,
// go.stealthscale.io/spec/definition, go.stealthscale.io/spec/github,
// go.stealthscale.io/spec/implementation and go.stealthscale.io/spec/vendored from this module,
// and go.thesmos.sh/core/errs.
package remote
