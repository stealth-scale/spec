// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
)

// failArg is the argument with which the test binary runs as a test of a checkout that fails.
const failArg = "spec-command-fail"

// failLine is the line that the test binary writes as a test of a checkout that fails.
const failLine = "the test failed"

// TestMain runs the test binary as a test of a checkout that fails when its one argument is
// failArg, and runs the tests otherwise.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == failArg {
		_, _ = fmt.Fprintln(os.Stderr, failLine)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestTry(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the working tree into each checkout", func(t *testing.T) {
			t.Parallel()
			parent := checkouts(t, implementationAt(t, first, firstCommit))
			res := run(t, nil, "-C", filepath.Join(parent, "assert-spec"), "try")
			assert.Equal(t, res.Status, cli.ExitOK, "the trial succeeds")
			assert.Contains(t, res.Stdout, "untested", "the row states the outcome")
			files.Equal(
				t,
				os.DirFS(filepath.Join(parent, "assert-go", copyPath)),
				treeOf(copyOf(t, second, "")),
				"the checkout has the local copy of the working tree",
			)
		})

		t.Run("moves the copy from before the trial back with --restore", func(t *testing.T) {
			t.Parallel()
			parent := checkouts(t, implementationAt(t, first, firstCommit))
			definition := filepath.Join(parent, "assert-spec")
			assert.Equal(t, run(t, nil, "-C", definition, "try").Status, cli.ExitOK, "the trial succeeds")
			res := run(t, nil, "-C", definition, "try", "--restore")
			assert.Equal(t, res.Status, cli.ExitOK, "the restore succeeds")
			assert.Contains(t, res.Stdout, "restored", "the row states the outcome")
			files.Equal(
				t,
				os.DirFS(filepath.Join(parent, "assert-go", copyPath)),
				treeOf(copyOf(t, first, firstCommit)),
				"the checkout has its copy from before the trial",
			)
		})

		t.Run("returns 1 for an implementation without a checkout", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", filepath.Join(checkouts(t, nil), "assert-spec"), "try")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the checkout")
		})

		t.Run("returns 1 for a repository without the configuration file", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", t.TempDir(), "try")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
		})

		t.Run("returns 1 for a repository that does not exist", func(t *testing.T) {
			t.Parallel()
			res := run(t, nil, "-C", filepath.Join(t.TempDir(), "missing"), "try")
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			parent := checkouts(t, implementationAt(t, first, firstCommit))
			res := start(t, nil, &clitest.Input{
				Args:       []string{"-C", filepath.Join(parent, "assert-spec"), "try"},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}

// TestTryProcess runs the test binary as the test of a checkout. It runs its cases one at a time,
// because two processes of a coverage run that exit at once write one file of coverage data.
func TestTryProcess(t *testing.T) {
	executable, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")

	t.Run("Main", func(t *testing.T) {
		t.Run("returns 1 for a test that fails", func(t *testing.T) {
			impl := implementationAt(t, first, firstCommit)
			impl[spec.ConfigFile] = implConfig + "    test: [" + strconv.Quote(executable) + ", " + failArg + "]\n"
			parent := checkouts(t, impl)
			res := run(t, nil, "-C", filepath.Join(parent, "assert-spec"), "try")
			assert.Equal(t, res.Status, cli.ExitFailure, "the failed test fails the run")
			assert.Contains(t, res.Stdout, "failed", "the row states the outcome")
			assert.Contains(t, res.Stderr, failLine, "the run writes the standard error of the test")
		})
	})
}

// checkouts returns a directory of the test with the definition of the cases at second in
// assert-spec, and the checkout of assert-go with the files impl, or no checkout for a nil impl.
func checkouts(t *testing.T, impl map[string]string) string {
	t.Helper()
	tree := map[string]string{}
	for p, content := range second {
		tree["assert-spec/"+p] = content
	}
	for p, content := range impl {
		tree["assert-go/"+p] = content
	}
	return workspace(t, tree)
}
