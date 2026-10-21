// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/conformance"
	"go.stealthscale.io/spec/manifest"
)

// The source of the copy of the cases.
const (
	repo   = spec.Repo("dokimasia/assert-spec")
	commit = "27b95377f6a3c1d2e4b5a6978c0d1e2f3a4b5c6d"
	golang = spec.Language("go")
)

// files are the files of the copy of the cases: two corpus files, the case index and the case of a
// fixture directory of mutate-spec.
var files = map[string]string{
	"corpus/equal.json":       manifest.Sum([]byte("equal")),
	"corpus/prop.json":        manifest.Sum([]byte("prop")),
	"spec/cases.json":         manifest.Sum([]byte("cases")),
	"spec/corpus/x/case.json": manifest.Sum([]byte("x")),
}

// theCopy is the manifest of the copy of go of the cases.
var theCopy = manifest.Manifest{
	Files:     files,
	Languages: map[spec.Language][]string{golang: {}},
	Repo:      repo,
	Commit:    commit,
	Digest:    manifest.Digest(files),
	Version:   "1.0.0",
	Cases:     "spec/cases.json",
}

// digest is the digest of the copy of the cases.
var digest = theCopy.Digest

// header is the first line of a report of go on the copy of the cases.
var header = `{"repo": "dokimasia/assert-spec", "commit": "` + commit +
	`", "digest": "` + digest + `", "language": "go"}` + "\n"

// index is the case index of the definition of the cases: cases of two files with one id in both,
// a case that go skips, a case of go and python, a case of rust alone, and a case of a file that the
// copy of go lacks.
var index = &conformance.Index{Cases: []conformance.Case{
	{File: "corpus/equal.json", ID: "equal/one"},
	{File: "corpus/equal.json", ID: "equal/two", Skip: map[spec.Language]string{golang: "the type refuses it"}},
	{File: "corpus/prop.json", ID: "equal/one", Languages: []spec.Language{golang, "python"}},
	{File: "spec/corpus/x/case.json", ID: "x", Languages: []spec.Language{"rust"}},
	{File: "overlays/rust.json", ID: "overlay"},
}}

// complete returns a new report of go on the copy of the cases with a result for each case of the
// index that applies to go.
func complete() *conformance.Report {
	return &conformance.Report{
		Header: conformance.Header{Repo: repo, Commit: commit, Digest: digest, Language: golang},
		Results: []conformance.Result{
			{File: "corpus/equal.json", ID: "equal/one", Outcome: conformance.Pass},
			{File: "corpus/equal.json", ID: "equal/two", Outcome: conformance.Skip, Reason: "the type refuses it"},
			{File: "corpus/prop.json", ID: "equal/one", Outcome: conformance.Pass},
		},
	}
}
