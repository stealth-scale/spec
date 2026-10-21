// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// ResultsVariable is the environment variable that names the file of the report of a run of a
// runner. A runner writes a report when the variable names a file.
const ResultsVariable = "SPEC_RESULTS"

// Outcome is the outcome of a case in a run of a runner.
type Outcome string

// The outcomes of a case. A report writes each as its value.
const (
	// Pass is the outcome of a case whose checks the implementation passes.
	Pass Outcome = "pass"

	// Fail is the outcome of a case whose checks the implementation fails.
	Fail Outcome = "fail"

	// Skip is the outcome of a case that the runner does not run, for the reason that the case
	// index declares.
	Skip Outcome = "skip"
)

// Valid reports whether o is [Pass], [Fail] or [Skip].
func (o Outcome) Valid() bool {
	switch o {
	case Pass, Fail, Skip:
		return true
	default:
		return false
	}
}

// Header is the first line of a report: the source and the digest of the copy that the run read,
// as its manifest states them, and the language of the runner.
type Header struct {
	// Repo is the repository of the definition of the copy.
	Repo spec.Repo `json:"repo"`

	// Commit is the commit of the copy, and empty for a local copy.
	Commit string `json:"commit"`

	// Digest is the digest of the copy.
	Digest string `json:"digest"`

	// Language is the language of the runner.
	Language spec.Language `json:"language"`
}

// Result is the outcome of one case in a run, on a line of its own in a report.
type Result struct {
	// File is the file of the case, relative to the root of the definition.
	File string `json:"file"`

	// ID is the id of the case.
	ID string `json:"id"`

	// Outcome is the outcome of the case.
	Outcome Outcome `json:"outcome"`

	// Reason is the reason of a skip, and optional for any other outcome.
	Reason string `json:"reason,omitempty"`
}

// Report is the report of a run of a runner, which the runner writes as JSON lines to the file
// that [ResultsVariable] names: the header on the first line, and one result on each further line.
//
//	{"repo": "dokimasia/assert-spec", "commit": "27b9537…", "digest": "sha256:…", "language": "go"}
//	{"file": "corpus/equal.json", "id": "equal/null-against-empty-list", "outcome": "pass"}
//	{"file": "corpus/equal.json", "id": "equal/int-against-string", "outcome": "skip", "reason": "…"}
type Report struct {
	// Header is the header of the report.
	Header Header

	// Results are the results of the report, in the order of the report.
	Results []Result
}

// DecodeReport returns the report that data encodes: a header and any number of results, each a
// JSON object. The objects may be separated by any white space.
//
// Error modes:
//   - an error that wraps [ErrReport] for data without a header, and for a header or a result that
//     is no JSON object of its kind, such as one with an unknown key.
//   - an error that wraps [ErrReport] for a header whose language is not valid, and for a result
//     whose file is no path below the root of a definition, whose id is empty, whose outcome is not
//     valid, or which is a skip without a reason.
func DecodeReport(data []byte) (*Report, error) {
	var r Report
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r.Header); err != nil {
		return nil, fmt.Errorf("%w: the header: %w", ErrReport, err)
	}
	if !r.Header.Language.Valid() {
		return nil, fmt.Errorf("%w: the header states the language %q", ErrReport, r.Header.Language)
	}
	for line := 2; ; line++ {
		var res Result
		err := dec.Decode(&res)
		if errors.Is(err, io.EOF) {
			return &r, nil
		}
		switch {
		case err != nil:
			return nil, fmt.Errorf("%w: result %d: %w", ErrReport, line, err)
		case !manifest.ValidPath(res.File):
			return nil, fmt.Errorf("%w: result %d: %q is no path of a file of a definition", ErrReport, line, res.File)
		case res.ID == "":
			return nil, fmt.Errorf("%w: result %d has no id", ErrReport, line)
		case !res.Outcome.Valid():
			return nil, fmt.Errorf("%w: result %d has the outcome %q", ErrReport, line, res.Outcome)
		case res.Outcome == Skip && res.Reason == "":
			return nil, fmt.Errorf("%w: result %d skips without a reason", ErrReport, line)
		}
		r.Results = append(r.Results, res)
	}
}
