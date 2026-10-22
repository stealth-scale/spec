// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/vendored"
)

// The values of the copies that the cases build. other is a file of the repository in the
// directory of the copy, outside the entries of the copy.
const (
	repo   = spec.Repo("dokimasia/assert-spec")
	commit = "27b95377f6a3c1d2e4b5a6978c0d1e2f3a4b5c6d"
	dir    = "conformance/spec"
	zones  = "internal/zone/zones.json"
	other  = dir + "/definition.go"
)

// otherText is the content of the file other.
const otherText = "package spec\n"

// config configures the definitions of the cases.
const config = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "spec/*.json", "corpus/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
`

// first is the first revision of the definition of the cases.
var first = map[string]string{
	spec.ConfigFile:      config,
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
	spec.ConfigFile:      config,
	"VERSION":            "1.1.0\n",
	"spec/zones.json":    `{"zones": 2}` + "\n",
	"corpus/a.json":      `{"case": "a", "version": 2}` + "\n",
	"corpus/c.json":      `{"case": "c"}` + "\n",
	"overlays/go.json":   `{"language": "go"}` + "\n",
	"overlays/rust.json": `{"language": "rust"}` + "\n",
}

// thirdConfig configures a definition without overlays.
const thirdConfig = `definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, "spec/*.json", "corpus/*.json"]
`

// third is a revision of the definition without overlays, so its copy for entry lacks the entry
// overlays of the copies of first and second.
var third = map[string]string{
	spec.ConfigFile:   thirdConfig,
	"VERSION":         "1.2.0\n",
	"spec/zones.json": `{"zones": 3}` + "\n",
	"corpus/a.json":   `{"case": "a", "version": 3}` + "\n",
}

// entry is the entry of the cases: the go files of the definition at dir, and the zone table at
// zones.
var entry = implementation.Entry{
	Definition: repo,
	Track:      implementation.DefaultTrack,
	Path:       dir,
	Languages:  []spec.Language{"go"},
	Copies:     []implementation.Copy{{From: "spec/zones.json", To: zones}},
}

// copyPath is the directory of the copy of entry in the separators of the system.
var copyPath = filepath.FromSlash(dir)

// errDisk is the error of the call of a disk that a case makes fail.
var errDisk = errors.New("disk: injected failure")

// failing is a disk that counts its calls, and fails the call whose number is fail with errDisk.
// A fail of zero fails no call.
type failing struct {
	vendored.Disk

	calls int
	fail  int
}

// ReadFile counts the call, and reads name from the disk that f wraps.
func (f *failing) ReadFile(name string) ([]byte, error) {
	if f.next() {
		return nil, errDisk
	}
	return f.Disk.ReadFile(name)
}

// WriteFile counts the call, and writes name on the disk that f wraps.
func (f *failing) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if f.next() {
		return errDisk
	}
	return f.Disk.WriteFile(name, data, perm)
}

// MkdirAll counts the call, and creates name on the disk that f wraps.
func (f *failing) MkdirAll(name string, perm fs.FileMode) error {
	if f.next() {
		return errDisk
	}
	return f.Disk.MkdirAll(name, perm)
}

// Rename counts the call, and renames oldname on the disk that f wraps.
func (f *failing) Rename(oldname, newname string) error {
	if f.next() {
		return errDisk
	}
	return f.Disk.Rename(oldname, newname)
}

// RemoveAll counts the call, and removes name from the disk that f wraps.
func (f *failing) RemoveAll(name string) error {
	if f.next() {
		return errDisk
	}
	return f.Disk.RemoveAll(name)
}

// Lstat counts the call, and describes name on the disk that f wraps.
func (f *failing) Lstat(name string) (fs.FileInfo, error) {
	if f.next() {
		return nil, errDisk
	}
	return f.Disk.Lstat(name)
}

// next counts a call, and reports whether the call fails.
func (f *failing) next() bool {
	f.calls++
	return f.calls == f.fail
}

// cleaner is a test or a case of a property, which runs a function when it ends.
type cleaner interface {
	assert.TB
	Cleanup(f func())
}

// openRoot opens the root of the directory path, which tb closes when it ends.
func openRoot(tb cleaner, path string) *os.Root {
	tb.Helper()
	r, err := os.OpenRoot(path)
	assert.NoError(tb, err, "the directory opens as a root")
	tb.Cleanup(func() { _ = r.Close() })
	return r
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

// build returns the copy of the definition contents for e at the commit at.
func build(tb assert.TB, contents map[string]string, e *implementation.Entry, at string) *vendored.Copy {
	tb.Helper()
	c, err := vendored.Build(load(tb, contents), e, at)
	assert.NoError(tb, err, "the copy of the case builds")
	return c
}
