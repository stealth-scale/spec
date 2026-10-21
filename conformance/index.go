// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/manifest"
)

// Case is a case of a case index: the file of the definition that states it, its id in that file,
// the languages that it applies to, and the languages that skip it with the reason of each.
type Case struct {
	// Skip maps each language that skips the case to the reason, which the runner of the language
	// writes with its result.
	Skip map[spec.Language]string `json:"skip,omitempty"`

	// File is the slash-separated path of the file of the case, relative to the root of the
	// definition.
	File string `json:"file"`

	// ID is the id of the case, unique in its file.
	ID string `json:"id"`

	// Languages are the languages that the case applies to, such as the languages of the fixtures
	// of a case of mutate-spec. An empty list applies the case to every language.
	Languages []spec.Language `json:"languages,omitempty"`
}

// Index is the case index of a definition, which the tooling of the definition writes: every case
// of the corpus, each with its file and its id. A case is identified by its file and its id
// together, because two files can state one id.
//
//	{"cases": [
//	  {"file": "corpus/equal.json", "id": "equal/int-against-string", "skip": {"go": "..."}},
//	  {"file": "spec/corpus/arithmetic/case.json", "id": "arithmetic", "languages": ["go"]}
//	]}
type Index struct {
	// Cases are the cases of the index.
	Cases []Case `json:"cases"`
}

// DecodeIndex returns the case index that data encodes.
//
// Error modes:
//   - an error that wraps [ErrIndex] for data that is no JSON object of an index, such as an object
//     without cases or with an unknown key, and for data after the object.
//   - an error that wraps [ErrIndex] for a case that breaks a rule: a file that is no path below the
//     root of a definition, an empty id, a file and an id of an earlier case, a language that is not
//     valid, [spec.All] or a language that the case lists twice, and a skip without a reason.
func DecodeIndex(data []byte) (*Index, error) {
	var index Index
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&index); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIndex, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: data follows the object", ErrIndex)
	}
	if index.Cases == nil {
		return nil, fmt.Errorf("%w: the index states no cases", ErrIndex)
	}
	seen := make(map[key]bool, len(index.Cases))
	for _, c := range index.Cases {
		k := key{file: c.File, id: c.ID}
		switch {
		case !manifest.ValidPath(c.File):
			return nil, fmt.Errorf("%w: %q is no path of a file of a definition", ErrIndex, c.File)
		case c.ID == "":
			return nil, fmt.Errorf("%w: a case of %s has no id", ErrIndex, c.File)
		case seen[k]:
			return nil, fmt.Errorf("%w: the index lists %s of %s twice", ErrIndex, c.ID, c.File)
		}
		seen[k] = true
		for i, l := range c.Languages {
			if !l.Valid() || l == spec.All || slices.Contains(c.Languages[:i], l) {
				return nil, fmt.Errorf("%w: the languages %v of %s of %s", ErrIndex, c.Languages, c.ID, c.File)
			}
		}
		for _, l := range slices.Sorted(maps.Keys(c.Skip)) {
			if !l.Valid() || c.Skip[l] == "" {
				return nil, fmt.Errorf("%w: the skip of %q of %s of %s", ErrIndex, l, c.ID, c.File)
			}
		}
	}
	return &index, nil
}

// key identifies a case by its file and its id.
type key struct {
	file, id string
}
