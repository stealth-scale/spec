// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package definition

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/cli/yaml"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// section is the key of the section definition in the configuration file.
const section = "definition"

// Placeholder is the text in a pattern of [Config.LanguageFiles] that the name of each language of
// a definition replaces.
const Placeholder = "{language}"

// The defaults of the keys of the section definition that a section leaves out.
const (
	// DefaultRoot is the root of a definition at the root of its repository.
	DefaultRoot = "."

	// DefaultVersion is the version file of a definition.
	DefaultVersion = "VERSION"

	// DefaultManifest is the manifest of a definition.
	DefaultManifest = "spec/manifest.json"
)

// Config is the section definition of the configuration file of a definition repository. The
// keys of the section are the names of the fields in kebab case, such as language-files:
//
//	definition:
//	  repo: dokimasia/assert-spec
//	  languages: [go, rust]
//	  files: [VERSION, "spec/*.json", "corpus/**/*.json", "overlays/*.json"]
//	  language-files: ["overlays/{language}.json"]
//	  implementations:
//	    - repo: dokimasia/assert-go
//	      languages: [go]
//
// Root, Version, Manifest and Cases are slash-separated paths. Version, Manifest, Cases and the
// patterns are relative to Root, and Root is relative to the root of the repository.
type Config struct {
	// Repo is the repository of the definition.
	Repo spec.Repo `required:""`

	// Root is the directory of the definition in its repository, [DefaultRoot] by default.
	Root string

	// Version is the file whose content is the version of the definition, [DefaultVersion] by
	// default.
	Version string

	// Manifest is the manifest of the definition, [DefaultManifest] by default. The patterns never
	// select it, because a manifest cannot list its own digest.
	Manifest string

	// Cases is the case index of the definition, or empty for a definition without one. The
	// patterns select it.
	Cases string

	// Implementations are the repositories that implement the definition, each with the languages
	// that it implements.
	Implementations []Implementation

	// Languages are the languages of the definition.
	Languages []spec.Language `required:""`

	// Files are the patterns of the files that an implementation vendors, as doublestar matches
	// them, such as corpus/**/*.json.
	Files []string `required:""`

	// LanguageFiles are the patterns of the files of one language, with [Placeholder] for the
	// language, such as overlays/{language}.json. [Tree.Manifest] assigns a file that Files selects
	// to the language whose pattern matches it. A file that does not match the pattern of any
	// language is shared by every language. Files alone selects the files of the definition.
	LanguageFiles []string
}

// Implementation is a repository that implements a definition, with the languages that it
// implements, or [spec.All] for every language of the definition.
type Implementation struct {
	// Repo is the repository.
	Repo spec.Repo `required:""`

	// Languages are the languages that the repository implements.
	Languages []spec.Language `required:""`
}

// Parse returns the section definition of data, the content of the configuration file of a
// repository. It decodes the section alone, so a file may contain other sections, such as the
// section implements. A key that the section leaves out takes its default.
//
// Error modes:
//   - an error that wraps [ErrConfig] for data that is no YAML, for a file without the section, and
//     for a section that go.stealthscale.io/cli refuses to decode, such as one with an unknown key
//     or without a required key. Such an error also wraps the *cli.Error of the key at fault.
//   - an error that wraps [ErrConfig] for a section that breaks a rule. Parse refuses a repository
//     that is not valid, a path that manifest.PortablePath refuses and an empty list of languages
//     or of files. A language must be valid, and the list must not repeat it. A pattern must be
//     relative and valid for doublestar, and a pattern of LanguageFiles must contain
//     [Placeholder]. Parse also refuses an implementation that the list repeats or that implements
//     a language that the definition does not declare.
func Parse(data []byte) (Config, error) {
	tree, err := yaml.Decode(data)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if tree[section] == nil {
		return Config{}, fmt.Errorf("%w: %s has no section %s", ErrConfig, spec.ConfigFile, section)
	}
	c := Config{Root: DefaultRoot, Version: DefaultVersion, Manifest: DefaultManifest}
	if err := cli.Decode(tree, section, &c); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if err := c.check(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// check returns an error that wraps [ErrConfig] for the first rule of the section that c breaks,
// in the order of the keys of [Parse], and nil when c breaks none.
func (c *Config) check() error {
	if !c.Repo.Valid() {
		return fmt.Errorf("%w: %q is no repository of GitHub", ErrConfig, c.Repo)
	}
	if c.Root != DefaultRoot && !manifest.PortablePath(c.Root) {
		return fmt.Errorf("%w: the root %q is no portable directory of the repository", ErrConfig, c.Root)
	}
	for _, p := range []string{c.Version, c.Manifest} {
		if !manifest.PortablePath(p) {
			return fmt.Errorf("%w: %q is no portable path below the root", ErrConfig, p)
		}
	}
	if c.Cases != "" && !manifest.PortablePath(c.Cases) {
		return fmt.Errorf("%w: the case index %q is no portable path below the root", ErrConfig, c.Cases)
	}
	if err := c.checkLanguages(); err != nil {
		return err
	}
	if err := c.checkPatterns(); err != nil {
		return err
	}
	return c.checkImplementations()
}

// checkLanguages returns an error that wraps [ErrConfig] for an empty list of languages, for a
// language that is not valid, for [spec.All] and for a language that the list repeats.
func (c *Config) checkLanguages() error {
	if len(c.Languages) == 0 {
		return fmt.Errorf("%w: the definition declares no language", ErrConfig)
	}
	for i, l := range c.Languages {
		if !l.Valid() || l == spec.All {
			return fmt.Errorf("%w: %q is no language of a definition", ErrConfig, l)
		}
		if slices.Contains(c.Languages[:i], l) {
			return fmt.Errorf("%w: the definition declares %s twice", ErrConfig, l)
		}
	}
	return nil
}

// checkPatterns returns an error that wraps [ErrConfig] for an empty list of files, for a pattern
// that doublestar refuses or that is absolute, and for a pattern of LanguageFiles without
// [Placeholder].
func (c *Config) checkPatterns() error {
	if len(c.Files) == 0 {
		return fmt.Errorf("%w: the definition states no files", ErrConfig)
	}
	patterns := slices.Clone(c.Files)
	for _, p := range c.LanguageFiles {
		if !strings.Contains(p, Placeholder) {
			return fmt.Errorf("%w: the pattern %q of the files of a language has no %s", ErrConfig, p, Placeholder)
		}
		patterns = append(patterns, strings.ReplaceAll(p, Placeholder, string(c.Languages[0])))
	}
	for _, p := range patterns {
		if strings.HasPrefix(p, "/") || !doublestar.ValidatePattern(p) {
			return fmt.Errorf("%w: %q is no pattern of the files below the root", ErrConfig, p)
		}
	}
	return nil
}

// checkImplementations returns an error that wraps [ErrConfig] for an implementation with a
// repository that is not valid or that an earlier implementation names, for an implementation
// without languages, and for a language that the definition does not declare. [spec.All] is the
// only language of an implementation that names it.
func (c *Config) checkImplementations() error {
	for i, impl := range c.Implementations {
		switch {
		case !impl.Repo.Valid():
			return fmt.Errorf("%w: %q is no repository of GitHub", ErrConfig, impl.Repo)
		case slices.ContainsFunc(c.Implementations[:i], func(o Implementation) bool { return o.Repo == impl.Repo }):
			return fmt.Errorf("%w: the definition lists %s twice", ErrConfig, impl.Repo)
		case len(impl.Languages) == 0:
			return fmt.Errorf("%w: %s implements no language", ErrConfig, impl.Repo)
		}
		for _, l := range impl.Languages {
			if l == spec.All && len(impl.Languages) == 1 {
				continue
			}
			if !slices.Contains(c.Languages, l) {
				return fmt.Errorf(
					"%w: %s implements %q, which the definition does not declare",
					ErrConfig,
					impl.Repo,
					l,
				)
			}
		}
	}
	return nil
}
