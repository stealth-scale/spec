// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package trial

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/vendored"
)

// Outcome is what [Try] or [Restore] did in the checkout of an implementation.
type Outcome string

// The outcomes of [Try] and [Restore].
const (
	// Passed is the outcome of a checkout whose test passed against the copy of the working tree.
	Passed Outcome = "passed"

	// Failed is the outcome of a checkout whose test ended with an exit status other than 0.
	Failed Outcome = "failed"

	// Untested is the outcome of a checkout that has the copy of the working tree and no test.
	Untested Outcome = "untested"

	// Restored is the outcome of a checkout whose copy from before the trial is back.
	Restored Outcome = "restored"

	// Unchanged is the outcome of a checkout without a copy from before a trial.
	Unchanged Outcome = "unchanged"
)

// Request is the trial of the working tree of a definition in the checkouts of its
// implementations.
type Request struct {
	// Tree is the definition at its working tree.
	Tree *definition.Tree

	// Stdout receives the standard output of each test.
	Stdout io.Writer

	// Stderr receives the standard error of each test.
	Stderr io.Writer

	// Dir is the directory of the checkouts. The checkout of each implementation of Tree is the
	// directory with the name of its repository below Dir, such as assert-go for
	// dokimasia/assert-go.
	Dir string
}

// Row is what [Try] or [Restore] did in the checkout of one implementation.
//
// # Concurrency
//
// A Row is a value, which any number of goroutines may read at once.
type Row struct {
	// Err is the error of the trial of the checkout, and nil for a trial without an error.
	Err error

	// Implementation is the implementation repository.
	Implementation spec.Repo

	// Checkout is the directory of the checkout.
	Checkout string

	// Outcome is what happened in the checkout, and empty when an error ended the trial before an
	// outcome.
	Outcome Outcome
}

// Result is what [Try] or [Restore] did in the checkout of each implementation.
//
// # Concurrency
//
// A Result is a value, which any number of goroutines may read at once.
type Result struct {
	// Rows has the row of each implementation, in the order of the configuration of the
	// definition.
	Rows []Row
}

// Try writes the copy of the working tree of req.Tree into the checkout of each implementation of
// the definition, and runs the test of its entry there. It first moves the copy of the checkout
// aside with vendored.Stash, so [Restore] can move it back. A second Try keeps the copy that the
// first one moved aside. The copy of the working tree is a local copy, without a commit. Each test
// runs in its checkout under ctx, one checkout at a time, and writes its output to req.Stdout and
// req.Stderr.
//
// A checkout that fails has the error in its row, and Try continues with the next checkout:
//   - the error of the file system for a checkout or a configuration file that cannot be read,
//     which wraps fs.ErrNotExist for a missing checkout.
//   - the errors of implementation.Parse and implementation.Find, and of vendored.Build.
//   - the errors of vendored.Stash and vendored.Overwrite, such as the error that wraps
//     fs.ErrNotExist for a checkout without a copy.
//   - the error of os/exec for a test that cannot start.
func Try(ctx context.Context, req *Request) Result {
	return each(req, func(root *os.Root, e *implementation.Entry, dir string) (Outcome, error) {
		c, err := vendored.Build(req.Tree, e, "")
		if err != nil {
			return "", fmt.Errorf("trial: %w", err)
		}
		if err = vendored.Stash(root, e); err != nil {
			return "", fmt.Errorf("trial: %w", err)
		}
		if err = vendored.Overwrite(root, c); err != nil {
			return "", fmt.Errorf("trial: %w", err)
		}
		if len(e.Test) == 0 {
			return Untested, nil
		}
		//nolint:gosec // The test is the command that the configuration file of the checkout states.
		cmd := exec.CommandContext(ctx, e.Test[0], e.Test[1:]...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, req.Stdout, req.Stderr
		err = cmd.Run()
		if _, failed := errors.AsType[*exec.ExitError](err); failed {
			return Failed, nil
		}
		if err != nil {
			return "", fmt.Errorf("trial: run the test of %s: %w", dir, err)
		}
		return Passed, nil
	})
}

// Restore moves the copy that [Try] moved aside back into the checkout of each implementation of
// the definition of req.Tree, with vendored.Unstash. A checkout without a copy aside has the outcome
// [Unchanged].
//
// A checkout that fails has the error in its row, and Restore continues with the next checkout:
//   - the error of the file system for a checkout or a configuration file that cannot be read.
//   - the errors of implementation.Parse and implementation.Find.
//   - the error of vendored.Unstash.
func Restore(req *Request) Result {
	return each(req, func(root *os.Root, e *implementation.Entry, _ string) (Outcome, error) {
		restored, err := vendored.Unstash(root, e)
		if err != nil {
			return "", fmt.Errorf("trial: %w", err)
		}
		if !restored {
			return Unchanged, nil
		}
		return Restored, nil
	})
}

// each runs step in the checkout of each implementation of the definition of req.Tree, and returns
// the row of each, as [Try] and [Restore] state.
func each(req *Request, step func(root *os.Root, e *implementation.Entry, dir string) (Outcome, error)) Result {
	result := Result{Rows: make([]Row, 0, len(req.Tree.Config.Implementations))}
	for _, impl := range req.Tree.Config.Implementations {
		dir := filepath.Join(req.Dir, impl.Repo.Name())
		outcome, err := visit(req.Tree.Config.Repo, dir, step)
		result.Rows = append(result.Rows, Row{Err: err, Implementation: impl.Repo, Checkout: dir, Outcome: outcome})
	}
	return result
}

// visit opens the checkout at dir, reads the entry of the definition def in its configuration file,
// and returns the outcome of step there. It closes the checkout after step.
//
// Error modes:
//   - the error of the file system for a checkout or a configuration file that cannot be read.
//   - the errors of implementation.Parse and implementation.Find.
//   - the error of step.
func visit(
	def spec.Repo, dir string, step func(root *os.Root, e *implementation.Entry, dir string) (Outcome, error),
) (outcome Outcome, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("trial: open the checkout %s: %w", dir, err)
	}
	//dokimi:mutate-skip sbr-delete: the close of an open root does not fail, and a caller cannot observe a root that is not closed
	defer func() { err = errors.Join(err, root.Close()) }()
	data, err := root.ReadFile(spec.ConfigFile)
	if err != nil {
		return "", fmt.Errorf("trial: read %s of %s: %w", spec.ConfigFile, dir, err)
	}
	entries, err := implementation.Parse(data)
	if err != nil {
		return "", fmt.Errorf("trial: %s: %w", dir, err)
	}
	e, err := implementation.Find(entries, def)
	if err != nil {
		return "", fmt.Errorf("trial: %s: %w", dir, err)
	}
	return step(root, e, dir)
}
