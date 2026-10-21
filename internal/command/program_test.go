// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"bytes"
	"runtime/debug"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec/internal/command"
)

func TestProgram(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the help of the program with each command", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "--help")
			assert.Equal(t, res.Status, cli.ExitOK, "the help succeeds")
			expect.That(t, res.Stdout).
				Contains("manifest", "the help names the command manifest").
				Contains("sync", "the help names the command sync").
				Contains("check", "the help names the command check").
				Contains("verify", "the help names the command verify").
				Contains("propose", "the help names the command propose").
				Contains("status", "the help names the command status").
				Contains("try", "the help names the command try")
		})

		t.Run("writes the version of the build of the program", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{Build: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{Args: []string{"--version"}})
			assert.Equal(t, res.Status, cli.ExitOK, "the version succeeds")
			assert.Equal(t, res.Stdout, "spec v1.2.3\n", "the program writes the version of its module")
		})

		t.Run("writes (devel) for a build without its information", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "--version")
			assert.Equal(t, res.Status, cli.ExitOK, "the version succeeds")
			assert.Equal(t, res.Stdout, "spec (devel)\n", "the program writes the version of a working tree")
		})

		t.Run("returns 2 for an unknown command", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "unknown")
			assert.Equal(t, res.Status, cli.ExitUsage, "an unknown command is a usage error")
			assert.HasPrefix(t, res.Stderr, "spec: ", "the failure names the program")
		})

		t.Run("returns 1 without an environment", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			assert.Equal(t, program.Main(t.Context(), nil), cli.ExitFailure, "a run needs an environment")
		})

		t.Run("returns 1 without a standard error", func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			program := &command.Program{}
			status := program.Main(t.Context(), &cli.Env{Stdout: &stdout, Args: []string{"status"}})
			assert.Equal(t, status, cli.ExitFailure, "a run needs a standard error")
			assert.Empty(t, stdout.String(), "the run writes nothing")
		})
	})
}
