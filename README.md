<!--
  ~ Copyright Stealth Scale B.V. 2026
  ~ SPDX-License-Identifier: MIT
-->

# spec

spec vendors the conformance definition of a polyglot library into each implementation of the
library. A definition is a repository of the cases that every implementation must pass, such as
[dokimasia/assert-spec](https://github.com/dokimasia/assert-spec). Each implementation keeps a copy
of the files of its languages. spec writes that copy, opens the pull requests that update it, and
checks that the runner of each language reports a result for every case.

## Install

```sh
go install go.stealthscale.io/spec/cmd/spec@latest
```

Each [release](https://github.com/stealth-scale/spec/releases) also has an archive of the binary
for Linux, macOS and Windows on amd64 and arm64, with the completion scripts of bash, zsh and fish.
It also has deb, rpm and apk packages for Linux. `checksums.txt` lists the SHA-256 of each asset,
and cosign signs it with the identity of the release workflow.

## Configure a definition

Declare the section `definition` in the file `.spec.yaml` of the definition repository:

```yaml
definition:
  repo: dokimasia/assert-spec
  languages: [go, rust]
  files: [VERSION, spec/cases.json, "corpus/**/*.json", "overlays/*.json"]
  language-files: ["overlays/{language}.json"]
  cases: spec/cases.json
  implementations:
    - {repo: dokimasia/assert-go, languages: [go]}
    - {repo: dokimasia/assert-rust, languages: [rust]}
```

| Key | Value |
|---|---|
| `repo` | The repository of the definition, as owner/name |
| `languages` | The languages of the definition |
| `files` | The patterns of the files that an implementation vendors, which [doublestar](https://github.com/bmatcuk/doublestar) matches |
| `language-files` | The patterns of the files of one language, with `{language}` for the language. Every language shares each other file |
| `cases` | The case index of the definition, or no key for a definition without one |
| `root` | The directory of the definition in its repository, `.` by default |
| `version` | The file whose content is the version, `VERSION` by default |
| `manifest` | The manifest of the definition, `spec/manifest.json` by default |
| `implementations` | The implementation repositories, each with the languages that it implements |

After each change of the files, run `spec manifest`. The manifest lists the digest of each file
and the files of each language. It also states the version and the digest of the whole set. In CI,
`spec manifest --check` fails when the manifest differs from the files.

The case index lists each case with its file and its id. A case can list the languages that it
applies to, and the languages that skip it with the reason of each:

```json
{"cases": [
  {"file": "corpus/equal.json", "id": "equal/int-against-string", "skip": {"go": "the compiler refuses the mismatch"}},
  {"file": "corpus/files.json", "id": "files/link", "languages": ["go"]}
]}
```

The tooling of the definition writes the index, because only that tooling knows how its corpus
states a case.

## Configure an implementation

Declare the section `implements` in the file `.spec.yaml` of the implementation repository. Each
entry names one definition:

```yaml
implements:
  - definition: dokimasia/assert-spec
    track: main
    path: conformance/spec
    languages: [go]
    copies:
      - {from: spec/zones.json, to: internal/zone/zones.json}
    test: [make, test-go]
```

| Key | Value |
|---|---|
| `definition` | The repository of the definition |
| `track` | The branch of the definition that the copy follows, `main` by default, or `release` for the newest stable release |
| `path` | The directory of the copy |
| `languages` | The languages that the copy vendors, or `[all]` for every language of the definition |
| `copies` | The files of the copy that the repository also needs at another path, such as a file that a Go package embeds |
| `test` | The command that `spec try` runs in the repository |

Run `spec sync` to write the copy. The copy contains the files of its languages at their paths in
the definition, and the file `manifest.json`. That manifest is the lock of the copy: it states the
repository and the commit of the copy, and the digest of each file. A change of a file of another
language leaves the copy as it is.

The directory of a copy can contain other files, such as the source files of the package that
reads the copy. The copy owns the entries of the directory that its files start with, such as
`corpus` and `overlays`, and `manifest.json`. spec writes, checks and moves those entries alone.

`spec sync` refuses to replace a copy whose files differ from its manifest, so it never overwrites
an edit. To replace such a copy, pass `--force`. The flags `--locked`, `--commit SHA` and
`--from DIR` select another source:

| Flag | Source of the copy |
|---|---|
| none | The head of the track of the entry |
| `--locked` | The commit of the manifest of the copy |
| `--commit SHA` | The commit SHA of the definition, for one definition |
| `--from DIR` | The checkout of the definition at DIR, which gives a local copy without a commit |

## Check the copies

| Command | Fails when |
|---|---|
| `spec check` | A copy differs from its manifest, comes from a checkout, or has a copied file that differs from its source |
| `spec check --remote` | The commit of a copy is not on the track of its entry. The run also reports whether the head of the track has a newer copy |
| `spec verify FILE` | The report of a runner lacks a result of a case, fails a case, or skips a case that the index does not skip |

A runner writes its report to the file that the variable `SPEC_RESULTS` names, in JSON lines. The
first line states the repository, the commit and the digest of the copy, and the language of the
runner. Each further line states the result of one case:

```json
{"repo": "dokimasia/assert-spec", "commit": "27b9537f4935f764239f30599adccc6f56c919ce", "digest": "sha256:9b86…", "language": "go"}
{"file": "corpus/equal.json", "id": "equal/null-against-empty-list", "outcome": "pass"}
{"file": "corpus/equal.json", "id": "equal/int-against-string", "outcome": "skip", "reason": "the compiler refuses the mismatch"}
```

A job of CI installs spec with the action `setup-spec` of this repository, checks the copy, runs
the tests with `SPEC_RESULTS`, and verifies the report:

```yaml
- uses: stealth-scale/spec/.github/actions/setup-spec@<commit> # <version>
  with:
    version: <version>
- run: spec check --remote
  env:
    GITHUB_TOKEN: ${{ github.token }}
- run: make test-go
  env:
    SPEC_RESULTS: ${{ runner.temp }}/spec-results.jsonl
- run: spec verify "${{ runner.temp }}/spec-results.jsonl"
```

The action builds spec with the go command of the runner. It adds only the directory of spec to
the `PATH`, so the toolchain of the tests of the job does not change.

## Update the copies

`spec propose --repo OWNER/NAME` compares each copy of the implementation with the head of its
track through the GitHub API, without a checkout:

- A copy that differs gets a pull request from the branch `spec-sync/` and the repository of the
  definition. Its commits write the files that differ, and remove the files in the entries of the
  copy that the copy lacks. A repository with `.changeset/config.json` also gets an empty changeset.
- A copy that matches gets its open pull request closed with a comment.

The body of the pull request states the version and the commit of the copy before and after, the
commits of the definition between the two, and the files that change.

Each implementation calls the reusable workflow `propose.yml` of this repository on a schedule,
from a job that grants `contents: write` and `pull-requests: write`:

```yaml
name: Spec

on:
  schedule:
    - cron: "0 6 * * 1-5"
  workflow_dispatch:

permissions: {}

jobs:
  propose:
    permissions:
      contents: write
      pull-requests: write
    uses: stealth-scale/spec/.github/workflows/propose.yml@<commit> # <version>
    with:
      version: <version>
```

The workflow runs `spec propose` with the token of the calling job. GitHub holds the workflow runs
of a pull request that this token opens or updates until a person with write access approves
them. To start the checks at once, run `spec propose` in a job of your own with the token of a
GitHub App in `GITHUB_TOKEN`.

In the definition repository, `spec propose --all` proposes to every implementation of the section
`definition`, with your own token in `GITHUB_TOKEN`. `spec status` writes one row per
implementation:

```text
IMPLEMENTATION       VERSION  COMMIT   NEWER  PULL  CHECKS
dokimasia/assert-go  9.0.0    27b9537  yes    #12   success
```

`NEWER` states whether the head of the track has a copy with another digest. `CHECKS` is the state
of the check runs of the open pull request, or of the default branch without one.

## Try a change of the definition

`spec try` writes the working tree of the definition into the checkout of each implementation
beside the repository, such as `../assert-go`, and runs the command `test` of its entry there. A
trial does not use the network. `spec try --restore` moves the copy from before the trial back
into each checkout.

## Exit statuses

| Status | Run |
|---|---|
| 0 | The run succeeds |
| 1 | A check fails, or the run fails with an error without another status |
| 2 | The command line or a configuration file is invalid, or `spec propose` has no token |
| 3 | A request to GitHub fails |

The flag `-C DIR` runs a command in the repository at DIR. The variables `GITHUB_API_URL` and
`GITHUB_GRAPHQL_URL` select another host of GitHub, as GitHub Actions sets them.

`spec sync`, `spec check --remote` and `spec status` read public repositories without a token.
GitHub allows a client without a token 60 requests per hour from one address. Set `GITHUB_TOKEN`
for a private repository or for more requests. `spec propose` needs a token.

## Platforms

spec runs on Linux, macOS and Windows. A copy has the same bytes on each system when the
repositories follow two rules:

- A manifest lists only paths that all three systems can store. `spec manifest` refuses a name with
  one of the characters `<>:"|?*`, a name that ends with a dot or a space, and the name of a device
  of Windows, such as `con.json`. It also refuses two paths that differ only in case, because
  Windows and macOS store them as one file.
- A digest covers the bytes of a file, so a checkout must keep the line endings of the commit. The
  installer of Git for Windows sets `core.autocrlf` to `true`, which checks text files out with CRLF
  line endings. Set this attribute in the `.gitattributes` of each repository, as ergon's baseline
  does:

  ```gitattributes
  * text=auto eol=lf
  ```

## Development

`make check` runs the gate of the repository: the formatters, the linters and the tests of every
language. [CONTRIBUTING.md](CONTRIBUTING.md) describes how to propose a change.
[docs/architecture/packages.md](docs/architecture/packages.md) describes the packages of the module.

## License

spec is licensed under MIT. See [LICENSE](LICENSE).
