// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"regexp"
	"slices"
)

// DigestPrefix starts each digest of a manifest. It names the hash, SHA-256.
const DigestPrefix = "sha256:"

// digestPattern matches a digest: DigestPrefix and 64 lowercase hexadecimal digits.
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Sum returns the digest of data: [DigestPrefix] and the SHA-256 of data in lowercase
// hexadecimal.
func Sum(data []byte) string {
	sum := sha256.Sum256(data)
	return DigestPrefix + hex.EncodeToString(sum[:])
}

// Digest returns the digest of files, which maps each path to the digest of its file: the [Sum] of
// the lines "path digest", each with a line feed at its end, in the order of the bytes of the
// paths. A reader recomputes it from the list without the files. The digest of no file is the Sum
// of no byte.
func Digest(files map[string]string) string {
	var lines []byte
	for _, p := range slices.Sorted(maps.Keys(files)) {
		lines = append(lines, p...)
		lines = append(lines, ' ')
		lines = append(lines, files[p]...)
		lines = append(lines, '\n')
	}
	return Sum(lines)
}
