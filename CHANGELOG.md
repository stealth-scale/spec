# go.stealthscale.io/spec

## 0.2.0

### Minor Changes

- 1a079fb: Let a copy share its directory with the other files of the implementation.

## 0.1.0

### Minor Changes

- 94e4e35: Add `spec check` to check that each copy of an implementation is intact and comes from its track.
- c47b8fa: Add the reusable workflow `propose.yml` and the action `setup-spec` for the CI of an implementation.
- 94e4e35: Add `spec manifest` to write the manifest of a definition with the files of each language.
- 94e4e35: Add `spec propose` to open, update and close the pull requests that sync the copies of an implementation.
- 94e4e35: Add `spec status` to report the copy of a definition in each of its implementations.
- 94e4e35: Add `spec sync` to write the vendored copy of each definition of an implementation.
- 94e4e35: Add `spec try` to try the working tree of a definition in the checkouts of its implementations.
- 94e4e35: Add `spec verify` to check the report of a runner against the case index of its copy.
- 9ed90a1: Publish the binary `spec` for Linux, macOS and Windows in each GitHub release.
