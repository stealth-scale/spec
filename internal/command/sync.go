// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// syncHelp is the help text of the command sync.
const syncHelp = "Write the copy of each definition of the repository at the head of its track."

// Sync is the options of the command sync.
type Sync struct {
	// Commit is the commit of the definition that the run syncs to, for one definition.
	Commit string `help:"The commit of the definition to sync to, for one definition."`

	// From is the checkout of the definition that the run syncs from, for one definition.
	From string `help:"The checkout of the definition to sync from, for one definition."`

	// Definitions are the definitions whose copies the run writes, or empty for every definition of
	// the configuration file.
	Definitions []spec.Repo `arg:"" default:"" help:"The definitions to sync, every definition by default."`

	// Locked syncs each copy to the commit of its manifest.
	Locked bool `help:"Sync each copy to the commit of its manifest."`

	// Force replaces a copy whose files differ from its manifest.
	Force bool `help:"Replace a copy whose files differ from its manifest."`
}

// runSync writes the copy of each selected entry of the repository of run, and writes the
// directory, the definition, the version and the commit of each copy to the standard output. The
// copy comes from the head of the track of the entry, from the commit of --commit, from the commit
// of the manifest of the copy with --locked, or from the checkout of --from, which gives a local
// copy. It writes each copy with vendored.Write, or with vendored.Overwrite for --force, and
// continues with the next entry after an entry that fails.
//
// Error modes:
//   - an error that wraps [ErrUsage] for two of --commit, --from and --locked, for --commit or
//     --from with other than one entry, and for a checkout of --from of another definition.
//   - the error of the file system for a repository, a configuration file or a checkout that
//     cannot be opened or read.
//   - the errors of implementation.Parse and implementation.Find.
//   - an error that wraps [ErrLock] with --locked for a copy without the manifest of a commit of
//     its definition.
//   - the errors of the client of GitHub, of remote.Resolve and of remote.Fetch, and of
//     definition.Open and vendored.Build for --from.
//   - the errors of vendored.Write and vendored.Overwrite, such as an error that wraps
//     vendored.ErrNotIntact for an edited copy without --force.
//   - an error that wraps the error of the standard output.
func (p *Program) runSync(ctx context.Context, run *cli.Run[Global, Sync]) error {
	opts := run.Config()
	modes := 0
	for _, set := range []bool{opts.Commit != "", opts.From != "", opts.Locked} {
		if set {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("%w: --commit, --from and --locked exclude each other", ErrUsage)
	}
	return inRoot(run.Global().Dir, func(root *os.Root) error {
		entries, err := readEntries(root)
		if err != nil {
			return err
		}
		selected := entries
		if len(opts.Definitions) > 0 {
			selected = make([]implementation.Entry, 0, len(opts.Definitions))
			for _, def := range opts.Definitions {
				e, err := implementation.Find(entries, def)
				if err != nil {
					return fmt.Errorf("command: %w", err)
				}
				selected = append(selected, *e)
			}
		}
		if (opts.Commit != "" || opts.From != "") && len(selected) != 1 {
			return fmt.Errorf("%w: --commit and --from sync one definition", ErrUsage)
		}
		write := vendored.Write
		if opts.Force {
			write = vendored.Overwrite
		}
		var rows [][]string
		var failures []error
		for i := range selected {
			c, err := p.source(ctx, run, root, &selected[i])
			if err == nil {
				err = write(root, c)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("command: %s: %w", selected[i].Path, err))
				continue
			}
			rows = append(rows, []string{c.Dir, string(c.Manifest.Repo), c.Manifest.Version, short(c.Manifest.Commit)})
		}
		if err := table(run.Stdout(), copyHeader, rows); err != nil {
			return err
		}
		return errors.Join(failures...)
	})
}

// source returns the copy of the entry e of the repository at root that the options of run select,
// as [Program.runSync] states. It returns the errors that runSync states for one entry.
func (p *Program) source(
	ctx context.Context, run *cli.Run[Global, Sync], root *os.Root, e *implementation.Entry,
) (*vendored.Copy, error) {
	opts := run.Config()
	if opts.From != "" {
		return local(opts.From, e)
	}
	gh, err := p.client(run.Global(), readOnly)
	if err != nil {
		return nil, err
	}
	commit, err := commitOf(ctx, gh, opts, root, e)
	if err != nil {
		return nil, err
	}
	c, err := remote.Fetch(ctx, gh, e, commit)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	return c, nil
}

// commitOf returns the commit of the definition of the entry e that the options opts select: the
// commit of --commit, the commit of the manifest of the copy at root with --locked, or the head of
// the track of e.
//
// Error modes:
//   - the error of lockOf with --locked.
//   - the error of remote.Resolve.
func commitOf(
	ctx context.Context, gh *github.Client, opts *Sync, root *os.Root, e *implementation.Entry,
) (string, error) {
	switch {
	case opts.Commit != "":
		return opts.Commit, nil
	case opts.Locked:
		return lockOf(root, e)
	}
	target, err := remote.Resolve(ctx, gh, e)
	if err != nil {
		return "", fmt.Errorf("command: %w", err)
	}
	return target.Commit, nil
}

// lockOf returns the commit of the manifest of the copy of the entry e at root, which is the lock of
// the copy.
//
// Error modes:
//   - an error that wraps [ErrLock] for a manifest that cannot be read or does not decode, and for
//     the manifest of a local copy or of a copy of another definition.
func lockOf(root *os.Root, e *implementation.Entry) (string, error) {
	// A manifest that cannot be read is empty data, which Decode refuses, so one check reports both
	// errors.
	data, readErr := root.ReadFile(filepath.FromSlash(path.Join(e.Path, vendored.ManifestFile)))
	lock, err := manifest.Decode(data)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrLock, e.Path, errors.Join(readErr, err))
	}
	if lock.Repo != e.Definition || lock.Commit == "" {
		return "", fmt.Errorf("%w: the manifest of %s names no commit of %s", ErrLock, e.Path, e.Definition)
	}
	return lock.Commit, nil
}

// local returns the local copy of the entry e from the checkout of its definition at dir.
//
// Error modes:
//   - the error of the file system for a checkout that cannot be opened, and the errors of
//     definition.Open and vendored.Build.
//   - an error that wraps [ErrUsage] for a checkout of another definition.
func local(dir string, e *implementation.Entry) (*vendored.Copy, error) {
	var c *vendored.Copy
	err := inRoot(dir, func(src *os.Root) error {
		tree, err := definition.Open(src)
		if err != nil {
			return fmt.Errorf("command: %w", err)
		}
		if tree.Config.Repo != e.Definition {
			return fmt.Errorf(
				"%w: the checkout %s is %s, and the entry names %s",
				ErrUsage,
				dir,
				tree.Config.Repo,
				e.Definition,
			)
		}
		if c, err = vendored.Build(tree, e, ""); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		return nil
	})
	return c, err
}
