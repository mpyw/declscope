#!/usr/bin/env bash
# Verify every FSL spec. Two of them are expected to fail: they model unsound
# designs and exist to hold the counterexample. See spec/README.md.
#
# fslc is the FSL verifier: https://github.com/ymm-oss/fsl
set -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

if ! command -v fslc > /dev/null 2>&1; then
  echo "fslc not found; skipping the specs. Install: https://github.com/ymm-oss/fsl" >&2
  exit 0
fi

## These must verify, and must be inductive rather than true only to a depth.
proving=(boundary_fix directive_effect knobs label_rules)
## These must NOT verify. A green run here means the counterexample was lost.
failing=(rename_sound rename_siblings)

status=0
for f in "${proving[@]}"; do
  if fslc verify "$f.fsl" --engine induction > /dev/null 2>&1; then
    echo "  ok       $f.fsl (proved)"
  else
    echo "  FAILED   $f.fsl is not proved" >&2
    status=1
  fi
done
for f in "${failing[@]}"; do
  if fslc verify "$f.fsl" --depth 8 > /dev/null 2>&1; then
    echo "  FAILED   $f.fsl verified, but it models an unsound design" >&2
    status=1
  else
    echo "  ok       $f.fsl (violated, as intended)"
  fi
done
exit $status
