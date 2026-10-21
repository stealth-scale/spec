// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/internal/command"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/vendored"
)

// copyPath is the directory of the copy of the implementation of the cases in the separators of the
// system.
var copyPath = filepath.Join("conformance", "spec")

func TestSync(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the copy of the head of the track", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "sync")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.1.0", "2222222"), "the run states the copy")
			files.Equal(t, os.DirFS(filepath.Join(dir, copyPath)), treeOf(copyOf(t, second, secondCommit)), "the copy")
		})

		t.Run("writes the copy of the newest release for the track release", func(t *testing.T) {
			t.Parallel()
			h := newHub(t)
			h.repos[definitionRepo].releases["v1.0.0"] = firstCommit
			impl := implementationAt(t, first, firstCommit)
			impl[spec.ConfigFile] = strings.Replace(implConfig, "path:", "track: release\n    path:", 1)
			res := run(t, h, "-C", workspace(t, impl), "sync")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.0.0", "1111111"), "the copy comes from the release")
		})

		t.Run("writes the copy of the commit of --commit", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "sync", "--commit", sideCommit)
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.1.0", "3333333"), "the copy comes from the commit")
		})

		t.Run("writes the copy of the commit of its manifest with --locked", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "sync", "--locked")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.0.0", "1111111"), "the copy comes from its lock")
		})

		t.Run("writes a local copy of the checkout of --from", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, nil, "-C", dir, "sync", "--from", workspace(t, second))
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.1.0", "-"), "the copy comes from the checkout")
			files.Equal(t, os.DirFS(filepath.Join(dir, copyPath)), treeOf(copyOf(t, second, "")), "the local copy")
		})

		t.Run("replaces an edited copy with --force", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, merge(
				implementationAt(t, first, firstCommit),
				map[string]string{"conformance/spec/corpus/a.json": "edited\n"},
			))
			res := run(t, newHub(t), "-C", dir, "sync", "--force")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is replaced")
			files.Equal(t, os.DirFS(filepath.Join(dir, copyPath)), treeOf(copyOf(t, second, secondCommit)), "the copy")
		})

		t.Run("returns 1 for an edited copy without --force", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, merge(
				implementationAt(t, first, firstCommit),
				map[string]string{"conformance/spec/corpus/a.json": "edited\n"},
			))
			res := run(t, newHub(t), "-C", dir, "sync")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run refuses the edited copy")
		})

		t.Run("writes the copy of the definition of an argument", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "sync", definitionRepo)
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is written")
			assert.Equal(t, res.Stdout, copyTable("1.1.0", "2222222"), "the run states the copy")
		})

		t.Run("returns 1 for an argument without an entry", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "sync", otherRepo)
			assert.Equal(t, res.Status, cli.ExitFailure, "the run finds no entry")
		})

		t.Run("returns 1 for a lock of another definition with --locked", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, first, firstCommit)
			lock, err := manifest.Decode([]byte(impl["conformance/spec/manifest.json"]))
			assert.NoError(t, err, "the manifest of the copy decodes")
			lock.Repo = otherRepo
			impl["conformance/spec/manifest.json"] = string(lock.Encode())
			res := run(t, newHub(t), "-C", workspace(t, impl), "sync", "--locked")
			assert.Equal(t, res.Status, cli.ExitFailure, "the copy has no lock of its definition")
			assert.Contains(t, res.Stderr, "the copy has no lock", "the failure states the cause")
		})

		t.Run("returns 2 without a token", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args: []string{"-C", workspace(t, implementationAt(t, first, firstCommit)), "sync"},
			})
			assert.Equal(t, res.Status, cli.ExitUsage, "a run of GitHub needs a token")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args: []string{
					"-C", workspace(t, implementationAt(t, first, firstCommit)),
					"sync", "--from", workspace(t, second),
				},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})

		twoEntries := implConfig + "  - {definition: dokimasia/mutate-spec, path: conformance/mutate, languages: [go]}\n"
		other := merge(second, map[string]string{
			spec.ConfigFile: strings.Replace(definitionConfig, "assert-spec", "mutate-spec", 1),
		})

		t.Run("writes the copy of each definition of the configuration file", func(t *testing.T) {
			t.Parallel()
			h := newHub(t)
			h.repos[otherRepo] = &repository{
				commits:  map[string]map[string]string{firstCommit: other},
				branches: map[string]string{"main": firstCommit},
				releases: map[string]string{},
				history:  []string{firstCommit},
			}
			impl := merge(implementationAt(t, first, firstCommit), map[string]string{spec.ConfigFile: twoEntries})
			res := run(t, h, "-C", workspace(t, impl), "sync")
			assert.Equal(t, res.Status, cli.ExitOK, "the copies are written")
			assert.Equal(
				t,
				res.Stdout,
				"COPY                DEFINITION             VERSION  COMMIT\n"+
					"conformance/spec    dokimasia/assert-spec  1.1.0    2222222\n"+
					"conformance/mutate  dokimasia/mutate-spec  1.1.0    1111111\n",
				"the run states each copy in the order of the configuration file",
			)
		})

		tests := []struct {
			name       string
			giveImpl   map[string]string
			giveRemove []string
			giveArgs   []string
			giveFrom   map[string]string
			giveFail   string
			wantStatus int
			wantStderr string
		}{
			{
				name:       "returns 2 for --commit with --locked",
				giveArgs:   []string{"--commit", sideCommit, "--locked"},
				wantStatus: cli.ExitUsage,
				wantStderr: "--commit, --from and --locked exclude each other",
			},
			{
				name:       "returns 2 for --from with --commit",
				giveArgs:   []string{"--commit", sideCommit},
				giveFrom:   second,
				wantStatus: cli.ExitUsage,
				wantStderr: "--commit, --from and --locked exclude each other",
			},
			{
				name:       "returns 2 for --commit with two definitions",
				giveImpl:   map[string]string{spec.ConfigFile: twoEntries},
				giveArgs:   []string{"--commit", sideCommit},
				wantStatus: cli.ExitUsage,
				wantStderr: "--commit and --from sync one definition",
			},
			{
				name:       "returns 2 for --from with two definitions",
				giveImpl:   map[string]string{spec.ConfigFile: twoEntries},
				giveFrom:   second,
				wantStatus: cli.ExitUsage,
				wantStderr: "--commit and --from sync one definition",
			},
			{
				name:       "returns 2 for a checkout of --from of another definition",
				giveFrom:   other,
				wantStatus: cli.ExitUsage,
				wantStderr: "is dokimasia/mutate-spec, and the entry names dokimasia/assert-spec",
			},
			{
				name:       "returns 2 for a language that the checkout of --from lacks",
				giveImpl:   map[string]string{spec.ConfigFile: strings.Replace(implConfig, "[go]", "[python]", 1)},
				giveFrom:   second,
				wantStatus: cli.ExitUsage,
				wantStderr: "unknown language",
			},
			{
				name:       "returns 1 for a checkout of --from without the configuration file",
				giveFrom:   map[string]string{"VERSION": "1.0.0\n"},
				wantStatus: cli.ExitFailure,
				wantStderr: "read " + spec.ConfigFile,
			},
			{
				name:       "returns 1 for a copy without a manifest with --locked",
				giveRemove: []string{"conformance/spec/manifest.json"},
				giveArgs:   []string{"--locked"},
				wantStatus: cli.ExitFailure,
				wantStderr: "corrupt manifest",
			},
			{
				name:       "returns 1 for a local copy with --locked",
				giveImpl:   implementationAt(t, first, ""),
				giveArgs:   []string{"--locked"},
				wantStatus: cli.ExitFailure,
				wantStderr: "the manifest of conformance/spec names no commit of dokimasia/assert-spec",
			},
			{
				name:       "returns 1 for a repository without the configuration file",
				giveRemove: []string{spec.ConfigFile},
				wantStatus: cli.ExitFailure,
				wantStderr: "read " + spec.ConfigFile,
			},
			{
				name:       "returns 3 for an archive that GitHub fails",
				giveFail:   "tarball",
				wantStatus: command.ExitGitHub,
				wantStderr: "the files of dokimasia/assert-spec at " + secondCommit,
			},
			{
				name:       "returns 3 for a branch that GitHub fails",
				giveFail:   "branch",
				wantStatus: command.ExitGitHub,
				wantStderr: "the branch main of dokimasia/assert-spec",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				impl := merge(implementationAt(t, first, firstCommit), tt.giveImpl)
				for _, p := range tt.giveRemove {
					delete(impl, p)
				}
				args := append([]string{"-C", workspace(t, impl), "sync"}, tt.giveArgs...)
				if tt.giveFrom != nil {
					args = append(args, "--from", workspace(t, tt.giveFrom))
				}
				h := newHub(t)
				if tt.giveFail != "" {
					h.failures[tt.giveFail] = 500
				}
				res := run(t, h, args...)
				assert.Equal(t, res.Status, tt.wantStatus, "the run returns the status of its error")
				assert.Contains(t, res.Stderr, tt.wantStderr, "the failure states its cause")
			})
		}

		t.Run("returns 1 for a checkout of --from that does not exist", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, nil, "-C", dir, "sync", "--from", filepath.Join(t.TempDir(), "missing"))
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 1 for a repository that does not exist", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", filepath.Join(t.TempDir(), "missing"), "sync")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})
	})
}

// treeOf returns the tree of the files of c below its directory.
func treeOf(c *vendored.Copy) files.Tree {
	tree := files.Tree{}
	for p, content := range c.Files {
		if rel, ok := strings.CutPrefix(p, c.Dir+"/"); ok {
			tree[rel] = files.Bytes(content)
		}
	}
	return tree
}
