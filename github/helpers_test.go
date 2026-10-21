// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package github_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/github"
)

// The repository and the commit of the cases.
const (
	repo   = "dokimasia/assert-spec"
	commit = "27b95377f6a3c1d2e4b5a6978c0d1e2f3a4b5c6d"
	token  = "token of the cases"
)

// The headers of the requests of a client.
const (
	apiVersion = "2022-11-28"
	jsonType   = "application/vnd.github+json"
	agent      = "spec"
	forgeAgent = "ergon"
)

// timeout bounds each request of a case, so a handler that a defect stalls fails the case.
const timeout = 5 * time.Second

// entry is an entry of an archive of the cases.
type entry struct {
	name string
	body string
	link string
	kind byte
}

// serve starts a server of the routes of the case, which the test closes when it ends, and returns
// a client of it whose archives may have up to limit bytes, or the default for a limit of 0. The
// server refuses a request without the headers of a client, and it responds with status 500 to a
// request of a route that the case does not define.
func serve(t *testing.T, limit int64, routes map[string]http.HandlerFunc) *github.Client {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			contentType := ""
			if r.ContentLength > 0 {
				contentType = jsonType
			}
			switch {
			case r.Header.Get("Authorization") != "Bearer "+token:
				http.Error(w, "the request has no token", http.StatusUnauthorized)
			case r.Header.Get("X-GitHub-Api-Version") != apiVersion:
				http.Error(w, "the request names no version of the API", http.StatusBadRequest)
			case !slices.Contains([]string{agent, forgeAgent}, r.Header.Get("User-Agent")):
				http.Error(w, "the request names an unknown agent", http.StatusBadRequest)
			case r.Header.Get("Content-Type") != contentType:
				http.Error(w, "the request states another type of its body", http.StatusBadRequest)
			default:
				handler(w, r)
			}
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "the case defines no route of the request", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c, err := github.New(&http.Client{Timeout: timeout}, github.Config{
		Token:           token,
		API:             server.URL,
		GraphQL:         server.URL + "/graphql",
		MaxArchiveBytes: limit,
	})
	assert.NoError(t, err, "New returns a client of the server")
	return c
}

// reply returns a handler that writes body with status.
func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// noise returns 32 KiB of bytes that gzip cannot compress: a chain of SHA-256 digests.
func noise() []byte {
	out := make([]byte, 0, 32<<10)
	sum := sha256.Sum256(nil)
	for len(out) < cap(out) {
		out = append(out, sum[:]...)
		sum = sha256.Sum256(sum[:])
	}
	return out
}

// tarball returns the gzip archive of entries, as GitHub serves the archive of a commit.
func tarball(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		header := &tar.Header{Name: e.name, Typeflag: e.kind, Linkname: e.link, Mode: 0o644, Size: int64(len(e.body))}
		switch e.kind {
		case tar.TypeReg:
		case tar.TypeXGlobalHeader:
			header = &tar.Header{Typeflag: e.kind, PAXRecords: map[string]string{"comment": commit}}
		default:
			header.Size = 0
		}
		assert.NoError(t, tw.WriteHeader(header), "the header of the entry writes")
		if e.kind == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			assert.NoError(t, err, "the body of the entry writes")
		}
	}
	assert.NoError(t, tw.Close(), "the archive closes")
	assert.NoError(t, zw.Close(), "the compression closes")
	return b.Bytes()
}
