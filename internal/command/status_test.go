// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/internal/command"
	"go.stealthscale.io/spec/propose"
)

// statusHeader is the header of the table of a run of status.
const statusHeader = "IMPLEMENTATION       VERSION  COMMIT   NEWER  PULL  CHECKS\n"

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the copy in each implementation", func(t *testing.T) {
			t.Parallel()
			res := run(t, newHub(t), "-C", workspace(t, first), "status")
			assert.Equal(t, res.Status, cli.ExitOK, "the status succeeds")
			assert.Equal(
				t,
				res.Stdout,
				statusHeader+"dokimasia/assert-go  1.0.0    1111111  yes    -     success\n",
				"the row states the copy, the newer copy and the checks of the default branch",
			)
		})

		t.Run("reports the open sync pull request with the state of its checks", func(t *testing.T) {
			t.Parallel()
			h := newHub(t)
			branch := propose.BranchPrefix + definitionRepo
			h.repos[implRepo].pulls[branch] = 4
			h.repos[implRepo].checks[branch] = "failure"
			res := run(t, h, "-C", workspace(t, first), "status")
			assert.Equal(t, res.Status, cli.ExitOK, "the status succeeds")
			assert.Equal(
				t,
				res.Stdout,
				statusHeader+"dokimasia/assert-go  1.0.0    1111111  yes    #4    failure\n",
				"the row states the pull request and its checks",
			)
		})

		t.Run("returns 3 for an implementation that GitHub fails", func(t *testing.T) {
			t.Parallel()
			h := newHub(t)
			h.failures["tarball"] = 500
			res := run(t, h, "-C", workspace(t, first), "status")
			assert.Equal(t, res.Status, command.ExitGitHub, "the run returns the error of the row")
			assert.Equal(
				t,
				res.Stdout,
				"IMPLEMENTATION  VERSION  COMMIT  NEWER  PULL  CHECKS\n",
				"the table has no row of the implementation",
			)
		})

		t.Run("returns 1 for a repository without the configuration file", func(t *testing.T) {
			t.Parallel()
			res := run(t, newHub(t), "-C", t.TempDir(), "status")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
		})

		t.Run("returns 2 for a configuration file that breaks a rule", func(t *testing.T) {
			t.Parallel()
			dir := workspace(t, map[string]string{spec.ConfigFile: "definition: {repo: dokimasia/assert-spec}\n"})
			res := run(t, newHub(t), "-C", dir, "status")
			assert.Equal(t, res.Status, cli.ExitUsage, "a configuration that breaks a rule is a usage error")
		})

		t.Run("returns 2 without a token", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(
				t.Context(),
				program.Main,
				&clitest.Input{Args: []string{"-C", workspace(t, first), "status"}},
			)
			assert.Equal(t, res.Status, cli.ExitUsage, "a run of GitHub needs a token")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			res := start(t, newHub(t), &clitest.Input{
				Args:       []string{"-C", workspace(t, first), "status"},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}
