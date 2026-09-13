# Formal specs

Machine-checked statements about the rules, written in [FSL](https://github.com/ymm-oss/fsl)
and verified with `fslc`. They are documentation that cannot drift: the claims below
are the ones `fslc verify` actually proves, over **every** configuration, not a
sample of them.

They check the **design**, not the implementation. A counterexample here means the
rules contradict each other; confirming that the Go code agrees takes a run against
the binary.

## What is proved

| Spec | Claim | Scope covered |
| --- | --- | --- |
| `label_rules.fsl` | `qualify` and `unqualify` never both fire for one declaration | Every combination of `rules.qualify`, `rules.unqualify`, `rules.exportedLabels`, namespace count, kind, exportedness, namespace presence, core membership and label |
| `label_rules.fsl` | Applying either label fix removes the violation it addresses, and neither is applied where its violation does not exist | As above |
| `label_rules.fsl` | No rename is ever applied to an exported declaration | As above |
| `label_rules.fsl` | Each naming key bites in both directions, and each silence is witnessed with every other reason for silence pinned | As above |
| `label_rules.fsl` | The core namespace is outside both rules, whatever is configured | As above |
| `boundary_fix.fsl` | Inserting `//declscope:package` always removes the boundary crossing, and is withheld wherever any level already stated a scope | Every combination of `defaults.unexported`, kind, exportedness, reference shape, and the declaration, block, type and file directives |
| `boundary_fix.fsl` | The fix is offered only where a crossing exists, and never overwrites the declaration's own directive | As above |
| `knobs.fsl` | `defaults.unexported` and every directive level demonstrably change an outcome, for each kind of declaration | As above |
| `knobs.fsl` | The containing type's directive reaches fields and nothing else — witnessed by a package-level declaration and a method that still fire | As above |
| `knobs.fsl` | An exported declaration carries no boundary by default, for every kind | As above |
| `knobs.fsl` | A directive narrows an exported declaration anyway, including a type's directive reaching an exported field | As above |
| `knobs.fsl` | A reference from inside the namespace never crosses a boundary | As above |
| `directive_effect.fsl` | A scope directive is used when anything in its reach binds to it, and reported when nothing does | Every combination of stated scope, enclosing directive, `defaults.unexported`, target presence and exportedness, and two enclosed declarations by exportedness and shadowing |
| `directive_effect.fsl` | Binding is quantified over configurations *and* over enclosing directives — so restating the default of the day never counts, and `//declscope:package` under an enclosing `private` always does | As above |
| `knobs.fsl` | A boundary is reported only for a private scope and only across a namespace, and on an exported declaration only where a directive narrowed it — each guard witnessed by an invariant its removal breaks | As above |
| `label_rules.fsl` | A fix is eventually applied wherever one is offered, which is what the `fair` on the fix actions claims | As above, plus whether a rename is offered at all |
| `rename_guarded.fsl` | The guard `renameSafe` applies — every scope Go resolves through — makes the rename sound, and dropping any one of the four checks breaks it | Every binding environment at the reference site |
| `rename_reach.fsl` | Nothing the fix leaves unedited still writes the old name, and the new name is never left declared twice — over every reason a file of the package can sit outside what the fix edits | Generated and `exclude`d files, unseen in-package tests, and build-excluded files, against both names |
| `rename_reach.fsl` | The build-excluded guard is no broader than it needs to be: a rename beside an excluded file that writes neither name is still offered | As above |
| `rename_sound.fsl` | **Fails** — models a guard that checks package scope only, and enumerates what a sound guard must check beyond it | As above |
| `rename_siblings.fsl` | **Fails** — models fixes that check their target against the pre-fix names only, and shows two of them converging on one name | Every pair of rename targets |

## The two that fail

These are not regressions to repair in the spec. Each models a guard weaker than
the one the implementation applies, and fails on purpose: the counterexamples are
the specification of what a rename guard must check, and every condition they force
is one of those listed under
[Withheld renames](../README.md#withheld-renames).

`rename_sound.fsl` models Go's resolution order — local, then file (imports),
then package, then the universe of predeclared names — and asks whether a rename
leaves every reference pointing where it did. Strengthening the guard one
condition at a time enumerates the minimum it must check:

| Guard | Result | What the counterexample finds |
| --- | --- | --- |
| Package scope only *(as modeled)* | Violated | A local variable at the reference site binds the new name — **compiles, silently changes behavior** |
| + local scope | Violated | An import in some file binds the new name |
| + file scope | Violated | The new name is predeclared (`len`, `error`, …) |
| + universe | **Verified** | — |

The predeclared case is the indirect one. A package-level name legitimately
shadows the universe, so the reference being renamed resolves correctly; the harm
lands on every *other* reference that wanted the builtin. That shows up only once
the spec also asks whether the rename captures references it was never meant to
touch.

Two causes are outside *this* model, because they concern the set of references
and the set of fixes rather than scope resolution. `rename_reach.fsl` covers the
first — which files the fix actually edits, and what it has to establish about
the ones it does not — and `rename_siblings.fsl` covers the second.

That split is worth stating plainly, because the reach half went unmodelled at
first and the one defect that could break a build lived exactly there: a file
excluded by a `_GOOS` suffix or a `//go:build` line was in none of the guard's
rows, so `-fix` rewrote the configuration it could see and left the other one
calling a name that no longer existed.

## Running them

`fslc verify` is a bounded model checker: it holds the whole reachable state space
for the depth it is given. Each spec is therefore kept to the variables its own
properties read. A single spec over the full product of the configuration is one
action with eighteen parameters, which is several million action instances per
step and needs gigabytes for the same claims these prove in single-digit
megabytes.

```console
./spec/verify.sh     # what CI runs: six proved, two violated
```

Or one at a time:

```console
fslc check  label_rules.fsl
fslc verify label_rules.fsl      --depth 5
fslc verify boundary_fix.fsl     --depth 4
fslc verify knobs.fsl            --depth 4
fslc verify directive_effect.fsl --depth 4
fslc verify rename_guarded.fsl   --depth 4
fslc verify rename_reach.fsl     --depth 3
fslc verify rename_sound.fsl     --depth 2   # expected: violated
fslc verify rename_siblings.fsl  --depth 3   # expected: violated
```

`knobs.fsl` and `directive_effect.fsl` configure once and then have only the
actions that record a report, so their reachables are witnessed at step 1 or 2;
the others add a fix action and witness at step 2. The deadlock warning a bounded
run prints is the shape of the model, not a failure, and `rename_guarded.fsl`
also reports a vacuous antecedent — which is the guard working, and is stated as
`NothingResolvedNewName` rather than left as a warning.

The six that pass are `proved` under `--engine induction`, which is what
`verify.sh` and CI assert. Bounded verification alone would let an invariant be
true to a depth without being inductive, and reading the exit code alone would
let a spec that stopped parsing pass as "violated, as intended" — `fslc` exits
non-zero for a parse error too. `verify.sh` reads the JSON verdict, pins each
failing spec to the invariant it must break, and checks that `knobs.fsl` and
`boundary_fix.fsl` still share one scope model.

## Negative controls

A spec that passes whether or not the code is correct proves nothing. These were
built by writing the wrong design and checking that the spec rejects it. Each row
is a semantics that contradicts the documented one; each was run:

| Wrong design | Result |
| --- | --- |
| The containing type's directive reaches every kind, not only fields | `reachable_failed` |
| A `var`/`const`/`type` block's directive reaches every kind | `reachable_failed` |
| The block level is dropped | `reachable_failed` |
| The file level outranks the declaration's own directive | `reachable_failed` |
| Exportedness is an exemption rather than a default, so a directive cannot narrow an exported declaration | `reachable_failed` |
| A boundary is reported without a cross-namespace reference | `violated` |
| A boundary is reported for a scope that is not private | `violated` |
| `rules.exportedLabels` is ignored and exported names are always named | `reachable_failed` |
| `rules.exportedLabels` also gates on the namespace count | `reachable_failed` |
| The naming rules reach members | `reachable_failed` |
| The core namespace is named like any other | `reachable_failed` |
| A label fix does not record itself | `reachable_failed` |
| A fix is applied to an exported declaration | `violated` |
| `fair` is dropped from a fix action | `violated` (`leadsTo`) |
| A fix is offered where no violation exists, or over the author's own directive | `violated` |
| A scope directive stops at the first thing in its reach, or at the last | `reachable_failed` |
| The binding test reads `defaults.unexported` instead of quantifying over it | `violated` |
| The binding test asks only whether the name is exported, ignoring an enclosing directive | `violated` |
| A `var`/`const`/`type` block's directive does not reach the fields of a type it holds | `reachable_failed` |
| `boundary_fix.fsl` stops reading `blockDir`, or stops discriminating exportedness | `reachable_failed` |
| `fair` is dropped from `fixBoundary` | `violated` (`leadsTo`) |
| A scope directive ignores a nearer directive that shadows it | `violated` |
| Any one of the four scope checks in `renameSafe` is dropped | `violated` |
| The build-excluded file is not consulted, so a rename disturbs a name only another configuration writes | `violated` |
| An unseen in-package test file does not withhold the rename | `violated` |

Three habits keep those controls sharp. **Record the report.** A spec whose only
action assigns the whole state at once cannot carry an invariant that any state
could break: every property over the resulting `def`s is a tautology, and
`--engine induction` buys nothing over `check`. Recording what the code *does* —
`reported`, `bound`, `fixed`, `lastFix` — puts the guards that produce it under
the invariant, which is where a guard has to be for its deletion to be noticed.
**Pin every other reason.** A reachable
named for one distinction must fix the variables that could satisfy it for
another reason — `ExportedSilentByDefault` pins the kind, the namespace, the core
flag, the mode and the label, so exportedness is the only thing left doing the
work. **Prefer a witness to a restatement.** An invariant that re-spells the
definition it guards pins that definition but proves nothing about behaviour;
`TypeDirInertOnPkg` and `TypeDirInertOnMethod` instead assert that the
declaration still fires, which a leaking semantics cannot satisfy.

A change here that makes every check pass unconditionally has stopped the check
discriminating.
