package linefix

// A //line directive without a column leaves the adjusted column at 0. The
// field does not start its line, so the fix still breaks the line first.
//
//line user.tmpl:1
type kept struct{ flag bool } // want `field kept.flag is private to namespace "user", but is used from namespace "order"`

func UserMake() kept { return kept{flag: true} }
