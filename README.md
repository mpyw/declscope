# declscope

[![CI](https://github.com/mpyw/declscope/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/declscope/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/declscope.svg)](https://pkg.go.dev/github.com/mpyw/declscope)

Keep your Go packages **flat** without letting them turn into a free-for-all.

declscope adds a visibility level *below* Go's package-wide `unexported` — a `private` that binds a declaration to its own file — for package-level declarations, methods and struct fields alike, and enforces it statically.

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

### What it reports

One `store` package. `csv.go` builds a `User` and reaches into what `user.go` declares:

```go
// user.go
package store

type User struct {
	ID    int64
	email string
}

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
```
```go
// csv.go
package store

func csvParse(rec []string) (*User, error) {
	id, err := strconv.ParseInt(rec[0], 10, 64)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, email: normalize(rec[1])}, nil
}
```

Nothing here is unusual, and nothing the compiler can object to. `email` is unexported so that it is only ever written through the normalizer, and `csv.go` writes it directly.

```console
$ declscope ./...
user.go:7:2:  field User.email is private to namespace "user", but is used from namespace "csv"
user.go:10:6: func normalize is private to namespace "user", but is used from namespace "csv"
user.go:10:6: func normalize does not carry the prefix of namespace "user"; rename it to userNormalize
```

Each crossing has two answers: keep the boundary and put the parsing behind a constructor, or share the declarations on purpose. `declscope -fix` takes the second, and leaves the decision written down:

```go
// user.go
type User struct {
	ID int64
	//declscope:package
	email string
}

//declscope:package
func userNormalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
```
```go
// csv.go
	return &User{ID: id, email: userNormalize(rec[1])}, nil
```

The directive says the declaration is shared; the name says which unit it came from. Both are visible at the call site, and the next file to reach for an unshared declaration is reported the same way.

> [!TIP]
> `-fix` always widens, because that is the repair it can apply mechanically. Where the boundary is worth keeping, move the call instead and leave the declaration alone.

## Installation and usage

### <a href="https://mise.jdx.dev/"><img src="https://mise.jdx.dev/logo.svg" height="28" alt=""></a> Using [mise](https://mise.jdx.dev/) (macOS/Linux/Windows)

**Recommended.** mise's `github` backend installs the prebuilt binary from GitHub Releases, so no registry and no Go toolchain are needed:

```bash
mise use -g "github:mpyw/declscope"
declscope ./...
```

Or pin it per project in `mise.toml`:

```toml
[tools]
"github:mpyw/declscope" = "latest"
```

> [!NOTE]
> The `go`-based methods below build declscope from source, with the Go toolchain that the invoking command resolves. `go install pkg@version` and `go run pkg@version` ignore the `toolchain` directive of the module they install and treat its `go` directive as a lower bound only, so they build declscope with the `go` release already on `PATH`. `go get -tool` builds it inside your own module, so your module's toolchain applies. `go tool` additionally requires Go 1.24 or later on `PATH`.

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

> [!NOTE]
> `go vet` accepts declscope's `-config` flag and runs the tool from each package's own directory, so a path given to it must be absolute. A config file found by the [lookup](#configuration) needs no flag.

> [!IMPORTANT]
> `go vet` runs the vet tool over the standard library packages in the dependency graph as well, type-checking them from source. The tool must therefore be built with a Go release at least as new as the toolchain that builds the analyzed module; otherwise every standard library package is reported as `package requires newer Go version`. The prebuilt binaries on GitHub Releases are built with the toolchain pinned in `go.mod`, while a binary produced by `go install` inherits the `go` release on your `PATH`.

### Using [`go run`](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program)

```bash
go run github.com/mpyw/declscope/cmd/declscope@latest ./...
```

> [!CAUTION]
> To prevent supply chain attacks, pin to a specific version tag instead of `@latest` in CI/CD pipelines (e.g., `@v0.0.1`).

<details>
<summary><a href="https://curl.se/"><img src="https://cdn.simpleicons.org/curl" height="20" alt=""></a> Downloading the tarball directly (macOS/Linux/Windows)</summary>

Without a package manager, the archive for a platform is on [GitHub Releases](https://github.com/mpyw/declscope/releases):

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

On Windows, download `declscope_${VERSION}_windows_${ARCH}.zip` and extract `declscope.exe` somewhere on the `PATH`.

</details>

### Flags

```console
declscope [flags] [packages]           # analyze
declscope baseline [flags] [packages]  # record current violations; see Adopting on an existing codebase
```

| Flag | Default | Effect |
| --- | --- | --- |
| `-config` | *(discovered)* | Path to a YAML config file, overriding the [`.declscope.yaml` lookup](#configuration) |
| `-test` | `true` | Analyze test files (`*_test.go`) as well |
| `-fix` | `false` | Apply suggested fixes |
| `-diff` | `false` | With `-fix`, print a unified diff instead of writing the files |

`-test`, `-fix` and `-diff` are `go/analysis` driver flags; `declscope -help` lists the rest.

Every diagnostic carries **at most one** fix, so `-fix` never has to choose: a [`boundary`](#boundary) is fixed by inserting `//declscope:package`, a [`qualify`](#qualify) or [`unqualify`](#unqualify) violation by renaming, and a rename never changes a declaration's reach. A rename is offered only when it is [provably safe](#withheld-renames).

> [!IMPORTANT]
> Only the test variant of a package sees its in-package `_test.go` files, so in a package that has them, renames and unused-directive reports come from the test variant alone. With `-test=false`, such a package gets no rename fix and no unused-directive report.

## Namespaces

A **namespace** is the unit within which a `private` declaration may be used. By default a namespace is derived from the file name, so that each file is its own namespace:

| File | Namespace |
| --- | --- |
| `user_repository.go` | `userRepository` |
| `user_repository_test.go` | `userRepository` — a test shares its subject's namespace |
| `parser_linux.go`, `parser_linux_amd64.go` | `parser` — GOOS/GOARCH suffixes are build constraints, not namespaces |
| `v2_client.go` | `v2Client` |
| `user_id.go`, `parse_json.go` | `userID`, `parseJSON` — an initialism is spelled the way Go spells it |
| `foo-bar.go`, `Foo.go` | `fooBar`, `foo` — any separator, and a PascalCase stem, normalize to lowerCamelCase |
| `2fa_auth.go` | `2faAuth` — an identity, but never a label |

The namespace is derived from the file name rather than being the file name, so that renaming a file does not rename every identifier it declares, and so that a `_test.go` file reaches its subject's `private` declarations by sharing its namespace.

A namespace has two roles:

| Role | Question it answers | Which namespaces have it |
| --- | --- | --- |
| Identity | Is this use inside the same namespace as the declaration? | Every file with a stem, `2fa.go` included: `2fa_test.go` shares its namespace like any other test |
| Label | Which prefix does [`qualify`](#qualify) require on the file's package-level declarations? | Only one that can start an identifier; no identifier begins with a digit, so `2faAuth` bounds its declarations and the [naming rules](#naming-rules) ask nothing of them |

Files opt into a **shared** namespace with a directive before the package clause, which is how one logical unit spans several files:

```go
//declscope:namespace user
package repo
```

The name must be an unexported identifier. Placement alongside a package comment is described under [File-level directives](#file-level-directives).

The number of namespaces in a package, which [`rules.qualify: ondemand`](#qualify) keys off, is counted over the package's non-test files: a test file joins its subject's namespace rather than adding one, and `integration_test.go` adds none.

### The core namespace

One namespace in a package may be its **core**: the unit the package is named for, whose label is empty.

```go
// client.go
//declscope:core

package transport
```

Several files may carry it, and they share the one core namespace — `//declscope:core` merges files the way `//declscope:namespace` does, with a name that is not written because there is none. A file may not carry both: a core file's namespace *is* the core.

Because the core's label is empty, there is no prefix for [`qualify`](#qualify) to require or for [`unqualify`](#unqualify) to drop, and the core is outside them both. Nothing is lost by that: "unlabeled" still names exactly one unit, so reading `Load()` in any file still says whose it is. The core is simply the unit whose name the package already carries.

This is what a package reaches for when it turns [`rules.exportedLabels`](#configuration) on. Labels on an exported name are the package's API, and the core is how a package says *these names are the API — leave them as they are*:

```go
// with client.go marked core, and retry.go not:
transport.New                 // core: no label asked for
transport.RetryWithBackoff    // retry: labeled like any other unit
```

Without it, `ondemand` reads the namespace count, so a package that gains or loses its second unit changes what it asks of names that did not move. Inside the core that question never arises.

## Scopes

The subject of a **scope** is a package's unexported surface: its unexported package-level declarations, and the unexported [members](#members) of any type, exported or not. An exported identifier is the package's API, published to every importer; a boundary holding a sibling file back from what the whole program may already use would draw a line the compiler has erased. Scope does not apply to it.

Within that surface a **scope** is how far a declaration may be used. There are two:

| Scope | Meaning | Rust equivalent |
| --- | --- | --- |
| `package` | Usable anywhere in the package | `pub(super)` |
| `private` | Usable only inside its own [namespace](#namespaces) | No modifier |

Rust has both natively, because a file is a module: an item with no modifier is visible in its own module and the modules beneath it, which is the namespace, and `pub(super)` lifts it to the parent module and so to the sibling files. Go collapses the two because a package spans its files; `private` is the default a Rust module already has.

There is no `public`. Go already spells that with a capital letter, and a use beyond the package edge is one declscope never sees ([Limits of the analysis](#limits-of-the-analysis)) — so `public` and `package` would name a distinction the analysis could not make, and a scope that cannot be checked is the convention-in-someone's-head this tool exists to replace.

### Scope resolution

A declaration's scope is decided by the first row that applies:

| The declaration | Scope |
| --- | --- |
| Is reachable from outside the package | None. It is outside the subject and carries no boundary |
| Carries a scope directive (`//declscope:package`, `//declscope:private`) | The directive's |
| Is contained by something that carries one — a [field](#members) by its type, a spec by its `var`/`const`/`type` block | The container's |
| Sits in a file carrying a [file-level scope directive](#file-level-directives) | The file's |
| Otherwise | [`defaults.unexported`](#configuration), `private` unless configured |

**Reachable from outside** is not quite the same as exported. A package-level declaration is reachable when it is exported. A [member](#members) is reachable when it is exported *and* its type is: an exported field of an unexported type is published to nobody, whatever its capitalization says. `type entry struct { Key string }` is capitalized for `encoding/json`, not for an audience, and it stays in the subject — which is where a DTO, whose every field is exported, most needs to be.

An exported declaration takes the first row and stops there, but a scope directive on an exported **type** is not thereby wasted: it reaches the type's unexported fields through the third row, which is the only thing a scope directive on an exported declaration can ever do.

Widening is always stated, by a directive on the declaration, on what contains it, on its file, or by the `defaults` key for the package. The name of a declaration plays no part: a namespace prefix is an ownership label ([`qualify`](#qualify)) and grants nothing, so a prefix can be added for legibility without changing what the declaration reaches, and a codebase that prefixes everything loses no protection.

```go
// user.go   (namespace: user)

func UserLoad()   {} // exported: no scope, no boundary
func userCache()  {} // private: only namespace "user" may use it

//declscope:package
func userShared() {} // package: usable anywhere in the package
```

### Members

A **member** is a method or a struct field, and the two are not governed alike, because they are not written alike.

A **field** — and an interface's method name — is written *inside* its type's declaration. Its file is not a choice: it is where the type is. So the type's directive reaches it, for the same reason a directive on a `var (...)` block reaches its specs; this is containment, not inheritance, and there is no second file for it to disagree with.

A **method** is an ordinary top-level declaration that happens to name a receiver. The file it is written in gives it its namespace, exactly as for a `func`, and nothing above it reaches it.

| | Package-level declaration | Field | Method |
| --- | --- | --- | --- |
| Bounding namespace | Its file's | Its **type**'s file's — the file it is written in | Its file's |
| Contained by | Its `var`/`const`/`type` block | Its **type** | Nothing |
| [Naming rules](#naming-rules) | Apply | Do not apply | Do not apply |

So splitting a type's methods across files works the way Go programmers already write it: `marshalKey` in `json.go` belongs to namespace `json`, and `json.go` may use it. What `sort.go` may not do is reach into `User`'s **fields** — and that is the boundary that was doing the work all along. A method grown on another namespace's type can only leak that type's internals by touching them, and touching them is reported.

A member is already qualified by its type at every use (`u.save()`), so it collides with nothing and a label would only stutter (`u.userSave()`). What a member lacks in Go is encapsulation — every unexported field is visible to its whole package — and the `private` scope supplies it:

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

A **rule** is one check. There are four, and a rule's name is at once the diagnostic's category, its configuration key, the key of its [baseline](#adopting-on-an-existing-codebase) entry, and what [`//declscope:ignore`](#directives) targets. A report with no rule would be one nothing could silence and no baseline could absorb.

| Rule | Reports | Fix | Configurable |
| --- | --- | --- | --- |
| [`boundary`](#boundary) | A declaration used from outside the namespace it is private to | Insert `//declscope:package` | No |
| [`qualify`](#qualify) | A package-level declaration missing its namespace label | Rename to add the label | `rules.qualify`, `rules.exportedLabels` |
| [`unqualify`](#unqualify) | A namespace label present where it is not required | Rename to drop the label | `rules.unqualify`, `rules.exportedLabels` |
| [`directive`](#unused-and-malformed-directives) | A directive that binds nothing, or that is malformed or misplaced | None | No |

Reach enforcement has no switch; naming discipline has. `boundary` is silenced per declaration with `//declscope:ignore boundary`, or per codebase with a [baseline](#adopting-on-an-existing-codebase). `qualify` and `unqualify` are mirrors and never both apply to one declaration: `unqualify` is inert wherever `qualify` requires the label.

### `boundary`

`boundary` reports a `private` declaration used from outside its namespace. It covers package-level declarations and [members](#members) alike; for a member the boundary is the namespace of its type.

```go
// user.go
func userCache() int { return 1 }
```
```go
// order.go
func orderRun() int { return userCache() } // reported
```

| Declaration | Message |
| --- | --- |
| Package-level, scope from `defaults` | `func userCache is private to namespace "user", but is used from namespace "order"` |
| Member | `field User.id is private to namespace "user", but is used from namespace "order"` |
| Scope stated by the declaration's own directive | `func userCache is declared private by //declscope:private, but is used from namespace "order"` |
| Field, scope taken from its type's directive | `field User.id is declared private by //declscope:private on User, but is used from namespace "order"` |
| Scope from a file-level directive | `func userCache is declared private by the file's //declscope:private, but is used from namespace "order"` |

Every use site is attached to the diagnostic as related information.

The report lands on the **declaration**. A method grown on another namespace's type is not itself a crossing — it belongs to the file that wrote it — but the fields it reaches for are reported where they are declared:

```go
// order.go  (namespace: order)
func (u *User) leak() int { return u.id } // u.id is reported, against user.go
```

The fix inserts `//declscope:package` above the declaration. A declaration that states its **own** scope with a directive is reported without a fix: the directive and the use site are both deliberate, and `-fix` must not overwrite the one the author wrote. A `private` inherited from the declaration's type or from a file-level directive does not withhold it — the inserted directive sits on the declaration, which outranks both, so the fix states an exception to a default rather than overwriting a decision.

### Naming rules

`qualify` and `unqualify` govern one property of a package-level declaration: whether its name carries the namespace as a prefix, the **label**. The label says which unit owns the declaration, which is what makes a cross-namespace use legible at the call site, in a stack trace and in a grep result:

```go
// order.go
func orderRun() int {
    return userShared() // the user unit's, and shared
}
```

The label grants nothing; reach is stated by [scope](#scope-resolution) alone.

Neither rule applies to:

| Declaration | Reason |
| --- | --- |
| A [member](#members) | Already qualified by its type |
| `func main` in package `main` | A name the toolchain requires |
| A declaration in a namespace that cannot be a label (`2fa.go`) | There is no prefix to ask for or to drop |
| An exported identifier, unless [`rules.exportedLabels`](#configuration) is on | Off by default: how the package's API is spelled is the author's decision, not a linter's |
| A declaration in the [core namespace](#the-core-namespace) | Its label is empty; there is none to require or to drop |

Unlike [scope](#scopes), which stops at the export line because reach beyond it cannot be checked, the naming rules have a reason to cross it. What a label answers — *which unit owns this name* — is asked wherever the name is read bare, and inside the package an exported name is read exactly as bare as an unexported one: `Load()`, not `store.Load()`. The package qualifier that makes an external use self-explanatory is absent at precisely the use sites the label exists for.

So the rule is available for exported declarations and off by default, and where it is on the rename is never offered — see [Withheld renames](#withheld-renames). The violation is reported either way.

Where [`rules.qualify`](#qualify) is `ondemand`, the namespace count it depends on is the one described under [Namespaces](#namespaces).

**Label matching.** A name carries the label when it begins with the namespace **ignoring case** and the label then ends at a word boundary: in `user_id.go` (namespace `userID`) `userIDCache`, `userIdCache` and `userIdcache` all carry it, while `useridentity` does not. A name equal to its namespace (`type user` in `user.go`) carries the label for `qualify` and carries nothing for `unqualify` to drop; that comparison, too, ignores case.

**Rename spelling.** Both halves of a rename are spelled the way Go spells an initialism: `id` in `user.go` becomes `userID` and `urlPath` becomes `userURLPath`, never `userId`; `userID` drops to `id` and `userURLPath` to `urlPath`, never `iD`. A rename never changes what a name is visible to, so it carries the exportedness across in both directions: under [`rules.exportedLabels`](#configuration) `Load` in `user.go` is reported against `UserLoad`, never `userLoad`, and `UserID` drops to `ID`, never `id`. A rename that quietly unexported a declaration would delete the package's API to satisfy a linter. There the suggested name is also all the author gets — no rename is offered for an exported declaration ([Withheld renames](#withheld-renames)) — so the message has to be enough to act on.

**Fix.** The fix of either rule renames every use of the declaration in the package. It is offered only when it is provably safe ([Withheld renames](#withheld-renames)); the violation is reported either way.

#### `qualify`

`qualify` requires the label.

| `rules.qualify` | Effect |
| --- | --- |
| `ondemand` *(default)* | Required only once the package has a **second namespace**. In a package with one there is no boundary for a label to mark, and a prefix repeated on every declaration would distinguish nothing. |
| `always` | Required in every package, so that a package gaining its second namespace is not a mass rename. |
| `never` | Off. |

```
func id does not carry the prefix of namespace "user"; rename it to userID
```

#### `unqualify`

`unqualify` forbids the label wherever `qualify` does not require it. With both on, the spelling of every package-level name the rules reach is determined in both directions, and fixable in either direction wherever a rename is [provably safe](#withheld-renames).

| `rules.unqualify` | Effect |
| --- | --- |
| `false` *(default)* | Off. |
| `true` | Forbidden wherever `qualify` does not require the label. |

`unqualify` is a boolean, not a mode. Where the label is required it has nothing left to decide, so every question about *when* a label applies is already answered by [`rules.qualify`](#qualify); all this key adds is whether the other direction is enforced too. Spelling it `always`/`never` would suggest a third setting it could never have, and would misdescribe the `true` case, which is conditional by construction: under the default `qualify: ondemand` it acts only in a package with one namespace, and under `qualify: never` — where nothing is ever required — it acts everywhere, stripping labels throughout. That last pairing is the coherent way to say *this codebase does not use labels; take them off*.

Under `unqualify: true`, every prefix that matches the namespace is treated as the label, since nothing in a name tells `userID`-the-label from `userID`-the-word. A prefix that is part of the concept is declared on the declaration:

```go
//declscope:ignore unqualify
var userID int
```

Where no new name can be derived, the violation is still reported, with the reason and without a fix. Being unable to spell the new name is a limit of the fix, not a reason to let the label stand.

| Name | Reported as |
| --- | --- |
| `userCache` in `user.go` | `func userCache carries the label of namespace "user", which is not required here; rename it to cache` |
| `userType` in `user.go` | `func userType carries the label of namespace "user", which is not required here, but "type" is a keyword; rename it by hand` |
| `userIdcache` in `user_id.go` | `… but what follows the label does not start a new word; rename it by hand` |

#### Withheld renames


A rename is offered only when it provably changes nothing but the spelling. Go resolves a name from the inside out — local scope, then the file's imports, then the package, then the predeclared names — so a new name that is free at package level can still be bound at a use site, and the wrong rename **compiles and computes something else**:

```go
var count = 10
func Add(fooCount int) int { return fooCount + count } // Add(1) == 11
```
```go
// after a careless rename of count to fooCount
func Add(fooCount int) int { return fooCount + fooCount } // Add(1) == 2
```

The violation is reported, and the fix withheld, when any of the following holds:

| Condition | Why |
| --- | --- |
| The declaration is exported | Its uses outside the package are never analyzed, so the rename could not be completed — and finishing it by hand is an API change, which is the author's call |
| The new name is already declared in the package | Would not compile |
| The new name is predeclared (`len`, `error`, `string`, …) | The declaration compiles and shadows the builtin for the whole package |
| Any file of the package imports the new name | Go rejects a package-level name that any file imports |
| At some use of the declaration, the new name is bound by a local, parameter, result or type parameter | The use would silently resolve to that instead |
| The declaration is used from a generated or `exclude`d file | Those files are never rewritten, so the use would dangle |
| A `//go:linkname` or `//export` directive names the declaration | The directive names it as text, which a rename cannot follow |
| Another fix in the same run already renames something to that name | Two declarations would end up with one name (`a.go:bX` and `a_b.go:x` both label to `aBX`; `fooBar` and `fooBAR` both drop to `bar`) |
| The package has in-package `_test.go` files that this variant does not see, or its directory cannot be listed | The test files may declare or use the name; the test variant, which sees every file, decides, and its fix covers the non-test files too |

Each condition is conservative: a doubt withholds the fix, never the diagnostic. The first row is unconditional rather than a doubt — no run of declscope can ever see the whole of an exported name's uses — so under [`rules.exportedLabels`](#configuration) an exported violation is always reported and never fixed. The last row is why, with `-test=false`, no rename is offered in a package that has in-package tests; under the default `-test=true` the test variant is analyzed as well, by `declscope` and by `go vet` alike, and its fix is the one applied.

## Directives

A **directive** is a comment beginning `//declscope:` (`/*declscope:` … `*/` is also recognized).

| Directive | Level | Effect |
| --- | --- | --- |
| `//declscope:package`, `//declscope:private` | Declaration or file | States the [scope](#scope-resolution) instead of inheriting it from the level above — the type, then the file, then `defaults`. On a type it also reaches the type's [members](#members); on a file it is the default for what the file declares |
| `//declscope:ignore` | Declaration or file | Silences every rule |
| `//declscope:ignore <rules>` | Declaration or file | Silences the named [rules](#rules), comma-separated (`unqualify`, `unqualify,qualify`) |
| `//declscope:core` | File | Joins the file to the package's **core** [namespace](#the-core-namespace); mutually exclusive with `//declscope:namespace` |
| `//declscope:namespace <name>` | File | Sets the file's [namespace](#namespaces); the name must be an unexported identifier |

A trailing `// reason` is allowed after any directive:

```go
//declscope:package // shared with the reporting code
func userHelper() {}
```

### Declaration-level placement

A declaration-level directive is written in the doc comment of a declaration, or as a trailing comment on its first or last line — for a multi-line declaration, the line of its opening or closing brace or parenthesis:

```go
func userHelper() {} //declscope:package

type user struct { //declscope:ignore boundary
	name string
}

var ( //declscope:package
	userLimit = 10
	userSeed  = 1
)
```

A comment on the brace line that belongs to a field (`struct { n int //declscope:ignore`) is the field's, as it would be on any other line.

A directive on a parenthesized `var`/`const`/`type` block applies to every spec in it. A spec may carry its own, and the two kinds combine differently: a **scope** directive on the spec replaces the block's, since a declaration in the subject has exactly one scope, while **ignores accumulate** — the spec's are added to the block's, so a narrower ignore never re-enables a rule the block turned off.

```go
//declscope:ignore boundary
var (
	userSeed = 1
	//declscope:ignore qualify
	limit = 2 // boundary is silenced by the block; qualify by the spec
)
```

A directive written anywhere else after the package clause — separated from its declaration by a blank line, inside a function body, on a line in the middle of a struct — binds to nothing and is reported as **misplaced** rather than silently dropped, since an author who wrote a suppression believes something is silenced.

### File-level directives

A file-level directive is written before the package clause and applies to the whole file. `//declscope:namespace`, `//declscope:ignore` and the scope directives are the file-level directives.

A file-level scope directive is a **default**, not a blanket: it supplies the scope of every declaration in the file that states none of its own and inherits none from its type, exactly as [`defaults.unexported`](#configuration) does for the package, and a declaration can still narrow itself back with `//declscope:private`. For a [member](#members) the file consulted is the one the member is written in, which is not always the one that bounds it ([Members](#members)).

`//declscope:namespace` matches Go's [directive syntax](https://go.dev/doc/comment#syntax), so `go/doc` strips it from the rendered documentation. In a file with a package comment it goes where Go places directives — at the bottom of the doc comment, after a blank comment line:

```go
// Package repo stores users.
//
//declscope:namespace user
package repo
```

In a file with no package comment, a blank line separates the directive from the package clause:

```go
//declscope:namespace user

package repo
```

Flush against `package` it also works, but when another file in the package carries the real package comment it leaves a stray blank line in the rendered documentation.

A file-level ignore takes the same argument as the declaration-level form, so the directive means one thing wherever it appears. It is what a file of small helpers wants, rather than a directive on each of them:

```go
// util.go   (namespace: util)
//declscope:ignore qualify,unqualify

package store
```

A utility file whose whole contents are meant to be package-wide says so in one line — with a scope, not an ignore:

```go
// util.go   (namespace: util)
//declscope:package

package store

func utilMust(err error) { ... }
func utilFirst[T any](s []T) T { ... }
```

The difference from `//declscope:ignore boundary` is the difference between endorsing and suppressing. These declarations *are* `package`, so [`qualify`](#qualify) still asks them for the label — which is the point, because `utilMust(err)` read from another file says whose helper it is — and one that turns out not to be shared can be narrowed back with `//declscope:private`. Under an ignore the rule is off, so the label goes unasked and a `//declscope:private` written inside the file would mean nothing at all.

The directive belongs to the **file**, not to the namespace: the namespace comes from the file name or from `//declscope:namespace`, and the package clause has nothing to do with either. Files that share a namespace each need their own; one file cannot silence a rule on behalf of another.

> [!WARNING]
> A file-level scope directive is a default, so a declaration added to the file later inherits it as well. That is what a wholly shared file means; when the file is not wholly shared, leave the default alone and write `//declscope:package` on the declarations that are. Reaching for `//declscope:ignore boundary` instead is worse than either choice: it states no scope, it silences the rule, so nothing in the file can be narrowed back and the suppression outlives whatever justified it. For violations that already exist, a [baseline](#adopting-on-an-existing-codebase) suppresses them without standing future ones down.

### Ignore levels

A diagnostic is silenced by any directive that covers its rule at any of these levels:

| Level | Covers |
| --- | --- |
| The declaration | Itself |
| The **type**, for a method or field | Every member the type owns, wherever the member is declared |
| The file, before the package clause | Every declaration in that file |
| A [baseline](#adopting-on-an-existing-codebase) | Every violation recorded in it |

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

Every level is consulted, and every **ignore** directive that covers the rule counts as used, so overlapping ignores at different levels never make one another look unused. A scope directive is judged differently, and an outer one can leave an inner one with nothing to bind — see [Unused and malformed directives](#unused-and-malformed-directives). Ignores are consulted before the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

### Unused and malformed directives

A directive that decides nothing is reported, so that neither a suppression nor a claim of intent outlives what justified it. `//declscope:ignore unqualify` is unused if nothing but `unqualify` would have fired.

A **scope** directive is judged the same way, and the test is what it binds rather than what it sits on: is there anything in its reach that takes its scope from it. Because [scope](#scopes) stops at what is reachable from outside the package, an exported declaration is not itself something a scope directive can bind — on an exported type the directive lives through the type's unexported [fields](#members), and on an exported func, or a type whose every field is exported, it binds nothing and is reported.

The test is deliberately *structural*: it does not ask whether the scope named differs from the one already in force. Such a test would make `//declscope:private` — written to record that a declaration is private **on purpose** rather than by default — an error, which is the opposite of what a directive is for. It would also turn a single line of `.declscope.yaml`, or a config file appearing in a parent directory, into hundreds of new diagnostics in files nobody touched.

These reports carry the `directive` rule, so `//declscope:ignore directive` silences one and a [baseline](#adopting-on-an-existing-codebase) records one, like any other rule.

Accounting is per physical directive, however many declarations it reaches: one on a `var (...)` block, on `var a, b`, or on `x, y int` in a struct is used as soon as **any** of them needed it, and is reported once — not once per name — when none did. A directive used up only by a member still counts as used.

| Report | Meaning |
| --- | --- |
| `unused //declscope:ignore boundary on userSeed, limit` | No named declaration needed it |
| `unused file-level //declscope:ignore qualify` | Nothing in the file needed it |
| `unused //declscope:ignore: no checked declaration carries it` | Written on something declscope does not check: `init`, `_`, an embedded field |
| `unused //declscope:package on Helper: nothing it reaches takes a scope` | Written on an exported declaration with no unexported field beneath it |
| `unused file-level //declscope:package` | Every declaration in the file is out of the subject, or states its own scope |

A directive is called unused only by a pass that sees **every** reference in the package. When a package has in-package `_test.go` files, the ordinary variant cannot see what they use, so a directive needed only by a test would be unused there and necessary in the test variant, with no way to satisfy both; the ordinary variant leaves the judgment to the test variant, which sees every file. Under `-test` (the default) that variant runs and nothing is lost. With `-test=false`, a package with in-package tests gets no unused-directive report at all.

A malformed directive is reported at the comment:

| Directive | Report |
| --- | --- |
| `//declscope:foo` | `unknown directive declscope:foo` |
| `//declscope:package x` | `//declscope:package takes no argument` |
| `//declscope:private` and `//declscope:package` on one declaration, or on one file | `conflicting scope directives: …` |
| `//declscope:core` and `//declscope:namespace` on one file | `conflicting namespace directives: a core file's namespace is the core` |
| `//declscope:ignore foo` | `unknown rule "foo" in declscope:ignore (want one of boundary, qualify, unqualify)` |
| `//declscope:namespace` after the package clause | `declscope:namespace must appear before the package clause` |
| `//declscope:namespace` with no name, a second one, or a name that is not an unexported identifier | Reported as such |
| `//declscope:core` with an argument, or a second one on the same file | Reported as such |
| `//declscope:public` | `was removed: an exported declaration carries no boundary, so the directive stated nothing — delete the line` |

## Configuration

Configuration is optional. It is read from `.declscope.yaml` (or `.declscope.yml`), looked up from the analyzed package's directory upwards and stopping at the module root (the directory holding `go.mod`), so a subtree can relax or tighten the rules on its own. The [`-config`](#flags) flag names a file explicitly and skips the lookup. An empty file is a valid config that changes nothing.

```yaml
defaults:                 # this resolves members too, not only package-level declarations
  unexported: private     # package | private

rules:
  qualify: ondemand       # always | never | ondemand
  unqualify: false        # true | false
  exportedLabels: false   # true | false

exclude:
  - "**/mock_*.go"

baseline: .declscope-baseline.yaml   # relative to this file; found automatically if named by default
```

| Key | Values | Default | Effect |
| --- | --- | --- | --- |
| `defaults.unexported` | `package`, `private` | `private` | Scope of a declaration or member that carries no scope directive of its own, inherits none from its type, and sits in no file that supplies one |
| `rules.qualify` | `always`, `never`, `ondemand` | `ondemand` | When the namespace label is required; see [`qualify`](#qualify) |
| `rules.unqualify` | `true`, `false` | `false` | Whether a label is forbidden where it is not required; see [`unqualify`](#unqualify) |
| `rules.exportedLabels` | `true`, `false` | `false` | Whether the naming rules also reach exported declarations. The violation is reported; the rename is never offered ([Withheld renames](#withheld-renames)). A package turning this on wants [`//declscope:core`](#the-core-namespace) on the files holding its API |
| `exclude` | Glob patterns matched against the file path: `*` and `?` within a path segment, `**` across segments, anchored at any segment boundary | None | Files that are neither checked nor treated as reference sites |
| `baseline` | A path relative to the config file | The nearest `.declscope-baseline.yaml` at or above the package, stopping at the module root | The baseline to consult |

There is no `defaults.exported`: an exported declaration has no scope to default ([Scopes](#scopes)). `boundary` has no key; see [Rules](#rules). An unknown key is an error rather than a silent no-op, so that a typo in a rule name cannot leave the rule at its default with no sign of it. A value a key does not accept is an error naming the values it does.

### Upgrading from the three-scope releases

Three things were removed with the `public` scope, and each is answered by name
rather than by the parser's generic complaint:

| Written | What happens |
| --- | --- |
| `//declscope:public` | Reported, with a fix that deletes the line — so `declscope -fix ./...` performs the migration |
| `defaults.exported` | The config is refused, saying why there is no scope for an exported declaration to default to |
| `rules.unqualify: always` / `never` | The config is refused, saying the key is now `true` or `false` and which one the old word meant |

A `defaults.exported: private` that was holding an exported struct's internals under a boundary has a replacement: leave the fields unexported, or make the type unexported. Either way they stay in the subject on their own, with no configuration at all — an exported field of an unexported type is [reachable from nobody](#scope-resolution).

Existing baseline entries for violations that can no longer be produced simply stop matching; regenerating drops them, and the deletion in `git diff .declscope-baseline.yaml` records the rules changing rather than any cleanup.

## Adopting on an existing codebase

A **baseline** records the violations a codebase has, so that turning declscope on does not report every boundary that was never enforced:

```console
declscope baseline ./...       # writes .declscope-baseline.yaml
```

Recorded violations are suppressed; new ones are reported. The file is discovered by the same upward lookup as the config, so its presence is all it takes.

```yaml
packages:
  github.com/you/app/store:
    boundary:
      - User.name
      - helper
    qualify:
      - helper
```

An entry is keyed by **package, rule and declaration** — never by position — so it survives the code being moved, the file being renamed and the package being reformatted. A member is recorded as `Type.member`.

A baseline is regenerated, never edited:

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

An entry whose violation has been fixed disappears on regeneration, and the analyzer never reports an entry as stale: a package's test variant sees references the ordinary variant does not, so "matched nothing" is not a reliable signal from inside one pass. Regeneration analyzes the test variants too, so a violation that only a test's reference produces is recorded.

A baseline suppresses; it does not endorse. Nothing is written into the source, so the rules apply to every new declaration, and an entry can be removed only by fixing the violation. This is what distinguishes it from silencing the same violations with directives. A configured baseline that does not exist yet behaves as an empty one.

### Where the baseline is written

```console
declscope baseline [-o path] [-config path] [packages]   # packages default to ./...
```

Each package's entries go to the file the analyzer will consult for that package, resolved in this order:

| Target | When |
| --- | --- |
| The baseline named by the package's nearest config file | The config file has a `baseline` key |
| The nearest existing `.declscope-baseline.yaml` between the package and the working directory | No config names one |
| A new `.declscope-baseline.yaml` in the working directory | Neither of the above exists |

`-o` bypasses that lookup and gathers every entry into the one file named, leaving its placement to the caller.

A subtree with its own baseline keeps it, and a run from a subdirectory writes under that subdirectory rather than rewriting a baseline above it with only part of its entries. Every file written is regenerated wholesale, and the existing one is never read, so a baseline that fails to parse is replaced like any other.

A package whose lookup cannot reach the working directory — one in another module, or outside the directory the command runs from — makes the run refuse, naming the packages, rather than record entries where nothing would find them.

## Using it with an AI agent

declscope runs wherever an agent's edits are checked, and its fixes are applied there:

```bash
declscope ./...        # in CI, and in the agent's build/verify loop
declscope -fix ./...   # deterministic: at most one fix per diagnostic
```

On an existing codebase, `declscope baseline ./...` is run once first, so that the agent is shown only the boundaries its own edits cross.

Two properties distinguish this from a written convention:

| Property | Effect on the agent |
| --- | --- |
| The diagnostic names the namespace crossed | The agent is told *why* the use is wrong, not only that it is, and the repair is mechanical |
| A directive is a durable record of intent | When `//declscope:package` is in the source, the next agent to read the file inherits the decision instead of re-deriving it, and the next one that widens something silently is caught |

A line in `CLAUDE.md`, or the equivalent for the agent in use, is enough:

```markdown
Run `declscope ./...` before finishing. Do not widen a declaration's scope to make
a call site compile: either keep the call inside the namespace, or state the new
scope with `//declscope:package` and say why.
```

The second sentence is the one the mechanism depends on. The cheapest repair for a `boundary` diagnostic is to widen the declaration, and `//declscope:package` inserted for that reason is itself a change of reach; the instruction requires that change to be written and justified where the next reader finds it.

## Where declscope sits

Three linters draw boundaries in Go, at three scales:

![A Go program drawn as nested frames. Between the api and store packages, depguard asks whether one package may import another; a green arrow runs from api to store and a red one back from store to api is crossed out. Inside store, between user.go and csv.go, declscope asks whether one file may reach another's declaration; a red arrow from csvParse to User.email is crossed out. At the edge of the program, deadcode asks whether anything is reachable at all; the mail package sits greyed out with no arrow entering it, captioned unreachable.](docs/boundaries.png)

| Linter | Scale | The question it answers |
| --- | --- | --- |
| [`depguard`](https://github.com/OpenPeeDeeP/depguard) | Between packages | May this package import that one? |
| **declscope** | Within one package | May this file reach that declaration? |
| [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) | Whole program | Is this reachable at all? |

`depguard` reads the import graph and enforces the lines drawn by the package layout: a domain package may not import a transport one. It says nothing about a package's inside, where every unexported name is visible to every file.

declscope draws lines inside the package, between namespaces, so that a package can stay flat and keep a boundary the compiler does not provide.

`deadcode` roots a reachability analysis at a `main` package and reports what no path reaches. It answers a question declscope structurally cannot: `boundary` needs a use to find, so a declaration used by nobody produces no crossing and no diagnostic.

The three compose: `depguard` keeps the package graph honest, declscope keeps each package honest inside, and `deadcode` removes what neither needs to reach. Where [Limits of the analysis](#limits-of-the-analysis) says "an unused-code linter", the tool meant is `deadcode`, or staticcheck's `unused` for a library with no `main` to root from.

## Limits of the analysis

| Construct | Treatment |
| --- | --- |
| Generated files (`// Code generated ... DO NOT EDIT.`) | Excluded entirely: neither checked nor treated as reference sites |
| Files matching [`exclude`](#configuration) | The same |
| A use of an *exported* identifier outside its package | Out of scope: everything is checked within a single package, and `go/analysis` has no upward view of the program. An unused-code linter answers it. This is why [scope](#scopes) stops at the export line, and why, under [`rules.exportedLabels`](#configuration), a naming violation on an exported declaration is reported without a fix |
| Namespaces in different packages | Never collide, for the same reason: a namespace is implicitly package-qualified |
| An embedded field | Not a member: it has no name of its own, only the embedded type's. Embedding is a **use** of that type, so `type B struct{ aCount }` written outside `aCount`'s namespace is a `boundary`, and renaming the type rewrites the embedding and every `b.aCount` selection through it |
| Members of generic types | Checked like any other: `List[int].items`, and `l.items` inside `List[T]`'s own methods, are uses of `List.items` |
| Fields of anonymous structs, and of types declared inside a function | Not checked |
| An unexported method grown on another namespace's type but **never used** | Not reported: `boundary` needs a use to find, and a method with none is dead code, an unused-code linter's business |

## License

MIT
