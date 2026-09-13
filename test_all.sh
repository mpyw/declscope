#!/usr/bin/env bash

set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Pinned to the version CI uses. golangci-lint refuses to load a module whose
# toolchain directive is newer than the Go it was built with, so a golangci-lint
# on PATH is often too old; going through `go run` keeps local and CI identical.
GOLANGCI_LINT="github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1"

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

# Run all tests
run_test "analyzer" \
    go test -v ./...

run_test "lint" \
    go run "$GOLANGCI_LINT" run ./...

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
