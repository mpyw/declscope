//declscope:private

package fixunusedstrictsurplus

// cLocal's directive restates its block's, which cTaker keeps in use.
// Deleting it would make cLocal one of the block's dependents, which surplus
// judges.
//
//declscope:package // want `//declscope:package on cTaker: no use from another namespace is visible to declscope`
var (
	cTaker = 1
	//declscope:package // want `unused //declscope:package on cLocal: it already has package scope` `//declscope:package on cLocal: no use from another namespace is visible to declscope`
	cLocal = 2
)

func cOther() int { return 3 }

var _ = cTaker + cLocal + cOther()
