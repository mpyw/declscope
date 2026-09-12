# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

**declscope** is a Go linter that adds two pseudo visibility levels between `Exported` and `unexported`, and enforces them with [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis). It exists so that a package can stay **flat** without losing every internal boundary.

| Scope | Meaning |
| --- | --- |
| `public` | Usable outside the package |
| `package` | Usable anywhere in the package |
| `file` | Usable only inside its own **namespace** |

### Core concept: reach is stated, ownership is named

The single most important thing to understand before changing anything here is that **the name never determines reach**.

Everything unexported is private to its namespace; `//declscope:package` is the only thing that widens it. An earlier design made a namespace prefix mean package-internal, which was rejected: it overloaded one signal with two meanings, so a prefix added purely for legibility silently widened a declaration, and a codebase that prefixed everything for readability would have ended up with nothing protected. Removing that also removed the `demotion` rule (which existed only to patch the overloading), the rename fix for boundary crossings, and the possibility of a diagnostic carrying two conflicting alternatives.

The prefix instead does a separate job: it is an **ownership label** on unexported package-level declarations, making the owning unit legible at every use site. Only the naming rules (`promote`, `demote`) are configurable. Reach enforcement is not, and neither is whether members are checked — an earlier `rules.members` key gated collection of methods and fields entirely, silently switching off more than its name suggested. A cross-cutting toggle in a map keyed by rule names is the shape to avoid.

`rules.promote` is tri-state (`internal.PromoteMode`) and defaults to `ondemand`, requiring it only once a package has a second namespace — in a package with one, every other rule is structurally inert anyway, since every reference is already inside the single namespace.

**Methods and struct fields are governed differently.** They are already namespaced by the type that owns them and cannot collide, so a label would produce `u.userSave()`, exactly the stutter Go idiom avoids. Their problem is encapsulation, not naming, so the boundary is the namespace of the **type**, not of the file, and a member violation is never fixed by renaming.

### Namespaces

A namespace is the unit of file privacy, defaulting to the camelCased file name so that each file is its own namespace, and overridable with `//declscope:namespace <name>` before the package clause.

The namespace count that `rules.promote: ondemand` keys off is taken from the package's **non-test** files (`collection.namespaces`). A test file joins its subject's namespace rather than creating a boundary, and counting one whose name matches no source file (`integration_test.go`) would make a package's test variant disagree with the package itself.

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
  rule/                   the rule vocabulary, shared by diagnostics, config, baseline and ignores
  namespace/              file name -> namespace, prefix matching, qualify/unqualify
  scope/                  the three-level Scope enum
  directive/              //declscope:... comment parsing
  config/                 YAML loading and lookup
  baseline/               baseline file format, lookup and regeneration
cmd/declscope/            singlechecker entry point, plus the `baseline` subcommand
```

`internal/{analyzer,collect,options,report}.go` form one logical unit and declare `//declscope:namespace analyzer` so that declscope passes its own check. Keep that directive when adding files to that unit.

### Structural constraints

- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified; `pass.Pkg` scoping does that for free and no `analysis.Fact` is needed.
- Whether an *exported* identifier is used outside its package is deliberately **out of scope**: `go/analysis` has no upward view of the program. Answering it would require a separate whole-program mode driven by `packages.Load`, which would not fit a plain Analyzer. Combine with an unused-code linter instead.
- Generated files are excluded as declaration sites **and** as reference sites, since a violation in generated code is not actionable.

## Rules

`internal/rule` holds the one vocabulary: `escape`, `promote`, `demote`. The same name is the diagnostic's `Category`, the `Rule` field of a baseline key, and what an ignore directive targets. **Adding a rule means adding it there**, not inventing a string at the report site.

`promote` and `demote` are exclusive **by construction**: `checkDemote` returns early wherever `opts.Promote.required(c.namespaces)` holds, so they can never contradict each other on one declaration. Preserve that property when adding checks.

A `foreign-method` rule existed briefly and was removed. `checkEscape` reports a method at its *declaration*, not at the call, so a method grown on another namespace's type is already caught wherever it is used — the declaration-site rule only ever added a second diagnostic at the same position. The one case it covered alone was a foreign method that is never called, which is dead code and an unused-code linter's business, the same reasoning that kept "exported but unused outside the package" out of scope.

`namespace.Unqualify` (demote's rename) lowers a leftover initialism the way Go spells one (`userID` → `id`, `userURLPath` → `urlPath`), which the naive version got wrong (`iD`). It returns a second value explaining any refusal.

`checkDemote` exempts a name identical to its namespace. The causality usually runs the other way there — `user.go` is named after the `user` it declares — so there is no label to strip, and the only advice available would have been "rename it by hand". `promote` still accepts such a name.

**Not being able to derive a rename is never a reason to stay silent.** `checkDemote` gates on `namespace.HasPrefix` — whether there is a label at all — and then reports either way, embedding `Unqualify`'s reason when it has no suggestion. An earlier version skipped the declaration entirely, which left a codebase half-converted under `demote: true` with nothing saying why. `checkPromote` has always behaved this way when its rename target is taken; keep new rules consistent with it.

## Directives

```go
//declscope:public            // state the scope instead of deriving it from the name
//declscope:package
//declscope:file
//declscope:ignore            // silence every rule for the declaration
//declscope:ignore demote     // silence named rules only (escape, promote, demote)
//declscope:namespace <name>  // file level, before the package clause
```

Ignores accumulate (`Decl.Ignores`) and each is reported unused on its own. `ignored()` marks **every** directive covering a rule as used, not just the first, so overlapping directives are not misreported as unused.

`//declscope:namespace` matches Go's directive syntax, so `go/doc` strips it from rendered documentation. In a file with a package comment it belongs at the bottom of that comment after a blank `//` line; in a file without one it is separated from the package clause by a blank line, because flush against `package` it becomes an empty package comment and adds a stray blank line to the rendered package doc. `FileNamespace` scans `file.Comments` rather than `file.Doc`, so every placement is recognised.

Placement of declaration-level directives: a declaration's doc comment, or a trailing comment on the same line. `ast.FuncDecl` has no `Comment` field, so trailing directives on functions are found through `fileInfo.lineComments`. A directive on a parenthesized block applies to every spec in it; a spec's own directive overrides it (`directive.Decl.Merge`).

Unused ignore directives are reported, matching the convention in `mpyw/gormreuse` and `mpyw/zerologlintctx`. Ignores are consulted **before** the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

## Suggested fixes

Every diagnostic carries **at most one** fix, which is what makes `-fix` unambiguous:

- a boundary crossing is fixed by inserting `//declscope:package`
- a missing label is fixed by renaming

They can never conflict, because a rename does not change reach. (`x/tools`' `ApplyFixes` applies only the first fix of a diagnostic and logs `ignoring alternative fix` for the rest, so carrying alternatives was always a liability.)

Renames are skipped when `pass.Pkg.Scope().Lookup(newName)` is non-nil, since renaming into an existing package-level name would not compile. They are also never offered for members, or for a file whose name yields no valid namespace.

**A declaration whose scope came from a directive gets no fix at all** (`t.dir.HasScope`). Both the directive and the use site are deliberate, so `-fix` must not overwrite the author's directive. An earlier version emitted `//declscope:package` next to an existing `//declscope:file`, producing code the linter itself rejected.

`namespace.Qualify` prepends blindly, so its suggestion can stutter when the name already contains the namespace word (`defaultBaselineName` → `baselineDefaultBaselineName`). The fix is a suggestion; a human renaming it to `baselineDefaultName` is expected and fine.

`directiveFix` checks whether the anchor starts its line (`startsLine`, via `pass.ReadFile`). A field of a single-line struct does not, and inserting the directive there attached it to the `struct {` line instead of the field.

## Baseline

`declscope baseline ./...` regenerates `.declscope-baseline.yaml` wholesale; it is never hand-edited. Entries are keyed by (package, rule, declaration) — deliberately not by position — so they survive code motion and file renames.

Generation does not go through `singlechecker`: a baseline entry has to identify a violation structurally, and a driver only returns rendered diagnostics. The analyzer declares no `Requires` and exports no facts, so `cmd/declscope/baseline.go` drives it over `go/packages` with a hand-built `analysis.Pass`, calling `internal.Collect`. Keep that entry point working if the analyzer ever gains dependencies.

The analyzer deliberately does **not** report unmatched baseline entries. A package's test variant sees references the ordinary variant does not, so an entry that matched nothing in one pass is not evidence it is stale. Regeneration is what prunes, and its diff is the record of what was fixed.

## Testing

```bash
go test ./...          # analysistest + unit tests
./test_all.sh          # tests, golangci-lint, and dogfooding
```

- `testdata/src/*` are `analysistest` packages. `promotealways/`, `demote/` and `demoteinert/` carry their own `.declscope.yaml`, which also exercises config discovery end to end.
- Goldens are plain files, not txtar archives, because no diagnostic carries alternative fixes any more.
- Keep each testdata package focused on one rule. `promoterule/order.go` deliberately touches nothing in namespace `user`, so the label rule is tested without escape diagnostics landing on the same lines.
- Diagnostics on directives are reported at the comment, so their `// want` comments belong on the directive line, not the declaration line.
- Do not add a `.declscope.yaml` or `.declscope-baseline.yaml` at the repository root: both are found by an upward lookup from each analyzed package, so a default-named file at the root would reach every `testdata` package and change what the tests assert. The settings declscope holds itself to live in `.declscope-strict.yaml` and are applied with an explicit `-config` in CI and `test_all.sh`.
- `testdata/src/baselined` carries its own `.declscope-baseline.yaml`, which also exercises the lookup end to end.

## Conventions

- Module: `github.com/mpyw/declscope`, matching the layout of `mpyw/gormreuse` and `mpyw/zerologlintctx`.
- `go.mod` pins `toolchain go1.27.0` while keeping the `go` directive at 1.25.0, because golangci-lint refuses to load a module whose `go` directive is newer than the Go it was built with.
- Distribution is via GitHub Releases (goreleaser) with **mise as the recommended install path**; `go install` / `go tool` / `go vet -vettool` also work.
