// Package namespace resolves the pseudo-namespace that a Go source file
// belongs to.
//
// A namespace is the unit of privacy that declscope enforces. By
// default it is derived from the file name, so each file is its own namespace,
// but files may opt into a shared namespace with a directive:
//
//	//declscope:namespace user
//	package repo
//
// Deriving the default from the file name rather than using the file name
// itself keeps renaming a file from cascading into renaming every identifier
// it declares.
//
// A namespace does two jobs, and they are deliberately kept apart. As an
// identity it answers "is this reference inside the same namespace?", and
// every file that has a stem has one. As a label it is the prefix that the
// naming rules ask an unexported declaration to carry, and only a namespace
// that can start an identifier qualifies; IsLabel tells the two apart. 2fa.go
// is a namespace that a test file can share, but no identifier begins with a
// digit, so it can never be a label.
package namespace

import (
	"fmt"
	"go/ast"
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
//	user_repository.go      -> userRepository
//	user_repository_test.go -> userRepository  (tests share their subject's namespace)
//	parser_linux.go         -> parser          (GOOS/GOARCH suffixes are build
//	parser_linux_amd64.go   -> parser           constraints, not namespaces)
//	user_id.go              -> userID          (initialisms are spelled the way Go does)
//	foo-bar.go, foo.bar.go  -> fooBar          (any separator, not only _)
//	Foo.go, HTTPServer.go   -> foo, httpServer (a PascalCase stem is lowered)
//	v2_client.go            -> v2Client
//	2fa_auth.go             -> 2faAuth
//
// Of always returns the normalized stem when there is one, so that a file's
// identity never depends on whether the stem also makes a usable prefix:
// 2fa_test.go must share the namespace of 2fa.go even though no identifier can
// start with a digit. Whether the result can serve as a label is a separate
// question, answered by IsLabel. Only a file with no stem at all yields "".
func Of(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".go")

	// _test must be stripped before the build suffixes: the canonical shape is
	// name_GOOS_GOARCH_test.go.
	base = trimSegment(base, func(s string) bool { return s == "test" })
	base = trimSegment(base, isKnownArch)
	base = trimSegment(base, isKnownOS)

	return camel(base)
}

// IsLabel reports whether ns can be written as a label: prepended to an
// unexported identifier, it has to yield another unexported identifier.
//
// A namespace that cannot (2faAuth, since no identifier may start with a
// digit) is still a perfectly good identity for the privacy rules, but the
// naming rules have nothing they could ask for and stay silent. Any string
// that starts an identifier and does not export it qualifies, so a keyword
// stem such as struct.go still labels its declarations (structHelper).
func IsLabel(ns string) bool {
	if ns == "" || ast.IsExported(ns) {
		return false
	}
	return token.IsIdentifier(ns) || token.IsKeyword(ns)
}

// HasPrefix reports whether an identifier carries ns as a namespace prefix.
//
// The comparison ignores case, so that the author need not guess which
// spelling of an initialism the namespace uses: userIDCache and userIdCache
// both carry the label of user_id.go. What it does insist on is that the
// label is a whole word and not a fragment of a longer one, so namespace
// "user" claims userCache and user2 but neither users nor usercache. A name
// that reproduces a word break inside a multi-word namespace has already
// shown the label is there, which is why userIdcache carries userId even
// though what follows is lowercase.
func HasPrefix(name, ns string) bool {
	if ns == "" {
		return false
	}
	rest, ok := cutFold(name, ns)
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	if unicode.IsUpper(r) || unicode.IsDigit(r) {
		return true
	}
	return hasWordBreak(name[:len(name)-len(rest)])
}

// Qualify returns name rewritten to carry ns as its prefix, which is the
// rename offered when a declaration is missing its label.
//
// The first word of name is capitalized the way Go spells it, so id becomes
// userID and urlPath becomes userURLPath rather than userId and userUrlPath.
// A namespace that cannot be a label (see IsLabel) leaves the name alone.
func Qualify(name, ns string) string {
	if !IsLabel(ns) || HasPrefix(name, ns) {
		return name
	}
	return ns + capitalize(name)
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

// camel converts a file stem to lowerCamelCase.
//
// Every run of characters that could not appear in an identifier is a
// separator, so foo_bar, foo-bar and foo.bar all yield fooBar. The first
// segment is lowered the way Go lowers a leading initialism (HTTPServer ->
// httpServer) and every later one is capitalized the way Go capitalizes one
// (id -> ID), so that the namespace of user_id.go is the userID a Go
// programmer would write.
func camel(base string) string {
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var b strings.Builder
	for i, part := range parts {
		if i == 0 {
			b.WriteString(lowerLeading(part))
			continue
		}
		b.WriteString(capitalize(part))
	}
	return b.String()
}

// capitalize upper-cases the first word of a lowerCamelCase segment the way
// Go would: a word that is a common initialism is upper-cased whole (id ->
// ID, idCache -> IDCache, Id -> ID), any other has its first letter raised.
func capitalize(word string) string {
	if word == "" {
		return ""
	}
	if isInitialism(word) {
		return strings.ToUpper(word)
	}
	i := 0
	for i < len(word) {
		r, size := utf8.DecodeRuneInString(word[i:])
		if !unicode.IsLower(r) {
			break
		}
		i += size
	}
	if i > 0 && isInitialism(word[:i]) {
		return strings.ToUpper(word[:i]) + word[i:]
	}
	r, size := utf8.DecodeRuneInString(word)
	return string(unicode.ToUpper(r)) + word[size:]
}

// lowerLeading lowers the run of capitals a segment starts with, the way Go
// spells an identifier that begins with an initialism: ID yields id, URLPath
// yields urlPath rather than uRLPath, and Foo yields foo. A segment that does
// not start with a capital is returned unchanged.
func lowerLeading(s string) string {
	runes := []rune(s)
	upper := 0
	for upper < len(runes) && unicode.IsUpper(runes[upper]) {
		upper++
	}
	switch {
	case upper == 0:
		return s
	case upper == len(runes):
		// The whole segment is one initialism: ID -> id.
		return strings.ToLower(s)
	default:
		// An initialism followed by a word keeps the capital that starts that
		// word: URLPath -> urlPath.
		if upper > 1 {
			upper--
		}
		return strings.ToLower(string(runes[:upper])) + string(runes[upper:])
	}
}

// cutFold is strings.CutPrefix comparing under simple case folding, so that
// userIDCache matches the prefix userId.
func cutFold(name, prefix string) (rest string, ok bool) {
	for _, want := range prefix {
		if name == "" {
			return "", false
		}
		got, size := utf8.DecodeRuneInString(name)
		if got != want && unicode.ToLower(got) != unicode.ToLower(want) {
			return "", false
		}
		name = name[size:]
	}
	return name, true
}

// hasWordBreak reports whether s contains a capital anywhere but its first
// rune, which in a lowerCamelCase identifier means it is more than one word.
func hasWordBreak(s string) bool {
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func isInitialism(s string) bool { return commonInitialisms[strings.ToUpper(s)] }
func isKnownOS(s string) bool    { return knownOS[s] }
func isKnownArch(s string) bool  { return knownArch[s] }

// commonInitialisms is golint's list of the words that are spelled in
// capitals. It is vendored rather than pulled in as a dependency: the list is
// small and stable, and the module otherwise depends only on x/tools and
// yaml.v3.
var commonInitialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true,
	"DNS": true, "EOF": true, "GUID": true, "HTML": true, "HTTP": true,
	"HTTPS": true, "ID": true, "IP": true, "JSON": true, "LHS": true,
	"QPS": true, "RAM": true, "RHS": true, "RPC": true, "SLA": true,
	"SMTP": true, "SQL": true, "SSH": true, "TCP": true, "TLS": true,
	"TTL": true, "UDP": true, "UI": true, "UID": true, "UUID": true,
	"URI": true, "URL": true, "UTF8": true, "VM": true, "XML": true,
	"XMPP": true, "XSRF": true, "XSS": true,
}

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
// The prefix is matched the way HasPrefix matches it, ignoring case. The
// leading run of capitals left behind is lowered the way Go spells an
// identifier that starts with an initialism, so userID yields id and
// userURLPath yields urlPath rather than iD and uRLPath.
//
// When no rename can be derived, short is empty and why says so, for reporting
// the violation without a fix: being unable to spell the new name is a limit
// of the fix, not a reason to let the label stand.
func Unqualify(name, ns string) (short, why string) {
	if ns == "" {
		return "", "the name does not begin with the namespace"
	}
	rest, ok := cutFold(name, ns)
	if !ok {
		return "", "the name does not begin with the namespace"
	}
	if rest == "" {
		return "", "nothing would remain"
	}

	r, _ := utf8.DecodeRuneInString(rest)
	if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
		// The label was confirmed by a word break inside the namespace, but
		// what follows it does not start a word of its own, so there is no
		// clean place to cut: userIdcache is a label and then a fragment.
		return "", "what follows the label does not start a new word"
	}
	rest = lowerLeading(rest)

	switch {
	case token.IsKeyword(rest):
		return "", fmt.Sprintf("%q is a keyword", rest)
	case !token.IsIdentifier(rest):
		return "", fmt.Sprintf("%q is not a valid identifier", rest)
	}
	return rest, ""
}
