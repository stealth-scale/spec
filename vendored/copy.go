// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
)

// ManifestFile is the name of the manifest of a copy, at the root of the directory of the copy.
const ManifestFile = "manifest.json"

// Copy is the content of a vendored copy of a definition: the files of the view of the languages
// of an entry, the manifest of the copy, and the files of the copies of the entry.
//
// # Concurrency
//
// [Write], [Overwrite] and the callers of [Build] read a Copy and never write it, so any number of
// goroutines may read it at once.
type Copy struct {
	// Dir is the slash-separated directory of the copy, relative to the root of the repository.
	Dir string

	// Files maps the slash-separated path of each file of the copy, relative to the root of the
	// repository, to its content: the files of the view below Dir, the manifest of the copy at
	// ManifestFile below Dir, and each file that a copy of the entry writes outside Dir.
	Files map[string][]byte

	// Manifest is the manifest of the copy: the view of the languages of the entry, with the
	// repository of the definition and the commit, or Local for a copy of a checkout.
	Manifest manifest.Manifest
}

// Build returns the copy of tree that the entry e vendors, from commit of the repository of e, or
// from a checkout on the disk for an empty commit. The copy contains the files of the view of the
// languages of e below the path of e at their paths in the definition, the manifest of the view
// with its source, and the file of each copy of e.
//
// Error modes:
//   - the error of [definition.Tree.Manifest], and the error of [manifest.Manifest.View], which
//     wraps [manifest.ErrLanguage] for a language of e that the definition does not declare.
//   - an error that wraps [ErrCopy] for a file of the view at ManifestFile, and for a copy of e
//     from a file that the view does not have.
func Build(tree *definition.Tree, e *implementation.Entry, commit string) (*Copy, error) {
	whole, err := tree.Manifest()
	if err != nil {
		return nil, fmt.Errorf("vendored: %w", err)
	}
	view, err := whole.View(e.Languages)
	if err != nil {
		return nil, fmt.Errorf("vendored: %w", err)
	}
	if _, ok := view.Files[ManifestFile]; ok {
		return nil, fmt.Errorf(
			"%w: the definition has a file at %s, the path of the manifest of a copy",
			ErrCopy,
			ManifestFile,
		)
	}
	view.Repo, view.Commit, view.Local = e.Definition, commit, commit == ""
	files := make(map[string][]byte, len(view.Files)+1+len(e.Copies))
	for p := range view.Files {
		files[path.Join(e.Path, p)] = tree.Files[p]
	}
	files[path.Join(e.Path, ManifestFile)] = view.Encode()
	for _, c := range e.Copies {
		if _, ok := view.Files[c.From]; !ok {
			return nil, fmt.Errorf("%w: the copy to %s names %s, which is no file of the copy", ErrCopy, c.To, c.From)
		}
		files[c.To] = tree.Files[c.From]
	}
	return &Copy{Files: files, Manifest: view, Dir: e.Path}, nil
}

// Entries returns the names of the entries that the copies with the manifests ms own in the
// directory of a copy: [ManifestFile], and the first element of the path of each file of each
// manifest. It returns each name once, in order. A copy of the files VERSION, corpus/a.json and
// spec/zones.json owns the entries VERSION, corpus, manifest.json and spec. Every other entry of the
// directory belongs to the repository.
func Entries(ms ...manifest.Manifest) []string {
	names := []string{ManifestFile}
	for _, m := range ms {
		for p := range m.Files {
			first, _, _ := strings.Cut(p, "/")
			names = append(names, first)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}
