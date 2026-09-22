package surplusstrictbaselined

func orderRun(c userCard, k boxKind) int { return c.id + int(k) }

var _ = orderRun
