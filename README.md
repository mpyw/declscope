# declscope

[![CI](https://github.com/mpyw/declscope/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/declscope/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/declscope.svg)](https://pkg.go.dev/github.com/mpyw/declscope)

Keep your Go packages **flat** without letting them turn into a free-for-all.

declscope holds each declaration to the file that declares it. Another file may not use it. This is the `private` that Go has no word for, checked at build time.

## Why

Go advises few, large packages. Splitting a package to create a boundary has a price:

- Import cycles, and interfaces written only to break them
- **Every name the two halves share has to be exported.** You wanted a boundary between two files, and you published an API. Keeping the exposure down means an `internal/` at every boundary
- **A wrong boundary costs more.** Moving a declaration between files is free. Moving it between packages breaks every importer

`internal/` bounds who may import a package. The reach is the tree rooted at the parent of `internal`, not the whole module. It still adds no level below unexported. A shared name is still capitalized, and still reaches every file in that package.

So most Go code is better off flat. The cost appears inside the package. Go has two levels of visibility, and the lower one covers the whole package:

| Level | Reach |
| --- | --- |
| Exported | Every importer |
| Unexported | **Every file in the package** |

There is no third level. In a flat package, every helper is a package-wide name. Every field is a package-wide reach.

Teams handle this with a convention: *this helper belongs to this file, so do not call it from another file.* The convention is real. It lives only in the heads of the people who wrote the package. The compiler does not know it.

An AI agent does not know it either. The agent sees an unexported helper in scope, so it calls it. It sees an unexported field, so it writes to it. Each choice compiles. Each one passes a quick review. The package becomes a mesh, one edit at a time.

declscope checks the convention instead. The boundary stays inside the flat package, and the compiler is not asked to enforce it. When an agent crosses a boundary, it is told what it crossed and given a fix. The decision is then written in the source, where the next agent reads it.

### What it reports

One `database` package holds a repository per entity.

```go
// user_repository.go
package database

import (
	"context"
	"database/sql"
	"strings"

	"example.com/app/domain"
)

type UserRepository struct{ db *sql.DB }

func (r *UserRepository) Find(ctx context.Context, id int64) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, email FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var email string
	if err := row.Scan(&u.ID, &email); err != nil {
		return nil, err
	}
	u.Email = normalizeEmail(email)
	return &u, nil
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
```
```go
// order_repository.go
package database

import (
	"database/sql"

	"example.com/app/domain"
)

type OrderRepository struct{ db *sql.DB }

func scanOrder(rows *sql.Rows) (domain.Order, error) {
	var o domain.Order
	var email string
	if err := rows.Scan(&o.ID, &email); err != nil {
		return domain.Order{}, err
	}
	o.BuyerEmail = normalizeEmail(email)
	return o, nil
}
```

The two helpers do the same job. Only one of them can be called `scan`, so the package names them apart by hand. That mark says which repository owns which helper, and nothing holds anyone to it.

`normalizeEmail` was written for `scanUser`. `scanOrder` calls it, and the compiler accepts that.

```console
$ declscope ./...
user_repository.go:28:6: func normalizeEmail is private to namespace "userRepository", but is used from namespace "orderRepository"
order_repository.go:17:17:      used here, in namespace "orderRepository"
```

A crossing has two answers.

| Answer | How |
| --- | --- |
| Keep the boundary | Move the call inside the namespace |
| Share on purpose | Write `//declscope:package`, which `-fix` inserts |

Here the helper belongs to neither repository. The repair is a third file, and a stated scope.

```go
// email.go
package database

import "strings"

//declscope:package
func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
```

The decision now sits in the source, where the next reader finds it.

> [!TIP]
> `-fix` always widens, because that is the repair a tool can apply. It writes the directive where the declaration stands, and never moves a declaration to another file. Where the boundary is worth keeping, move the call and leave the declaration alone.

### Where declscope sits

Three linters draw boundaries in Go, at three scales.

![A Go program drawn as nested frames. Between the api and database packages, depguard asks whether one package may import another. A green arrow runs from api to database, and a red one back from database to api is crossed out. Inside database, between user_repository.go and order_repository.go, declscope asks whether one file may reach another's declaration. A red arrow from scanOrder to normalizeEmail is crossed out. At the edge of the program, deadcode asks whether anything is reachable at all. The mail package sits greyed out with no arrow entering it, captioned unreachable.](docs/boundaries.png)

| Linter | Scale | The question it answers |
| --- | --- | --- |
| [`depguard`](https://github.com/OpenPeeDeeP/depguard) | Between packages | May this package import that one? |
| **declscope** | Within one package | May this file reach that declaration? |
| [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) | Whole program | Is this reachable at all? |

The three compose. `depguard` keeps the package graph honest, declscope keeps each package honest inside, and `deadcode` removes what neither needs to reach.

## Install

| Method | Command | Needs |
| --- | --- | --- |
| **[mise](https://mise.jdx.dev/)** *(recommended)* | `mise use -g "github:mpyw/declscope"` | Nothing. Installs the prebuilt binary |
| `go tool` | `go get -tool github.com/mpyw/declscope/cmd/declscope@latest` | Go 1.24+ |
| `go install` | `go install github.com/mpyw/declscope/cmd/declscope@latest` | A Go toolchain |
| Release archive | See below | Nothing |

```bash
declscope ./...
```

<details>
<summary>Pin a version, run through <code>go vet</code>, or install from an archive</summary>

Pin it per project in `mise.toml`:

```toml
[tools]
"github:mpyw/declscope" = "0.5.0"
```

As a tool dependency in `go.mod`:

```bash
go get -tool github.com/mpyw/declscope/cmd/declscope@latest
go tool declscope ./...
```

Through `go vet`, which runs it with the same package loading as the rest of your vet checks:

```bash
go vet -vettool=$(which declscope) ./...
```

Without installing anything:

```bash
go run github.com/mpyw/declscope/cmd/declscope@latest ./...
```

From a release archive, verified against the published checksums:

```bash
VERSION=0.5.0
curl -LO "https://github.com/mpyw/declscope/releases/download/v${VERSION}/declscope_${VERSION}_darwin_arm64.tar.gz"
curl -LO "https://github.com/mpyw/declscope/releases/download/v${VERSION}/checksums.txt"
shasum -a 256 -c checksums.txt --ignore-missing
tar xzf "declscope_${VERSION}_darwin_arm64.tar.gz"
```

</details>

### Flags

| Flag | Default | Effect |
| --- | --- | --- |
| `-config` | *(discovered)* | Path to a YAML config file. Skips the [`.declscope.yaml` lookup](#configuration) |
| `-test` | `true` | Analyze `*_test.go` files as well |
| `-fix` | `false` | Apply suggested fixes |
| `-diff` | `false` | With `-fix`, print a diff instead of writing files |

`-test`, `-fix` and `-diff` come from `go/analysis`. `declscope -help` lists the rest.

Every diagnostic carries **at most one** fix, so `-fix` never has to choose.

## Configuration

Configuration is optional, and this is all of it.

- Read from `.declscope.yaml` or `.declscope.yml`.
- Looked up from the analyzed package's directory **upwards**, stopping at the module root. A subtree can relax or tighten the rules on its own.
- [`-config`](#flags) names a file explicitly and skips the lookup.
- An empty file is a valid config that changes nothing.

```yaml
defaults:
  unexported: private     # package | private

rules:
  naming:
    qualify: never        # always | never | ondemand
    exported: false       # true | false
    vocabulary:
      mouse: [wheel]
  allowBoundary: false   # true | false
  allowSurplus: false    # true | false

filter:
  only: []              # nothing outside these, when set
  omit:
    - "**/mock_*.go"    # and not these

baseline: .declscope-baseline.yaml
```

| Key | Values | Default | Effect |
| --- | --- | --- | --- |
| `defaults.unexported` | `package`, `private` | `private` | Scope of a declaration that carries no directive and inherits none |
| `rules.naming.qualify` | `always`, `never`, `ondemand` | `never` | When a name must carry its namespace. See [the naming rule](#when-it-applies) |
| `rules.naming.exported` | `true`, `false` | `false` | Whether the naming rule also reaches exported declarations. The rename is never offered there |
| `rules.naming.vocabulary` | Namespace to a list of words | None | Extra words that carry a namespace. See [what carries a namespace](#what-carries-a-namespace) |
| `rules.allowBoundary` | `true`, `false` | `false` | Turns the [`boundary`](#boundary) rule off, leaving only the naming rule. See [reach without a boundary](#reach-without-a-boundary) |
| `rules.allowSurplus` | `true`, `false` | `false` | Turns the [`surplus`](#surplus) rule off. The rule is on, so the key names what switching it does |
| `filter.only` | Path globs, read against the config file's own directory | None | When set, no file outside them is read. Empty places no restriction |
| `filter.omit` | The same globs | None | Files taken back out, whether or not `only` let them through |
| `baseline` | A path relative to the config file | The nearest `.declscope-baseline.yaml` | The [baseline](#adopting-on-an-existing-codebase) to consult |

In a glob:

| Pattern | Matches |
| --- | --- |
| `*`, `?` | Within one path segment |
| `**` | Across segments |

`filter` decides which files are read at all. A file matches a list when it matches **any** pattern in it.

| Written | Read |
| --- | --- |
| Neither | Every file |
| `only` alone | Nothing outside the patterns |
| `omit` alone | Everything except the patterns |
| Both | `only` first, then `omit` taken out of what it left |

An empty `only` places no restriction rather than matching nothing, which is why a repository with no config is read whole. `omit` is the stronger of the two: a file it names is not read even when `only` admitted it.

> [!NOTE]
> The order in "only first, then omit" is for the reader. Both lists ask about one path, so narrowing before subtracting and subtracting before narrowing name the same set.

A filter pattern is read **against the directory of the config file that states it**, and whether it is anchored there follows the rules a `.gitignore` uses:

| Pattern | Matches |
| --- | --- |
| `gen.go` | A file of that name at any depth. A bare name carries no place |
| `/gen.go` | The one beside this config file. A leading `/` means "here", not the root of the filesystem |
| `**/gen/**` | That directory at any depth |
| `gen/**` | The one directory beside this config file, and no other |

A pattern that already holds a separator is anchored whether or not it starts with one, so `/gen/**` and `gen/**` are the same rule. The leading `/` earns its keep on a bare name, which would otherwise float.

So the same line means different things in different files: `internal/tui/**` in the config at the repository root reaches `internal/tui`, and in `internal/.declscope.yaml` it reaches `internal/internal/tui`. A pattern cannot leave its own directory — a `..` in one is an error rather than a rule that matches nothing.

The directory of the config file is the origin rather than the module root, because a config governs only the packages that find it by walking up. A pattern anchored at the module root but written in a nested config could only name files that never consult that config, so it would match nothing by construction.

> [!NOTE]
> An **unknown key is an error**, not a silent no-op. A typo in a rule name cannot leave the rule at its default with no sign of it. The message names the key, and the keys its section does take. A value a key does not accept is an error in the same way.
>
> `defaults` takes only `unexported`, because an exported declaration has no scope to default. `boundary` has no key at all.

## Directives

A **directive** is a comment beginning `//declscope:`. The form `/*declscope: ... */` also works.

| Directive | Level | Effect |
| --- | --- | --- |
| `//declscope:package`, `//declscope:private` | Declaration or file | States the [scope](#scope-resolution). On a type it also reaches the type's [fields](#members), but not its methods |
| `//declscope:ignore` | Declaration or file | Silences every rule |
| `//declscope:ignore <rules>` | Declaration or file | Silences the named [rules](#rules), comma-separated |
| `//declscope:core` | File | Joins the file to the [core namespace](#the-core-namespace) |
| `//declscope:namespace <name>` | File | Joins the file to a [shared namespace](#namespaces) |

### Placement

A declaration-level directive goes in the doc comment, directly above the declaration.

```go
//declscope:package
func userSeed() {}
```

A file-level directive goes above the package clause. `//declscope:namespace` must come before it.

```go
//declscope:package

package database
```

Three placements are accepted there. Go excludes a `//tool:name` comment from a doc comment, so none of them reaches the rendered documentation.

| Placement | |
| --- | --- |
| A blank line between the directive and `package` | What this README writes |
| The directive directly above `package` | Accepted |
| At the bottom of the package doc comment, after a blank `//` line | Go's own convention for a directive in a doc comment |

> [!WARNING]
> Above the package clause, only the `//` form stays out of the documentation. Go excludes `//tool:name` from a doc comment, and it does not exclude `/*declscope: ... */`.
>
> ```go
> /*declscope:namespace shared*/
> package blk
> ```
> ```console
> $ go doc .
> package blk // import "example.com/blk"
>
> declscope:namespace shared
> ```
>
> The directive still takes effect. It also becomes the package comment, and pkg.go.dev shows it. Use the `//` form on a file.

A directive on a block reaches every spec in it. A directive on one spec overrides the block's.

```go
//declscope:package
var (
	seed  = 1
	//declscope:private
	limit = 2 // this one is private
)
```

A directive on a type reaches its fields and its interface method names. It does not reach its methods, which take their own file's level.

### Ignore levels

Every level is consulted. An ignore counts as used whenever it covers a rule that would have fired, so overlapping ignores never make one another look unused.

Ignores are consulted **before** the baseline, so a suppression the baseline would also have absorbed still counts as the directive doing its job.

### Unused and malformed directives

A directive that decides nothing is reported. Neither a suppression nor a claim of intent should outlive what justified it.

A **scope** directive is judged by what it binds. A declaration is bound when the scope named is one it could not have had **under any configuration**.

| The declaration | `//declscope:package` | `//declscope:private` |
| --- | --- | --- |
| Unexported | Binds. The default may be either scope | Binds, for the same reason |
| Exported | Inert. It has no boundary under any configuration | Binds. It narrows something nothing else would |

Quantifying over configurations keeps the answer out of the configuration's hands. A directive that names today's default is never reported, and one line of `.declscope.yaml` never turns into hundreds of diagnostics.

<details>
<summary>What each unused-directive report means</summary>

| Report | Meaning |
| --- | --- |
| `unused //declscope:ignore boundary on userSeed, limit` | No named declaration needed it |
| `unused file-level //declscope:ignore qualify` | Nothing in the file needed it |
| `unused //declscope:ignore: no checked declaration carries it` | Written on something declscope does not check, such as `init` or `_` |
| `unused //declscope:package: every declaration it reaches states its own scope` | A block's directive that every spec overrode |
| `unused //declscope:package on Helper: nothing it reaches takes a scope` | Everything it reaches is exported |
| `unused file-level //declscope:package` | Every declaration in the file states its own scope, or is out of the subject |

</details>

Accounting is **per physical directive**, however many declarations it reaches. One written on a `var (...)` block counts as used as soon as any spec needed it. When none did, it is reported once.

These reports carry the `directive` rule, so `//declscope:ignore directive` silences one. A bare `//declscope:ignore` covers it at the file level, but not on the declaration carrying it. An ignore that could exempt itself would answer the one report written to catch it.

A malformed directive is reported at the comment.

<details>
<summary>Malformed directives</summary>

| Directive | Report |
| --- | --- |
| `//declscope:foo` | `unknown directive declscope:foo` |
| `//declscope:package x` | `//declscope:package takes no argument` |
| `//declscope:private` and `//declscope:package` together | `conflicting scope directives: ...` |
| `//declscope:core` and `//declscope:namespace` together | `conflicting namespace directives: a core file's namespace is the core` |
| `//declscope:ignore foo` | `unknown rule "foo" in declscope:ignore (want one of boundary, qualify, surplus, directive)` |
| `//declscope:namespace` after the package clause | `declscope:namespace must appear before the package clause` |

</details>

<details>
<summary>Which variant reports an unused directive</summary>

> [!IMPORTANT]
> An **ignore** is called unused only by a pass that sees **every** reference in the package. An ignore needed only by a test would otherwise be unused in one variant and necessary in another.
>
> | Variant | Unused-**ignore** reports | Unused-**scope** reports |
> | --- | --- | --- |
> | Package with no in-package tests | Yes | Yes |
> | Ordinary variant, package with in-package tests | Deferred to the test variant | Yes |
> | Test variant | Yes | Yes |
> | `-test=false`, package with in-package tests | **None** | Yes |
>
> A **scope** directive reads no references, so it is judged in every variant.

</details>

## Namespaces

A **namespace** is the unit within which a `private` declaration may be used. By default each file is its own namespace, named after the file.

| File | Namespace |
| --- | --- |
| `user_repository.go` | `userRepository` |
| `user_repository_test.go` | `userRepository`. A test shares the namespace of its subject |
| `parser_linux.go` | `parser`. A GOOS or GOARCH suffix is a build constraint, not a namespace |
| `user_id.go`, `parse_json.go` | `userID`, `parseJSON`. An initialism is spelled the way Go spells it |
| `foo-bar.go`, `Foo.go` | `fooBar`, `foo`. Any separator, and a PascalCase stem, become lowerCamelCase |
| `2fa_auth.go` | `2faAuth`. A valid namespace, but never a prefix |

The namespace comes *from* the file name and is not equal to it. That is what lets you rename a file without renaming what it declares.

Files join a **shared** namespace with a directive before the package clause. This is how one unit spans several files.

```go
//declscope:namespace user
package repo
```

The name must be an unexported identifier.

### The core namespace

One namespace in a package may be its **core**. This is the unit the package is named for.

```go
// client.go
//declscope:core

package transport
```

- Several files may carry `//declscope:core`. They share the one core namespace.
- The core has no prefix, so [the naming rule](#the-naming-rule) asks nothing of its declarations.
- Nothing is lost by that. Having no prefix still names exactly one unit.

> [!IMPORTANT]
> `//declscope:core` sets the namespace and nothing else. It carries no scope.
>
> A core declaration still takes [`defaults.unexported`](#configuration). By default it is private *to the core*, and a file outside the core that names it crosses a boundary.
>
> | Written on a file | Effect |
> | --- | --- |
> | `//declscope:core` | The file joins the core. What it declares keeps the default scope |
> | `//declscope:core` and `//declscope:package` | Both apply. The file is core, and what it declares is package-wide |
> | `//declscope:core` and `//declscope:namespace` | Refused. A core file's namespace *is* the core |

## Scopes

The **subject** of the analysis is a package's unexported surface: its unexported package-level declarations, and the unexported [members](#members) of any type. An exported identifier is published to every importer, so nothing narrows it by default.

A **scope** is how far a declaration may be used. There are two.

| Scope | Meaning | Rust equivalent |
| --- | --- | --- |
| `package` | Usable anywhere in the package | `pub(super)` |
| `private` | Usable only inside its own [namespace](#namespaces) | No modifier |

There is no `public`. Go already spells that with a capital letter, and a use beyond the package edge is one declscope never sees.

### Scope resolution

A declaration's scope comes from the first row that applies.

| The declaration | Scope |
| --- | --- |
| Carries `//declscope:package` or `//declscope:private` | The directive's |
| Is contained by something that carries one | The container's |
| Sits in a file carrying a [file-level directive](#directives) | The file's |
| Is exported | `package` |
| Otherwise | [`defaults.unexported`](#configuration), which is `private` unless configured |

Containment means a [field](#members) inside its type, or a spec inside its `var`, `const` or `type` block.

> [!IMPORTANT]
> Exportedness decides the default, and nothing else. The owner plays no part. An **exported field of an unexported type** still resolves to `package`, so making the type unexported protects nothing.
>
> State the boundary instead. `//declscope:private` on the type binds every field it reaches.
>
> ```go
> // Capitalized for encoding/json, not for an audience.
> //declscope:private
> type entry struct {
> 	Key   string `json:"key"`
> 	Value []byte `json:"value"`
> }
> ```

Surplus is always **stated**. It comes from a directive, or from the `defaults` key.

A declaration's *name* plays no part in its scope. A prefix marks ownership and grants nothing, so you can add one for legibility without changing what the declaration reaches.

### Members

A **member** is a name written *inside* a type's declaration. There are two kinds:

- a struct's **field**
- an interface's **method name**

A declaration's namespace is the file it is **written in**. There is no exception to that. A member is written inside its type, so its namespace is the type's file's, and the type's directive reaches it. This is **containment, not inheritance**. There is no second file for it to disagree with.

A **method with a receiver** is not a member. It is an ordinary top-level declaration that names a receiver. Its own file gives it its namespace.

| | Package-level declaration | Member | Method with a receiver |
| --- | --- | --- | --- |
| Bounding namespace | Its file's | Its **type**'s file's | Its file's |
| Contained by | Its `var`, `const` or `type` block | Its **type** | Nothing |
| [Naming rule](#the-naming-rule) | Applies | Does not apply | Only when filed away from its type |

The two rules ask different questions.

| Rule | Question |
| --- | --- |
| [`boundary`](#boundary) | May this file touch this declaration? |
| [Naming](#the-naming-rule) | Reading this name **on its own**, can the reader tell which unit owns it? |

A method is never read on its own. Every use writes the receiver first, so `u.save()` hands the reader `u` to follow. A member is the same case.

What the receiver names is the **type's** unit. While the method sits beside its type, that is the unit holding it, and the question is answered. A method filed in another namespace points the reader at a unit that does not hold it. The naming rule reaches that one, and asks for the namespace it is written in.

Splitting a type's methods across files is ordinary Go. A builder often declares its state in one file and its chainable methods in another.

```go
// statement.go   (namespace: statement)
package database

type Statement struct {
	Table  string
	wheres []string
}
```
```go
// query.go   (namespace: query)
package database

func (s *Statement) Where(cond string) *Statement {
	s.wheres = append(s.wheres, cond)
	return s
}
```

Writing `Where` in `query.go` is not itself a crossing. The field it reaches is one, and the report lands on `statement.go`.

```console
$ declscope ./...
statement.go:5:2: field Statement.wheres is private to namespace "statement", but is used from namespace "query"
query.go:4:4:   used here, in namespace "query"
query.go:4:22:  used here, in namespace "query"
```

Under the naming rule, `Where` is asked to carry `query` as well.

```console
query.go:3:21: method Where does not carry namespace "query" anywhere in its name; rename it to QueryWhere, or to another name that carries "query"
```

Neither report asks for a rename here. The two files are one unit split in two, and `//declscope:namespace statement` on `query.go` settles both.

> [!TIP]
> An unexported interface method is the **sealed interface** idiom. Only this package can spell the name, so only this package can implement the interface. declscope gives it file granularity.
>
> Satisfying an interface is **not** a use of the name. A method set is resolved, not written. Naming the method does cross.

What a member lacks in Go is encapsulation. Every unexported field is visible to the whole package. The `private` scope supplies what is missing.

> [!WARNING]
> Operations on the **whole value** name no field: copying it, comparing it, zeroing it. They are outside what this can see. See [Limits](#limits).

## Rules

A **rule** is one check. There are four. A rule's name is the diagnostic's category, its [baseline](#adopting-on-an-existing-codebase) key, and what [`//declscope:ignore`](#directives) targets. It is also the configuration key, except where a key reads better named for what it switches: `rules.allowSurplus` turns off `surplus`.

| Rule | Reports | Fix | Configurable |
| --- | --- | --- | --- |
| [`boundary`](#boundary) | A declaration used from outside the namespace it is private to | Insert `//declscope:package` | `rules.allowBoundary` |
| [`qualify`](#the-naming-rule) | A name that does not carry its namespace | Rename to prefix it | `rules.naming.*` |
| [`surplus`](#surplus) | A `//declscope:package` with no visible use from another namespace | None | `rules.allowSurplus` |
| [`directive`](#unused-and-malformed-directives) | A directive that binds nothing, or is malformed | None | No |

Reach enforcement is on, naming discipline is off, and the surplus audit is on. Each is one key away from the other setting.

### `boundary`

`boundary` reports a `private` declaration used from outside its namespace. It covers package-level declarations and [members](#members) alike. For a member, the boundary is the namespace of its type.

The [example above](#what-it-reports) is this rule, in its `defaults` case. The message names where the scope came from.

| Where the scope came from | Message |
| --- | --- |
| `defaults` | `func normalizeEmail is private to namespace "userRepository", but is used from namespace "orderRepository"` |
| The declaration's own directive | `func normalizeEmail is declared private by //declscope:private, but is used from namespace "orderRepository"` |
| Its type's directive | `field Statement.wheres is declared private by //declscope:private on Statement, but is used from namespace "query"` |
| A file-level directive | `func normalizeEmail is declared private by the file's //declscope:private, but is used from namespace "orderRepository"` |

Every crossing use site is attached to the diagnostic. Uses from inside the declaration's own namespace are not.

The report lands on the **declaration**. A method written on another namespace's type is not itself a crossing, because it belongs to the file that wrote it. The fields it reaches for are reported where they are declared.

```go
// query.go  (namespace: query)
func (s *Statement) Where(cond string) *Statement {
	s.wheres = append(s.wheres, cond) // reported, against statement.go
	return s
}
```

> [!IMPORTANT]
> A declaration that states its **own** scope is reported **without a fix**. The directive and the use site are both deliberate, and `-fix` must not overwrite what the author wrote.
>
> A `private` inherited from a type or a file does not withhold the fix. The inserted directive sits on the declaration, which outranks both.

#### Reach without a boundary

`rules.allowBoundary: true` switches this rule off. What is left is the naming rule, for a repository that wants the ownership mark in a name without the scope behind it.

```yaml
rules:
  naming:
    qualify: ondemand
  allowBoundary: true
  allowSurplus: true
```

Set `allowSurplus` alongside it. `surplus` audits `//declscope:package`, and that directive stops meaning anything once nothing checks reach, so the audit would report directives that no longer have a job.

> [!TIP]
> This is not how to adopt declscope gradually. A [baseline](#adopting-on-an-existing-codebase) records what a codebase already has and still reports what is new. A switch reports nothing, and a repository that means to turn it on later never finds out how much it would cost.

### The naming rule

**Off by default.** Turn it on with `rules.naming.qualify`.

The rule asks that a package-level declaration carry the namespace of its file somewhere in its name. The namespace in the name says which unit owns the declaration. That makes a cross-namespace use readable at the call site, and in a stack trace.

```go
// order_repository.go
func scanOrder(rows *sql.Rows) (domain.Order, error) {
	// ...
	o.BuyerEmail = normalizeEmail(email) // the email unit's, and shared
}
```

The mark grants nothing. Reach is stated by [scope](#scope-resolution) alone.

> [!NOTE]
> The rule is opt-in. Whether a prefix reads well depends on the file name, and no tool can see that. A prefix from `comments.go` reads as a noun phrase, `commentsAttached`. The same prefix from `collect.go` reads as a command, `collectAddFunc`.

#### When it applies

| `rules.naming.qualify` | Effect |
| --- | --- |
| `never` *(default)* | Off |
| `ondemand` | Required once the package has a **second namespace**. In a package with one namespace, a prefix repeated everywhere distinguishes nothing. A file holding only a package clause and comments, as a `doc.go` usually does, is not counted |
| `always` | Required in every package, so that gaining a second namespace is not a mass rename |

It never applies to:

<details>
<summary>Declarations the rule never reaches</summary>

| Declaration | Reason |
| --- | --- |
| A [member](#members), or a method written beside its type | Already qualified by that type at every use |
| `func main` in package `main` | A name the toolchain requires |
| `TestXxx`, `BenchmarkXxx`, `FuzzXxx`, `ExampleXxx` in a `_test.go` file | The same. `go test` finds them by name |
| A declaration in a namespace that cannot start an identifier, as in `2fa.go` | The fix prefixes, and no identifier begins with a digit |
| An exported identifier, unless `rules.naming.exported` is on | How the API is spelled is the author's decision |
| A declaration in the [core namespace](#the-core-namespace) | The core has no prefix |

</details>

#### What carries a namespace

The namespace must appear in the name, ignoring case, starting at a **word boundary**. The match may end inside a word.

| In `user_id.go`, namespace `userID` | Carries it |
| --- | --- |
| `userIDCache`, `userIdCache` | Yes |
| `loadUserID`, `parseUserIds` | Yes. Anywhere in the name, and the right edge may run on |
| `poweruserID` | No. `user` does not start a word there |
| `user` in `user.go` | Yes. The name is the namespace |

The left edge is anchored because the right one is not. Without the anchor, `key` would be found in `monkey`.

Two English inflections change the namespace's own spelling, so the free right edge cannot reach them. Both are accepted.

| Namespace | Also carried by |
| --- | --- |
| `store` | `storing`. The final `e` drops before `ing` |
| `apply` | `applies`, `applied`. The final `y` turns to `i` |

Only these whole forms are generated, and always from the namespace's side. The name is never stemmed, so `story` and `storm` do not carry `store`.

`rules.naming.vocabulary` lists extra words that carry a namespace:

```yaml
rules:
  naming:
    vocabulary:
      mouse: [wheel]
      index: [indices]
```

A listed word goes through the same test, so `wheelDelta` carries `mouse` and `pinwheel` does not.

> [!WARNING]
> The vocabulary is for the irregular few. A namespace that needs a long list is a sign of a design problem. The file declares things it is not about, and splitting the file says more than listing them.

#### The rename

The fix prefixes, keeping Go's spelling of an initialism and the original exportedness.

| Case | Example | Never |
| --- | --- | --- |
| Unexported | `id` → `userID` | `userId` |
| Exported, under `rules.naming.exported` | `Load` → `UserLoad` | `userLoad` |

The suggestion is one answer, not the only one. Any spelling that carries the namespace settles the rule.

> [!NOTE]
> Sometimes the namespace is what is wrong, not the name. A namespace comes from the file name, and a file name may hold words that name no unit.
>
> ```console
> user_repository.go:18:6: func scanUser does not carry namespace "userRepository" anywhere in its name; rename it to userRepositoryScanUser, or to another name that carries "userRepository"
> ```
>
> The unit here is `user`, not `userRepository`. Writing `//declscope:namespace user` on the file settles the rule and leaves every name alone.

A rename never changes what a name is visible to. A rename that quietly unexported a declaration would delete the package's API to satisfy a linter.

#### Withheld renames

A rename is offered only when it provably changes nothing but the spelling. The violation is reported either way.

Go resolves a name from the inside out, so a new name that is free at package level can still be bound at a use site.

> [!CAUTION]
> The wrong rename **compiles and computes something else**.
>
> ```go
> var count = 10
> func Add(fooCount int) int { return fooCount + count } // Add(1) == 11
> ```
> ```go
> // after a careless rename of count to fooCount
> func Add(fooCount int) int { return fooCount + fooCount } // Add(1) == 2
> ```

<details>
<summary>The four reasons a fix is withheld</summary>

| Reason | When |
| --- | --- |
| The rename could not be completed | The declaration is exported, a use sits in a generated, filtered-out or build-excluded file, or a `//go:linkname` or `//export` names it as text |
| The new name is taken | It is already declared in the package, predeclared like `len`, imported by some file, or claimed by another fix in the same run |
| The new name would resolve elsewhere | At some use it is bound by a local, parameter, result or type parameter |
| This pass does not read every file | The package has `_test.go` files this variant cannot see. The test variant sees them all and decides for both |

</details>

A doubt withholds the fix, never the diagnostic.

### `surplus`

**On by default.** Turn it off with `rules.allowSurplus: true`.

`surplus` is the converse of `boundary`. It reports a `//declscope:package` directive when declscope sees no use of what it widens from another namespace. The scope is wider than any visible use justifies.

```go
// email.go
package database

//declscope:package
func normalizeEmail(s string) string { return s }
```

```console
$ declscope ./...
email.go:3:1: //declscope:package on normalizeEmail: no use from another namespace is visible to declscope
```

One physical comment gets one report, however many declarations take their scope from it. The message lists them. A declaration that states its own scope does not depend on an outer directive, so it neither keeps that directive alive nor appears under it.

> [!IMPORTANT]
> The rule concludes from an **absence**, and its advice is to delete a directive. A directive can hold up something declscope cannot see, so the rule stays quiet on any doubt. There is no fix for the same reason.

The whole comment stays quiet when any declaration it reaches may be needed.

| Kept alive by | Why the rule cannot rule it out |
| --- | --- |
| A use from another namespace | The directive is doing its job |
| An exported name in the comment's reach | Importers reach it, which one package never sees |
| A method in an interface contract of the package | An interface value reaches the method without spelling it |
| An unexported method carried by an exported type | An importer can embed the type and complete a satisfaction |
| A struct conversion involving the field's type | The conversion pairs every field by name and spells none |
| `//go:linkname` or `//export` naming the declaration | The directive names it as text |

The rule also switches off for a whole package when some reference site was never read.

| Switched off by | What was not read |
| --- | --- |
| A generated, filtered-out, cgo or assembly source | Those files are never read as reference sites |
| A build-excluded file of the package | It may hold the one use |
| In-package `_test.go` files this variant does not see | The test variant sees every file and decides |

> [!NOTE]
> Reach that spells no name and leaves no trace, such as reflection, is invisible here as everywhere. Adopt the rule where the package's reach is expressed in source.

## Adopting on an existing codebase

> [!TIP]
> [`skills/declscope-adoption`](skills/declscope-adoption/SKILL.md) is a skill for an AI agent doing this work. It carries what each diagnostic shape means structurally, and the measurement traps that produce false confidence. Install it with `gh skill install mpyw/declscope`.

A **baseline** records the violations a codebase already has. Turning declscope on then reports only what is new.

```console
declscope baseline ./...       # writes .declscope-baseline.yaml
```

The file is found by the same upward lookup as the config, so its presence is all it takes.

```yaml
packages:
  github.com/you/app/database:
    boundary:
      userRepository:
        - normalizeEmail
      statement:
        - Statement.wheres
      (core):
        - open
```

| | Spelling |
| --- | --- |
| A package-level declaration | `normalizeEmail`, under its file's namespace |
| A [member](#members) | `Type.member`, under the namespace of its **type**'s file |
| The [core namespace](#the-core-namespace) | `(core)` |

An entry is keyed by package, rule, namespace and declaration, never by position. It survives the code moving within its file.

It does **not** survive a declaration moving to another namespace. `boundary` states which namespaces a use crosses. Once the declaration lives elsewhere, the same use site produces a different violation.

A baseline is regenerated, never edited.

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

A baseline **suppresses and does not endorse**:

- Nothing is written into the source, so the rules apply to every new declaration.
- An entry is removed only by fixing the violation.
- A configured baseline that does not exist yet behaves as an empty one.

> [!NOTE]
> The analyzer never reports an entry as stale. A package's test variant sees references the ordinary variant does not, so one pass cannot tell whether an entry matched nothing. Regeneration analyzes the test variants too.

<details>
<summary>Where the baseline is written</summary>

```console
declscope baseline [-o path] [-config path] [packages]   # packages default to ./...
```

Each package's entries go to the file the analyzer will consult for that package.

| Target | When |
| --- | --- |
| The baseline named by the package's nearest config file | That config has a `baseline` key |
| The nearest existing `.declscope-baseline.yaml` above the package | No config names one |
| A new `.declscope-baseline.yaml` in the working directory | Neither of the above exists |

`-o` bypasses that lookup and gathers every entry into one file.

Every file written is regenerated **wholesale**. The existing one is never read, so a baseline that fails to parse is replaced like any other.

> [!WARNING]
> Some packages cannot reach the working directory by that lookup, such as one in another module. The run then **refuses** and names those packages, rather than recording entries where nothing would find them.

</details>

## Using it with an AI agent

declscope runs wherever an agent's edits are checked.

```bash
declscope ./...        # in CI, and in the agent's build loop
declscope -fix ./...   # deterministic: at most one fix per diagnostic
```

On an existing codebase, run `declscope baseline ./...` once first. The agent is then shown only the boundaries its own edits cross.

Two properties distinguish this from a written convention.

| Property | Effect on the agent |
| --- | --- |
| The diagnostic names the namespace crossed | The agent is told why the use is wrong, and the repair is mechanical |
| A directive is a durable record of intent | The next agent inherits the decision instead of re-deriving it |

## Limits

declscope reads one package at a time, and counts a use only where a name is written.

| Not seen | Consequence |
| --- | --- |
| Uses outside the package | A scope beyond `package` could not be checked, so none exists |
| Whole-value operations on a struct | Copying, comparing or zeroing a value names no field |
| Reflection, `//go:linkname`, generated files | These reach a declaration without spelling it |
| A declaration nobody uses | `boundary` needs a use to find, so unused code produces no diagnostic |

For the last row, use an unused-code linter: [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode), or staticcheck's `unused` for a library with no `main` to root from.

## License

MIT
