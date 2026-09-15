package surpluscarrier

type hidden struct{}

//declscope:package
func (hidden) tick() {}

// Wrapper carries tick out of the package through the embedding.
type Wrapper struct{ hidden }
