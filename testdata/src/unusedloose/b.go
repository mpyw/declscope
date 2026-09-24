//declscope:private // want `unused file-level //declscope:private`

package directiveloose

// The spec's private narrows its block's package. Deleting it would widen the
// spec, so it binds, whatever the file says. The block's directive and the
// file's reach no declaration that takes their scope, and are reported.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches states its own scope`
var (
	//declscope:private
	bNarrowed = 1
)

var _ = bNarrowed
