// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package command is the command-line application of the program spec, on go.stealthscale.io/cli.
//
// [Program.Main] runs one command of a command line. The global flag -C names the directory of the
// repository, and the variables GITHUB_TOKEN, GITHUB_API_URL and GITHUB_GRAPHQL_URL configure the
// access to GitHub.
//
// # Commands
//
//   - manifest writes the manifest of a definition, and checks it with --check.
//   - sync writes the copy of each definition of an implementation.
//   - check checks the copies of an implementation, and their tracks with --remote.
//   - verify checks the report of a runner against the case index of its copy.
//   - propose opens, updates or closes the pull requests that sync implementations.
//   - status reports the copy of a definition in each of its implementations.
//   - try tries the working tree of a definition in the checkouts of its implementations.
//
// # Errors
//
// [Program] states the exit status of each kind of error. The sentinels of the package classify
// the errors of the commands themselves: [ErrUsage], [ErrStale], [ErrLock], [ErrLocal],
// [ErrOffTrack] and [ErrFailed].
//
// # Dependency position
//
// Imports bytes, cmp, context, errors, fmt, io, io/fs, net/http, os, path, path/filepath,
// runtime/debug, slices, strconv, strings, text/tabwriter and time from the standard library, the
// packages
// go.stealthscale.io/spec, conformance, definition, github, implementation, manifest, propose,
// remote, status, trial and vendored from this module, go.dokimi.dev/ergon/service/forge,
// go.stealthscale.io/cli and go.thesmos.sh/core/errs.
package command
