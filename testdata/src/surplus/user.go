package surplus

// Reported: no other namespace is seen to use it, and a use from the same
// namespace keeps nothing.
//
//declscope:package // want `//declscope:package on userSeed: no use from another namespace is visible to declscope`
var userSeed = 1

var _ = userSeed

// One comment reaches every spec and is judged once, so one report lists both.
//
//declscope:package // want `//declscope:package on userLimit, userCap: no use from another namespace is visible to declscope`
var (
	userLimit = 2
	userCap   = 3
)

// A type's directive also reaches its fields, and the report lists them.
//
//declscope:package // want `//declscope:package on userBox, userBox.item: no use from another namespace is visible to declscope`
type userBox struct{ item int }

//declscope:package // want `//declscope:package on userBox.zero: no use from another namespace is visible to declscope`
func (userBox) zero() int { return 0 }

// An ignore silences the rule like any other.
//
//declscope:package
//declscope:ignore surplus
var userQuiet = 4

// Key is exported, so importers reach it without this analysis seeing them.
// The whole comment stays quiet rather than advise against that doubt.
//
//declscope:package
type userEntry struct{ Key string }
