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

		// No valid identifier can start with a digit, so the file has no
		// namespace and can only promote declarations with a directive.
		{"2fa_auth.go", ""},
	}
	for _, tt := range tests {
		if got := namespace.Of(tt.path); got != tt.want {
			t.Errorf("Of(%q) = %q, want %q", tt.path, got, tt.want)
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
		{"cache", "user", false},
		{"anything", "", false},
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
		{"user", "user"},     // nothing would remain
		{"userType", "user"}, // would leave the keyword "type"
		{"userFunc", "user"}, // would leave the keyword "func"
		{"user2", "user"},    // would leave "2", which cannot start an identifier

		// Unreachable: checkDemote gates on HasPrefix, so these never arrive.
		// They pin that the function is total, rather than returning a
		// nonsense rename for input it was not designed for.
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
