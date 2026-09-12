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
		// the namespace of 2fa.go. Whether it can also be a label is
		// IsLabel's question, not Of's.
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

func TestIsLabel(t *testing.T) {
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
		if got := namespace.IsLabel(tt.ns); got != tt.want {
			t.Errorf("IsLabel(%q) = %v, want %v", tt.ns, got, tt.want)
		}
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name, ns string
		want     bool
	}{
		{"userCache", "user", true},
		{"user", "user", true},
		{"user2", "user", true},
		{"users", "user", false},
		{"userscache", "user", false},
		{"usercache", "user", false},
		{"cache", "user", false},
		{"anything", "", false},

		// Case is ignored, so the author need not guess which spelling of
		// an initialism the namespace uses.
		{"userIDCache", "userId", true},
		{"userIdCache", "userID", true},
		{"userID", "userId", true},
		{"userid", "userID", true},
		{"useridCache", "userID", true},
		{"parseJsonTree", "parseJSON", true},

		// A word break inside a multi-word namespace already confirms the
		// label, so a lowercase continuation is a fragment after it rather
		// than proof there was none.
		{"userIdcache", "userId", true},
		{"userIdcache", "userID", true},
		{"userIDcache", "userId", true},

		// But a single word followed by lowercase is just a longer word.
		{"useridentity", "userID", false},
		{"userids", "userID", false},

		// Unrelated names.
		{"orderIDCache", "userID", false},
		{"user", "userID", false},
	}
	for _, tt := range tests {
		if got := namespace.HasPrefix(tt.name, tt.ns); got != tt.want {
			t.Errorf("HasPrefix(%q, %q) = %v, want %v", tt.name, tt.ns, got, tt.want)
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

		// A name that already carries the label in another spelling is left
		// alone rather than doubled up.
		{"userIDCache", "userId", "userIDCache"},
		{"userIdCache", "userID", "userIdCache"},

		// A namespace that cannot be a label leaves the name unchanged.
		{"helper", "2faAuth", "helper"},
		{"helper", "Foo", "helper"},
	}
	for _, tt := range tests {
		if got := namespace.Qualify(tt.name, tt.ns); got != tt.qualified {
			t.Errorf("Qualify(%q, %q) = %q, want %q", tt.name, tt.ns, got, tt.qualified)
		}
	}
}

func TestUnqualify(t *testing.T) {
	tests := []struct {
		name, ns, want string
	}{
		{"userHelper", "user", "helper"},
		// A namespace of several words, as user_repository.go yields.
		{"userRepositoryCache", "userRepository", "cache"},

		// An initialism left behind is spelled the way Go spells one, rather
		// than by lowering only the first letter.
		{"userID", "user", "id"},           // not iD
		{"userURLPath", "user", "urlPath"}, // not uRLPath
		{"userIO", "user", "io"},

		// The label is matched the way HasPrefix matches it, ignoring case.
		{"userIdCache", "userID", "cache"},
		{"userIDCache", "userId", "cache"},
	}
	for _, tt := range tests {
		got, why := namespace.Unqualify(tt.name, tt.ns)
		if got != tt.want || why != "" {
			t.Errorf("Unqualify(%q, %q) = %q, %q, want %q and no reason", tt.name, tt.ns, got, why, tt.want)
		}
	}
}

// TestUnqualifyDeclines checks that every refusal explains itself, since the
// caller reports the violation regardless and puts the reason in the message.
func TestUnqualifyDeclines(t *testing.T) {
	tests := []struct{ name, ns string }{
		// Reachable: there is a label, it is not wanted here, and no rename
		// can be derived. These become "rename it by hand" diagnostics.
		{"userType", "user"}, // would leave the keyword "type"
		{"userFunc", "user"}, // would leave the keyword "func"
		{"user2", "user"},    // would leave "2", which cannot start an identifier
		// The label is confirmed by the word break inside the namespace, but
		// what follows is a fragment, so there is no clean place to cut.
		{"userIdcache", "userID"},
		// Identical to the namespace in another spelling: checkUnqualify exempts
		// it, and nothing would remain anyway.
		{"userId", "userID"},

		// Unreachable: checkUnqualify gates these out before calling, so they
		// only pin that the function stays total rather than returning a
		// nonsense rename for input it was not designed for.
		{"user", "user"},   // identical to the namespace, so carries no label
		{"users", "user"},  // not a word boundary, so never a label
		{"helper", "user"}, // does not begin with the namespace
		{"anything", ""},   // a file whose name yields no namespace
	}
	for _, tt := range tests {
		got, why := namespace.Unqualify(tt.name, tt.ns)
		if got != "" {
			t.Errorf("Unqualify(%q, %q) = %q, want no rename", tt.name, tt.ns, got)
		}
		if why == "" {
			t.Errorf("Unqualify(%q, %q) declined without explaining itself", tt.name, tt.ns)
		}
	}
}
