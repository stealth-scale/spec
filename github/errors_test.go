// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package github_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/github"
	"go.thesmos.sh/core/errs"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give error
		want errs.Class
	}{
		{name: "ErrGitHub", give: github.ErrGitHub, want: errs.Transient},
		{name: "ErrArchive", give: github.ErrArchive, want: errs.Integrity},
		{name: "ErrNoRelease", give: github.ErrNoRelease, want: errs.NotFound},
		{name: "ErrConfig", give: github.ErrConfig, want: errs.Invalid},
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
