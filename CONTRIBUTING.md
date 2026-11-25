# Contributing to umbra

## Development

umbra is a Go library with nothing but the standard library behind it, and
uses [just](https://github.com/casey/just):

```sh
just setup         # check the tools, and install the pre-commit hooks
just test          # all tests, in every module
just lint          # golangci-lint, and umbra's own architecture contracts
just run version   # build the command from source and run it here, e.g. `just run check --config cmd/umbra/testdata/umbra.toml`
just integration   # the tests that read a real PostgreSQL, in Docker
just fuzz          # fuzz the tag grammar
just update-golden # rewrite the golden files, the command's too; review them with `git diff`
```

The repository holds three Go modules: the library at the root, the `umbra`
command in `cmd/umbra`, which has dependencies of its own, and the
`integration` tests. A `go.work`, which git ignores and the recipes create,
builds them against each other's checkout, because a tagged module can't
depend on a directory.

Run `just lint` before committing; the pre-commit hooks run the same checks on
the files you commit. CI runs the tests on Linux, macOS and Windows, and on the
oldest Go that `go.mod` names; the linters, the architecture contracts,
`govulncheck` on the command and a fuzz run on Linux; the tests against
PostgreSQL; and a build of the release archives.

See [AGENTS.md](AGENTS.md) for the architecture and conventions.

## The user guide

The user guide is an [mdBook](https://rust-lang.github.io/mdBook/) in
[docs](docs/introduction.md). `just book` serves it, rebuilt on every change; it
needs mdBook 0.4.40, the version that CI builds it with. Every change to it on
`main` is published to [GitHub Pages](https://feds01.github.io/umbra/). When a
change alters what users see, update the guide in the same change.

## Pull requests

A pull request's title is its entry in the changelog, so write it as a
conventional commit, as in `feat(cli): …`. Features, fixes, performance and
documentation are listed; other types are not. Pull requests are merged with a
merge commit, not squashed, because the changelog is written from the merges.

## Releasing

Run `just changelog <version>`, which writes [CHANGELOG.md](CHANGELOG.md) with
[git-cliff](https://git-cliff.org) from the titles of the pull requests merged
since the last release (see [cliff.toml](cliff.toml)). Merge that to `main`,
then run `just release <version>` on it. It tags the library, `v<version>`, and
then the command, `cmd/umbra/v<version>`, on a commit that requires that
release, so that `go install` works.

The `v<version>` tag starts the release workflow. It builds the archives for
macOS and Linux, publishes the GitHub release with the release's section of the
changelog as its notes and an attestation of each archive, has the Go module
proxy fetch the version, and pushes the Homebrew formula to
[feds01/homebrew-tap](https://github.com/feds01/homebrew-tap). A pre-release,
such as `0.1.0-rc.1`, gets only the GitHub release. The formula is pushed with
`HOMEBREW_TAP_TOKEN`, which expires: renew it before it does. A failed job can
be re-run, and the formula is skipped when the tap already has it. If
`just release` pushed `v<version>` but not the command's tag, run
`just release-command <version>` to finish it. A tag a Go
module proxy has fetched is permanent, so a bad release is followed by the next
patch version, not re-tagged.
