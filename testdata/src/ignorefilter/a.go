//declscope:ignore filter // want `unused file-level //declscope:ignore filter`

package ignorefilter

// The filter rule reports a chain of config files, and the package it reports
// had nothing read from it, so no ignore there is ever read. An ignore naming
// the rule silences nothing, and the unused rule reports it like any other.
func helper() int { return 1 }

var _ = helper()
