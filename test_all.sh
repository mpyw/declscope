#!/usr/bin/env bash

set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Every tool is pinned in mise.toml and reached through the PATH mise sets, so
# local and CI run the same binaries. `go run <tool>@<version>` did that too,
# but relinked golangci-lint on every run: 31 seconds against 0 once installed.

# Track results
declare -a failed_tests=()

run_test() {
    local name="$1"
    shift

    echo "=== $name ==="
    "$@"
    local status=$?
    if [ $status -eq 0 ]; then
        echo -e "${GREEN}[$name] OK${NC}"
    else
        echo -e "${RED}[$name] FAILED${NC}"
        failed_tests+=("$name")
    fi
    return $status
}

echo ""

# Run all tests. coverage.sh runs them once, with the subcommand binary built
# with -cover, and prints the statement coverage of both profiles merged.
run_test "analyzer" \
    env GOTESTFLAGS=-v ./coverage.sh

## go.mod's toolchain and mise.toml's go say the same thing in two places,
## which is the cost of pinning Go with mise. golangci-lint refuses to load a
## module whose go directive is newer than the Go it was built with, so a drift
## here surfaces as an unrelated-looking lint failure. Check it first instead.
run_test "toolchain" \
    bash -c '
      mod=$(sed -n "s/^toolchain go//p" go.mod)
      mise=$(sed -n "s/^go = \"\(.*\)\"/\1/p" mise.toml)
      if [ "$mod" != "$mise" ]; then
        echo "go.mod toolchain=$mod but mise.toml go=$mise" >&2
        exit 1
      fi
      echo "toolchain $mod"
    '

run_test "lint" \
    golangci-lint run ./...

# declscope is subject to its own rules, at the strictest setting. Silence is
# the assertion: every namespace crossing inside the tool is stated in the
# source, so anything printed here is a boundary nobody wrote down.
run_test "dogfood" \
    go run ./cmd/declscope -config .declscope-strict.yaml ./...

# The formal specs. Skipped with a notice when fslc is not installed, so that a
# contributor without it is not blocked; CI installs it.
run_test "spec" \
    ./spec/verify.sh

# Summary
echo ""
echo "===== Summary ====="
if [ ${#failed_tests[@]} -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Failed tests:${NC}"
    for test in "${failed_tests[@]}"; do
        echo -e "  ${RED}- $test${NC}"
    done
    exit 1
fi
