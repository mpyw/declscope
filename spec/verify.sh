#!/usr/bin/env bash
# Verify every FSL spec. Each must be proved. See spec/README.md.
#
# Every verdict is read from the JSON `result`, never from the exit code. fslc
# exits non-zero for a parse error and an internal error as well as for a
# violation, so the exit code alone cannot tell them apart.
#
# fslc is the FSL verifier: https://github.com/ymm-oss/fsl
set -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

if ! command -v fslc > /dev/null 2>&1; then
  echo "fslc not found; skipping the specs. Install it with: mise install (mise.toml pins its version)" >&2
  exit 0
fi

## Reads one top-level string field out of fslc's JSON, without needing jq.
field() {
  sed -n "s/.*\"$2\"[[:space:]]*:[[:space:]]*\"\([A-Za-z_]*\)\".*/\1/p" <<< "$1" | head -n 1
}

## These must verify, and must be inductive rather than true only to a depth.
proving=(boundary_fix boundary_off config_inherit filter filter_chain fix_members knobs naming_rules rename_guarded rename_reach rename_reserved rename_selections shrink shrink_claim shrink_ignore shrink_settle surplus_strict type_fix_members unused_ignore unused_ignore_unseen unused_modes)

status=0

## Every spec in the directory must be named above. The list is hand-written,
## so a file added — or renamed — would otherwise never run, and the gate would
## stay green over a spec nothing checks.
for f in *.fsl; do
  name=${f%.fsl}
  if [[ " ${proving[*]} " != *" $name "* ]]; then
    echo "  FAILED   $f is not named above, so nothing verifies it" >&2
    status=1
  fi
done

## knobs.fsl and boundary_fix.fsl share the scope model verbatim. The copy is by
## hand, and they are the pair most likely to be edited separately, so a one-
## sided edit would leave the two specs proving things about different systems
## while every verdict below stayed green. Comments and blank lines are ignored;
## the code is not.
shared() {
  {
    sed -n '/^  enum Scope/,/^  enum ReferencedFrom/p' "$1"
    sed -n '/def defaultScope/,/def boundaryFires/p' "$1"
  } | grep -v '^[[:space:]]*//' | grep -v '^[[:space:]]*$'
}
if ! diff <(shared knobs.fsl) <(shared boundary_fix.fsl) > /dev/null; then
  echo "  FAILED   knobs.fsl and boundary_fix.fsl no longer share one scope model" >&2
  diff <(shared knobs.fsl) <(shared boundary_fix.fsl) >&2
  status=1
else
  echo "  ok       knobs.fsl and boundary_fix.fsl share one scope model"
fi

## A leadsTo without a ranking measure is checked only to --depth, and the
## spec still comes back proved. So each one must be proved by its decreases,
## or a measure deleted or never written would leave a bounded check behind a
## green verdict.
for f in "${proving[@]}"; do
  out=$(fslc verify "$f.fsl" --engine induction 2>&1)
  declared=$(grep -c '^[[:space:]]*leadsTo ' "$f.fsl")
  ranked=$(grep -c '"proof"[[:space:]]*:[[:space:]]*"ranking"' <<< "$out")
  if [[ "$(field "$out" result)" != "proved" ]]; then
    echo "  FAILED   $f.fsl is $(field "$out" result), want proved" >&2
    status=1
  elif [[ "$ranked" != "$declared" ]]; then
    echo "  FAILED   $f.fsl proves $ranked of its $declared leadsTo by ranking; the rest are bounded" >&2
    status=1
  else
    echo "  ok       $f.fsl (proved)"
  fi
done
exit $status
