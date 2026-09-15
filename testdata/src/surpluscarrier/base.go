package surpluscarrier

// Base is exported: an importer can embed it, inherit run, and complete an
// interface satisfaction this analysis never sees.
type Base struct{}

//declscope:package
func (Base) run() {}
