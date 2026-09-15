package surpluscarrier

// Sealed is the sealed-interface idiom: the requirement travels with the
// exported interface, and an importer that embeds it passes it on.
type Sealed interface {
	Do()
	//declscope:package
	seal()
}
