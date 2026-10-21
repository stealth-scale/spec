// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"fmt"
	"slices"

	"go.stealthscale.io/spec/manifest"
)

// Verify checks that the report r accounts for every case of index that applies to the language of
// r in the copy whose manifest is m. A case applies when the copy has its file and its languages
// name the language of r or nothing. The report accounts for the cases when its header states the
// source and the digest of m and a language of m, and each case that applies has one result: a
// pass, or a skip for the reason that the index declares for the language.
//
// Error modes:
//   - a [*VerifyError], which wraps [ErrVerify], with every problem of r: a header of another
//     copy, a language that m does not state, a result of no case that applies, a second result of
//     a case, a case that fails, a skip that the index does not declare or declares for another
//     reason, a pass of a case that the index declares as skipped, and a case without a result.
func Verify(index *Index, m *manifest.Manifest, r *Report) error {
	var problems []string
	h := r.Header
	if h.Repo != m.Repo || h.Commit != m.Commit || h.Digest != m.Digest {
		problems = append(problems, fmt.Sprintf(
			"the report comes from %s at %q with the digest %s, and the copy from %s at %q with the digest %s",
			h.Repo, h.Commit, h.Digest, m.Repo, m.Commit, m.Digest,
		))
	}
	if _, ok := m.Languages[h.Language]; !ok {
		problems = append(problems, fmt.Sprintf("the copy vendors no language %s", h.Language))
	}
	applies := make(map[key]Case, len(index.Cases))
	var order []key
	for _, c := range index.Cases {
		_, inCopy := m.Files[c.File]
		if !inCopy || len(c.Languages) > 0 && !slices.Contains(c.Languages, h.Language) {
			continue
		}
		k := key{file: c.File, id: c.ID}
		applies[k] = c
		order = append(order, k)
	}
	seen := make(map[key]bool, len(r.Results))
	for _, res := range r.Results {
		k := key{file: res.File, id: res.ID}
		c, ok := applies[k]
		switch {
		case !ok:
			problems = append(
				problems,
				fmt.Sprintf("%s of %s is no case of %s in the copy", res.ID, res.File, h.Language),
			)
			continue
		case seen[k]:
			problems = append(problems, fmt.Sprintf("%s of %s has two results", res.ID, res.File))
			continue
		}
		seen[k] = true
		reason, declared := c.Skip[h.Language]
		switch {
		case res.Outcome == Fail:
			problems = append(problems, fmt.Sprintf("%s of %s fails", res.ID, res.File))
		case res.Outcome == Skip && !declared:
			problems = append(problems, fmt.Sprintf(
				"%s of %s skips, and the index declares no skip of %s", res.ID, res.File, h.Language,
			))
		case res.Outcome == Skip && res.Reason != reason:
			problems = append(problems, fmt.Sprintf(
				"%s of %s skips for %q, and the index declares %q", res.ID, res.File, res.Reason, reason,
			))
		case res.Outcome == Pass && declared:
			problems = append(problems, fmt.Sprintf(
				"%s of %s passes, and the index declares a skip of %s", res.ID, res.File, h.Language,
			))
		}
	}
	for _, k := range order {
		if !seen[k] {
			problems = append(problems, fmt.Sprintf("%s of %s has no result", k.id, k.file))
		}
	}
	if len(problems) > 0 {
		return &VerifyError{Problems: problems}
	}
	return nil
}
