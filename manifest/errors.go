// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest

import (
	"errors"

	"go.thesmos.sh/core/errs"
)

// The errors of the package. Each error that a function returns wraps one of them, with the rule
// that the input breaks after the text of the sentinel.
var (
	// ErrInvalid is the error of [New] for a version, a path or a language that a manifest cannot
	// state. It classifies as [errs.Invalid].
	ErrInvalid = errs.WithClass(errors.New("manifest: invalid input"), errs.Invalid)

	// ErrLanguage is the error of [Manifest.View] for a view of no language, and for a language
	// that the manifest does not state. It classifies as [errs.Invalid].
	ErrLanguage = errs.WithClass(errors.New("manifest: unknown language"), errs.Invalid)

	// ErrCorrupt is the error of [Decode] for data that is no manifest, and for a manifest that
	// breaks a rule of a manifest, such as a digest that is not the digest of its files. It
	// classifies as [errs.Integrity].
	ErrCorrupt = errs.WithClass(errors.New("manifest: corrupt manifest"), errs.Integrity)
)
