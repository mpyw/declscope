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

`rules.promote` is tri-state (`internal.PromoteMode`) and defaults to `ondemand`, requiring it only once a package has a second namespace — in a package with one, every other rule is structurally inert anyway, since every reference is already inside the single namespace. The documented spellings are `always`, `never` and `ondemand`, one enum rather than two booleans and a string; `true` and `false` stay accepted as aliases, which is why `ParsePromoteMode` takes both a bool and a string, and `PromoteMode.String()` returns the documented spelling so nothing printed points at an alias.

**Methods and struct fields are governed differently.** They are already namespaced by the type that owns them and cannot collide, so a label would produce `u.userSave()`, exactly the stutter Go idiom avoids. Their problem is encapsulation, not naming, so the boundary is the namespace of the **type**, not of the file, and a member violation is never fixed by renaming.

What sets members apart is *only* that boundary and their exemption from the label rule. Scope resolution is shared: `Options.resolve` serves both. A separate `resolveMember` used to hardcode public and file-private, which made `defaults.exported` and `defaults.unexported` apply to just half the declarations in a package — a knob that works on some declarations and not others is the shape to avoid.

### Namespaces

A namespace is the unit of file privacy, defaulting to the camelCased file name so that each file is its own namespace, and overridable with `//declscope:namespace <name>` before the package clause.

The namespace count that `rules.promote: ondemand` keys off is taken from the package's **non-test** files (`collection.namespaces`). A test file joins its subject's namespace rather than creating a boundary, and counting one whose name matches no source file (`integration_test.go`) would make a package's test variant disagree with the package itself.

The indirection is deliberate: using the file name *itself* would mean renaming a file cascades into renaming every identifier it declares. It also makes `_test.go` sharing its subject's namespace fall out naturally rather than needing a special case.

Normalization rules live in `internal/namespace` and are covered by a table test. GOOS/GOARCH suffixes are stripped because they are build constraints, not namespaces. Every run of non-identifier characters is a separator (`foo-bar.go` → `fooBar`), a PascalCase stem is lowered (`Foo.go` → `foo`), and an initialism in a later segment is spelled the way Go spells it (`user_id.go` → `userID`) using golint's `commonInitialisms` list, vendored rather than pulled in as a dependency.

**A namespace is an identity first and a label second, and the two are kept apart.** `namespace.Of` always returns the normalised stem — `2fa.go` yields `2fa` — so that `fileInfo.key()` never falls back to the file path and `2fa_test.go` shares its subject's namespace like every other test. `namespace.IsLabel` separately says whether that stem can be prepended to an unexported identifier; `checkPromote` and `checkDemote` gate on it and stay silent when it is false, because the alternative was suggesting `2faTotp` or `FooBar`. An earlier version had `Of` return `""` for a digit-leading stem, which lost the identity in order to protect the label — the second job was allowed to kill the first.

`namespace.HasPrefix` matches the label **ignoring case** and then requires a word boundary, so `userIDCache`, `userIdCache` and `userIdcache` all carry `userID` while `useridentity` does not: a word break the name reproduces inside a multi-word namespace already confirms the label, and only a single word running on in lowercase is a fragment. This is what stops the linter guessing which spelling of an initialism the author chose and demanding `userIdUserIDCache`. `Qualify` spells the first word of the name the same way (`id` → `userID`).

## Architecture

```
analyzer.go               Analyzer definition, -config flag, config discovery
internal/
  analyzer.go             Run: collect files -> targets -> refs -> report
  collect.go              fileInfo, target, reference collection
  options.go              resolved configuration, scope resolution, exclude globs
  report.go               diagnostics and suggested fixes
  rename.go               the conditions under which a rename fix is offered at all
  rule/                   the rule vocabulary, shared by diagnostics, config, baseline and ignores
  namespace/              file name -> namespace, prefix matching, qualify/unqualify
  scope/                  the three-level Scope enum
  directive/              //declscope:... comment parsing
  config/                 YAML loading and lookup
  baseline/               baseline file format, lookup and regeneration
cmd/declscope/            singlechecker entry point, plus the `baseline` subcommand
```

`internal/{analyzer,collect,options,report,rename}.go` form one logical unit and declare `//declscope:namespace analyzer` so that declscope passes its own check. `cmd/declscope/{main,baseline}.go` are one command and declare `//declscope:namespace main` for the same reason. Keep those directives when adding files to either unit — without them the label rule asks every declaration to carry a `baseline`/`analyzer` prefix, which is the tool reporting a boundary that is not really there.

### Structural constraints

- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified; `pass.Pkg` scoping does that for free and no `analysis.Fact` is needed.
- Whether an *exported* identifier is used outside its package is deliberately **out of scope**: `go/analysis` has no upward view of the program. Answering it would require a separate whole-program mode driven by `packages.Load`, which would not fit a plain Analyzer. Combine with an unused-code linter instead.
- Generated files are excluded as declaration sites **and** as reference sites, since a violation in generated code is not actionable.

### Reference collection

`collectRefs` is where every rule gets its evidence, and two facts about `go/types` shape it:

- **An ident can be a definition and a use at once.** An embedded field's ident is in `Defs` as the field `Var` *and* in `Uses` as the `TypeName`. `Defs` and `Uses` are therefore consulted independently; an earlier version returned after `Defs`, which dropped the type use and with it both a rename edit and an `escape` diagnostic. The embedded field itself is deliberately **not** a target (`addFields` skips it) — it has no name of its own to hide or label — but a selection through it (`u.count`) is spelled with the type's name, so `embeddedTypeName` adds those idents to the type's rename set. Aliases are kept as aliases there: a field embedding `A` is spelled `A`, whatever `A` denotes.
- **`Uses` records the instantiated member of a generic type**, for a selection on `List[int]` and even on `List[T]` inside `List`'s own methods, while `byObj` is keyed by the declared object. Every object taken from `Uses` goes through `origin()` (`(*types.Var).Origin()` / `(*types.Func).Origin()`) before the lookup and before it joins `idents`. Without that, every field and method of a generic type was silently unchecked. `receiver` needs no such step: `(*types.Named).Obj()` already names the origin's type name.

`testdata/src/generics`, `embedded` and `fixembedded` pin both, including the receiver resolution of a method on `List[T]`.

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
//declscope:ignore <rules>    // also valid at file level, applying to the whole file
```

`parseIgnore` is shared by both levels, so `//declscope:ignore` cannot come to mean different things depending on where it is written. File-level directives live on `fileInfo.ignores`; declaration-level ones on `Decl.Ignores`. A file-level ignore is scoped to its **file**, not to its namespace, so files sharing a namespace each need their own — one file silently changing another's diagnostics would be much harder to trace back.

`collection.silenced` walks the levels: the declaration, then the type that owns it (`target.ownerObj`, for methods and fields, wherever the member is declared), then the file. It consults **all** of them rather than stopping at the first hit, and `ignored()` marks **every** directive covering the rule as used, so overlapping directives at different levels never make each other look unused.

Unused directives are therefore reported in a **second pass**, after every finding has been seen: a type's directive is often used up by a member reached later in the target list. `target.ignoresUsed` lives on the target for the same reason, rather than in the reporting loop.

`//declscope:namespace` matches Go's directive syntax, so `go/doc` strips it from rendered documentation. In a file with a package comment it belongs at the bottom of that comment after a blank `//` line; in a file without one it is separated from the package clause by a blank line, because flush against `package` it becomes an empty package comment and adds a stray blank line to the rendered package doc. `FileNamespace` scans `file.Comments` rather than `file.Doc`, so every placement is recognised.

Placement of declaration-level directives: a declaration's doc comment, or a trailing comment on the same line. `ast.FuncDecl` has no `Comment` field, so trailing directives on functions are found through `fileInfo.lineComments`. A directive on a parenthesized block applies to every spec in it, and `directive.Decl.Merge` layers the spec's own over it: a scope directive on the spec replaces the block's, while ignores accumulate. The union is deliberate — a narrower ignore must not silently re-enable a rule the block turned off — and `testdata/src/ignorescope` pins it, so keep the docs and that test in step.

Unused ignore directives are reported, matching the convention in `mpyw/gormreuse` and `mpyw/zerologlintctx`. Ignores are consulted **before** the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

## Suggested fixes

Every diagnostic carries **at most one** fix, which is what makes `-fix` unambiguous:

- a boundary crossing is fixed by inserting `//declscope:package`
- a missing label is fixed by renaming

They can never conflict, because a rename does not change reach. (`x/tools`' `ApplyFixes` applies only the first fix of a diagnostic and logs `ignoring alternative fix` for the rest, so carrying alternatives was always a liability.)

**A rename is offered only when it is provably safe** (`renameSafe` in `internal/rename.go`); the diagnostic is reported either way. This is the agreed answer to a class of P0s (#1, #2, #3, #5, #6) where `-fix` produced code that did not compile or, worse, compiled into a program computing something else: the old guard checked only `pass.Pkg.Scope().Lookup(newName)`, but Go resolves a name from the inside out, so `var count` renamed to `fooCount` inside `func Add(fooCount int) int { return fooCount + count }` silently became `fooCount + fooCount`. Writing a full renamer was rejected in favour of withholding; a withheld fix costs one manual edit, a wrong one is a bug the linter cannot see. `spec/rename_sound.fsl` and `spec/rename_siblings.fsl` model the resolution order and the sibling collision.

The conditions, each conservative:

- the new name is not in package scope, not predeclared (`types.Universe`), and not bound in **any** file scope (`fileScopesBind`) — Go rejects a package-level name that any file imports, so this cannot be limited to files with references
- at every ident naming the object, `pass.Pkg.Scope().Innermost(pos).LookupParent(newName, pos)` finds nothing. Verified experimentally: this catches parameters, named results, locals declared before the reference, closure parameters, range variables, type parameters, receivers, the same file's imports and the universe; it correctly ignores locals declared after the reference, field names and labels; and it does **not** see another file's imports, which is why the file-scope check above is separate
- the object is not named from a file the pass did not collect (generated or `exclude`d), since those are never rewritten (`usedOutside`, normalised the same way as `collectRefs`)
- no `//go:linkname` or `//export` in the package names the object as text
- no earlier fix in the same pass has claimed the name (`reserved`). Fixes are generated from one pre-fix state and cannot see each other; `Qualify` is not injective across namespaces and `Unqualify` lowers initialisms, so two declarations can target one name. Reservation makes the **order** of targets load-bearing, which is why `report` sorts by (file name, offset) rather than `token.Pos`: go/packages parses files concurrently, so the order files enter the FileSet differs between runs, and sorting by `Pos` made a different sibling win in the `-json` run than in the `-fix` run
- the package has no in-package `_test.go` files that this pass does not see (`hasUnseenTests`, a directory listing plus `parser.PackageClauseOnly`). The non-test variant cannot see what test files declare or use, so it withholds every rename and defers to the test variant, which sees every file and whose fix rewrites the non-test files too. The driver coalesces identical edits from both variants, so with `-test` on (the default) nothing is lost; with `-test=false`, packages with tests get no rename fix. An earlier draft parsed the sibling test files and summarised what they spell, import and would themselves be renamed to; that was more permissive but heuristic (an unaliased import's name is only known by loading it), and the blunt rule is the one that is provably consistent between the two variants.

Renames are also never offered for members. A namespace that cannot be a label at all (`namespace.IsLabel` is false) produces no naming diagnostic in the first place, rather than a diagnostic without a rename.

**A declaration whose scope came from a directive gets no fix at all** (`t.dir.HasScope`). Both the directive and the use site are deliberate, so `-fix` must not overwrite the author's directive. An earlier version emitted `//declscope:package` next to an existing `//declscope:file`, producing code the linter itself rejected.

`namespace.Qualify` prepends blindly, so its suggestion can stutter when the name already contains the namespace word (`defaultBaselineName` → `baselineDefaultBaselineName`). The fix is a suggestion; a human renaming it to `baselineDefaultName` is expected and fine.

`directiveFix` checks whether the anchor starts its line (`startsLine`, via `pass.ReadFile`). A field of a single-line struct does not, and inserting the directive there attached it to the `struct {` line instead of the field.

## Baseline

`declscope baseline ./...` regenerates baseline files wholesale; they are never hand-edited. Entries are keyed by (package, rule, declaration) — deliberately not by position — so they survive code motion and file renames.

Generation does not go through `singlechecker`: a baseline entry has to identify a violation structurally, and a driver only returns rendered diagnostics. The analyzer declares no `Requires` and exports no facts, so `cmd/declscope/baseline.go` drives it over `go/packages` with a hand-built `analysis.Pass`, calling `internal.Collect`. Keep that entry point working if the analyzer ever gains dependencies.

**The target is resolved per package, on the analyzer's lookup path.** The analyzer finds a baseline from each package's own directory (`config.Resolve`: the file the nearest config names, else `FindBaseline`'s upward search for a default-named file), so a baseline only suppresses what is written where that lookup ends. An earlier version resolved one output path from the working directory and poured every package into it, which left a subtree with its own configured or default-named baseline reporting forever (#8). `collect` therefore groups keys by target: the configured path from `config.ResolveForBaseline`, else `config.DefaultBaseline(dir, cwd)`, which walks the same path as `FindBaseline` but **stops at the working directory** — the nearest existing default-named file below cwd, else a new file in cwd. Stopping there is what makes the README's "the working directory" true and prevents the other data loss (#9): an upward search from a subdirectory found the module-root baseline and rewrote it with only that subtree's entries. A file above cwd is left alone; a new file in cwd shadows it for exactly the packages under cwd. A package whose lookup never reaches cwd (another module in a workspace, or a directory outside it) makes the whole run refuse, naming the packages, because a file written in cwd would never be found from there. `-o` bypasses all of this and gathers everything into one file. Refusing when packages disagreed was the alternative considered; it would have made the documented "its presence is all it takes" depend on the layout instead of holding.

**Regeneration never loads the existing baseline.** `Options.Compile` used to load it, so both the output-path resolution and the per-package resolve read a file that the run was about to replace, and a baseline that failed to parse blocked its own regeneration — the one remedy the documentation offers (#15). Loading now happens in `config.Resolve`, which analysis uses; `config.ResolveForBaseline` applies the same config and hands the configured path back without reading it. `baseline.Save` returns the number of entries written, after deduplication, and that is the count the subcommand reports: a package and its test variant hand over the same key twice (#17).

The analyzer deliberately does **not** report unmatched baseline entries. A package's test variant sees references the ordinary variant does not, so an entry that matched nothing in one pass is not evidence it is stale. Regeneration is what prunes, and its diff is the record of what was fixed.

## Testing

```bash
go test ./...          # analysistest + unit tests
./test_all.sh          # tests, golangci-lint, and dogfooding
```

- `testdata/src/*` are `analysistest` packages. `promotealways/`, `demote/` and `demoteinert/` carry their own `.declscope.yaml`, which also exercises config discovery end to end.
- Goldens are plain files, not txtar archives, because no diagnostic carries alternative fixes any more.
- `convergence_test.go` applies `-fix` through the real binary in **one** pass and checks that the result type-checks, that every diagnostic which offered a fix is gone, and that no diagnostic appeared that was not there before. It type-checks with `go vet`, not `go build`, because vet also compiles the test variant — a rename applied to the non-test files alone builds and then fails the first `go test`. Keep it to one pass: repeating it would hide a fix that only works the second time. Each way a rename can be unsafe has a case there, arranged so that the wrong rename fails to type-check (a captured parameter is given a type the expression rejects), since the test cannot run the result.
- `testdata/src/fix*` pin where a fix is offered and where it is withheld. `RunWithSuggestedFixes` compares a golden only for files that received edits, so a package that tests withholding also carries one declaration that *is* renamed: a wrongly offered fix then fails for want of a golden, and the golden shows the guard is precise rather than merely off.
- Keep each testdata package focused on one rule. `promoterule/order.go` deliberately touches nothing in namespace `user`, so the label rule is tested without escape diagnostics landing on the same lines.
- Diagnostics on directives are reported at the comment, so their `// want` comments belong on the directive line, not the declaration line.
- Do not add a `.declscope.yaml` or `.declscope-baseline.yaml` at the repository root: both are found by an upward lookup from each analyzed package, so a default-named file at the root would reach every `testdata` package and change what the tests assert. The settings declscope holds itself to live in `.declscope-strict.yaml` and are applied with an explicit `-config` in CI and `test_all.sh`.
- `testdata/src/baselined` carries its own `.declscope-baseline.yaml`, which also exercises the lookup end to end.

## Conventions

- Module: `github.com/mpyw/declscope`, matching the layout of `mpyw/gormreuse` and `mpyw/zerologlintctx`.
- `go.mod` pins `toolchain go1.27.0` while keeping the `go` directive at 1.25.0, because golangci-lint refuses to load a module whose `go` directive is newer than the Go it was built with.
- Distribution is via GitHub Releases (goreleaser) with **mise as the recommended install path**; `go install` / `go tool` / `go vet -vettool` also work.
