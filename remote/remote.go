// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package remote

import (
	"context"
	"fmt"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/vendored"
)

// GitHub is the host of the implementations and the definitions that the functions of the package
// read through. [*github.Client] implements it. Each method takes a repository as owner/name.
type GitHub interface {
	// DefaultBranch returns the default branch of repo.
	DefaultBranch(ctx context.Context, repo string) (string, error)

	// Branch returns the commit of the branch name of repo, and reports whether repo has the
	// branch.
	Branch(ctx context.Context, repo, name string) (string, bool, error)

	// File returns the content of the file at the slash-separated path of repo at ref, and reports
	// whether ref has the file.
	File(ctx context.Context, repo, ref, path string) ([]byte, bool, error)

	// LatestRelease returns the tag and the commit of the newest stable release of repo.
	LatestRelease(ctx context.Context, repo string) (string, string, error)

	// Archive returns the content of each regular file of the commit sha of repo, by the
	// slash-separated path of the file.
	Archive(ctx context.Context, repo, sha string) (map[string][]byte, error)
}

var _ GitHub = (*github.Client)(nil)

// Implementation is an implementation repository at the head of its default branch.
type Implementation struct {
	// Repo is the repository.
	Repo spec.Repo

	// Base is the default branch.
	Base string

	// Head is the commit at the head of Base.
	Head string

	// Entries are the entries of the section implements of the configuration file at Head, in the
	// order of the section.
	Entries []implementation.Entry
}

// Open returns the implementation repository repo at the head of its default branch, with the
// entries of its configuration file.
//
// Error modes:
//   - an error that wraps [ErrRepository] for a repository that is not valid, for a repository
//     without its default branch, and for a default branch without the configuration file.
//   - the error of implementation.Parse, which wraps implementation.ErrConfig.
//   - the error of gh for a call that fails.
func Open(ctx context.Context, gh GitHub, repo spec.Repo) (*Implementation, error) {
	if !repo.Valid() {
		return nil, fmt.Errorf("%w: %q is no repository of GitHub", ErrRepository, repo)
	}
	name := string(repo)
	base, err := gh.DefaultBranch(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("remote: the default branch of %s: %w", name, err)
	}
	head, found, err := gh.Branch(ctx, name, base)
	if err != nil {
		return nil, fmt.Errorf("remote: the head of %s: %w", base, err)
	}
	if !found {
		return nil, fmt.Errorf("%w: %s has no branch %s", ErrRepository, name, base)
	}
	data, found, err := gh.File(ctx, name, head, spec.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("remote: read %s: %w", spec.ConfigFile, err)
	}
	if !found {
		return nil, fmt.Errorf("%w: %s has no %s on %s", ErrRepository, name, spec.ConfigFile, base)
	}
	entries, err := implementation.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("remote: %s: %w", name, err)
	}
	return &Implementation{Repo: repo, Base: base, Head: head, Entries: entries}, nil
}

// Target is the commit of a definition that an entry follows.
type Target struct {
	// Commit is the commit.
	Commit string

	// Tag is the tag of the release at Commit for an entry of the track release, and empty for an
	// entry that follows a branch.
	Tag string
}

// Resolve returns the commit of the definition of e that e follows: the head of the branch of the
// track of e, or the newest stable release for the track [implementation.Release].
//
// Error modes:
//   - an error that wraps [ErrTrack] for a definition without the branch of the track.
//   - the error of gh for a call that fails, such as the error of a definition without a stable
//     release, which wraps github.ErrNoRelease for a [*github.Client].
func Resolve(ctx context.Context, gh GitHub, e *implementation.Entry) (Target, error) {
	def := string(e.Definition)
	if e.Track == implementation.Release {
		tag, commit, err := gh.LatestRelease(ctx, def)
		if err != nil {
			return Target{}, fmt.Errorf("remote: the newest release of %s: %w", def, err)
		}
		return Target{Commit: commit, Tag: tag}, nil
	}
	commit, found, err := gh.Branch(ctx, def, e.Track)
	if err != nil {
		return Target{}, fmt.Errorf("remote: the branch %s of %s: %w", e.Track, def, err)
	}
	if !found {
		return Target{}, fmt.Errorf("%w: %s has no branch %s", ErrTrack, def, e.Track)
	}
	return Target{Commit: commit}, nil
}

// Fetch returns the copy of the definition of e at commit that e vendors, as vendored.Build builds
// it from the archive of the commit.
//
// Error modes:
//   - the error of gh for an archive that cannot be read.
//   - the errors of definition.Load and vendored.Build for a definition that breaks a rule, such as
//     the error that wraps manifest.ErrLanguage for a language of e that the definition lacks.
func Fetch(ctx context.Context, gh GitHub, e *implementation.Entry, commit string) (*vendored.Copy, error) {
	files, err := gh.Archive(ctx, string(e.Definition), commit)
	if err != nil {
		return nil, fmt.Errorf("remote: the files of %s at %s: %w", e.Definition, commit, err)
	}
	tree, err := definition.Load(files)
	if err != nil {
		return nil, fmt.Errorf("remote: %s at %s: %w", e.Definition, commit, err)
	}
	c, err := vendored.Build(tree, e, commit)
	if err != nil {
		return nil, fmt.Errorf("remote: %w", err)
	}
	return c, nil
}
