# Repository instructions

declscope is a Go linter that enforces a `private` scope within a flat package. A namespace normally comes from a file name. A `package` declaration can be used from other namespaces in the same package; a `private` declaration cannot. Read [the implementation notes](design/implementation.md) before changing a rule, a fix, configuration, `shrink`, or their tests. They record the evidence and rejected alternatives behind the current behavior. Read [the README](README.md) for the user-facing contract and [the specs](spec/README.md) for formal properties.

## Contracts to preserve

- Exportedness alone decides the analyzer's default scope: exported declarations are `package`, unexported declarations are `private`. A directive can override either. The analyzer sees one package, so it must not infer whether an exported declaration is used by importers.
- `declscope shrink` is a separate, module-wide command. It judges exported declarations only where `internal/` limits possible importers. A report may survive uncertainty; an automatic fix must be withheld unless it is sound.
- Keep the default naming mode at `never` and `surplus` and `unused` at `strict` unless a change to that policy is deliberate and measured. Changes to these modes can add diagnostics to unchanged repositories. The repository checks itself with `.declscope-strict.yaml` through an explicit `-config`.
- A directive records the author's scope decision. A boundary fix must not overwrite a declaration's own scope directive. Suggested fixes must converge in one pass without creating new reports or breaking type checking.
- Keep the rule name shared between diagnostics, config, ignores, and baseline keys in `internal/rule`. Preserve the distinction between malformed directives and unused ones.

## Repository workflow

- `go test ./...` runs the Go tests. `mise x -- ./test_all.sh` runs the complete CI checks, including coverage, linting, dogfooding, `shrink`, and formal specs. Use the latter when changing analyzer behavior or fixes.
- For documentation changes, run `mise run docs-build` or the equivalent site pipeline. The site is generated from `README.md` and `docs/`; generated `.site/` files are not committed.
- Keep `skills/declscope-adoption/` as the distributed skill's source. `skills.go` embeds that path and `gh skill install` reads it. This skill is for repositories adopting declscope, so do not add it to this repository's agent skill discovery paths.
- Do not add `.declscope.yaml` or `.declscope-baseline.yaml` at the repository root. Config lookup would reach the `testdata` packages and change their expectations. Use `.declscope-strict.yaml` with `-config` for this repository.
- Fixture packages in `testdata/src/` pin diagnostics and fixes. `internal/shrink/testdata/` contains whole test modules. Keep each fixture focused on the rule or behavior it proves.
