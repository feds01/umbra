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

# Let the modules build against each other's checkout. go.work stays out of git, as a
# tagged module can't depend on a directory, and is rewritten for the modules git tracks.
[private]
workspace:
    @rm -f go.work
    @go work init {{modules}}

# Install the umbra command from this checkout into GOBIN
install: workspace
    cd cmd/umbra && go install .

# Remove the installed umbra command
uninstall:
    rm -f "$(go env GOBIN)/umbra" "$(go env GOPATH)/bin/umbra"

# Build the command from this checkout and run it in the current directory, e.g. `just run version`
[no-cd]
run *args: workspace
    GOWORK="{{justfile_directory()}}/go.work" go build -o "{{justfile_directory()}}/.bin/umbra" github.com/feds01/umbra/cmd/umbra
    "{{justfile_directory()}}/.bin/umbra" "$@"

# Build every module
build: workspace
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && go build ./...)
    done

# Run all tests, extra args go to `go test`, e.g. `just test -run Check`
test *args: workspace
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && go test ./... "$@")
    done

# Run the tests against PostgreSQL in Docker, which is removed afterwards
integration: workspace
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
golangci: workspace
    #!/usr/bin/env sh
    set -eu
    for module in {{modules}}; do
        (cd "$module" && golangci-lint run ./...)
    done

# Format all code
fmt: workspace
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
