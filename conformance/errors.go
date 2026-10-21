// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"errors"
	"strings"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrIndex is the error of [DecodeIndex] for data that is no case index, and for an index that
	// breaks a rule. It classifies as [errs.Integrity].
	ErrIndex = errs.WithClass(errors.New("conformance: invalid case index"), errs.Integrity)

	// ErrReport is the error of [DecodeReport] for data that is no report, and for a report that
	// breaks a rule. It classifies as [errs.Integrity].
	ErrReport = errs.WithClass(errors.New("conformance: invalid report"), errs.Integrity)

	// ErrVerify is the error of [Verify] for a report that does not account for the cases of a
	// copy. A [*VerifyError] wraps it with the problems of the report. It classifies as
	// [errs.Integrity].
	ErrVerify = errs.WithClass(errors.New("conformance: the report does not account for the cases"), errs.Integrity)
)

// VerifyError is the error of [Verify] for a report with problems: a header of another copy, a
// case without a result or with two, a result of no case of the copy, a case that fails, and a
// skip that the index does not declare.
//
// The zero value states no problem.
type VerifyError struct {
	// Problems are the problems of the report, each a sentence without a full stop, in the order of
	// the header, the results and then the cases of the index.
	Problems []string
}

// Error returns the problems after the name of the package, such as "conformance: the report does
// not account for the cases: a of corpus/a.json has no result".
func (e *VerifyError) Error() string {
	return "conformance: the report does not account for the cases: " + strings.Join(e.Problems, "; ")
}

// Unwrap returns [ErrVerify].
func (*VerifyError) Unwrap() error {
	return ErrVerify //nolint:wrapcheck // Unwrap returns the sentinel that the error wraps.
}
