package fixrenamed

// userLoad is read from order.go. The renamed keyword still widens it, so
// that use is not reported, and the fix rewrites the keyword alone.
//
//declscope:package // want `//declscope:package is renamed //declscope:shared; it still means shared`
func userLoad() int { return 1 }

// userSave keeps its reason through the rewrite.
//
//declscope:package // order.go writes through it // want `//declscope:package is renamed //declscope:shared; it still means shared`
func userSave() int { return 2 }

// UserName is exported, so the directive restates its scope. Its deletion is
// offered instead of the rename, since both would edit the one comment.
//
//declscope:package // want `unused //declscope:shared on UserName: it already has shared scope`
func UserName() string { return "" }
