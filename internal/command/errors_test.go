// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/internal/command"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give error
		want errs.Class
	}{
		{name: "ErrUsage", give: command.ErrUsage, want: errs.Invalid},
		{name: "ErrStale", give: command.ErrStale, want: errs.Integrity},
		{name: "ErrLock", give: command.ErrLock, want: errs.Integrity},
		{name: "ErrLocal", give: command.ErrLocal, want: errs.Integrity},
		{name: "ErrOffTrack", give: command.ErrOffTrack, want: errs.Integrity},
		{name: "ErrFailed", give: command.ErrFailed, want: errs.Integrity},
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
