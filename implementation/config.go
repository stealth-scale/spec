// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package implementation

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/yaml"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// section is the key of the section implements in the configuration file.
const section = "implements"

// The tracks of an entry.
const (
	// DefaultTrack is the branch that an entry follows when it states no track.
	DefaultTrack = "main"

	// Release is the track of an entry that follows the newest stable release of its definition.
	Release = "release"
)

// lockSuffix ends every name that git refuses as the name of a branch.
const lockSuffix = ".lock"

// branchName matches the name of a branch: names of ASCII letters, digits, dashes, underscores and
// single dots that slashes join, each starting with a letter, a digit or an underscore and ending
// with a character other than a dot.
var branchName = regexp.MustCompile(
	`^[A-Za-z0-9_][A-Za-z0-9_-]*(\.[A-Za-z0-9_-]+)*(/[A-Za-z0-9_][A-Za-z0-9_-]*(\.[A-Za-z0-9_-]+)*)*$`,
)

// Entry is an entry of the section implements of the configuration file of an implementation
// repository: a definition that the repository vendors, and the copy of the definition. The keys
// of an entry are the names of the fields in lower case:
//
//	implements:
//	  - definition: dokimasia/assert-spec
//	    path: conformance/spec
//	    languages: [go]
//	    copies:
//	      - from: spec/zones.json
//	        to: internal/prop/zone/zones.json
//	    test: [make, test-go]
//
// Path and the paths of the copies are slash-separated.
type Entry struct {
	// Definition is the repository of the definition.
	Definition spec.Repo `required:""`

	// Track is the branch of the definition that the copy follows, or [Release] for the newest
	// stable release. It is [DefaultTrack] by default.
	Track string `default:"main"`

	// Path is the directory of the copy, relative to the root of the repository.
	Path string `required:""`

	// Languages are the languages that the copy vendors, or [spec.All] for every language of the
	// definition.
	Languages []spec.Language `required:""`

	// Copies are the files of the copy that the repository also needs outside the copy.
	Copies []Copy

	// Test is the program and the arguments that check the implementation against the copy, which
	// spec try runs in the repository, or empty for none.
	Test []string
}

// Find returns the entry of entries that vendors the definition def, as an element of entries.
// [Parse] refuses a section that lists a definition twice, so entries of Parse have one entry of
// def at most.
//
// Error modes:
//   - an error that wraps [ErrEntry] for entries without an entry of def.
func Find(entries []Entry, def spec.Repo) (*Entry, error) {
	for i := range entries {
		if entries[i].Definition == def {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrEntry, def)
}

// Copy is a file of a copy that a repository also needs at another path. A Go package embeds only
// the files below its own directory, so a package outside the copy needs such a second file.
type Copy struct {
	// From is the path of the file in the copy, relative to the root of the definition.
	From string `required:""`

	// To is the path of the second file, relative to the root of the repository.
	To string `required:""`
}

// Parse returns the entries of the section implements of data, the content of the configuration
// file of a repository, in the order of the section. It decodes the section alone, so a file may
// contain other sections. An entry without a track follows [DefaultTrack].
//
// Error modes:
//   - an error that wraps [ErrConfig] for data that is no YAML, for a file without the section, for
//     a section without entries, and for a section that go.stealthscale.io/cli refuses to decode,
//     such as an entry with an unknown key or without a required key. Such an error also wraps the
//     *cli.Error of the key at fault.
//   - an error that wraps [ErrConfig] for an entry that breaks a rule. Parse refuses a definition
//     that is not valid, a track that is no name of a branch and a path that manifest.PortablePath
//     refuses, the root included. It refuses a list of languages that is empty, that repeats a
//     language or that contains [spec.All] beside another language. It refuses a language that is
//     not valid. It also refuses a copy with a path that PortablePath refuses and a test whose
//     program is empty.
//   - an error that wraps [ErrConfig] for two entries of one definition, for two entries whose paths
//     are equal or nested, for a copy into the path of an entry, and for two copies to one path. It
//     compares the paths without case. Windows and macOS store two paths that differ only in case
//     as one file.
func Parse(data []byte) ([]Entry, error) {
	tree, err := yaml.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if tree[section] == nil {
		return nil, fmt.Errorf("%w: %s has no section %s", ErrConfig, spec.ConfigFile, section)
	}
	var entries []Entry
	if err := cli.Decode(tree, section, &entries); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: the section %s lists no definition", ErrConfig, section)
	}
	for i := range entries {
		if err := entries[i].check(); err != nil {
			return nil, err
		}
	}
	if err := checkEntries(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// check returns an error that wraps [ErrConfig] for the first rule of an entry that e breaks, and
// nil when e breaks none.
func (e *Entry) check() error {
	switch {
	case !e.Definition.Valid():
		return fmt.Errorf("%w: %q is no repository of GitHub", ErrConfig, e.Definition)
	case !branchName.MatchString(e.Track) || strings.HasSuffix(e.Track, lockSuffix):
		return fmt.Errorf("%w: the track %q of %s is no name of a branch", ErrConfig, e.Track, e.Definition)
	case !manifest.PortablePath(e.Path):
		return fmt.Errorf(
			"%w: the path %q of %s is no portable directory below the root",
			ErrConfig,
			e.Path,
			e.Definition,
		)
	case len(e.Languages) == 0:
		return fmt.Errorf("%w: %s vendors no language", ErrConfig, e.Definition)
	case len(e.Test) > 0 && e.Test[0] == "":
		return fmt.Errorf("%w: the test of %s names no program", ErrConfig, e.Definition)
	}
	for i, l := range e.Languages {
		if !l.Valid() || l == spec.All && len(e.Languages) > 1 || slices.Contains(e.Languages[:i], l) {
			return fmt.Errorf("%w: the languages %v of %s", ErrConfig, e.Languages, e.Definition)
		}
	}
	for _, c := range e.Copies {
		if !manifest.PortablePath(c.From) || !manifest.PortablePath(c.To) {
			return fmt.Errorf("%w: the copy of %q to %q has a path that is not portable", ErrConfig, c.From, c.To)
		}
	}
	return nil
}

// checkEntries returns an error that wraps [ErrConfig] for two entries of one definition, for two
// entries whose paths are equal or nested, for a copy into the path of an entry, and for two
// copies to one path. It compares the paths by their manifest.Fold. Windows and macOS compare
// paths in the same way. It returns nil when entries break none of these rules.
func checkEntries(entries []Entry) error {
	targets := map[string]bool{}
	for i, e := range entries {
		dir := manifest.Fold(e.Path)
		for _, o := range entries[:i] {
			other := manifest.Fold(o.Path)
			switch {
			case e.Definition == o.Definition:
				return fmt.Errorf("%w: the section lists %s twice", ErrConfig, e.Definition)
			case dir == other || strings.HasPrefix(dir, other+"/") || strings.HasPrefix(other, dir+"/"):
				return fmt.Errorf("%w: the paths %s and %s overlap", ErrConfig, o.Path, e.Path)
			}
		}
		for _, c := range e.Copies {
			to := manifest.Fold(c.To)
			if targets[to] {
				return fmt.Errorf("%w: two copies write %s", ErrConfig, c.To)
			}
			targets[to] = true
			for _, o := range entries {
				other := manifest.Fold(o.Path)
				if to == other || strings.HasPrefix(to, other+"/") {
					return fmt.Errorf("%w: the copy to %s writes into the copy at %s", ErrConfig, c.To, o.Path)
				}
			}
		}
	}
	return nil
}
