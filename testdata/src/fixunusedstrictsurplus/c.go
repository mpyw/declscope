//declscope:private

package fixunusedstrictsurplus

// cLocal's directive restates its block's, which cTaker keeps in use.
// Deleting it would make cLocal one of the block's dependents, which surplus
// judges.
//
//declscope:shared // want `//declscope:shared on cTaker: no use from another namespace is visible to declscope`
var (
	cTaker = 1
	//declscope:shared // want `unused //declscope:shared on cLocal: it already has shared scope` `//declscope:shared on cLocal: no use from another namespace is visible to declscope`
	cLocal = 2
)

func cOther() int { return 3 }

var _ = cTaker + cLocal + cOther()
