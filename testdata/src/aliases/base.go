package aliases

// Base is exported, so alias.go naming it crosses nothing. Its unexported
// field is still bounded by this file, and no directive written on an alias
// reaches it.
type Base struct {
	n int // want `field Base.n is private to namespace "base", but is used from namespace "use"`
}
