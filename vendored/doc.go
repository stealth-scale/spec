// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package vendored builds, writes and checks the vendored copy of a definition in an
// implementation repository.
//
// A copy is the view of the languages of an entry of the section implements. It contains every
// shared file of the definition and the files of those languages. Each file of the copy is at its
// path in the definition, below the path of the entry. The copy has its own manifest at
// [ManifestFile] below its directory. That manifest is the manifest of the view, with the
// repository of the definition and the commit that the copy comes from. It is the lock of the copy.
// A change of a file of another language leaves the copy as it is, because the view does not
// contain the file.
//
// # Entries
//
// A copy owns the entries of its directory that its files start with, and [ManifestFile], as
// [Entries] returns them. A copy of VERSION, corpus/a.json and spec/zones.json owns the entries
// VERSION, corpus, manifest.json and spec. Every other entry of the directory belongs to the
// repository, such as the source files of the package that reads the copy. The operations read,
// write and move the entries of a copy alone.
//
// # Operations
//
//   - [Build] returns the [Copy] of a definition tree for an entry.
//   - [Write] writes a copy, and refuses to replace a copy that differs from its manifest.
//   - [Overwrite] writes a copy, whatever the entries of its directory contain.
//   - [Check] checks a copy against its manifest and its entry.
//   - [Stash] moves a copy aside, and [Unstash] moves it back, so a trial of another copy can end
//     with the copy from before the trial.
//
// Write and Overwrite write the new copy beside the old one and move each of its entries into
// place, so a reader finds the old or the new version of each entry. An entry or a file that the new
// copy drops does not remain.
//
// # Disk
//
// The operations read and write through a [Disk]. [*os.Root] implements Disk, so the operations
// never read or write a file outside the repository. A path of a [Copy] is slash-separated. The
// operations convert it to the separators of the system.
//
// # Errors
//
// A copy with problems returns a [*NotIntactError], which wraps [ErrNotIntact]. A copy that cannot
// be built returns an error that wraps [ErrCopy].
//
// # Dependency position
//
// Imports errors, fmt, io/fs, maps, os, path, path/filepath, slices and strings from the standard
// library, go.stealthscale.io/spec, go.stealthscale.io/spec/definition,
// go.stealthscale.io/spec/implementation and go.stealthscale.io/spec/manifest from this module,
// and go.thesmos.sh/core/errs.
package vendored
