# Formal specs

Machine-checked statements about the rules, written in [FSL](https://github.com/ymm-oss/fsl)
and verified with `fslc`. They are documentation that cannot drift: the claims below
are the ones `fslc verify` actually proves, over **every** configuration, not a
sample of them.

They check the **design**, not the implementation. A counterexample here means the
rules contradict each other; confirming that the Go code agrees still takes a run
against the binary.

## What is proved

| Spec | Claim | Scope covered |
| --- | --- | --- |
| `label_rules.fsl` | `promote` and `demote` never both fire for one declaration | every combination of `rules.promote`, `rules.demote`, namespace count, kind, exportedness, namespace presence and label |
| `label_rules.fsl` | applying either label fix removes the violation it addresses | as above |
| `escape_fix.fsl` | inserting `//declscope:package` always removes the boundary crossing | every combination of `defaults.exported`, `defaults.unexported`, exportedness, scope directive and reference shape |
| `knobs.fsl` | every configuration key demonstrably changes an outcome, for members as well as package-level declarations | as above, plus kind |
| `knobs.fsl` | a reference from inside the namespace never crosses a boundary | as above |
| `rename_sound.fsl` | **fails** — states exactly what a rename must check to be sound, and shows today's guard is not enough | every binding environment at the reference site |
| `rename_siblings.fsl` | **fails** — two fixes in one run can rename two declarations to the same name | every pair of rename targets |

## The two that fail

These are not regressions to repair in the spec; they are the specification of
defects the implementation still has, kept here so the fix has a target to hit.

`rename_sound.fsl` models Go's resolution order — local, then file (imports),
then package, then the universe of predeclared names — and asks whether a rename
leaves every reference pointing where it did. Strengthening the guard one
condition at a time enumerates the minimum it must check:

| Guard | Result | What the counterexample found |
| --- | --- | --- |
| package scope only *(today)* | violated | a local variable at the reference site binds the new name — **compiles, silently changes behaviour** |
| + local scope | violated | an import in some file binds the new name |
| + file scope | violated | the new name is predeclared (`len`, `error`, …) |
| + universe | **verified** | — |

The predeclared case is worth reading twice. A package-level name legitimately
shadows the universe, so the reference being renamed is fine; the harm lands on
every *other* reference that wanted the builtin. That only shows up once the spec
also asks whether the rename captures references it was never meant to touch.

Two causes are deliberately outside this model, because they concern the set of
references and the set of fixes rather than scope resolution: a reference in a
file the analyzer does not collect (generated, or excluded by config) is never
rewritten, and `rename_siblings.fsl` covers the second.

## Running them

`fslc verify` is a bounded model checker: it holds the whole reachable state space
for the depth it is given. Keep each spec to the variables its own properties read
— an earlier single spec covering the full product used one action with eleven
parameters, which is 31,104 action instances per step, and needed gigabytes for the
same claims these prove in single-digit megabytes.

```console
fslc check  label_rules.fsl
fslc verify label_rules.fsl --depth 3
fslc verify escape_fix.fsl  --depth 3
fslc verify knobs.fsl           --depth 2
fslc verify rename_sound.fsl    --depth 2   # expected: violated
fslc verify rename_siblings.fsl --depth 3   # expected: violated
```

## Negative controls

A spec that passes whether or not the code is correct proves nothing. `knobs.fsl`
was validated by re-running it against the semantics as they were before members
were resolved from `defaults`: it returns `reachable_failed`, because the knob
cannot be shown to bite. Keep that property — if a change here makes every check
pass unconditionally, the check has stopped discriminating.
