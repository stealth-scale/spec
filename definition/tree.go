// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// gitDir is the directory of git in a checkout. [Open] skips it, because no archive of a
// repository contains it.
const gitDir = ".git"

// Tree is a definition at one revision: its configuration, its version, and the content of each
// file that its configuration selects. [Open] reads it from a checkout, and [Load] from the files of
// a repository, such as the files of an archive.
//
// # Concurrency
//
// The methods of a Tree read it and never write it, so any number of goroutines may call them at
// once.
type Tree struct {
	// Files maps the slash-separated path of each file that the configuration selects, relative to
	// the root of the definition, to its content.
	Files map[string][]byte

	// Version is the content of the version file, without white space at its ends.
	Version string

	// Config is the section definition of the configuration file.
	Config Config
}

// Open reads the definition of the repository at root: the section definition of its
// configuration file, the version file, and each regular file below the root of the definition that
// a pattern of the files selects. It skips the directory .git and every link, as an archive of the
// repository contains neither.
//
// Error modes:
//   - the error of [os.Root.ReadFile] for a configuration file that cannot be read, which wraps
//     [fs.ErrNotExist] for a repository without one.
//   - the errors of [Parse].
//   - an error that wraps [ErrConfig] for a root that cannot be opened, for a version file that
//     does not exist or is empty, for a pattern of the files that matches no file, and for a case
//     index that the patterns do not select.
//   - the error of the file system for a directory or a selected file that cannot be read.
func Open(root *os.Root) (tree *Tree, err error) {
	data, err := root.ReadFile(spec.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("definition: read %s: %w", spec.ConfigFile, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, err
	}
	sub, err := root.OpenRoot(filepath.FromSlash(c.Root))
	if err != nil {
		return nil, fmt.Errorf("%w: open the root %s: %w", ErrConfig, c.Root, err)
	}
	//dokimi:mutate-skip sbr-delete: the close of an open root does not fail, and a caller cannot observe a root that is not closed
	defer func() { err = errors.Join(err, sub.Close()) }()
	fsys := sub.FS()
	version, err := fs.ReadFile(fsys, c.Version)
	if err != nil {
		return nil, fmt.Errorf("%w: read the version file %s: %w", ErrConfig, c.Version, err)
	}
	var paths []string
	err = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == gitDir:
			return fs.SkipDir
		case d.Type().IsRegular():
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("definition: walk the root %s: %w", c.Root, err)
	}
	selected, err := c.selected(paths)
	if err != nil {
		return nil, err
	}
	contents := make(map[string][]byte, len(selected))
	for _, p := range selected {
		if contents[p], err = fs.ReadFile(fsys, p); err != nil {
			return nil, fmt.Errorf("definition: read %s: %w", p, err)
		}
	}
	return newTree(&c, version, contents)
}

// Load returns the definition of the files of a repository, which maps the slash-separated path of
// each regular file, relative to the root of the repository, to its content. It reads the section
// definition of the configuration file and the version file, and keeps each file below the root of
// the definition that a pattern of the files selects. It reads files as [Open] reads a checkout of
// the same files.
//
// Error modes:
//   - an error that wraps [ErrConfig] for files without the configuration file.
//   - the errors of [Parse].
//   - an error that wraps [ErrConfig] for a version file that does not exist or is empty, for a
//     pattern of the files that matches no file, and for a case index that the patterns do not
//     select.
func Load(files map[string][]byte) (*Tree, error) {
	data, ok := files[spec.ConfigFile]
	if !ok {
		return nil, fmt.Errorf("%w: the files have no %s", ErrConfig, spec.ConfigFile)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, err
	}
	prefix := c.Root + "/"
	if c.Root == DefaultRoot {
		prefix = ""
	}
	below := make(map[string][]byte, len(files))
	for p, content := range files {
		if rel, found := strings.CutPrefix(p, prefix); found {
			below[rel] = content
		}
	}
	version, ok := below[c.Version]
	if !ok {
		return nil, fmt.Errorf("%w: the files have no version file %s", ErrConfig, c.Version)
	}
	selected, err := c.selected(slices.Collect(maps.Keys(below)))
	if err != nil {
		return nil, err
	}
	contents := make(map[string][]byte, len(selected))
	for _, p := range selected {
		contents[p] = below[p]
	}
	return newTree(&c, version, contents)
}

// Manifest returns the manifest of the files of t at the version of t, with the case index of the
// configuration. It assigns each file to the language whose pattern of the files of a language
// matches it, and lists every language of the configuration, a language without files with an
// empty list.
//
// Error modes:
//   - an error that wraps [ErrConfig] for a file that the patterns of two languages match.
//   - the error of [manifest.New] for a path of a file that a manifest cannot list, such as a path
//     with a backslash.
func (t *Tree) Manifest() (manifest.Manifest, error) {
	languages := make(map[spec.Language][]string, len(t.Config.Languages))
	for _, l := range t.Config.Languages {
		languages[l] = []string{}
	}
	for _, p := range slices.Sorted(maps.Keys(t.Files)) {
		var owner spec.Language
		for _, l := range t.Config.Languages {
			for _, pattern := range t.Config.LanguageFiles {
				if !doublestar.MatchUnvalidated(strings.ReplaceAll(pattern, Placeholder, string(l)), p) {
					continue
				}
				if owner != "" && owner != l {
					return manifest.Manifest{}, fmt.Errorf(
						"%w: the files of %s and of %s both match %s", ErrConfig, owner, l, p,
					)
				}
				owner = l
			}
		}
		if owner != "" {
			languages[owner] = append(languages[owner], p)
		}
	}
	m, err := manifest.New(t.Version, t.Files, languages)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("definition: the manifest of %s: %w", t.Config.Repo, err)
	}
	m.Cases = t.Config.Cases
	return m, nil
}

// newTree returns the tree of the configuration c with the content of the version file version and
// the selected files contents. It returns an error that wraps [ErrConfig] for a version file without
// a character other than white space.
func newTree(c *Config, version []byte, contents map[string][]byte) (*Tree, error) {
	v := strings.TrimSpace(string(version))
	if v == "" {
		return nil, fmt.Errorf("%w: the version file %s is empty", ErrConfig, c.Version)
	}
	return &Tree{Files: contents, Version: v, Config: *c}, nil
}

// selected returns the paths of paths that a pattern of the files of c selects, in the order of
// their bytes and without the manifest. paths are slash-separated and relative to the root of the
// definition. It returns an error that wraps [ErrConfig] for a pattern that matches no path other
// than the manifest, and for a case index that no pattern selects.
func (c *Config) selected(paths []string) ([]string, error) {
	keep := map[string]bool{}
	for _, pattern := range c.Files {
		matched := false
		for _, p := range paths {
			if p != c.Manifest && doublestar.MatchUnvalidated(pattern, p) {
				keep[p] = true
				matched = true
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: the pattern %q matches no file", ErrConfig, pattern)
		}
	}
	if c.Cases != "" && !keep[c.Cases] {
		return nil, fmt.Errorf("%w: no pattern selects the case index %s", ErrConfig, c.Cases)
	}
	return slices.Sorted(maps.Keys(keep)), nil
}
