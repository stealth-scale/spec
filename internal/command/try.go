// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/trial"
)

// tryHelp is the help text of the command try.
const tryHelp = "Try the working tree of the definition in the checkouts of its implementations."

// tryHeader is the header of the table of the command try.
var tryHeader = []string{implementationColumn, "CHECKOUT", "OUTCOME"}

// Try is the options of the command try.
type Try struct {
	// Restore moves the copy from before the trial back into each checkout.
	Restore bool `help:"Move the copy from before the trial back into each checkout."`
}

// runTry runs trial.Try, or trial.Restore with --restore, for the definition of the repository of
// run. The checkouts are the directories beside the repository. It writes the implementation, the
// checkout and the outcome of each checkout to the standard output, after the output of the tests.
// A checkout with an error has no line in the table.
//
// Error modes:
//   - the error of the file system for a repository that cannot be opened, and the error of
//     definition.Open.
//   - the error of each checkout, joined, and an error that wraps [ErrFailed] for each checkout
//     whose test failed.
//   - an error that wraps the error of the standard output.
func runTry(ctx context.Context, run *cli.Run[Global, Try]) error {
	dir := run.Global().Dir
	var tree *definition.Tree
	err := inRoot(dir, func(root *os.Root) error {
		var err error
		if tree, err = definition.Open(root); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	req := &trial.Request{Tree: tree, Dir: filepath.Join(dir, ".."), Stdout: run.Stdout(), Stderr: run.Stderr()}
	var result trial.Result
	if run.Config().Restore {
		result = trial.Restore(req)
	} else {
		result = trial.Try(ctx, req)
	}
	var rows [][]string
	var failures []error
	for _, row := range result.Rows {
		if row.Err != nil {
			failures = append(failures, fmt.Errorf("command: %s: %w", row.Implementation, row.Err))
			continue
		}
		if row.Outcome == trial.Failed {
			failures = append(failures, fmt.Errorf("%w: %s", ErrFailed, row.Implementation))
		}
		rows = append(rows, []string{string(row.Implementation), row.Checkout, string(row.Outcome)})
	}
	if err := table(run.Stdout(), tryHeader, rows); err != nil {
		return err
	}
	return errors.Join(failures...)
}
