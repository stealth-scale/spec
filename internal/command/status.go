// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/status"
)

// statusHelp is the help text of the command status.
const statusHelp = "Report the copy of the definition of the repository in each of its implementations."

// statusHeader is the header of the table of the command status.
var statusHeader = []string{implementationColumn, versionColumn, commitColumn, "NEWER", pullColumn, "CHECKS"}

// Status is the options of the command status, which has none.
type Status struct{}

// runStatus writes the row of status.Status of each implementation that the definition of the
// repository of run registers to the standard output: the version and the commit of the copy,
// whether a newer copy exists, the open sync pull request and the state of its checks. A row with
// an error has no line in the table.
//
// Error modes:
//   - the errors of the configuration file of the repository, and the error of the client of
//     GitHub.
//   - the error of each row, joined.
//   - an error that wraps the error of the standard output.
func (p *Program) runStatus(ctx context.Context, run *cli.Run[Global, Status]) error {
	c, err := readDefinition(run.Global().Dir)
	if err != nil {
		return err
	}
	gh, err := p.client(run.Global(), readOnly)
	if err != nil {
		return err
	}
	repos := make([]spec.Repo, 0, len(c.Implementations))
	for _, impl := range c.Implementations {
		repos = append(repos, impl.Repo)
	}
	result := status.Status(ctx, gh, &status.Request{Definition: c.Repo, Implementations: repos})
	var rows [][]string
	var failures []error
	for _, row := range result.Rows {
		if row.Err != nil {
			failures = append(failures, fmt.Errorf("command: %s: %w", row.Implementation, row.Err))
			continue
		}
		rows = append(rows, []string{
			string(row.Implementation),
			cmp.Or(row.Version, none),
			short(row.Commit),
			flagCell(row.Newer),
			pullCell(row.Pull),
			string(row.Checks),
		})
	}
	if err := table(run.Stdout(), statusHeader, rows); err != nil {
		return err
	}
	return errors.Join(failures...)
}
