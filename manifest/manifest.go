// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"go.stealthscale.io/spec"
)

// The keys of the JSON object of a manifest. The json tags of [Manifest] spell the same keys.
const (
	keyCases     = "cases"
	keyCommit    = "commit"
	keyDigest    = "digest"
	keyFiles     = "files"
	keyLanguages = "languages"
	keyLocal     = "local"
	keyRepo      = "repo"
	keyVersion   = "version"
)

// indent indents each level of the JSON of a manifest by four spaces.
const indent = "    "

// forbidden contains the characters that no path of a manifest contains: the backslash, which
// Windows reads as a separator, and NUL.
const forbidden = "\\\x00"

// unportable contains the printable characters that Windows refuses in a name. Windows also
// refuses the control characters below U+0020.
const unportable = `<>:"|?*`

// commitPattern matches the name of a commit of git: 40 lowercase hexadecimal digits of SHA-1, or
// 64 of SHA-256.
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// reserved contains the names of the devices of Windows in upper case. Windows opens the device
// for a name whose part before its first dot is the name of a device in any case, such as
// con.json, so no file has such a name. The superscript digits count as the digits of a port.
var reserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true, "CONIN$": true, "CONOUT$": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true,
	"COM8": true, "COM9": true, "COM\U000000B9": true, "COM\U000000B2": true, "COM\U000000B3": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true,
	"LPT8": true, "LPT9": true, "LPT\U000000B9": true, "LPT\U000000B2": true, "LPT\U000000B3": true,
}

// Manifest is the manifest of a definition or of a copy of a definition.
//
// The manifest of a definition lists each file that an implementation vendors and maps each
// language to its own files. It also states the version of the definition. A file that is not in
// the list of any language is shared by every language. The manifest of a copy is the
// [Manifest.View] of the languages of an implementation, with the repository of the definition and
// the commit that the copy comes from, or Local for a copy of a checkout on the disk.
//
// The zero value states no file and no version, and [Decode] refuses its encoding.
//
// # Concurrency
//
// A Manifest is a value whose maps the methods read and never write, so any number of goroutines
// may call its methods at once. A caller that changes a map synchronizes its readers.
type Manifest struct {
	// Files maps the slash-separated path of each file to its digest, as [Sum] computes it.
	Files map[string]string `json:"files"`

	// Languages maps each language to the paths of its own files, in the order of their bytes. A
	// language without files of its own maps to an empty list. A manifest without languages is
	// nil, and every one of its files is shared.
	Languages map[spec.Language][]string `json:"languages"`

	// Repo is the repository of the definition of a copy, and empty for a definition.
	Repo spec.Repo `json:"repo"`

	// Commit is the commit of the definition that a copy comes from, and empty for a definition
	// and for a local copy.
	Commit string `json:"commit"`

	// Digest is the [Digest] of Files.
	Digest string `json:"digest"`

	// Version is the version of the definition, such as 9.0.0.
	Version string `json:"version"`

	// Cases is the path of the case index of the definition, which Files lists, or empty for a
	// manifest without a case index.
	Cases string `json:"cases"`

	// Local is true for a copy of a checkout of the definition on the disk, which no commit
	// identifies.
	Local bool `json:"local"`
}

// New returns the manifest of the files contents of a definition at version. contents maps the
// slash-separated path of each file to its content, and languages maps each language to the paths
// of its own files. The manifest lists the files of each language in the order of their bytes, and
// it does not share a slice with languages.
//
// Error modes:
//   - an error that wraps [ErrInvalid] for an empty version, and for a version with white space at
//     an end.
//   - an error that wraps [ErrInvalid] for a path that [PortablePath] refuses, such as an absolute
//     path, a path with an element of .., a path with a backslash and the path of a device of
//     Windows.
//   - an error that wraps [ErrInvalid] for two paths that differ only in case, and for the path of
//     a file that is also the path of a directory.
//   - an error that wraps [ErrInvalid] for a language that is not valid, for [spec.All], for a file
//     of a language that contents lacks, and for a file that two languages list or that one
//     language lists twice.
func New(version string, contents map[string][]byte, languages map[spec.Language][]string) (Manifest, error) {
	if err := checkVersion(ErrInvalid, version); err != nil {
		return Manifest{}, err
	}
	files := make(map[string]string, len(contents))
	for _, p := range slices.Sorted(maps.Keys(contents)) {
		if err := checkPath(ErrInvalid, p); err != nil {
			return Manifest{}, err
		}
		files[p] = Sum(contents[p])
	}
	if err := checkCollisions(ErrInvalid, files); err != nil {
		return Manifest{}, err
	}
	own := make(map[spec.Language][]string, len(languages))
	for l, paths := range languages {
		list := append(make([]string, 0, len(paths)), paths...)
		slices.Sort(list)
		own[l] = list
	}
	if err := checkLanguages(ErrInvalid, files, own); err != nil {
		return Manifest{}, err
	}
	return Manifest{Files: files, Languages: own, Digest: Digest(files), Version: version}, nil
}

// View returns the manifest of the files that the languages langs vendor: every shared file of m,
// and the files of each language of langs. View takes every language of m for [spec.All]. The view
// states the version of m, the languages of langs, the Digest of its own files, the case index of m
// when it keeps the file of the index, and no source. It does not share a map or a slice with m.
//
// Error modes:
//   - an error that wraps [ErrLanguage] for an empty langs, and for a language that m does not
//     state, such as any language of a manifest without languages.
func (m *Manifest) View(langs []spec.Language) (Manifest, error) {
	if len(langs) == 0 {
		return Manifest{}, fmt.Errorf("%w: the view names no language", ErrLanguage)
	}
	keep := make(map[spec.Language]bool, len(m.Languages))
	for _, l := range langs {
		if l == spec.All {
			for k := range m.Languages {
				keep[k] = true
			}
			continue
		}
		if _, ok := m.Languages[l]; !ok {
			return Manifest{}, fmt.Errorf("%w: the manifest states no language %q", ErrLanguage, l)
		}
		keep[l] = true
	}
	files := maps.Clone(m.Files)
	own := make(map[spec.Language][]string, len(keep))
	for l, paths := range m.Languages {
		if keep[l] {
			own[l] = slices.Clone(paths)
			continue
		}
		for _, p := range paths {
			delete(files, p)
		}
	}
	cases := m.Cases
	if _, ok := files[cases]; !ok {
		cases = ""
	}
	return Manifest{Files: files, Languages: own, Digest: Digest(files), Version: m.Version, Cases: cases}, nil
}

// Encode returns the JSON object of m with the keys in the order of their bytes, an indent of four
// spaces for each level and a line feed at the end. The object always has the keys digest, files
// and version. It has languages when m states languages, repo, commit and cases when m states them,
// and local when Local is true. Encode does not escape the characters of HTML, so the JSON of a
// manifest of printable ASCII paths is the JSON that json.dumps(indent=4, sort_keys=True) of
// Python writes for it.
//
// Encode does not check m, and [Decode] refuses the encoding of a manifest that breaks a rule.
func (m *Manifest) Encode() []byte {
	object := map[string]any{keyDigest: m.Digest, keyFiles: m.Files, keyVersion: m.Version}
	if m.Languages != nil {
		object[keyLanguages] = m.Languages
	}
	if m.Repo != "" {
		object[keyRepo] = m.Repo
	}
	if m.Commit != "" {
		object[keyCommit] = m.Commit
	}
	if m.Local {
		object[keyLocal] = m.Local
	}
	if m.Cases != "" {
		object[keyCases] = m.Cases
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	// The object contains strings, a bool, and lists and maps of strings, which encode without an
	// error, and a bytes.Buffer grows without one.
	_ = enc.Encode(object)
	return b.Bytes()
}

// Decode returns the manifest that data encodes, as [Manifest.Encode] writes it. It accepts the
// keys in any order and any white space, and a manifest without languages. It returns the zero
// Manifest with each error.
//
// Error modes:
//   - an error that wraps [ErrCorrupt] for data that is no JSON object of a manifest, such as an
//     object with an unknown key, and for data after the object.
//   - an error that wraps [ErrCorrupt] for a manifest without files, for a version, a path or two
//     paths that [New] refuses, for the digest of a file that is not a digest of [Sum], and for a
//     digest that is not the [Digest] of the files.
//   - an error that wraps [ErrCorrupt] for languages that New refuses, for a list of files of a
//     language that is null, and for a case index that the files do not list.
//   - an error that wraps [ErrCorrupt] for a repository that is not valid, for a commit that is not
//     a commit of git, for a definition with a commit or Local, and for a copy without exactly one
//     of a commit and Local.
func Decode(data []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("%w: data follows the object", ErrCorrupt)
	}
	if err := checkVersion(ErrCorrupt, m.Version); err != nil {
		return Manifest{}, err
	}
	if m.Files == nil {
		return Manifest{}, fmt.Errorf("%w: the manifest states no files", ErrCorrupt)
	}
	for _, p := range slices.Sorted(maps.Keys(m.Files)) {
		if err := checkPath(ErrCorrupt, p); err != nil {
			return Manifest{}, err
		}
		if !digestPattern.MatchString(m.Files[p]) {
			return Manifest{}, fmt.Errorf("%w: the digest %q of %s is no digest", ErrCorrupt, m.Files[p], p)
		}
	}
	if err := checkCollisions(ErrCorrupt, m.Files); err != nil {
		return Manifest{}, err
	}
	if want := Digest(m.Files); m.Digest != want {
		return Manifest{}, fmt.Errorf("%w: the digest %s is not the digest %s of the files", ErrCorrupt, m.Digest, want)
	}
	if err := checkLanguages(ErrCorrupt, m.Files, m.Languages); err != nil {
		return Manifest{}, err
	}
	if _, listed := m.Files[m.Cases]; m.Cases != "" && !listed {
		return Manifest{}, fmt.Errorf("%w: the case index %s is no file of the manifest", ErrCorrupt, m.Cases)
	}
	if err := m.checkSource(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// checkSource returns an error that wraps [ErrCorrupt] for a repository of m that is not valid,
// for a commit that is not a commit of git, for a definition with a commit or Local, and for a
// copy without exactly one of a commit and Local. It returns nil for every other m.
func (m *Manifest) checkSource() error {
	switch {
	case m.Repo != "" && !m.Repo.Valid():
		return fmt.Errorf("%w: %q is no repository of GitHub", ErrCorrupt, m.Repo)
	case m.Commit != "" && !commitPattern.MatchString(m.Commit):
		return fmt.Errorf("%w: %q is no commit of git", ErrCorrupt, m.Commit)
	case m.Repo == "" && (m.Commit != "" || m.Local):
		return fmt.Errorf("%w: the manifest of a definition states a commit or a local source", ErrCorrupt)
	case m.Repo != "" && (m.Commit != "") == m.Local:
		return fmt.Errorf("%w: the manifest of a copy states a commit or a local source, and not both", ErrCorrupt)
	}
	return nil
}

// checkVersion returns an error that wraps sentinel for an empty version and for a version with
// white space at an end, and nil for every other version.
func checkVersion(sentinel error, version string) error {
	if version == "" || strings.TrimSpace(version) != version {
		return fmt.Errorf("%w: the version %q is empty or has white space at an end", sentinel, version)
	}
	return nil
}

// ValidPath reports whether p is a slash-separated path below the root of a definition, which a
// manifest can list: a path that [fs.ValidPath] accepts, other than the root itself, without a
// backslash and without NUL.
func ValidPath(p string) bool {
	return p != "." && fs.ValidPath(p) && !strings.ContainsAny(p, forbidden)
}

// PortablePath reports whether Linux, macOS and Windows can each store a file at p. p is portable
// when [ValidPath] accepts it and Windows accepts each of its names. Windows refuses a name with a
// control character or one of the characters <>:"|?*, and a name that ends with a space or a dot.
// It also refuses a name whose part before its first dot, without its trailing spaces, is the name
// of a device, such as con.json, COM1 or conout$.log.
func PortablePath(p string) bool {
	if !ValidPath(p) {
		return false
	}
	for name := range strings.SplitSeq(p, "/") {
		if strings.ContainsFunc(name, func(r rune) bool { return r < ' ' }) || strings.ContainsAny(name, unportable) {
			return false
		}
		if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
			return false
		}
		base, _, _ := strings.Cut(name, ".")
		if reserved[strings.ToUpper(strings.TrimRight(base, " "))] {
			return false
		}
	}
	return true
}

// Fold returns p with each rune replaced by the smallest rune of its case orbit, the runes that
// unicode.SimpleFold steps through. Two paths have the same Fold exactly when strings.EqualFold
// reports true for them. Windows and macOS store two such paths as one file by default.
func Fold(p string) string {
	var b strings.Builder
	for _, r := range p {
		// SimpleFold steps to the next larger rune of the orbit, and from its largest rune back to
		// the smallest one.
		next := unicode.SimpleFold(r)
		for next > r {
			r, next = next, unicode.SimpleFold(next)
		}
		b.WriteRune(next)
	}
	return b.String()
}

// checkPath returns an error that wraps sentinel for a p that [PortablePath] refuses, and nil for
// every other p.
func checkPath(sentinel error, p string) error {
	if !PortablePath(p) {
		return fmt.Errorf("%w: %q is no portable path of a file of a definition", sentinel, p)
	}
	return nil
}

// checkCollisions returns an error that wraps sentinel for two paths of files that differ only in
// case, also in the name of a directory, and for the path of a file that is also the path of a
// directory. It returns nil for every other files.
func checkCollisions(sentinel error, files map[string]string) error {
	// node is a path of a file or of a directory, by its Fold.
	type node struct {
		path string
		dir  bool
	}
	seen := make(map[string]node, len(files))
	for _, p := range slices.Sorted(maps.Keys(files)) {
		for q, dir := p, false; q != "."; q, dir = path.Dir(q), true {
			key := Fold(q)
			n, ok := seen[key]
			switch {
			case !ok:
				seen[key] = node{path: q, dir: dir}
			case n.path != q:
				return fmt.Errorf("%w: %s and %s differ only in case", sentinel, n.path, q)
			case n.dir != dir:
				return fmt.Errorf("%w: %s is a file and a directory", sentinel, q)
			}
		}
	}
	return nil
}

// checkLanguages returns an error that wraps sentinel when languages break a rule over files: a
// language that is not valid, [spec.All], a list of files that is nil, a file that files lacks,
// and a file that two languages list or that one language lists twice. It checks the languages in
// the order of their names, and returns nil when languages follow every rule.
func checkLanguages(sentinel error, files map[string]string, languages map[spec.Language][]string) error {
	owners := map[string]spec.Language{}
	for _, l := range slices.Sorted(maps.Keys(languages)) {
		if !l.Valid() || l == spec.All {
			return fmt.Errorf("%w: %q is no language of a definition", sentinel, l)
		}
		paths := languages[l]
		if paths == nil {
			return fmt.Errorf("%w: the files of %s are null", sentinel, l)
		}
		for _, p := range paths {
			if _, ok := files[p]; !ok {
				return fmt.Errorf("%w: %s lists %s, which is no file of the manifest", sentinel, l, p)
			}
			if owner, ok := owners[p]; ok {
				return fmt.Errorf("%w: %s and %s both list %s", sentinel, owner, l, p)
			}
			owners[p] = l
		}
	}
	return nil
}
