# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

**declscope** is a Go linter that adds two pseudo visibility levels between `Exported` and `unexported`, and enforces them with [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis).

| Scope | Meaning |
| --- | --- |
| `public` | Usable outside the package |
| `package` | Usable anywhere in the package |
| `file` | Usable only inside its own **namespace** |

### Core concept: two problems, two rules

The single most important thing to understand before changing anything here is that **package-level declarations and members are governed by different rules on purpose**.

**Package-level identifiers** compete in one flat scope, so the problem is *namespace pollution*, and the rule is **name-driven**:

- Exported → `public`
- Unexported carrying the file's namespace as a prefix → `package`
- Every other unexported → `file`

**Methods and struct fields** are already namespaced by the type that owns them and cannot collide with anything, so there is no pollution to prevent. Applying the prefix rule would produce `u.userSave()`, which is exactly the stutter Go idiom avoids. The problem for members is *encapsulation*, so the rule is **boundary-driven**, and the boundary is the namespace of the **type**, not of the file.

A consequence worth remembering: a member violation is never fixed by renaming. Its only suggested fix is a directive.

### Namespaces

A namespace is the unit of file privacy, defaulting to the camelCased file name so that each file is its own namespace, and overridable with `//declscope:namespace <name>` before the package clause.

The indirection is deliberate: using the file name *itself* would mean renaming a file cascades into renaming every identifier it declares. It also makes `_test.go` sharing its subject's namespace fall out naturally rather than needing a special case.

Normalization rules live in `internal/namespace` and are covered by a table test. GOOS/GOARCH suffixes are stripped because they are build constraints, not namespaces.

## Architecture

```
analyzer.go               Analyzer definition, -config flag, config discovery
internal/
  analyzer.go             Run: collect files -> targets -> refs -> report
  collect.go              fileInfo, target, reference collection
  options.go              resolved configuration, scope resolution, exclude globs
  report.go               diagnostics and suggested fixes
  namespace/              file name -> namespace, prefix matching, qualify/unqualify
  scope/                  the three-level Scope enum
  directive/              //declscope:... comment parsing
  config/                 YAML loading and lookup
cmd/declscope/            singlechecker entry point
```

`internal/{analyzer,collect,options,report}.go` form one logical unit and declare `//declscope:namespace analyzer` so that declscope passes its own check. Keep that directive when adding files to that unit.

### Structural constraints

- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified; `pass.Pkg` scoping does that for free and no `analysis.Fact` is needed.
- Whether an *exported* identifier is used outside its package is deliberately **out of scope**: `go/analysis` has no upward view of the program. Answering it would require a separate whole-program mode driven by `packages.Load`, which would not fit a plain Analyzer. Combine with an unused-code linter instead.
- Generated files are excluded as declaration sites **and** as reference sites, since a violation in generated code is not actionable.

## Directives

```go
//declscope:public            // state the scope instead of deriving it from the name
//declscope:package
//declscope:file
//declscope:ignore            // suppress every diagnostic for the declaration
//declscope:namespace <name>  // file level, before the package clause
```

`//declscope:namespace` matches Go's directive syntax, so `go/doc` strips it from rendered documentation. In a file with a package comment it belongs at the bottom of that comment after a blank `//` line; in a file without one it is separated from the package clause by a blank line, because flush against `package` it becomes an empty package comment and adds a stray blank line to the rendered package doc. `FileNamespace` scans `file.Comments` rather than `file.Doc`, so every placement is recognised.

Placement of declaration-level directives: a declaration's doc comment, or a trailing comment on the same line. `ast.FuncDecl` has no `Comment` field, so trailing directives on functions are found through `fileInfo.lineComments`. A directive on a parenthesized block applies to every spec in it; a spec's own directive overrides it (`directive.Decl.Merge`).

Unused `//declscope:ignore` directives are reported, matching the convention in `mpyw/gormreuse` and `mpyw/zerologlintctx`.

## Suggested fixes

A cross-namespace violation on a package-level declaration offers two **alternatives**:

1. rename to carry the namespace prefix (edits every ident in the package)
2. add `//declscope:package`

Order matters. `-fix` applies only the first fix of each diagnostic and prints `ignoring alternative fix ...` for the rest, so fix 1 is the default repair and fix 2 is what editors surface as a second code action.

Renames are skipped when `pass.Pkg.Scope().Lookup(newName)` is non-nil, since renaming into an existing package-level name would not compile.

## Testing

```bash
go test ./...          # analysistest + unit tests
./test_all.sh          # tests, golangci-lint, and dogfooding
```

- `testdata/src/*` are `analysistest` packages. `demotion/` and `foreign/` carry their own `.declscope.yaml`, which also exercises config discovery end to end.
- `testdata/src/fixes/*.golden` are **txtar archives with one section per fix message**, which is how `analysistest` compares alternative fixes separately instead of merging them into one nonsensical result.
- Diagnostics on directives are reported at the comment, so their `// want` comments belong on the directive line, not the declaration line.
- Do not add a `.declscope.yaml` at the repository root: `config.Find` would pick it up for every `testdata` package and change what the tests assert.

## Conventions

- Module: `github.com/mpyw/declscope`, matching the layout of `mpyw/gormreuse` and `mpyw/zerologlintctx`.
- `go.mod` pins `toolchain go1.27.0` while keeping the `go` directive at 1.25.0, because golangci-lint refuses to load a module whose `go` directive is newer than the Go it was built with.
- Distribution is via GitHub Releases (goreleaser) with **mise as the recommended install path**; `go install` / `go tool` / `go vet -vettool` also work.
