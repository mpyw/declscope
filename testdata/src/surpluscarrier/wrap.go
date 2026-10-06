package surpluscarrier

type hidden struct{}

//declscope:shared
func (hidden) tick() {}

// Wrapper carries tick out of the package through the embedding.
type Wrapper struct{ hidden }
