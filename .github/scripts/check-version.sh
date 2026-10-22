#!/usr/bin/env bash
# Checks that the version written in the README's install commands and in the
# distributed skills is the same everywhere. With an argument (vX.Y.Z or
# X.Y.Z), it also checks that it equals that version; the Tag workflow passes
# the version it is about to tag. v0.20.3 was released with the old version
# still written in these places, and v0.20.4 had to follow.
#
# Run from the repository root: .github/scripts/check-version.sh [vX.Y.Z]
set -euo pipefail

want="${1:-}"
want="${want#v}"

# file|literal text before the version|literal text after it. A leading ^
# anchors the text before at the start of a line, so VERSION= does not also
# match GO_VERSION=.
places=(
  'README.md|mise use "github:mpyw/declscope@|"'
  'README.md|"github:mpyw/declscope" = "|"'
  'README.md|^VERSION=|'
  'skills/declscope-authoring/SKILL.md|Written against **declscope |**'
  'skills/declscope-adoption/SKILL.md|Written against **declscope |**'
)

semver='[0-9]+\.[0-9]+\.[0-9]+'
# Escapes a literal for use in an extended regular expression. / is not
# special there, and GNU grep 3.8 and later warns about \/.
quote() { printf '%s' "$1" | sed 's/[][\.*^$+?(){}|]/\\&/g'; }

failed=0
seen=""
for place in "${places[@]}"; do
  IFS='|' read -r file before after <<<"$place"
  anchor=""
  if [[ "$before" == ^* ]]; then
    anchor="^"
    before="${before#^}"
  fi
  label="$file: ${anchor}${before}X.Y.Z${after}"
  pattern="$anchor$(quote "$before")($semver)$(quote "$after")"
  # Each match is the literals around the version, which are stripped as
  # literals: a regular expression would let .* eat the leading digits of a
  # two-digit major.
  found="$(grep -oE "$pattern" "$file" | while IFS= read -r match; do
    match="${match#"$before"}"
    printf '%s\n' "${match%"$after"}"
  done | sort -u || true)"
  if [[ -z "$found" ]]; then
    echo "Error: $label: not found" >&2
    failed=1
    continue
  fi
  if [[ "$(wc -l <<<"$found")" -gt 1 ]]; then
    echo "Error: $label: several versions: $(tr '\n' ' ' <<<"$found")" >&2
    failed=1
    continue
  fi
  if [[ -n "$want" && "$found" != "$want" ]]; then
    echo "Error: $label: says $found, but the version being tagged is $want" >&2
    failed=1
  elif [[ -z "$want" && -n "$seen" && "$found" != "$seen" ]]; then
    echo "Error: $label: says $found, but an earlier place says $seen" >&2
    failed=1
  fi
  seen="${seen:-$found}"
done

if [[ "$failed" -ne 0 ]]; then
  echo "Bump every version string above in a commit before tagging." >&2
  exit 1
fi
echo "Version strings agree: ${want:-$seen}"
