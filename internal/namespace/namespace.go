// Package namespace resolves the pseudo-namespace that a Go source file
// belongs to.
//
// A namespace is the unit of "file privacy" that declscope enforces. By
// default it is derived from the file name, so each file is its own namespace,
// but files may opt into a shared namespace with a directive:
//
//	//declscope:namespace user
//	package repo
//
// Deriving the default from the file name rather than using the file name
// itself keeps renaming a file from cascading into renaming every identifier
// it declares.
package namespace

import (
	"go/token"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Of returns the default namespace for a file path.
//
// The path is reduced to its base name and then normalized:
//
//	user_repository.go    -> userRepository
//	user_repository_test.go -> userRepository  (tests share their subject's namespace)
//	parser_linux.go       -> parser            (GOOS/GOARCH suffixes are build
//	parser_linux_amd64.go -> parser             constraints, not namespaces)
//	v2_client.go          -> v2Client
//	http2.go              -> http2
//
// Of returns "" when the file name cannot yield a valid Go identifier prefix
// (for example "2fa_auth.go", since no identifier may start with a digit).
// A file with no namespace has no way to express package-internal scope
// through naming, so its declarations can only be promoted with a directive.
func Of(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".go")

	// _test must be stripped before the build suffixes: the canonical shape is
	// name_GOOS_GOARCH_test.go.
	base = trimSegment(base, func(s string) bool { return s == "test" })
	base = trimSegment(base, isKnownArch)
	base = trimSegment(base, isKnownOS)

	name := camel(base)
	if !isIdentifierStart(name) {
		return ""
	}
	return name
}

// HasPrefix reports whether an identifier carries ns as a namespace prefix.
//
// The character following the prefix must begin a new word, so that namespace
// "user" claims userCache but not users.
func HasPrefix(name, ns string) bool {
	if ns == "" {
		return false
	}
	rest, ok := strings.CutPrefix(name, ns)
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsUpper(r) || unicode.IsDigit(r)
}

// Qualify returns name rewritten to carry ns as its prefix, which is the
// rename offered when a file-private declaration needs to become
// package-internal.
func Qualify(name, ns string) string {
	if ns == "" || HasPrefix(name, ns) {
		return name
	}
	r, size := utf8.DecodeRuneInString(name)
	return ns + string(unicode.ToUpper(r)) + name[size:]
}

// trimSegment drops the final underscore-separated segment when match accepts
// it. The segment is only dropped when something would remain, mirroring
// go/build: linux.go is an ordinary file, foo_linux.go is constrained.
func trimSegment(base string, match func(string) bool) string {
	i := strings.LastIndex(base, "_")
	if i <= 0 {
		return base
	}
	if !match(base[i+1:]) {
		return base
	}
	return base[:i]
}

// camel converts a snake_case file stem to lowerCamelCase.
func camel(base string) string {
	parts := strings.Split(base, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(part)
			continue
		}
		r, size := utf8.DecodeRuneInString(part)
		b.WriteRune(unicode.ToUpper(r))
		b.WriteString(part[size:])
	}
	return b.String()
}

func isIdentifierStart(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLetter(r) || r == '_'
}

func isKnownOS(s string) bool   { return knownOS[s] }
func isKnownArch(s string) bool { return knownArch[s] }

// Mirrors the lists in go/internal/syslist, which is not importable.
var knownOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "nacl": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "wasip1": true, "windows": true, "zos": true,
}

var knownArch = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true,
	"arm64": true, "arm64be": true, "loong64": true, "mips": true,
	"mipsle": true, "mips64": true, "mips64le": true, "mips64p32": true,
	"mips64p32le": true, "mips8": true, "ppc": true, "ppc64": true,
	"ppc64le": true, "riscv": true, "riscv64": true, "s390": true,
	"s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}

// Unqualify returns name with ns stripped from its front, which is the rename
// offered when a namespace label is not wanted.
//
// The leading run of capitals left behind is lowered the way Go spells an
// identifier that starts with an initialism, so userID yields id and
// userURLPath yields urlPath rather than iD and uRLPath.
//
// It returns "" when nothing usable remains: the name was only the namespace,
// or stripping it would leave a keyword.
func Unqualify(name, ns string) string {
	rest, ok := strings.CutPrefix(name, ns)
	if !ok || rest == "" {
		return ""
	}

	// Measure the leading run of capitals.
	runes := []rune(rest)
	upper := 0
	for upper < len(runes) && unicode.IsUpper(runes[upper]) {
		upper++
	}
	switch {
	case upper == 0:
		// Not a word boundary, so the prefix was never a label.
		return ""
	case upper == len(runes):
		// The remainder is one initialism: ID -> id.
		rest = strings.ToLower(rest)
	default:
		// An initialism followed by a word keeps the capital that starts that
		// word: URLPath -> urlPath.
		if upper > 1 {
			upper--
		}
		rest = strings.ToLower(string(runes[:upper])) + string(runes[upper:])
	}

	if !token.IsIdentifier(rest) || token.IsKeyword(rest) {
		return ""
	}
	return rest
}
