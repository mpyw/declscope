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

| Scope | Meaning |
| --- | --- |
| `public` | Usable outside the package. |
| `package` | Usable anywhere in the package. |
| `file` | Usable only inside its own **namespace**. |

Roughly: `file` is Rust's module-private, `package` is `pub(crate)`, `public` is `pub`.

## The rules

Everything unexported is private to its **namespace** — by default, to its own file. Widening is always an explicit act:

```go
// user.go   (namespace: user)

func UserLoad()    {} // public: usable outside the package
func userCache()   {} // namespace-private: only user.go may touch it

//declscope:package
func userShared()  {} // package-internal: usable anywhere in the package
```

Reach is stated by the directive, never by the name. This matters: if a prefix meant "package-wide", you could not also use one simply to say *which unit a declaration belongs to* — adding one for legibility would silently widen it, and a codebase that prefixed everything for readability would end up with nothing protected at all.

So the prefix does a different job.

### The prefix is an ownership label

An unexported package-level declaration must carry its namespace as a prefix. It grants nothing; it says who owns it, which is what makes a cross-file call legible at the call site, in a stack trace and in a grep result:

```go
// order.go
func orderRun() int {
    return userShared() // obviously the user unit's, and obviously shared
}
```

By default this is required only once a package has a **second namespace** — in a package with one, there is no boundary for a label to mark, and a prefix repeated on every declaration would distinguish nothing. Set `rules.prefix` to `true` to require it unconditionally, or `false` to drop the rule.

Exported identifiers are exempt: they are already qualified by the package name at every external use site.

### Methods and struct fields are bounded by their type

A method or field is already namespaced by the type that owns it. `u.save()` cannot collide with anything, so there is no pollution to prevent, and a label here would produce `u.userSave()` — exactly the stutter Go idiom avoids.

What is missing for members is not a namespace but **encapsulation**, so the boundary is the namespace of the **type**, not of the file:

```go
// user.go   (namespace: user)
type User struct { id int }
func (u *User) normalize() {}
```

```go
// order.go  (namespace: order)
func f(u *User) {
    u.id = 1      // reported: User's internals belong to namespace "user"
    u.normalize() // reported
}
```

## Namespaces

A namespace is the unit of file privacy. By default it is derived from the file name, so **each file is its own namespace**:

| File | Namespace |
| --- | --- |
| `user_repository.go` | `userRepository` |
| `user_repository_test.go` | `userRepository` — a test shares its subject's namespace |
| `parser_linux.go`, `parser_linux_amd64.go` | `parser` — GOOS/GOARCH suffixes are build constraints, not namespaces |
| `v2_client.go` | `v2Client` |
| `2fa_auth.go` | *(none — no identifier may start with a digit)* |

Files can opt into a **shared** namespace, which is how you split one logical unit across several files:

```go
//declscope:namespace user
package repo
```

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

Deriving the default from the file name rather than using the file name itself is deliberate: renaming a file should not cascade into renaming every identifier it declares.

## Directives

```go
//declscope:public      // state the scope explicitly instead of deriving it
//declscope:package
//declscope:file

//declscope:ignore      // suppress every diagnostic for this declaration

//declscope:namespace <name>   // before the package clause; overrides the file's namespace
```

Placement: the doc comment of a declaration, or a trailing comment on the same line. A directive on a parenthesized `var`/`const`/`type` block applies to every spec in it, and a directive on a spec overrides it. A trailing `// reason` is allowed.

```go
//declscope:package // shared with the reporting code
func helper() {}

func helper() {} //declscope:package
```

Unused `//declscope:ignore` directives are reported, so suppressions do not outlive the problem.

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

```bash
# Apply the first suggested fix of each diagnostic
declscope -fix ./...
```

Every diagnostic carries at most one fix, so `-fix` is unambiguous: a boundary crossing is fixed by inserting `//declscope:package`, and a missing label by renaming. The two can never conflict, because a rename does not change a declaration's reach.

A declaration whose scope was **already stated with a directive** is reported without a fix. Both the directive and the use site are deliberate statements, and `-fix` must not silently overwrite the one the author wrote.

## Adopting on an existing codebase

Turning declscope on for a codebase that predates it would report every boundary that was never enforced. Record them instead:

```console
declscope baseline ./...       # writes .declscope-baseline.yaml
```

Recorded violations are suppressed; new ones are still reported. The file is discovered by the same upward lookup as the config, so its presence is all it takes.

```yaml
packages:
  github.com/you/app/store:
    escape:
      - User.name
      - helper
```

An entry is keyed by **package, rule and declaration** — never by position — so it survives the code being moved, the file being renamed and the package being reformatted. Regenerate rather than edit:

```console
declscope baseline ./...
git diff .declscope-baseline.yaml   # the record of what was cleaned up
```

Entries for violations that have since been fixed simply disappear, which is why the analyzer never reports an entry as stale: a package's test variant sees references the ordinary variant does not, so "matched nothing" is not a reliable signal from inside one pass.

A baseline suppresses, it does not endorse. Nothing is written into the source, the convention still applies to every new declaration, and an entry can only be removed by actually fixing the violation. That is the difference between this and a `-fix` mode that writes `//declscope:package` everywhere: the latter would permanently opt the codebase out of the convention it was adopted for.

## Configuration

Optional. `.declscope.yaml` (or `.yml`), looked up from the analyzed package's directory upwards, stopping at the module root — so a subtree can relax or tighten the rules on its own.

```yaml
defaults:
  exported: public      # public | package | file
  unexported: file

rules:
  prefix: ondemand        # true | false | ondemand (required once a package has two namespaces)
  members: true           # bound unexported methods/fields by their type's namespace
  foreign-methods: false  # report unexported methods grown on another namespace's type

exclude:
  - "**/mock_*.go"

baseline: .declscope-baseline.yaml   # relative to this file; found automatically if named by default
```

Unknown keys are an error rather than a silent no-op: a typo in a rule name would otherwise leave the rule at its default with no sign of it.

### `prefix`

`ondemand` (the default) requires the label only in a package with more than one namespace. `true` requires it unconditionally, which costs a little stutter in single-file packages but means a package gaining its second namespace is not a mass rename. `false` drops the rule, leaving reach enforcement without any naming discipline.

### `foreign-methods`

Reports an unexported method grown on a type belonging to another namespace, at the declaration rather than at the call. Off by default because the cross-namespace reference rule already catches it wherever the method is actually used.

This does **not** break the sealed-interface pattern: implementing `isSealed()` on your own type declares a method owned by *your* type, and satisfying an interface creates no reference to the interface's method.

## Using it with an AI agent

The point of declscope is that the boundary stops being tacit knowledge, so put it where the agent will hit it:

```bash
declscope ./...        # in CI, and in the agent's build/verify loop
declscope -fix ./...   # deterministic: applies the rename, reports the alternative it skipped
```

On an existing codebase, run `declscope baseline ./...` once first, so the agent is only ever shown the boundaries *it* crossed.

Two things make this work better than a written convention:

- **The diagnostic names the namespace it crossed**, so the agent is told *why* the call is wrong, not merely that it is. Its repair is a rename or a directive, both mechanical.
- **A directive is a durable record of intent.** When `//declscope:package` ends up in the source, the next agent to read the file inherits the decision instead of re-deriving it — and the next one that widens something silently gets caught.

A line in `CLAUDE.md` (or the equivalent for your agent) is usually enough:

```markdown
Run `declscope ./...` before finishing. Do not widen a declaration's scope to make
a call site compile: either keep the call inside the namespace, or state the new
scope with `//declscope:package` and say why.
```

That last clause matters. Left to itself an agent will take the cheapest path out of a diagnostic, and the cheapest path here is to widen everything. Making the widening explicit is the whole mechanism.

## Scope of the analysis

- Generated files (`// Code generated ... DO NOT EDIT.`) are excluded entirely — neither checked nor treated as reference sites.
- Everything is checked **within a single package**. Namespaces are therefore implicitly package-qualified and never collide across packages.
- Whether an *exported* identifier is used outside its package is out of scope: `go/analysis` has no upward view of the program, and answering it would require a separate whole-program mode. Combine with an unused-code linter for that.
- Embedded fields are skipped, since their name comes from the embedded type.

## License

MIT
