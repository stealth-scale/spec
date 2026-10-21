// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package status_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/propose"
	"go.stealthscale.io/spec/status"
	"go.stealthscale.io/spec/vendored"
)

// The repositories and the commits of the cases.
const (
	definitionRepo = spec.Repo("dokimasia/assert-spec")
	goRepo         = spec.Repo("dokimasia/assert-go")
	rustRepo       = spec.Repo("dokimasia/assert-rust")
	firstCommit    = "1111111111111111111111111111111111111111"
	secondCommit   = "2222222222222222222222222222222222222222"
	rustCommit     = "3333333333333333333333333333333333333333"
	goHead         = "4444444444444444444444444444444444444444"
	rustHead       = "5555555555555555555555555555555555555555"
)

// branch is the branch of the sync pull request of the definition of the cases.
const branch = propose.BranchPrefix + string(definitionRepo)

// The configuration files of the implementations of the cases.
const (
	goConfig   = "implements: [{definition: dokimasia/assert-spec, path: spec, languages: [go]}]\n"
	rustConfig = "implements: [{definition: dokimasia/assert-spec, path: spec, languages: [rust]}]\n"
)

// first is the first revision of the definition of the cases.
var first = map[string]string{
	spec.ConfigFile: `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
`,
	"VERSION":            "1.0.0\n",
	"corpus/a.json":      `{"case": "a"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// second is the next revision of the definition, with a changed case that every language shares.
var second = merge(
	first,
	map[string]string{"VERSION": "1.1.0\n", "corpus/a.json": `{"case": "a", "version": 2}` + "\n"},
)

// rust is the revision of the definition after first that changes the overlay of rust alone.
var rust = merge(first, map[string]string{"overlays/rust.json": `{"language": "rust", "version": 2}` + "\n"})

// errFake is the error of the call of the fake that a case makes fail.
var errFake = errors.New("github: injected failure")

// fake is a GitHub of the cases in memory: the definition on its branch main, the implementations
// on theirs, the open sync pull requests and the state of the check runs of each ref.
type fake struct {
	// branches maps a repository to the commit of each of its branches.
	branches map[spec.Repo]map[string]string

	// commits maps a repository to the files of each of its commits.
	commits map[spec.Repo]map[string]map[string]string

	// pulls maps a repository and the branch of a pull request, joined by a space, to the number of
	// the open pull request.
	pulls map[string]int

	// checks maps a repository and a ref, joined by a space, to the state of its check runs.
	checks map[string]github.State

	// fail maps a method and a repository, joined by a space, to the error that the method returns
	// for that repository. File takes the path in place of the repository.
	fail map[string]error
}

var _ status.GitHub = (*fake)(nil)

// newFake returns a fake with the definition at first on main, the implementations at goHead and
// rustHead with their copies of first, and the check runs of both heads.
func newFake(tb assert.TB) *fake {
	tb.Helper()
	return &fake{
		branches: map[spec.Repo]map[string]string{
			definitionRepo: {"main": firstCommit},
			goRepo:         {"main": goHead},
			rustRepo:       {"main": rustHead},
		},
		commits: map[spec.Repo]map[string]map[string]string{
			definitionRepo: {firstCommit: first, secondCommit: second, rustCommit: rust},
			goRepo:         {goHead: implementationAt(tb, goConfig, firstCommit)},
			rustRepo:       {rustHead: implementationAt(tb, rustConfig, firstCommit)},
		},
		pulls: map[string]int{},
		checks: map[string]github.State{
			string(goRepo) + " " + goHead:     github.StateSuccess,
			string(rustRepo) + " " + rustHead: github.StatePending,
		},
		fail: map[string]error{},
	}
}

// DefaultBranch returns main.
func (f *fake) DefaultBranch(_ context.Context, repo string) (string, error) {
	return "main", f.fail["DefaultBranch "+repo]
}

// Branch returns the commit of the branch name of repo.
func (f *fake) Branch(_ context.Context, repo, name string) (string, bool, error) {
	sha, ok := f.branches[spec.Repo(repo)][name]
	return sha, ok, f.fail["Branch "+repo]
}

// File returns the file path of repo at the commit ref.
func (f *fake) File(_ context.Context, repo, ref, path string) ([]byte, bool, error) {
	content, ok := f.commits[spec.Repo(repo)][ref][path]
	if !ok {
		return nil, false, f.fail["File "+path]
	}
	return []byte(content), true, f.fail["File "+path]
}

// LatestRelease returns github.ErrNoRelease, because the definition of the cases has no release.
func (*fake) LatestRelease(_ context.Context, _ string) (string, string, error) {
	return "", "", github.ErrNoRelease
}

// Archive returns the files of the commit sha of repo.
func (f *fake) Archive(_ context.Context, repo, sha string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for p, content := range f.commits[spec.Repo(repo)][sha] {
		files[p] = []byte(content)
	}
	return files, f.fail["Archive "+repo]
}

// PullRequest returns the number of the open pull request of repo from head.
func (f *fake) PullRequest(_ context.Context, repo, head, _ string) (int, bool, error) {
	n, ok := f.pulls[repo+" "+head]
	return n, ok, f.fail["PullRequest "+repo]
}

// Checks returns the state of the check runs of ref of repo.
func (f *fake) Checks(_ context.Context, repo, ref string) (github.State, error) {
	return f.checks[repo+" "+ref], f.fail["Checks "+repo]
}

func TestStatus(t *testing.T) {
	t.Parallel()

	both := &status.Request{Definition: definitionRepo, Implementations: []spec.Repo{goRepo, rustRepo}}

	t.Run("Status", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the version and the commit of the copy in each implementation", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, status.Status(t.Context(), newFake(t), both), status.Result{Rows: []status.Row{
				{Implementation: goRepo, Version: "1.0.0", Commit: firstCommit, Checks: github.StateSuccess},
				{Implementation: rustRepo, Version: "1.0.0", Commit: firstCommit, Checks: github.StatePending},
			}}, "each row states the copy on the default branch and the checks of its head")
		})

		t.Run("reports a newer copy at the head of the track", func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			f.branches[definitionRepo]["main"] = secondCommit
			rows := status.Status(t.Context(), f, both).Rows
			assert.Length(t, rows, 2, "Status reports each implementation")
			assert.True(t, rows[0].Newer, "the case of every language changed for go")
			assert.True(t, rows[1].Newer, "the case of every language changed for rust")
		})

		t.Run("reports no newer copy for a change of another language", func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			f.branches[definitionRepo]["main"] = rustCommit
			rows := status.Status(t.Context(), f, both).Rows
			assert.Length(t, rows, 2, "Status reports each implementation")
			assert.False(t, rows[0].Newer, "the copy of go has no file of rust")
			assert.True(t, rows[1].Newer, "the overlay of rust changed")
		})

		t.Run("reports the open sync pull request with the state of its checks", func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			f.pulls[string(goRepo)+" "+branch] = 9
			f.checks[string(goRepo)+" "+branch] = github.StateFailure
			rows := status.Status(t.Context(), f, both).Rows
			assert.Length(t, rows, 2, "Status reports each implementation")
			assert.Equal(t, rows[0].Pull, 9, "the row states the pull request")
			assert.Equal(t, rows[0].Checks, github.StateFailure, "the row states the checks of the pull request")
		})

		t.Run("reports no version for a copy without a valid manifest", func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			delete(f.commits[goRepo][goHead], "spec/manifest.json")
			rows := status.Status(t.Context(), f, both).Rows
			assert.Length(t, rows, 2, "Status reports each implementation")
			assert.Equal(t, rows[0], status.Row{
				Implementation: goRepo,
				Checks:         github.StateSuccess,
				Newer:          true,
			}, "a copy without a manifest has no version, and the head has a newer copy")
		})

		t.Run(
			"returns implementation.ErrEntry for an implementation without an entry of the definition",
			func(t *testing.T) {
				t.Parallel()
				f := newFake(t)
				f.commits[goRepo][goHead][spec.ConfigFile] = "implements: [{definition: a/b, path: spec, languages: [go]}]\n"
				rows := status.Status(t.Context(), f, both).Rows
				assert.Length(t, rows, 2, "Status reports each implementation")
				assert.ErrorIs(t, rows[0].Err, implementation.ErrEntry, "the row states the missing entry")
			},
		)

		t.Run("reports the rows after an implementation that fails", func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			f.fail["DefaultBranch "+string(goRepo)] = errFake
			rows := status.Status(t.Context(), f, both).Rows
			assert.Length(t, rows, 2, "Status reports each implementation")
			assert.ErrorIs(t, rows[0].Err, errFake, "the row states the error of GitHub")
			assert.Equal(t, rows[0].Implementation, goRepo, "the row of the error names its implementation")
			assert.Equal(t, rows[1].Version, "1.0.0", "the next row states its copy")
		})

		tests := []struct {
			name     string
			giveFail string
		}{
			{name: "returns the error of the manifest of the copy", giveFail: "File spec/manifest.json"},
			{name: "returns the error of the head of the definition", giveFail: "Branch " + string(definitionRepo)},
			{name: "returns the error of the archive of the definition", giveFail: "Archive " + string(definitionRepo)},
			{name: "returns the error of the pull request", giveFail: "PullRequest " + string(goRepo)},
			{name: "returns the error of the checks", giveFail: "Checks " + string(goRepo)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				f := newFake(t)
				f.fail[tt.giveFail] = errFake
				rows := status.Status(t.Context(), f, both).Rows
				assert.Length(t, rows, 2, "Status reports each implementation")
				assert.ErrorIs(t, rows[0].Err, errFake, "the row states the error of GitHub")
			})
		}
	})
}

// merge returns a copy of base with the files of changes.
func merge(base, changes map[string]string) map[string]string {
	files := maps.Clone(base)
	maps.Copy(files, changes)
	return files
}

// implementationAt returns the files of an implementation with the configuration file config and
// the copy that spec sync writes of first at the commit at.
func implementationAt(tb assert.TB, config, at string) map[string]string {
	tb.Helper()
	files := make(map[string][]byte, len(first))
	for p, text := range first {
		files[p] = []byte(text)
	}
	tree, err := definition.Load(files)
	assert.NoError(tb, err, "the definition of the case loads")
	entries, err := implementation.Parse([]byte(config))
	assert.NoError(tb, err, "the configuration of the implementation parses")
	assert.Length(tb, entries, 1, "the configuration of the case names one definition")
	c, err := vendored.Build(tree, &entries[0], at)
	assert.NoError(tb, err, "the copy of the case builds")
	impl := map[string]string{spec.ConfigFile: config}
	for p, content := range c.Files {
		impl[p] = string(content)
	}
	return impl
}
