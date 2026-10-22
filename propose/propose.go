// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package propose

import (
	"context"
	"crypto/sha1" //nolint:gosec // git names the content of a file by the SHA-1 of its blob.
	"encoding/hex"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/ergon/service/release"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
	"go.stealthscale.io/spec/remote"
	"go.stealthscale.io/spec/vendored"
)

// BranchPrefix starts the branch of the pull request of a definition. The repository of the
// definition follows it, as in spec-sync/dokimasia/assert-spec.
const BranchPrefix = "spec-sync/"

// changesetConfig is the configuration file of changesets. A proposal into a repository with this
// file adds an empty changeset, because ergon refuses a change of a module without a changeset.
const changesetConfig = ".changeset/config.json"

// maxBody is the most bytes of the body of a pull request that [Propose] keeps whole. GitHub
// accepts a body of up to 65,536 characters.
const maxBody = 60_000

// blobPrefix starts the header that git hashes before the content of a blob. The length of the
// content in decimal and a NUL byte follow it.
const blobPrefix = "blob "

// backtick is the delimiter of a Markdown code span.
const backtick = "`"

// The texts of a pull request.
const (
	// titleFormat is the title of a pull request and the message of its commits. It takes the name
	// of the definition and its version.
	titleFormat = "build: sync %s to %s"

	// changesetFile is the path of the changeset of a pull request. It takes the repository of the
	// definition with a dash for the slash.
	changesetFile = ".changeset/spec-sync-%s.md"

	// changesetFormat is the changeset of a pull request, which releases nothing. It takes the name
	// of the definition and its version.
	changesetFormat = "---\n---\n\nSync the vendored copy of %s to %s.\n"

	// introFormat opens the body. It takes the repository of the definition, the directory of the
	// copy and the commit of the definition in the words of headFormat or releaseFormat.
	introFormat = "`spec propose` opened this pull request. It syncs the vendored copy of %s at %s to %s."

	// headFormat names the head of a branch. It takes the branch.
	headFormat = "the head of %s"

	// releaseFormat names a release. It takes the tag of the release.
	releaseFormat = "the release %s"

	// fromFormat states the source of the copy on the base branch. It takes the version, the
	// repository and the commit of the manifest of that copy.
	fromFormat = "- From: %s at %s@%s"

	// localFormat states the source of a copy of a checkout on the base branch. It takes the version
	// of the manifest of that copy.
	localFormat = "- From: %s of a local checkout"

	// unknownSource states that the copy on the base branch has no manifest that decodes.
	unknownSource = "- From: a copy without a valid manifest"

	// toFormat states the source of the copy of the pull request. It takes the version, the
	// repository of the definition and the commit.
	toFormat = "- To: %s at %s@%s"

	// commitFormat lists a commit of the definition. It takes the repository of the definition, the
	// commit and the code span of its subject.
	commitFormat = "- %s@%s %s"

	// item starts the item of a list.
	item = "- "

	// cutNote ends a body that [Propose] cuts at maxBody.
	cutNote = "\n\n`spec propose` cut the rest of this body at 60,000 bytes.\n"

	// closeFormat is the comment on a pull request that [Propose] closes. It takes the repository of
	// the definition, the version and the commit of the copy.
	closeFormat = "`spec propose` closed this pull request. The vendored copy of %[1]s on the base branch " +
		"is current with %[2]s at %[1]s@%[3]s."
)

// The headings of the sections of the body of a pull request.
const (
	commitsHeading = "## Commits"
	changedHeading = "## Changed files"
	removedHeading = "## Removed files"
)

// GitHub is the host of an implementation repository and of its definitions, which [Propose]
// reads and writes through. [*github.Client] implements it. Each method takes a repository as
// owner/name.
//
// # Concurrency
//
// Propose calls the methods of a GitHub one at a time, on the goroutine of its caller.
type GitHub interface {
	release.Proposer
	remote.GitHub

	// Blobs returns the blob of each file of the commit sha of repo, as git names it, by the
	// slash-separated path of the file.
	Blobs(ctx context.Context, repo, sha string) (map[string]string, error)

	// Compare returns the comparison of the commit head of repo with the commit base, and reports
	// whether repo has both commits.
	Compare(ctx context.Context, repo, base, head string) (github.Comparison, bool, error)

	// ClosePullRequest writes comment on the pull request number of repo, and then closes it.
	ClosePullRequest(ctx context.Context, repo string, number int, comment string) error
}

var _ GitHub = (*github.Client)(nil)

// Action is what [Propose] did for the definition of an entry.
type Action string

// The actions of [Propose].
const (
	// Current is the action for a copy whose files match the definition, without an open pull
	// request.
	Current Action = "current"

	// Closed is the action for a copy whose files match the definition, whose open pull request
	// Propose closed.
	Closed Action = "closed"

	// Proposed is the action for a copy whose files differ from the definition, whose pull request
	// Propose opened or updated.
	Proposed Action = "proposed"
)

// Proposal is what [Propose] did for the definition of an entry of the implementation.
//
// # Concurrency
//
// A Proposal is a value, which any number of goroutines may read at once.
type Proposal struct {
	// Definition is the repository of the definition.
	Definition spec.Repo

	// Version is the version of the definition at Commit.
	Version string

	// Commit is the commit of the definition that the entry follows.
	Commit string

	// Action is what Propose did.
	Action Action

	// Pull is the number of the pull request that Propose closed, opened or updated, and 0 for
	// [Current].
	Pull int
}

// Request is the implementation repository that [Propose] syncs.
type Request struct {
	// Repo is the implementation repository.
	Repo spec.Repo
}

// Result is what [Propose] did for each entry of the implementation.
//
// # Concurrency
//
// A Result is a value, which any number of goroutines may read at once.
type Result struct {
	// Proposals has the proposal of each entry, in the order of the configuration file.
	Proposals []Proposal
}

// Propose syncs the vendored copies of the implementation repository req.Repo through gh. It reads
// the configuration file of the repository at the head of its default branch. For each entry it
// builds the copy of the commit of the definition that the entry follows, which is the head of a
// branch or the newest stable release. It compares the files of that copy with the blobs of the
// head:
//
//   - A copy whose files match, without an open pull request, gets the action [Current].
//   - A copy whose files match gets the action [Closed]. Propose closes its open pull request with a
//     comment.
//   - A copy whose files differ gets the action [Proposed]. Propose opens or updates its pull
//     request.
//
// The pull request of a definition comes from the branch [BranchPrefix] and the repository of the
// definition, such as spec-sync/dokimasia/assert-spec. Its commits write each file of the copy
// whose blob differs, and remove each file that the copy lacks in an entry of the copy or of the
// copy on the default branch, as vendored.Entries names the entries. The other files of the
// directory of the copy belong to the repository, and the pull request leaves them as they are. A
// repository with the configuration of changesets also gets an empty changeset. The body states the
// version and the commit of the copy before and after, the commits of the definition between the
// two, and the files that change. Propose cuts a body above 60,000 bytes at a line.
//
// Error modes:
//   - the error of remote.Open, which wraps remote.ErrRepository for a repository that is no
//     implementation, and implementation.ErrConfig for a configuration that breaks a rule.
//   - the error of remote.Resolve, which wraps remote.ErrTrack for a definition without the branch
//     that an entry follows.
//   - the error of remote.Fetch for a definition that breaks a rule, such as the error that wraps
//     manifest.ErrLanguage for a language that the definition lacks.
//   - the error of release.Propose, which wraps release.ErrMoved for a default branch whose head
//     moved during the call.
//   - the error of gh for a call that fails, such as the error of a definition without a stable
//     release, which wraps github.ErrNoRelease for a [*github.Client].
func Propose(ctx context.Context, gh GitHub, req *Request) (Result, error) {
	impl, err := remote.Open(ctx, gh, req.Repo)
	if err != nil {
		return Result{}, fmt.Errorf("propose: %w", err)
	}
	name := string(impl.Repo)
	blobs, err := gh.Blobs(ctx, name, impl.Head)
	if err != nil {
		return Result{}, fmt.Errorf("propose: the files of %s: %w", name, err)
	}
	_, changesets := blobs[changesetConfig]
	r := repository{gh: gh, blobs: blobs, name: name, base: impl.Base, head: impl.Head, changesets: changesets}
	result := Result{Proposals: make([]Proposal, 0, len(impl.Entries))}
	for i := range impl.Entries {
		p, err := r.propose(ctx, &impl.Entries[i])
		if err != nil {
			return Result{}, err
		}
		result.Proposals = append(result.Proposals, p)
	}
	return result, nil
}

// repository is the implementation repository of a call of [Propose]: its host, the blobs of the
// head of its default branch, its name, that branch and its head, and whether it has the
// configuration of changesets.
type repository struct {
	gh         GitHub
	blobs      map[string]string
	name       string
	base       string
	head       string
	changesets bool
}

// propose syncs the copy of the entry e, and returns what it did, as [Propose] states. It returns
// the errors that Propose states for one entry.
func (r *repository) propose(ctx context.Context, e *implementation.Entry) (Proposal, error) {
	target, err := remote.Resolve(ctx, r.gh, e)
	if err != nil {
		return Proposal{}, fmt.Errorf("propose: %w", err)
	}
	c, err := remote.Fetch(ctx, r.gh, e, target.Commit)
	if err != nil {
		return Proposal{}, fmt.Errorf("propose: %w", err)
	}
	files := map[string][]byte{}
	for file, content := range c.Files {
		//nolint:gosec // git names the content of a file by the SHA-1 of its blob.
		blob := sha1.Sum(slices.Concat([]byte(blobPrefix+strconv.Itoa(len(content))+"\x00"), content))
		if hex.EncodeToString(blob[:]) != r.blobs[file] {
			files[file] = content
		}
	}
	data, _, err := r.gh.File(ctx, r.name, r.head, path.Join(c.Dir, vendored.ManifestFile))
	if err != nil {
		return Proposal{}, fmt.Errorf("propose: read the manifest of %s: %w", c.Dir, err)
	}
	// A manifest that the base branch lacks is empty data, which Decode refuses as well.
	var prev *manifest.Manifest
	entries := vendored.Entries(c.Manifest)
	old, err := manifest.Decode(data)
	if err == nil {
		prev, entries = &old, vendored.Entries(c.Manifest, old)
	}
	var deleted []string
	for file := range r.blobs {
		rel, below := strings.CutPrefix(file, c.Dir+"/")
		first, _, _ := strings.Cut(rel, "/")
		_, owned := slices.BinarySearch(entries, first)
		if _, kept := c.Files[file]; !kept && below && owned {
			deleted = append(deleted, file)
		}
	}
	slices.Sort(deleted)
	def, version := string(e.Definition), c.Manifest.Version
	proposal := Proposal{Definition: e.Definition, Version: version, Commit: target.Commit, Action: Current}
	branch := BranchPrefix + def
	if len(files) == 0 && len(deleted) == 0 {
		return r.closeCurrent(ctx, proposal, branch)
	}
	source := fmt.Sprintf(headFormat, e.Track)
	if target.Tag != "" {
		source = fmt.Sprintf(releaseFormat, target.Tag)
	}
	body, err := r.body(ctx, c, prev, source, files, deleted)
	if err != nil {
		return Proposal{}, err
	}
	if r.changesets {
		file := fmt.Sprintf(changesetFile, strings.ReplaceAll(def, "/", "-"))
		files[file] = fmt.Appendf(nil, changesetFormat, e.Definition.Name(), version)
	}
	number, err := release.Propose(ctx, r.gh, &release.Proposal{
		Files:   files,
		Repo:    r.name,
		Base:    r.base,
		Branch:  branch,
		Head:    r.head,
		Title:   fmt.Sprintf(titleFormat, e.Definition.Name(), version),
		Body:    body,
		Deleted: deleted,
	})
	if err != nil {
		return Proposal{}, fmt.Errorf("propose: the pull request of %s: %w", def, err)
	}
	proposal.Action, proposal.Pull = Proposed, number
	return proposal, nil
}

// closeCurrent closes the open pull request from branch of a copy whose files match the
// definition, with a comment, and returns proposal with the action [Closed] and the number of the
// pull request. It returns proposal unchanged when no pull request from branch is open.
//
// Error modes:
//   - the error of r.gh for a call that fails.
func (r *repository) closeCurrent(ctx context.Context, proposal Proposal, branch string) (Proposal, error) {
	number, open, err := r.gh.PullRequest(ctx, r.name, branch, r.base)
	if err != nil {
		return Proposal{}, fmt.Errorf("propose: the pull request from %s: %w", branch, err)
	}
	if !open {
		return proposal, nil
	}
	comment := fmt.Sprintf(closeFormat, proposal.Definition, proposal.Version, proposal.Commit)
	if err := r.gh.ClosePullRequest(ctx, r.name, number, comment); err != nil {
		return Proposal{}, fmt.Errorf("propose: close the pull request %d: %w", number, err)
	}
	proposal.Action, proposal.Pull = Closed, number
	return proposal, nil
}

// body returns the body of the pull request that syncs the copy at c.Dir to c from source, the
// commit of the definition in words. prev is the manifest of the copy on the base branch, and nil
// for a copy without a manifest that decodes. The pull request writes files and removes deleted,
// which is sorted. body lists the commits of the definition since the commit of prev when prev
// comes from the same definition.
//
// Error modes:
//   - the error of r.gh for a call that fails.
func (r *repository) body(
	ctx context.Context, c *vendored.Copy, prev *manifest.Manifest, source string, files map[string][]byte,
	deleted []string,
) (string, error) {
	def := string(c.Manifest.Repo)
	lines := []string{fmt.Sprintf(introFormat, def, code(c.Dir), source), ""}
	switch {
	case prev == nil:
		lines = append(lines, unknownSource)
	case prev.Local:
		lines = append(lines, fmt.Sprintf(localFormat, prev.Version))
	default:
		lines = append(lines, fmt.Sprintf(fromFormat, prev.Version, prev.Repo, prev.Commit))
	}
	lines = append(lines, fmt.Sprintf(toFormat, c.Manifest.Version, def, c.Manifest.Commit))
	var commits []string
	if prev != nil && prev.Repo == c.Manifest.Repo && prev.Commit != "" {
		// The comparison with a commit that the definition lacks, such as a commit that a force push
		// removed, has no commits.
		comparison, _, err := r.gh.Compare(ctx, def, prev.Commit, c.Manifest.Commit)
		if err != nil {
			return "", fmt.Errorf("propose: compare %s with %s: %w", prev.Commit, c.Manifest.Commit, err)
		}
		for _, k := range comparison.Commits {
			commits = append(commits, fmt.Sprintf(commitFormat, def, k.SHA, code(k.Subject)))
		}
	}
	changed := make([]string, 0, len(files))
	for _, file := range slices.Sorted(maps.Keys(files)) {
		changed = append(changed, item+code(file))
	}
	removed := make([]string, 0, len(deleted))
	for _, file := range deleted {
		removed = append(removed, item+code(file))
	}
	sections := []struct {
		heading string
		items   []string
	}{{commitsHeading, commits}, {changedHeading, changed}, {removedHeading, removed}}
	for _, s := range sections {
		if len(s.items) > 0 {
			lines = append(lines, "", s.heading, "")
			lines = append(lines, s.items...)
		}
	}
	text := strings.Join(lines, "\n") + "\n"
	if len(text) > maxBody {
		text = text[:max(0, strings.LastIndexByte(text[:maxBody], '\n'))] + cutNote
	}
	return text, nil
}

// code returns text as a Markdown code span, in which GitHub links no mention and no issue. The
// span of text without a backtick has one backtick on each side. The span of other text has a run
// of backticks longer than each run in text on each side, with a space between the run and text.
func code(text string) string {
	if !strings.Contains(text, backtick) {
		return backtick + text + backtick
	}
	fence := backtick + backtick
	for strings.Contains(text, fence) {
		fence += backtick
	}
	return fence + " " + text + " " + fence
}
