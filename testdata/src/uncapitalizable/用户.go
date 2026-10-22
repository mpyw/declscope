package uncapitalizable

// 用户 has no capital form, so an exported name could neither carry it nor take
// it as a prefix and stay exported. The naming rule asks nothing here, as for
// 2fa.go; it used to tell the author to rename Load to 用户Load, which
// unexports it.
func Load() int { return 1 }

func helper() int { return 2 }

var _ = helper
