// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"context"
	"errors"
	"fmt"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/propose"
)

// proposeHelp is the help text of the command propose.
const proposeHelp = "Open, update or close the pull requests that sync the copies of implementations."

// proposeHeader is the header of the table of the command propose.
var proposeHeader = []string{implementationColumn, definitionColumn, versionColumn, "ACTION", pullColumn}

// Propose is the options of the command propose.
type Propose struct {
	// Repo is the implementation repository to sync.
	Repo spec.Repo `help:"The implementation repository to sync."`

	// All syncs every implementation that the definition of the repository registers.
	All bool `help:"Sync every implementation that the definition of the repository registers."`
}

// runPropose runs propose.Propose for the implementation of --repo, or for every implementation
// that the definition of the repository of run registers with --all. It writes the implementation,
// the definition, the version, the action and the pull request of each proposal to the standard
// output, and continues with the next implementation after an implementation that fails.
//
// Error modes:
//   - an error that wraps [ErrUsage] for neither or both of --repo and --all.
//   - with --all, the errors of the configuration file of the repository.
//   - the error of the client of GitHub, and the error of propose.Propose.
//   - an error that wraps the error of the standard output.
func (p *Program) runPropose(ctx context.Context, run *cli.Run[Global, Propose]) error {
	opts := run.Config()
	if (opts.Repo != "") == opts.All {
		return fmt.Errorf("%w: pass --repo or --all", ErrUsage)
	}
	repos := []spec.Repo{opts.Repo}
	if opts.All {
		c, err := readDefinition(run.Global().Dir)
		if err != nil {
			return err
		}
		repos = make([]spec.Repo, 0, len(c.Implementations))
		for _, impl := range c.Implementations {
			repos = append(repos, impl.Repo)
		}
	}
	gh, err := p.client(run.Global(), readWrite)
	if err != nil {
		return err
	}
	var rows [][]string
	var failures []error
	for _, repo := range repos {
		result, err := propose.Propose(ctx, gh, &propose.Request{Repo: repo})
		if err != nil {
			failures = append(failures, fmt.Errorf("command: %w", err))
			continue
		}
		for _, pr := range result.Proposals {
			rows = append(
				rows,
				[]string{string(repo), string(pr.Definition), pr.Version, string(pr.Action), pullCell(pr.Pull)},
			)
		}
	}
	if err := table(run.Stdout(), proposeHeader, rows); err != nil {
		return err
	}
	return errors.Join(failures...)
}
