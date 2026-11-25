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
    cd cmd/umbra && go test ./... -update

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

# Write CHANGELOG.md with a section for the release `version`, e.g. `just changelog 0.1.0`: the pull requests merged since the last tag, by their titles
changelog version:
    @command -v git-cliff >/dev/null || { echo "git-cliff not found, install it with 'brew install git-cliff'"; exit 1; }
    git cliff --tag "v{{version}}" --output CHANGELOG.md

# Fail unless the argument is a version, like 0.1.0 or 0.1.0-rc.1, which the recipes paste into scripts
[private]
check-version version:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "{{version}}" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$' || { echo "error: '{{version}}' is not a version, like 0.1.0 or 0.1.0-rc.1" >&2; exit 1; }

# Tag and push a release from main: the library at v<version>, then the command at cmd/umbra/v<version>, e.g. `just release 0.1.0`
release version: (check-version version)
    #!/usr/bin/env bash
    set -euo pipefail

    fail() {
        echo "error: $1" >&2
        exit 1
    }

    # Releases are cut from an up-to-date, clean main.
    branch="$(git rev-parse --abbrev-ref HEAD)"
    if [ "$branch" != "main" ]; then
        fail "releases are cut from main (on '$branch')"
    fi
    if [ -n "$(git status --porcelain)" ]; then
        fail "the working tree is not clean"
    fi
    git fetch origin main --tags --quiet
    if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
        fail "main is not in sync with origin/main"
    fi

    # CI has passed on this commit: a version can't be taken back once the Go module
    # proxy has fetched it.
    command -v gh >/dev/null || fail "gh not found, install it with 'brew install gh'"
    sha="$(git rev-parse HEAD)"
    conclusions="$(gh api --paginate "repos/{owner}/{repo}/commits/$sha/check-runs" --jq '.check_runs[] | .conclusion // "pending"')"
    if [ -z "$conclusions" ] || grep -qvE '^(success|skipped)$' <<<"$conclusions"; then
        fail "CI hasn't passed on $sha: see 'gh run list --commit $sha'"
    fi

    # Neither tag exists yet.
    tag="v{{version}}"
    for name in "$tag" "cmd/umbra/$tag"; do
        if git rev-parse --quiet --verify "refs/tags/$name" >/dev/null; then
            fail "tag $name already exists"
        fi
    done

    # A release's notes are its section of the changelog. A pre-release has none.
    if [[ "{{version}}" != *-* ]] && ! grep -q "^## \[{{version}}\]" CHANGELOG.md 2>/dev/null; then
        fail "CHANGELOG.md has no section for {{version}}: run 'just changelog {{version}}' and merge it"
    fi

    read -r -p "Release $tag? [y/N] " answer
    case "$answer" in
        y | Y) ;;
        *)
            echo "aborted"
            exit 1
            ;;
    esac

    git tag -a "$tag" -m "Release $tag"
    git push origin "$tag"
    just release-command "{{version}}"

# Tag the command for `go install github.com/feds01/umbra/cmd/umbra@v<version>`, on a commit that requires the library's release and is on no branch; `just release` runs it, and a failed run can be repeated
release-command version: (check-version version)
    #!/usr/bin/env bash
    set -euo pipefail
    tag="v{{version}}"

    # The commit is made in a worktree of the library's tag, so that main stays as it is.
    worktree="$(mktemp -d)"
    trap 'git worktree remove --force "$worktree"' EXIT
    git worktree add --quiet --detach "$worktree" "$tag"

    # The workspace is off to read the release itself, which GitHub has as soon as
    # the tag is pushed, but the proxy may not have yet.
    cd "$worktree/cmd/umbra"
    export GOWORK=off GOPROXY="${GOPROXY:-direct}"
    go get "github.com/feds01/umbra@$tag"
    go mod tidy
    git commit --all --quiet --message "build: require umbra $tag"
    git tag -a "cmd/umbra/$tag" -m "Release cmd/umbra/$tag"
    git push origin "cmd/umbra/$tag"

# Print the changelog's section for the release `version`, which a release's notes are
notes version: (check-version version)
    #!/usr/bin/env bash
    set -euo pipefail
    section="$(awk '/^## \[/ { printing = index($0, "[{{version}}]") > 0; next } printing' CHANGELOG.md 2>/dev/null || true)"
    if [ -n "$section" ]; then
        echo "$section"
    elif [[ "{{version}}" == *-* ]]; then
        release="{{version}}"
        echo "A pre-release of ${release%%-*}."
    else
        echo "error: CHANGELOG.md has no section for {{version}}" >&2
        exit 1
    fi

# Build the release archives and their checksums into dist/, e.g. `just dist 0.1.0`
dist version: workspace (check-version version)
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf dist
    mkdir dist
    for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
        os="${target%/*}"
        arch="${target#*/}"
        name="umbra-{{version}}-$os-$arch"
        mkdir "dist/$name"
        (cd cmd/umbra && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
            -ldflags "-s -w -X main.version={{version}}" -o "../../dist/$name/umbra" .)
        cp LICENSE README.md "dist/$name"
        tar -C dist -czf "dist/$name.tar.gz" "$name"
        rm -r "dist/$name"
    done
    (cd dist && shasum -a 256 *.tar.gz > checksums.txt)

# Print the Homebrew formula of the release `version`, from the checksums of its archives
[no-cd]
formula version checksums: (check-version version)
    #!/usr/bin/env bash
    set -euo pipefail
    sum() {
        local archive="umbra-{{version}}-$1.tar.gz"
        awk -v archive="$archive" '$2 == archive { print $1; found = 1 } END { exit !found }' "{{checksums}}" \
            || { echo "error: no checksum for $archive in {{checksums}}" >&2; exit 1; }
    }
    # Assignments, unlike a substitution inside the formula below, stop the script when
    # a checksum is missing.
    darwin_arm64="$(sum darwin-arm64)"
    darwin_amd64="$(sum darwin-amd64)"
    linux_arm64="$(sum linux-arm64)"
    linux_amd64="$(sum linux-amd64)"
    url="https://github.com/feds01/umbra/releases/download/v{{version}}/umbra-{{version}}"
    cat <<FORMULA
    class Umbra < Formula
      desc "Classify database columns for personal data and render masked views"
      homepage "https://github.com/feds01/umbra"
      license "MIT"

      on_macos do
        on_arm do
          url "$url-darwin-arm64.tar.gz"
          sha256 "$darwin_arm64"
        end
        on_intel do
          url "$url-darwin-amd64.tar.gz"
          sha256 "$darwin_amd64"
        end
      end

      on_linux do
        on_arm do
          url "$url-linux-arm64.tar.gz"
          sha256 "$linux_arm64"
        end
        on_intel do
          url "$url-linux-amd64.tar.gz"
          sha256 "$linux_amd64"
        end
      end

      def install
        bin.install "umbra"
      end

      test do
        assert_match "umbra #{version}", shell_output("#{bin}/umbra version")
      end
    end
    FORMULA

# Commit the Homebrew formula, which `just formula` wrote to `tap`/Formula/umbra.rb, to the tap checked out at `tap`, and push it, with the token in TAP_TOKEN if that is set (run by the release workflow)
[no-cd]
publish-formula tap version:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{tap}}"
    git config user.name "github-actions[bot]"
    git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

    # A formula that the tap already has is skipped, so that the job can be re-run.
    git add Formula/umbra.rb
    if git diff --cached --quiet; then
        echo "The tap already has umbra {{version}}"
        exit 0
    fi
    git commit --message "umbra {{version}}"

    # The token is given to the push alone, and isn't kept in the checkout.
    header=()
    if [ -n "${TAP_TOKEN:-}" ]; then
        header=(-c "http.https://github.com/.extraheader=AUTHORIZATION: basic $(printf 'x-access-token:%s' "$TAP_TOKEN" | base64 | tr -d '\n')")
    fi
    git ${header[@]+"${header[@]}"} push

# Serve the user guide in docs, rebuilt on every change; needs mdBook 0.4.40
book:
    mdbook serve docs --open

# Remove build artifacts
clean:
    rm -rf dist .bin docs/book
