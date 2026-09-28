package surplussatisfiespairs

//declscope:package
type cell[T any] struct{}

//declscope:package
func (cell[T]) peek() (v T) { return v }

//declscope:package
func (*cell[T]) poke(T) {}
