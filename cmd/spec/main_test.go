// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"testing"

	"go.dokimi.dev/assert"
)

// mainVariable is the environment variable whose presence makes the test binary run main.
const mainVariable = "SPEC_TEST_MAIN"

// TestMain runs main when the environment has mainVariable, and runs the tests otherwise.
func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv(mainVariable); ok {
		main()
	}
	os.Exit(m.Run())
}

// TestMainProcess runs the test binary as the command. It runs its cases one at a time, because
// two processes of a coverage run that exit at once write one file of coverage data.
func TestMainProcess(t *testing.T) {
	executable, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")

	t.Run("main", func(t *testing.T) {
		t.Run("writes the version of the build", func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), executable, "--version")
			cmd.Env = append(os.Environ(), mainVariable+"=")
			out, err := cmd.Output()
			assert.NoError(t, err, "the command writes its version")
			assert.HasPrefix(t, string(out), "spec ", "the version names the command")
		})

		t.Run("exits with 2 for an unknown command", func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), executable, "unknown")
			cmd.Env = append(os.Environ(), mainVariable+"=")
			exit := assert.ErrorAs[*exec.ExitError](t, cmd.Run(), "the command exits with a status other than 0")
			assert.Equal(t, exit.ExitCode(), 2, "an unknown command is a usage error")
		})
	})
}
