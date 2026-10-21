// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// ErrConfig is the error of [Parse], [Open], [Load] and [Tree.Manifest] for a section definition
// that breaks a rule, and for a definition whose files contradict its section, such as a pattern
// that matches no file or a version file that does not exist. The rule follows the text of the
// sentinel. It classifies as [errs.Invalid].
var ErrConfig = errs.WithClass(errors.New("definition: invalid configuration"), errs.Invalid)
