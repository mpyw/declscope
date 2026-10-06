package surplussatisfiespairs

//declscope:shared
type cell[T any] struct{}

//declscope:shared
func (cell[T]) peek() (v T) { return v }

//declscope:shared
func (*cell[T]) poke(T) {}
