package generics

// drain is grown on List from another namespace. Its receiver is the generic
// List[T], which must still resolve to List, so that the method is bounded by
// List's namespace rather than by the file declaring it.
func (l *List[T]) drain() []T { // want `method List.drain is private to namespace "list", but is used from namespace "use"`
	out := l.items
	l.items = nil
	return out
}

func Use(l *List[int]) int {
	l.push(1)
	n := listNode[int]{val: 1}
	_ = l.drain()
	return len(l.items) + n.val + l.Len()
}
