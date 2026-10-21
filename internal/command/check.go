// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// checkHelp is the help text of the command check.
const checkHelp = "Check that each copy of the repository is intact and comes from a commit."

// The statuses of a comparison of the commit of a copy with the head of a track, for a commit that
// is on the track.
const (
	statusAhead     = "ahead"
	statusIdentical = "identical"
)

// remoteHeader is the header of the table of the command check with --remote.
var remoteHeader = slices.Concat(copyHeader, []string{"TRACK", "HEAD", "NEWER"})

// Check is the options of the command check.
type Check struct {
	// Remote also checks that the commit of each copy is on the track of its entry, and reports
	// whether the head of the track has a newer copy.
	Remote bool `help:"Also check that each copy comes from its track, and report a newer copy."`
}

// runCheck checks each copy of the repository of run with vendored.Check, and refuses a local copy.
// With --remote it also checks that the commit of each copy is an ancestor of the head of the track
// of its entry, or that head itself. It then reports whether that head has a copy with another
// digest. It writes a row of each copy that it checked to the standard output, and continues with
// the next copy after a copy that fails.
//
// Error modes:
//   - the error of the file system for a repository or a configuration file that cannot be opened
//     or read, and the error of implementation.Parse.
//   - the error of vendored.Check, which wraps vendored.ErrNotIntact for a copy with problems.
//   - an error that wraps [ErrLocal] for a local copy.
//   - with --remote, an error that wraps [ErrOffTrack] for a commit that is not on the track, and
//     the errors of the client of GitHub, of remote.Resolve and of remote.Fetch.
//   - an error that wraps the error of the standard output.
func (p *Program) runCheck(ctx context.Context, run *cli.Run[Global, Check]) error {
	return inRoot(run.Global().Dir, func(root *os.Root) error {
		entries, err := readEntries(root)
		if err != nil {
			return err
		}
		var gh *github.Client
		header := copyHeader
		if run.Config().Remote {
			if gh, err = p.client(run.Global()); err != nil {
				return err
			}
			header = remoteHeader
		}
		var rows [][]string
		var failures []error
		for i := range entries {
			row, err := checkEntry(ctx, gh, root, &entries[i])
			if err != nil {
				failures = append(failures, err)
				continue
			}
			rows = append(rows, row)
		}
		if err := table(run.Stdout(), header, rows); err != nil {
			return err
		}
		return errors.Join(failures...)
	})
}

// checkEntry checks the copy of the entry e at root, and with a non-nil gh the track of e, and
// returns the row of the copy. It returns the errors that [Program.runCheck] states for one copy.
func checkEntry(ctx context.Context, gh *github.Client, root *os.Root, e *implementation.Entry) ([]string, error) {
	m, err := vendored.Check(root, e)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	if m.Local {
		return nil, fmt.Errorf("%w: %s", ErrLocal, e.Path)
	}
	row := make([]string, 0, len(remoteHeader))
	row = append(row, e.Path, string(e.Definition), m.Version, short(m.Commit))
	if gh == nil {
		return row, nil
	}
	target, err := remote.Resolve(ctx, gh, e)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	// The comparison with a commit that the definition lacks has no status, so the commit is not on
	// the track.
	comparison, _, err := gh.Compare(ctx, string(e.Definition), m.Commit, target.Commit)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	if comparison.Status != statusAhead && comparison.Status != statusIdentical {
		return nil, fmt.Errorf(
			"%w: the commit %s of %s is not on the track %s of %s",
			ErrOffTrack, m.Commit, e.Path, e.Track, e.Definition,
		)
	}
	c, err := remote.Fetch(ctx, gh, e, target.Commit)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	return append(row, e.Track, short(target.Commit), flagCell(c.Manifest.Digest != m.Digest)), nil
}
