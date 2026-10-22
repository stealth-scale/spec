// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package github_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/ergon/service/forge"
	"go.stealthscale.io/spec/github"
)

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns forge.ErrToken without a token", func(t *testing.T) {
			t.Parallel()
			_, err := github.New(&http.Client{Timeout: timeout}, github.Config{})
			assert.ErrorIs(t, err, forge.ErrToken, "New needs a token or Anonymous")
		})

		t.Run("returns a client that reads without a token for Anonymous", func(t *testing.T) {
			t.Parallel()
			server := serveAuthorization(t, "")
			c, err := github.New(
				&http.Client{Timeout: timeout, Transport: server.Client().Transport},
				github.Config{API: server.URL, Anonymous: true},
			)
			assert.NoError(t, err, "New returns a client without a token")
			branch, err := c.DefaultBranch(t.Context(), repo)
			assert.NoError(t, err, "DefaultBranch reads without a token")
			assert.Equal(t, branch, "trunk", "DefaultBranch returns the branch of the repository")
			sha, found, err := c.Branch(t.Context(), repo, "main")
			assert.NoError(t, err, "Branch of the forge reads without a token")
			assert.True(t, found, "the repository has the branch")
			assert.Equal(t, sha, commit, "Branch returns the commit of the branch")
		})

		t.Run("returns ErrConfig for Anonymous without a transport", func(t *testing.T) {
			t.Parallel()
			_, err := github.New(&http.Client{Timeout: timeout}, github.Config{Anonymous: true})
			assert.ErrorIs(t, err, github.ErrConfig, "a client without a token needs a transport")
		})

		t.Run("returns a client that sends its token for Anonymous", func(t *testing.T) {
			t.Parallel()
			server := serveAuthorization(t, "Bearer "+token)
			c, err := github.New(
				&http.Client{Timeout: timeout, Transport: server.Client().Transport},
				github.Config{Token: token, API: server.URL, Anonymous: true},
			)
			assert.NoError(t, err, "New returns a client with a token")
			branch, err := c.DefaultBranch(t.Context(), repo)
			assert.NoError(t, err, "DefaultBranch sends the token")
			assert.Equal(t, branch, "trunk", "DefaultBranch returns the branch of the repository")
			_, found, err := c.Branch(t.Context(), repo, "main")
			assert.NoError(t, err, "Branch of the forge sends the token")
			assert.True(t, found, "the repository has the branch")
		})
	})

	t.Run("DefaultBranch", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the default branch of the repository", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec": reply(http.StatusOK, `{"default_branch": "trunk"}`),
			})
			branch, err := c.DefaultBranch(t.Context(), repo)
			assert.NoError(t, err, "DefaultBranch reads the repository")
			assert.Equal(t, branch, "trunk", "DefaultBranch returns the branch of the repository")
		})

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec": reply(http.StatusNotFound, `{"message": "Not Found"}`),
			})
			_, err := c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch refuses a status of an error")
			assert.HasSuffix(t, err.Error(), ": 404 Not Found: Not Found", "the error ends with the message of GitHub")
		})

		t.Run("returns ErrGitHub for a server that refuses the connection", func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.NotFoundHandler())
			server.Close()
			c, err := github.New(&http.Client{Timeout: timeout}, github.Config{Token: token, API: server.URL})
			assert.NoError(t, err, "New returns a client of the closed server")
			_, err = c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch returns the error of the transport")
			sent := assert.ErrorAs[*url.Error](t, err, "the error wraps the error of the request")
			assert.Equal(t, sent.Op, "Get", "the request fails in the transport")
		})

		t.Run("returns ErrGitHub for an address that is no URL", func(t *testing.T) {
			t.Parallel()
			c, err := github.New(&http.Client{Timeout: timeout}, github.Config{Token: token, API: "http://[::1"})
			assert.NoError(t, err, "New returns a client of any address")
			_, err = c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch refuses an address that is no URL")
			parse := assert.ErrorAs[*url.Error](t, err, "the error wraps the error of the parse")
			assert.Equal(t, parse.Op, "parse", "the address fails to parse")
		})

		t.Run("returns ErrGitHub for a response that ends early", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec": func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, `{"default_branch": "trunk"}`)
				},
			})
			_, err := c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch returns the error of the read")
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "the error wraps the error of the read")
			assert.Contains(t, err.Error(), "read GET /repos/dokimasia/assert-spec", "the error names the read")
		})

		t.Run("returns ErrGitHub with the text of a refusal that is no JSON", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec": reply(http.StatusBadGateway, "the proxy failed"),
			})
			_, err := c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch refuses a status of an error")
			assert.Contains(t, err.Error(), "the proxy failed", "the error states the text of the refusal")
		})

		t.Run("returns ErrGitHub for a body that is no JSON", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec": reply(http.StatusOK, `repository`),
			})
			_, err := c.DefaultBranch(t.Context(), repo)
			assert.ErrorIs(t, err, github.ErrGitHub, "DefaultBranch refuses a body that does not decode")
		})
	})

	t.Run("Archive", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the regular files without the directory of the archive", func(t *testing.T) {
			t.Parallel()
			data := tarball(t,
				entry{name: "pax_global_header", kind: tar.TypeXGlobalHeader},
				entry{name: "dokimasia-assert-spec-27b9537/", kind: tar.TypeDir},
				entry{name: "dokimasia-assert-spec-27b9537/VERSION", body: "9.0.0\n", kind: tar.TypeReg},
				entry{name: "dokimasia-assert-spec-27b9537/corpus/a.json", body: "{}\n", kind: tar.TypeReg},
				entry{name: "dokimasia-assert-spec-27b9537/link", link: "VERSION", kind: tar.TypeSymlink},
			)
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/tarball/" + commit: reply(http.StatusOK, string(data)),
			})
			files, err := c.Archive(t.Context(), repo, commit)
			assert.NoError(t, err, "Archive reads the archive of the commit")
			assert.Equal(t, files, map[string][]byte{
				"VERSION":       []byte("9.0.0\n"),
				"corpus/a.json": []byte("{}\n"),
			}, "Archive returns the content of each regular file at its path in the repository")
		})

		t.Run("returns the files of an archive whose tar has the size of the limit", func(t *testing.T) {
			t.Parallel()
			data := tarball(t, entry{name: "x/VERSION", body: "9.0.0\n", kind: tar.TypeReg})
			zr, err := gzip.NewReader(bytes.NewReader(data))
			assert.NoError(t, err, "the archive of the case decompresses")
			plain, err := io.ReadAll(zr)
			assert.NoError(t, err, "the tar of the case reads")
			c := serve(t, int64(len(plain)), map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/tarball/" + commit: reply(http.StatusOK, string(data)),
			})
			files, err := c.Archive(t.Context(), repo, commit)
			assert.NoError(t, err, "Archive reads a tar of the size of the limit")
			assert.Equal(t, files, map[string][]byte{"VERSION": []byte("9.0.0\n")}, "Archive returns the file")
		})

		t.Run("follows the redirect to the server of the archives", func(t *testing.T) {
			t.Parallel()
			data := tarball(t, entry{name: "x/VERSION", body: "9.0.0\n", kind: tar.TypeReg})
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/tarball/" + commit: func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, "/codeload/"+commit, http.StatusFound)
				},
				"GET /codeload/" + commit: reply(http.StatusOK, string(data)),
			})
			files, err := c.Archive(t.Context(), repo, commit)
			assert.NoError(t, err, "Archive follows the redirect")
			assert.Equal(
				t,
				files,
				map[string][]byte{"VERSION": []byte("9.0.0\n")},
				"Archive returns the redirected archive",
			)
		})

		tests := []struct {
			name  string
			limit int64
			give  func(t *testing.T) []byte
		}{
			{
				name: "returns ErrArchive for data that is no gzip",
				give: func(*testing.T) []byte { return []byte("archive") },
			},
			{
				name: "returns ErrArchive for an archive that ends in the content of a file",
				give: func(t *testing.T) []byte {
					t.Helper()
					data := tarball(t, entry{name: "x/noise", body: string(noise()), kind: tar.TypeReg})
					return data[:len(data)-1024]
				},
			},
			{
				name: "returns ErrArchive for gzip data that is no tar",
				give: func(*testing.T) []byte {
					var b bytes.Buffer
					zw := gzip.NewWriter(&b)
					_, _ = zw.Write([]byte(strings.Repeat("no tar ", 200)))
					_ = zw.Close()
					return b.Bytes()
				},
			},
			{
				name: "returns ErrArchive for a path that leaves the archive",
				give: func(t *testing.T) []byte {
					t.Helper()
					return tarball(t, entry{name: "x/../../evil", body: "x", kind: tar.TypeReg})
				},
			},
			{
				name: "returns ErrArchive for a file outside the directory of the archive",
				give: func(t *testing.T) []byte {
					t.Helper()
					return tarball(t, entry{name: "VERSION", body: "9.0.0\n", kind: tar.TypeReg})
				},
			},
			{
				name:  "returns ErrArchive for an archive above the limit that ends at a block",
				limit: 1023,
				give: func(t *testing.T) []byte {
					t.Helper()
					return tarball(t, entry{name: "x/VERSION", body: "9.0.0\n", kind: tar.TypeReg})
				},
			},
			{
				name:  "returns ErrArchive for an archive above the limit that ends in a file",
				limit: 1024,
				give: func(t *testing.T) []byte {
					t.Helper()
					return tarball(t, entry{name: "x/big", body: string(make([]byte, 4096)), kind: tar.TypeReg})
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				c := serve(t, tt.limit, map[string]http.HandlerFunc{
					"GET /repos/dokimasia/assert-spec/tarball/" + commit: reply(http.StatusOK, string(tt.give(t))),
				})
				_, err := c.Archive(t.Context(), repo, commit)
				assert.ErrorIs(t, err, github.ErrArchive, "Archive refuses the archive")
			})
		}

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/tarball/" + commit: reply(
					http.StatusNotFound,
					`{"message": "Not Found"}`,
				),
			})
			_, err := c.Archive(t.Context(), repo, commit)
			assert.ErrorIs(t, err, github.ErrGitHub, "Archive refuses a status of an error")
		})

		t.Run("returns ErrGitHub for a download above the limit", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 16, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/tarball/" + commit: reply(http.StatusOK, string(noise()[:64])),
			})
			_, err := c.Archive(t.Context(), repo, commit)
			assert.ErrorIs(t, err, github.ErrGitHub, "Archive refuses a download above the limit")
		})
	})

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the content of the file at the ref", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/conformance/spec/manifest.json": func(
					w http.ResponseWriter, r *http.Request,
				) {
					if r.URL.Query().Get("ref") != "main" || r.Header.Get("Accept") != "application/vnd.github.raw" {
						http.Error(w, "the request names no ref or asks for JSON", http.StatusBadRequest)
						return
					}
					_, _ = io.WriteString(w, "{}\n")
				},
			})
			content, found, err := c.File(t.Context(), "dokimasia/assert-go", "main", "conformance/spec/manifest.json")
			assert.NoError(t, err, "File reads the file")
			assert.True(t, found, "File finds the file")
			assert.Equal(t, string(content), "{}\n", "File returns the raw content")
		})

		t.Run("returns the content of a path with a question mark", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/{path...}": func(w http.ResponseWriter, r *http.Request) {
					if r.PathValue("path") != "corpus/a?b.json" || r.URL.Query().Get("ref") != "main" {
						http.Error(w, "the request names another path or no ref", http.StatusBadRequest)
						return
					}
					_, _ = io.WriteString(w, "{}\n")
				},
			})
			content, found, err := c.File(t.Context(), "dokimasia/assert-go", "main", "corpus/a?b.json")
			assert.NoError(t, err, "File reads the file")
			assert.True(t, found, "File finds the file")
			assert.Equal(t, string(content), "{}\n", "File returns the raw content")
		})

		t.Run("returns the content of a file of 10 MiB", func(t *testing.T) {
			t.Parallel()
			want := bytes.Repeat([]byte("a"), 10<<20)
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/big.json": reply(http.StatusOK, string(want)),
			})
			content, _, err := c.File(t.Context(), "dokimasia/assert-go", "main", "big.json")
			assert.NoError(t, err, "File reads a file of the largest size")
			assert.Equal(t, content, want, "File returns every byte of the file")
		})

		t.Run("returns ErrGitHub for a file above 10 MiB", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/big.json": reply(
					http.StatusOK,
					strings.Repeat("a", 10<<20+1),
				),
			})
			_, _, err := c.File(t.Context(), "dokimasia/assert-go", "main", "big.json")
			assert.ErrorIs(t, err, github.ErrGitHub, "File refuses a file above the largest size")
		})

		t.Run("returns ErrGitHub for an informational status", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/.spec.yaml": reply(http.StatusSwitchingProtocols, ""),
			})
			_, _, err := c.File(t.Context(), "dokimasia/assert-go", "main", ".spec.yaml")
			assert.ErrorIs(t, err, github.ErrGitHub, "File refuses a status below 2xx")
		})

		t.Run("returns ErrGitHub for a status of 300", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/.spec.yaml": reply(http.StatusMultipleChoices, `{}`),
			})
			_, _, err := c.File(t.Context(), "dokimasia/assert-go", "main", ".spec.yaml")
			assert.ErrorIs(t, err, github.ErrGitHub, "File refuses a status above 2xx")
		})

		t.Run("reports false for a file that the ref lacks", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/.spec.yaml": reply(
					http.StatusNotFound,
					`{"message": "Not Found"}`,
				),
			})
			_, found, err := c.File(t.Context(), "dokimasia/assert-go", "main", ".spec.yaml")
			assert.NoError(t, err, "File reads a missing file without an error")
			assert.False(t, found, "File finds no file")
		})

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/contents/.spec.yaml": reply(http.StatusInternalServerError, `{}`),
			})
			_, _, err := c.File(t.Context(), "dokimasia/assert-go", "main", ".spec.yaml")
			assert.ErrorIs(t, err, github.ErrGitHub, "File refuses a status of an error")
		})
	})

	t.Run("Blobs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the blob of each file of the commit", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/git/trees/" + commit: func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("recursive") != "1" {
						http.Error(w, "the request reads one level", http.StatusBadRequest)
						return
					}
					_, _ = io.WriteString(w, `{"truncated": false, "tree": [
  {"path": "conformance", "type": "tree", "sha": "aaaa"},
  {"path": "conformance/spec/VERSION", "type": "blob", "sha": "bbbb"},
  {"path": "go.mod", "type": "blob", "sha": "cccc"}
]}`)
				},
			})
			blobs, err := c.Blobs(t.Context(), "dokimasia/assert-go", commit)
			assert.NoError(t, err, "Blobs reads the tree")
			assert.Equal(t, blobs, map[string]string{"conformance/spec/VERSION": "bbbb", "go.mod": "cccc"}, "the blobs")
		})

		t.Run("returns ErrGitHub for a tree that GitHub truncated", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/git/trees/" + commit: reply(
					http.StatusOK,
					`{"truncated": true, "tree": []}`,
				),
			})
			_, err := c.Blobs(t.Context(), "dokimasia/assert-go", commit)
			assert.ErrorIs(t, err, github.ErrGitHub, "Blobs refuses a tree without every file")
		})

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/git/trees/" + commit: reply(http.StatusNotFound, `{}`),
			})
			_, err := c.Blobs(t.Context(), "dokimasia/assert-go", commit)
			assert.ErrorIs(t, err, github.ErrGitHub, "Blobs refuses a status of an error")
		})
	})

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the status and the commits between two commits", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/compare/" + commit + "...main": reply(http.StatusOK, `{
  "status": "behind", "ahead_by": 0, "behind_by": 2,
  "commits": [
    {"sha": "1111", "commit": {"message": "feat: add the case\n\nThe body."}},
    {"sha": "2222", "commit": {"message": "fix: correct the case"}}
  ]
}`),
			})
			comparison, found, err := c.Compare(t.Context(), repo, commit, "main")
			assert.NoError(t, err, "Compare reads the comparison")
			assert.True(t, found, "Compare finds both commits")
			assert.Equal(t, comparison, github.Comparison{
				Status:   "behind",
				BehindBy: 2,
				Commits: []github.Commit{
					{SHA: "1111", Subject: "feat: add the case"},
					{SHA: "2222", Subject: "fix: correct the case"},
				},
			}, "Compare returns the counts and the subject of each commit")
		})

		t.Run("reports false for a commit that the repository lacks", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/compare/" + commit + "...main": reply(
					http.StatusNotFound,
					`{"message": "Not Found"}`,
				),
			})
			_, found, err := c.Compare(t.Context(), repo, commit, "main")
			assert.NoError(t, err, "Compare reads a missing commit without an error")
			assert.False(t, found, "Compare finds no comparison")
		})

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/compare/" + commit + "...main": reply(
					http.StatusInternalServerError,
					`{}`,
				),
			})
			_, _, err := c.Compare(t.Context(), repo, commit, "main")
			assert.ErrorIs(t, err, github.ErrGitHub, "Compare refuses a status of an error")
		})

		t.Run("returns ErrGitHub for a body that is no JSON", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-spec/compare/" + commit + "...main": reply(http.StatusOK, `comparison`),
			})
			_, _, err := c.Compare(t.Context(), repo, commit, "main")
			assert.ErrorIs(t, err, github.ErrGitHub, "Compare refuses a body that does not decode")
		})
	})

	t.Run("ClosePullRequest", func(t *testing.T) {
		t.Parallel()

		t.Run("comments on the pull request and closes it", func(t *testing.T) {
			t.Parallel()
			var mu sync.Mutex
			var calls []string
			record := func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				mu.Lock()
				calls = append(calls, r.Method+" "+r.URL.Path+" "+string(body))
				mu.Unlock()
				_, _ = io.WriteString(w, `{}`)
			}
			c := serve(t, 0, map[string]http.HandlerFunc{
				"POST /repos/dokimasia/assert-go/issues/7/comments": record,
				"PATCH /repos/dokimasia/assert-go/pulls/7":          record,
			})
			assert.NoError(
				t,
				c.ClosePullRequest(t.Context(), "dokimasia/assert-go", 7, "The copy is current."),
				"closes",
			)
			assert.Equal(t, calls, []string{
				`POST /repos/dokimasia/assert-go/issues/7/comments {"body":"The copy is current."}`,
				`PATCH /repos/dokimasia/assert-go/pulls/7 {"state":"closed"}`,
			}, "ClosePullRequest comments first and then closes")
		})

		t.Run("returns ErrGitHub for a comment that fails", func(t *testing.T) {
			t.Parallel()
			var closed atomic.Bool
			c := serve(t, 0, map[string]http.HandlerFunc{
				"POST /repos/dokimasia/assert-go/issues/7/comments": reply(http.StatusForbidden, `{}`),
				"PATCH /repos/dokimasia/assert-go/pulls/7": func(w http.ResponseWriter, _ *http.Request) {
					closed.Store(true)
					_, _ = io.WriteString(w, `{}`)
				},
			})
			err := c.ClosePullRequest(t.Context(), "dokimasia/assert-go", 7, "The copy is current.")
			assert.ErrorIs(t, err, github.ErrGitHub, "ClosePullRequest returns the error of the comment")
			assert.False(t, closed.Load(), "ClosePullRequest leaves the pull request open")
		})

		t.Run("returns ErrGitHub for a close that fails", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"POST /repos/dokimasia/assert-go/issues/7/comments": reply(http.StatusCreated, `{}`),
				"PATCH /repos/dokimasia/assert-go/pulls/7":          reply(http.StatusForbidden, `{}`),
			})
			err := c.ClosePullRequest(t.Context(), "dokimasia/assert-go", 7, "The copy is current.")
			assert.ErrorIs(t, err, github.ErrGitHub, "ClosePullRequest returns the error of the close")
		})
	})

	t.Run("Checks", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want github.State
		}{
			{name: "returns StateNone without check runs", give: `{"check_runs": []}`, want: github.StateNone},
			{
				name: "returns StateSuccess for runs that succeed or skip",
				give: `{"check_runs": [{"status": "completed", "conclusion": "success"}, ` +
					`{"status": "completed", "conclusion": "skipped"}, {"status": "completed", "conclusion": "neutral"}]}`,
				want: github.StateSuccess,
			},
			{
				name: "returns StatePending for a run that has not completed",
				give: `{"check_runs": [{"status": "completed", "conclusion": "success"}, {"status": "in_progress"}]}`,
				want: github.StatePending,
			},
			{
				name: "returns StatePending for a run that has not completed before a run that succeeds",
				give: `{"check_runs": [{"status": "in_progress"}, {"status": "completed", "conclusion": "success"}]}`,
				want: github.StatePending,
			},
			{
				name: "returns StateFailure for a run that fails beside a run that has not completed",
				give: `{"check_runs": [{"status": "queued"}, {"status": "completed", "conclusion": "failure"}]}`,
				want: github.StateFailure,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				c := serve(t, 0, map[string]http.HandlerFunc{
					"GET /repos/dokimasia/assert-go/commits/main/check-runs": reply(http.StatusOK, tt.give),
				})
				state, err := c.Checks(t.Context(), "dokimasia/assert-go", "main")
				assert.NoError(t, err, "Checks reads the check runs")
				assert.Equal(t, state, tt.want, "Checks sums up the check runs")
			})
		}

		t.Run("returns ErrGitHub for a status outside 2xx", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/assert-go/commits/main/check-runs": reply(http.StatusNotFound, `{}`),
			})
			_, err := c.Checks(t.Context(), "dokimasia/assert-go", "main")
			assert.ErrorIs(t, err, github.ErrGitHub, "Checks refuses a status of an error")
		})
	})

	t.Run("LatestRelease", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tag and the commit of the highest stable release", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases": reply(http.StatusOK, `[
  {"tag_name": "v2.0.0-rc.1"}, {"tag_name": "v1.9.0"}, {"tag_name": "v1.10.0"},
  {"tag_name": "tools/v3.0.0"}, {"tag_name": "v1.10.1", "draft": true}
]`),
				"GET /repos/dokimasia/dokimi/git/ref/tags/v1.10.0": reply(
					http.StatusOK, `{"object": {"type": "commit", "sha": "`+commit+`"}}`,
				),
			})
			tag, sha, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.NoError(t, err, "LatestRelease reads the releases")
			assert.Equal(t, tag, "v1.10.0", "LatestRelease orders versions by their numbers")
			assert.Equal(t, sha, commit, "LatestRelease returns the commit of the tag")
		})

		t.Run("returns the release that GitHub lists first of two releases of one version", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases": reply(
					http.StatusOK,
					`[{"tag_name": "v1.0.0"}, {"tag_name": "1.0.0"}]`,
				),
				"GET /repos/dokimasia/dokimi/git/ref/tags/v1.0.0": reply(
					http.StatusOK, `{"object": {"type": "commit", "sha": "`+commit+`"}}`,
				),
			})
			tag, _, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.NoError(t, err, "LatestRelease reads the releases")
			assert.Equal(t, tag, "v1.0.0", "LatestRelease keeps the first release of the version")
		})

		t.Run("returns ErrNoRelease without a stable release", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases": reply(http.StatusOK, `[{"tag_name": "v1.0.0-beta"}]`),
			})
			_, _, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.ErrorIs(t, err, github.ErrNoRelease, "LatestRelease finds no stable release")
			assert.Contains(t, err.Error(), "dokimasia/dokimi has no stable release", "the error names the repository")
		})

		t.Run("returns ErrNoRelease for a tag that the repository lacks", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases":            reply(http.StatusOK, `[{"tag_name": "v1.0.0"}]`),
				"GET /repos/dokimasia/dokimi/git/ref/tags/v1.0.0": reply(http.StatusNotFound, `{}`),
			})
			_, _, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.ErrorIs(t, err, github.ErrNoRelease, "LatestRelease refuses a release without its tag")
		})

		t.Run("returns ErrGitHub for a list of releases that fails", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases": reply(http.StatusInternalServerError, `{}`),
			})
			_, _, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.ErrorIs(t, err, github.ErrGitHub, "LatestRelease returns the error of the list")
		})

		t.Run("returns ErrGitHub for a tag that fails", func(t *testing.T) {
			t.Parallel()
			c := serve(t, 0, map[string]http.HandlerFunc{
				"GET /repos/dokimasia/dokimi/releases":            reply(http.StatusOK, `[{"tag_name": "v1.0.0"}]`),
				"GET /repos/dokimasia/dokimi/git/ref/tags/v1.0.0": reply(http.StatusInternalServerError, `{}`),
			})
			_, _, err := c.LatestRelease(t.Context(), "dokimasia/dokimi")
			assert.ErrorIs(t, err, github.ErrGitHub, "LatestRelease returns the error of the tag")
		})
	})
}

// serveAuthorization starts a server of the repository of the cases and its branch main, which the
// test closes when it ends. The server responds to a request whose header Authorization is
// authorization, and refuses every other request with status 401.
func serveAuthorization(t *testing.T, authorization string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("Authorization") != authorization:
			http.Error(w, "the request has another authorization", http.StatusUnauthorized)
		case r.URL.Path == "/repos/"+repo:
			_, _ = io.WriteString(w, `{"default_branch": "trunk"}`)
		case r.URL.Path == "/repos/"+repo+"/git/ref/heads/main":
			_, _ = io.WriteString(w, `{"object": {"type": "commit", "sha": "`+commit+`"}}`)
		default:
			http.Error(w, "the case defines no route of the request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
