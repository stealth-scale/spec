<!--
  ~ Copyright Stealth Scale B.V. 2026
  ~ SPDX-License-Identifier: MIT
-->

# Packages

The module `go.stealthscale.io/spec` has one package per concept. A package imports only the
packages above it in this table, so the table is also the order in which the packages build on each
other.

| Package | Contains | Imports from the module |
|---|---|---|
| `spec` | The name of a repository, the name of a language, and the name of the configuration file | none |
| `manifest` | The manifest of a definition and of a copy: the digest of each file, the files of each language, the view of some languages, and the JSON of a manifest | `spec` |
| `definition` | The section `definition` of the configuration file, and the tree of a definition from a checkout or from the files of an archive | `spec`, `manifest` |
| `implementation` | The section `implements` of the configuration file, and the entry of a definition | `spec`, `manifest` |
| `vendored` | The copy of an entry: its build from a tree, its write to a repository, its check, and the move of a copy aside for a trial | `spec`, `definition`, `implementation`, `manifest` |
| `conformance` | The case index of a definition, the report of a runner, and the check of a report against the index | `spec`, `manifest` |
| `github` | The client of the GitHub API, on the client of ergon's forge | `manifest` |
| `remote` | An implementation at the head of its default branch, the commit that an entry follows, and the copy of a commit | `spec`, `definition`, `github`, `implementation`, `vendored` |
| `propose` | The pull requests that sync the copies of an implementation | `spec`, `github`, `implementation`, `manifest`, `remote`, `vendored` |
| `status` | The status of the copy of a definition in each of its implementations | `spec`, `github`, `implementation`, `manifest`, `propose`, `remote`, `vendored` |
| `trial` | The trial of the working tree of a definition in the checkouts of its implementations | `spec`, `definition`, `implementation`, `vendored` |
| `internal/command` | The commands of the program and their exit statuses | every package above |
| `cmd/spec` | The program, with the transport of its requests and the signals that cancel a run | `internal/command` |

## Data

A run reads and writes four kinds of file:

| File | Writer | Reader |
|---|---|---|
| `.spec.yaml` | A person | `definition`, `implementation` |
| The manifest of a definition | `spec manifest` | A person or a tool that compares a definition with its manifest |
| `manifest.json` of a copy | `spec sync`, `spec propose`, `spec try` | `vendored.Check`, `spec verify`, the runner of an implementation |
| The report of a runner | The runner of an implementation | `spec verify` |

The digest of a manifest is the SHA-256 of the sorted lines `path digest`, each with a line feed.
The JSON of a manifest has its keys in the order of their bytes and an indent of four spaces. For
paths of printable ASCII, `json.dumps(indent=4, sort_keys=True)` of Python writes the same bytes.
A definition can check its manifest with tooling of either language.

## Requests to GitHub

`remote`, `propose` and `status` read GitHub through interfaces. The client of `github` implements
them in the program, and fakes in memory implement them in the specs. A sync reads the archive of
a commit in one request. `propose` also reads the blobs of the default branch in one request.

GitHub keeps a pull request open while a sync updates it, because `release.Propose` of ergon
commits on a staging branch and moves the branch of the pull request in one update.
