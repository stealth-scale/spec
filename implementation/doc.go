// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package implementation reads the section implements of the configuration file of an
// implementation repository.
//
// Each [Entry] of the section names a definition that the repository vendors, the branch or the
// release of the definition that the copy follows, the directory of the copy, the languages that
// the copy vendors, the files of the copy that the repository also needs at other paths, and the
// program that checks the implementation. [Parse] decodes and checks the section, and [Find]
// returns the entry of one definition.
//
// # Errors
//
// Each section that breaks a rule returns an error that wraps [ErrConfig]. A definition without an
// entry returns an error that wraps [ErrEntry].
//
// # Dependency position
//
// Imports errors, fmt, regexp, slices and strings from the standard library,
// go.stealthscale.io/spec and go.stealthscale.io/spec/manifest from this module,
// go.stealthscale.io/cli, go.stealthscale.io/cli/yaml and go.thesmos.sh/core/errs.
package implementation
