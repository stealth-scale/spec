// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package remote_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// The repositories and the commits of the cases.
const (
	definitionRepo = "dokimasia/assert-spec"
	implRepo       = "dokimasia/assert-go"
	headCommit     = "1111111111111111111111111111111111111111"
	releaseCommit  = "2222222222222222222222222222222222222222"
	implCommit     = "3333333333333333333333333333333333333333"
)

// implConfig configures the implementation of the cases.
const implConfig = "implements: [{definition: dokimasia/assert-spec, path: spec, languages: [go]}]\n"

// files are the files of the definition of the cases.
var files = map[string][]byte{
	spec.ConfigFile: []byte(`definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
`),
	"VERSION":            []byte("1.0.0\n"),
	"corpus/a.json":      []byte("{}\n"),
	"overlays/go.json":   []byte(`{"language": "go"}` + "\n"),
	"overlays/rust.json": []byte(`{"language": "rust"}` + "\n"),
}

// errFake is the error of the call of the fake that a case makes fail.
var errFake = errors.New("github: injected failure")

// fake is a GitHub of the cases in memory: the default branch main of each repository, the commit
// of each branch, the files of each commit, and the newest stable release of the definition.
type fake struct {
	// branches maps a repository to the commit of each of its branches.
	branches map[string]map[string]string

	// commits maps a repository to the files of each of its commits.
	commits map[string]map[string]map[string][]byte

	// fail maps a method to the error that it returns.
	fail map[string]error

	// release is the tag and the commit of the newest stable release of the definition, and empty
	// for a definition without one.
	release [2]string
}

var _ remote.GitHub = (*fake)(nil)

// newFake returns a fake with the definition on main at headCommit, its release v1.0.0 at
// releaseCommit, and the implementation on main at implCommit with the configuration file config.
func newFake(config string) *fake {
	return &fake{
		branches: map[string]map[string]string{
			definitionRepo: {"main": headCommit},
			implRepo:       {"main": implCommit},
		},
		commits: map[string]map[string]map[string][]byte{
			definitionRepo: {headCommit: files, releaseCommit: files},
			implRepo:       {implCommit: {spec.ConfigFile: []byte(config)}},
		},
		fail:    map[string]error{},
		release: [2]string{"v1.0.0", releaseCommit},
	}
}

// DefaultBranch returns main.
func (f *fake) DefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", f.fail["DefaultBranch"]
}

// Branch returns the commit of the branch name of repo.
func (f *fake) Branch(_ context.Context, repo, name string) (string, bool, error) {
	sha, ok := f.branches[repo][name]
	return sha, ok, f.fail["Branch"]
}

// File returns the file path of repo at the commit ref.
func (f *fake) File(_ context.Context, repo, ref, path string) ([]byte, bool, error) {
	content, ok := f.commits[repo][ref][path]
	return content, ok, f.fail["File"]
}

// LatestRelease returns the release of the fake, and github.ErrNoRelease for a fake without one.
func (f *fake) LatestRelease(_ context.Context, _ string) (string, string, error) {
	if f.release[0] == "" {
		return "", "", github.ErrNoRelease
	}
	return f.release[0], f.release[1], f.fail["LatestRelease"]
}

// Archive returns the files of the commit sha of repo.
func (f *fake) Archive(_ context.Context, repo, sha string) (map[string][]byte, error) {
	return f.commits[repo][sha], f.fail["Archive"]
}

func TestRemote(t *testing.T) {
	t.Parallel()

	entry := &implementation.Entry{
		Definition: definitionRepo,
		Track:      implementation.DefaultTrack,
		Path:       "spec",
		Languages:  []spec.Language{"go"},
	}

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the entries of the configuration file at the head of the default branch", func(t *testing.T) {
			t.Parallel()
			impl, err := remote.Open(t.Context(), newFake(implConfig), implRepo)
			assert.NoError(t, err, "Open reads the implementation")
			assert.Equal(t, impl, &remote.Implementation{
				Repo:    implRepo,
				Base:    "main",
				Head:    implCommit,
				Entries: []implementation.Entry{*entry},
			}, "Open returns the branch, its head and the entries")
		})

		t.Run("returns ErrRepository for a repository that is not valid", func(t *testing.T) {
			t.Parallel()
			_, err := remote.Open(t.Context(), newFake(implConfig), "assert-go")
			assert.ErrorIs(t, err, remote.ErrRepository, "Open refuses a name without an owner")
			assert.Contains(t, err.Error(), `"assert-go" is no repository of GitHub`, "the error names the repository")
		})

		tests := []struct {
			name     string
			give     func(f *fake)
			giveFail string
			want     error
			wantText string
		}{
			{
				name:     "returns the error of the default branch",
				giveFail: "DefaultBranch",
				want:     errFake,
				wantText: "the default branch of " + implRepo,
			},
			{
				name:     "returns the error of the head",
				giveFail: "Branch",
				want:     errFake,
				wantText: "the head of main",
			},
			{
				name:     "returns the error of the configuration file",
				giveFail: "File",
				want:     errFake,
				wantText: "read " + spec.ConfigFile,
			},
			{
				name:     "returns ErrRepository for a repository without its default branch",
				give:     func(f *fake) { delete(f.branches[implRepo], "main") },
				want:     remote.ErrRepository,
				wantText: implRepo + " has no branch main",
			},
			{
				name:     "returns ErrRepository for a default branch without the configuration file",
				give:     func(f *fake) { delete(f.commits[implRepo][implCommit], spec.ConfigFile) },
				want:     remote.ErrRepository,
				wantText: implRepo + " has no " + spec.ConfigFile + " on main",
			},
			{
				name:     "returns implementation.ErrConfig for a configuration that breaks a rule",
				give:     func(f *fake) { f.commits[implRepo][implCommit][spec.ConfigFile] = []byte("implements: []\n") },
				want:     implementation.ErrConfig,
				wantText: "the section implements lists no definition",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				f := newFake(implConfig)
				if tt.give != nil {
					tt.give(f)
				}
				if tt.giveFail != "" {
					f.fail[tt.giveFail] = errFake
				}
				_, err := remote.Open(t.Context(), f, implRepo)
				assert.ErrorIs(t, err, tt.want, "Open returns the error")
				assert.Contains(t, err.Error(), tt.wantText, "the error names the step that fails")
			})
		}
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the head of the branch of the track", func(t *testing.T) {
			t.Parallel()
			target, err := remote.Resolve(t.Context(), newFake(implConfig), entry)
			assert.NoError(t, err, "Resolve reads the branch")
			assert.Equal(t, target, remote.Target{Commit: headCommit}, "the target is the head without a tag")
		})

		t.Run("returns the newest stable release for the track release", func(t *testing.T) {
			t.Parallel()
			release := *entry
			release.Track = implementation.Release
			target, err := remote.Resolve(t.Context(), newFake(implConfig), &release)
			assert.NoError(t, err, "Resolve reads the releases")
			assert.Equal(t, target, remote.Target{Commit: releaseCommit, Tag: "v1.0.0"}, "the target is the release")
		})

		tests := []struct {
			name      string
			giveTrack string
			giveFail  string
			want      error
		}{
			{
				name:      "returns ErrTrack for a branch that the definition lacks",
				giveTrack: "develop",
				want:      remote.ErrTrack,
			},
			{name: "returns the error of the branch", giveTrack: "main", giveFail: "Branch", want: errFake},
			{
				name:      "returns the error of the newest release",
				giveTrack: implementation.Release,
				giveFail:  "LatestRelease",
				want:      errFake,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				e := *entry
				e.Track = tt.giveTrack
				f := newFake(implConfig)
				f.fail[tt.giveFail] = errFake
				_, err := remote.Resolve(t.Context(), f, &e)
				assert.ErrorIs(t, err, tt.want, "Resolve returns the error")
			})
		}

		t.Run("returns github.ErrNoRelease for a definition without a stable release", func(t *testing.T) {
			t.Parallel()
			release := *entry
			release.Track = implementation.Release
			f := newFake(implConfig)
			f.release = [2]string{}
			_, err := remote.Resolve(t.Context(), f, &release)
			assert.ErrorIs(t, err, github.ErrNoRelease, "Resolve returns the error of the host")
		})
	})

	t.Run("Fetch", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the copy of the definition at the commit", func(t *testing.T) {
			t.Parallel()
			c, err := remote.Fetch(t.Context(), newFake(implConfig), entry, headCommit)
			assert.NoError(t, err, "Fetch builds the copy")
			tree, err := definition.Load(files)
			assert.NoError(t, err, "the definition of the case loads")
			want, err := vendored.Build(tree, entry, headCommit)
			assert.NoError(t, err, "the copy of the case builds")
			assert.Equal(t, c, want, "Fetch returns the copy that vendored.Build builds from the archive")
		})

		t.Run("returns the error of the archive", func(t *testing.T) {
			t.Parallel()
			f := newFake(implConfig)
			f.fail["Archive"] = errFake
			_, err := remote.Fetch(t.Context(), f, entry, headCommit)
			assert.ErrorIs(t, err, errFake, "Fetch returns the error of the host")
		})

		t.Run("returns definition.ErrConfig for an archive without the configuration file", func(t *testing.T) {
			t.Parallel()
			f := newFake(implConfig)
			f.commits[definitionRepo][headCommit] = map[string][]byte{"VERSION": []byte("1.0.0\n")}
			_, err := remote.Fetch(t.Context(), f, entry, headCommit)
			assert.ErrorIs(t, err, definition.ErrConfig, "Fetch returns the error of the definition")
		})

		t.Run("returns manifest.ErrLanguage for a language that the definition lacks", func(t *testing.T) {
			t.Parallel()
			python := *entry
			python.Languages = []spec.Language{"python"}
			_, err := remote.Fetch(t.Context(), newFake(implConfig), &python, headCommit)
			assert.ErrorIs(t, err, manifest.ErrLanguage, "Fetch returns the error of the copy")
		})
	})
}
