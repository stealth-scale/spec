// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/internal/command"
)

// reportFile is the report of a runner in the cases.
const reportFile = "results.jsonl"

func TestVerify(t *testing.T) {
	t.Parallel()

	digest := copyOf(t, first, firstCommit).Manifest.Digest
	header := `{"repo": "dokimasia/assert-spec", "commit": "` + firstCommit + `", "digest": "` + digest +
		`", "language": "go"}` + "\n"
	passing := header + `{"file": "corpus/a.json", "id": "a", "outcome": "pass"}` + "\n"

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a report that accounts for every case", func(t *testing.T) {
			t.Parallel()
			res := verify(t, implementationAt(t, first, firstCommit), &clitest.Input{
				Args:  []string{reportFile},
				Files: clitest.Files{reportFile: passing},
			})
			assert.Equal(t, res.Status, cli.ExitOK, "the report accounts for every case")
			assert.Equal(
				t,
				res.Stdout,
				"dokimasia/assert-spec 1.0.0: the report of go accounts for every case.\n",
				"the run states the definition and the language",
			)
		})

		t.Run("reads the report from the standard input", func(t *testing.T) {
			t.Parallel()
			res := verify(
				t,
				implementationAt(t, first, firstCommit),
				&clitest.Input{Args: []string{"-"}, Stdin: passing},
			)
			assert.Equal(t, res.Status, cli.ExitOK, "the report accounts for every case")
		})

		t.Run("returns 1 for a report with a failing case", func(t *testing.T) {
			t.Parallel()
			res := verify(t, implementationAt(t, first, firstCommit), &clitest.Input{
				Args:  []string{reportFile},
				Files: clitest.Files{reportFile: strings.Replace(passing, "pass", "fail", 1)},
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "a failing case fails the run")
			assert.Contains(t, res.Stderr, "a of corpus/a.json fails", "the failure names the case")
		})

		tests := []struct {
			name       string
			giveImpl   map[string]string
			giveReport string
			wantStderr string
		}{
			{
				name:       "returns 1 for a report that does not decode",
				giveImpl:   implementationAt(t, first, firstCommit),
				giveReport: "report\n",
				wantStderr: "invalid report: the header: ",
			},
			{
				name:       "returns 1 for a report of a definition without an entry",
				giveImpl:   implementationAt(t, first, firstCommit),
				giveReport: strings.Replace(passing, "assert-spec", "mutate-spec", 1),
				wantStderr: "no entry of the definition: dokimasia/mutate-spec",
			},
			{
				name: "returns 1 for a copy that is not intact",
				giveImpl: merge(
					implementationAt(t, first, firstCommit),
					map[string]string{"conformance/spec/corpus/a.json": "edited\n"},
				),
				giveReport: passing,
				wantStderr: "the copy at conformance/spec is not intact: corpus/a.json differs from its digest",
			},
			{
				name: "returns 1 for a copy without a case index",
				giveImpl: implementationAt(t, merge(first, map[string]string{
					spec.ConfigFile: strings.Replace(definitionConfig, "  cases: spec/cases.json\n", "", 1),
				}), firstCommit),
				giveReport: passing,
				wantStderr: "the copy at conformance/spec has no case index",
			},
			{
				name: "returns 1 for a case index that does not decode",
				giveImpl: implementationAt(
					t,
					merge(first, map[string]string{"spec/cases.json": "index\n"}),
					firstCommit,
				),
				giveReport: passing,
				wantStderr: "the case index of conformance/spec: ",
			},
			{
				name:       "returns 1 for a repository without the configuration file",
				giveImpl:   map[string]string{"go.mod": "module a\n"},
				giveReport: passing,
				wantStderr: "read " + spec.ConfigFile,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				res := verify(t, tt.giveImpl, &clitest.Input{
					Args:  []string{reportFile},
					Files: clitest.Files{reportFile: tt.giveReport},
				})
				assert.Equal(t, res.Status, cli.ExitFailure, "the run returns its error")
				assert.Contains(t, res.Stderr, tt.wantStderr, "the failure states its cause")
			})
		}

		t.Run("returns 1 for a missing report", func(t *testing.T) {
			t.Parallel()
			res := verify(t, implementationAt(t, first, firstCommit), &clitest.Input{Args: []string{reportFile}})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 1 for a report that cannot be read", func(t *testing.T) {
			t.Parallel()
			res := verify(
				t,
				implementationAt(t, first, firstCommit),
				&clitest.Input{Args: []string{"-"}, FailStdin: true},
			)
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the read")
			assert.Contains(t, res.Stderr, "read the report -", "the failure names the read")
		})

		t.Run("returns 1 for a repository that does not exist", func(t *testing.T) {
			t.Parallel()
			program := &command.Program{}
			res := clitest.Main(t.Context(), program.Main, &clitest.Input{
				Args:  []string{"-C", filepath.Join(t.TempDir(), "missing"), "verify", reportFile},
				Files: clitest.Files{reportFile: passing},
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the open")
		})

		t.Run("returns 1 for an output that cannot be written", func(t *testing.T) {
			t.Parallel()
			res := verify(t, implementationAt(t, first, firstCommit), &clitest.Input{
				Args:       []string{reportFile},
				Files:      clitest.Files{reportFile: passing},
				FailStdout: true,
			})
			assert.Equal(t, res.Status, cli.ExitFailure, "the run returns the error of the output")
		})
	})
}

// verify runs the command verify in a repository of the test with the files impl, with in after
// the name of the command.
func verify(t *testing.T, impl map[string]string, in *clitest.Input) clitest.Result {
	t.Helper()
	in.Args = append([]string{"-C", workspace(t, impl), "verify"}, in.Args...)
	program := &command.Program{}
	return clitest.Main(t.Context(), program.Main, in)
}
