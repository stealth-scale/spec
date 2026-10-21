// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/prop"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/conformance"
	"go.stealthscale.io/spec/manifest"
)

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a result of every case that applies to the language", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, conformance.Verify(index, &theCopy, complete()), "Verify accepts a complete report")
		})

		t.Run("returns nil for a report of a local copy", func(t *testing.T) {
			t.Parallel()
			local := theCopy
			local.Commit, local.Local = "", true
			report := complete()
			report.Header.Commit = ""
			assert.NoError(t, conformance.Verify(index, &local, report), "Verify accepts a report of a local copy")
		})

		t.Run("returns nil for a pass with a reason", func(t *testing.T) {
			t.Parallel()
			report := complete()
			report.Results[0].Reason = "the case runs in 2 ms"
			assert.NoError(t, conformance.Verify(index, &theCopy, report), "Verify accepts the reason of a pass")
		})

		// source is the last part of the problem that Verify reports for a report of another copy.
		source := `, and the copy from dokimasia/assert-spec at "` + commit + `" with the digest ` + digest

		tests := []struct {
			name string
			give func(r *conformance.Report)
			want string
		}{
			{
				name: "returns ErrVerify for a report of another commit",
				give: func(r *conformance.Report) { r.Header.Commit = "1111111111111111111111111111111111111111" },
				want: `the report comes from dokimasia/assert-spec at "1111111111111111111111111111111111111111" ` +
					"with the digest " + digest + source,
			},
			{
				name: "returns ErrVerify for a report of another digest",
				give: func(r *conformance.Report) { r.Header.Digest = manifest.Sum(nil) },
				want: `the report comes from dokimasia/assert-spec at "` + commit + `" with the digest ` +
					manifest.Sum(nil) + source,
			},
			{
				name: "returns ErrVerify for a report of another repository",
				give: func(r *conformance.Report) { r.Header.Repo = "dokimasia/mutate-spec" },
				want: `the report comes from dokimasia/mutate-spec at "` + commit + `" with the digest ` + digest + source,
			},
			{
				name: "returns ErrVerify for a report of a language that the copy does not vendor",
				give: func(r *conformance.Report) {
					r.Header.Language = "python"
					r.Results[1] = conformance.Result{
						File:    "corpus/equal.json",
						ID:      "equal/two",
						Outcome: conformance.Pass,
					}
				},
				want: "the copy vendors no language python",
			},
			{
				name: "returns ErrVerify for a case without a result",
				give: func(r *conformance.Report) { r.Results = r.Results[:2] },
				want: "equal/one of corpus/prop.json has no result",
			},
			{
				name: "returns ErrVerify for a case with two results",
				give: func(r *conformance.Report) { r.Results = append(r.Results, r.Results[0]) },
				want: "equal/one of corpus/equal.json has two results",
			},
			{
				name: "returns ErrVerify for a result of a case that the index does not have",
				give: func(r *conformance.Report) {
					r.Results = append(
						r.Results,
						conformance.Result{File: "corpus/equal.json", ID: "equal/three", Outcome: conformance.Pass},
					)
				},
				want: "equal/three of corpus/equal.json is no case of go in the copy",
			},
			{
				name: "returns ErrVerify for a result of a case of another language",
				give: func(r *conformance.Report) {
					r.Results = append(
						r.Results,
						conformance.Result{File: "spec/corpus/x/case.json", ID: "x", Outcome: conformance.Pass},
					)
				},
				want: "x of spec/corpus/x/case.json is no case of go in the copy",
			},
			{
				name: "returns ErrVerify for a result of a case of a file that the copy lacks",
				give: func(r *conformance.Report) {
					r.Results = append(
						r.Results,
						conformance.Result{File: "overlays/rust.json", ID: "overlay", Outcome: conformance.Pass},
					)
				},
				want: "overlay of overlays/rust.json is no case of go in the copy",
			},
			{
				name: "returns ErrVerify for a case that fails",
				give: func(r *conformance.Report) { r.Results[0].Outcome = conformance.Fail },
				want: "equal/one of corpus/equal.json fails",
			},
			{
				name: "returns ErrVerify for a skip that the index does not declare",
				give: func(r *conformance.Report) {
					r.Results[0] = conformance.Result{
						File: "corpus/equal.json", ID: "equal/one", Outcome: conformance.Skip, Reason: "no time",
					}
				},
				want: "equal/one of corpus/equal.json skips, and the index declares no skip of go",
			},
			{
				name: "returns ErrVerify for a skip for another reason",
				give: func(r *conformance.Report) { r.Results[1].Reason = "no time" },
				want: `equal/two of corpus/equal.json skips for "no time", and the index declares "the type refuses it"`,
			},
			{
				name: "returns ErrVerify for a pass of a case that the index declares as skipped",
				give: func(r *conformance.Report) {
					r.Results[1] = conformance.Result{
						File: "corpus/equal.json", ID: "equal/two", Outcome: conformance.Pass,
					}
				},
				want: "equal/two of corpus/equal.json passes, and the index declares a skip of go",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				report := complete()
				tt.give(report)
				err := conformance.Verify(index, &theCopy, report)
				problems := assert.ErrorAs[*conformance.VerifyError](
					t,
					err,
					"Verify reports the problems of the report",
				)
				assert.Equal(t, problems.Problems, []string{tt.want}, "Verify reports the problem of the case alone")
			})
		}

		t.Run("returns one problem for the one result that a report leaves out", func(t *testing.T) {
			t.Parallel()
			prop.ForAll(t, "Verify finds the result that a complete report leaves out", func(c *prop.Case) {
				n := c.Draw(prop.Integer(1, 12), "cases")
				generated := &conformance.Index{}
				full := &conformance.Report{Header: complete().Header}
				for i := range n {
					id := "case/" + strconv.Itoa(i)
					result := conformance.Result{File: "corpus/equal.json", ID: id, Outcome: conformance.Pass}
					entry := conformance.Case{File: "corpus/equal.json", ID: id}
					if c.Draw(prop.Boolean(), "skipped") {
						entry.Skip = map[spec.Language]string{golang: "skipped " + id}
						result.Outcome, result.Reason = conformance.Skip, "skipped "+id
					}
					generated.Cases = append(generated.Cases, entry)
					full.Results = append(full.Results, result)
				}
				assert.NoError(c, conformance.Verify(generated, &theCopy, full), "Verify accepts the complete report")
				drop := c.Draw(prop.Integer(0, n-1), "dropped")
				partial := &conformance.Report{Header: full.Header}
				partial.Results = append(append(partial.Results, full.Results[:drop]...), full.Results[drop+1:]...)
				problems := assert.ErrorAs[*conformance.VerifyError](
					c, conformance.Verify(generated, &theCopy, partial), "Verify refuses the partial report",
				)
				assert.Length(c, problems.Problems, 1, "the partial report has one problem")
				assert.Contains(
					c,
					problems.Problems[0],
					full.Results[drop].ID+" ",
					"the problem names the dropped case",
				)
			})
		})
	})
}
