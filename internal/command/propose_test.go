// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec/internal/command"
	"go.stealthscale.io/spec/propose"
)

// The tables of a run of propose of the implementation of the cases, whose copy is current.
const (
	currentTable = "IMPLEMENTATION       DEFINITION             VERSION  ACTION   PULL\n" +
		"dokimasia/assert-go  dokimasia/assert-spec  1.1.0    current  -\n"
	closedTable = "IMPLEMENTATION       DEFINITION             VERSION  ACTION  PULL\n" +
		"dokimasia/assert-go  dokimasia/assert-spec  1.1.0    closed  #4\n"
)

func TestPropose(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the action of the implementation of --repo", func(t *testing.T) {
			t.Parallel()
			res := run(t, currentHub(t), "propose", "--repo", implRepo)
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is current")
			assert.Equal(t, res.Stdout, currentTable, "the run states the action")
		})

		t.Run("closes the open pull request of a current copy", func(t *testing.T) {
			t.Parallel()
			h := currentHub(t)
			h.repos[implRepo].pulls[propose.BranchPrefix+definitionRepo] = 4
			res := run(t, h, "propose", "--repo", implRepo)
			assert.Equal(t, res.Status, cli.ExitOK, "the pull request is closed")
			assert.Equal(t, res.Stdout, closedTable, "the run states the closed pull request")
		})

		t.Run("reports the action of each registered implementation with --all", func(t *testing.T) {
			t.Parallel()
			res := run(t, currentHub(t), "-C", workspace(t, first), "propose", "--all")
			assert.Equal(t, res.Status, cli.ExitOK, "the copy is current")
			assert.Equal(t, res.Stdout, currentTable, "the run states the action")
		})

		t.Run("returns 2 without --repo or --all", func(t *testing.T) {
			t.Parallel()
			res := run(t, currentHub(t), "propose")
			assert.Equal(t, res.Status, cli.ExitUsage, "the run needs a target")
		})

		t.Run("returns 2 for --repo with --all", func(t *testing.T) {
			t.Parallel()
			res := run(t, currentHub(t), "-C", workspace(t, first), "propose", "--repo", implRepo, "--all")
			assert.Equal(t, res.Status, cli.ExitUsage, "the run takes one target")
		})

		t.Run("returns 3 for an implementation that GitHub fails", func(t *testing.T) {
			t.Parallel()
			h := currentHub(t)
			h.failures["repository"] = 500
			res := run(t, h, "propose", "--repo", implRepo)
			assert.Equal(t, res.Status, command.ExitGitHub, "the run returns the error of GitHub")
		})

		t.Run("returns 1 for a repository without the configuration file with --all", func(t *testing.T) {
			t.Parallel()
			res := run(t, currentHub(t), "-C", t.TempDir(), "propose", "--all")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
		})

		t.Run("returns 2 without a token", func(t *testing.T) {
			t.Parallel()
			in := &clitest.Input{Args: []string{"propose", "--repo", implRepo}, Vars: map[string]string{}}
			res := start(t, newHub(t), in)
			assert.Equal(t, res.Status, cli.ExitUsage, "a proposal needs a token")
			assert.Contains(t, res.Stderr, "no token", "the failure states the cause")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			res := start(
				t,
				currentHub(t),
				&clitest.Input{Args: []string{"propose", "--repo", implRepo}, FailStdout: true},
			)
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}

// currentHub returns the hub of newHub with the copy of the implementation at the head of the
// definition.
func currentHub(t *testing.T) *hub {
	t.Helper()
	h := newHub(t)
	h.repos[implRepo].commits[implHead] = implementationAt(t, second, secondCommit)
	return h
}
