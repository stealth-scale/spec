// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package manifest encodes, decodes and checks the manifest of a definition and of a copy of a
// definition.
//
// A manifest lists files by their slash-separated paths, each with its digest. [Sum] computes the
// digest of a file, and [Digest] the digest of a list of files from the sorted lines "path digest",
// so a reader recomputes it without the files. The manifest of a definition also states the
// version of the definition and the files of each language. [Manifest.View] returns the manifest
// of the files that some languages vendor, and the manifest of a copy adds the source of the copy.
//
// # Construction
//
//   - [New] returns the manifest of the content of each file of a definition.
//   - [Manifest.View] returns the manifest of the shared files and the files of some languages.
//   - [Decode] returns the manifest that [Manifest.Encode] writes, and checks every rule of a
//     manifest.
//
// # Encoding
//
// [Manifest.Encode] writes a JSON object with sorted keys and an indent of four spaces, which is
// the JSON that the manifest scripts of the definitions write with the json module of Python. Two
// tools that encode one manifest write the same bytes, so the digest of a manifest does not depend
// on the tool that wrote it.
//
// # Paths
//
// A manifest lists only paths that Linux, macOS and Windows can each store. [PortablePath] refuses
// a path that leaves the root, a backslash, and a name that Windows refuses, such as con.json or a
// name that ends with a dot. [New] and [Decode] also refuse two paths that differ only in case,
// because Windows and macOS store them as one file. [Fold] returns the form in which those systems
// compare two paths. [ValidPath] checks only that a path is below the root.
//
// # Errors
//
// Each error of the package wraps one of [ErrInvalid], [ErrLanguage] and [ErrCorrupt].
//
// # Dependency position
//
// Imports bytes, crypto/sha256, encoding/hex, encoding/json, errors, fmt, io, io/fs, maps, path,
// regexp, slices, strings and unicode from the standard library, go.stealthscale.io/spec from this
// module, and go.thesmos.sh/core/errs.
package manifest
