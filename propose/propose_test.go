// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package propose_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/golden"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/propose"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// The repositories and the commits of the cases.
const (
	definitionRepo = spec.Repo("dokimasia/assert-spec")
	otherRepo      = spec.Repo("dokimasia/mutate-spec")
	implRepo       = spec.Repo("dokimasia/assert-go")
	firstCommit    = "1111111111111111111111111111111111111111"
	secondCommit   = "2222222222222222222222222222222222222222"
	mainCommit     = "3333333333333333333333333333333333333333"
	otherCommit    = "4444444444444444444444444444444444444444"
	largeCommit    = "5555555555555555555555555555555555555555"
	thirdCommit    = "6666666666666666666666666666666666666666"
)

// versionBlob is the blob of git of the version file of first, as git hash-object computes it.
const versionBlob = "3eefcb9dd5b38e2c1dc061052455dd97bcd51e6c"

// maxBody is the most bytes of the body of a pull request that Propose keeps whole.
const maxBody = 60_000

// cutNote ends the body of a pull request that Propose cuts.
const cutNote = "\n\n`spec propose` cut the rest of this body at 60,000 bytes.\n"

// branch is the branch of the pull request of the definition of the cases.
const branch = propose.BranchPrefix + string(definitionRepo)

// definitionConfig configures the definition of the cases.
const definitionConfig = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "spec/*.json", "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
`

// implConfig configures the implementation of the cases: the go files of the definition, and the
// zone table at internal/zone/zones.json.
const implConfig = `implements:
  - definition: dokimasia/assert-spec
    path: conformance/spec
    languages: [go]
    copies:
      - from: spec/zones.json
        to: internal/zone/zones.json
`

// first is the first revision of the definition of the cases.
var first = map[string]string{
	spec.ConfigFile:      definitionConfig,
	"VERSION":            "1.0.0\n",
	"spec/zones.json":    `{"zones": 1}` + "\n",
	"corpus/a.json":      `{"case": "a"}` + "\n",
	"corpus/b.json":      `{"case": "b"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// second is the next revision of the definition. It changes the zone table and corpus/a.json, and
// it replaces corpus/b.json with corpus/c.json.
var second = map[string]string{
	spec.ConfigFile:      definitionConfig,
	"VERSION":            "1.1.0\n",
	"spec/zones.json":    `{"zones": 2}` + "\n",
	"corpus/a.json":      `{"case": "a", "version": 2}` + "\n",
	"corpus/c.json":      `{"case": "c"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// third is a revision of the definition without overlays, so its copy lacks the entry overlays.
var third = map[string]string{
	spec.ConfigFile: `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "spec/*.json", "corpus/*.json"]
`,
	"VERSION":         "1.2.0\n",
	"spec/zones.json": `{"zones": 3}` + "\n",
	"corpus/a.json":   `{"case": "a", "version": 3}` + "\n",
}

// releaseConfig and developConfig configure the implementation of the cases to follow the newest
// stable release and the branch develop of the definition.
var (
	releaseConfig = strings.Replace(implConfig, "path:", "track: release\n    path:", 1)
	developConfig = strings.Replace(implConfig, "path:", "track: develop\n    path:", 1)
)

// errFake is the error of the call of the fake that a case makes fail.
var errFake = errors.New("github: injected failure")

// fake is a GitHub of the cases in memory: the files of each commit of each repository, the
// branches and the releases, and the pull requests that a proposal opens. Its methods are for one
// goroutine.
type fake struct {
	// commits maps a repository to the files of each of its commits.
	commits map[spec.Repo]map[string]map[string]string

	// branches maps a repository to the commit of each of its branches.
	branches map[spec.Repo]map[string]string

	// releases maps a repository to the tag and the commit of its newest stable release.
	releases map[spec.Repo][2]string

	// pulls maps the branch of each open pull request of the implementation to its number.
	pulls map[string]int

	// subjects maps a commit of a definition to the subject of its message.
	subjects map[string]string

	// fail maps a method to the error that it returns. The key of File and of Branch adds the path
	// and the branch after a space, as in "File .spec.yaml".
	fail map[string]error

	// bodies are the bodies of the pull requests that the fake opened or updated, by number.
	bodies map[int]string

	// titles are the titles of the pull requests that the fake opened or updated, by number.
	titles map[int]string

	// comments are the comments of the pull requests that the fake closed, by number.
	comments map[int]string

	// proposals are the files and the deletions of each commit that the fake made.
	proposals []commit

	// closed are the numbers of the pull requests that the fake closed.
	closed []int

	// next is the number of the next pull request, and the number of the next commit.
	next int
}

// commit is a commit that the fake made on a branch.
type commit struct {
	files   map[string]string
	branch  string
	deleted []string
}

var _ propose.GitHub = (*fake)(nil)

// newFake returns a fake with the definition at first on the branch main, its release v1.0.0 at
// first, and the implementation on main at mainCommit with the files of impl.
func newFake(impl map[string]string) *fake {
	return &fake{
		commits: map[spec.Repo]map[string]map[string]string{
			definitionRepo: {firstCommit: first, secondCommit: second},
			implRepo:       {mainCommit: impl},
		},
		branches: map[spec.Repo]map[string]string{
			definitionRepo: {"main": firstCommit},
			implRepo:       {"main": mainCommit},
		},
		releases: map[spec.Repo][2]string{definitionRepo: {"v1.0.0", firstCommit}},
		pulls:    map[string]int{},
		subjects: map[string]string{secondCommit: "feat: add the case c"},
		fail:     map[string]error{},
		bodies:   map[int]string{},
		titles:   map[int]string{},
		comments: map[int]string{},
		next:     7,
	}
}

// DefaultBranch returns main.
func (f *fake) DefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", f.fail["DefaultBranch"]
}

// File returns the file path of repo at ref, a branch or a commit.
func (f *fake) File(_ context.Context, repo, ref, path string) ([]byte, bool, error) {
	if err := f.fail["File "+path]; err != nil {
		return nil, false, err
	}
	if sha, ok := f.branches[spec.Repo(repo)][ref]; ok {
		ref = sha
	}
	content, ok := f.commits[spec.Repo(repo)][ref][path]
	if !ok {
		return nil, false, nil
	}
	return []byte(content), true, nil
}

// Blobs returns the blob of each file of the commit sha of repo, as git names it.
func (f *fake) Blobs(_ context.Context, repo, sha string) (map[string]string, error) {
	blobs := map[string]string{}
	for p, content := range f.commits[spec.Repo(repo)][sha] {
		blob := sha1.Sum([]byte("blob " + strconv.Itoa(len(content)) + "\x00" + content))
		blobs[p] = hex.EncodeToString(blob[:])
	}
	return blobs, f.fail["Blobs"]
}

// Archive returns the files of the commit sha of repo.
func (f *fake) Archive(_ context.Context, repo, sha string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for p, content := range f.commits[spec.Repo(repo)][sha] {
		files[p] = []byte(content)
	}
	return files, f.fail["Archive"]
}

// Compare returns an error for an empty base, because an empty string is not the name of a commit.
// For a base that repo lacks, it returns false. For a base that is head, it returns an identical
// comparison. For any other base, it returns the commit head as the one commit after base.
func (f *fake) Compare(_ context.Context, repo, base, head string) (github.Comparison, bool, error) {
	if base == "" {
		return github.Comparison{}, false, errors.New("github: compare with an empty base")
	}
	if _, ok := f.commits[spec.Repo(repo)][base]; !ok {
		return github.Comparison{}, false, f.fail["Compare"]
	}
	if base == head {
		return github.Comparison{Status: "identical"}, true, f.fail["Compare"]
	}
	return github.Comparison{
		Status:  "ahead",
		AheadBy: 1,
		Commits: []github.Commit{{SHA: head, Subject: f.subjects[head]}},
	}, true, f.fail["Compare"]
}

// ClosePullRequest records the close of the pull request number with comment.
func (f *fake) ClosePullRequest(_ context.Context, _ string, number int, comment string) error {
	f.closed = append(f.closed, number)
	f.comments[number] = comment
	return f.fail["ClosePullRequest"]
}

// LatestRelease returns the release of repo.
func (f *fake) LatestRelease(_ context.Context, repo string) (string, string, error) {
	r, ok := f.releases[spec.Repo(repo)]
	if !ok {
		return "", "", github.ErrNoRelease
	}
	return r[0], r[1], f.fail["LatestRelease"]
}

// Branch returns the commit of the branch name of repo.
func (f *fake) Branch(_ context.Context, repo, name string) (string, bool, error) {
	if err := f.fail["Branch "+name]; err != nil {
		return "", false, err
	}
	sha, ok := f.branches[spec.Repo(repo)][name]
	return sha, ok, nil
}

// SetBranch points the branch name of repo at sha.
func (f *fake) SetBranch(_ context.Context, repo, name, sha string) error {
	f.branches[spec.Repo(repo)][name] = sha
	return nil
}

// DeleteBranch deletes the branch name of repo.
func (f *fake) DeleteBranch(_ context.Context, repo, name string) error {
	delete(f.branches[spec.Repo(repo)], name)
	return nil
}

// Commit makes a commit of files and of the deletion of deleted on head of repo, and records it.
func (f *fake) Commit(
	_ context.Context, repo, branch, head, _ string, files map[string][]byte, deleted []string,
) (string, error) {
	if err := f.fail["Commit"]; err != nil {
		return "", err
	}
	tree := maps.Clone(f.commits[spec.Repo(repo)][head])
	written := map[string]string{}
	for p, content := range files {
		tree[p] = string(content)
		written[p] = string(content)
	}
	for _, p := range deleted {
		delete(tree, p)
	}
	f.next++
	sha := fmt.Sprintf("%040x", f.next)
	f.commits[spec.Repo(repo)][sha] = tree
	f.proposals = append(f.proposals, commit{files: written, branch: branch, deleted: deleted})
	return sha, nil
}

// PullRequest returns the number of the open pull request from head.
func (f *fake) PullRequest(_ context.Context, _, head, _ string) (int, bool, error) {
	n, ok := f.pulls[head]
	return n, ok, f.fail["PullRequest"]
}

// CreatePullRequest opens a pull request from head, and returns its number.
func (f *fake) CreatePullRequest(_ context.Context, _, head, _, title, body string) (int, error) {
	f.next++
	f.pulls[head] = f.next
	f.titles[f.next], f.bodies[f.next] = title, body
	return f.next, nil
}

// UpdatePullRequest records the title and the body of the pull request number.
func (f *fake) UpdatePullRequest(_ context.Context, _ string, number int, title, body string) error {
	f.titles[number], f.bodies[number] = title, body
	return nil
}

func TestPropose(t *testing.T) {
	t.Parallel()

	t.Run("Propose", func(t *testing.T) {
		t.Parallel()

		t.Run("opens a pull request whose branch has the copy of the head of the definition", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			assert.Equal(t, proposeOne(t, f), propose.Proposal{
				Definition: definitionRepo,
				Version:    "1.1.0",
				Commit:     secondCommit,
				Pull:       f.pulls[branch],
				Action:     propose.Proposed,
			}, "Propose reports the pull request of the definition")
			assert.Equal(
				t,
				f.commits[implRepo][f.branches[implRepo][branch]],
				implementationAt(t, implConfig, second, secondCommit),
				"the branch has the files that spec sync writes",
			)
		})

		t.Run("commits only the files that differ from the base", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			proposeOne(t, f)
			assert.Length(t, f.proposals, 1, "the proposal is one commit")
			assert.NotContains(
				t,
				f.proposals[0].files,
				"conformance/spec/overlays/go.json",
				"an equal file is no change",
			)
			assert.Equal(t, f.proposals[0].deleted, []string{"conformance/spec/corpus/b.json"}, "the dropped case")
		})

		t.Run("titles the pull request after the definition and its version", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			assert.Equal(
				t,
				f.titles[proposeOne(t, f).Pull],
				"build: sync assert-spec to 1.1.0",
				"the title names the definition and its version",
			)
		})

		t.Run("writes the body of a pull request", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			golden.MatchAt(
				t,
				filepath.Join("testdata", "golden", "body.md"),
				[]byte(f.bodies[proposeOne(t, f).Pull]),
				golden.ShouldUpdate(),
			)
		})

		t.Run("writes an empty changeset in a repository of changesets", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			impl[".changeset/config.json"] = `{"baseBranch": "main"}` + "\n"
			f := newFake(impl)
			f.branches[definitionRepo]["main"] = secondCommit
			proposeOne(t, f)
			assert.Equal(
				t,
				f.commits[implRepo][f.branches[implRepo][branch]][".changeset/spec-sync-dokimasia-assert-spec.md"],
				"---\n---\n\nSync the vendored copy of assert-spec to 1.1.0.\n",
				"the branch has a changeset that releases nothing",
			)
		})

		t.Run("updates the open pull request of the definition", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			f.pulls[branch] = 5
			assert.Equal(t, proposeOne(t, f).Pull, 5, "Propose reports the open pull request")
			assert.Equal(t, f.titles[5], "build: sync assert-spec to 1.1.0", "the pull request has the new title")
		})

		t.Run("closes the open pull request of a current copy", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.pulls[branch] = 5
			assert.Equal(t, proposeOne(t, f), propose.Proposal{
				Definition: definitionRepo,
				Version:    "1.0.0",
				Commit:     firstCommit,
				Pull:       5,
				Action:     propose.Closed,
			}, "Propose reports the closed pull request")
			assert.Equal(t, f.closed, []int{5}, "Propose closes the pull request of the current copy")
			assert.Equal(
				t,
				f.comments[5],
				"`spec propose` closed this pull request. The vendored copy of dokimasia/assert-spec on the base "+
					"branch is current with 1.0.0 at dokimasia/assert-spec@"+firstCommit+".",
				"the comment states the version and the commit of the copy",
			)
		})

		t.Run("returns Current for a current copy without a pull request", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			blobs, err := f.Blobs(t.Context(), string(implRepo), mainCommit)
			assert.NoError(t, err, "the fake returns the blobs")
			assert.Equal(t, blobs["conformance/spec/VERSION"], versionBlob, "the fake names a blob as git does")
			assert.Equal(t, proposeOne(t, f).Action, propose.Current, "the copy is current")
			assert.Empty(t, f.proposals, "Propose commits nothing")
		})

		t.Run("follows the newest release for the track release", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, releaseConfig, first, firstCommit))
			f.releases[definitionRepo] = [2]string{"v1.1.0", secondCommit}
			p := proposeOne(t, f)
			assert.Equal(t, p.Commit, secondCommit, "Propose syncs to the commit of the release")
			assert.Contains(t, f.bodies[p.Pull], "to the release v1.1.0.", "the body names the release")
		})

		t.Run("leaves the commits out of the body of a copy of another repository", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			old, err := manifest.Decode([]byte(impl["conformance/spec/manifest.json"]))
			assert.NoError(t, err, "the manifest of the copy decodes")
			old.Repo = otherRepo
			impl["conformance/spec/manifest.json"] = string(old.Encode())
			f := newFake(impl)
			f.branches[definitionRepo]["main"] = secondCommit
			expect.That(t, f.bodies[proposeOne(t, f).Pull]).
				Contains("- From: 1.0.0 at dokimasia/mutate-spec@"+firstCommit+"\n", "the body names the old source").
				NotContains("feat: add the case c", "the body lists no commit of another repository")
		})

		t.Run("leaves the commits out of the body of a local copy", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, ""))
			f.branches[definitionRepo]["main"] = secondCommit
			expect.That(t, f.bodies[proposeOne(t, f).Pull]).
				Contains("- From: 1.0.0 of a local checkout\n", "the body names the local source").
				NotContains("feat: add the case c", "the body lists no commit of a checkout")
		})

		t.Run("leaves the commits out of the body of a copy whose commit the definition lacks", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, otherCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			assert.NotContains(t, f.bodies[proposeOne(t, f).Pull], "## Commits", "the body lists no commit")
		})

		t.Run("proposes a copy whose manifest does not decode", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			impl["conformance/spec/manifest.json"] = "{}\n"
			f := newFake(impl)
			p := proposeOne(t, f)
			assert.Equal(t, p.Action, propose.Proposed, "the copy differs from the definition")
			assert.Contains(
				t,
				f.bodies[p.Pull],
				"- From: a copy without a valid manifest\n",
				"the body states that the old version is unknown",
			)
		})

		t.Run("proposes an edited file at the commit of the copy", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			impl["conformance/spec/corpus/a.json"] = `{"case": "edited"}` + "\n"
			f := newFake(impl)
			assert.Equal(t, proposeOne(t, f).Action, propose.Proposed, "the copy differs from the definition")
			assert.Equal(
				t,
				f.commits[implRepo][f.branches[implRepo][branch]],
				implementationAt(t, implConfig, first, firstCommit),
				"the branch has the file of the definition",
			)
		})

		t.Run("proposes the removal of the files that the copy lacks in the order of their paths", func(t *testing.T) {
			t.Parallel()
			const count = 20
			impl := implementationAt(t, implConfig, first, firstCommit)
			want, items := make([]string, 0, count), make([]string, 0, count)
			for i := range count {
				file := fmt.Sprintf("conformance/spec/corpus/x%02d.json", i)
				impl[file] = "{}\n"
				want = append(want, file)
				items = append(items, "- `"+file+"`")
			}
			f := newFake(impl)
			p := proposeOne(t, f)
			assert.Equal(t, p.Action, propose.Proposed, "the copy has files too many")
			assert.Length(t, f.proposals, 1, "the proposal is one commit")
			assert.Equal(t, f.proposals[0].deleted, want, "the commit removes the files in the order of their paths")
			assert.HasSuffix(
				t,
				f.bodies[p.Pull],
				"## Removed files\n\n"+strings.Join(items, "\n")+"\n",
				"the body lists the files in the order of their paths",
			)
		})

		t.Run("leaves the other files of the directory of the copy as they are", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			impl["conformance/spec/definition.go"] = "package spec\n"
			impl["conformance/spec/testdata/x.json"] = "{}\n"
			impl["VERSION"] = "0.1.0\n"
			f := newFake(impl)
			f.branches[definitionRepo]["main"] = secondCommit
			proposeOne(t, f)
			assert.Length(t, f.proposals, 1, "the proposal is one commit")
			assert.Equal(
				t,
				f.proposals[0].deleted,
				[]string{"conformance/spec/corpus/b.json"},
				"the dropped case alone",
			)
		})

		t.Run("proposes the removal of an entry of the old copy that the new copy lacks", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.commits[definitionRepo][thirdCommit] = third
			f.branches[definitionRepo]["main"] = thirdCommit
			proposeOne(t, f)
			assert.Length(t, f.proposals, 1, "the proposal is one commit")
			assert.Equal(
				t,
				f.proposals[0].deleted,
				[]string{"conformance/spec/corpus/b.json", "conformance/spec/overlays/go.json"},
				"the dropped case and the overlay",
			)
		})

		t.Run("proposes one pull request for each definition", func(t *testing.T) {
			t.Parallel()
			impl := implementationAt(t, implConfig, first, firstCommit)
			impl[spec.ConfigFile] = implConfig + `  - definition: dokimasia/mutate-spec
    path: conformance/mutate
    languages: [go]
`
			f := newFake(impl)
			f.commits[otherRepo] = map[string]map[string]string{otherCommit: {
				spec.ConfigFile: "definition: {repo: dokimasia/mutate-spec, languages: [go], files: [VERSION]}\n",
				"VERSION":       "2.0.0\n",
			}}
			f.branches[otherRepo] = map[string]string{"main": otherCommit}
			result, err := propose.Propose(t.Context(), f, &propose.Request{Repo: implRepo})
			assert.NoError(t, err, "Propose reads both definitions")
			assert.Length(t, result.Proposals, 2, "Propose reports each definition")
			assert.Equal(t, result.Proposals[0].Action, propose.Current, "the copy of assert-spec is current")
			assert.Equal(t, result.Proposals[1].Definition, otherRepo, "Propose keeps the order of the configuration")
			assert.Equal(t, result.Proposals[1].Action, propose.Proposed, "the copy of mutate-spec is missing")
		})

		t.Run("writes a subject with backticks as one code span", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			f.branches[definitionRepo]["main"] = secondCommit
			f.subjects[secondCommit] = "fix: quote `a` and ``b``"
			assert.Contains(
				t,
				f.bodies[proposeOne(t, f).Pull],
				"@"+secondCommit+" ``` fix: quote `a` and ``b`` ```\n",
				"a fence longer than every run of backticks encloses the subject",
			)
		})

		t.Run("keeps a body of 60,000 bytes whole", func(t *testing.T) {
			t.Parallel()
			short := bodyWith(t, "x")
			body := bodyWith(t, strings.Repeat("x", 1+maxBody-len(short)))
			assert.Length(t, []byte(body), maxBody, "the body has every line")
		})

		t.Run("cuts a body above 60,000 bytes after its last whole line below the limit", func(t *testing.T) {
			t.Parallel()
			short := bodyWith(t, "x")
			name := strings.Repeat("x", 2+maxBody-len(short))
			whole := strings.Replace(short, "/x.json`", "/"+name+".json`", 1)
			assert.Length(t, []byte(whole), maxBody+1, "the whole body has one byte above the limit")
			assert.Equal(
				t,
				bodyWith(t, name),
				whole[:strings.LastIndexByte(whole[:maxBody], '\n')]+cutNote,
				"the body ends with the note after the last line feed below the limit",
			)
		})

		t.Run("returns ErrRepository for a repository that is not valid", func(t *testing.T) {
			t.Parallel()
			f := newFake(implementationAt(t, implConfig, first, firstCommit))
			_, err := propose.Propose(t.Context(), f, &propose.Request{Repo: "assert-go"})
			assert.ErrorIs(t, err, remote.ErrRepository, "Propose refuses a name without an owner")
		})

		tests := []struct {
			name     string
			give     func(f *fake)
			giveFail string
			want     error
		}{
			{name: "returns the error of the default branch", giveFail: "DefaultBranch", want: errFake},
			{name: "returns the error of the head of the implementation", giveFail: "Branch main", want: errFake},
			{name: "returns the error of the configuration file", giveFail: "File " + spec.ConfigFile, want: errFake},
			{name: "returns the error of the blobs", giveFail: "Blobs", want: errFake},
			{
				name: "returns the error of the head of the definition",
				give: func(f *fake) {
					f.commits[implRepo][mainCommit][spec.ConfigFile] = developConfig
					f.branches[definitionRepo]["develop"] = firstCommit
				},
				giveFail: "Branch develop",
				want:     errFake,
			},
			{
				name:     "returns the error of the newest release",
				give:     func(f *fake) { f.commits[implRepo][mainCommit][spec.ConfigFile] = releaseConfig },
				giveFail: "LatestRelease",
				want:     errFake,
			},
			{name: "returns the error of the archive", giveFail: "Archive", want: errFake},
			{name: "returns the error of the pull request of a current copy", giveFail: "PullRequest", want: errFake},
			{
				name:     "returns the error of the close of a pull request",
				give:     func(f *fake) { f.pulls[branch] = 5 },
				giveFail: "ClosePullRequest",
				want:     errFake,
			},
			{
				name:     "returns the error of the manifest of the copy",
				give:     func(f *fake) { f.branches[definitionRepo]["main"] = secondCommit },
				giveFail: "File conformance/spec/manifest.json",
				want:     errFake,
			},
			{
				name:     "returns the error of the comparison",
				give:     func(f *fake) { f.branches[definitionRepo]["main"] = secondCommit },
				giveFail: "Compare",
				want:     errFake,
			},
			{
				name:     "returns the error of the commit of a proposal",
				give:     func(f *fake) { f.branches[definitionRepo]["main"] = secondCommit },
				giveFail: "Commit",
				want:     errFake,
			},
			{
				name: "returns ErrRepository for a repository without a configuration file",
				give: func(f *fake) { delete(f.commits[implRepo][mainCommit], spec.ConfigFile) },
				want: remote.ErrRepository,
			},
			{
				name: "returns ErrRepository for a repository without its default branch",
				give: func(f *fake) { delete(f.branches[implRepo], "main") },
				want: remote.ErrRepository,
			},
			{
				name: "returns implementation.ErrConfig for a configuration that breaks a rule",
				give: func(f *fake) { f.commits[implRepo][mainCommit][spec.ConfigFile] = "implements: []\n" },
				want: implementation.ErrConfig,
			},
			{
				name: "returns ErrTrack for a branch that the definition lacks",
				give: func(f *fake) { f.commits[implRepo][mainCommit][spec.ConfigFile] = developConfig },
				want: remote.ErrTrack,
			},
			{
				name: "returns github.ErrNoRelease for a definition without a release",
				give: func(f *fake) {
					delete(f.releases, definitionRepo)
					f.commits[implRepo][mainCommit][spec.ConfigFile] = releaseConfig
				},
				want: github.ErrNoRelease,
			},
			{
				name: "returns definition.ErrConfig for a definition without its section",
				give: func(f *fake) { f.commits[definitionRepo][firstCommit] = map[string]string{spec.ConfigFile: "x: 1\n"} },
				want: definition.ErrConfig,
			},
			{
				name: "returns manifest.ErrLanguage for a language that the definition lacks",
				give: func(f *fake) {
					f.commits[implRepo][mainCommit][spec.ConfigFile] = strings.Replace(
						implConfig,
						"[go]",
						"[python]",
						1,
					)
				},
				want: manifest.ErrLanguage,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				f := newFake(implementationAt(t, implConfig, first, firstCommit))
				if tt.give != nil {
					tt.give(f)
				}
				if tt.giveFail != "" {
					f.fail[tt.giveFail] = errFake
				}
				_, err := propose.Propose(t.Context(), f, &propose.Request{Repo: implRepo})
				assert.ErrorIs(t, err, tt.want, "Propose returns the error")
			})
		}
	})
}

// implementationAt returns the files of an implementation with the configuration file config and
// the copy that spec sync writes of the definition contents at the commit at, or of a checkout for
// an empty at.
func implementationAt(tb assert.TB, config string, contents map[string]string, at string) map[string]string {
	tb.Helper()
	files := make(map[string][]byte, len(contents))
	for p, text := range contents {
		files[p] = []byte(text)
	}
	tree, err := definition.Load(files)
	assert.NoError(tb, err, "the definition of the case loads")
	entries, err := implementation.Parse([]byte(config))
	assert.NoError(tb, err, "the configuration of the implementation parses")
	assert.Length(tb, entries, 1, "the configuration of the case names one definition")
	c, err := vendored.Build(tree, &entries[0], at)
	assert.NoError(tb, err, "the copy of the case builds")
	impl := map[string]string{spec.ConfigFile: config, "go.mod": "module a\n"}
	for p, content := range c.Files {
		impl[p] = string(content)
	}
	return impl
}

// bodyWith returns the body of the pull request that syncs the implementation of the cases from
// first to second with one more case file. The base name of that file is name.
func bodyWith(t *testing.T, name string) string {
	t.Helper()
	files := maps.Clone(second)
	files["corpus/"+name+".json"] = `{"case": "large"}` + "\n"
	f := newFake(implementationAt(t, implConfig, first, firstCommit))
	f.commits[definitionRepo][largeCommit] = files
	f.branches[definitionRepo]["main"] = largeCommit
	return f.bodies[proposeOne(t, f).Pull]
}

// proposeOne runs Propose on the implementation of the cases in f, and returns its one proposal.
func proposeOne(t *testing.T, f *fake) propose.Proposal {
	t.Helper()
	result, err := propose.Propose(t.Context(), f, &propose.Request{Repo: implRepo})
	assert.NoError(t, err, "Propose syncs the implementation")
	assert.Length(t, result.Proposals, 1, "Propose reports the definition of the implementation")
	return result.Proposals[0]
}
