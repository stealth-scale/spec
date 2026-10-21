// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/prop"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/manifest"
)

// assertConfig configures a definition with the layout of assert-spec.
const assertConfig = `definition:
  repo: dokimasia/assert-spec
  cases: spec/cases.json
  languages: [go, rust]
  files: [VERSION, "spec/*.json", "corpus/**/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
`

// assertFiles are the files of a definition with the layout of assert-spec.
var assertFiles = map[string]string{
	spec.ConfigFile:           assertConfig,
	"VERSION":                 "9.0.0\n",
	"spec/assertions.json":    "{}\n",
	"spec/cases.json":         `{"cases": []}` + "\n",
	"spec/manifest.json":      "{}\n",
	"corpus/equal.json":       `{"assertion": "equal"}` + "\n",
	"corpus/prop/shrink.json": `{"kind": "shrink"}` + "\n",
	"corpus/prop/shrink.yaml": "kind: shrink\n",
	"overlays/go.json":        `{"language": "go"}` + "\n",
	"overlays/rust.json":      `{"language": "rust"}` + "\n",
	"tools/spec-sync.sh":      "#!/bin/sh\n",
}

// assertSelected are the paths of assertFiles that the configuration selects.
var assertSelected = []string{
	"VERSION",
	"corpus/equal.json",
	"corpus/prop/shrink.json",
	"overlays/go.json",
	"overlays/rust.json",
	"spec/assertions.json",
	"spec/cases.json",
}

func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the files that the patterns select", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, assertFiles))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(
				t,
				slices.Sorted(maps.Keys(tree.Files)),
				assertSelected,
				"the tree contains the selected files",
			)
		})

		t.Run("returns the content of each file", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, assertFiles))
			assert.NoError(t, err, "Open reads the definition")
			for p, data := range tree.Files {
				assert.Equal(t, string(data), assertFiles[p], "the tree contains the content of the file")
			}
		})

		t.Run("returns the version without white space", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, assertFiles))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(t, tree.Version, "9.0.0", "the tree states the version without its line feed")
		})

		t.Run("returns the configuration", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, assertFiles))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(
				t,
				tree.Config.Repo,
				"dokimasia/assert-spec",
				"the tree states the repository of its configuration",
			)
		})

		t.Run("leaves out the files below .git", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, map[string]string{
				spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: ['**/*.json']}\n",
				"VERSION":       "1.0.0\n",
				"a.json":        "{}\n",
				".git/b.json":   "{}\n",
			}))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(
				t,
				slices.Sorted(maps.Keys(tree.Files)),
				[]string{"a.json"},
				"the tree leaves out the files of git",
			)
		})

		t.Run("leaves out a link", func(t *testing.T) {
			t.Parallel()
			dir := files.Workspace(t, files.Tree{
				spec.ConfigFile: files.Text("definition: {repo: a/b, languages: [go], files: ['*.json']}\n"),
				"VERSION":       files.Text("1.0.0\n"),
				"a.json":        files.Text("{}\n"),
				"b.json":        files.Link("a.json"),
			})
			tree, err := definition.Open(openRoot(t, dir))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(t, slices.Sorted(maps.Keys(tree.Files)), []string{"a.json"}, "the tree leaves out the link")
		})

		t.Run("reads the files below the root", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, map[string]string{
				spec.ConfigFile:      "definition: {repo: a/b, root: spec, manifest: manifest.json, languages: [go], files: ['*.json']}\n",
				"VERSION":            "1.0.0\n",
				"a.json":             "{}\n",
				"spec/VERSION":       "2.0.0\n",
				"spec/b.json":        "{}\n",
				"spec/manifest.json": "{}\n",
			}))
			assert.NoError(t, err, "Open reads the definition below its root")
			assert.Equal(
				t,
				slices.Sorted(maps.Keys(tree.Files)),
				[]string{"b.json"},
				"the paths are relative to the root",
			)
			assert.Equal(t, tree.Version, "2.0.0", "the version file is relative to the root")
		})

		t.Run("returns fs.ErrPermission for a directory that cannot be read", func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skip("Windows stores no permission bit that forbids the read of a directory")
			}
			dir := files.Workspace(t, files.Tree{
				spec.ConfigFile: files.Text("definition: {repo: a/b, languages: [go], files: [VERSION]}\n"),
				"VERSION":       files.Text("1.0.0\n"),
				"corpus":        files.Dir().WithMode(0o000),
			})
			_, err := definition.Open(openRoot(t, dir))
			assert.ErrorIs(t, err, fs.ErrPermission, "Open returns the error of the walk")
		})

		t.Run("returns fs.ErrPermission for a selected file that cannot be read", func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skip("Windows stores no permission bit that forbids the read of a file")
			}
			dir := files.Workspace(t, files.Tree{
				spec.ConfigFile: files.Text("definition: {repo: a/b, languages: [go], files: ['*.json']}\n"),
				"VERSION":       files.Text("1.0.0\n"),
				"a.json":        files.Text("{}\n").WithMode(0o000),
			})
			_, err := definition.Open(openRoot(t, dir))
			assert.ErrorIs(t, err, fs.ErrPermission, "Open returns the error of the read")
		})

		t.Run("returns fs.ErrNotExist without a configuration file", func(t *testing.T) {
			t.Parallel()
			_, err := definition.Open(root(t, map[string]string{"VERSION": "1.0.0\n"}))
			assert.ErrorIs(t, err, fs.ErrNotExist, "Open needs the configuration file")
		})

		t.Run("reads a checkout whose .git is a file, as in a worktree", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Open(root(t, map[string]string{
				spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: ['*.json']}\n",
				".git":          "gitdir: ../main/.git/worktrees/a\n",
				"VERSION":       "1.0.0\n",
				"a.json":        "{}\n",
			}))
			assert.NoError(t, err, "Open reads the definition")
			assert.Equal(
				t,
				slices.Sorted(maps.Keys(tree.Files)),
				[]string{"a.json"},
				"the walk reads past the file .git",
			)
		})

		tests := []struct {
			name     string
			give     map[string]string
			wantText string
		}{
			{
				name:     "returns ErrConfig for a configuration file without the section definition",
				give:     map[string]string{spec.ConfigFile: "implements: []\n", "VERSION": "1.0.0\n"},
				wantText: "has no section definition",
			},
			{
				name: "returns ErrConfig for a pattern that matches no file",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: [VERSION, '*.json']}\n",
					"VERSION":       "1.0.0\n",
				},
				wantText: `the pattern "*.json" matches no file`,
			},
			{
				name: "returns ErrConfig for a pattern that matches the manifest alone",
				give: map[string]string{
					spec.ConfigFile:      "definition: {repo: a/b, languages: [go], files: [VERSION, 'spec/*.json']}\n",
					"VERSION":            "1.0.0\n",
					"spec/manifest.json": "{}\n",
				},
				wantText: `the pattern "spec/*.json" matches no file`,
			},
			{
				name: "returns ErrConfig for a version file that does not exist",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: ['*.json']}\n",
					"a.json":        "{}\n",
				},
				wantText: "read the version file VERSION",
			},
			{
				name: "returns ErrConfig for an empty version",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: [VERSION]}\n",
					"VERSION":       "\n",
				},
				wantText: "the version file VERSION is empty",
			},
			{
				name: "returns ErrConfig for a case index that the patterns do not select",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, cases: cases.json, languages: [go], files: [VERSION]}\n",
					"VERSION":       "1.0.0\n",
					"cases.json":    "{}\n",
				},
				wantText: "no pattern selects the case index cases.json",
			},
			{
				name: "returns ErrConfig for a root that does not exist",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, root: spec, languages: [go], files: [VERSION]}\n",
					"VERSION":       "1.0.0\n",
				},
				wantText: "open the root spec",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := definition.Open(root(t, tt.give))
				assert.ErrorIs(t, err, definition.ErrConfig, "Open refuses a definition that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the rule that the definition breaks")
			})
		}
	})

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tree that Open returns for the same files", func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			var cases atomic.Int64
			prop.ForAll(t, "Load and Open read the same tree from the same files", func(c *prop.Case) {
				// Each name starts with a letter that no name of a device of Windows starts with.
				contents := c.Draw(prop.Dict(
					prop.StringMatching(`(spec/)?[d-k][a-z]{0,3}(/[d-k][a-z]{0,3})?\.(json|yaml)`),
					prop.String(prop.MaxSize(8)),
					prop.MaxSize(6),
				), "files")
				contents[spec.ConfigFile] = "definition:\n  repo: a/b\n  root: " +
					c.Draw(prop.SampledFrom(".", "spec"), "root") +
					"\n  languages: [go]\n  files: [VERSION, '**/*.json']\n"
				contents["VERSION"] = "1.0.0\n"
				contents["spec/VERSION"] = "2.0.0\n"
				contents["spec/z.json"] = "{}\n"
				name := strconv.FormatInt(cases.Add(1), 10)
				files.Write(c, base, files.Tree{name: files.Dir()})
				dir := filepath.Join(base, name)
				files.Write(c, dir, treeOf(contents))
				opened, err := definition.Open(openRoot(c, dir))
				assert.NoError(c, err, "Open reads the definition")
				loaded, err := definition.Load(bytesOf(contents))
				assert.NoError(c, err, "Load reads the definition")
				assert.Equal(c, loaded, opened, "Load and Open return the same tree")
			})
		})

		tests := []struct {
			name     string
			give     map[string]string
			wantText string
		}{
			{
				name:     "returns ErrConfig without a configuration file",
				give:     map[string]string{"VERSION": "1.0.0\n"},
				wantText: "the files have no .spec.yaml",
			},
			{
				name: "returns ErrConfig for a version file that does not exist",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: ['*.json']}\n",
					"a.json":        "{}\n",
				},
				wantText: "the files have no version file VERSION",
			},
			{
				name:     "returns ErrConfig for a configuration file without the section definition",
				give:     map[string]string{spec.ConfigFile: "implements: []\n", "VERSION": "1.0.0\n"},
				wantText: "has no section definition",
			},
			{
				name: "returns ErrConfig for a pattern that matches no file",
				give: map[string]string{
					spec.ConfigFile: "definition: {repo: a/b, languages: [go], files: [VERSION, '*.json']}\n",
					"VERSION":       "1.0.0\n",
				},
				wantText: `the pattern "*.json" matches no file`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := definition.Load(bytesOf(tt.give))
				assert.ErrorIs(t, err, definition.ErrConfig, "Load refuses a definition that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the rule that the definition breaks")
			})
		}
	})

	t.Run("Manifest", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest of the files of each language", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Load(bytesOf(assertFiles))
			assert.NoError(t, err, "Load reads the definition")
			m, err := tree.Manifest()
			assert.NoError(t, err, "Manifest builds the manifest of the definition")
			want, err := manifest.New("9.0.0", tree.Files, map[spec.Language][]string{
				"go":   {"overlays/go.json"},
				"rust": {"overlays/rust.json"},
			})
			assert.NoError(t, err, "New builds the manifest of the files")
			want.Cases = "spec/cases.json"
			assert.Equal(t, m, want, "the manifest assigns each overlay to its language and states the index")
		})

		t.Run("returns an empty list for a language without files", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Load(map[string][]byte{
				spec.ConfigFile: []byte("definition: {repo: a/b, languages: [go, rust], files: [VERSION, '*.json'], " +
					"language-files: ['{language}.json']}\n"),
				"VERSION": []byte("1.0.0\n"),
				"go.json": []byte("{}\n"),
			})
			assert.NoError(t, err, "Load reads the definition")
			m, err := tree.Manifest()
			assert.NoError(t, err, "Manifest builds the manifest of the definition")
			assert.Equal(t, m.Languages, map[spec.Language][]string{"go": {"go.json"}, "rust": {}}, "rust has no files")
		})

		t.Run("returns manifest.ErrInvalid for a path that a manifest cannot list", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Load(map[string][]byte{
				spec.ConfigFile: []byte("definition: {repo: a/b, languages: [go], files: ['*.json']}\n"),
				"VERSION":       []byte("1.0.0\n"),
				`a\b.json`:      []byte("{}\n"),
			})
			assert.NoError(t, err, "Load reads the definition")
			_, err = tree.Manifest()
			assert.ErrorIs(t, err, manifest.ErrInvalid, "Manifest returns the error of the manifest")
		})

		t.Run("assigns a file that two patterns of one language match to that language", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Load(map[string][]byte{
				spec.ConfigFile: []byte("definition: {repo: a/b, languages: [go, rust], files: [VERSION, '*.json'], " +
					"language-files: ['{language}.json', '{language}*.json']}\n"),
				"VERSION": []byte("1.0.0\n"),
				"go.json": []byte("{}\n"),
			})
			assert.NoError(t, err, "Load reads the definition")
			m, err := tree.Manifest()
			assert.NoError(t, err, "Manifest builds the manifest of the definition")
			assert.Equal(t, m.Languages, map[spec.Language][]string{"go": {"go.json"}, "rust": {}}, "go.json is of go")
		})

		t.Run("returns ErrConfig for a file of two languages", func(t *testing.T) {
			t.Parallel()
			tree, err := definition.Load(map[string][]byte{
				spec.ConfigFile: []byte(
					"definition: {repo: a/b, languages: [go, golang], files: [VERSION, '*.json'], " +
						"language-files: ['{language}*.json']}\n",
				),
				"VERSION":     []byte("1.0.0\n"),
				"golang.json": []byte("{}\n"),
			})
			assert.NoError(t, err, "Load reads the definition")
			_, err = tree.Manifest()
			assert.ErrorIs(t, err, definition.ErrConfig, "Manifest refuses a file that two languages claim")
		})
	})
}

// cleaner is a test or a case of a property, which runs a function when it ends.
type cleaner interface {
	assert.TB
	Cleanup(f func())
}

// root writes the text files contents into a workspace of the test, and returns the root of the
// workspace, which the test closes when it ends.
func root(t *testing.T, contents map[string]string) *os.Root {
	t.Helper()
	return openRoot(t, files.Workspace(t, treeOf(contents)))
}

// openRoot opens the root of dir, which tb closes when it ends.
func openRoot(tb cleaner, dir string) *os.Root {
	tb.Helper()
	r, err := os.OpenRoot(dir)
	assert.NoError(tb, err, "the directory opens as a root")
	tb.Cleanup(func() { _ = r.Close() })
	return r
}

// treeOf returns the tree of the text files contents.
func treeOf(contents map[string]string) files.Tree {
	tree := make(files.Tree, len(contents))
	for p, text := range contents {
		tree[p] = files.Text(text)
	}
	return tree
}

// bytesOf returns the text files contents with their content as bytes.
func bytesOf(contents map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(contents))
	for p, text := range contents {
		out[p] = []byte(text)
	}
	return out
}
