package surplusstrict

func readerUse(c *fileCursor) int {
	fileAdvance(c)
	return c.pos
}

var _ = readerUse
