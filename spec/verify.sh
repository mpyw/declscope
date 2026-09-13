#!/usr/bin/env bash
# Verify every FSL spec. Two of them are expected to fail: they model unsound
# designs and exist to hold the counterexample. See spec/README.md.
#
# Every verdict is read from the JSON `result`, never from the exit code. fslc
# exits non-zero for a parse error and an internal error as well as for a
# violation, so an exit-code test would let a spec rot into unparseable text and
# still call it "violated, as intended" — the two files holding the rename
# counterexamples could stop being FSL at all and CI would stay green.
#
# The failing pair is also pinned to the invariant it must break. A spec that
# fails for some new, unrelated reason has stopped holding the counterexample it
# was written for.
#
# fslc is the FSL verifier: https://github.com/ymm-oss/fsl
set -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

if ! command -v fslc > /dev/null 2>&1; then
  echo "fslc not found; skipping the specs. Install: https://github.com/ymm-oss/fsl" >&2
  exit 0
fi

## Reads one top-level string field out of fslc's JSON, without needing jq.
field() {
  sed -n "s/.*\"$2\"[[:space:]]*:[[:space:]]*\"\([A-Za-z_]*\)\".*/\1/p" <<< "$1" | head -n 1
}

## These must verify, and must be inductive rather than true only to a depth.
proving=(boundary_fix directive_effect knobs label_rules rename_guarded)
## These must NOT verify, and must break on the named invariant.
failing_specs=(rename_sound rename_siblings)
failing_invariants=(RenameKeepsReferenceOnTarget NamesStayDistinct)

status=0

## knobs.fsl and boundary_fix.fsl share the scope model verbatim. The copy is by
## hand, and they are the pair most likely to be edited separately, so a one-
## sided edit would leave the two specs proving things about different systems
## while every verdict below stayed green. Comments and blank lines are ignored;
## the code is not.
shared() {
  sed -n '/def reachableOutside/,/def boundaryFires/p' "$1" | grep -v '^[[:space:]]*//' | grep -v '^[[:space:]]*$'
}
if ! diff <(shared knobs.fsl) <(shared boundary_fix.fsl) > /dev/null; then
  echo "  FAILED   knobs.fsl and boundary_fix.fsl no longer share one scope model" >&2
  diff <(shared knobs.fsl) <(shared boundary_fix.fsl) >&2
  status=1
else
  echo "  ok       knobs.fsl and boundary_fix.fsl share one scope model"
fi

for f in "${proving[@]}"; do
  out=$(fslc verify "$f.fsl" --engine induction 2>&1)
  if [[ "$(field "$out" result)" == "proved" ]]; then
    echo "  ok       $f.fsl (proved)"
  else
    echo "  FAILED   $f.fsl is $(field "$out" result), want proved" >&2
    status=1
  fi
done
for i in "${!failing_specs[@]}"; do
  f=${failing_specs[$i]}
  want=${failing_invariants[$i]}
  out=$(fslc verify "$f.fsl" --depth 8 2>&1)
  case "$(field "$out" result)" in
    violated)
      if [[ "$(field "$out" invariant)" == "$want" ]]; then
        echo "  ok       $f.fsl (violated on $want, as intended)"
      else
        echo "  FAILED   $f.fsl broke on $(field "$out" invariant), want $want" >&2
        status=1
      fi
      ;;
    verified | proved)
      echo "  FAILED   $f.fsl verified, but it models an unsound design" >&2
      status=1
      ;;
    *)
      echo "  FAILED   $f.fsl did not run: $(field "$out" result)" >&2
      status=1
      ;;
  esac
done
exit $status
