package generics

// List is exported, but its unexported members are still bounded by the
// namespace declaring it. go/types records the instantiated member for a
// selection on List[int] — and on List[T] inside the type's own methods —
// so the lookup has to go through the origin object.
type List[T any] struct {
	items []T // want `field List.items is private to namespace "list", but is used from namespace "use"`
	// size is only touched inside this namespace.
	size int
}

func (l *List[T]) push(v T) { // want `method List.push is private to namespace "list", but is used from namespace "use"`
	l.items = append(l.items, v)
	l.size++
}

// Len is exported and therefore public.
func (l *List[T]) Len() int { return l.size }

// listNode is a generic type private to this namespace.
type listNode[T any] struct { // want `type listNode is private to namespace "list", but is used from namespace "use"`
	val T // want `field listNode.val is private to namespace "list", but is used from namespace "use"`
}
