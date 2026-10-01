<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
    <img src="docs/assets/logo-light.svg" alt="" width="128" height="128">
  </picture>
  <h1>declscope</h1>

  [![CI](https://github.com/mpyw/declscope/actions/workflows/ci.yml/badge.svg)](https://github.com/mpyw/declscope/actions/workflows/ci.yml)
  [![Codecov](https://codecov.io/gh/mpyw/declscope/graph/badge.svg)](https://codecov.io/gh/mpyw/declscope)
  [![Go Reference](https://pkg.go.dev/badge/github.com/mpyw/declscope.svg)](https://pkg.go.dev/github.com/mpyw/declscope)

  <!-- site:skip -->
  <p><a href="https://mpyw.me/declscope/"><img src="https://github.com/user-attachments/assets/69d90557-eacd-4480-8834-cf7f5f28d341" alt="Documentation on GitHub Pages" width="480"></a></p>
  <!-- /site:skip -->

  <p>Write-ups: 🇯🇵 <a href="https://zenn.dev/yumemi_inc/articles/go-declscope-file-scoped-private">JP</a> · 🇺🇸 <a href="https://dev.to/mpyw/declscope-a-linter-that-brings-file-scoped-private-to-gos-flat-packages-2en9">EN</a></p>
</div>

Keep your Go packages **flat** without letting them turn into a free-for-all.

declscope holds each declaration to the file that declares it. Another file may not use it. This is the `private` that Go has no word for, checked at build time.

## Why

Go has two levels of visibility, and the lower one covers the whole package.

| Level | Reach |
| --- | --- |
| Exported | Every importer |
| Unexported | **Every file in the package** |

There is no third level. In a flat package, every helper is a package-wide name, and every field is a package-wide reach.

Splitting the package to get a boundary has a price.

- Import cycles, and interfaces written only to break them.
- **Every name the two halves share has to be exported.** You wanted a boundary between two files, and you published an API.
- **A wrong boundary costs more.** Moving a declaration between files is free. Moving it between packages breaks every importer.

`internal/` does not help here. It limits who may import a package, but it adds no level below unexported. So most Go code is better off flat, and the cost appears inside the package.

Teams handle this with a convention: *this helper belongs to this file.* The convention lives only in the heads of the people who wrote the package.

An AI agent does not know it either. It sees an unexported helper in scope, so it calls it. It sees an unexported field, so it writes to it. Each edit compiles and passes a quick review, and the package becomes a mesh.

declscope checks the convention instead. When an agent crosses a boundary, it is told what it crossed and given a fix. The decision is then written in the source, where the next agent reads it.

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

The two scan helpers are named apart by hand, since only one could be called `scan`. That mark says which repository owns which helper, and nothing holds anyone to it.

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

> [!TIP]
> `-fix` always widens, because that is the repair a tool can apply. It writes the directive where the declaration stands, and never moves a declaration to another file. Where the boundary is worth keeping, move the call yourself.

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
| **[mise](https://mise.jdx.dev/)** *(recommended)* | `mise use "github:mpyw/declscope@0.17.0"` | Nothing. Installs the prebuilt binary |
| `go tool` | `go get -tool github.com/mpyw/declscope/cmd/declscope@latest` | Go 1.27+ |
| `go install` | `go install github.com/mpyw/declscope/cmd/declscope@latest` | Go 1.27+ |
| Release archive | See below | Nothing |

```bash
declscope ./...
```

<details>
<summary>Pin a version, run through <code>go vet</code>, or install from an archive</summary>

`mise use` pins the version in the project's `mise.toml`, so every checkout and CI run the same one. Add `-g` to install it for every project on your machine instead.

```toml
[tools]
"github:mpyw/declscope" = "0.17.0"
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
VERSION=0.17.0
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
| `-V=full` | | Print the version and exit. A build from a checkout answers `devel` |

`-test`, `-fix` and `-diff` come from `go/analysis`. `declscope -help` lists the rest.

### Subcommands

| Command | What it does | Flags of its own |
| --- | --- | --- |
| `declscope survey [packages]` | [Report what was checked and what it found](#measuring-what-is-there), one row per package | `-format`, `-test`, `-config`, `-allow-errors` |
| `declscope inspect <package>` | [Report the shape of one package](#measuring-what-is-there): its namespaces and the crossings between them | `-format`, `-test`, `-config` |
| `declscope baseline [packages]` | [Record the violations a codebase already has](#adopting-on-an-existing-codebase), `shrink`'s included | `-config`, `-o`, `-shrink` |
| `declscope shrink [packages]` | [Unexport what no importer uses](#unexporting-what-no-importer-uses), inside `internal/` | `-fix`, `-config` |
| `declscope skill install` | Install the authoring and adoption skills for an AI agent | `--agent`, `--scope` |

`declscope <subcommand> -help` lists each one's flags.

## Configuration

Every setting is optional. The table below lists each setting and its default value. The YAML block shows all available keys; some values are examples.

```yaml
defaults:
  unexported: private     # package | private

rules:
  naming:
    qualify: never        # always | never | ondemand
    exported: false       # true | false
    vocabulary:
      mouse: [wheel]
  boundary: on           # off | on
  surplus: strict        # off | loose | strict
  unused: strict         # off | loose | strict

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
| `rules.boundary` | `off`, `on` | `on` | Whether the [`boundary`](#boundary) rule reports. See [reach without a boundary](#reach-without-a-boundary) |
| `rules.surplus` | `off`, `loose`, `strict` | `strict` | How much the [`surplus`](#surplus) rule reports. See [strict](#strict) |
| `rules.unused` | `off`, `loose`, `strict` | `strict` | How much the [`unused`](#unused-directives) rule reports. See [off, loose and strict](#off-loose-and-strict) |
| `filter.only` | Path globs | None | When set, no file outside them is read. Empty places no restriction |
| `filter.omit` | Path globs | None | Files taken back out, whether or not `only` let them through |
| `baseline` | A path relative to the config file | The nearest `.declscope-baseline.yaml` | The [baseline](#adopting-on-an-existing-codebase) to consult |

> [!TIP]
> For the recommended checks, put this in `.declscope.yaml`:
>
> ```yaml
> rules:
>   naming:
>     qualify: ondemand
>     exported: true
> ```
>
> `ondemand` checks names once a package has a second namespace. `exported: true` includes exported declarations. `surplus` and `unused` default to `strict`.

- The file is `.declscope.yaml` or `.declscope.yml`. An empty one changes nothing.
- [`-config`](#flags) names one file, and reads no other.
- An **unknown key is an error**, and so is a value a key does not accept. The message names what the section does take, so a typo never leaves a rule at its default in silence.

`defaults` takes only `unexported`, because an exported declaration has no scope to default.

### How two config files compose

Every `.declscope.yaml` between the analyzed package and the module root is read, **outermost first**. A nearer file does not replace the one above it. Each key composes on its own.

| Key | Down the chain |
| --- | --- |
| `defaults.*`, `rules.*` except `vocabulary`, `baseline` | The nearest file that states the key wins. A key no file states takes the built-in default |
| `rules.naming.vocabulary` | Merged per namespace. The nearer file wins the namespaces it states |
| `filter.only` | **Intersected.** A file is read when it matches every stating file's list |
| `filter.omit` | **Unioned.** A file matching any level's list is not read |

> [!IMPORTANT]
> **A config file can only ever shrink what is read.** An `omit` written at the root holds everywhere below it, and no nested file can undo it. `filter` has no negation, so nothing like `.gitignore`'s `!` can put a path back.

### The filter rule

`filter` decides which files are read at all. A file matches a list when it matches **any** pattern in it.

| Written | Read |
| --- | --- |
| Neither | Every file |
| `only` alone | Nothing outside the patterns |
| `omit` alone | Everything except the patterns |
| Both | What `only` admits, minus what `omit` names |

An empty `only` places no restriction, which is why a repository with no config is read whole.

A pattern is read against the directory of **the config file that states it**. Anchoring follows the rules of `.gitignore`.

| Pattern | Matches |
| --- | --- |
| `gen.go` | A file of that name at any depth |
| `/gen.go` | The one beside this config file |
| `gen/**` | That directory beside this config file, and no other |
| `**/gen/**` | That directory at any depth |
| `*`, `?` | Within one path segment |

One report comes from the configuration rather than from the code. It fires when a file's `only` matches files, but an `only` above it removes them all, so the package is read as empty.

```console
sub/a.go:1:1: filter.only stated in /repo/sub matches 1 file(s) here, but an only above it removes them all, so this package is read as empty
```

The report is that narrow on purpose. A package that reads nothing is usually the point, as with a root `only` naming one subtree, or an `omit` naming a directory.

## Directives

A **directive** is a line comment `//declscope:name`, with a lowercase name, no spaces, and any argument after a space. Any other comment starting with `declscope:` is [malformed](#malformed-directives). A trailing `// reason` is ignored.

```go
//declscope:package // shared with the reporting code
```

| Directive | Level | Effect |
| --- | --- | --- |
| `//declscope:package`, `//declscope:private` | Declaration or file | States the [scope](#scope-resolution). On a type it also reaches the type's [fields](#members), but not its methods |
| `//declscope:ignore` | Declaration or file | Silences every rule the analyzer reports. [`overexported`](#silencing-it) must be named |
| `//declscope:ignore <rules>` | Declaration or file | Silences the named [rules](#rules), comma-separated |
| `//declscope:core` | File | Joins the file to the [core namespace](#the-core-namespace) |
| `//declscope:namespace <name>` | File | Joins the file to a [shared namespace](#namespaces) |

### Placement

A declaration-level directive goes in the doc comment, directly above the declaration. A file-level directive goes above the package clause.

```go
//declscope:package

package database

//declscope:package
func userSeed() {}
```

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

<details>
<summary>Where a file-level directive may sit</summary>

`//declscope:namespace` must come before the package clause. Three placements are accepted there.

<table>
<thead>
<tr><th>Placement</th><th>Example</th></tr>
</thead>
<tbody>
<tr>
<td>A blank line before <code>package</code>, as this README writes</td>
<td>

```go
//declscope:namespace billing

package app
```

</td>
</tr>
<tr>
<td>Directly above <code>package</code></td>
<td>

```go
//declscope:namespace billing
package app
```

</td>
</tr>
<tr>
<td>At the bottom of the package doc comment, after a blank <code>//</code> line: Go's own convention for a directive in a doc comment</td>
<td>

```go
// Package app bills users.
//
//declscope:namespace billing
package app
```

</td>
</tr>
</tbody>
</table>

Go excludes a `//tool:name` comment from a doc comment, so none of them reaches the rendered documentation.

</details>

### Ignore levels

- Every level is consulted. An ignore counts as used whenever it covers a rule that would have fired, so overlapping ignores never make one another look unused.
- Ignores are consulted **before** the baseline. A suppression the baseline would also have absorbed still counts as used.

### Unused directives

The `unused` rule reports a directive that changes nothing. A directive that declscope cannot read belongs to the [`directive`](#malformed-directives) rule instead.

#### Off, loose and strict

| `rules.unused` | `//declscope:ignore` is reported when | `//declscope:package` or `//declscope:private` is reported when | `-fix` |
| --- | --- | --- | --- |
| `off` | Never | Never | None |
| `loose` | It silenced no report | Deleting it would change no declaration's scope under any config | None |
| `strict` *(default)* | It silenced no report | Deleting it would change no declaration's scope under the current config | Deletes a redundant scope directive, unless [withheld](#the-strict-fix) |

- One comment gets one report, however many declarations it reaches. A block's directive is reported once.
- Without the directive, a declaration falls to the next row of [scope resolution](#scope-resolution). Only `defaults.unexported` there depends on the config.
- Under `strict`, a directive that `loose` keeps also stays when deleting it would make a nearer directive redundant.
- `off` still leaves the `directive` rule on.
- An ignore of `boundary` is not reported where a file the build leaves out names a declaration it reaches. That file may cross into it under another build tag, where the ignore silences the crossing. The run that reads the file still reports an ignore with nothing to silence.

> [!WARNING]
> Under `strict`, changing `defaults.unexported` changes the reports. With `unexported: package`, every `//declscope:package` on an unexported declaration that nothing else widens is reported, and one `-fix` run deletes them. `loose` never reports a directive only because it names the current default.

<details>
<summary>Example: one file under each mode</summary>

```go
package app

type user struct {
	//declscope:private
	name string
}

//declscope:package
func Helper() {}

//declscope:ignore boundary
func greet(u user) string { return u.name }
```

| Line | `off` | `loose` | `strict` |
| --- | --- | --- | --- |
| 4 | Nothing | Nothing | `unused //declscope:private on user.name: it already has private scope` |
| 8 | Nothing | `unused //declscope:package on Helper: nothing it reaches takes a scope` | `unused //declscope:package on Helper: it already has package scope` |
| 11 | Nothing | `unused //declscope:ignore boundary on greet` | `unused //declscope:ignore boundary on greet` |

Line 4 is where the modes differ. Deleting line 4 leaves `user.name` private only because `defaults.unexported` is private, which `loose` does not rely on.

</details>

#### Report messages

Every unused report names the directive and says why it is unused. `loose` and `strict` word the reason differently.

#### The strict fix

`-fix` deletes a directive that `strict` reports, unless deleting it would change another report. Then the report stays, with no fix.

<details>
<summary>Example: one directive deleted, one withheld</summary>

```go
// user.go
package app

//declscope:private
func normalize() {}

//declscope:private
func trim() {}
```

```go
// order.go
package app

var _ = normalize
```

```console
$ declscope ./...    # with rules.unused: strict
user.go:5:6: func normalize is declared private by //declscope:private, but is used from namespace "order"
order.go:4:9: 	used here, in namespace "order"
user.go:4:1: unused //declscope:private on normalize: it already has private scope
user.go:7:1: unused //declscope:private on trim: it already has private scope
```

`-fix` changes `user.go`. The `boundary` report names `normalize`'s directive, so that one stays.

```diff
 //declscope:private
 func normalize() {}
 
-//declscope:private
 func trim() {}
```

</details>

#### Silencing an unused report

| Ignore | Silences the unused report of |
| --- | --- |
| `//declscope:ignore unused` on a declaration | Every other directive on that declaration |
| Bare `//declscope:ignore` on a declaration | The scope directive on that declaration. Not the other ignores there, so a group of ignores cannot excuse one another |
| `//declscope:ignore unused`, or a bare one, above the package clause | Every other directive in the file |
| Any ignore on a type or a block | Nothing inside it. For other rules it reaches inside, but here one ignore would hide every stale directive under it |

**No ignore silences its own unused report.**

<details>
<summary>Example</summary>

```go
package app

//declscope:package
//declscope:ignore unused
func A() {}

//declscope:package
//declscope:ignore directive
func B() {}

//declscope:ignore unused
type T struct {
	//declscope:package
	F int
}

//declscope:ignore qualify
//declscope:ignore unused
func C() {}

//declscope:ignore unused
func D() {}
```

```console
$ declscope ./...
user.go:7:1: unused //declscope:package on B: nothing it reaches takes a scope
user.go:13:2: unused //declscope:package on T.F: nothing it reaches takes a scope
user.go:8:1: unused //declscope:ignore directive on B
user.go:11:1: unused //declscope:ignore unused on T
user.go:21:1: unused //declscope:ignore unused on D
```

| Declaration | Result | Why |
| --- | --- | --- |
| `A` | Quiet | `ignore unused` silences the scope directive's report |
| `B` | Both reported | `ignore directive` names another rule, so it silenced nothing |
| `T` | Both reported | The type's ignore does not reach the field |
| `C` | Quiet | `ignore unused` silences the report on `ignore qualify` |
| `D` | Reported | Its only candidate is its own report |

</details>

### Malformed directives

Only `//declscope:name` is a directive. Any other comment starting with `declscope:`, and an unknown, conflicting or misplaced directive, is reported by the `directive` rule and has no effect.

<details>
<summary>Every malformed-directive report</summary>

The rule is always on, and has no configuration key. `//declscope:ignore directive` silences it, on the declaration or above the package clause.

| Directive | Report |
| --- | --- |
| `//declscope:foo` | `unknown directive declscope:foo` |
| `// declscope:package`, `/*declscope:package*/`, or any other comment starting with `declscope:` that is not `//declscope:name` | `malformed declscope directive: write it as //declscope:name` |
| `//declscope:package x` | `//declscope:package takes no argument` |
| `//declscope:private` and `//declscope:package` together | `conflicting scope directives: ...` |
| `//declscope:core` and `//declscope:namespace` together | `conflicting namespace directives: a core file's namespace is the core` |
| `//declscope:ignore foo` | `unknown rule "foo" in declscope:ignore (want one of boundary, qualify, surplus, unused, directive, filter, overexported)` |
| `//declscope:ignore ,` or `//declscope:ignore boundary,` | `empty rule name in declscope:ignore` |
| `//declscope:namespace` after the package clause | `declscope:namespace must appear before the package clause` |
| A directive attached to no declaration, such as one inside a function body | `misplaced declscope:package: no declaration here for it to bind to; ...` |

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

The namespace comes *from* the file name and is not equal to it. So you can rename a file without renaming what it declares.

Files join a **shared** namespace with a directive before the package clause. This is how one unit spans several files. The name must be an unexported identifier.

```go
//declscope:namespace user
package repo
```

### The core namespace

One namespace in a package may be its **core**: the unit the package is named for. Several files may carry `//declscope:core`, and they share the one core namespace.

```go
// client.go
//declscope:core

package transport
```

The core has no prefix, so [the naming rule](#the-naming-rule) asks nothing of its declarations. No prefix still names exactly one unit.

> [!IMPORTANT]
> `//declscope:core` sets the namespace and nothing else. It carries no scope. A core declaration still takes [`defaults.unexported`](#configuration): by default it is private *to the core*, and a file outside the core that names it crosses a boundary.
>
> | Written on a file | Effect |
> | --- | --- |
> | `//declscope:core` | The file joins the core. What it declares keeps the default scope |
> | `//declscope:core` and `//declscope:package` | Both apply. The file is core, and what it declares is package-wide |
> | `//declscope:core` and `//declscope:namespace` | Refused. A core file's namespace *is* the core |

## Scopes

The **subject** of the analysis is a package's unexported surface: its unexported package-level declarations, and the unexported [members](#members) of any type.

A **scope** is how far a declaration may be used. There are two.

| Scope | Meaning | Rust equivalent |
| --- | --- | --- |
| `package` | Usable anywhere in the package | `pub(super)` |
| `private` | Usable only inside its own [namespace](#namespaces) | No modifier |

There is no `public`. Go already spells that with a capital letter, and declscope never sees a use beyond the package edge.

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

An unexported declaration is `package` only where something says so: a directive, or the `defaults` key. Its *name* plays no part. A prefix marks ownership and grants nothing.

> [!IMPORTANT]
> Exportedness decides the default, and nothing else. An **exported field of an unexported type** still resolves to `package`, so making the type unexported protects nothing.
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

### Members

A **member** is a name written *inside* a type's declaration: a struct's **field**, or an interface's **method name**. It is written inside its type, so its namespace is the type's file's, and the type's directive reaches it.

A **method with a receiver** is not a member. It is an ordinary top-level declaration, and its own file gives it its namespace.

| | Package-level declaration | Member | Method with a receiver |
| --- | --- | --- | --- |
| Bounding namespace | Its file's | Its **type**'s file's | Its file's |
| Contained by | Its `var`, `const` or `type` block | Its **type** | Nothing |
| [Naming rule](#the-naming-rule) | Applies | Does not apply | Only when filed away from its type |

The naming rule skips members because a member is never read on its own. Every use writes its value first, so `u.name` hands the reader `u` to follow. A method is read the same way, through its receiver.

What Go lacks for a member is encapsulation. Every unexported field is visible to the whole package. The `private` scope supplies it.

<details>
<summary>A type's methods filed in another file</summary>

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

The naming rule asks the method to carry `query`. A method is read through its receiver, and `s.Where` points the reader at `Statement`'s unit, which does not hold it.

```console
query.go:3:21: method Where does not carry namespace "query" anywhere in its name; rename it to QueryWhere, or to another name that carries "query"
```

Neither report asks for a rename here. The two files are one unit split in two, and `//declscope:namespace statement` on `query.go` settles both.

</details>

> [!TIP]
> An unexported interface method is the **sealed interface** idiom. Only this package can spell the name, so only this package can implement the interface. declscope gives it file granularity.
>
> Satisfying an interface is **not** a use of the name, because a method set is resolved, not written. Naming the method does cross.

> [!WARNING]
> Operations on the **whole value** name no field: copying it, comparing it, zeroing it. They are outside what declscope can see. See [Limits](#limits).

## Rules

A **rule** is one check. A rule's name is the diagnostic's category, its [baseline](#adopting-on-an-existing-codebase) key, and what [`//declscope:ignore`](#directives) targets.

| Rule | Reports | Fix | Configured by | Default |
| --- | --- | --- | --- | --- |
| [`boundary`](#boundary) | A declaration used from outside the namespace it is private to | Insert `//declscope:package` | `rules.boundary` | `on` |
| [`qualify`](#the-naming-rule) | A name that does not carry its namespace | Rename to prefix it | `rules.naming.*` | Off |
| [`surplus`](#surplus) | Package scope with no visible use from another namespace | Under `strict`, insert `//declscope:private` | `rules.surplus` | `strict` |
| [`unused`](#unused-directives) | An ignore that silenced nothing, or a scope directive that changes no scope | Under `strict`, delete a redundant scope directive | `rules.unused` | `strict` |
| [`directive`](#malformed-directives) | A directive that is malformed, unknown, conflicting or misplaced | None | No | On |
| [`filter`](#the-filter-rule) | A `filter.only` that an `only` above it cancels | None | No | On |
| [`overexported`](#unexporting-what-no-importer-uses) | An exported declaration of an `internal/` package that nothing outside its package uses | Unexport it | Running `declscope shrink` | Not run by the analyzer |

Every diagnostic carries **at most one** fix, so `-fix` never has to choose.

### `boundary`

`boundary` reports a `private` declaration used from outside its namespace. For a [member](#members), the boundary is the namespace of its type. The [example above](#what-it-reports) is this rule.

The message names where the scope came from.

| Where the scope came from | Message |
| --- | --- |
| `defaults` | `func normalizeEmail is private to namespace "userRepository", but is used from namespace "orderRepository"` |
| The declaration's own directive | `func normalizeEmail is declared private by //declscope:private, but is used from namespace "orderRepository"` |
| Its type's directive | `field Statement.wheres is declared private by //declscope:private on Statement, but is used from namespace "query"` |
| A file-level directive | `func normalizeEmail is declared private by the file's //declscope:private, but is used from namespace "orderRepository"` |

Every crossing use site is attached to the diagnostic, and a use from inside the declaration's own namespace is not. The report lands on the **declaration**, not on the use.

| Case | Fix |
| --- | --- |
| The declaration states its **own** scope | None. The directive and the use are both deliberate, and `-fix` must not overwrite what the author wrote |
| The `private` is inherited from a type or a file | Offered. The inserted directive sits on the declaration, which outranks both |
| A type and its members cross together | One directive, on the type. The members are reported without a fix, since a second directive on them would bind nothing |

#### Reach without a boundary

`rules.boundary: off` switches this rule off. What is left is the naming rule, for a repository that wants the ownership mark in a name without the scope behind it.

```yaml
rules:
  naming:
    qualify: ondemand
  boundary: off
  surplus: off
```

Set `surplus: off` alongside it. `surplus` audits `//declscope:package`, which means nothing once reach is not checked.

> [!TIP]
> This is not how to adopt declscope gradually. A [baseline](#adopting-on-an-existing-codebase) records what a codebase already has and still reports what is new. A switch reports nothing, so you never learn what turning it on later would cost.

### The naming rule

**Off by default.** Turn it on with `rules.naming.qualify`.

The rule asks a package-level declaration to carry its file's namespace somewhere in its name. The name then says which unit owns it, at the call site and in a stack trace.

```go
// order_repository.go
func scanOrder(rows *sql.Rows) (domain.Order, error) {
	// ...
	o.BuyerEmail = normalizeEmail(email) // the email unit's, and shared
}
```

The mark grants nothing. Reach is stated by [scope](#scope-resolution) alone.

> [!NOTE]
> The rule is opt-in because whether a prefix reads well depends on the file name, and no tool can see that. A prefix from `comments.go` reads as a noun phrase, `commentsAttached`. A prefix from `collect.go` reads as a command, `collectAddFunc`.

#### When it applies

| `rules.naming.qualify` | Effect |
| --- | --- |
| `never` *(default)* | Off |
| `ondemand` | Required once the package has a **second namespace**. A file holding only a package clause and comments, as a `doc.go` usually does, is not counted |
| `always` | Required in every package, so that gaining a second namespace is not a mass rename |

<details>
<summary>Declarations the rule never reaches</summary>

[Members](#members) and [the core namespace](#the-core-namespace) are explained in their own sections.

<table>
<thead>
<tr><th>Declaration</th><th>Example</th><th>Reason</th></tr>
</thead>
<tbody>
<tr>
<td>A member, or a method written beside its type</td>
<td>

```go
// user.go
type user struct{ name string }

func (u user) greet() string { return u.name }
```

</td>
<td>Already qualified by that type at every use</td>
</tr>
<tr>
<td><code>func main</code> in package <code>main</code></td>
<td>

```go
// cmd/app/run.go
package main

func main() {}
```

</td>
<td>A name the toolchain requires</td>
</tr>
<tr>
<td>A test function in a <code>_test.go</code> file</td>
<td>

```go
// user_test.go
func TestGreet(t *testing.T) {}
func BenchmarkGreet(b *testing.B) {}
func ExampleGreet() {}
```

</td>
<td>The same. <code>go test</code> finds <code>TestXxx</code>, <code>BenchmarkXxx</code>, <code>FuzzXxx</code> and <code>ExampleXxx</code> by name</td>
</tr>
<tr>
<td>A declaration in a namespace that cannot start an identifier</td>
<td>

```go
// 2fa.go
func verify() bool { return true }
```

</td>
<td>The fix prefixes, and no identifier begins with a digit</td>
</tr>
<tr>
<td>An exported identifier, unless <code>rules.naming.exported</code> is on</td>
<td>

```go
// user.go
func Greet() {}
```

</td>
<td>How the API is spelled is the author's decision</td>
</tr>
<tr>
<td>A declaration in the core namespace</td>
<td>

```go
//declscope:core

package app

func open() {}
```

</td>
<td>The core has no prefix</td>
</tr>
</tbody>
</table>

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

Two English inflections change the namespace's own spelling. Both are accepted.

| Namespace | Also carried by |
| --- | --- |
| `store` | `storing`. The final `e` drops before `ing` |
| `apply` | `applies`, `applied`. The final `y` turns to `i` |

Only these whole forms are generated, from the namespace's side. The name is never stemmed, so `story` and `storm` do not carry `store`.

`rules.naming.vocabulary` lists extra words that carry a namespace. A listed word goes through the same test, so `wheelDelta` carries `mouse` and `pinwheel` does not.

```yaml
rules:
  naming:
    vocabulary:
      mouse: [wheel]
      index: [indices]
```

> [!WARNING]
> The vocabulary is for the irregular few. A namespace that needs a long list is a sign that the file declares things it is not about. Splitting the file says more than listing them.

#### The rename

The fix prefixes, keeping Go's spelling of an initialism and the original exportedness.

| Case | Example | Never |
| --- | --- | --- |
| Unexported | `id` → `userID` | `userId` |
| Exported, under `rules.naming.exported` | `Load` → `UserLoad` | `userLoad` |

The suggestion is one answer, not the only one. Any spelling that carries the namespace settles the rule. A rename never changes what a name is visible to.

> [!NOTE]
> Sometimes the namespace is what is wrong, not the name. A file name may hold words that name no unit.
>
> ```console
> user_repository.go:18:6: func scanUser does not carry namespace "userRepository" anywhere in its name; rename it to userRepositoryScanUser, or to another name that carries "userRepository"
> ```
>
> The unit here is `user`, not `userRepository`. `//declscope:namespace user` on the file settles the rule and leaves every name alone.

#### Withheld renames

A rename is offered only when it provably changes nothing but the spelling. The violation is reported either way. A doubt withholds the fix, never the diagnostic.

> [!CAUTION]
> Go resolves a name from the inside out, so a new name that is free at package level can still be bound at a use site. The wrong rename **compiles and computes something else**.
>
> ```go
> var count = 10
> func Add(fooCount int) int { return fooCount + count } // Add(1) == 11
> ```
> ```go
> // after a careless rename of count to fooCount
> func Add(fooCount int) int { return fooCount + fooCount } // Add(1) == 2
> ```

### `surplus`

`surplus` is the converse of `boundary`. It reports package scope that no visible use from another namespace needs.

| `rules.surplus` | Reports | Fix |
| --- | --- | --- |
| `off` | Nothing | |
| `loose` | A `//declscope:package` that nothing it reaches needs | None |
| `strict` *(default)* | What `loose` reports, plus each declaration a directive in use widens for nothing | Insert `//declscope:private` |

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

One comment gets one report, however many declarations take their scope from it. A declaration that states its own scope does not depend on the comment, so it neither keeps it alive nor appears under it.

> [!IMPORTANT]
> The rule concludes from an **absence**. A directive can hold up something declscope cannot see, so the rule stays quiet on any doubt. For the same reason `loose` offers no fix: its advice is to delete a directive.

#### `strict`

Under `loose`, one used declaration keeps its whole directive quiet. The rest of what the directive reaches may still be wider than it needs. `strict` reports each of those, and `-fix` narrows it.

```go
// account.go
package bank

//declscope:package
type account struct {
	id      int
	balance int
}

func accountDeposit(a *account, n int) { a.balance += n }
```

```go
// ledger.go
package bank

func ledgerKey(a account) int { return a.id }
```

```console
$ declscope ./...
account.go:7:2: field account.balance takes package scope from //declscope:package on account, but no use from another namespace is visible to declscope
```

`-fix` changes `account.go`:

```diff
 //declscope:package
 type account struct {
-	id      int
+	id int
+	//declscope:private
 	balance int
 }
```

Every enclosing directive is judged the same way.

| The directive is on | Judged one by one |
| --- | --- |
| A struct or interface type | Each field, or each method name |
| A `var`, `const` or `type` block | Each spec |
| The file | Each declaration in the file, and each member of a type that states no scope |

`strict` is the default. `loose` checks only directives as a whole; select it explicitly if that is the policy your repository wants.

> [!TIP]
> Convention puts private fields last, after the fields other namespaces read. The fix never reorders fields: order is observable through unkeyed composite literals, positional encodings, `unsafe` offsets and 64-bit atomic alignment. Move them yourself where none of those apply.

## Unexporting what no importer uses

`declscope shrink` finds the exported declarations of `internal/` packages that nothing outside their package uses. With `-fix` it unexports them.

**Why it matters.** An exported declaration takes package scope by default, so the analyzer never reports a boundary on it. An exported name that nothing outside needs hides a declaration from every check here. Unexported, it takes `private`, and the analyzer checks who reaches it.

**Why a subcommand.** The analyzer reads one package, and any importer might use an exported name. Inside `internal/`, Go limits the importers to one directory tree. `shrink` loads the whole module, so it sees every one of them. In a workspace, it loads every module of the workspace. A nested module whose path lies under an `internal/` parent can import the package too, so `shrink` loads it as well, and counts its uses. `go vet` and golangci-lint never run it.

```go
// internal/user/user.go
package user

import "fmt"

func Load(id int) string { return Format(id) }

// Format renders an ID for display.
func Format(id int) string { return fmt.Sprint(id) }

type Record struct {
	ID   int
	Name string
}

func Dump(r Record) { fmt.Println(r) }
```

`api/api.go` calls `user.Load` and names `user.Dump`. Nothing outside `user` names `Format` or `Record`:

```console
$ declscope shrink
internal/user/user.go:8:6: func Format is exported, but nothing outside example.com/app/internal/user uses it
```

`declscope shrink -fix` renames `Format` to `format`, and the doc comment that opens with it. `Record` is not reported: `fmt.Println` reads its fields through reflection, which is a use.

### Run it before the analyzer

**Apply `shrink -fix` first, then work on the analyzer's reports.** A declaration it unexports becomes private to its namespace. Wherever another file uses it, the analyzer now reports a crossing, which `declscope -fix` or a move settles. The other order takes a second round.

```console
$ declscope shrink -fix ./...
$ declscope ./...
```

Run them in the same order in CI. `shrink` exits 3 when it reports anything, as the analyzer does. With `-fix`, only the reports left without a fix count.

### Patterns

The patterns only choose which packages are reported. To find uses, `shrink` always reads every package of the module, whatever the patterns name.

For example, this reports only on `internal/user`:

```console
$ declscope shrink ./internal/user/...
internal/user/user.go:8:6: func Format is exported, but nothing outside example.com/app/internal/user uses it
```

`Load` is not reported, although `api/` is outside the pattern. `api/api.go` calls `user.Load`, and that call is a use.

Otherwise, a pattern behaves as it does for `go vet`.

<details>
<summary>What counts as a use, and when the fix is withheld</summary>

A fix is offered only where no use can exist outside the package. Where a use may exist but cannot be proved, the report stays and says why no fix is offered.

<table>
<thead>
<tr><th>Code</th><th>Result</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
// internal/user/user.go
type ID int

func (ID) String() string { return "" }

func New() ID { return 0 }

// api/api.go
var _ fmt.Stringer = user.New()
```

</td>
<td>Not reported. <code>String</code> satisfies <code>fmt.Stringer</code></td>
</tr>
<tr>
<td>

```go
// internal/user/user.go
type Record struct {
	ID   int
	Name string
}

func Dump(r Record) { fmt.Println(r) }
```

</td>
<td>Not reported. <code>fmt</code> reads the fields through reflection</td>
</tr>
<tr>
<td>

```go
// internal/user/user.go
type Record struct {
	Name string `json:"name"`
}
```

</td>
<td>Not reported. A tag says a marshaller reads the struct</td>
</tr>
<tr>
<td>

```go
// api/api_windows.go, excluded from this build
var _ = user.Format
```

</td>
<td>Not reported. A build-excluded file that writes <code>user.Format</code> uses it</td>
</tr>
<tr>
<td>

```go
// internal/user/user_test.go
package user_test

func TestFormat(t *testing.T) { _ = user.Format(1) }
```

</td>
<td>Reported: <code>(no fix: external tests use it)</code></td>
</tr>
<tr>
<td>

```go
// api/api_windows.go, excluded from this build
var _ io.Closer = user.Open()
```

</td>
<td>Reported for <code>Conn.Close</code>: <code>(no fix: a build-excluded file of another package may use it)</code>. The file is not type-checked, and a method satisfies an interface without its name</td>
</tr>
<tr>
<td>

```go
// internal/user/user.go
func Format(id int) string { return format(id) }

func format(id int) string { return "" }
```

</td>
<td>Reported: <code>(no fix: the unexported name is taken or would be captured)</code></td>
</tr>
<tr>
<td>

```go
// internal/user/user.go
var Version = "dev"
```

</td>
<td>Reported: <code>(no fix: a string variable may be set by -ldflags -X, which names it)</code></td>
</tr>
</tbody>
</table>

Deleting unused code is out of scope. Once a declaration is unexported, staticcheck's `unused` and gopls' `unusedfunc` report it if nothing uses it.

</details>

<details>
<summary>Which packages it does not judge</summary>

`shrink` stands down wherever an importer could be unseen. Each `internal/` package it skips is named on stderr with the reason, and the exit status ignores it.

<table>
<thead>
<tr><th>Code</th><th>Why</th><th><code>shrink</code> prints on stderr</th></tr>
</thead>
<tbody>
<tr>
<td>

```go
// api/api.go
package api
```

</td>
<td>Another module may import it</td>
<td>Nothing. Only <code>internal/</code> packages are listed</td>
</tr>
<tr>
<td>

```go
// internal/tool/main.go
package main
```

</td>
<td><code>-buildmode=plugin</code> looks its exported symbols up by name</td>
<td><code>not judged: example.com/app/internal/tool: package main</code></td>
</tr>
<tr>
<td>

```go
// internal/c/c.go
// int one() { return 1; }
import "C"
```

</td>
<td>cgo names Go symbols where <code>go/types</code> does not look</td>
<td><code>not judged: example.com/app/internal/c: the package uses cgo</code></td>
</tr>
<tr>
<td>

```text
internal/fast/fast.go
internal/fast/fast_arm64.s
```

</td>
<td>So does assembly, for any architecture</td>
<td><code>not judged: example.com/app/internal/fast: the package holds non-Go files</code></td>
</tr>
<tr>
<td>

```text
// tools/go.mod
module example.com/app/tools

require example.com/app v0.0.0

replace example.com/app => ../../elsewhere
```

</td>
<td>A nested module may import the package, and this one reads it from another directory, so its uses are of other source. A published version, or a module that fails to load, is the same</td>
<td><code>not judged: example.com/app/internal/user: the nested module example.com/app/tools may import it, and it reads example.com/app/internal/user from /path/to/elsewhere</code></td>
</tr>
<tr>
<td>

```console
$ declscope shrink ./internal/user/testdata/fixture
```

</td>
<td><code>./...</code> leaves it out, so this run has read none of its uses</td>
<td><code>not judged: example.com/app/internal/user/testdata/fixture: ./... leaves it out, so this run does not load it</code></td>
</tr>
</tbody>
</table>

An interface's method names, and the test functions of a `_test.go` file, are never reported either. Every implementation would rename too, and `go test` finds a test by name.

A package that does not type-check refuses the whole run, since it would show no uses at all.

</details>

> [!IMPORTANT]
> Two uses are outside what `shrink` can see. Silence either with the ignore below.
>
> - A name written as a string that no interface leads to, such as a template field or a constant given to `reflect.Value.MethodByName`.
> - A module outside this module's directory tree whose path extends an `internal/` parent, such as a `/v2` on another branch. Go checks `internal/` by import path.

### Silencing it

Write `//declscope:ignore overexported // <why>` where any other ignore goes: on the declaration, on a field's type, or before the package clause. A bare `//declscope:ignore` does not reach this rule.

`shrink` reports an ignore of it that silenced nothing. `//declscope:ignore unused` answers that report, as it does for the analyzer.

One ignore passes every build configuration. Where a field, a method, or a type a struct embeds is used outside its package only by files behind a build constraint, its ignore is not reported unused, though it silences nothing there:

```go
// internal/a/a.go
type Mode struct {
	//declscope:ignore overexported // app/gui.go sets it behind the gui tag
	Flag string
}
```

```go
// app/gui.go
//go:build gui

var _ = a.Mode{Flag: "gui"}
```

| Run | What happens to `Flag` |
| --- | --- |
| `declscope shrink ./...` | `app/gui.go` is left out and read for names only, so `Flag` would be reported with no fix. The ignore silences it |
| `GOFLAGS=-tags=gui declscope shrink ./...` | `app/gui.go` uses `Flag`. The ignore is kept for the run without the tag |

## Measuring what is there

Two subcommands report what the analyzer found. Neither decides anything. The exit status is zero whatever the counts say, since gating is what the analyzer and a baseline are for.

| Command | Unit | Answers |
| --- | --- | --- |
| `declscope survey [packages]` | package | **Which package do I open first?** |
| `declscope inspect <package>` | namespace, crossing | **What shape is this package in?** |

```console
$ declscope survey -config .declscope-strict.yaml ./internal/measure/...
## Checks in force

| check      | value                                                                  | packages |
| ---------- | ---------------------------------------------------------------------- | -------: |
| config     | `.declscope-strict.yaml`                                               |        1 |
| rules      | boundary on, qualify ondemand, exported, surplus strict, unused strict |        1 |
| type check | 1 package ok, 0 failed                                                 |          |

## Findings

| rule        | found | ignored | baselined | reported |
| ----------- | ----: | ------: | --------: | -------: |
| `boundary`  |     0 |       0 |         0 |        0 |
| `qualify`   |     0 |       0 |         0 |        0 |
| `surplus`   |     0 |       0 |         0 |        0 |
| `unused`    |     0 |       0 |         - |        0 |
| `directive` |     0 |       0 |         - |        0 |
| `filter`    |     0 |       0 |         - |        0 |

## Packages — boundary

| package                                      | reported | baselined | declared | largest crossing                 |
| -------------------------------------------- | -------: | --------: | -------: | -------------------------------- |
| `github.com/mpyw/declscope/internal/measure` |        0 |         0 |       20 | markdown → cell (7 declarations) |
```

That is one package of this repository. A codebase adopting declscope has numbers in the first two columns, and a row per package.

- **The state of the checks comes first**, because a count means nothing without it. A zero from a rule that was off, a package that did not compile, or a baseline that absorbed everything looks like a zero from clean code.
- A rule prints `-` rather than `0` where it was not asked. It may be switched off, or it may stand itself down, as `surplus` does for a package with a file it cannot read.
- A package that does not type-check stops the run. `-allow-errors` continues and names it under `type check`.
- So does a run whose patterns match no package. A pattern that matches nothing beside others that do gets a warning. `inspect` does the same.
- Rows are ordered by how much is outstanding: undecided, reported and baselined together. `largest crossing` names where it is concentrated.

> [!TIP]
> Look for the row with nothing reported, much baselined and nothing declared. Nothing was decided there, everything was deferred, and the analyzer alone calls it clean.

```console
$ declscope inspect -config .declscope-strict.yaml ./internal/measure
## Crossings

| crossing          | mutual | declared | baselined | reported | clears |  reached | uses |
| ----------------- | -----: | -------: | --------: | -------: | -----: | -------: | ---: |
| markdown → cell   |        |        7 |         0 |        0 |      0 |  7 of 10 |   15 |
| markdown → sink   |        |        5 |         0 |        0 |      0 |  5 of 17 |   28 |
| golden → (core)   |        |        4 |         0 |        0 |      0 |  4 of 78 |    9 |
| format → json     |        |        2 |         0 |        0 |      0 | 2 of 105 |    2 |

114 declarations are further: open, package-scoped by default rather than by decision, so left out of the table and of the diagram.

Declarations crossed: 0 reported, 0 baselined, 20 declared.
```

One row per directed edge, so a mutual pair is two rows. **`clears` is the number the decision turns on**: how many findings would go away if the two namespaces became one.

Both commands take `-test=false`. On a large package it changes the picture. In `net/http` the heaviest crossing is `export_test.go` reaching the transport internals, which is what that file is for.

<details>
<summary>The other columns, and the formats</summary>

| Column | What it says |
| --- | --- |
| `clears` | Findings that would go away if these two namespaces merged. A declaration a third namespace also reaches survives the merge, so a heavy crossing can clear much less than it reaches |
| `reached` | Declarations of the reached namespace this edge touches, over every declaration it holds. `12 of 19` says the second namespace holds the working parts of the first. Open crossings are counted under the table, not in the rows |
| `declared` / `baselined` / `reported` | Declarations on this edge, by what became of each. A declaration reached from two namespaces is two rows and one finding, so these sum to more than the rule found. The line under the table gives the rule's own count, which is the survey's |
| `saturation`, in the `Qualify` table | How much of a namespace the naming rule is unsatisfied by. Near the top, the namespace name is usually what is wrong |

| `-format` | For |
| --- | --- |
| `markdown` *(default)* | A terminal **and** an issue or pull request. Cells are padded, so the output aligns in both. `inspect` adds a Mermaid diagram |
| `json` | An agent, and anything scripted |

The JSON carries four arrays.

| Array | Holds |
| --- | --- |
| `edges`, `names` | One flat row each |
| `crossings` | The fold the crossing table prints, `clears` included |
| `namespaces` | The denominators every ratio divides by |

`findings` carries the same `asked` flag the tables print a dash for. Markdown is a rendering of the same data.

</details>

## Adopting on an existing codebase

A **baseline** records the violations a codebase already has: the analyzer's, and what [`declscope shrink`](#unexporting-what-no-importer-uses) reports. Both then report only what is new.

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
    overexported:
      userRepository:
        - FindUser
```

| | Spelling |
| --- | --- |
| A package-level declaration | `normalizeEmail`, under its file's namespace |
| A [member](#members) | `Type.member`, under the namespace of its **type**'s file |
| The [core namespace](#the-core-namespace) | `(core)` |

An entry is keyed by package, rule, namespace and declaration, never by position. It survives code moving within its file. It does **not** survive a declaration moving to another namespace, since the same use then crosses a different boundary.

A baseline is regenerated, never edited.

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

> [!TIP]
> A regeneration records what one build configuration sees. A module that runs declscope under several `GOFLAGS=-tags=...` or `GOOS` settings keeps only the last run's entries.

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

For example, with a config in `internal/billing/` naming its own baseline, and a baseline already in `internal/legacy/`:

```text
.declscope-baseline.yaml               internal/user's entries
internal/
├── user/user.go
├── billing/
│   ├── .declscope.yaml                baseline: billing-baseline.yaml
│   ├── billing-baseline.yaml          internal/billing's entries
│   └── billing.go
└── legacy/
    ├── .declscope-baseline.yaml       internal/legacy's entries
    └── legacy.go
```

```console
$ declscope baseline ./...
declscope: recorded 1 violation(s) in .declscope-baseline.yaml
declscope: recorded 1 violation(s) in internal/billing/billing-baseline.yaml
declscope: recorded 1 violation(s) in internal/legacy/.declscope-baseline.yaml
```

`-o` bypasses that lookup and gathers every entry into one file.

Recording `shrink`'s reports loads the whole module once more. `-shrink=false` skips it, for a module that does not run `shrink`.

If no pattern matches a package, the run stops and writes nothing, `-o` included. A pattern that matches nothing beside others that do gets a warning, as it does from `go vet`.

Every file written is regenerated **wholesale**. The existing one is never read, so a baseline that fails to parse is replaced like any other.

> [!WARNING]
> Some packages cannot reach the working directory by that lookup, such as one in another module. The run then **refuses** and names them, rather than recording entries nothing would find.

</details>

> [!TIP]
> [`skills/declscope-adoption`](skills/declscope-adoption/SKILL.md) is a skill for an AI agent doing this work. It covers what each diagnostic shape means, and the measurement traps that produce false confidence. Its companion, [`skills/declscope-authoring`](skills/declscope-authoring/SKILL.md), is for everyday code: where a declaration goes, and how to name it accurately. The binary carries both:
>
> ```console
> declscope skill install            # the agents already set up in this project
> declscope skill install --agent claude-code --scope user
> declscope skill list               # where they are, and whether they are current
> ```
>
> Without the binary, `gh skill install mpyw/declscope declscope-authoring --agent claude-code` writes to the same directories, and the same with `declscope-adoption`. The binary's installer is [`go-skill-embed`](https://github.com/mpyw/go-skill-embed), which takes them from `gh skill install`.

## Using it with an AI agent

declscope runs wherever an agent's edits are checked.

```bash
declscope ./...        # in CI, and in the agent's build loop
declscope -fix ./...   # deterministic: at most one fix per diagnostic
```

On an existing codebase, run `declscope baseline ./...` once first. The agent is then shown only the boundaries its own edits cross.

`declscope skill install` installs both skills. `declscope-authoring` says where a new declaration belongs, and how to name one without bending the name to pass the naming rule.

| Property | Effect on the agent |
| --- | --- |
| The diagnostic names the namespace crossed | The agent is told why the use is wrong, and the repair is mechanical |
| A directive is a durable record of intent | The next agent inherits the decision instead of re-deriving it |

For the work of introducing it, give the agent `-format=json` rather than the diagnostics.

```bash
declscope survey -format=json ./...             # what is in force, and which package to open
declscope inspect -format=json ./internal/cmd   # its namespaces, crossings, edges and names
```

Rank by `crossings[].clears`, and pass `-test=false` to take test scaffolding out of the ranking. The JSON keeps an agent from counting message fragments, and it refuses rather than reporting a zero for a package that did not compile.

## Limits

declscope reads one package at a time, and counts a use only where a name is written.

| Not seen | Consequence |
| --- | --- |
| Uses outside the package | A scope beyond `package` could not be checked, so none exists. [`declscope shrink`](#unexporting-what-no-importer-uses) loads the module to judge exportedness inside `internal/` |
| Whole-value operations on a struct | Copying, comparing or zeroing a value names no field |
| A composite literal of a type parameter | `T{1}` fills the fields of whatever `T` is instantiated with, without naming them |
| Reflection, `//go:linkname`, generated files | These reach a declaration without spelling it |
| A declaration nobody uses | `boundary` needs a use to find, so unused code produces no diagnostic |

For the last row, use an unused-code linter: [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode), or staticcheck's `unused` for a library with no `main` to root from.

## License

MIT
