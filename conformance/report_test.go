// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.stealthscale.io/spec/conformance"
)

func TestReport(t *testing.T) {
	t.Parallel()

	t.Run("ResultsVariable", func(t *testing.T) {
		t.Parallel()

		t.Run("is SPEC_RESULTS", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				conformance.ResultsVariable,
				"SPEC_RESULTS",
				"the variable keeps the name that the runners read",
			)
		})
	})

	t.Run("Outcome", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give conformance.Outcome
				want bool
			}{
				{name: "reports true for pass", give: conformance.Pass, want: true},
				{name: "reports true for fail", give: conformance.Fail, want: true},
				{name: "reports true for skip", give: conformance.Skip, want: true},
				{name: "reports false for the empty outcome", give: "", want: false},
				{name: "reports false for an outcome in upper case", give: "PASS", want: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether the outcome is one of the three")
				})
			}
		})

		t.Run("is the spelling of each outcome in a report", func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				[]conformance.Outcome{conformance.Pass, conformance.Fail, conformance.Skip},
				[]conformance.Outcome{"pass", "fail", "skip"},
				"the outcomes keep the spellings that the runners write",
			)
		})
	})

	t.Run("DecodeReport", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the header and every result", func(t *testing.T) {
			t.Parallel()
			report, err := conformance.DecodeReport([]byte(header + `
{"file": "corpus/equal.json", "id": "equal/one", "outcome": "pass"}
{"file": "corpus/equal.json", "id": "equal/two", "outcome": "skip", "reason": "the type refuses it"}
`))
			assert.NoError(t, err, "DecodeReport accepts a report")
			assert.Equal(t, report, &conformance.Report{
				Header: conformance.Header{Repo: repo, Commit: commit, Digest: digest, Language: golang},
				Results: []conformance.Result{
					{File: "corpus/equal.json", ID: "equal/one", Outcome: conformance.Pass},
					{
						File:    "corpus/equal.json",
						ID:      "equal/two",
						Outcome: conformance.Skip,
						Reason:  "the type refuses it",
					},
				},
			}, "the report states its header and its results in order")
		})

		t.Run("returns a report without results", func(t *testing.T) {
			t.Parallel()
			report, err := conformance.DecodeReport([]byte(header))
			assert.NoError(t, err, "DecodeReport accepts a header alone")
			assert.Empty(t, report.Results, "the report has no results")
		})

		tests := []struct {
			name     string
			give     string
			wantText string
		}{
			{name: "returns ErrReport for an empty report", give: "", wantText: "the header: EOF"},
			{name: "returns ErrReport for a header that is no JSON", give: "header", wantText: "the header: "},
			{
				name:     "returns ErrReport for a key that a header does not have",
				give:     `{"language": "go", "runner": "x"}`,
				wantText: "the header: ",
			},
			{
				name:     "returns ErrReport for a header without a language",
				give:     `{"repo": "a/b"}`,
				wantText: `the header states the language ""`,
			},
			{name: "returns ErrReport for a result that is no JSON", give: header + "\nresult", wantText: "result 2: "},
			{
				name:     "returns ErrReport for a second result that is no JSON",
				give:     header + `{"file": "a", "id": "b", "outcome": "pass"}` + "\nresult",
				wantText: "result 3: ",
			},
			{
				name:     "returns ErrReport for a key that a result does not have",
				give:     header + `{"file": "a", "id": "b", "outcome": "pass", "time": 1}`,
				wantText: "result 2: ",
			},
			{
				name:     "returns ErrReport for a result whose file leaves the copy",
				give:     header + `{"file": "../a", "id": "b", "outcome": "pass"}`,
				wantText: `result 2: "../a" is no path of a file of a definition`,
			},
			{
				name:     "returns ErrReport for a result without an id",
				give:     header + `{"file": "a", "outcome": "pass"}`,
				wantText: "result 2 has no id",
			},
			{
				name:     "returns ErrReport for a result of an unknown outcome",
				give:     header + `{"file": "a", "id": "b", "outcome": "ok"}`,
				wantText: `result 2 has the outcome "ok"`,
			},
			{
				name:     "returns ErrReport for a skip without a reason",
				give:     header + `{"file": "a", "id": "b", "outcome": "skip"}`,
				wantText: "result 2 skips without a reason",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := conformance.DecodeReport([]byte(tt.give))
				assert.ErrorIs(t, err, conformance.ErrReport, "DecodeReport refuses a report that breaks a rule")
				assert.Contains(t, err.Error(), tt.wantText, "the error states the part of the report at fault")
			})
		}
	})
}
