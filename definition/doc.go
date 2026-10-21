// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package definition reads a definition: the section definition of the configuration file of a
// definition repository, and the files that the section selects.
//
// The section states the repository of the definition, its languages, the patterns of the files
// that an implementation vendors and the patterns that assign a file to one language. [Parse]
// decodes and checks the section. A [Tree] contains the section, the version and the selected
// files of one revision of the definition, and [Tree.Manifest] returns their manifest.
//
// # Sources
//
//   - [Open] reads a checkout on the disk through an [os.Root], so it reads no file outside the
//     repository.
//   - [Load] reads the files of a repository, such as the files of an archive of a commit.
//
// Both read the same tree from the same files: they skip links and the directory .git, select the
// same files and refuse the same configurations.
//
// # Errors
//
// Each configuration that breaks a rule returns an error that wraps [ErrConfig].
//
// # Dependency position
//
// Imports errors, fmt, io/fs, maps, os, path/filepath, slices and strings from the standard library,
// go.stealthscale.io/spec and go.stealthscale.io/spec/manifest from this module,
// github.com/bmatcuk/doublestar/v4, go.stealthscale.io/cli, go.stealthscale.io/cli/yaml and
// go.thesmos.sh/core/errs.
package definition
