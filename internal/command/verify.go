// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec/conformance"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/vendored"
)

// verifyHelp is the help text of the command verify.
const verifyHelp = "Check that the report of a runner accounts for every case of its copy."

// Verify is the options of the command verify.
type Verify struct {
	// File is the report of a runner, or - for the standard input.
	File cli.InputFile `arg:"" help:"The report of a runner, or - for the standard input."`
}

// runVerify checks the report of run against the case index of the copy of the definition that the
// header of the report names, with conformance.Verify. It writes the definition, its version and
// the language of the report to the standard output.
//
// Error modes:
//   - the error of the input file of the report, and the error of conformance.DecodeReport.
//   - the error of the file system for a repository or a configuration file that cannot be opened
//     or read, and the errors of implementation.Parse and implementation.Find.
//   - the error of vendored.Check for a copy that is not intact.
//   - an error that wraps conformance.ErrIndex for a copy without a case index, and for a case index
//     that cannot be read or does not decode.
//   - the error of conformance.Verify, which wraps conformance.ErrVerify for a report that does not
//     account for the cases.
//   - an error that wraps the error of the standard output.
func runVerify(_ context.Context, run *cli.Run[Global, Verify]) error {
	file := run.Config().File
	f, err := run.Open(file)
	if err != nil {
		return fmt.Errorf("command: open the report %s: %w", file, err)
	}
	data, err := io.ReadAll(f)
	if err = errors.Join(err, f.Close()); err != nil {
		return fmt.Errorf("command: read the report %s: %w", file, err)
	}
	report, err := conformance.DecodeReport(data)
	if err != nil {
		return fmt.Errorf("command: %w", err)
	}
	return inRoot(run.Global().Dir, func(root *os.Root) error {
		entries, err := readEntries(root)
		if err != nil {
			return err
		}
		e, err := implementation.Find(entries, report.Header.Repo)
		if err != nil {
			return fmt.Errorf("command: %w", err)
		}
		m, err := vendored.Check(root, e)
		if err != nil {
			return fmt.Errorf("command: %w", err)
		}
		if m.Cases == "" {
			return fmt.Errorf("%w: the copy at %s has no case index", conformance.ErrIndex, e.Path)
		}
		// A case index that cannot be read is empty data, which DecodeIndex refuses, so one check
		// reports both errors.
		raw, readErr := root.ReadFile(filepath.FromSlash(path.Join(e.Path, m.Cases)))
		index, err := conformance.DecodeIndex(raw)
		if err != nil {
			return fmt.Errorf("command: the case index of %s: %w", e.Path, errors.Join(readErr, err))
		}
		if err = conformance.Verify(index, &m, report); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		_, err = fmt.Fprintf(
			run.Stdout(),
			"%s %s: the report of %s accounts for every case.\n",
			e.Definition, m.Version, report.Header.Language,
		)
		if err != nil {
			return fmt.Errorf("command: write the output: %w", err)
		}
		return nil
	})
}
