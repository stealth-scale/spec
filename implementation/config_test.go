// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package implementation_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/implementation"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every key of each entry", func(t *testing.T) {
			t.Parallel()
			entries, err := implementation.Parse([]byte(`
implements:
  - definition: dokimasia/assert-spec
    track: release
    path: conformance/spec
    languages: [go, rust]
    copies:
      - from: spec/zones.json
        to: internal/prop/zone/zones.json
    test: [make, test-go]
  - definition: dokimasia/mutate-spec
    path: conformance/mutate
    languages: [all]
`))
			assert.NoError(t, err, "Parse accepts a section with every key")
			assert.Equal(t, entries, []implementation.Entry{
				{
					Definition: "dokimasia/assert-spec",
					Track:      implementation.Release,
					Path:       "conformance/spec",
					Languages:  []spec.Language{"go", "rust"},
					Copies:     []implementation.Copy{{From: "spec/zones.json", To: "internal/prop/zone/zones.json"}},
					Test:       []string{"make", "test-go"},
				},
				{
					Definition: "dokimasia/mutate-spec",
					Track:      implementation.DefaultTrack,
					Path:       "conformance/mutate",
					Languages:  []spec.Language{spec.All},
				},
			}, "the entries state every key of the section")
		})

		t.Run("ignores the section definition", func(t *testing.T) {
			t.Parallel()
			_, err := implementation.Parse([]byte(`
definition:
  repo: a/b
implements:
  - {definition: a/b, path: spec, languages: [go]}
`))
			assert.NoError(t, err, "Parse decodes the section implements alone")
		})

		tests := []struct {
			name     string
			give     string
			wantText string
		}{
			{name: "returns ErrConfig for data that is no YAML", give: "implements: [\n", wantText: "yaml: decode"},
			{
				name:     "returns ErrConfig for a file without the section implements",
				give:     "definition: {}\n",
				wantText: "has no section implements",
			},
			{
				name:     "returns ErrConfig for a section that is null",
				give:     "implements:\n",
				wantText: "has no section implements",
			},
			{
				name:     "returns ErrConfig for a section without entries",
				give:     "implements: []\n",
				wantText: "the section implements lists no definition",
			},
			{
				name:     "returns ErrConfig for a section that is no list",
				give:     "implements: {definition: a/b}\n",
				wantText: "invalid implements",
			},
			{
				name:     "returns ErrConfig for a key that an entry does not have",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], paths: [a]}]\n",
				wantText: "unknown key implements[0].paths",
			},
			{
				name:     "returns ErrConfig for an entry without a definition",
				give:     "implements: [{path: spec, languages: [go]}]\n",
				wantText: "missing implements[0].definition",
			},
			{
				name:     "returns ErrConfig for a definition that is not valid",
				give:     "implements: [{definition: assert-spec, path: spec, languages: [go]}]\n",
				wantText: `"assert-spec" is no repository of GitHub`,
			},
			{
				name:     "returns ErrConfig for an empty track",
				give:     "implements: [{definition: a/b, track: '', path: spec, languages: [go]}]\n",
				wantText: `the track "" of a/b is no name of a branch`,
			},
			{
				name:     "returns ErrConfig for a track with a space",
				give:     "implements: [{definition: a/b, track: 'main line', path: spec, languages: [go]}]\n",
				wantText: `the track "main line" of a/b is no name of a branch`,
			},
			{
				name:     "returns ErrConfig for a track with two dots in a row",
				give:     "implements: [{definition: a/b, track: 'a..b', path: spec, languages: [go]}]\n",
				wantText: `the track "a..b" of a/b is no name of a branch`,
			},
			{
				name:     "returns ErrConfig for a track that starts with a dash",
				give:     "implements: [{definition: a/b, track: '-main', path: spec, languages: [go]}]\n",
				wantText: `the track "-main" of a/b is no name of a branch`,
			},
			{
				name:     "returns ErrConfig for a track that ends with .lock",
				give:     "implements: [{definition: a/b, track: main.lock, path: spec, languages: [go]}]\n",
				wantText: `the track "main.lock" of a/b is no name of a branch`,
			},
			{
				name:     "returns ErrConfig for an entry without a path",
				give:     "implements: [{definition: a/b, languages: [go]}]\n",
				wantText: "missing implements[0].path",
			},
			{
				name:     "returns ErrConfig for the root as the path",
				give:     "implements: [{definition: a/b, path: ., languages: [go]}]\n",
				wantText: `the path "." of a/b is no portable directory below the root`,
			},
			{
				name:     "returns ErrConfig for a path that leaves the repository",
				give:     "implements: [{definition: a/b, path: ../spec, languages: [go]}]\n",
				wantText: `the path "../spec" of a/b is no portable directory below the root`,
			},
			{
				name:     "returns ErrConfig for a path that Windows cannot store",
				give:     "implements: [{definition: a/b, path: conformance/con, languages: [go]}]\n",
				wantText: `the path "conformance/con" of a/b is no portable directory below the root`,
			},
			{
				name:     "returns ErrConfig for an entry without languages",
				give:     "implements: [{definition: a/b, path: spec}]\n",
				wantText: "missing implements[0].languages",
			},
			{
				name:     "returns ErrConfig for an empty list of languages",
				give:     "implements: [{definition: a/b, path: spec, languages: []}]\n",
				wantText: "a/b vendors no language",
			},
			{
				name:     "returns ErrConfig for a language that is not valid",
				give:     "implements: [{definition: a/b, path: spec, languages: [Go]}]\n",
				wantText: "the languages [Go] of a/b",
			},
			{
				name:     "returns ErrConfig for all beside another language",
				give:     "implements: [{definition: a/b, path: spec, languages: [all, go]}]\n",
				wantText: "the languages [all go] of a/b",
			},
			{
				name:     "returns ErrConfig for a language that the entry lists twice",
				give:     "implements: [{definition: a/b, path: spec, languages: [go, go]}]\n",
				wantText: "the languages [go go] of a/b",
			},
			{
				name:     "returns ErrConfig for a copy from a path that leaves the definition",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: ../a, to: b}]}]\n",
				wantText: `the copy of "../a" to "b" has a path that is not portable`,
			},
			{
				name:     "returns ErrConfig for a copy to a path that leaves the repository",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: ../b}]}]\n",
				wantText: `the copy of "a" to "../b" has a path that is not portable`,
			},
			{
				name:     "returns ErrConfig for a copy to a path that Windows cannot store",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: 'b:c'}]}]\n",
				wantText: `the copy of "a" to "b:c" has a path that is not portable`,
			},
			{
				name:     "returns ErrConfig for a copy into the path of the copy",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: spec/b}]}]\n",
				wantText: "the copy to spec/b writes into the copy at spec",
			},
			{
				name:     "returns ErrConfig for a copy onto the path of the copy",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: spec}]}]\n",
				wantText: "the copy to spec writes into the copy at spec",
			},
			{
				name:     "returns ErrConfig for a test with an empty program",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], test: ['']}]\n",
				wantText: "the test of a/b names no program",
			},
			{
				name:     "returns ErrConfig for two entries of one definition",
				give:     "implements: [{definition: a/b, path: spec, languages: [go]}, {definition: a/b, path: rust, languages: [rust]}]\n",
				wantText: "the section lists a/b twice",
			},
			{
				name:     "returns ErrConfig for two entries of one path",
				give:     "implements: [{definition: a/b, path: spec, languages: [go]}, {definition: a/c, path: spec, languages: [go]}]\n",
				wantText: "the paths spec and spec overlap",
			},
			{
				name: "returns ErrConfig for a path below the path of another entry",
				give: "implements: [{definition: a/b, path: spec, languages: [go]}, " +
					"{definition: a/c, path: spec/c, languages: [go]}]\n",
				wantText: "the paths spec and spec/c overlap",
			},
			{
				name: "returns ErrConfig for a path above the path of an earlier entry",
				give: "implements: [{definition: a/b, path: spec/c, languages: [go]}, " +
					"{definition: a/c, path: spec, languages: [go]}]\n",
				wantText: "the paths spec/c and spec overlap",
			},
			{
				name: "returns ErrConfig for two entries whose paths differ only in case",
				give: "implements: [{definition: a/b, path: spec, languages: [go]}, " +
					"{definition: a/c, path: Spec, languages: [go]}]\n",
				wantText: "the paths spec and Spec overlap",
			},
			{
				name: "returns ErrConfig for a path below the path of another entry in another case",
				give: "implements: [{definition: a/b, path: Spec, languages: [go]}, " +
					"{definition: a/c, path: spec/c, languages: [go]}]\n",
				wantText: "the paths Spec and spec/c overlap",
			},
			{
				name: "returns ErrConfig for a copy into the path of another entry",
				give: "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: other/a}]}, " +
					"{definition: a/c, path: other, languages: [go]}]\n",
				wantText: "the copy to other/a writes into the copy at other",
			},
			{
				name: "returns ErrConfig for a copy into the path of another entry in another case",
				give: "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: Other/a}]}, " +
					"{definition: a/c, path: other, languages: [go]}]\n",
				wantText: "the copy to Other/a writes into the copy at other",
			},
			{
				name:     "returns ErrConfig for two copies to one path",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: c}, {from: b, to: c}]}]\n",
				wantText: "two copies write c",
			},
			{
				name:     "returns ErrConfig for two copies to paths that differ only in case",
				give:     "implements: [{definition: a/b, path: spec, languages: [go], copies: [{from: a, to: c}, {from: b, to: C}]}]\n",
				wantText: "two copies write C",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := implementation.Parse([]byte(tt.give))
				assert.ErrorIs(t, err, implementation.ErrConfig, "Parse refuses a section that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the rule that the section breaks")
			})
		}
	})

	t.Run("Find", func(t *testing.T) {
		t.Parallel()

		entries := []implementation.Entry{{Definition: "a/b", Path: "b"}, {Definition: "a/c", Path: "c"}}

		t.Run("returns the element of the entries that vendors the definition", func(t *testing.T) {
			t.Parallel()
			e, err := implementation.Find(entries, "a/c")
			assert.NoError(t, err, "Find finds the entry")
			assert.Equal(t, e, &entries[1], "Find returns the element itself", assert.ByIdentity())
		})

		t.Run("returns ErrEntry for a definition without an entry", func(t *testing.T) {
			t.Parallel()
			_, err := implementation.Find(entries, "a/d")
			assert.ErrorIs(t, err, implementation.ErrEntry, "Find refuses a definition that no entry names")
		})
	})
}
