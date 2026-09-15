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
		// 実測で「接頭辞を強いると語が二重になる」形。すべて通るべき。
		{"statementReducer", "reducer", true},
		{"NewRegistry", "registry", true},
		{"LoadConfig", "config", true},
		{"NewTracer", "tracer", true},
		{"hasDirective", "directive", true},
		{"AWSScope", "scope", true},
		{"BuildIgnoreMap", "ignore", true},
		// 語の派生。右端が語の途中で終わる。
		{"SpecifierParser", "parse", true},
		{"CheckConflicts", "conflict", true},
		{"APIParser", "parse", true},
		// ファイル stem と識別子でトークン分割がずれる場合。
		{"NewAzureAppConfigParamStrategy", "azureAppconfigParam", true},
		// 名前空間そのもの。
		{"collect", "collect", true},
		// 綴りが変わる活用形。名前空間の側から生成した完全形だけを受理する。
		{"storing", "store", true},
		{"storingWorker", "store", true},
		{"NewStoring", "store", true},
		{"parsing", "parse", true},
		{"ignoringMap", "ignore", true},
		{"applies", "apply", true},
		{"appliedResults", "apply", true},
		{"applying", "apply", true}, // apply がそのまま含まれる。生成形ではなく素の含有
		// 複合語の名前空間では末尾の語だけが活用する。
		{"userStoring", "userStore", true},
		// 語幹化の誤受理。stor を右端自由で照合すると通ってしまう形で、
		// 生成側方式ではすべて落ちる。
		{"story", "store", false},
		{"storm", "store", false},
		{"stories", "store", false},
		{"userStory", "userStore", false},
		{"appliance", "apply", false},
		// 母音の後の e / y は綴りが変わらないので何も生成しない。
		{"freeing", "free", true}, // free + ing。素の含有で通る
		{"freing", "free", false},
		{"deploys", "deploy", true}, // deploy + s。素の含有で通る
		{"deploies", "deploy", false},
		// 左端が語境界でない。通ってはいけない。
		{"monkey", "key", false},
		{"UntagCommand", "tag", false},
		// 名前空間をまったく含まない。
		{"Wrap", "client", false},
		{"nounSecret", "command", false},
		// 左端固定の代償として受理されるもの。意図的。
		{"models", "mode", true},
	}
	for _, tt := range tests {
		if got := namespace.Contains(tt.name, tt.ns); got != tt.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", tt.name, tt.ns, got, tt.want)
		}
	}
}
