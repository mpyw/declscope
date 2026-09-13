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
| `boundary_fix.fsl` | Inserting `//declscope:package` always removes the boundary crossing, over a `private` inherited from the containing type or the file | Every combination of `defaults.unexported`, kind, exportedness, owner exportedness, reference shape, and the declaration, type and file directives |
| `boundary_fix.fsl` | The fix is offered only where a crossing exists, and never overwrites the declaration's own directive | As above |
| `knobs.fsl` | `defaults.unexported` and every directive level demonstrably change an outcome, for each kind of declaration | As above |
| `knobs.fsl` | The containing type's directive reaches fields and nothing else — witnessed by a package-level declaration and a method that still fire | As above |
| `knobs.fsl` | A declaration reachable from outside the package never carries a boundary, while an exported member of an *unexported* type does | As above |
| `knobs.fsl` | A reference from inside the namespace never crosses a boundary | As above |
| `directive_effect.fsl` | A scope directive is used when anything in its reach binds to it, and reported when nothing does | Every combination of target presence and subjecthood, and two enclosed declarations by subjecthood and shadowing |
| `rename_sound.fsl` | **Fails** — models a guard that checks package scope only, and enumerates what a sound guard must check beyond it | Every binding environment at the reference site |
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

Two causes are deliberately outside this model, because they concern the set of
references and the set of fixes rather than scope resolution: a reference in a
file the analyzer does not collect (generated, or excluded by config) is never
rewritten, and `rename_siblings.fsl` covers the second.

## Running them

`fslc verify` is a bounded model checker: it holds the whole reachable state space
for the depth it is given. Each spec is therefore kept to the variables its own
properties read. A single spec over the full product of the configuration is one
action with eighteen parameters, which is several million action instances per
step and needs gigabytes for the same claims these prove in single-digit
megabytes.

```console
./spec/verify.sh     # what CI runs: four proved, two violated
```

Or one at a time:

```console
fslc check  label_rules.fsl
fslc verify label_rules.fsl      --depth 3
fslc verify boundary_fix.fsl     --depth 3
fslc verify knobs.fsl            --depth 2
fslc verify directive_effect.fsl --depth 2
fslc verify rename_sound.fsl     --depth 2   # expected: violated
fslc verify rename_siblings.fsl  --depth 3   # expected: violated
```

Every spec configures once, so each reachable is witnessed at step 1 and the
deadlock warning that follows is the shape of the model, not a failure. The four
that pass are also `proved` under `--engine induction`, which is what
`verify.sh` and CI assert — bounded verification alone would let an invariant be
true to a depth without being inductive.

Every spec here configures once and stops, so `configure` is the only action and
each reachable is witnessed at step 1; the deadlock warning that follows is the
shape of the model, not a failure. All five of the passing specs are also `proved`
under `fslc verify <file> --engine induction`.

## Negative controls

A spec that passes whether or not the code is correct proves nothing. These were
built by writing the wrong design and checking that the spec rejects it. Each row
is a semantics that contradicts the documented one; each was run:

| Wrong design | Result |
| --- | --- |
| The containing type's directive reaches every kind, not only fields | `reachable_failed` |
| An exported declaration carries a boundary | `violated` |
| A member's exportedness ignores its owner type | `reachable_failed` |
| The file level outranks the declaration's own directive | `reachable_failed` |
| `rules.exportedLabels` is ignored and exported names are always named | `reachable_failed` |
| The naming rules reach members | `reachable_failed` |
| The core namespace is named like any other | `reachable_failed` |
| A fix is offered where no violation exists, or over the author's own directive | `violated` |
| A scope directive stops at the first thing in its reach | `reachable_failed` |
| A scope directive ignores a nearer directive that shadows it | `violated` |

Two habits keep those controls sharp. **Pin every other reason.** A reachable
named for one distinction must fix the variables that could satisfy it for
another reason — `ExportedSilentByDefault` pins the kind, the namespace, the core
flag, the mode and the label, so exportedness is the only thing left doing the
work. **Prefer a witness to a restatement.** An invariant that re-spells the
definition it guards pins that definition but proves nothing about behaviour;
`TypeDirInertOnPkg` and `TypeDirInertOnMethod` instead assert that the
declaration still fires, which a leaking semantics cannot satisfy.

A change here that makes every check pass unconditionally has stopped the check
discriminating.
