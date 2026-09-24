# Formal specs

Machine-checked statements about the rules, written in [FSL](https://github.com/ymm-oss/fsl)
and verified with `fslc`. They are documentation that cannot drift: the claims below
are the ones `fslc verify` actually proves, over **every** configuration, not a
sample of them.

They check the **design**, not the implementation. A counterexample here means the
rules contradict each other; confirming that the Go code agrees takes a run against
the binary.

## What is proved

> [!NOTE]
> **Member** in these specs means a name written inside a type's declaration: a
> struct field, and an interface's method name. **Method** means a method with a
> receiver, which is an ordinary top-level declaration however much it reads like
> a member. The two are governed differently, and most of `knobs.fsl` is about
> keeping them apart.

| Spec | Claim | Scope covered |
| --- | --- | --- |
| `naming_rules.fsl` | `qualify` fires only where the name carries its namespace nowhere. A name carrying it inside — `statementReducer` in `reducer.go` — is silent even under `always` | Every combination of `rules.naming.qualify`, `rules.naming.exported`, namespace count, kind, exportedness, core membership, prefixability and where the name carries the namespace |
| `naming_rules.fsl` | Applying the prefix fix removes the violation, and no fix is applied where no violation exists | As above |
| `naming_rules.fsl` | No rename is ever applied to an exported declaration | As above |
| `naming_rules.fsl` | Each naming key bites, and each silence is witnessed with every other reason for silence pinned | As above |
| `naming_rules.fsl` | The core namespace is outside the rule, whatever is configured | As above |
| `boundary_off.fsl` | `rules.boundary: off` silences `boundary` and nothing else. `qualify` and `surplus` still fire with it on, and `rules.surplus: off` still gates only `surplus` | Every combination of the two switches, the resolved scope, the reference shape, the naming mode and its inputs, and the two surplus inputs |
| `boundary_off.fsl` | Each of the three rules keeps its own guards, witnessed by the report each one records rather than by a restatement of its definition | As above |
| `filter.fsl` | `filter.only` narrows and `filter.omit` subtracts. Either list admits or rejects on any one of its patterns, an empty `only` places no restriction, and `omit` still bites inside `only` | Every combination of which of three patterns each list holds and which of them the file matches |
| `filter.fsl` | The order of the two lists is not observable: narrowing then subtracting and subtracting then narrowing name the same set | As above |
| `config_inherit.fsl` | A config file states some keys and leaves the rest alone: the nearest file that states a key wins, and a key no nearer file states still comes from the root | Every combination of what three levels state about one value key, and of what two levels state about two vocabulary namespaces |
| `config_inherit.fsl` | `vocabulary` merges per namespace rather than the nearer map replacing the other, and neither namespace can take the other's value | As above |
| `filter_chain.fsl` | `only` intersects and `omit` unions down the chain, so **a config file can only ever shrink what is read** and an `omit` at the root holds everywhere below it | Every combination of which lists two levels state and which of two files matches each of the four |
| `filter_chain.fsl` | The filter report fires exactly when the nearest `only` is cancelled by one above it, and stays quiet where a package reads nothing on purpose | As above |
| `boundary_fix.fsl` | Inserting `//declscope:package` always removes the boundary crossing, and is withheld wherever any level already stated a scope | Every combination of `defaults.unexported`, kind, exportedness, reference shape, and the declaration, block, type and file directives |
| `boundary_fix.fsl` | The fix is offered only where a crossing exists, and never overwrites the declaration's own directive | As above |
| `fix_members.fsl` | Two directive insertions in one run never converge: no directive `-fix` writes binds nothing, and whichever of the two it writes settles the crossing | Every combination of `defaults.unexported`, the exportedness and reference shape of a type and one of its members, and the declaration, type and file directives |
| `fix_members.fsl` | The narrower fix is subsumed, not disabled: a member whose type does not cross still carries its own directive | As above |
| `knobs.fsl` | `defaults.unexported` and every directive level demonstrably change an outcome, for each kind of declaration | As above |
| `knobs.fsl` | The containing type's directive reaches its members and nothing else, witnessed by a package-level declaration and a method with a receiver that still fire | As above |
| `knobs.fsl` | An exported declaration carries no boundary by default, for every kind | As above |
| `knobs.fsl` | A directive narrows an exported declaration anyway, including a type's directive reaching an exported member | As above |
| `knobs.fsl` | A reference from inside the namespace never crosses a boundary | As above |
| `directive_effect.fsl` | A scope directive is used when anything in its reach binds to it, and reported when nothing does | Every combination of stated scope, enclosing directive, `defaults.unexported`, target presence and exportedness, and two enclosed declarations by exportedness and shadowing |
| `directive_effect.fsl` | Binding is quantified over configurations *and* over enclosing directives — so restating the default of the day never counts, and `//declscope:package` under an enclosing `private` always does | As above |
| `directive_strict.fsl` | Under `rules.directive: strict` a scope directive is also reported when everything it reaches, shadowed or not, would have the scope it names without it under the configuration in force. `loose` reports exactly what `directive_effect.fsl` does, and `strict` keeps every `loose` report | Every combination of `rules.directive`, stated scope, `defaults.unexported`, a declaration taking the directive's scope and one a nearer directive shadows, each by exportedness and next level out, and the five reasons the fix is withheld, one of them a pass that cannot see every crossing |
| `directive_strict.fsl` | The fix that deletes the directive leaves the declaration's scope where it was, and leaves the nearer directive judged against the scope it was judged against. It is offered only under `strict`, and never where it would change another report | As above |
| `knobs.fsl` | A boundary is reported only for a private scope and only across a namespace, and on an exported declaration only where a directive narrowed it — each guard witnessed by an invariant its removal breaks | As above |
| `naming_rules.fsl` | A fix is eventually applied wherever one is offered, which is what the `fair` on the fix actions claims | As above, plus whether a rename is offered at all |
| `rename_guarded.fsl` | The guard `renameSafe` applies — every scope Go resolves through — makes the rename sound, and dropping any one of the four checks breaks it | Every binding environment at the reference site |
| `rename_reach.fsl` | Nothing the fix leaves unedited still writes the old name, and the new name is never left declared twice — over every reason a file of the package can sit outside what the fix edits | Generated and `exclude`d files, unseen in-package tests, and build-excluded files, against both names |
| `rename_reach.fsl` | The build-excluded guard is no broader than it needs to be: a rename beside an excluded file that writes neither name is still offered | As above |
| `surplus.fsl` | `surplus` never reports a declaration reachable by any path: a spelled use, an interface satisfaction, an exported carrier, a linkname, or an opaque source | Every combination of the rule switch, the directive, the five reach paths, and whether the pass read every file |
| `surplus.fsl` | A pass that did not read every file reports nothing, and each suppressor is witnessed alone, with every other one off | As above |
| `surplus_strict.fsl` | Neither `surplus` finding is made of a declaration reachable by any path the rule counts: a spelled use, a struct conversion, a linkname, or an opaque source | Every combination of `rules.surplus`, why the declaration has its scope (the enclosing directive, its own, exportedness, `defaults.unexported`, no enclosing directive), the level the directive is written at, the reach path, the shape of the entry, what else the directive reaches, and whether the pass read every file |
| `surplus_strict.fsl` | `strict` reports a superset of `loose` and never lands on the declaration `loose` already covers. It reports only where an enclosing directive widened the declaration, at the file, block and type level alike, and never a type whose narrowing would narrow a reached member | As above |
| `surplus_strict.fsl` | The inserted `//declscope:private` binds, settles the report, starts no boundary report on the declaration or a member it narrows, never makes `loose` fire, and never leaves the enclosing directive binding nothing. Where it would, the fix is withheld and the report is not | As above |
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

Convergence is not the renames' alone. The directive fix converges its own way,
and `fix_members.fsl` proves the guard for it: a directive written on a type
reaches the type's members, so a member fixed in the same run was left carrying
a directive that bound nothing. That one is in the proved list rather than here,
because the implementation holds the guard.

## Running them

`fslc verify` is a bounded model checker: it holds the whole reachable state space
for the depth it is given. Each spec is therefore kept to the variables its own
properties read. A single spec over the full product of the configuration is one
action with eighteen parameters, which is several million action instances per
step and needs gigabytes for the same claims these prove in single-digit
megabytes.

```console
./spec/verify.sh     # what CI runs: fourteen proved, two violated
```

Or one at a time:

```console
fslc check  naming_rules.fsl
fslc verify naming_rules.fsl      --depth 5
fslc verify boundary_off.fsl     --depth 2
fslc verify config_inherit.fsl   --depth 2
fslc verify filter.fsl           --depth 2
fslc verify filter_chain.fsl     --depth 2
fslc verify boundary_fix.fsl     --depth 4
fslc verify fix_members.fsl      --depth 4
fslc verify knobs.fsl            --depth 4
fslc verify directive_effect.fsl --depth 4
fslc verify directive_strict.fsl --depth 6
fslc verify rename_guarded.fsl   --depth 4
fslc verify rename_reach.fsl     --depth 3
fslc verify surplus.fsl         --depth 2
fslc verify surplus_strict.fsl  --depth 4
fslc verify rename_sound.fsl     --depth 2   # expected: violated
fslc verify rename_siblings.fsl  --depth 3   # expected: violated
```

`knobs.fsl`, `directive_effect.fsl` and `surplus.fsl` configure once and then
have only the actions that record a report, so their reachables are witnessed at
step 1 or 2; the others add a fix action and witness at step 2.
`surplus_strict.fsl` configures in two steps, so that no one action carries the
product of every parameter, and witnesses by step 4. `directive_strict.fsl`
configures in four steps for the same reason, judges, and fixes, so it witnesses
by step 6. The deadlock warning a bounded
run prints is the shape of the model, not a failure, and `rename_guarded.fsl`
also reports a vacuous antecedent — which is the guard working, and is stated as
`NothingResolvedNewName` rather than left as a warning.

The fourteen that pass are `proved` under `--engine induction`, which is what
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
| The containing type's directive reaches every kind, not only its members | `reachable_failed` |
| A `var`/`const`/`type` block's directive reaches every kind | `reachable_failed` |
| The block level is dropped | `reachable_failed` |
| The file level outranks the declaration's own directive | `reachable_failed` |
| Exportedness is an exemption rather than a default, so a directive cannot narrow an exported declaration | `reachable_failed` |
| A boundary is reported without a cross-namespace reference | `violated` |
| A boundary is reported for a scope that is not private | `violated` |
| `rules.naming.exported` is ignored and exported names are always named | `reachable_failed` |
| `rules.naming.exported` also gates on the namespace count | `reachable_failed` |
| The naming rule reaches members | `reachable_failed` |
| Containment is narrowed back to a prefix requirement | `reachable_failed` (`InsideSilentUnderAlways`, `InsideWouldHaveFiredBefore`) |
| The core namespace is named like any other | `reachable_failed` |
| A prefix fix does not record itself | `reachable_failed` |
| A fix is applied to an exported declaration | `violated` |
| `fair` is dropped from a fix action | `violated` (`leadsTo`) |
| A fix is offered where no violation exists, or over the author's own directive | `violated` |
| A scope directive stops at the first thing in its reach, or at the last | `reachable_failed` |
| The binding test reads `defaults.unexported` instead of quantifying over it | `violated` |
| The binding test asks only whether the name is exported, ignoring an enclosing directive | `violated` |
| A `var`/`const`/`type` block's directive does not reach the members of a type it holds | `reachable_failed` |
| `boundary_fix.fsl` stops reading `blockDir`, or stops discriminating exportedness | `reachable_failed` |
| `fair` is dropped from `fixBoundary` | `violated` (`leadsTo`) |
| A scope directive ignores a nearer directive that shadows it | `violated` |
| Any one of the four scope checks in `renameSafe` is dropped | `violated` |
| The build-excluded file is not consulted, so a rename disturbs a name only another configuration writes | `violated` |
| An unseen in-package test file does not withhold the rename | `violated` |
| Both directive insertions fire, so a member is fixed under a type fixed in the same run | `violated` (`InsertedMemberDirectiveBinds`) |
| The narrower insertion wins, so the type's fix is dropped instead of the member's | `reachable_failed` (`OnlyTheWiderFixIsWritten`) |
| `strict` reports without checking reach | `violated` (`NeverReportsReachable`) |
| `strict` also reports where `loose` already reports the directive | `violated` (`NeverTwiceOnOneDeclaration`) |
| `strict` reports under `loose` | `violated` (`StrictOnlyUnderStrict`) |
| `strict` ignores `defaults.unexported` | `violated` (`NeverReportsUnderPackageDefault`) |
| `strict` narrows a type one of whose members is reached | `violated` (`NeverNarrowsAReachedMember`) |
| `strict` reports a member of a type it already narrows | `violated` (`NeverRepeatsTheOwnersFinding`) |
| The `strict` fix is offered where it would leave the directive binding nothing | `violated` (`FixKeepsTheDirectiveBound`) |
| `strict` ignores the declarations a nearer directive shadows | `violated` (`StrictAddsOnlyRedundant`) |
| The `strict` fix is offered on a declaration another namespace uses | `violated` (`FixKeepsOtherReports`) |
| The `strict` fix is offered by a pass that does not read every file | `violated` (`FixKeepsOtherReports`) |
| `strict` keeps `loose`'s quantifier over configurations | `reachable_failed` |
| `strict` reports under `loose` | `violated` (`LooseIsDirectiveEffect`) |
| `strict` drops the reports `loose` makes | `violated` (`LooseIsDirectiveEffect`) |
| `rules.boundary: off` is wired into `surplus` as well | `reachable_failed` (`SurplusFiresWhileBoundaryOff`) |
| `rules.boundary: off` is wired into `qualify` as well | `reachable_failed` (`QualifyFiresWhileBoundaryOff`) |
| The `rules.boundary` gate is dropped from the boundary report | `violated` (`BoundarySilencedWhenOff`) |
| An empty `filter.only` matches nothing rather than placing no restriction | `reachable_failed` (`NoListsAdmitsAFileMatchingNothing`) |
| `filter.omit` is ignored once `filter.only` is set | `violated` (`DecisionMatchesTheRule`) |
| `filter.only` is ignored once `filter.omit` is set | `violated` (`DecisionMatchesTheRule`) |
| The nearest config file wins outright, rather than the nearest one that states the key | `violated` (`MidWinsWhenNearIsSilent`) |
| `vocabulary` is replaced by the nearer map rather than merged | `violated` (`UnstatedNearInheritsItsKey`) |
| `omit` is replaced down the chain rather than unioned | `violated` (`MonotoneDown`) |
| `only` is unioned down the chain rather than intersected | `violated` (`MonotoneDown`) |
| The filter report fires whenever a package reads nothing | `violated` (`WarnedExactlyWhenTheNearestOnlyIsCancelled`) |

Three habits keep those controls sharp. **Record the report.** A spec whose only
action assigns the whole state at once cannot carry an invariant that any state
could break: every property over the resulting `def`s is a tautology, and
`--engine induction` buys nothing over `check`. Recording what the code *does* —
`reported`, `bound`, `fixed`, `lastFix` — puts the guards that produce it under
the invariant, which is where a guard has to be for its deletion to be noticed.
**Pin every other reason.** A reachable
named for one distinction must fix the variables that could satisfy it for
another reason — `ExportedSilentByDefault` pins the kind, the namespace, the core
flag, the mode and the prefix, so exportedness is the only thing left doing the
work. **Prefer a witness to a restatement.** An invariant that re-spells the
definition it guards pins that definition but proves nothing about behaviour;
`TypeDirInertOnPkg` and `TypeDirInertOnMethod` instead assert that the
declaration still fires, which a leaking semantics cannot satisfy.

A change here that makes every check pass unconditionally has stopped the check
discriminating.
