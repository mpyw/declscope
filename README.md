# declscope

[![CI](https://github.com/mpyw/declscope/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/declscope/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/declscope.svg)](https://pkg.go.dev/github.com/mpyw/declscope)

Keep your Go packages **flat** without letting them turn into a free-for-all.

Within one package, declscope holds a declaration to the file that declares it. What is unexported, or what a directive narrows, may be used only from there. This is the `private` that Go has no word for, enforced statically.

## Why

Go's advice is to prefer few, large packages. Nesting packages to manufacture encapsulation buys a boundary at a real price:

- Import cycles, and interfaces duplicated to break them
- Stutter (`user.UserRepository`)
- A directory tree that stops reflecting the domain

Most Go codebases are better off flat. The bill comes due *inside* the package, because Go has two visibility levels and the lower one spans the whole thing:

| Level | Reach |
| --- | --- |
| Exported | Every importer |
| Unexported | **Every file in the package** |

There is no third level. In a flat package, every helper is a package-wide name. Every field is a package-wide reach.

One rule keeps this workable: *this helper belongs to this file. Do not reach for it from another file.* The rule is real, but it lives only in the heads of the people who wrote the package. The compiler does not know it.

### That convention does not survive an AI agent

- An agent sees an unexported helper in scope, so it calls it.
- It sees an unexported field, so it writes to it.
- Every one of those choices compiles, passes review at a glance, and is locally reasonable.

The structure erodes a little at a time. In the end the package is a mesh, and nobody can say which parts were meant to be separate.

The usual reaction is to split packages so that the compiler holds the line. That pays the full price of nesting to buy back a boundary the code never needed to lose.

declscope takes the other route. It makes the boundary **machine-checkable inside a flat package**. The agent is told what it crossed, and is given a fix it can apply. The decision is then written in the source, where the next agent reads it.

### What it reports

One `store` package. `csv.go` builds a `User` and reaches into what `user.go` declares:

```go
// user.go
package store

import "strings"

type User struct {
	ID    int64
	email string
}

func emailNormalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
```
```go
// csv.go
package store

import "strconv"

func csvParse(rec []string) (*User, error) {
	id, err := strconv.ParseInt(rec[0], 10, 64)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, email: emailNormalize(rec[1])}, nil
}
```

Nothing here is unusual, and nothing the compiler can object to. `email` is unexported so that it is only ever written through the normalizer, and `csv.go` writes it directly.

```console
$ declscope ./...
user.go:7:2:  field User.email is private to namespace "user", but is used from namespace "csv"
user.go:10:6: func emailNormalize is private to namespace "user", but is used from namespace "csv"
user.go:10:6: func emailNormalize does not carry namespace "user" anywhere in its name; rename it to userEmailNormalize
```

Each crossing has two answers:

| Answer | How |
| --- | --- |
| Keep the boundary | Move the call inside the namespace — here, put the parsing behind a constructor |
| Share on purpose | State it with `//declscope:package`, which `-fix` inserts |

`-fix` takes the second, and leaves the decision written down:

```go
// user.go
type User struct {
	ID int64
	//declscope:package
	email string
}

//declscope:package
func userEmailNormalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
```
```go
// csv.go
	return &User{ID: id, email: userEmailNormalize(rec[1])}, nil
```

The directive says the declaration is shared. The name says which unit it came from. Both are visible at the call site, and the next file to reach for an unshared declaration is reported the same way.

> [!TIP]
> `-fix` always widens, because that is the repair it can apply mechanically. Where the boundary is worth keeping, move the call instead and leave the declaration alone.

### Where declscope sits

Three linters draw boundaries in Go, at three scales:

![A Go program drawn as nested frames. Between the api and store packages, depguard asks whether one package may import another; a green arrow runs from api to store and a red one back from store to api is crossed out. Inside store, between user.go and csv.go, declscope asks whether one file may reach another's declaration; a red arrow from csvParse to User.email is crossed out. At the edge of the program, deadcode asks whether anything is reachable at all; the mail package sits greyed out with no arrow entering it, captioned unreachable.](docs/boundaries.png)

| Linter | Scale | The question it answers |
| --- | --- | --- |
| [`depguard`](https://github.com/OpenPeeDeeP/depguard) | Between packages | May this package import that one? |
| **declscope** | Within one package | May this file reach that declaration? |
| [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) | Whole program | Is this reachable at all? |

- **`depguard`** reads the import graph and enforces the lines drawn by the package layout: a domain package may not import a transport one. It says nothing about a package's inside, where every unexported name is visible to every file.
- **declscope** draws lines inside the package, between namespaces, so a package can stay flat and keep a boundary the compiler does not provide.
- **`deadcode`** roots a reachability analysis at a `main` package and reports what no path reaches. It answers a question declscope structurally cannot. `boundary` needs a *use* to find, so a declaration nobody uses produces no crossing and no diagnostic.

The three compose. `depguard` keeps the package graph honest, declscope keeps each package honest inside, and `deadcode` removes what neither needs to reach.

> [!NOTE]
> Where [Limits of the analysis](#limits-of-the-analysis) says "an unused-code linter", the tool meant is `deadcode`, or staticcheck's `unused` for a library with no `main` to root from.

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
> The `go`-based methods below build declscope from source, with the Go toolchain that the invoking command resolves. `go install pkg@version` and `go run pkg@version` ignore the module's `toolchain` directive. They read its `go` directive as a lower bound only. So they build declscope with the `go` release already on your `PATH`.
>
> `go get -tool` builds it inside your own module, so your module's toolchain applies. `go tool` also needs Go 1.24 or later on `PATH`.

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
> `go vet` also runs the tool over the standard library packages in the dependency graph, and type-checks them from source. So the tool must be built with a Go release at least as new as the toolchain that builds the analyzed module. Otherwise every standard library package is reported as `package requires newer Go version`.
>
> The prebuilt binaries on GitHub Releases are built with the toolchain pinned in `go.mod`. A binary from `go install` uses the `go` release on your `PATH` instead.

### Using [`go run`](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program)

```bash
go run github.com/mpyw/declscope/cmd/declscope@latest ./...
```

> [!CAUTION]
> To prevent supply chain attacks, pin to a specific version tag instead of `@latest` in CI/CD pipelines (e.g., `@v0.1.0`).

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

`-test`, `-fix` and `-diff` are `go/analysis` driver flags. `declscope -help` lists the rest.

Every diagnostic carries **at most one** fix, so `-fix` never has to choose. A [`boundary`](#boundary) is fixed by inserting `//declscope:package`. A [`qualify`](#qualify) violation is fixed by renaming. A rename never changes a declaration's reach, and is offered only when [provably safe](#withheld-renames).

> [!IMPORTANT]
> Only the test variant of a package sees its in-package `_test.go` files. So in a package that has them, renames and unused-**ignore** reports come from the test variant alone. With `-test=false`, such a package gets neither.
>
> An unused **scope** directive is reported in every variant. What it binds is decided by the declarations it reaches, never by who uses them.

## Configuration

Configuration is optional, and this is all of it. Every key links to the section that explains it.

- Read from `.declscope.yaml` (or `.declscope.yml`).
- Looked up from the analyzed package's directory **upwards**, stopping at the module root (the directory holding `go.mod`). So a subtree can relax or tighten the rules on its own.
- [`-config`](#flags) names a file explicitly and skips the lookup.
- An empty file is a valid config that changes nothing.

```yaml
defaults:                 # this resolves members too, not only package-level declarations
  unexported: private     # package | private

rules:
  naming:
    qualify: ondemand     # always | never | ondemand
    exported: false       # true | false

exclude:
  - "**/mock_*.go"

baseline: .declscope-baseline.yaml   # relative to this file; found automatically if named by default
```

| Key | Values | Default | Effect |
| --- | --- | --- | --- |
| `defaults.unexported` | `package`, `private` | `private` | Scope of a declaration or member that carries no scope directive of its own, inherits none from its type, and sits in no file that supplies one |
| `rules.naming.qualify` | `always`, `never`, `ondemand` | `never` | When a name must carry its namespace; see [`qualify`](#qualify) |
| `rules.naming.exported` | `true`, `false` | `false` | Whether the naming rule also reaches exported declarations. The violation is reported. The rename is never offered ([Withheld renames](#withheld-renames)). A package turning this on wants [`//declscope:core`](#the-core-namespace) on the files holding its API |
| `exclude` | Glob patterns matched against the file path: `*` and `?` within a path segment, `**` across segments, anchored at any segment boundary | None | Files that are neither checked nor treated as reference sites |
| `baseline` | A path relative to the config file | The nearest `.declscope-baseline.yaml` at or above the package, stopping at the module root | The baseline to consult |

> [!NOTE]
> - `defaults` takes only `unexported`. An exported declaration has no scope to default ([Scopes](#scopes)).
> - `boundary` has no key. See [Rules](#rules).
> - An **unknown key is an error**, not a silent no-op. A typo in a rule name cannot leave the rule at its default with no sign of it. The message names the key, and the keys its section does take.
> - A value a key does not accept is also an error, and it names the values the key does take.

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
| `2fa_auth.go` | `2faAuth` — an identity, but never a prefix |

> [!NOTE]
> The namespace is *derived from* the file name, not equal to it. That is what lets a file be renamed without renaming every identifier it declares, and lets a `_test.go` file reach its subject's `private` declarations.

A namespace has two roles:

| Role | Question it answers | Which namespaces have it |
| --- | --- | --- |
| Identity | Is this use inside the same namespace as the declaration? | Every file with a stem, `2fa.go` included: `2fa_test.go` shares its namespace like any other test |
| Naming | Which namespace does [`qualify`](#qualify) ask the file's package-level declarations to carry? | Only one that can start an identifier, since the fix prefixes. No identifier begins with a digit, so `2faAuth` bounds its declarations and the [naming rule](#the-naming-rule) asks nothing of them |

Files opt into a **shared** namespace with a directive before the package clause, which is how one logical unit spans several files:

```go
//declscope:namespace user
package repo
```

The name must be an unexported identifier. Placement alongside a package comment is described under [File-level directives](#file-level-directives).

> [!NOTE]
> The **namespace count** is taken over the package's non-test files only. It is what [`rules.naming.qualify: ondemand`](#qualify) keys off. A test file joins its subject's namespace rather than adding one, and `integration_test.go` adds none.

### The core namespace

One namespace in a package may be its **core**: the unit the package is named for, whose prefix is empty.

```go
// client.go
//declscope:core

package transport
```

- Several files may carry `//declscope:core`. They share the one core namespace. This is how `//declscope:namespace` merges files, except that the core has no name to write.
- The core's prefix is empty. So [`qualify`](#qualify) has nothing to ask a name to carry. The core is outside the rule.
- Nothing is lost by that. Having no prefix still names exactly one unit, so `Load()` read in any file still says which unit owns it.

> [!IMPORTANT]
> `//declscope:core` decides the **namespace**, and nothing else. It carries no scope.
>
> A core declaration still takes [`defaults.unexported`](#configuration), so by default it is private *to the core*, and a file outside the core that names it crosses a boundary. Widening is stated the same way as anywhere else.
>
> | Written on a file | Effect |
> | --- | --- |
> | `//declscope:core` | The file joins the core namespace. What it declares keeps the default scope |
> | `//declscope:core` and `//declscope:package` | Both apply. The file is core, and what it declares is package-wide |
> | `//declscope:core` and `//declscope:namespace` | Refused. A core file's namespace *is* the core |

A package that turns [`rules.naming.exported`](#configuration) on will want a core. Prefixes on exported names are the package's API. The core is how a package says *these names are the API. Leave them as they are.*

```go
// with client.go marked core, and retry.go not:
transport.New                 // core: no prefix asked for
transport.RetryWithBackoff    // retry: prefixed like any other unit
```

> [!TIP]
> Without a core, `ondemand` reads the namespace count. A package that gains or loses its second unit then changes what it asks of names that did not move. Inside the core, that question never arises.

## Scopes

By default the **subject** is a package's unexported surface. That means its unexported package-level declarations, and the unexported [members](#members) of any type.

An exported identifier is published to every importer, so nothing narrows it by default. A directive still binds it when the author writes one.

Within that surface, a **scope** is how far a declaration may be used. There are two:

| Scope | Meaning | Rust equivalent |
| --- | --- | --- |
| `package` | Usable anywhere in the package | `pub(super)` |
| `private` | Usable only inside its own [namespace](#namespaces) | No modifier |

> [!NOTE]
> Rust has both levels natively, because a file is a module. An item with no modifier is visible in its own module, which is the namespace. `pub(super)` lifts it to the sibling files. Go collapses the two levels because a package spans its files. `private` is the default a Rust module already has.

> [!IMPORTANT]
> There is no `public`. Go already spells that with a capital letter. A use beyond the package edge is one declscope never sees ([Limits of the analysis](#limits-of-the-analysis)), so `public` and `package` would name a distinction the analysis cannot make. A scope that cannot be checked is exactly the unwritten convention this tool exists to replace.

### Scope resolution

A declaration's scope is decided by the first row that applies:

| The declaration | Scope |
| --- | --- |
| Carries a scope directive (`//declscope:package`, `//declscope:private`) | The directive's |
| Is contained by something that carries one — a [field](#members) by its type, a spec by its `var`/`const`/`type` block | The container's |
| Sits in a file carrying a [file-level scope directive](#file-level-directives) | The file's |
| Is exported | `package`. Nothing narrows it that the analysis did not read in the source |
| Otherwise | [`defaults.unexported`](#configuration), `private` unless configured |

> [!IMPORTANT]
> Exportedness decides the default, and nothing else. The owner plays no part. An **exported field of an unexported type** still resolves to `package`, so making the type unexported protects nothing.
>
> To hold exported fields under a boundary, state it. `//declscope:private` on the type binds every field it reaches:
>
> ```go
> // A DTO capitalized for encoding/json rather than for an audience. Nothing
> // guesses that. One line says it, and the fields are bound by it.
> //
> //declscope:private
> type entry struct {
> 	Key   string `json:"key"`
> 	Value []byte `json:"value"`
> }
> ```
>
> What the author knows, the author states. A directive binds whatever it reaches, exported or not.

Widening is always **stated**. It comes from a directive on the declaration, on what contains it, or on its file, or else from the `defaults` key.

A declaration's *name* plays no part. The namespace in a name marks ownership ([`qualify`](#qualify)) and grants nothing. So a prefix can be added for legibility without changing what the declaration reaches, and a codebase that prefixes everything loses no protection.

```go
// user.go   (namespace: user)

func UserLoad()   {} // exported: no scope, no boundary
func userCache()  {} // private: only namespace "user" may use it

//declscope:package
func userShared() {} // package: usable anywhere in the package
```

### Members

A **member** is a name written *inside* a type's declaration. Two things are:

- a struct's **field**
- an interface's **method name**

For a member, the file is not a choice. It is wherever the type is. So the type's directive reaches it, for the same reason a directive on a `var (...)` block reaches its specs. This is **containment, not inheritance**: there is no second file for it to disagree with.

A **method with a receiver** is not a member, however much it reads like one. It is an ordinary top-level declaration that happens to name a receiver. Its own file gives it its namespace, exactly as for a `func`, and nothing above it reaches it.

| | Package-level declaration | Member | Method with a receiver |
| --- | --- | --- | --- |
| Bounding namespace | Its file's | Its **type**'s file's, which is where it is written | Its file's |
| Contained by | Its `var`/`const`/`type` block | Its **type** | Nothing |
| [Naming rules](#the-naming-rule) | Apply | Do not apply | Do not apply |

> [!TIP]
> An unexported interface method is the **sealed interface** idiom: only this package can spell the name, so only this package can implement. The boundary gives it file granularity.
>
> ```go
> // user.go
> type Sealed interface {
> 	sealed() bool
> }
> ```
> ```go
> // order.go
> func orderRun(s Sealed) bool { return s.sealed() } // reported
> ```
>
> Satisfying the interface is **not** a use of the name. A method set is resolved, not written, so a type implementing `Sealed` from another namespace crosses nothing. Naming the method does cross.

> [!IMPORTANT]
> A **type alias** declares a name, not a type, and containment reads the declaration as written. A directive on one therefore reaches:
>
> | Written | Reached |
> | --- | --- |
> | `type al = impl` | `al` itself. Not `impl`, and not `impl`'s members: those sit inside `impl`'s own declaration, in whatever file holds it |
> | `type entry = struct{ key string }` | `entry`, **and** `key`, which is written inside this declaration |
>
> ```go
> // impl.go
> type impl struct{ n int }
>
> // alias.go
> //declscope:package
> type al = impl // widens al; impl.n keeps the boundary impl.go gives it
> ```
>
> The alias name is an ordinary declaration, so it is bounded by its own file and a directive on it is neither ignored nor malformed. To widen or narrow a defined type's members, write the directive where they are.

So splitting a type's methods across files works the way Go programmers already write it. `marshalKey` in `json.go` belongs to namespace `json`, and `json.go` may use it.

What `sort.go` may not do is reach into `User`'s **members**. That is the boundary that carries the weight. A method written on another namespace's type reaches that type's internals by naming them, and naming them is reported.

> [!WARNING]
> Operations on the **whole value** name no field: copying it, comparing it, zeroing it. They are outside what this can see. See [Limits of the analysis](#limits-of-the-analysis).

The naming rule does not reach members or methods. Both are already qualified by their type at every use (`u.save()`). They collide with nothing, and a prefix would only stutter (`u.userSave()`).

What a member lacks in Go is encapsulation. Every unexported field is visible to its whole package. The `private` scope supplies what is missing:

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

A **rule** is one check. There are three. A rule's name is simultaneously:

- the diagnostic's category
- its configuration key
- the key of its [baseline](#adopting-on-an-existing-codebase) entry
- what [`//declscope:ignore`](#directives) targets

A report with no rule would be one nothing could silence and no baseline could absorb.

| Rule | Reports | Fix | Configurable |
| --- | --- | --- | --- |
| [`boundary`](#boundary) | A declaration used from outside the namespace it is private to | Insert `//declscope:package` | No |
| [`qualify`](#qualify) | A package-level declaration whose name does not carry its namespace | Rename to prefix it | `rules.naming.qualify`, `rules.naming.exported` |
| [`directive`](#unused-and-malformed-directives) | A directive that binds nothing, or that is malformed or misplaced | None — what to write instead is the author's decision | No |

> [!NOTE]
> Reach enforcement has no switch. Naming discipline has one. `boundary` is silenced per declaration with `//declscope:ignore boundary`, or per codebase with a [baseline](#adopting-on-an-existing-codebase).

### `boundary`

`boundary` reports a `private` declaration used from outside its namespace. It covers package-level declarations and [members](#members) alike. For a member, the boundary is the namespace of its type.

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

Every **crossing** use site is attached to the diagnostic as related information. Uses from inside the declaration's own namespace are not.

The report lands on the **declaration**. A method written on another namespace's type is not itself a crossing, because it belongs to the file that wrote it. But the fields it reaches for are reported where they are declared:

```go
// order.go  (namespace: order)
func (u *User) leak() int { return u.id } // u.id is reported, against user.go
```

The fix inserts `//declscope:package` above the declaration.

> [!IMPORTANT]
> A declaration that states its **own** scope with a directive is reported **without a fix**. The directive and the use site are both deliberate, and `-fix` must not overwrite what the author wrote.
>
> A `private` inherited from the declaration's *type* or from a *file-level* directive does not withhold the fix. The inserted directive sits on the declaration, which outranks both. So the fix states an exception to a default. It does not overwrite a decision.

### The naming rule

`qualify` governs one property of a package-level declaration: whether its name carries the namespace of its file, anywhere, starting at a word boundary. `statementReducer` carries `reducer`, and `NewTracer` carries `tracer`, as plainly as `userShared` carries `user`. The namespace need not lead the name. Go names a type for what it is and puts the category last, and it spells a constructor `NewX`.

The namespace in the name says which unit owns the declaration. That is what makes a cross-namespace use readable at the call site, in a stack trace, and in a grep result:

```go
// order.go
func orderRun() int {
    return userShared() // the user unit's, and shared
}
```

The mark grants nothing. Reach is stated by [scope](#scope-resolution) alone.

> [!NOTE]
> **"Package-level" means declared at the top level**, as opposed to a [member](#members). It does not mean the `package` scope.
>
> The rule reads no declaration's scope. A `private` declaration is asked for the namespace exactly as a `package` one is. Ownership and reach are separate questions.

The rule does not apply to:

| Declaration | Reason |
| --- | --- |
| A [member](#members) | Already qualified by its type |
| `func main` in package `main` | A name the toolchain requires |
| `TestXxx`, `BenchmarkXxx`, `FuzzXxx`, `ExampleXxx` in a `_test.go` file | The same. `go test` collects them by name, so a rename would leave a function nothing runs |
| A declaration in a namespace that cannot be a prefix (`2fa.go`) | The name could carry `2fa` inside, but the fix prefixes, and no identifier starts with a digit. The rule stays out rather than report what it cannot remedy |
| An exported identifier, unless [`rules.naming.exported`](#configuration) is on | Off by default: how the package's API is spelled is the author's decision, not a linter's |
| A declaration in the [core namespace](#the-core-namespace) | Its prefix is empty; there is nothing to require |

> [!NOTE]
> [Scope](#scopes) stops at the export line, because reach beyond it cannot be checked. The naming rule has a reason to cross it.
>
> The namespace answers one question: *which unit owns this name?* That question is asked wherever the name is read **bare**. Inside the package, an exported name is read exactly as bare as an unexported one: `Load()`, not `store.Load()`. The package qualifier that makes an external use self-explanatory is missing at precisely the use sites the mark exists for.
>
> So the rule is available for exported declarations, and off by default. Where it is on, the rename is never offered ([Withheld renames](#withheld-renames)). The violation is reported either way.

Where [`rules.naming.qualify`](#qualify) is `ondemand`, the namespace count it depends on is the one described under [Namespaces](#namespaces).

**Namespace matching.** A name carries the namespace when the namespace is written anywhere in it, **ignoring case**, starting at a **word boundary**. The match may end inside a word, so a plural or an agent noun still carries it.

| In `user_id.go` (namespace `userID`) | Carries the namespace |
| --- | --- |
| `userIDCache`, `userIdCache`, `useridentity` | Yes |
| `loadUserID`, `parseUserIds` | Yes — anywhere in the name, and the right edge may run on |
| `poweruserID` | No — `user` does not start a word there |
| `user` in `user.go` | Yes — the name is the namespace |

> [!NOTE]
> The left edge is anchored because the right one is not. Without the anchor, `key` would be found in `monkey`, and every short namespace would stop meaning anything. With it, the cost is a name that opens with the namespace by accident: `mode` is found in `models`. That is the narrower failure of the two.

**Rename spelling.** The fix prefixes. Both halves are spelled the way Go spells an initialism, and exportedness is kept:

| Case | Example | Never |
| --- | --- | --- |
| Unexported | `id` → `userID`, `urlPath` → `userURLPath` | `userId` |
| Exported, under [`rules.naming.exported`](#configuration) | `Load` → `UserLoad` | `userLoad` |

> [!IMPORTANT]
> A rename never changes what a name is visible to. A rename that quietly unexported a declaration would delete the package's API to satisfy a linter.
>
> For an exported declaration, the suggested name is all the author gets, since no rename is offered ([Withheld renames](#withheld-renames)). So the message has to be enough to act on.

> [!NOTE]
> The suggestion is a suggestion. `statementReducer` is never asked to become `reducerStatementReducer`, since it already carries `reducer`. A name that carries nothing gets a plain prefix, and the author may prefer to weave the namespace in elsewhere. Any spelling that carries the namespace at a word boundary settles the rule.

**Fix.** The fix renames every use of the declaration in the package. It is offered only when [provably safe](#withheld-renames). The violation is reported either way.

#### `qualify`

`qualify` requires the namespace in the name.

| `rules.naming.qualify` | Effect |
| --- | --- |
| `never` *(default)* | Off. The convention is opt-in: whether a name reads well with its namespace in it depends on the file name's part of speech, which no tool can see. |
| `ondemand` | Required only once the package has a **second namespace**. In a package with one namespace there is no boundary for the name to mark. A namespace repeated in every name would distinguish nothing. |
| `always` | Required in every package, so that a package gaining its second namespace is not a mass rename. |

```
func id does not carry namespace "user" anywhere in its name; rename it to userID
```

#### Withheld renames

A rename is offered only when it provably changes nothing but the spelling.

Go resolves a name from the inside out: local scope, then the file's imports, then the package, then the predeclared names. So a new name that is free at package level can still be bound at a use site.

> [!CAUTION]
> The wrong rename **compiles and computes something else**:
>
> ```go
> var count = 10
> func Add(fooCount int) int { return fooCount + count } // Add(1) == 11
> ```
> ```go
> // after a careless rename of count to fooCount
> func Add(fooCount int) int { return fooCount + fooCount } // Add(1) == 2
> ```

The violation is reported, and the fix withheld, when any of the following holds:

| Condition | Why |
| --- | --- |
| The declaration is exported | Its uses outside the package are never analyzed, so the rename could not be completed. Finishing it by hand is an API change, which is the author's call |
| The new name is already declared in the package | Would not compile |
| The new name is predeclared (`len`, `error`, `string`, …) | The declaration compiles and shadows the builtin for the whole package |
| Any file of the package imports the new name | Go rejects a package-level name that any file imports |
| At some use of the declaration, the new name is bound by a local, parameter, result or type parameter | The use would silently resolve to that instead |
| The declaration is used from a generated or `exclude`d file | Those files are never rewritten, so the use would dangle |
| A file the build configuration excludes writes the declaration's name, or already declares the new one | A `_GOOS` suffix or a `//go:build` line keeps the file out of this configuration. The fix rewrites only what it can see. The other configuration would be left calling a name that no longer exists, or declaring the new one twice |
| A `//go:linkname` or `//export` directive names the declaration | The directive names it as text, which a rename cannot follow |
| Another fix in the same run already renames something to that name | Two declarations would end up with one name: `a.go`'s `bX` and `a_b.go`'s `x` both prefix to `aBX` |
| The package has in-package `_test.go` files that this variant does not see, or its directory cannot be listed | The test files may declare or use the name; the test variant, which sees every file, decides, and its fix covers the non-test files too |

Each condition is conservative: **a doubt withholds the fix, never the diagnostic.** The *exported* row is not a doubt but a certainty. No run of declscope can ever see all the uses of an exported name.

The two rows about files this pass cannot see withhold different amounts, because they differ in what a later run can recover:

| Unseen file | Withholds | Why that much |
| --- | --- | --- |
| In-package `_test.go` | **Every** rename in the package | The test variant does see every test file, so deferring to it costs nothing. Under the default `-test=true` that variant runs, in `declscope` and in `go vet` alike, and its fix covers the non-test files too. With `-test=false`, such a package gets no rename at all |
| Build-excluded (`_GOOS` suffix, `//go:build`) | Only a rename **colliding** with a name that file writes | No variant ever sees it, so withholding wholesale would disable the fix permanently for anything carrying a `_GOOS` suffix. The directory is read instead, and the collision decided per name |

## Directives

A **directive** is a comment beginning `//declscope:` (`/*declscope:` … `*/` is also recognized).

| Directive | Level | Effect |
| --- | --- | --- |
| `//declscope:package`, `//declscope:private` | Declaration or file | States the [scope](#scope-resolution) instead of inheriting it from the level above, which is the type, then the file, then `defaults`. On a type it also reaches the type's [fields](#members), but not its methods: a method takes its own file's level. On a file it is the default for what the file declares |
| `//declscope:ignore` | Declaration or file | Silences every rule |
| `//declscope:ignore <rules>` | Declaration or file | Silences the named [rules](#rules), comma-separated (`qualify`, `boundary,qualify`) |
| `//declscope:core` | File | Joins the file to the package's **core** [namespace](#the-core-namespace). Carries no scope. Mutually exclusive with `//declscope:namespace` |
| `//declscope:namespace <name>` | File | Sets the file's [namespace](#namespaces); the name must be an unexported identifier |

A trailing `// reason` is allowed after any directive:

```go
//declscope:package // shared with the reporting code
func userHelper() {}
```

### Declaration-level placement

A declaration-level directive goes in the doc comment of a declaration, or as a trailing comment on its first or last line. For a multi-line declaration, that is the line of its opening or closing brace or parenthesis:

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

A directive on a parenthesized `var`/`const`/`type` block applies to every spec in it. A spec may carry its own, and the two kinds combine differently:

| On a spec | Against the block's | Why |
| --- | --- | --- |
| **Scope** | **Replaces** it | A declaration in the subject has exactly one scope |
| **Ignore** | **Accumulates** with it | A narrower ignore must never re-enable a rule the block turned off |

```go
//declscope:ignore boundary
var (
	userSeed = 1
	//declscope:ignore qualify
	limit = 2 // boundary is silenced by the block; qualify by the spec
)
```

> [!WARNING]
> A directive written anywhere else after the package clause binds to nothing. That covers one separated from its declaration by a blank line, one inside a function body, and one on a line in the middle of a struct.
>
> Such a directive is reported as **misplaced** rather than dropped in silence. An author who wrote a suppression believes something is silenced.

### File-level directives

A file-level directive is written before the package clause and applies to the whole file. `//declscope:namespace`, `//declscope:ignore` and the scope directives are the file-level directives.

> [!IMPORTANT]
> A file-level scope directive is a **default, not a blanket**. It supplies the scope of every declaration in the file that states none of its own and inherits none from its type. This is what [`defaults.unexported`](#configuration) does for the package. A declaration can still narrow itself back with `//declscope:private`.
>
> For a [member](#members), the file consulted is the one the member is *written in*. That is not always the file that *bounds* it ([Members](#members)).

`//declscope:namespace` matches Go's [directive syntax](https://go.dev/doc/comment#syntax), so `go/doc` strips it from the rendered documentation. In a file with a package comment, it goes where Go places directives: at the bottom of the doc comment, after a blank comment line:

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

> [!NOTE]
> Flush against `package` also works. But if another file in the package carries the real package comment, that leaves a stray blank line in the rendered documentation.

A file-level ignore takes the same argument as the declaration-level form, so the directive means one thing wherever it appears:

```go
//declscope:ignore boundary,qualify

package store
```

A **utility file** is the clearest case for the file level, and it wants directives rather than ignores. Its contents are shared, and its helpers belong to no unit in particular. Both facts are stated, one directive each:

```go
// util.go
//declscope:core    // the unprefixed unit, so nothing is asked of its names
//declscope:package // usable from every file

package store

func must(err error) { ... }
func first[T any](s []T) T { ... }
```

[`//declscope:core`](#the-core-namespace) decides the namespace and `//declscope:package` decides the scope, so `must(err)` reads the same from every file and reaches every one of them.

Without the core, the namespace is required in the names and the same helpers are `utilMust` and `utilFirst`. That is a real choice rather than a worse one: the namespace tells a distant call site whose helper it is. What matters is that both options **state** something. An ignore states nothing.

That is the difference between **endorsing** and **suppressing**:

| | `//declscope:package` on the file | `//declscope:ignore boundary` on the file |
| --- | --- | --- |
| States a scope | Yes | No |
| A declaration can be narrowed back | Yes, with `//declscope:private` | No. It would mean nothing at all |
| [`qualify`](#qualify) still asks for the namespace | Yes, unless the file is also core | No, the rule is off |

> [!NOTE]
> The directive belongs to the **file**, not to the namespace. The namespace comes from the file name or from `//declscope:namespace`, and the package clause has nothing to do with either. Files that share a namespace each need their own. One file cannot silence a rule on behalf of another.

> [!WARNING]
> A file-level scope directive is a default, so a declaration added to the file later inherits it too. That is what a wholly shared file means.
>
> When the file is not wholly shared, leave the default alone. Write `//declscope:package` on the declarations that are shared.
>
> Do not reach for `//declscope:ignore boundary` instead. It is worse than either choice. It states no scope and it silences the rule, so nothing in the file can be narrowed back, and the suppression outlives whatever justified it. For violations that already exist, use a [baseline](#adopting-on-an-existing-codebase): it suppresses them without standing future ones down.

### Ignore levels

A diagnostic is silenced by any directive that covers its rule at any of these levels:

| Level | Covers |
| --- | --- |
| The declaration | Itself |
| The **type**, for a field | Every field written inside its declaration. A method is an ordinary top-level declaration and the type's ignore does not reach it, exactly as its scope directive does not |
| The file, before the package clause | Every declaration in that file |
| A [baseline](#adopting-on-an-existing-codebase) | Every violation recorded in it |

> [!TIP]
> A directive on a type is what an open struct wants, rather than one on every field:
>
> ```go
> //declscope:ignore boundary
> type User struct {
> 	name string
> 	id   int
> }
>
> // NOT covered: a method is its own declaration and needs its own directive,
> // the same way it takes its own file's scope rather than its type's.
> func (u *User) normalize() { ... }
> ```

- Every level is consulted. Every **ignore** that covers the rule counts as used, so overlapping ignores at different levels never make one another look unused.
- A **scope** directive is judged differently. An outer one can leave an inner one with nothing to bind: see [Unused and malformed directives](#unused-and-malformed-directives).
- Ignores are consulted **before** the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

### Unused and malformed directives

A directive that decides nothing is reported, so that neither a suppression nor a claim of intent outlives what justified it. `//declscope:ignore qualify` is unused if nothing but `qualify` would have fired.

A **scope** directive is judged by what it binds, not by what it sits on. A declaration is bound when the scope named is one it could not have had anyway, **under every configuration**. Quantifying over configurations is what keeps the answer out of the configuration's hands:

| The declaration | `//declscope:package` | `//declscope:private` |
| --- | --- | --- |
| Unexported | Binds — the default may be either scope | Binds, for the same reason |
| Exported | Inert. It has no boundary under any configuration, so naming `package` decides nothing | Binds. It narrows something nothing else would |

> [!NOTE]
> A directive that names the **default of the day** is never reported. Recording that a declaration is private *on purpose* stays legal, because tomorrow's default may differ.
>
> Meanwhile `//declscope:package` on an exported func binds nothing anywhere in its reach, and is reported. The same goes for one on a type whose every field is exported.
>
> Comparing against `defaults.unexported` directly would have done neither. It would have turned the deliberate record into an error. It would also have turned one line of `.declscope.yaml`, or a config file appearing in a parent directory, into hundreds of diagnostics in files nobody touched.

Silencing and accounting:

- These reports carry the `directive` rule, so `//declscope:ignore directive` silences one, at the declaration or the file.
- A bare `//declscope:ignore` covers it at the **file** level, but not on the declaration carrying it. An ignore that could exempt itself from ever being called unused would answer the one report written to catch it.
- They take no [baseline](#adopting-on-an-existing-codebase) entry, and want none. A baseline exists so that turning declscope on does not report boundaries a codebase never enforced. That is history nobody can edit. A directive the author wrote is not history: deleting it removes the report.
- Accounting is **per physical directive**, however many declarations it reaches. Take one written on a `var (...)` block, on `var a, b`, or on `x, y int` in a struct. It counts as used as soon as **any** of them needed it. When none did, it is reported once, not once per name. A directive used up only by a member still counts as used.

| Report | Meaning |
| --- | --- |
| `unused //declscope:ignore boundary on userSeed, limit` | No named declaration needed it |
| `unused file-level //declscope:ignore qualify` | Nothing in the file needed it |
| `unused //declscope:ignore: no checked declaration carries it` | Written on something declscope does not check: `init`, `_`, an embedded field |
| `unused //declscope:package: every declaration it reaches states its own scope` | A block's directive that every spec overrode. One spec taking it is enough to keep it |
| `unused //declscope:package on Helper: nothing it reaches takes a scope` | Every declaration it reaches is exported, where `package` is the scope they already have |
| `unused file-level //declscope:package` | Every declaration in the file is out of the subject, or states its own scope |

> [!IMPORTANT]
> An **ignore** is called unused only by a pass that sees **every** reference in the package. Otherwise an ignore needed only by a test would be unused in the ordinary variant and necessary in the test variant. No single answer would satisfy both.
>
> | Variant | Unused-**ignore** reports | Unused-**scope** reports |
> | --- | --- | --- |
> | Package with no in-package tests | Yes | Yes |
> | Ordinary variant, package with in-package tests | Deferred to the test variant | Yes |
> | Test variant | Yes, it sees every file | Yes |
> | `-test=false`, package with in-package tests | **None at all** | Yes |
>
> A **scope** directive reads no references, so it is judged in every variant.

A malformed directive is reported at the comment:

| Directive | Report |
| --- | --- |
| `//declscope:foo` | `unknown directive declscope:foo` |
| `//declscope:package x` | `//declscope:package takes no argument` |
| `//declscope:private` and `//declscope:package` on one declaration, or on one file | `conflicting scope directives: …` |
| `//declscope:core` and `//declscope:namespace` on one file | `conflicting namespace directives: a core file's namespace is the core` |
| `//declscope:ignore foo` | `unknown rule "foo" in declscope:ignore (want one of boundary, qualify, directive)` |
| `//declscope:namespace` after the package clause | `declscope:namespace must appear before the package clause` |
| `//declscope:namespace` with no name, a second one, or a name that is not an unexported identifier | Reported as such |
| `//declscope:core` with an argument, or a second one on the same file | Reported as such |

## Adopting on an existing codebase

A **baseline** records the violations a codebase has, so that turning declscope on does not report every boundary that was never enforced:

```console
declscope baseline ./...       # writes .declscope-baseline.yaml
```

Recorded violations are suppressed. New ones are reported. The file is discovered by the same upward lookup as the config, so its presence is all it takes.

```yaml
packages:
  github.com/you/app/store:
    boundary:
      user:
        - User.name
        - helper
      (core):
        - dial
    qualify:
      user:
        - helper
```

> [!IMPORTANT]
> An entry is keyed by **package, rule, namespace and declaration**, never by position. It survives the code moving within its file, and the package being reformatted.
>
> It deliberately does **not** survive a declaration moving to a file in another namespace. `boundary` is a statement about which namespaces a use crosses. Once the declaration is declared elsewhere, the same use site produces a different violation. An entry blind to that would go on suppressing a crossing nobody recorded.

| | Spelling |
| --- | --- |
| A package-level declaration | `helper`, under its file's namespace |
| A [member](#members) | `Type.member`, under the namespace of its **type**'s file |
| The [core namespace](#the-core-namespace) | `(core)`. It has no name of its own, and a normalized namespace is alphanumeric, so the parentheses cannot collide with one |

> [!TIP]
> Renaming a file changes the namespace of everything it declares. Every entry recorded against it stops matching at once. This is the same rule doing the same thing: those are different crossings now. The answer is the same as for any other change. Regenerate the baseline and read the diff.

A baseline is regenerated, never edited:

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

> [!NOTE]
> An entry whose violation has been fixed disappears on regeneration.
>
> The analyzer **never reports an entry as stale**. A package's test variant sees references the ordinary variant does not, so "this entry matched nothing" is not a reliable signal from inside one pass. Regeneration analyzes the test variants too, so a violation that only a test's reference produces is still recorded.

A baseline **suppresses; it does not endorse**:

- Nothing is written into the source, so the rules apply to every new declaration.
- An entry is removed only by fixing the violation. This is what distinguishes it from silencing the same violations with directives.
- A configured baseline that does not exist yet behaves as an empty one.

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

- A subtree with its own baseline keeps it. A run from a subdirectory writes under that subdirectory rather than rewriting a baseline above it with only part of its entries.
- Every file written is regenerated **wholesale**. The existing one is never read, so a baseline that fails to parse is replaced like any other.

> [!WARNING]
> Some packages cannot reach the working directory by that lookup: one in another module, or one outside the directory the command runs from. The run then **refuses** and names those packages. It does not record entries where nothing would find them.

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
| A directive is a durable record of intent | When `//declscope:package` is in the source, the next agent inherits the decision instead of re-deriving it. The next agent that widens something in silence is caught |

A line in `CLAUDE.md`, or the equivalent for the agent in use, is enough:

```markdown
Run `declscope ./...` before finishing. Do not widen a declaration's scope to make
a call site compile: either keep the call inside the namespace, or state the new
scope with `//declscope:package` and say why.
```

> [!IMPORTANT]
> The second sentence is the one the mechanism depends on. The cheapest repair for a `boundary` diagnostic is to **widen** the declaration. A `//declscope:package` inserted for that reason is itself a change of reach. The instruction makes the agent write that change down, and justify it, where the next reader will find it.

## Limits of the analysis

| Construct | Treatment |
| --- | --- |
| Generated files (`// Code generated ... DO NOT EDIT.`) | Excluded entirely: neither checked nor treated as reference sites |
| Files matching [`exclude`](#configuration) | The same |
| A use of an *exported* identifier outside its package | Out of scope. Everything is checked within one package, and `go/analysis` has no upward view of the program. An unused-code linter answers this instead. This is why [scope](#scopes) stops at the export line, and why [`rules.naming.exported`](#configuration) reports a naming violation on an exported declaration without a fix |
| Namespaces in different packages | Never collide, for the same reason: a namespace is implicitly package-qualified |
| An embedded field | Not a member: it has no name of its own, only the embedded type's. Embedding is a **use** of that type. So `type B struct{ aCount }` written outside `aCount`'s namespace is a `boundary`. Renaming the type rewrites the embedding, and every `b.aCount` selection through it |
| Members of generic types | Checked like any other: `List[int].items`, and `l.items` inside `List[T]`'s own methods, are uses of `List.items` |
| Fields of anonymous structs, and of types declared inside a function | Not checked |
| A type alias | Its own name is a declaration and takes a scope like any other. A directive on it does not reach the members of the aliased **defined** type (see [Members](#members)), so their reach is stated where they are written. An alias to a struct written inline does contain its fields |
| Satisfying an interface | Not a use of its method names. A method set is resolved, not written, so a type implementing an interface from another namespace crosses nothing. Naming a method does cross |
| An unexported method grown on another namespace's type but **never used** | Not reported: `boundary` needs a use to find, and a method with none is dead code, an unused-code linter's business |

## License

MIT
