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

// Restating the scope already in force is NOT reported. The test is quantified
// over configurations rather than read off the current one: defaults.unexported
// may be either scope, so nothing about this could have gone another way only
// because of how the file is set today.
//
//declscope:private
func restates() int { return 2 }

var _ = restates

// A block directive that every spec overrides reaches no declaration, the same
// as one written where nothing checked can carry it — and the report has to
// separate the two, since one line is redundant and the other is misplaced.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches states its own scope`
var (
	//declscope:private
	seed = 1
	//declscope:private
	limit = 2
)

// One spec taking the block's scope is enough to keep it: nothing is reported
// here.
//
//declscope:package
var (
	//declscope:private
	other  = 3
	shared = 4
)

// Written where no checked declaration can carry it.
//
//declscope:package // want `unused //declscope:package: no checked declaration carries it`
func init() {}

// The blank function is declared but never checked, like init.
//
//declscope:package // want `unused //declscope:package: no checked declaration carries it`
func _() {}

var _ = seed + limit + other + shared
