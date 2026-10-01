package ignoreunseen

// aHelper is crossed into only by tagged.go, which the build leaves out. A
// configuration that reads that file reports the crossing, and the ignore
// answers it there, so it is not unused here.
//
//declscope:ignore boundary // tagged.go calls it behind the never tag
func aHelper() int { return 1 }

// A bare ignore covers boundary too.
//
//declscope:ignore
func aBare() int { return 2 }

// No unseen file names aStale, so its ignore silences nothing anywhere.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on aStale`
func aStale() int { return 3 }

// qualify is judged on the declaration, not on its uses, so an unseen file
// naming aQualify cannot make its ignore needed.
//
//declscope:ignore qualify // want `unused //declscope:ignore qualify on aQualify`
func aQualify() int { return 4 }

func A() int { return aHelper() + aBare() + aStale() + aQualify() }

// tagged.go writes an aPoint by position, which crosses into x without
// naming it. The type's name counts for its fields.
type aPoint struct {
	//declscope:ignore boundary // tagged.go writes it by position
	x int
}

// tagged.go writes an aCoord through an alias, which counts too.
type aCoord struct {
	//declscope:ignore boundary // tagged.go writes it by position, through aAlias
	y int
}

type aAlias = aCoord

// Nothing unseen names aPair or its field, so the ignore silences nothing.
type aPair struct {
	//declscope:ignore boundary // want `unused //declscope:ignore boundary on aPair.z`
	z int
}

func B() int { return aPoint{x: 1}.x + aAlias{y: 2}.y + aPair{z: 3}.z }
