// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/conformance"
)

func TestIndex(t *testing.T) {
	t.Parallel()

	t.Run("DecodeIndex", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every case of the index", func(t *testing.T) {
			t.Parallel()
			got, err := conformance.DecodeIndex([]byte(`{"cases": [
  {"file": "corpus/equal.json", "id": "equal/one"},
  {"file": "corpus/equal.json", "id": "equal/two", "skip": {"go": "the type refuses it"}},
  {"file": "spec/corpus/x/case.json", "id": "x", "languages": ["rust"]}
]}`))
			assert.NoError(t, err, "DecodeIndex accepts an index")
			assert.Equal(t, got, &conformance.Index{Cases: []conformance.Case{
				{File: "corpus/equal.json", ID: "equal/one"},
				{
					File: "corpus/equal.json",
					ID:   "equal/two",
					Skip: map[spec.Language]string{"go": "the type refuses it"},
				},
				{File: "spec/corpus/x/case.json", ID: "x", Languages: []spec.Language{"rust"}},
			}}, "the index states every case")
		})

		t.Run("returns one id of two files", func(t *testing.T) {
			t.Parallel()
			got, err := conformance.DecodeIndex([]byte(`{"cases": [
  {"file": "corpus/a.json", "id": "one"},
  {"file": "corpus/b.json", "id": "one"}
]}`))
			assert.NoError(t, err, "DecodeIndex accepts one id in two files")
			assert.Length(t, got.Cases, 2, "the index states both cases")
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "returns ErrIndex for data that is no JSON", give: "cases"},
			{name: "returns ErrIndex for data after the object", give: `{"cases": []} {}`},
			{name: "returns ErrIndex for a key that an index does not have", give: `{"cases": [], "count": 0}`},
			{name: "returns ErrIndex for an index without cases", give: `{}`},
			{
				name: "returns ErrIndex for a key that a case does not have",
				give: `{"cases": [{"file": "a", "id": "b", "name": "c"}]}`,
			},
			{
				name: "returns ErrIndex for a case whose file leaves the definition",
				give: `{"cases": [{"file": "../a", "id": "b"}]}`,
			},
			{name: "returns ErrIndex for a case without an id", give: `{"cases": [{"file": "a"}]}`},
			{
				name: "returns ErrIndex for a case that the index lists twice",
				give: `{"cases": [{"file": "a", "id": "b"}, {"file": "a", "id": "b"}]}`,
			},
			{
				name: "returns ErrIndex for a language of a case that is not valid",
				give: `{"cases": [{"file": "a", "id": "b", "languages": ["Go"]}]}`,
			},
			{
				name: "returns ErrIndex for the language all of a case",
				give: `{"cases": [{"file": "a", "id": "b", "languages": ["all"]}]}`,
			},
			{
				name: "returns ErrIndex for a language that a case lists twice",
				give: `{"cases": [{"file": "a", "id": "b", "languages": ["go", "go"]}]}`,
			},
			{
				name: "returns ErrIndex for a skip of a language that is not valid",
				give: `{"cases": [{"file": "a", "id": "b", "skip": {"Go": "reason"}}]}`,
			},
			{
				name: "returns ErrIndex for a skip without a reason",
				give: `{"cases": [{"file": "a", "id": "b", "skip": {"go": ""}}]}`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := conformance.DecodeIndex([]byte(tt.give))
				assert.ErrorIs(t, err, conformance.ErrIndex, "DecodeIndex refuses an index that breaks a rule")
			})
		}
	})
}
