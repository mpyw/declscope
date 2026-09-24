#!/usr/bin/env bash
#
# coverage.sh runs the tests and prints what they cover, per package.
#
# Two profiles are merged. `go test` counts what runs in its own process. The
# subcommand tests drive a binary built with -cover, which writes its counters
# to DECLSCOPE_COVERDIR instead (see cmd/declscope/cover_test.go). Read alone,
# the first profile reports cmd/declscope as almost untested.
#
# It writes coverage.txt (go test), coverage-bin.txt (the binary) and
# coverage-merged.txt (both). GOTESTFLAGS adds flags to `go test`, as CI does
# with -race. The exit status is go test's.

set -euo pipefail

cd "$(dirname "$0")"

# The binary's raw counters are only read by covdata below, so they live in a
# temporary directory, removed however the script ends. mktemp names it by an
# absolute path, which cover_test.go needs.
covdir="$(mktemp -d)"
trap 'rm -rf "$covdir"' EXIT

# -count=1 turns off go test's result cache. A cached package runs no test, so
# the subcommand tests start no binary, the counter directory stays empty, and
# cmd/declscope reads as about 1% covered. The cache also cannot help here: the
# counter directory is new on every run.
#
# shellcheck disable=SC2086 # GOTESTFLAGS is a list of flags
DECLSCOPE_COVERDIR="$covdir" go test -count=1 ${GOTESTFLAGS:-} \
    -covermode=atomic -coverpkg=./... -coverprofile=coverage.txt ./...
go tool covdata textfmt -i="$covdir" -o=coverage-bin.txt

# A block is listed once per test binary that links it, so the counts are
# summed per block. A block counts as covered when any run reached it.
{
    echo "mode: atomic"
    tail -q -n +2 coverage.txt coverage-bin.txt |
        awk '{ count[$1 " " $2] += $3 } END { for (k in count) print k, count[k] }' |
        sort
} > coverage-merged.txt

echo ""
echo "Statement coverage, both profiles merged:"
awk '
    NR == 1 { next }
    {
        split($1, loc, ":")
        pkg = loc[1]
        sub(/\/[^\/]*$/, "", pkg)
        all[pkg] += $2
        total += $2
        if ($3 > 0) { hit[pkg] += $2; covered += $2 }
    }
    END {
        for (p in all) printf "  %-45s %6.1f%%  (%d/%d)\n", p, 100 * hit[p] / all[p], hit[p], all[p] | "sort"
        close("sort")
        printf "  %-45s %6.1f%%  (%d/%d)\n", "total", 100 * covered / total, covered, total
    }
' coverage-merged.txt
