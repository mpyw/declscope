package unusedscope

// Nothing beneath an exported func takes a scope, so the directive binds
// nothing and is reported.
//
//declscope:package // want `unused //declscope:package on Helper: nothing it reaches takes a scope`
func Helper() int { return 1 }

// An exported type whose every field is exported is the same case.
//
//declscope:package // want `unused //declscope:package on Box: nothing it reaches takes a scope`
type Box struct{ Name string }

// But an exported type with an unexported field does have a subject beneath it,
// and the directive lives through it. This is the whole of the carve-out.
//
//declscope:package
type Entry struct{ key string }

// Restating the scope already in force is NOT reported. The test is structural,
// not semantic: asking whether the scope differs would make recording a
// deliberate private an error, and would turn one line of .declscope.yaml into
// a diagnostic on every directive in the tree.
//
//declscope:private
func restates() int { return 2 }

var _ = restates
