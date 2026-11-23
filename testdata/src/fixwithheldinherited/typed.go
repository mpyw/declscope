package fixwithheldinherited

// The member takes private from its type alone.
//
//declscope:private
type typedBox struct {
	n int // want `field typedBox.n is declared private by //declscope:private on typedBox, but is used from namespace "order"`
}

func TypedMake() typedBox { return typedBox{} }
