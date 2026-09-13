package exportedscope

func useRun(c *Config, e *entry) int {
	c.Timeout = 1
	_ = e.Key
	return Seal()
}

var _ = useRun
