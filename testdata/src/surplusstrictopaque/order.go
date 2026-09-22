package surplusstrictopaque

func orderRun(c userCard, b userBox) int { return c.id + len([]any{b}) }

var _ = orderRun
