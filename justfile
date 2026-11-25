# umbra development commands. Run `just` to list them.

set positional-arguments

# The directory of every Go module that git tracks
modules := replace(`git ls-files '*go.mod' | xargs -n1 dirname`, "\n", " ")

# List available recipes
default:
    @just --list --unsorted

# Check the tools, and install the pre-commit hooks
setup:
    @command -v go >/dev/null || { echo "go not found, install it with 'brew install go'"; exit 1; }
    @command -v golangci-lint >/dev/null || { echo "golangci-lint not found, install it with 'brew install golangci-lint'"; exit 1; }
    @command -v zenith >/dev/null || { echo "zenith not found, install it with 'brew install feds01/tap/zenith'"; exit 1; }
    @command -v pre-commit >/dev/null || { echo "pre-commit not found, install it with 'brew install pre-commit' or 'pipx install pre-commit'"; exit 1; }
    pre-commit install

# Build every module
build:
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && go build ./...)
    done

# Run all tests, extra args go to `go test`, e.g. `just test -run Check`
test *args:
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && go test ./... "$@")
    done

# Run the tests against PostgreSQL in Docker, which is removed afterwards
integration:
    #!/usr/bin/env bash
    set -euo pipefail
    container="$(docker run --detach --rm --publish 127.0.0.1::5432 --env POSTGRES_HOST_AUTH_METHOD=trust postgres:17)"
    trap 'docker stop "$container" >/dev/null' EXIT
    until docker exec "$container" pg_isready --quiet; do
        sleep 0.5
    done
    port="$(docker port "$container" 5432/tcp | head -1 | sed 's/.*://')"
    export UMBRA_TEST_DATABASE_URL="postgres://postgres@127.0.0.1:$port/postgres?sslmode=disable"
    for module in {{modules}}; do
        (cd "$module" && go test -count=1 ./...)
    done

# Fuzz the tag grammar
fuzz time="30s":
    go test -run='^$' -fuzz=FuzzGrammar -fuzztime={{time}} .

# Rewrite the golden files from the current output; review them with `git diff`
update-golden:
    go test ./postgres ./bigquery -update

# Run golangci-lint, formatters included
golangci:
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && golangci-lint run ./...)
    done

# Format all code
fmt:
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && golangci-lint fmt ./...)
    done

# Check umbra's own contracts, in zenith.toml
architecture:
    zenith check

# Run golangci-lint and umbra's own contracts
lint: golangci architecture

# Tidy the go.mod of every module
tidy:
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && go mod tidy)
    done

# Run the tests and the linters
ci: test lint

# Serve the package docs as pkg.go.dev shows them
docs:
    go run golang.org/x/pkgsite/cmd/pkgsite@latest -open .

# Remove build artifacts
clean:
    rm -rf dist .bin
