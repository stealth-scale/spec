// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/prop"
	"go.dokimi.dev/assert/stateful"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/vendored"
)

func TestDisk(t *testing.T) {
	t.Parallel()

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the files of a new copy", func(t *testing.T) {
			t.Parallel()
			repoDir := files.Workspace(t, files.Tree{"go.mod": files.Text("module a\n")})
			c := build(t, first, &entry, commit)
			assert.NoError(t, vendored.Write(openRoot(t, repoDir), c), "Write writes a new copy")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(c),
				"the directory contains the copy alone",
			)
			files.HasContent(
				t,
				filepath.Join(repoDir, filepath.FromSlash(zones)),
				first["spec/zones.json"],
				"the zone table",
			)
		})

		t.Run("replaces an intact copy", func(t *testing.T) {
			t.Parallel()
			repoDir := t.TempDir()
			root := openRoot(t, repoDir)
			assert.NoError(t, vendored.Write(root, build(t, first, &entry, commit)), "Write writes the first copy")
			c := build(t, second, &entry, commit)
			assert.NoError(t, vendored.Write(root, c), "Write replaces the intact copy")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(c),
				"the directory contains the second copy alone",
			)
		})

		t.Run("returns ErrNotIntact for an edited copy", func(t *testing.T) {
			t.Parallel()
			repoDir := t.TempDir()
			root := openRoot(t, repoDir)
			assert.NoError(t, vendored.Write(root, build(t, first, &entry, commit)), "Write writes the first copy")
			files.Write(t, repoDir, files.Tree{dir + "/corpus/a.json": files.Text("edited\n")})
			var err error
			files.Unchanged(t, os.DirFS(repoDir), func() {
				err = vendored.Write(root, build(t, second, &entry, commit))
			}, "Write leaves an edited copy as it is")
			assert.ErrorIs(t, err, vendored.ErrNotIntact, "Write refuses an edited copy")
		})

		t.Run("returns ErrNotIntact for a directory without a manifest", func(t *testing.T) {
			t.Parallel()
			repoDir := files.Workspace(t, files.Tree{dir + "/a.json": files.Text("{}\n")})
			err := vendored.Write(openRoot(t, repoDir), build(t, first, &entry, commit))
			assert.ErrorIs(t, err, vendored.ErrNotIntact, "Write refuses a directory that is no copy")
		})

		t.Run("returns ErrNotIntact for a directory whose manifest does not decode", func(t *testing.T) {
			t.Parallel()
			repoDir := files.Workspace(t, files.Tree{dir + "/" + vendored.ManifestFile: files.Text("{}\n")})
			root := openRoot(t, repoDir)
			c := build(t, first, &entry, commit)
			var err error
			files.Unchanged(t, os.DirFS(repoDir), func() {
				err = vendored.Write(root, c)
			}, "Write leaves a directory with a corrupt manifest as it is")
			assert.ErrorIs(t, err, vendored.ErrNotIntact, "Write refuses a directory that is no copy")
		})

		t.Run("writes the file of each copy outside the directory", func(t *testing.T) {
			t.Parallel()
			const overlay = "internal/overlay/go.json"
			e := entry
			e.Copies = []implementation.Copy{
				{From: "spec/zones.json", To: zones},
				{From: "overlays/go.json", To: overlay},
			}
			repoDir := t.TempDir()
			assert.NoError(
				t,
				vendored.Write(openRoot(t, repoDir), build(t, first, &e, commit)),
				"Write writes the copy",
			)
			files.HasContent(
				t,
				filepath.Join(repoDir, filepath.FromSlash(zones)),
				first["spec/zones.json"],
				"the zone table",
			)
			files.HasContent(
				t,
				filepath.Join(repoDir, filepath.FromSlash(overlay)),
				first["overlays/go.json"],
				"the overlay of go",
			)
		})

		t.Run("leaves no directory beside the copy", func(t *testing.T) {
			t.Parallel()
			repoDir := files.Workspace(t, files.Tree{
				"conformance/.spec.spec-new/a": files.Text("left by a write that ended early\n"),
				"conformance/.spec.spec-old/b": files.Text("left by a write that ended early\n"),
			})
			c := build(t, first, &entry, commit)
			assert.NoError(t, vendored.Write(openRoot(t, repoDir), c), "Write writes the copy")
			files.Absent(t, filepath.Join(repoDir, "conformance", ".spec.spec-new"), "the staging directory is gone")
			files.Absent(t, filepath.Join(repoDir, "conformance", ".spec.spec-old"), "the old copy is gone")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(c),
				"the directory contains no file of the write that ended early",
			)
		})

		t.Run("leaves the other files of the repository as they are", func(t *testing.T) {
			t.Parallel()
			others := files.Tree{
				"go.mod":                files.Text("module a\n"),
				"conformance/README.md": files.Text("# Conformance\n"),
			}
			repoDir := files.Workspace(t, others)
			assert.NoError(
				t,
				vendored.Write(openRoot(t, repoDir), build(t, first, &entry, commit)),
				"Write writes the copy",
			)
			files.Contains(t, os.DirFS(repoDir), others, "the other files remain")
		})

		t.Run("returns the error of a call of the disk that fails", func(t *testing.T) {
			t.Parallel()
			c := build(t, second, &entry, commit)
			root, _ := writtenAt(t, first)
			counter := &failing{Disk: root}
			assert.NoError(t, vendored.Write(counter, c), "Write replaces the copy")
			for call := 1; call <= counter.calls; call++ {
				t.Run("returns the error of call "+strconv.Itoa(call), func(t *testing.T) {
					t.Parallel()
					root, _ := writtenAt(t, first)
					err := vendored.Write(&failing{Disk: root, fail: call}, c)
					assert.ErrorIs(t, err, errDisk, "Write returns the error of the disk")
				})
			}
		})

		t.Run("keeps a copy that Check accepts across every sequence of writes and edits", func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			var cases atomic.Int64
			revisions := []*vendored.Copy{build(t, first, &entry, commit), build(t, second, &entry, commit)}
			prop.ForAll(t, "a copy is intact after a write and not after an edit", func(c *prop.Case) {
				name := strconv.FormatInt(cases.Add(1), 10)
				files.Write(c, base, files.Tree{name: files.Dir()})
				repoDir := filepath.Join(base, name)
				root := openRoot(c, repoDir)
				var current *vendored.Copy
				edited := false
				revision := func(c *prop.Case, _ struct{}) any { return c.Draw(prop.Integer(0, 1), "revision") }
				stateful.Steps(c, stateful.Machine[struct{}]{
					Actions: []stateful.Action[struct{}]{
						{
							Name:  "write",
							Input: revision,
							Run: func(c *prop.Case, _ int, input any) {
								next := revisions[input.(int)]
								err := vendored.Write(root, next)
								if current != nil && edited {
									assert.ErrorIs(c, err, vendored.ErrNotIntact, "Write refuses an edited copy")
									return
								}
								assert.NoError(c, err, "Write replaces an intact copy")
								current, edited = next, false
							},
						},
						{
							Name:  "overwrite",
							Input: revision,
							Run: func(c *prop.Case, _ int, input any) {
								next := revisions[input.(int)]
								assert.NoError(c, vendored.Overwrite(root, next), "Overwrite replaces any copy")
								current, edited = next, false
							},
						},
						{
							Name:    "edit",
							Enabled: func(struct{}) bool { return current != nil },
							Run: func(c *prop.Case, _ int, _ any) {
								files.Write(c, repoDir, files.Tree{dir + "/corpus/a.json": files.Text("edited\n")})
								edited = true
							},
						},
						{
							Name: "check",
							Run: func(c *prop.Case, _ int, _ any) {
								m, err := vendored.Check(root, &entry)
								if current == nil || edited {
									assert.ErrorIs(
										c,
										err,
										vendored.ErrNotIntact,
										"Check refuses a missing or edited copy",
									)
									return
								}
								assert.NoError(c, err, "Check accepts the written copy")
								assert.Equal(c, m, current.Manifest, "Check returns the manifest of the written copy")
							},
						},
					},
					Invariant: func(c *prop.Case, _ struct{}) {
						files.Absent(
							c,
							filepath.Join(repoDir, "conformance", ".spec.spec-new"),
							"no staging directory remains",
						)
						files.Absent(c, filepath.Join(repoDir, "conformance", ".spec.spec-old"), "no old copy remains")
					},
				})
			})
		})
	})

	t.Run("Overwrite", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces an edited copy", func(t *testing.T) {
			t.Parallel()
			repoDir := t.TempDir()
			root := openRoot(t, repoDir)
			assert.NoError(t, vendored.Write(root, build(t, first, &entry, commit)), "Write writes the first copy")
			files.Write(t, repoDir, files.Tree{dir + "/corpus/a.json": files.Text("edited\n")})
			c := build(t, second, &entry, commit)
			assert.NoError(t, vendored.Overwrite(root, c), "Overwrite replaces the edited copy")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(c),
				"the directory contains the second copy alone",
			)
		})

		t.Run("replaces a directory without a manifest", func(t *testing.T) {
			t.Parallel()
			repoDir := files.Workspace(t, files.Tree{dir + "/a.json": files.Text("{}\n")})
			c := build(t, first, &entry, commit)
			assert.NoError(t, vendored.Overwrite(openRoot(t, repoDir), c), "Overwrite replaces the directory")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(c),
				"the directory contains the copy alone",
			)
		})

		t.Run("returns the error of a call of the disk that fails", func(t *testing.T) {
			t.Parallel()
			c := build(t, first, &entry, commit)
			counter := &failing{Disk: openRoot(t, t.TempDir())}
			assert.NoError(t, vendored.Overwrite(counter, c), "Overwrite writes the copy")
			for call := 1; call <= counter.calls; call++ {
				t.Run("returns the error of call "+strconv.Itoa(call), func(t *testing.T) {
					t.Parallel()
					err := vendored.Overwrite(&failing{Disk: openRoot(t, t.TempDir()), fail: call}, c)
					assert.ErrorIs(t, err, errDisk, "Overwrite returns the error of the disk")
				})
			}
		})
	})

	t.Run("Stash", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the copy aside", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			assert.NoError(t, vendored.Stash(root, &entry), "Stash moves the copy")
			files.Absent(t, filepath.Join(repoDir, copyPath), "the directory of the copy is empty")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, "conformance", ".spec.spec-try")),
				treeOf(build(t, first, &entry, commit)),
				"the copy is beside its directory",
			)
		})

		t.Run("keeps the copy that it moved aside earlier", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			assert.NoError(t, vendored.Stash(root, &entry), "Stash moves the first copy")
			second := build(t, second, &entry, commit)
			assert.NoError(t, vendored.Overwrite(root, second), "Overwrite writes the second copy")
			assert.NoError(t, vendored.Stash(root, &entry), "Stash keeps the first copy")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(second),
				"the directory contains the second copy",
			)
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, "conformance", ".spec.spec-try")),
				treeOf(build(t, first, &entry, commit)),
				"the first copy is still beside its directory",
			)
		})

		t.Run("returns fs.ErrNotExist for an entry without a copy", func(t *testing.T) {
			t.Parallel()
			err := vendored.Stash(openRoot(t, t.TempDir()), &entry)
			assert.ErrorIs(t, err, fs.ErrNotExist, "Stash returns the error of the move")
		})

		t.Run("returns the error of a call of the disk that fails", func(t *testing.T) {
			t.Parallel()
			root, _ := writtenAt(t, first)
			counter := &failing{Disk: root}
			assert.NoError(t, vendored.Stash(counter, &entry), "Stash moves the copy")
			for call := 1; call <= counter.calls; call++ {
				t.Run("returns the error of call "+strconv.Itoa(call), func(t *testing.T) {
					t.Parallel()
					root, _ := writtenAt(t, first)
					err := vendored.Stash(&failing{Disk: root, fail: call}, &entry)
					assert.ErrorIs(t, err, errDisk, "Stash returns the error of the disk")
				})
			}
		})
	})

	t.Run("Unstash", func(t *testing.T) {
		t.Parallel()

		t.Run("restores the copy that Stash moved aside", func(t *testing.T) {
			t.Parallel()
			root, repoDir := stashedAt(t)
			restored, err := vendored.Unstash(root, &entry)
			assert.NoError(t, err, "Unstash restores the copy")
			assert.True(t, restored, "Unstash reports the restore")
			files.Equal(
				t,
				os.DirFS(filepath.Join(repoDir, copyPath)),
				treeOf(build(t, first, &entry, commit)),
				"the first copy is back in its directory",
			)
			files.Absent(t, filepath.Join(repoDir, "conformance", ".spec.spec-try"), "nothing is beside the copy")
			files.HasContent(
				t,
				filepath.Join(repoDir, filepath.FromSlash(zones)),
				first["spec/zones.json"],
				"the zone table of the first copy is back",
			)
		})

		t.Run("reports false without a copy aside", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			var restored bool
			var err error
			files.Unchanged(t, os.DirFS(repoDir), func() {
				restored, err = vendored.Unstash(root, &entry)
			}, "Unstash changes nothing")
			assert.NoError(t, err, "Unstash finds no copy aside")
			assert.False(t, restored, "Unstash reports no restore")
		})

		t.Run("returns fs.ErrNotExist for a restored copy without the file of a copy", func(t *testing.T) {
			t.Parallel()
			root, repoDir := stashedAt(t)
			assert.NoError(
				t,
				os.Remove(filepath.Join(repoDir, "conformance", ".spec.spec-try", "spec", "zones.json")),
				"the case removes the zone table of the copy aside",
			)
			_, err := vendored.Unstash(root, &entry)
			assert.ErrorIs(t, err, fs.ErrNotExist, "Unstash returns the error of the read")
		})

		t.Run("returns the error of a call of the disk that fails", func(t *testing.T) {
			t.Parallel()
			root, _ := stashedAt(t)
			counter := &failing{Disk: root}
			_, err := vendored.Unstash(counter, &entry)
			assert.NoError(t, err, "Unstash restores the copy")
			for call := 1; call <= counter.calls; call++ {
				t.Run("returns the error of call "+strconv.Itoa(call), func(t *testing.T) {
					t.Parallel()
					root, _ := stashedAt(t)
					_, err := vendored.Unstash(&failing{Disk: root, fail: call}, &entry)
					assert.ErrorIs(t, err, errDisk, "Unstash returns the error of the disk")
				})
			}
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest of an intact copy", func(t *testing.T) {
			t.Parallel()
			root, _ := writtenAt(t, first)
			m, err := vendored.Check(root, &entry)
			assert.NoError(t, err, "Check accepts the copy that Write wrote")
			assert.Equal(t, m, build(t, first, &entry, commit).Manifest, "Check returns the manifest of the copy")
		})

		t.Run("returns the manifest of a copy of every language", func(t *testing.T) {
			t.Parallel()
			e := entry
			e.Languages = []spec.Language{spec.All}
			c := build(t, first, &e, commit)
			root := openRoot(t, t.TempDir())
			assert.NoError(t, vendored.Write(root, c), "Write writes a copy of every language")
			m, err := vendored.Check(root, &e)
			assert.NoError(t, err, "Check accepts a copy of every language")
			assert.Equal(t, m, c.Manifest, "Check returns the manifest of the copy")
		})

		t.Run("returns ErrNotIntact for a missing copy", func(t *testing.T) {
			t.Parallel()
			_, err := vendored.Check(openRoot(t, t.TempDir()), &entry)
			assert.ErrorIs(t, err, vendored.ErrNotIntact, "Check refuses a repository without the copy")
		})

		t.Run("returns ErrNotIntact for a corrupt manifest", func(t *testing.T) {
			t.Parallel()
			const corrupt = "{}\n"
			_, want := manifest.Decode([]byte(corrupt))
			assert.HasError(t, want, "the manifest of the case does not decode")
			repoDir := files.Workspace(t, files.Tree{dir + "/" + vendored.ManifestFile: files.Text(corrupt)})
			_, err := vendored.Check(openRoot(t, repoDir), &entry)
			problems := assert.ErrorAs[*vendored.NotIntactError](
				t,
				err,
				"Check refuses a manifest that does not decode",
			)
			assert.Equal(t, problems.Problems, []string{want.Error()}, "the problem is the error of the decode")
		})

		tests := []struct {
			name      string
			giveTree  files.Tree
			giveEntry func(e *implementation.Entry)
			want      string
		}{
			{
				name:     "returns ErrNotIntact for a file that differs from its digest",
				giveTree: files.Tree{dir + "/corpus/a.json": files.Text("edited\n")},
				want:     "corpus/a.json differs from its digest",
			},
			{
				name:     "returns ErrNotIntact for a file that the manifest does not list",
				giveTree: files.Tree{dir + "/corpus/z.json": files.Text("{}\n")},
				want:     "the manifest does not list corpus/z.json",
			},
			{
				name:      "returns ErrNotIntact for a copy of another definition",
				giveEntry: func(e *implementation.Entry) { e.Definition = "dokimasia/mutate-spec" },
				want:      `the copy comes from "dokimasia/assert-spec", and the entry names dokimasia/mutate-spec`,
			},
			{
				name:      "returns ErrNotIntact for a copy of other languages",
				giveEntry: func(e *implementation.Entry) { e.Languages = []spec.Language{"go", "rust"} },
				want:      "the copy vendors [go], and the entry names [go rust]",
			},
			{
				name:     "returns ErrNotIntact for a copy to a file with other content",
				giveTree: files.Tree{zones: files.Text("{}\n")},
				want:     zones + " differs from spec/zones.json of the copy",
			},
			{
				name: "returns ErrNotIntact for a copy of a file that the copy lacks",
				giveEntry: func(e *implementation.Entry) {
					e.Copies = []implementation.Copy{{From: "overlays/rust.json", To: zones}}
				},
				want: "the copy has no overlays/rust.json for the copy to " + zones,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				root, repoDir := writtenAt(t, first)
				if tt.giveTree != nil {
					files.Write(t, repoDir, tt.giveTree)
				}
				e := entry
				if tt.giveEntry != nil {
					tt.giveEntry(&e)
				}
				_, err := vendored.Check(root, &e)
				problems := assert.ErrorAs[*vendored.NotIntactError](t, err, "Check reports the problems of the copy")
				assert.Equal(t, problems.Problems, []string{tt.want}, "Check reports the problem of the case alone")
			})
		}

		t.Run("returns ErrNotIntact for a file of the copy that is missing", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			assert.NoError(
				t,
				os.Remove(filepath.Join(repoDir, copyPath, "corpus", "b.json")),
				"the case removes a file",
			)
			_, err := vendored.Check(root, &entry)
			problems := assert.ErrorAs[*vendored.NotIntactError](t, err, "Check reports the problems of the copy")
			assert.Equal(
				t,
				problems.Problems,
				[]string{"corpus/b.json is missing"},
				"the problem names the missing file",
			)
		})

		t.Run("returns ErrNotIntact for a copy to a file that is missing", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			assert.NoError(
				t,
				os.Remove(filepath.Join(repoDir, filepath.FromSlash(zones))),
				"the case removes the zone table",
			)
			_, err := vendored.Check(root, &entry)
			problems := assert.ErrorAs[*vendored.NotIntactError](t, err, "Check reports the problems of the copy")
			assert.Equal(t, problems.Problems, []string{zones + " is missing"}, "the problem names the missing file")
		})

		t.Run("returns ErrNotIntact for a link in the copy", func(t *testing.T) {
			t.Parallel()
			root, repoDir := writtenAt(t, first)
			files.Write(t, repoDir, files.Tree{dir + "/corpus/link.json": files.Link("a.json")})
			_, err := vendored.Check(root, &entry)
			problems := assert.ErrorAs[*vendored.NotIntactError](t, err, "Check reports the problems of the copy")
			assert.Equal(
				t,
				problems.Problems,
				[]string{"corpus/link.json is no regular file"},
				"the problem names the link",
			)
		})

		t.Run("returns fs.ErrPermission for a directory of the copy that cannot be read", func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skip("Windows stores no permission bit that forbids the read of a directory")
			}
			root, repoDir := writtenAt(t, first)
			files.Write(t, repoDir, files.Tree{dir + "/corpus": files.Dir().WithMode(0o000)})
			_, err := vendored.Check(root, &entry)
			assert.ErrorIs(t, err, fs.ErrPermission, "Check returns the error of the walk")
		})

		t.Run("returns the error of a call of the disk that fails", func(t *testing.T) {
			t.Parallel()
			root, _ := writtenAt(t, first)
			counter := &failing{Disk: root}
			_, err := vendored.Check(counter, &entry)
			assert.NoError(t, err, "Check accepts the copy")
			for call := 1; call <= counter.calls; call++ {
				t.Run("returns the error of call "+strconv.Itoa(call), func(t *testing.T) {
					t.Parallel()
					root, _ := writtenAt(t, first)
					_, err := vendored.Check(&failing{Disk: root, fail: call}, &entry)
					assert.ErrorIs(t, err, errDisk, "Check returns the error of the disk")
				})
			}
		})
	})
}

// writtenAt returns the root and the directory of a repository of the test with the copy of
// contents for entry. The test removes the directory also after a case locks a directory in it.
func writtenAt(t *testing.T, contents map[string]string) (*os.Root, string) {
	t.Helper()
	repoDir := files.Workspace(t, files.Tree{})
	root := openRoot(t, repoDir)
	assert.NoError(t, vendored.Write(root, build(t, contents, &entry, commit)), "Write writes the copy of the case")
	return root, repoDir
}

// stashedAt returns the root and the directory of a repository of the test whose copy of first for
// entry Stash moved aside, and whose directory of the copy then received the copy of second.
func stashedAt(t *testing.T) (*os.Root, string) {
	t.Helper()
	root, repoDir := writtenAt(t, first)
	assert.NoError(t, vendored.Stash(root, &entry), "Stash moves the first copy aside")
	assert.NoError(t, vendored.Overwrite(root, build(t, second, &entry, commit)), "Overwrite writes the second copy")
	return root, repoDir
}

// treeOf returns the tree of the files of c below its directory.
func treeOf(c *vendored.Copy) files.Tree {
	tree := files.Tree{}
	for _, p := range slices.Sorted(maps.Keys(c.Files)) {
		if rel, ok := strings.CutPrefix(p, c.Dir+"/"); ok {
			tree[rel] = files.Bytes(c.Files[p])
		}
	}
	return tree
}
