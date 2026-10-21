// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package propose opens, updates and closes the pull requests that sync the vendored copies of an
// implementation repository on GitHub.
//
// [Propose] needs no checkout. It reads the implementation and builds the copy of each entry with
// go.stealthscale.io/spec/remote, and compares the blobs of the copy with the blobs of the default
// branch. A definition has one pull request in each implementation, from the branch [BranchPrefix]
// and the repository of the definition. release.Propose of ergon commits the files of the pull
// request on a staging branch, so an update of the pull request does not close it.
//
// # Actions
//
//   - [Current]: the copy matches the definition, and no pull request is open.
//   - [Closed]: the copy matches the definition, and Propose closed the open pull request.
//   - [Proposed]: the copy differs, and Propose opened or updated the pull request.
//
// # Errors
//
// Propose returns the errors of go.stealthscale.io/spec/remote, such as an error that wraps
// remote.ErrRepository for a repository that is no implementation.
//
// # Dependency position
//
// Imports context, crypto/sha1, encoding/hex, fmt, maps, path, slices, strconv and strings from the
// standard library, go.stealthscale.io/spec, go.stealthscale.io/spec/github,
// go.stealthscale.io/spec/implementation, go.stealthscale.io/spec/manifest,
// go.stealthscale.io/spec/remote and go.stealthscale.io/spec/vendored from this module, and
// go.dokimi.dev/ergon/service/release.
package propose
