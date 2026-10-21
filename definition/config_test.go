// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every key of the section definition", func(t *testing.T) {
			t.Parallel()
			c, err := definition.Parse([]byte(`
definition:
  repo: dokimasia/dokimi
  root: spec
  version: spec/VERSION
  manifest: spec/digest.json
  cases: cases.json
  languages: [go, rust]
  files: [cases.json, "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
  implementations:
    - repo: dokimasia/dokimi-go
      languages: [go]
    - repo: dokimasia/dokimi
      languages: [all]
`))
			assert.NoError(t, err, "Parse accepts a section with every key")
			assert.Equal(t, c, definition.Config{
				Repo:          "dokimasia/dokimi",
				Root:          "spec",
				Version:       "spec/VERSION",
				Manifest:      "spec/digest.json",
				Cases:         "cases.json",
				Languages:     []spec.Language{"go", "rust"},
				Files:         []string{"cases.json", "overlays/*.json"},
				LanguageFiles: []string{"overlays/{language}.json"},
				Implementations: []definition.Implementation{
					{Repo: "dokimasia/dokimi-go", Languages: []spec.Language{"go"}},
					{Repo: "dokimasia/dokimi", Languages: []spec.Language{spec.All}},
				},
			}, "the configuration states every key of the section")
		})

		t.Run("returns the defaults of the keys that the section leaves out", func(t *testing.T) {
			t.Parallel()
			c, err := definition.Parse([]byte(`
definition:
  repo: dokimasia/mutate-spec
  languages: [go]
  files: [VERSION]
`))
			assert.NoError(t, err, "Parse accepts a section with the required keys")
			assert.Equal(t, c, definition.Config{
				Repo:      "dokimasia/mutate-spec",
				Root:      definition.DefaultRoot,
				Version:   definition.DefaultVersion,
				Manifest:  definition.DefaultManifest,
				Languages: []spec.Language{"go"},
				Files:     []string{"VERSION"},
			}, "the configuration states the default of each key that the section leaves out")
		})

		t.Run("ignores the section implements", func(t *testing.T) {
			t.Parallel()
			_, err := definition.Parse([]byte(`
implements:
  - definition: dokimasia/assert-spec
definition:
  repo: dokimasia/assert-spec
  languages: [go]
  files: [VERSION]
`))
			assert.NoError(t, err, "Parse decodes the section definition alone")
		})

		tests := []struct {
			name     string
			give     string
			wantText string
		}{
			{name: "returns ErrConfig for data that is no YAML", give: "definition: [\n", wantText: "yaml: decode"},
			{
				name:     "returns ErrConfig for a file without the section definition",
				give:     "implements: []\n",
				wantText: "has no section definition",
			},
			{
				name:     "returns ErrConfig for a section that is null",
				give:     "definition:\n",
				wantText: "has no section definition",
			},
			{
				name:     "returns ErrConfig for a key that the section does not have",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], file: [a]}\n",
				wantText: "unknown key definition.file",
			},
			{
				name:     "returns ErrConfig for a section without a repository",
				give:     "definition: {languages: [go], files: [VERSION]}\n",
				wantText: "missing definition.repo",
			},
			{
				name:     "returns ErrConfig for a repository that is not valid",
				give:     "definition: {repo: assert-spec, languages: [go], files: [VERSION]}\n",
				wantText: `"assert-spec" is no repository of GitHub`,
			},
			{
				name:     "returns ErrConfig for an absolute root",
				give:     "definition: {repo: a/b, root: /spec, languages: [go], files: [VERSION]}\n",
				wantText: `the root "/spec" is no portable directory of the repository`,
			},
			{
				name:     "returns ErrConfig for a root that leaves the repository",
				give:     "definition: {repo: a/b, root: ../spec, languages: [go], files: [VERSION]}\n",
				wantText: `the root "../spec" is no portable directory of the repository`,
			},
			{
				name:     "returns ErrConfig for a root that Windows cannot store",
				give:     "definition: {repo: a/b, root: aux, languages: [go], files: [VERSION]}\n",
				wantText: `the root "aux" is no portable directory of the repository`,
			},
			{
				name:     "returns ErrConfig for a version file that is the root",
				give:     "definition: {repo: a/b, version: ., languages: [go], files: [VERSION]}\n",
				wantText: `"." is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for a version file that Windows cannot store",
				give:     "definition: {repo: a/b, version: 'VERSION.', languages: [go], files: [VERSION]}\n",
				wantText: `"VERSION." is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for a manifest that leaves the root",
				give:     "definition: {repo: a/b, manifest: ../manifest.json, languages: [go], files: [VERSION]}\n",
				wantText: `"../manifest.json" is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for a case index that leaves the root",
				give:     "definition: {repo: a/b, cases: ../cases.json, languages: [go], files: [VERSION]}\n",
				wantText: `the case index "../cases.json" is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for a case index that Windows cannot store",
				give:     "definition: {repo: a/b, cases: 'cases?.json', languages: [go], files: [VERSION]}\n",
				wantText: `the case index "cases?.json" is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for a section without languages",
				give:     "definition: {repo: a/b, files: [VERSION]}\n",
				wantText: "missing definition.languages",
			},
			{
				name:     "returns ErrConfig for an empty list of languages",
				give:     "definition: {repo: a/b, languages: [], files: [VERSION]}\n",
				wantText: "the definition declares no language",
			},
			{
				name:     "returns ErrConfig for a language that is not valid",
				give:     "definition: {repo: a/b, languages: [Go], files: [VERSION]}\n",
				wantText: `"Go" is no language of a definition`,
			},
			{
				name:     "returns ErrConfig for the language all",
				give:     "definition: {repo: a/b, languages: [all], files: [VERSION]}\n",
				wantText: `"all" is no language of a definition`,
			},
			{
				name:     "returns ErrConfig for a language that the section lists twice",
				give:     "definition: {repo: a/b, languages: [go, go], files: [VERSION]}\n",
				wantText: "the definition declares go twice",
			},
			{
				name:     "returns ErrConfig for a section without files",
				give:     "definition: {repo: a/b, languages: [go]}\n",
				wantText: "missing definition.files",
			},
			{
				name:     "returns ErrConfig for an empty list of files",
				give:     "definition: {repo: a/b, languages: [go], files: []}\n",
				wantText: "the definition states no files",
			},
			{
				name:     "returns ErrConfig for a pattern of files that is not valid",
				give:     "definition: {repo: a/b, languages: [go], files: ['corpus/[a.json']}\n",
				wantText: `"corpus/[a.json" is no pattern of the files below the root`,
			},
			{
				name:     "returns ErrConfig for an absolute pattern of files",
				give:     "definition: {repo: a/b, languages: [go], files: [/VERSION]}\n",
				wantText: `"/VERSION" is no pattern of the files below the root`,
			},
			{
				name:     "returns ErrConfig for a pattern of the files of a language without the language",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], language-files: [overlays/go.json]}\n",
				wantText: `the pattern "overlays/go.json" of the files of a language has no {language}`,
			},
			{
				name:     "returns ErrConfig for a pattern of the files of a language that is not valid",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], language-files: ['[{language}.json']}\n",
				wantText: `"[go.json" is no pattern of the files below the root`,
			},
			{
				name:     "returns ErrConfig for an absolute pattern of the files of a language",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], language-files: ['/{language}.json']}\n",
				wantText: `"/go.json" is no pattern of the files below the root`,
			},
			{
				name:     "returns ErrConfig for an implementation with a repository that is not valid",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], implementations: [{repo: c, languages: [go]}]}\n",
				wantText: `"c" is no repository of GitHub`,
			},
			{
				name: "returns ErrConfig for an implementation that the section lists twice",
				give: "definition: {repo: a/b, languages: [go], files: [VERSION], " +
					"implementations: [{repo: a/c, languages: [go]}, {repo: a/c, languages: [go]}]}\n",
				wantText: "the definition lists a/c twice",
			},
			{
				name:     "returns ErrConfig for an implementation without languages",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], implementations: [{repo: a/c}]}\n",
				wantText: "missing definition.implementations[0].languages",
			},
			{
				name:     "returns ErrConfig for an implementation with an empty list of languages",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], implementations: [{repo: a/c, languages: []}]}\n",
				wantText: "a/c implements no language",
			},
			{
				name:     "returns ErrConfig for an empty version file",
				give:     "definition: {repo: a/b, version: '', languages: [go], files: [VERSION]}\n",
				wantText: `"" is no portable path below the root`,
			},
			{
				name:     "returns ErrConfig for an implementation of a language that the section does not have",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], implementations: [{repo: a/c, languages: [rust]}]}\n",
				wantText: `a/c implements "rust", which the definition does not declare`,
			},
			{
				name:     "returns ErrConfig for an implementation of all and another language",
				give:     "definition: {repo: a/b, languages: [go], files: [VERSION], implementations: [{repo: a/c, languages: [all, go]}]}\n",
				wantText: `a/c implements "all", which the definition does not declare`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := definition.Parse([]byte(tt.give))
				assert.ErrorIs(t, err, definition.ErrConfig, "Parse refuses a section that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the rule that the section breaks")
			})
		}
	})
}
