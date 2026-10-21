// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/manifest"
)

func TestDigest(t *testing.T) {
	t.Parallel()

	t.Run("Sum", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns the SHA-256 of the empty content",
				give: "",
				want: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			{
				name: "returns the SHA-256 of abc",
				give: "abc",
				want: "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					manifest.Sum([]byte(tt.give)),
					tt.want,
					"Sum returns the prefix and the SHA-256 in hexadecimal",
				)
			})
		}
	})

	t.Run("Digest", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the Sum of the sorted lines of each path and its digest", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				"overlays/rust.json": "sha256:bce1ea5b6029fc4f5936d107fa20bf7304f7a3790555a359455db292545eb0ec",
				"VERSION":            "sha256:59854984853104df5c353e2f681a15fc7924742f9a2e468c29af248dce45ce03",
				"overlays/go.json":   "sha256:4e6841799cb5043f1dd3bdfa51366117d6310ecdacbc013cd44b1c1cddadb01b",
				"corpus/a.json":      "sha256:ca3d163bab055381827226140568f3bef7eaac187cebd76878e0b63e9e442356",
			}
			assert.Equal(
				t,
				manifest.Digest(files),
				"sha256:7a5a550b2b8a607fa3673f8dbed0166d647fb342013be53e0c2d23f34f551433",
				"Digest returns the digest that the script of the definitions computes for the same files",
			)
		})

		t.Run("returns the Sum of the empty content for no file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, manifest.Digest(nil), manifest.Sum(nil), "the digest of no file is the digest of no line")
		})
	})
}
