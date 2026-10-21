// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest_test

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"
	"go.dokimi.dev/assert/prop"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// The values of the manifests that the cases build.
const (
	version = "1.0.0"
	commit  = "27b95377f6a3c1d2e4b5a6978c0d1e2f3a4b5c6d"
	repo    = spec.Repo("dokimasia/assert-spec")
	golang  = spec.Language("go")
	rust    = spec.Language("rust")
)

// contents are the files of the definition of the golden files.
var contents = map[string][]byte{
	"VERSION":            []byte("1.0.0\n"),
	"corpus/a.json":      []byte("{}\n"),
	"overlays/go.json":   []byte(`{"language": "go"}` + "\n"),
	"overlays/rust.json": []byte(`{"language": "rust"}` + "\n"),
}

// languages are the files of each language of contents.
var languages = map[spec.Language][]string{
	golang: {"overlays/go.json"},
	rust:   {"overlays/rust.json"},
}

// manifests generates the manifests that New returns for up to eight files, each of no language,
// of go or of rust. Each name starts with a letter that no name of a device of Windows starts with.
var manifests = prop.Composite(func(c *prop.Case) manifest.Manifest {
	paths := c.Draw(prop.List(
		prop.StringMatching(`[d-k][a-z]{0,5}(/[d-k][a-z]{0,5}){0,2}\.json`), prop.Unique(), prop.MaxSize(8),
	), "paths")
	files := make(map[string][]byte, len(paths))
	own := map[spec.Language][]string{golang: {}, rust: {}}
	for _, p := range paths {
		files[p] = c.Draw(prop.Bytes(prop.MaxSize(16)), "content")
		if l := c.Draw(prop.SampledFrom[spec.Language]("", golang, rust), "language"); l != "" {
			own[l] = append(own[l], p)
		}
	}
	m, err := manifest.New(c.Draw(prop.SampledFrom("1.0.0", "9.0.0"), "version"), files, own)
	assert.NoError(c, err, "New returns the manifest of the generated files")
	if len(paths) > 0 && c.Draw(prop.Boolean(), "cases") {
		m.Cases = paths[0]
	}
	return m
})

func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the Sum of each file", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Length(t, m.Files, len(contents), "the manifest lists every file")
			for p, data := range contents {
				assert.Equal(t, m.Files[p], manifest.Sum(data), "the manifest states the Sum of the file")
			}
		})

		t.Run("returns the Digest of the files", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Equal(t, m.Digest, manifest.Digest(m.Files), "the digest of the manifest is the digest of its files")
		})

		t.Run("returns the version", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Equal(t, m.Version, version, "the manifest states the version")
		})

		t.Run("returns the files of each language in the order of their paths", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, map[spec.Language][]string{
				golang: {"overlays/go.json", "corpus/a.json"},
			})
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Equal(
				t,
				m.Languages,
				map[spec.Language][]string{golang: {"corpus/a.json", "overlays/go.json"}},
				"the manifest lists the files of the language sorted",
			)
		})

		t.Run("returns an empty list for a language without files", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, map[spec.Language][]string{golang: nil})
			assert.NoError(t, err, "New accepts a language without files")
			assert.Equal(t, m.Languages, map[spec.Language][]string{golang: {}}, "the language has an empty list")
		})

		t.Run("returns lists that share no array with the argument", func(t *testing.T) {
			t.Parallel()
			own := map[spec.Language][]string{golang: {"overlays/go.json"}}
			m, err := manifest.New(version, contents, own)
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Pure(t, func() []string { return slices.Clone(m.Languages[golang]) }, func() {
				own[golang][0] = "corpus/a.json"
			}, "a change of the argument leaves the manifest as it is")
		})

		tests := []struct {
			name     string
			give     func() (string, map[string][]byte, map[spec.Language][]string)
			wantText string
		}{
			{
				name: "returns ErrInvalid for an empty version",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return "", contents, languages
				},
				wantText: `the version "" is empty or has white space at an end`,
			},
			{
				name: "returns ErrInvalid for a version with a line break",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return "1.0.0\n", contents, languages
				},
				wantText: `the version "1.0.0\n" is empty or has white space at an end`,
			},
			{
				name: "returns ErrInvalid for an absolute path",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"/VERSION": nil}, nil
				},
				wantText: `"/VERSION" is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for a path that leaves the definition",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"../VERSION": nil}, nil
				},
				wantText: `"../VERSION" is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for the path of the root",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{".": nil}, nil
				},
				wantText: `"." is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for a path with a backslash",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{`corpus\a.json`: nil}, nil
				},
				wantText: `"corpus\\a.json" is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for a path with a NUL byte",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"corpus/a\x00.json": nil}, nil
				},
				wantText: `"corpus/a\x00.json" is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for a path that Windows cannot store",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"corpus/aux.json": nil}, nil
				},
				wantText: `"corpus/aux.json" is no portable path of a file of a definition`,
			},
			{
				name: "returns ErrInvalid for two files whose paths differ only in case",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"corpus/a.json": nil, "corpus/A.json": nil}, nil
				},
				wantText: "corpus/A.json and corpus/a.json differ only in case",
			},
			{
				name: "returns ErrInvalid for two directories whose paths differ only in case",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"corpus/a.json": nil, "Corpus/b.json": nil}, nil
				},
				wantText: "Corpus and corpus differ only in case",
			},
			{
				name: "returns ErrInvalid for the path of a file that is also a directory",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, map[string][]byte{"corpus": nil, "corpus/a.json": nil}, nil
				},
				wantText: "corpus is a file and a directory",
			},
			{
				name: "returns ErrInvalid for an invalid language",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, contents, map[spec.Language][]string{"Go": nil}
				},
				wantText: `"Go" is no language of a definition`,
			},
			{
				name: "returns ErrInvalid for the language all",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, contents, map[spec.Language][]string{spec.All: nil}
				},
				wantText: `"all" is no language of a definition`,
			},
			{
				name: "returns ErrInvalid for a file of a language that the contents do not have",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, contents, map[spec.Language][]string{golang: {"overlays/python.json"}}
				},
				wantText: "go lists overlays/python.json, which is no file of the manifest",
			},
			{
				name: "returns ErrInvalid for a file of two languages",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, contents, map[spec.Language][]string{
						golang: {"overlays/go.json"},
						rust:   {"overlays/go.json"},
					}
				},
				wantText: "go and rust both list overlays/go.json",
			},
			{
				name: "returns ErrInvalid for a file that one language lists twice",
				give: func() (string, map[string][]byte, map[spec.Language][]string) {
					return version, contents, map[spec.Language][]string{
						golang: {"overlays/go.json", "overlays/go.json"},
					}
				},
				wantText: "go and go both list overlays/go.json",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := manifest.New(tt.give())
				assert.ErrorIs(t, err, manifest.ErrInvalid, "New refuses a manifest that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the rule that the manifest breaks")
			})
		}
	})

	t.Run("View", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the shared files and the files of its language", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			view, err := m.View([]spec.Language{golang})
			assert.NoError(t, err, "View accepts a language of the manifest")
			want := maps.Clone(m.Files)
			delete(want, "overlays/rust.json")
			assert.Equal(t, view.Files, want, "the view lists every file but those of rust")
			assert.Equal(
				t,
				view.Languages,
				map[spec.Language][]string{golang: {"overlays/go.json"}},
				"the view states the files of its language",
			)
			assert.Equal(
				t,
				view.Digest,
				"sha256:3c0bc3efc5967f6c3cc2f628b456f74b91eaa8637c3f5521bf237d6525d4e6ca",
				"the digest of the view is the digest that the script of the definitions computes for its files",
			)
			assert.Equal(t, view.Version, version, "the view states the version of the manifest")
		})

		t.Run("returns every file for all", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			view, err := m.View([]spec.Language{spec.All})
			assert.NoError(t, err, "View accepts all")
			assert.Equal(t, view, m, "the view of all languages is the manifest")
		})

		t.Run("returns the files of every language that it names", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			view, err := m.View([]spec.Language{rust, golang})
			assert.NoError(t, err, "View accepts two languages of the manifest")
			assert.Equal(t, view, m, "the view of both languages is the manifest")
		})

		t.Run("returns a manifest that shares no map with the receiver", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			assert.Pure(t, func() manifest.Manifest {
				return manifest.Manifest{Files: maps.Clone(m.Files), Languages: map[spec.Language][]string{
					golang: slices.Clone(m.Languages[golang]),
				}}
			}, func() {
				view, err := m.View([]spec.Language{golang})
				assert.NoError(t, err, "View accepts a language of the manifest")
				view.Files["VERSION"] = manifest.Sum(nil)
				view.Languages[golang][0] = "VERSION"
			}, "a change of the view leaves the manifest as it is")
		})

		t.Run("keeps every shared file and only the files of its language", func(t *testing.T) {
			t.Parallel()
			prop.ForAll(t, "a view keeps every shared file and only the files of its language", func(c *prop.Case) {
				m := c.Draw(manifests, "manifest")
				l, other := golang, rust
				if c.Draw(prop.Boolean(), "rust") {
					l, other = rust, golang
				}
				view, err := m.View([]spec.Language{l})
				assert.NoError(c, err, "View accepts a language of the manifest")
				for p, digest := range m.Files {
					if slices.Contains(m.Languages[other], p) {
						assert.NotContains(c, view.Files, p, "the view leaves out a file of the other language")
						continue
					}
					assert.Equal(c, view.Files[p], digest, "the view keeps a file that the other language does not own")
				}
				assert.Equal(
					c,
					view.Digest,
					manifest.Digest(view.Files),
					"the digest of the view is the digest of its files",
				)
			})
		})

		t.Run("keeps the case index of a shared file", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			m.Cases = "corpus/a.json"
			view, err := m.View([]spec.Language{golang})
			assert.NoError(t, err, "View accepts a language of the manifest")
			assert.Equal(t, view.Cases, "corpus/a.json", "the view keeps the case index")
		})

		t.Run("drops the case index of a file that the view leaves out", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			m.Cases = "overlays/rust.json"
			view, err := m.View([]spec.Language{golang})
			assert.NoError(t, err, "View accepts a language of the manifest")
			assert.Equal(t, view.Cases, "", "the view of go has no index of rust")
		})

		t.Run("returns ErrLanguage for a language that the manifest does not state", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			_, err = m.View([]spec.Language{golang, "python"})
			assert.ErrorIs(t, err, manifest.ErrLanguage, "View refuses a language that the manifest lacks")
		})

		t.Run("returns ErrLanguage for no language", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			_, err = m.View(nil)
			assert.ErrorIs(t, err, manifest.ErrLanguage, "View refuses a view without a language")
		})
	})

	t.Run("Encode", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the JSON of a definition with sorted keys", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			golden.MatchAt(t, filepath.Join("testdata", "golden", "definition.json"), m.Encode(), golden.ShouldUpdate())
		})

		t.Run("writes the repository and the commit of a copy", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			view, err := m.View([]spec.Language{golang})
			assert.NoError(t, err, "View accepts a language of the manifest")
			view.Repo, view.Commit = repo, commit
			golden.MatchAt(t, filepath.Join("testdata", "golden", "copy.json"), view.Encode(), golden.ShouldUpdate())
		})

		t.Run("writes local for a local copy", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			m.Repo, m.Local = repo, true
			text := string(m.Encode())
			assert.Contains(t, text, `    "local": true,`, "the copy states that it is local")
			assert.NotContains(t, text, `"commit"`, "a local copy states no commit")
		})

		t.Run("writes the case index of a manifest with one", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, contents, languages)
			assert.NoError(t, err, "New accepts the contents of a definition")
			m.Cases = "corpus/a.json"
			assert.Contains(t, string(m.Encode()), `    "cases": "corpus/a.json",`, "the manifest states its index")
		})

		t.Run("writes no languages for a manifest without languages", func(t *testing.T) {
			t.Parallel()
			m := manifest.Manifest{Files: map[string]string{}, Digest: manifest.Digest(nil), Version: version}
			assert.NotContains(t, string(m.Encode()), `"languages"`, "a manifest without languages states none")
		})

		t.Run("writes the characters of HTML as they are", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.New(version, map[string][]byte{"a&b.json": nil}, nil)
			assert.NoError(t, err, "New accepts a path with an ampersand")
			assert.Contains(t, string(m.Encode()), `"a&b.json"`, "the path keeps its characters")
		})
	})

	t.Run("ValidPath", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a file at the root", give: "VERSION", want: true},
			{name: "reports true for a file below a directory", give: "corpus/prop/shrink.json", want: true},
			{name: "reports false for the root", give: ".", want: false},
			{name: "reports false for the empty path", give: "", want: false},
			{name: "reports false for an absolute path", give: "/VERSION", want: false},
			{name: "reports false for a path with an element of ..", give: "corpus/../VERSION", want: false},
			{name: "reports false for a path with a trailing slash", give: "corpus/", want: false},
			{name: "reports false for a path with a backslash", give: `corpus\a.json`, want: false},
			{name: "reports false for a path with NUL", give: "a\x00.json", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					manifest.ValidPath(tt.give),
					tt.want,
					"ValidPath reports whether a manifest can list the path",
				)
			})
		}
	})

	t.Run("PortablePath", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a file below a directory", give: "corpus/prop/shrink.json", want: true},
			{name: "reports true for a name with a space and an ampersand", give: "a b&c.json", want: true},
			{name: "reports true for a name that starts with the name of a device", give: "console.json", want: true},
			{name: "reports true for the name of a port above 9", give: "com10.json", want: true},
			{name: "reports true for a name that starts with a dot", give: "corpus/.keep", want: true},
			{name: "reports false for a path that ValidPath refuses", give: "corpus/../VERSION", want: false},
			{name: "reports false for a name with a colon", give: "corpus/a:b.json", want: false},
			{name: "reports false for a name with a question mark", give: "a?.json", want: false},
			{name: "reports false for a name with a tab", give: "a\tb.json", want: false},
			{name: "reports false for a name with the last control character", give: "a\x1fb.json", want: false},
			{name: "reports false for a name that ends with a dot", give: "corpus/a.", want: false},
			{name: "reports false for a directory whose name ends with a space", give: "corpus /a.json", want: false},
			{name: "reports false for the name of a device", give: "nul", want: false},
			{name: "reports false for the name of a device in upper case", give: "CON.json", want: false},
			{name: "reports false for a directory with the name of a device", give: "aux/a.json", want: false},
			{name: "reports false for the name of a device with two extensions", give: "prn.tar.gz", want: false},
			{name: "reports false for the name of a device before a space", give: "con .json", want: false},
			{name: "reports false for the name of a serial port", give: "com1.json", want: false},
			{name: "reports false for the name of a parallel port", give: "lpt9", want: false},
			{
				name: "reports false for the name of a port with a superscript digit",
				give: "COM\U000000B9.json",
				want: false,
			},
			{name: "reports false for the name of the output of the console", give: "conout$.log", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(
					t,
					manifest.PortablePath(tt.give),
					tt.want,
					"PortablePath reports whether Linux, macOS and Windows can store a file at the path",
				)
			})
		}
	})

	t.Run("Fold", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one form for two paths exactly when strings.EqualFold reports true", func(t *testing.T) {
			t.Parallel()
			// The runes of the paths include the Kelvin sign and the long s, which strings.EqualFold
			// matches with k and with s, and runes without case.
			runes := prop.SampledFrom('a', 'A', 'k', 'K', '\U0000212A', 's', 'S', '\U0000017F', '1', '/')
			paths := prop.List(runes, prop.MaxSize(4)).Map(func(r []rune) string { return string(r) })
			prop.ForAll(t, "Fold agrees with strings.EqualFold", func(c *prop.Case) {
				a, b := c.Draw(paths, "a"), c.Draw(paths, "b")
				assert.Equal(
					c,
					manifest.Fold(a) == manifest.Fold(b),
					strings.EqualFold(a, b),
					"two paths have one Fold exactly when they are equal without case",
				)
			})
		})
	})

	t.Run("Decode", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest that Encode writes", func(t *testing.T) {
			t.Parallel()
			prop.RoundTrip(t, func(m manifest.Manifest) ([]byte, error) {
				return m.Encode(), nil
			}, manifest.Decode, "Decode returns the manifest that Encode writes", prop.Using(manifests))
		})

		t.Run("returns the source of a copy", func(t *testing.T) {
			t.Parallel()
			m, err := manifest.Decode(encode(t, copyObject()))
			assert.NoError(t, err, "Decode accepts the manifest of a copy")
			assert.Equal(t, m.Repo, repo, "the copy states its repository")
			assert.Equal(t, m.Commit, commit, "the copy states its commit")
		})

		t.Run("returns a local copy", func(t *testing.T) {
			t.Parallel()
			object := copyObject()
			delete(object, "commit")
			object["local"] = true
			m, err := manifest.Decode(encode(t, object))
			assert.NoError(t, err, "Decode accepts the manifest of a local copy")
			assert.True(t, m.Local, "the copy states that it is local")
		})

		t.Run("returns a manifest without languages", func(t *testing.T) {
			t.Parallel()
			object := copyObject()
			delete(object, "languages")
			m, err := manifest.Decode(encode(t, object))
			assert.NoError(t, err, "Decode accepts a manifest without languages")
			assert.Nil(t, m.Languages, "the manifest states no languages")
		})

		tests := []struct {
			name string
			give func(object map[string]any)
		}{
			{
				name: "returns ErrCorrupt for a key that a manifest does not have",
				give: func(object map[string]any) { object["extra"] = true },
			},
			{
				name: "returns ErrCorrupt for an empty version",
				give: func(object map[string]any) { object["version"] = "" },
			},
			{
				name: "returns ErrCorrupt for a version with a line break",
				give: func(object map[string]any) { object["version"] = "1.0.0\n" },
			},
			{
				name: "returns ErrCorrupt for a manifest without files",
				give: func(object map[string]any) {
					delete(object, "files")
					delete(object, "languages")
					delete(object, "cases")
					object["digest"] = manifest.Digest(nil)
				},
			},
			{
				name: "returns ErrCorrupt for a path that leaves the definition",
				give: func(object map[string]any) {
					object["files"].(map[string]any)["../a.json"] = manifest.Sum(nil)
					seal(object)
				},
			},
			{
				name: "returns ErrCorrupt for a path that Windows cannot store",
				give: func(object map[string]any) {
					object["files"].(map[string]any)["aux.json"] = manifest.Sum(nil)
					seal(object)
				},
			},
			{
				name: "returns ErrCorrupt for two files whose paths differ only in case",
				give: func(object map[string]any) {
					object["files"].(map[string]any)["version"] = manifest.Sum(nil)
					seal(object)
				},
			},
			{
				name: "returns ErrCorrupt for the digest of a file without its prefix",
				give: func(object map[string]any) {
					object["files"].(map[string]any)["VERSION"] = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
					seal(object)
				},
			},
			{
				name: "returns ErrCorrupt for the digest of a file in upper-case hexadecimal",
				give: func(object map[string]any) {
					object["files"].(map[string]any)["VERSION"] = "sha256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"
					seal(object)
				},
			},
			{
				name: "returns ErrCorrupt for a digest that is not the Digest of the files",
				give: func(object map[string]any) { object["digest"] = manifest.Sum(nil) },
			},
			{
				name: "returns ErrCorrupt for an invalid language",
				give: func(object map[string]any) { object["languages"] = map[string]any{"Go": []any{}} },
			},
			{
				name: "returns ErrCorrupt for the language all",
				give: func(object map[string]any) { object["languages"] = map[string]any{"all": []any{}} },
			},
			{
				name: "returns ErrCorrupt for a file of a language that the files do not have",
				give: func(object map[string]any) {
					object["languages"] = map[string]any{"go": []any{"overlays/python.json"}}
				},
			},
			{
				name: "returns ErrCorrupt for a file of two languages",
				give: func(object map[string]any) {
					object["languages"] = map[string]any{"go": []any{"VERSION"}, "rust": []any{"VERSION"}}
				},
			},
			{
				name: "returns ErrCorrupt for a file that one language lists twice",
				give: func(object map[string]any) {
					object["languages"] = map[string]any{"go": []any{"VERSION", "VERSION"}}
				},
			},
			{
				name: "returns ErrCorrupt for a list of files that is null",
				give: func(object map[string]any) { object["languages"] = map[string]any{"go": nil} },
			},
			{
				name: "returns ErrCorrupt for a case index that the files do not list",
				give: func(object map[string]any) { object["cases"] = "spec/cases.json" },
			},
			{
				name: "returns ErrCorrupt for an invalid repository",
				give: func(object map[string]any) { object["repo"] = "assert-spec" },
			},
			{
				name: "returns ErrCorrupt for an invalid commit",
				give: func(object map[string]any) { object["commit"] = "27b9537" },
			},
			{
				name: "returns ErrCorrupt for a local copy with a commit",
				give: func(object map[string]any) { object["local"] = true },
			},
			{
				name: "returns ErrCorrupt for a copy without a commit that is not local",
				give: func(object map[string]any) { delete(object, "commit") },
			},
			{
				name: "returns ErrCorrupt for a definition with a commit",
				give: func(object map[string]any) { delete(object, "repo") },
			},
			{
				name: "returns ErrCorrupt for a definition that is local",
				give: func(object map[string]any) {
					delete(object, "repo")
					delete(object, "commit")
					object["local"] = true
				},
			},
			{
				name: "returns ErrCorrupt for a digest of a file that is no text",
				give: func(object map[string]any) { object["files"].(map[string]any)["VERSION"] = 1 },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				object := copyObject()
				tt.give(object)
				m, err := manifest.Decode(encode(t, object))
				assert.ErrorIs(t, err, manifest.ErrCorrupt, "Decode refuses a manifest that breaks a rule")
				assert.Equal(t, m, manifest.Manifest{}, "Decode returns the zero manifest with the error")
			})
		}

		raw := []struct {
			name string
			give string
		}{
			{name: "returns ErrCorrupt for data that is no JSON", give: "manifest"},
			{name: "returns ErrCorrupt for data after the object", give: string(encode(t, copyObject())) + "{}"},
			{name: "returns ErrCorrupt for null", give: "null"},
		}
		for _, tt := range raw {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				m, err := manifest.Decode([]byte(tt.give))
				assert.ErrorIs(t, err, manifest.ErrCorrupt, "Decode refuses data that is no manifest")
				assert.Equal(t, m, manifest.Manifest{}, "Decode returns the zero manifest with the error")
			})
		}
	})
}

// copyObject returns the JSON object of the manifest of a valid copy of go, for a case to break.
func copyObject() map[string]any {
	object := map[string]any{
		"commit": commit,
		"files": map[string]any{
			"VERSION":          manifest.Sum(contents["VERSION"]),
			"overlays/go.json": manifest.Sum(contents["overlays/go.json"]),
		},
		"languages": map[string]any{"go": []any{"overlays/go.json"}},
		"repo":      string(repo),
		"version":   version,
	}
	seal(object)
	return object
}

// seal sets the digest of the JSON object of a manifest to the Digest of those of its files whose
// digests are text.
func seal(object map[string]any) {
	digests := map[string]string{}
	for p, d := range object["files"].(map[string]any) {
		if text, ok := d.(string); ok {
			digests[p] = text
		}
	}
	object["digest"] = manifest.Digest(digests)
}

// encode returns the JSON of object.
func encode(t *testing.T, object map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(object)
	assert.NoError(t, err, "the object of a case encodes")
	return data
}
