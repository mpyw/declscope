# Formal specs

Machine-checked statements about the rules, written in [FSL](https://github.com/mpyw/fsl)
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
fslc verify knobs.fsl       --depth 2
```

## Negative controls

A spec that passes whether or not the code is correct proves nothing. `knobs.fsl`
was validated by re-running it against the semantics as they were before members
were resolved from `defaults`: it returns `reachable_failed`, because the knob
cannot be shown to bite. Keep that property — if a change here makes every check
pass unconditionally, the check has stopped discriminating.
