// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package github

import (
	"archive/tar"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/ergon/service/forge"
	"go.stealthscale.io/spec/manifest"
)

// The headers of a request of the REST API.
const (
	// apiVersion is the version of the REST API of the requests.
	apiVersion = "2022-11-28"

	// jsonType is the media type of a response of JSON, and of the body of a request.
	jsonType = "application/vnd.github+json"

	// rawType is the media type of the raw content of a file.
	rawType = "application/vnd.github.raw"

	// userAgent names the program in each request, which GitHub requires.
	userAgent = "spec"
)

// The values of the responses of the REST API that a client reads.
const (
	// blobType is the type of the entry of a file in a tree of git.
	blobType = "blob"

	// completed is the status of a check run that has ended.
	completed = "completed"

	// closedState is the body of the request that closes a pull request.
	closedState = `{"state":"closed"}`
)

// DefaultMaxArchiveBytes is the largest archive that a client reads by default: 256 MiB, as the
// download and as the tar that it decompresses to.
const DefaultMaxArchiveBytes = 256 << 20

// maxResponse is the largest response of the JSON API and the largest file that a client reads,
// in bytes.
const maxResponse = 10 << 20

// stableTag matches the tag of a stable release: three numbers without leading zeros that dots
// join, after an optional v.
var stableTag = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// passed are the conclusions of a check run that has ended, which count as a success.
var passed = []string{"success", "skipped", "neutral"}

// Config is the access of a [Client] to GitHub. It has the token, the addresses of the APIs and
// the limit of an archive.
type Config struct {
	// Token authorizes every request, such as the token of the variable GITHUB_TOKEN of a
	// workflow.
	Token string

	// API is the address of the REST API, or empty for forge.DefaultAPI.
	API string

	// GraphQL is the address of the GraphQL API, or empty for forge.DefaultGraphQL.
	GraphQL string

	// MaxArchiveBytes is the largest archive that [Client.Archive] reads, in bytes, or 0 for
	// [DefaultMaxArchiveBytes].
	MaxArchiveBytes int64
}

// Client is the client of the GitHub API of the program spec. It embeds the client of ergon's
// forge. The methods of the forge read the branches, the tags and the releases of a repository, and
// they open its pull requests with signed commits. Through the forge, a Client implements the
// Proposer of ergon's release package. The methods that Client declares read the default branch, the archive
// and the tree of a commit, a file at a ref, a comparison of two commits and the checks of a
// commit. They also close a pull request.
//
// # Concurrency
//
// A Client is safe for concurrent use, as its http.Client is.
type Client struct {
	*forge.Client

	// http sends the requests.
	http *http.Client

	// api is the address of the REST API, without a slash at its end.
	api string

	// token authorizes every request.
	token string

	// limit is the largest archive that Archive reads, in bytes.
	limit int64
}

// Comparison is the comparison of two commits of a repository.
type Comparison struct {
	// Status is the relation of the head to the base: ahead, behind, identical or diverged.
	Status string

	// Commits are the commits of the head that the base does not have, the oldest first, up to the
	// 250 that GitHub lists.
	Commits []Commit

	// AheadBy is the number of the commits of the head that the base does not have.
	AheadBy int

	// BehindBy is the number of the commits of the base that the head does not have.
	BehindBy int
}

// Commit is a commit of a [Comparison].
type Commit struct {
	// SHA is the name of the commit.
	SHA string

	// Subject is the first line of the message of the commit.
	Subject string
}

// State is the state of the check runs of a commit.
type State string

// The states of the check runs of a commit.
const (
	// StateNone is the state of a commit without check runs.
	StateNone State = "none"

	// StatePending is the state of a commit with a check run that has not ended, and none that
	// failed.
	StatePending State = "pending"

	// StateSuccess is the state of a commit whose check runs succeeded, were skipped or ended as
	// neutral.
	StateSuccess State = "success"

	// StateFailure is the state of a commit with a check run that ended with another conclusion,
	// such as a failure.
	StateFailure State = "failure"
)

// New returns a client of the GitHub API that c configures, which sends its requests through h.
//
// Error modes:
//   - an error that wraps forge.ErrToken for a c without a token.
func New(h *http.Client, c Config) (*Client, error) {
	f, err := forge.New(h, forge.Config{Token: c.Token, API: c.API, GraphQL: c.GraphQL})
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	return &Client{
		Client: f,
		http:   h,
		api:    strings.TrimRight(cmp.Or(c.API, forge.DefaultAPI), "/"),
		token:  c.Token,
		limit:  cmp.Or(c.MaxArchiveBytes, DefaultMaxArchiveBytes),
	}, nil
}

// DefaultBranch returns the default branch of repo, as owner/name.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails, such as for a repository that does
//     not exist.
func (c *Client) DefaultBranch(ctx context.Context, repo string) (string, error) {
	var r struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.getJSON(ctx, "/repos/"+repo, &r); err != nil {
		return "", err
	}
	return r.DefaultBranch, nil
}

// Archive returns the regular files of the commit sha of repo, which maps the slash-separated path
// of each file, relative to the root of the repository, to its content. It reads the archive that
// GitHub serves for the commit, and leaves out its directories, its links and its headers.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails, and for a download larger than the
//     limit of the archive.
//   - an error that wraps [ErrArchive] for data that is no gzip archive of tar, for an archive that
//     ends early, for an archive whose tar is larger than the limit, and for an entry outside the
//     directory of the archive.
func (c *Client) Archive(ctx context.Context, repo, sha string) (map[string][]byte, error) {
	data, err := c.send(ctx, http.MethodGet, "/repos/"+repo+"/tarball/"+sha, jsonType, nil, c.limit)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}
	limited := &io.LimitedReader{R: zr, N: c.limit + 1}
	tr := tar.NewReader(limited)
	files := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrArchive, err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		// A name without a slash leaves rel empty, which is no valid path.
		_, rel, _ := strings.Cut(h.Name, "/")
		if !manifest.ValidPath(rel) {
			return nil, fmt.Errorf("%w: the entry %q is outside the directory of the archive", ErrArchive, h.Name)
		}
		// tr keeps the error of a read that fails, and the next call of tr.Next returns it.
		content, _ := io.ReadAll(tr)
		files[rel] = content
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("%w: the archive has more than %d bytes", ErrArchive, c.limit)
	}
	return files, nil
}

// File returns the content of the file at the slash-separated path of repo at ref, a branch, a tag
// or a commit, and reports whether ref has the file.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails, other than for a file that ref
//     lacks, and for a file larger than 10 MiB.
func (c *Client) File(ctx context.Context, repo, ref, path string) ([]byte, bool, error) {
	segments := strings.Split(path, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	content, err := c.send(
		ctx,
		http.MethodGet,
		"/repos/"+repo+"/contents/"+strings.Join(segments, "/")+"?ref="+url.QueryEscape(ref),
		rawType,
		nil,
		maxResponse,
		http.StatusNotFound,
	)
	if err != nil {
		return nil, false, err
	}
	return content, content != nil, nil
}

// Blobs returns the blob of each file of the commit sha of repo, by the slash-separated path of the
// file relative to the root of the repository. The blob is the name that git gives to the content
// of a file.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails, and for a tree that GitHub
//     truncates because it is too large.
func (c *Client) Blobs(ctx context.Context, repo, sha string) (map[string]string, error) {
	var r struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := c.getJSON(ctx, "/repos/"+repo+"/git/trees/"+sha+"?recursive=1", &r); err != nil {
		return nil, err
	}
	if r.Truncated {
		return nil, fmt.Errorf("%w: GitHub truncates the tree of %s at %s", ErrGitHub, repo, sha)
	}
	blobs := make(map[string]string, len(r.Tree))
	for _, e := range r.Tree {
		if e.Type == blobType {
			blobs[e.Path] = e.SHA
		}
	}
	return blobs, nil
}

// Compare returns the comparison of the commit head of repo with the commit base, and reports
// whether repo has both commits. head and base are names of commits, branches or tags.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails, other than for a commit that repo
//     lacks, and for a response that does not decode.
func (c *Client) Compare(ctx context.Context, repo, base, head string) (Comparison, bool, error) {
	var r struct {
		Status  string `json:"status"`
		Commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		} `json:"commits"`
		AheadBy  int `json:"ahead_by"`
		BehindBy int `json:"behind_by"`
	}
	path := "/repos/" + repo + "/compare/" + base + "..." + head
	data, err := c.send(ctx, http.MethodGet, path, jsonType, nil, maxResponse, http.StatusNotFound)
	if err != nil {
		return Comparison{}, false, err
	}
	if data == nil {
		return Comparison{}, false, nil
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Comparison{}, false, fmt.Errorf("%w: decode %s: %w", ErrGitHub, path, err)
	}
	out := Comparison{Status: r.Status, AheadBy: r.AheadBy, BehindBy: r.BehindBy}
	for _, commit := range r.Commits {
		subject, _, _ := strings.Cut(commit.Commit.Message, "\n")
		out.Commits = append(out.Commits, Commit{SHA: commit.SHA, Subject: subject})
	}
	return out, true, nil
}

// ClosePullRequest writes comment on the pull request number of repo, and then closes it.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a comment or a close that fails, after which the pull
//     request remains open.
func (c *Client) ClosePullRequest(ctx context.Context, repo string, number int, comment string) error {
	n := strconv.Itoa(number)
	// A map of strings encodes without an error.
	body, _ := json.Marshal(map[string]string{"body": comment})
	if _, err := c.send(
		ctx,
		http.MethodPost,
		"/repos/"+repo+"/issues/"+n+"/comments",
		jsonType,
		body,
		maxResponse,
	); err != nil {
		return err
	}
	_, err := c.send(ctx, http.MethodPatch, "/repos/"+repo+"/pulls/"+n, jsonType, []byte(closedState), maxResponse)
	return err
}

// Checks returns the state of the check runs of ref of repo, a branch, a tag or a commit, from
// the first 100 runs that GitHub lists: [StateFailure] for a run that failed, then
// [StatePending] for a run that has not ended, then [StateSuccess], and [StateNone] without runs.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails.
func (c *Client) Checks(ctx context.Context, repo, ref string) (State, error) {
	var r struct {
		Runs []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := c.getJSON(ctx, "/repos/"+repo+"/commits/"+ref+"/check-runs?per_page=100", &r); err != nil {
		return "", err
	}
	state := StateNone
	for _, run := range r.Runs {
		switch {
		case run.Status != completed:
			state = StatePending
		case !slices.Contains(passed, run.Conclusion):
			return StateFailure, nil
		case state == StateNone:
			state = StateSuccess
		}
	}
	return state, nil
}

// LatestRelease returns the tag and the commit of the highest stable release of repo. A release is
// stable when its tag is three numbers after an optional v, such as v1.10.0, and versions compare
// by their numbers, not by the dates of their releases. Of two releases of one version, such as
// v1.0.0 and 1.0.0, the release that GitHub lists first counts. Drafts do not count.
//
// Error modes:
//   - an error that wraps [ErrGitHub] for a request that fails.
//   - an error that wraps [ErrNoRelease] for a repository without a stable release, and for a
//     release whose tag the repository lacks.
func (c *Client) LatestRelease(ctx context.Context, repo string) (string, string, error) {
	releases, err := c.Releases(ctx, repo)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrGitHub, err)
	}
	var tag string
	var best []string
	for _, r := range releases {
		numbers := stableTag.FindStringSubmatch(r.Tag)
		if numbers == nil {
			continue
		}
		higher := slices.CompareFunc(numbers[1:], best, func(a, b string) int {
			return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
		}) > 0
		if higher {
			tag, best = r.Tag, numbers[1:]
		}
	}
	if tag == "" {
		return "", "", fmt.Errorf("%w: %s has no stable release", ErrNoRelease, repo)
	}
	sha, found, err := c.Tag(ctx, repo, tag)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrGitHub, err)
	}
	if !found {
		return "", "", fmt.Errorf("%w: %s has no tag %s", ErrNoRelease, repo, tag)
	}
	return tag, sha, nil
}

// getJSON sends a GET request of path to the REST API, and decodes the JSON of the response into
// out. It returns the errors of send, and an error that wraps [ErrGitHub] for a response that does
// not decode.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	data, err := c.send(ctx, http.MethodGet, path, jsonType, nil, maxResponse)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: decode %s: %w", ErrGitHub, path, err)
	}
	return nil
}

// send sends a request of method to path of the REST API, and returns the body of the response.
// The request has body as JSON when body is not nil, and accept as the media type of the response.
// send returns a nil body for a status of allowed. It returns an error that wraps [ErrGitHub] for a
// request that fails, for a response of more than maxBytes bytes, and for a status outside 2xx and
// allowed. The error of such a status contains the message of GitHub.
func (c *Client) send(
	ctx context.Context, method, path, accept string, body []byte, maxBytes int64, allowed ...int,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.api+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrGitHub, method, path, err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", jsonType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrGitHub, method, path, err)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err = errors.Join(err, resp.Body.Close()); err != nil {
		return nil, fmt.Errorf("%w: read %s %s: %w", ErrGitHub, method, path, err)
	}
	switch {
	case int64(len(data)) > maxBytes:
		return nil, fmt.Errorf("%w: %s %s: the response has more than %d bytes", ErrGitHub, method, path, maxBytes)
	case slices.Contains(allowed, resp.StatusCode):
		return nil, nil
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		return data, nil
	}
	var refusal struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &refusal) != nil {
		refusal.Message = string(data)
	}
	return nil, fmt.Errorf("%w: %s %s: %s: %s", ErrGitHub, method, path, resp.Status, refusal.Message)
}
