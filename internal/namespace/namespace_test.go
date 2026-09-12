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
