// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec/definition"
)

// manifestHelp is the help text of the command manifest.
const manifestHelp = "Write the manifest of the definition of the repository."

// Manifest is the options of the command manifest.
type Manifest struct {
	// Check fails the run for a manifest that differs from the files of the definition.
	Check bool `help:"Fail when the manifest differs from the files of the definition, and write nothing."`
}

// runManifest writes the manifest of the definition of the repository of run, or checks it with
// --check. It writes the path, the version, the number of the files and the digest of the manifest
// to the standard output.
//
// Error modes:
//   - the error of the file system for a repository that cannot be opened, and for a manifest that
//     cannot be read or written.
//   - the errors of definition.Open and of definition.Tree.Manifest.
//   - an error that wraps [ErrStale] with --check for a manifest that differs from the files of the
//     definition or does not exist.
//   - an error that wraps the error of the standard output.
func runManifest(_ context.Context, run *cli.Run[Global, Manifest]) error {
	return inRoot(run.Global().Dir, func(root *os.Root) error {
		tree, err := definition.Open(root)
		if err != nil {
			return fmt.Errorf("command: %w", err)
		}
		m, err := tree.Manifest()
		if err != nil {
			return fmt.Errorf("command: %w", err)
		}
		data := m.Encode()
		file := path.Join(tree.Config.Root, tree.Config.Manifest)
		name := filepath.FromSlash(file)
		if run.Config().Check {
			// A manifest that the repository lacks is empty data, which differs from every manifest.
			old, err := root.ReadFile(name)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("command: read %s: %w", file, err)
			}
			if !bytes.Equal(old, data) {
				return fmt.Errorf("%w: %s", ErrStale, file)
			}
		} else {
			if err := root.MkdirAll(filepath.Dir(name), dirMode); err != nil {
				return fmt.Errorf("command: create the directory of %s: %w", file, err)
			}
			if err := root.WriteFile(name, data, fileMode); err != nil {
				return fmt.Errorf("command: write %s: %w", file, err)
			}
		}
		if _, err := fmt.Fprintf(
			run.Stdout(),
			"%s: %s, %d files, %s\n",
			file,
			m.Version,
			len(m.Files),
			m.Digest,
		); err != nil {
			return fmt.Errorf("command: write the output: %w", err)
		}
		return nil
	})
}
