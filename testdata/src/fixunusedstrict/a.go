//declscope:private // want `unused file-level //declscope:private: every declaration it reaches already has private scope`

package fixdirectivestrict

// aDoc has a doc comment, separated from the directive by a bare line that
// the fix takes along.
//
//declscope:private // want `unused //declscope:private on aDoc: it already has private scope`
func aDoc() int { return 1 }

//declscope:private // want `unused //declscope:private on aBare: it already has private scope`
func aBare() int { return 2 }

// aIgnored keeps its ignore: only the scope directive goes, and the bare line
// still separates the doc comment from what is left.
//
//declscope:private // want `unused //declscope:private on aIgnored: it already has private scope`
//declscope:ignore qualify // want `unused //declscope:ignore qualify on aIgnored`
func aIgnored() int { return 3 }

type aShape struct {
	side int //declscope:private // want `unused //declscope:private on aShape.side: it already has private scope`

	// edge is documented.
	//
	//declscope:private // want `unused //declscope:private on aShape.edge: it already has private scope`
	edge int
}

//declscope:private // want `unused //declscope:private on aSeed, aLimit: each already has private scope`
var (
	aSeed  = 1
	aLimit = 2
)

var _ = aDoc() + aBare() + aIgnored() + aShape{}.side + aShape{}.edge + aSeed + aLimit
