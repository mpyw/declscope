package surplussatisfies

// The type's directive reaches v too; both are spelled from use.go, which is
// what keeps this comment. Only get is reached through the contract alone.
//
//declscope:package
type box[T any] struct{ v T }

//declscope:package
func (b box[T]) get() T { return b.v }
