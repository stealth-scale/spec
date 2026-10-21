// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/internal/command"
)

// manifestPath is the manifest of the definition of the cases in the separators of the system.
var manifestPath = filepath.Join("spec", "manifest.json")

func TestManifest(t *testing.T) {
	t.Parallel()

	m, err := load(t, first).Manifest()
	assert.NoError(t, err, "the manifest of the case builds")
	summary := "spec/manifest.json: 1.0.0, " + strconv.Itoa(len(m.Files)) + " files, " + m.Digest + "\n"

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the manifest of the definition", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, first)
			res := run(t, nil, "-C", dir, "manifest")
			assert.Equal(t, res.Status, cli.ExitOK, "the manifest is written")
			assert.Equal(t, res.Stdout, summary, "the run states the manifest")
			files.HasContent(t, filepath.Join(dir, manifestPath), string(m.Encode()), "the manifest of the files")
		})

		t.Run("returns 0 for a current manifest with --check", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, merge(first, map[string]string{"spec/manifest.json": string(m.Encode())}))
			res := run(t, nil, "-C", dir, "manifest", "--check")
			assert.Equal(t, res.Status, cli.ExitOK, "the manifest is current")
			assert.Equal(t, res.Stdout, summary, "the run states the manifest")
		})

		t.Run("returns 1 for a stale manifest with --check", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, merge(first, map[string]string{"spec/manifest.json": "{}\n"}))
			var res clitest.Result
			files.Unchanged(t, os.DirFS(dir), func() {
				res = run(t, nil, "-C", dir, "manifest", "--check")
			}, "the check writes nothing")
			assert.Equal(t, res.Status, cli.ExitFailure, "the check fails")
			assert.Contains(t, res.Stderr, "the manifest differs from the definition", "the failure states the cause")
		})

		t.Run("returns 1 for a missing manifest with --check", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", workspace(t, first), "manifest", "--check")
			assert.Equal(t, res.Status, cli.ExitFailure, "the check fails")
			assert.Contains(
				t,
				res.Stderr,
				"the manifest differs from the definition: spec/manifest.json",
				"a missing manifest is stale",
			)
		})

		t.Run("returns 1 for a manifest that cannot be read with --check", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, first)
			files.Write(t, dir, files.Tree{"spec/manifest.json": files.Dir()})
			res := run(t, nil, "-C", dir, "manifest", "--check")
			assert.Equal(t, res.Status, cli.ExitFailure, "the check returns the error of the read")
			assert.Contains(t, res.Stderr, "read spec/manifest.json", "the failure names the read")
		})

		t.Run("returns 1 for a manifest that cannot be written", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, first)
			files.Write(t, dir, files.Tree{"spec/manifest.json": files.Dir()})
			res := run(t, nil, "-C", dir, "manifest")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the write")
		})

		t.Run("returns 1 for a directory of the manifest that cannot be created", func(t *testing.T) {
			t.Parallel()
			config := definitionConfig + "  manifest: out/manifest.json\n"
			dir := workspace(t, merge(first, map[string]string{spec.ConfigFile: config, "out": "a file\n"}))
			res := run(t, nil, "-C", dir, "manifest")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the directory")
			assert.Contains(
				t,
				res.Stderr,
				"create the directory of out/manifest.json",
				"the failure names the directory",
			)
		})

		t.Run("returns 1 for a repository without the configuration file", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", t.TempDir(), "manifest")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
		})

		t.Run("returns 1 for a repository that does not exist", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", filepath.Join(t.TempDir(), "missing"), "manifest")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 2 for a file of two languages", func(t *testing.T) {
			t.Parallel()
			config := `definition:
  repo: dokimasia/assert-spec
  languages: [go, gox]
  files: [VERSION, "overlays/*.json"]
  language-files: ["overlays/{language}*.json"]
`
			dir := workspace(t, map[string]string{
				spec.ConfigFile:     config,
				"VERSION":           "1.0.0\n",
				"overlays/gox.json": "{}\n",
			})
			res := run(t, nil, "-C", dir, "manifest")
			assert.Equal(t, res.Status, cli.ExitUsage, "a configuration that breaks a rule is a usage error")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args:       []string{"-C", workspace(t, first), "manifest"},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}
