// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package spec_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
)

func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("ConfigFile", func(t *testing.T) {
		t.Parallel()

		t.Run("is .spec.yaml", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, spec.ConfigFile, ".spec.yaml", "the configuration file keeps its persisted name")
		})
	})

	t.Run("Language", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give spec.Language
				want bool
			}{
				{name: "reports true for one word", give: "go", want: true},
				{name: "reports true for words that a dash joins", give: "objective-c", want: true},
				{name: "reports true for a digit after the first letter", give: "c99", want: true},
				{name: "reports true for all", give: spec.All, want: true},
				{name: "reports false for the empty name", give: "", want: false},
				{name: "reports false for an upper-case letter", give: "Go", want: false},
				{name: "reports false for a leading digit", give: "9c", want: false},
				{name: "reports false for a leading dash", give: "-go", want: false},
				{name: "reports false for a trailing dash", give: "go-", want: false},
				{name: "reports false for two dashes in a row", give: "objective--c", want: false},
				{name: "reports false for a space", give: "type script", want: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					assert.Equal(
						t,
						tt.give.Valid(),
						tt.want,
						"Valid reports whether the name follows the rule of a language",
					)
				})
			}
		})
	})
}
