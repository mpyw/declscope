# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project overview

**declscope** is a Go linter that adds a pseudo visibility level below `unexported`, and a real one for methods and struct fields, and enforces them with [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis). It exists so that a package can stay **flat** without losing every internal boundary.

| Scope | Meaning |
| --- | --- |
| `package` | Usable anywhere in the package — what Go's unexported already means |
| `private` | Usable only inside its own **namespace** |

**Exportedness decides the default and nothing else.** An exported declaration resolves to `package` — no boundary — unless a directive gives it one. Do not reach for "is it reachable from outside the package": a single-package analysis cannot answer it, since a type escapes through an exported signature, an embedding, an alias, or an interface it satisfies, and an earlier attempt to guess reported boundaries on API every importer reaches. What the author knows, the author states: a directive binds whatever it reaches, exported or not, which is how a DTO capitalized for a serializer is protected. A use outside the package is never analyzed, so `public` would be a level nothing could check; there is no `defaults.exported`, and adding one back means claiming an enforcement the analysis cannot perform. `package main` is not an exception either: `-buildmode=plugin` makes its exported symbols reachable by name from another program (verified by building one and looking the symbol up), so applying `defaults.unexported` to them would report a boundary on every plugin's surface.

**Members and methods are not one thing.** A member is written *inside* its type's declaration: a struct field, and an interface's method name. Its file is where the type is, not a binding we chose, so the type's directive contains it the way a `var (...)` block contains its specs. A method with a receiver is an ordinary top-level declaration — its file gives it its namespace, and nothing above it reaches it. Binding methods to their type's file instead made a method unusable from the file that wrote it, with no scope able to express what the author meant. Encapsulation never rested on it: a method reaching another namespace's private member is caught by the **member**. Satisfying an interface is not a use of its method names, and counting it as one would make the sealed-interface idiom a violation everywhere, repairable only by a directive that changes nothing about Go semantics.

### Core concept: reach is stated, ownership is named

The single most important thing to understand before changing anything here is that **the name never determines reach**.

Everything unexported is private to its namespace unless a directive or `defaults.unexported` widens it; a **name** never does. A namespace prefix must never mean package-internal: that overloads one signal with two meanings, so a prefix added purely for legibility would silently widen a declaration, and a codebase that prefixed everything for readability would end up with nothing protected. The same separation is what keeps a boundary crossing from ever having a rename fix, and a diagnostic from ever carrying two conflicting alternatives.

The prefix does a separate job: it is an **ownership prefix** on package-level declarations, making the owning unit legible at every use site. It applies to unexported declarations, and to exported ones when `rules.naming.exported` is on — inside the package an exported name is read as bare as any other, so the package qualifier that explains an external use is absent exactly where the prefix is wanted. A rename carries exportedness across in both directions (`Load` → `UserLoad`, `UserID` → `ID`): one that quietly unexported a declaration would delete the API to satisfy a linter. An exported rename is never *offered*, only reported, since the uses outside the package cannot be seen.

`rules.naming` holds the two mirror rules plus `exported`, which widens both of them. It is the one deliberate cross-cutting key and it **broadens** what is reported. A cross-cutting key that switches reporting *off* — a `members: false` gating the collection of methods and fields — is the shape to avoid: it silences more than its name suggests, and it is why reach enforcement is not configurable at all.

`rules.naming.qualify` reads an `internal.Mode` — `Always`, `Never` or `OnDemand` — and `Mode.Applies` is the predicate it gates on. It defaults to `ondemand`, requiring the prefix only once a package has a second namespace; in a package with one, every other rule is structurally inert anyway, since every reference is already inside the single namespace. It declares the values it accepts as an `internal.ModeSet`, so a rejected value is answered with what that setting accepts, and `Mode.String()` returns the same spelling the config takes.

`rules.naming.unqualify` and `rules.naming.exported` are plain booleans, defaulting to `false`. Do not give `unqualify` a third value: `qualify` already answers *when* a prefix applies, so all this key decides is whether the other direction is enforced too, and where the prefix is required it is inert by construction.

A file marked `//declscope:core` joins the package's core namespace, whose prefix is empty, so both naming rules skip it. Several files may share the core; `core` and `namespace` on one file conflict, since a core file's namespace *is* the core.

**Members are exempt from the naming rule.** They are already namespaced by the type that owns them and cannot collide, so a prefix would produce `u.userSave()`, exactly the stutter Go idiom avoids. Their problem is encapsulation, not naming, and a member violation is never fixed by renaming.

Scope resolution is shared: `collection.bind` (`internal/scopesite.go`) serves every kind, so that `defaults.unexported`, the file-level directive and the declaration's own govern every declaration in a package. A **field** takes one extra step first, from the type that contains it; a method takes none. A knob that works on some declarations and not others is the shape to avoid.

### Namespaces

A namespace is the unit of privacy, defaulting to the camelCased file name so that each file is its own namespace, and overridable with `//declscope:namespace <name>` before the package clause.

The namespace count that `rules.naming.qualify: ondemand` keys off is taken from the package's **non-test** files (`collection.namespaces`). A test file joins its subject's namespace rather than creating a boundary, and counting one whose name matches no source file (`integration_test.go`) would make a package's test variant disagree with the package itself.

The indirection is deliberate: using the file name *itself* would mean renaming a file cascades into renaming every identifier it declares. It also makes `_test.go` sharing its subject's namespace fall out naturally rather than needing a special case.

Normalization rules live in `internal/namespace` and are covered by a table test. GOOS/GOARCH suffixes are stripped because they are build constraints, not namespaces. Every run of non-identifier characters is a separator (`foo-bar.go` → `fooBar`), a PascalCase stem is lowered (`Foo.go` → `foo`), and an initialism in a later segment is spelled the way Go spells it (`user_id.go` → `userID`) using golint's `commonInitialisms` list, vendored rather than pulled in as a dependency.

**A namespace is an identity first and a prefix second, and the two are kept apart.** `namespace.Of` always returns the normalized stem — `2fa.go` yields `2fa` — so that `fileInfo.key()` never falls back to the file path and `2fa_test.go` shares its subject's namespace like every other test. `namespace.CanPrefix` separately says whether that stem can be prepended to an unexported identifier; `checkQualify` and `checkUnqualify` gate on it and stay silent when it is false, because the alternative is suggesting `2faTotp` or `FooBar`. Returning `""` from `Of` for a digit-leading stem would lose the identity in order to protect the prefix; the second job must never kill the first.

`namespace.HasPrefix` matches the prefix **ignoring case** and then requires a word boundary, so `userIDCache`, `userIdCache` and `userIdcache` all carry `userID` while `useridentity` does not: a word break the name reproduces inside a multi-word namespace already confirms the prefix, and only a single word running on in lowercase is a fragment. This is what stops the linter guessing which spelling of an initialism the author chose and demanding `userIdUserIDCache`. `Qualify` spells the first word of the name the same way (`id` → `userID`).

## Architecture

```text
analyzer.go               Analyzer definition, -config flag, config discovery
internal/
  analyzer.go             core. Run: collect files -> targets -> refs -> report
  subject.go              core. The model every stage shares: fileInfo, target,
                          ref, kind, the index over them, and the problem sink
  collect.go              builds the index: files, targets, references
  options.go              resolved configuration: the booleans and the exclude globs
  mode.go                 Mode, the vocabulary rules.naming.qualify is spelled in
  scopesite.go            scope resolution (bind), and which directives bound nothing
  report.go               diagnostics and suggested fixes
  ignore.go               ignore accounting: which comment silenced what, and which bound to nothing
  rename.go               the conditions under which a rename fix is offered at all
  rule/                   the rule vocabulary, shared by diagnostics, config, baseline and ignores
  namespace/              file name -> namespace, prefix matching, qualify/unqualify
  scope/                  the two-level Scope enum
  directive/              //declscope:... comment parsing
  config/                 YAML loading and lookup
  baseline/               baseline file format, lookup and regeneration
cmd/declscope/            singlechecker entry point, plus the `baseline` subcommand
```

Each file of `internal/` is its own namespace, taken from its stem. `analyzer.go` and `subject.go` join the **core** instead: the model they hold is read by every stage, and in a named namespace each type would have to carry that namespace's prefix, so the builder's name would be spelled into the model every other file reads.

Each stage's own working state is embedded into `collection` from a struct
declared in the file that owns it: `ignoreBook`, `scopesiteBook`, `collectBook`,
`renameBook`. The type carries `//declscope:package` so the core can embed it,
and the field carries `//declscope:private` to narrow itself back — the type's
directive would otherwise reach the field and undo the point. Embedding rather
than nesting keeps the call sites spelling `c.ignores`, and reaching one from
the wrong stage is a boundary crossing.

The maps are filled on first use by the owning file, so the constructor in
collect.go never writes another stage's state. `//declscope:package` on a
constructor would have worked too, but the lazy form is what rename.go already
did.

`subject.go` carries a file-level `//declscope:package` because the shared model is shared on purpose. Everything else that crosses a namespace says so one declaration at a time, and those directives are the record of what each stage hands to another. `cmd/declscope/baseline.go` is its own namespace for the same reason; only `main.go` is `main`.

> [!IMPORTANT]
> Keep those directives when adding files to either unit. Without them the naming rule asks every declaration to carry a `baseline`/`analyzer` prefix, which is the tool reporting a boundary that is not really there.

### Structural constraints

- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified; `pass.Pkg` scoping does that for free and no `analysis.Fact` is needed.
- Whether an *exported* identifier is used outside its package is deliberately **out of scope**: `go/analysis` has no upward view of the program. Answering it would require a separate whole-program mode driven by `packages.Load`, which would not fit a plain Analyzer. Combine with an unused-code linter instead.
- Generated files are excluded as declaration sites **and** as reference sites, since a violation in generated code is not actionable.

### Reference collection

`collectRefs` is where every rule gets its evidence, and two facts about `go/types` shape it:

- **An ident can be a definition and a use at once.** An embedded field's ident is in `Defs` as the field `Var` *and* in `Uses` as the `TypeName`. `Defs` and `Uses` are therefore consulted independently; returning after a hit in `Defs` would drop the type use and with it both a rename edit and a `boundary` diagnostic. The embedded field itself is deliberately **not** a target (`addFields` skips it) — it has no name of its own to hide or prefix — but a selection through it (`u.count`) is spelled with the type's name, so `embeddedTypeName` adds those idents to the type's rename set. Aliases are kept as aliases there: a field embedding `A` is spelled `A`, whatever `A` denotes.
- **`Uses` records the instantiated member of a generic type**, for a selection on `List[int]` and even on `List[T]` inside `List`'s own methods, while `byObj` is keyed by the declared object. Every object taken from `Uses` goes through `origin()` (`(*types.Var).Origin()` / `(*types.Func).Origin()`) before the lookup and before it joins `idents`. Without that, every field and method of a generic type would be silently unchecked. `receiver` needs no such step: `(*types.Named).Obj()` already names the origin's type name.

`testdata/src/generics`, `embedded` and `fixembedded` pin both, including the receiver resolution of a method on `List[T]`.

## Rules

`internal/rule` holds the one vocabulary. The same name is the diagnostic's `Category`, the `Rule` field of a baseline key, and what an ignore directive targets. **Adding a rule means adding it there**, not inventing a string at the report site.

| Rule | Reports | Fix |
| --- | --- | --- |
| `boundary` | A declaration used from outside the namespace it is private to | Insert `//declscope:package`, unless the **declaration's own** directive states the scope |
| `qualify` | A package-level declaration missing its namespace prefix | Rename via `namespace.Qualify`, when provably safe and not exported |
| `unqualify` | A namespace prefix present where it is not required | Rename via `namespace.Unqualify`, when derivable, provably safe and not exported |
| `directive` | A directive that binds nothing, or that is malformed or misplaced | None |

`qualify` and `unqualify` are exclusive **by construction**: `checkUnqualify` returns early wherever `opts.Qualify.Applies(c.namespaces)` holds, so they can never contradict each other on one declaration. Preserve that property when adding checks.

There is no declaration-site rule for a method grown on another namespace's type. `checkBoundary` reports a method at its *declaration*, not at the call, so such a method is already caught wherever it is used — a declaration-site rule would only add a second diagnostic at the same position. The one case it would cover alone is a foreign method that is never called, which is dead code and an unused-code linter's business, the same reasoning that keeps "exported but unused outside the package" out of scope.

`namespace.Unqualify` (unqualify's rename) lowers a leftover initialism the way Go spells one (`userID` → `id`, `userURLPath` → `urlPath`, never `iD`). It returns a second value explaining any refusal.

`checkUnqualify` exempts a name identical to its namespace. The causality usually runs the other way there — `user.go` is named after the `user` it declares — so there is no prefix to strip, and the only advice available would be "rename it by hand". `qualify` accepts such a name too.

**Not being able to derive a rename is never a reason to stay silent.** `checkUnqualify` gates on `namespace.HasPrefix` — whether there is a prefix at all — and then reports either way, embedding `Unqualify`'s reason when it has no suggestion. Skipping the declaration would leave a codebase half-converted under `unqualify: true` with nothing saying why. The same principle is why an exported declaration is reported under `rules.naming.exported` even though no rename can ever be offered for it. `checkQualify` behaves the same way when its rename target is taken; keep new rules consistent with both.

## Directives

| Directive | Level | Effect |
| --- | --- | --- |
| `//declscope:package` | Declaration or file | State the scope instead of deriving it from the defaults. On a type it also reaches the type's fields, not its methods; before the package clause it is the file's default, still overridable per declaration |
| `//declscope:private` | Declaration or file | Same |
| `//declscope:ignore` | Declaration or file | Silence every rule for the declaration, or for the whole file |
| `//declscope:ignore <rules>` | Declaration or file | Silence only the named rules (`boundary`, `qualify`, `unqualify`, `directive`) |
| `//declscope:core` | File, before the package clause | Join the package's core namespace; conflicts with `//declscope:namespace` |
| `//declscope:namespace <name>` | File, before the package clause | Override the namespace derived from the file name |

`parseIgnore` is shared by both levels, so `//declscope:ignore` cannot come to mean different things depending on where it is written. File-level directives live on `fileInfo.ignores`; declaration-level ones on `Decl.Ignores`. A file-level ignore is scoped to its **file**, not to its namespace, so files sharing a namespace each need their own — one file silently changing another's diagnostics would be much harder to trace back.

### The suppression chain

Scope resolution walks the same levels in the same order — the declaration, the containing type for a field, then the file the declaration is written in. The two chains share an **order**, not a rule: scope resolution takes the first hit, suppression consults all of them. Keep the order aligned when either changes.

`bind` marks a directive bound only when the scope it names is one the declaration could not have had under **any** configuration (`inert`, same file). That quantifier is the whole of why the unused-directive test does not read `defaults.unexported`: comparing against it directly would flip every directive in the tree when one line of YAML changed, and would make recording a deliberate `private` an error.

`collection.silenced` walks the levels: the declaration, then, **for a field**, the type that owns it (`target.ownerObj`), then the file. It consults **all** of them rather than stopping at the first hit, and `ignored()` marks **every** directive covering the rule as used, so overlapping directives at different levels never make each other look unused.

Unused directives are therefore reported in a **second pass**, after every finding has been seen: a type's directive is often used up by a member reached later in the target list.

**Accounting is per physical directive, never per target.** `Decl.Merge` copies a block's ignore into every spec, and one `dir` is shared by every name in `var a, b` and `x, y int`, so a per-target "used" flag would judge one comment N times and report it unused whenever any sibling did not need it. `collection.ignores` keys an `ignoreSite` by the directive's position; `add` names every target the directive reaches on it, `ignored` marks the site used, and `reportUnusedIgnores` reports each unused site once, listing the declarations it was written for. Every declaration-level comment group goes through `collection.parseDecl`, which registers the site, records problems once per comment (so a bogus directive on a block is reported once, not once per spec), and marks the group **consumed** — see below.

**Only a pass that sees every reference may call a directive unused.** The ordinary variant of a package with in-package `_test.go` files cannot see the references those files make, so a directive needed only by a test would be reported unused there and reported necessary by the test variant. `reportUnusedIgnores` returns early when `hasUnseenTests` holds — the same predicate that withholds renames, kept on `collection` because both features ask it — and defers to the test variant, on exactly the reasoning that keeps unmatched baseline entries unreported (below). A more permissive rule — report in the ordinary variant unless the test variant *could* need it — would mean parsing the sibling test files for references, which is the heuristic path the rename guard also rejects.

> [!NOTE]
> The consequence, documented in the README, is that `-test=false` yields no unused-ignore report for packages with in-package tests.

Two reports live here and must not be conflated. **Misplaced** is structural: go/parser attached the comment to no node. **Unused** is about reach: the directive is attached correctly but nothing it covers takes its scope from it (`spec/directive_effect.fsl`). That test is deliberately structural rather than semantic — asking whether the scope *differs* from the one already in force would make recording a deliberate `private` an error, and would turn one line of `.declscope.yaml` into hundreds of diagnostics in untouched files.

**A directive that binds to nothing is reported as misplaced, never dropped.** go/parser attaches a comment such as the one in `type X struct { //declscope:ignore boundary` to nothing, so the contract has two halves. Binding: `looseTrailing` finds an unattached comment trailing a node's **first or last line**, and `addFunc`, `addGenDecl` (for the block's own `(`/`)` lines) and `specGroups` read it; `attached` lists the groups the parser did hang on something inside the node, which win, so a field's trailing comment on the brace line stays the field's — and `specGroups` parses only the spec's *own* Doc/Comment plus the loose groups, never the fields', because feeding the fields' groups into the type's parse lands a field's `//declscope:package` on the type (the convergence test catches this). Reporting: `collection.stray` walks every comment group after the package clause that `parseDecl` never consumed and reports each directive in it through `directive.Stray`. The first-or-last-line rule is deliberately blunt: a directive on a middle line of a multi-line signature or struct is misplaced, because the alternative is guessing which declaration a comment "near" one belongs to.

### Placement

`//declscope:namespace` matches Go's directive syntax, so `go/doc` strips it from rendered documentation. In a file with a package comment it belongs at the bottom of that comment after a blank `//` line; in a file without one it is separated from the package clause by a blank line, because flush against `package` it becomes an empty package comment and adds a stray blank line to the rendered package doc. `directive.ParseFile` scans `file.Comments` rather than `file.Doc`, so every placement is recognized.

Declaration-level directives go in a declaration's doc comment, or in a trailing comment on its first or last line. `ast.FuncDecl` has no `Comment` field, and no node owns a comment after an opening brace or parenthesis, so those are found through `fileInfo.lineComments` (`looseTrailing`). A directive on a parenthesized block applies to every spec in it, and `directive.Decl.Merge` layers the spec's own over it: a scope directive on the spec replaces the block's, while ignores accumulate. The union is deliberate — a narrower ignore must not silently re-enable a rule the block turned off — and `testdata/src/ignorescope` pins it, so keep the docs and that test in step.

Unused ignore directives are reported, matching the convention in `mpyw/gormreuse` and `mpyw/zerologlintctx`. Ignores are consulted **before** the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

## Suggested fixes

Every diagnostic carries **at most one** fix, which is what makes `-fix` unambiguous:

- A boundary crossing is fixed by inserting `//declscope:package`.
- A missing prefix is fixed by renaming.

They can never conflict, because a rename does not change reach. (`x/tools`' `ApplyFixes` applies only the first fix of a diagnostic and logs `ignoring alternative fix` for the rest, so carrying alternatives is a liability.)

### Why the rename fix withholds

**A rename is offered only when it is provably safe** (`renameSafe` in `internal/rename.go`); the diagnostic is reported either way. A guard that checks only `pass.Pkg.Scope().Lookup(newName)` produces code that does not compile or, worse, compiles into a program computing something else: Go resolves a name from the inside out, so `var count` renamed to `fooCount` inside `func Add(fooCount int) int { return fooCount + count }` silently becomes `fooCount + fooCount`. A full renamer is rejected in favor of withholding; a withheld fix costs one manual edit, a wrong one is a bug the linter cannot see. `spec/rename_sound.fsl` and `spec/rename_siblings.fsl` model the resolution order and the sibling collision.

The conditions, each conservative:

- The new name is not in package scope, not predeclared (`types.Universe`), and not bound in **any** file scope (`fileScopesBind`) — Go rejects a package-level name that any file imports, so this cannot be limited to files with references.
- At every ident naming the object, `pass.Pkg.Scope().Innermost(pos).LookupParent(newName, pos)` finds nothing. This catches parameters, named results, locals declared before the reference, closure parameters, range variables, type parameters, receivers, the same file's imports and the universe; it correctly ignores locals declared after the reference, field names and prefixes; and it does **not** see another file's imports, which is why the file-scope check above is separate.
- The object is not named from a file the pass did not collect (generated or `exclude`d), since those are never rewritten (`usedOutside`, normalized the same way as `collectRefs`).
- No `//go:linkname` or `//export` in the package names the object as text.
- No earlier fix in the same pass has claimed the name (`reserved`). Fixes are generated from one pre-fix state and cannot see each other; `Qualify` is not injective across namespaces and `Unqualify` lowers initialisms, so two declarations can target one name. Reservation makes the **order** of targets load-bearing, which is why `report` sorts by (file name, offset) rather than `token.Pos`: go/packages parses files concurrently, so the order files enter the FileSet differs between runs, and sorting by `Pos` would make a different sibling win in the `-json` run than in the `-fix` run.
- The package has no in-package `_test.go` files that this pass does not see (`hasUnseenTests`, a directory listing plus `parser.PackageClauseOnly`). The non-test variant cannot see what test files declare or use, so it withholds every rename and defers to the test variant, which sees every file and whose fix rewrites the non-test files too. The driver coalesces identical edits from both variants, so with `-test` on (the default) nothing is lost; with `-test=false`, packages with tests get no rename fix. Parsing the sibling test files to summarize what they spell, import and would themselves be renamed to would be more permissive but heuristic (an unaliased import's name is only known by loading it); the blunt rule is the one that is provably consistent between the two variants.

Renames are also never offered for members. A namespace that cannot be a prefix at all (`namespace.CanPrefix` is false) produces no naming diagnostic in the first place, rather than a diagnostic without a rename.

**A declaration whose own directive states its scope gets no fix** — `t.dir.HasScope` must consult the declaration level only. Both that directive and the use site are deliberate, so `-fix` must not overwrite the author's decision. A scope inherited from the containing type or from the file is a *default*, and the fix overrides it: the inserted directive sits on the declaration, which outranks both, so it states an exception rather than overwriting anything. `spec/boundary_fix.fsl`'s `FixOverridesTypeAndFile` pins that, and its `trans` guards pin that no fix is offered where no violation exists.

`namespace.Qualify` prepends blindly, so its suggestion can stutter when the name already contains the namespace word (`defaultBaselineName` → `baselineDefaultBaselineName`). The fix is a suggestion; a human renaming it to `baselineDefaultName` is expected and fine.

`directiveFix` checks whether the anchor starts its line (`startsLine`, via `pass.ReadFile`). A field of a single-line struct does not, and inserting the directive there would attach it to the `struct {` line instead of the field.

## Baseline

`declscope baseline ./...` regenerates baseline files wholesale; they are never hand-edited. Entries are keyed by (package, rule, declaration) — deliberately not by position — so they survive code motion and file renames.

Generation does not go through `singlechecker`: a baseline entry has to identify a violation structurally, and a driver only returns rendered diagnostics. The analyzer declares no `Requires` and exports no facts, so `cmd/declscope/baseline.go` drives it over `go/packages` with a hand-built `analysis.Pass`, calling `internal.Collect`. Keep that entry point working if the analyzer ever gains dependencies.

**The target is resolved per package, on the analyzer's lookup path.** The analyzer finds a baseline from each package's own directory (`config.Resolve`: the file the nearest config names, else `FindBaseline`'s upward search for a default-named file), so a baseline only suppresses what is written where that lookup ends. Resolving one output path from the working directory and pouring every package into it would leave a subtree with its own configured or default-named baseline reporting forever. `collect` therefore groups keys by target:

| Situation | Where the package's entries go |
| --- | --- |
| `-o <file>` given | That one file, for every package |
| The nearest config names a baseline | The named file, resolved against the config (`config.ResolveForBaseline`) |
| A default-named file exists between the package and the working directory | The nearest such file (`config.DefaultBaseline`) |
| Otherwise, and the walk reaches the working directory | A new `.declscope-baseline.yaml` in the working directory |
| The walk never reaches the working directory | The whole run refuses, naming the packages |

`config.DefaultBaseline` walks the same path as `FindBaseline` but **stops at the working directory**. Stopping there is what makes the README's "the working directory" true and prevents data loss: an upward search from a subdirectory would find the module-root baseline and rewrite it with only that subtree's entries. A file above cwd is left alone; a new file in cwd shadows it for exactly the packages under cwd. A package whose lookup never reaches cwd (another module in a workspace, or a directory outside it) makes the whole run refuse, because a file written in cwd would never be found from there. `-o` bypasses all of this and gathers everything into one file. Refusing whenever packages disagree is the alternative; it would make the documented "its presence is all it takes" depend on the layout instead of holding.

**Regeneration never loads the existing baseline.** Loading happens in `config.Resolve`, which analysis uses; `config.ResolveForBaseline` applies the same config and hands the configured path back without reading it, and `Options.Compile` does not load it either. Otherwise a baseline that fails to parse would block its own regeneration — the one remedy the documentation offers. `baseline.Save` returns the number of entries written, after deduplication, and that is the count the subcommand reports: a package and its test variant hand over the same key twice.

The analyzer deliberately does **not** report unmatched baseline entries. A package's test variant sees references the ordinary variant does not, so an entry that matched nothing in one pass is not evidence it is stale. Regeneration is what prunes, and its diff is the record of what was fixed.

## Testing

```bash
go test ./...          # analysistest + unit tests
./test_all.sh          # tests, golangci-lint, and dogfooding
```

- `testdata/src/*` are `analysistest` packages. Several carry their own `.declscope.yaml` (`qualifyalways/`, `unqualify/`, `unqualifyinert/`, and the `fix*` packages that need a non-default rule), which also exercises config discovery end to end.
- Goldens are plain files, not txtar archives, because no diagnostic carries alternative fixes.
- `convergence_test.go` applies `-fix` through the real binary in **one** pass and checks that the result type-checks, that every diagnostic which offered a fix is gone, and that no diagnostic appeared that was not there before. It type-checks with `go vet`, not `go build`, because vet also compiles the test variant — a rename applied to the non-test files alone builds and then fails the first `go test`. Keep it to one pass: repeating it would hide a fix that only works the second time. Each way a rename can be unsafe has a case there, arranged so that the wrong rename fails to type-check (a captured parameter is given a type the expression rejects), since the test cannot run the result.
- `testdata/src/fix*` pin where a fix is offered and where it is withheld. `RunWithSuggestedFixes` compares a golden only for files that received edits, so a package that tests withholding also carries one declaration that *is* renamed: a wrongly offered fix then fails for want of a golden, and the golden shows the guard is precise rather than merely off.
- Keep each testdata package focused on one rule. `qualifyrule/order.go` deliberately touches nothing in namespace `user`, so the naming rule is tested without boundary diagnostics landing on the same lines.
- Diagnostics on directives are reported at the comment, so their `// want` comments belong on the directive line, not the declaration line.
- `analysistest` analyzes **both** variants of a package with `_test.go` files and checks every `// want` in each, so a `want` in a non-test file must hold in both. A diagnostic that only the test variant reports can be pinned only from inside a `_test.go` file, which the ordinary variant never checks; `testdata/src/testonlyignore` is shaped that way. Deferral itself is pinned by the *absence* of a report.
- `testdata/src/blockignore`, `testonlyignore` and `braceignore` pin the three halves of the unused-directive contract (one comment judged once; only a complete pass judges; a directive binds on its first or last line or is reported misplaced). A sink like `var _ = userE` written in another namespace is a cross-namespace **reference** and will use up the very directive the test means to show unused — keep sinks in the declaring file.
- `testdata/src/baselined` carries its own `.declscope-baseline.yaml`, which also exercises the lookup end to end.
- `testdata/src/braceignore/user.go` is deliberately not gofmt-clean; it is fixture data for comment placement, so exclude it when checking `gofmt -l .`.

> [!WARNING]
> Do not add a `.declscope.yaml` or `.declscope-baseline.yaml` at the repository root. Both are found by an upward lookup from each analyzed package, so a default-named file at the root would reach every `testdata` package and change what the tests assert. The settings declscope holds itself to live in `.declscope-strict.yaml` and are applied with an explicit `-config` in CI and `test_all.sh`.

### Formal specs

`spec/*.fsl` are machine-checked models of the rules, the configuration space and the rename guard; `spec/README.md` lists what each proves and the exact `fslc` commands and depths. `spec/verify.sh` runs them — four must be `proved` under induction, two must stay `violated` — and is wired into `test_all.sh` and a CI job. Documentation that nothing re-runs is documentation that drifts.

> [!CAUTION]
> `fslc verify` is a bounded model checker that holds the whole reachable state space in memory. A spec that models the full product in one action with many parameters needs gigabytes for the same claims the split specs prove in single-digit megabytes. Keep each spec to the variables its own properties read, keep the depths given in `spec/README.md`, and do not run `fslc verify` on a machine that cannot spare the memory.

## Conventions

- Module: `github.com/mpyw/declscope`, matching the layout of `mpyw/gormreuse` and `mpyw/zerologlintctx`.
- `go.mod` pins `toolchain go1.27.0` while keeping the `go` directive at 1.25.0, because golangci-lint refuses to load a module whose `go` directive is newer than the Go it was built with.
  The pin governs builds made inside this module, including the release build. It does not reach `go install pkg@version` or `go run pkg@version`, which ignore a dependency module's `toolchain` directive and treat its `go` directive as a lower bound only, so those commands build declscope with the user's own `go` release.
- Distribution is via GitHub Releases (goreleaser) with **mise as the recommended install path**; `go install` / `go tool` / `go vet -vettool` also work.
