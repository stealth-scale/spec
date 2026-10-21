// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/definition"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrConfig", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies as Invalid", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				errs.Classify(definition.ErrConfig),
				errs.Invalid,
				"a broken configuration fails every time",
			)
		})
	})
}
