// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/manifest"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrInvalid", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Invalid", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, errs.Classify(manifest.ErrInvalid), errs.Invalid, "an invalid input fails every time")
		})
	})

	t.Run("ErrLanguage", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Invalid", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, errs.Classify(manifest.ErrLanguage), errs.Invalid, "an unknown language fails every time")
		})
	})

	t.Run("ErrCorrupt", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Integrity", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				errs.Classify(manifest.ErrCorrupt),
				errs.Integrity,
				"a corrupt manifest failed verification",
			)
		})
	})
}
