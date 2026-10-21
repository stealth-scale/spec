// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package status

import (
	"context"
	"fmt"
	"path"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/propose"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// GitHub is the host of a definition and of its implementations, which [Status] reads through.
// [*github.Client] implements it. Each method takes a repository as owner/name.
//
// # Concurrency
//
// Status calls the methods of a GitHub one at a time, on the goroutine of its caller.
type GitHub interface {
	remote.GitHub

	// PullRequest returns the number of the open pull request of repo from head into base, and
	// reports whether one is open.
	PullRequest(ctx context.Context, repo, head, base string) (int, bool, error)

	// Checks returns the state of the check runs of ref of repo, a branch or a commit.
	Checks(ctx context.Context, repo, ref string) (github.State, error)
}

var _ GitHub = (*github.Client)(nil)

// Request is the definition whose implementations [Status] reports.
type Request struct {
	// Definition is the repository of the definition.
	Definition spec.Repo

	// Implementations are the implementation repositories of the definition.
	Implementations []spec.Repo
}

// Row is the status of the copy of a definition in one implementation repository.
//
// # Concurrency
//
// A Row is a value, which any number of goroutines may read at once.
type Row struct {
	// Err is the error that ended the read of the implementation, and nil for a row whose other
	// fields are complete.
	Err error

	// Implementation is the implementation repository.
	Implementation spec.Repo

	// Version is the version of the copy on the default branch, and empty for a copy without a
	// manifest that decodes.
	Version string

	// Commit is the commit of the definition that the copy comes from, and empty for a copy of a
	// checkout and for a copy without a manifest that decodes.
	Commit string

	// Checks is the state of the check runs of the open sync pull request, or of the default branch
	// without one.
	Checks github.State

	// Pull is the number of the open sync pull request, and 0 without one.
	Pull int

	// Newer is true when the commit that the entry follows has a copy whose digest differs from the
	// digest of the copy on the default branch.
	Newer bool
}

// Result is the status of the copy of a definition in each of its implementations.
//
// # Concurrency
//
// A Result is a value, which any number of goroutines may read at once.
type Result struct {
	// Rows has the row of each implementation, in the order of the request.
	Rows []Row
}

// Status returns the status of the copy of req.Definition in each implementation of req, read
// through gh. A row reads the entry of the definition in the configuration file at the head of the
// default branch of its implementation, and the manifest of the copy of that entry. It builds the
// copy of the commit that the entry follows to tell whether that commit has a newer copy. It then
// reads the open pull request from the branch propose.BranchPrefix and the repository of the
// definition, and the state of its check runs. Without an open pull request, the state is the
// state of the head of the default branch.
//
// A row that cannot be read has the error in Err, and Status continues with the next row:
//   - an error that wraps implementation.ErrEntry for an implementation without an entry of the
//     definition.
//   - the errors of remote.Open, remote.Resolve and remote.Fetch.
//   - the error of gh for a call that fails.
func Status(ctx context.Context, gh GitHub, req *Request) Result {
	result := Result{Rows: make([]Row, 0, len(req.Implementations))}
	for _, repo := range req.Implementations {
		row, err := read(ctx, gh, req.Definition, repo)
		row.Implementation, row.Err = repo, err
		result.Rows = append(result.Rows, row)
	}
	return result
}

// read returns the row of the copy of def in the implementation repository repo, without its
// Implementation and its Err. It returns the errors that [Status] states for a row.
func read(ctx context.Context, gh GitHub, def, repo spec.Repo) (Row, error) {
	impl, err := remote.Open(ctx, gh, repo)
	if err != nil {
		return Row{}, fmt.Errorf("status: %w", err)
	}
	e, err := implementation.Find(impl.Entries, def)
	if err != nil {
		return Row{}, fmt.Errorf("status: %s: %w", repo, err)
	}
	data, _, err := gh.File(ctx, string(repo), impl.Head, path.Join(e.Path, vendored.ManifestFile))
	if err != nil {
		return Row{}, fmt.Errorf("status: read the manifest of %s: %w", e.Path, err)
	}
	var row Row
	// A manifest that the branch lacks is empty data, which Decode refuses as well. The row of a
	// copy without a manifest that decodes has no version and no commit.
	old, err := manifest.Decode(data)
	//dokimi:mutate-skip ror-true: Decode returns the zero manifest with an error, and its version and commit are empty
	if err == nil {
		row.Version, row.Commit = old.Version, old.Commit
	}
	target, err := remote.Resolve(ctx, gh, e)
	if err != nil {
		return Row{}, fmt.Errorf("status: %w", err)
	}
	c, err := remote.Fetch(ctx, gh, e, target.Commit)
	if err != nil {
		return Row{}, fmt.Errorf("status: %w", err)
	}
	row.Newer = c.Manifest.Digest != old.Digest
	branch := propose.BranchPrefix + string(def)
	number, open, err := gh.PullRequest(ctx, string(repo), branch, impl.Base)
	if err != nil {
		return Row{}, fmt.Errorf("status: the pull request from %s: %w", branch, err)
	}
	ref := impl.Head
	if open {
		row.Pull, ref = number, branch
	}
	if row.Checks, err = gh.Checks(ctx, string(repo), ref); err != nil {
		return Row{}, fmt.Errorf("status: the checks of %s: %w", ref, err)
	}
	return row, nil
}
