// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package trial_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/trial"
	"go.stealthscale.io/spec/vendored"
)

// The repositories, the checkout and the commit of the cases.
const (
	goRepo      = spec.Repo("dokimasia/assert-go")
	rustRepo    = spec.Repo("dokimasia/assert-rust")
	checkout    = "assert-go"
	firstCommit = "1111111111111111111111111111111111111111"
)

// The arguments with which the test binary runs as the test of a checkout. passArg exits with 0
// in a directory with a configuration file, and failArg exits with 1.
const (
	passArg = "spec-trial-pass"
	failArg = "spec-trial-fail"
)

// The lines that the test binary writes as the test of a checkout.
const (
	passLine = "the test passed"
	failLine = "the test failed"
)

// definitionConfig configures the definition of the cases, which registers assert-go and
// assert-rust.
const definitionConfig = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
  implementations:
    - {repo: dokimasia/assert-go, languages: [go]}
    - {repo: dokimasia/assert-rust, languages: [rust]}
`

// implConfig configures the checkout of the cases: the go files of the definition at spec, and
// the version file at internal/VERSION.
const implConfig = `implements:
  - definition: dokimasia/assert-spec
    path: spec
    languages: [go]
    copies: [{from: VERSION, to: internal/VERSION}]
`

// first is the revision of the definition in the checkouts of the cases.
var first = map[string]string{
	spec.ConfigFile:      definitionConfig,
	"VERSION":            "1.0.0\n",
	"corpus/a.json":      `{"case": "a"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// second is the working tree of the definition of the cases, with a changed case.
var second = map[string]string{
	spec.ConfigFile:      definitionConfig,
	"VERSION":            "1.1.0\n",
	"corpus/a.json":      `{"case": "a", "version": 2}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// TestMain runs the test binary as the test of a checkout when its one argument is passArg or
// failArg, and runs the tests otherwise.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case passArg:
			if _, err := os.Stat(spec.ConfigFile); err != nil {
				os.Exit(2)
			}
			_, _ = fmt.Fprintln(os.Stdout, passLine)
			os.Exit(0)
		case failArg:
			_, _ = fmt.Fprintln(os.Stderr, failLine)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestTrial(t *testing.T) {
	t.Parallel()

	t.Run("Try", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the local copy of the working tree into a checkout without a test", func(t *testing.T) {
			t.Parallel()
			dir := checkoutAt(t, implConfig)
			result := trial.Try(t.Context(), request(t, dir, &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			assert.Equal(t, result.Rows[0], trial.Row{
				Implementation: goRepo,
				Checkout:       filepath.Join(dir, checkout),
				Outcome:        trial.Untested,
			}, "Try writes the copy and runs no test")
			files.Equal(
				t,
				os.DirFS(filepath.Join(dir, checkout, "spec")),
				treeOf(t, second, ""),
				"the checkout has the local copy of the working tree",
			)
			files.Equal(
				t,
				os.DirFS(filepath.Join(dir, checkout, ".spec.spec-try")),
				treeOf(t, first, firstCommit),
				"the copy of the checkout is aside",
			)
		})

		t.Run("returns fs.ErrNotExist for an implementation without a checkout", func(t *testing.T) {
			t.Parallel()
			result := trial.Try(t.Context(), request(t, checkoutAt(t, implConfig), &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			assert.Equal(t, result.Rows[1].Implementation, rustRepo, "the second row is of assert-rust")
			assert.ErrorIs(t, result.Rows[1].Err, fs.ErrNotExist, "Try finds no checkout of assert-rust")
		})

		t.Run("returns the error of a test that cannot start", func(t *testing.T) {
			t.Parallel()
			missing := filepath.Join(t.TempDir(), "missing")
			dir := checkoutAt(t, implConfig+"    test: ["+strconv.Quote(missing)+"]\n")
			result := trial.Try(t.Context(), request(t, dir, &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			start := exec.CommandContext(t.Context(), missing).Run()
			assert.HasError(t, start, "os/exec cannot start the missing program")
			assert.HasError(t, result.Rows[0].Err, "Try returns the error of the start")
			assert.Contains(t, result.Rows[0].Err.Error(), start.Error(), "the row states the error of os/exec")
		})

		t.Run("returns fs.ErrPermission for a copy that cannot be written", func(t *testing.T) {
			t.Parallel()
			dir := checkoutAt(t, implConfig)
			files.Write(
				t,
				dir,
				files.Tree{checkout + "/internal/VERSION": files.Text(first["VERSION"]).WithMode(0o444)},
			)
			result := trial.Try(t.Context(), request(t, dir, &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			assert.ErrorIs(t, result.Rows[0].Err, fs.ErrPermission, "Try returns the error of the write")
		})

		tests := []struct {
			name string
			give func(t *testing.T, dir string)
			want error
		}{
			{
				name: "returns fs.ErrNotExist for a checkout without a configuration file",
				give: func(t *testing.T, dir string) {
					t.Helper()
					assert.NoError(t, os.Remove(filepath.Join(dir, checkout, spec.ConfigFile)), "the case removes it")
				},
				want: fs.ErrNotExist,
			},
			{
				name: "returns implementation.ErrConfig for a configuration that breaks a rule",
				give: func(t *testing.T, dir string) {
					t.Helper()
					files.Write(t, dir, files.Tree{checkout + "/" + spec.ConfigFile: files.Text("implements: []\n")})
				},
				want: implementation.ErrConfig,
			},
			{
				name: "returns implementation.ErrEntry for a checkout without an entry of the definition",
				give: func(t *testing.T, dir string) {
					t.Helper()
					files.Write(t, dir, files.Tree{
						checkout + "/" + spec.ConfigFile: files.Text(
							strings.Replace(implConfig, "assert-spec", "mutate-spec", 1),
						),
					})
				},
				want: implementation.ErrEntry,
			},
			{
				name: "returns manifest.ErrLanguage for a language that the definition lacks",
				give: func(t *testing.T, dir string) {
					t.Helper()
					files.Write(t, dir, files.Tree{
						checkout + "/" + spec.ConfigFile: files.Text(
							strings.Replace(implConfig, "[go]", "[python]", 1),
						),
					})
				},
				want: manifest.ErrLanguage,
			},
			{
				name: "returns fs.ErrNotExist for a checkout without a copy",
				give: func(t *testing.T, dir string) {
					t.Helper()
					assert.NoError(t, os.RemoveAll(filepath.Join(dir, checkout, "spec")), "the case removes the copy")
				},
				want: fs.ErrNotExist,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				dir := checkoutAt(t, implConfig)
				tt.give(t, dir)
				result := trial.Try(t.Context(), request(t, dir, &bytes.Buffer{}, &bytes.Buffer{}))
				assert.Length(t, result.Rows, 2, "Try reports each implementation")
				assert.ErrorIs(t, result.Rows[0].Err, tt.want, "Try returns the error of the checkout")
			})
		}
	})

	t.Run("Restore", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the copy from before the trial back", func(t *testing.T) {
			t.Parallel()
			dir := checkoutAt(t, implConfig)
			req := request(t, dir, &bytes.Buffer{}, &bytes.Buffer{})
			trial.Try(t.Context(), req)
			result := trial.Restore(req)
			assert.Length(t, result.Rows, 2, "Restore reports each implementation")
			assert.Equal(t, result.Rows[0].Outcome, trial.Restored, "Restore moves the copy back")
			files.Equal(
				t,
				os.DirFS(filepath.Join(dir, checkout, "spec")),
				treeOf(t, first, firstCommit),
				"the checkout has its copy from before the trial",
			)
			files.HasContent(
				t,
				filepath.Join(dir, checkout, "internal", "VERSION"),
				first["VERSION"],
				"the copy of the version file is from before the trial",
			)
		})

		t.Run("returns Unchanged for a checkout without a copy aside", func(t *testing.T) {
			t.Parallel()
			dir := checkoutAt(t, implConfig)
			result := trial.Restore(request(t, dir, &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Restore reports each implementation")
			assert.Equal(t, result.Rows[0], trial.Row{
				Implementation: goRepo,
				Checkout:       filepath.Join(dir, checkout),
				Outcome:        trial.Unchanged,
			}, "Restore finds no copy aside")
		})

		t.Run("returns fs.ErrNotExist for an implementation without a checkout", func(t *testing.T) {
			t.Parallel()
			result := trial.Restore(request(t, checkoutAt(t, implConfig), &bytes.Buffer{}, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Restore reports each implementation")
			assert.ErrorIs(t, result.Rows[1].Err, fs.ErrNotExist, "Restore finds no checkout of assert-rust")
		})

		t.Run("returns fs.ErrNotExist for a copy aside without the file of a copy", func(t *testing.T) {
			t.Parallel()
			dir := checkoutAt(t, implConfig)
			req := request(t, dir, &bytes.Buffer{}, &bytes.Buffer{})
			trial.Try(t.Context(), req)
			assert.NoError(
				t,
				os.Remove(filepath.Join(dir, checkout, ".spec.spec-try", "VERSION")),
				"the case removes the version file of the copy aside",
			)
			result := trial.Restore(req)
			assert.Length(t, result.Rows, 2, "Restore reports each implementation")
			assert.ErrorIs(t, result.Rows[0].Err, fs.ErrNotExist, "Restore returns the error of the copy aside")
		})
	})
}

// TestTrialProcess runs the test binary as the test of a checkout. It runs its cases one at a time,
// because two processes of a coverage run that exit at once write one file of coverage data.
func TestTrialProcess(t *testing.T) {
	executable, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")

	t.Run("Try", func(t *testing.T) {
		t.Run("returns Passed for a test that exits with 0 in its checkout", func(t *testing.T) {
			dir := checkoutAt(t, implConfig+"    test: ["+strconv.Quote(executable)+", "+passArg+"]\n")
			var stdout bytes.Buffer
			result := trial.Try(t.Context(), request(t, dir, &stdout, &bytes.Buffer{}))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			assert.Equal(t, result.Rows[0].Outcome, trial.Passed, "the test passes")
			assert.Equal(t, stdout.String(), passLine+"\n", "Try writes the standard output of the test to Stdout")
		})

		t.Run("returns Failed for a test that exits with another status", func(t *testing.T) {
			dir := checkoutAt(t, implConfig+"    test: ["+strconv.Quote(executable)+", "+failArg+"]\n")
			var stderr bytes.Buffer
			result := trial.Try(t.Context(), request(t, dir, &bytes.Buffer{}, &stderr))
			assert.Length(t, result.Rows, 2, "Try reports each implementation")
			assert.Equal(t, result.Rows[0].Outcome, trial.Failed, "the test fails")
			assert.Equal(t, stderr.String(), failLine+"\n", "Try writes the standard error of the test to Stderr")
		})
	})
}

// checkoutAt returns a directory of the test with the checkout of assert-go: the configuration file
// config, and the copy of first at firstCommit.
func checkoutAt(t *testing.T, config string) string {
	t.Helper()
	entries, err := implementation.Parse([]byte(config))
	assert.NoError(t, err, "the configuration of the case parses")
	assert.Length(t, entries, 1, "the configuration of the case names one definition")
	c, err := vendored.Build(load(t, first), &entries[0], firstCommit)
	assert.NoError(t, err, "the copy of the case builds")
	tree := files.Tree{checkout + "/" + spec.ConfigFile: files.Text(config)}
	for _, p := range slices.Sorted(maps.Keys(c.Files)) {
		tree[checkout+"/"+p] = files.Bytes(c.Files[p])
	}
	return files.Workspace(t, tree)
}

// request returns the trial of second in the checkouts of dir, with the output of each test to
// stdout and stderr.
func request(t *testing.T, dir string, stdout, stderr *bytes.Buffer) *trial.Request {
	t.Helper()
	return &trial.Request{Tree: load(t, second), Dir: dir, Stdout: stdout, Stderr: stderr}
}

// load returns the definition of the text files contents.
func load(tb assert.TB, contents map[string]string) *definition.Tree {
	tb.Helper()
	files := make(map[string][]byte, len(contents))
	for p, text := range contents {
		files[p] = []byte(text)
	}
	tree, err := definition.Load(files)
	assert.NoError(tb, err, "the definition of the case loads")
	return tree
}

// treeOf returns the tree of the files of the copy of contents for the entry of implConfig at the
// commit at, below the directory of the copy.
func treeOf(t *testing.T, contents map[string]string, at string) files.Tree {
	t.Helper()
	entries, err := implementation.Parse([]byte(implConfig))
	assert.NoError(t, err, "the configuration of the case parses")
	assert.Length(t, entries, 1, "the configuration of the case names one definition")
	c, err := vendored.Build(load(t, contents), &entries[0], at)
	assert.NoError(t, err, "the copy of the case builds")
	tree := files.Tree{}
	for p, content := range c.Files {
		if rel, ok := strings.CutPrefix(p, c.Dir+"/"); ok {
			tree[rel] = files.Bytes(content)
		}
	}
	return tree
}
