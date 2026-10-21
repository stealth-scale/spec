// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package implementation_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/implementation"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give error
		want errs.Class
	}{
		{name: "ErrConfig", give: implementation.ErrConfig, want: errs.Invalid},
		{name: "ErrEntry", give: implementation.ErrEntry, want: errs.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("classifies as "+tt.want.String(), func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, errs.Classify(tt.give), tt.want, "the sentinel states what a caller does")
			})
		})
	}
}
