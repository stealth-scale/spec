// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	"go.stealthscale.io/cli/clitest"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/internal/command"
	"go.stealthscale.io/spec/vendored"
)

// The repositories and the commits of the cases.
const (
	definitionRepo = "dokimasia/assert-spec"
	otherRepo      = "dokimasia/mutate-spec"
	implRepo       = "dokimasia/assert-go"
	firstCommit    = "1111111111111111111111111111111111111111"
	secondCommit   = "2222222222222222222222222222222222222222"
	sideCommit     = "3333333333333333333333333333333333333333"
	implHead       = "4444444444444444444444444444444444444444"
	missingCommit  = "5555555555555555555555555555555555555555"
)

// token is the token of GitHub of a run of the program in the cases.
const token = "token-of-the-cases"

// definitionConfig configures the definition of the cases, which registers assert-go.
const definitionConfig = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  cases: spec/cases.json
  files: [VERSION, spec/cases.json, "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
  implementations:
    - {repo: dokimasia/assert-go, languages: [go]}
`

// implConfig configures the implementation of the cases: the go files of the definition at
// conformance/spec.
const implConfig = `implements:
  - definition: dokimasia/assert-spec
    path: conformance/spec
    languages: [go]
`

// first is the first revision of the definition of the cases.
var first = map[string]string{
	spec.ConfigFile:      definitionConfig,
	"VERSION":            "1.0.0\n",
	"spec/cases.json":    `{"cases": [{"file": "corpus/a.json", "id": "a"}]}` + "\n",
	"corpus/a.json":      `{"case": "a"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// second is the next revision of the definition, with a changed case.
var second = merge(
	first,
	map[string]string{"VERSION": "1.1.0\n", "corpus/a.json": `{"case": "a", "version": 2}` + "\n"},
)

// hub is a GitHub of the cases in memory, which serves the requests of the REST API that the
// program sends. net/http serves each request on a goroutine of its own, so mu guards the state.
type hub struct {
	// repos maps owner/name to each repository.
	repos map[string]*repository

	// failures maps the name of a route of ServeHTTP, such as tarball, to the status of the response
	// of the route in place of its data.
	failures map[string]int

	// mu guards repos and failures.
	mu sync.Mutex
}

// repository is a repository of a hub on its default branch main.
type repository struct {
	// commits maps each commit to its files.
	commits map[string]map[string]string

	// branches maps each branch to its commit.
	branches map[string]string

	// releases maps the tag of each stable release to its commit.
	releases map[string]string

	// pulls maps the branch of each open pull request to its number.
	pulls map[string]int

	// checks maps a ref to the conclusion of its one check run.
	checks map[string]string

	// history are the commits of main, the oldest first.
	history []string
}

// newHub returns a hub with the definition at secondCommit on main after firstCommit, the commit
// sideCommit of no branch, and the implementation at implHead with its copy of first.
func newHub(t *testing.T) *hub {
	t.Helper()
	return &hub{
		repos: map[string]*repository{
			definitionRepo: {
				commits:  map[string]map[string]string{firstCommit: first, secondCommit: second, sideCommit: second},
				branches: map[string]string{"main": secondCommit},
				releases: map[string]string{},
				history:  []string{firstCommit, secondCommit},
			},
			implRepo: {
				commits:  map[string]map[string]string{implHead: implementationAt(t, first, firstCommit)},
				branches: map[string]string{"main": implHead},
				pulls:    map[string]int{},
				checks:   map[string]string{implHead: "success"},
				history:  []string{implHead},
			},
		},
		failures: map[string]int{},
	}
}

// ServeHTTP responds to the requests of the REST API that the program sends.
func (h *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	mux := http.NewServeMux()
	routes := []struct {
		handle  func(w http.ResponseWriter, r *http.Request, repo *repository)
		name    string
		pattern string
	}{
		{h.repository, "repository", "GET /repos/{owner}/{name}"},
		{h.branch, "branch", "GET /repos/{owner}/{name}/git/ref/heads/{branch...}"},
		{h.tag, "tag", "GET /repos/{owner}/{name}/git/ref/tags/{tag...}"},
		{h.releaseList, "releases", "GET /repos/{owner}/{name}/releases"},
		{h.contents, "contents", "GET /repos/{owner}/{name}/contents/{path...}"},
		{h.tree, "tree", "GET /repos/{owner}/{name}/git/trees/{sha}"},
		{h.tarball, "tarball", "GET /repos/{owner}/{name}/tarball/{sha}"},
		{h.compare, "compare", "GET /repos/{owner}/{name}/compare/{basehead}"},
		{h.pullList, "pulls", "GET /repos/{owner}/{name}/pulls"},
		{h.checkRuns, "checks", "GET /repos/{owner}/{name}/commits/{ref...}"},
		{h.accept, "comment", "POST /repos/{owner}/{name}/issues/{number}/comments"},
		{h.accept, "close", "PATCH /repos/{owner}/{name}/pulls/{number}"},
	}
	for _, route := range routes {
		mux.HandleFunc(route.pattern, func(w http.ResponseWriter, r *http.Request) {
			repo, ok := h.repos[r.PathValue("owner")+"/"+r.PathValue("name")]
			if !ok {
				http.Error(w, `{"message": "Not Found"}`, http.StatusNotFound)
				return
			}
			if status := h.failures[route.name]; status != 0 {
				http.Error(w, `{"message": "injected failure"}`, status)
				return
			}
			route.handle(w, r, repo)
		})
	}
	mux.ServeHTTP(w, r)
}

// repository responds with the default branch.
func (*hub) repository(w http.ResponseWriter, _ *http.Request, _ *repository) {
	reply(w, map[string]string{"default_branch": "main"})
}

// branch responds with the commit of a branch.
func (*hub) branch(w http.ResponseWriter, r *http.Request, repo *repository) {
	sha, ok := repo.branches[r.PathValue("branch")]
	if !ok {
		http.Error(w, `{"message": "Not Found"}`, http.StatusNotFound)
		return
	}
	reply(w, map[string]any{"object": map[string]string{"type": "commit", "sha": sha}})
}

// tag responds with the commit of the tag of a release.
func (*hub) tag(w http.ResponseWriter, r *http.Request, repo *repository) {
	sha, ok := repo.releases[r.PathValue("tag")]
	if !ok {
		http.Error(w, `{"message": "Not Found"}`, http.StatusNotFound)
		return
	}
	reply(w, map[string]any{"object": map[string]string{"type": "commit", "sha": sha}})
}

// releaseList responds with every release on the first page.
func (*hub) releaseList(w http.ResponseWriter, _ *http.Request, repo *repository) {
	releases := make([]map[string]string, 0, len(repo.releases))
	for _, tag := range slices.Sorted(maps.Keys(repo.releases)) {
		releases = append(releases, map[string]string{"tag_name": tag})
	}
	reply(w, releases)
}

// contents responds with the raw content of a file at a branch or a commit.
func (*hub) contents(w http.ResponseWriter, r *http.Request, repo *repository) {
	ref := r.URL.Query().Get("ref")
	if sha, ok := repo.branches[ref]; ok {
		ref = sha
	}
	content, ok := repo.commits[ref][r.PathValue("path")]
	if !ok {
		http.Error(w, `{"message": "Not Found"}`, http.StatusNotFound)
		return
	}
	_, _ = w.Write([]byte(content))
}

// tree responds with the blob of each file of a commit, as git names it.
func (*hub) tree(w http.ResponseWriter, r *http.Request, repo *repository) {
	files := repo.commits[r.PathValue("sha")]
	entries := make([]map[string]string, 0, len(files))
	for p, content := range files {
		blob := sha1.Sum([]byte("blob " + strconv.Itoa(len(content)) + "\x00" + content))
		entries = append(entries, map[string]string{"path": p, "type": "blob", "sha": hex.EncodeToString(blob[:])})
	}
	reply(w, map[string]any{"truncated": false, "tree": entries})
}

// tarball responds with the archive of a commit. The files of the archive are in one directory.
func (*hub) tarball(w http.ResponseWriter, r *http.Request, repo *repository) {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	tw := tar.NewWriter(zw)
	files := repo.commits[r.PathValue("sha")]
	for _, p := range slices.Sorted(maps.Keys(files)) {
		_ = tw.WriteHeader(&tar.Header{
			Name:     "repo-" + r.PathValue("sha")[:7] + "/" + p,
			Mode:     0o644,
			Size:     int64(len(files[p])),
			Typeflag: tar.TypeReg,
		})
		_, _ = tw.Write([]byte(files[p]))
	}
	_ = tw.Close()
	_ = zw.Close()
	_, _ = w.Write(b.Bytes())
}

// compare responds with the comparison of two commits along main. Two commits of main are
// identical, ahead or behind, and a base off main is diverged. A base that the repository lacks
// gets the status Not Found.
func (*hub) compare(w http.ResponseWriter, r *http.Request, repo *repository) {
	base, head, _ := strings.Cut(r.PathValue("basehead"), "...")
	if _, ok := repo.commits[base]; !ok {
		http.Error(w, `{"message": "Not Found"}`, http.StatusNotFound)
		return
	}
	from, to := slices.Index(repo.history, base), slices.Index(repo.history, head)
	status, commits := "diverged", []map[string]any{}
	switch {
	case from < 0 || to < 0:
	case from == to:
		status = "identical"
	case from < to:
		status = "ahead"
		for _, sha := range repo.history[from+1 : to+1] {
			commits = append(
				commits,
				map[string]any{"sha": sha, "commit": map[string]string{"message": "feat: " + sha[:7]}},
			)
		}
	default:
		status = "behind"
	}
	reply(w, map[string]any{"status": status, "ahead_by": len(commits), "commits": commits})
}

// pullList responds with the open pull request from the branch of the query head, as owner:branch.
func (*hub) pullList(w http.ResponseWriter, r *http.Request, repo *repository) {
	_, head, _ := strings.Cut(r.URL.Query().Get("head"), ":")
	pulls := []map[string]int{}
	if n, ok := repo.pulls[head]; ok {
		pulls = append(pulls, map[string]int{"number": n})
	}
	reply(w, pulls)
}

// checkRuns responds with the one check run of a ref, or none. GitHub accepts a ref of a branch
// with slashes, so the route takes the rest of the path before /check-runs.
func (*hub) checkRuns(w http.ResponseWriter, r *http.Request, repo *repository) {
	runs := []map[string]string{}
	if conclusion, ok := repo.checks[strings.TrimSuffix(r.PathValue("ref"), "/check-runs")]; ok {
		runs = append(runs, map[string]string{"status": "completed", "conclusion": conclusion})
	}
	reply(w, map[string]any{"check_runs": runs})
}

// accept responds to a request that writes with an empty object.
func (*hub) accept(w http.ResponseWriter, _ *http.Request, _ *repository) {
	reply(w, map[string]string{})
}

// reply writes v as the JSON of a response.
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// run runs the program with args, as start runs it.
func run(t *testing.T, h *hub, args ...string) clitest.Result {
	t.Helper()
	return start(t, h, &clitest.Input{Args: args})
}

// start runs the program with in, the token of the cases and the API of h, and returns the result.
// A nil h gives a run without an address of an API.
func start(t *testing.T, h *hub, in *clitest.Input) clitest.Result {
	t.Helper()
	in.Vars = map[string]string{"GITHUB_TOKEN": token}
	program := &command.Program{}
	if h != nil {
		server := httptest.NewServer(h)
		t.Cleanup(server.Close)
		in.Vars["GITHUB_API_URL"] = server.URL
		program.Transport = server.Client().Transport
	}
	return clitest.Main(t.Context(), program.Main, in)
}

// copyTable returns the table of the copy of the implementation of the cases at version and commit,
// which the commands sync and check write.
func copyTable(version, commit string) string {
	return "COPY              DEFINITION             VERSION  COMMIT\n" +
		"conformance/spec  dokimasia/assert-spec  " + version + "    " + commit + "\n"
}

// merge returns a copy of base with the files of changes.
func merge(base, changes map[string]string) map[string]string {
	files := maps.Clone(base)
	maps.Copy(files, changes)
	return files
}

// load returns the definition of the text files contents.
func load(tb assert.TB, contents map[string]string) *definition.Tree {
	tb.Helper()
	files := make(map[string][]byte, len(contents))
	for p, text := range contents {
		files[p] = []byte(text)
	}
	tree, err := definition.Load(files)
	assert.NoError(tb, err, "the definition of the case loads")
	return tree
}

// copyOf returns the copy of contents for the entry of implConfig at the commit at.
func copyOf(tb assert.TB, contents map[string]string, at string) *vendored.Copy {
	tb.Helper()
	entries, err := implementation.Parse([]byte(implConfig))
	assert.NoError(tb, err, "the configuration of the case parses")
	assert.Length(tb, entries, 1, "the configuration of the case names one definition")
	c, err := vendored.Build(load(tb, contents), &entries[0], at)
	assert.NoError(tb, err, "the copy of the case builds")
	return c
}

// implementationAt returns the files of the implementation of the cases with the copy of contents
// at the commit at.
func implementationAt(tb assert.TB, contents map[string]string, at string) map[string]string {
	tb.Helper()
	impl := map[string]string{spec.ConfigFile: implConfig}
	for p, content := range copyOf(tb, contents, at).Files {
		impl[p] = string(content)
	}
	return impl
}

// workspace returns a directory of the test with the text files contents.
func workspace(t *testing.T, contents map[string]string) string {
	t.Helper()
	tree := files.Tree{}
	for p, text := range contents {
		tree[p] = files.Text(text)
	}
	return files.Workspace(t, tree)
}
