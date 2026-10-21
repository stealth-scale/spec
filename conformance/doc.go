// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package conformance checks that a run of a runner accounts for every case of a copy of a
// definition.
//
// The tooling of a definition writes a case index, which the manifest of the definition names:
// every case of the corpus with its file, its id, its languages and the languages that skip it.
// The runner of a language writes a report of a run to the file that [ResultsVariable] names: the
// copy that it read and the outcome of each case. [Verify] then checks the report against the index
// and the manifest of the copy. A case that a runner drops, skips without a declaration or fails
// is a problem of the report, whichever language the runner is written in.
//
// # Formats
//
//   - [Index] is the case index, a JSON object, which [DecodeIndex] decodes.
//   - [Report] is the report of a run, as JSON lines, which [DecodeReport] decodes.
//
// # Errors
//
// An index that breaks a rule returns an error that wraps [ErrIndex], and a report that breaks a
// rule an error that wraps [ErrReport]. A report that does not account for the cases returns a
// [*VerifyError], which wraps [ErrVerify].
//
// # Dependency position
//
// Imports bytes, encoding/json, errors, fmt, io, maps, slices and strings from the standard
// library, go.stealthscale.io/spec and go.stealthscale.io/spec/manifest from this module, and
// go.thesmos.sh/core/errs.
package conformance
