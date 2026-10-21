// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package status reports the copy of a definition in each of its implementations on GitHub.
//
// [Status] returns one [Row] per implementation that the definition registers: the version and the
// commit of the copy on the default branch, whether the commit that the entry follows has a newer
// copy, the open sync pull request, and the state of the check runs of that pull request or of the
// default branch. A newer copy is a copy whose digest differs, so a change of a file of another
// language is no newer copy.
//
// # Errors
//
// A row that cannot be read has its error in [Row.Err], and the other rows are still read. An
// implementation without an entry of the definition has an error that wraps
// implementation.ErrEntry.
//
// # Dependency position
//
// Imports context, fmt and path from the standard library, and go.stealthscale.io/spec,
// go.stealthscale.io/spec/github, go.stealthscale.io/spec/implementation,
// go.stealthscale.io/spec/manifest, go.stealthscale.io/spec/propose,
// go.stealthscale.io/spec/remote and go.stealthscale.io/spec/vendored from this module.
package status
