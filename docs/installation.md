# Installation

## The library

```sh
go get github.com/feds01/umbra
```

Go 1.24 or later. The library imports nothing outside the standard library.

## The command

With [Homebrew](https://brew.sh):

```sh
brew install feds01/tap/umbra
```

With Go:

```sh
go install github.com/feds01/umbra/cmd/umbra@latest
```

Or download an archive for macOS or Linux, on amd64 or arm64, from the
[releases](https://github.com/feds01/umbra/releases). Each one is attested, so you
can check where it was built:

```sh
gh attestation verify umbra-0.1.0-linux-amd64.tar.gz --repo feds01/umbra
```

Check the install with `umbra version`.
