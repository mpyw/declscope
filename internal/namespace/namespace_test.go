package namespace

import (
	"slices"
	"testing"
)

func TestOf(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"user.go", "user"},
		{"user_repository.go", "userRepository"},
		{"a/b/user_repository.go", "userRepository"},

		// Tests share their subject's namespace.
		{"user_test.go", "user"},
		{"user_repository_test.go", "userRepository"},

		// GOOS/GOARCH suffixes are build constraints, not namespaces.
		{"parser_linux.go", "parser"},
		{"parser_amd64.go", "parser"},
		{"parser_linux_amd64.go", "parser"},
		{"parser_linux_amd64_test.go", "parser"},
		{"file_windows_arm64.go", "file"},

		// A bare GOOS/GOARCH name is an ordinary file, matching go/build.
		{"linux.go", "linux"},
		{"amd64.go", "amd64"},

		// Only the documented order is stripped: GOARCH never precedes GOOS.
		{"parser_amd64_linux.go", "parserAmd64"},

		// go/build reads a suffix only before the first dot. What follows the
		// dot stays in the namespace.
		{"foo_linux.pb.go", "fooPb"},
		{"foo.bar_linux.go", "fooBarLinux"},
		{"x.linux_amd64.go", "xLinuxAmd64"},
		{"foo_linux_test.pb.go", "fooTestPb"},
		{"foo.bar_test.go", "fooBar"},
		{"foo_test.pb.go", "fooTestPb"},

		// Only a name ending in _test.go is a test file, as go/build tells
		// one, so a build suffix after _test leaves an ordinary file.
		{"foo_test_linux.go", "fooTest"},
		{"foo_test_amd64.go", "fooTest"},
		{"foo_test_linux_amd64.go", "fooTest"},
		{"foo_linux_test.go", "foo"},
		{"foo_test_linux_test.go", "fooTest"},

		// mips8 is not a GOARCH go/build knows.
		{"foo_mips8.go", "fooMips8"},

		{"http2.go", "http2"},
		{"v2_client.go", "v2Client"},
		{"doc.go", "doc"},
		{"_shadow.go", "shadow"},

		// A digit-leading stem is still an identity, so 2fa_test.go shares
		// the namespace of 2fa.go. Whether it can also be a prefix is
		// CanPrefix's question, not of's.
		{"2fa_auth.go", "2faAuth"},
		{"2fa.go", "2fa"},
		{"2fa_test.go", "2fa"},

		// An initialism in a later segment is spelled the way Go spells it.
		{"user_id.go", "userID"},
		{"request_id.go", "requestID"},
		{"api_url.go", "apiURL"},
		{"parse_json.go", "parseJSON"},
		{"user_Id.go", "userID"},
		// So is a plural initialism, the s kept lower-case.
		{"user_ids.go", "userIDs"},
		{"user_urls.go", "userURLs"},
		{"user_Ids.go", "userIDs"},
		{"user_idents.go", "userIdents"},
		// The first segment is a word of its own and stays as written.
		{"http_client.go", "httpClient"},
		{"id_cache.go", "idCache"},

		// Any separator yields lowerCamelCase, not only _.
		{"foo-bar.go", "fooBar"},
		{"foo.bar.go", "fooBar"},
		{"foo-bar_test.go", "fooBar"},
		{"foo-bar-baz.go", "fooBarBaz"},

		// A PascalCase stem is lowered the way Go lowers a leading
		// initialism.
		{"Foo.go", "foo"},
		{"FooBar.go", "fooBar"},
		{"HTTPServer.go", "httpServer"},
		{"ID.go", "id"},
		{"User_Repository.go", "userRepository"},
		// The first segment is lowered as Unexported lowers a name, so a
		// plural initialism or one before a digit stays one word.
		{"IDs.go", "ids"},
		{"URLs.go", "urls"},
		{"IPv4.go", "ipv4"},
		{"UUIDs_test.go", "uuids"},
		{"URLPath.go", "urlPath"},
		{"HTTP2Client.go", "http2Client"},
		{"OAuth.go", "oAuth"},
		{"IDs_cache.go", "idsCache"},

		// Only a file with no stem at all yields nothing.
		{".go", ""},
		{"_.go", ""},
	}
	for _, tt := range tests {
		if got := of(tt.path); got != tt.want {
			t.Errorf("of(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestCanPrefix(t *testing.T) {
	tests := []struct {
		ns   string
		want bool
	}{
		{"user", true},
		{"userRepository", true},
		{"userID", true},
		{"v2Client", true},
		{"http2", true},
		// A keyword still starts an identifier: structHelper is one.
		{"struct", true},
		{"äpfel", true},
		// A titlecase letter is not exported, but its capital is.
		{"\u01c5x", true},

		// Nothing can be prepended to these and yield an unexported
		// identifier.
		{"", false},
		{"2faAuth", false},
		{"Foo", false},
		{"foo-bar", false},
		{"foo.bar", false},
		// The first letter has no capital form, so an exported name could
		// not take the prefix and stay exported.
		{"_shadow", false},
		{"用户", false},
		// Its capital form is titlecase, which does not export a name either.
		{"\u1f80x", false},
	}
	for _, tt := range tests {
		if got := CanPrefix(tt.ns); got != tt.want {
			t.Errorf("CanPrefix(%q) = %v, want %v", tt.ns, got, tt.want)
		}
	}
}

func TestQualify(t *testing.T) {
	tests := []struct {
		name, ns, qualified string
	}{
		{"helper", "user", "userHelper"},
		{"cache", "userRepository", "userRepositoryCache"},
		{"userHelper", "user", "userHelper"},
		{"helper", "", "helper"},

		// The first word is capitalized the way Go spells an initialism.
		{"id", "user", "userID"},
		{"idCache", "user", "userIDCache"},
		{"urlPath", "user", "userURLPath"},
		{"lookup", "userID", "userIDLookup"},
		{"identity", "user", "userIdentity"}, // not an initialism
		{"ids", "user", "userIDs"},           // but its plural is
		{"idsCache", "user", "userIDsCache"},

		// A name that already carries the prefix in another spelling is left
		// alone rather than doubled up.
		{"userIDCache", "userId", "userIDCache"},
		{"userIdCache", "userID", "userIdCache"},

		// A name that carries the namespace inside is already qualified, so
		// nothing is prepended. The rule would not have fired on it either;
		// the guard keeps Qualify idempotent under the same test.
		{"LoadConfig", "config", "LoadConfig"},
		{"statementReducer", "reducer", "statementReducer"},

		// A namespace that cannot be a prefix leaves the name unchanged.
		{"helper", "2faAuth", "helper"},
		{"helper", "Foo", "helper"},
		{"Load", "用户", "Load"},
		{"Save", "_foo", "Save"},

		// An exported name keeps its exportedness, so the prefix is raised.
		{"Load", "user", "UserLoad"},

		// An empty name gains the namespace alone, not a stray capital.
		{"", "user", "user"},
	}
	for _, tt := range tests {
		if got := Qualify(tt.name, tt.ns); got != tt.qualified {
			t.Errorf("Qualify(%q, %q) = %q, want %q", tt.name, tt.ns, got, tt.qualified)
		}
	}
}

func TestContainsCases(t *testing.T) {
	tests := []struct {
		name, ns string
		want     bool
	}{
		// Names the prefix rule doubled a word in, measured on real code.
		{"statementReducer", "reducer", true},
		{"NewRegistry", "registry", true},
		{"LoadConfig", "config", true},
		{"NewTracer", "tracer", true},
		{"hasDirective", "directive", true},
		{"AWSScope", "scope", true},
		{"BuildIgnoreMap", "ignore", true},
		// Derived words. The match ends inside a word.
		{"SpecifierParser", "parse", true},
		{"CheckConflicts", "conflict", true},
		{"APIParser", "parse", true},
		// The file stem and the identifier split into words differently.
		{"NewAzureAppConfigParamStrategy", "azureAppconfigParam", true},
		// The name is the namespace.
		{"collect", "collect", true},
		// Inflections that change the spelling. Only whole forms generated from
		// the namespace are accepted.
		{"storing", "store", true},
		{"storingWorker", "store", true},
		{"NewStoring", "store", true},
		{"parsing", "parse", true},
		{"ignoringMap", "ignore", true},
		{"applies", "apply", true},
		{"appliedResults", "apply", true},
		{"applying", "apply", true}, // plain containment, not a generated form
		// In a compound namespace, only the final word inflects.
		{"userStoring", "userStore", true},
		// What stemming would wrongly accept. Matching a bare stor with a free
		// right edge lets these through. Generating from the namespace does not.
		{"story", "store", false},
		{"storm", "store", false},
		{"stories", "store", false},
		{"userStory", "userStore", false},
		{"appliance", "apply", false},
		// After a vowel, e and y keep their spelling, so nothing is generated.
		{"freeing", "free", true}, // free + ing, carried by plain containment
		{"freing", "free", false},
		{"deploys", "deploy", true}, // deploy + s, carried by plain containment
		{"deploies", "deploy", false},
		// The left edge is not a word boundary. These must not pass.
		{"monkey", "key", false},
		{"UntagCommand", "tag", false},
		// The namespace does not appear at all.
		{"Wrap", "client", false},
		{"nounSecret", "command", false},
		// A plural initialism is one word, so its last capital and s do not
		// spell a short namespace.
		{"fetchURLs", "ls", false},
		{"URLs", "ls", false},
		{"userIDs2", "ds", false},
		{"UIDsCache", "ds", false},
		{"fetchURLs", "urls", true},
		{"UIDsCache", "cache", true},
		// But a word opened by the last capital still is one.
		{"URLSet", "set", true},
		{"APIServer", "server", true},
		{"IDStore", "store", true},
		// Accepted as the price of anchoring only the left edge. Intended.
		{"models", "mode", true},
		// The core namespace has no name, so no name carries it.
		{"helper", "", false},
		// A change between letters and digits opens a word, even before a
		// lowercase letter.
		{"v2user", "user", true},
		// A name shorter than the namespace cannot hold it.
		{"use", "user", false},
		// A vocabulary word comes from YAML unchecked, so it may end in
		// U+FFFD, the rune an exhausted name decodes as. The name must still
		// run out before the word does.
		{"user", "user\uFFFD", false},
	}
	for _, tt := range tests {
		if got := Contains(tt.name, tt.ns); got != tt.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", tt.name, tt.ns, got, tt.want)
		}
	}
}

func TestLooksInflected(t *testing.T) {
	tests := []struct {
		ns   string
		want bool
	}{
		// The forms inflections generates, read back from the namespace.
		{"tracing", true},
		{"storing", true},
		{"walking", true},
		{"mapping", true},
		{"applies", true},
		{"unified", true},
		// Only the final word of a compound namespace inflects.
		{"userTracing", true},
		{"tracingUser", false},
		{"exprString", false},
		// No vowel before the suffix, so there is no stem to list.
		{"string", false},
		{"ring", false},
		{"ping", false},
		{"dies", false},
		// After a vowel, e and y keep their spelling, so nothing is generated.
		{"freeing", false},
		{"deploying", true}, // deploy + ing: y is not a vowel here
		{"agreed", false},
		// Not an inflected form at all.
		{"trace", false},
		{"user", false},
		{"ing", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := LooksInflected(tt.ns); got != tt.want {
			t.Errorf("LooksInflected(%q) = %v, want %v", tt.ns, got, tt.want)
		}
	}
}

func TestUnexported(t *testing.T) {
	tests := map[string]string{
		"Load":        "load",
		"HTTPClient":  "httpClient",
		"ID":          "id",
		"IDs":         "ids",
		"URLs":        "urls",
		"URLPath":     "urlPath",
		"HTTPServer":  "httpServer",
		"APIKey":      "apiKey",
		"IDToken":     "idToken",
		"UUIDs":       "uuids",
		"UIDs":        "uids",
		"UIDsCache":   "uidsCache",
		"UIs":         "uis",
		"OAuth":       "oAuth",
		"HTTP2Client": "http2Client",
		"IPv4":        "ipv4",
		"ABC2Foo":     "abc2Foo",
		"MAX":         "max",
		"X":           "x",
		"load":        "load",
		"Äpfel":       "äpfel",
	}
	for in, want := range tests {
		if got, ok := Unexported(in); !ok || got != want {
			t.Errorf("Unexported(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"MAX_RETRIES", "Foo_bar"} {
		if got, ok := Unexported(in); ok {
			t.Errorf("Unexported(%q) = %q, want no spelling", in, got)
		}
	}
}

// TestPluralInitialismIsOneWord generates names that spell a common
// initialism, singular and plural, at every position, and checks them
// against short namespaces such as ls.go's and ds.go's. wordStart opened a
// word at the last capital of URLs, so ls was found in fetchURLs but not in
// fetchUrls, and shrink renamed an exported URLs that carried ls to urls,
// which does not.
func TestPluralInitialismIsOneWord(t *testing.T) {
	var namespaces []string
	for r := 'a'; r <= 'z'; r++ {
		namespaces = append(namespaces, string(r), string(r)+"s")
	}
	var names []string
	for word := range commonInitialisms {
		for _, w := range []string{word, word + "s"} {
			names = append(names,
				w, w+"Cache", w+"2", "Fetch"+w, "Fetch"+w+"Cache", "Fetch"+w+"2",
				"fetch"+w, "fetch"+w+"Cache", "fetch"+w+"2")
		}
	}
	for _, name := range names {
		renamed, ok := Unexported(name)
		for _, ns := range namespaces {
			// shrink renames with Unexported, and the rename must not take away
			// a namespace the name carried.
			if ok && Contains(name, ns) && !Contains(renamed, ns) {
				t.Errorf("Contains(%q, %q) holds, but not for its rename %q", name, ns, renamed)
			}
			if CanPrefix(ns) {
				if q := Qualify(name, ns); !Contains(q, ns) {
					t.Errorf("Qualify(%q, %q) = %q does not carry it", name, ns, q)
				}
			}
		}
	}
}

// TestLowerLeadingMatchesUnexported pins that a file named after an exported
// declaration takes the namespace its unexported name would have, since both
// are spelled by lowerLeading.
func TestLowerLeadingMatchesUnexported(t *testing.T) {
	for _, name := range []string{
		"Load", "HTTPClient", "ID", "IDs", "URLs", "URLPath", "HTTPServer",
		"HTTPSProxy", "OAuth", "HTTP2Client", "IPv4", "MAX", "X", "load", "Äpfel",
		"UIDs", "UIDsCache",
	} {
		want, _ := Unexported(name)
		if got := of(name + ".go"); got != want {
			t.Errorf("of(%q) = %q, Unexported(%q) = %q", name+".go", got, name, want)
		}
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name  string
		files []Declared
		want  []Resolved
	}{
		{
			"derived",
			[]Declared{{Path: "user.go"}, {Path: "user_repository.go"}},
			[]Resolved{{Name: "user"}, {Name: "userRepository"}},
		},
		{
			"named directive",
			[]Declared{{Path: "user.go", Named: true, Name: "custom"}},
			[]Resolved{{Name: "custom"}},
		},
		{
			"core directive",
			[]Declared{{Path: "user.go", Core: true}},
			[]Resolved{{Core: true}},
		},
		{
			"test file joins core subject",
			[]Declared{{Path: "user.go", Core: true}, {Path: "user_test.go"}},
			[]Resolved{{Core: true}, {Core: true}},
		},
		{
			"test file joins non-core subject",
			[]Declared{{Path: "user.go"}, {Path: "user_test.go"}},
			[]Resolved{{Name: "user"}, {Name: "user"}},
		},
		{
			// Only the core is joined. A test file keeps the namespace of its
			// own stem unless it states the same directive.
			"test file keeps its stem under named subject",
			[]Declared{{Path: "user.go", Named: true, Name: "custom"}, {Path: "user_test.go"}},
			[]Resolved{{Name: "custom"}, {Name: "user"}},
		},
		{
			// A file that names its namespace keeps it: the directive is the
			// author's decision, and joining the core would drop it silently.
			"test file naming its namespace does not join core subject",
			[]Declared{{Path: "user.go", Core: true}, {Path: "user_test.go", Named: true, Name: "other"}},
			[]Resolved{{Core: true}, {Name: "other"}},
		},
		{
			// A core test file does not take its subject with it: the subject
			// is read by both variants, the test file by one.
			"subject does not join core test file",
			[]Declared{{Path: "client.go"}, {Path: "client_test.go", Core: true}},
			[]Resolved{{Name: "client"}, {Core: true}},
		},
		{
			// A GOOS sibling is built only sometimes, so joining it would make
			// the namespace of foo.go depend on the build.
			"plain sibling does not join core GOOS file",
			[]Declared{{Path: "foo.go"}, {Path: "foo_linux.go", Core: true}},
			[]Resolved{{Name: "foo"}, {Core: true}},
		},
		{
			"GOOS sibling does not join core file",
			[]Declared{{Path: "foo.go", Core: true}, {Path: "foo_linux.go"}},
			[]Resolved{{Core: true}, {Name: "foo"}},
		},
		{
			// A test file joins the file it tests, named in full.
			"GOOS test file joins its own subject only",
			[]Declared{
				{Path: "foo_linux.go", Core: true},
				{Path: "foo_test.go"},
				{Path: "foo_linux_test.go"},
			},
			[]Resolved{{Core: true}, {Name: "foo"}, {Core: true}},
		},
		{
			"test-suffixed GOOS file is no test file",
			[]Declared{{Path: "foo.go", Core: true}, {Path: "foo_test_linux.go"}},
			[]Resolved{{Core: true}, {Name: "fooTest"}},
		},
		{
			"mixed core and named",
			[]Declared{
				{Path: "core.go", Core: true},
				{Path: "core_test.go"},
				{Path: "helper.go", Named: true, Name: "util"},
			},
			[]Resolved{
				{Core: true},
				{Core: true},
				{Name: "util"},
			},
		},
		{
			"empty stem",
			[]Declared{{Path: ".go"}},
			[]Resolved{{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.files); !slices.Equal(got, tt.want) {
				t.Errorf("Resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSpell(t *testing.T) {
	tests := []struct {
		name string
		r    Resolved
		path string
		want string
	}{
		{"core", Resolved{Core: true}, "user.go", "(core)"},
		{"named", Resolved{Name: "custom"}, "user.go", "custom"},
		{"no stem", Resolved{}, ".go", "(file .go)"},
		{"no stem in a directory", Resolved{}, "a/b/.go", "(file .go)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Spell(tt.r, tt.path); got != tt.want {
				t.Errorf("Spell(%+v, %q) = %q, want %q", tt.r, tt.path, got, tt.want)
			}
		})
	}
}
