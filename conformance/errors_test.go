// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/conformance"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give error
	}{
		{name: "ErrIndex", give: conformance.ErrIndex},
		{name: "ErrReport", give: conformance.ErrReport},
		{name: "ErrVerify", give: conformance.ErrVerify},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("classifies as Integrity", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, errs.Classify(tt.give), errs.Integrity, "the data failed verification")
			})
		})
	}

	t.Run("VerifyError", func(t *testing.T) {
		t.Parallel()

		problems := &conformance.VerifyError{Problems: []string{"a has no result", "b fails"}}

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns every problem", func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					problems.Error(),
					"conformance: the report does not account for the cases: a has no result; b fails",
					"the text states the problems",
				)
			})
		})

		t.Run("Unwrap", func(t *testing.T) {
			t.Parallel()

			t.Run("returns ErrVerify", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, problems.Unwrap(), conformance.ErrVerify, "the error wraps its sentinel")
			})
		})
	})
}
