// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package spec_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
)

func TestRepo(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give spec.Repo
			want bool
		}{
			{name: "reports true for owner/name", give: "dokimasia/assert-spec", want: true},
			{name: "reports true for a name with a dot and an underscore", give: "a/b.c_d", want: true},
			{name: "reports true for a name that starts with a dot", give: "a/.github", want: true},
			{
				name: "reports true for an owner of 39 characters",
				give: spec.Repo(strings.Repeat("a", 39) + "/b"),
				want: true,
			},
			{
				name: "reports true for a name of 100 characters",
				give: spec.Repo("a/" + strings.Repeat("b", 100)),
				want: true,
			},
			{name: "reports false for the empty repository", give: "", want: false},
			{name: "reports false for a repository without a slash", give: "assert-spec", want: false},
			{
				name: "reports false for an owner of 40 characters",
				give: spec.Repo(strings.Repeat("a", 40) + "/b"),
				want: false,
			},
			{
				name: "reports false for a name of 101 characters",
				give: spec.Repo("a/" + strings.Repeat("b", 101)),
				want: false,
			},
			{name: "reports false for an owner that starts with a dash", give: "-a/b", want: false},
			{name: "reports false for an owner with an underscore", give: "a_b/c", want: false},
			{name: "reports false for the name .", give: "a/.", want: false},
			{name: "reports false for the name ..", give: "a/..", want: false},
			{name: "reports false for a second slash", give: "a/b/c", want: false},
			{name: "reports false for an empty name", give: "a/", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether the repository is owner/name")
			})
		}
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give spec.Repo
			want string
		}{
			{name: "returns the name after the slash", give: "dokimasia/assert-spec", want: "assert-spec"},
			{name: "returns the repository without a slash", give: "assert-spec", want: "assert-spec"},
			{name: "returns the rest after the first slash", give: "a/b/c", want: "b/c"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					tt.give.Name(),
					tt.want,
					"Name returns the part of the repository after its first slash",
				)
			})
		}
	})
}
