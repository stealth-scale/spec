// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package implementation

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// The sentinels of the package.
var (
	// ErrConfig is the error of [Parse] for a section implements that breaks a rule. The rule
	// follows the text of the sentinel. It classifies as [errs.Invalid].
	ErrConfig = errs.WithClass(errors.New("implementation: invalid configuration"), errs.Invalid)

	// ErrEntry is the error of [Find] for entries without an entry of the definition. It classifies
	// as [errs.NotFound].
	ErrEntry = errs.WithClass(errors.New("implementation: no entry of the definition"), errs.NotFound)
)
