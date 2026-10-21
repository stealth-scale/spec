// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package spec

import "regexp"

// ConfigFile is the name of the configuration file at the root of a definition repository and of
// an implementation repository. Its section definition configures a definition, and its section
// implements configures an implementation.
const ConfigFile = ".spec.yaml"

// languageName matches the name of a language: words of lower-case ASCII letters and digits that
// single dashes join, the first word starting with a letter.
var languageName = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Language is the name of a language of a definition, such as go or typescript.
//
// The zero value is no language, and [Language.Valid] reports false for it.
type Language string

// All is the language of an implementation of every language of its definition. A list of
// languages that contains All does not contain another language. A definition does not declare
// All.
const All Language = "all"

// Valid reports whether l consists of words of lower-case ASCII letters and digits that single
// dashes join, the first word starting with a letter, such as go or objective-c.
func (l Language) Valid() bool {
	return languageName.MatchString(string(l))
}
