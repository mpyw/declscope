package namespace_test

import (
	"testing"

	"github.com/mpyw/declscope/internal/namespace"
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

		{"http2.go", "http2"},
		{"v2_client.go", "v2Client"},
		{"doc.go", "doc"},
		{"_shadow.go", "shadow"},

		// A digit-leading stem is still an identity, so 2fa_test.go shares
		// the namespace of 2fa.go. Whether it can also be a prefix is
		// CanPrefix's question, not Of's.
		{"2fa_auth.go", "2faAuth"},
		{"2fa.go", "2fa"},
		{"2fa_test.go", "2fa"},

		// An initialism in a later segment is spelled the way Go spells it.
		{"user_id.go", "userID"},
		{"request_id.go", "requestID"},
		{"api_url.go", "apiURL"},
		{"parse_json.go", "parseJSON"},
		{"user_Id.go", "userID"},
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

		// Only a file with no stem at all yields nothing.
		{".go", ""},
		{"_.go", ""},
	}
	for _, tt := range tests {
		if got := namespace.Of(tt.path); got != tt.want {
			t.Errorf("Of(%q) = %q, want %q", tt.path, got, tt.want)
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
		{"_shadow", true},

		// Nothing can be prepended to these and yield an unexported
		// identifier.
		{"", false},
		{"2faAuth", false},
		{"Foo", false},
		{"foo-bar", false},
		{"foo.bar", false},
	}
	for _, tt := range tests {
		if got := namespace.CanPrefix(tt.ns); got != tt.want {
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
		{"ids", "user", "userIds"},           // nor its plural

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

		// An exported name keeps its exportedness, so the prefix is raised.
		{"Load", "user", "UserLoad"},

		// An empty name gains the namespace alone, not a stray capital.
		{"", "user", "user"},
	}
	for _, tt := range tests {
		if got := namespace.Qualify(tt.name, tt.ns); got != tt.qualified {
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
		if got := namespace.Contains(tt.name, tt.ns); got != tt.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", tt.name, tt.ns, got, tt.want)
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
		if got, ok := namespace.Unexported(in); !ok || got != want {
			t.Errorf("Unexported(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"MAX_RETRIES", "Foo_bar"} {
		if got, ok := namespace.Unexported(in); ok {
			t.Errorf("Unexported(%q) = %q, want no spelling", in, got)
		}
	}
}
