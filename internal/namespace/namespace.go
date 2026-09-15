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
// every file that has a stem has one. As a mark it is what the naming rule
// asks a declaration's name to carry somewhere (Contains), offering a prefix
// when it is absent (Qualify), and only a namespace that can start an
// identifier qualifies for the fix; CanPrefix tells the two apart. 2fa.go is a
// namespace that a test file can share, but no identifier begins with a digit,
// so it can never be a prefix.
package namespace

import (
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
// start with a digit. Whether the result can serve as a prefix is a separate
// question, answered by CanPrefix. Only a file with no stem at all yields "".
func Of(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".go")

	// _test must be stripped before the build suffixes: the canonical shape is
	// name_GOOS_GOARCH_test.go.
	base = trimSegment(base, func(s string) bool { return s == "test" })
	base = trimSegment(base, isKnownArch)
	base = trimSegment(base, isKnownOS)

	return camel(base)
}

// CanPrefix reports whether ns can be written as a prefix: prepended to an
// unexported identifier, it has to yield another unexported identifier.
//
// A namespace that cannot (2faAuth, since no identifier may start with a
// digit) is still a perfectly good identity for the privacy rules, but the
// naming rules have nothing they could ask for and stay silent. Any string
// that starts an identifier and does not export it qualifies, so a keyword
// stem such as struct.go still prefixes its declarations (structHelper).
func CanPrefix(ns string) bool {
	if ns == "" || ast.IsExported(ns) {
		return false
	}
	return token.IsIdentifier(ns) || token.IsKeyword(ns)
}

// Qualify returns name rewritten to carry ns as a prefix, which is the rename
// offered when a name does not carry its namespace anywhere (see Contains). A
// name that already carries it is left alone, so the fix can never double a
// word the name already has.
//
// The first word of name is capitalized the way Go spells it, so id becomes
// userID and urlPath becomes userURLPath rather than userId and userUrlPath.
// A namespace that cannot be a prefix (see CanPrefix) leaves the name alone.
//
// The result keeps name's exportedness. A namespace is always an unexported
// identifier, so prepending one blindly would lower-case an exported name:
// Load in user.go would be renamed to userLoad, deleting the package's API to
// satisfy a linter. An exported name takes an exported prefix instead.
func Qualify(name, ns string) string {
	if !CanPrefix(ns) || Contains(name, ns) {
		return name
	}
	out := ns + capitalize(name)
	if ast.IsExported(name) {
		return capitalize(out)
	}
	return out
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

// Contains reports whether ns is written in name, beginning at a word boundary.
// The match may end inside a word, so parse is carried by SpecifierParser and
// conflict by CheckConflicts: a name that spells its namespace as a plural or
// an agent noun carries it as plainly as one that spells it whole.
//
// The left edge is anchored because the right one is not. Without the anchor,
// key would be found in monkey and every short namespace would stop meaning
// anything. With it, the cost is names that open with the namespace by accident
// — mode is found in models — which is the narrower failure of the two.
//
// The free right edge covers every derivation that keeps the namespace's own
// spelling (renders, rendering, Parser), but not the two English inflections
// that change it: a final e dropped before -ing (store → storing) and a final
// y turned to i (apply → applies, applied). Those are accepted through forms
// generated from the namespace — see inflections — never by stemming the name:
// a stem of store loose in the name would also accept story and storm, so only
// the whole generated forms count.
//
// Matching is case-folded: a namespace is an identity, and the leading letter
// of a name is decided by whether it is exported, not by the namespace.
func Contains(name, ns string) bool {
	if ns == "" {
		return false
	}
	if containsForm(name, ns) {
		return true
	}
	for _, form := range inflections(ns) {
		if containsForm(name, form) {
			return true
		}
	}
	return false
}

// containsForm is one spelling's share of Contains: the form must begin at a
// word boundary of name, and may end anywhere.
func containsForm(name, form string) bool {
	for i := range name {
		if !wordStart(name, i) {
			continue
		}
		if _, ok := cutFold(name[i:], form); ok {
			return true
		}
	}
	return false
}

// inflections returns the spellings of ns that English writes with a changed
// stem, which the free right edge of Contains cannot reach:
//
//	store → storing    (the final e drops before -ing)
//	apply → applies, applied    (the final y turns to i)
//
// Each is a complete form, never a bare stem: matching stor with a free right
// edge would accept story and storm, and matching appli would accept appliance.
// In a compound namespace only the final word inflects (azureAppconfigParam),
// and the final word is where the tail letters sit, so inspecting the last two
// runes is enough. A final e or y after a vowel does not change spelling
// (freeing, deploys) and generates nothing.
func inflections(ns string) []string {
	last, lastSize := utf8.DecodeLastRuneInString(ns)
	prev, _ := utf8.DecodeLastRuneInString(ns[:len(ns)-lastSize])
	if prev == utf8.RuneError || !unicode.IsLetter(prev) || isVowel(prev) {
		return nil
	}
	stem := ns[:len(ns)-lastSize]
	switch unicode.ToLower(last) {
	case 'e':
		return []string{stem + "ing"}
	case 'y':
		return []string{stem + "ies", stem + "ied"}
	}
	return nil
}

// isVowel reports an English vowel letter, case-folded. y is deliberately not
// one here: apply's y is what inflects, so for the letter before the tail the
// question never arises in a real namespace.
func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// wordStart reports whether the rune at byte offset i opens a word of a
// camelCase identifier. An initialism counts as one word, so the P of APIParser
// opens a word while the I of API does not: inside a run of capitals, only the
// last one does, and only because a lowercase rune follows it.
func wordStart(name string, i int) bool {
	if i == 0 {
		return true
	}
	r, size := utf8.DecodeRuneInString(name[i:])
	prev, _ := utf8.DecodeLastRuneInString(name[:i])
	if unicode.IsDigit(r) != unicode.IsDigit(prev) {
		return true
	}
	if !unicode.IsUpper(r) {
		return false
	}
	if !unicode.IsUpper(prev) {
		return true
	}
	next, _ := utf8.DecodeRuneInString(name[i+size:])
	return unicode.IsLower(next)
}
