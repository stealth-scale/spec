// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored

import (
	"errors"
	"strings"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrNotIntact is the error of [Write] and [Check] for a copy whose files do not match its
	// manifest, and of Check for a copy that does not match its entry. A [*NotIntactError] wraps
	// it with the problems of the copy. It classifies as [errs.Integrity].
	ErrNotIntact = errs.WithClass(errors.New("vendored: the copy is not intact"), errs.Integrity)

	// ErrCopy is the error of [Build] for a copy of an entry that names a file outside the view,
	// and for a file of the definition at the path of the manifest of a copy. It classifies as
	// [errs.Invalid].
	ErrCopy = errs.WithClass(errors.New("vendored: invalid copy"), errs.Invalid)
)

// NotIntactError is the error of [Write] and [Check] for a copy with problems: the files that
// differ from the manifest of the copy, that it does not list or that are missing, and the
// differences between the copy and its entry.
//
// The zero value states no directory and no problem.
type NotIntactError struct {
	// Dir is the slash-separated directory of the copy, relative to the root of the repository.
	Dir string

	// Problems are the problems of the copy, each a sentence without a full stop, in the order of
	// the paths of the files and then the checks of the entry.
	Problems []string
}

// Error returns the directory of the copy and its problems after the name of the package, such
// as "vendored: the copy at spec is not intact: a.json is missing".
func (e *NotIntactError) Error() string {
	return "vendored: the copy at " + e.Dir + " is not intact: " + strings.Join(e.Problems, "; ")
}

// Unwrap returns [ErrNotIntact].
func (*NotIntactError) Unwrap() error {
	return ErrNotIntact //nolint:wrapcheck // Unwrap returns the sentinel that the error wraps.
}
