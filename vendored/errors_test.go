// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/vendored"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrNotIntact", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Integrity", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, errs.Classify(vendored.ErrNotIntact), errs.Integrity, "a copy failed verification")
		})
	})

	t.Run("ErrCopy", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Invalid", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				errs.Classify(vendored.ErrCopy),
				errs.Invalid,
				"a copy that cannot be built fails every time",
			)
		})
	})

	t.Run("NotIntactError", func(t *testing.T) {
		t.Parallel()

		problems := &vendored.NotIntactError{Dir: "spec", Problems: []string{"a.json is missing", "b.json is missing"}}

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the directory and every problem", func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					problems.Error(),
					"vendored: the copy at spec is not intact: a.json is missing; b.json is missing",
					"the text states the directory and the problems",
				)
			})
		})

		t.Run("Unwrap", func(t *testing.T) {
			t.Parallel()

			t.Run("returns ErrNotIntact", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, problems.Unwrap(), vendored.ErrNotIntact, "the error wraps its sentinel")
			})
		})

		t.Run("classifies as Integrity", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, errs.Classify(problems), errs.Integrity, "the error classifies as its sentinel")
		})
	})
}
