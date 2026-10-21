// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package spec defines the names that every package of the module shares: the configuration file
// of a repository, the languages of a definition and the repositories of GitHub.
//
// A definition is the data of a specification repository, such as a catalogue, a corpus of cases
// and an overlay for each language. An implementation is a repository that implements the
// definition in one or more languages and vendors a copy of the files of those languages. Both
// keep their settings in [ConfigFile] at the root of their repository.
//
// # Names
//
//   - [Language] is the name of a language of a definition, such as go. [All] is the language of
//     an implementation of every language of its definition.
//   - [Repo] is a repository of GitHub as owner/name, such as dokimasia/assert-spec.
//
// Each type reports with Valid whether a value follows its rule.
//
// # Dependency position
//
// Imports regexp and strings from the standard library.
package spec
