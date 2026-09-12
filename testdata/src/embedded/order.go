package embedded

type Order struct {
	userCount
	*userTotal
}

// Count reaches n through the embedding, which is a use of userCount.n.
func (o *Order) Count() int { return o.n }
