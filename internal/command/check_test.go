// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/internal/command"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an intact copy", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", workspace(t, implementationAt(t, first, firstCommit)), "check")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is intact")
			assert.Equal(t, res.Stdout, copyTable("1.0.0", "1111111"), "the run states the copy")
		})

		t.Run("returns 1 for an edited copy", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, merge(
				implementationAt(t, first, firstCommit),
				map[string]string{"conformance/spec/corpus/a.json": "edited\n"},
			))
			res := run(t, nil, "-C", dir, "check")
			assert.Equal(t, res.Status, cli.ExitFailure, "the copy is not intact")
			assert.Contains(t, res.Stderr, "corpus/a.json", "the failure names the edited file")
		})

		t.Run("returns 1 for a local copy", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", workspace(t, implementationAt(t, first, "")), "check")
			assert.Equal(t, res.Status, cli.ExitFailure, "a copy needs a commit")
			assert.Contains(t, res.Stderr, "the copy comes from a checkout", "the failure states the cause")
		})

		t.Run("reports a newer copy at the head of the track with --remote", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			res := run(t, newHub(t), "-C", dir, "check", "--remote")
			assert.Equal(t, res.Status, cli.ExitOK, "the commit of the copy is on the track")
			assert.Equal(t, res.Stdout, remoteTable("1.0.0", "1111111", "yes"), "the head has a newer copy")
		})

		t.Run("reports no newer copy for a copy of the head with --remote", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, second, secondCommit))
			res := run(t, newHub(t), "-C", dir, "check", "--remote")
			assert.Equal(t, res.Status, cli.ExitOK, "the commit of the copy is the head of the track")
			assert.Equal(t, res.Stdout, remoteTable("1.1.0", "2222222", "no"), "the copy is the copy of the head")
		})

		tests := []struct {
			name       string
			giveCommit string
			giveFail   string
			wantStatus int
		}{
			{
				name:       "returns 1 for a copy of a commit off its track with --remote",
				giveCommit: sideCommit,
				wantStatus: cli.ExitFailure,
			},
			{
				name:       "returns 1 for a copy of a commit that the definition lacks with --remote",
				giveCommit: missingCommit,
				wantStatus: cli.ExitFailure,
			},
			{
				name:       "returns 3 for a comparison that GitHub fails with --remote",
				giveCommit: firstCommit,
				giveFail:   "compare",
				wantStatus: command.ExitGitHub,
			},
			{
				name:       "returns 3 for a branch that GitHub fails with --remote",
				giveCommit: firstCommit,
				giveFail:   "branch",
				wantStatus: command.ExitGitHub,
			},
			{
				name:       "returns 3 for an archive that GitHub fails with --remote",
				giveCommit: firstCommit,
				giveFail:   "tarball",
				wantStatus: command.ExitGitHub,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				h := newHub(t)
				if tt.giveFail != "" {
					h.failures[tt.giveFail] = 500
				}
				dir := workspace(t, implementationAt(t, first, tt.giveCommit))
				res := run(t, h, "-C", dir, "check", "--remote")
				assert.Equal(t, res.Status, tt.wantStatus, "the run returns the status of its error")
			})
		}

		t.Run("reports a newer copy without a token with --remote", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, implementationAt(t, first, firstCommit))
			in := &clitest.Input{Args: []string{"-C", dir, "check", "--remote"}, Vars: map[string]string{}}
			res := start(t, newHub(t), in)
			assert.Equal(t, res.Status, cli.ExitOK, "the run reads a public definition without a token")
			assert.Equal(t, res.Stdout, remoteTable("1.0.0", "1111111", "yes"), "the head has a newer copy")
		})

		t.Run("returns 2 for an anonymous run without a transport with --remote", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args: []string{"-C", workspace(t, implementationAt(t, first, firstCommit)), "check", "--remote"},
			})
			assert.Equal(t, res.Status, cli.ExitUsage, "a run without a token needs a transport")
			assert.Contains(t, res.Stderr, "invalid configuration", "the failure states the cause")
		})

		t.Run("returns 1 for a repository without the configuration file", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, first, firstCommit)
			delete(impl, spec.ConfigFile)
			res := run(t, nil, "-C", workspace(t, impl), "check")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
		})

		t.Run("returns 1 for a repository that does not exist", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", filepath.Join(t.TempDir(), "missing"), "check")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 2 for a configuration file that breaks a rule", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", workspace(t, map[string]string{spec.ConfigFile: "implements: []\n"}), "check")
			assert.Equal(t, res.Status, cli.ExitUsage, "a configuration that breaks a rule is a usage error")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args:       []string{"-C", workspace(t, implementationAt(t, first, firstCommit)), "check"},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}

// remoteTable returns the table of a run of check with --remote of the copy of the implementation of
// the cases at version and commit, whose track main has its head at secondCommit.
func remoteTable(version, commit, newer string) string {
	return "COPY              DEFINITION             VERSION  COMMIT   TRACK  HEAD     NEWER\n" +
		"conformance/spec  dokimasia/assert-spec  " + version + "    " + commit + "  main   2222222  " + newer + "\n"
}
