# declscope

[![CI](https://github.com/mpyw/declscope/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/declscope/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/declscope.svg)](https://pkg.go.dev/github.com/mpyw/declscope)

Keep your Go packages **flat** without letting them turn into a free-for-all.

declscope adds a visibility level between `Exported` and `unexported`, plus a real `private` for methods and struct fields, and enforces both statically.

## Why

Go's advice is to prefer few, large packages. Nesting packages to manufacture encapsulation buys a boundary at a real price: import cycles, interfaces duplicated to break them, stutter (`user.UserRepository`), and a directory tree that stops reflecting the domain. Most Go codebases are better off flat.

The bill for flat comes due *inside* the package, because Go has two visibility levels and the lower one spans the whole thing:

```go
// user.go
func helper() int { ... }          // visible to every file in the package

type User struct {
    name string                    // visible to every file in the package
}
func (u *User) normalize() { ... } // visible to every file in the package
```

In a flat package of any size, every helper becomes a package-wide name and every field a package-wide reach. The boundary that keeps this workable — *this helper belongs to this file, don't reach for it from over there* — is real, but it lives only in the heads of the people who wrote the package. The compiler knows nothing about it.

### That convention does not survive an AI agent

An agent reading the package sees `helper()` in scope, so it calls it. It sees `u.name`, so it writes to it. Every one of those choices compiles, passes review at a glance, and is locally reasonable. What erodes is the structure, a little at a time, until the package is a mesh and nobody can say which parts were ever meant to be separate.

The usual reaction is to start splitting packages so the compiler will hold the line — paying the whole nesting price to buy back a boundary the code never needed to lose.

declscope takes the other route: it makes the boundary **machine-checkable inside a flat package**, so the agent gets told, with a fix it can apply, and the intent ends up written down in the source where the next agent will read it.

## Namespaces

A **namespace** is the unit declscope enforces privacy within. Everything below is defined in terms of it, so it comes first.

By default a namespace is derived from the file name, which makes **each file its own namespace**:

| File | Namespace |
| --- | --- |
| `user_repository.go` | `userRepository` |
| `user_repository_test.go` | `userRepository` — a test shares its subject's namespace |
| `parser_linux.go`, `parser_linux_amd64.go` | `parser` — GOOS/GOARCH suffixes are build constraints, not namespaces |
| `v2_client.go` | `v2Client` |
| `user_id.go`, `parse_json.go` | `userID`, `parseJSON` — an initialism is spelled the way Go spells it |
| `foo-bar.go`, `Foo.go` | `fooBar`, `foo` — any separator, and a PascalCase stem, normalise to lowerCamelCase |
| `2fa_auth.go` | `2faAuth` — a namespace, but never a label (see below) |

A namespace does two jobs. As an **identity** it answers "is this use inside the same namespace?", and every file with a stem has one — `2fa_test.go` shares the namespace of `2fa.go` like any other test. As a **label** it is the prefix [`qualify`](#qualify) asks a declaration to carry, and only a namespace that can start an unexported identifier qualifies. No identifier begins with a digit, so `2faAuth` bounds its declarations but the naming rules ask nothing of them.

Files can opt into a **shared** namespace, which is how one logical unit spans several files:

```go
//declscope:namespace user
package repo
```

Deriving the default from the file name rather than using the file name itself is deliberate: renaming a file should not cascade into renaming every identifier it declares. It is also why a `_test.go` file can reach its subject's file-private declarations without any special case — it is simply in the same namespace.

### Coexisting with a package comment

`//declscope:namespace` matches Go's [directive syntax](https://go.dev/doc/comment#syntax), so `go/doc` strips it from the rendered documentation. Follow the convention Go prescribes for directives — bottom of the doc comment, preceded by a blank comment line:

```go
// Package repo stores users.
//
//declscope:namespace user
package repo
```

In a file with no package comment, separate the directive from the package clause with a blank line:

```go
//declscope:namespace user

package repo
```

Putting it flush against `package` there also works, but when another file in the package carries the real package comment it leaves a stray blank line in the rendered documentation.

## Scopes

| Scope | Meaning |
| --- | --- |
| `public` | Usable outside the package. |
| `package` | Usable anywhere in the package. |
| `file` | Usable only inside its own namespace. |

Roughly: `file` is Rust's module-private, `package` is `pub(crate)`, `public` is `pub`.

Exported identifiers are `public`. **Everything else starts at `file`**, and widening is always an explicit act:

```go
// user.go   (namespace: user)

func UserLoad()   {} // public: usable outside the package
func userCache()  {} // file: only user.go may touch it

//declscope:package
func userShared() {} // package: usable anywhere in the package
```

### Reach is stated by the directive, never by the name

This is the decision everything else follows from. If a naming convention meant "package-wide", you could not also use one simply to say *which unit a declaration belongs to*: a prefix added for legibility would silently widen it, and a codebase that prefixed everything for readability would end up with nothing protected at all.

Freeing the name of that job is what lets the namespace prefix become an ownership **label** instead, which grants nothing. See [`qualify`](#qualify).

### Members are bounded by their type, not their file

A method or struct field is already namespaced by the type that owns it. `u.save()` cannot collide with anything, so there is no pollution to prevent, and a label here would produce `u.userSave()` — exactly the stutter Go idiom avoids.

What is missing for members is not a namespace but **encapsulation**, which Go cannot express at all: every unexported field is visible to its whole package. So members get the same three scopes, resolved the same way and from the same `defaults`, but bounded by the namespace of their **type** rather than of their file:

```go
// user.go   (namespace: user)
type User struct{ id int }

func (u *User) normalize() {}
```

```go
// order.go  (namespace: order)
func orderUse(u *User) {
    u.id = 1      // reported: User's internals belong to namespace "user"
    u.normalize() // reported
}
```

## Rules

There are three. Each has one name, and that name is what appears as the diagnostic's category, what configures it, what keys its baseline entry, and what an ignore directive targets.

| Rule | Reports | Fix | Configurable |
| --- | --- | --- | --- |
| [`boundary`](#boundary) | a declaration used from outside the namespace it is private to | insert `//declscope:package` | no |
| [`qualify`](#qualify) | an unexported package-level declaration missing its namespace label | rename to add the label | `rules.qualify` |
| [`unqualify`](#unqualify) | a namespace label present where it is not required | rename to drop the label | `rules.unqualify` |

The split is deliberate. **Reach enforcement is the point of the linter and cannot be switched off**; **naming discipline is a matter of taste and can be.** To quiet `boundary`, silence the individual declaration with `//declscope:ignore`, or record what the codebase already has with a [baseline](#adopting-on-an-existing-codebase).

`qualify` and `unqualify` are mirrors and never both apply to one declaration: `unqualify` is inert wherever `qualify` requires the label.

### `boundary`

The core rule: a declaration private to its namespace, used from another one.

```go
// user.go
func userCache() int { return 1 }
```
```go
// order.go
func orderRun() int { return userCache() } // reported
```

```
func userCache is file-private to namespace "user", but is used from namespace "order"
```

It covers package-level declarations and members alike — for a member the boundary is the namespace of its type. The fix inserts `//declscope:package`, the only thing that widens reach.

For a method, the report lands on the **declaration**, so a method grown on a type belonging to another namespace is caught where it is written:

```go
// order.go  (namespace: order) — but User belongs to namespace "user"
func (u *User) normalize() { u.ID++ } // reported here, not at the call
```

A declaration whose scope was **already stated** with a directive is reported without a fix. Both the directive and the use site are deliberate statements, and `-fix` must not silently overwrite the one the author wrote.

### `qualify`

An unexported package-level declaration must carry its namespace as a prefix. It grants nothing; it says who owns it, which is what makes a cross-file call legible at the call site, in a stack trace and in a grep result:

```go
// order.go
func orderRun() int {
    return userShared() // obviously the user unit's, and obviously shared
}
```

Exported identifiers are exempt: they are already qualified by the package name at every external use site. Members are exempt for the stutter reason above.

| `rules.qualify` | Effect |
| --- | --- |
| `ondemand` *(default)* | Required only once a package has a **second namespace**. In a package with one there is no boundary for a label to mark, and a prefix repeated on every declaration would distinguish nothing. |
| `always` | Required unconditionally. Costs a little stutter in single-file packages, but means a package gaining its second namespace is not a mass rename. |
| `never` | Off, leaving reach enforcement without any naming discipline. |

`true` and `false` are accepted as aliases of `always` and `never`.

The label is matched **ignoring case**, and then has to end at a word boundary: in `user_id.go` (namespace `userID`) `userIDCache`, `userIdCache` and `userIdcache` all carry it, while `useridentity` does not. You never have to guess which spelling of an initialism the linter chose. The rename it offers spells both halves the way Go does — `id` in `user.go` becomes `userID`, `urlPath` becomes `userURLPath` — never `userId`.

Where the rename cannot be shown safe — the target is already taken, or renaming would change what the code resolves to — the violation is reported without a fix. See [When a rename is withheld](#when-a-rename-is-withheld). Where the namespace cannot be a label at all (`2fa.go`), `qualify` and `unqualify` stay silent instead: there is no prefix they could ask for.

### `unqualify`

The mirror of `qualify`: where the label is not required, it must not be there. With both on, the spelling of every unexported package-level name is determined in both directions and fixable either way.

Enabling it asserts that a namespace prefix in this codebase *always* means the label — nothing in a name can tell `userID`-the-label from `userID`-the-word, and `unqualify` will offer to rename it to `id`. Where the prefix is part of the concept, say so on the declaration:

```go
//declscope:ignore unqualify
var userID int
```

A name identical to its namespace (`type user` in `user.go`) carries no label to drop: the file is named after what it declares, not the other way about. `qualify` still accepts such a name, since the owning unit is legible from it.

The rename spells a leftover initialism the way Go does (`userID` → `id`, `userURLPath` → `urlPath`). Where no rename can be derived — dropping the label would leave a keyword, say — the violation is still reported, with the reason and without a fix:

```
func userType carries the label of namespace "user", which is not required here,
but "type" is a keyword; rename it by hand
```

Being unable to spell the new name is a limit of the fix, not a reason to let the label stand.

### When a rename is withheld

A rename is offered only when it provably changes nothing but the spelling. Go resolves a name from the inside out — local scope, then the file's imports, then the package, then the predeclared names — so a new name that is free at package level can still be bound at a use site, and the wrong rename **compiles and computes something else**:

```go
var count = 10
func Add(fooCount int) int { return fooCount + count } // Add(1) == 11
```
```go
// after a careless rename of count to fooCount
func Add(fooCount int) int { return fooCount + fooCount } // Add(1) == 2
```

The violation is still reported, but the fix is withheld and the rename left to a human, when any of the following holds:

| Condition | Why |
| --- | --- |
| the new name is already declared in the package | would not compile |
| the new name is predeclared (`len`, `error`, `string`, …) | the declaration compiles and shadows the builtin for the whole package |
| any file of the package imports the new name | Go rejects a package-level name that any file imports |
| at some use of the declaration, the new name is bound by a local, parameter, result or type parameter | the use would silently resolve to that instead |
| the declaration is used from a generated or `exclude`d file | those files are never rewritten, so the use would dangle |
| a `//go:linkname` or `//export` directive names the declaration | the directive names it as text, which a rename cannot follow |
| another fix in the same run already renames something to that name | two declarations would end up with one name (`a.go:bX` and `a_b.go:x` both label to `aBX`; `fooBar` and `fooBAR` both drop to `bar`) |
| the package has in-package `_test.go` files that this variant does not see | the test files may declare or use the name; the test variant, which sees every file, decides and its fix covers the non-test files too |

The last row means that with `-test=false`, no rename is offered in a package that has tests. Under the default `-test=true` nothing changes: `go vet` and `declscope` analyze the test variant as well, and its fix is the one applied.

Every check errs towards withholding. A rename that is withheld costs one manual edit; a rename that is wrong is a bug the linter itself cannot see.

## Directives

```go
//declscope:public      // state the scope explicitly instead of deriving it
//declscope:package
//declscope:file

//declscope:ignore               // silence every rule for this declaration
//declscope:ignore unqualify        // silence one
//declscope:ignore unqualify,qualify

//declscope:namespace <name>   // before the package clause; overrides the file's namespace
```

An ignore names rules from the table above, so a declaration can opt out of one check while staying subject to the rest.

### Where an ignore can be written

A diagnostic is silenced by the nearest directive that covers its rule, and the levels stack:

| Level | Covers |
| --- | --- |
| the declaration | itself |
| the **type**, for a method or field | every member the type owns, wherever the member is declared |
| the file, before the package clause | every declaration in that file |
| a [baseline](#adopting-on-an-existing-codebase) | violations recorded when the linter was adopted |

A directive on a type is what an open struct wants, rather than one on every field:

```go
//declscope:ignore boundary
type User struct {
	name string
	id   int
}

// covered too, even from another file
func (u *User) normalize() { ... }
```

Each directive is judged on its own, so one used up only by a member still counts as used, and one that silences nothing is reported wherever it was written. A directive is one comment however many declarations it reaches: one on a `var (...)` block, or on `var a, b`, or on `x, y int` in a struct, is used as soon as **any** of them needed it, and is reported once — not once per name — when none did.

### File-level ignore

Written before the package clause, an ignore applies to every declaration in the file. It takes the same argument, so the directive means one thing wherever it appears:

```go
// util.go   (namespace: util)
//declscope:ignore qualify,unqualify

package store
```

This is what a file full of small helpers wants, rather than a directive on each of them. A utility file whose whole contents are meant to be package-wide can say so in one line:

```go
// util.go   (namespace: util)
//declscope:ignore boundary,qualify

package store

func must(err error) { ... }
func first[T any](s []T) T { ... }
```

The directive belongs to the **file**, not to the namespace, and the package clause has nothing to do with either: the namespace comes from the file name, or from `//declscope:namespace`. Files that share a namespace therefore each need their own — one file cannot silence a rule on behalf of another.

Keeping `boundary` and writing `//declscope:package` per declaration is the stricter option, and the one to prefer when the file is not wholly shared — silencing `boundary` removes the boundary for everything in the file, including declarations added later. For violations that already exist, a [baseline](#adopting-on-an-existing-codebase) suppresses them without standing future ones down.

A file-level ignore that silences nothing is reported, like any other.

Placement: the doc comment of a declaration, or a trailing comment on its first or last line — for a multi-line declaration, the line of its opening or closing brace or parenthesis. A trailing `// reason` is allowed.

```go
//declscope:package // shared with the reporting code
func userHelper() {}

func userHelper() {} //declscope:package

type user struct { //declscope:ignore boundary
	name string
}

var ( //declscope:package
	userLimit = 10
	userSeed  = 1
)
```

A comment on the brace line that belongs to a field (`struct { n int //declscope:ignore`) is the field's, as it would be on any other line. A directive written anywhere else after the package clause — separated from its declaration by a blank line, inside a function body, on a line in the middle of a struct — binds to nothing, and is reported as **misplaced** rather than silently dropped, since an author who wrote a suppression believes something is silenced.

A directive on a parenthesized `var`/`const`/`type` block applies to every spec in it. A spec may carry its own, and the two kinds combine differently: a **scope** directive on the spec replaces the block's, since a declaration has exactly one scope, while **ignores accumulate** — the spec's are added to the block's, so a narrower ignore never re-enables a rule the block turned off.

```go
//declscope:ignore boundary
var (
	userSeed = 1
	//declscope:ignore qualify
	limit = 2 // boundary is still silenced by the block; qualify by the spec
)
```

Unused `//declscope:ignore` directives are reported, so suppressions do not outlive the problem. `//declscope:ignore unqualify` is unused if nothing but `unqualify` would have fired. Where directives at different levels both cover a rule, all of them count as used, so overlapping never makes one look unused.

A directive is called unused only by a pass that sees **every** reference in the package. When a package has in-package `_test.go` files, the ordinary variant cannot see what they use, so a directive needed only by a test would be unused there and necessary in the test variant, with no way to satisfy both; the ordinary variant therefore leaves the judgement to the test variant, which sees every file. Under `-test` (the default) that variant runs and nothing is lost. With `-test=false`, a package with in-package tests gets no unused-directive report at all.

## Configuration

Optional. `.declscope.yaml` (or `.yml`), looked up from the analyzed package's directory upwards, stopping at the module root — so a subtree can relax or tighten the rules on its own.

```yaml
defaults:                # these resolve members too, not only package-level declarations
  exported: public       # public | package | file
  unexported: file

rules:
  qualify: ondemand     # always | never | ondemand (true/false: aliases of always/never)
  unqualify: false

exclude:
  - "**/mock_*.go"

baseline: .declscope-baseline.yaml   # relative to this file; found automatically if named by default
```

Only the naming rules appear here; see [Rules](#rules) for why. Unknown keys are an error rather than a silent no-op: a typo in a rule name would otherwise leave the rule at its default with no sign of it. The rules' former names (`rules.promote`, `rules.demote`) are refused the same way, with the key to write instead.

## Adopting on an existing codebase

Turning declscope on for a codebase that predates it would report every boundary that was never enforced. Record them instead:

```console
declscope baseline ./...       # writes .declscope-baseline.yaml
```

Recorded violations are suppressed; new ones are still reported. The file is discovered by the same upward lookup as the config, so its presence is all it takes.

```yaml
packages:
  github.com/you/app/store:
    boundary:
      - User.name
      - helper
    qualify:
      - helper
```

An entry is keyed by **package, rule and declaration** — never by position — so it survives the code being moved, the file being renamed and the package being reformatted. Regenerate rather than edit:

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

Entries for violations that have since been fixed simply disappear, which is why the analyzer never reports an entry as stale: a package's test variant sees references the ordinary variant does not, so "matched nothing" is not a reliable signal from inside one pass.

A baseline suppresses, it does not endorse. Nothing is written into the source, so the convention still applies to every new declaration, and an entry can only be removed by actually fixing the violation — which is what makes it different from silencing the same violations with directives.

```console
declscope baseline [-o path] [-config path] [packages]
```

Each package's entries go to the file the analyzer will consult for that package: the baseline its nearest config file names, else the nearest existing `.declscope-baseline.yaml` between the package and the working directory, else a new `.declscope-baseline.yaml` in the working directory. A subtree with its own baseline keeps it, and a run from a subdirectory writes under that subdirectory rather than rewriting a baseline above it with only part of its entries. Every file written is regenerated wholesale, and the existing one is never read, so a baseline that fails to parse is replaced like any other — including one written under the rules' former names (`escape`, `promote`, `demote`), which the analyzer refuses with a message naming the new ones rather than reading it as an alias.

A package whose lookup cannot reach the working directory — one in another module, or outside the directory the command runs from — is refused rather than recorded where nothing would find it. `-o` gathers every entry into the one file named instead, and leaves placing it to you.

## Installation & Usage

### <a href="https://mise.jdx.dev/"><img src="https://mise.jdx.dev/logo.svg" height="28" alt=""></a> Using [mise](https://mise.jdx.dev/) (macOS/Linux/Windows)

**Recommended.** declscope is installable directly from GitHub Releases via mise's `github` backend — no extra registry required, and no Go toolchain needed because the binaries are prebuilt:

```bash
mise use -g "github:mpyw/declscope"
declscope ./...
```

Or pin it per project in `mise.toml`:

```toml
[tools]
"github:mpyw/declscope" = "latest"
```

> [!IMPORTANT]
> The `go`-based methods below build declscope from source. `go.mod` pins `toolchain go1.27.0`, so with the default `GOTOOLCHAIN=auto` the `go` command downloads a matching toolchain automatically unless `GOTOOLCHAIN=local` is set. `go tool` also needs Go 1.24+ on `PATH`, which is where tool directives were introduced.

### Using [`go tool`](https://pkg.go.dev/cmd/go#hdr-Run_specified_go_tool)

```bash
# Add to go.mod as a tool dependency
go get -tool github.com/mpyw/declscope/cmd/declscope@latest

# Run via go tool
go tool declscope ./...
```

### Using [`go install`](https://pkg.go.dev/cmd/go#hdr-Compile_and_install_packages_and_dependencies)

```bash
go install github.com/mpyw/declscope/cmd/declscope@latest
declscope ./...
```

### Using [`go vet`](https://pkg.go.dev/cmd/go#hdr-Report_likely_mistakes_in_packages)

```bash
go install github.com/mpyw/declscope/cmd/declscope@latest
go vet -vettool=$(which declscope) ./...
```

Note that `go vet` cannot pass declscope's own `-config` flag; the config file is still discovered from the filesystem.

### Using [`go run`](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program)

```bash
go run github.com/mpyw/declscope/cmd/declscope@latest ./...
```

> [!CAUTION]
> To prevent supply chain attacks, pin to a specific version tag instead of `@latest` in CI/CD pipelines (e.g., `@v0.1.0`).

<details>
<summary><a href="https://curl.se/"><img src="https://cdn.simpleicons.org/curl" height="20" alt=""></a> Downloading the tarball directly (macOS/Linux/Windows)</summary>

No package manager? Grab the archive for your platform from [GitHub Releases](https://github.com/mpyw/declscope/releases):

```bash
export VERSION=0.0.0
export OS=linux    # or darwin
export ARCH=amd64  # or arm64
export BASE_URL="https://github.com/mpyw/declscope/releases/download/v${VERSION}"

# Download the archive and the release's checksum list
curl -LO "${BASE_URL}/declscope_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -LO "${BASE_URL}/checksums.txt"

# Verify before installing (use `shasum -a 256 -c` on macOS)
sha256sum --ignore-missing -c checksums.txt

tar xzf "declscope_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo mv declscope /usr/local/bin/
```

On Windows, download `declscope_${VERSION}_windows_${ARCH}.zip` and extract `declscope.exe` somewhere on your `PATH`.

</details>

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | *(discovered)* | Path to a YAML config file, overriding the `.declscope.yaml` lookup |
| `-test` | `true` | Analyze test files (`*_test.go`) — built-in driver flag |
| `-fix` | `false` | Apply suggested fixes automatically — built-in driver flag |

Every diagnostic carries **at most one** fix, so `-fix` is unambiguous: a boundary crossing is fixed by inserting `//declscope:package`, a label by renaming. The two can never conflict, because a rename does not change a declaration's reach. A rename is offered only when it is [provably safe](#when-a-rename-is-withheld); in a package with in-package tests it comes from the test variant, so `-test=false` withholds it.

## Using it with an AI agent

The point of declscope is that the boundary stops being tacit knowledge, so put it where the agent will hit it:

```bash
declscope ./...        # in CI, and in the agent's build/verify loop
declscope -fix ./...   # deterministic: at most one fix per diagnostic
```

On an existing codebase, run `declscope baseline ./...` once first, so the agent is only ever shown the boundaries *it* crossed.

Two things make this work better than a written convention:

- **The diagnostic names the namespace it crossed**, so the agent is told *why* the call is wrong, not merely that it is, and the repair is mechanical.
- **A directive is a durable record of intent.** When `//declscope:package` ends up in the source, the next agent to read the file inherits the decision instead of re-deriving it — and the next one that widens something silently gets caught.

A line in `CLAUDE.md` (or the equivalent for your agent) is usually enough:

```markdown
Run `declscope ./...` before finishing. Do not widen a declaration's scope to make
a call site compile: either keep the call inside the namespace, or state the new
scope with `//declscope:package` and say why.
```

That last clause matters. Left to itself an agent will take the cheapest path out of a diagnostic, and the cheapest path here is to widen everything. Making the widening explicit is the whole mechanism.

## Limits of the analysis

- Generated files (`// Code generated ... DO NOT EDIT.`) are excluded entirely — neither checked nor treated as reference sites.
- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified and never collide across packages.
- Whether an *exported* identifier is used outside its package is out of scope: `go/analysis` has no upward view of the program, and answering it would require a separate whole-program mode. Combine with an unused-code linter for that.
- An embedded field is not checked as a member: it has no name of its own, only the embedded type's. Embedding a type is still a **use** of that type, so `type B struct{ aCount }` written outside `aCount`'s namespace is a `boundary`, and renaming the type rewrites the embedding and every `b.aCount` selection through it.
- Members of generic types are checked like any other: `List[int].items` and `l.items` inside `List[T]`'s own methods are uses of `List.items`.
- Fields of anonymous structs, and of types declared inside a function, are not checked.
- An unexported method grown on another namespace's type but **never called** is not reported. `boundary` needs a reference to find, and a method with none is dead code — the business of an unused-code linter, not this one.

## License

MIT
