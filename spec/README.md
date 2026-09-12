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
| `label_rules.fsl` | `qualify` and `unqualify` never both fire for one declaration | Every combination of `rules.qualify`, `rules.unqualify`, namespace count, kind, exportedness, namespace presence and label |
| `label_rules.fsl` | Applying either label fix removes the violation it addresses | As above |
| `boundary_fix.fsl` | Inserting `//declscope:package` always removes the boundary crossing | Every combination of `defaults.exported`, `defaults.unexported`, exportedness, scope directive and reference shape |
| `knobs.fsl` | Every configuration key demonstrably changes an outcome, for members as well as package-level declarations | As above, plus kind |
| `knobs.fsl` | A reference from inside the namespace never crosses a boundary | As above |
| `rename_sound.fsl` | **Fails** — models a guard that checks package scope only, and enumerates what a sound guard must check beyond it | Every binding environment at the reference site |
| `rename_siblings.fsl` | **Fails** — models fixes that check their target against the pre-fix names only, and shows two of them converging on one name | Every pair of rename targets |

## The two that fail

These are not regressions to repair in the spec. Each models a guard weaker than
the one the implementation applies, and fails on purpose: the counterexamples are
the specification of what a rename guard must check, and every condition they force
is one of those listed under
[When a rename is withheld](../README.md#when-a-rename-is-withheld).

`rename_sound.fsl` models Go's resolution order — local, then file (imports),
then package, then the universe of predeclared names — and asks whether a rename
leaves every reference pointing where it did. Strengthening the guard one
condition at a time enumerates the minimum it must check:

| Guard | Result | What the counterexample finds |
| --- | --- | --- |
| Package scope only *(as modelled)* | Violated | A local variable at the reference site binds the new name — **compiles, silently changes behaviour** |
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
action with eleven parameters, which is 31,104 action instances per step and needs
gigabytes for the same claims these prove in single-digit megabytes.

```console
fslc check  label_rules.fsl
fslc verify label_rules.fsl --depth 3
fslc verify boundary_fix.fsl --depth 3
fslc verify knobs.fsl           --depth 2
fslc verify rename_sound.fsl    --depth 2   # expected: violated
fslc verify rename_siblings.fsl --depth 3   # expected: violated
```

## Negative controls

A spec that passes whether or not the code is correct proves nothing. `knobs.fsl`
discriminates: run against semantics in which members do not resolve from
`defaults`, it returns `reachable_failed`, because the knob cannot be shown to
bite. A change here that makes every check pass unconditionally has stopped the
check discriminating.
