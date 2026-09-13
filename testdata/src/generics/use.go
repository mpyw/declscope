package generics

// drain is grown on List from another namespace. It belongs to the file that
// wrote it, so this file may call it; what it reaches for does not. The
// receiver is the generic List[T], which must still resolve to List, so that
// the field selection below is recognized as a use of List.items — that is the
// crossing, and it is reported against list.go.
func (l *List[T]) drain() []T {
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
