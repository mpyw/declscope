package surplusstrictbaselined

// old is recorded in the baseline, so it stays quiet. fresh is not.
//
//declscope:shared
type userCard struct {
	id    int
	old   int
	fresh int // want `field userCard.fresh takes shared scope from //declscope:shared on userCard, but no use from another namespace is visible to declscope`
}

func userRead(c userCard) int { return c.old + c.fresh }

var _ = userRead
