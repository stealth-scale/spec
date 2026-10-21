// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package spec

import (
	"regexp"
	"strings"
)

// repoName matches a repository of GitHub as owner/name. The owner has one to 39 ASCII letters,
// digits and dashes, and starts with a letter or a digit. The name has one to 100 ASCII letters,
// digits, dots, dashes and underscores.
var repoName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9._-]{1,100}$`)

// The names that GitHub refuses as the name of a repository, although repoName matches them.
const (
	// currentName is the name of the current directory.
	currentName = "."

	// parentName is the name of the parent directory.
	parentName = ".."
)

// Repo is a repository of GitHub as owner/name, such as dokimasia/assert-spec.
//
// The zero value is no repository, and [Repo.Valid] reports false for it.
type Repo string

// Valid reports whether r is owner/name. The owner has one to 39 ASCII letters, digits and dashes,
// and starts with a letter or a digit. The name has one to 100 ASCII letters, digits, dots, dashes
// and underscores, and is neither . nor ..
func (r Repo) Valid() bool {
	_, name, _ := strings.Cut(string(r), "/")
	return repoName.MatchString(string(r)) && name != currentName && name != parentName
}

// Name returns the name of r after its slash, such as assert-spec, and r itself when r has no
// slash.
func (r Repo) Name() string {
	_, name, found := strings.Cut(string(r), "/")
	if !found {
		return string(r)
	}
	return name
}
