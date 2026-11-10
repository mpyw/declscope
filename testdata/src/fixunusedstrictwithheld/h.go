package fixunusedstrictwithheld

// g_other.go writes an hSlot through a named pointer, as []hPSlot{{1}}, which
// fills w by position and names only hPSlot. The crossing into w may be there.
type hSlot struct {
	//declscope:private // want `unused //declscope:private on hSlot.w: it already has private scope`
	w int
}

type hPSlot *hSlot

var _ = hSlot{w: 1}.w
