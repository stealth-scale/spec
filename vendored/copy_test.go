// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored_test

import (
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/vendored"
)

func TestCopy(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the files of the view below the path of the entry", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			assert.Equal(t, slices.Sorted(maps.Keys(c.Files)), []string{
				dir + "/VERSION",
				dir + "/corpus/a.json",
				dir + "/corpus/b.json",
				dir + "/" + vendored.ManifestFile,
				dir + "/overlays/go.json",
				dir + "/spec/zones.json",
				zones,
			}, "the copy contains the shared files, the files of go, its manifest and the zone table")
		})

		t.Run("returns the content of each file of the definition", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			assert.Equal(t, string(c.Files[dir+"/corpus/a.json"]), first["corpus/a.json"], "the copy keeps the bytes")
		})

		t.Run("returns the content of the file of a copy of the entry", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			assert.Equal(
				t,
				string(c.Files[zones]),
				first["spec/zones.json"],
				"the zone table has the bytes of the definition",
			)
		})

		t.Run("returns the manifest of the view with its source", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			want := view(t, first)
			want.Repo, want.Commit = repo, commit
			assert.Equal(t, c.Manifest, want, "the manifest is the view of go from the commit")
		})

		t.Run("returns the manifest that Encode writes as the file of the manifest", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			assert.Equal(t, c.Files[dir+"/"+vendored.ManifestFile], c.Manifest.Encode(), "the copy writes its manifest")
		})

		t.Run("returns a local manifest for an empty commit", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, "")
			assert.True(t, c.Manifest.Local, "a copy without a commit is local")
		})

		t.Run("returns the path of the entry as the directory", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			assert.Equal(t, c.Dir, dir, "the copy states its directory")
		})

		t.Run("returns ErrCopy for a copy of a file that the view lacks", func(t *testing.T) {
			t.Parallel()
			e := entry
			e.Copies = []implementation.Copy{{From: "overlays/rust.json", To: zones}}
			_, err := vendored.Build(load(t, first), &e, commit)
			assert.ErrorIs(t, err, vendored.ErrCopy, "Build refuses a copy of a file of another language")
		})

		t.Run("returns ErrCopy for a file of the definition at the path of the manifest", func(t *testing.T) {
			t.Parallel()
			contents := maps.Clone(first)
			contents[vendored.ManifestFile] = "{}\n"
			contents[spec.ConfigFile] = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "*.json"]
`
			e := entry
			e.Copies = nil
			_, err := vendored.Build(load(t, contents), &e, commit)
			assert.ErrorIs(t, err, vendored.ErrCopy, "Build refuses a file that the manifest of the copy would replace")
		})

		t.Run("returns manifest.ErrLanguage for a language that the definition lacks", func(t *testing.T) {
			t.Parallel()
			e := entry
			e.Languages = []spec.Language{"python"}
			_, err := vendored.Build(load(t, first), &e, commit)
			assert.ErrorIs(t, err, manifest.ErrLanguage, "Build returns the error of the view")
		})

		t.Run("returns definition.ErrConfig for a file of two languages", func(t *testing.T) {
			t.Parallel()
			contents := map[string]string{
				spec.ConfigFile: `definition:
  repo: dokimasia/assert-spec
  languages: [go, golang]
  files: [VERSION, "overlays/*.json"]
  language-files: ["overlays/{language}*.json"]
`,
				"VERSION":              "1.0.0\n",
				"overlays/go.json":     "{}\n",
				"overlays/golang.json": "{}\n",
			}
			e := entry
			e.Copies = nil
			_, err := vendored.Build(load(t, contents), &e, commit)
			assert.ErrorIs(t, err, definition.ErrConfig, "Build returns the error of the manifest")
		})
	})

	t.Run("Entries", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first element of the path of each file and the manifest in order", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				vendored.Entries(build(t, first, &entry, commit).Manifest),
				[]string{"VERSION", "corpus", vendored.ManifestFile, "overlays", "spec"},
				"the copy owns the entries that its files start with",
			)
		})

		t.Run("returns each entry of two manifests once", func(t *testing.T) {
			t.Parallel()
			old := manifest.Manifest{Files: map[string]string{"VERSION": "", "corpus/a.json": ""}}
			current := manifest.Manifest{Files: map[string]string{"corpus/b.json": "", "overlays/go.json": ""}}
			assert.Equal(
				t,
				vendored.Entries(old, current),
				[]string{"VERSION", "corpus", vendored.ManifestFile, "overlays"},
				"the entries of both copies",
			)
		})

		t.Run("returns the manifest alone without a manifest", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, vendored.Entries(), []string{vendored.ManifestFile}, "every copy owns its manifest")
		})
	})
}

// view returns the manifest of the go files of the definition contents.
func view(tb assert.TB, contents map[string]string) manifest.Manifest {
	tb.Helper()
	m, err := load(tb, contents).Manifest()
	assert.NoError(tb, err, "the manifest of the definition builds")
	v, err := m.View([]spec.Language{"go"})
	assert.NoError(tb, err, "the view of go builds")
	return v
}
