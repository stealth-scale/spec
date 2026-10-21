// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package github calls the GitHub API for the program spec: it reads the definitions and the
// implementations on GitHub, and opens and closes the pull requests of a sync.
//
// A [Client] embeds the client of ergon's forge, whose methods read the branches, the tags and the
// releases of a repository and commit and open the pull request of a proposal. The methods of
// Client add the reads that the forge lacks: the default branch, the archive and the tree of a
// commit, a file at a ref, the comparison of two commits, the checks of a commit and the newest
// stable release, and the close of a pull request.
//
// # Limits
//
// A response of the JSON API and a file may have up to 10 MiB. An archive may have up to
// [Config.MaxArchiveBytes], as the download and as the tar that it decompresses to, so a
// repository cannot make the program read an unbounded amount of data. An entry of an archive
// outside the directory of the archive fails the archive.
//
// # Errors
//
// A request that fails returns an error that wraps [ErrGitHub]. An archive that breaks a rule
// returns an error that wraps [ErrArchive], and a repository without a stable release an error that
// wraps [ErrNoRelease].
//
// # Dependency position
//
// Imports archive/tar, bytes, cmp, compress/gzip, context, encoding/json, errors, fmt, io, net/http,
// net/url, regexp, slices, strconv and strings from the standard library,
// go.stealthscale.io/spec/manifest from this module, go.dokimi.dev/ergon/service/forge and
// go.thesmos.sh/core/errs.
package github
